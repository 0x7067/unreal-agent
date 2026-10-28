package lifecycleproof

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/taskgraph"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

func remote(t *testing.T, kind operation.RemoteJobPlanType, version operation.RemoteJobPlanVersion) operation.Operation {
	t.Helper()
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: kind, Version: version, Data: []byte(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	return operation.Operation{ID: "remote", Type: spec.Type, Version: spec.Version, State: spec.State, MaxOutputLength: spec.MaxOutputLength, Status: operation.StatusCompleted}
}

func TestKnownKindsAndVersions(t *testing.T) {
	for _, kind := range []operation.RemoteJobPlanType{toolsubagent.RunPlanType, toolsubagent.MessagePlanType, toolsubagent.ControlPlanType, taskgraph.RunPlanType, taskgraph.ControlPlanType, "custom.run", "subagent.future"} {
		for _, version := range []operation.RemoteJobPlanVersion{1, 2, 99} {
			t.Run(fmt.Sprintf("%s/%d", kind, version), func(t *testing.T) {
				op := remote(t, kind, version)
				evidence := &operation.CleanupUncertainError{UnprovenRemoteIDs: []operation.ID{op.ID}}
				got := Reconcile(evidence, nil, map[operation.ID]operation.Operation{op.ID: op}, map[operation.RemoteJobPlanType]operation.RemoteJobPlanVersion{kind: version})
				wantOK := version == 1 && kind != "custom.run" && kind != "subagent.future"
				if (got == nil) != wantOK {
					t.Fatalf("got %v, want success %v", got, wantOK)
				}
			})
		}
	}
}

func TestFailuresPreserveEvidence(t *testing.T) {
	op := remote(t, toolsubagent.RunPlanType, 1)
	unrelated := errors.New(strings.Repeat("large", 10000))
	tests := []struct {
		name     string
		edit     func(*operation.Operation)
		evidence error
		owners   error
		joined   map[operation.RemoteJobPlanType]operation.RemoteJobPlanVersion
		missing  bool
	}{
		{name: "no capability"},
		{name: "empty capability", joined: map[operation.RemoteJobPlanType]operation.RemoteJobPlanVersion{}},
		{name: "wrong capability", joined: map[operation.RemoteJobPlanType]operation.RemoteJobPlanVersion{toolsubagent.RunPlanType: 2}},
		{name: "missing ID", missing: true},
		{name: "mismatched ID", edit: func(o *operation.Operation) { o.ID = "different" }},
		{name: "wrong type", edit: func(o *operation.Operation) { o.Type = operation.TypeShell }},
		{name: "canonical unsupported", edit: func(o *operation.Operation) { o.Status = operation.StatusUnsupported }},
		{name: "wrong envelope", edit: func(o *operation.Operation) { o.Version++ }},
		{name: "bad state", edit: func(o *operation.Operation) { o.State = []byte(`{`) }},
		{name: "missing plan", edit: func(o *operation.Operation) { o.State = []byte(`{}`) }},
		{name: "zero plan version", edit: func(o *operation.Operation) {
			o.State = []byte(`{"Plan":{"Type":"subagent.run","Version":0,"Data":{}}}`)
		}},
		{name: "zero output limit", edit: func(o *operation.Operation) { o.MaxOutputLength = 0 }},
		{name: "unknown", evidence: &operation.CleanupUncertainError{UnprovenRemoteIDs: []operation.ID{"remote"}, UnknownRemoteIDs: []operation.ID{"actual-unknown"}}},
		{name: "unsupported", evidence: &operation.CleanupUncertainError{UnprovenRemoteIDs: []operation.ID{"remote"}, UnsupportedOperationIDs: []operation.ID{"actual-unsupported"}}},
		{name: "empty evidence", evidence: &operation.CleanupUncertainError{}},
		{name: "sentinel", evidence: operation.ErrCleanupUncertain},
		{name: "unrelated", evidence: unrelated},
		{name: "owner failure", owners: unrelated},
		{name: "owner only", evidence: errors.New("marker"), owners: unrelated},
		{name: "second ID missing", evidence: &operation.CleanupUncertainError{UnprovenRemoteIDs: []operation.ID{"remote", "missing"}}},
		{name: "empty ID", evidence: &operation.CleanupUncertainError{UnprovenRemoteIDs: []operation.ID{""}}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := op
			if tt.edit != nil {
				tt.edit(&current)
			}
			evidence := tt.evidence
			if evidence == nil {
				evidence = &operation.CleanupUncertainError{UnprovenRemoteIDs: []operation.ID{"remote"}}
			}
			canonical := map[operation.ID]operation.Operation{"remote": current}
			if tt.missing {
				delete(canonical, "remote")
			}
			joined := tt.joined
			if joined == nil && tt.name != "no capability" {
				joined = map[operation.RemoteJobPlanType]operation.RemoteJobPlanVersion{toolsubagent.RunPlanType: 1}
			}
			got := Reconcile(evidence, tt.owners, canonical, joined)
			if got == nil || !errors.Is(got, evidence) {
				t.Fatalf("lost manager evidence: %v", got)
			}
			if tt.owners != nil && !errors.Is(got, tt.owners) {
				t.Fatal("lost owner evidence")
			}
			if len(got.Error()) > 128 {
				t.Fatal("unbounded diagnostic")
			}
			var original, retained *operation.CleanupUncertainError
			if errors.As(evidence, &original) && (!errors.As(got, &retained) || retained != original) {
				t.Fatal("changed structured evidence")
			}
		})
	}
}

func TestErrorsAndImmutability(t *testing.T) {
	op := remote(t, toolsubagent.RunPlanType, 1)
	canonical := map[operation.ID]operation.Operation{op.ID: op}
	joined := map[operation.RemoteJobPlanType]operation.RemoteJobPlanVersion{toolsubagent.RunPlanType: 1}
	evidence := &operation.CleanupUncertainError{UnprovenRemoteIDs: []operation.ID{op.ID}}
	independent := errors.New("independent")
	for _, err := range []error{errors.Join(evidence, independent), errors.Join(independent, evidence), fmt.Errorf("wrapped: %w", evidence), errors.Join(evidence)} {
		got := Reconcile(err, nil, canonical, joined)
		if got == nil || !errors.Is(got, err) {
			t.Fatal("wrapper/join erased")
		}
		if errors.Is(err, independent) && !errors.Is(got, independent) {
			t.Fatal("independent error erased")
		}
	}
	if got := Reconcile(nil, independent, nil, nil); got == nil || !errors.Is(got, independent) {
		t.Fatal("nil manager erased owner")
	}
	if Reconcile(nil, nil, nil, nil) != nil {
		t.Fatal("local-only failure")
	}
	before := string(op.State)
	var wg sync.WaitGroup
	for range 8 {
		wg.Go(func() {
			for range 20 {
				if Reconcile(evidence, nil, canonical, joined) != nil {
					t.Error("proof rejected")
				}
			}
		})
	}
	wg.Wait()
	if string(op.State) != before || !reflect.DeepEqual(evidence.UnprovenRemoteIDs, []operation.ID{"remote"}) || canonical[op.ID].ID != op.ID || joined[toolsubagent.RunPlanType] != 1 {
		t.Fatal("mutated input")
	}
}

// No process or remote handler is started: this obtains genuine immutable
// manager evidence for an unsupported operation through the concrete stop API.
func TestConcreteManagerEvidence(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	manager := operation.NewLocalOperationManager(ctx)
	if err := manager.Add(operation.Operation{ID: "unsupported", Type: "future", Version: 1}); err != nil {
		cancel()
		t.Fatal(err)
	}
	for update := range manager.Updates() {
		if update.ID == "unsupported" {
			break
		}
	}
	cancel()
	err := manager.AwaitStopped(t.Context())
	var evidence *operation.CleanupUncertainError
	if !errors.As(err, &evidence) || len(evidence.UnsupportedOperationIDs) == 0 {
		t.Fatalf("missing concrete evidence: %v", err)
	}
	before := append([]operation.ID(nil), evidence.UnsupportedOperationIDs...)
	got := Reconcile(err, nil, nil, nil)
	if !errors.Is(got, operation.ErrCleanupUncertain) {
		t.Fatal("unsupported erased")
	}
	var retained *operation.CleanupUncertainError
	if !errors.As(got, &retained) || retained != evidence || !reflect.DeepEqual(before, evidence.UnsupportedOperationIDs) {
		t.Fatal("evidence changed")
	}
}
