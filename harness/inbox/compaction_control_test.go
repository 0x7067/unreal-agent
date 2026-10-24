package inbox

import (
	"encoding/json/v2"
	"testing"
)

func TestCompactionControlsRoundTripAndBounds(t *testing.T) {
	for _, control := range []ControlMessage{
		{Mode: Compact}, {Mode: CompactionDisabled, Reason: "Original context preserved; use /compact."},
		{Mode: SetCompactionPolicy, Parameters: CompactionPolicy{FailureLimit: 3}},
	} {
		payload, err := json.Marshal(control)
		if err != nil {
			t.Fatal(err)
		}
		decoded, err := (Input{ID: "control", Kind: InputControl, Payload: payload}).DecodeControlMessage()
		if err != nil || decoded.Mode != control.Mode {
			t.Fatalf("decoded=%+v error=%v", decoded, err)
		}
	}
	for _, payload := range []string{
		`{"Mode":"compaction_policy","Parameters":{"FailureLimit":-1}}`,
		`{"Mode":"compaction_policy","Parameters":{"FailureLimit":0}}`,
		`{"Mode":"compaction_policy"}`, `{"Mode":"compaction_disabled"}`,
		`{"Mode":"compact","Parameters":{}}`,
	} {
		if _, err := (Input{ID: "invalid", Kind: InputControl, Payload: []byte(payload)}).DecodeControlMessage(); err == nil {
			t.Fatalf("accepted %s", payload)
		}
	}
}
