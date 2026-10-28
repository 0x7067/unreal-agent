package coordinator

import (
	"encoding/json/v2"
	"testing"
	"testing/synctest"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
)

func TestSessionShutdownScopeMismatchDoesNotCancelModel(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 0)
		run.current.dependencies.ControlScope = "new-owner/2"
		run.start(t)
		run.input(t, externalEvent(t, 0, "question", "question"))
		data, _ := json.Marshal(inbox.ControlMessage{Mode: inbox.ShutdownSession, Parameters: inbox.ShutdownSessionRequest{Version: 1, Scope: "old-owner/1"}})
		run.input(t, inbox.Input{ID: "old-shutdown", Kind: inbox.InputControl, Payload: data})
		if run.calls[0].ctx.Err() != nil {
			t.Fatal("stale shutdown canceled current model")
		}
		if run.current.state.lifecycle.ShutdownResults["old-shutdown"].Phase != "refused" {
			t.Fatal("missing explicit stale-scope refusal")
		}
		run.respond(t, 0, textResponse("still running"))
		run.assertRunning(t)
	})
}
func TestSessionShutdownRetiresOnlyAfterDurableIntentButDoesNotClaimDrain(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		run := newStopTestRun(t, 0)
		run.current.dependencies.ControlScope = "owner/1"
		run.start(t)
		run.input(t, externalEvent(t, 0, "question", "question"))
		recorded := false
		run.store.onAppendInput = func(value inbox.Input) {
			if value.ID == "shutdown" {
				recorded = true
				if run.calls[0].ctx.Err() != nil {
					t.Fatal("model canceled before durable intent")
				}
			}
		}
		data, _ := json.Marshal(inbox.ControlMessage{Mode: inbox.ShutdownSession, Parameters: inbox.ShutdownSessionRequest{Version: 1, Scope: "owner/1"}})
		run.input(t, inbox.Input{ID: "shutdown", Kind: inbox.InputControl, Payload: data})
		run.assertStopped(t)
		if !recorded || run.current.state.lifecycle.ValidateResume() == nil {
			t.Fatal("coordinator retirement was misrepresented as ownership drain")
		}
	})
}
