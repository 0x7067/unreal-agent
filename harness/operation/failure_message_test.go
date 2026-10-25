package operation

import (
	"encoding/json/v2"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/primitives"
)

func TestShellPrimitiveFailureMessages(t *testing.T) {
	for _, tc := range []struct{ message, want string }{
		{"", "shell primitive failed without an error message"},
		{" \n\t", "shell primitive failed without an error message"},
		{"error", "shell primitive failed: error"},
		{"unknown error", "shell primitive failed: unknown error"},
		{"open /private/capture/out: permission denied", "open /private/capture/out: permission denied"},
	} {
		t.Run(tc.message, func(t *testing.T) {
			got := shellPrimitiveFailure(primitives.PrimitiveEvent{Result: primitives.PrimitiveFailureResult{Error: tc.message}})
			if got == nil || got.Error() != tc.want {
				t.Fatalf("failure = %v, want %q", got, tc.want)
			}
		})
	}
	if got := shellPrimitiveFailure(primitives.PrimitiveEvent{}); got.Error() != "shell primitive failed with an invalid result" {
		t.Fatal(got)
	}
}

func TestLegacyFailureCheckpointsNeedNoFailureReason(t *testing.T) {
	// These are pre-FailureReason wire values, not newly generated state.
	for _, tc := range []struct{ name, wire string }{
		{"shell", `{"ID":"legacy","Type":"shell","Version":3,"Status":"failed","MaxOutputLength":1000,"State":{"Input":{"Shell":"/bin/sh"},"BaseDirectory":"/tmp","TerminalError":"open /capture/out: permission denied"}}`},
		{"remote", `{"ID":"legacy","Type":"remote_job","Version":2,"Status":"failed","MaxOutputLength":1000,"State":{"Plan":{"Type":"test","Version":1,"Data":{}},"TerminalError":"open /capture/out: permission denied"}}`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var current Operation
			if err := json.Unmarshal([]byte(tc.wire), &current); err != nil {
				t.Fatal(err)
			}
			var message string
			if tc.name == "shell" {
				state, err := DecodeShellState(current)
				if err != nil {
					t.Fatal(err)
				}
				message = state.TerminalError
			} else {
				state, err := DecodeRemoteJobState(current)
				if err != nil {
					t.Fatal(err)
				}
				message = state.TerminalError
			}
			if message != "open /capture/out: permission denied" {
				t.Fatalf("lost diagnostic: %q", message)
			}
			encoded, err := json.Marshal(current)
			if err != nil {
				t.Fatal(err)
			}
			if strings.Contains(string(encoded), "FailureReason") {
				t.Fatalf("introduced wire field: %s", encoded)
			}
		})
	}
}
