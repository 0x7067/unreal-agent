package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func localStateFixture() attachLocalState {
	return attachLocalState{Version: 1, Namespace: strings.Repeat("a", 64), Draft: "hello\n世界", ConversationOffset: 4, ToolsOffset: 2, DetailsOffset: 3, SelectedCallID: "call-1", ToolsFocused: true, SelectionPane: "conversation", SelectionStart: 2, SelectionEnd: 4, Retained: []attachLocalInput{{ID: "12345678-1234-4234-8234-123456789abc", Text: "retry\nme"}}}
}
func localStateDirectory(t *testing.T) string {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Join(root, "private", "nested")
}
func TestAttachLocalStateRoundTrip(t *testing.T) {
	dir := localStateDirectory(t)
	want := localStateFixture()
	if err := saveAttachLocalState(dir, want); err != nil {
		t.Fatal(err)
	}
	got, err := loadAttachLocalState(dir, want.Namespace)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
	want.Draft = "replacement"
	if err := saveAttachLocalState(dir, want); err != nil {
		t.Fatal(err)
	}
	got, err = loadAttachLocalState(dir, want.Namespace)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("replacement: %#v %v", got, err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil || len(entries) != 1 {
		t.Fatalf("temp file leak: %v %v", entries, err)
	}
	for _, p := range []string{dir, filepath.Join(dir, want.Namespace+".json")} {
		st, err := os.Stat(p)
		if err != nil {
			t.Fatal(err)
		}
		wantMode := os.FileMode(0600)
		if st.IsDir() {
			wantMode = 0700
		}
		if st.Mode().Perm() != wantMode {
			t.Fatalf("mode %s", st.Mode())
		}
	}
}
func TestAttachLocalStateRejectsInvalid(t *testing.T) {
	cases := map[string]func(*attachLocalState){
		"version": func(s *attachLocalState) { s.Version = 3 }, "namespace": func(s *attachLocalState) { s.Namespace = "../outside" }, "negative": func(s *attachLocalState) { s.ToolsOffset = -1 }, "offset": func(s *attachLocalState) { s.SelectionEnd = 1000000001 }, "identifier-control": func(s *attachLocalState) { s.SelectedCallID = "\x1b[2J" }, "identifier-size": func(s *attachLocalState) { s.DetailsCallID = strings.Repeat("x", 129) }, "draft-size": func(s *attachLocalState) { s.Draft = strings.Repeat("x", attachLocalTextLimit+1) }, "draft-utf8": func(s *attachLocalState) { s.Draft = "\xff" }, "UUID": func(s *attachLocalState) { s.Retained[0].ID = "bad" }, "duplicate": func(s *attachLocalState) { s.Retained = append(s.Retained, s.Retained[0]) }, "count": func(s *attachLocalState) { s.Retained = make([]attachLocalInput, 65) }, "text-size": func(s *attachLocalState) { s.Retained[0].Text = strings.Repeat("x", attachLocalTextLimit+1) }, "encoded-size": func(s *attachLocalState) { s.Retained[0].Text = strings.Repeat("\x00", attachLocalTextLimit/6+1) },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			dir := localStateDirectory(t)
			s := localStateFixture()
			if err := saveAttachLocalState(dir, s); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, s.Namespace+".json")
			before, _ := os.ReadFile(path)
			mutate(&s)
			if err := saveAttachLocalState(dir, s); err == nil {
				t.Fatal("accepted invalid state")
			}
			after, _ := os.ReadFile(path)
			if string(before) != string(after) {
				t.Fatal("old state changed")
			}
		})
	}
}
func TestAttachLocalStateRejectsJSON(t *testing.T) {
	for _, kind := range []string{"corrupt", "unknown", "control-input", "namespace", "version", "trailing", "oversize"} {
		t.Run(kind, func(t *testing.T) {
			dir := localStateDirectory(t)
			s := localStateFixture()
			if err := saveAttachLocalState(dir, s); err != nil {
				t.Fatal(err)
			}
			data, _ := json.Marshal(s)
			switch kind {
			case "corrupt":
				data = []byte("{")
			case "unknown":
				data = append(data[:len(data)-1], []byte(`,"unexpected":1}`)...)
			case "control-input":
				data = []byte(`{"Version":1,"Namespace":"` + s.Namespace + `","Retained":[{"ID":"12345678-1234-4234-8234-123456789abc","Text":"test","Kind":"control"}]}`)
			case "namespace":
				data = []byte(strings.Replace(string(data), s.Namespace, strings.Repeat("b", 64), 1))
			case "version":
				data = []byte(strings.Replace(string(data), `"Version":1`, `"Version":3`, 1))
			case "trailing":
				data = append(data, []byte(" {}")...)
			case "oversize":
				data = []byte(strings.Repeat(" ", attachLocalFileLimit+1))
			}
			if err := os.WriteFile(filepath.Join(dir, s.Namespace+".json"), data, 0600); err != nil {
				t.Fatal(err)
			}
			if _, err := loadAttachLocalState(dir, s.Namespace); err == nil {
				t.Fatal("accepted invalid JSON")
			}
		})
	}
}
func TestAttachLocalStateUnsafePathsNoOutsideMutation(t *testing.T) {
	for _, kind := range []string{"ancestor-symlink", "file-symlink", "file-mode", "directory-mode", "nonregular"} {
		t.Run(kind, func(t *testing.T) {
			dir := localStateDirectory(t)
			s := localStateFixture()
			if err := saveAttachLocalState(dir, s); err != nil {
				t.Fatal(err)
			}
			outside := filepath.Join(filepath.Dir(dir), "outside")
			if err := os.WriteFile(outside, []byte("untouched"), 0600); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(dir, s.Namespace+".json")
			switch kind {
			case "ancestor-symlink":
				alias := filepath.Join(filepath.Dir(dir), "alias")
				if err := os.Symlink(dir, alias); err != nil {
					t.Fatal(err)
				}
				dir = filepath.Join(alias, "must-not-create")
			case "file-symlink":
				os.Remove(path)
				if err := os.Symlink(outside, path); err != nil {
					t.Fatal(err)
				}
			case "file-mode":
				os.Chmod(path, 0644)
			case "directory-mode":
				os.Chmod(dir, 0755)
			case "nonregular":
				os.Remove(path)
				if err := os.Mkdir(path, 0600); err != nil {
					t.Fatal(err)
				}
			}
			if err := saveAttachLocalState(dir, s); err == nil {
				t.Fatal("unsafe save succeeded")
			}
			if _, err := loadAttachLocalState(dir, s.Namespace); err == nil {
				t.Fatal("unsafe load succeeded")
			}
			data, err := os.ReadFile(outside)
			if err != nil || string(data) != "untouched" {
				t.Fatalf("outside mutation: %q %v", data, err)
			}
			if kind == "ancestor-symlink" {
				if _, err := os.Stat(filepath.Join(filepath.Dir(dir), "must-not-create")); !os.IsNotExist(err) {
					t.Fatalf("created through symlink: %v", err)
				}
			}
		})
	}
}
func TestAttachLocalStateAggregateLimit(t *testing.T) {
	s := localStateFixture()
	s.Retained = nil
	for i := 0; i < 34; i++ {
		id := []byte("12345678-1234-4234-8234-123456789a00")
		const hex = "0123456789abcdef"
		id[34] = hex[i/16]
		id[35] = hex[i%16]
		s.Retained = append(s.Retained, attachLocalInput{ID: string(id), Text: strings.Repeat("x", attachLocalTextLimit-2)})
	}
	if err := validateAttachLocalState(s); err == nil {
		t.Fatal("accepted aggregate overflow")
	}
}

func TestAttachLocalStateNormalizedPayloadBoundary(t *testing.T) {
	s := localStateFixture()
	s.Retained[0].Text = strings.Repeat("<", attachLocalTextLimit-2)
	if err := validateAttachLocalState(s); err != nil {
		t.Fatalf("valid normalized JSON payload rejected: %v", err)
	}
	s.Retained[0].Text += "x"
	if err := validateAttachLocalState(s); err == nil {
		t.Fatal("oversized normalized payload accepted")
	}
}

func TestAttachLocalStateLeaseFencesDisconnectedFrontend(t *testing.T) {
	dir := t.TempDir() + "/clients"
	namespace := strings.Repeat("a", 64)
	first, err := lockAttachLocalState(dir, namespace)
	if err != nil {
		t.Fatal(err)
	}
	defer first.Close()
	if other, err := lockAttachLocalState(dir, namespace); err == nil {
		other.Close()
		t.Fatal("competing local-state writer acquired lease")
	}
	first.Close()
	replacement, err := lockAttachLocalState(dir, namespace)
	if err != nil {
		t.Fatal("replacement could not acquire released lease", err)
	}
	replacement.Close()
	if err := os.Remove(filepath.Join(dir, namespace+".lock")); err != nil {
		t.Fatal(err)
	}
	target := filepath.Join(t.TempDir(), "outside")
	if err := os.WriteFile(target, []byte("unchanged"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, filepath.Join(dir, namespace+".lock")); err != nil {
		t.Fatal(err)
	}
	if f, err := lockAttachLocalState(dir, namespace); err == nil {
		f.Close()
		t.Fatal("symlink lock accepted")
	}
	data, _ := os.ReadFile(target)
	if string(data) != "unchanged" {
		t.Fatal("outside lock target changed")
	}
}
