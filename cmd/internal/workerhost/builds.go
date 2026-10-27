package workerhost

import (
	"context"
	"fmt"
)

// CheckInstalled probes the explicitly configured trusted candidate, not PATH or
// workspace configuration. It does not retire/reload the executing generation.
// A successful probe is evidence at this instant, not a promise about future
// pathname replacements. The temporary immutable candidate is always discarded.
func (h *Host) CheckInstalled(ctx context.Context) (Status, error) {
	select {
	case h.query <- struct{}{}:
		defer func() { <-h.query }()
	case <-ctx.Done():
		return Status{}, context.Cause(ctx)
	case <-h.ctx.Done():
		return Status{}, ErrClosed
	}
	h.mu.Lock()
	unavailable := h.closing || h.closed
	h.mu.Unlock()
	if unavailable {
		return Status{}, ErrClosed
	}
	stage, err := h.stage(ctx, h.config.Candidate)
	if err != nil {
		return Status{}, err
	}
	defer h.discardStage(stage.command[0])
	build := stage.identity.Build + " [" + stage.digest[:12] + "]"
	h.mu.Lock()
	h.installedBuild = build
	s := Status{InstalledBuild: build, ReloadState: "reload required to change running worker"}
	if p := h.current; p != nil {
		s.Generation, s.Build = p.generation, p.build
	}
	h.mu.Unlock()
	if s.Build == build {
		s.ReloadState = "running worker matches installed candidate"
	}
	s.Message = fmt.Sprintf("Installed candidate checked · %s", s.ReloadState)
	return s, nil
}
