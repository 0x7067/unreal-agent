// Package lifecycleguard fences writer entrypoints during unfinished controls.
// It never infers cleanup or recovery permission from an absent process/socket.
package lifecycleguard

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/ingress"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

// ValidatePending refuses automatic delivery of an old generation's accepted
// lifecycle intent. Journal admission precedes canonical persistence; checking
// only session history leaves a crash window before the intent reached a worker.
func ValidatePending(inputs []inbox.Input) error {
	for _, input := range inputs {
		if input.Kind != inbox.InputControl {
			continue
		}
		control, err := input.DecodeControlMessage()
		if err != nil {
			return err
		}
		if control.Mode == inbox.AbortForeground || control.Mode == inbox.ShutdownSession {
			return fmt.Errorf("%w: durable ingress lifecycle intent %s needs explicit recovery", sessionstore.ErrLifecycleRecoveryRequired, input.ID)
		}
	}
	return nil
}

// AcquireStandaloneOwner shares the host/front-end authority lease with resident
// hosts. Take it BEFORE the session writer lease and keep it through all cleanup.
// Private workers borrow their parent's authority and must not acquire it again.
func AcquireStandaloneOwner(directory string, id session.ID) (lease *localfile.Lease, result error) {
	lease, err := localfile.AcquireLease(directory, session.ID("host-"+string(id)))
	if err != nil {
		return nil, fmt.Errorf("own session authority: %w", err)
	}
	defer func() {
		if result != nil {
			result = errors.Join(result, lease.Close())
			lease = nil
		}
	}()
	root := filepath.Join(directory, "ingress")
	// Do not create journals for unrelated standalone sessions. Open validates
	// ownership, private modes and no-follow traversal before reading an existing
	// journal; same-UID concurrent path replacement remains outside the trust model.
	_, err = os.Lstat(filepath.Join(root, string(id)))
	if errors.Is(err, os.ErrNotExist) {
		return lease, nil
	}
	if err != nil {
		return lease, err
	}
	journal, err := ingress.Open(root, id)
	if err != nil {
		return lease, err
	}
	err = ValidatePending(journal.Pending())
	return lease, errors.Join(err, journal.Close())
}
