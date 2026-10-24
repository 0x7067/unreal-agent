//go:build linux || darwin

package agentrunner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
	"uuid"

	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

func runnerLeaseDirectory(t *testing.T) string {
	t.Helper()
	directory, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return directory
}

func leaseRunnerEnvironment(name string) string {
	if name == providers.APIKeyEnvironment {
		return "secret"
	}
	return ""
}

func TestRunLeasePrecedesReplayAndCreate(t *testing.T) {
	for _, existing := range []bool{false, true} {
		t.Run(fmt.Sprintf("existing-%v", existing), func(t *testing.T) {
			workspace, directory := t.TempDir(), runnerLeaseDirectory(t)
			lease, err := localfile.AcquireLease(directory, "locked")
			if err != nil {
				t.Fatal(err)
			}
			defer lease.Close()
			path := filepath.Join(directory, "locked.session.jsonl")
			if existing {
				// If Run replays before acquiring ownership, this corrupt history
				// produces a decode error instead of ErrSessionLocked.
				if err := os.WriteFile(path, []byte("not session history\n"), 0o600); err != nil {
					t.Fatal(err)
				}
			}
			client := &fakeClient{respond: func(context.Context, llm.Request) (llm.Response, error) {
				return llm.Response{}, errors.New("locked session reached model")
			}}
			err = Run(t.Context(), []string{"-workspace", workspace, "-session-directory", directory},
				leaseRunnerEnvironment, func() []string { return nil },
				strings.NewReader(`{"session_id":"locked","prompt":"hello"}`), io.Discard, io.Discard, testConfig(client))
			if !errors.Is(err, localfile.ErrSessionLocked) {
				t.Fatalf("Run = %v, want ErrSessionLocked before replay/create", err)
			}
			if !existing {
				if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("locked run created session history: %v", err)
				}
			}
		})
	}
}

func TestRunRejectsConcurrentResume(t *testing.T) {
	workspace, directory := t.TempDir(), runnerLeaseDirectory(t)
	store, err := localfile.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(t.Context(), "concurrent-resume"); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*time.Second)
	defer cancel()
	started, release := make(chan struct{}), make(chan struct{})
	client := &fakeClient{respond: func(ctx context.Context, _ llm.Request) (llm.Response, error) {
		select {
		case started <- struct{}{}:
		case <-ctx.Done():
			return llm.Response{}, ctx.Err()
		}
		select {
		case <-release:
			return llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: "done"}}}}, nil
		case <-ctx.Done():
			return llm.Response{}, ctx.Err()
		}
	}}
	request := `{"session_id":"concurrent-resume","messages":[{"content":"hello","message_id":"69621f8d-4f4d-49a5-8f7d-3b24fd855c01"}]}`
	run := func(client *fakeClient) error {
		return Run(ctx, []string{"-workspace", workspace, "-session-directory", directory},
			leaseRunnerEnvironment, func() []string { return nil }, strings.NewReader(request), io.Discard, io.Discard, testConfig(client))
	}
	first := make(chan error, 1)
	go func() { first <- run(client) }()
	select {
	case <-started:
	case err := <-first:
		t.Fatalf("first run exited before model: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	second := &fakeClient{respond: func(context.Context, llm.Request) (llm.Response, error) {
		return llm.Response{}, errors.New("concurrent model request")
	}}
	if err := run(second); !errors.Is(err, localfile.ErrSessionLocked) {
		t.Fatalf("concurrent resume = %v, want ErrSessionLocked", err)
	}
	close(release)
	select {
	case err := <-first:
		if err != nil {
			t.Fatal(err)
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err := run(second); err != nil {
		t.Fatalf("resume after shutdown = %v", err)
	}
}

func TestRunNewSessionLeaseHeldThroughToolShutdown(t *testing.T) {
	workspace, directory := t.TempDir(), runnerLeaseDirectory(t)
	client := &fakeClient{respond: func(context.Context, llm.Request) (llm.Response, error) {
		return llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: "done"}}}}, nil
	}}
	config := testConfig(client)
	var id session.ID
	var shutdownChecked bool
	config.ParseRequest = func(input io.Reader) (Request, ToolFactory, error) {
		request, factory, err := parseTestRequest(input)
		if err != nil {
			return Request{}, nil, err
		}
		return request, func(ctx context.Context, tools ToolConfig) (Tools, error) {
			id = tools.SessionID
			if _, err := uuid.Parse(string(id)); err != nil {
				return Tools{}, fmt.Errorf("new session ID is not a UUID: %w", err)
			}
			if lease, err := localfile.AcquireLease(directory, id); !errors.Is(err, localfile.ErrSessionLocked) {
				_ = lease.Close()
				return Tools{}, fmt.Errorf("new session is not leased at tool startup: %v", err)
			}
			configured, err := factory(ctx, tools)
			if err != nil {
				return Tools{}, err
			}
			oldClose := configured.Close
			configured.Close = func() error {
				shutdownChecked = true
				lease, err := localfile.AcquireLease(directory, id)
				if !errors.Is(err, localfile.ErrSessionLocked) {
					_ = lease.Close()
					return fmt.Errorf("lease released before tools shut down: %v", err)
				}
				if oldClose != nil {
					return oldClose()
				}
				return nil
			}
			return configured, nil
		}, nil
	}
	err := Run(t.Context(), []string{"-workspace", workspace, "-session-directory", directory},
		leaseRunnerEnvironment, func() []string { return nil }, strings.NewReader(`{"prompt":"hello"}`), io.Discard, io.Discard, config)
	if err != nil {
		t.Fatal(err)
	}
	if !shutdownChecked {
		t.Fatal("tool shutdown was not checked")
	}
	lease, err := localfile.AcquireLease(directory, id)
	if err != nil {
		t.Fatalf("lease retained after Run returned: %v", err)
	}
	defer lease.Close()
	store, err := localfile.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Resume(t.Context(), id); err != nil {
		t.Fatalf("new-session lease ID differs from created ID: %v", err)
	}
}

func TestRunLeaseCanonicalizesCallerAlias(t *testing.T) {
	// The runner explicitly accepts configured aliases, unlike AcquireLease's
	// no-symlinks API. Both store and lock must use the resolved target.
	for _, missing := range []bool{false, true} {
		t.Run(fmt.Sprintf("missing-suffix-%v", missing), func(t *testing.T) {
			workspace, root := t.TempDir(), runnerLeaseDirectory(t)
			alias := filepath.Join(root, "alias")
			target := filepath.Join(root, "target")
			if err := os.Mkdir(target, 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(target, alias); err != nil {
				t.Fatal(err)
			}
			configured, directory := alias, target
			if missing {
				configured = filepath.Join(alias, "nested", "sessions")
				directory = filepath.Join(target, "nested", "sessions")
			}
			canonical, err := canonicalLeaseDirectory(configured)
			if err != nil || canonical != directory {
				t.Fatalf("canonical path = %q, %v; want %q", canonical, err, directory)
			}
			if missing {
				if _, err := os.Stat(directory); !errors.Is(err, os.ErrNotExist) {
					t.Fatalf("canonicalization created missing directory: %v", err)
				}
			}
			lease, err := localfile.AcquireLease(directory, "canonical")
			if err != nil {
				t.Fatal(err)
			}
			defer lease.Close()
			client := &fakeClient{respond: func(context.Context, llm.Request) (llm.Response, error) {
				return llm.Response{}, errors.New("aliased locked session reached model")
			}}
			err = Run(t.Context(), []string{"-workspace", workspace, "-session-directory", configured},
				leaseRunnerEnvironment, func() []string { return nil },
				strings.NewReader(`{"session_id":"canonical","prompt":"hello"}`), io.Discard, io.Discard, testConfig(client))
			if !errors.Is(err, localfile.ErrSessionLocked) {
				t.Fatalf("Run with alias = %v, want ErrSessionLocked", err)
			}
		})
	}
}
