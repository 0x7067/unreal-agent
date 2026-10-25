package workerproto

// Adversarial frame-boundary coverage for the handoff pipe. The host and the
// worker share Write/Scanner (protocol.go:67-103), so a bound that only one side
// accepts is a real handoff defect: either a legal frame becomes unwritable or a
// legal write becomes unreadable.

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"
)

// byteReader hands out step bytes per Read, modelling arbitrarily fragmented pipe
// reads: frames must be reassembled from whatever chunking the transport produces.
type byteReader struct {
	data []byte
	step int
}

func (r *byteReader) Read(p []byte) (int, error) {
	if len(r.data) == 0 {
		return 0, io.EOF
	}
	n := min(len(p), r.step, len(r.data))
	copy(p, r.data[:n])
	r.data = r.data[n:]
	return n, nil
}

// frameAtLimit builds a Go string whose JSON encoding is exactly limit bytes
// (two framing quotes plus limit-2 payload bytes).
func frameAtLimit(limit int) string {
	return strings.Repeat("x", limit-2)
}

// TestScannerAcceptsExactlyMaxFrameAndRejectsOneMore pins the documented frame
// bound in both directions: Write accepts a frame whose encoded JSON is exactly
// MaxFrame, so Scanner must accept it too, and one byte more must be refused by
// both sides (protocol.go:73-75, protocol.go:101).
func TestScannerAcceptsExactlyMaxFrameAndRejectsOneMore(t *testing.T) {
	atLimit := frameAtLimit(MaxFrame)
	if err := Write(io.Discard, atLimit); err != nil {
		t.Fatalf("Write at the documented limit: %v", err)
	}
	overLimit := "\"" + strings.Repeat("x", MaxFrame-1) + "\""
	if err := Write(io.Discard, overLimit); err == nil {
		t.Fatal("Write accepted a frame above MaxFrame")
	}
	var encoded bytes.Buffer
	if err := Write(&encoded, atLimit); err != nil {
		t.Fatal(err)
	}
	scanner := Scanner(&byteReader{data: encoded.Bytes(), step: 64 << 10})
	if !scanner.Scan() {
		t.Fatalf("Scanner refused a frame Write emitted at the limit: %v", scanner.Err())
	}
	if len(scanner.Bytes()) != MaxFrame {
		t.Fatalf("scanned frame = %d bytes, want %d", len(scanner.Bytes()), MaxFrame)
	}
	if scanner.Scan() {
		t.Fatal("Scanner produced a second frame")
	}
	if scanner.Err() != nil {
		t.Fatalf("Scanner ended with %v", scanner.Err())
	}
	over := Scanner(strings.NewReader(overLimit + "\n"))
	if over.Scan() || over.Err() == nil {
		t.Fatalf("Scanner accepted a frame above MaxFrame: %v", over.Err())
	}
}

// TestScannerReassemblesEveryChunking feeds one byte stream through reads of 1, 3,
// 7 and whole-buffer sizes and requires identical, ordered frames. A short read
// must never manufacture a frame boundary (protocol.go:91-103).
func TestScannerReassemblesEveryChunking(t *testing.T) {
	var stream bytes.Buffer
	want := []string{"hello", "item", "ready"}
	for _, typ := range want {
		if err := Write(&stream, Event{Type: typ}); err != nil {
			t.Fatal(err)
		}
	}
	encoded := append([]byte(nil), stream.Bytes()...)
	for _, step := range []int{1, 3, 7, len(encoded)} {
		t.Run(fmt.Sprintf("step-%d", step), func(t *testing.T) {
			scanner := Scanner(&byteReader{data: append([]byte(nil), encoded...), step: step})
			var got []string
			for scanner.Scan() {
				var event Event
				if err := json.Unmarshal(scanner.Bytes(), &event, json.RejectUnknownMembers(true)); err != nil {
					t.Fatal(err)
				}
				got = append(got, event.Type)
			}
			if err := scanner.Err(); err != nil {
				t.Fatal(err)
			}
			if strings.Join(got, ",") != strings.Join(want, ",") {
				t.Fatalf("frames = %v, want %v", got, want)
			}
		})
	}
}

// TestScannerNeverDeliversPartialWriteAsFrame covers the io.Pipe transport the
// host actually uses: a frame split across two writes is delivered once, whole,
// and only after its newline arrives (protocol.go:93-100).
func TestScannerNeverDeliversPartialWriteAsFrame(t *testing.T) {
	reader, writer := io.Pipe()
	// Bound both writes and reads even if a broken scanner stops consuming.
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	stop := context.AfterFunc(ctx, func() {
		_ = reader.CloseWithError(ctx.Err())
		_ = writer.CloseWithError(ctx.Err())
	})
	defer stop()
	defer reader.Close()
	defer writer.Close()
	scanner := Scanner(reader)
	type scanResult struct {
		frame string
		err   error
		more  bool
	}
	delivered := make(chan scanResult, 2)
	go func() {
		for scanner.Scan() {
			delivered <- scanResult{frame: string(scanner.Bytes()), more: true}
		}
		delivered <- scanResult{err: scanner.Err()}
	}()
	frame := "{\"Type\":\"ready\"}"
	// io.Pipe writes only complete once the scanner has consumed them, so this is
	// a rendezvous, not a sleep: the scanner really holds the unterminated prefix.
	if _, err := writer.Write([]byte(frame[:7])); err != nil {
		t.Fatal(err)
	}
	select {
	case result := <-delivered:
		t.Fatalf("partial write delivered before its newline: %#v", result)
	default:
	}
	if _, err := writer.Write([]byte(frame[7:] + "\n")); err != nil {
		t.Fatal(err)
	}
	receive := func() scanResult {
		t.Helper()
		select {
		case result := <-delivered:
			return result
		case <-ctx.Done():
			t.Fatalf("timed out waiting for scanner: %v", ctx.Err())
			return scanResult{}
		}
	}
	result := receive()
	if !result.more || result.err != nil || result.frame != frame {
		t.Fatalf("frame = %q, want %q", result.frame, frame)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	if end := receive(); end.more || end.err != nil {
		t.Fatalf("scanner end = %#v", end)
	}
}
