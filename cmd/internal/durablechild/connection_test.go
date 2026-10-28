package durablechild

import (
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func fixture(t *testing.T) (Config, Request) {
	t.Helper()
	root := t.TempDir()
	if e := os.Chmod(root, 0700); e != nil {
		t.Fatal(e)
	}
	runtime := filepath.Join(root, "runtime")
	if e := os.Mkdir(runtime, 0700); e != nil {
		t.Fatal(e)
	}
	return Config{Directory: root, RuntimeDirectory: runtime, Workspace: root, Command: []string{"/nonexistent-durable-child-command"}, Model: "test"}, Request{ParentID: "parent", OperationID: "op", Prompt: "hello", Lifetime: "background"}
}
func TestPlanStableIdentity(t *testing.T) {
	cfg, req := fixture(t)
	a, e := plan(cfg, req)
	if e != nil {
		t.Fatal(e)
	}
	b, e := plan(cfg, req)
	if e != nil || a.Link != b.Link {
		t.Fatal("unstable link")
	}
	if len(a.Link.DeliveryID) > 128 {
		t.Fatal("oversized ID")
	}
	req.Prompt = "changed"
	b, _ = plan(cfg, req)
	if a.Link.Key != b.Link.Key || a.Link.ChildID != b.Link.ChildID || a.Link.PlanDigest == b.Link.PlanDigest {
		t.Fatal("wrong identity/digest")
	}
}
func TestNotSubmittedNoEffects(t *testing.T) {
	cfg, req := fixture(t)
	_, e := Open(t.Context(), cfg, req, false)
	if !errors.Is(e, ErrNotSubmitted) {
		t.Fatal(e)
	}
	if _, e = os.Stat(filepath.Join(cfg.Directory, "durable-children")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("created reservation without launch permission")
	}
}
func TestReservationIsDurableAndNeverReexecuted(t *testing.T) {
	cfg, req := fixture(t)
	_, e := Open(t.Context(), cfg, req, true)
	if !errors.Is(e, ErrUncertain) {
		t.Fatal(e)
	}
	r, _ := plan(cfg, req)
	marker := filepath.Join(r.Link.Directory, "launch.json")
	stored, e := readRecord(marker)
	if e != nil || stored.Link != r.Link {
		t.Fatalf("record: %v", e)
	}
	st, _ := os.Stat(marker)
	if st.Mode().Perm() != 0600 {
		t.Fatal("nonprivate marker")
	}
	req.Prompt = "different"
	_, e = Open(t.Context(), cfg, req, true)
	if !errors.Is(e, ErrConflict) {
		t.Fatal(e)
	}
	log := filepath.Join(r.Link.Directory, "host.log")
	if e = os.Remove(log); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, _ = Open(ctx, cfg, stored.Request, true)
	if _, e = os.Stat(log); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("reexecuted")
	}
}
func TestMalformedMarkerFailsClosed(t *testing.T) {
	cfg, req := fixture(t)
	r, _ := plan(cfg, req)
	if e := os.MkdirAll(r.Link.Directory, 0700); e != nil {
		t.Fatal(e)
	}
	for _, b := range [][]byte{[]byte("{"), []byte(`{"Version":2}`)} {
		if e := os.WriteFile(filepath.Join(r.Link.Directory, "launch.json"), b, 0600); e != nil {
			t.Fatal(e)
		}
		_, e := Open(t.Context(), cfg, req, true)
		if !errors.Is(e, ErrUncertain) {
			t.Fatal(e)
		}
	}
}
func TestUnknownLaunchNoExecution(t *testing.T) {
	cfg, req := fixture(t)
	r, _ := plan(cfg, req)
	if e := os.MkdirAll(r.Link.Directory, 0700); e != nil {
		t.Fatal(e)
	}
	b, _ := json.Marshal(r)
	if e := os.WriteFile(filepath.Join(r.Link.Directory, "launch.json"), b, 0600); e != nil {
		t.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 10*1000000)
	defer cancel()
	_, e := Open(ctx, cfg, req, true)
	if e == nil {
		t.Fatal("missing endpoint accepted")
	}
	if _, e = os.Stat(filepath.Join(r.Link.Directory, "host.log")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("executed existing reservation")
	}
}

func TestStagedCommandPathDoesNotChangeReservation(t *testing.T) {
	cfg, req := fixture(t)
	original, err := plan(cfg, req)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(original.Link.Directory, 0700); err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(original.Link.Directory, "launch.json")
	b, err := json.Marshal(original)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(marker, b, 0600); err != nil {
		t.Fatal(err)
	}
	cfg.Command = []string{"/different/staged/runner"}
	replacement, err := plan(cfg, req)
	if err != nil || replacement.Link != original.Link {
		t.Fatalf("staged path changed identity: %+v err=%v", replacement.Link, err)
	}
	for i := 0; i < 2; i++ {
		ctx, cancel := context.WithTimeout(t.Context(), 10*time.Millisecond)
		c, err := Open(ctx, cfg, req, false)
		cancel()
		if c != nil {
			if c.Link() != original.Link {
				t.Fatal("connection link changed")
			}
			_ = c.Close()
		}
		if !errors.Is(err, context.DeadlineExceeded) {
			t.Fatalf("existing reservation not retried as connection only: %v", err)
		}
	}
	stored, err := readRecord(marker)
	if err != nil || stored.Link != original.Link || stored.Config.Command[0] != original.Config.Command[0] {
		t.Fatalf("reservation changed: %+v err=%v", stored, err)
	}
	if _, err = os.Stat(filepath.Join(original.Link.Directory, "host.log")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("existing reservation executed: %v", err)
	}
}

func TestReadRecordRejectsUnknownMembers(t *testing.T) {
	cfg, req := fixture(t)
	r, err := plan(cfg, req)
	if err != nil {
		t.Fatal(err)
	}
	if err = os.MkdirAll(r.Link.Directory, 0700); err != nil {
		t.Fatal(err)
	}
	b, err := json.Marshal(r)
	if err != nil {
		t.Fatal(err)
	}
	marker := filepath.Join(r.Link.Directory, "launch.json")
	// The top-level launch envelope is strict, not merely its identity fields.
	b = append(append([]byte(nil), b[:len(b)-1]...), []byte(`,"Unexpected":true}`)...)
	if err = os.WriteFile(marker, b, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err = readRecord(marker); err == nil {
		t.Fatal("unknown metadata accepted")
	}
}

func TestReadRecordRejectsNonprivateAndSymlinkPaths(t *testing.T) {
	for _, kind := range []string{"public marker", "marker symlink", "ancestor symlink"} {
		t.Run(kind, func(t *testing.T) {
			cfg, req := fixture(t)
			r, err := plan(cfg, req)
			if err != nil {
				t.Fatal(err)
			}
			if err = os.MkdirAll(r.Link.Directory, 0700); err != nil {
				t.Fatal(err)
			}
			b, err := json.Marshal(r)
			if err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(r.Link.Directory, "launch.json")
			if err = os.WriteFile(marker, b, 0600); err != nil {
				t.Fatal(err)
			}
			switch kind {
			case "public marker":
				err = os.Chmod(marker, 0644)
			case "marker symlink":
				target := filepath.Join(r.Link.Directory, "real.json")
				if err = os.Rename(marker, target); err == nil {
					err = os.Symlink(target, marker)
				}
			case "ancestor symlink":
				alias := filepath.Join(cfg.Directory, "alias")
				err = os.Symlink(r.Link.Directory, alias)
				marker = filepath.Join(alias, "launch.json")
			}
			if err != nil {
				t.Fatal(err)
			}
			if _, err = readRecord(marker); err == nil {
				t.Fatal("unsafe metadata accepted")
			}
		})
	}
}
