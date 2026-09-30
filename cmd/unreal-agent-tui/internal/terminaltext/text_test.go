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
