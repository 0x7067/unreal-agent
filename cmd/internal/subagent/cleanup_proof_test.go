package subagent

import (
	"context"
	"encoding/json/v2"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
	"testing"
	"time"
)

func TestCleanupProofCanonicalFreshFence(t *testing.T) {
	sup := &Supervisor{config: Config{RequireCleanupProof: true}}
	s := &state{supervisor: sup}
	c := &child{generation: 2, launched: true, cleanupScope: "fresh", lastSequence: 4}
	line := func(seq sessionstore.Sequence, scope string) []byte {
		payload, _ := json.Marshal(inbox.ControlMessage{Mode: inbox.ExecutionDrained, Parameters: inbox.ExecutionDrainReceipt{Version: 1, Scope: scope}})
		raw, err := json.Marshal(sessionstore.Item{Sequence: seq, Kind: sessionstore.ItemInput, Data: inbox.Input{ID: "receipt", Kind: inbox.InputControl, Payload: payload}})
		if err != nil {
			t.Fatal(err)
		}
		return raw
	}
	s.line(c, 1, line(5, "fresh"))
	s.line(c, 2, line(4, "fresh"))
	s.line(c, 2, line(5, "old"))
	if c.cleanupProven {
		t.Fatal("stale generation, sequence or scope accepted")
	}
	s.line(c, 2, []byte(`{"type":"execution_drained","Scope":"fresh"}`))
	if c.cleanupProven {
		t.Fatal("noncanonical event accepted")
	}
	s.line(c, 2, line(6, "fresh"))
	if !c.cleanupProven {
		t.Fatal("fresh canonical receipt rejected")
	}
}

func TestCleanupProofNeverLaunchedNeedsNoProof(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	s, err := New(ctx, Config{RequireCleanupProof: true, Command: []string{"unused"}, Inbox: lifecycleInbox{}})
	if err != nil {
		t.Fatal(err)
	}
	cancel()
	if err = s.Wait(t.Context()); err != nil {
		t.Fatal(err)
	}
}

func TestCleanupProofMissingExitFailsWait(t *testing.T) {
	s := &Supervisor{done: make(chan struct{}), activity: &activityProjection{}, config: Config{RequireCleanupProof: true}}
	state := &state{supervisor: s}
	c := &child{name: "leaf", generation: 1, launched: true}
	state.exited(c, 1, nil)
	close(s.done)
	if err := s.Wait(t.Context()); err == nil {
		t.Fatal("missing proof accepted")
	}
}

func TestCleanupProofActualExitWithoutReceipt(t *testing.T) {
	for _, required := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "required"}[required], func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			s, err := New(ctx, Config{RequireCleanupProof: required, Command: []string{"/bin/sh", "-c", "read request; exit 0", "cleanup-test"}, Inbox: lifecycleInbox{}})
			if err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(toolsubagent.RunPlan{Name: "leaf", Prompt: "test"})
			spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: toolsubagent.RunPlanType, Version: toolsubagent.PlanVersion, Data: data})
			if err != nil {
				t.Fatal(err)
			}
			op := operation.Operation{ID: "leaf", Type: spec.Type, Version: spec.Version, State: spec.State, Status: operation.StatusReady, MaxOutputLength: spec.MaxOutputLength}
			if err := s.run.AddRemoteJob(op); err != nil {
				t.Fatal(err)
			}
			deadline := time.NewTimer(5 * time.Second)
			defer deadline.Stop()
			for {
				select {
				case update := <-s.run.updates:
					if update.Status == operation.StatusCompleted || update.Status == operation.StatusFailed {
						if required && update.Status != operation.StatusFailed {
							t.Fatal("missing proof completed child")
						}
						cancel()
						err := s.Wait(t.Context())
						if (err != nil) != required {
							t.Fatalf("required=%v Wait=%v", required, err)
						}
						return
					}
				case <-deadline.C:
					t.Fatal("child did not finish")
				}
			}
		})
	}
}
