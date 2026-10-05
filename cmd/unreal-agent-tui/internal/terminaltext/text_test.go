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
