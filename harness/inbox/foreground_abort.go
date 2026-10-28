package inbox

import (
	"encoding/json/v2"
	"fmt"
	"strings"
)

const (
	AbortForeground         ControlMode = "abort_foreground"
	ForegroundAbortFinished ControlMode = "foreground_abort_finished"
)

// ForegroundAbort is deliberately turn-bound: replaying an old control must not
// cancel a later turn. Only the coordinator may emit ForegroundAbortResult.
type ForegroundAbort struct {
	Version      int
	TargetTurnID string
	Scope        string `json:",omitzero"`
}

type ForegroundAbortResult struct {
	Version      int
	InputID      ID
	TargetTurnID string
	Status       string // completed, refused, stale, or uncertain (never cleanup proof)
	Error        string `json:",omitzero"`
}

func decodeForegroundAbort(raw []byte) (ForegroundAbort, error) {
	var value ForegroundAbort
	if err := json.Unmarshal(raw, &value, json.RejectUnknownMembers(true)); err != nil {
		return value, fmt.Errorf("decode foreground abort: %w", err)
	}
	if value.Version != 1 || strings.TrimSpace(value.TargetTurnID) == "" || len(value.TargetTurnID) > 128 || len(value.Scope) > 256 {
		return value, fmt.Errorf("unsupported or invalid foreground abort target/version")
	}
	return value, nil
}
func decodeForegroundAbortResult(raw []byte) (ForegroundAbortResult, error) {
	var value ForegroundAbortResult
	if err := json.Unmarshal(raw, &value, json.RejectUnknownMembers(true)); err != nil {
		return value, fmt.Errorf("decode foreground abort outcome: %w", err)
	}
	if value.Version != 1 || value.InputID == "" || strings.TrimSpace(value.TargetTurnID) == "" || len(value.TargetTurnID) > 128 || len(value.Error) > 4096 {
		return value, fmt.Errorf("unsupported or invalid foreground abort outcome")
	}
	switch value.Status {
	case "completed", "refused", "stale", "uncertain":
	default:
		return value, fmt.Errorf("unsupported foreground abort outcome %q", value.Status)
	}
	return value, nil
}
