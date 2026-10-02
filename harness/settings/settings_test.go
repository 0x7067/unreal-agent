package settings

import (
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "models.json")
	builtin := Settings{Providers: map[string]Provider{
		"anthropic": {
			Info:   ProviderInfo{ID: "anthropic", Name: "Anthropic"},
			Models: []Model{{ID: "claude-opus-5-5", Name: "Claude Opus 5.5", ContextWindow: 1_000_000, CompactionThreshold: 500_000}},
		},
		"openai": {
			Info: ProviderInfo{ID: "openai", Name: "OpenAI"},
			Models: []Model{
				{ID: "gpt-6-astra", Name: "GPT-6 Astra", ContextWindow: 1_050_000, CompactionThreshold: 244_800},
				{ID: "gpt-6.1-sol", Name: "GPT-6.1 Sol", ContextWindow: 1_050_000, CompactionThreshold: 244_800},
				{ID: "gpt-6-luna", Name: "GPT-6 Luna", ContextWindow: 1_050_000, CompactionThreshold: 244_800},
			},
		},
		"openai-codex": {
			Info: ProviderInfo{ID: "openai-codex", Name: "OpenAI Codex"},
			Models: []Model{
				{ID: "gpt-6-astra", Name: "GPT-6 Astra", ContextWindow: 1_050_000, CompactionThreshold: 244_800},
				{ID: "gpt-6.1-sol", Name: "GPT-6.1 Sol", ContextWindow: 1_050_000, CompactionThreshold: 244_800},
				{ID: "gpt-6-luna", Name: "GPT-6 Luna", ContextWindow: 1_050_000, CompactionThreshold: 244_800},
			},
		},
	}}
	configured, err := Load(path)
	if err != nil || !reflect.DeepEqual(configured, builtin) {
		t.Fatalf("missing settings = %#v, error = %v", configured, err)
	}
	writeSettings(t, path, `{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[
		{"id":"gpt-6-astra","name":"Custom Astra","context_window":200000,"compaction_threshold":150000},
		{"id":"custom","name":"Custom model","context_window":150000},
		{"id":"zero","name":"Zero threshold","context_window":100000,"compaction_threshold":0}
	]}}}`)
	configured, err = Load(path)
	if err != nil {
		t.Fatal(err)
	}
	want := Settings{Providers: maps.Clone(builtin.Providers)}
	want.Providers["openai"] = Provider{
		Info: ProviderInfo{ID: "openai", Name: "OpenAI"},
		Models: []Model{
			{ID: "gpt-6-astra", Name: "Custom Astra", ContextWindow: 200_000, CompactionThreshold: 150_000},
			{ID: "gpt-6.1-sol", Name: "GPT-6.1 Sol", ContextWindow: 1_050_000, CompactionThreshold: 244_800},
			{ID: "gpt-6-luna", Name: "GPT-6 Luna", ContextWindow: 1_050_000, CompactionThreshold: 244_800},
			{ID: "custom", Name: "Custom model", ContextWindow: 150_000, CompactionThreshold: 75_000},
			{ID: "zero", Name: "Zero threshold", ContextWindow: 100_000, CompactionThreshold: 50_000},
		},
	}
	if !reflect.DeepEqual(configured, want) {
		t.Fatalf("settings = %#v", configured)
	}
	configured.Providers["anthropic"].Models[0] = Model{CompactionThreshold: 1}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	configured, err = Load(path)
	if err != nil || !reflect.DeepEqual(configured, builtin) {
		t.Fatalf("overrides changed built-in settings: %#v, error = %v", configured, err)
	}
}

func TestLoadDefaultsThresholdToHalfContextWindow(t *testing.T) {
	for _, test := range []struct {
		window, threshold int64
	}{
		{window: 1, threshold: 0},
		{window: 100_000, threshold: 50_000},
		{window: 100_001, threshold: 50_000},
		{window: math.MaxInt64, threshold: 4_611_686_018_427_387_903},
	} {
		t.Run(fmt.Sprint(test.window), func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "models.json")
			writeSettings(t, path, fmt.Sprintf(`{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":"gpt-6-astra","name":"Custom Astra","context_window":%d}]}}}`, test.window))
			configured, err := Load(path)
			if err != nil {
				t.Fatal(err)
			}
			if got := configured.Model("openai", "gpt-6-astra").CompactionThreshold; got != test.threshold {
				t.Fatalf("threshold = %d, want %d", got, test.threshold)
			}
		})
	}
}

func TestLoadRejectsInvalidModels(t *testing.T) {
	for _, contents := range []string{
		``,
		`{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":}}`,
		`{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":{"other":{"compaction_threshold":1000}}}}}`,
		`{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"name":"Other","context_window":100000}]}}}`,
		`{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":" ","name":"Other","context_window":100000}]}}}`,
		`{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":"other","context_window":100000}]}}}`,
		`{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":"other","name":" ","context_window":100000}]}}}`,
		`{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":"other","name":"Other"}]}}}`,
		`{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":"other","name":"Other","context_window":0}]}}}`,
		`{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":"other","name":"Other","context_window":-1}]}}}`,
		`{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":"other","name":"Other","context_window":1.5}]}}}`,
		`{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":"other","name":"Other","context_window":"100000"}]}}}`,
		`{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":"other","name":"Other","context_window":9223372036854775808}]}}}`,
		`{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":"other","name":"Other","context_window":100000,"compaction_threshold":-1}]}}}`,
		`{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":"other","name":"Other","context_window":100000,"compaction_threshold":1.5}]}}}`,
		`{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":"other","name":"Other","context_window":100000,"compaction_threshold":"1000"}]}}}`,
		`{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":"other","name":"Other","context_window":100000,"compaction_threshold":9223372036854775808}]}}}`,
		`{"providers":{"openai":{"info":{"id":"openai","name":"OpenAI"},"models":[{"id":"other","name":"First","context_window":100000},{"id":"other","name":"Second","context_window":200000}]}}}`,
	} {
		t.Run(contents, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "models.json")
			writeSettings(t, path, contents)
			if _, err := Load(path); err == nil {
				t.Fatal("accepted invalid settings")
			}
		})
	}
}

func TestLoadMergesModelsWithinProviders(t *testing.T) {
	path := filepath.Join(t.TempDir(), "settings.json")
	writeSettings(t, path, `{"providers":{
		"openai":{"info":{"id":"openai","name":"Custom OpenAI"},"models":[
			{"id":"custom","name":"Custom","context_window":200000}
		]},
		"other":{"info":{"id":"other","name":"Other"},"models":[
			{"id":"gpt-6-astra","name":"Other Astra","context_window":100000},
			{"id":"custom","name":"Other custom","context_window":80000}
		]}
	}}`)
	configured, err := Load(path)
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range []struct {
		provider, model string
		threshold       int64
	}{
		{provider: "openai", model: "gpt-6-astra", threshold: 244_800},
		{provider: "openai-codex", model: "gpt-6-astra", threshold: 244_800},
		{provider: "openai", model: "gpt-6.1-sol", threshold: 244_800},
		{provider: "openai-codex", model: "gpt-6.1-sol", threshold: 244_800},
		{provider: "openai", model: "gpt-6-luna", threshold: 244_800},
		{provider: "openai-codex", model: "gpt-6-luna", threshold: 244_800},
		{provider: "other", model: "gpt-6-astra", threshold: 50_000},
		{provider: "openai", model: "custom", threshold: 100_000},
		{provider: "other", model: "custom", threshold: 40_000},
		{provider: "missing", model: "gpt-6-astra"},
		{provider: "openai", model: "missing"},
	} {
		if got := configured.Model(test.provider, test.model).CompactionThreshold; got != test.threshold {
			t.Fatalf("%s/%s threshold = %d, want %d", test.provider, test.model, got, test.threshold)
		}
	}
	if configured.Providers["openai"].Info.Name != "Custom OpenAI" {
		t.Fatal("provider info override was not loaded")
	}
}

func TestLoadRejectsInvalidProviders(t *testing.T) {
	for _, contents := range []string{
		`{"providers":[]}`,
		`{"providers":{"": {"info":{"id":"","name":"Empty"}}}}`,
		`{"providers":{"other": {"models":[]}}}`,
		`{"providers":{"other": {"info":{"name":"Other"}}}}`,
		`{"providers":{"other": {"info":{"id":"different","name":"Other"}}}}`,
		`{"providers":{"other": {"info":{"id":"other"}}}}`,
		`{"providers":{"other": {"info":{"id":"other","name":" "}}}}`,
	} {
		t.Run(contents, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "settings.json")
			writeSettings(t, path, contents)
			if _, err := Load(path); err == nil {
				t.Fatal("accepted invalid provider")
			}
		})
	}
}

func TestLoadReportsReadError(t *testing.T) {
	if _, err := Load(t.TempDir()); err == nil {
		t.Fatal("accepted directory as settings file")
	}
}

func writeSettings(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o600); err != nil {
		t.Fatal(err)
	}
}
