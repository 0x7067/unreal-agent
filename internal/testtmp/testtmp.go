// Package testtmp prepares the test temporary directory for code that refuses
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
