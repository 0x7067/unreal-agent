package main

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachclient"
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/cmd/internal/resident"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/internal/testtmp"
)

func TestAttachOptionsNeverBootstrapConfiguration(t *testing.T) {
	for _, args := range [][]string{{"-h"}, {"-provider", "untrusted"}, {"-load-dotenv"}, {"-worker-executable", "/untrusted"}, {"-runtime-directory", "/tmp/private", "-session-directory", "/tmp/state", "-session", "id"}} {
		var out bytes.Buffer
		opts, err := parseAttachOptions(args, func(name string) string { t.Fatalf("unexpected environment/config read %s", name); return "" }, &out)
		if len(args) > 2 && (err != nil || opts.ui.sessionID != "id") {
			t.Fatal(opts, err)
		}
	}
}
func TestAttachedUIControlsAndUncertainty(t *testing.T) {
	m := toolsModel(t, defaultTheme, 100, 30, 0)
	m.attached = true
	m.attachConnected = true
	m.responding = true
	m.workerGeneration = 1
	m.renderConversation()
	for _, text := range []string{"/reload", "/compact", "/agents stop name", "/builds"} {
		m.composer.SetValue(text)
		before := len(m.pendingSubmissions)
		if cmd := m.startSubmission(text); cmd != nil || m.composer.Value() != text || len(m.pendingSubmissions) != before {
			t.Fatal("attached local control dispatched", text)
		}
	}
	if strings.Contains(m.View().Content, "/reload at idle boundary") || strings.Contains(m.View().Content, "not checked · /builds") {
		t.Fatal("attached view advertises forbidden controls")
	}
	m.lines = []message{{label: "You", text: "message", inputID: "stable", submission: submissionSending}}
	m.applyAttachedPending(inbox.Input{ID: "queued", Kind: inbox.InputExternal, Payload: []byte(`"queued input"`)})
	index := m.submissionIndex("queued")
	if index < 0 || m.lines[index].submission != submissionQueued {
		t.Fatal("pending snapshot omitted or marked persisted")
	}
	m.applyInput(inbox.Input{ID: "queued", Kind: inbox.InputExternal, Payload: []byte(`"queued input"`)})
	if m.lines[index].submission != submissionPersisted {
		t.Fatal("pending did not reconcile with persistence")
	}
	m.pendingSubmissions = map[inbox.ID]struct{}{"stable": {}}
	m.finishSubmission(submitted{id: "stable", text: "message", err: fmt.Errorf("%w: stable", attachclient.ErrUncertain)})
	if m.lines[0].submission != submissionUncertain || strings.Contains(m.lines[0].submission.status(), "not persisted") {
		t.Fatal("uncertain submission misrepresented as rejected")
	}
	updated, _ := m.Update(attachDisconnected{errors.New("EOF")})
	m = updated.(model)
	if m.attachConnected || !strings.Contains(m.View().Content, "host state unknown") {
		t.Fatal("disconnect is not explicit")
	}
	m.composer.SetValue("keep draft")
	if cmd := m.startSubmission("keep draft"); cmd != nil || m.composer.Value() != "keep draft" {
		t.Fatal("disconnected send lost draft")
	}
	// The actual quit key remains a local Bubble Tea quit, not a stop input.
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("detach key missing")
	}
}

func TestAttachedUIHeadlessTurnAndLateToolOutput(t *testing.T) {
	for _, withTool := range []bool{false, true} {
		t.Run(fmt.Sprint("tool=", withTool), func(t *testing.T) {
			t.Setenv("UNREAL_TUI_DIAGNOSTICS_HELPER", "1")
			t.Setenv("OPENAI_API_KEY", "fake")
			t.Setenv("UNREAL_HARNESS_LLM_API_KEY", "fake")
			root, err := testtmp.MkdirTemp("u12-")
			if err != nil {
				t.Fatal(err)
			}
			defer os.RemoveAll(root)
			root, err = filepath.EvalSymlinks(root)
			if err != nil {
				t.Fatal(err)
			}
			ctx, cancel := context.WithTimeout(t.Context(), 35*time.Second)
			defer cancel()
			var calls atomic.Int64
			started := make(chan struct{}, 1)
			release := make(chan struct{})
			var once sync.Once
			marker, releaseFile := filepath.Join(root, "shell-start"), filepath.Join(root, "shell-release")
			answer := "one answer survived TUI replacement"
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				_, _ = io.Copy(io.Discard, r.Body)
				n := calls.Add(1)
				var output []any
				if withTool && n == 1 {
					command := fmt.Sprintf("printf 'HEADLESS_PARTIAL_OUTPUT\\n'; touch '%s'; while ! test -f '%s'; do sleep 0.02; done; printf 'HEADLESS_FINAL_OUTPUT\\n'", marker, releaseFile)
					args, _ := json.Marshal(map[string]string{"command": command})
					output = []any{map[string]any{"type": "function_call", "id": "tool", "call_id": "shell-call", "name": "Bash", "arguments": string(args)}}
				} else {
					if n == 1 {
						started <- struct{}{}
						select {
						case <-release:
						case <-r.Context().Done():
							return
						}
					}
					output = []any{map[string]any{"id": "answer", "type": "message", "role": "assistant", "content": []any{map[string]any{"type": "output_text", "text": answer}}}}
				}
				w.Header().Set("Content-Type", "text/event-stream")
				data, _ := json.Marshal(map[string]any{"type": "response.completed", "response": map[string]any{"id": fmt.Sprint(n), "status": "completed", "output": output, "usage": map[string]int{"input_tokens": 1, "output_tokens": 1}}})
				_, _ = fmt.Fprintf(w, "data: %s\n\n", data)
			}))
			defer server.Close()
			defer once.Do(func() { close(release) })
			state := filepath.Join(root, "state")
			exe, _ := os.Executable()
			hostCtx, stopHost := context.WithCancel(ctx)
			hostDone := make(chan error, 1)
			ready := make(chan resident.Info, 1)
			go func() {
				hostDone <- resident.Serve(hostCtx, resident.Config{RuntimeDirectory: root, Worker: workerhost.Config{Command: []string{exe, "worker"}, Bootstrap: workerproto.Bootstrap{Directory: state, Workspace: root, SessionID: "attached", Provider: "openai", BaseURL: server.URL, Model: llm.Model{ID: "fake"}, MaxAttempts: 1, Shell: "/bin/sh"}}}, func(i resident.Info) { ready <- i })
			}()
			defer func() {
				once.Do(func() { close(release) })
				_ = os.WriteFile(releaseFile, nil, 0600)
				stopHost()
				select {
				case err := <-hostDone:
					if err != nil {
						t.Error(err)
					}
				case <-time.After(25 * time.Second):
					t.Error("headless host join unconfirmed")
				}
			}()
			select {
			case <-ready:
			case err := <-hostDone:
				t.Fatal("host startup", err)
			case <-ctx.Done():
				t.Fatal(ctx.Err())
			}
			dial := func() *attachclient.Client {
				t.Helper()
				for {
					c, err := attachclient.Dial(ctx, root, state, "attached")
					if err == nil {
						return c
					}
					if !strings.Contains(err.Error(), "busy") && !errors.Is(err, io.EOF) {
						t.Fatal(err)
					}
					select {
					case <-ctx.Done():
						t.Fatal(ctx.Err())
					case <-time.After(5 * time.Millisecond):
					}
				}
			}
			seed := dial()
			seedSnapshot := seed.Snapshot()
			seedRead := make(chan struct{})
			go func() {
				defer close(seedRead)
				for {
					if _, err := seed.Next(); err != nil {
						return
					}
				}
			}()
			if err := seed.Submit(ctx, inbox.Input{ID: "stable-seed", Kind: inbox.InputExternal, Payload: []byte(`"start once"`)}); err != nil {
				t.Fatal(err)
			}
			if withTool {
				for {
					if _, err := os.Stat(marker); err == nil {
						break
					}
					select {
					case <-ctx.Done():
						t.Fatal("shell not started")
					case <-time.After(10 * time.Millisecond):
					}
				}
			} else {
				select {
				case <-started:
				case <-ctx.Done():
					t.Fatal("model not started")
				}
			}
			seed.Close()
			<-seedRead
			first := dial()
			snap := first.Snapshot()
			opts := options{theme: mustTheme(t), sessionID: "attached"}
			ui := newAttachedModel(ctx, first, snap, opts)
			if snap.Hello.Incarnation != seedSnapshot.Hello.Incarnation || snap.Header.Status.Generation != 1 || ui.lastItemSequence != snap.Header.Head || ui.submissionIndex("stable-seed") < 0 {
				t.Fatal("attached view does not reconstruct same host/turn")
			}
			if withTool {
				if len(ui.tools) != 1 || ui.tools[0].name != "Bash" || ui.tools[0].status == "Completed" {
					t.Fatal("late join did not reconstruct live tool card", ui.tools)
				}
			} else if !ui.responding {
				t.Fatal("blocked turn rendered idle")
			}
			first.Close() // No host Close/Stop/lease handoff from a frontend.
			lease, err := localfile.AcquireLease(state, "attached")
			if err == nil {
				lease.Close()
				t.Fatal("detach released worker lease")
			}
			once.Do(func() { close(release) })
			if withTool {
				if err := os.WriteFile(releaseFile, nil, 0600); err != nil {
					t.Fatal(err)
				}
			}
			store, err := localfile.New(state)
			if err != nil {
				t.Fatal(err)
			}
			// The session file is durable before the host publishes the item.
			// Record the stored head that contains the answer, then wait only
			// until the attach snapshot's head reaches it. The UI on that
			// snapshot is asserted once.
			var committed sessionstore.Sequence
			for {
				var after sessionstore.Sequence
				var high sessionstore.Sequence
				sawAnswer := false
				for {
					page, err := store.Items(ctx, "attached", after, 100)
					if err != nil {
						t.Fatal(err)
					}
					data, _ := json.Marshal(page.Items)
					if bytes.Contains(data, []byte(answer)) {
						sawAnswer = true
					}
					for _, item := range page.Items {
						if item.Sequence > high {
							high = item.Sequence
						}
					}
					if !page.More {
						break
					}
					after = page.NextAfter
				}
				if sawAnswer {
					committed = high
					break
				}
				select {
				case <-ctx.Done():
					t.Fatal("answer not committed")
				case <-time.After(10 * time.Millisecond):
				}
			}
			var final attachclient.Snapshot
			var kept *attachclient.Client
			for {
				last := dial()
				final = last.Snapshot()
				if final.Header.Status == nil || final.Hello.Incarnation != snap.Hello.Incarnation || final.Header.Status.Generation != snap.Header.Status.Generation {
					gotGen := uint64(0)
					if final.Header.Status != nil {
						gotGen = final.Header.Status.Generation
					}
					_ = last.Close()
					t.Fatalf("fresh UI replaced worker: incarnation %s -> %s generation %d -> %d", snap.Hello.Incarnation, final.Hello.Incarnation, snap.Header.Status.Generation, gotGen)
				}
				if final.Header.Head >= committed {
					kept = last
					break
				}
				_ = last.Close()
				select {
				case <-ctx.Done():
					t.Fatalf("publisher head %d did not reach stored sequence %d", final.Header.Head, committed)
				case <-time.After(10 * time.Millisecond):
				}
			}
			fresh := newAttachedModel(ctx, kept, final, opts)
			_ = kept.Close()
			count := 0
			for _, line := range fresh.lines {
				if line.text == answer {
					count++
				}
			}
			if fresh.responding || count != 1 {
				t.Fatalf("fresh UI did not converge: responding=%v answers=%d head=%d stored=%d", fresh.responding, count, final.Header.Head, committed)
			}
			expected := int64(1)
			if withTool {
				expected = 2
				if len(fresh.tools) != 1 || !strings.Contains(fresh.tools[0].output, "HEADLESS_FINAL_OUTPUT") {
					t.Fatal("committed tool output missing", fresh.tools)
				}
			}
			if calls.Load() != expected {
				t.Fatalf("reconnect repeated model calls: got %d want %d", calls.Load(), expected)
			}
			t.Logf("same incarnation %s generation=1; calls=%d; fresh prefix H=%d; answer once", snap.Hello.Incarnation, calls.Load(), final.Header.Head)
		})
	}
}
func mustTheme(t *testing.T) theme {
	t.Helper()
	th, err := loadTheme(defaultTheme)
	if err != nil {
		t.Fatal(err)
	}
	return th
}

func TestAttachedSnapshotUnavailablePendingIsVisible(t *testing.T) {
	snapshot := attachclient.Snapshot{Header: attachproto.Event{Status: &attachproto.Status{Generation: 1}, Model: &llm.Model{ID: "fake"}, PendingAvailable: false}}
	m := newAttachedModel(t.Context(), nil, snapshot, options{theme: mustTheme(t), sessionID: "hosted"})
	if !strings.Contains(m.View().Content, "Pending ingress view unavailable") {
		t.Fatal("incomplete mirror silently presented as complete")
	}
}
