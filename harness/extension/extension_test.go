package extension

import (
	"context"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"testing"
	"time"
)

func desc(g uint64, impl string) Descriptor { return Descriptor{Name, g, 1, 1, impl} }
func op(t *testing.T, d Descriptor, id string) operation.Operation {
	t.Helper()
	s, e := NewSpec(d, "hello")
	if e != nil {
		t.Fatal(e)
	}
	return operation.Operation{ID: operation.ID(id), Type: s.Type, Version: s.Version, MaxOutputLength: s.MaxOutputLength, State: s.State, Status: operation.StatusReady}
}
func TestPinnedGenerationAndJoinedCancel(t *testing.T) {
	r := NewRegistry()
	a, b := desc(1, "A"), desc(2, "B")
	if e := r.Install(a); e != nil {
		t.Fatal(e)
	}
	if e := r.Select(1); e != nil {
		t.Fatal(e)
	}
	entered := make(chan struct{})
	gate := make(chan struct{})
	h, e := NewHandler(Config{Registry: r, BeforeExecute: func(ctx context.Context, d Descriptor) error {
		if d.Generation == 1 {
			close(entered)
			select {
			case <-gate:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		return nil
	}})
	if e != nil {
		t.Fatal(e)
	}
	defer h.Close()
	if e = h.AddRemoteJob(op(t, a, "a")); e != nil {
		t.Fatal(e)
	}
	select {
	case <-entered:
	case <-time.After(time.Second):
		t.Fatal("worker not started")
	}
	if e = r.Install(b); e != nil {
		t.Fatal(e)
	}
	if e = r.Select(2); e != nil {
		t.Fatal(e)
	}
	if e = r.Remove(1); e == nil {
		t.Fatal("removed leased A")
	}
	if e = h.AddRemoteJob(op(t, b, "b")); e != nil {
		t.Fatal(e)
	}
	v := <-h.RemoteJobUpdates()
	s, _, e := Decode(v)
	if e != nil || s.TerminalResult != "B:hello" {
		t.Fatalf("B: %+v %v", s, e)
	}
	close(gate)
	v = <-h.RemoteJobUpdates()
	s, _, e = Decode(v)
	if e != nil || s.TerminalResult != "A:hello" {
		t.Fatalf("A: %+v %v", s, e)
	}
	if e = h.Close(); e != nil {
		t.Fatal(e)
	}
	if e = h.AwaitStopped(context.Background()); e != nil {
		t.Fatal(e)
	}
	if e = r.Remove(1); e != nil {
		t.Fatal(e)
	}
	if e = ValidateRecovery(v, r); e != nil {
		t.Fatal("historical result needs no code", e)
	}
	if e = ValidateRecovery(op(t, a, "missing"), r); e == nil {
		t.Fatal("missing A recovered using B")
	}
}
func TestCancelJoinsGate(t *testing.T) {
	r := NewRegistry()
	_ = r.Install(desc(1, "A"))
	entered := make(chan struct{})
	exited := make(chan struct{})
	h, _ := NewHandler(Config{Registry: r, BeforeExecute: func(ctx context.Context, _ Descriptor) error {
		close(entered)
		<-ctx.Done()
		close(exited)
		return ctx.Err()
	}})
	defer h.Close()
	if e := h.AddRemoteJob(op(t, desc(1, "A"), "cancel")); e != nil {
		t.Fatal(e)
	}
	<-entered
	if e := h.CancelRemoteJob("cancel", ""); e != nil {
		t.Fatal(e)
	}
	select {
	case <-exited:
	default:
		t.Fatal("gate not joined")
	}
	v := <-h.RemoteJobUpdates()
	if v.Status != operation.StatusCanceled {
		t.Fatal(v.Status)
	}
}
func TestRegistryAndVersions(t *testing.T) {
	r := NewRegistry()
	for g := uint64(1); g <= MaxGenerations; g++ {
		if e := r.Install(desc(g, "A")); e != nil {
			t.Fatal(e)
		}
	}
	_ = r.Select(1)
	bad := desc(17, "B")
	if e := r.Install(bad); e == nil {
		t.Fatal("capacity ignored")
	}
	bad = desc(18, "B")
	bad.SchemaVersion = 2
	if e := r.Install(bad); e == nil {
		t.Fatal("schema ignored")
	}
	if e := r.Select(99); e == nil {
		t.Fatal("unknown selection")
	}
	if e := r.Remove(1); e == nil {
		t.Fatal("removed selected")
	}
	d, _ := r.Selected()
	if d.Generation != 1 {
		t.Fatal("failed install changed selection")
	}
	current := op(t, desc(1, "A"), "bad")
	current.Version++
	if e := ValidateRecovery(current, r); e == nil {
		t.Fatal("operation version ignored")
	}
	if e := r.Remove(2); e != nil {
		t.Fatal(e)
	}
	if e := r.Install(desc(2, "B")); e == nil {
		t.Fatal("generation reused")
	}
}
