package attachproto

import (
	"fmt"
	"os"
)

// authorizePeerUID applies the same policy to kernel credentials on both
// supported transports. Wire fields never supply this identity.
func authorizePeerUID(uid uint32) error {
	if uid != uint32(os.Geteuid()) {
		return fmt.Errorf("Unix peer effective UID %d differs from ours", uid)
	}
	return nil
}
