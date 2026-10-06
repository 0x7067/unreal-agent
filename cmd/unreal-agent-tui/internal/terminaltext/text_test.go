package terminaltext

import "testing"

func TestCleanExternalText(t *testing.T) {
	for _, control := range []string{"\x1b[2J", "\x1b]52;c;QVVESVQ=\a", "\x1b]0;title\a", "\x1bPpayload\x1b\\", "\x00\b\a"} {
		if got := Clean("before" + control + "after"); got != "beforeafter" {
			t.Fatalf("unsafe text: %q", got)
		}
	}
	if got := Clean("a\r\nb\rc\t界"); got != "a\nb\nc\t界" {
		t.Fatalf("text damaged: %q", got)
	}
}

func TestCleanStyledPreservesFormattingAndUnicode(t *testing.T) {
	text := "\x1b[1m界 👩‍💻 é\x1b[0m\n\t"
	for _, control := range []string{"\x1b[2J", "\x1b[>4;0m", "\x1b[?4m", "\x1b[1$m", "\x1b]52;c;QVVESVQ=\a", "\x1b]8;;file:///tmp/script.sh\x1b\\", "\x1bPpayload\x1b\\", "\x00\b\a"} {
		if got := CleanStyled(text + control + "end"); got != text+"end" {
			t.Fatalf("style lost or unsafe control retained: %q", got)
		}
	}
	for _, end := range []string{"\a", "\x1b\\"} {
		link := "\x1b]8;id=1;https://example.com" + end + text + "\x1b]8;;" + end
		if got := CleanStyled(link); got != link {
			t.Fatalf("hyperlink lost: %q", got)
		}
	}
}

func TestCleanOutput(t *testing.T) {
	for _, tt := range []struct {
		name, input, want string
	}{
		{"empty", "", ""},
		{"literal whitespace and Unicode", "  界 👩‍💻 é \t\n\n end  ", "  界 👩‍💻 é \t\n\n end  "},
		{"carriage returns", "a\r\nb\rc\r\r\nd\t\r", "a\nb\nc\n\nd\t\n"},
		{"standard styling", "\x1b[m\x1b[1;31mred\x1b[0m", "\x1b[m\x1b[1;31mred\x1b[0m"},
		{"indexed color", "\x1b[38;5;196mred\x1b[48;5;21mblue\x1b[0m", "\x1b[38;5;196mred\x1b[48;5;21mblue\x1b[0m"},
		{"true color", "\x1b[38;2;255;128;0mtext\x1b[48;2;1;2;3m!", "\x1b[38;2;255;128;0mtext\x1b[48;2;1;2;3m!"},
		{"colon colors", "\x1b[38:2::255:128:0mtext\x1b[48:5:42m!", "\x1b[38:2::255:128:0mtext\x1b[48:5:42m!"},
		{"cursor and erase", "a\x1b[H\x1b[2J\x1b[1;20H\x1b[2Kz", "az"},
		{"private CSI", "a\x1b[?25l\x1b[?1049h\x1b[>4;0m\x1b[=1mz", "az"},
		{"malformed SGR", "a\x1b[1$m\x1b[1 m\x1b[31\x00mz", "az"},
		{"OSC hyperlink", "\x1b]8;id=1;https://example.com\x1b\\label\x1b]8;;\x1b\\", "label"},
		{"OSC hyperlink BEL", "\x1b]8;;https://example.com\alabel\x1b]8;;\a", "label"},
		{"OSC title and clipboard", "a\x1b]0;title\a\x1b]2;title\x1b\\\x1b]52;c;QVVESVQ=\az", "az"},
		{"control string payload hides escapes", "a\x1b]0;title\x1b[31mhidden\az", "az"},
		{"DCS", "a\x1bP1;2|payload\x1b\\z", "az"},
		{"SOS PM APC", "a\x1bXpayload\x1b\\\x1b^payload\x1b\\\x1b_payload\x1b\\z", "az"},
		{"ESC commands", "a\x1b7\x1b8\x1b(B\x1b#8\x1bc\x1b=z", "az"},
		{"C0 and DEL", "a\x00\x01\a\b\v\f\x0e\x1f\x7fz", "az"},
		{"UTF8 C1", "a\u0080\u0085\u009c\u009b2J\u009b31mz", "az"},
		{"raw C1", "a\x80\x85\x9c\x9b2J\x9b31mz", "az"},
		{"UTF8 C1 strings", "a\u009d0;title\u009c\u0090payload\u009cz", "az"},
		{"raw C1 strings", "a\x9d0;title\x9c\x90payload\x9cz", "az"},
		{"mixed string terminators", "a\x1b]0;title\u009c\u009d0;title\x1b\\z", "az"},
		{"resynchronize ESC in CSI", "a\x1b[31\x1b[2J\x1b[32mz", "a\x1b[32mz"},
		{"styles around removed commands", "\x1b[31ma\x1b[2Jb\x1b[0m", "\x1b[31mab\x1b[0m"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := CleanOutput(tt.input)
			if got != tt.want {
				t.Fatalf("CleanOutput(%q) = %q, want %q", tt.input, got, tt.want)
			}
			if again := CleanOutput(got); again != got {
				t.Fatalf("not idempotent: %q became %q", got, again)
			}
		})
	}
}

func TestCleanOutputTruncatedControls(t *testing.T) {
	for _, control := range []string{
		"\x1b", "\x1b(", "\x1b[", "\x1b[31", "\x1b[38;2;255;",
		"\x1b[?25", "\x1b[1$", "\x1b]", "\x1b]0;hidden",
		"\x1b]8;;https://example.com", "\x1b]52;c;payload\x1b",
		"\x1bPpayload", "\x1bXpayload", "\x1b^payload", "\x1b_payload",
		"\u009b31", "\x9b31", "\u009d0;hidden", "\x9d0;hidden",
		"\u0090hidden", "\x90hidden", "\u0098hidden", "\x98hidden",
		"\u009ehidden", "\x9ehidden", "\u009fhidden", "\x9fhidden",
	} {
		t.Run(control, func(t *testing.T) {
			if got := CleanOutput("before" + control); got != "before" {
				t.Fatalf("truncated control exposed: %q", got)
			}
		})
	}
}

func TestCleanOutputControlStringTerminators(t *testing.T) {
	for _, start := range []string{"\x1b]", "\x1bP", "\x1bX", "\x1b^", "\x1b_", "\u009d", "\u0090", "\u0098", "\u009e", "\u009f", "\x9d", "\x90", "\x98", "\x9e", "\x9f"} {
		for _, end := range []string{"\x1b\\", "\u009c", "\x9c"} {
			if got := CleanOutput("before" + start + "hidden" + end + "after"); got != "beforeafter" {
				t.Errorf("control string %q terminated with %q: %q", start, end, got)
			}
		}
	}
}
