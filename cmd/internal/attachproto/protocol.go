// Package attachproto defines the local same-UID attach protocol. It is separate
// from the private worker pipe protocol and never accepts worker bootstrap data.
package attachproto

import (
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

const (
	Version         = 1
	MaxFrame        = 1 << 20
	MaxPayload      = 256 << 10
	MaxQueuedFrames = 256
	MaxQueuedBytes  = 8 << 20
)

// Cursor identifies a host-published checkpoint, not client write authority.
type Cursor struct {
	Incarnation string
	Revision    uint64
	Head        sessionstore.Sequence
}

type Request struct {
	Cursor     *Cursor `json:",omitzero"`
	Type       string
	ID         string
	Protocol   int
	SessionID  session.ID
	Connection string                `json:",omitzero"`
	After      sessionstore.Sequence `json:",omitzero"`
	Input      *inbox.Input          `json:",omitzero"`
	Runtime    *RuntimeRequest       `json:",omitzero"`
}

type Status struct {
	Generation uint64
	Build      string
	Stopped    bool
	Error      string `json:",omitzero"`
}

// Snapshot state is explicitly host-published, not the newest filesystem head.
// Operations/activity are unavailable until their separate bridges are shipped.
type Event struct {
	Runtime             *RuntimeStatus `json:",omitzero"`
	Base                *Cursor        `json:",omitzero"`
	Type                string
	ID                  string `json:",omitzero"`
	Protocol            int
	SessionID           session.ID
	Incarnation         string `json:",omitzero"`
	Connection          string `json:",omitzero"`
	HostPID             int    `json:",omitzero"`
	Revision            uint64
	Head                sessionstore.Sequence
	Item                *sessionstore.Item `json:",omitzero"`
	Pending             []inbox.Input      `json:",omitzero"`
	PendingAvailable    bool
	Status              *Status    `json:",omitzero"`
	Model               *llm.Model `json:",omitzero"`
	OperationsAvailable bool
	ActivityAvailable   bool
	Capabilities        []string `json:",omitzero"`
	InputID             inbox.ID `json:",omitzero"`
	Code                string   `json:",omitzero"`
	Error               string   `json:",omitzero"`
}

// Transport implementation API (kept here as a shared implementation contract):
// Encode(value any) ([]byte, error): deterministic JSONL, MaxFrame incl newline.
// Write(io.Writer, any) error: complete frame, short-write handling.
// Scanner(io.Reader) *bufio.Scanner: bounded frames, rejects incomplete tail.
// ReadRequest(*bufio.Scanner) (Request,error), ReadEvent(*bufio.Scanner) (Event,error):
// strict unknown-field rejection and no trailing/partial JSON; EOF preserved.
// EndpointPath(runtimeDir,stateDir string,id session.ID) (string,error).
// Listen(runtimeDir,stateDir string,id session.ID) (*Endpoint,error): caller MUST
// own workerhost ownership first. Endpoint {Listener *net.UnixListener; Path string}
// has idempotent Close() error that checks inode identity before removing socket.
// Dial(context.Context,runtimeDir,stateDir string,id session.ID) (*net.UnixConn,error).
// CheckPeer(*net.UnixConn) error: same effective UID, both peers, fail closed.
