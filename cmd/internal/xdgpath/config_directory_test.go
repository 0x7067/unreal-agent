package xdgpath

import (
	"strings"
	"testing"
)

func TestConfigDirectory(t *testing.T) {
	t.Setenv("HOME", "/process/home")
	for _, test := range []struct{ name, xdg, home, want string }{
		{name: "XDG config home", xdg: "/config", home: "/home/user", want: "/config/unreal-agent"},
		{name: "XDG without home", xdg: "/config", want: "/config/unreal-agent"},
		{name: "home fallback", home: "/home/user", want: "/home/user/.config/unreal-agent"},
		{name: "process home fallback", want: "/process/home/.config/unreal-agent"},
		{name: "relative XDG ignored", xdg: "relative/config", home: "/home/user", want: "/home/user/.config/unreal-agent"},
	} {
		t.Run(test.name, func(t *testing.T) {
			path, err := ConfigDirectory(func(name string) string {
				return map[string]string{"XDG_CONFIG_HOME": test.xdg, "HOME": test.home}[name]
			})
			if err != nil || path != test.want {
				t.Fatalf("directory = %q, error = %v; want %q", path, err, test.want)
			}
		})
	}
}

func TestConfigDirectoryRejectsInvalidHome(t *testing.T) {
	t.Setenv("HOME", "")
	for _, home := range []string{"", "relative/home"} {
		_, err := ConfigDirectory(func(name string) string {
			return map[string]string{"HOME": home}[name]
		})
		if err == nil || !strings.Contains(err.Error(), "XDG_CONFIG_HOME") {
			t.Fatalf("error = %v, want config location error", err)
		}
	}
}
