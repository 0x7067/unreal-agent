package agent

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// confinedShell writes a /bin/sh wrapper in dir that gives the review shell an
// empty HOME and XDG directories under dir, and drops the variables that point
// tools at credentials. git's ssh loads no keys, because ssh finds them through
// the passwd home, not HOME. gh, git and codex then find none of the user's
// credentials. This is not a sandbox: the shell still runs as the user, a
// direct ssh still loads the user's keys, and absolute paths stay readable.
func confinedShell(dir string) (string, error) {
	home := filepath.Join(dir, "home")
	if err := os.MkdirAll(home, 0o700); err != nil {
		return "", fmt.Errorf("create review shell home: %w", err)
	}
	script := "#!/bin/sh\n" +
		"HOME='" + strings.ReplaceAll(home, "'", `'\''`) + "'\n" +
		`XDG_CONFIG_HOME="$HOME/.config" XDG_DATA_HOME="$HOME/.local/share" XDG_STATE_HOME="$HOME/.local/state" XDG_CACHE_HOME="$HOME/.cache"` + "\n" +
		"export HOME XDG_CONFIG_HOME XDG_DATA_HOME XDG_STATE_HOME XDG_CACHE_HOME\n" +
		"unset CODEX_HOME GH_CONFIG_DIR GNUPGHOME SSH_AUTH_SOCK GIT_CONFIG_GLOBAL GIT_CONFIG_SYSTEM GIT_CONFIG_COUNT GIT_CONFIG_PARAMETERS GIT_ASKPASS SSH_ASKPASS\n" +
		"GIT_SSH_COMMAND='ssh -F /dev/null -o IdentityFile=none -o IdentitiesOnly=yes'\n" +
		"GIT_CONFIG_NOSYSTEM=1\n" +
		"export GIT_SSH_COMMAND GIT_CONFIG_NOSYSTEM\n" +
		`exec /bin/sh "$@"` + "\n"
	path := filepath.Join(dir, "sh")
	if err := os.WriteFile(path, []byte(script), 0o700); err != nil {
		return "", fmt.Errorf("write review shell: %w", err)
	}
	return path, nil
}
