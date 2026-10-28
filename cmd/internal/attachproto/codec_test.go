package attachproto

import (
	"bytes"
	"errors"
	"io"
	"math"
	"strings"
	"testing"
)

func TestCodecDeterministicAndExactUint64(t *testing.T) {
	a, err := Encode(map[string]any{"z": uint64(math.MaxUint64), "a": "first"})
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != "{\"a\":\"first\",\"z\":18446744073709551615}\n" {
		t.Fatalf("unexpected encoding %s", a)
	}
	for i := 0; i < 20; i++ {
		b, err := Encode(map[string]any{"a": "first", "z": uint64(math.MaxUint64)})
		if err != nil || !bytes.Equal(a, b) {
			t.Fatalf("not deterministic: %s %v", b, err)
		}
	}
	frame, err := Encode(Event{Type: "snapshot", Revision: math.MaxUint64})
	if err != nil {
		t.Fatal(err)
	}
	event, err := ReadEvent(Scanner(bytes.NewReader(frame)))
	if err != nil || event.Revision != math.MaxUint64 {
		t.Fatalf("precision lost: %+v %v", event, err)
	}
}

func TestCodecRejectsInvalidFrames(t *testing.T) {
	for name, frame := range map[string]string{
		"unknown":        "{\"Type\":\"hello\",\"Unknown\":true}\n",
		"incompleteJSON": "{\"Type\":\n",
		"incompleteLine": "{\"Type\":\"hello\"}",
		"trailingJSON":   "{}{}\n",
		"array":          "[]\n", "null": "null\n", "blank": "\n",
		"duplicate": "{\"Type\":\"a\",\"Type\":\"b\"}\n",
		"oversize":  strings.Repeat(" ", MaxFrame) + "\n",
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := ReadRequest(Scanner(strings.NewReader(frame))); err == nil {
				t.Fatal("accepted invalid request")
			}
			if _, err := ReadEvent(Scanner(strings.NewReader(frame))); err == nil {
				t.Fatal("accepted invalid event")
			}
		})
	}
	if _, err := ReadRequest(Scanner(strings.NewReader(""))); !errors.Is(err, io.EOF) {
		t.Fatalf("clean EOF: %v", err)
	}
	if _, err := ReadRequest(Scanner(strings.NewReader("{}"))); !errors.Is(err, io.ErrUnexpectedEOF) {
		t.Fatalf("partial EOF: %v", err)
	}
}

func TestCodecFrameBoundary(t *testing.T) {
	// JSON string encoding adds two quotes, and Encode adds one newline.
	if frame, err := Encode(strings.Repeat("x", MaxFrame-3)); err != nil || len(frame) != MaxFrame {
		t.Fatalf("exact bound: len=%d err=%v", len(frame), err)
	}
	if _, err := Encode(strings.Repeat("x", MaxFrame-2)); err == nil {
		t.Fatal("accepted oversized encoding")
	}
	frame := "{\"Type\":\"" + strings.Repeat("x", MaxFrame-12) + "\"}\n"
	if len(frame) != MaxFrame {
		t.Fatalf("bad fixture size %d", len(frame))
	}
	scanner := Scanner(strings.NewReader(frame + "{}\n"))
	if _, err := ReadRequest(scanner); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRequest(scanner); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRequest(scanner); !errors.Is(err, io.EOF) {
		t.Fatalf("EOF: %v", err)
	}
}

type shortWriter struct{}

func (shortWriter) Write(p []byte) (int, error) { return len(p) - 1, nil }

type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }
func TestWriteRejectsShortWrite(t *testing.T) {
	if err := Write(shortWriter{}, Request{}); !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("short write: %v", err)
	}
	sentinel := errors.New("writer failure")
	if err := Write(failingWriter{sentinel}, Request{}); !errors.Is(err, sentinel) {
		t.Fatalf("writer failure: %v", err)
	}
	var output bytes.Buffer
	if err := Write(&output, Request{}); err != nil {
		t.Fatal(err)
	}
	if _, err := ReadRequest(Scanner(&output)); err != nil {
		t.Fatal(err)
	}
}
