package main

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestReviewCommandPrefersSiblingAndPassesExitCode(t *testing.T) {
	siblingDir := t.TempDir()
	pathDir := t.TempDir()
	writeReviewScript(t, siblingDir, "#!/bin/sh\nprintf 'sibling:%s\\n' \"$*\"\nprintf 'sibling-err:%s\\n' \"$*\" >&2\nexit 4\n")
	pathBin := writeReviewScript(t, pathDir, "#!/bin/sh\nprintf 'path\\n'\nexit 0\n")

	var stdout, stderr bytes.Buffer
	code := reviewCommand([]string{"help", "--model", "m"}, strings.NewReader(""), &stdout, &stderr, []string{},
		func() (string, error) { return filepath.Join(siblingDir, "unreal-agent-runner"), nil },
		func(name string) (string, error) {
			if name != reviewBinary {
				t.Fatalf("lookPath %q", name)
			}
			return pathBin, nil
		})
	if code != 4 {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
	if stdout.String() != "sibling:help --model m\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
	if stderr.String() != "sibling-err:help --model m\n" {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func TestReviewCommandUsesPathAndPassesSuccess(t *testing.T) {
	pathDir := t.TempDir()
	pathBin := writeReviewScript(t, pathDir, "#!/bin/sh\nprintf 'path:%s\\n' \"$*\"\nexit 0\n")
	empty := t.TempDir()

	var stdout, stderr bytes.Buffer
	code := reviewCommand([]string{"render", "markdown", "-h"}, nil, &stdout, &stderr, []string{},
		func() (string, error) { return filepath.Join(empty, "unreal-agent-runner"), nil },
		func(string) (string, error) { return pathBin, nil })
	if code != 0 || stderr.Len() != 0 {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
	if stdout.String() != "path:render markdown -h\n" {
		t.Fatalf("stdout = %q", stdout.String())
	}
}

func TestReviewCommandSkipsUnusableSibling(t *testing.T) {
	dir := t.TempDir()
	plain := filepath.Join(dir, reviewBinary)
	if err := os.WriteFile(plain, []byte("#!/bin/sh\nexit 9\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	pathBin := writeReviewScript(t, t.TempDir(), "#!/bin/sh\nprintf 'path\\n'\nexit 0\n")

	var stdout, stderr bytes.Buffer
	code := reviewCommand(nil, nil, &stdout, &stderr, []string{},
		func() (string, error) { return filepath.Join(dir, "unreal-agent-runner"), nil },
		func(string) (string, error) { return pathBin, nil })
	if code != 0 || stdout.String() != "path\n" || stderr.Len() != 0 {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
}

func TestReviewCommandMissingBinary(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code := reviewCommand([]string{"help"}, nil, &stdout, &stderr, nil,
		func() (string, error) { return "", errors.New("no executable") },
		func(string) (string, error) { return "", exec.ErrNotFound })
	if code != 1 || stdout.Len() != 0 {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
	for _, want := range []string{
		"unreal-agent-runner: unreal-review not found next to unreal-agent-runner or on PATH",
		"Usage: unreal-agent-runner review [args...]",
		`Run "unreal-agent-runner review help"`,
	} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr missing %q:\n%s", want, stderr.String())
		}
	}
}

func TestReviewCommandExecFailure(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, reviewBinary)
	if err := os.WriteFile(bin, []byte("not a program"), 0o755); err != nil {
		t.Fatal(err)
	}
	var stdout, stderr bytes.Buffer
	code := reviewCommand([]string{"run"}, nil, &stdout, &stderr, []string{},
		func() (string, error) { return filepath.Join(dir, "unreal-agent-runner"), nil },
		func(string) (string, error) { return "", exec.ErrNotFound })
	if code != 1 || stdout.Len() != 0 {
		t.Fatalf("exit = %d, stdout = %q, stderr = %q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stderr.String(), "unreal-agent-runner:") || strings.Contains(stderr.String(), "not found") {
		t.Fatalf("stderr = %q", stderr.String())
	}
}

func writeReviewScript(t *testing.T, dir, body string) string {
	t.Helper()
	path := filepath.Join(dir, reviewBinary)
	if err := os.WriteFile(path, []byte(body), 0o755); err != nil {
		t.Fatal(err)
	}
	return path
}
