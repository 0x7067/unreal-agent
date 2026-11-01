package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// confinedShell writes a /bin/sh wrapper in dir that gives the review shell an
// empty HOME and XDG directories under dir, and drops CODEX_HOME,
// GH_CONFIG_DIR, GNUPGHOME and SSH_AUTH_SOCK. Tools such as gh, git, ssh and
// codex then find none of the user's credentials. This is not a sandbox: the
// shell still runs as the user and can read absolute paths.
func confinedShell(dir string) (string, error) {
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		return "", fmt.Errorf("create review shell home: %w", err)
	}
	script := "#!/bin/sh\n" +
		"HOME='" + strings.ReplaceAll(home, "'", `'\''`) + "'\n" +
		`XDG_CONFIG_HOME="$HOME/.config" XDG_DATA_HOME="$HOME/.local/share" XDG_STATE_HOME="$HOME/.local/state" XDG_CACHE_HOME="$HOME/.cache"` + "\n" +
		"export HOME XDG_CONFIG_HOME XDG_DATA_HOME XDG_STATE_HOME XDG_CACHE_HOME\n" +
		"unset CODEX_HOME GH_CONFIG_DIR GNUPGHOME SSH_AUTH_SOCK\n" +
		`exec /bin/sh "$@"` + "\n"
	path := filepath.Join(dir, "sh")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		return "", fmt.Errorf("write review shell: %w", err)
	}
	return path, nil
}
