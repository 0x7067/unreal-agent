package taskgraph

import (
	"os"
	"path/filepath"
	"syscall"
	"testing"
)

func fingerprintWrite(t *testing.T, dir, name, value string) {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(value), 0644); err != nil {
		t.Fatal(err)
	}
}
func fingerprintMust(t *testing.T, dir string, paths []string) string {
	t.Helper()
	got, err := Fingerprint(dir, paths)
	if err != nil {
		t.Fatal(err)
	}
	return got
}
func TestFingerprintContentAndMembership(t *testing.T) {
	dir := t.TempDir()
	fingerprintWrite(t, dir, "src/a", "one")
	initial := fingerprintMust(t, dir, []string{"src"})
	if initial != fingerprintMust(t, dir, []string{"src"}) {
		t.Fatal("unstable fingerprint")
	}
	fingerprintWrite(t, dir, "unrelated", "ignore")
	if initial != fingerprintMust(t, dir, []string{"src"}) {
		t.Fatal("unrelated path changed fingerprint")
	}
	fingerprintWrite(t, dir, "src/a", "two")
	changed := fingerprintMust(t, dir, []string{"src"})
	if initial == changed {
		t.Fatal("content mutation ignored")
	}
	fingerprintWrite(t, dir, "src/b", "new")
	added := fingerprintMust(t, dir, []string{"src"})
	if added == changed {
		t.Fatal("addition ignored")
	}
	if err := os.Remove(filepath.Join(dir, "src/b")); err != nil {
		t.Fatal(err)
	}
	if fingerprintMust(t, dir, []string{"src"}) != changed {
		t.Fatal("delete not reflected")
	}
	if err := os.Chmod(filepath.Join(dir, "src/a"), 0600); err != nil {
		t.Fatal(err)
	}
	if fingerprintMust(t, dir, []string{"src"}) == changed {
		t.Fatal("mode ignored")
	}
}
func TestFingerprintNilEmptyAndGit(t *testing.T) {
	dir := t.TempDir()
	all := fingerprintMust(t, dir, nil)
	empty := fingerprintMust(t, dir, []string{})
	fingerprintWrite(t, dir, ".git/index", "ignore")
	if all != fingerprintMust(t, dir, nil) {
		t.Fatal("root .git was included")
	}
	fingerprintWrite(t, dir, "nested/.git/index", "ignore")
	nested := fingerprintMust(t, dir, nil)
	fingerprintWrite(t, dir, "nested/.git/index", "changed")
	if nested != fingerprintMust(t, dir, nil) {
		t.Fatal("nested .git was included")
	}
	fingerprintWrite(t, dir, "file", "change")
	if all == fingerprintMust(t, dir, nil) {
		t.Fatal("nil did not cover workspace")
	}
	if empty != fingerprintMust(t, dir, []string{}) {
		t.Fatal("empty claims depend on content")
	}
	if fingerprintMust(t, dir, nil) != fingerprintMust(t, dir, []string{"."}) {
		t.Fatal("nil is not root")
	}
}
func TestFingerprintMissingNestedPath(t *testing.T) {
	dir := t.TempDir()
	missing := fingerprintMust(t, dir, []string{"a/b/c"})
	fingerprintWrite(t, dir, "a/b/c", "present")
	if missing == fingerprintMust(t, dir, []string{"a/b/c"}) {
		t.Fatal("creation ignored")
	}
	if err := os.RemoveAll(filepath.Join(dir, "a")); err != nil {
		t.Fatal(err)
	}
	if missing != fingerprintMust(t, dir, []string{"a/b/c"}) {
		t.Fatal("missing path identity unstable")
	}
}
func TestFingerprintUnsafeAliases(t *testing.T) {
	dir := t.TempDir()
	outside := t.TempDir()
	fingerprintWrite(t, outside, "secret", "outside")
	if err := os.Symlink(outside, filepath.Join(dir, "alias")); err != nil {
		t.Fatal(err)
	}
	for _, paths := range [][]string{{"../secret"}, {outside}, {"alias"}, {"alias/secret"}, {".git/index"}, nil} {
		if _, err := Fingerprint(dir, paths); err == nil {
			t.Fatalf("accepted unsafe claims %v", paths)
		}
	}
	parent := t.TempDir()
	if err := os.Symlink(dir, filepath.Join(parent, "workspace")); err != nil {
		t.Fatal(err)
	}
	if _, err := Fingerprint(filepath.Join(parent, "workspace"), []string{}); err != nil {
		t.Fatalf("trusted workspace alias rejected: %v", err)
	}
	fingerprintWrite(t, dir, "local/file", "inside")
	if err := os.Symlink("local", filepath.Join(dir, "local-alias")); err != nil {
		t.Fatal(err)
	}
	if _, err := Fingerprint(dir, []string{"local-alias/file"}); err == nil {
		t.Fatal("accepted internal symlink ancestor")
	}
}
func TestFingerprintOrderAndName(t *testing.T) {
	dir := t.TempDir()
	fingerprintWrite(t, dir, "a", "same")
	fingerprintWrite(t, dir, "b", "same")
	if fingerprintMust(t, dir, []string{"b", "a", "a"}) != fingerprintMust(t, dir, []string{"a", "b"}) {
		t.Fatal("claim order changed hash")
	}
	if fingerprintMust(t, dir, []string{"a"}) == fingerprintMust(t, dir, []string{"b"}) {
		t.Fatal("path name ignored")
	}
}

func TestFingerprintReadFailuresAndTypes(t *testing.T) {
	dir := t.TempDir()
	fingerprintWrite(t, dir, "regular", "value")
	if _, err := Fingerprint(dir, []string{"regular/child"}); err == nil {
		t.Fatal("accepted non-directory ancestor")
	}
	if err := syscall.Mkfifo(filepath.Join(dir, "fifo"), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Fingerprint(dir, []string{"fifo"}); err == nil {
		t.Fatal("accepted unsupported FIFO")
	}
	if err := os.Chmod(filepath.Join(dir, "regular"), 0); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(filepath.Join(dir, "regular"), 0600)
	if f, err := os.Open(filepath.Join(dir, "regular")); err == nil {
		f.Close()
		t.Skip("current user can read mode-zero files")
	}
	if _, err := Fingerprint(dir, []string{"regular"}); err == nil {
		t.Fatal("returned fingerprint for unreadable input")
	}
}
