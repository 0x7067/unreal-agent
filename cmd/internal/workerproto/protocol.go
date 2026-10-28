// Package workerproto defines the private, versioned TUI/worker pipe protocol.
// It is not the public one-shot runner protocol.
package workerproto

import (
	"bufio"
	"bytes"
	"encoding/json/v2"
	"fmt"
	"io"

	"github.com/unreallabsai/unreal-agent/cmd/internal/subagent"
	"github.com/unreallabsai/unreal-agent/harness/executionbudget"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

// Version 8 replaces idle-only quiesce with ordered natural drain, scoped
// interventions and owner-authoritative completion. Old peers are rejected.
const Version = 8
const SessionVersion = 2
const MaxFrame = 32 << 20

// Identity is available without loading config/auth, opening a session, or
// contacting a provider. Compatibility is checked before retiring a worker.
type Identity struct {
	Protocol       int
	SessionVersion int
	Build          string
}

func (i Identity) Validate() error {
	if i.Protocol != Version || i.SessionVersion != SessionVersion {
		return fmt.Errorf("incompatible worker: protocol %d, session version %d", i.Protocol, i.SessionVersion)
	}
	return nil
}

type Bootstrap struct {
	ControlScope          string `json:",omitzero"`
	SessionID             session.ID
	Directory             string
	Workspace             string
	Provider              string
	BaseURL               string
	Model                 llm.Model
	MaxAttempts           int
	Shell                 string
	Subagents             bool
	DurableSubagents      bool
	DurableGraphs         bool
	ChildRuntimeDirectory string
	DurableChildCommand   []string
	SubagentCommand       []string
	SystemPrompt          string
	Budget                executionbudget.Policy
	MaxOperations         int
}

type Request struct {
	Type      string // start, input, activity_snapshot, drain, stop
	ID        string
	Protocol  int
	Bootstrap *Bootstrap   `json:",omitzero"`
	Input     *inbox.Input `json:",omitzero"`
}

type Event struct {
	Type      string // hello, item, ready, activity_snapshot, drained, error
	ID        string
	Identity  *Identity                   `json:",omitzero"`
	Item      *sessionstore.Item          `json:",omitzero"`
	Error     string                      `json:",omitzero"`
	Resources *operation.ActivitySnapshot `json:",omitzero"`
	Activity  *subagent.FleetSnapshot     `json:",omitzero"`
}

// Write emits one complete frame. Callers must serialize writes.
func Write(w io.Writer, value any) error {
	data, err := json.Marshal(value)
	if err != nil {
		return err
	}
	if len(data) > MaxFrame {
		return fmt.Errorf("worker frame exceeds %d bytes", MaxFrame)
	}
	data = append(data, '\n')
	for len(data) != 0 {
		n, err := w.Write(data)
		if err != nil {
			return err
		}
		if n <= 0 {
			return io.ErrShortWrite
		}
		data = data[n:]
	}
	return nil
}

// Scanner bounds each JSONL frame; consumers must reject invalid frames.
func Scanner(r io.Reader) *bufio.Scanner {
	scanner := bufio.NewScanner(r)
	scanner.Split(func(data []byte, atEOF bool) (int, []byte, error) {
		// A complete JSON value without its framing newline is still an
		// incomplete transport write; never accept it as an acknowledged frame.
		if atEOF && len(data) > 0 && bytes.IndexByte(data, '\n') < 0 {
			return 0, nil, io.ErrUnexpectedEOF
		}
		return bufio.ScanLines(data, atEOF)
	})
	scanner.Buffer(make([]byte, 64<<10), MaxFrame+1)
	return scanner
}
