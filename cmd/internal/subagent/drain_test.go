package subagent

import (
	"bufio"
	"context"
	"encoding/json/v2"
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

func TestPendingWorkInitiallyIdleAndQueuedAdmission(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		ctx, cancel := context.WithCancel(t.Context())
		defer cancel()
		changed := make(chan struct{}, 1)
		// Deliberately no actor: admission must be visible before requests can be
		// received, rather than relying on an actor RPC or activity snapshot.
		s := &Supervisor{ctx: ctx, requests: make(chan func(*state)), admissions: make(map[operation.ID]bool), config: Config{OnWorkChanged: func() {
			select {
			case changed <- struct{}{}:
			default:
			}
		}}}
		if s.HasPendingWork() {
			t.Fatal("unused supervisor owns work")
		}
		done := make(chan error, 1)
		go func() { done <- (&handler{supervisor: s}).AddRemoteJob(operation.Operation{ID: "queued"}) }()
		synctest.Wait()
		if !s.HasPendingWork() {
			t.Fatal("unprocessed Add admission was invisible")
		}
		select {
		case <-changed:
		default:
			t.Fatal("admission did not wake owner")
		}
		cancel()
		if err := <-done; !errors.Is(err, context.Canceled) {
			t.Fatalf("admission result: %v", err)
		}
		if s.HasPendingWork() {
			t.Fatal("canceled admission retained work")
		}
		select {
		case <-changed:
		default:
			t.Fatal("admission drain did not wake owner")
		}
		if s.workRequests != 0 {
			t.Fatalf("request count = %d", s.workRequests)
		}
	})
}

func TestPendingWorkActorPublicationCoversQueuesAndFinals(t *testing.T) {
	tests := map[string]func(*state){
		"process":        func(s *state) { s.activeProcesses = 1 },
		"preparation":    func(s *state) { s.pendingPreps = 1 },
		"terminal retry": func(s *state) { s.pendingFinals = []terminalRetry{{}} },
		"run update":     func(s *state) { s.runQueue = []operation.Operation{{}} },
		"message update": func(s *state) { s.messageQueue = []operation.Operation{{}} },
		"launch owner":   func(s *state) { s.owners = map[operation.ID]*child{"owner": {}} },
		"message":        func(s *state) { s.messages = map[operation.ID]*pendingMessage{"message": {}} },
		"parent input":   func(s *state) { s.deliveryInputs = []inbox.Input{{}} },
	}
	for name, set := range tests {
		t.Run(name, func(t *testing.T) {
			supervisor := &Supervisor{}
			s := &state{supervisor: supervisor}
			set(s)
			s.publishWork(0)
			if !supervisor.HasPendingWork() {
				t.Fatal("actor ownership not published")
			}
			(&state{supervisor: supervisor}).publishWork(0)
			if supervisor.HasPendingWork() {
				t.Fatal("drained actor retained work")
			}
		})
	}
}

func TestPendingWorkConcurrentReservationsAndRequests(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	s, err := New(ctx, Config{Command: []string{"unused"}, Inbox: lifecycleInbox{}})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	if s.HasPendingWork() {
		t.Fatal("idle actor/workers counted as work")
	}
	var workers sync.WaitGroup
	for range 32 {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for range 25 {
				if err := s.reserveDelivery(ctx, 1); err != nil {
					t.Error(err)
					return
				}
				if !s.HasPendingWork() {
					t.Error("reservation invisible")
				}
				s.releaseDelivery(1)
				if err := s.do(func(*state) {}); err != nil {
					t.Error(err)
					return
				}
				_ = s.HasPendingWork()
			}
		}()
	}
	workers.Wait()
	cancel()
	if err := s.Wait(t.Context()); err != nil {
		t.Fatal(err)
	}
	if s.HasPendingWork() {
		t.Fatal("joined idle actor retained work")
	}
	s.workMu.Lock()
	defer s.workMu.Unlock()
	if s.workRequests != 0 || s.workDeliveries != 0 {
		t.Fatalf("unbalanced ownership counts: requests=%d deliveries=%d", s.workRequests, s.workDeliveries)
	}
}

const drainHelperEnv = "UNREAL_SUBAGENT_DRAIN_HELPER"

// A committed background report is followed by a fresh cleanup receipt and EOF,
// but the actual process stays alive until the parent releases its file gate.
func TestPendingWorkChild(t *testing.T) {
	dir := os.Getenv(drainHelperEnv)
	if dir == "" {
		return
	}
	if _, err := bufio.NewReader(os.Stdin).ReadBytes('\n'); err != nil {
		t.Fatal(err)
	}
	var scope string
	for i, arg := range os.Args {
		if arg == CleanupProofScopeFlag && i+1 < len(os.Args) {
			scope = os.Args[i+1]
		}
	}
	if scope == "" {
		t.Fatal("missing cleanup scope")
	}
	response := sessionstore.ModelResponse{TurnID: "drain-turn", Response: llm.Response{Stop: llm.StopComplete, Output: []llm.Item{
		{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: "Background report, not owner drain."}},
		{Type: llm.ItemToolCall, Data: llm.ToolCall{Name: tool.SendMessageName, CallID: "parent", Arguments: `{"to":"parent","message":"delivery must join"}`}},
	}}}
	emit := func(item sessionstore.Item) {
		data, err := json.Marshal(item)
		if err != nil {
			t.Fatal(err)
		}
		if _, err = os.Stdout.Write(append(data, '\n')); err != nil {
			t.Fatal(err)
		}
	}
	emit(sessionstore.Item{Sequence: 1, Kind: sessionstore.ItemModelResponse, Data: response})
	payload, _ := json.Marshal(inbox.ControlMessage{Mode: inbox.ExecutionDrained, Parameters: inbox.ExecutionDrainReceipt{Version: 1, Scope: scope}})
	emit(sessionstore.Item{Sequence: 2, Kind: sessionstore.ItemInput, Data: inbox.Input{ID: "receipt", Kind: inbox.InputControl, Payload: payload}})
	_ = os.Stdout.Close()
	if err := os.WriteFile(filepath.Join(dir, "output-closed"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	lifecycleFile(t, filepath.Join(dir, "release-process"))
}

func TestPendingWorkBackgroundReportWaitsForProcessOutputAndParentDelivery(t *testing.T) {
	dir := t.TempDir()
	ctx, cancel := context.WithCancel(t.Context())
	gate := activityGatedInbox{entered: make(chan struct{}), release: make(chan struct{})}
	changed := make(chan struct{}, 1)
	s, err := New(ctx, Config{RequireCleanupProof: true, Command: []string{os.Args[0], "-test.run=^TestPendingWorkChild$", "--"}, Env: append(os.Environ(), drainHelperEnv+"="+dir), Inbox: gate, Stderr: os.Stderr, OnWorkChanged: func() {
		select {
		case changed <- struct{}{}:
		default:
		}
	}})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	var release sync.Once
	t.Cleanup(func() {
		_ = os.WriteFile(filepath.Join(dir, "release-process"), nil, 0600)
		release.Do(func() { close(gate.release) })
		cancel()
		wait, stop := context.WithTimeout(context.Background(), 15*time.Second)
		defer stop()
		if err := s.Wait(wait); err != nil {
			t.Errorf("cleanup: %v", err)
		}
	})
	if s.HasPendingWork() {
		t.Fatal("new supervisor owns work")
	}
	op := supervisionOperation(t, "background-drain", toolsubagent.RunPlanType, toolsubagent.RunPlan{Name: "background", Prompt: "report", Lifetime: "background"})
	if err := s.run.AddRemoteJob(op); err != nil {
		t.Fatal(err)
	}
	deadline := time.NewTimer(10 * time.Second)
	defer deadline.Stop()
	for {
		select {
		case update := <-s.run.updates:
			if update.Status != operation.StatusCompleted {
				continue
			}
			if !s.HasPendingWork() {
				t.Fatal("committed background report released live ownership")
			}
			goto acknowledged
		case <-deadline.C:
			t.Fatal("background report never acknowledged")
		}
	}
acknowledged:
	lifecycleFile(t, filepath.Join(dir, "output-closed"))
	activityAwait(t, s, func(a ActivitySnapshot) bool { return len(a.Children) == 1 && a.Children[0].OutputEOFObserved })
	if !s.HasPendingWork() {
		t.Fatal("stdout EOF released a live process")
	}
	select {
	case <-gate.entered:
	case <-deadline.C:
		t.Fatal("parent Submit never entered")
	}
	if err := os.WriteFile(filepath.Join(dir, "release-process"), nil, 0600); err != nil {
		t.Fatal(err)
	}
	activityAwait(t, s, func(a ActivitySnapshot) bool {
		return a.Children[0].ReaderReturned && a.Children[0].ProcessWaitReturned
	})
	if !s.HasPendingWork() {
		t.Fatal("process/output join released unacknowledged parent delivery")
	}
	// Discard earlier transition notifications. The release must produce a fresh
	// wakeup once delivery completion is processed and ownership really drains.
	select {
	case <-changed:
	default:
	}
	release.Do(func() { close(gate.release) })
	for s.HasPendingWork() {
		select {
		case <-changed:
		case <-deadline.C:
			t.Fatal("joined ownership failed to drain/wake")
		}
	}
	s.workMu.Lock()
	defer s.workMu.Unlock()
	if s.workRequests != 0 || s.workDeliveries != 0 {
		t.Fatalf("unbalanced final counts: requests=%d deliveries=%d", s.workRequests, s.workDeliveries)
	}
}
