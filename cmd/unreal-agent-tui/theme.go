package main

import (
	"strings"

	"charm.land/bubbles/v2/textarea"
	glamouransi "charm.land/glamour/v2/ansi"
	"charm.land/glamour/v2/styles"
	"charm.land/lipgloss/v2"
	"github.com/charmbracelet/x/ansi"
)

type theme struct {
	Background    string
	Foreground    string
	Muted         string
	Hint          string
	Status        string
	StatusActive  string
	Surface       string
	User          string
	Selection     string
	Accent        string
	Success       string
	PendingDim    string
	PendingBright string
	Warning       string
	Error         string
}

func defaultTheme() theme {
	return theme{
		Background: "#15181e", Foreground: "#d9e2ec", Muted: "#a3adba", Hint: "#697585",
		Status: "#252f3c", StatusActive: "#405473", Surface: "#1b2029", User: "#252f3c", Selection: "#344357",
		Accent: "#84b9ff", Success: "#8ccb9e", Warning: "#e6ba73", PendingDim: "#705016", PendingBright: "#ffe28a", Error: "#ff8f91",
	}
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
	style.Code.Color, style.Code.BackgroundColor = &t.Accent, &t.Surface
	style.BlockQuote.Color = &t.Muted
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
