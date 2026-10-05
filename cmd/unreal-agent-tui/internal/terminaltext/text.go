package terminaltext

import (
	"net/url"
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

// CleanStyled keeps SGR formatting and web links but removes terminal commands, including
// controls that Markdown rendering can introduce by decoding HTML entities.
func CleanStyled(value string) string {
	var out strings.Builder
	var state byte
	for len(value) > 0 {
		sequence, _, n, next := ansi.DecodeSequence(value, state, nil)
		r, _ := utf8.DecodeRuneInString(sequence)
		if !unicode.IsControl(r) || sequence == "\n" || sequence == "\t" ||
			strings.HasPrefix(sequence, "\x1b[") && strings.HasSuffix(sequence, "m") &&
				strings.Trim(sequence[2:len(sequence)-1], "0123456789;:") == "" || safeHyperlink(sequence) {
			out.WriteString(sequence)
		}
		value, state = value[n:], next
	}
	return out.String()
}

func safeHyperlink(sequence string) bool {
	payload, ok := strings.CutPrefix(sequence, "\x1b]8;")
	if !ok {
		return false
	}
	payload, ended := strings.CutSuffix(payload, "\x1b\\")
	if !ended {
		payload, ended = strings.CutSuffix(payload, "\a")
	}
	if !ended {
		return false
	}
	_, target, ok := strings.Cut(payload, ";")
	if !ok || target == "" {
		return ok
	}
	u, err := url.Parse(target)
	return err == nil && (u.Scheme == "https" || u.Scheme == "http") && u.Hostname() != ""
}
