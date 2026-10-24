package ingress

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
)

func tempDirectory(t *testing.T) string {
	t.Helper()
	path, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chmod(path, 0700); err != nil {
		t.Fatal(err)
	}
	return path
}

func input(id string) inbox.Input {
	return inbox.Input{ID: inbox.ID(id), Kind: inbox.InputExternal, Payload: []byte(`{ "text": "hello" }`)}
}
func openTest(t *testing.T, dir string) *Journal {
	t.Helper()
	if runtime.GOOS != "linux" && runtime.GOOS != "darwin" {
		t.Skip("flock platform required")
	}
	j, err := Open(dir, "session")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = j.Close() })
	return j
}
func mustSubmit(t *testing.T, j *Journal, value inbox.Input) {
	t.Helper()
	if err := j.Submit(context.Background(), value); err != nil {
		t.Fatal(err)
	}
}
func mustAck(t *testing.T, j *Journal, value inbox.Input) {
	t.Helper()
	if err := j.Acknowledge(context.Background(), value); err != nil {
		t.Fatal(err)
	}
}
func pendingIDs(j *Journal) string {
	var ids []string
	for _, v := range j.Pending() {
		ids = append(ids, string(v.ID))
	}
	return strings.Join(ids, ",")
}

func TestIdentityOrderReopenAndClones(t *testing.T) {
	dir := tempDirectory(t)
	j := openTest(t, dir)
	a, b, c := input("a"), input("b"), input("c")
	mustSubmit(t, j, a)
	mustSubmit(t, j, b)
	mustSubmit(t, j, c)
	mustSubmit(t, j, a)
	mustAck(t, j, b)
	mustAck(t, j, b)
	mustAck(t, j, input("unknown"))
	if got := pendingIDs(j); got != "a,c" {
		t.Fatal(got)
	}
	before, err := os.Stat(filepath.Join(dir, "session", "journal.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	mustSubmit(t, j, a)
	mustAck(t, j, b)
	after, err := os.Stat(filepath.Join(dir, "session", "journal.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if before.Size() != after.Size() {
		t.Fatal("idempotent operations appended records")
	}
	a.Payload[2] = 'X'
	pending := j.Pending()
	pending[0].Payload[2] = 'Y'
	if !bytes.Equal(j.Pending()[0].Payload, input("a").Payload) {
		t.Fatal("payload aliased")
	}
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	j = openTest(t, dir)
	if got := pendingIDs(j); got != "a,c" {
		t.Fatal(got)
	}
	if !bytes.Equal(j.Pending()[0].Payload, input("a").Payload) {
		t.Fatal("replay changed payload whitespace")
	}
	mustSubmit(t, j, input("b"))
	if got := pendingIDs(j); got != "a,c" {
		t.Fatal("acknowledged identity was readmitted", got)
	}
	for _, id := range []string{"a", "b"} {
		conflict := input(id)
		conflict.Payload = []byte(`{"text":"hello"}`)
		if err := j.Submit(context.Background(), conflict); !errors.Is(err, ErrConflict) {
			t.Fatalf("submit conflict: %v", err)
		}
		if err := j.Acknowledge(context.Background(), conflict); !errors.Is(err, ErrConflict) {
			t.Fatalf("ack conflict: %v", err)
		}
		conflict = input(id)
		conflict.Kind = inbox.InputCrash
		if err := j.Submit(context.Background(), conflict); !errors.Is(err, ErrConflict) {
			t.Fatalf("kind conflict: %v", err)
		}
	}
}

func TestNilPayloadReopen(t *testing.T) {
	dir := tempDirectory(t)
	j := openTest(t, dir)
	value := inbox.Input{ID: "nil", Kind: inbox.InputExternal}
	mustSubmit(t, j, value)
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	j = openTest(t, dir)
	mustSubmit(t, j, value)
	if got := j.Pending(); len(got) != 1 || got[0].Payload != nil {
		t.Fatalf("nil payload not preserved: %#v", got)
	}
}

func TestIncompleteTailRecovery(t *testing.T) {
	for _, tail := range []string{`{"version":1`, `{"version":1,"operation":"submit"}`, strings.Repeat("x", MaxRecordBytes+100)} {
		t.Run(fmt.Sprint(len(tail)), func(t *testing.T) {
			dir := tempDirectory(t)
			j := openTest(t, dir)
			mustSubmit(t, j, input("a"))
			mustAck(t, j, input("a"))
			mustSubmit(t, j, input("b"))
			if err := j.Close(); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "session", "journal.jsonl")
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			f, err := os.OpenFile(path, os.O_WRONLY|os.O_APPEND, 0600)
			if err != nil {
				t.Fatal(err)
			}
			if _, err := io.WriteString(f, tail); err != nil {
				t.Fatal(err)
			}
			if err := f.Close(); err != nil {
				t.Fatal(err)
			}
			j = openTest(t, dir)
			if got := pendingIDs(j); got != "b" {
				t.Fatal(got)
			}
			recovered, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(recovered, original) {
				t.Fatal("recovery changed committed prefix")
			}
			mustSubmit(t, j, input("c"))
			if err := j.Close(); err != nil {
				t.Fatal(err)
			}
			j = openTest(t, dir)
			if got := pendingIDs(j); got != "b,c" {
				t.Fatal(got)
			}
		})
	}
}

func TestCommittedCorruptionFailsClosed(t *testing.T) {
	for _, corruption := range []string{"\n", "not json\n", `{"version":999}` + "\n", `{"version":1,"version":1}` + "\n", strings.Repeat("x", MaxRecordBytes+1) + "\n"} {
		t.Run(fmt.Sprint(len(corruption)), func(t *testing.T) {
			dir := tempDirectory(t)
			j := openTest(t, dir)
			mustSubmit(t, j, input("a"))
			if err := j.Close(); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, "session", "journal.jsonl")
			original, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			data := append(append([]byte(nil), original...), []byte(corruption+"incomplete")...)
			if err := os.WriteFile(path, data, 0600); err != nil {
				t.Fatal(err)
			}
			if reopened, err := Open(dir, "session"); err == nil {
				_ = reopened.Close()
				t.Fatal("accepted corrupt complete record")
			}
			after, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(data, after) {
				t.Fatal("corruption repair modified committed bytes")
			}
		})
	}
}

func TestRecordBoundsAndCanceledContext(t *testing.T) {
	j := openTest(t, tempDirectory(t))
	huge := input("huge")
	huge.Payload = []byte(`"` + strings.Repeat("x", MaxRecordBytes) + `"`)
	if err := j.Submit(context.Background(), huge); err == nil {
		t.Fatal("accepted oversized payload")
	}
	encodedHuge := input("encoded-huge")
	encodedHuge.Payload = []byte(`"` + strings.Repeat("x", 3*MaxRecordBytes/4) + `"`)
	if err := j.Submit(context.Background(), encodedHuge); err == nil {
		t.Fatal("accepted oversized encoded record")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if err := j.Submit(ctx, input("a")); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if got := pendingIDs(j); got != "" {
		t.Fatal(got)
	}
	mustSubmit(t, j, input("a"))
	if err := j.Acknowledge(ctx, input("a")); !errors.Is(err, context.Canceled) {
		t.Fatal(err)
	}
	if got := pendingIDs(j); got != "a" {
		t.Fatal(got)
	}
}

// Faults are injected at the real descriptor boundary so persistence/recovery
// can be checked without relying on nondeterministic filesystem exhaustion.
type faultyFile struct {
	durableFile
	writeBytes int
	writeErr   error
	syncErr    error
	writes     int
}

func (f *faultyFile) Write(data []byte) (int, error) {
	f.writes++
	if f.writeBytes >= 0 && f.writeBytes < len(data) {
		n, err := f.durableFile.Write(data[:f.writeBytes])
		if err != nil {
			return n, err
		}
		return n, f.writeErr
	}
	n, err := f.durableFile.Write(data)
	if err != nil {
		return n, err
	}
	return n, f.writeErr
}
func (f *faultyFile) Sync() error {
	if f.syncErr != nil {
		return f.syncErr
	}
	return f.durableFile.Sync()
}

func TestStickyAmbiguousWrites(t *testing.T) {
	injected := errors.New("injected disk error")
	for _, test := range []struct {
		name              string
		n                 int
		writeErr, syncErr error
		survives          bool
	}{
		{"partial-write-error", 10, injected, nil, false},
		{"short-write", 10, nil, nil, false},
		{"complete-write-error", -1, injected, nil, true},
		{"sync-error", -1, nil, injected, true},
	} {
		t.Run(test.name, func(t *testing.T) {
			dir := tempDirectory(t)
			j := openTest(t, dir)
			faulty := &faultyFile{durableFile: j.file, writeBytes: test.n, writeErr: test.writeErr, syncErr: test.syncErr}
			j.file = faulty
			if err := j.Submit(context.Background(), input("a")); !errors.Is(err, ErrUncertain) {
				t.Fatal(err)
			}
			if got := pendingIDs(j); got != "" {
				t.Fatal("uncertain admission exposed as confirmed", got)
			}
			if err := j.Submit(context.Background(), input("b")); !errors.Is(err, ErrUncertain) {
				t.Fatal(err)
			}
			if err := j.Acknowledge(context.Background(), input("unknown")); !errors.Is(err, ErrUncertain) {
				t.Fatal(err)
			}
			if faulty.writes != 1 {
				t.Fatal("wrote after failure")
			}
			if err := j.Close(); !errors.Is(err, ErrUncertain) {
				t.Fatal(err)
			}
			j = openTest(t, dir)
			want := ""
			if test.survives {
				want = "a"
			}
			if got := pendingIDs(j); got != want {
				t.Fatalf("got %q want %q", got, want)
			}
			mustSubmit(t, j, input("b"))
		})
	}
}

func TestStickyUncertainAcknowledgement(t *testing.T) {
	dir := tempDirectory(t)
	j := openTest(t, dir)
	mustSubmit(t, j, input("a"))
	j.file = &faultyFile{durableFile: j.file, writeBytes: -1, syncErr: errors.New("injected fsync error")}
	if err := j.Acknowledge(context.Background(), input("a")); !errors.Is(err, ErrUncertain) {
		t.Fatal(err)
	}
	if got := pendingIDs(j); got != "a" {
		t.Fatal("uncertain acknowledgement hidden", got)
	}
	_ = j.Close()
	j = openTest(t, dir)
	if got := pendingIDs(j); got != "" {
		t.Fatal("complete acknowledgement not replayed", got)
	}
}

func TestConcurrentCloseAndOperations(t *testing.T) {
	dir := tempDirectory(t)
	j := openTest(t, dir)
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			value := input(fmt.Sprint(i))
			err := j.Submit(context.Background(), value)
			if err != nil && !errors.Is(err, ErrClosed) {
				t.Error(err)
			}
			_ = j.Pending()
			err = j.Acknowledge(context.Background(), value)
			if err != nil && !errors.Is(err, ErrClosed) {
				t.Error(err)
			}
			if err := j.Close(); err != nil {
				t.Error(err)
			}
		}(i)
	}
	wg.Wait()
	if err := j.Close(); err != nil {
		t.Fatal(err)
	}
	if err := j.Submit(context.Background(), input("late")); !errors.Is(err, ErrClosed) {
		t.Fatal(err)
	}
	_ = openTest(t, dir)
}
