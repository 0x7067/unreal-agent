package specpath

import (
	"fmt"
	"os"
	"path/filepath"
)

// moduleRoot walks up from the working directory to the directory containing go.mod.
func moduleRoot() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("working directory: %w", err)
	}
	dir := wd
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("go.mod not found above %s", wd)
		}
		dir = parent
	}
}

// Dir returns docs/formal/review/spec under the module root.
func Dir() (string, error) {
	root, err := moduleRoot()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, "docs", "formal", "review", "spec"), nil
}

// File joins Dir with name.
func File(name string) (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}
