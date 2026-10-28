package subagent

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

func backgroundReportFixture(t *testing.T) (*state, *child) {
	t.Helper()
	owner := supervisionOperation(t, "retained-ticket", toolsubagent.RunPlanType, toolsubagent.RunPlan{Name: "retained", Lifetime: "background"})
	owner.Status = operation.StatusAwaiting
	supervisor := &Supervisor{ctx: context.Background(), config: Config{RequireCleanupProof: true}, activity: &activityProjection{}, runPrep: make(chan prepJob, 8), done: make(chan struct{})}
	c := &child{name: "retained", sessionID: "child", generation: 1, launched: true, live: true, cleanupScope: "fresh-private-scope", owner: &owner, metadata: runMetadata{Lifetime: "background"}}
	s := &state{supervisor: supervisor, children: map[string]*child{c.name: c}, owners: map[operation.ID]*child{owner.ID: c}, activeProcesses: 1}
	supervisor.activity.begin(c.name, c.sessionID, c.generation, owner.ID, time.Time{})
	return s, c
}

func backgroundReportResponse() llm.Response {
	return llm.Response{Stop: llm.StopComplete, Output: []llm.Item{
		{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: "Owned work continues."}},
		{Type: llm.ItemToolCall, Data: llm.ToolCall{Name: "Bash", CallID: "active", Arguments: `{"command":"owned work"}`}},
	}}
}

func backgroundReportLine(t *testing.T, s *state, c *child, seq sessionstore.Sequence, response llm.Response) {
	t.Helper()
	data, err := json.Marshal(sessionstore.Item{Sequence: seq, Kind: sessionstore.ItemModelResponse, Data: sessionstore.ModelResponse{TurnID: "child-turn", Response: response}})
	if err != nil {
		t.Fatal(err)
	}
	s.line(c, c.generation, data)
}

func TestBackgroundReportAcknowledgmentGates(t *testing.T) {
	for _, name := range []string{"legacy", "task", "default", "no-scope", "not-launched", "partial", "failure", "nonassistant", "blank", "tool-only", "unsupported", "canceling", "wrong-plan-lifetime"} {
		t.Run(name, func(t *testing.T) {
			s, c := backgroundReportFixture(t)
			response := backgroundReportResponse()
			switch name {
			case "legacy":
				s.supervisor.config.RequireCleanupProof = false
			case "task":
				c.metadata.Lifetime = "task"
			case "default":
				c.metadata.Lifetime = ""
			case "no-scope":
				c.cleanupScope = ""
			case "not-launched":
				c.launched = false
			case "partial":
				response.Stop = llm.StopInterrupted
			case "failure":
				response.Failure = &llm.Failure{Code: "failed"}
			case "nonassistant":
				response.Output[0].Data = llm.Message{Role: llm.RoleUser, Text: "not assistant"}
			case "blank":
				response.Output[0].Data = llm.Message{Role: llm.RoleAssistant, Text: "  "}
			case "tool-only":
				response.Output = response.Output[1:]
			case "unsupported":
				c.owner.Status = operation.StatusUnsupported
			case "canceling":
				c.owner.Status = operation.StatusCanceling
			case "wrong-plan-lifetime":
				remote, _ := operation.DecodeRemoteJobState(*c.owner)
				remote.Plan.Data = []byte(`{"Lifetime":"task"}`)
				c.owner.State, _ = json.Marshal(remote)
			}
			backgroundReportLine(t, s, c, 1, response)
			if c.backgroundAcknowledged || len(s.supervisor.runPrep) != 0 || len(s.pendingFinals) != 0 {
				t.Fatal("ineligible response acknowledged background ownership")
			}
		})
	}
}

func TestBackgroundReportAcknowledgesOnceRetainsStructuralStopAndRequiresReceipt(t *testing.T) {
	s, c := backgroundReportFixture(t)
	backgroundReportLine(t, s, c, 1, backgroundReportResponse())
	backgroundReportLine(t, s, c, 2, backgroundReportResponse())
	if !c.backgroundAcknowledged || c.owner == nil || s.owners[c.owner.ID] != c || !c.live || s.activeProcesses != 1 || len(s.supervisor.runPrep) != 1 {
		t.Fatal("ack lost structural child ownership or was duplicated")
	}
	job := <-s.supervisor.runPrep
	step, err := operation.UpdateRemoteJob(job.value, job.remote, job.status)
	if err != nil {
		t.Fatal(err)
	}
	s.prepared(prepResult{job: job, step: step})
	if len(s.runQueue) != 1 || s.runQueue[0].Status != operation.StatusCompleted {
		t.Fatal("missing Run acknowledgment")
	}
	remote, err := operation.DecodeRemoteJobState(s.runQueue[0])
	if err != nil || !strings.Contains(remote.TerminalResult, "child remains owned/running") {
		t.Fatalf("ack falsely describes completion: %+v %v", remote, err)
	}
	a := s.supervisor.ActivitySnapshot().Children[0]
	if a.OwnerTerminalEnqueued || a.ProcessWaitReturned || a.ReaderReturned || a.ReportAvailable {
		t.Fatalf("ticket acknowledgment claimed drain/final report: %+v", a)
	}
	// Hub's structural stop targets exactly this retained owner ID. Its public
	// capability remains valid and cancellation must not rewrite the completed ticket.
	var control bytes.Buffer
	c.stdin = backgroundControlWriter{&control}
	c.stop = func() {}
	s.cancel(c.owner.ID, "structural AgentControl stop")
	if !c.stopping || !strings.Contains(control.String(), `"Mode":"hard"`) || len(s.supervisor.runPrep) != 0 {
		t.Fatal("retained structural stop lost, or acknowledged ticket rewritten")
	}
	s.exited(c, c.generation, nil)
	close(s.supervisor.done)
	if err := s.supervisor.Wait(context.Background()); err == nil {
		t.Fatal("acknowledged child without fresh receipt passed Wait")
	}
	if len(s.runQueue) != 1 {
		t.Fatal("child exit republished acknowledged ticket")
	}
}

type backgroundControlWriter struct{ io.Writer }

func (backgroundControlWriter) Close() error { return nil }

func TestBackgroundReportBoundedRetryAndFreshGeneration(t *testing.T) {
	s, c := backgroundReportFixture(t)
	// Exhaust just the owned preparation queue. A single retained retry, rather
	// than duplicate reports or lost ownership, must carry the acknowledgment.
	for i := 0; i < cap(s.supervisor.runPrep); i++ {
		s.supervisor.runPrep <- prepJob{}
	}
	backgroundReportLine(t, s, c, 4, backgroundReportResponse())
	backgroundReportLine(t, s, c, 5, backgroundReportResponse())
	if len(s.pendingFinals) != 1 || !c.backgroundAcknowledged {
		t.Fatal("acknowledgment backpressure was dropped or duplicated")
	}
	for len(s.supervisor.runPrep) > 0 {
		<-s.supervisor.runPrep
	}
	s.retryFinals()
	if len(s.pendingFinals) != 0 || len(s.supervisor.runPrep) != 1 {
		t.Fatal("bounded acknowledgment retry did not resume")
	}
	// A report from an old generation and duplicate canonical sequence cannot
	// acknowledge a replacement generation.
	c.backgroundAcknowledged = false
	raw, err := json.Marshal(sessionstore.Item{Sequence: 6, Kind: sessionstore.ItemModelResponse, Data: sessionstore.ModelResponse{Response: backgroundReportResponse()}})
	if err != nil {
		t.Fatal(err)
	}
	s.line(c, c.generation-1, raw)
	backgroundReportLine(t, s, c, 5, backgroundReportResponse())
	if c.backgroundAcknowledged {
		t.Fatal("stale generation or replay acknowledged current background branch")
	}
}

func TestBackgroundReportHubStopRetainsExactCapability(t *testing.T) {
	// Testing cancels t.Context BEFORE registered Cleanup callbacks. This
	// synthetic child has no process goroutine to post its exit automatically;
	// keep the actor owner live until the explicit unit exit barrier below.
	ctx, cancel := context.WithCancel(context.Background())
	hub := NewHub(ctx)
	supervisor, err := New(ctx, Config{RequireCleanupProof: true, Hub: hub, Command: []string{"unused-unit-fixture"}, Inbox: lifecycleInbox{}})
	if err != nil {
		cancel()
		t.Fatal(err)
	}
	var control bytes.Buffer
	var retained *child
	// This is an actor/unit test: no subprocess or receipt is synthesized. It
	// exercises the actual Hub handle/generation dispatch for an acknowledged Run.
	if err := supervisor.do(func(s *state) {
		owner := supervisionOperation(t, "hub-background", toolsubagent.RunPlanType, toolsubagent.RunPlan{Name: "retained", Lifetime: "background"})
		owner.Status = operation.StatusAwaiting
		handle, bindErr := hub.bind(supervisor, "hub-child", 1)
		if bindErr != nil {
			t.Error(bindErr)
			return
		}
		retained = &child{name: "retained", sessionID: "hub-child", generation: 1, handle: handle, owner: &owner, backgroundAcknowledged: true, launched: true, live: true, cleanupScope: "fresh", stdin: backgroundControlWriter{&control}, stop: func() {}}
		s.children[retained.name] = retained
		s.owners[owner.ID] = retained
		s.activeProcesses = 1
		supervisor.activity.begin(retained.name, retained.sessionID, retained.generation, owner.ID, time.Time{})
	}); err != nil {
		cancel()
		t.Fatal(err)
	}
	// Actor barrier before reading the capability installed above.
	ready := make(chan struct{})
	_ = supervisor.do(func(*state) { close(ready) })
	<-ready
	t.Cleanup(func() {
		exited := make(chan struct{})
		if err := supervisor.do(func(s *state) {
			if retained != nil && retained.live {
				s.exited(retained, retained.generation, nil)
			}
			close(exited)
		}); err != nil {
			t.Error(err)
		} else {
			<-exited
		}
		cancel()
		wait, stop := context.WithTimeout(context.Background(), 3*time.Second)
		defer stop()
		if err := supervisor.Wait(wait); err == nil || !strings.Contains(err.Error(), "without fresh cleanup proof") {
			t.Errorf("expected joined missing-proof result, not timeout: %v", err)
		}
		if err := hub.Wait(wait); err != nil {
			t.Error(err)
		}
	})
	if retained == nil {
		t.Fatal("unit capability not installed")
	}
	capability := ChildActivity{Handle: retained.handle, Generation: retained.generation}
	op := controlSubmit(t, hub, "stop-retained", "stop", capability, "")
	update := controlAwait(t, hub, op.ID, true)
	if update.Status != operation.StatusCompleted {
		t.Fatalf("structural stop rejected: %+v", update)
	}
	checked := make(chan struct{})
	if err := supervisor.do(func(s *state) {
		if !retained.stopping || !strings.Contains(control.String(), `"Mode":"hard"`) || retained.owner == nil || s.owners[retained.owner.ID] != retained {
			t.Error("Hub stop lost acknowledged child's exact structural ownership")
		}
		close(checked)
	}); err != nil {
		t.Fatal(err)
	}
	<-checked
}
