package agentrunner

import (
	"context"
	"io"
	"slices"

	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/harness/executionbudget"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

type Config struct {
	Name         string
	Providers    []providers.Provider
	ParseRequest func(io.Reader) (Request, ToolFactory, error)
	// SubagentCommand starts a subagent runner: an executable and its leading
	// arguments. Nil runs this executable with no leading arguments.
	SubagentCommand []string
	// Optional process-local operation policy. A zero Budget uses environment
	// thresholds, which default to disabled. Sharing ResourceAccount with an
	// embedding runner bounds all of its nested managers together. Remote child
	// process internals are not claimed to be counted by this live account.
	ResourceAccount *executionbudget.Account
	Budget          executionbudget.Policy
}

type ToolConfig struct {
	Translators tool.StaticTranslators
	Names       []string
	SessionID   session.ID
	Getenv      func(string) string
}

type Tools struct {
	Registry   tool.Registry
	RemoteJobs []operation.RemoteJobHandler
	Close      func() error
}

type ToolFactory func(context.Context, ToolConfig) (Tools, error)

// ParseRequest decodes a request that configures only the static tools.
func ParseRequest(input io.Reader) (Request, ToolFactory, error) {
	var parsed Request
	if err := DecodeRequest(input, &parsed); err != nil {
		return Request{}, nil, err
	}
	return parsed, func(_ context.Context, config ToolConfig) (Tools, error) {
		return Tools{Registry: tool.NewRegistry(config.Translators, parsed.EnabledTools(config.Names...)...)}, nil
	}, nil
}

func (parsed Request) EnabledTools(names ...string) []string {
	enabled := make([]string, 0, len(names))
	for _, name := range names {
		if !slices.Contains(parsed.DisallowedTools, name) {
			enabled = append(enabled, name)
		}
	}
	return enabled
}
