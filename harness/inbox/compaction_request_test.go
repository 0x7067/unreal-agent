package inbox

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"math"
	"reflect"
	"strings"
	"testing"
)

func validCompactionRequest() CompactionRequestRecord {
	return CompactionRequestRecord{Version: 1, Request: ModelRequestRecord{Version: 1, Action: "started", RequestID: "request", TurnID: "turn", TurnType: "compaction", Attempt: 1, Limit: 3, Inputs: 2, ExternalInputs: 1}}
}

func compactionControl(r CompactionRequestRecord, reason string) (ControlMessage, error) {
	payload, err := json.Marshal(ControlMessage{Mode: CompactionRequest, Reason: reason, Parameters: r})
	if err != nil {
		return ControlMessage{}, err
	}
	return (Input{ID: "control", Kind: InputControl, Payload: payload}).DecodeControlMessage()
}

func TestCompactionRequestRoundTrip(t *testing.T) {
	for _, action := range []string{"started", "interrupted", "exhausted", "stopped", "superseded"} {
		for _, manual := range []ID{"", "manual", ID(strings.Repeat("x", 128))} {
			for _, failures := range []int{0, 10, math.MaxInt - 100000} {
				r := validCompactionRequest()
				r.Request.Action = action
				r.ManualInputID = manual
				r.FailuresBefore = failures
				got, err := compactionControl(r, strings.Repeat("r", 2048))
				if err != nil {
					t.Fatal(err)
				}
				typed, ok := got.Parameters.(CompactionRequestRecord)
				if !ok || !reflect.DeepEqual(typed, r) || got.Mode != CompactionRequest || len(got.Reason) != 2048 {
					t.Fatalf("round trip: %#v", got)
				}
			}
		}
	}
}

func TestCompactionRequestInvalidRecords(t *testing.T) {
	cases := map[string]func(*CompactionRequestRecord){
		"version zero":           func(r *CompactionRequestRecord) { r.Version = 0 },
		"version future":         func(r *CompactionRequestRecord) { r.Version = 2 },
		"nested version zero":    func(r *CompactionRequestRecord) { r.Request.Version = 0 },
		"nested version future":  func(r *CompactionRequestRecord) { r.Request.Version = 2 },
		"regular turn":           func(r *CompactionRequestRecord) { r.Request.TurnType = "regular" },
		"unknown turn":           func(r *CompactionRequestRecord) { r.Request.TurnType = "other" },
		"unknown action":         func(r *CompactionRequestRecord) { r.Request.Action = "other" },
		"empty request":          func(r *CompactionRequestRecord) { r.Request.RequestID = "" },
		"padded request":         func(r *CompactionRequestRecord) { r.Request.RequestID = " request" },
		"long request":           func(r *CompactionRequestRecord) { r.Request.RequestID = strings.Repeat("x", 129) },
		"empty turn":             func(r *CompactionRequestRecord) { r.Request.TurnID = "" },
		"padded turn":            func(r *CompactionRequestRecord) { r.Request.TurnID = "turn " },
		"long turn":              func(r *CompactionRequestRecord) { r.Request.TurnID = strings.Repeat("x", 129) },
		"attempt zero":           func(r *CompactionRequestRecord) { r.Request.Attempt = 0 },
		"attempt beyond limit":   func(r *CompactionRequestRecord) { r.Request.Attempt = 4 },
		"limit zero":             func(r *CompactionRequestRecord) { r.Request.Limit = 0 },
		"limit too large":        func(r *CompactionRequestRecord) { r.Request.Limit = 100001 },
		"negative inputs":        func(r *CompactionRequestRecord) { r.Request.Inputs = -1 },
		"negative external":      func(r *CompactionRequestRecord) { r.Request.ExternalInputs = -1 },
		"external beyond inputs": func(r *CompactionRequestRecord) { r.Request.ExternalInputs = 3 },
		"negative failures":      func(r *CompactionRequestRecord) { r.FailuresBefore = -1 },
		"overflow failures":      func(r *CompactionRequestRecord) { r.FailuresBefore = math.MaxInt - 99999 },
		"padded manual":          func(r *CompactionRequestRecord) { r.ManualInputID = " manual" },
		"blank manual":           func(r *CompactionRequestRecord) { r.ManualInputID = " " },
		"long manual":            func(r *CompactionRequestRecord) { r.ManualInputID = ID(strings.Repeat("x", 129)) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := validCompactionRequest()
			mutate(&r)
			if _, err := compactionControl(r, ""); err == nil {
				t.Fatal("accepted invalid record")
			}
		})
	}
	if _, err := compactionControl(validCompactionRequest(), strings.Repeat("é", 1025)); err == nil {
		t.Fatal("accepted reason over byte bound")
	}
}

func TestCompactionRequestStrictJSON(t *testing.T) {
	payload, err := json.Marshal(ControlMessage{Mode: CompactionRequest, Parameters: validCompactionRequest()})
	if err != nil {
		t.Fatal(err)
	}
	var base map[string]jsontext.Value
	if err := json.Unmarshal(payload, &base); err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		name, parameters string
		outer            bool
	}{
		{"missing", "", false}, {"null", "null", false}, {"array", "[]", false}, {"scalar", "1", false},
		{"missing nested", `{"Version":1}`, false}, {"null nested", `{"Version":1,"Request":null}`, false},
		{"unknown outer", "", true},
		{"unknown record", strings.Replace(string(base["Parameters"]), `"Version":1`, `"Unknown":1,"Version":1`, 1), false},
		{"record reason", strings.Replace(string(base["Parameters"]), `"Version":1`, `"Reason":"no","Version":1`, 1), false},
		{"unknown nested", strings.Replace(string(base["Parameters"]), `"Action":`, `"Unknown":1,"Action":`, 1), false},
		{"nested wrong type", `{"Version":1,"Request":"compaction"}`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			envelope := make(map[string]jsontext.Value)
			for k, v := range base {
				envelope[k] = v
			}
			if tc.outer {
				envelope["Unknown"] = jsontext.Value(`1`)
			} else if tc.parameters == "" {
				delete(envelope, "Parameters")
			} else {
				envelope["Parameters"] = jsontext.Value(tc.parameters)
			}
			raw, err := json.Marshal(envelope)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := (Input{ID: "control", Kind: InputControl, Payload: raw}).DecodeControlMessage(); err == nil {
				t.Fatalf("accepted %s", raw)
			}
		})
	}
}
