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

const m21Session session.ID = "m21-recovery-private"
const m21Operation operation.ID = "m21-original-intent"

func m21Inputs() []inbox.Input {
	return []inbox.Input{
		{ID: "m21-first", Kind: inbox.InputExternal, Payload: []byte(`"m21 FIFO first"`)},
		{ID: "m21-second", Kind: inbox.InputExternal, Payload: []byte(`"m21 FIFO second"`)},
	}
}

// This child is a controlled persistence fixture, NOT an interception of a
// runtime-internal crash window. Each marker follows successful durable APIs.
// Ordinary -test.run selection deliberately leaves the existing TestMain alone.
func TestM21RecoveryProducer(t *testing.T) {
	boundary := os.Getenv("UA_M21_RECOVERY_BOUNDARY")
	if boundary == "" {
		t.Skip("owned persistence producer only")
	}
	directory := os.Getenv("UA_M21_RECOVERY_DIRECTORY")
	hostLease, err := localfile.AcquireLease(directory, session.ID("host-"+string(m21Session)))
	if err != nil {
		t.Fatal(err)
	}
	defer hostLease.Close()
	lease, err := localfile.AcquireLease(directory, m21Session)
	if err != nil {
		t.Fatal(err)
	}
	defer lease.Close()
	journal, err := ingress.Open(filepath.Join(directory, "ingress"), m21Session)
	if err != nil {
		t.Fatal(err)
	}
	defer journal.Close()
	store, err := localfile.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Create(t.Context(), m21Session); err != nil {
		t.Fatal(err)
	}
	if boundary == "intent" {
		path := filepath.Join(os.Getenv("UA_M21_RECOVERY_WORKSPACE"), "original.png")
		var encoded bytes.Buffer
		if err := png.Encode(&encoded, image.NewRGBA(image.Rect(0, 0, 2, 2))); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, encoded.Bytes(), 0600); err != nil {
			t.Fatal(err)
		}
		spec, err := operation.NewViewImageSpec(path, operation.ViewImageConfig{MaxSize: 1 << 20, MaxWidth: 100, MaxHeight: 100})
		if err != nil {
			t.Fatal(err)
		}
		op := operation.Operation{ID: m21Operation, Type: spec.Type, Version: spec.Version, State: spec.State.Clone(), Status: operation.StatusReady}
		if err := store.AppendTurn(t.Context(), m21Session, session.Turn{ID: "m21-turn", Type: session.TurnRegular}); err != nil {
			t.Fatal(err)
		}
		args, err := json.Marshal(map[string]string{"path": path})
		if err != nil {
			t.Fatal(err)
		}
		if err := store.AppendModelResponse(t.Context(), m21Session, sessionstore.ModelResponse{TurnID: "m21-turn", Response: llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "m21-call", Name: "ViewImage", Arguments: string(args)}}}}}); err != nil {
			t.Fatal(err)
		}
		if err := store.AppendToolCallStatus(t.Context(), m21Session, sessionstore.ToolCallStatus{TurnID: "m21-turn", CallID: "m21-call", Status: tool.CallStatus{WaitingFor: []operation.ID{op.ID}}, Operations: []operation.Operation{op}}); err != nil {
			t.Fatal(err)
		}
	} else {
		for _, input := range m21Inputs() {
			if err := journal.Submit(t.Context(), input); err != nil {
				t.Fatal(err)
			}
		}
		if boundary == "persisted" {
			if err := store.AppendInput(t.Context(), m21Session, m21Inputs()[0]); err != nil {
				t.Fatal(err)
			}
		} else if boundary != "journal" {
			t.Fatalf("unknown boundary %q", boundary)
		}
	}
	fmt.Printf("M21-COMMITTED:%s\n", boundary)
	// No deferred release can run after SIGKILL. The parent must join this exact Cmd.
	for {
		time.Sleep(time.Hour)
	}
}

func m21ReadItems(t *testing.T, directory string) []sessionstore.Item {
	t.Helper()
	store, err := localfile.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	var items []sessionstore.Item
	after := sessionstore.BeforeFirst
	for {
		page, err := store.Items(t.Context(), m21Session, after, 100)
		if err != nil {
			t.Fatal(err)
		}
		items = append(items, page.Items...)
		if !page.More {
			return items
		}
		after = page.NextAfter
	}
}

func m21AssertLeases(t *testing.T, directory string, held bool) {
	t.Helper()
	for _, id := range []session.ID{m21Session, session.ID("host-" + string(m21Session))} {
		lease, err := localfile.AcquireLease(directory, id)
		if held {
			if lease != nil {
				_ = lease.Close()
			}
			if !errors.Is(err, localfile.ErrSessionLocked) {
				t.Fatalf("lease %s while owned: %v", id, err)
			}
		} else {
			if err != nil {
				t.Fatalf("lease %s after owned Wait/Close: %v", id, err)
			}
			if err := lease.Close(); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestM21OwnedProcessLossAndRestart(t *testing.T) {
	for _, boundary := range []string{"journal", "persisted", "intent"} {
		t.Run(boundary, func(t *testing.T) {
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			root := t.TempDir()
			directory, workspace := filepath.Join(root, "state"), filepath.Join(root, "workspace")
			for _, path := range []string{root, directory, workspace, filepath.Join(directory, "ingress")} {
				if err := os.MkdirAll(path, 0700); err != nil {
					t.Fatal(err)
				}
				if err := os.Chmod(path, 0700); err != nil {
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
			var calls atomic.Int64
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				id := calls.Add(1)
				// Consume request bodies, exercising the real provider wire path.
				if _, err := io.Copy(io.Discard, r.Body); err != nil {
					return
				}
				completed(w, id, assistant(id, "M21 recovery done"))
			}))
			defer server.Close()
			config := Config{Command: []string{executable, "-test.run=^$", "worker"}, Bootstrap: workerproto.Bootstrap{SessionID: m21Session, Directory: directory, Workspace: workspace, Provider: "openai", BaseURL: server.URL, Model: llm.Model{ID: "fake", CompactionThreshold: 64000}, MaxAttempts: 1, Shell: "/bin/sh", SystemPrompt: "Deterministic private recovery fixture"}, Stderr: os.Stderr}
			child := exec.Command(executable, "-test.run=^TestM21RecoveryProducer$", "-test.count=1")
			child.Env = append(os.Environ(), "UA_M21_RECOVERY_BOUNDARY="+boundary, "UA_M21_RECOVERY_DIRECTORY="+directory, "UA_M21_RECOVERY_WORKSPACE="+workspace)
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
			go func() {
				scanner := bufio.NewScanner(stdout)
				for scanner.Scan() {
					if scanner.Text() == "M21-COMMITTED:"+boundary {
						marker <- true
						return
					}
				}
				marker <- false
			}()
			select {
			case ok := <-marker:
				if !ok {
					t.Fatalf("producer exited before boundary: %s", stderr.String())
				}
			case <-ctx.Done():
				t.Fatal("producer boundary unconfirmed; cleanup kills and joins owned Cmd")
			}
			m21AssertLeases(t, directory, true)
			competitor, err := Start(ctx, config)
			if competitor != nil {
				_ = competitor.Close(ctx)
				t.Fatal("competing host acquired producer authority")
			}
			if !errors.Is(err, localfile.ErrSessionLocked) {
				t.Fatalf("competing Start = %v", err)
			}
			pending, err := ingress.Open(filepath.Join(directory, "ingress"), m21Session)
			if pending != nil {
				_ = pending.Close()
			}
			if !errors.Is(err, ingress.ErrLocked) {
				t.Fatalf("producer ingress ownership = %v", err)
			}
			before := m21ReadItems(t, directory)
			if boundary == "journal" && len(before) != 0 {
				t.Fatalf("journal-only session history = %+v", before)
			}
			if boundary == "persisted" && (len(before) != 1 || !reflect.DeepEqual(before[0].Data, m21Inputs()[0])) {
				t.Fatalf("persisted boundary = %+v", before)
			}
			if boundary == "intent" {
				if len(before) != 3 {
					t.Fatalf("intent boundary history = %+v", before)
				}
				status, ok := before[2].Data.(sessionstore.ToolCallStatus)
				if !ok || len(status.Operations) != 1 || status.Operations[0].ID != m21Operation || status.Operations[0].Status != operation.StatusReady {
					t.Fatalf("intent boundary = %+v", before[2])
				}
			}
			if err := child.Process.Kill(); err != nil {
				t.Fatal(err)
			}
			err = child.Wait()
			joined = true
			var exit *exec.ExitError
			if !errors.As(err, &exit) {
				t.Fatalf("owned producer Wait = %v", err)
			}
			status, ok := exit.Sys().(syscall.WaitStatus)
			if !ok || !status.Signaled() || status.Signal() != syscall.SIGKILL {
				t.Fatalf("producer did not die by owned SIGKILL: %v", err)
			}
			m21AssertLeases(t, directory, false)
			reopened, err := ingress.Open(filepath.Join(directory, "ingress"), m21Session)
			if err != nil {
				t.Fatal(err)
			}
			wantPending := m21Inputs()
			if boundary == "intent" {
				wantPending = []inbox.Input{}
			}
			if !reflect.DeepEqual(reopened.Pending(), wantPending) {
				t.Fatalf("committed FIFO = %+v", reopened.Pending())
			}
			if err := reopened.Close(); err != nil {
				t.Fatal(err)
			}
			records := &recorder{changed: make(chan struct{}, 1)}
			config.OnItem, config.OnStatus = records.item, records.status
			host, err := Start(ctx, config)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				closeCtx, c := context.WithTimeout(context.Background(), 10*time.Second)
				defer c()
				if err := host.Close(closeCtx); err != nil {
					t.Error(err)
				}
			}()
			if competitor, err := Start(ctx, config); competitor != nil || !errors.Is(err, localfile.ErrSessionLocked) {
				if competitor != nil {
					_ = competitor.Close(ctx)
				}
				t.Fatalf("second live host = %v", err)
			}
			wantResponses := 1
			if boundary == "intent" {
				wantResponses = 2
			} // Original tool call plus recovered answer.
			if boundary != "intent" {
				for _, input := range m21Inputs() {
					waitInput(t, records, string(input.ID))
				}
			}
			waitResponses(t, records, wantResponses)
			if err := host.Reload(ctx, nil); err != nil {
				t.Fatalf("recovery ordered drain barrier: %v", err)
			}
			recovered := m21ReadItems(t, directory)
			var inputs []inbox.Input
			completedCount := 0
			for _, item := range recovered {
				if input, ok := item.Data.(inbox.Input); ok && input.Kind == inbox.InputExternal {
					inputs = append(inputs, input)
				}
				if status, ok := item.Data.(sessionstore.ToolCallStatus); ok {
					if status.Status.Error != "" {
						t.Fatalf("canonical tool completion failed: %+v", status)
					}
					for _, op := range status.Operations {
						if boundary != "intent" || op.ID != m21Operation {
							t.Fatalf("unexpected operation %+v", op)
						}
						if op.Status == operation.StatusCompleted {
							completedCount++
							state, err := operation.DecodeViewImageState(op)
							if err != nil || state.Result == nil || state.Result.Error != "" || state.Result.Content == "" {
								t.Fatalf("real image outcome = %+v, %v", state, err)
							}
						}
					}
				}
			}
			if boundary == "intent" {
				if completedCount != 1 || len(inputs) != 0 {
					t.Fatalf("completion/input counts %d/%d", completedCount, len(inputs))
				}
			} else if !reflect.DeepEqual(inputs, m21Inputs()) {
				t.Fatalf("recovered inputs FIFO once = %+v", inputs)
			}
			if len(host.journal.Pending()) != 0 {
				t.Fatal("recovered inputs not journal acknowledged")
			}
			firstCalls := calls.Load()
			// Inbox batching may combine the two inputs into one model turn, or
			// the first request may precede second-input admission. Both are
			// legitimate; the persisted inputs themselves must be FIFO once.
			responseCount := 0
			for _, item := range recovered {
				if item.Kind == sessionstore.ItemModelResponse {
					responseCount++
				}
			}
			seededResponses := 0
			if boundary == "intent" {
				seededResponses = 1
			}
			if firstCalls < 1 || firstCalls > 2 || int(firstCalls) != responseCount-seededResponses || boundary == "intent" && firstCalls != 1 {
				t.Fatalf("provider requests/responses = %d/%d (seeded %d)", firstCalls, responseCount, seededResponses)
			}
			wantResponses = responseCount
			if err := host.Close(ctx); err != nil {
				t.Fatal(err)
			}
			m21AssertLeases(t, directory, false)
			// Successful Close appends its own stop and fresh drain controls.
			// Those are legitimate lifecycle records, not recovered external work.
			stable := m21ReadItems(t, directory)
			replay := &recorder{changed: make(chan struct{}, 1)}
			config.OnItem, config.OnStatus = replay.item, replay.status
			host, err = Start(ctx, config)
			if err != nil {
				t.Fatal(err)
			}
			waitResponses(t, replay, wantResponses)
			replayScope := host.ControlScope()
			if err := host.Reload(ctx, nil); err != nil {
				t.Fatalf("terminal replay barrier: %v", err)
			}
			got := m21ReadItems(t, directory)
			if len(got) != len(stable)+2 || !reflect.DeepEqual(got[:len(stable)], stable) {
				t.Fatalf("terminal restart changed committed prefix or added work: %d -> %d items", len(stable), len(got))
			}
			// Natural retirement now records its ordinary process-lifetime stop,
			// followed by fresh owner cleanup evidence. Neither record is a new
			// model turn, tool intent, effect, or recovered external submission.
			var tailIDs []inbox.ID
			for index, mode := range []inbox.ControlMode{inbox.StopWhenIdle, inbox.ExecutionDrained} {
				item := got[len(stable)+index]
				input, ok := item.Data.(inbox.Input)
				if item.Kind != sessionstore.ItemInput || !ok || input.Kind != inbox.InputControl || input.ID == "" {
					t.Fatalf("retirement tail %d is not a canonical control: %+v", index, item)
				}
				if item.Sequence != stable[len(stable)-1].Sequence+sessionstore.Sequence(index+1) {
					t.Fatal("retirement tail lost canonical sequence continuity")
				}
				tailIDs = append(tailIDs, input.ID)
				control, err := input.DecodeControlMessage()
				if err != nil || control.Mode != mode {
					t.Fatalf("unexpected replay tail %d: %+v %v", index, control, err)
				}
				if mode == inbox.StopWhenIdle {
					if control.Reason != "worker replacement" || control.Parameters != nil {
						t.Fatalf("unexpected natural-stop intent: %+v", control)
					}
				} else if control.Parameters.(inbox.ExecutionDrainReceipt) != (inbox.ExecutionDrainReceipt{Version: 1, Scope: replayScope}) {
					t.Fatal("replay cleanup tail is not fresh scope-bound evidence")
				}
			}
			if tailIDs[0] == tailIDs[1] {
				t.Fatal("stop and cleanup evidence reused one input identity")
			}

			if calls.Load() != firstCalls {
				t.Fatal("terminal restart made extra provider request")
			}
			if err := host.Close(ctx); err != nil {
				t.Fatal(err)
			}
			m21AssertLeases(t, directory, false)
			t.Logf("owned producer killed and joined: %s: %d provider requests; committed prefix unchanged and only ordered stop/fresh cleanup tail on second restart; terminal completion is not direct manager-admission instrumentation", boundary, firstCalls)
		})
	}
}
