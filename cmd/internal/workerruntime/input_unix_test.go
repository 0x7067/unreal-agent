//go:build linux || darwin

package workerruntime

import (
	"os"
	"testing"
	"time"

	"golang.org/x/sys/unix"
)

func TestInheritedBlockingPipeRestoresCallerDescriptor(t *testing.T) {
	input, writer, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer input.Close()
	defer writer.Close()
	// Fd deliberately emulates an inherited descriptor, not os.Pipe's pollable
	// *File. The duplicated original stays open throughout cancellation.
	fd, err := unix.Dup(int(input.Fd()))
	if err != nil {
		t.Fatal(err)
	}
	original := os.NewFile(uintptr(fd), "inherited-stdin")
	defer original.Close()
	flags, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if err != nil {
		t.Fatal(err)
	}
	if flags&unix.O_NONBLOCK != 0 {
		t.Fatal("fixture is not blocking")
	}
	prepared, cleanup, err := prepareInput(original)
	if err != nil {
		t.Fatal(err)
	}
	pump := newInputPump(prepared)
	// Hand demand directly to the pump, so join must interrupt a read rather
	// than merely stopping an idle actor. No frame is ever written.
	pump.demand <- struct{}{}
	joined := make(chan struct{})
	go func() { pump.join(); close(joined) }()
	select {
	case <-joined:
	case <-time.After(time.Second):
		t.Fatal("inherited reader did not join")
	}
	cleanup()
	restored, err := unix.FcntlInt(uintptr(fd), unix.F_GETFL, 0)
	if err != nil || restored != flags {
		t.Fatalf("flags changed: %x -> %x (%v)", flags, restored, err)
	}
	if _, err := writer.Write([]byte("x")); err != nil {
		t.Fatal(err)
	}
	var b [1]byte
	if _, err := original.Read(b[:]); err != nil || b[0] != 'x' {
		t.Fatalf("caller descriptor not usable: %q %v", b, err)
	}
}
