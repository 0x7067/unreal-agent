// Package settings reads user configuration shared by Harness applications.
package settings

import (
	_ "embed"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"slices"
	"strings"
)

//go:embed default_settings.json
var defaultSettingsJSON []byte

type Settings struct {
	Providers map[string]Provider `json:"providers"`
}

type Provider struct {
	Info   ProviderInfo `json:"info"`
	Models []Model      `json:"models"`
}

type ProviderInfo struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

type Model struct {
	ID                  string `json:"id"`
	Name                string `json:"name"`
	ContextWindow       int64  `json:"context_window"`
	CompactionThreshold int64  `json:"compaction_threshold"`
}

func Load(path string) (Settings, error) {
	configured, err := decodeSettings(defaultSettingsJSON)
	if err != nil {
		return Settings{}, fmt.Errorf("decode built-in settings: %w", err)
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return configured, nil
	}
	if err != nil {
		return Settings{}, fmt.Errorf("read user settings %q: %w", path, err)
	}
	overrides, err := decodeSettings(data)
	if err != nil {
		return Settings{}, fmt.Errorf("decode user settings %q: %w", path, err)
	}
	for id, override := range overrides.Providers {
		provider := configured.Providers[id]
		provider.Info = override.Info
		for _, model := range override.Models {
			index := slices.IndexFunc(provider.Models, func(current Model) bool { return current.ID == model.ID })
			if index == -1 {
				provider.Models = append(provider.Models, model)
			} else {
				provider.Models[index] = model
			}
		}
		configured.Providers[id] = provider
	}
	return configured, nil
}

func (configured Settings) Model(providerID, modelID string) Model {
	for _, model := range configured.Providers[providerID].Models {
		if model.ID == modelID {
			return model
		}
	}
	return Model{}
}

func decodeSettings(data []byte) (Settings, error) {
	var config Settings
	if err := json.Unmarshal(data, &config); err != nil {
		return Settings{}, err
	}
	for id, provider := range config.Providers {
		if strings.TrimSpace(id) == "" || provider.Info.ID != id {
			return Settings{}, fmt.Errorf("provider %q info.id must match its non-empty key", id)
		}
		if strings.TrimSpace(provider.Info.Name) == "" {
			return Settings{}, fmt.Errorf("provider %q name must not be empty", id)
		}
		seen := make(map[string]bool, len(provider.Models))
		for index, model := range provider.Models {
			if strings.TrimSpace(model.ID) == "" {
				return Settings{}, fmt.Errorf("provider %q model id must not be empty", id)
			}
			if strings.TrimSpace(model.Name) == "" {
				return Settings{}, fmt.Errorf("provider %q model %q name must not be empty", id, model.ID)
			}
			if model.ContextWindow <= 0 {
				return Settings{}, fmt.Errorf("provider %q model %q context_window must be positive", id, model.ID)
			}
			if seen[model.ID] {
				return Settings{}, fmt.Errorf("provider %q duplicate model id %q", id, model.ID)
			}
			seen[model.ID] = true
			if model.CompactionThreshold < 0 {
				return Settings{}, fmt.Errorf("provider %q model %q compaction_threshold must not be negative", id, model.ID)
			}
			if model.CompactionThreshold == 0 {
				model.CompactionThreshold = model.ContextWindow / 2
			}
			provider.Models[index] = model
		}
	}
	return config, nil
}
