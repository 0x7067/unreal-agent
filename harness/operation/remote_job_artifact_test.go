//go:build darwin || linux || freebsd || openbsd || netbsd || dragonfly

package operation_test

import (
	"context"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/operation"
)

func artifactRemote(t *testing.T, directory string) operation.Operation {
	t.Helper()
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: "test", Version: 1, Data: []byte(`{}`)})
	if err != nil {
		t.Fatal(err)
	}
	var state operation.RemoteJobState
	if err := json.Unmarshal(spec.State, &state); err != nil {
		t.Fatal(err)
	}
	state.OutputDirectory = directory
	spec.State, err = json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	return operation.Operation{ID: "remote-job", Type: spec.Type, Version: spec.Version, State: spec.State, Status: operation.StatusReady, MaxOutputLength: 12}
}
func artifactState(t *testing.T, op operation.Operation) operation.RemoteJobState {
	t.Helper()
	s, e := operation.DecodeRemoteJobState(op)
	if e != nil {
		t.Fatal(e)
	}
	return s
}
func artifactUpdate(t *testing.T, op operation.Operation, s operation.RemoteJobState) operation.Operation {
	t.Helper()
	step, e := operation.UpdateRemoteJob(op, s, operation.StatusCompleted)
	if e != nil {
		t.Fatal(e)
	}
	return *step.Operation
}

func TestRemoteArtifactsRecoveryAndReplay(t *testing.T) {
	for _, field := range []string{"result", "error"} {
		t.Run(field, func(t *testing.T) {
			dir := t.TempDir()
			op := artifactRemote(t, dir)
			s := artifactState(t, op)
			full := strings.Repeat("é full output\n", 100)
			if field == "result" {
				s.TerminalResult = full
			} else {
				s.TerminalError = full
			}
			op = artifactUpdate(t, op, s)
			s = artifactState(t, op)
			path, preview, size := s.ResultPath, s.TerminalResult, s.ResultBytes
			if field == "error" {
				path, preview, size = s.ErrorPath, s.TerminalError, s.ErrorBytes
			}
			if !strings.Contains(preview, "complete output in "+path) || path == "" || size != len(full) {
				t.Fatalf("missing artifact: %#v", s)
			}
			b, e := os.ReadFile(path)
			if e != nil || string(b) != full {
				t.Fatalf("full recovery: %v %q", e, b)
			}
			fi, e := os.Stat(path)
			if e != nil || fi.Mode().Perm() != 0o600 {
				t.Fatalf("file permissions: %v %v", fi, e)
			}
			di, e := os.Stat(filepath.Dir(path))
			if e != nil || di.Mode().Perm() != 0o700 {
				t.Fatalf("directory permissions: %v %v", di, e)
			}
			for range 3 {
				encoded, e := json.Marshal(op)
				if e != nil {
					t.Fatal(e)
				}
				if e = json.Unmarshal(encoded, &op); e != nil {
					t.Fatal(e)
				}
				op = artifactUpdate(t, op, artifactState(t, op))
			}
			b, e = os.ReadFile(path)
			if e != nil || string(b) != full {
				t.Fatalf("replay overwrote artifact: %v", e)
			}
			next := artifactState(t, op)
			if next.ResultPath != s.ResultPath || next.ErrorPath != s.ErrorPath || next.TerminalResult != s.TerminalResult || next.TerminalError != s.TerminalError {
				t.Fatal("replay changed checkpoint")
			}
			// New small payloads clear stale paths without creating extra files.
			next.TerminalResult = "small"
			next.TerminalError = ""
			next.ResultTruncated = false
			next.ErrorTruncated = false
			op = artifactUpdate(t, op, next)
			next = artifactState(t, op)
			if next.ResultPath != "" || next.ErrorPath != "" || next.TerminalResult != "small" {
				t.Fatalf("stale artifact: %#v", next)
			}
			// A new oversized full payload gets its own immutable content-addressed file.
			next.TerminalResult = full + "new"
			op = artifactUpdate(t, op, next)
			next = artifactState(t, op)
			if next.ResultPath == "" || next.ResultPath == path {
				t.Fatal("new payload did not get new artifact")
			}
			b, e = os.ReadFile(path)
			if e != nil || string(b) != full {
				t.Fatal("new payload clobbered old artifact")
			}
		})
	}
}

func TestRemoteArtifactsSmallAndLegacy(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not-created")
	op := artifactRemote(t, dir)
	s := artifactState(t, op)
	s.TerminalResult = "small"
	s.TerminalError = "é"
	op = artifactUpdate(t, op, s)
	s = artifactState(t, op)
	if s.TerminalResult != "small" || s.TerminalError != "é" || s.ResultPath != "" || s.ErrorPath != "" || s.ResultTruncated || s.ErrorTruncated {
		t.Fatalf("small output changed: %#v", s)
	}
	if _, e := os.Stat(dir); !os.IsNotExist(e) {
		t.Fatalf("small output created files: %v", e)
	}
	s.TerminalResult = "old preview"
	s.ResultTruncated = true
	s.ResultBytes = 1000
	op = artifactUpdate(t, op, s)
	s = artifactState(t, op)
	if s.ResultPath != "" || !strings.Contains(s.TerminalResult, "original output not retained") {
		t.Fatalf("old preview advertised as full: %#v", s)
	}
	if _, e := os.Stat(dir); !os.IsNotExist(e) {
		t.Fatal("archived already truncated output")
	}
}

func TestRemoteArtifactsPathFailures(t *testing.T) {
	for _, name := range []string{"invalid-id", "dot-id", "long-id", "ancestor-symlink", "directory-symlink", "file-symlink", "file-hardlink", "private-directory", "file-as-directory", "file-collision"} {
		t.Run(name, func(t *testing.T) {
			base := t.TempDir()
			dir := filepath.Join(base, "outputs")
			if e := os.Mkdir(dir, 0o700); e != nil {
				t.Fatal(e)
			}
			full := strings.Repeat("full", 100)
			op := artifactRemote(t, dir)
			s := artifactState(t, op)
			s.TerminalResult = full
			switch name {
			case "invalid-id":
				op.ID = "../escape"
			case "dot-id":
				op.ID = ".."
			case "long-id":
				op.ID = operation.ID(strings.Repeat("x", 129))
			case "ancestor-symlink":
				link := filepath.Join(base, "link")
				if e := os.Symlink(dir, link); e != nil {
					t.Fatal(e)
				}
				op = artifactRemote(t, link)
				s = artifactState(t, op)
				s.TerminalResult = full
			case "directory-symlink":
				if e := os.Symlink(base, filepath.Join(dir, "remote-outputs")); e != nil {
					t.Fatal(e)
				}
			case "private-directory":
				if e := os.Mkdir(filepath.Join(dir, "remote-outputs"), 0o755); e != nil {
					t.Fatal(e)
				}
				// Establish the unsafe mode even under a restrictive process umask.
				if e := os.Chmod(filepath.Join(dir, "remote-outputs"), 0o755); e != nil {
					t.Fatal(e)
				}
			case "file-as-directory":
				if e := os.WriteFile(filepath.Join(dir, "remote-outputs"), []byte("sentinel"), 0o600); e != nil {
					t.Fatal(e)
				}
			default:
				first := artifactState(t, artifactUpdate(t, op, s))
				path := first.ResultPath
				if path == "" {
					t.Fatal("setup failed")
				}
				if e := os.Remove(path); e != nil {
					t.Fatal(e)
				}
				target := filepath.Join(base, "target")
				if e := os.WriteFile(target, []byte("sentinel"), 0o600); e != nil {
					t.Fatal(e)
				}
				var e error
				if name == "file-symlink" {
					e = os.Symlink(target, path)
				} else if name == "file-hardlink" {
					e = os.Link(target, path)
				} else {
					e = os.WriteFile(path, []byte("sentinel"), 0o600)
				}
				if e != nil {
					t.Fatal(e)
				}
				defer func() {
					b, e := os.ReadFile(target)
					if e != nil || string(b) != "sentinel" {
						t.Fatal("clobbered target")
					}
				}()
			}
			s = artifactState(t, artifactUpdate(t, op, s))
			if s.ResultPath != "" || !strings.Contains(s.TerminalResult, "complete output unavailable") || len(s.TerminalResult) > 250 {
				t.Fatalf("unsafe failure preview: %#v", s)
			}
		})
	}
}

func TestRemoteArtifactsManagerConfigurationBeforeDispatch(t *testing.T) {
	for _, configured := range []bool{false, true} {
		t.Run(map[bool]string{false: "legacy", true: "session-default"}[configured], func(t *testing.T) {
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			h := newTestRemoteJobHandler()
			var m *operation.LocalOperationManager
			dir := t.TempDir()
			if configured {
				m = operation.NewLocalOperationManagerWithOptions(ctx, operation.LocalOperationManagerOptions{OutputDirectory: dir}, h)
			} else {
				m = operation.NewLocalOperationManager(ctx, h)
			}
			op := newTestRemoteJobOperation(t, "manager-artifact")
			op.MaxOutputLength = 10
			if e := m.Add(op); e != nil {
				t.Fatal(e)
			}
			op = <-h.adds
			s := artifactState(t, op)
			if configured && s.OutputDirectory != dir {
				t.Fatalf("configuration missing before handler: %#v", s)
			}
			full := strings.Repeat("test", 100)
			s.TerminalResult = full
			h.updates <- artifactUpdate(t, op, s)
			s = artifactState(t, receiveRemoteJobUpdate(t, m.Updates()))
			if configured {
				b, e := os.ReadFile(s.ResultPath)
				if e != nil || string(b) != full {
					t.Fatalf("manager recovery: %v", e)
				}
			} else if s.ResultPath != "" {
				t.Fatal("legacy constructor wrote artifact")
			}
		})
	}
}

func TestRemoteArtifactReplayFromOriginalIsExclusive(t *testing.T) {
	op := artifactRemote(t, t.TempDir())
	s := artifactState(t, op)
	s.TerminalResult = strings.Repeat("full replay", 100)
	first := artifactState(t, artifactUpdate(t, op, s))
	before, e := os.Stat(first.ResultPath)
	if e != nil {
		t.Fatal(e)
	}
	second := artifactState(t, artifactUpdate(t, op, s))
	after, e := os.Stat(second.ResultPath)
	if e != nil {
		t.Fatal(e)
	}
	if first.ResultPath != second.ResultPath || !os.SameFile(before, after) || !before.ModTime().Equal(after.ModTime()) {
		t.Fatal("idempotent payload replaced an existing artifact")
	}
	// Changed text with an inherited truncation flag is not proof of a full
	// original. Drop stale paths instead of capturing a potentially lossy value.
	checkpoint := artifactUpdate(t, op, s)
	next := artifactState(t, checkpoint)
	next.TerminalResult = "different preview"
	next.ResultTruncated = true
	next = artifactState(t, artifactUpdate(t, checkpoint, next))
	if next.ResultPath != "" || !strings.Contains(next.TerminalResult, "original output not retained") {
		t.Fatalf("advertised stale or lossy output: %#v", next)
	}
}

func TestRemoteArtifactManagerRejectsTraversalBeforeDispatch(t *testing.T) {
	h := newTestRemoteJobHandler()
	m := operation.NewLocalOperationManagerWithOptions(t.Context(), operation.LocalOperationManagerOptions{OutputDirectory: t.TempDir()}, h)
	op := newTestRemoteJobOperation(t, "../escape")
	if e := m.Add(op); e != nil {
		t.Fatal(e)
	}
	failed := receiveRemoteJobUpdate(t, m.Updates())
	if failed.Status != operation.StatusFailed {
		t.Fatalf("invalid ID not rejected: %#v", failed)
	}
	select {
	case <-h.adds:
		t.Fatal("invalid ID dispatched to handler")
	default:
	}
}
