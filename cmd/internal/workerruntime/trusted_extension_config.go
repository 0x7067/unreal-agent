package workerruntime

import (
	"context"
	"errors"
	"fmt"
	"io"
	"sync"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/durablejob"
	"github.com/unreallabsai/unreal-agent/harness/extension"
)

const extensionConfigEnvironment = "UNREAL_AGENT_EXTENSION_CONFIG"

type extensionConfiguration struct {
	Version  int
	Known    []extension.Descriptor
	Selected uint64
}
type configuredExtensions struct {
	path     string
	registry *extension.Registry
	known    map[uint64]extension.Descriptor
	mu       sync.Mutex
	done     chan struct{}
}

func loadExtensions(path string) (*configuredExtensions, error) {
	if path == "" {
		return nil, nil
	}
	owner := &configuredExtensions{path: path, registry: extension.NewRegistry(), known: map[uint64]extension.Descriptor{}}
	if err := owner.refresh(); err != nil {
		return nil, err
	}
	return owner, nil
}
func (e *configuredExtensions) refresh() error {
	var cfg extensionConfiguration
	if err := durablejob.ReadPrivateConfig(e.path, &cfg); err != nil {
		return err
	}
	if cfg.Version != 1 || len(cfg.Known) == 0 || len(cfg.Known) > extension.MaxGenerations {
		return errors.New("unsupported trusted extension config")
	}
	// Validate the entire candidate before touching the live selection/leases.
	candidate := extension.NewRegistry()
	next := map[uint64]extension.Descriptor{}
	for _, d := range cfg.Known {
		if err := candidate.Install(d); err != nil {
			return err
		}
		next[d.Generation] = d
	}
	if err := candidate.Select(cfg.Selected); err != nil {
		return err
	}
	e.mu.Lock()
	defer e.mu.Unlock()
	for g, d := range e.known {
		if next[g] != d {
			return errors.New("installed generation removal/change refused; pending old calls must retain code")
		}
	}
	for _, d := range cfg.Known {
		if _, ok := e.known[d.Generation]; !ok {
			if err := e.registry.Install(d); err != nil {
				return err
			}
		}
	}
	if err := e.registry.Select(cfg.Selected); err != nil {
		return err
	}
	e.known = next
	return nil
}
func (e *configuredExtensions) watch(ctx context.Context, stderr io.Writer) {
	e.done = make(chan struct{})
	go func() {
		defer close(e.done)
		tick := time.NewTicker(100 * time.Millisecond)
		defer tick.Stop()
		last := ""
		var selected uint64
		for {
			select {
			case <-ctx.Done():
				return
			case <-tick.C:
				if err := e.refresh(); err != nil {
					text := err.Error()
					if len(text) > 512 {
						text = text[:512]
					}
					if text != last {
						_, _ = fmt.Fprintln(stderr, "trusted extension install refused:", text)
					}
					last = text
				} else {
					if descriptor, err := e.registry.Selected(); err == nil && descriptor.Generation != selected {
						selected = descriptor.Generation
						_, _ = fmt.Fprintln(stderr, "trusted extension selected generation:", selected)
					}
					last = ""
				}
			}
		}
	}()
}

// Configuration contains only compiled identities; source, callbacks, code paths
// and credentials are not admitted. Plan generation is persisted before dispatch.
