package inbox

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"strings"
	"testing"
)

func validModelRequest() ModelRequestRecord {
	return ModelRequestRecord{Version: 1, Action: "started", RequestID: "request", TurnID: "turn", TurnType: "regular", Attempt: 1, Limit: 3, Inputs: 2, ExternalInputs: 1}
}

func TestModelRequestRoundTrip(t *testing.T) {
	for _, action := range []string{"started", "interrupted", "exhausted", "stopped", "superseded"} {
		for _, turnType := range []string{"regular", "compaction"} {
			r := validModelRequest()
			r.Action = action
			r.TurnType = turnType
			r.RequestID = strings.Repeat("x", 128)
			r.TurnID = strings.Repeat("y", 128)
			r.Attempt = 100000
			r.Limit = 100000
			payload, err := json.Marshal(ControlMessage{Mode: ModelRequest, Reason: strings.Repeat("r", 2048), Parameters: r})
			if err != nil {
				t.Fatal(err)
			}
			got, err := (Input{Kind: InputControl, Payload: payload}).DecodeControlMessage()
			if err != nil {
				t.Fatal(err)
			}
			if decoded, ok := got.Parameters.(ModelRequestRecord); !ok || decoded != r {
				t.Fatalf("typed roundtrip: %#v", got)
			}
		}
	}
	r := validModelRequest()
	r.Inputs = 0
	r.ExternalInputs = 0
	r.Limit = 1
	if err := r.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestModelRequestInvalidRecords(t *testing.T) {
	cases := map[string]func(*ModelRequestRecord){
		"missing version":    func(r *ModelRequestRecord) { r.Version = 0 },
		"future version":     func(r *ModelRequestRecord) { r.Version = 2 },
		"negative version":   func(r *ModelRequestRecord) { r.Version = -1 },
		"action":             func(r *ModelRequestRecord) { r.Action = "finished" },
		"empty action":       func(r *ModelRequestRecord) { r.Action = "" },
		"turn type":          func(r *ModelRequestRecord) { r.TurnType = "other" },
		"empty turn type":    func(r *ModelRequestRecord) { r.TurnType = "" },
		"request missing":    func(r *ModelRequestRecord) { r.RequestID = "" },
		"turn missing":       func(r *ModelRequestRecord) { r.TurnID = "" },
		"request whitespace": func(r *ModelRequestRecord) { r.RequestID = " request" },
		"turn whitespace":    func(r *ModelRequestRecord) { r.TurnID = "turn\t" },
		"request long":       func(r *ModelRequestRecord) { r.RequestID = strings.Repeat("x", 129) },
		"turn long bytes":    func(r *ModelRequestRecord) { r.TurnID = strings.Repeat("é", 65) },
		"limit zero":         func(r *ModelRequestRecord) { r.Limit = 0 },
		"limit negative":     func(r *ModelRequestRecord) { r.Limit = -1 },
		"limit large":        func(r *ModelRequestRecord) { r.Limit = 100001 },
		"attempt zero":       func(r *ModelRequestRecord) { r.Attempt = 0 },
		"attempt negative":   func(r *ModelRequestRecord) { r.Attempt = -1 },
		"attempt large":      func(r *ModelRequestRecord) { r.Attempt = r.Limit + 1 },
		"inputs negative":    func(r *ModelRequestRecord) { r.Inputs = -1 },
		"external negative":  func(r *ModelRequestRecord) { r.ExternalInputs = -1 },
		"external greater":   func(r *ModelRequestRecord) { r.ExternalInputs = r.Inputs + 1 },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := validModelRequest()
			mutate(&r)
			if r.Validate() == nil {
				t.Fatal("Validate accepted invalid record")
			}
			raw, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := decodeModelRequest(raw); err == nil {
				t.Fatal("decoder accepted invalid record")
			}
		})
	}
}

func TestModelRequestMalformedEnvelope(t *testing.T) {
	raw, err := json.Marshal(validModelRequest())
	if err != nil {
		t.Fatal(err)
	}
	valid := string(raw)
	for _, params := range []string{"null", "{}", "[]", `"text"`, "1", "true", "{", strings.TrimSuffix(valid, "}") + `,"Unknown":1}`, strings.TrimSuffix(valid, "}") + `,"Reason":"not allowed"}`, strings.Replace(valid, `"Version":1`, `"Version":null`, 1), strings.Replace(valid, `"Attempt":1`, `"Attempt":1.5`, 1)} {
		payload := `{"Mode":"model_request","Parameters":` + params + `}`
		if _, err := (Input{Kind: InputControl, Payload: jsontext.Value(payload)}).DecodeControlMessage(); err == nil {
			t.Errorf("accepted %s", payload)
		}
	}
	for _, payload := range []string{
		`{"Mode":"model_request"}`,
		`{"Mode":"model_request","Parameters":` + valid + `,"Unknown":true}`,
		`{"Mode":"model_request","Parameters":` + valid + `,"Reason":"` + strings.Repeat("x", 2049) + `"}`,
		`{"Mode":"unsupported","Parameters":` + valid + `}`,
		`{"Mode":"unsupported"}`,
		`{"Mode":"hard","Parameters":` + valid + `}`,
	} {
		if _, err := (Input{Kind: InputControl, Payload: jsontext.Value(payload)}).DecodeControlMessage(); err == nil {
			t.Errorf("accepted %s", payload)
		}
	}
}
