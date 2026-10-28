package durablechild

import (
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/subagent"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

// The connection fixture intentionally omits the optional transport name; run-v2
// intent requires a valid explicit name as well as background lifetime.
func handlerConfigFixture(t *testing.T) (Config, Request) {
	t.Helper()
	cfg, req := fixture(t)
	req.Name = "helper"
	return cfg, req
}

func remoteSpec(t *testing.T, typ operation.RemoteJobPlanType, version operation.RemoteJobPlanVersion, payload any) operation.Operation {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: typ, Version: version, Data: data})
	if err != nil {
		t.Fatal(err)
	}
	return operation.Operation{ID: "op", Type: spec.Type, Version: spec.Version, MaxOutputLength: spec.MaxOutputLength, State: spec.State, Status: operation.StatusReady}
}
func handlerRun(t *testing.T, req Request) operation.Operation {
	t.Helper()
	op := remoteSpec(t, toolsubagent.RunPlanType, toolsubagent.DurablePlanVersion, toolsubagent.DurableRunPlan{Version: 1, ParentID: req.ParentID, Run: toolsubagent.RunPlan{Name: req.Name, Prompt: req.Prompt, Lifetime: req.Lifetime}})
	op.ID = req.OperationID
	if _, err := toolsubagent.DecodeDurableRun(op); err != nil {
		t.Fatalf("invalid run fixture: %v", err)
	}
	return op
}
func handlerFixture(t *testing.T, cfg Config, req Request) (*Handler, *subagent.Hub, context.CancelFunc) {
	t.Helper()
	ctx, cancel := context.WithCancel(t.Context())
	hub := subagent.NewHub(ctx)
	h, err := NewHandler(ctx, HandlerConfig{Owner: cfg, ParentID: req.ParentID, Hub: hub})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	stop := func() {
		cancel()
		wait, done := context.WithTimeout(context.Background(), 2*time.Second)
		defer done()
		if err := h.Wait(wait); err != nil {
			t.Errorf("handler join: %v", err)
		}
		if err := hub.Wait(wait); err != nil {
			t.Errorf("hub join: %v", err)
		}
	}
	t.Cleanup(stop)
	return h, hub, stop
}
func handlerState(t *testing.T, op operation.Operation, mutate func(*operation.RemoteJobState)) operation.Operation {
	t.Helper()
	s, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		t.Fatal(err)
	}
	mutate(&s)
	op.State, err = json.Marshal(s)
	if err != nil {
		t.Fatal(err)
	}
	return op
}
func handlerHandle(t *testing.T, op operation.Operation) Handle {
	t.Helper()
	s, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		t.Fatal(err)
	}
	var v Handle
	if err := json.Unmarshal(s.Handle, &v); err != nil {
		t.Fatal(err)
	}
	return v
}
func handlerAwait(t *testing.T, h *Handler) operation.Operation {
	t.Helper()
	select {
	case op, ok := <-h.RemoteJobUpdates():
		if !ok {
			t.Fatal("updates closed")
		}
		return op
	case <-time.After(8 * time.Second):
		t.Fatal("missing handler update")
	}
	return operation.Operation{}
}
func noReservation(t *testing.T, cfg Config) {
	t.Helper()
	if _, err := os.Lstat(filepath.Join(cfg.Directory, "durable-children")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("unexpected reservation effects: %v", err)
	}
}
func savedHandle(t *testing.T, cfg Config, req Request) Handle {
	t.Helper()
	r, err := plan(cfg, req)
	if err != nil {
		t.Fatal(err)
	}
	return Handle{Version: 1, SessionID: string(r.Link.ChildID), Handle: "old-capability", Generation: 1, Link: r.Link, Phase: "uncertain", ResultDeliveryID: "child-result-" + r.Link.Key}
}
func withHandlerHandle(t *testing.T, op operation.Operation, v Handle) operation.Operation {
	t.Helper()
	return handlerState(t, op, func(s *operation.RemoteJobState) {
		var err error
		s.Handle, err = json.Marshal(v)
		if err != nil {
			t.Fatal(err)
		}
	})
}

func TestHandlerRequiresParentAndHub(t *testing.T) {
	cfg, req := handlerConfigFixture(t)
	ctx, cancel := context.WithCancel(t.Context())
	hub := subagent.NewHub(ctx)
	defer func() {
		cancel()
		wait, done := context.WithTimeout(context.Background(), time.Second)
		defer done()
		if err := hub.Wait(wait); err != nil {
			t.Error(err)
		}
	}()
	for _, c := range []HandlerConfig{{Owner: cfg, ParentID: req.ParentID}, {Owner: cfg, Hub: hub}} {
		if h, err := NewHandler(ctx, c); err == nil || h != nil {
			t.Fatal("accepted missing parent/hub")
		}
	}
	h, _, _ := handlerFixture(t, cfg, req)
	if h.RemoteJobPlanType() != toolsubagent.RunPlanType || h.RemoteJobPlanVersion() != toolsubagent.DurablePlanVersion {
		t.Fatal("wrong handler route")
	}
	if err := h.OwnershipProof(); err != nil {
		t.Fatal(err)
	}
}

func TestHandlerRejectsIntentBeforeBindingOrIO(t *testing.T) {
	for _, name := range []string{"parent mismatch", "malformed handle", "mismatched link", "unknown phase", "pending answer sequence", "pending answer turn", "unknown handle member", "task lifetime", "default lifetime"} {
		t.Run(name, func(t *testing.T) {
			cfg, req := handlerConfigFixture(t)
			h, hub, _ := handlerFixture(t, cfg, req)
			op := handlerRun(t, req)
			v := savedHandle(t, cfg, req)
			switch name {
			case "parent mismatch":
				other := req
				other.ParentID = "other-parent"
				op = handlerRun(t, other)
			case "malformed handle":
				op = handlerState(t, op, func(s *operation.RemoteJobState) { s.Handle = []byte(`"not-a-handle"`) })
			case "mismatched link":
				v.Link.PlanDigest = "foreign"
				op = withHandlerHandle(t, op, v)
			case "unknown phase":
				v.Phase = "drained"
				op = withHandlerHandle(t, op, v)
			case "pending answer sequence":
				v.AnswerSequence = 1
				op = withHandlerHandle(t, op, v)
			case "pending answer turn":
				v.AnswerTurnID = "turn"
				op = withHandlerHandle(t, op, v)
			case "unknown handle member":
				op = withHandlerHandle(t, op, v)
				op = handlerState(t, op, func(s *operation.RemoteJobState) {
					s.Handle = append(s.Handle[:len(s.Handle)-1], []byte(`,"Unknown":true}`)...)
				})
			case "task lifetime", "default lifetime":
				op = handlerState(t, op, func(s *operation.RemoteJobState) {
					var p toolsubagent.DurableRunPlan
					if err := json.Unmarshal(s.Plan.Data, &p); err != nil {
						t.Fatal(err)
					}
					p.Run.Lifetime = "task"
					if name == "default lifetime" {
						p.Run.Lifetime = ""
					}
					s.Plan.Data, _ = json.Marshal(p)
				})
			}
			if err := h.AddRemoteJob(op); err == nil {
				t.Fatal("accepted invalid intent")
			}
			if len(hub.Snapshot().Agents) != 0 || len(h.entries) != 0 {
				t.Fatal("invalid intent bound authority")
			}
			noReservation(t, cfg)
		})
	}
}

func TestHandlerCapacityBeforeBindingOrIO(t *testing.T) {
	for _, name := range []string{"entries", "bytes", "individual state"} {
		t.Run(name, func(t *testing.T) {
			cfg, req := handlerConfigFixture(t)
			h, hub, _ := handlerFixture(t, cfg, req)
			h.mu.Lock()
			if name == "entries" {
				for i := 0; i < 128; i++ {
					h.entries[operation.ID(strings.Repeat("x", i+1))] = &entry{}
				}
			} else if name == "bytes" {
				h.bytes = 16 << 20
			}
			h.mu.Unlock()
			op := handlerRun(t, req)
			if name == "individual state" {
				op = handlerState(t, op, func(s *operation.RemoteJobState) { s.OutputDirectory = strings.Repeat("x", 1<<20) })
			}
			if err := h.AddRemoteJob(op); err == nil {
				t.Fatal("capacity accepted")
			}
			if len(hub.Snapshot().Agents) != 0 {
				t.Fatal("capacity bound authority")
			}
			noReservation(t, cfg)
		})
	}
}

func TestHandlerDuplicateOperationRejectsChangedPrompt(t *testing.T) {
	cfg, req := handlerConfigFixture(t)
	h, hub, _ := handlerFixture(t, cfg, req)
	op := handlerRun(t, req)
	if err := h.AddRemoteJob(op); err != nil {
		t.Fatal(err)
	}
	first := handlerAwait(t, h)
	if err := h.AddRemoteJob(op); err != nil {
		t.Fatalf("exact duplicate: %v", err)
	}
	req.Prompt = "different immutable prompt"
	if err := h.AddRemoteJob(handlerRun(t, req)); err == nil {
		t.Error("same operation ID accepted changed prompt")
	}
	if len(hub.Snapshot().Agents) != 1 || len(h.entries) != 1 {
		t.Fatal("duplicate created authority")
	}
	if got := handlerHandle(t, first); got.Phase != "uncertain" {
		t.Fatal(got)
	}
}

func TestHandlerReservationAndFreshParentAuthority(t *testing.T) {
	cfg, req := handlerConfigFixture(t)
	op := handlerRun(t, req)
	h1, _, stop1 := handlerFixture(t, cfg, req)
	if err := h1.AddRemoteJob(op); err != nil {
		t.Fatal(err)
	}
	update1 := handlerAwait(t, h1)
	v1 := handlerHandle(t, update1)
	if update1.Status != operation.StatusAwaiting || v1.Phase != "uncertain" || v1.Diagnostic == "" {
		t.Fatalf("not explicit uncertainty: %+v", v1)
	}
	r, err := readRecord(filepath.Join(v1.Link.Directory, "launch.json"))
	if err != nil || r.Link != v1.Link {
		t.Fatalf("reservation: %+v %v", r, err)
	}
	e1 := h1.entries[op.ID]
	stop1()
	if _, err := h1.control(t.Context(), e1, subagent.ControlPlan{Action: "status", Handle: v1.Handle, Generation: v1.Generation}); !errors.Is(err, context.Canceled) {
		t.Fatalf("old context control: %v", err)
	}
	h2, _, _ := handlerFixture(t, cfg, req)
	if err := h2.AddRemoteJob(update1); err != nil {
		t.Fatal(err)
	}
	update2 := handlerAwait(t, h2)
	v2 := handlerHandle(t, update2)
	if v2.Handle == v1.Handle || v2.Link != v1.Link || v2.ResultDeliveryID != v1.ResultDeliveryID {
		t.Fatalf("bad authority rotation: old=%+v new=%+v", v1, v2)
	}
	if v2.Phase != "uncertain" || update2.Status != operation.StatusAwaiting {
		t.Fatal("replacement invented execution")
	}
	e2 := h2.entries[op.ID]
	for _, p := range []subagent.ControlPlan{{Action: "status", Handle: v1.Handle, Generation: v1.Generation}, {Action: "stop", Handle: v2.Handle, Generation: v2.Generation + 1}, {Action: "message", Handle: v2.Handle, Generation: v2.Generation, Message: "not replayable"}} {
		if _, err := h2.control(t.Context(), e2, p); err == nil {
			t.Fatalf("invalid control accepted: %+v", p)
		}
	}
	got, err := h2.control(t.Context(), e2, subagent.ControlPlan{Action: "status", Handle: v2.Handle, Generation: v2.Generation})
	if err != nil {
		t.Fatal(err)
	}
	observation, ok := got.(Handle)
	if !ok || observation.Link != v2.Link || observation.Phase != "uncertain" {
		t.Fatalf("not observation: %#v", got)
	}
	if _, ok := got.(toolsubagent.ControlOutcome); ok {
		t.Fatal("status manufactured drain outcome")
	}
	if _, err := h2.control(t.Context(), e2, subagent.ControlPlan{Action: "stop", Handle: v2.Handle, Generation: v2.Generation}); !errors.Is(err, ErrUncertain) {
		t.Fatalf("unavailable stop: %v", err)
	}
	if err := h2.OwnershipProof(); err == nil {
		t.Fatal("independent entry claimed global drain without manager")
	}
}

func TestHandlerRetainedOwnershipIsNotDrain(t *testing.T) {
	cfg, req := handlerConfigFixture(t)
	h, hub, stop := handlerFixture(t, cfg, req)
	op := handlerRun(t, req)
	if err := h.AdoptRetained(op); err == nil {
		t.Fatal("adopted nonterminal history")
	}
	v := savedHandle(t, cfg, req)
	v.Phase = "reported_owner_retained"
	v.AnswerSequence = 7
	v.AnswerTurnID = "canonical-answer"
	op = withHandlerHandle(t, op, v)
	op.Status = operation.StatusCompleted
	if err := h.AdoptRetained(op); err != nil {
		t.Fatal(err)
	}
	if len(hub.Snapshot().Agents) != 1 {
		t.Fatal("retained owner absent from fresh hub")
	}
	if err := h.OwnershipProof(); err == nil {
		t.Fatal("terminal retained entry claimed global drain without a manager")
	}
	stop()
	noReservation(t, cfg)
	if err := h.OwnershipProof(); err == nil {
		t.Fatal("parent join mistaken for independently owned child drain")
	}
}

func retainedHistory(ops ...operation.Operation) sessionstore.LifecycleHistory {
	h := sessionstore.LifecycleHistory{LastRemoteSequence: 1, RetainedRemoteOperations: map[operation.ID]operation.Operation{}}
	for _, op := range ops {
		h.RetainedRemoteOperations[op.ID] = op
	}
	return h
}
func TestHandlerReconcileMissingReservation(t *testing.T) {
	for _, old := range []bool{false, true} {
		t.Run(map[bool]string{false: "ready without handle", true: "committed handle"}[old], func(t *testing.T) {
			cfg, req := handlerConfigFixture(t)
			op := handlerRun(t, req)
			if old {
				op = withHandlerHandle(t, op, savedHandle(t, cfg, req))
			}
			history := retainedHistory(op)
			before := op
			before.State = op.State.Clone()
			err := ReconcileHistory(t.Context(), cfg, req.ParentID, history, []operation.Operation{op})
			if old && !errors.Is(err, ErrUncertain) {
				t.Fatalf("lost committed marker: %v", err)
			}
			if !old && err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(before, op) || !reflect.DeepEqual(before, history.RetainedRemoteOperations[op.ID]) {
				t.Fatal("reconcile mutated canonical intent")
			}
			if history.ValidateResume() == nil {
				t.Fatal("reconcile invented drain receipt")
			}
			noReservation(t, cfg)
		})
	}
}
func TestHandlerReconcileMixedHistoryFailsClosed(t *testing.T) {
	for _, name := range []string{"invalid plan", "legacy remote", "terminal unknown phase"} {
		t.Run(name, func(t *testing.T) {
			cfg, req := handlerConfigFixture(t)
			good := handlerRun(t, req)
			bad := good
			bad.ID = "bad"
			switch name {
			case "invalid plan":
				bad = handlerState(t, bad, func(s *operation.RemoteJobState) { s.Plan.Data = []byte(`{"Version":99}`) })
			case "legacy remote":
				bad = handlerState(t, bad, func(s *operation.RemoteJobState) { s.Plan.Version = toolsubagent.PlanVersion })
			case "terminal unknown phase":
				badReq := req
				badReq.OperationID = bad.ID
				v := savedHandle(t, cfg, badReq)
				v.Phase = "unknown"
				bad = withHandlerHandle(t, bad, v)
				bad.Status = operation.StatusCompleted
			}
			history := retainedHistory(good, bad)
			if err := ReconcileHistory(t.Context(), cfg, req.ParentID, history, []operation.Operation{good, bad}); err == nil {
				t.Fatal("mixed ownership authorized resume")
			}
			noReservation(t, cfg)
		})
	}
}

func TestHandlerInertControlStrictClassification(t *testing.T) {
	for _, status := range []operation.Status{operation.StatusCompleted, operation.StatusFailed, operation.StatusCanceled, operation.StatusReady, operation.StatusAwaiting} {
		for _, action := range []string{"list", "status", "stop", "message", "unknown"} {
			t.Run(string(status)+"/"+action, func(t *testing.T) {
				p := toolsubagent.ControlPlan{Action: action}
				if action != "list" {
					p.Handle = "opaque"
					p.Generation = 1
				}
				if action == "message" {
					p.Message = "hello"
				}
				op := remoteSpec(t, toolsubagent.ControlPlanType, toolsubagent.PlanVersion, p)
				op.Status = status
				want := (status == operation.StatusCompleted || status == operation.StatusFailed || status == operation.StatusCanceled) && (action == "list" || action == "status" || action == "stop")
				if got := InertControl(op); got != want {
					t.Fatalf("classification %v want %v", got, want)
				}
				if want {
					for _, mutation := range []string{"version", "unknown member", "missing authority"} {
						bad := handlerState(t, op, func(s *operation.RemoteJobState) {
							switch mutation {
							case "version":
								s.Plan.Version++
							case "unknown member":
								s.Plan.Data = append(s.Plan.Data[:len(s.Plan.Data)-1], []byte(`,"Unknown":true}`)...)
							case "missing authority":
								s.Plan.Data = []byte(`{"Action":"stop"}`)
							}
						})
						if InertControl(bad) {
							t.Fatalf("accepted %s", mutation)
						}
					}
				}
			})
		}
	}
}
