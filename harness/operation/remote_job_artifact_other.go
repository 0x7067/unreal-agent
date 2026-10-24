//go:build !darwin && !linux && !freebsd && !openbsd && !netbsd && !dragonfly

package operation

import "errors"

// Fail closed on platforms without this descriptor-relative no-follow writer.
// Inline previews explicitly report that the complete output is unavailable.
func saveRemoteJobArtifact(directory, name, text string) error {
	return errors.New("secure remote job artifacts are unsupported on this platform")
}
