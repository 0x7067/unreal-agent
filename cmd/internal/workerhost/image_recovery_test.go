package workerhost

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"image"
	"image/png"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"syscall"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/ingress"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

const m22Operation operation.ID = "m22-original-image"

func m22PNG(path string, width, height int) error {
	var data bytes.Buffer
	if err := png.Encode(&data, image.NewRGBA(image.Rect(0, 0, width, height))); err != nil {
		return err
	}
	return os.WriteFile(path, data.Bytes(), 0600)
}

// This is a controlled durable-boundary producer, not an interception inside
// workerruntime. The awaiting cases really read/compute via the actual manager,
// then intentionally do not commit their completed update before owned SIGKILL.
func TestM22ImageProducer(t *testing.T) {
	cut := os.Getenv("UA_M22_IMAGE_CUT")
	if cut == "" {
		t.Skip("owned producer only")
	}
	directory, workspace := os.Getenv("UA_M22_STATE"), os.Getenv("UA_M22_WORKSPACE")
	for _, id := range []session.ID{session.ID("host-" + string(m21Session)), m21Session} {
		lease, err := localfile.AcquireLease(directory, id)
		if err != nil {
			t.Fatal(err)
		}
		defer lease.Close()
	}
	journal, err := ingress.Open(filepath.Join(directory, "ingress"), m21Session)
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	store, err := localfile.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := store.Create(t.Context(), m21Session); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(workspace, "image.png")
	if err := m22PNG(path, 2, 3); err != nil {
		t.Fatal(err)
	}
	spec, err := operation.NewViewImageSpec(path, operation.ViewImageConfig{MaxSize: 1 << 20, MaxWidth: 64, MaxHeight: 64})
	if err != nil {
		t.Fatal(err)
	}
	op := operation.Operation{ID: m22Operation, Type: spec.Type, Version: spec.Version, State: spec.State.Clone(), Status: operation.StatusReady}
	if strings.HasPrefix(cut, "awaiting-") || cut == "committed-result" {
		op.Status = operation.StatusAwaiting
	}
	if cut == "future-version" {
		op.Version++
	}
	if err := store.AppendTurn(t.Context(), m21Session, session.Turn{ID: "m22-turn", Type: session.TurnRegular}); err != nil {
		t.Fatal(err)
	}
	args, err := json.Marshal(map[string]string{"path": path})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.AppendModelResponse(t.Context(), m21Session, sessionstore.ModelResponse{TurnID: "m22-turn", Response: llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "m22-call", Name: tool.ViewImageName, Arguments: string(args)}}}}}); err != nil {
		t.Fatal(err)
	}
	persist := func(current operation.Operation) {
		t.Helper()
		if err := store.AppendToolCallStatus(t.Context(), m21Session, sessionstore.ToolCallStatus{TurnID: "m22-turn", CallID: "m22-call", Status: tool.CallStatus{WaitingFor: []operation.ID{current.ID}}, Operations: []operation.Operation{current}}); err != nil {
			t.Fatal(err)
		}
	}
	persist(op)
	if op.Status == operation.StatusAwaiting {
		ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
		defer cancel()
		manager := operation.NewLocalOperationManager(ctx)
		if err := manager.Add(op); err != nil {
			t.Fatal(err)
		}
		for {
			select {
			case update := <-manager.Updates():
				if update.Status == operation.StatusFailed || update.Status == operation.StatusUnsupported {
					t.Fatalf("actual original read failed: %+v", update)
				}
				if update.Status != operation.StatusCompleted {
					continue
				}
				state, err := operation.DecodeViewImageState(update)
				if err != nil || state.Result == nil || state.Result.OriginalWidth != 2 || state.Result.OriginalHeight != 3 || state.Result.Content == "" {
					t.Fatalf("original real read not proven: %+v %v", state, err)
				}
				if cut == "committed-result" {
					persist(update)
				}
				fmt.Printf("M22-ORIGINAL-READ:2x3\n")
				goto readFinished
			case <-ctx.Done():
				t.Fatal("original read did not complete")
			}
		}
	}
readFinished:
	fmt.Printf("M22-COMMITTED:%s\n", cut)
	for {
		time.Sleep(time.Hour)
	}
}

func TestM22OwnedImageRecovery(t *testing.T) {
	for _, cut := range []string{"ready-current", "awaiting-current", "awaiting-missing", "awaiting-corrupt", "committed-result", "future-version"} {
		t.Run(cut, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 35*time.Second)
			defer cancel()
			root := t.TempDir()
			directory, workspace := filepath.Join(root, "state"), filepath.Join(root, "workspace")
			for _, p := range []string{root, directory, workspace, filepath.Join(directory, "ingress")} {
				if err := os.MkdirAll(p, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(p, 0700); err != nil {
					t.Fatal(err)
				}
			}
			t.Setenv("UA_HOST_TEST_REAL", "1")
			t.Setenv("OPENAI_API_KEY", "fake-loopback-only")
			t.Setenv("UNREAL_HARNESS_LLM_API_KEY", "fake-loopback-only")
			t.Setenv("HOME", root)
			t.Setenv("XDG_CONFIG_HOME", filepath.Join(root, "config"))
			executable, err := os.Executable()
			if err != nil {
				t.Fatal(err)
			}
			child := exec.Command(executable, "-test.run=^TestM22ImageProducer$", "-test.count=1")
			child.Env = append(os.Environ(), "UA_M22_IMAGE_CUT="+cut, "UA_M22_STATE="+directory, "UA_M22_WORKSPACE="+workspace)
			var stderr bytes.Buffer
			child.Stderr = &stderr
			stdout, err := child.StdoutPipe()
			if err != nil {
				t.Fatal(err)
			}
			if err := child.Start(); err != nil {
				t.Fatal(err)
			}
			joined := false
			defer func() {
				if !joined {
					_ = child.Process.Kill()
					_ = child.Wait()
				}
			}()
			marker := make(chan bool, 1)
			var readObserved atomic.Bool
			go func() {
				scanner := bufio.NewScanner(stdout)
				for scanner.Scan() {
					if scanner.Text() == "M22-ORIGINAL-READ:2x3" {
						readObserved.Store(true)
					}
					if scanner.Text() == "M22-COMMITTED:"+cut {
						marker <- true
						return
					}
				}
				marker <- false
			}()
			select {
			case ok := <-marker:
				if !ok {
					_ = child.Process.Kill()
					_ = child.Wait()
					joined = true
					t.Fatalf("producer before boundary: %s", stderr.String())
				}
			case <-ctx.Done():
				t.Fatal("producer boundary unconfirmed")
			}
			if (strings.HasPrefix(cut, "awaiting-") || cut == "committed-result") && !readObserved.Load() {
				t.Fatal("claimed after-read boundary without actual read")
			}
			m21AssertLeases(t, directory, true)
			if err := child.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			err = child.Wait()
			joined = true
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatalf("owned producer Wait: %v", err)
			}
			status, ok := exit.Sys().(syscall.WaitStatus)
			if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
				t.Fatalf("owned SIGKILL unproven: %v", err)
			}
			m21AssertLeases(t, directory, false)
			path := filepath.Join(workspace, "image.png")
			switch cut {
			case "ready-current", "awaiting-current":
				if err := m22PNG(path, 5, 7); err != nil {
					t.Fatal(err)
				}
			case "awaiting-missing", "committed-result":
				if err := os.Remove(path); err != nil {
					t.Fatal(err)
				}
			case "awaiting-corrupt":
				if err := os.WriteFile(path, []byte("not an image"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			canonical := filepath.Join(directory, string(m21Session)+".session.jsonl")
			before, err := os.ReadFile(canonical)
			if err != nil {
				t.Fatal(err)
			}
			var calls atomic.Int64
			var sawImage, sawError atomic.Bool
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				id := calls.Add(1)
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
					w.WriteHeader(500)
					return
				}
				sawImage.Store(bytes.Contains(body, []byte("data:image/")))
				sawError.Store(bytes.Contains(body, []byte("Error:")))
				completed(w, id, assistant(id, "M22 image recovery done"))
			}))
			defer server.Close()
			records := &recorder{changed: make(chan struct{}, 1)}
			config := Config{Command: []string{executable, "-test.run=^$", "worker"}, Bootstrap: workerproto.Bootstrap{SessionID: m21Session, Directory: directory, Workspace: workspace, Provider: "openai", BaseURL: server.URL, Model: llm.Model{ID: "fake", CompactionThreshold: 64000}, MaxAttempts: 1, Shell: "/bin/sh", SystemPrompt: "private image recovery fixture"}, OnItem: records.item, OnStatus: records.status, Stderr: os.Stderr}
			host, err := Start(ctx, config)
			if cut == "future-version" {
				if host != nil {
					_ = host.Close(ctx)
				}
				if err == nil || !strings.Contains(err.Error(), "version=2") || calls.Load() != 0 {
					t.Fatalf("future image was not refused before provider: %v calls=%d", err, calls.Load())
				}
				after, err := os.ReadFile(canonical)
				if err != nil || !bytes.Equal(before, after) {
					t.Fatalf("future refusal mutated canonical history: %v", err)
				}
				m21AssertLeases(t, directory, false)
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				closeCtx, stop := context.WithTimeout(context.Background(), 10*time.Second)
				defer stop()
				if err := host.Close(closeCtx); err != nil {
					t.Error(err)
				}
			}()
			waitResponses(t, records, 2)
			if err := host.Close(ctx); err != nil {
				t.Fatal(err)
			}
			m21AssertLeases(t, directory, false)
			stable := m21ReadItems(t, directory)
			terminal := 0
			wantFailure := cut == "awaiting-missing" || cut == "awaiting-corrupt"
			for _, item := range stable {
				if status, ok := item.Data.(sessionstore.ToolCallStatus); ok {
					for _, op := range status.Operations {
						if op.ID != m22Operation {
							t.Fatalf("operation identity changed: %+v", op)
						}
						if op.Status != operation.StatusCompleted && op.Status != operation.StatusFailed {
							continue
						}
						terminal++
						state, err := operation.DecodeViewImageState(op)
						if err != nil || state.Result == nil {
							t.Fatalf("missing versioned image outcome: %+v %v", state, err)
						}
						if wantFailure {
							if op.Status != operation.StatusFailed || state.Result.Error == "" || state.Result.Content != "" {
								t.Fatalf("invented success for failed current source: %+v", state)
							}
						} else {
							width, height := 5, 7
							if cut == "committed-result" {
								width, height = 2, 3
							}
							if op.Status != operation.StatusCompleted || state.Result.Error != "" || state.Result.Content == "" || state.Result.OriginalWidth != width || state.Result.OriginalHeight != height {
								t.Fatalf("wrong current/committed source result: %+v", state)
							}
						}
					}
				}
			}
			if terminal != 1 || calls.Load() != 1 {
				t.Fatalf("terminal/provider counts=%d/%d", terminal, calls.Load())
			}
			if wantFailure && (!sawError.Load() || sawImage.Load()) || !wantFailure && !sawImage.Load() {
				t.Fatalf("real provider tool result image/error=%v/%v want failure=%v", sawImage.Load(), sawError.Load(), wantFailure)
			}
			// A second actual runtime replays terminal results. Its own shutdown controls
			// may extend history; no operation/model work may alter the existing prefix.
			replay := &recorder{changed: make(chan struct{}, 1)}
			config.OnItem, config.OnStatus = replay.item, replay.status
			host, err = Start(ctx, config)
			if err != nil {
				t.Fatal(err)
			}
			scope := host.ControlScope()
			waitResponses(t, replay, 2)
			if err := host.Close(ctx); err != nil {
				t.Fatal(err)
			}
			got := m21ReadItems(t, directory)
			if len(got) != len(stable)+2 || !reflect.DeepEqual(got[:len(stable)], stable) || calls.Load() != 1 {
				t.Fatalf("terminal replay made work: items %d -> %d calls=%d", len(stable), len(got), calls.Load())
			}
			stopInput, ok := got[len(stable)].Data.(inbox.Input)
			if !ok {
				t.Fatal("missing own stop tail")
			}
			control, err := stopInput.DecodeControlMessage()
			if err != nil || control.Mode != inbox.StopHard {
				t.Fatalf("unexpected stop tail: %+v %v", control, err)
			}
			drain, ok := got[len(stable)+1].Data.(inbox.Input)
			if !ok {
				t.Fatal("missing own drain tail")
			}
			proof, err := drain.DecodeControlMessage()
			if err != nil || proof.Mode != inbox.ExecutionDrained || proof.Parameters.(inbox.ExecutionDrainReceipt).Scope != scope {
				t.Fatalf("unexpected drain tail: %+v %v", proof, err)
			}
			m21AssertLeases(t, directory, false)
			t.Logf("%s: actual old read=%v, owned SIGKILL+Wait, one versioned terminal result, changed/missing current source policy, no replay work", cut, readObserved.Load())
		})
	}
}
