package attachproto

import (
	"os"
	"testing"
)

func TestKernelPeerUIDPolicy(t *testing.T) {
	own := uint32(os.Geteuid())
	if err := authorizePeerUID(own); err != nil {
		t.Fatal("same effective UID rejected", err)
	}
	for _, uid := range []uint32{own + 1, own - 1, ^uint32(0), 0} {
		if uid == own {
			continue
		}
		if err := authorizePeerUID(uid); err == nil {
			t.Fatalf("foreign kernel UID %d authorized as %d", uid, own)
		}
	}
	if err := CheckPeer(nil); err == nil {
		t.Fatal("missing kernel peer accepted")
	}
}
