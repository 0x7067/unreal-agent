package durablejob

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/jobexecutor"
	"github.com/unreallabsai/unreal-agent/harness/operation"
)

const token = "adapter-test-token-at-least-24-characters"

func private(t *testing.T) string {
	t.Helper()
	d, e := os.MkdirTemp("/tmp", "dj-")
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { os.RemoveAll(d) })
	return d
}
func client(t *testing.T, root string) *jobexecutor.Client {
	t.Helper()
	return jobexecutor.NewClient(filepath.Join(root, "executor.sock"), token)
}
func adapter(t *testing.T, root string) *Handler {
	t.Helper()
	h, e := New(context.Background(), Config{Client: client(t, root), EnvironmentID: "test", ControllerID: "owner", Jobs: []string{"effect"}, PollInterval: 10 * time.Millisecond, RequestTimeout: time.Second, MaxFailures: 2})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { h.Close() })
	return h
}
func server(t *testing.T, root, script string, drop func(jobexecutor.Handle) bool) *jobexecutor.Service {
	t.Helper()
	s, e := jobexecutor.Start(jobexecutor.Config{Directory: root, EnvironmentID: "test", Token: token, Jobs: map[string]jobexecutor.TrustedJob{"effect": {Path: "/bin/sh", Args: []string{"-c", script}, Dir: filepath.Dir(root), Env: []string{"PATH=/usr/bin:/bin"}}}, DropSubmitResponse: drop})
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(func() { s.Close() })
	return s
}
func op(t *testing.T, id string, limit int) operation.Operation {
	t.Helper()
	p := Plan{Version: Version, EnvironmentID: "test", ControllerID: "owner", JobName: "effect"}
	raw, e := json.Marshal(p)
	if e != nil {
		t.Fatal(e)
	}
	s := operation.RemoteJobState{Plan: operation.RemoteJobPlan{Type: PlanType, Version: Version, Data: raw}}
	b, e := encodeState(s)
	if e != nil {
		t.Fatal(e)
	}
	return operation.Operation{ID: operation.ID(id), Type: operation.TypeRemoteJob, Version: operation.VersionRemoteJob, Status: operation.StatusReady, MaxOutputLength: limit, State: b}
}
func state(t *testing.T, o operation.Operation) (operation.RemoteJobState, Checkpoint) {
	t.Helper()
	s, e := operation.DecodeRemoteJobState(o)
	if e != nil {
		t.Fatal(e)
	}
	var cp Checkpoint
	if e = json.Unmarshal(s.Handle, &cp); e != nil {
		t.Fatal(e)
	}
	return s, cp
}
func next(t *testing.T, h *Handler, pred func(operation.Operation) bool) operation.Operation {
	t.Helper()
	deadline := time.NewTimer(5 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case o, ok := <-h.RemoteJobUpdates():
			if !ok {
				t.Fatal("adapter closed")
			}
			if pred(o) {
				return o
			}
		case <-deadline.C:
			t.Fatal("update deadline")
		}
	}
}
func done(o operation.Operation) bool {
	return o.Status == operation.StatusCompleted || o.Status == operation.StatusCanceled || o.Status == operation.StatusFailed
}

func TestLostSubmitRestartSameExecutionAndClose(t *testing.T) {
	base := private(t)
	root := filepath.Join(base, "executor")
	effect := filepath.Join(base, "effect")
	release := filepath.Join(base, "release")
	var drops atomic.Int32
	server(t, root, "echo one >> "+effect+"; printf first; while [ ! -f "+release+" ]; do sleep .02; done; printf second", func(jobexecutor.Handle) bool { return drops.Add(1) <= 2 })
	h := adapter(t, root)
	initial := op(t, "lost", 64)
	if e := h.AddRemoteJob(initial); e != nil {
		t.Fatal(e)
	}
	lost := next(t, h, func(o operation.Operation) bool {
		s, _ := state(t, o)
		return strings.Contains(s.TerminalError, "unknown")
	})
	_, saved := state(t, lost)
	if done(lost) {
		t.Fatal("lost acknowledgment falsely terminal")
	}
	if e := h.Close(); e != nil {
		t.Fatal(e)
	}
	c := client(t, root)
	defer c.Close()
	snap, e := c.Inspect(context.Background(), saved.Handle, 0)
	if e != nil || snap.State != "running" {
		t.Fatalf("Close canceled independent job: %+v %v", snap, e)
	}
	h2 := adapter(t, root)
	if e = h2.AddRemoteJob(lost); e != nil {
		t.Fatal(e)
	}
	partial := next(t, h2, func(o operation.Operation) bool { _, cp := state(t, o); return string(cp.Output) == "first" })
	_, cp1 := state(t, partial)
	if e = os.WriteFile(release, []byte("go"), 0600); e != nil {
		t.Fatal(e)
	}
	terminal := next(t, h2, done)
	_, cp2 := state(t, terminal)
	if terminal.Status != operation.StatusCompleted || string(cp2.Output) != "firstsecond" || cp2.Offset <= cp1.Offset || cp1.Handle != cp2.Handle {
		t.Fatalf("bad reconciliation: %+v %+v", cp1, cp2)
	}
	b, e := os.ReadFile(effect)
	if e != nil || string(b) != "one\n" {
		t.Fatalf("execution count %q %v", b, e)
	}
}
func TestCancelJoinsAndStaleAuthority(t *testing.T) {
	base := private(t)
	root := filepath.Join(base, "executor")
	server(t, root, "printf ready; exec sleep 30", nil)
	h := adapter(t, root)
	o := op(t, "cancel", 64)
	if e := h.AddRemoteJob(o); e != nil {
		t.Fatal(e)
	}
	running := next(t, h, func(o operation.Operation) bool { _, cp := state(t, o); return string(cp.Output) == "ready" })
	_, cp := state(t, running)
	c := client(t, root)
	defer c.Close()
	stale := cp.Handle
	stale.AttemptID = "wrong"
	if _, e := c.Cancel(context.Background(), stale); e == nil {
		t.Fatal("stale cancel accepted")
	}
	if e := h.CancelRemoteJob(o.ID, "test"); e != nil {
		t.Fatal(e)
	}
	terminal := next(t, h, done)
	if terminal.Status != operation.StatusCanceled {
		t.Fatalf("status %s", terminal.Status)
	}
	snap, e := c.Inspect(context.Background(), cp.Handle, 0)
	if e != nil || snap.State != "canceled" || snap.ExitCode != -1 {
		t.Fatalf("missing executor Wait cancellation ack: %+v %v", snap, e)
	}
	if e = h.CancelRemoteJob(o.ID, "again"); e == nil {
		t.Fatal("retired worker accepted cancel")
	}
}
func TestStrictCheckpointAndPlanBytes(t *testing.T) {
	root := filepath.Join(private(t), "absent")
	h := adapter(t, root)
	original := op(t, "strict", 64)
	s, _ := operation.DecodeRemoteJobState(original)
	s.Plan.Data = jsontext.Value("{ \"Version\":1, \"EnvironmentID\":\"test\", \"ControllerID\":\"owner\", \"JobName\":\"effect\" }")
	b, e := encodeState(s)
	if e != nil {
		t.Fatal(e)
	}
	original.State = b
	if e = h.AddRemoteJob(original); e != nil {
		t.Fatal(e)
	}
	updated := next(t, h, func(operation.Operation) bool { return true })
	us, cp := state(t, updated)
	if !bytes.Equal(s.Plan.Data, us.Plan.Data) {
		t.Fatalf("plan bytes changed: %q -> %q", s.Plan.Data, us.Plan.Data)
	}
	malformed := []jsontext.Value{jsontext.Value(`{}`), jsontext.Value(`{"Version":1}`), jsontext.Value(`{"Version":2}`), jsontext.Value(`{"Version":1,"Unexpected":true}`), jsontext.Value(`null`)}
	good, _ := json.Marshal(cp)
	var fields map[string]jsontext.Value
	json.Unmarshal(good, &fields)
	delete(fields, "Version")
	missing, _ := json.Marshal(fields)
	malformed = append(malformed, missing)
	for i, raw := range malformed {
		candidate := original
		s.Handle = raw
		candidate.State, _ = encodeState(s)
		candidate.ID = operation.ID("bad" + strings.Repeat("x", i))
		if e = h.AddRemoteJob(candidate); e == nil {
			t.Fatalf("accepted checkpoint %s", raw)
		}
	}
	for _, raw := range []string{`{"Version":2,"EnvironmentID":"test","ControllerID":"owner","JobName":"effect"}`, `{"Version":1,"EnvironmentID":"other","ControllerID":"owner","JobName":"effect"}`, `{"Version":1,"EnvironmentID":"test","ControllerID":"owner","JobName":"effect","Path":"/bin/sh"}`} {
		candidate := op(t, "badplan", 64)
		ps, _ := operation.DecodeRemoteJobState(candidate)
		ps.Plan.Data = jsontext.Value(raw)
		candidate.State, _ = encodeState(ps)
		if e = h.AddRemoteJob(candidate); e == nil {
			t.Fatal("accepted invalid plan")
		}
	}
}
func TestUnavailableBoundedNoFalseCompletion(t *testing.T) {
	h := adapter(t, filepath.Join(private(t), "absent"))
	o := op(t, "unavailable", 64)
	if e := h.AddRemoteJob(o); e != nil {
		t.Fatal(e)
	}
	last := next(t, h, func(o operation.Operation) bool { _, cp := state(t, o); return cp.Failures == 2 })
	if done(last) {
		t.Fatal("unavailable falsely terminal")
	}
	if e := h.CancelRemoteJob(o.ID, "parked"); e == nil {
		t.Fatal("exhausted worker accepted cancel")
	}
	select {
	case <-h.RemoteJobUpdates():
		t.Fatal("retry policy unbounded")
	case <-time.After(50 * time.Millisecond):
	}
}
func TestOutputLimitPreservesRemoteOffset(t *testing.T) {
	root := filepath.Join(private(t), "executor")
	server(t, root, "printf 123456789; sleep .05; printf abcdef", nil)
	h := adapter(t, root)
	o := op(t, "limit", 4)
	if e := h.AddRemoteJob(o); e != nil {
		t.Fatal(e)
	}
	terminal := next(t, h, done)
	s, cp := state(t, terminal)
	if terminal.Status != operation.StatusCompleted || string(cp.Output) != "1234" || cp.Offset != 15 || !cp.Truncated || s.ResultBytes != 4 {
		t.Fatalf("output bounds failed: %+v %+v", s, cp)
	}
}
