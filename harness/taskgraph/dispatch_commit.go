package taskgraph

import (
	"reflect"

	"github.com/unreallabsai/unreal-agent/harness/operation"
)

// NotifyCheckpointCommitted is a command-layer acknowledgment AFTER the exact
// operation checkpoint was durably saved. It is not a generic manager interface,
// an execution receipt, or permission to replay a reservation after restart.
// Admission here is volatile; the runtime actor alone can perform child dispatch.
func (r *Runtime) NotifyCheckpointCommitted(op operation.Operation) error {
	if r.config.DurableParentID == "" {
		return nil
	}
	op.State = op.State.Clone()
	return r.request(func(s *runtimeState) { s.checkpointCommitted(op) })
}
func (s *runtimeState) checkpointCommitted(op operation.Operation) {
	if s.runtime.ctx.Err() != nil {
		return
	}
	p, cp, err := DecodeDurableRun(op)
	if err != nil || cp == nil || p.ParentID != s.runtime.config.DurableParentID || cp.Scope != s.runtime.config.DurableScope {
		return
	}
	g := s.graphs[p.Run.Name]
	if g == nil || g.terminal || g.owner.ID != op.ID {
		return
	}
	for id, gen := range g.pendingDispatch {
		for i := range g.Graph.Nodes {
			n := &g.Graph.Nodes[i]
			a := g.Attempts[n.Task.ID]
			saved := cp.State.Attempts[n.Task.ID]
			if a == nil || saved == nil || a.ChildOperation.ID != id || a.Generation != gen || saved.Generation != gen || !saved.CommandDispatched || saved.Phase != "execution" || saved.BeforeReads != a.BeforeReads || !reflect.DeepEqual(a.ChildOperation, saved.ChildOperation) {
				continue
			}
			delete(g.pendingDispatch, id)
			if n.Status != Running || n.Stale || g.Canceling {
				// This live actor still knows the reserved slot was never dispatched. A
				// recovered actor has no such ephemeral proof and cannot take this branch.
				a.CommandDispatched = false
				a.setPhase("input_validation")
				if n.Status == Canceling {
					g.Graph.Canceled(n.Task.ID, n.Generation)
				}
				s.save(g)
				s.advance(g)
				continue
			}
			if err := g.manager.Add(a.ChildOperation); err != nil {
				a.fail("execution", err)
				g.Graph.Complete(n.Task.ID, n.Generation, "", err)
				s.notify(g, n)
				s.advance(g)
			} else {
				s.advance(g)
			}
		}
	}
}
