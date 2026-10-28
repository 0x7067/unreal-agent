package attachproto

import (
	"crypto/sha256"
	"fmt"
	"net"
	"path/filepath"
	"strings"
	"sync"

	"github.com/unreallabsai/unreal-agent/harness/session"
)

// Endpoint owns a listener and a separate, persistent endpoint lock. The caller
// must already hold workerhost ownership before calling Listen. Do not copy an
// Endpoint after use, or remove its lock file (including after Close).
type Endpoint struct {
	Listener *net.UnixListener
	Path     string

	once    sync.Once
	closeFn func() error
	err     error
}

// Close stops the listener before inode-checked cleanup, and releases endpoint
// ownership last. Concurrent and repeated calls are safe.
func (endpoint *Endpoint) Close() error {
	if endpoint == nil {
		return nil
	}
	endpoint.once.Do(func() {
		if endpoint.closeFn != nil {
			endpoint.err = endpoint.closeFn()
		} else if endpoint.Listener != nil {
			endpoint.err = endpoint.Listener.Close()
		}
	})
	return endpoint.err
}

func endpointPath(runtimeDir, stateDir string, id session.ID) (string, error) {
	if !filepath.IsAbs(runtimeDir) || filepath.Clean(runtimeDir) != runtimeDir {
		return "", fmt.Errorf("runtime directory must be an explicit clean absolute path")
	}
	// Session IDs are namespace components, not paths. Match the store's portable
	// filename subset, without silently normalizing unsafe or empty identifiers.
	if id == "" || id == "." || id == ".." || len(id) > 255 {
		return "", fmt.Errorf("invalid session ID")
	}
	for _, c := range string(id) {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-' || c == '_' || c == '.') {
			return "", fmt.Errorf("invalid session ID")
		}
	}
	if strings.TrimSpace(stateDir) == "" {
		return "", fmt.Errorf("state directory is empty")
	}
	absolute, err := filepath.Abs(stateDir)
	if err != nil {
		return "", fmt.Errorf("resolve state directory: %w", err)
	}
	canonical, err := filepath.EvalSymlinks(absolute)
	if err != nil {
		return "", fmt.Errorf("canonicalize state directory: %w", err)
	}
	digest := sha256.Sum256([]byte(canonical + "\x00" + string(id)))
	path := filepath.Join(runtimeDir, fmt.Sprintf("%x.sock", digest))
	// Darwin has the smaller sockaddr_un budget. Keep a platform-independent
	// limit with room for the terminating NUL on all supported platforms.
	if len(path) > 103 {
		return "", fmt.Errorf("Unix endpoint path exceeds 103 bytes")
	}
	return path, nil
}
