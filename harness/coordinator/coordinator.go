// Package coordinator defines the owner of the central event loop.
package coordinator

import (
	"context"
	"errors"
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
	SessionID              session.ID
	Inbox                  *inbox.Inbox
	Restored               sessionstore.ResumeState
	Sessions               sessionstore.Store
	ContextBuilder         contextbuilder.Builder
	LLM                    llm.Adapter
	Tools                  tool.Registry
	Operations             operation.Manager
}

var (
	// ErrBusy means quiescence was refused without canceling live work.
	ErrBusy = errors.New("coordinator is busy")
	// ErrQuiesced is returned by Run only after successful idle retirement.
	ErrQuiesced = errors.New("coordinator quiesced")
	// ErrStopped means Run exited without quiescing. Its cause, if any, is
	// joined to this sentinel when returned by TryQuiesce.
	ErrStopped = errors.New("coordinator stopped without quiescing")
)

type Coordinator interface {
	// Run owns one session's decision loop until a stop control completes or
	// the context is canceled. It returns nil for a completed stop. It is
	// single-use; its caller must cancel the Inbox when Run returns. Successful
	// quiescent retirement returns ErrQuiesced, never nil.
	Run(context.Context) error

	// TryQuiesce asks the decision loop to retire only if it is idle, fencing
	// inbox admission and draining accepted inputs before its final decision.
	// Busy requests do not cancel any work. Before Run it waits for Run or ctx
	// cancellation; after Run it returns nil if quiesced, otherwise ErrStopped
	// joined with Run's cause. Once admitted, the request waits for a definitive
	// loop result: cancellation observed before commit aborts retirement, but
	// cancellation after commit cannot mask success. A nil result guarantees
	// retirement has committed; Run may still be unwinding its defers.
	TryQuiesce(context.Context) error
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
