package agentrunner

import (
	"fmt"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

// validateM23RequestControls returns only the synthesized controls that passed
// the entire ledger oracle. Callers must retain every other input in their
// original-input/FIFO/byte comparison. Pass full history, not a resume suffix,
// when available: only a history beginning at sequence 1 proves the exact
// external-input cursor. Inputs also includes heartbeats and settled tool calls;
// this oracle deliberately does not guess that cursor from tool-status count.
func validateM23RequestControls(t *testing.T, items []sessionstore.Item) map[inbox.ID]bool {
	t.Helper()
	validated, err := checkM23RequestControls(items)
	if err != nil {
		t.Fatal(err)
	}
	return validated
}

func checkM23RequestControls(items []sessionstore.Item) (map[inbox.ID]bool, error) {
	type attempt struct {
		record                           inbox.ModelRequestRecord
		turnSeen, completed, hardStopped bool
	}
	validated := make(map[inbox.ID]bool)
	inputIDs := make(map[inbox.ID]bool)
	requestIDs := make(map[string]bool)
	turns := make(map[session.TurnID]bool)
	responses := make(map[session.TurnID]bool)
	legacyPending := make(map[session.TurnID]bool)
	byTurn := make(map[session.TurnID]*attempt)
	var latest *attempt
	external := 0
	completeHistory := len(items) == 0 || items[0].Sequence == 1
	fail := func(index int, format string, args ...any) (map[inbox.ID]bool, error) {
		return nil, fmt.Errorf("model request oracle item %d: %s", index, fmt.Sprintf(format, args...))
	}
	for index, item := range items {
		switch value := item.Data.(type) {
		case inbox.Input:
			if item.Kind != sessionstore.ItemInput || value.ID == "" || inputIDs[value.ID] {
				return fail(index, "invalid or duplicate input identity/kind: %#v", value)
			}
			inputIDs[value.ID] = true
			if value.Kind == inbox.InputExternal {
				external++
				continue
			}
			if value.Kind != inbox.InputControl {
				continue
			}
			// Decode every control, not just a permissive Mode-only projection. This
			// rejects malformed envelopes, unknown fields, versions, and parameter types.
			control, err := value.DecodeControlMessage()
			if err != nil {
				return fail(index, "invalid control: %v", err)
			}
			if control.Mode == inbox.StopHard {
				if latest != nil && !latest.completed {
					latest.hardStopped = true
				}
			}
			if control.Mode != inbox.ModelRequest {
				continue
			}
			r, ok := control.Parameters.(inbox.ModelRequestRecord)
			if !ok {
				return fail(index, "parameters have type %T", control.Parameters)
			}
			// Keep the policy oracle independent of the production Validate method.
			if r.Version != 1 || r.TurnType != "regular" || r.Limit != 2 || r.Attempt < 1 || r.Attempt > r.Limit || r.Inputs < 0 || r.ExternalInputs < 0 || r.Inputs < r.ExternalInputs {
				return fail(index, "invalid request policy/cursors: %#v", r)
			}
			for _, id := range []string{r.RequestID, r.TurnID} {
				if id == "" || strings.TrimSpace(id) != id || len(id) > 128 {
					return fail(index, "invalid request/turn identity: %#v", r)
				}
			}
			switch r.Action {
			case "started":
				if len(legacyPending) != 0 {
					return fail(index, "new charge follows incomplete legacy turn")
				}
				if completeHistory && r.ExternalInputs != external {
					return fail(index, "external cursor = %d, history has %d", r.ExternalInputs, external)
				}
				if turns[session.TurnID(r.TurnID)] || byTurn[session.TurnID(r.TurnID)] != nil {
					return fail(index, "turn identity reused: %q", r.TurnID)
				}
				retry := latest != nil && !latest.completed && latest.record.Action == "interrupted" && latest.record.RequestID == r.RequestID
				if retry {
					if r.Attempt != latest.record.Attempt+1 || r.Limit != latest.record.Limit || r.TurnType != latest.record.TurnType {
						return fail(index, "invalid retry attempt: %#v", r)
					}
				} else {
					if latest != nil && !latest.completed && (latest.record.Action == "started" || latest.record.Action == "interrupted") {
						return fail(index, "new logical request overlaps unfinished attempt")
					}
					if r.Attempt != 1 || requestIDs[r.RequestID] {
						return fail(index, "new logical request reused identity/attempt: %#v", r)
					}
				}
				latest = &attempt{record: r}
				byTurn[session.TurnID(r.TurnID)] = latest
				requestIDs[r.RequestID] = true
			case "interrupted", "exhausted", "stopped", "superseded":
				if latest == nil || latest.completed {
					return fail(index, "outcome has no unfinished charge")
				}
				expected := latest.record
				expected.Action = r.Action
				if r != expected {
					return fail(index, "outcome changed prior charge: got %#v, prior %#v", r, latest.record)
				}
				switch r.Action {
				case "interrupted":
					if latest.record.Action != "started" || latest.hardStopped {
						return fail(index, "interruption does not follow live charge")
					}
				case "exhausted":
					if latest.record.Action != "interrupted" || r.Attempt != r.Limit {
						return fail(index, "premature exhaustion")
					}
				case "stopped", "superseded":
					if latest.record.Action != "started" && latest.record.Action != "interrupted" {
						return fail(index, "outcome does not follow live/interrupted charge")
					}
				}
				latest.record = r
			default:
				return fail(index, "unknown request action %q", r.Action)
			}
			validated[value.ID] = true
		case session.Turn:
			if item.Kind != sessionstore.ItemTurn || value.ID == "" || turns[value.ID] {
				return fail(index, "invalid or reused turn: %#v", value)
			}
			turns[value.ID] = true
			if latest == nil {
				legacyPending[value.ID] = true
				continue
			}
			if value.Type != session.TurnRegular {
				continue
			}
			if latest.completed || latest.record.Action != "started" || latest.turnSeen || latest.record.TurnID != string(value.ID) || latest.record.TurnType != string(value.Type) {
				return fail(index, "regular turn lacks matching preceding charge: %#v", value)
			}
			latest.turnSeen = true
		case sessionstore.ModelResponse:
			if item.Kind != sessionstore.ItemModelResponse || !turns[value.TurnID] || responses[value.TurnID] {
				return fail(index, "response has missing/duplicate turn: %q", value.TurnID)
			}
			responses[value.TurnID] = true
			delete(legacyPending, value.TurnID)
			if a := byTurn[value.TurnID]; a != nil {
				if !a.turnSeen {
					return fail(index, "response preceded charged turn")
				}
				// A late usage-only response for interrupted/stopped work is inert and
				// cannot authorize completion or erase the retry budget.
				if a.record.Action == "started" && !a.hardStopped {
					a.completed = true
				}
			}
		}
	}
	return validated, nil
}
