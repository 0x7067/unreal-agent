package main

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/unreallabsai/unreal-agent/review/findings"
)

// Re-executing the test binary never enters the paid review implementation.
// The helper exercises the real exec/interrupt/Wait path with local findings.
func TestMain(m *testing.M) {
	if len(os.Args) > 1 && os.Args[1] == "review" {
		os.Exit(reviewTestProcess())
	}
	os.Exit(m.Run())
}

func reviewTestProcess() int {
	mode := os.Getenv("UNREAL_TUI_REVIEW_TEST")
	if mode == "" {
		return 99
	}
	var out string
	for i, arg := range os.Args {
		if arg == "--out" && i+1 < len(os.Args) {
			out = os.Args[i+1]
		}
	}
	if out == "" {
		return 98
	}
	args, _ := json.Marshal(os.Args[1:])
	if err := os.WriteFile(out+".args", args, 0o600); err != nil {
		return 97
	}
	switch mode {
	case "wait":
		interrupts := make(chan os.Signal, 1)
		signal.Notify(interrupts, os.Interrupt)
		defer signal.Stop(interrupts)
		if err := os.WriteFile(out+".ready", nil, 0o600); err != nil {
			return 96
		}
		<-interrupts
		_ = findings.WriteFile(out, findings.Report{Run: &findings.Run{Status: findings.StatusRunning}})
		fmt.Fprintln(os.Stderr, "Review paused; checkpoint saved.")
		return 1
	case "fail":
		fmt.Fprintln(os.Stderr, "\x1b[2Jlocal review failed")
		return 1
	case "success":
		if err := findings.WriteFile(out, findings.Report{Summary: findings.CleanVerdict}); err != nil {
			return 95
		}
		return 0
	default:
		return 94
	}
}

func reviewTestModel(t *testing.T, mode string) model {
	t.Helper()
	m := toolsModel(t, defaultTheme, 120, 40, 0)
	m.inputs = &promptWriter{}
	m.review = newReviewSetup(options{model: "fake-model", effort: "high"}, t.TempDir(), "created-session", append(os.Environ(), "UNREAL_TUI_REVIEW_TEST="+mode))
	if err := os.MkdirAll(filepath.Dir(m.review.out), 0o700); err != nil {
		t.Fatal(err)
	}
	return m
}

func waitReview(t *testing.T, m *model, cmd tea.Cmd) reviewFinished {
	t.Helper()
	if cmd == nil || m.reviewing == nil {
		t.Fatal("review did not start")
	}
	done := make(chan tea.Msg, 1)
	go func() { done <- cmd() }()
	select {
	case msg := <-done:
		return msg.(reviewFinished)
	case <-time.After(5 * time.Second):
		_ = m.reviewing.Process.Kill()
		<-done
		t.Fatal("review helper did not finish")
		return reviewFinished{}
	}
}

func TestReviewSetupUsesCreatedSession(t *testing.T) {
	env := []string{"CUSTOM=value"}
	setup := newReviewSetup(options{model: "selected-model", effort: "high"}, "/session-state", "created-session", env)
	if setup.out != filepath.Join("/session-state", "operations", "created-session", "review.jsonl") || !slices.Equal(setup.args, []string{"--model", "selected-model", "--thinking-level", "high"}) || !slices.Equal(setup.env, env) {
		t.Fatalf("setup: %+v", setup)
	}
}

func TestReviewRunSendsFindingsAndPreservesDraft(t *testing.T) {
	m := reviewTestModel(t, "success")
	m.composer.SetValue("/review --model override")
	cmd := m.startPrompt()
	if m.composer.Value() != "" || m.reviewing == nil || m.sending {
		t.Fatal("review did not start independently of normal sending")
	}
	m.composer.SetValue("unfinished draft")
	msg := waitReview(t, &m, cmd)
	if msg.err != nil {
		t.Fatalf("review failed: %v %s", msg.err, msg.stderr)
	}
	updated, cmd := m.Update(msg)
	m = updated.(model)
	if m.reviewing != nil || m.composer.Value() != "unfinished draft" {
		t.Fatal("completion lost draft or retained process")
	}
	m = applyFileCommands(m, cmd)
	writer := m.inputs.(*promptWriter)
	if len(writer.inputs) != 1 {
		t.Fatalf("findings not sent: %+v", writer.inputs)
	}
	var prompt string
	if err := json.Unmarshal(writer.inputs[0].Payload, &prompt); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(prompt, "Review findings ("+m.review.out+"):") || !strings.Contains(prompt, findings.CleanVerdict) {
		t.Fatalf("review prompt: %q", prompt)
	}
	raw, err := os.ReadFile(m.review.out + ".args")
	if err != nil {
		t.Fatal(err)
	}
	var args []string
	if err := json.Unmarshal(raw, &args); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(args, []string{"review", "run", "--out", m.review.out, "--model", "fake-model", "--thinking-level", "high", "--model", "override"}) {
		t.Fatalf("child args: %q", args)
	}
	// Completed checkpoints start fresh on the next invocation.
	cmd = m.startReview("")
	msg = waitReview(t, &m, cmd)
	_ = m.finishReview(msg)
	raw, _ = os.ReadFile(m.review.out + ".args")
	_ = json.Unmarshal(raw, &args)
	if !slices.Contains(args, "--fresh") {
		t.Fatalf("completed checkpoint not refreshed: %q", args)
	}
}

func TestReviewCancelAndDuplicateRun(t *testing.T) {
	m := reviewTestModel(t, "wait")
	// Cancellation works even while an ordinary inbox send is pending.
	m.sending = true
	m.composer.SetValue("/review")
	cmd := m.startPrompt()
	if m.reviewing == nil {
		t.Fatal("review blocked by normal send")
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(m.review.out + ".ready"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			_ = m.reviewing.Process.Kill()
			_ = cmd()
			t.Fatal("helper did not become ready")
		}
		time.Sleep(5 * time.Millisecond)
	}
	process := m.reviewing
	if m.startReview("") != nil || m.reviewing != process || !strings.Contains(m.lines[len(m.lines)-1].text, "already running") {
		t.Fatal("duplicate review started")
	}
	m.composer.SetValue("/review cancel")
	if m.startPrompt() != nil {
		t.Fatal("cancel started a second wait")
	}
	msg := waitReview(t, &m, cmd)
	if msg.err == nil {
		t.Fatal("cancel unexpectedly succeeded")
	}
	if m.finishReview(msg) != nil || m.reviewing != nil || !strings.Contains(m.lines[len(m.lines)-1].text, "checkpoint saved") {
		t.Fatal("cancel did not report paused review")
	}
	if len(m.inputs.(*promptWriter).inputs) != 0 {
		t.Fatal("canceled review sent findings")
	}
	if report, err := findings.ReadFile(m.review.out); err != nil || report.Complete() {
		t.Fatalf("checkpoint not retained: %+v, %v", report, err)
	}
	if m.startReview("cancel") != nil || m.lines[len(m.lines)-1].text != "No review running." {
		t.Fatal("idle cancel not reported")
	}
}

func TestReviewRejectsOutOverride(t *testing.T) {
	for _, arg := range []string{"--out /tmp/other", "--out=/tmp/other", "-out /tmp/other"} {
		m := reviewTestModel(t, "success")
		if m.startReview(arg) != nil || m.reviewing != nil || !strings.Contains(m.lines[len(m.lines)-1].text, "--out is not allowed") {
			t.Fatalf("out accepted: %q", arg)
		}
	}
}

func TestReviewFailureAndMissingReport(t *testing.T) {
	m := reviewTestModel(t, "fail")
	cmd := m.startReview("")
	msg := waitReview(t, &m, cmd)
	if m.finishReview(msg) != nil || m.lines[len(m.lines)-1].text != "local review failed" || m.reviewing != nil {
		t.Fatalf("failure: %+v", m.lines)
	}
	m.composer.SetValue("keep draft")
	if m.finishReview(reviewFinished{err: errors.New("interrupted")}) != nil || m.lines[len(m.lines)-1].text != "interrupted" {
		t.Fatal("missing stderr did not use process error")
	}
	if m.finishReview(reviewFinished{}) != nil || !strings.Contains(m.lines[len(m.lines)-1].text, "review.jsonl") || m.composer.Value() != "keep draft" {
		t.Fatal("missing report not surfaced without losing draft")
	}
	if err := os.WriteFile(m.review.out, []byte("not json"), 0o600); err != nil {
		t.Fatal(err)
	}
	if m.finishReview(reviewFinished{}) != nil || !strings.Contains(m.lines[len(m.lines)-1].text, "parse") {
		t.Fatal("malformed report not surfaced")
	}
	if len(m.inputs.(*promptWriter).inputs) != 0 {
		t.Fatal("failed review submitted findings")
	}
}

// Exit must reap the child even if Bubble Tea never executes its wait command.
func TestReviewExitCancelsAndJoinsChild(t *testing.T) {
	m := reviewTestModel(t, "wait")
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	m.ctx = ctx
	cmd := m.startReview("")
	deadline := time.Now().Add(5 * time.Second)
	for {
		if _, err := os.Stat(m.review.out + ".ready"); err == nil {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			<-m.reviewDone
			t.Fatal("review helper not ready")
		}
		time.Sleep(5 * time.Millisecond)
	}
	done := m.reviewDone
	process := m.reviewing
	if process.WaitDelay != 5*time.Second {
		t.Fatal("review exit is not bounded")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(7 * time.Second):
		_ = process.Process.Kill()
		<-done
		t.Fatal("review child was not joined after cancellation")
	}
	if process.ProcessState == nil {
		t.Fatal("done closed before process Wait")
	}
	msg := cmd().(reviewFinished)
	if msg.err == nil || !strings.Contains(msg.stderr, "checkpoint saved") {
		t.Fatalf("exit did not interrupt gracefully: %+v", msg)
	}
}
