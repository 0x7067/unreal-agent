package tool

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestDiscoverSkills(t *testing.T) {
	directory := filepath.Join(t.TempDir(), "skills")
	if err := os.MkdirAll(filepath.Join(directory, "alpha"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(directory, "beta"), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "alpha", "SKILL.md"), []byte(`---
name: alpha
description: Handle alpha tasks.
---

Alpha instructions.
`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(directory, "beta", "SKILL.md"), []byte(`---
name: beta
description: Handle beta tasks.
metadata:
  ignored: true
---

Beta instructions.
`), 0o600); err != nil {
		t.Fatal(err)
	}

	skills, skillErrors := DiscoverSkills(directory)
	if len(skillErrors) != 0 {
		t.Fatalf("skill errors = %v", skillErrors)
	}
	want := []Skill{
		{
			Name:        "alpha",
			Description: "Handle alpha tasks.",
			Path:        filepath.Join(directory, "alpha", "SKILL.md"),
		},
		{
			Name:        "beta",
			Description: "Handle beta tasks.",
			Path:        filepath.Join(directory, "beta", "SKILL.md"),
		},
	}
	if got := skills; !reflect.DeepEqual(got, want) {
		t.Fatalf("skills = %#v, want %#v", got, want)
	}
}

func TestDiscoverSkillsAllowsMissingDirectory(t *testing.T) {
	skills, skillErrors := DiscoverSkills(filepath.Join(t.TempDir(), "missing"))
	if len(skillErrors) != 0 {
		t.Fatalf("skill errors = %v", skillErrors)
	}
	if got := skills; len(got) != 0 {
		t.Fatalf("skills = %#v, want empty", got)
	}
}

func TestDiscoverSkillsYAMLMetadata(t *testing.T) {
	for _, test := range []struct {
		name, metadata, wantDescription string
	}{
		{"quoted", "name: \"review\"\ndescription: 'Review code.'\n", "Review code."},
		{"folded", "name: review\ndescription: >-\n  Review code\n  carefully.\n", "Review code carefully."},
		{"literal", "name: review\ndescription: |\n  Review code.\n  Check tests.\n", "Review code.\nCheck tests."},
		{"comments", "name: review # stable name\ndescription: Review code. # instructions\n", "Review code."},
		{"quoted punctuation", "name: review\ndescription: \"Review: code # carefully.\"\n", "Review: code # carefully."},
		{"CRLF", "name: \"review\"\r\ndescription: >-\r\n  Review code\r\n  carefully.\r\n", "Review code carefully."},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := t.TempDir()
			path := filepath.Join(directory, "review", "SKILL.md")
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("---\n"+test.metadata+"---\nBody: [not YAML\n"), 0o600); err != nil {
				t.Fatal(err)
			}
			skills, skillErrors := DiscoverSkills(directory)
			want := []Skill{{Name: "review", Description: test.wantDescription, Path: path}}
			if !reflect.DeepEqual(skills, want) || len(skillErrors) != 0 {
				t.Fatalf("skills = %#v, want %#v; errors = %v", skills, want, skillErrors)
			}
		})
	}
}

func TestDiscoverSkillsContinuesAfterInvalidFrontmatter(t *testing.T) {
	for _, test := range []struct {
		name, contents, wantError string
	}{
		{"missing opening delimiter", "name: invalid\n", "missing opening YAML frontmatter delimiter"},
		{"missing closing delimiter", "---\nname: invalid\n", "missing closing YAML frontmatter delimiter"},
		{"invalid YAML", "---\nname: invalid\ndescription: [unfinished\n---\n", "decode YAML frontmatter"},
		{"invalid name type", "---\nname: [invalid]\ndescription: Invalid skill.\n---\n", "decode YAML frontmatter"},
		{"invalid description type", "---\nname: invalid\ndescription: {nested: value}\n---\n", "decode YAML frontmatter"},
		{"duplicate key", "---\nname: invalid\nname: duplicate\ndescription: Invalid skill.\n---\n", "decode YAML frontmatter"},
	} {
		t.Run(test.name, func(t *testing.T) {
			directory := filepath.Join(t.TempDir(), "skills")
			invalidPath := filepath.Join(directory, "invalid", "SKILL.md")
			validPath := filepath.Join(directory, "valid", "SKILL.md")
			if err := os.MkdirAll(filepath.Dir(invalidPath), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.MkdirAll(filepath.Dir(validPath), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(invalidPath, []byte(test.contents), 0o600); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(validPath, []byte("---\nname: valid\ndescription: Valid skill.\n---\n"), 0o600); err != nil {
				t.Fatal(err)
			}

			skills, skillErrors := DiscoverSkills(directory)
			want := []Skill{{Name: "valid", Description: "Valid skill.", Path: validPath}}
			if got := skills; !reflect.DeepEqual(got, want) {
				t.Fatalf("skills = %#v, want %#v", got, want)
			}
			if len(skillErrors) != 1 ||
				!strings.Contains(skillErrors[0].Error(), invalidPath) ||
				!strings.Contains(skillErrors[0].Error(), test.wantError) {
				t.Fatalf("skill errors = %v", skillErrors)
			}
		})
	}
}

func TestDiscoverSkillsRejectsInvalidMetadataAndDuplicateNames(t *testing.T) {
	directory := t.TempDir()
	for _, entry := range []struct {
		name, contents string
	}{
		{"alpha", "---\nname: review\ndescription: Review code.\n---\n"},
		{"duplicate", "---\nname: \"review\"\ndescription: Duplicate.\n---\n"},
		{"missing-name", "---\ndescription: Missing name.\n---\n"},
		{"missing-description", "---\nname: missing-description\n---\n"},
		{"empty-name", "---\nname: ' '\ndescription: Empty name.\n---\n"},
		{"empty-description", "---\nname: empty-description\ndescription: ''\n---\n"},
	} {
		path := filepath.Join(directory, entry.name, "SKILL.md")
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(entry.contents), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	skills, skillErrors := DiscoverSkills(directory)
	want := []Skill{{Name: "review", Description: "Review code.", Path: filepath.Join(directory, "alpha", "SKILL.md")}}
	if !reflect.DeepEqual(skills, want) || len(skillErrors) != 5 {
		t.Fatalf("discovered skills = %#v, errors = %v", skills, skillErrors)
	}
	registry := NewRegistry(StaticTranslators{})
	for _, skill := range skills {
		if _, err := registry.RegisterSkill(skill); err != nil {
			t.Fatalf("discovered skill cannot be registered: %v", err)
		}
	}
}
