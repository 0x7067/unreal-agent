package main

import (
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
	"github.com/unreallabsai/unreal-agent/harness/settings"
)

func TestStartupDefaultsAndOverrides(t *testing.T) {
	for _, test := range []struct {
		name, provider, model, effort, theme string
		args                                 []string
		env                                  map[string]string
	}{
		{name: "discover subscription", model: "gpt-6.1-sol", effort: "high", theme: "turbo-vision"},
		{name: "theme override", args: []string{"-theme", "lite"}, model: "gpt-6.1-sol", effort: "high", theme: "lite"},
		{name: "explicit Codex", args: []string{"-provider", "openai-codex"}, provider: "openai-codex", model: "gpt-6.1-sol", effort: "high", theme: "turbo-vision"},
		{name: "Codex default effort", args: []string{"-effort", "default"}, model: "gpt-6.1-sol", effort: "high", theme: "turbo-vision"},
		{name: "explicit OpenAI", args: []string{"-provider", "openai"}, provider: "openai", model: "gpt-6-astra", effort: "medium", theme: "turbo-vision"},
		{name: "explicit model", args: []string{"-model", "custom-model", "-effort", "low"}, model: "custom-model", effort: "low", theme: "turbo-vision"},
		{name: "environment", env: map[string]string{"UNREAL_HARNESS_LLM_PROVIDER": "ollama", "UNREAL_HARNESS_LLM_MODEL": "qwen3.8:27b", "UNREAL_HARNESS_LLM_REASONING_EFFORT": "xhigh"}, provider: "ollama", model: "qwen3.8:27b", effort: "xhigh", theme: "turbo-vision"},
		{name: "flags override environment", args: []string{"-provider", "openai-codex", "-model", "gpt-6-luna", "-effort", "low"}, env: map[string]string{"UNREAL_HARNESS_LLM_PROVIDER": "ollama", "UNREAL_HARNESS_LLM_MODEL": "qwen3.8:27b", "UNREAL_HARNESS_LLM_REASONING_EFFORT": "xhigh"}, provider: "openai-codex", model: "gpt-6-luna", effort: "low", theme: "turbo-vision"},
	} {
		t.Run(test.name, func(t *testing.T) {
			opts, err := parseOptions(test.args, func(name string) string { return test.env[name] }, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			if opts.provider != test.provider || opts.model != test.model || opts.effort != test.effort {
				t.Fatalf("startup selection = %q / %q / %q, want %q / %q / %q", opts.provider, opts.model, opts.effort, test.provider, test.model, test.effort)
			}
			palette, err := loadTheme(test.theme)
			if err != nil {
				t.Fatal(err)
			}
			if opts.theme != palette {
				t.Fatalf("startup theme differs from %q", test.theme)
			}
		})
	}
}

func TestStartupMissingProviderModel(t *testing.T) {
	_, err := parseOptions([]string{"-provider", "ollama"}, func(string) string { return "" }, io.Discard)
	if err == nil || !strings.Contains(err.Error(), `provider "ollama" requires -model`) ||
		!strings.Contains(err.Error(), "unreal-agent-tui -provider ollama -model qwen3.8:27b") {
		t.Fatalf("missing model guidance = %v", err)
	}
}

func TestDefaultCodexModelHasCompactionSettings(t *testing.T) {
	opts, err := parseOptions(nil, func(string) string { return "" }, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	configured, err := settings.Load(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	if model := configured.Model("openai-codex", opts.model); model.CompactionThreshold <= 0 || model.CompactionThreshold >= model.ContextWindow {
		t.Fatalf("default model lacks usable compaction settings: %+v", model)
	}
}

func TestStartupHelpAndVersion(t *testing.T) {
	var help strings.Builder
	_, err := parseOptions([]string{"-help"}, func(string) string { return "" }, &help)
	if !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("help error = %v", err)
	}
	for _, text := range []string{"unreal-agent-tui [flags]", "discover a Codex subscription", "default turbo-vision", "gpt-6.1-sol"} {
		if !strings.Contains(help.String(), text) {
			t.Fatalf("help is missing %q", text)
		}
	}
	opts, err := parseOptions([]string{"-version"}, func(string) string { return "" }, io.Discard)
	if err != nil || !opts.version {
		t.Fatalf("version requires startup configuration: %v", err)
	}
}

func TestRunWithoutSubscriptionShowsLaunchExamples(t *testing.T) {
	for _, test := range []struct{ name, body string }{
		{name: "no auth file"},
		{name: "API key login", body: `{"auth_mode":"apikey","OPENAI_API_KEY":"sk-secret"}`},
		{name: "invalid auth file", body: "{invalid-secret}"},
	} {
		t.Run(test.name, func(t *testing.T) {
			home := t.TempDir()
			if test.body != "" {
				if err := os.WriteFile(filepath.Join(home, "auth.json"), []byte(test.body), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			err := run(t.Context(), nil, func(name string) string {
				if name == "CODEX_HOME" {
					return home
				}
				if name == "XDG_CONFIG_HOME" {
					return home
				}
				return ""
			}, io.Discard)
			if err == nil {
				t.Fatal("missing subscription started a session")
			}
			for _, text := range []string{"no usable Codex subscription login found", "codex -c", "unreal-agent-tui -provider openai -model gpt-6.1-sol", "unreal-agent-tui -provider ollama -model qwen3.8:27b"} {
				if !strings.Contains(err.Error(), text) {
					t.Fatalf("startup guidance is missing %q: %v", text, err)
				}
			}
			if strings.Contains(err.Error(), "secret") {
				t.Fatal("startup guidance exposed credentials")
			}
		})
	}
}

func TestSubscriptionDialogChoice(t *testing.T) {
	for _, test := range []struct {
		name     string
		keys     []tea.Key
		accepted bool
	}{
		{name: "confirm", keys: []tea.Key{{Code: tea.KeyEnter}}, accepted: true},
		{name: "yes shortcut", keys: []tea.Key{{Code: 'y', Text: "y"}}, accepted: true},
		{name: "decline", keys: []tea.Key{{Code: tea.KeyTab}, {Code: tea.KeyTab}, {Code: tea.KeyRight}, {Code: tea.KeyEnter}}},
		{name: "no shortcut", keys: []tea.Key{{Code: 'n', Text: "n"}}},
		{name: "escape", keys: []tea.Key{{Code: tea.KeyEscape}}},
		{name: "interrupt", keys: []tea.Key{{Code: 'c', Mod: tea.ModCtrl}}},
		{name: "choose yes", keys: []tea.Key{{Code: tea.KeyDown}, {Code: tea.KeyDown}, {Code: tea.KeyRight}, {Code: tea.KeyLeft}, {Code: tea.KeyEnter}}, accepted: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			opts, err := parseOptions(nil, func(string) string { return "" }, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			dialog := newSubscriptionDialog(opts, settings.Settings{})
			if dialog.Init() != nil {
				t.Fatal("subscription dialog schedules background activity")
			}
			var cmd tea.Cmd
			for _, key := range test.keys {
				var updated tea.Model
				updated, cmd = dialog.Update(tea.KeyPressMsg(key))
				dialog = updated.(subscriptionDialog)
			}
			if dialog.accepted != test.accepted || cmd == nil {
				t.Fatalf("accepted = %t, want %t; quit command present = %t", dialog.accepted, test.accepted, cmd != nil)
			}
			if _, ok := cmd().(tea.QuitMsg); !ok {
				t.Fatal("choice did not close the dialog")
			}
		})
	}
}

func TestSubscriptionDialogRendering(t *testing.T) {
	opts, err := parseOptions(nil, func(string) string { return "" }, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	for _, size := range [][2]int{{20, 8}, {60, 18}, {80, 24}, {120, 40}} {
		dialog := newSubscriptionDialog(opts, settings.Settings{})
		updated, _ := dialog.Update(tea.WindowSizeMsg{Width: size[0], Height: size[1]})
		content := updated.(subscriptionDialog).View().Content
		if lipgloss.Width(content) != size[0] || lipgloss.Height(content) != size[1] {
			t.Fatalf("dialog dimensions = %dx%d, want %dx%d", lipgloss.Width(content), lipgloss.Height(content), size[0], size[1])
		}
		if size[0] == 20 {
			if !strings.Contains(content, "Resize terminal") {
				t.Fatal("small terminal lacks resize guidance")
			}
			continue
		}
		for _, text := range []string{"Codex subscription available", "gpt-6.1-sol", "Reasoning:", "high", "Yes", "No", "remembered"} {
			if !strings.Contains(ansi.Strip(content), text) {
				t.Fatalf("dialog is missing %q", text)
			}
		}
		assertRenderedColors(t, content, "gpt-6.1-sol", "#ffffff", "#000000")
	}
}

func TestSubscriptionDialogSelectors(t *testing.T) {
	opts, err := parseOptions(nil, func(string) string { return "" }, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	configured, err := settings.Load(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	provider := configured.Providers["openai-codex"]
	provider.Models = append(provider.Models, settings.Model{ID: "custom-model"})
	configured.Providers["openai-codex"] = provider
	dialog := newSubscriptionDialog(opts, configured)
	for _, test := range []struct {
		key           tea.Key
		model, effort string
	}{
		{key: tea.Key{Code: tea.KeyRight}, model: "gpt-6-astra", effort: "high"},
		{key: tea.Key{Code: tea.KeyRight}, model: "gpt-6-luna", effort: "high"},
		{key: tea.Key{Code: tea.KeyRight}, model: "gpt-6-sol", effort: "high"},
		{key: tea.Key{Code: tea.KeyRight}, model: "gpt-5.6-sol", effort: "high"},
		{key: tea.Key{Code: tea.KeyRight}, model: "gpt-5.6-terra", effort: "high"},
		{key: tea.Key{Code: tea.KeyRight}, model: "gpt-5.6-luna", effort: "high"},
		{key: tea.Key{Code: tea.KeyRight}, model: "custom-model", effort: "high"},
		{key: tea.Key{Code: tea.KeyRight}, model: "gpt-6.1-sol", effort: "high"},
		{key: tea.Key{Code: tea.KeyLeft}, model: "custom-model", effort: "high"},
		{key: tea.Key{Code: tea.KeyTab}, model: "custom-model", effort: "high"},
		{key: tea.Key{Code: tea.KeyRight}, model: "custom-model", effort: "xhigh"},
		{key: tea.Key{Code: tea.KeyRight}, model: "custom-model", effort: "max"},
		{key: tea.Key{Code: tea.KeyRight}, model: "custom-model", effort: "low"},
		{key: tea.Key{Code: tea.KeyLeft}, model: "custom-model", effort: "max"},
	} {
		updated, cmd := dialog.Update(tea.KeyPressMsg(test.key))
		dialog = updated.(subscriptionDialog)
		if cmd != nil || dialog.accepted {
			t.Fatal("changing a selector accepted the subscription")
		}
		if dialog.opts.model != test.model || dialog.opts.effort != test.effort {
			t.Fatalf("selector choice = %s / %s, want %s / %s", dialog.opts.model, dialog.opts.effort, test.model, test.effort)
		}
	}
}
