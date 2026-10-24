package workerhost

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
)

type staged struct {
	command  []string
	identity workerproto.Identity
	digest   string
}

// stage copies the explicitly selected executable to a private immutable path.
// Rebuilding an installed/local path cannot change an already staged generation.
func (h *Host) stage(ctx context.Context, command []string) (result staged, err error) {
	if len(command) == 0 || command[0] == "" {
		return result, errors.New("worker executable is required")
	}
	if !filepath.IsAbs(command[0]) {
		return result, errors.New("worker executable must be an absolute trusted path")
	}
	resolved, err := filepath.EvalSymlinks(command[0])
	if err != nil {
		return result, err
	}
	source, err := os.Open(resolved)
	if err != nil {
		return result, err
	}
	defer source.Close()
	before, err := source.Stat()
	if err != nil {
		return result, err
	}
	if !before.Mode().IsRegular() || before.Mode().Perm()&0111 == 0 {
		return result, errors.New("worker candidate is not a regular executable")
	}
	directory := filepath.Join(h.config.Bootstrap.Directory, "workers", string(h.config.Bootstrap.SessionID))
	// The session directory is trusted, but do not follow a pre-existing stage symlink.
	if err = privateDirectory(directory); err != nil {
		return result, err
	}
	temporary, err := os.CreateTemp(directory, "candidate-*")
	if err != nil {
		return result, err
	}
	name := temporary.Name()
	keep := false
	defer func() {
		if temporary != nil {
			err = errors.Join(err, temporary.Close())
		}
		if !keep {
			_ = os.Remove(name)
		}
	}()
	digest := sha256.New()
	if _, err = io.Copy(io.MultiWriter(temporary, digest), source); err != nil {
		return result, err
	}
	after, err := source.Stat()
	if err != nil {
		return result, err
	}
	if after.Size() != before.Size() || !after.ModTime().Equal(before.ModTime()) {
		return result, errors.New("worker candidate changed while staging; retry after the build finishes")
	}
	if err = temporary.Chmod(0500); err != nil {
		return result, err
	}
	if err = temporary.Sync(); err != nil {
		return result, err
	}
	// Linux refuses exec while any writable descriptor to the inode is open.
	if err = temporary.Close(); err != nil {
		return result, err
	}
	temporary = nil
	result.digest = hex.EncodeToString(digest.Sum(nil))
	// Keep the unique inode: no running candidate is overwritten or implicitly selected.
	result.command = append([]string{name}, command[1:]...)
	// The old worker stays alive throughout this read-only capability probe.
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	stopHostCancel := context.AfterFunc(h.ctx, cancel)
	defer stopHostCancel()
	args := append([]string(nil), command[1:]...)
	if len(args) == 0 || args[len(args)-1] != "worker" {
		return result, errors.New("worker command must end in worker")
	}
	args[len(args)-1] = "worker-probe"
	probe := exec.CommandContext(probeCtx, name, args...)
	probe.Stderr = h.config.Stderr
	isolateProcess(probe)
	probe.Cancel = func() error { return killProcessGroup(probe) }
	probe.WaitDelay = time.Second
	output, err := probe.StdoutPipe()
	if err != nil {
		return result, err
	}
	defer output.Close()
	if err = probe.Start(); err != nil {
		return result, err
	}
	defer func() { _ = killProcessGroup(probe) }()
	stopRead := context.AfterFunc(probeCtx, func() { _ = output.Close() })
	defer stopRead()
	scanner := workerproto.Scanner(output)
	var event workerproto.Event
	if !scanner.Scan() {
		_ = probe.Process.Kill()
		_ = probe.Wait()
		return result, fmt.Errorf("worker probe returned no hello: %w", errors.Join(scanner.Err(), io.ErrUnexpectedEOF))
	}
	if err = json.Unmarshal(scanner.Bytes(), &event, json.RejectUnknownMembers(true)); err != nil {
		_ = probe.Process.Kill()
		_ = probe.Wait()
		return result, fmt.Errorf("decode worker probe: %w", err)
	}
	if event.Type != "hello" || event.Identity == nil {
		_ = probe.Process.Kill()
		_ = probe.Wait()
		return result, errors.New("worker probe did not identify protocol")
	}
	if err = event.Identity.Validate(); err != nil {
		_ = probe.Process.Kill()
		_ = probe.Wait()
		return result, err
	}
	if scanner.Scan() {
		_ = probe.Process.Kill()
		_ = probe.Wait()
		return result, errors.New("worker probe emitted unexpected extra frame")
	}
	if err = scanner.Err(); err != nil {
		_ = probe.Process.Kill()
		_ = probe.Wait()
		return result, err
	}
	if err = probe.Wait(); err != nil {
		return result, fmt.Errorf("worker probe failed: %w", err)
	}
	result.identity = *event.Identity
	// Cancel deferred deletion only after complete successful probing/close.
	dir, openErr := os.Open(directory)
	if openErr != nil {
		return result, openErr
	}
	syncErr := dir.Sync()
	closeErr := dir.Close()
	if err = errors.Join(syncErr, closeErr); err != nil {
		return result, err
	}
	keep = true
	h.mu.Lock()
	h.stages = append(h.stages, name)
	h.mu.Unlock()
	return result, nil
}

func privateDirectory(path string) error {
	// Reject symlinks in each existing directory component. The app's private
	// state root must not be swapped by another user during operation.
	path = filepath.Clean(path)
	parent := filepath.Dir(path)
	if parent != path {
		if err := privateDirectory(parent); err != nil {
			return err
		}
	}
	info, err := os.Lstat(path)
	if errors.Is(err, os.ErrNotExist) {
		return os.Mkdir(path, 0700)
	}
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return fmt.Errorf("worker directory is not a real directory: %s", path)
	}
	return nil
}
