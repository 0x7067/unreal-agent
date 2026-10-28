package residentcli

import (
	"bytes"
	"context"
	"strings"
	"testing"
)

func TestHostRetireRequiresExplicitFencesBeforeDial(t *testing.T) {
	for _, args := range [][]string{nil, {"--session", "private"}, {"--runtime-directory", "/not-used", "--session-directory", "/not-used", "--session", "private", "--expected-incarnation", "old"}} {
		var out, stderr bytes.Buffer
		err := Run(context.Background(), append([]string{"host-retire"}, args...), func(string) string { return "" }, strings.NewReader(""), &out, &stderr)
		if err == nil || !strings.Contains(err.Error(), "requires explicit") || out.Len() != 0 {
			t.Fatalf("unfenced retirement %v: %v output=%q", args, err, out.String())
		}
	}
	if !Handles([]string{"host-retire"}) {
		t.Fatal("public retirement command not routed")
	}
}
