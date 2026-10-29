// Package testtmp prepares test temporary directories for code that refuses
// symlinked path ancestors.
package testtmp

import (
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

// MkdirTemp creates a short directory under the symlink-free /tmp for tests
// that bind Unix sockets: macOS limits their paths to 103 bytes and links /tmp
// to /private/tmp. Keep prefixes short; the caller removes the directory.
func MkdirTemp(prefix string) (string, error) {
	base, err := filepath.EvalSymlinks("/tmp")
	if err != nil {
		return "", err
	}
	return os.MkdirTemp(base, prefix)
}
