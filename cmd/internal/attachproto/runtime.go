package attachproto

import (
	"errors"
	"strings"
	"unicode"
)

// This is an opt-in discriminated variant of attach/v1, advertised separately.
// Hosts must never send it to a client that has not requested runtime control.
// v2 capabilities gate scheduled replacement semantics; wire records remain v1.
const CapabilityRuntime = "runtime_replacement_v2"
const CapabilityRuntimeDrain = "runtime_drain_v2"
const CapabilityHostRetire = "host_retire_v2"

type RuntimeRequest struct {
	Version            int
	Action             string
	ExpectedGeneration uint64 `json:",omitzero"`
	TargetID           string `json:",omitzero"`
}
type RuntimeStatus struct {
	Version        int
	Generation     uint64
	RunningBuild   string
	CandidateBuild string `json:",omitzero"`
	State          string
	Message        string `json:",omitzero"`
	Error          string `json:",omitzero"`
}

func boundedRuntimeText(s string, n int) bool {
	return len(s) <= n && !strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) && r != '\n' && r != '\t' })
}
func ValidateRuntimeRequest(r RuntimeRequest) error {
	if r.Version != 1 {
		return errors.New("unsupported runtime control version")
	}
	switch r.Action {
	case "check":
		if r.ExpectedGeneration != 0 || r.TargetID != "" {
			return errors.New("check has mutation parameters")
		}
	case "replace", "drain", "retire-host":
		if r.ExpectedGeneration == 0 || r.TargetID != "" {
			return errors.New("replacement needs exact current generation")
		}
	case "cancel":
		if r.ExpectedGeneration != 0 || strings.TrimSpace(r.TargetID) != r.TargetID || r.TargetID == "" || !boundedRuntimeText(r.TargetID, 128) {
			return errors.New("cancel needs current client request identity")
		}
	default:
		return errors.New("unsupported runtime control action")
	}
	return nil
}
func (s RuntimeStatus) Terminal() bool {
	return s.State != "checking" && s.State != "replacing" && s.State != "queued"
}
func ValidateRuntimeStatus(s RuntimeStatus) error {
	if s.Version != 1 || !boundedRuntimeText(s.RunningBuild, 512) || !boundedRuntimeText(s.CandidateBuild, 512) || !boundedRuntimeText(s.Message, 1024) || !boundedRuntimeText(s.Error, 1024) {
		return errors.New("invalid runtime status version or bounds")
	}
	switch s.State {
	case "checking", "replacing", "queued", "ready", "blocked", "completed", "canceled", "cancel_requested", "stopped", "failed", "drained":
	default:
		return errors.New("unsupported runtime replacement state")
	}
	if s.State == "drained" && (s.Generation != 0 || s.RunningBuild != "" || s.Error != "") {
		return errors.New("drained host status still has running identity/error")
	}
	if (s.State == "ready" || s.State == "completed") && (s.Generation == 0 || s.RunningBuild == "" || s.Error != "") {
		return errors.New("runtime success lacks running identity")
	}
	return nil
}
