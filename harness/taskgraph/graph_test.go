package taskgraph

import (
	"encoding/json/v2"
	"errors"
	"reflect"
	"testing"
)

func TestCheckpointPreservesUnknownClaims(t *testing.T) {
	a := task("a")
	a.Reads, a.Writes = nil, nil
	g := mustGraph(t, a)
	data, err := json.Marshal(g)
	if err != nil {
		t.Fatal(err)
	}
	var restored Graph
	if err := json.Unmarshal(data, &restored); err != nil {
		t.Fatal(err)
	}
	if restored.Nodes[0].Task.Reads != nil || restored.Nodes[0].Task.Writes != nil {
		t.Fatal("unknown claims became independent claims in checkpoint")
	}
}

func task(id string, deps ...string) Task {
	return Task{ID: id, Command: "true", DependsOn: deps, Reads: []string{}, Writes: []string{}, Acceptance: "checked"}
}
func mustGraph(t *testing.T, tasks ...Task) Graph {
	t.Helper()
	g, err := NewGraph("test", tasks, 4)
	if err != nil {
		t.Fatal(err)
	}
	return g
}
func finish(t *testing.T, g *Graph, id string) {
	t.Helper()
	n := g.Find(id)
	if err := g.Complete(id, n.Generation, "ok", nil); err != nil {
		t.Fatal(err)
	}
	if err := g.Accept(id, n.Generation, "checked"); err != nil {
		t.Fatal(err)
	}
}
func TestAcceptanceUnlocksWhileIndependentRuns(t *testing.T) {
	g := mustGraph(t, task("a"), task("b"), task("c", "a"))
	if err := g.Start("a"); err != nil {
		t.Fatal(err)
	}
	g.Start("b")
	gen := g.Find("a").Generation
	g.Complete("a", gen, "ok", nil)
	if err := g.Start("c"); err == nil {
		t.Fatal("completed prerequisite admitted")
	}
	g.Accept("a", gen, "proof")
	if err := g.Start("c"); err != nil {
		t.Fatal(err)
	}
	if g.AllAccepted() {
		t.Fatal("live graph accepted")
	}
}
func TestClaimsAndCapacity(t *testing.T) {
	a, b, c, d := task("a"), task("b"), task("c"), task("d")
	a.Reads = []string{"src"}
	b.Writes = []string{"src/a.go"}
	c.Reads = []string{"src/b.go"}
	d.Writes = []string{"other"}
	g := mustGraph(t, a, b, c, d)
	if got := g.Ready(); !reflect.DeepEqual(got, []string{"a", "c", "d"}) {
		t.Fatal(got)
	}
	g.Start("a")
	if err := g.Start("b"); err == nil {
		t.Fatal("writer overlaps reader")
	}
	g.Start("c")
	g.Start("d")
	g.Concurrency = 3
	if len(g.Ready()) != 0 {
		t.Fatal("capacity exceeded")
	}
}
func TestUnknownClaimsConservative(t *testing.T) {
	a, b := task("a"), task("b")
	a.Writes = nil
	b.Reads = nil
	g := mustGraph(t, a, b)
	g.Start("a")
	if err := g.Start("b"); err == nil {
		t.Fatal("unknown writer overlaps workspace reader")
	}
}
func TestCriticalPath(t *testing.T) {
	a, b, c := task("a"), task("b"), task("c", "a")
	a.EstimatedSeconds = 1
	b.EstimatedSeconds = 4
	c.EstimatedSeconds = 10
	g := mustGraph(t, b, a, c)
	g.Concurrency = 1
	if got := g.Ready(); !reflect.DeepEqual(got, []string{"a"}) {
		t.Fatal(got)
	}
}
func TestRevisionAtomicAndDescendantInvalidation(t *testing.T) {
	g := mustGraph(t, task("a"), task("b", "a"))
	g.Start("a")
	finish(t, &g, "a")
	g.Start("b")
	finish(t, &g, "b")
	before := g.clone()
	bad := task("a", "b")
	if err := g.Revise([]Task{bad}); err == nil {
		t.Fatal("cycle accepted")
	}
	if !reflect.DeepEqual(g, before) {
		t.Fatal("failed revision mutated graph")
	}
	a := task("a")
	a.Command = "changed"
	if err := g.Revise([]Task{a}); err != nil {
		t.Fatal(err)
	}
	if g.Find("b").Status != Pending || g.Find("b").Evidence != "" {
		t.Fatal(g.Find("b"))
	}
	old := g.Find("a").Generation - 1
	if err := g.Accept("a", old, "old"); err == nil {
		t.Fatal("stale generation accepted")
	}
}
func TestLiveInvalidationKeepsLease(t *testing.T) {
	a, b, c := task("a"), task("b", "a"), task("c")
	b.Writes = []string{"output"}
	c.Reads = []string{"output"}
	g := mustGraph(t, a, b, c)
	g.Start("a")
	finish(t, &g, "a")
	g.Start("b")
	gen := g.Find("b").Generation
	g.Invalidate("a")
	if n := g.Find("b"); !n.Stale || n.Status != Running || n.Generation != gen {
		t.Fatal(n)
	}
	if err := g.Start("c"); err == nil {
		t.Fatal("stale live lease dropped")
	}
	if err := g.Complete("b", gen, "stale", nil); err != nil {
		t.Fatal(err)
	}
	if g.Find("b").Status != Pending || g.Find("b").Generation == gen {
		t.Fatal(g.Find("b"))
	}
	if err := g.Accept("b", gen, "stale"); err == nil {
		t.Fatal("accepted stale output")
	}
	if err := g.Start("c"); err != nil {
		t.Fatal(err)
	}
}
func TestReviseActiveAffectedAtomic(t *testing.T) {
	g := mustGraph(t, task("a"), task("b", "a"))
	g.Start("a")
	finish(t, &g, "a")
	g.Start("b")
	before := g.clone()
	a := task("a")
	a.Command = "new"
	if err := g.Revise([]Task{a}); err == nil {
		t.Fatal("superseded active descendant")
	}
	if !reflect.DeepEqual(g, before) {
		t.Fatal("mutation")
	}
}
func TestFailureRejectRetryAndCancel(t *testing.T) {
	g := mustGraph(t, task("a"), task("b", "a"), task("c"))
	g.Start("a")
	gen := g.Find("a").Generation
	g.Complete("a", gen, "", errors.New("bad"))
	if err := g.Start("b"); err == nil {
		t.Fatal("failed dependency admitted")
	}
	if err := g.Start("c"); err != nil {
		t.Fatal(err)
	}
	g.Retry("a")
	if g.Find("a").Generation == gen {
		t.Fatal("generation reused")
	}
	g.Start("a")
	gen = g.Find("a").Generation
	g.Complete("a", gen, "candidate", nil)
	g.Reject("a", gen, "not correct")
	if g.Find("a").Status != Failed {
		t.Fatal("reject")
	}
	g.Retry("a")
	g.Start("a")
	gen = g.Find("a").Generation
	g.Cancel("a")
	if g.Find("a").Status != Canceling {
		t.Fatal("lease lost")
	}
	if err := g.Retry("a"); err == nil {
		t.Fatal("retry live canceled worker")
	}
	g.Canceled("a", gen)
	if err := g.Retry("a"); err != nil {
		t.Fatal(err)
	}
}
func TestValidation(t *testing.T) {
	for _, p := range []string{"/tmp", "../outside", "a/../b", ".git/config", "a/.git/config", "C:/tmp", "a\\b"} {
		a := task("a")
		a.Writes = []string{p}
		if _, err := NewGraph("bad", []Task{a}, 4); err == nil {
			t.Errorf("unsafe path %q", p)
		}
	}
	for _, n := range []int{-1, 17} {
		if _, err := NewGraph("bad", []Task{task("a")}, n); err == nil {
			t.Fatal(n)
		}
	}
	g, err := NewGraph("default", []Task{task("a")}, 0)
	if err != nil || g.Concurrency != 4 {
		t.Fatal(g, err)
	}
	a := task("a")
	a.Prompt = "both"
	if _, err := NewGraph("bad", []Task{a}, 4); err == nil {
		t.Fatal("ambiguous execution")
	}
	if _, err := NewGraph("bad", []Task{task("a", "missing")}, 4); err == nil {
		t.Fatal("missing dependency")
	}
}

func TestCancelAcceptedPrerequisiteInvalidatesEvidence(t *testing.T) {
	g := mustGraph(t, task("a"), task("b", "a"))
	g.Start("a")
	finish(t, &g, "a")
	g.Start("b")
	finish(t, &g, "b")
	if !g.AllAccepted() {
		t.Fatal("accepted graph not complete")
	}
	g.Cancel("a")
	g.Retry("a")
	g.Start("a")
	finish(t, &g, "a")
	if g.AllAccepted() || g.Find("b").Status != Pending {
		t.Fatal("canceled prerequisite retained descendant evidence")
	}
}
func TestIndependentRevisionPreservesLiveAttempt(t *testing.T) {
	g := mustGraph(t, task("a"), task("b"))
	g.Start("a")
	before := *g.Find("a")
	b := task("b")
	b.Command = "changed"
	if err := g.Revise([]Task{b, task("c", "b")}); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(*g.Find("a"), before) {
		t.Fatal("independent active task changed")
	}
}
func TestCancellationRetainsLeaseUntilAcknowledged(t *testing.T) {
	a, b := task("a"), task("b")
	a.Writes = []string{"out"}
	b.Reads = []string{"out"}
	g := mustGraph(t, a, b)
	g.Start("a")
	gen := g.Find("a").Generation
	g.Cancel("a")
	if err := g.Start("b"); err == nil {
		t.Fatal("canceling task released claim")
	}
	if err := g.Canceled("a", gen-1); err == nil {
		t.Fatal("stale cancellation acknowledged")
	}
	if err := g.Canceled("a", gen); err != nil {
		t.Fatal(err)
	}
	if err := g.Start("b"); err != nil {
		t.Fatal(err)
	}
}
func TestSharedReadsAndFileBoundaries(t *testing.T) {
	a, b, c := task("a"), task("b"), task("c")
	a.Reads = []string{"src"}
	b.Reads = []string{"src/a"}
	c.Writes = []string{"src2"}
	g := mustGraph(t, a, b, c)
	if len(g.Ready()) != 3 {
		t.Fatal(g.Ready())
	}
	for _, id := range []string{"a", "b", "c"} {
		if err := g.Start(id); err != nil {
			t.Fatal(err)
		}
	}
}

func TestCheckpointValidation(t *testing.T) {
	g := mustGraph(t, task("a"))
	if err := g.Validate(); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Graph){func(g *Graph) { g.Version = 2 }, func(g *Graph) { g.Concurrency = 17 }, func(g *Graph) { g.Nodes[0].Generation = 0 }, func(g *Graph) { g.Nodes[0].Status = "unknown" }, func(g *Graph) { g.Nodes[0].Stale = true }, func(g *Graph) { g.Nodes[0].Task.DependsOn = []string{"a"} }, func(g *Graph) { g.Nodes[0].Task.Reads = []string{"a/./b"} }, func(g *Graph) { g.Nodes[0].Status = Accepted }} {
		bad := g.clone()
		mutate(&bad)
		before := bad.clone()
		if err := bad.Validate(); err == nil {
			t.Fatal("bad checkpoint accepted", bad)
		}
		if !reflect.DeepEqual(bad, before) {
			t.Fatal("validation mutated checkpoint")
		}
	}
	if _, err := NewGraph("", []Task{task("a")}, 4); err == nil {
		t.Fatal("empty name")
	}
	if _, err := NewGraph("empty", nil, 4); err == nil {
		t.Fatal("empty tasks")
	}
}

func TestCaseInsensitiveClaimAliasesConflict(t *testing.T) {
	a, b := task("a"), task("b")
	a.Writes = []string{"Src"}
	b.Writes = []string{"src/main.go"}
	g := mustGraph(t, a, b)
	g.Start("a")
	if err := g.Start("b"); err == nil {
		t.Fatal("case alias writer admitted")
	}
}
