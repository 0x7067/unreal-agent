package taskgraph

// Exhaustive model check of the taskgraph reducer (graph.go).
//
// The reducer is pure and the runtime (runtime.go Runtime.loop) drives it from a
// single goroutine through exactly the public events enumerated here, so every
// possible runtime interleaving maps to a sequence of these events. This test
// explores all such sequences (BFS over states with memoization) for the seed
// graphs below and asserts the safety promises at every reachable state:
//
//  1. Claim exclusion: no two reserved tasks (Running, Canceling, or Completed
//     awaiting acceptance) have overlapping effective claims in any direction.
//     Overlap is recomputed here independently of graph.go's conflicts(), so a
//     weakened or removed check in Start breaks this property.
//  2. Dependency gating: a Running or Canceling task that is not stale has all
//     dependencies Accepted and not stale.
//  3. Concurrency bound: the number of Running plus Canceling tasks never
//     exceeds Graph.Concurrency.
//  4. Accepted closure: every Accepted task's dependencies are Accepted and not
//     stale (also enforced by Validate; checked independently here).
//  5. Checkpoint validity: every reachable state passes Graph.Validate(), the
//     same check the runtime applies to a restored checkpoint on resume.
//  6. Ready/Start agreement: starting the batch returned by Ready() in order
//     always succeeds, which is what runtimeState.advance assumes.
//
// Bounds (a pass covers these, not all imaginable graphs):
//   - the seed graphs listed in mcShapes below;
//   - Generation <= mcGenerationCap: a successor state in which any task's
//     generation would exceed the cap is not explored (each task is reset at
//     most twice per path), bounding retry/revise/invalidate/cancel loops;
//     note Cancel on a non-active task resets descendants without raising its
//     own generation, so resets are not evenly distributed;
//   - result and evidence text do not affect reducer control flow and are
//     represented by fixed strings; state identity keeps only whether
//     acceptance evidence is nonblank;
//   - Revise is explored through the fixed single-task proposals per shape
//     (upserts of existing task IDs only; adding new tasks is not covered);
//   - events that return an error without changing the state (stale
//     generation, ineligible start) add no successor and are not enumerated.

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	json "encoding/json/v2"
)

const (
	mcGenerationCap   = 3
	mcMaxStatesPerSha = 400_000
)

var mcFail = errors.New("model check failure result")

type mcShape struct {
	name        string
	tasks       []Task
	concurrency int
	// Each proposal is the full declaration of one task, upserted via Revise.
	proposals [][]Task
}

func mcShapes() []mcShape {
	return []mcShape{
		{
			name: "chain-with-shared-reader",
			tasks: []Task{
				{ID: "A", Command: "a", Reads: []string{}, Writes: []string{"shared"}, Acceptance: "acc", EstimatedSeconds: 1},
				{ID: "B", Command: "b", DependsOn: []string{"A"}, Reads: []string{"shared"}, Writes: []string{"b.txt"}, Acceptance: "acc", EstimatedSeconds: 2},
				{ID: "C", Command: "c", Reads: []string{"shared"}, Writes: []string{"c.txt"}, Acceptance: "acc", EstimatedSeconds: 3},
			},
			concurrency: 2,
			proposals: [][]Task{
				{{ID: "C", Command: "c", DependsOn: []string{"A"}, Reads: []string{"shared"}, Writes: []string{"c.txt"}, Acceptance: "acc", EstimatedSeconds: 3}},
				{{ID: "C", Command: "c", Reads: []string{"shared"}, Writes: []string{"c2.txt"}, Acceptance: "acc", EstimatedSeconds: 3}},
			},
		},
		{
			name: "independent-cap",
			tasks: []Task{
				{ID: "X", Command: "x", Reads: []string{}, Writes: []string{"x"}, Acceptance: "acc", EstimatedSeconds: 1},
				{ID: "Y", Command: "y", Reads: []string{}, Writes: []string{"y"}, Acceptance: "acc", EstimatedSeconds: 2},
				{ID: "Z", Command: "z", Reads: []string{}, Writes: []string{"z"}, Acceptance: "acc", EstimatedSeconds: 3},
			},
			concurrency: 2,
			proposals: [][]Task{
				{{ID: "Y", Command: "y", DependsOn: []string{"X"}, Reads: []string{}, Writes: []string{"y"}, Acceptance: "acc", EstimatedSeconds: 2}},
				{{ID: "Y", Command: "y", Reads: []string{}, Writes: []string{"y"}, Acceptance: "acc", EstimatedSeconds: 9}},
			},
		},
		{
			name: "nil-and-empty-claims",
			tasks: []Task{
				{ID: "A", Command: "a", Acceptance: "acc", EstimatedSeconds: 1}, // nil claims cover the workspace
				{ID: "B", Command: "b", Reads: []string{}, Writes: []string{}, Acceptance: "acc", EstimatedSeconds: 2},
			},
			concurrency: 2,
			proposals: [][]Task{
				{{ID: "B", Command: "b", DependsOn: []string{"A"}, Reads: []string{}, Writes: []string{}, Acceptance: "acc", EstimatedSeconds: 2}},
			},
		},
		{
			name: "directory-prefix-claims",
			tasks: []Task{
				{ID: "A", Command: "a", Reads: []string{}, Writes: []string{"dir/f1"}, Acceptance: "acc", EstimatedSeconds: 1},
				{ID: "B", Command: "b", DependsOn: []string{"A"}, Reads: []string{"dir"}, Writes: []string{"dir/f2"}, Acceptance: "acc", EstimatedSeconds: 2},
				{ID: "C", Command: "c", Reads: []string{}, Writes: []string{"dir/f3"}, Acceptance: "acc", EstimatedSeconds: 3},
			},
			concurrency: 2,
			proposals: [][]Task{
				{{ID: "B", Command: "b", DependsOn: []string{"A"}, Reads: []string{"dir"}, Writes: []string{"dir/f2", "extra"}, Acceptance: "acc", EstimatedSeconds: 2}},
			},
		},
	}
}

// mcEffectiveClaims mirrors the documented claim semantics: nil claims
// conservatively cover the whole workspace; explicit empty claims cover
// nothing. It is written independently of claims() in graph.go on purpose.
func mcEffectiveClaims(paths []string) []string {
	if paths == nil {
		return []string{"."}
	}
	return paths
}

func mcOverlaps(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	return a == "." || b == "." || a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}

func mcIntersects(a, b []string) bool {
	for _, x := range mcEffectiveClaims(a) {
		for _, y := range mcEffectiveClaims(b) {
			if mcOverlaps(x, y) {
				return true
			}
		}
	}
	return false
}

func mcConflict(a, b Task) bool {
	return mcIntersects(a.Writes, b.Writes) || mcIntersects(a.Writes, b.Reads) || mcIntersects(a.Reads, b.Writes)
}

func mcReserved(n *Node) bool {
	return n.Status == Running || n.Status == Canceling || n.Status == Completed
}

func mcViolations(g Graph) []string {
	var out []string
	for i := range g.Nodes {
		for j := i + 1; j < len(g.Nodes); j++ {
			a, b := &g.Nodes[i], &g.Nodes[j]
			if mcReserved(a) && mcReserved(b) && mcConflict(a.Task, b.Task) {
				out = append(out, fmt.Sprintf("claim exclusion: reserved tasks %s (%s, writes %v reads %v) and %s (%s, writes %v reads %v) have overlapping claims",
					a.Task.ID, a.Status, a.Task.Writes, a.Task.Reads, b.Task.ID, b.Status, b.Task.Writes, b.Task.Reads))
			}
		}
	}
	active := 0
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if n.Status == Running || n.Status == Canceling {
			active++
		}
		if n.Status == Running || n.Status == Canceling {
			if !n.Stale {
				for _, d := range n.Task.DependsOn {
					p := g.Find(d)
					if p == nil || p.Status != Accepted || p.Stale {
						out = append(out, fmt.Sprintf("dependency gating: active task %s (%s) depends on %s which is not accepted-and-fresh", n.Task.ID, n.Status, d))
					}
				}
			}
		}
		if n.Status == Accepted {
			if strings.TrimSpace(n.Evidence) == "" {
				out = append(out, fmt.Sprintf("accepted evidence: task %s is Accepted without evidence", n.Task.ID))
			}
			for _, d := range n.Task.DependsOn {
				p := g.Find(d)
				if p == nil || p.Status != Accepted || p.Stale {
					out = append(out, fmt.Sprintf("accepted closure: task %s is Accepted but dependency %s is not accepted-and-fresh", n.Task.ID, d))
				}
			}
		}
	}
	if active > g.Concurrency {
		out = append(out, fmt.Sprintf("concurrency bound: %d active tasks exceed limit %d", active, g.Concurrency))
	}
	if err := g.Validate(); err != nil {
		out = append(out, fmt.Sprintf("checkpoint validity: reachable state fails Validate: %v", err))
	}
	if ids := g.Ready(); len(ids) > 0 {
		c := g.clone()
		for _, id := range ids {
			if err := c.Start(id); err != nil {
				out = append(out, fmt.Sprintf("ready/start agreement: Ready() returned %v but Start(%s) failed: %v", ids, id, err))
				break
			}
		}
	}
	return out
}

type mcEntry struct {
	g     Graph
	trace []string
}

func mcTaskKey(t Task) string {
	b, err := json.Marshal(t)
	if err != nil {
		panic(err)
	}
	return string(b)
}

func mcStateKey(g Graph, taskIDs map[string]int) string {
	var sb strings.Builder
	for i := range g.Nodes {
		n := &g.Nodes[i]
		k := mcTaskKey(n.Task)
		id, ok := taskIDs[k]
		if !ok {
			id = len(taskIDs)
			taskIDs[k] = id
		}
		ev := 0
		if strings.TrimSpace(n.Evidence) != "" {
			ev = 1
		}
		fmt.Fprintf(&sb, "%d:%s:%t:%d:%d;", id, n.Status, n.Stale, n.Generation, ev)
	}
	return sb.String()
}

func TestGraphExhaustiveSafety(t *testing.T) {
	for _, shape := range mcShapes() {
		t.Run(shape.name, func(t *testing.T) {
			base, err := NewGraph(shape.name, shape.tasks, shape.concurrency)
			if err != nil {
				t.Fatalf("seed graph must be valid: %v", err)
			}
			taskIDs := map[string]int{}
			visited := map[string]struct{}{}
			key := mcStateKey(base, taskIDs)
			visited[key] = struct{}{}
			queue := []mcEntry{{g: base.clone()}}
			for len(queue) > 0 {
				if len(visited) > mcMaxStatesPerSha {
					t.Fatalf("state cap of %d exceeded for shape %s: tighten mcGenerationCap or the shape", mcMaxStatesPerSha, shape.name)
				}
				cur := queue[0]
				queue = queue[1:]
				if vs := mcViolations(cur.g); len(vs) > 0 {
					t.Fatalf("safety violation in shape %s after %d steps:\n  %s\ntrace:\n%s",
						shape.name, len(cur.trace), strings.Join(vs, "\n  "), mcFormatTrace(cur.trace))
				}
				for _, succ := range mcSuccessors(cur, shape) {
					k := mcStateKey(succ.g, taskIDs)
					if _, seen := visited[k]; seen {
						continue
					}
					visited[k] = struct{}{}
					queue = append(queue, succ)
				}
			}
			t.Logf("shape %s: %d reachable states explored exhaustively", shape.name, len(visited))
		})
	}
}

func mcFormatTrace(trace []string) string {
	var sb strings.Builder
	for i, step := range trace {
		fmt.Fprintf(&sb, "  %d. %s\n", i+1, step)
	}
	return sb.String()
}

func mcSuccessors(cur mcEntry, shape mcShape) []mcEntry {
	var out []mcEntry
	g := cur.g.clone()
	apply := func(desc string, mutate func(h *Graph) error) {
		h := g.clone()
		ph := &h
		_ = mutate(ph) // errors that change no state add nothing; a state change is explored regardless
		if mcStateKeyEqual(*ph, g) {
			return
		}
		for i := range ph.Nodes {
			// Generation bound: an event that pushes any task past the cap ends
			// exploration along that path (Cancel on a non-active task resets its
			// descendants without raising its own generation, so a per-task gate
			// alone cannot bound the space).
			if ph.Nodes[i].Generation > mcGenerationCap {
				return
			}
		}
		trace := make([]string, len(cur.trace), len(cur.trace)+1)
		copy(trace, cur.trace)
		out = append(out, mcEntry{g: *ph, trace: append(trace, desc)})
	}
	for i := range g.Nodes {
		n := &g.Nodes[i]
		id, gen := n.Task.ID, n.Generation
		apply(fmt.Sprintf("Start(%s)", id), func(h *Graph) error { return h.Start(id) })
		apply(fmt.Sprintf("Complete(%s, ok)", id), func(h *Graph) error { return h.Complete(id, gen, "ok", nil) })
		apply(fmt.Sprintf("Complete(%s, err)", id), func(h *Graph) error { return h.Complete(id, gen, "", mcFail) })
		apply(fmt.Sprintf("Accept(%s)", id), func(h *Graph) error { return h.Accept(id, gen, "evidence") })
		apply(fmt.Sprintf("Reject(%s)", id), func(h *Graph) error { return h.Reject(id, gen, "reason") })
		apply(fmt.Sprintf("Retry(%s)", id), func(h *Graph) error { return h.Retry(id) })
		apply(fmt.Sprintf("Cancel(%s)", id), func(h *Graph) error { return h.Cancel(id) })
		apply(fmt.Sprintf("Canceled(%s)", id), func(h *Graph) error { return h.Canceled(id, gen) })
		apply(fmt.Sprintf("Invalidate(%s)", id), func(h *Graph) error { return h.Invalidate(id) })
	}
	for _, proposal := range shape.proposals {
		apply(fmt.Sprintf("Revise(%s)", proposal[0].ID), func(h *Graph) error { return h.Revise(proposal) })
	}
	return out
}

func mcStateKeyEqual(a, b Graph) bool {
	for i := range a.Nodes {
		x, y := &a.Nodes[i], &b.Nodes[i]
		xe, ye := strings.TrimSpace(x.Evidence) != "", strings.TrimSpace(y.Evidence) != ""
		if x.Task.ID != y.Task.ID || mcTaskKey(x.Task) != mcTaskKey(y.Task) ||
			x.Status != y.Status || x.Stale != y.Stale || x.Generation != y.Generation || xe != ye {
			return false
		}
	}
	return len(a.Nodes) == len(b.Nodes)
}
