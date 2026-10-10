package main

import (
	"context"
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

type graphSubmitter struct {
	manager operation.Manager
	next    int
	err     error
}

func (s *graphSubmitter) Submit(spec operation.Spec) operation.ID {
	s.next++
	id := operation.ID("operation-" + string(rune('a'+s.next)))
	s.err = s.manager.Add(operation.Operation{ID: id, Type: spec.Type, Version: spec.Version, State: spec.State, Status: operation.StatusReady, MaxOutputLength: spec.MaxOutputLength})
	return id
}

func TestTaskOperationsGraphLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(t.Context(), 5*time.Second)
	defer cancel()
	workspace := t.TempDir()
	inputs, err := inbox.New(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	registry, manager, err := newTaskOperations(ctx, workspace, t.TempDir(), "/bin/sh", inputs, false, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		cancel()
		for range manager.Updates() {
		}
	}()
	if _, enabled := registry.Resolve(tool.AgentName); enabled {
		t.Fatal("Agent enabled without child factory")
	}
	translator, enabled := registry.Resolve(tool.TaskGraphName)
	if !enabled {
		t.Fatal("TaskGraph unavailable")
	}
	submitter := &graphSubmitter{manager: manager}
	status := translator.Translate(submitter, llm.ToolCall{CallID: "graph", Arguments: `{"action":"start","name":"ui","tasks":[{"id":"write","command":"printf proof > result.txt","reads":[],"writes":["result.txt"],"acceptance":"proof file"}]}`})
	if submitter.err != nil || status.Error != "" {
		t.Fatalf("start=%+v error=%v", status, submitter.err)
	}
	palette, err := loadTheme(defaultTheme)
	if err != nil {
		t.Fatal(err)
	}
	m := newModel(ctx, inputs, registry, workspace, t.TempDir(), options{theme: palette})
	card := &toolCard{name: tool.TaskGraphName}
	accepted, finished, running := false, false, false
	for !finished {
		select {
		case input := <-inputs.Output():
			var text string
			if err := json.Unmarshal(input.Payload, &text); err != nil {
				t.Fatal(err)
			}
			if strings.Contains(text, "generation 1: completed") {
				contents, err := os.ReadFile(filepath.Join(workspace, "result.txt"))
				if err != nil || string(contents) != "proof" {
					t.Fatalf("candidate=%q %v", contents, err)
				}
				control := translator.Translate(submitter, llm.ToolCall{CallID: "accept", Arguments: `{"action":"accept","name":"ui","task_id":"write","generation":1,"evidence":"read proof file"}`})
				if control.Error != "" || submitter.err != nil {
					t.Fatalf("accept=%+v %v", control, submitter.err)
				}
				accepted = true
			}
		case op := <-manager.Updates():
			if op.ID != status.WaitingFor[0] {
				continue
			}
			result := m.readToolResult(card, sessionstore.ToolCallStatus{CallID: "graph", Status: status, Operations: []operation.Operation{op}})
			if card.failure != "" {
				t.Fatalf("card failed: %s", card.failure)
			}
			switch op.Status {
			case operation.StatusAwaiting:
				running = true
				if card.status != awaiting {
					t.Fatalf("running card=%s", card.status)
				}
			case operation.StatusCompleted:
				text, _ := resultText(result)
				if !accepted || card.status != "Completed" || !strings.Contains(text, "all current task generations accepted") {
					t.Fatalf("accepted=%v card=%+v text=%q", accepted, card, text)
				}
				finished = true
			}
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		}
	}
	if !running {
		t.Fatal("no running graph result")
	}
}

func TestSubagentFactoryOwnsFreshChannels(t *testing.T) {
	parentCtx, stopParent := context.WithCancel(t.Context())
	defer stopParent()
	graphCtx, stopGraph := context.WithCancel(t.Context())
	defer stopGraph()
	inputs, err := inbox.New(parentCtx, nil)
	if err != nil {
		t.Fatal(err)
	}
	factory, log, err := newSubagentFactory(options{}, t.TempDir(), t.TempDir(), t.TempDir(), inputs)
	if err != nil {
		t.Fatal(err)
	}
	defer log.Close()
	parent, err := factory(parentCtx)
	if err != nil {
		t.Fatal(err)
	}
	graph, err := factory(graphCtx)
	if err != nil {
		t.Fatal(err)
	}
	if len(parent) != 2 || len(graph) != 2 {
		t.Fatalf("handlers: %d %d", len(parent), len(graph))
	}
	for i := range parent {
		if parent[i].RemoteJobUpdates() == graph[i].RemoteJobUpdates() {
			t.Fatal("supervisors share update consumers")
		}
	}
	parentManager := operation.NewLocalOperationManager(parentCtx, parent...)
	graphManager := operation.NewLocalOperationManager(graphCtx, graph...)
	stopGraph()
	select {
	case _, ok := <-graphManager.Updates():
		if ok {
			t.Fatal("unexpected graph update")
		}
	case <-time.After(time.Second):
		t.Fatal("graph manager did not terminate")
	}
	select {
	case <-parentManager.Updates():
		t.Fatal("graph cancellation closed parent manager")
	default:
	}
	stopParent()
	for range parentManager.Updates() {
	}
}
