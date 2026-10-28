package attachproto

import (
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
)

func TestValidateLifecycleStrictAdmission(t *testing.T) {
	abort := `{"Mode":"abort_foreground","Parameters":{"Version":1,"TargetTurnID":"turn","Scope":"host/7"}}`
	shutdown := `{"Mode":"shutdown_session","Parameters":{"Version":1,"Scope":"host/7"}}`
	for _, payload := range []string{abort, shutdown} {
		input := inbox.Input{ID: "stable", Kind: inbox.InputControl, Payload: []byte(payload)}
		if err := ValidateLifecycle(input, "host/7"); err != nil {
			t.Fatal(err)
		}
		for name, bad := range map[string]inbox.Input{
			"blank ID": {ID: " \t", Kind: input.Kind, Payload: input.Payload},
			"long ID":  {ID: inbox.ID(strings.Repeat("x", 129)), Kind: input.Kind, Payload: input.Payload},
			"external": {ID: input.ID, Kind: inbox.InputExternal, Payload: input.Payload},
			"crash":    {ID: input.ID, Kind: inbox.InputCrash, Payload: input.Payload},
		} {
			t.Run(name, func(t *testing.T) {
				if ValidateLifecycle(bad, "host/7") == nil {
					t.Fatal("accepted invalid input")
				}
			})
		}
		for _, scope := range []string{"", "host/6", "other/7", " host/7"} {
			if ValidateLifecycle(input, scope) == nil {
				t.Fatalf("accepted scope %q", scope)
			}
		}
	}
	for name, payload := range map[string]string{
		"unknown envelope":  strings.Replace(abort, `"Mode":`, `"Injected":true,"Mode":`, 1),
		"unknown parameter": strings.Replace(abort, `"Version":`, `"Injected":true,"Version":`, 1),
		"duplicate":         strings.Replace(abort, `"Version":1`, `"Version":1,"Version":1`, 1),
		"bad version":       strings.Replace(abort, `"Version":1`, `"Version":2`, 1),
		"empty target":      strings.Replace(abort, `"turn"`, `" "`, 1),
		"long target":       strings.Replace(abort, `"turn"`, `"`+strings.Repeat("t", 129)+`"`, 1),
		"empty scope":       strings.Replace(abort, `"host/7"`, `""`, 1),
		"missing scope":     `{"Mode":"abort_foreground","Parameters":{"Version":1,"TargetTurnID":"turn"}}`,
		"null":              `null`,
		"stop":              `{"Mode":"hard"}`,
		"invoke":            `{"Mode":"invoke_tool","Parameters":{"Call":{"Name":"AgentControl","CallID":"call","Arguments":"{}"}}}`,
		"abort outcome":     `{"Mode":"foreground_abort_finished","Parameters":{"Version":1,"InputID":"id","TargetTurnID":"turn","Status":"completed"}}`,
		"shutdown outcome":  `{"Mode":"session_shutdown_finished","Parameters":{"Version":1,"InputID":"id","Scope":"host/7","Phase":"completed"}}`,
		"receipt":           `{"Mode":"execution_drained","Parameters":{"Version":1,"Scope":"host/7"}}`,
		"oversize":          `{"Mode":"shutdown_session","Reason":"` + strings.Repeat("r", MaxPayload) + `","Parameters":{"Version":1,"Scope":"host/7"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			if ValidateLifecycle(inbox.Input{ID: "stable", Kind: inbox.InputControl, Payload: []byte(payload)}, "host/7") == nil {
				t.Fatal("accepted injection or invalid intent")
			}
		})
	}
	// The existing payload limit also bounds lifecycle frames, without wire fields.
	prefix := `{"Mode":"shutdown_session","Reason":"`
	suffix := `","Parameters":{"Version":1,"Scope":"host/7"}}`
	payload := prefix + strings.Repeat("r", MaxPayload-len(prefix)-len(suffix)) + suffix
	input := inbox.Input{ID: inbox.ID(strings.Repeat("i", 128)), Kind: inbox.InputControl, Payload: []byte(payload)}
	if err := ValidateLifecycle(input, "host/7"); err != nil {
		t.Fatal(err)
	}
	frame, err := Encode(Request{Type: "lifecycle", Input: &input})
	if err != nil || len(frame) > MaxFrame {
		t.Fatalf("frame bounds: %d %v", len(frame), err)
	}
}
