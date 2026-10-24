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

// CleanOutput sanitizes raw shell output, keeping only text, whitespace, and
// SGR styling. Unlike CleanStyled, it never permits hyperlinks. Unterminated
// terminal strings are discarded along with their payload, not exposed as text.
func CleanOutput(value string) string {
	value = strings.ReplaceAll(value, "\r\n", "\n")
	value = strings.ReplaceAll(value, "\r", "\n")
	var out strings.Builder
	out.Grow(len(value))
	for i := 0; i < len(value); {
		r, n := utf8.DecodeRuneInString(value[i:])
		// Terminals can receive C1 controls as either single bytes or UTF-8.
		if n == 1 && value[i] >= 0x80 && value[i] <= 0x9f {
			r = rune(value[i])
		}
		switch r {
		case '\x1b':
			start := i
			i += n
			if i == len(value) {
				continue
			}
			switch value[i] {
			case '[':
				i = outputCSIEnd(value, i+1)
				if safeSGR(value[start:i]) {
					out.WriteString(value[start:i])
				}
			case ']', 'P', 'X', '^', '_': // OSC, DCS, SOS, PM, APC
				i = outputStringEnd(value, i+1, value[i] == ']')
			default:
				// Other ESC commands consist of intermediates and a final.
				for i < len(value) && value[i] >= 0x20 && value[i] <= 0x2f {
					i++
				}
				if i < len(value) && value[i] >= 0x30 && value[i] <= 0x7e {
					i++
				}
			}
		case '\u009b': // C1 CSI is not an allowed SGR representation.
			i = outputCSIEnd(value, i+n)
		case '\u0090', '\u0098', '\u009d', '\u009e', '\u009f':
			i = outputStringEnd(value, i+n, r == '\u009d')
		default:
			if !unicode.IsControl(r) || r == '\n' || r == '\t' {
				out.WriteRune(r)
			}
			i += n
		}
	}
	return out.String()
}

// outputCSIEnd consumes even malformed/private CSI parameters. Only safeSGR
// decides whether the entire sequence may be emitted. A new ESC starts a fresh
// command, so malformed input cannot hide a later control from the scanner.
func outputCSIEnd(value string, start int) int {
	for i := start; i < len(value); i++ {
		if value[i] == '\x1b' {
			return i
		}
		if value[i] >= 0x40 && value[i] <= 0x7e {
			return i + 1
		}
	}
	return len(value)
}

func outputStringEnd(value string, start int, osc bool) int {
	for i := start; i < len(value); {
		if value[i] == '\x1b' && i+1 < len(value) && value[i+1] == '\\' {
			return i + 2
		}
		r, n := utf8.DecodeRuneInString(value[i:])
		if r == '\u009c' || value[i] == 0x9c || osc && r == '\a' {
			return i + n
		}
		i += n
	}
	return len(value)
}

func safeSGR(sequence string) bool {
	return strings.HasPrefix(sequence, "\x1b[") && strings.HasSuffix(sequence, "m") &&
		strings.Trim(sequence[2:len(sequence)-1], "0123456789;:") == ""
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
			safeSGR(sequence) || safeHyperlink(sequence) {
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
