package main

import (
	"errors"
	"flag"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/cmd/internal/agentrunner"
)

func TestOptionsDotEnvOptIn(t *testing.T) {
	for _, test := range []struct {
		name     string
		args     []string
		loads    int
		endpoint string
		help     bool
	}{
		{name: "default", endpoint: ""},
		{name: "opt-in", args: []string{"-load-dotenv"}, loads: 1, endpoint: "http://workspace.invalid"},
		{name: "opt-out", args: []string{"-load-dotenv=false"}, endpoint: ""},
		{name: "CLI endpoint wins", args: []string{"-load-dotenv", "-base-url", "http://explicit.invalid"}, loads: 1, endpoint: "http://explicit.invalid"},
		{name: "help", args: []string{"-load-dotenv", "-help"}, help: true},
		{name: "version", args: []string{"-load-dotenv", "-version"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			env := map[string]string{}
			loads := 0
			opts, err := parseOptionsWithLoader(test.args, func(name string) string { return env[name] }, io.Discard, func() error { loads++; env["UNREAL_HARNESS_LLM_BASE_URL"] = "http://workspace.invalid"; return nil })
			if test.help {
				if !errors.Is(err, flag.ErrHelp) {
					t.Fatalf("help error=%v", err)
				}
			} else if err != nil {
				t.Fatal(err)
			}
			if loads != test.loads || opts.baseURL != test.endpoint {
				t.Fatalf("loads=%d opts=%#v", loads, opts)
			}
		})
	}
}

func TestRunHelpAndVersionNeverReadDotEnv(t *testing.T) {
	t.Chdir(t.TempDir())
	if err := os.Mkdir(".env", 0700); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"-help"}, {"-version"}, {"-load-dotenv", "-help"}, {"-load-dotenv", "-version"}} {
		err := run(t.Context(), args, func(string) string { return "" }, io.Discard)
		if err != nil && !errors.Is(err, flag.ErrHelp) {
			t.Fatalf("%v: %v", args, err)
		}
	}
	err := run(t.Context(), []string{"-load-dotenv"}, func(string) string { return "" }, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "read environment file") {
		t.Fatalf("opt-in read error = %v", err)
	}
	configHome := t.TempDir()
	err = run(t.Context(), []string{"-provider", "openai"}, func(name string) string {
		if name == "XDG_CONFIG_HOME" {
			return configHome
		}
		return ""
	}, io.Discard)
	if err == nil || !strings.Contains(err.Error(), "OPENAI_API_KEY must be set") {
		t.Fatalf("default must skip dotenv and stop at missing fake credentials: %v", err)
	}
}

func TestOptionsLoadExplicitDotEnvAndRestore(t *testing.T) {
	name := "UNREAL_HARNESS_LLM_BASE_URL"
	old, present := os.LookupEnv(name)
	_ = os.Unsetenv(name)
	t.Cleanup(func() {
		if present {
			_ = os.Setenv(name, old)
		} else {
			_ = os.Unsetenv(name)
		}
	})
	path := filepath.Join(t.TempDir(), ".env")
	if err := os.WriteFile(path, []byte(name+"=http://loopback.invalid\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var scope io.Closer
	opts, err := parseOptionsWithLoader([]string{"-load-dotenv", "-provider", "openai"}, os.Getenv, io.Discard, func() error { var err error; scope, err = agentrunner.LoadDotEnv(path); return err })
	if scope != nil {
		defer scope.Close()
	}
	if err != nil || opts.baseURL != "http://loopback.invalid" {
		t.Fatalf("opts=%#v error=%v", opts, err)
	}
}
