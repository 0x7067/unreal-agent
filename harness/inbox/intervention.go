package inbox

import (
	"encoding/json/v2"
	"fmt"
	"strings"
)

// ValidateIntervention admits only generation-bound controls of existing owners.
// It grants no launch, revise or retry capability. DecodeControlMessage remains
// compatible with historical unscoped AgentControl records outside this lane.
func ValidateIntervention(input Input, scope string) error {
	control, err := input.DecodeControlMessage()
	if err != nil {
		return err
	}
	invocation, ok := control.Parameters.(ToolInvocation)
	if control.Mode != InvokeTool || !ok || scope == "" || invocation.Scope != scope {
		return fmt.Errorf("intervention requires the exact current runtime scope")
	}
	return validateInterventionCall(invocation)
}

func validateInterventionCall(invocation ToolInvocation) error {
	switch invocation.Call.Name {
	case "AgentControl":
		var a struct {
			Action     string `json:"action"`
			Handle     string `json:"handle"`
			Generation uint64 `json:"generation"`
			Message    string `json:"message"`
		}
		if err := json.Unmarshal([]byte(invocation.Call.Arguments), &a, json.RejectUnknownMembers(true)); err != nil {
			return err
		}
		switch a.Action {
		case "list":
			if a.Handle == "" && a.Generation == 0 && a.Message == "" {
				return nil
			}
		case "status", "stop", "message":
			if strings.TrimSpace(a.Handle) != "" && a.Generation > 0 && ((a.Action == "message" && strings.TrimSpace(a.Message) != "") || (a.Action != "message" && a.Message == "")) {
				return nil
			}
		}
	case "TaskGraph":
		var a struct {
			Action     string `json:"action"`
			Name       string `json:"name"`
			TaskID     string `json:"task_id"`
			Generation uint64 `json:"generation"`
			Evidence   string `json:"evidence"`
		}
		if err := json.Unmarshal([]byte(invocation.Call.Arguments), &a, json.RejectUnknownMembers(true)); err != nil {
			return err
		}
		if strings.TrimSpace(a.Name) == "" {
			break
		}
		switch a.Action {
		case "status", "cancel":
			if a.TaskID == "" && a.Generation == 0 && a.Evidence == "" {
				return nil
			}
		case "accept", "reject":
			if strings.TrimSpace(a.TaskID) != "" && a.Generation > 0 && strings.TrimSpace(a.Evidence) != "" {
				return nil
			}
		}
	}
	return fmt.Errorf("unsupported intervention: only exact existing-owner controls are allowed")
}
