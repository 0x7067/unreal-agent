package filesearch

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestScanWorkspaceWithoutExternalTools(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	t.Setenv("SHELL", "/nonexistent-shell")
	root := t.TempDir()
	for _, name := range []string{"src/main.go", "README.md", ".env", "docs/with spaces.md", "café/Éclair.txt", ".git/objects/object", "nested/.git/config", ".hg/store/data", ".svn/entries", "bad\nname"} {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, nil, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	outside := t.TempDir()
	if err := os.WriteFile(filepath.Join(outside, "outside.txt"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	for _, link := range [][2]string{{outside, "linked-directory"}, {"README.md", "linked-file.md"}, {"missing", "broken-link"}} {
		if err := os.Symlink(link[0], filepath.Join(root, link[1])); err != nil {
			t.Fatal(err)
		}
	}
	index, err := Scan(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	matches, err := index.Search(t.Context(), "", 20)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, match := range matches {
		paths = append(paths, match.Path)
	}
	slices.Sort(paths)
	want := []string{".env", "README.md", "café/Éclair.txt", "docs/with spaces.md", "linked-file.md", "src/main.go"}
	if !slices.Equal(paths, want) {
		t.Fatalf("paths = %q, want %q", paths, want)
	}
}

func TestScanMissingRootAndCancellation(t *testing.T) {
	if _, err := Scan(t.Context(), filepath.Join(t.TempDir(), "missing")); !os.IsNotExist(err) {
		t.Fatalf("missing root error = %v", err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := Scan(ctx, t.TempDir()); err != context.Canceled {
		t.Fatalf("canceled scan error = %v", err)
	}
}

func TestScanRespectsScopedGitignoreRules(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	root := t.TempDir()
	files := map[string]string{
		".gitignore":                  "# comment\n/root-only.txt\n*.log\n!keep.log\nbuild/\ncache/\n!cache/keep.txt\nliteral\\ space.txt\n\\#secret\n\\!secret\nsrc/**/generated?.go\n",
		"root-only.txt":               "",
		"nested/root-only.txt":        "",
		"debug.log":                   "",
		"keep.log":                    "",
		"nested/debug.log":            "",
		"nested/keep.log":             "",
		"nested/.gitignore":           "!child.log\n/only-here.txt\n",
		"nested/child.log":            "",
		"nested/only-here.txt":        "",
		"nested/deeper/only-here.txt": "",
		"build/output.bin":            "",
		"another/build/output.bin":    "",
		"another/cache/keep.txt":      "",
		"cache/keep.txt":              "",
		"literal space.txt":           "",
		"#secret":                     "",
		"!secret":                     "",
		"src/generated1.go":           "",
		"src/x/y/generated2.go":       "",
		"other/src/generated1.go":     "",
		"src/x/y/generated-long.go":   "",
		"src/x/y/ordinary.go":         "",
		"directory/.gitignore":        "build/\n",
		"directory/build":             "",
		"untracked.go":                "",
		".hidden-file":                "",
	}
	for name, content := range files {
		path := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	index, err := Scan(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	matches, err := index.Search(t.Context(), "", 100)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, match := range matches {
		paths = append(paths, match.Path)
	}
	slices.Sort(paths)
	want := []string{".gitignore", ".hidden-file", "directory/.gitignore", "directory/build", "keep.log", "nested/.gitignore", "nested/child.log", "nested/deeper/only-here.txt", "nested/keep.log", "nested/root-only.txt", "other/src/generated1.go", "src/x/y/generated-long.go", "src/x/y/ordinary.go", "untracked.go"}
	if !slices.Equal(paths, want) {
		t.Fatalf("paths = %q, want %q", paths, want)
	}
}

func TestIgnoreFilesAreBoundedAndSymlinksAreNotRead(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte(strings.Repeat("x", (1<<20)+1)), 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := Scan(t.Context(), root); err == nil || !strings.Contains(err.Error(), "exceeds") {
		t.Fatalf("oversized ignore error = %v", err)
	}
	if err := os.Remove(filepath.Join(root, ".gitignore")); err != nil {
		t.Fatal(err)
	}
	outside := filepath.Join(t.TempDir(), "ignore")
	if err := os.WriteFile(outside, []byte("*.go"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, ".gitignore")); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "main.go"), nil, 0o600); err != nil {
		t.Fatal(err)
	}
	index, err := Scan(t.Context(), root)
	if err != nil {
		t.Fatal(err)
	}
	matches, err := index.Search(t.Context(), "main", 8)
	if err != nil || len(matches) != 1 || matches[0].Path != "main.go" {
		t.Fatalf("symlinked ignore followed: %v, %v", matches, err)
	}
}
