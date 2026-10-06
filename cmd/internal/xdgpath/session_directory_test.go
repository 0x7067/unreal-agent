package xdgpath

import (
	"strings"
	"testing"
)

func TestSessionDirectory(t *testing.T) {
	for _, test := range []struct {
		name, stateHome, userHome, processHome, want string
		wantError                                    bool
	}{
		{name: "XDG state home", stateHome: "/state", userHome: "/home/user", want: "/state/unreal-agent/sessions"},
		{name: "XDG without home", stateHome: "/state", want: "/state/unreal-agent/sessions"},
		{name: "home fallback", userHome: "/home/user", processHome: "/process/home", want: "/home/user/.local/state/unreal-agent/sessions"},
		{name: "process home fallback", processHome: "/process/home", want: "/process/home/.local/state/unreal-agent/sessions"},
		{name: "relative XDG ignored", stateHome: "relative/state", userHome: "/home/user", want: "/home/user/.local/state/unreal-agent/sessions"},
		{name: "missing home", stateHome: "relative/state", wantError: true},
		{name: "relative home", userHome: "relative/home", processHome: "/process/home", wantError: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("HOME", test.processHome)
			env := map[string]string{"XDG_STATE_HOME": test.stateHome, "HOME": test.userHome}
			got, err := SessionDirectory(func(key string) string { return env[key] })
			if test.wantError {
				if err == nil || !strings.Contains(err.Error(), "XDG_STATE_HOME") {
					t.Fatalf("error = %v, want actionable state location error", err)
				}
			} else if err != nil || got != test.want {
				t.Fatalf("SessionDirectory = %q, %v; want %q", got, err, test.want)
			}
		})
	}
}
