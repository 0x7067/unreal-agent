package inbox

import (
	"encoding/json/v2"
	"fmt"
	"strings"
)

const (
	ShutdownSession         ControlMode = "shutdown_session"
	SessionShutdownFinished ControlMode = "session_shutdown_finished"
	ExecutionDrained        ControlMode = "execution_drained"
)

// ShutdownSessionRequest is an intent, not a cleanup acknowledgment. Scope binds
// it to one host incarnation / worker generation, never a future owner.
type ShutdownSessionRequest struct {
	Version int
	Scope   string
}
type SessionShutdownResult struct {
	Version int
	InputID ID
	Scope   string
	Phase   string // work_drained, completed, uncertain
	Error   string `json:",omitzero"`
}

// ExecutionDrainReceipt is emitted only by a trusted runner after joining its
// effect/output/model owners. A fresh per-launch nonce prevents old history from
// serving as proof for a replacement process. It does not prove lease release.
type ExecutionDrainReceipt struct {
	Version int
	Scope   string
}

func validLifecycleScope(scope string) bool {
	return strings.TrimSpace(scope) != "" && len(scope) <= 256
}
func decodeOwnedLifecycle(mode ControlMode, raw []byte) (any, error) {
	switch mode {
	case ShutdownSession:
		var value ShutdownSessionRequest
		if err := json.Unmarshal(raw, &value, json.RejectUnknownMembers(true)); err != nil {
			return nil, err
		}
		if value.Version != 1 || !validLifecycleScope(value.Scope) {
			return nil, fmt.Errorf("unsupported shutdown intent scope/version")
		}
		return value, nil
	case SessionShutdownFinished:
		var value SessionShutdownResult
		if err := json.Unmarshal(raw, &value, json.RejectUnknownMembers(true)); err != nil {
			return nil, err
		}
		if value.Version != 1 || value.InputID == "" || !validLifecycleScope(value.Scope) || len(value.Error) > 4096 {
			return nil, fmt.Errorf("invalid shutdown outcome")
		}
		switch value.Phase {
		case "work_drained", "completed", "uncertain", "refused":
		default:
			return nil, fmt.Errorf("unsupported shutdown outcome phase %q", value.Phase)
		}
		return value, nil
	case ExecutionDrained:
		var value ExecutionDrainReceipt
		if err := json.Unmarshal(raw, &value, json.RejectUnknownMembers(true)); err != nil {
			return nil, err
		}
		if value.Version != 1 || !validLifecycleScope(value.Scope) {
			return nil, fmt.Errorf("invalid execution drain receipt")
		}
		return value, nil
	default:
		return nil, fmt.Errorf("unsupported owned lifecycle mode %q", mode)
	}
}
