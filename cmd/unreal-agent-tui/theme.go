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
	Background          string `json:"background"`
	Foreground          string `json:"foreground"`
	Muted               string `json:"muted"`
	Hint                string `json:"hint"`
	Status              string `json:"status"`
	Surface             string `json:"surface"`
	User                string `json:"user"`
	Selection           string `json:"selection"`
	Accent              string `json:"accent"`
	Success             string `json:"success"`
	Warning             string `json:"warning"`
	Error               string `json:"error"`
	SurfaceForeground   string `json:"surface_foreground,omitempty"`
	SurfaceMuted        string `json:"surface_muted,omitempty"`
	SurfaceHint         string `json:"surface_hint,omitempty"`
	SurfaceAccent       string `json:"surface_accent,omitempty"`
	SurfaceKey          string `json:"surface_key,omitempty"`
	StatusForeground    string `json:"status_foreground,omitempty"`
	StatusAccent        string `json:"status_accent,omitempty"`
	StatusError         string `json:"status_error,omitempty"`
	UserForeground      string `json:"user_foreground,omitempty"`
	SelectionForeground string `json:"selection_foreground,omitempty"`
	CodeBackground      string `json:"code_background,omitempty"`
	Delimiter           string `json:"delimiter,omitempty"`
	// Accept legacy palette colors so existing JSON themes continue to load.
	StatusActive  string `json:"status_active,omitempty"`
	PendingDim    string `json:"pending_dim,omitempty"`
	PendingBright string `json:"pending_bright,omitempty"`
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
	for _, fallback := range []struct {
		color *string
		value string
	}{
		{&t.SurfaceForeground, t.Foreground}, {&t.SurfaceMuted, t.Muted},
		{&t.SurfaceHint, t.Hint}, {&t.SurfaceAccent, t.Accent}, {&t.SurfaceKey, t.Hint},
		{&t.StatusForeground, t.Muted}, {&t.StatusAccent, t.Accent}, {&t.StatusError, t.Error},
		{&t.UserForeground, t.Foreground}, {&t.SelectionForeground, t.Foreground},
		{&t.CodeBackground, t.Surface},
	} {
		if *fallback.color == "" {
			*fallback.color = fallback.value
		}
	}
	colors := map[string]string{
		"background": t.Background, "foreground": t.Foreground, "muted": t.Muted, "hint": t.Hint,
		"status": t.Status, "surface": t.Surface, "user": t.User,
		"selection": t.Selection, "accent": t.Accent, "success": t.Success,
		"warning": t.Warning, "error": t.Error,
		"surface_foreground": t.SurfaceForeground, "surface_muted": t.SurfaceMuted,
		"surface_hint": t.SurfaceHint, "surface_accent": t.SurfaceAccent, "surface_key": t.SurfaceKey,
		"status_foreground": t.StatusForeground, "status_accent": t.StatusAccent, "status_error": t.StatusError,
		"user_foreground": t.UserForeground, "selection_foreground": t.SelectionForeground,
		"code_background": t.CodeBackground,
	}
	for field, value := range map[string]string{
		"delimiter": t.Delimiter, "status_active": t.StatusActive,
		"pending_dim": t.PendingDim, "pending_bright": t.PendingBright,
	} {
		if value != "" {
			colors[field] = value
		}
	}
	for field, value := range colors {
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
		Text: textStyle(t.SurfaceForeground), CursorLine: textStyle(t.SurfaceForeground),
		Placeholder: textStyle(t.SurfaceMuted), Prompt: textStyle(t.SurfaceAccent),
		Selection: textStyle(t.SelectionForeground).Background(lipgloss.Color(t.Selection)),
	}
	return textarea.Styles{Focused: style, Blurred: style,
		Cursor: textarea.CursorStyle{Color: lipgloss.Color(t.SurfaceAccent)}}
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
	style.Code.Color, style.Code.BackgroundColor = &t.Accent, &t.CodeBackground
	style.BlockQuote.Color = &t.Muted
	style.CodeBlock.Color = &t.Foreground
	style.CodeBlock.Margin = new(uint(0))
	style.CodeBlock.Chroma = &glamouransi.Chroma{
		Text:          glamouransi.StylePrimitive{Color: &t.Foreground},
		Background:    glamouransi.StylePrimitive{BackgroundColor: &t.CodeBackground},
		Comment:       glamouransi.StylePrimitive{Color: &t.Hint},
		Keyword:       glamouransi.StylePrimitive{Color: &t.Accent},
		NameFunction:  glamouransi.StylePrimitive{Color: &t.Accent},
		LiteralString: glamouransi.StylePrimitive{Color: &t.Success},
		LiteralNumber: glamouransi.StylePrimitive{Color: &t.Warning},
		Error:         glamouransi.StylePrimitive{Color: &t.Error},
	}
	return style
}
