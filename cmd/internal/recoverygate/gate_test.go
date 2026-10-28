package recoverygate

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"fmt"
	"path/filepath"
	"time"

	"errors"
	"github.com/unreallabsai/unreal-agent/harness/executionbudget"
	"github.com/unreallabsai/unreal-agent/harness/primitives"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/operation"
)

func ready() operation.Operation {
	return operation.Operation{ID: "value-1", Type: operation.TypeValue, Version: operation.VersionValue, Status: operation.StatusReady, State: jsontext.Value(`{"Value":{"ok":true}}`)}
}

func TestValidReadyValue(t *testing.T) {
	for _, state := range []string{`{"Value":{"ok":true}}`, `{"Value":null}`, `{"Value":[1,"a"]}`} {
		op := ready()
		op.State = jsontext.Value(state)
		if err := Validate([]operation.Operation{op}); err != nil {
			t.Fatal(err)
		}
	}
	if err := Validate(nil); err != nil {
		t.Fatal(err)
	}
}

func TestRefusedStates(t *testing.T) {
	for _, state := range []string{"", "{", `{}`, `{"Value":}`, `{"Value":1,"Value":2}`, `null`} {
		t.Run(state, func(t *testing.T) {
			op := ready()
			op.State = jsontext.Value(state)
			_, original := operation.DecodeValue(op)
			if original == nil {
				t.Fatalf("fixture unexpectedly decodes: %s", state)
			}
			err := Validate([]operation.Operation{op})
			if err == nil || errors.Unwrap(err) == nil || errors.Unwrap(err).Error() != original.Error() {
				t.Fatalf("original cause lost: %v", err)
			}
			for _, s := range []string{`id="value-1"`, `type="value"`, `version=1`, `status="ready"`, "invalid value state", original.Error()} {
				if !strings.Contains(err.Error(), s) {
					t.Errorf("%q missing %q", err, s)
				}
			}
		})
	}
}

func TestRecoveryTypeVersionStatusMatrix(t *testing.T) {
	image, err := operation.NewViewImageSpec("/owned/image.png", operation.ViewImageConfig{MaxSize: 1024, MaxWidth: 16, MaxHeight: 16})
	if err != nil {
		t.Fatal(err)
	}
	types := []operation.Type{operation.TypeValue, operation.TypeViewImage, operation.TypeSkillUse, operation.TypeShell, operation.TypeRemoteJob, "", "unknown", "subagent.run", "taskgraph.run"}
	statuses := []operation.Status{operation.StatusReady, operation.StatusAwaiting, operation.StatusCanceling, operation.StatusUnsupported, "unknown", "running"}
	for _, typ := range types {
		for _, version := range []operation.Version{0, 1, 2, 3, 4, 5, ^operation.Version(0)} {
			for _, status := range statuses {
				op := ready()
				op.Type = typ
				if typ == operation.TypeViewImage {
					op.State = image.State.Clone()
				}
				op.Version = version
				op.Status = status
				err := Validate([]operation.Operation{op})
				allowed := typ == operation.TypeValue && version == operation.VersionValue && status == operation.StatusReady || typ == operation.TypeViewImage && version == operation.VersionViewImage && (status == operation.StatusReady || status == operation.StatusAwaiting)
				if allowed != (err == nil) {
					t.Fatalf("%+v: %v", op, err)
				}
				if !allowed && !errors.Is(err, operation.ErrUnsupported) {
					t.Fatalf("not conservative unsupported error: %v", err)
				}
			}
		}
	}
	op := ready()
	op.ID = ""
	if err := Validate([]operation.Operation{op}); err == nil || !strings.Contains(err.Error(), "missing operation identity") {
		t.Fatalf("empty identity: %v", err)
	}
}

func TestTerminalHistoryIsInert(t *testing.T) {
	for _, status := range []operation.Status{operation.StatusCompleted, operation.StatusFailed, operation.StatusCanceled} {
		for _, typ := range []operation.Type{"", "unknown", operation.TypeValue, operation.TypeShell, operation.TypeRemoteJob} {
			op := operation.Operation{Type: typ, Version: ^operation.Version(0), Status: status, State: jsontext.Value(`not JSON`)}
			if err := Validate([]operation.Operation{op}); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestDeterministicBoundedDiagnosticsAndNoMutation(t *testing.T) {
	a := ready()
	a.ID = "a"
	a.State = jsontext.Value(`{`)
	b := a
	b.State = jsontext.Value(`{}`)
	b.Idempotency = jsontext.Value(`{"key":1}`)
	c := ready()
	c.ID = "z"
	c.Type = operation.TypeShell
	ops := []operation.Operation{c, b, a}
	before := make([]operation.Operation, len(ops))
	for i, op := range ops {
		before[i] = op
		before[i].State = slices.Clone(op.State)
		before[i].Idempotency = slices.Clone(op.Idempotency)
	}
	first := Validate(ops)
	if first == nil || !strings.Contains(first.Error(), `id="a"`) {
		t.Fatalf("wrong first refusal: %v", first)
	}
	if !reflect.DeepEqual(ops, before) {
		t.Fatal("input mutated")
	}
	for _, order := range [][]operation.Operation{{a, b, c}, {b, c, a}, {c, a, b}, {b, a, c}, {a, c, b}} {
		if got := Validate(order); got == nil || got.Error() != first.Error() {
			t.Fatalf("unstable refusal: %v vs %v", got, first)
		}
	}
	huge := ready()
	huge.ID = operation.ID(strings.Repeat("\x00\n", 10000))
	huge.Type = operation.Type(strings.Repeat("type", 10000))
	huge.Status = operation.Status(strings.Repeat("status", 10000))
	err := Validate([]operation.Operation{huge})
	if err == nil || len(err.Error()) > 2048 || strings.Contains(err.Error(), "\n") {
		t.Fatalf("unbounded/unsafe diagnostic: %v", err)
	}
	for _, label := range []string{"id=", "type=", "version=", "status=", "..."} {
		if !strings.Contains(err.Error(), label) {
			t.Errorf("missing %s", label)
		}
	}
	huge = ready()
	huge.ID = operation.ID(strings.Repeat("x", 10000))
	huge.State = jsontext.Value(`{`)
	err = Validate([]operation.Operation{huge})
	if err == nil || len(err.Error()) > 2048 || errors.Unwrap(err) == nil || !strings.Contains(err.Error(), "invalid value state") {
		t.Fatalf("decode bounds/cause: %v", err)
	}
	valid := []operation.Operation{ready()}
	saved := slices.Clone(valid[0].State)
	if err := Validate(valid); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(saved, valid[0].State) {
		t.Fatal("valid state mutated")
	}
}

func TestImageRecoveryStatesAreNarrowlyApproved(t *testing.T) {
	spec, err := operation.NewViewImageSpec("/owned/current.png", operation.ViewImageConfig{MaxSize: 1024, MaxHeight: 16, MaxWidth: 16})
	if err != nil {
		t.Fatal(err)
	}
	op := operation.Operation{ID: "image-intent", Type: spec.Type, Version: spec.Version, Status: operation.StatusReady, State: spec.State.Clone()}
	if err := Validate([]operation.Operation{op}); err != nil {
		t.Fatal(err)
	}
	for _, which := range []string{"future", "malformed", "empty-path", "result", "canceling"} {
		t.Run(which, func(t *testing.T) {
			bad := op
			switch which {
			case "future":
				bad.Version++
			case "malformed":
				bad.State = jsontext.Value(`{`)
			case "empty-path":
				bad.State = jsontext.Value(`{"Path":"","Config":{"MaxSize":10,"MaxHeight":1,"MaxWidth":1}}`)
			case "result":
				bad.State = jsontext.Value(`{"Path":"/owned/current.png","Config":{"MaxSize":10,"MaxHeight":1,"MaxWidth":1},"Result":{}}`)
			case "canceling":
				bad.Status = operation.StatusCanceling
			}
			if err := Validate([]operation.Operation{bad}); err == nil {
				t.Fatalf("unclassified image accepted: %+v", bad)
			}
		})
	}
}

func TestImageResultBearingNonterminalStatesRefuse(t *testing.T) {
	for _, status := range []operation.Status{operation.StatusReady, operation.StatusAwaiting} {
		op := operation.Operation{ID: "result-must-be-terminal", Type: operation.TypeViewImage, Version: operation.VersionViewImage, Status: status, State: jsontext.Value(`{"Path":"/owned/image.png","Config":{"MaxSize":1024,"MaxHeight":16,"MaxWidth":16},"Result":{"Content":"untrusted"}}`)}
		if err := Validate([]operation.Operation{op}); !errors.Is(err, operation.ErrUnsupported) {
			t.Fatalf("result-bearing %s accepted: %v", status, err)
		}
	}
}

func shellReadFixture(t *testing.T, version operation.Version, phase operation.ShellPhase) operation.Operation {
	t.Helper()
	input := operation.ShellInput{Shell: "/bin/sh", Command: "printf done"}
	if version == operation.VersionShellBudget {
		input.Budget = executionbudget.Policy{WarnAfter: time.Second, Timeout: 2 * time.Second}
	}
	spec, err := operation.NewShellSpec(input, "/owned/captures", 8)
	if err != nil {
		t.Fatal(err)
	}
	op := operation.Operation{ID: "recorded-exit", Type: spec.Type, Version: version, Status: operation.StatusAwaiting, MaxOutputLength: spec.MaxOutputLength, State: spec.State}
	state, err := operation.DecodeShellState(op)
	if err != nil {
		t.Fatal(err)
	}
	exit := 7
	state.Phase, state.PendingExitCode = phase, &exit
	state.OutPath = filepath.Join(state.BaseDirectory, string(op.ID), operation.ShellOutFilename)
	state.ErrPath = filepath.Join(state.BaseDirectory, string(op.ID), operation.ShellErrFilename)
	switch phase {
	case operation.ShellPhaseReadOutTail:
		state.OutSize, state.InlineOut = 40, make([]byte, 16)
	case operation.ShellPhaseReadErr:
		state.OutSize, state.InlineOut, state.InlineOutTail = 40, make([]byte, 16), make([]byte, 16)
	case operation.ShellPhaseReadErrTail:
		state.OutSize, state.InlineOut = 2, []byte("ok")
		state.ErrSize, state.InlineErr = 40, make([]byte, 16)
	}
	op.State, err = json.Marshal(state)
	if err != nil {
		t.Fatal(err)
	}
	return op
}

func TestShellRecordedExitReadMatrix(t *testing.T) {
	phases := []operation.ShellPhase{"", operation.ShellPhaseCreateDirectory, operation.ShellPhaseCreateOut, operation.ShellPhaseCreateErr, operation.ShellPhaseProcess, operation.ShellPhaseReadOut, operation.ShellPhaseReadOutTail, operation.ShellPhaseReadErr, operation.ShellPhaseReadErrTail, "unknown"}
	for _, version := range []operation.Version{0, 1, 2, 3, 4, 5, ^operation.Version(0)} {
		for _, phase := range phases {
			for _, status := range []operation.Status{operation.StatusReady, operation.StatusAwaiting, operation.StatusCanceling, operation.StatusUnsupported, "running"} {
				t.Run(fmt.Sprintf("v%d/%s/%s", version, phase, status), func(t *testing.T) {
					fixtureVersion := operation.VersionShell
					if version == operation.VersionShellBudget {
						fixtureVersion = version
					}
					op := shellReadFixture(t, fixtureVersion, phase)
					op.Version, op.Status = version, status
					before := op.State.Clone()
					err := Validate([]operation.Operation{op})
					allowed := (version == 3 || version == 4) && status == operation.StatusAwaiting && slices.Contains([]operation.ShellPhase{operation.ShellPhaseReadOut, operation.ShellPhaseReadOutTail, operation.ShellPhaseReadErr, operation.ShellPhaseReadErrTail}, phase)
					if allowed != (err == nil) {
						t.Fatalf("allowed=%v: %v", allowed, err)
					}
					if err != nil && !errors.Is(err, operation.ErrUnsupported) {
						t.Fatal(err)
					}
					if !reflect.DeepEqual(before, op.State) {
						t.Fatal("mutated input")
					}
					if allowed {
						shell, err := operation.NewShell(op)
						if err != nil {
							t.Fatal(err)
						}
						step, err := shell.Handle(nil)
						if err != nil {
							t.Fatal(err)
						}
						if len(step.Dispatches) != 1 || step.Dispatches[0].Type != primitives.PrimitiveDispatchIORead {
							t.Fatalf("not read-only: %+v", step)
						}
					} else if status == operation.StatusReady || status == operation.StatusCanceling || phase == operation.ShellPhaseProcess || phase == operation.ShellPhaseCreateErr {
						if !strings.Contains(err.Error(), "prior execution uncertain") || !strings.Contains(err.Error(), "ownership reconciliation required") {
							t.Fatal(err)
						}
					}
				})
			}
		}
	}
}

func TestShellReadInvalidState(t *testing.T) {
	changes := map[string]func(*operation.ShellState){
		"missing-exit":    func(s *operation.ShellState) { s.PendingExitCode = nil },
		"pgid":            func(s *operation.ShellState) { s.ProcessGroupID = 42 },
		"negative-pgid":   func(s *operation.ShellState) { s.ProcessGroupID = -1 },
		"result":          func(s *operation.ShellState) { s.Result = &operation.ShellResult{} },
		"terminal-error":  func(s *operation.ShellState) { s.TerminalError = "failed" },
		"error-truncated": func(s *operation.ShellState) { s.ErrorTruncated = true },
		"shell-path":      func(s *operation.ShellState) { s.Input.Shell = "sh" },
		"base-path":       func(s *operation.ShellState) { s.BaseDirectory = "relative" },
		"out-path":        func(s *operation.ShellState) { s.OutPath = "/other/out" },
		"err-path":        func(s *operation.ShellState) { s.ErrPath = "" },
		"negative-size":   func(s *operation.ShellState) { s.OutSize = -1 },
		"partial-out":     func(s *operation.ShellState) { s.InlineOut = append(s.InlineOut, 1) },
		"partial-err":     func(s *operation.ShellState) { s.InlineErr = append(s.InlineErr, 1) },
		"partial-tail":    func(s *operation.ShellState) { s.InlineErrTail = []byte{1} },
		"truncation":      func(s *operation.ShellState) { s.OutTruncated = false },
		"budget-negative": func(s *operation.ShellState) { s.Input.Budget.Timeout = -1 },
		"budget-order":    func(s *operation.ShellState) { s.Input.Budget = executionbudget.Policy{WarnAfter: 2, Timeout: 1} },
	}
	for _, version := range []operation.Version{operation.VersionShell, operation.VersionShellBudget} {
		for _, phase := range []operation.ShellPhase{operation.ShellPhaseReadOut, operation.ShellPhaseReadOutTail, operation.ShellPhaseReadErr, operation.ShellPhaseReadErrTail} {
			for name, change := range changes {
				t.Run(fmt.Sprintf("v%d/%s/%s", version, phase, name), func(t *testing.T) {
					op := shellReadFixture(t, version, phase)
					state, err := operation.DecodeShellState(op)
					if err != nil {
						t.Fatal(err)
					}
					change(&state)
					op.State, err = json.Marshal(state)
					if err != nil {
						t.Fatal(err)
					}
					before := op.State.Clone()
					first := Validate([]operation.Operation{op})
					if !errors.Is(first, operation.ErrUnsupported) {
						t.Fatalf("accepted invalid state: %v", first)
					}
					if next := Validate([]operation.Operation{op}); next.Error() != first.Error() || !reflect.DeepEqual(before, op.State) {
						t.Fatal("nondeterministic or mutating")
					}
				})
			}
		}
	}
	for _, version := range []operation.Version{operation.VersionShell, operation.VersionShellBudget} {
		op := shellReadFixture(t, version, operation.ShellPhaseReadOut)
		if err := Validate([]operation.Operation{op}); err != nil {
			t.Fatal(err)
		}
		for _, raw := range []string{"", "{", "null", "{}", `{"Input":1}`, `{"Phase":"read_out","Phase":"read_err"}`} {
			bad := op
			bad.State = jsontext.Value(raw)
			if err := Validate([]operation.Operation{bad}); !errors.Is(err, operation.ErrUnsupported) {
				t.Fatalf("malformed %q: %v", raw, err)
			}
		}
		for _, limit := range []int{0, -1, operation.MaxOutputLength + 1} {
			bad := op
			bad.MaxOutputLength = limit
			if err := Validate([]operation.Operation{bad}); !errors.Is(err, operation.ErrUnsupported) {
				t.Fatal(err)
			}
		}
	}
	op := shellReadFixture(t, operation.VersionShellBudget, operation.ShellPhaseReadOut)
	op.Version = operation.VersionShell
	if err := Validate([]operation.Operation{op}); !errors.Is(err, operation.ErrUnsupported) {
		t.Fatalf("v3 budget misuse: %v", err)
	}
	op = shellReadFixture(t, operation.VersionShell, operation.ShellPhaseReadOut)
	op.ID = "../escape"
	if err := Validate([]operation.Operation{op}); !errors.Is(err, operation.ErrUnsupported) {
		t.Fatalf("identity: %v", err)
	}
}

func TestShellReadTailBoundaryAndZeroBudgetV4(t *testing.T) {
	for _, phase := range []operation.ShellPhase{operation.ShellPhaseReadOutTail, operation.ShellPhaseReadErrTail} {
		op := shellReadFixture(t, operation.VersionShell, phase)
		op.Version = operation.VersionShellBudget // v4 accepts a zero policy too.
		if err := Validate([]operation.Operation{op}); err != nil {
			t.Fatal(err)
		}
		state, err := operation.DecodeShellState(op)
		if err != nil {
			t.Fatal(err)
		}
		if phase == operation.ShellPhaseReadOutTail {
			state.OutSize = 32
		} else {
			state.ErrSize = 32
		}
		op.State, err = json.Marshal(state)
		if err != nil {
			t.Fatal(err)
		}
		if err := Validate([]operation.Operation{op}); !errors.Is(err, operation.ErrUnsupported) {
			t.Fatalf("tail at read limit: %v", err)
		}
	}
}

func TestShellReadUnknownMembersRefused(t *testing.T) {
	op := shellReadFixture(t, operation.VersionShell, operation.ShellPhaseReadOut)
	op.State = append(slices.Clone(op.State[:len(op.State)-1]), []byte(`,"FutureExecutorAuthority":true}`)...)
	if err := Validate([]operation.Operation{op}); !errors.Is(err, operation.ErrUnsupported) {
		t.Fatalf("unknown shell recovery member accepted: %v", err)
	}
}
