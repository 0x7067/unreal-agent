package attachproto

import (
	"fmt"
	"strings"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
)

const CapabilityIntervention = "reload_intervention.v1"

// ValidateIntervention admits only bounded, generation-bound controls of existing
// owners. expectedScope comes from the authenticated host incarnation/generation;
// intervention capability does not grant launch, retry, or revise authority.
func ValidateIntervention(input inbox.Input, expectedScope string) error {
	if strings.TrimSpace(string(input.ID)) == "" || len(input.ID) > 128 || input.Kind != inbox.InputControl || len(input.Payload) > MaxPayload {
		return fmt.Errorf("intervention requires control input with bounded nonblank stable ID and payload")
	}
	return inbox.ValidateIntervention(input, expectedScope)
}
