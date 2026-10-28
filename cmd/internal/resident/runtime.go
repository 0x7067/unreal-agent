package resident

import (
	"context"
	"errors"
	"strings"
	"time"
	"unicode"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
)

func runtimeText(s string, n int) string {
	s = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, s)
	if len(s) > n {
		s = strings.ToValidUTF8(s[:n], "")
	}
	return s
}

// runtimeSnapshot requires publisher.mu. It is observation, never authority to
// skip Host.Reload's probe, quiescence, retirement, joins or lease fences.
func (p *publisher) runtimeSnapshot(state string) attachproto.RuntimeStatus {
	return attachproto.RuntimeStatus{Version: 1, Generation: p.status.Generation, RunningBuild: runtimeText(p.status.Build, 512), CandidateBuild: runtimeText(p.runtimeStatus.InstalledBuild, 512), State: state}
}
func (p *publisher) runtimeReply(c *client, id string, s attachproto.RuntimeStatus) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if p.active != c {
		return
	}
	s.RunningBuild = runtimeText(s.RunningBuild, 512)
	s.CandidateBuild = runtimeText(s.CandidateBuild, 512)
	s.Message = runtimeText(s.Message, 1024)
	s.Error = runtimeText(s.Error, 1024)
	e := p.event("runtime")
	e.ID = id
	e.Connection = c.token
	e.Runtime = &s
	p.enqueue(c, e)
}
func (p *publisher) startRuntime(ctx context.Context, host *workerhost.Host, c *client, r attachproto.Request) {
	p.mu.Lock()
	select {
	case <-c.stopped:
		p.mu.Unlock()
		return
	default:
	}
	if p.active != c || ctx.Err() != nil {
		p.mu.Unlock()
		return
	}
	s := p.runtimeSnapshot("failed")
	if !p.runtimeEnabled {
		p.mu.Unlock()
		s.Error = "runtime replacement capability is disabled"
		p.runtimeReply(c, r.ID, s)
		return
	}
	if r.Runtime.Action == "cancel" {
		c.runtimeMu.Lock()
		cancel := c.runtimeCancel
		match := cancel != nil && c.runtimeID == r.Runtime.TargetID
		c.runtimeMu.Unlock()
		p.mu.Unlock()
		if match {
			cancel()
			s.State = "cancel_requested"
			s.Message = "cancellation admitted for this client's runtime request; retirement/completion is not yet confirmed"
		} else {
			s.Error = "no matching current client runtime request"
		}
		p.runtimeReply(c, r.ID, s)
		return
	}
	if r.Runtime.Action != "check" && r.Runtime.ExpectedGeneration != p.status.Generation {
		p.mu.Unlock()
		s.Error = "stale runtime generation; inspect current candidate/running state"
		p.runtimeReply(c, r.ID, s)
		return
	}
	c.runtimeMu.Lock()
	if c.runtimeCancel != nil {
		c.runtimeMu.Unlock()
		p.mu.Unlock()
		s.State = "blocked"
		s.Error = "runtime control already in progress for this frontend"
		p.runtimeReply(c, r.ID, s)
		return
	}
	control, cancel := context.WithTimeout(ctx, 45*time.Second)
	c.runtimeID, c.runtimeCancel = r.ID, cancel
	c.runtimeWG.Add(1)
	c.runtimeMu.Unlock()
	p.mu.Unlock()
	s.State = "checking"
	if r.Runtime.Action != "check" {
		s.State = "replacing"
	}
	if r.Runtime.Action == "drain" {
		s.State = "queued"
	}
	s.Message = "trusted host runtime control in progress; session input admission remains independent"
	p.runtimeReply(c, r.ID, s)
	go func() {
		defer c.runtimeWG.Done()
		defer cancel()
		defer func() {
			c.runtimeMu.Lock()
			if c.runtimeID == r.ID {
				c.runtimeID = ""
				c.runtimeCancel = nil
			}
			c.runtimeMu.Unlock()
		}()
		var result workerhost.Status
		var err error
		if r.Runtime.Action == "check" {
			result, err = host.CheckInstalled(control)
		} else {
			if r.Runtime.Action == "retire-host" {
				err = host.RetireForHostReplacement(control)
			} else if r.Runtime.Action == "drain" {
				err = p.drainRuntime(control, host, c, r)
			} else {
				err = host.Reload(control, nil)
			}
		}
		p.mu.Lock()
		out := p.runtimeSnapshot("failed")
		if r.Runtime.Action == "check" && err == nil {
			p.runtimeStatus.InstalledBuild = result.InstalledBuild
			out.CandidateBuild = runtimeText(result.InstalledBuild, 512)
			out.Generation, out.RunningBuild = result.Generation, runtimeText(result.Build, 512)
		}
		stopped := p.status.Stopped
		p.mu.Unlock()
		switch {
		case err == nil && r.Runtime.Action == "retire-host" && stopped:
			out.State = "drained"
			out.Message = "worker naturally drained and joined; admission sealed; ingress retained until explicit host stop and actual wait"
		case err == nil && r.Runtime.Action == "check" && out.Generation > 0 && out.RunningBuild != "":
			out.State = "ready"
			out.Message = result.ReloadState
		case err == nil && r.Runtime.Action != "check" && out.Generation > r.Runtime.ExpectedGeneration && !stopped:
			out.State = "completed"
			out.Message = "old process retired and joined; replacement running"
		case stopped || out.RunningBuild == "":
			out.State = "stopped"
			out.Message = "worker is stopped; accepted inputs remain durable; no automatic rollback"
		case errors.Is(err, workerhost.ErrBusy):
			out.State = "blocked"
			out.Message = "active or persistence-unconfirmed work blocks replacement; no work was killed to upgrade"
		case errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded):
			out.State = "canceled"
			out.Message = "runtime request ended without confirmed replacement; inspect current running state"
		default:
			out.Message = "runtime replacement was not confirmed"
		}
		if err != nil {
			out.Error = err.Error()
		}
		p.runtimeReply(c, r.ID, out)
	}()
}

// Natural drain is deliberately not checkpoint/replay. Every attempt uses the
// unchanged Host quiescence/fence/join protocol. Busy work continues untouched;
// bounded request cancellation never cancels session operations.
func (p *publisher) drainRuntime(ctx context.Context, host *workerhost.Host, c *client, r attachproto.Request) error {
	for attempt := 0; attempt < 46; attempt++ {
		if err := ctx.Err(); err != nil {
			return err
		}
		p.mu.Lock()
		current := p.status.Generation
		p.mu.Unlock()
		if current != r.Runtime.ExpectedGeneration {
			return errors.New("stale queued replacement generation; inspect current state")
		}
		err := host.Reload(ctx, nil)
		if !errors.Is(err, workerhost.ErrBusy) {
			return err
		}
		p.mu.Lock()
		queued := p.runtimeSnapshot("queued")
		p.mu.Unlock()
		queued.Message = "queued for natural idle drain; active work is not killed or replayed; candidate is rechecked at handoff"
		p.runtimeReply(c, r.ID, queued)
		timer := time.NewTimer(time.Second)
		select {
		case <-ctx.Done():
			timer.Stop()
			return ctx.Err()
		case <-timer.C:
		}
	}
	return context.DeadlineExceeded
}
