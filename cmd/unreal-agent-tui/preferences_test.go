package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/settings"
)

func TestAcceptedStartupChoiceIsRemembered(t *testing.T) {
	path := filepath.Join(t.TempDir(), "unreal-agent", "preferences.json")
	preferences, err := loadStartupPreferences(path)
	if err != nil || preferences != (startupPreferences{}) {
		t.Fatalf("first launch preferences = %+v, error = %v", preferences, err)
	}
	opts, err := parseOptions(nil, func(string) string { return "" }, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	opts, showDialog := startupSelection(opts, preferences)
	if !showDialog {
		t.Fatal("first launch skipped consent")
	}
	configured, err := settings.Load(filepath.Join(t.TempDir(), "settings.json"))
	if err != nil {
		t.Fatal(err)
	}
	dialog := newSubscriptionDialog(opts, configured)
	for _, key := range []tea.Key{{Code: tea.KeyRight}, {Code: tea.KeyTab}, {Code: tea.KeyLeft}, {Code: tea.KeyLeft}, {Code: tea.KeyEnter}} {
		updated, _ := dialog.Update(tea.KeyPressMsg(key))
		dialog = updated.(subscriptionDialog)
	}
	if !dialog.accepted || dialog.opts.model != "gpt-6-astra" || dialog.opts.effort != "low" {
		t.Fatalf("accepted dialog choice = %+v", dialog)
	}
	if err := saveStartupPreferences(path, startupPreferences{
		Provider: dialog.opts.provider, Model: dialog.opts.model, ReasoningEffort: llm.ReasoningEffort(dialog.opts.effort),
	}); err != nil {
		t.Fatal(err)
	}
	preferences, err = loadStartupPreferences(path)
	if err != nil {
		t.Fatal(err)
	}
	defaults, err := parseOptions(nil, func(string) string { return "" }, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	second, showDialog := startupSelection(defaults, preferences)
	if showDialog || second.provider != "openai-codex" || second.model != "gpt-6-astra" || second.effort != "low" {
		t.Fatalf("second launch = %+v, show dialog = %t", second, showDialog)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o600 {
		t.Fatalf("preferences permissions = %o", info.Mode().Perm())
	}
}

func TestSavedStartupChoicePrecedence(t *testing.T) {
	preferences := startupPreferences{Provider: "openai-codex", Model: "gpt-6-astra", ReasoningEffort: llm.ReasoningEffortLow}
	for _, test := range []struct {
		name, provider, model, effort string
		args                          []string
		env                           map[string]string
		dialog                        bool
	}{
		{name: "saved defaults", provider: "openai-codex", model: "gpt-6-astra", effort: "low"},
		{name: "reopen setup", args: []string{"-setup"}, provider: "openai-codex", model: "gpt-6-astra", effort: "low", dialog: true},
		{name: "explicit Codex", args: []string{"-provider", "openai-codex"}, provider: "openai-codex", model: "gpt-6-astra", effort: "low"},
		{name: "model flag", args: []string{"-model", "other-model"}, provider: "openai-codex", model: "other-model", effort: "low"},
		{name: "effort flag", args: []string{"-effort", "max"}, provider: "openai-codex", model: "gpt-6-astra", effort: "max"},
		{name: "explicit default effort", args: []string{"-effort", "default"}, provider: "openai-codex", model: "gpt-6-astra", effort: "high"},
		{name: "empty model flag", args: []string{"-model", ""}, provider: "openai-codex", model: "gpt-6.1-sol", effort: "low"},
		{name: "environment", env: map[string]string{"UNREAL_HARNESS_LLM_MODEL": "env-model", "UNREAL_HARNESS_LLM_REASONING_EFFORT": "medium"}, provider: "openai-codex", model: "env-model", effort: "medium"},
		{name: "flags override environment", args: []string{"-model", "flag-model", "-effort", "max"}, env: map[string]string{"UNREAL_HARNESS_LLM_MODEL": "env-model", "UNREAL_HARNESS_LLM_REASONING_EFFORT": "medium"}, provider: "openai-codex", model: "flag-model", effort: "max"},
		{name: "other provider", args: []string{"-provider", "openai"}, provider: "openai", model: "gpt-6-astra", effort: "medium"},
	} {
		t.Run(test.name, func(t *testing.T) {
			opts, err := parseOptions(test.args, func(name string) string { return test.env[name] }, io.Discard)
			if err != nil {
				t.Fatal(err)
			}
			opts, dialog := startupSelection(opts, preferences)
			if opts.provider != test.provider || opts.model != test.model || opts.effort != test.effort || dialog != test.dialog {
				t.Fatalf("selection = %s / %s / %s, dialog = %t", opts.provider, opts.model, opts.effort, dialog)
			}
		})
	}
	if _, err := parseOptions([]string{"-setup", "-provider", "openai"}, func(string) string { return "" }, io.Discard); err == nil {
		t.Fatal("Codex setup accepted a different provider")
	}
}

func TestStartupPreferencesRejectInvalidFiles(t *testing.T) {
	for _, data := range []string{
		"{invalid}",
		`{}`,
		`{"provider":"openai","model":"gpt-6.1-sol","reasoning_effort":"high"}`,
		`{"provider":"openai-codex","model":"","reasoning_effort":"high"}`,
		`{"provider":"openai-codex","model":"gpt-6.1-sol","reasoning_effort":"default"}`,
		`{"provider":"openai-codex","model":"gpt-6.1-sol","reasoning_effort":"high","unexpected":true}`,
	} {
		path := filepath.Join(t.TempDir(), "preferences.json")
		if err := os.WriteFile(path, []byte(data), 0o600); err != nil {
			t.Fatal(err)
		}
		if _, err := loadStartupPreferences(path); err == nil || !strings.Contains(err.Error(), "use -setup") {
			t.Fatalf("invalid preferences error = %v", err)
		}
	}
}

func TestStartupPreferencesUpdatePreservesOtherConfiguration(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "preferences.json")
	settingsPath := filepath.Join(directory, "settings.json")
	if err := os.WriteFile(settingsPath, []byte("existing model configuration"), 0o600); err != nil {
		t.Fatal(err)
	}
	preferences := startupPreferences{Provider: "openai-codex", Model: "gpt-6.1-sol", ReasoningEffort: llm.ReasoningEffortHigh}
	if err := saveStartupPreferences(path, preferences); err != nil {
		t.Fatal(err)
	}
	preferences.ReasoningEffort = llm.ReasoningEffortMedium
	if err := saveStartupPreferences(path, preferences); err != nil {
		t.Fatal(err)
	}
	loaded, err := loadStartupPreferences(path)
	if err != nil || loaded != preferences {
		t.Fatalf("updated preferences = %+v, error = %v", loaded, err)
	}
	if err := saveStartupPreferences(path, startupPreferences{}); err == nil {
		t.Fatal("invalid preferences replaced the saved choice")
	}
	loaded, err = loadStartupPreferences(path)
	if err != nil || loaded != preferences {
		t.Fatalf("failed write changed preferences: %+v, error = %v", loaded, err)
	}
	data, err := os.ReadFile(settingsPath)
	if err != nil || string(data) != "existing model configuration" {
		t.Fatal("saving preferences changed model configuration")
	}
	entries, err := os.ReadDir(directory)
	if err != nil || len(entries) != 2 {
		t.Fatalf("saving preferences left temporary files: %v", err)
	}
}
