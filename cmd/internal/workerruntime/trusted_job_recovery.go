package workerruntime

import (
	"context"
	"errors"
	"fmt"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"

	"github.com/unreallabsai/unreal-agent/cmd/internal/durablejob"
	"github.com/unreallabsai/unreal-agent/cmd/internal/recoverygate"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
)

type trustedJobs struct {
	config durablejob.RuntimeConfig
	token  string
}

func loadTrustedJobs(getenv func(string) string) (*trustedJobs, error) {
	path := getenv(durablejob.ConfigEnvironment)
	if path == "" {
		return nil, nil
	}
	var cfg durablejob.RuntimeConfig
	if err := durablejob.ReadPrivateConfig(path, &cfg); err != nil {
		return nil, err
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	token := getenv(durablejob.TokenEnvironment)
	client, err := cfg.Client(token)
	if err != nil {
		return nil, err
	}
	client.Close()
	return &trustedJobs{config: cfg, token: token}, nil
}
func validateWorkerRecovery(ops []operation.Operation, jobs *trustedJobs, id session.ID) error {
	var native []operation.Operation
	for _, op := range ops {
		if op.Status == operation.StatusCompleted || op.Status == operation.StatusFailed || op.Status == operation.StatusCanceled {
			native = append(native, op)
			continue
		}
		if op.Type == operation.TypeRemoteJob {
			state, err := operation.DecodeRemoteJobState(op)
			if err == nil && state.Plan.Type == durablejob.PlanType {
				if jobs == nil {
					return errors.New("independent executor configuration unavailable; no recovery/fallback")
				}
				if err := durablejob.ValidateRecovery(op, jobs.config.Authority(id)); err != nil {
					return fmt.Errorf("independent job recovery refused: %w", err)
				}
				continue
			}
		}
		native = append(native, op)
	}
	return recoverygate.Validate(native)
}

// Per-owner exception under the already held session lease. It does not invent
// an execution-drain receipt and never relaxes pending lifecycle-intent fences.
func validateConfiguredOwnership(ctx context.Context, b workerproto.Bootstrap, h sessionstore.LifecycleHistory, latest []operation.Operation, jobs *trustedJobs) error {
	if jobs == nil {
		return validateDurableOwnership(ctx, b, h, latest)
	}
	allowed := map[operation.ID]bool{}
	remaining := map[operation.ID]operation.Operation{}
	newest := map[operation.ID]operation.Operation{}
	for id, op := range h.RemoteOperations {
		newest[id] = op
	}
	for _, op := range latest {
		newest[op.ID] = op
	}
	for id, op := range h.RetainedRemoteOperations {
		if current, ok := newest[id]; ok {
			op = current
		}
		state, err := operation.DecodeRemoteJobState(op)
		if err != nil {
			return err
		}
		if state.Plan.Type != durablejob.PlanType {
			remaining[id] = op
			continue
		}
		cfg := jobs.config.Authority(b.SessionID)
		if op.Status == operation.StatusCompleted || op.Status == operation.StatusFailed || op.Status == operation.StatusCanceled {
			err = durablejob.TerminalCleanupProof(op, cfg)
		} else {
			err = durablejob.ValidateRecovery(op, cfg)
		}
		if err != nil {
			return fmt.Errorf("independent owner %s requires reconciliation: %w", id, err)
		}
		allowed[id] = true
	}
	if len(allowed) == 0 {
		return validateDurableOwnership(ctx, b, h, latest)
	}
	if len(remaining) > 0 {
		subset := h
		subset.RetainedRemoteOperations = remaining
		if err := validateDurableOwnership(ctx, b, subset, latest); err != nil {
			return err
		}
		for id := range remaining {
			allowed[id] = true
		}
	}
	return h.ValidateResumeWithOwners(allowed)
}
