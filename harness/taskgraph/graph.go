// Package taskgraph implements a pure reducer for accepted task DAGs.
package taskgraph

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"math"
	"path"
	"reflect"
	"slices"
	"sort"
	"strings"
)

const (
	Pending   = "pending"
	Running   = "running"
	Completed = "completed"
	Accepted  = "accepted"
	Failed    = "failed"
	Canceling = "canceling"
	Canceled  = "canceled"
)

type Task struct {
	ID          string   `json:"id"`
	Description string   `json:"description,omitempty"`
	Command     string   `json:"command,omitempty"`
	Prompt      string   `json:"prompt,omitempty"`
	DependsOn   []string `json:"depends_on,omitempty"`
	// Nil claims conservatively cover the workspace; explicit empty claims do not.
	Reads            []string `json:"reads"`
	Writes           []string `json:"writes"`
	Acceptance       string   `json:"acceptance"`
	EstimatedSeconds float64  `json:"estimated_seconds,omitempty"`
}

// MarshalJSON keeps unknown claims distinct from explicitly empty claims.
// That distinction controls both reservations and evidence freshness on resume.
func (t Task) MarshalJSON() ([]byte, error) {
	type encodedTask Task
	return json.Marshal(encodedTask(t), json.FormatNilSliceAsNull(true))
}

type Node struct {
	Task       Task   `json:"task"`
	Generation uint64 `json:"generation"`
	Status     string `json:"status"`
	Result     string `json:"result,omitempty"`
	Evidence   string `json:"evidence,omitempty"`
	Stale      bool   `json:"stale,omitempty"`
}
type Graph struct {
	Version     int    `json:"version"`
	Name        string `json:"name"`
	Concurrency int    `json:"concurrency"`
	Nodes       []Node `json:"nodes"`
}

func NewGraph(name string, tasks []Task, concurrency int) (Graph, error) {
	if strings.TrimSpace(name) == "" || len(tasks) == 0 {
		return Graph{}, errors.New("graph requires a name and at least one task")
	}
	if concurrency == 0 {
		concurrency = 4
	}
	if concurrency < 1 || concurrency > 16 {
		return Graph{}, errors.New("concurrency must be between 1 and 16")
	}
	g := Graph{Version: 1, Name: name, Concurrency: concurrency}
	for _, task := range tasks {
		g.Nodes = append(g.Nodes, Node{Task: task, Generation: 1, Status: Pending})
	}
	if err := g.validate(); err != nil {
		return Graph{}, err
	}
	return g, nil
}

// Validate checks a restored checkpoint without changing it. Claims must already
// be canonical so admission can safely compare the persisted path strings.
func (g Graph) Validate() error {
	if g.Version != 1 || strings.TrimSpace(g.Name) == "" || len(g.Nodes) == 0 || g.Concurrency < 1 || g.Concurrency > 16 {
		return errors.New("invalid graph version, name, tasks, or concurrency")
	}
	next := g.clone()
	if err := next.validate(); err != nil {
		return err
	}
	activeCount := 0
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if !reflect.DeepEqual(n.Task, next.Nodes[i].Task) {
			return fmt.Errorf("task %s has noncanonical claims", n.Task.ID)
		}
		if n.Generation == 0 || (n.Stale && !active(n)) {
			return fmt.Errorf("task %s has invalid generation or stale state", n.Task.ID)
		}
		switch n.Status {
		case Pending, Running, Completed, Accepted, Failed, Canceling, Canceled:
		default:
			return fmt.Errorf("task %s has invalid status %q", n.Task.ID, n.Status)
		}
		if n.Status == Accepted {
			if strings.TrimSpace(n.Evidence) == "" {
				return fmt.Errorf("task %s lacks acceptance evidence", n.Task.ID)
			}
		}
		// Invalidation leaves active descendants stale until their children
		// terminate. Only those stale attempts may outlive accepted prerequisites.
		if n.Status == Accepted || n.Status == Completed || (active(n) && !n.Stale) {
			for _, id := range n.Task.DependsOn {
				p := next.Find(id)
				if p.Status != Accepted || p.Stale {
					return fmt.Errorf("%s task %s has unaccepted or stale prerequisite %s", n.Status, n.Task.ID, id)
				}
			}
		}
		if active(n) {
			activeCount++
			if activeCount > g.Concurrency {
				return errors.New("active tasks exceed graph concurrency")
			}
		}
		// Completed candidates and stale active attempts still own their claims.
		if reserved(n) {
			for j := 0; j < i; j++ {
				other := &g.Nodes[j]
				if reserved(other) && conflicts(n.Task, other.Task) {
					return fmt.Errorf("reserved tasks %s and %s have conflicting claims", other.Task.ID, n.Task.ID)
				}
			}
		}
	}
	return nil
}
func (g *Graph) Find(id string) *Node {
	for i := range g.Nodes {
		if g.Nodes[i].Task.ID == id {
			return &g.Nodes[i]
		}
	}
	return nil
}
func (g Graph) clone() Graph {
	c := g
	c.Nodes = append([]Node(nil), g.Nodes...)
	for i := range c.Nodes {
		t := &c.Nodes[i].Task
		t.DependsOn = slices.Clone(t.DependsOn)
		t.Reads = slices.Clone(t.Reads)
		t.Writes = slices.Clone(t.Writes)
	}
	return c
}
func normalizeClaims(paths []string) ([]string, error) {
	if paths == nil {
		return nil, nil
	}
	out := make([]string, 0, len(paths))
	seen := map[string]bool{}
	for _, p := range paths {
		if p == "" || strings.HasPrefix(p, "/") || strings.ContainsAny(p, "\\:\x00") {
			return nil, fmt.Errorf("unsafe workspace path %q", p)
		}
		for _, part := range strings.Split(p, "/") {
			if part == ".." || strings.EqualFold(part, ".git") {
				return nil, fmt.Errorf("unsafe workspace path %q", p)
			}
		}
		p = path.Clean(p)
		if !seen[p] {
			seen[p] = true
			out = append(out, p)
		}
	}
	sort.Strings(out)
	return out, nil
}
func (g *Graph) validate() error {
	ids := map[string]bool{}
	for i := range g.Nodes {
		t := &g.Nodes[i].Task
		t.DependsOn = slices.Clone(t.DependsOn)
		if strings.TrimSpace(t.ID) == "" || ids[t.ID] {
			return fmt.Errorf("duplicate or empty task ID %q", t.ID)
		}
		ids[t.ID] = true
		if (strings.TrimSpace(t.Command) == "") == (strings.TrimSpace(t.Prompt) == "") {
			return fmt.Errorf("task %s requires exactly one command or prompt", t.ID)
		}
		if strings.TrimSpace(t.Acceptance) == "" {
			return fmt.Errorf("task %s requires acceptance conditions", t.ID)
		}
		if t.EstimatedSeconds < 0 || math.IsNaN(t.EstimatedSeconds) || math.IsInf(t.EstimatedSeconds, 0) {
			return fmt.Errorf("task %s has invalid duration", t.ID)
		}
		var err error
		t.Reads, err = normalizeClaims(t.Reads)
		if err != nil {
			return err
		}
		t.Writes, err = normalizeClaims(t.Writes)
		if err != nil {
			return err
		}
	}
	for _, n := range g.Nodes {
		seen := map[string]bool{}
		for _, d := range n.Task.DependsOn {
			if !ids[d] || seen[d] {
				return fmt.Errorf("task %s has missing or duplicate dependency %q", n.Task.ID, d)
			}
			seen[d] = true
		}
	}
	color := map[string]int{}
	var visit func(string) error
	visit = func(id string) error {
		if color[id] == 1 {
			return fmt.Errorf("dependency cycle at %s", id)
		}
		if color[id] == 2 {
			return nil
		}
		color[id] = 1
		for _, d := range g.Find(id).Task.DependsOn {
			if err := visit(d); err != nil {
				return err
			}
		}
		color[id] = 2
		return nil
	}
	for id := range ids {
		if err := visit(id); err != nil {
			return err
		}
	}
	// Acceptance binds evidence to a task's claimed paths. A task writing into an
	// ancestor's claims, or unrelated tasks writing into each other's claims,
	// would invalidate accepted evidence and rerun indefinitely.
	descendants := map[string]map[string]bool{}
	for _, n := range g.Nodes {
		descendants[n.Task.ID] = g.affected(map[string]bool{n.Task.ID: true})
	}
	for _, a := range g.Nodes {
		for _, b := range g.Nodes {
			if a.Task.ID == b.Task.ID {
				continue
			}
			if descendants[a.Task.ID][b.Task.ID] && writesInto(b.Task, a.Task) {
				return fmt.Errorf("task %s writes paths claimed by its prerequisite %s; declare disjoint claims or merge the tasks", b.Task.ID, a.Task.ID)
			}
			if a.Task.ID < b.Task.ID && !descendants[a.Task.ID][b.Task.ID] && !descendants[b.Task.ID][a.Task.ID] &&
				writesInto(a.Task, b.Task) && writesInto(b.Task, a.Task) {
				return fmt.Errorf("tasks %s and %s write paths claimed by each other; declare disjoint claims or a dependency", a.Task.ID, b.Task.ID)
			}
		}
	}
	return nil
}
func writesInto(writer, owner Task) bool {
	return intersects(writer.Writes, owner.Reads) || intersects(writer.Writes, owner.Writes)
}
func active(n *Node) bool { return n.Status == Running || n.Status == Canceling }

// Completed candidates keep their claims until accepted or rejected so a
// conflicting task cannot change them before the parent inspects them.
func reserved(n *Node) bool { return active(n) || n.Status == Completed }
func claims(p []string) []string {
	if p == nil {
		return []string{"."}
	}
	return p
}
func overlaps(a, b string) bool {
	// Portable claims conservatively reserve aliases on case-insensitive volumes.
	a, b = strings.ToLower(a), strings.ToLower(b)
	return a == "." || b == "." || a == b || strings.HasPrefix(a, b+"/") || strings.HasPrefix(b, a+"/")
}
func intersects(a, b []string) bool {
	for _, x := range claims(a) {
		for _, y := range claims(b) {
			if overlaps(x, y) {
				return true
			}
		}
	}
	return false
}
func conflicts(a, b Task) bool {
	return intersects(a.Writes, b.Writes) || intersects(a.Writes, b.Reads) || intersects(a.Reads, b.Writes)
}
func (g *Graph) eligible(n *Node) bool {
	if n.Status != Pending || n.Stale {
		return false
	}
	for _, d := range n.Task.DependsOn {
		p := g.Find(d)
		if p == nil || p.Status != Accepted || p.Stale {
			return false
		}
	}
	return true
}

func (g *Graph) WaitReason(id string) string {
	n := g.Find(id)
	if n == nil || n.Status == Accepted {
		return ""
	}
	if n.Status != Pending {
		return n.Status
	}
	var unmet []string
	for _, dep := range n.Task.DependsOn {
		p := g.Find(dep)
		if p != nil && p.Status == Accepted {
			continue
		}
		status := "missing"
		if p != nil {
			status = p.Status
		}
		unmet = append(unmet, status+" task "+dep)
	}
	if len(unmet) > 0 {
		return "waiting on " + strings.Join(unmet, ", ")
	}
	var fileConflicts []string
	for i := range g.Nodes {
		other := &g.Nodes[i]
		if reserved(other) && conflicts(n.Task, other.Task) {
			fileConflicts = append(fileConflicts, other.Status+" task "+other.Task.ID)
		}
	}
	if len(fileConflicts) > 0 {
		return "conflicts with " + strings.Join(fileConflicts, ", ")
	}
	if g.eligible(n) {
		activeCount := 0
		for i := range g.Nodes {
			if active(&g.Nodes[i]) {
				activeCount++
			}
		}
		if activeCount >= g.Concurrency {
			return "concurrency full"
		}
	}
	return Pending
}

// Ready returns an admissible batch in descending remaining critical-path order.
// Start rechecks eligibility against live reservations, independently of this batch.
func (g *Graph) Ready() []string {
	var held []Task
	var ready []*Node
	running := 0
	for i := range g.Nodes {
		n := &g.Nodes[i]
		if active(n) {
			running++
		}
		if reserved(n) {
			held = append(held, n.Task)
		}
		if g.eligible(n) {
			ready = append(ready, n)
		}
	}
	memo := map[string]float64{}
	var score func(string) float64
	score = func(id string) float64 {
		if v, ok := memo[id]; ok {
			return v
		}
		n := g.Find(id)
		dur := n.Task.EstimatedSeconds
		if dur == 0 {
			dur = 1
		}
		tail := 0.0
		for _, other := range g.Nodes {
			if other.Status == Accepted {
				continue
			}
			for _, d := range other.Task.DependsOn {
				if d == id {
					tail = math.Max(tail, score(other.Task.ID))
				}
			}
		}
		memo[id] = dur + tail
		return dur + tail
	}
	sort.SliceStable(ready, func(i, j int) bool { return score(ready[i].Task.ID) > score(ready[j].Task.ID) })
	out := []string{}
	for _, n := range ready {
		if running >= g.Concurrency {
			break
		}
		safe := true
		for _, t := range held {
			if conflicts(n.Task, t) {
				safe = false
				break
			}
		}
		if safe {
			out = append(out, n.Task.ID)
			held = append(held, n.Task)
			running++
		}
	}
	return out
}
func (g *Graph) Start(id string) error {
	n := g.Find(id)
	if n == nil || !g.eligible(n) {
		return fmt.Errorf("task %s is not ready", id)
	}
	count := 0
	for i := range g.Nodes {
		other := &g.Nodes[i]
		if active(other) {
			count++
		}
		if reserved(other) && conflicts(n.Task, other.Task) {
			return fmt.Errorf("task %s conflicts with %s task %s", id, other.Status, other.Task.ID)
		}
	}
	if count >= g.Concurrency {
		return errors.New("graph concurrency is full")
	}
	n.Status = Running
	return nil
}
func (g *Graph) attempt(id string, generation uint64) (*Node, error) {
	n := g.Find(id)
	if n == nil {
		return nil, fmt.Errorf("unknown task %s", id)
	}
	if n.Generation != generation {
		return nil, fmt.Errorf("task %s has stale generation %d; current is %d", id, generation, n.Generation)
	}
	return n, nil
}
func reset(n *Node) {
	n.Generation++
	n.Status = Pending
	n.Result = ""
	n.Evidence = ""
	n.Stale = false
}
func (g *Graph) Complete(id string, generation uint64, result string, runErr error) error {
	n, err := g.attempt(id, generation)
	if err != nil {
		return err
	}
	if n.Status != Running {
		return fmt.Errorf("task %s is not running", id)
	}
	if n.Stale {
		reset(n)
		return nil
	}
	n.Result = result
	if runErr != nil {
		n.Status = Failed
		n.Evidence = runErr.Error()
	} else {
		n.Status = Completed
	}
	return nil
}
func (g *Graph) Accept(id string, generation uint64, evidence string) error {
	n, err := g.attempt(id, generation)
	if err != nil {
		return err
	}
	if n.Status != Completed || n.Stale {
		return fmt.Errorf("task %s has no current completed result", id)
	}
	if strings.TrimSpace(evidence) == "" {
		return errors.New("acceptance evidence is required")
	}
	for _, d := range n.Task.DependsOn {
		p := g.Find(d)
		if p.Status != Accepted || p.Stale {
			return fmt.Errorf("task %s prerequisite %s is not accepted", id, d)
		}
	}
	n.Status = Accepted
	n.Evidence = evidence
	return nil
}
func (g *Graph) Reject(id string, generation uint64, reason string) error {
	n, err := g.attempt(id, generation)
	if err != nil {
		return err
	}
	if n.Status != Completed {
		return fmt.Errorf("task %s is not completed", id)
	}
	if strings.TrimSpace(reason) == "" {
		return errors.New("rejection reason is required")
	}
	n.Status = Failed
	n.Evidence = reason
	return nil
}
func (g *Graph) affected(roots map[string]bool) map[string]bool {
	out := map[string]bool{}
	for id := range roots {
		out[id] = true
	}
	for changed := true; changed; {
		changed = false
		for _, n := range g.Nodes {
			if out[n.Task.ID] {
				continue
			}
			for _, d := range n.Task.DependsOn {
				if out[d] {
					out[n.Task.ID] = true
					changed = true
					break
				}
			}
		}
	}
	return out
}

// Revise atomically upserts declarations and invalidates changed tasks and descendants.
func (g *Graph) Revise(tasks []Task) error {
	next := g.clone()
	seen := map[string]bool{}
	for _, t := range tasks {
		if seen[t.ID] {
			return fmt.Errorf("duplicate revision task %s", t.ID)
		}
		seen[t.ID] = true
		if n := next.Find(t.ID); n != nil {
			n.Task = t
		} else {
			next.Nodes = append(next.Nodes, Node{Task: t, Status: Pending, Generation: 1})
		}
	}
	if err := next.validate(); err != nil {
		return err
	}
	roots := map[string]bool{}
	for _, n := range next.Nodes {
		old := g.Find(n.Task.ID)
		if old == nil || !reflect.DeepEqual(n.Task, old.Task) {
			roots[n.Task.ID] = true
		}
	}
	affected := next.affected(roots)
	for id := range g.affected(roots) {
		affected[id] = true
	}
	for id := range affected {
		if old := g.Find(id); old != nil && active(old) {
			return fmt.Errorf("revision affects active task %s; wait for termination", id)
		}
	}
	for i := range next.Nodes {
		n := &next.Nodes[i]
		if affected[n.Task.ID] && g.Find(n.Task.ID) != nil {
			reset(n)
		}
	}
	*g = next
	return nil
}
func (g *Graph) Invalidate(id string) error {
	if g.Find(id) == nil {
		return fmt.Errorf("unknown task %s", id)
	}
	for affected := range g.affected(map[string]bool{id: true}) {
		n := g.Find(affected)
		if active(n) {
			n.Stale = true
			n.Evidence = ""
		} else {
			reset(n)
		}
	}
	return nil
}
func (g *Graph) Retry(id string) error {
	n := g.Find(id)
	if n == nil {
		return fmt.Errorf("unknown task %s", id)
	}
	if n.Status != Failed && n.Status != Canceled {
		return fmt.Errorf("task %s must be failed or canceled to retry", id)
	}
	reset(n)
	return nil
}
func (g *Graph) Cancel(id string) error {
	n := g.Find(id)
	if n == nil {
		return fmt.Errorf("unknown task %s", id)
	}
	// A canceled prerequisite cannot continue to justify descendant evidence.
	for descendant := range g.affected(map[string]bool{id: true}) {
		if descendant == id {
			continue
		}
		child := g.Find(descendant)
		if active(child) {
			child.Stale = true
			child.Evidence = ""
		} else {
			reset(child)
		}
	}
	if active(n) {
		n.Status = Canceling
	} else {
		n.Status = Canceled
		n.Evidence = ""
	}
	return nil
}
func (g *Graph) Canceled(id string, generation uint64) error {
	n, err := g.attempt(id, generation)
	if err != nil {
		return err
	}
	if n.Status != Canceling {
		return fmt.Errorf("task %s is not canceling", id)
	}
	n.Status = Canceled
	n.Stale = false
	n.Evidence = ""
	return nil
}
func (g *Graph) AllAccepted() bool {
	if len(g.Nodes) == 0 {
		return false
	}
	for _, n := range g.Nodes {
		if n.Status != Accepted || n.Stale {
			return false
		}
	}
	return true
}
