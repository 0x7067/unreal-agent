package sessionpath

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func Resolve(configured string, getenv func(string) string) (string, error) {
	if configured = strings.TrimSpace(configured); configured != "" {
		return filepath.Abs(configured)
	}
	stateHome := getenv("XDG_STATE_HOME")
	if !filepath.IsAbs(stateHome) {
		userHome := getenv("HOME")
		if userHome == "" {
			var err error
			userHome, err = os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("find home directory: %w; set an absolute XDG_STATE_HOME or HOME", err)
			}
		}
		if !filepath.IsAbs(userHome) {
			return "", errors.New("set an absolute XDG_STATE_HOME or HOME")
		}
		stateHome = filepath.Join(userHome, ".local", "state")
	}
	return filepath.Join(stateHome, "unreal-agent/sessions"), nil
}
