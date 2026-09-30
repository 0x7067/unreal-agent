package main

import (
	"embed"
	"encoding/hex"
	"encoding/json/v2"
	"fmt"
	"os"
	"strings"

	"charm.land/bubbles/v2/textarea"
	glamouransi "charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type theme struct {
	Background    string `json:"background"`
	Foreground    string `json:"foreground"`
	Muted         string `json:"muted"`
	Hint          string `json:"hint"`
	Status        string `json:"status"`
	StatusActive  string `json:"status_active"`
	Surface       string `json:"surface"`
	User          string `json:"user"`
	Selection     string `json:"selection"`
	Accent        string `json:"accent"`
	Success       string `json:"success"`
	PendingDim    string `json:"pending_dim"`
	PendingBright string `json:"pending_bright"`
	Warning       string `json:"warning"`
	Error         string `json:"error"`
}

//go:embed themes/*.json
var themeFiles embed.FS

func loadTheme(name string) (theme, error) {
	var t theme
	var data []byte
	var err error
	if strings.HasSuffix(name, ".json") {
		data, err = os.ReadFile(name)
	} else {
		data, err = themeFiles.ReadFile("themes/" + name + ".json")
	}
	if err != nil {
		return t, fmt.Errorf("load theme %q: %w", name, err)
	}
	if err := json.Unmarshal(data, &t, json.RejectUnknownMembers(true)); err != nil {
		return t, fmt.Errorf("decode theme %q: %w", name, err)
	}
	for field, value := range map[string]string{
		"background": t.Background, "foreground": t.Foreground, "muted": t.Muted, "hint": t.Hint,
		"status": t.Status, "status_active": t.StatusActive, "surface": t.Surface, "user": t.User,
		"selection": t.Selection, "accent": t.Accent, "success": t.Success,
		"pending_dim": t.PendingDim, "pending_bright": t.PendingBright, "warning": t.Warning, "error": t.Error,
	} {
		if _, err := hex.DecodeString(strings.TrimPrefix(value, "#")); err != nil || len(value) != 7 || value[0] != '#' {
			return t, fmt.Errorf("theme %q: %s must be a #RRGGBB color", name, field)
		}
	}
	return t, nil
}

func textStyle(color string) lipgloss.Style {
	return lipgloss.NewStyle().Foreground(lipgloss.Color(color))
}

func renderSurface(style lipgloss.Style, value string) string {
	base := ansi.NewStyle().ForegroundColor(style.GetForeground()).BackgroundColor(style.GetBackground()).String()
	// Nested text styles reset colors before padding; restore the surrounding surface.
	return strings.ReplaceAll(style.Render(value), ansi.ResetStyle, ansi.ResetStyle+base) + ansi.ResetStyle
}

func (t theme) composerStyles() textarea.Styles {
	style := textarea.StyleState{
		Text: textStyle(t.Foreground), CursorLine: textStyle(t.Foreground),
		Placeholder: textStyle(t.Muted), Prompt: textStyle(t.Accent),
		Selection: textStyle(t.Foreground).Background(lipgloss.Color(t.Selection)),
	}
	return textarea.Styles{Focused: style, Blurred: style,
		Cursor: textarea.CursorStyle{Color: lipgloss.Color(t.Accent), Blink: true}}
}

func (t theme) markdownStyles() glamouransi.StyleConfig {
	style := styles.DarkStyleConfig
	style.Document.Color = &t.Foreground
	style.Document.Margin = new(uint(0))
	style.Document.BlockPrefix, style.Document.BlockSuffix = "", ""
	style.Heading.Color = &t.Accent
	for _, heading := range []*glamouransi.StyleBlock{&style.H1, &style.H2, &style.H3, &style.H4, &style.H5, &style.H6} {
		*heading = glamouransi.StyleBlock{}
	}
	style.Link.Color, style.LinkText.Color = &t.Accent, &t.Accent
	style.Image.Color, style.ImageText.Color = &t.Accent, &t.Muted
	style.HorizontalRule.Color = &t.Hint
	style.Code.Color, style.Code.BackgroundColor = &t.Accent, &t.Surface
	style.BlockQuote.Color = &t.Muted
	style.CodeBlock.Color = &t.Foreground
	style.CodeBlock.Margin = new(uint(0))
	style.CodeBlock.Chroma = &glamouransi.Chroma{
		Text:          glamouransi.StylePrimitive{Color: &t.Foreground},
		Background:    glamouransi.StylePrimitive{BackgroundColor: &t.Surface},
		Comment:       glamouransi.StylePrimitive{Color: &t.Hint},
		Keyword:       glamouransi.StylePrimitive{Color: &t.Accent},
		NameFunction:  glamouransi.StylePrimitive{Color: &t.Accent},
		LiteralString: glamouransi.StylePrimitive{Color: &t.Success},
		LiteralNumber: glamouransi.StylePrimitive{Color: &t.Warning},
		Error:         glamouransi.StylePrimitive{Color: &t.Error},
	}
	return style
}
