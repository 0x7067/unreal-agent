package localfile

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/unreallabsai/unreal-agent/harness/session"
)

// ErrSessionLocked means another owner holds the exclusive session lease.
var ErrSessionLocked = errors.New("session is locked")

const sessionLeaseSuffix = ".session.lock"

// Lease is exclusive ownership of a session across processes. Callers must
// acquire it before reading/replaying or creating the session and keep it until
// all session writers and operations have shut down. The store itself remains a
// single-writer store; its methods do not acquire leases automatically.
//
// The lock file is deliberately persistent: removing it, including after Close,
// would allow two owners to lock different inodes for the same session.
// A Lease must not be copied after first use.
type Lease struct {
	file *os.File
	once sync.Once
	err  error
}

// AcquireLease acquires a nonblocking, process-exclusive lease for id. The
// directory must be owned by the current user and effectively private: it or
// an ancestor must be current-user-owned with no group/world permissions. A
// nonprivate leaf beneath that ancestor is allowed without changing its mode.
// Before that ancestor, traversal requires root/current-user-owned directories
// without group/world write, except sticky directories with current-user-owned
// children (e.g. /tmp). This prevents other users from renaming the lock's path.
// Callers must keep these directory permissions/ownership stable while leased;
// other processes running as the same user are trusted not to replace the lock.
// The lock itself must be private and current-user-owned. Symlink directory
// components and symlink/hardlink lock files are rejected: callers wanting to
// trust aliases must explicitly resolve them first; checks then cover only the
// resolved path, not the alias. Lock files
// are separate from session history and must never be unlinked by callers.
// The operating system releases ownership when the process exits, even on crash.
// Platforms without a secure implementation fail closed.
func AcquireLease(directory string, id session.ID) (*Lease, error) {
	if err := validateSessionID(id); err != nil {
		return nil, err
	}
	if strings.TrimSpace(directory) == "" {
		return nil, errors.New("session lease directory is empty")
	}
	absolute, err := filepath.Abs(directory)
	if err != nil {
		return nil, fmt.Errorf("resolve session lease directory: %w", err)
	}
	file, err := acquireLeaseFile(absolute, string(id)+sessionLeaseSuffix)
	if err != nil {
		return nil, fmt.Errorf("acquire session %q lease: %w", id, err)
	}
	return &Lease{file: file}, nil
}

// Close releases ownership without removing or replacing the lock file. It is
// safe to call repeatedly or concurrently. A nil or zero Lease is already closed.
func (lease *Lease) Close() error {
	if lease == nil {
		return nil
	}
	lease.once.Do(func() {
		if lease.file != nil {
			lease.err = lease.file.Close()
		}
	})
	return lease.err
}
