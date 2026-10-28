package workerruntime

import (
	"github.com/unreallabsai/unreal-agent/harness/extension"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/reminder"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	"testing"
	"time"
)

func TestWorkloadForegroundExactOwnershipAndUnknownFence(t *testing.T) {
	r := &workloadRegistry{Registry: tool.NewRegistry(tool.StaticTranslators{}), timer: &timerTranslator{parent: "parent"}}
	for _, tc := range []struct{ mode, owner, target, want string }{{"foreground", "owner", "owner", "foreground"}, {"foreground", "owner", "other", "background"}, {"background", "", "owner", "background"}} {
		spec, err := reminder.NewTaskSpec(reminder.TaskPlan{Version: 1, ParentID: "parent", DueTime: time.Now().UTC(), Ownership: reminder.Ownership{Mode: tc.mode, OwnerTurnID: tc.owner}, Message: "bounded"})
		if err != nil {
			t.Fatal(err)
		}
		op := operation.Operation{ID: "timer", Type: spec.Type, Version: spec.Version, MaxOutputLength: spec.MaxOutputLength, State: spec.State, Status: operation.StatusReady}
		got, handled, err := r.ClassifyRemoteForeground(op, tc.target)
		if err != nil || !handled || got != tc.want {
			t.Fatalf("%+v: %q %v %v", tc, got, handled, err)
		}
		op.Status = operation.StatusCompleted
		if _, _, err = r.ClassifyRemoteForeground(op, tc.target); err == nil {
			t.Fatal("status alone falsely proves timer drain")
		}
		if err = r.RemoteCleanupProof(op); err == nil {
			t.Fatal("terminal timer without checkpoint proves cleanup")
		}
	}
	reg := extension.NewRegistry()
	d := extension.Descriptor{Name: extension.Name, Generation: 1, SchemaVersion: 1, ImplementationVersion: 1, Implementation: "A"}
	if err := reg.Install(d); err != nil {
		t.Fatal(err)
	}
	r.echo = &configuredExtensions{registry: reg}
	for _, owner := range []string{"", "owner"} {
		var spec operation.Spec
		var err error
		if owner == "" {
			spec, err = extension.NewSpec(d, "x")
		} else {
			spec, err = extension.NewSpec(d, "x", owner)
		}
		if err != nil {
			t.Fatal(err)
		}
		op := operation.Operation{ID: "echo", Type: spec.Type, Version: spec.Version, State: spec.State, MaxOutputLength: spec.MaxOutputLength, Status: operation.StatusReady}
		got, handled, err := r.ClassifyRemoteForeground(op, "other")
		if owner == "" {
			if err == nil {
				t.Fatal("ownerless pending Echo silently cancelable")
			}
		} else if err != nil || !handled || got != "background" {
			t.Fatal(got, handled, err)
		}
	}
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: "future.remote", Version: 99, Data: []byte(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	op := operation.Operation{ID: "unknown", Type: spec.Type, Version: spec.Version, State: spec.State, MaxOutputLength: spec.MaxOutputLength, Status: operation.StatusReady}
	if _, handled, err := r.ClassifyRemoteForeground(op, "owner"); handled || err != nil {
		t.Fatal("unknown plan was claimed", handled, err)
	}
	if err := r.RemoteCleanupProof(op); err == nil {
		t.Fatal("unknown cleanup accepted")
	}
}
