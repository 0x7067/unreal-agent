package attachproto

import (
	"fmt"
	"strings"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
)

const (
	CapabilityForegroundAbort = "foreground_abort.v1"
	CapabilitySessionShutdown = "session_shutdown.v1"
)

// ValidateLifecycle admits only typed lifecycle intents for the current host
// incarnation/generation. It grants no authority to inject outcomes or receipts.
// expectedScope is fmt.Sprintf("%s/%d", hello.Incarnation, status.Generation).
// The caller must additionally bind a fresh foreground target and check the
// corresponding advertised capability before submitting an intent.
func ValidateLifecycle(input inbox.Input, expectedScope string) error {
	if strings.TrimSpace(string(input.ID)) == "" || len(input.ID) > 128 || input.Kind != inbox.InputControl || len(input.Payload) > MaxPayload {
		return fmt.Errorf("lifecycle submission requires control input with bounded nonblank stable ID and payload")
	}
	message, err := input.DecodeControlMessage()
	if err != nil {
		return err
	}
	var scope string
	switch message.Mode {
	case inbox.AbortForeground:
		parameters, ok := message.Parameters.(inbox.ForegroundAbort)
		if !ok {
			return fmt.Errorf("lifecycle foreground abort requires typed parameters")
		}
		scope = parameters.Scope
	case inbox.ShutdownSession:
		parameters, ok := message.Parameters.(inbox.ShutdownSessionRequest)
		if !ok {
			return fmt.Errorf("lifecycle shutdown requires typed parameters")
		}
		scope = parameters.Scope
	default:
		return fmt.Errorf("control mode %q is not a public lifecycle intent", message.Mode)
	}
	if strings.TrimSpace(expectedScope) == "" || strings.TrimSpace(scope) == "" || scope != expectedScope {
		return fmt.Errorf("lifecycle scope does not match attached host incarnation/generation")
	}
	return nil
}
