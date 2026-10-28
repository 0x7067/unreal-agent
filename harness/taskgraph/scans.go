package taskgraph

// All filesystem observations run off the actor. There is at most one scan
// worker per Runtime, including cancellation/drain. A scan owns immutable
// copies; only the actor can apply its result. No reducer statuses are added.
import (
	"context"
	"errors"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/operation"
)

type acceptRequest struct {
	op       operation.Operation
	plan     ControlPlan
	canceled bool
}
type scanJob struct {
	g                 *graphExecution
	id                string
	generation, epoch uint64
	kind              string
	cancel            context.CancelFunc
	startedAt         time.Time
	progress          atomic.Pointer[scanProgress]
}

// Workers publish only immutable progress values. All graph/attempt state stays
// actor-owned; a status snapshot never performs filesystem work or scheduling.
type scanProgress struct {
	phase     string
	startedAt time.Time
}
type scanStatus struct {
	Graph                    string    `json:"graph"`
	TaskID                   string    `json:"task_id"`
	Generation               uint64    `json:"generation"`
	Kind                     string    `json:"kind"`
	Phase                    string    `json:"phase"`
	StartedAt                time.Time `json:"started_at"`
	ElapsedMilliseconds      int64     `json:"elapsed_ms"`
	PhaseStartedAt           time.Time `json:"phase_started_at"`
	PhaseElapsedMilliseconds int64     `json:"phase_elapsed_ms"`
}

func (job *scanJob) setProgress(phase string) {
	job.progress.Store(&scanProgress{phase: phase, startedAt: time.Now().UTC()})
}
func (s *runtimeState) scanStatus() *scanStatus {
	job := s.scanning
	if job == nil {
		return nil
	}
	progress := job.progress.Load()
	now := time.Now().UTC()
	return &scanStatus{
		Graph: job.g.Graph.Name, TaskID: job.id, Generation: job.generation,
		Kind: job.kind, Phase: progress.phase, StartedAt: job.startedAt,
		ElapsedMilliseconds: now.Sub(job.startedAt).Milliseconds(),
		PhaseStartedAt:      progress.startedAt, PhaseElapsedMilliseconds: now.Sub(progress.startedAt).Milliseconds(),
	}
}

type acceptedObservation struct {
	id         string
	generation uint64
	err        error
}
type scanResult struct {
	job            *scanJob
	accepted       []acceptedObservation
	before, output string
	err            error
}

func (a *attempt) setPhase(phase string) { a.Phase = phase; a.PhaseStartedAt = time.Now().UTC() }
func (a *attempt) fail(category string, err error) {
	a.FailureCategory = category
	if err != nil {
		a.FailureReason = err.Error()
	}
	a.FinishedAt = time.Now().UTC()
}

func planWarnings(g Graph) []string {
	warnings := []string{}
	for i, n := range g.Nodes {
		for _, claim := range claims(unionClaims(n.Task.Reads, n.Task.Writes)) {
			if claim == "." {
				warnings = append(warnings, fmt.Sprintf("task %s claims the whole workspace; prefer explicit paths (ignored files are included and symlinks are unsupported)", n.Task.ID))
				break
			}
		}
		for j := 0; j < i; j++ {
			if conflicts(n.Task, g.Nodes[j].Task) {
				warnings = append(warnings, fmt.Sprintf("tasks %s and %s have conflicting read/write claims; execution is serialized and acceptance freshness may invalidate results", g.Nodes[j].Task.ID, n.Task.ID))
			}
		}
	}
	return warnings
}

func (s *runtimeState) startScan(g *graphExecution, n *Node, kind string) {
	if s.runtime.ctx.Err() != nil || s.scanning != nil || g.terminal || g.Canceling {
		return
	}
	ctx, cancel := context.WithCancel(s.runtime.ctx)
	job := &scanJob{g: g, id: n.Task.ID, generation: n.Generation, epoch: g.epoch, kind: kind, cancel: cancel, startedAt: time.Now().UTC()}
	job.setProgress(kind)
	s.scanning = job
	task := n.Task
	// Task slices are immutable; Revise replaces declarations, never edits them.
	type acceptedInput struct {
		id          string
		generation  uint64
		task        Task
		fingerprint string
	}
	inputs := []acceptedInput{}
	for _, p := range g.Graph.Nodes {
		if p.Status == Accepted {
			a := g.Attempts[p.Task.ID]
			fingerprint := ""
			if a != nil && a.Generation == p.Generation {
				fingerprint = a.OutputFingerprint
			}
			inputs = append(inputs, acceptedInput{p.Task.ID, p.Generation, p.Task, fingerprint})
		}
	}
	a := g.Attempts[n.Task.ID]
	expected := a.OutputFingerprint
	workspace := s.runtime.config.Workspace
	hook := s.runtime.config.scanHook
	s.runtime.scans.Add(1)
	go func() {
		defer s.runtime.scans.Done()
		defer cancel()
		result := scanResult{job: job}
		// Always deliver a result, even on cancellation. Ownership is released only
		// after the actor sees this drain notification. The buffered slot lets Wait
		// join after its event loop has exited.
		defer func() { s.runtime.scanResults <- result }()
		if hook != nil {
			if err := hook(ctx, kind); err != nil {
				result.err = err
				return
			}
		}
		if len(inputs) > 0 {
			job.setProgress("accepted_recheck")
			if hook != nil {
				if err := hook(ctx, "accepted_recheck"); err != nil {
					result.err = err
					return
				}
			}
		}
		for _, p := range inputs {
			now, err := fingerprintContext(ctx, workspace, unionClaims(p.task.Reads, p.task.Writes))
			if err == nil && (p.fingerprint == "" || now != p.fingerprint) {
				err = errors.New("accepted result inputs or outputs changed")
			}
			result.accepted = append(result.accepted, acceptedObservation{p.id, p.generation, err})
			if ctx.Err() != nil {
				result.err = ctx.Err()
				return
			}
		}
		job.setProgress(kind)
		switch kind {
		case "input_validation":
			_, result.err = fingerprintContext(ctx, workspace, unionClaims(task.Reads, task.Writes))
			if result.err == nil {
				result.before, result.err = readOnlyFingerprintContext(ctx, workspace, task)
			}
		case "output_validation":
			result.before, result.err = readOnlyFingerprintContext(ctx, workspace, task)
			if result.err == nil {
				result.output, result.err = fingerprintContext(ctx, workspace, unionClaims(task.Reads, task.Writes))
			}
		case "accept":
			result.output, result.err = fingerprintContext(ctx, workspace, unionClaims(task.Reads, task.Writes))
			if result.err == nil && result.output != expected {
				result.err = errors.New("result inputs or outputs changed; inspect current files and rerun the task")
			}
		}
	}()
}

func (s *runtimeState) scanned(result scanResult) {
	job := result.job
	if s.scanning != job {
		return
	}
	s.scanning = nil
	// A successful result may already be queued when shutdown wins. The loop's
	// select does not prioritize Done: drain ownership, but never apply it.
	if s.runtime.ctx.Err() != nil {
		return
	}
	g := job.g
	n := g.Graph.Find(job.id)
	a := g.Attempts[job.id]
	current := !g.terminal && n != nil && n.Generation == job.generation && a != nil && a.Generation == n.Generation
	valid := current && g.epoch == job.epoch && !n.Stale && n.Status != Canceling && !g.Canceling
	if current && n.Status == Canceling && (!a.CommandDispatched || terminal(a.ChildOperation.Status)) {
		g.Graph.Canceled(n.Task.ID, n.Generation)
		a.fail("canceled", context.Canceled)
	} else if current && n.Stale && n.Status == Running && (!a.CommandDispatched || terminal(a.ChildOperation.Status)) {
		// A prerequisite was invalidated while this worker ran. Complete discards
		// stale generations through the reducer's existing reset transition.
		g.Graph.Complete(n.Task.ID, n.Generation, "", nil)
	}
	if !valid {
		if job.kind == "accept" {
			s.finishAccept(g, errors.New("acceptance validation superseded by cancellation or graph revision; inspect status and retry"))
		}
		s.advance(g)
		return
	}
	changed := false
	for _, p := range result.accepted {
		parent := g.Graph.Find(p.id)
		if p.err != nil && parent != nil && parent.Generation == p.generation && parent.Status == Accepted {
			g.Graph.Invalidate(p.id)
			changed = true
		}
	}
	if changed {
		g.epoch++
	}
	// Invalidation can reset the target or make a reserved target stale.
	n = g.Graph.Find(job.id)
	if n.Generation != job.generation || n.Stale {
		if n.Status == Running && n.Stale {
			g.Graph.Complete(n.Task.ID, n.Generation, "", nil)
		}
		if job.kind == "accept" {
			s.finishAccept(g, errors.New("accepted prerequisite changed; previous candidate invalidated"))
		}
		s.advance(g)
		return
	}
	switch job.kind {
	case "input_validation":
		if result.err != nil {
			a.fail("input_validation", result.err)
			g.Graph.Complete(n.Task.ID, n.Generation, "", result.err)
			s.notify(g, n)
		} else {
			a.BeforeReads = result.before
			a.setPhase("execution")
			// Persist the dispatch boundary before handing control to the manager.
			// A crash from here onwards is conservatively an unknown launch.
			a.CommandDispatched = true
			if s.runtime.config.DurableParentID != "" {
				// A queued update is not a store acknowledgment. Reserve the
				// one dispatch slot durably; only NotifyCheckpointCommitted
				// can admit this child. Restart of this reserved cell is unknown.
				g.pendingDispatch[a.ChildOperation.ID] = a.Generation
				s.save(g)
				return
			}
			s.save(g)
			if s.runtime.ctx.Err() != nil {
				return
			}
			if err := g.manager.Add(a.ChildOperation); err != nil {
				a.fail("execution", err)
				g.Graph.Complete(n.Task.ID, n.Generation, "", err)
				s.notify(g, n)
			}
		}
	case "output_validation":
		text, runErr := childResult(a.ChildOperation)
		category := "execution"
		if a.ChildOperation.Type == operation.TypeShell {
			state, err := operation.DecodeShellState(a.ChildOperation)
			if err == nil && state.Result != nil {
				code := state.Result.ExitCode
				a.ExitCode = &code
			}
		}
		if result.err != nil {
			runErr = result.err
			category = "output_validation"
		} else if result.before != a.BeforeReads {
			runErr = errors.New("read-only inputs changed during execution; inspect declared reads and rerun; result is stale")
			category = "input_mutation"
		}
		a.OutputFingerprint = result.output
		a.FinishedAt = time.Now().UTC()
		if runErr != nil {
			a.fail(category, runErr)
		} else {
			a.setPhase("awaiting_acceptance")
		}
		g.Graph.Complete(n.Task.ID, n.Generation, text, runErr)
		s.notify(g, n)
	case "accept":
		err := result.err
		if err != nil {
			a.fail("result_mutation", err)
			g.Graph.Invalidate(n.Task.ID)
			g.epoch++
		} else {
			p := g.pendingAccept.plan
			err = g.Graph.Accept(p.TaskID, p.Generation, p.Evidence)
			if err == nil {
				g.epoch++
			}
		}
		s.finishAccept(g, err)
	case "recheck":
		g.finalFresh = result.err == nil && !changed
	}
	s.advance(g)
}
func (s *runtimeState) finishAccept(g *graphExecution, err error) {
	pending := g.pendingAccept
	if pending == nil {
		return
	}
	g.pendingAccept = nil
	if len(g.acceptQueue) > 0 {
		g.pendingAccept = g.acceptQueue[0]
		g.acceptQueue = g.acceptQueue[1:]
	}
	if pending.canceled {
		s.emit(s.runtime.control, pending.op, operation.StatusCanceled, &g.checkpoint, "acceptance canceled", nil)
	} else if err != nil {
		s.emit(s.runtime.control, pending.op, operation.StatusFailed, &g.checkpoint, "", err)
	} else {
		_ = s.snapshot(pending.op, g)
	}
}

func (s *runtimeState) advance(g *graphExecution) {
	if s.runtime.ctx.Err() != nil || g.terminal {
		return
	}
	if g.Canceling {
		if s.scanning != nil {
			s.save(g)
			return
		}
		for i := range g.Graph.Nodes {
			if active(&g.Graph.Nodes[i]) {
				s.save(g)
				return
			}
		}
		for g.pendingAccept != nil {
			s.finishAccept(g, errors.New("graph canceled during acceptance"))
		}
		g.terminal = true
		s.emit(s.runtime.run, g.owner, operation.StatusCanceled, &g.checkpoint, "", nil)
		for _, op := range g.cancelControls {
			s.emit(s.runtime.control, op, operation.StatusCompleted, &g.checkpoint, "graph canceled after scans and children terminated", nil)
		}
		g.stop()
		if s.active == g {
			s.active = nil
		}
		return
	}
	if s.scanning != nil {
		s.save(g)
		return
	}
	for g.pendingAccept != nil {
		if g.pendingAccept.canceled {
			s.finishAccept(g, context.Canceled)
			continue
		}
		p := g.pendingAccept.plan
		n, err := g.Graph.attempt(p.TaskID, p.Generation)
		if err == nil && n.Status != Completed {
			err = errors.New("task has no current completed result")
		}
		if err != nil {
			s.finishAccept(g, err)
		} else {
			s.startScan(g, n, "accept")
			s.save(g)
			return
		}
	}
	if g.Graph.AllAccepted() {
		if !g.finalFresh {
			s.startScan(g, &g.Graph.Nodes[0], "recheck")
			s.save(g)
			return
		}
		g.terminal = true
		s.emit(s.runtime.run, g.owner, operation.StatusCompleted, &g.checkpoint, "all current task generations accepted", nil)
		g.stop()
		if s.active == g {
			s.active = nil
		}
		return
	}
	// Existing reservations get validation priority over new admissions.
	for i := range g.Graph.Nodes {
		n := &g.Graph.Nodes[i]
		a := g.Attempts[n.Task.ID]
		if n.Status != Running || a == nil || a.Generation != n.Generation || a.Blocker != "" {
			continue
		}
		if n.Stale && (!a.CommandDispatched || terminal(a.ChildOperation.Status)) {
			g.Graph.Complete(n.Task.ID, n.Generation, "", nil)
			continue
		}
		if !a.CommandDispatched && a.Phase == "input_validation" {
			s.startScan(g, n, "input_validation")
			s.save(g)
			return
		}
		if terminal(a.ChildOperation.Status) {
			a.setPhase("output_validation")
			s.startScan(g, n, "output_validation")
			s.save(g)
			return
		}
	}
	for _, id := range g.Graph.Ready() {
		n := g.Graph.Find(id)
		child, err := s.newChild(g, n)
		if err != nil {
			g.Graph.Start(id)
			g.Graph.Complete(id, n.Generation, "", err)
			s.notify(g, n)
			continue
		}
		if err = g.Graph.Start(id); err != nil {
			continue
		}
		a := &attempt{Generation: n.Generation, ChildOperation: child, StartedAt: time.Now().UTC()}
		a.setPhase("input_validation")
		g.Attempts[id] = a
		s.startScan(g, n, "input_validation")
		break
	}
	s.save(g)
}
