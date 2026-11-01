package main

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/unreallabsai/unreal-agent/cmd/internal/review/render"
	"github.com/unreallabsai/unreal-agent/review/findings"
)

// reviewSetup is what /review needs from startup: the session's findings
// checkpoint, the TUI's model choice, and the environment for the child.
type reviewSetup struct {
	out  string
	args []string
	env  []string
}

// newReviewSetup keeps the checkpoint beside the session's operation output.
// A nil env inherits the TUI's environment unchanged.
func newReviewSetup(opts options, directory string, env []string) reviewSetup {
	setup := reviewSetup{out: filepath.Join(directory, "operations", opts.sessionID, "review.jsonl"), env: env}
	if opts.model != "" {
		setup.args = append(setup.args, "--model", opts.model)
	}
	if opts.effort != "" {
		setup.args = append(setup.args, "--thinking-level", opts.effort)
	}
	return setup
}

type reviewFinished struct {
	err    error
	stderr string
}

// startReview runs "<self> review run" in the TUI's working directory. User
// arguments come last, so they override the TUI's model and effort.
func (m *model) startReview(rest string) tea.Cmd {
	m.composer.Reset()
	if rest == "cancel" {
		if m.reviewing == nil {
			m.append("Review", "No review running.")
			return nil
		}
		_ = m.reviewing.Process.Signal(os.Interrupt)
		return nil
	}
	if m.reviewing != nil {
		m.append("Review", "A review is already running. /review cancel stops it.")
		return nil
	}
	self, err := os.Executable()
	if err != nil {
		m.append("Review", err.Error())
		return nil
	}
	user := strings.Fields(rest)
	if slices.ContainsFunc(user, func(arg string) bool {
		name, _, _ := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		return strings.HasPrefix(arg, "-") && name == "out"
	}) {
		m.append("Review", "/review writes its findings to "+m.review.out+"; --out is not allowed.")
		return nil
	}
	args := append([]string{"review", "run", "--out", m.review.out}, m.review.args...)
	if report, err := findings.ReadFile(m.review.out); err == nil && report.Complete() {
		args = append(args, "--fresh")
	}
	cmd := exec.CommandContext(m.ctx, self, append(args, user...)...)
	// Interrupt, not kill, so the review saves its checkpoint when the TUI exits.
	cmd.Cancel = func() error { return cmd.Process.Signal(os.Interrupt) }
	cmd.Env = m.review.env
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	if err := cmd.Start(); err != nil {
		m.append("Review", err.Error())
		return nil
	}
	m.reviewing = cmd
	return func() tea.Msg { err := cmd.Wait(); return reviewFinished{err, stderr.String()} }
}

// finishReview shows a failed or paused review, or sends the findings to the
// agent. The submitted message is also the transcript entry.
func (m *model) finishReview(msg reviewFinished) tea.Cmd {
	m.reviewing = nil
	if msg.err != nil {
		text := strings.TrimSpace(msg.stderr)
		if text == "" {
			text = msg.err.Error()
		}
		m.append("Review", text)
		return nil
	}
	report, err := findings.ReadFile(m.review.out)
	var markdown bytes.Buffer
	if err == nil {
		err = render.Markdown(&markdown, report)
	}
	if err != nil {
		m.append("Review", err.Error())
		return nil
	}
	draft := m.composer.Value()
	cmd := m.startSubmission("Review findings (" + m.review.out + "):\n\n" + markdown.String())
	m.composer.SetValue(draft)
	return cmd
}
