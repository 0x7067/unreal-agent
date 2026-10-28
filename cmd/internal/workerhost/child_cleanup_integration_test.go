package workerhost

import (
	"context"
	"encoding/json/v2"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/subagent"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

// This launches the actual runner via TestMain's agent-runner entry point.
// No fixture writes a receipt: strict Supervisor.Wait accepts only the runner's
// fresh nonce-bound canonical receipt, followed by actual reader/process joins.
func TestRealRunnerSupervisorCleanupProof(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		completed(w, 1, assistant(1, "leaf report"))
	}))
	defer server.Close()
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv("HOME", directory)
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(directory, "config"))
	t.Setenv("UNREAL_HARNESS_LLM_BASE_URL", server.URL)
	t.Setenv("UNREAL_HARNESS_LLM_PROVIDER", "openai")
	t.Setenv("UNREAL_HARNESS_LLM_API_KEY", "fake-loopback-only")
	t.Setenv("OPENAI_API_KEY", "fake-loopback-only")
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	inputs, err := inbox.New(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	supervisor, err := subagent.New(ctx, subagent.Config{RequireCleanupProof: true,
		Command: []string{executable, "agent-runner"}, SessionDirectory: directory,
		Workspace: directory, Model: "fake", Inbox: inputs, Stderr: os.Stderr})
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		joinCtx, stop := context.WithTimeout(context.Background(), 5*time.Second)
		defer stop()
		if err := supervisor.Wait(joinCtx); err != nil {
			t.Errorf("strict supervisor join: %v", err)
		}
		for range inputs.Output() {
		}
	}()
	data, _ := json.Marshal(toolsubagent.RunPlan{Name: "actual-leaf", Prompt: "report then drain"})
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: toolsubagent.RunPlanType, Version: toolsubagent.PlanVersion, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	handler := supervisor.Handlers()[0]
	if err := handler.AddRemoteJob(operation.Operation{ID: "actual-leaf", Type: spec.Type,
		Version: spec.Version, State: spec.State, MaxOutputLength: spec.MaxOutputLength, Status: operation.StatusReady}); err != nil {
		t.Fatal(err)
	}
	deadline := time.NewTimer(15 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case update := <-handler.RemoteJobUpdates():
			switch update.Status {
			case operation.StatusFailed, operation.StatusCanceled:
				t.Fatalf("actual runner failed: %+v", update)
			case operation.StatusCompleted:
				cancel()
				if err := supervisor.Wait(t.Context()); err != nil {
					t.Fatalf("strict cleanup proof rejected: %v", err)
				}
				snapshot := supervisor.ActivitySnapshot()
				if !snapshot.SupervisorJoined || len(snapshot.Children) != 1 || !snapshot.Children[0].ProcessWaitReturned || !snapshot.Children[0].ReaderReturned {
					t.Fatalf("missing actual joins: %+v", snapshot)
				}
				store, err := localfile.New(directory)
				if err != nil {
					t.Fatal(err)
				}
				history, err := sessionstore.ReadLifecycle(t.Context(), store, session.ID(snapshot.Children[0].SessionID))
				if err != nil || history.LastDrainSequence == 0 {
					t.Fatalf("actual child has no canonical receipt: %+v, %v", history, err)
				}
				return
			}
		case <-deadline.C:
			t.Fatalf("actual runner did not finish: %+v", supervisor.ActivitySnapshot())
		}
	}
}
