package workerruntime

import (
	"context"
	"errors"
	"github.com/unreallabsai/unreal-agent/cmd/internal/durablechild"
	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"os"
)

func durableOwnerConfig(b workerproto.Bootstrap) (durablechild.Config, error) {
	command := append([]string(nil), b.DurableChildCommand...)
	if len(command) == 0 {
		executable, err := os.Executable()
		if err != nil {
			return durablechild.Config{}, err
		}
		command = []string{executable}
	}
	if b.ChildRuntimeDirectory == "" {
		return durablechild.Config{}, errors.New("durable children require explicit private child runtime directory")
	}
	return durablechild.Config{Directory: b.Directory, RuntimeDirectory: b.ChildRuntimeDirectory, Workspace: b.Workspace, Provider: b.Provider, BaseURL: b.BaseURL, Model: b.Model.ID, Command: command}, nil
}
func validateChildOwnership(ctx context.Context, b workerproto.Bootstrap, h sessionstore.LifecycleHistory, latest []operation.Operation) error {
	if !b.DurableSubagents {
		return h.ValidateResume()
	}
	cfg, err := durableOwnerConfig(b)
	if err != nil {
		return err
	}
	return durablechild.ReconcileHistory(ctx, cfg, b.SessionID, h, latest)
}

// Resolve only inert metadata, before ownership checks and before client setup.
func normalizeDurableOwner(b workerproto.Bootstrap, available []providers.Provider) (workerproto.Bootstrap, error) {
	if b.DurableSubagents && b.BaseURL == "" {
		provider, err := providers.Find(available, b.Provider)
		if err != nil {
			return b, err
		}
		b.BaseURL = provider.BaseURL
	}
	return b, nil
}
