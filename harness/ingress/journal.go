// Package ingress provides a parent-owned, durable input admission journal.
// A successful Submit means the admission has been fsynced, not that any external
// effect has occurred. Acknowledge records completion independently. Identities,
// including acknowledged identities, are retained forever; there is no compaction.
package ingress

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"sync"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/session"
)

// MaxRecordBytes includes the terminating newline and the base64-encoded payload.
const MaxRecordBytes = 4 << 20

var (
	ErrLocked    = errors.New("ingress journal is already owned")
	ErrClosed    = errors.New("ingress journal is closed")
	ErrConflict  = errors.New("ingress input identity conflicts with an earlier admission")
	ErrUncertain = errors.New("ingress journal durability is uncertain; close and reopen to recover")
)

type durableFile interface {
	io.Reader
	io.Writer
	Sync() error
	Truncate(int64) error
	Close() error
}

type identity struct {
	input        inbox.Input
	acknowledged bool
}

// Journal serializes all methods, including Close. After a write or fsync error,
// mutations fail with ErrUncertain until reopened. Pending then reflects only
// confirmed operations, and must not be interpreted as a complete recovery view.
// Context cancellation is checked before starting I/O; once started, the durable
// outcome wins over cancellation. The caller must not mutate an input concurrently
// with a method call. Returned and retained payloads do not alias caller payloads.
type Journal struct {
	mu       sync.Mutex
	file     durableFile
	lock     *os.File
	entries  map[inbox.ID]*identity
	order    []inbox.ID
	failure  error
	closed   bool
	closeErr error
}

// Open exclusively owns directory/id until Close or process death. Both the
// storage directory and session directory must be private (0700); new directories
// and files are created with modes 0700 and 0600. Symlinks are never followed.
// The stable .lock inode must not be removed, even when the journal is closed.
func Open(directory string, id session.ID) (*Journal, error) {
	if !validSessionID(string(id)) {
		return nil, fmt.Errorf("invalid ingress session ID %q", id)
	}
	file, lock, err := openStorage(directory, string(id))
	if err != nil {
		return nil, err
	}
	j := &Journal{file: file, lock: lock, entries: make(map[inbox.ID]*identity)}
	if err := j.recover(); err != nil {
		_ = j.Close()
		return nil, fmt.Errorf("recover ingress: %w", err)
	}
	return j, nil
}

func validSessionID(s string) bool {
	if len(s) == 0 || len(s) > 128 || s == "." || s == ".." {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return false
		}
	}
	return true
}

// diskInput uses base64 bytes rather than embedded JSON so payload identity is
// byte-exact, including insignificant whitespace, across a restart.
type diskInput struct {
	ID      inbox.ID        `json:"id"`
	Kind    inbox.InputKind `json:"kind"`
	Payload *[]byte         `json:"payload"`
}
type record struct {
	Version   int       `json:"version"`
	Operation string    `json:"operation"`
	Input     diskInput `json:"input"`
}

func clone(input inbox.Input) inbox.Input { input.Payload = input.Payload.Clone(); return input }
func same(a, b inbox.Input) bool          { return a.Kind == b.Kind && bytes.Equal(a.Payload, b.Payload) }
func (j *Journal) available(ctx context.Context) error {
	if j.closed {
		return ErrClosed
	}
	if j.failure != nil {
		return j.failure
	}
	return context.Cause(ctx)
}

func (j *Journal) Submit(ctx context.Context, input inbox.Input) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if err := j.available(ctx); err != nil {
		return err
	}
	if len(input.ID) > MaxRecordBytes || len(input.Payload) > MaxRecordBytes {
		return errors.New("ingress input exceeds record size limit")
	}
	if err := input.Validate(); err != nil {
		return fmt.Errorf("ingress input: %w", err)
	}
	if previous, ok := j.entries[input.ID]; ok {
		if !same(previous.input, input) {
			return fmt.Errorf("%w: %q", ErrConflict, input.ID)
		}
		return nil
	}
	input = clone(input)
	if err := j.append("submit", input); err != nil {
		return err
	}
	j.entries[input.ID] = &identity{input: input}
	j.order = append(j.order, input.ID)
	return nil
}

// Pending returns independent copies in first-admission order. Acknowledged IDs
// never become pending again, even if submitted again with identical contents.
func (j *Journal) Pending() []inbox.Input {
	j.mu.Lock()
	defer j.mu.Unlock()
	result := make([]inbox.Input, 0)
	for _, id := range j.order {
		if e := j.entries[id]; !e.acknowledged {
			result = append(result, clone(e.input))
		}
	}
	return result
}

func (j *Journal) Acknowledge(ctx context.Context, input inbox.Input) error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if err := j.available(ctx); err != nil {
		return err
	}
	previous, ok := j.entries[input.ID]
	if !ok {
		return nil
	}
	if !same(previous.input, input) {
		return fmt.Errorf("%w: %q", ErrConflict, input.ID)
	}
	if previous.acknowledged {
		return nil
	}
	if err := j.append("acknowledge", previous.input); err != nil {
		return err
	}
	previous.acknowledged = true
	return nil
}

func (j *Journal) append(operation string, input inbox.Input) error {
	var payload *[]byte
	if input.Payload != nil {
		value := []byte(input.Payload)
		payload = &value
	}
	data, err := json.Marshal(record{Version: 1, Operation: operation, Input: diskInput{ID: input.ID, Kind: input.Kind, Payload: payload}})
	if err != nil {
		return fmt.Errorf("encode ingress record: %w", err)
	}
	// Reserve the longer acknowledgement spelling at admission, so every
	// accepted input can also be acknowledged within the same record bound.
	encodedSize := len(data)
	if operation == "submit" {
		encodedSize += len("acknowledge") - len("submit")
	}
	if encodedSize >= MaxRecordBytes {
		return errors.New("ingress input exceeds record size limit")
	}
	data = append(data, '\n')
	n, err := j.file.Write(data)
	if err == nil && n != len(data) {
		err = io.ErrShortWrite
	}
	if err == nil {
		err = j.file.Sync()
	}
	if err != nil {
		j.failure = fmt.Errorf("%w: %w", ErrUncertain, err)
		return j.failure
	}
	return nil
}

func (j *Journal) recover() error {
	reader := bufio.NewReaderSize(j.file, 64<<10)
	var offset int64
	for {
		var line []byte
		var size int64
		for {
			chunk, err := reader.ReadSlice('\n')
			size += int64(len(chunk))
			if size <= MaxRecordBytes {
				line = append(line, chunk...)
			}
			if err == bufio.ErrBufferFull {
				continue
			}
			if err == io.EOF {
				if size == 0 {
					return nil
				}
				// Only the suffix without a newline is uncommitted. Never rewrite any
				// complete record, even if the suffix happens to contain valid JSON.
				if err := j.file.Truncate(offset); err != nil {
					return fmt.Errorf("truncate incomplete tail: %w", err)
				}
				if err := j.file.Sync(); err != nil {
					return fmt.Errorf("sync truncated tail: %w", err)
				}
				return nil
			}
			if err != nil {
				return err
			}
			if size > MaxRecordBytes {
				return fmt.Errorf("record at byte %d exceeds size limit", offset)
			}
			if err := j.replay(line); err != nil {
				return fmt.Errorf("record at byte %d: %w", offset, err)
			}
			offset += size
			break
		}
	}
}

func (j *Journal) replay(line []byte) error {
	var r record
	if err := json.Unmarshal(line, &r, json.RejectUnknownMembers(true)); err != nil {
		return err
	}
	if r.Version != 1 {
		return fmt.Errorf("unsupported record version %d", r.Version)
	}
	input := inbox.Input{ID: r.Input.ID, Kind: r.Input.Kind}
	if r.Input.Payload != nil {
		input.Payload = *r.Input.Payload
	}
	if err := input.Validate(); err != nil {
		return err
	}
	previous, exists := j.entries[input.ID]
	switch r.Operation {
	case "submit":
		if exists {
			if !same(previous.input, input) {
				return ErrConflict
			}
			return nil
		}
		j.entries[input.ID] = &identity{input: input}
		j.order = append(j.order, input.ID)
	case "acknowledge":
		if !exists {
			return errors.New("acknowledgement without admission")
		}
		if !same(previous.input, input) {
			return ErrConflict
		}
		previous.acknowledged = true
	default:
		return fmt.Errorf("unsupported operation %q", r.Operation)
	}
	return nil
}

func (j *Journal) Close() error {
	j.mu.Lock()
	defer j.mu.Unlock()
	if j.closed {
		return j.closeErr
	}
	j.closed = true
	// Closing the data descriptor before releasing ownership prevents another
	// process from opening while an old descriptor can still be written.
	j.closeErr = errors.Join(j.failure, j.file.Close(), releaseLock(j.lock))
	return j.closeErr
}
