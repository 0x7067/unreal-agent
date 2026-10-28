package inbox

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"strings"
)

// ModelRequest identifies a bounded model-request lifecycle event.
const ModelRequest ControlMode = "model_request"

// ModelRequestRecord is the versioned model-request cursor. Human diagnostics
// belong in ControlMessage.Reason, not in this replayable record.
type ModelRequestRecord struct {
	Version        int
	Action         string
	RequestID      string
	TurnID         string
	TurnType       string
	Attempt        int
	Limit          int
	Inputs         int
	ExternalInputs int
}

// Validate checks the supported version and bounded request cursor.
func (r ModelRequestRecord) Validate() error {
	if r.Version != 1 {
		return fmt.Errorf("unsupported model request version %d", r.Version)
	}
	switch r.Action {
	case "started", "interrupted", "exhausted", "stopped", "superseded":
	default:
		return fmt.Errorf("invalid model request action %q", r.Action)
	}
	if r.TurnType != "regular" && r.TurnType != "compaction" {
		return fmt.Errorf("invalid model request turn type %q", r.TurnType)
	}
	for _, id := range []string{r.RequestID, r.TurnID} {
		if id == "" || strings.TrimSpace(id) != id || len(id) > 128 {
			return fmt.Errorf("invalid model request identity")
		}
	}
	if r.Limit < 1 || r.Limit > 100000 || r.Attempt < 1 || r.Attempt > r.Limit {
		return fmt.Errorf("invalid model request attempt/limit")
	}
	if r.Inputs < 0 || r.ExternalInputs < 0 || r.ExternalInputs > r.Inputs {
		return fmt.Errorf("invalid model request input counts")
	}
	return nil
}

func decodeModelRequest(raw jsontext.Value) (ModelRequestRecord, error) {
	var record ModelRequestRecord
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return record, fmt.Errorf("model request parameters are missing")
	}
	if err := json.Unmarshal(raw, &record, json.RejectUnknownMembers(true)); err != nil {
		return record, fmt.Errorf("decode model request: %w", err)
	}
	if err := record.Validate(); err != nil {
		return ModelRequestRecord{}, err
	}
	return record, nil
}
