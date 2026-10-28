package inbox

import (
	"encoding/json/v2"
	"testing"
)

func TestForegroundAbortStrictVersionedControls(t *testing.T) {
	for _, raw := range []string{
		`{"Mode":"abort_foreground","Parameters":{"Version":1,"TargetTurnID":"turn"}}`,
		`{"Mode":"foreground_abort_finished","Parameters":{"Version":1,"InputID":"abort","TargetTurnID":"turn","Status":"completed"}}`,
	} {
		value := Input{ID: "input", Kind: InputControl, Payload: []byte(raw)}
		request, err := value.DecodeControlMessage()
		if err != nil {
			t.Fatal(err)
		}
		data, err := json.Marshal(request)
		if err != nil {
			t.Fatal(err)
		}
		value.Payload = data
		if err := value.Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{
		`{"Mode":"abort_foreground","Parameters":{"Version":0,"TargetTurnID":"turn"}}`,
		`{"Mode":"abort_foreground","Parameters":{"Version":2,"TargetTurnID":"turn"}}`,
		`{"Mode":"abort_foreground","Parameters":{"Version":1,"TargetTurnID":""}}`,
		`{"Mode":"abort_foreground","Parameters":{"Version":1,"TargetTurnID":"turn","All":true}}`,
		`{"Mode":"foreground_abort_finished","Parameters":{"Version":1,"InputID":"abort","TargetTurnID":"turn","Status":"unsupported"}}`,
		`{"Mode":"foreground_abort_finished","Parameters":{"Version":1,"TargetTurnID":"turn","Status":"completed"}}`,
		`{"Mode":"foreground_abort_finished","Parameters":{"Version":1,"InputID":"abort","TargetTurnID":"turn","Status":"completed","Drain":true}}`,
	} {
		if err := (Input{ID: "input", Kind: InputControl, Payload: []byte(raw)}).Validate(); err == nil {
			t.Fatalf("accepted invalid control: %s", raw)
		}
	}
}

func TestOwnedLifecycleStrictVersionedControls(t *testing.T) {
	for _, raw := range []string{
		`{"Mode":"shutdown_session","Parameters":{"Version":1,"Scope":"host/1"}}`,
		`{"Mode":"session_shutdown_finished","Parameters":{"Version":1,"Scope":"host/1","InputID":"shutdown","Phase":"work_drained"}}`,
		`{"Mode":"execution_drained","Parameters":{"Version":1,"Scope":"fresh-launch-nonce"}}`,
	} {
		if err := (Input{ID: "input", Kind: InputControl, Payload: []byte(raw)}).Validate(); err != nil {
			t.Fatal(err)
		}
	}
	for _, raw := range []string{
		`{"Mode":"shutdown_session","Parameters":{"Version":2,"Scope":"host/1"}}`,
		`{"Mode":"shutdown_session","Parameters":{"Version":1,"Scope":" "}}`,
		`{"Mode":"shutdown_session","Parameters":{"Version":1,"Scope":"host/1","Force":true}}`,
		`{"Mode":"session_shutdown_finished","Parameters":{"Version":1,"Scope":"host/1","Phase":"completed"}}`,
		`{"Mode":"session_shutdown_finished","Parameters":{"Version":1,"Scope":"host/1","InputID":"shutdown","Phase":"unsupported"}}`,
		`{"Mode":"execution_drained","Parameters":{"Version":1,"Scope":""}}`,
		`{"Mode":"execution_drained","Parameters":{"Version":1,"Scope":"nonce","All":true}}`,
	} {
		if err := (Input{ID: "input", Kind: InputControl, Payload: []byte(raw)}).Validate(); err == nil {
			t.Fatalf("accepted invalid lifecycle: %s", raw)
		}
	}
}
