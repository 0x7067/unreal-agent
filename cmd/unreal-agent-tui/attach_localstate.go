package main

// This file stores client-local presentation state and separate retained
// external/control intents. Nothing here is a canonical cache, an automatic
// queue, or a source of session-journal writes. Retries are always explicit.
import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/google/uuid"
	"golang.org/x/sys/unix"
)

const (
	attachLocalStateVersion   = 2
	attachLocalTextLimit      = 256 << 10
	attachLocalAggregateLimit = 8 << 20
	attachLocalFileLimit      = 9 << 20
)

type attachLocalState struct {
	Version                                        int
	Namespace                                      string
	Draft                                          string
	ConversationOffset, ToolsOffset, DetailsOffset int
	SelectedCallID, DetailsCallID                  string
	ToolsFocused, ToolsCollapsed                   bool
	ConversationAnchor                             string
	SelectionPane                                  string
	SelectionStart, SelectionEnd                   int
	Retained                                       []attachLocalInput
	Controls                                       []attachLocalControl `json:",omitzero"`
}
type attachLocalInput struct {
	ID   string
	Text string
}

func validAttachLocalNamespace(s string) bool {
	if len(s) != 64 {
		return false
	}
	for _, c := range s {
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f') {
			return false
		}
	}
	return true
}
func validateAttachLocalState(s attachLocalState) error {
	if (s.Version != 1 && s.Version != attachLocalStateVersion) || !validAttachLocalNamespace(s.Namespace) {
		return errors.New("invalid local state version or namespace")
	}
	for _, n := range []int{s.ConversationOffset, s.ToolsOffset, s.DetailsOffset, s.SelectionStart, s.SelectionEnd} {
		if n < 0 || n > 1000000000 {
			return errors.New("local state offset out of bounds")
		}
	}
	for _, id := range []string{s.SelectedCallID, s.DetailsCallID, s.SelectionPane, s.ConversationAnchor} {
		if len(id) > 128 || !utf8.ValidString(id) {
			return errors.New("invalid local state identifier")
		}
		for _, c := range id {
			if unicode.IsControl(c) {
				return errors.New("control in local state identifier")
			}
		}
	}
	if !utf8.ValidString(s.Draft) || len(s.Draft) > attachLocalTextLimit {
		return errors.New("invalid local draft")
	}
	if len(s.Retained) > 64 {
		return errors.New("too many retained inputs")
	}
	seen := map[string]bool{}
	total := 0
	for _, input := range s.Retained {
		id, err := uuid.Parse(input.ID)
		if err != nil || len(input.ID) != 36 {
			return errors.New("invalid retained input UUID")
		}
		key := id.String()
		if seen[key] {
			return errors.New("duplicate retained input UUID")
		}
		seen[key] = true
		if !utf8.ValidString(input.Text) || len(input.Text) > attachLocalTextLimit {
			return errors.New("invalid retained input text")
		}
		payload, err := json.Marshal(input.Text)
		if err != nil || len(payload) > attachLocalTextLimit {
			return errors.New("retained input payload too large")
		}
		total += len(payload)
	}
	for _, c := range s.Controls {
		if seen[c.ID] {
			return errors.New("external/control retry identity collision")
		}
	}
	if err := validateAttachLocalControls(s); err != nil {
		return err
	}
	if total > attachLocalAggregateLimit {
		return errors.New("retained input aggregate too large")
	}
	return nil
}

// openAttachLocalDirectory walks via directory descriptors: no symlink, even in
// an ancestor, is followed, and later path replacement cannot redirect writes.
// Only the explicit final directory must be private; ancestors may be shared.
func openAttachLocalDirectory(path string, create bool) (*os.File, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path || path == "/" {
		return nil, errors.New("local state directory must be a clean absolute private path")
	}
	fd, err := unix.Open("/", unix.O_RDONLY|unix.O_DIRECTORY|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	for _, part := range strings.Split(strings.TrimPrefix(path, "/"), "/") {
		next, e := unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
		if errors.Is(e, unix.ENOENT) && create {
			e = unix.Mkdirat(fd, part, 0700)
			if e == nil {
				if syncErr := unix.Fsync(fd); syncErr != nil {
					unix.Close(fd)
					return nil, syncErr
				}
			}
			if e == nil || errors.Is(e, unix.EEXIST) {
				next, e = unix.Openat(fd, part, unix.O_RDONLY|unix.O_DIRECTORY|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0)
			}
		}
		unix.Close(fd)
		if e != nil {
			return nil, e
		}
		fd = next
	}
	f := os.NewFile(uintptr(fd), path)
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		f.Close()
		return nil, err
	}
	if st.Uid != uint32(os.Getuid()) || st.Mode&07777 != 0700 {
		f.Close()
		return nil, errors.New("local state directory must be owned by current user and mode 0700")
	}
	return f, nil
}
func openAttachLocalFile(dir *os.File, name string) (*os.File, error) {
	fd, err := unix.Openat(int(dir.Fd()), name, unix.O_RDONLY|unix.O_NOFOLLOW|unix.O_NONBLOCK|unix.O_CLOEXEC, 0)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), name)
	var st unix.Stat_t
	if err := unix.Fstat(fd, &st); err != nil {
		f.Close()
		return nil, err
	}
	if st.Mode&unix.S_IFMT != unix.S_IFREG || st.Mode&07777 != 0600 || st.Uid != uint32(os.Getuid()) {
		f.Close()
		return nil, errors.New("local state file must be a private owned regular file")
	}
	if st.Size > attachLocalFileLimit {
		f.Close()
		return nil, errors.New("local state file too large")
	}
	return f, nil
}
func loadAttachLocalState(directory, namespace string) (attachLocalState, error) {
	var state attachLocalState
	if !validAttachLocalNamespace(namespace) {
		return state, errors.New("invalid local state namespace")
	}
	dir, err := openAttachLocalDirectory(directory, false)
	if err != nil {
		return state, err
	}
	defer dir.Close()
	f, err := openAttachLocalFile(dir, namespace+".json")
	if err != nil {
		return state, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, attachLocalFileLimit+1))
	if err != nil {
		return state, err
	}
	if len(data) > attachLocalFileLimit || !utf8.Valid(data) {
		return state, errors.New("invalid or oversized local state JSON")
	}
	if err = json.Unmarshal(data, &state, json.RejectUnknownMembers(true)); err != nil {
		return attachLocalState{}, err
	}
	if state.Version == 1 {
		var members map[string]jsontext.Value
		if err := json.Unmarshal(data, &members); err != nil {
			return attachLocalState{}, err
		}
		if _, ok := members["Controls"]; ok {
			return attachLocalState{}, errors.New("version 1 cannot contain lifecycle controls")
		}
	}
	if state.Namespace != namespace {
		return attachLocalState{}, errors.New("local state namespace mismatch")
	}
	if err = validateAttachLocalState(state); err != nil {
		return attachLocalState{}, err
	}
	return state, nil
}
func saveAttachLocalState(directory string, state attachLocalState) error {
	if err := validateAttachLocalState(state); err != nil {
		return err
	}
	data, err := json.Marshal(state, json.Deterministic(true))
	if err != nil {
		return err
	}
	if len(data) > attachLocalFileLimit {
		return errors.New("local state JSON too large")
	}
	dir, err := openAttachLocalDirectory(directory, true)
	if err != nil {
		return err
	}
	defer dir.Close()
	fd := int(dir.Fd())
	name := state.Namespace + ".json"
	check := func() error {
		f, e := openAttachLocalFile(dir, name)
		if errors.Is(e, unix.ENOENT) {
			return nil
		}
		if e != nil {
			return e
		}
		return f.Close()
	}
	if err = check(); err != nil {
		return err
	}
	var random [16]byte
	if _, err = rand.Read(random[:]); err != nil {
		return err
	}
	temp := ".local-state-" + hex.EncodeToString(random[:]) + ".tmp"
	out, err := unix.Openat(fd, temp, unix.O_WRONLY|unix.O_CREAT|unix.O_EXCL|unix.O_NOFOLLOW|unix.O_CLOEXEC, 0600)
	if err != nil {
		return err
	}
	f := os.NewFile(uintptr(out), temp)
	defer func() { f.Close(); _ = unix.Unlinkat(fd, temp, 0) }()
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	if err = check(); err != nil {
		return err
	}
	if err = unix.Renameat(fd, temp, fd, name); err != nil {
		return fmt.Errorf("replace local state: %w", err)
	}
	return dir.Sync()
}

// lockAttachLocalState serializes one namespace's local retry/view writers.
// A disconnected frontend still owns this lease until it exits; otherwise it
// could overwrite a replacement's outbox even though the host fences sockets.
// Never unlink this stable lock inode. Closing the descriptor releases it.
func lockAttachLocalState(directory, namespace string) (*os.File, error) {
	if !validAttachLocalNamespace(namespace) {
		return nil, errors.New("invalid local state namespace")
	}
	dir, err := openAttachLocalDirectory(directory, true)
	if err != nil {
		return nil, err
	}
	defer dir.Close()
	name := namespace + ".lock"
	fd, err := unix.Openat(int(dir.Fd()), name, unix.O_RDONLY|unix.O_CREAT|unix.O_NOFOLLOW|unix.O_CLOEXEC|unix.O_NONBLOCK, 0600)
	if err != nil {
		return nil, err
	}
	f := os.NewFile(uintptr(fd), name)
	// Use the same owner/mode/regular-file validation as snapshots on this exact
	// opened descriptor, without reopening a mutable pathname.
	var st unix.Stat_t
	if err = unix.Fstat(fd, &st); err == nil && (st.Mode&unix.S_IFMT != unix.S_IFREG || st.Mode&0777 != 0600 || st.Uid != uint32(os.Geteuid())) {
		err = errors.New("unsafe local state lock")
	}
	if err == nil {
		err = unix.Flock(fd, unix.LOCK_EX|unix.LOCK_NB)
	}
	if err != nil {
		f.Close()
		return nil, fmt.Errorf("client-local state is busy or unsafe; close its previous frontend or choose a different -client-state-directory: %w", err)
	}
	return f, nil
}
