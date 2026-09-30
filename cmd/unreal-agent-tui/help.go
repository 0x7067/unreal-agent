package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/colorprofile"
)

func printHelp(flags *flag.FlagSet, output io.Writer) {
	palette, err := loadTheme("default")
	if err != nil {
		_, _ = fmt.Fprintln(output, err)
		return
	}
	heading := textStyle(palette.Accent).Bold(true)
	var help strings.Builder
	fmt.Fprintf(&help, "%s\n  unreal-agent-lite -model MODEL [flags]\n\nRun in your workspace.\n\n%s\n",
		heading.Render("Unreal Agent"), heading.Render("Options"))
	flags.VisitAll(func(option *flag.Flag) {
		value, usage := flag.UnquoteUsage(option)
		name := "-" + option.Name
		if value != "" {
			name += " " + value
		}
		fmt.Fprintf(&help, "  %s %s\n", heading.Render(fmt.Sprintf("%-22s", name)), usage)
	})
	fmt.Fprintf(&help, "\n%s\n", heading.Render("Themes"))
	entries, err := themeFiles.ReadDir("themes")
	if err != nil {
		_, _ = fmt.Fprintln(output, err)
		return
	}
	names := []string{"default"}
	for _, entry := range entries {
		if entry.Name() != "default.json" && strings.HasSuffix(entry.Name(), ".json") {
			names = append(names, strings.TrimSuffix(entry.Name(), ".json"))
		}
	}
	writer := colorprofile.NewWriter(output, os.Environ())
	for _, name := range names {
		palette, err := loadTheme(name)
		if err != nil {
			fmt.Fprintf(&help, "  %s: %v\n", name, err)
			continue
		}
		help.WriteString("  ")
		if writer.Profile >= colorprofile.ANSI {
			for _, color := range []string{palette.Background, palette.Surface, palette.Accent, palette.Success, palette.Warning, palette.Error} {
				help.WriteString(textStyle(color).Render("██"))
			}
			help.WriteString("  ")
		}
		help.WriteString(name + "\n")
	}
	fmt.Fprintf(&help, "\n%s\n  unreal-agent-lite -provider openai-codex -model gpt-5.6-luna\n  unreal-agent-lite -provider openai-codex -model gpt-5.6-luna -theme catppuccin-dark\n\nSessions: $XDG_STATE_HOME/unreal-agent/sessions (default ~/.local/state/unreal-agent/sessions)\n\nCodex login: codex -c 'cli_auth_credentials_store=\"file\"' login\nAPI key: UNREAL_HARNESS_LLM_API_KEY\n",
		heading.Render("Examples"))
	_, _ = io.WriteString(writer, help.String())
}
