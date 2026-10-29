// Package testtmp prepares test temporary directories for code that refuses
// symlinked path ancestors.
package testtmp

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// Resolve points TMPDIR at its symlink-free path. On macOS $TMPDIR lies under
// /var -> /private/var, which session leases, ingress, trusted files and remote
// artifacts correctly refuse because they open every component with O_NOFOLLOW.
// Call it first in TestMain; spawned test workers inherit the resolved TMPDIR.
func Resolve() {
	if dir, err := filepath.EvalSymlinks(os.TempDir()); err == nil {
		os.Setenv("TMPDIR", dir)
	}
}

// MkdirTemp creates a private directory, /private/tmp/<prefix><6 hex> on macOS,
// for tests that bind Unix sockets: macOS limits their paths to 103 bytes and
// links /tmp to /private/tmp. The fixed suffix (os.MkdirTemp's is up to 10
// digits) leaves 13 bytes for the prefix plus the socket's parent directory
// name under a 64-hex endpoint name. The caller removes the directory.
func MkdirTemp(prefix string) (string, error) {
	base, err := filepath.EvalSymlinks("/tmp")
	if err != nil {
		return "", err
	}
	for range 100 {
		suffix := make([]byte, 3)
		if _, err := rand.Read(suffix); err != nil {
			return "", err
		}
		dir := filepath.Join(base, prefix+hex.EncodeToString(suffix))
		if err := os.Mkdir(dir, 0o700); !errors.Is(err, fs.ErrExist) {
			return dir, err
		}
	}
	return "", fmt.Errorf("testtmp: no free directory for prefix %q", prefix)
}
