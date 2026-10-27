package workerproto

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

type shortWriter struct{ bytes.Buffer }

func (w *shortWriter) Write(data []byte) (int, error) {
	if len(data) > 3 {
		data = data[:3]
	}
	return w.Buffer.Write(data)
}

type zeroWriter struct{}

func (zeroWriter) Write([]byte) (int, error) { return 0, nil }
func TestWriteHandlesShortWritesAndScannerRejectsIncompleteFrames(t *testing.T) {
	var out shortWriter
	if err := Write(&out, Event{Type: "ready"}); err != nil {
		t.Fatal(err)
	}
	scanner := Scanner(bytes.NewReader(out.Bytes()))
	if !scanner.Scan() || !bytes.Contains(scanner.Bytes(), []byte(`"ready"`)) {
		t.Fatalf("missing frame: %v", scanner.Err())
	}
	if scanner.Scan() || scanner.Err() != nil {
		t.Fatal(scanner.Err())
	}
	truncated := Scanner(bytes.NewReader(bytes.TrimSuffix(out.Bytes(), []byte("\n"))))
	if truncated.Scan() || !errors.Is(truncated.Err(), io.ErrUnexpectedEOF) {
		t.Fatalf("accepted unterminated frame: %v", truncated.Err())
	}
	if err := Write(zeroWriter{}, Event{Type: "ready"}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("zero write: %v", err)
	}
}
func TestFrameBoundsAndBlankFrameVisibility(t *testing.T) {
	if err := Write(io.Discard, strings.Repeat("x", MaxFrame)); err == nil {
		t.Fatal("oversize write accepted")
	}
	scanner := Scanner(strings.NewReader(strings.Repeat("x", MaxFrame+2) + "\n"))
	if scanner.Scan() || scanner.Err() == nil {
		t.Fatal("oversize read accepted")
	}
	blank := Scanner(strings.NewReader("\n"))
	if !blank.Scan() || len(blank.Bytes()) != 0 {
		t.Fatal("blank frame hidden from protocol consumer")
	}
}

func TestVersionTwoRejectsOldPrivateWorkersNotOldSessions(t *testing.T) {
	if Version != 2 || SessionVersion != 2 {
		t.Fatalf("unexpected versions %d/%d", Version, SessionVersion)
	}
	if err := (Identity{Protocol: 1, SessionVersion: SessionVersion, Build: "old"}).Validate(); err == nil || !strings.Contains(err.Error(), "incompatible worker") {
		t.Fatalf("old worker not explicitly rejected: %v", err)
	}
	if err := (Identity{Protocol: Version, SessionVersion: 2, Build: "new"}).Validate(); err != nil {
		t.Fatal(err)
	}
}
