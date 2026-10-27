package agentrunner

import (
	"context"
	"encoding/json/v2"
	"os"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/cmd/internal/subagent"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

const controlChildEnvironment = "HARNESS_AGENTCONTROL_CHILD_TEST"

// Canceled model requests may overlap replacements. Fixtures derive decisions
// from persisted history, not mutable stages that a discarded response advances.
func controlHasCall(input []llm.Item, id string) bool {
	for _, item := range input {
		switch data := item.Data.(type) {
		case llm.ToolCall:
			if data.CallID == id {
				return true
			}
		case llm.ToolResult:
			if data.CallID == id {
				return true
			}
		}
	}
	return false
}

// This child never emits a candidate. The parent must discover and steer the
// graph attempt while live, rather than waiting for a conveniently final report.
func TestAgentControlGraphChildProcess(t *testing.T) {
	if os.Getenv(controlChildEnvironment) != "1" {
		return
	}
	separator := slices.Index(os.Args, "--")
	if separator < 0 {
		t.Fatal("missing child arguments")
	}
	os.Args = append(os.Args[:1], os.Args[separator+1:]...)
	client := &fakeClient{respond: func(ctx context.Context, request llm.Request) (llm.Response, error) {
		if !controlHasCall(request.Input, "ready") {
			return toolCallResponse("ready", "SendMessage", `{"to":"parent","message":"graph child active before completion"}`), nil
		}
		<-ctx.Done()
		return llm.Response{}, ctx.Err()
	}}
	Main(testConfig(client))
}

func TestRootControlsGraphChildBeforeCompletion(t *testing.T) {
	executable, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	t.Setenv(controlChildEnvironment, "1")
	t.Setenv(providers.APIKeyEnvironment, "secret")
	t.Setenv(llmProviderEnvironment, "")
	t.Setenv(llmBaseURLEnvironment, "")
	sessions := t.TempDir()
	store, err := localfile.New(sessions)
	if err != nil {
		t.Fatal(err)
	}
	var finished atomic.Bool
	feedback := "review feedback persisted before candidate"
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	client := &fakeClient{respond: func(ctx context.Context, request llm.Request) (llm.Response, error) {
		defer func() {
			if t.Failed() {
				cancel()
			}
		}()
		results := toolResults(request.Input)
		issue := func(id, name, args string) (llm.Response, error) {
			if controlHasCall(request.Input, id) {
				return messageResponse("waiting for " + id), nil
			}
			return toolCallResponse(id, name, args), nil
		}
		if !controlHasCall(request.Input, "graph") {
			return toolCallResponse("graph", "TaskGraph", `{"action":"start","name":"live-control","tasks":[{"id":"review","description":"review while running","prompt":"wait for feedback","reads":[],"writes":[],"acceptance":"parent inspected result"}]}`), nil
		}
		ready := false
		for _, item := range request.Input {
			if message, ok := item.Data.(llm.Message); ok && message.Role == llm.RoleUser && strings.Contains(message.Text, "graph child active before completion") {
				ready = true
			}
		}
		if !ready {
			return messageResponse("waiting for live child"), nil
		}
		if results["list"] == "" {
			return issue("list", "AgentControl", `{"action":"list"}`)
		}
		var list toolsubagent.ControlOutcome
		if err := json.Unmarshal([]byte(results["list"]), &list); err != nil {
			t.Fatalf("list=%s: %v", results["list"], err)
		}
		var fleet subagent.FleetSnapshot
		if err := json.Unmarshal(list.Snapshot, &fleet); err != nil {
			t.Fatalf("fleet=%s: %v", results["list"], err)
		}
		if len(fleet.Agents) != 1 {
			t.Fatalf("fleet=%+v", fleet)
		}
		child := fleet.Agents[0]
		if child.Handle == "" || child.Generation < 1 || child.TaskID != "review" || child.TaskGeneration != 1 {
			t.Fatalf("child identity=%+v", child)
		}
		args := func(action string, generation int, message string) string {
			data, err := json.Marshal(map[string]any{"action": action, "handle": child.Handle, "generation": generation, "message": message})
			if err != nil {
				t.Fatal(err)
			}
			return string(data)
		}
		if results["stale"] == "" {
			return issue("stale", "AgentControl", args("stop", child.Generation+1, ""))
		}
		if !strings.Contains(strings.ToLower(results["stale"]), "stale") {
			t.Fatalf("stale request=%s", results["stale"])
		}
		if results["foreign"] == "" {
			return issue("foreign", "AgentControl", `{"action":"stop","handle":"foreign-session-handle","generation":1}`)
		}
		if !strings.Contains(results["foreign"], "unknown") && !strings.Contains(results["foreign"], "cross-session") {
			t.Fatalf("foreign request=%s", results["foreign"])
		}
		if results["status"] == "" {
			return issue("status", "AgentControl", args("status", child.Generation, ""))
		}
		if strings.Contains(results["status"], "Error:") {
			t.Fatalf("status=%s", results["status"])
		}
		if results["message"] == "" {
			if results["graph"] != "" {
				t.Fatal("graph completed before steering")
			}
			return issue("message", "AgentControl", args("message", child.Generation, feedback))
		}
		var delivered toolsubagent.ControlOutcome
		if err := json.Unmarshal([]byte(results["message"]), &delivered); err != nil || delivered.Delivery != "persisted" {
			t.Fatalf("message lacks persistence proof: %s err=%v", results["message"], err)
		}
		// Independent proof: reopen the child's durable history, not an internal sent
		// flag or the control handler's assertion. Read with test context: replaced
		// model requests can be canceled during the read without invalidating proof.
		after := sessionstore.BeforeFirst
		found := false
		for {
			page, err := store.Items(t.Context(), session.ID(child.SessionID), after, 128)
			if err != nil {
				t.Fatal(err)
			}
			for _, item := range page.Items {
				input, ok := item.Data.(inbox.Input)
				if ok && item.Kind == sessionstore.ItemInput && strings.Contains(string(input.Payload), feedback) {
					found = true
				}
			}
			if !page.More {
				break
			}
			after = page.NextAfter
		}
		if !found {
			t.Fatalf("control completed but child's durable input missing: %s", results["message"])
		}
		if results["stop"] == "" {
			return issue("stop", "AgentControl", args("stop", child.Generation, ""))
		}
		var stopped toolsubagent.ControlOutcome
		if err := json.Unmarshal([]byte(results["stop"]), &stopped); err != nil {
			t.Fatalf("stop=%s err=%v", results["stop"], err)
		}
		if stopped.State != "cancellation_requested" && stopped.State != "drained" {
			t.Fatalf("stop barrier=%+v", stopped)
		}
		if stopped.State == "cancellation_requested" && stopped.DrainConfirmed {
			t.Fatalf("request falsely claims drain=%+v", stopped)
		}
		if results["cancel"] == "" {
			return issue("cancel", "TaskGraph", `{"action":"cancel","name":"live-control"}`)
		}
		if results["graph"] == "" {
			return messageResponse("waiting for actual graph drain"), nil
		}
		finished.Store(true)
		return messageResponse("graph worker steered and drained"), nil
	}}
	config := testConfig(client)
	config.SubagentCommand = []string{executable, "-test.run=^TestAgentControlGraphChildProcess$", "--"}
	var stdout, stderr strings.Builder
	code := RunMain(ctx, []string{"-workspace", t.TempDir(), "-session-directory", sessions}, os.Getenv, os.Environ, strings.NewReader(`{"prompt":"inspect running graph worker","model":"gpt-test","extra_allowed_tools":["Agent"]}`), &stdout, &stderr, config)
	if code != 0 || !finished.Load() {
		t.Fatalf("exit=%d finished=%v stderr=%s output=%s", code, finished.Load(), stderr.String(), stdout.String())
	}
}
