package lifecycleguard

import (
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/ingress"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

func TestStandaloneOwnerFencesAfterWorkerLeaseReleased(t *testing.T) {
	root := t.TempDir()
	if err := os.Chmod(root, 0700); err != nil {
		t.Fatal(err)
	}
	host, err := localfile.AcquireLease(root, "host-session")
	if err != nil {
		t.Fatal(err)
	}
	defer host.Close()
	worker, err := localfile.AcquireLease(root, "session")
	if err != nil {
		t.Fatal(err)
	}
	if err := worker.Close(); err != nil {
		t.Fatal(err)
	}
	if lease, err := AcquireStandaloneOwner(root, "session"); !errors.Is(err, localfile.ErrSessionLocked) {
		if lease != nil {
			lease.Close()
		}
		t.Fatalf("one-shot bypassed retained host authority: %v", err)
	}
}
func TestStandaloneOwnerRefusesJournalOnlyLifecycleBeforeCanonicalCommit(t *testing.T) {
	root := t.TempDir()
	os.Chmod(root, 0700)
	journalRoot := filepath.Join(root, "ingress")
	os.Mkdir(journalRoot, 0700)
	j, err := ingress.Open(journalRoot, "session")
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(inbox.ControlMessage{Mode: inbox.ShutdownSession, Parameters: inbox.ShutdownSessionRequest{Version: 1, Scope: "old-host/1"}})
	input := inbox.Input{ID: "shutdown-not-delivered", Kind: inbox.InputControl, Payload: data}
	if err := j.Submit(t.Context(), input); err != nil {
		t.Fatal(err)
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	if lease, err := AcquireStandaloneOwner(root, "session"); !errors.Is(err, sessionstore.ErrLifecycleRecoveryRequired) {
		if lease != nil {
			lease.Close()
		}
		t.Fatalf("journal-only intent not fenced: %v", err)
	}
	// Failed validation released the actual authority lease but did not erase data.
	lease, err := localfile.AcquireLease(root, "host-session")
	if err != nil {
		t.Fatal(err)
	}
	lease.Close()
	j, err = ingress.Open(journalRoot, "session")
	if err != nil {
		t.Fatal(err)
	}
	defer j.Close()
	if len(j.Pending()) != 1 {
		t.Fatal("validation lost lifecycle intent")
	}
}
func TestStandaloneOwnerDoesNotCreateUnrelatedJournal(t *testing.T) {
	root := t.TempDir()
	os.Chmod(root, 0700)
	lease, err := AcquireStandaloneOwner(root, "new-session")
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	if _, err := os.Lstat(filepath.Join(root, "ingress")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("created unrelated journal: %v", err)
	}
}
