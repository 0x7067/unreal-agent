package filesearch

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
)

func TestFuzzySearch(t *testing.T) {
	index := NewIndex([]string{"src/FooBar.go", "src/foo_bar_test.go", "deep/other.go", "docs/README.md", "README.md", "café/Éclair.txt"})
	for _, test := range []struct {
		query string
		want  []string
	}{
		{"fbg", []string{"src/FooBar.go", "src/foo_bar_test.go"}},
		{"README.md", []string{"README.md", "docs/README.md"}},
		{"SRC/fb", []string{"src/FooBar.go", "src/foo_bar_test.go"}},
		{"./src/fb", []string{"src/FooBar.go", "src/foo_bar_test.go"}},
		{"cÉ", []string{"café/Éclair.txt"}},
		{"Éc", []string{"café/Éclair.txt"}},
		{"zzz", nil},
	} {
		t.Run(test.query, func(t *testing.T) {
			matches, err := index.Search(t.Context(), test.query, 8)
			if err != nil {
				t.Fatal(err)
			}
			var paths []string
			for _, match := range matches {
				paths = append(paths, match.Path)
				var highlighted []rune
				text := []rune(strings.ToLower(match.Path))
				for i, position := range match.Positions {
					if i > 0 && match.Positions[i-1] >= position {
						t.Fatalf("positions are not ordered: %v", match.Positions)
					}
					highlighted = append(highlighted, text[position])
				}
				if string(highlighted) != strings.ToLower(strings.TrimPrefix(test.query, "./")) {
					t.Fatalf("highlight = %q for query %q", string(highlighted), test.query)
				}
			}
			if !slices.Equal(paths, test.want) {
				t.Fatalf("paths = %v, want %v", paths, test.want)
			}
		})
	}
}

func TestSearchRanksBoundaryMatchesAndBreaksTies(t *testing.T) {
	index := NewIndex([]string{"foo/other.go", "a/f_o_o.go", "a/foobar.go", "foo.go", "b/foo.go", "a/foo.go"})
	matches, err := index.Search(t.Context(), "foo", 6)
	if err != nil {
		t.Fatal(err)
	}
	var paths []string
	for _, match := range matches {
		paths = append(paths, match.Path)
	}
	want := []string{"a/f_o_o.go", "foo.go", "a/foo.go", "b/foo.go", "a/foobar.go", "foo/other.go"}
	if !slices.Equal(paths, want) {
		t.Fatalf("paths = %v, want %v", paths, want)
	}
}

func TestSearchPrefersExactFilenames(t *testing.T) {
	index := NewIndex([]string{"a/f_o_o", "a/Foo", "foo", "foo.txt"})
	matches, err := index.Search(t.Context(), "foo", 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 4 || matches[0].Path != "foo" || matches[1].Path != "a/Foo" {
		t.Fatalf("exact filename ranking = %v", matches)
	}
}

func TestSearchPrefersFilenamesOverDirectoryMatches(t *testing.T) {
	index := NewIndex([]string{
		"cmd/unreal-agent-tui/internal/runcontrol/stop.go",
		"cmd/unreal-agent-tui/internal/filesearch/fuzzy.go",
		"cmd/unreal-agent-tui/internal/filesearch/index.go",
		"cmd/unreal-agent-tui/internal/reasoning/effort.go",
		"cmd/unreal-agent-tui/ui.go",
		"other/ui.go",
	})
	matches, err := index.Search(t.Context(), "uig", 8)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 6 || matches[0].Path != "other/ui.go" || matches[1].Path != "cmd/unreal-agent-tui/ui.go" {
		t.Fatalf("filename ranking = %v", matches)
	}
	if !slices.Equal(matches[1].Positions, []int{21, 22, 24}) {
		t.Fatalf("filename highlights = %v", matches[1].Positions)
	}
}

func TestSearchKeepsBestMatchesAcrossBatches(t *testing.T) {
	paths := make([]string, 600)
	for i := range paths {
		paths[i] = fmt.Sprintf("unrelated%d", i)
	}
	paths[0], paths[256], paths[257], paths[599] = "x/foo.go", "b/foo.go", "a/foo.go", "foo.go"
	matches, err := NewIndex(paths).Search(t.Context(), "foo", 3)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"foo.go", "a/foo.go", "b/foo.go"}
	if len(matches) != len(want) {
		t.Fatalf("got %d matches, want %d", len(matches), len(want))
	}
	for i, match := range matches {
		if match.Path != want[i] {
			t.Fatalf("rank %d = %q, want %q", i, match.Path, want[i])
		}
	}
}

func TestEmptyQueryAndLimits(t *testing.T) {
	index := NewIndex([]string{"bbb", "aa", "ab", "x", "bad\nname", "bad\x1b[31m"})
	matches, err := index.Search(t.Context(), "", 3)
	if err != nil {
		t.Fatal(err)
	}
	if len(matches) != 3 || matches[0].Path != "x" || matches[1].Path != "aa" || matches[2].Path != "ab" {
		t.Fatalf("empty query = %v", matches)
	}
	for _, limit := range []int{-1, 0} {
		if matches, err := index.Search(t.Context(), "a", limit); err != nil || len(matches) != 0 {
			t.Fatalf("limit %d = %v, %v", limit, matches, err)
		}
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := index.Search(ctx, "a", 8); err != context.Canceled {
		t.Fatalf("canceled search error = %v", err)
	}
}

func BenchmarkSearch(b *testing.B) {
	paths := make([]string, 50000)
	for i := range paths {
		paths[i] = fmt.Sprintf("packages/component%d/src/FileSearcher%d.go", i, i)
	}
	index := NewIndex(paths)
	b.ResetTimer()
	for b.Loop() {
		if _, err := index.Search(b.Context(), "fsgo", 8); err != nil {
			b.Fatal(err)
		}
	}
}
