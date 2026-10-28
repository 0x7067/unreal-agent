package inbox

import (
	"bytes"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"math"
	"strings"
)

// CompactionRequest records a bounded compaction model-request lifecycle event.
const CompactionRequest ControlMode = "compaction_request"

// CompactionRequestRecord preserves the retry cursor and optional manual input
// identity. Diagnostics belong in the control envelope's Reason.
type CompactionRequestRecord struct {
	Version        int
	Request        ModelRequestRecord
	FailuresBefore int
	ManualInputID  ID `json:",omitzero"`
}

// Validate checks structural bounds; replay validates failure-count causality.
func (r CompactionRequestRecord) Validate() error {
	if r.Version != 1 {
		return fmt.Errorf("unsupported compaction request version %d", r.Version)
	}
	if err := r.Request.Validate(); err != nil {
		return err
	}
	if r.Request.TurnType != "compaction" {
		return fmt.Errorf("compaction request requires compaction turn type")
	}
	if r.FailuresBefore < 0 || r.FailuresBefore > math.MaxInt-100000 {
		return fmt.Errorf("invalid compaction failures before")
	}
	if strings.TrimSpace(string(r.ManualInputID)) != string(r.ManualInputID) || len(r.ManualInputID) > 128 {
		return fmt.Errorf("invalid compaction manual input ID")
	}
	return nil
}

func decodeCompactionRequest(raw jsontext.Value) (CompactionRequestRecord, error) {
	var record CompactionRequestRecord
	if len(raw) == 0 || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return record, fmt.Errorf("compaction request parameters are missing")
	}
	if err := json.Unmarshal(raw, &record, json.RejectUnknownMembers(true)); err != nil {
		return record, fmt.Errorf("decode compaction request: %w", err)
	}
	if err := record.Validate(); err != nil {
		return CompactionRequestRecord{}, err
	}
	return record, nil
}
