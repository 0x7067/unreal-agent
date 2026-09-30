package terminaltext

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/charmbracelet/x/ansi"
)

func Clean(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	return strings.Map(func(r rune) rune {
		if unicode.IsControl(r) && r != '\n' && r != '\t' {
			return -1
		}
		return r
	}, ansi.Strip(value))
}

// CleanStyled keeps SGR formatting but removes terminal commands, including
// controls that Markdown rendering can introduce by decoding HTML entities.
func CleanStyled(value string) string {
	var out strings.Builder
	var state byte
	for len(value) > 0 {
		sequence, _, n, next := ansi.DecodeSequence(value, state, nil)
		r, _ := utf8.DecodeRuneInString(sequence)
		if !unicode.IsControl(r) || sequence == "\n" || sequence == "\t" ||
			strings.HasPrefix(sequence, "\x1b[") && strings.HasSuffix(sequence, "m") &&
				strings.Trim(sequence[2:len(sequence)-1], "0123456789;:") == "" {
			out.WriteString(sequence)
		}
		value, state = value[n:], next
	}
	return out.String()
}
