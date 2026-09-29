package tool

import (
	"encoding/json/v2"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/primitives"
)

func TestSkillUseLoadsDiscoveredSkillByName(t *testing.T) {
	directory := t.TempDir()
	path := filepath.Join(directory, "review", "SKILL.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	want := "---\nname: \"review\" # stable name\ndescription: >-\n  Review code\n  carefully.\n---\n\nRead all instructions.\n"
	if err := os.WriteFile(path, []byte(want), 0o600); err != nil {
		t.Fatal(err)
	}
	skills, skillErrors := DiscoverSkills(directory)
	if len(skillErrors) != 0 || len(skills) != 1 {
		t.Fatalf("skills = %#v, errors = %v", skills, skillErrors)
	}
	registry := NewRegistry(StaticTranslators{}, SkillUseName)
	if _, err := registry.RegisterSkill(skills[0]); err != nil {
		t.Fatal(err)
	}
	translator, exists := registry.Resolve(SkillUseName)
	if !exists {
		t.Fatal("SkillUse is not registered")
	}
	ctx := &recordingContext{}
	status := translator.Translate(ctx, llm.ToolCall{Name: SkillUseName, Arguments: `{"name":"review"}`})
	if status.Error != "" || len(ctx.specs) != 1 ||
		len(status.WaitingFor) != 1 || status.WaitingFor[0] != "operation-1" {
		t.Fatalf("status = %#v, specs = %#v", status, ctx.specs)
	}

	current := operation.Operation{
		ID: "operation-1", Type: ctx.specs[0].Type, Version: ctx.specs[0].Version,
		Status: operation.StatusReady, State: ctx.specs[0].State,
	}
	manager := operation.NewLocalOperationManager(t.Context())
	if err := manager.Add(current); err != nil {
		t.Fatal(err)
	}
	completed := receiveSkillUseOperation(t, manager.Updates(), current.ID)
	result, err := translator.TranslateResult("call-1", status, []operation.Operation{completed})
	if err != nil {
		t.Fatal(err)
	}
	resultLLM := result.ToLLMResult()
	raw, ok := result.(SkillUseResult)
	if !ok || raw.CallID != "call-1" || raw.Running ||
		string(raw.Content) != want {
		t.Fatalf("structured result = %#v, ok = %t", raw, ok)
	}
	if resultLLM.CallID != "call-1" || resultLLM.Output[0].Value != want {
		t.Fatalf("result = %#v", result)
	}
}

func TestSkillUseRejectsInvalidSelection(t *testing.T) {
	registry := NewRegistry(StaticTranslators{}, SkillUseName)
	translator, exists := registry.Resolve(SkillUseName)
	if !exists {
		t.Fatal("SkillUse is not registered")
	}
	for _, test := range []struct {
		name      string
		call      llm.ToolCall
		wantError string
	}{
		{name: "wrong tool", call: llm.ToolCall{Name: "Other"}, wantError: "does not match"},
		{name: "invalid JSON", call: llm.ToolCall{Arguments: `{`}, wantError: "decode skill-use arguments"},
		{name: "missing name", call: llm.ToolCall{Arguments: `{}`}, wantError: `argument "name" must be set`},
		{name: "unknown skill", call: llm.ToolCall{Arguments: `{"name":"missing"}`}, wantError: `skill "missing" is not registered`},
	} {
		t.Run(test.name, func(t *testing.T) {
			status := translator.Translate(&recordingContext{}, test.call)
			if !strings.Contains(status.Error, test.wantError) || len(status.WaitingFor) != 0 {
				t.Fatalf("status = %#v", status)
			}
		})
	}
}

func TestSkillUseFailureKeepsPartialContentOutOfModelOutput(t *testing.T) {
	partial := []byte("partial skill instructions")
	encoded, err := json.Marshal(operation.SkillUseState{Path: "SKILL.md", Content: partial})
	if err != nil {
		t.Fatal(err)
	}
	current := operation.Operation{
		ID: "skill-operation", Type: operation.TypeSkillUse, Version: operation.VersionSkillUse,
		Status: operation.StatusAwaiting, State: encoded,
	}
	step, err := operation.AdvanceSkillUse(current, nil)
	if err != nil {
		t.Fatal(err)
	}
	read := step.Dispatches[0].Data.(primitives.IOReadRequest)
	for _, test := range []struct {
		name  string
		event primitives.PrimitiveEvent
		want  string
	}{
		{name: "canceled", event: primitives.PrimitiveEvent{Type: primitives.PrimitiveEventCanceled}, want: "skill-use operation canceled"},
		{
			name: "read failed", want: "read failed",
			event: primitives.PrimitiveEvent{Type: primitives.PrimitiveEventFailed, Result: primitives.PrimitiveFailureResult{Error: "read failed"}},
		},
		{
			name: "size mismatch", want: "skill read returned an invalid completion",
			event: primitives.PrimitiveEvent{Type: primitives.PrimitiveEventIOReadCompleted, Result: primitives.IOReadCompletedResult{Size: int64(len(partial) + 1)}},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			event := test.event
			event.Source = read.Source
			event.CorrelationID = read.CorrelationID
			terminal, err := operation.AdvanceSkillUse(*step.Operation, &event)
			if err != nil {
				t.Fatal(err)
			}
			result, err := (&skillUseTranslator{}).TranslateResult("skill-call", CallStatus{}, []operation.Operation{*terminal.Operation})
			if err != nil {
				t.Fatal(err)
			}
			skill, ok := result.(SkillUseResult)
			if !ok || string(skill.Content) != string(partial) || skill.Error != test.want {
				t.Fatalf("structured skill result = %#v", result)
			}
			model := result.ToLLMResult()
			if model.CallID != "skill-call" || model.Running || len(model.Output) != 1 ||
				model.Output[0].Kind != llm.ToolResultText || model.Output[0].Value != test.want {
				t.Fatalf("model result exposed partial instructions: %#v", model)
			}
		})
	}
}

func receiveSkillUseOperation(
	t *testing.T,
	updates <-chan operation.Operation,
	id operation.ID,
) operation.Operation {
	t.Helper()
	timer := time.NewTimer(15 * time.Second)
	defer timer.Stop()
	for {
		select {
		case current := <-updates:
			if current.ID == id && (current.Status == operation.StatusCompleted ||
				current.Status == operation.StatusFailed || current.Status == operation.StatusCanceled) {
				return current
			}
		case <-timer.C:
			t.Fatal("timed out waiting for skill-use operation")
		case <-t.Context().Done():
			t.Fatal(t.Context().Err())
		}
	}
}
