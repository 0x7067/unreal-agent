package coordinator

import (
	"encoding/json/v2"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/session"
)

func fixtureRequestStart(t *testing.T, turn session.TurnID, inputs, external int) inbox.Input {
	t.Helper()
	r := inbox.ModelRequestRecord{Version: 1, Action: "started", RequestID: "request-" + string(turn), TurnID: string(turn), TurnType: string(session.TurnRegular), Attempt: 1, Limit: 2, Inputs: inputs, ExternalInputs: external}
	payload, err := json.Marshal(inbox.ControlMessage{Mode: inbox.ModelRequest, Parameters: r})
	if err != nil {
		t.Fatal(err)
	}
	return inbox.Input{ID: inbox.ID("start-" + string(turn)), Kind: inbox.InputControl, Payload: payload}
}

func assertFixtureRequestStart(t *testing.T, value inbox.Input, turn session.TurnID, inputs, external int) inbox.ModelRequestRecord {
	t.Helper()
	message, err := value.DecodeControlMessage()
	if err != nil {
		t.Fatal(err)
	}
	record, ok := message.Parameters.(inbox.ModelRequestRecord)
	if !ok || message.Mode != inbox.ModelRequest || record.Validate() != nil || record.Action != "started" || record.Version != 1 || record.TurnID != string(turn) || record.TurnType != string(session.TurnRegular) || record.Attempt != 1 || record.Limit != 2 || record.Inputs != inputs || record.ExternalInputs != external {
		t.Fatalf("unexpected request charge: %+v", message)
	}
	return record
}
