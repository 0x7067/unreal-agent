package main

import (
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunLoadsModelSettingsBeforeOpeningTerminal(t *testing.T) {
	configHome := t.TempDir()
	if err := os.Mkdir(filepath.Join(configHome, "unreal-agent"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(configHome, "unreal-agent", "settings.json"), []byte(`{"providers":{"openrouter":{"info":{"id":"openrouter","name":"OpenRouter"},"models":[{"id":"selected","name":"Selected","context_window":100000,"compaction_threshold":-1}]}}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("unexpected request %s %s", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	for _, test := range []struct{ name, key string }{
		{name: "ordinary key", key: "test-key"},
		{name: "padded key", key: " test-key "},
		{name: "whitespace key", key: " \t "},
	} {
		t.Run(test.name, func(t *testing.T) {
			err := run(t.Context(), []string{"-provider", "openrouter", "-model", "selected", "-base-url", server.URL, "-max-attempts", "1"}, func(name string) string {
				if name == "XDG_CONFIG_HOME" {
					return configHome
				}
				if name == "OPENROUTER_API_KEY" {
					return test.key
				}
				return ""
			}, io.Discard)
			if err == nil || !strings.Contains(err.Error(), "load model settings") {
				t.Fatalf("run error = %v", err)
			}
		})
	}
}
