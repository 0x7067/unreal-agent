package main

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"github.com/unreallabsai/unreal-agent/harness/llm"
)

type startupPreferences struct {
	Provider        string              `json:"provider"`
	Model           string              `json:"model"`
	ReasoningEffort llm.ReasoningEffort `json:"reasoning_effort"`
}

func (p startupPreferences) validate() error {
	if p.Provider != "openai-codex" {
		return errors.New("saved provider must be openai-codex")
	}
	if p.Model == "" || p.Model != singleLine(p.Model) {
		return errors.New("saved model must be a non-empty model ID")
	}
	if !p.ReasoningEffort.Valid() {
		return errors.New("saved reasoning effort must be low, medium, high, xhigh, or max")
	}
	return nil
}

func loadStartupPreferences(path string) (startupPreferences, error) {
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return startupPreferences{}, nil
	}
	if err != nil {
		return startupPreferences{}, fmt.Errorf("read startup preferences %q: %w; use -setup to choose again", path, err)
	}
	var preferences startupPreferences
	if err := json.Unmarshal(data, &preferences, json.RejectUnknownMembers(true)); err != nil {
		return startupPreferences{}, fmt.Errorf("decode startup preferences %q: %w; use -setup to choose again", path, err)
	}
	if err := preferences.validate(); err != nil {
		return startupPreferences{}, fmt.Errorf("invalid startup preferences %q: %w; use -setup to choose again", path, err)
	}
	return preferences, nil
}

func saveStartupPreferences(path string, preferences startupPreferences) error {
	if err := preferences.validate(); err != nil {
		return err
	}
	data, err := json.Marshal(preferences, jsontext.WithIndent("  "))
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return fmt.Errorf("create startup preferences directory: %w", err)
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".preferences-*.json")
	if err != nil {
		return fmt.Errorf("create startup preferences file: %w", err)
	}
	defer func() {
		_ = file.Close()
		_ = os.Remove(file.Name())
	}()
	if _, err := file.Write(append(data, '\n')); err != nil {
		return fmt.Errorf("write startup preferences: %w", err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("close startup preferences: %w", err)
	}
	if err := os.Rename(file.Name(), path); err != nil {
		return fmt.Errorf("save startup preferences %q: %w", path, err)
	}
	return nil
}

func startupSelection(opts options, preferences startupPreferences) (options, bool) {
	if opts.provider != "" && opts.provider != "openai-codex" {
		return opts, false
	}
	showDialog := opts.setup || opts.provider == "" && preferences.Provider == ""
	opts.provider = "openai-codex"
	if preferences.Provider != "" {
		if !opts.modelExplicit {
			opts.model = preferences.Model
		}
		if !opts.effortExplicit {
			opts.effort = string(preferences.ReasoningEffort)
		}
	}
	return opts, showDialog
}
