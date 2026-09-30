package xdgpath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

func Directory(getenv func(string) string) (string, error) {
	if root := getenv("XDG_CONFIG_HOME"); filepath.IsAbs(root) {
		return filepath.Join(root, "unreal-agent"), nil
	}
	home := getenv("HOME")
	if home == "" {
		var err error
		home, err = os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("find home directory: %w; set an absolute XDG_CONFIG_HOME or HOME", err)
		}
	}
	if !filepath.IsAbs(home) {
		return "", errors.New("set an absolute XDG_CONFIG_HOME or HOME")
	}
	return filepath.Join(home, ".config", "unreal-agent"), nil
}
