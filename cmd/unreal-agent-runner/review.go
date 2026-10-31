package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const reviewBinary = "unreal-review"

const reviewUsage = `Usage: unreal-agent-runner review [args...]
Runs unreal-review with the remaining arguments and passes its exit code through.
unreal-review is the binary next to unreal-agent-runner, or unreal-review on PATH.
Run "unreal-agent-runner review help" for unreal-review's usage.
`

// reviewCommand runs unreal-review. The review CLI is package main and
// re-execs os.Executable to drop credentials from the shell environment, so
// calling it in-process would re-exec the runner. This looks beside the
// runner first, then on PATH.
func reviewCommand(args []string, stdin io.Reader, stdout, stderr io.Writer, env []string, executable func() (string, error), lookPath func(string) (string, error)) int {
	bin, err := findUnrealReview(executable, lookPath)
	if err != nil {
		fmt.Fprintf(stderr, "unreal-agent-runner: %s\n%s", err, reviewUsage)
		return 1
	}
	code, err := runReviewProcess(bin, args, stdin, stdout, stderr, env)
	if err != nil {
		fmt.Fprintf(stderr, "unreal-agent-runner: %v\n", err)
		return 1
	}
	return code
}

func findUnrealReview(executable func() (string, error), lookPath func(string) (string, error)) (string, error) {
	if executable != nil {
		if exe, err := executable(); err == nil {
			candidate := filepath.Join(filepath.Dir(exe), reviewBinary)
			if executableFile(candidate) {
				return candidate, nil
			}
		}
	}
	if lookPath != nil {
		if path, err := lookPath(reviewBinary); err == nil && executableFile(path) {
			return path, nil
		}
	}
	return "", errors.New("unreal-review not found next to unreal-agent-runner or on PATH")
}

func executableFile(path string) bool {
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0o111 != 0
}

func runReviewProcess(bin string, args []string, stdin io.Reader, stdout, stderr io.Writer, env []string) (int, error) {
	cmd := exec.Command(bin, args...)
	cmd.Stdin = stdin
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if env != nil {
		cmd.Env = env
	}
	err := cmd.Run()
	if err == nil {
		return 0, nil
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return 0, err
	}
	if code := exitErr.ExitCode(); code >= 0 {
		return code, nil
	}
	if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
		return 128 + int(status.Signal()), nil
	}
	return 1, nil
}
