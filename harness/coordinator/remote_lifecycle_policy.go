package coordinator

import (
	"errors"
	"github.com/unreallabsai/unreal-agent/harness/operation"
)

// Optional command-layer policy on the configured registry. These methods must
// be pure. Unknown plans return handled=false and keep the conservative native
// rules. Terminal cleanup must validate durable executor proof, not local joins.
// No callbacks or control credentials enter persisted session data.
type RemoteLifecyclePolicy interface {
	ClassifyRemoteForeground(operation.Operation, string) (mode string, handled bool, err error)
	RemoteCleanupProof(operation.Operation) error
}

func (c *coordinator) abortCleanupProof(ids []operation.ID) error {
	var local []operation.ID
	policy, _ := c.dependencies.Tools.(RemoteLifecyclePolicy)
	for _, id := range ids {
		current, ok := c.state.operations[id]
		if !ok {
			return errors.New("abort operation missing")
		}
		if current.Type == operation.TypeRemoteJob {
			if policy == nil {
				return errors.New("remote cleanup policy unavailable")
			}
			if err := policy.RemoteCleanupProof(current); err != nil {
				return err
			}
		} else {
			local = append(local, id)
		}
	}
	if len(local) > 0 {
		verifier, ok := c.dependencies.Operations.(operation.LocalCleanupVerifier)
		if !ok {
			return errors.New("local cleanup proof unavailable")
		}
		return verifier.LocalCleanupProof(local)
	}
	return nil
}
