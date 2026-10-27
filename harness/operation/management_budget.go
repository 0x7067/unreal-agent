package operation

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
)

// isManagementOperation recognizes only the versioned, non-launching control
// actions needed to inspect and cancel saturated work. This is a scheduling
// classification, not authorization: the owning handler still validates scope,
// generation, and all arguments. Unknown plans, versions, actions, ambiguous
// casing, and malformed payloads stay in the executing-work admission lane.
// Keep protocol labels here to avoid importing tool/taskgraph packages into the
// operation runtime (those packages themselves depend on operation).
func isManagementOperation(current Operation) bool {
	if current.Type != TypeRemoteJob {
		return false
	}
	state, err := DecodeRemoteJobState(current)
	if err != nil || state.Plan.Version != 1 {
		return false
	}
	if state.Plan.Type != "subagent.control" && state.Plan.Type != "taskgraph.control" {
		return false
	}
	var fields map[string]jsontext.Value
	if err := json.Unmarshal(state.Plan.Data, &fields); err != nil {
		return false
	}
	lower, hasLower := fields["action"]
	upper, hasUpper := fields["Action"]
	if hasLower == hasUpper {
		return false
	}
	raw := lower
	if hasUpper {
		raw = upper
	}
	var action string
	if err := json.Unmarshal(raw, &action); err != nil {
		return false
	}
	switch state.Plan.Type {
	case "subagent.control":
		return action == "list" || action == "status" || action == "stop"
	case "taskgraph.control":
		return action == "status" || action == "cancel"
	}
	return false
}
