package main

import (
	"context"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"strings"
	"testing"
)

type inspectingWriter struct{ submissions int }

func (w *inspectingWriter) Submit(context.Context, inbox.Input) error { w.submissions++; return nil }
func (w *inspectingWriter) CheckInstalled(context.Context) (workerhost.Status, error) {
	return workerhost.Status{Build: "old-running", InstalledBuild: "new-installed", ReloadState: "reload required"}, nil
}
func TestBuildsCommandIsLocalAndSeparatesExecutingFromInstalled(t *testing.T) {
	m := toolsModel(t, defaultTheme, 100, 30, 0)
	w := &inspectingWriter{}
	m.inputs = w
	m.workerBuild = "old-running"
	cmd := m.startSubmission("/builds")
	if cmd == nil {
		t.Fatal("missing build inspection")
	}
	m = submissionUpdate(m, cmd())
	if w.submissions != 0 {
		t.Fatal("build command became model input")
	}
	if m.workerBuild != "old-running" || m.installedBuild != "new-installed" {
		t.Fatal("conflated installed/running")
	}
	last := m.lines[len(m.lines)-1].text
	for _, part := range []string{"old-running", "new-installed", "restart", "idle boundary"} {
		if !strings.Contains(last, part) {
			t.Fatalf("missing %q in %s", part, last)
		}
	}
}
