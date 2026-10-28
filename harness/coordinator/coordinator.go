// Package coordinator defines the owner of the central event loop.
package coordinator

import (
	"context"
	"fmt"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/contextbuilder"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

type Dependencies struct {
	// CompactionFailureLimit bounds consecutive unusable summaries. Zero uses 3.
	// A recorded session policy takes precedence on resume.
	CompactionFailureLimit int
	ToolHeartbeatInterval  time.Duration
	// PendingWork observes owner-authoritative work outside operation envelopes.
	// WorkChanged wakes the loop when that ownership or producer traffic changes.
	// Nil preserves ordinary standalone behavior. Neither is snapshot permission.
	PendingWork func() bool
	WorkChanged <-chan struct{}
	// ActivitySummary is a bounded, redacted, snapshot-only callback; it must not perform I/O.
	ActivitySummary func() string
	// ControlScope binds public lifecycle intents to this runtime generation.
	ControlScope   string
	SessionID      session.ID
	Inbox          *inbox.Inbox
	Restored       sessionstore.ResumeState
	Sessions       sessionstore.Store
	ContextBuilder contextbuilder.Builder
	LLM            llm.Adapter
	Tools          tool.Registry
	Operations     operation.Manager
}

type Coordinator interface {
	// Run owns one session's decision loop until an ordinary stop completes or
	// the context is canceled. It returns nil for a completed stop. It is
	// single-use; its caller must cancel and join the Inbox when Run returns.
	Run(context.Context) error
}

func New(dependencies Dependencies) Coordinator {
	return &coordinator{
		dependencies: dependencies,
		state:        newLoopState(),
	}
}

// ModelFailureError reports a terminal provider failure after its response and
// usage have been durably recorded. It is not used for refusals or compaction.
type ModelFailureError struct {
	TurnID  session.TurnID
	Failure llm.Failure
}

func (err *ModelFailureError) Error() string {
	return fmt.Sprintf("model failed for turn %q (%s): %s", err.TurnID, err.Failure.Code, err.Failure.Message)
}

func terminalModelFailure(response llm.Response) bool {
	return response.Failure != nil && response.Stop == ""
}
