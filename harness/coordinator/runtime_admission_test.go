package coordinator

import (
	"context"
	"encoding/json/v2"
	"errors"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"testing"
	"testing/synctest"

	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/taskgraph"
)

// TestRuntimeAdmissionKeepsCoordinatorResponsive uses the real runtime, manager,
// coordinator and inbox. The model and durable store boundary are in-memory fakes.
// Neither persistence nor delivery may depend on releasing the runtime factory.
func TestRuntimeAdmissionKeepsCoordinatorResponsive(t *testing.T) {
	workspace := t.TempDir()
	base := t.TempDir()
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		entered := make(chan struct{})
		release := make(chan struct{})
		defer close(release)
		r, err := taskgraph.New(ctx, taskgraph.Config{
			Workspace: workspace, BaseDirectory: base,
			NewAgentHandlers: func(context.Context) ([]operation.RemoteJobHandler, error) {
				close(entered)
				<-release
				return nil, errors.New("injected factory gate released")
			},
		})
		if err != nil {
			t.Fatal(err)
		}
		m := operation.NewLocalOperationManager(ctx, r.Handlers()...)
		op := func(id string, typ operation.RemoteJobPlanType, plan any) operation.Operation {
			data, e := json.Marshal(plan)
			if e != nil {
				t.Fatal(e)
			}
			spec, e := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: typ, Version: taskgraph.PlanVersion, Data: data})
			if e != nil {
				t.Fatal(e)
			}
			return operation.Operation{ID: operation.ID(id), Type: spec.Type, Version: spec.Version, State: spec.State, Status: operation.StatusReady, MaxOutputLength: spec.MaxOutputLength}
		}
		first := op("run", taskgraph.RunPlanType, taskgraph.RunPlan{Name: "g", Tasks: []taskgraph.Task{{ID: "a", Prompt: "fake", Reads: []string{}, Writes: []string{}, Acceptance: "check"}}})
		if err := m.Add(first); err != nil {
			t.Fatal(err)
		}
		<-entered // runtime is now blocked in the factory
		second := op("status", taskgraph.ControlPlanType, taskgraph.ControlPlan{Name: "g", Action: "status"})
		if err := m.Add(second); err != nil {
			t.Fatal(err)
		}
		synctest.Wait() // second request admitted while runtime execution is still blocked

		run := newStopTestRun(t, 0)
		run.current.dependencies.Operations = m
		// Coordinator startup must dispatch a third operation without waiting for
		// runtime execution of either of the earlier requests.
		run.current.dependencies.Restored.Operations = []operation.Operation{op("third", taskgraph.ControlPlanType, taskgraph.ControlPlan{Name: "g", Action: "status"})}
		go func() { run.done <- run.current.Run(ctx) }()
		synctest.Wait()
		message := externalEvent(t, 0, "user-message", "Are you there?")
		if err := run.inputs.Submit(ctx, message); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		synctest.Sleep(2 * slurpIdleTimeout)
		synctest.Wait()
		if len(run.store.appendedInputs) != 1 || run.store.appendedInputs[0].ID != message.ID {
			t.Fatalf("accepted input was not persisted: %#v", run.store.appendedInputs)
		}
		if len(run.calls) != 1 {
			t.Fatalf("model requests = %d, want 1 before factory release", len(run.calls))
		}
		delivered := false
		for _, item := range run.calls[0].request.Input {
			if item.Type == llm.ItemMessage {
				value := item.Data.(llm.Message)
				delivered = delivered || value.Role == llm.RoleUser && value.Text == "Are you there?"
			}
		}
		if !delivered {
			t.Fatalf("model request did not include user input: %#v", run.calls[0].request.Input)
		}
		select {
		case <-release:
			t.Fatal("runtime gate released before input delivery")
		default:
		}
		t.Log("accepted input persisted and reached model before runtime gate release")
		cancel()
		synctest.Wait()
		if err := <-run.done; !errors.Is(err, context.Canceled) {
			t.Fatalf("shutdown = %v", err)
		}
	})
}
