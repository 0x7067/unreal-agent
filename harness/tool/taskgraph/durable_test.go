package taskgraph

import (
	"encoding/json/v2"
	"reflect"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	graph "github.com/unreallabsai/unreal-agent/harness/taskgraph"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

const durableCommandArguments = `{"action":"start","name":"goal","concurrency":2,"tasks":[{"id":"a","command":"true","acceptance":"checked","reads":[],"writes":[]},{"id":"b","command":"true","acceptance":"reviewed","depends_on":["a"]}]}`

func durableTestOperation(spec operation.Spec) operation.Operation {
	return operation.Operation{ID: "op-1", Type: spec.Type, Version: spec.Version, MaxOutputLength: spec.MaxOutputLength, State: spec.State, Idempotency: spec.Idempotency, Status: operation.StatusReady}
}

func durableTestMarshal(t *testing.T, value any) []byte {
	t.Helper()
	data, err := json.Marshal(value)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func durableTestTranslate(t *testing.T, translator tool.Translator, args string) (tool.CallStatus, operation.Operation) {
	t.Helper()
	ctx := &recordingContext{}
	status := translator.Translate(ctx, llm.ToolCall{Arguments: args})
	if status.Error != "" || !reflect.DeepEqual(status.WaitingFor, []operation.ID{"op-1"}) || len(ctx.specs) != 1 {
		t.Fatalf("status=%+v specs=%+v", status, ctx.specs)
	}
	return status, durableTestOperation(ctx.specs[0])
}

func TestDurableWrapsV1PlansWithoutChangingDefault(t *testing.T) {
	args := []string{
		durableCommandArguments,
		`{"action":"revise","name":"goal","tasks":[{"id":"c","command":"true","acceptance":"checked","depends_on":["existing"],"writes":[]}]}`,
		`{"action":"accept","name":"goal","task_id":"a","generation":7,"evidence":"checked revision abc"}`,
		`{"action":"reject","name":"goal","task_id":"a","generation":7,"evidence":"checked failure"}`,
		controlArgs("status"), controlArgs("cancel"), controlArgs("invalidate"), controlArgs("retry"),
	}
	for _, arguments := range args {
		t.Run(arguments, func(t *testing.T) {
			_, legacy := durableTestTranslate(t, New(), arguments)
			v1, err := operation.DecodeRemoteJobState(legacy)
			if err != nil {
				t.Fatal(err)
			}
			if v1.Plan.Version != graph.PlanVersion {
				t.Fatalf("default translator no longer produces v1: %+v", v1.Plan)
			}
			for _, owner := range []struct{ parent, scope string }{{"parent-old", "scope-old"}, {"parent-fresh", "scope-fresh"}} {
				_, op := durableTestTranslate(t, NewDurable(session.ID(owner.parent), owner.scope), arguments)
				state, err := operation.DecodeRemoteJobState(op)
				if err != nil {
					t.Fatal(err)
				}
				if state.Plan.Type != v1.Plan.Type || state.Plan.Version != graph.DurablePlanVersion || len(state.Handle) != 0 || state.TerminalResult != "" || state.TerminalError != "" {
					t.Fatalf("not a fresh inert durable plan: %+v", state)
				}
				var inner []byte
				if state.Plan.Type == graph.RunPlanType {
					p, checkpoint, err := graph.DecodeDurableRun(op)
					if err != nil || checkpoint != nil || p.Version != 1 || p.ParentID != session.ID(owner.parent) {
						t.Fatalf("plan=%+v checkpoint=%+v err=%v", p, checkpoint, err)
					}
					inner = durableTestMarshal(t, p.Run)
				} else {
					p, err := graph.DecodeDurableControl(op)
					if err != nil || p.Version != 1 || p.ParentID != session.ID(owner.parent) || p.Scope != owner.scope {
						t.Fatalf("plan=%+v err=%v", p, err)
					}
					inner = durableTestMarshal(t, p.Control)
				}
				var got, want any
				if err := json.Unmarshal(inner, &got); err != nil {
					t.Fatal(err)
				}
				if err := json.Unmarshal(v1.Plan.Data, &want); err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, want) {
					t.Fatalf("wrapping changed v1 plan (including null/empty claims): got %s want %s", inner, v1.Plan.Data)
				}
				if op.Type != legacy.Type || op.Version != legacy.Version || op.MaxOutputLength != legacy.MaxOutputLength {
					t.Fatalf("remote operation envelope changed: %+v", op)
				}
			}
		})
	}
}

func TestDurableRejectsInvalidIdentityWithoutSubmit(t *testing.T) {
	invalid := []string{"", " leading", "trailing ", ".", "..", "a/b", `a\b`, "a\nb", strings.Repeat("x", 257)}
	for _, identity := range invalid {
		for _, args := range []string{durableCommandArguments, controlArgs("status")} {
			ctx := &recordingContext{}
			status := NewDurable(session.ID(identity), "scope").Translate(ctx, llm.ToolCall{Arguments: args})
			if status.Error == "" || len(status.WaitingFor) != 0 || len(ctx.specs) != 0 {
				t.Fatalf("invalid parent %q submitted: status=%+v specs=%+v", identity, status, ctx.specs)
			}
		}
		// Only control plans carry the caller's scope; run scopes are assigned by the runtime.
		ctx := &recordingContext{}
		status := NewDurable("parent", identity).Translate(ctx, llm.ToolCall{Arguments: controlArgs("status")})
		if status.Error == "" || len(status.WaitingFor) != 0 || len(ctx.specs) != 0 {
			t.Fatalf("invalid scope %q submitted: status=%+v specs=%+v", identity, status, ctx.specs)
		}
	}
}

func TestDurableRejectsAgentTasksAndModelSelectedOwnershipWithoutSubmit(t *testing.T) {
	for _, action := range []string{"start", "revise"} {
		args := `{"action":"` + action + `","name":"goal","tasks":[{"id":"agent","prompt":"inspect changes","acceptance":"reviewed"}]}`
		// Agent tasks remain valid in the default v1 translator.
		durableTestTranslate(t, New(), args)
		ctx := &recordingContext{}
		status := NewDurable("parent", "scope").Translate(ctx, llm.ToolCall{Arguments: args})
		if status.Error == "" || len(status.WaitingFor) != 0 || len(ctx.specs) != 0 {
			t.Fatalf("durable agent task submitted: status=%+v specs=%+v", status, ctx.specs)
		}
	}
	for _, field := range []string{`"parent_id":"attacker"`, `"scope":"attacker"`, `"version":2`, `"run":{}`, `"control":{}`} {
		for _, args := range []string{durableCommandArguments, controlArgs("status")} {
			args = strings.TrimSuffix(args, "}") + "," + field + "}"
			ctx := &recordingContext{}
			status := NewDurable("trusted-parent", "trusted-scope").Translate(ctx, llm.ToolCall{Arguments: args})
			if status.Error == "" || len(status.WaitingFor) != 0 || len(ctx.specs) != 0 {
				t.Fatalf("model ownership accepted for %s: status=%+v specs=%+v", args, status, ctx.specs)
			}
		}
	}
}

func TestDurableResultProjectionPreservesSemanticsAndPersistedOperations(t *testing.T) {
	for _, args := range []string{durableCommandArguments, controlArgs("status")} {
		tr := NewDurable("parent", "scope")
		status, op := durableTestTranslate(t, tr, args)
		_, legacy := durableTestTranslate(t, New(), args)
		for _, phase := range []operation.Status{operation.StatusReady, operation.StatusAwaiting, operation.StatusCanceling, operation.StatusCompleted, operation.StatusFailed, operation.StatusCanceled} {
			for _, terminalError := range []string{"", "failure details"} {
				t.Run(string(phase)+"/"+args+"/"+terminalError, func(t *testing.T) {
					current, v1 := op, legacy
					for _, target := range []*operation.Operation{&current, &v1} {
						state, err := operation.DecodeRemoteJobState(*target)
						if err != nil {
							t.Fatal(err)
						}
						state.TerminalResult, state.TerminalError = "runtime update or partial output", terminalError
						state.ResultBytes, state.ResultTruncated = 1000, true
						state.ErrorBytes = len(terminalError)
						target.State = durableTestMarshal(t, state)
						target.Status = phase
					}
					ops := []operation.Operation{current}
					// Serialize before projection so shared State backing bytes cannot hide mutation.
					before := string(durableTestMarshal(t, ops))
					got, err := tr.TranslateResult("call-id", status, ops)
					if err != nil {
						t.Fatal(err)
					}
					want, err := New().TranslateResult("call-id", status, []operation.Operation{v1})
					if err != nil {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(got, want) || !reflect.DeepEqual(got.ToLLMResult(), want.ToLLMResult()) {
						t.Fatalf("durable result=%+v, v1=%+v", got, want)
					}
					if after := string(durableTestMarshal(t, ops)); after != before {
						t.Fatalf("persisted operations mutated: before=%s after=%s", before, after)
					}
				})
			}
		}
	}
}

func TestDurableResultRejectsLegacyUnknownAndNonStrictPlans(t *testing.T) {
	for _, args := range []string{durableCommandArguments, controlArgs("status")} {
		tr := NewDurable("parent", "scope")
		status, op := durableTestTranslate(t, tr, args)
		_, legacy := durableTestTranslate(t, New(), args)
		if _, err := tr.TranslateResult("call", status, []operation.Operation{legacy}); err == nil {
			t.Fatal("durable result accepted legacy v1 plan")
		}
		mutations := map[string]func(*operation.Operation, *operation.RemoteJobState){
			"legacy plan version": func(_ *operation.Operation, s *operation.RemoteJobState) { s.Plan.Version = graph.PlanVersion },
			"unknown plan version": func(_ *operation.Operation, s *operation.RemoteJobState) {
				s.Plan.Version = graph.DurablePlanVersion + 1
			},
			"unknown plan type":         func(_ *operation.Operation, s *operation.RemoteJobState) { s.Plan.Type = "unknown" },
			"legacy operation version":  func(o *operation.Operation, _ *operation.RemoteJobState) { o.Version-- },
			"unknown operation version": func(o *operation.Operation, _ *operation.RemoteJobState) { o.Version++ },
			"wrong operation type":      func(o *operation.Operation, _ *operation.RemoteJobState) { o.Type = operation.TypeValue },
			"unknown status":            func(o *operation.Operation, _ *operation.RemoteJobState) { o.Status = "unknown" },
			"unknown wrapper member": func(_ *operation.Operation, s *operation.RemoteJobState) {
				s.Plan.Data = append([]byte(`{"unknown":true,`), s.Plan.Data[1:]...)
			},
			"unknown wrapper version": func(_ *operation.Operation, s *operation.RemoteJobState) {
				s.Plan.Data = []byte(strings.Replace(string(s.Plan.Data), `"version":1`, `"version":99`, 1))
			},
			"invalid parent": func(_ *operation.Operation, s *operation.RemoteJobState) {
				s.Plan.Data = []byte(strings.Replace(string(s.Plan.Data), `"parent_id":"parent"`, `"parent_id":"../escape"`, 1))
			},
			"unknown nested plan member": func(_ *operation.Operation, s *operation.RemoteJobState) {
				s.Plan.Data = []byte(strings.Replace(string(s.Plan.Data), `"name":"goal"`, `"name":"goal","unknown":true`, 1))
			},
			"negative output metadata": func(_ *operation.Operation, s *operation.RemoteJobState) { s.ResultBytes = -1 },
		}
		for name, mutate := range mutations {
			t.Run(name+"/"+args, func(t *testing.T) {
				bad := op
				state, err := operation.DecodeRemoteJobState(bad)
				if err != nil {
					t.Fatal(err)
				}
				mutate(&bad, &state)
				bad.State = durableTestMarshal(t, state)
				before := string(durableTestMarshal(t, bad))
				if _, err := tr.TranslateResult("call", status, []operation.Operation{bad}); err == nil {
					t.Fatalf("accepted invalid durable result: %+v", bad)
				}
				if string(durableTestMarshal(t, bad)) != before {
					t.Fatal("rejected operation was mutated")
				}
			})
		}
		for _, prefix := range []string{`{"unknown":true,`, `{"Plan":{"unknown":true,`} {
			bad := op
			if strings.HasPrefix(prefix, `{"Plan":`) {
				bad.State = []byte(strings.Replace(string(op.State), `{"Plan":{`, prefix, 1))
			} else {
				bad.State = append([]byte(prefix), op.State[1:]...)
			}
			if string(bad.State) == string(op.State) {
				t.Fatal("unknown-member mutation missed envelope")
			}
			if _, err := tr.TranslateResult("call", status, []operation.Operation{bad}); err == nil {
				t.Fatal("accepted unknown remote envelope member")
			}
		}
	}
}

func TestDurableResultValidationErrorAndOperationCardinality(t *testing.T) {
	tr := NewDurable("parent", "scope")
	status := tool.ErrorStatus("invalid arguments", 0)
	got, err := tr.TranslateResult("invalid-call", status, nil)
	if err != nil || !reflect.DeepEqual(got, Result{CallID: "invalid-call", Error: status.Error}) {
		t.Fatalf("result=%+v err=%v", got, err)
	}
	validStatus, op := durableTestTranslate(t, tr, controlArgs("status"))
	if _, err := tr.TranslateResult("call", status, []operation.Operation{op}); err == nil {
		t.Fatal("accepted mixed validation error and operation")
	}
	for _, ops := range [][]operation.Operation{nil, {op, op}} {
		if _, err := tr.TranslateResult("call", validStatus, ops); err == nil {
			t.Fatalf("accepted %d operations", len(ops))
		}
	}
}
