package main

import (
	"bytes"
	"encoding/json/v2"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/taskgraph"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	"github.com/unreallabsai/unreal-agent/harness/tool/bash"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
	tooltaskgraph "github.com/unreallabsai/unreal-agent/harness/tool/taskgraph"
	"github.com/unreallabsai/unreal-agent/harness/tool/viewimage"
)

// Exercise the production frontend registry after the actual worker JSONL wire
// round trip, rather than a test-only registry backed by executable runtimes.
func TestWorkerToolResultsAndReplay(t *testing.T) {
	workspace := t.TempDir()
	command := "printf executed > '" + filepath.Join(workspace, "must-not-execute") + "'"
	for _, tc := range []struct {
		name, arguments, completedText string
		resultType                     tool.Result
	}{
		{tool.BashName, `{"command":` + string(workerResultJSON(t, command)) + `}`, "worker stdout", bash.Result{}},
		{tool.AgentName, `{"name":"helper","prompt":"Do not launch me"}`, "worker remote result", toolsubagent.Result{}},
		{tool.SendMessageName, `{"to":"helper","message":"Do not deliver me"}`, "worker remote result", toolsubagent.Result{}},
		{tool.TaskGraphName, `{"action":"start","name":"never-start","tasks":[{"id":"write","command":` + string(workerResultJSON(t, command)) + `,"acceptance":"not executed"}]}`, "worker remote result", tooltaskgraph.Result{}},
		{tool.ViewImageName, `{"path":"missing-image.png"}`, "Image: image/png · 12×8", viewimage.Result{}},
		{tool.SkillUseName, `{"name":"missing-skill"}`, "worker skill instructions", tool.SkillUseResult{}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var frames bytes.Buffer
			writeItem := func(kind sessionstore.ItemKind, data any) {
				t.Helper()
				item := sessionstore.Item{Kind: kind, Data: data}
				if err := workerproto.Write(&frames, workerproto.Event{Type: "item", Item: &item}); err != nil {
					t.Fatal(err)
				}
			}
			writeItem(sessionstore.ItemModelResponse, sessionstore.ModelResponse{
				TurnID: "turn", Response: llm.Response{Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{
					CallID: "call", Name: tc.name, Arguments: tc.arguments,
				}}}},
			})
			type expectation struct{ status, failure, text string }
			var expected []expectation
			for _, phase := range []operation.Status{operation.StatusReady, operation.StatusAwaiting, operation.StatusCanceling, operation.StatusCompleted, operation.StatusFailed, operation.StatusCanceled} {
				op := workerResultOperation(t, tc.name, phase, workspace, command)
				writeItem(sessionstore.ItemToolCallStatus, sessionstore.ToolCallStatus{
					TurnID: "turn", CallID: "call", Status: tool.CallStatus{WaitingFor: []operation.ID{op.ID}}, Operations: []operation.Operation{op},
				})
				want := expectation{status: awaiting}
				switch phase {
				case operation.StatusCompleted:
					want = expectation{status: "Completed", text: tc.completedText}
				case operation.StatusFailed:
					want = expectation{status: "Failed", failure: "worker operation failed", text: "worker operation failed"}
				case operation.StatusCanceled:
					want = expectation{status: "Canceled"}
				}
				expected = append(expected, want)
			}
			if tc.name == tool.BashName {
				op := workerResultOperation(t, tc.name, operation.StatusCompleted, workspace, command)
				state, err := operation.DecodeShellState(op)
				if err != nil {
					t.Fatal(err)
				}
				state.Result.ExitCode = 7
				op.State = workerResultJSON(t, state)
				writeItem(sessionstore.ItemToolCallStatus, sessionstore.ToolCallStatus{TurnID: "turn", CallID: "call", Operations: []operation.Operation{op}})
				expected = append(expected, expectation{"Failed", "Bash exited with code 7", "Exit code: 7"})
			}
			writeItem(sessionstore.ItemToolCallStatus, sessionstore.ToolCallStatus{TurnID: "turn", CallID: "call", Status: tool.CallStatus{Error: "worker validation failed"}})
			expected = append(expected, expectation{"Failed", "worker validation failed", "worker validation failed"})

			var firstReplay []toolCard
			for replay := range 2 {
				m := toolsModel(t, defaultTheme, 140, 35, 0)
				// toolsModel deliberately uses placeholders. Override with exactly
				// the production frontend registry, never newTaskOperations.
				m.registry = newUIRegistry(true, true)
				m.workspace, m.directory = workspace, workspace
				m.workerEvents = newWorkerEvents(t.Context())
				scanner := workerproto.Scanner(bytes.NewReader(frames.Bytes()))
				step := -1
				var cards []toolCard
				for scanner.Scan() {
					var event workerproto.Event
					if err := json.Unmarshal(scanner.Bytes(), &event); err != nil {
						t.Fatal(err)
					}
					if event.Type != "item" || event.Item == nil {
						t.Fatalf("invalid worker event: %+v", event)
					}
					m = update(m, workerEvent{message: *event.Item})
					if event.Item.Kind == sessionstore.ItemModelResponse {
						continue
					}
					step++
					data := event.Item.Data.(sessionstore.ToolCallStatus)
					translator, ok := m.registry.Resolve(tc.name)
					if !ok {
						t.Fatalf("production registry cannot resolve %s", tc.name)
					}
					result, err := translator.TranslateResult(data.CallID, data.Status, data.Operations)
					if err != nil || reflect.TypeOf(result) != reflect.TypeOf(tc.resultType) {
						t.Fatalf("replay %d step %d: result %T, error %v; want %T", replay, step, result, err, tc.resultType)
					}
					card := m.tools[0]
					want := expected[step]
					if len(m.tools) != 1 || card.name != tc.name || card.status != want.status || card.failure != want.failure || (want.text != "" && !strings.Contains(card.output, want.text)) {
						t.Fatalf("replay %d step %d: card=%+v, want=%+v", replay, step, card, want)
					}
					if strings.Contains(card.output+card.failure, "Unsupported tool result") {
						t.Fatalf("unsupported result: %+v", card)
					}
					if tc.name == tool.BashName && card.status == "Completed" {
						if !strings.Contains(card.output, "worker stderr") || !strings.Contains(card.paths, filepath.Join(workspace, "capture", "out")) {
							t.Fatalf("lost worker output/capture paths: %+v", card)
						}
					}
					cards = append(cards, card)
				}
				if err := scanner.Err(); err != nil {
					t.Fatal(err)
				}
				if step+1 != len(expected) {
					t.Fatalf("received %d results, want %d", step+1, len(expected))
				}
				if replay == 0 {
					firstReplay = cards
				} else if !reflect.DeepEqual(cards, firstReplay) {
					t.Fatalf("fresh frontend replay changed tool cards:\nfirst: %+v\nsecond: %+v", firstReplay, cards)
				}
			}
			// No process, graph, image read, skill read, capture directory or
			// subagent runtime is needed to render/replay worker-owned results.
			entries, err := os.ReadDir(workspace)
			if err != nil || len(entries) != 0 {
				t.Fatalf("frontend replay produced filesystem side effects: %v, %v", entries, err)
			}
		})
	}
}

func workerResultJSON(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func workerResultOperation(t *testing.T, name string, phase operation.Status, workspace, command string) operation.Operation {
	t.Helper()
	op := operation.Operation{ID: "worker-operation", Status: phase, MaxOutputLength: operation.DefaultMaxOutputLength}
	failure := ""
	if phase == operation.StatusFailed || phase == operation.StatusCanceled {
		failure = "worker operation failed"
	}
	switch name {
	case tool.BashName:
		op.Type, op.Version = operation.TypeShell, operation.VersionShell
		state := operation.ShellState{
			Input: operation.ShellInput{Command: command, Shell: "/bin/sh", Directory: workspace}, BaseDirectory: workspace,
			TerminalError: failure, OutPath: filepath.Join(workspace, "capture", "out"), ErrPath: filepath.Join(workspace, "capture", "err"),
		}
		if phase == operation.StatusCompleted {
			state.Result = &operation.ShellResult{Out: "worker stdout", Err: "worker stderr"}
		}
		op.State = workerResultJSON(t, state)
	case tool.ViewImageName:
		op.Type, op.Version = operation.TypeViewImage, operation.VersionViewImage
		state := operation.ViewImageState{Path: filepath.Join(workspace, "missing-image.png"), Config: operation.ViewImageConfig{MaxSize: 100, MaxWidth: 100, MaxHeight: 100}}
		if phase == operation.StatusCompleted {
			state.Result = &operation.ViewImageResult{Content: "aW1hZ2U=", EncodedMIMEType: "image/png", OriginalMIMEType: "image/png", OriginalWidth: 12, OriginalHeight: 8, ScaleRatio: 1}
		} else if failure != "" {
			state.Result = &operation.ViewImageResult{Error: failure}
		}
		op.State = workerResultJSON(t, state)
	case tool.SkillUseName:
		op.Type, op.Version = operation.TypeSkillUse, operation.VersionSkillUse
		state := operation.SkillUseState{Path: filepath.Join(workspace, "missing-skill", "SKILL.md"), TerminalError: failure}
		if phase == operation.StatusCompleted {
			state.Content = []byte("worker skill instructions")
		}
		op.State = workerResultJSON(t, state)
	default:
		op.Type, op.Version = operation.TypeRemoteJob, operation.VersionRemoteJob
		plan := operation.RemoteJobPlan{Type: toolsubagent.RunPlanType, Version: toolsubagent.PlanVersion, Data: workerResultJSON(t, toolsubagent.RunPlan{Name: "helper", Prompt: "Do not launch me"})}
		if name == tool.SendMessageName {
			plan.Type, plan.Data = toolsubagent.MessagePlanType, workerResultJSON(t, toolsubagent.MessagePlan{To: "helper", Message: "Do not deliver me"})
		} else if name == tool.TaskGraphName {
			plan.Type, plan.Version, plan.Data = taskgraph.RunPlanType, taskgraph.PlanVersion, workerResultJSON(t, taskgraph.RunPlan{Name: "never-start", Tasks: []taskgraph.Task{{ID: "write", Command: command, Acceptance: "not executed"}}})
		} else if name != tool.AgentName {
			t.Fatal(fmt.Sprintf("unknown fixture tool %q", name))
		}
		op.State = workerResultJSON(t, operation.RemoteJobState{Plan: plan, TerminalResult: "worker remote result", TerminalError: failure})
	}
	return op
}

func TestUnavailableToolResultPreservesConfigurationError(t *testing.T) {
	m := toolsModel(t, defaultTheme, 140, 35, 0)
	m.registry = tool.NewRegistry(tool.StaticTranslators{}, tool.BashName)
	card := &toolCard{name: tool.BashName}
	result := m.readToolResult(card, sessionstore.ToolCallStatus{CallID: "unconfigured"})
	text, _ := resultText(result)
	want := `static tool "Bash" is not configured`
	if card.status != "Failed" || card.failure != want || text != want {
		t.Fatalf("card=%+v, text=%q; want configuration error %q", card, text, want)
	}
}
