package coordinator

import (
	"context"
	"encoding/json/jsontext"
	"errors"
	"fmt"
	"reflect"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/contextbuilder"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

// These are bounded real-code projection checks, not a Go refinement proof.
// Each restart reopens the actual JSONL store with no shared write cache. The
// oracle tracks canonical checkpoints independently of history and Resume.
func TestDispatchExhaustiveRecovery(t *testing.T) {
	statuses := []operation.Status{operation.StatusReady, operation.StatusAwaiting, operation.StatusCompleted, operation.StatusFailed, operation.StatusCanceled}
	cases := 0
	for _, left := range statuses {
		for _, right := range statuses {
			for _, order := range [][2]int{{0, 1}, {1, 0}} {
				// Every pair of restart cuts around the two saves and reconciliation.
				// Equal cuts mean two consecutive restarts without intervening work.
				for first := 0; first <= 3; first++ {
					for second := first; second <= 3; second++ {
						name := fmt.Sprintf("%s-%s/order%d/cuts%d%d", left, right, order[0], first, second)
						t.Run(name, func(t *testing.T) {
							directory, store, registry, latest := dispatchSeed(t)
							settled := false
							restart := func() *coordinator {
								reopened, err := localfile.New(directory)
								if err != nil {
									t.Fatal(err)
								}
								store = reopened
								restored, err := store.Resume(t.Context(), "session-1")
								if err != nil {
									t.Fatal(err)
								}
								want := map[operation.ID]operation.Operation{}
								for _, value := range latest {
									if !settled || !dispatchOracleTerminal(value.Status) {
										want[value.ID] = value
									}
								}
								got := map[operation.ID]operation.Operation{}
								for _, value := range restored.Operations {
									got[value.ID] = value
								}
								if !reflect.DeepEqual(got, want) {
									t.Fatalf("recovery lost or changed checkpoint: got %#v want %#v", got, want)
								}
								manager := newFakeOperationManager()
								// Check at Add, not merely after dispatch completes.
								manager.addError = func(value operation.Operation) error {
									var canonical operation.Operation
									exists := false
									for _, candidate := range latest {
										if candidate.ID == value.ID {
											canonical, exists = candidate, true
										}
									}
									if !exists || dispatchOracleTerminal(canonical.Status) || !reflect.DeepEqual(value, canonical) {
										t.Fatalf("unsafe Add after crash: got %#v canonical %#v", value, canonical)
									}
									return nil
								}
								current := New(Dependencies{SessionID: "session-1", Sessions: store, Restored: restored, Tools: registry, Operations: manager, ContextBuilder: contextbuilder.NewBuilder()}).(*coordinator)
								if err := current.restore(t.Context()); err != nil {
									t.Fatal(err)
								}
								// The same ordered startup calls as Run, without the unrelated LLM loop.
								if _, err := current.scheduleToolCalls(t.Context()); err != nil {
									t.Fatal(err)
								}
								if err := current.dispatchOperationsToManager(); err != nil {
									t.Fatal(err)
								}
								assertDispatchIDs(t, manager.adds, latest)
								for _, value := range latest {
									if !reflect.DeepEqual(current.state.operations[value.ID], value) {
										t.Fatalf("restore did not overlay latest checkpoint for %s", value.ID)
									}
								}
								return current
							}
							current := restart()
							for cut := 0; cut <= 3; cut++ {
								for _, crashCut := range []int{first, second} {
									if cut == crashCut {
										current = restart()
									}
								}
								if cut < 2 {
									index := order[cut]
									value := latest[index]
									value.Status = []operation.Status{left, right}[index]
									// Different state detects stale overlays even when status stays Ready.
									value.State = jsontext.Value(fmt.Sprintf(`{"Value":%d}`, index+10))
									if err := current.handleOperationUpdate(t.Context(), value); err != nil {
										t.Fatal(err)
									}
									latest[index] = value
								} else if cut == 2 {
									completed, err := current.reconcileToolCalls(t.Context())
									if err != nil {
										t.Fatal(err)
									}
									settled = dispatchOracleTerminal(left) && dispatchOracleTerminal(right)
									expected := 0
									if settled {
										expected = 1
									}
									if len(completed) != expected {
										t.Fatalf("completed calls = %d want %d", len(completed), expected)
									}
									again, err := current.reconcileToolCalls(t.Context())
									if err != nil || len(again) != 0 {
										t.Fatalf("repeated reconciliation = %#v, %v", again, err)
									}
								}
							}
							// Always inspect final projection, including settled terminal removal.
							restart()
						})
						cases++
					}
				}
			}
		}
	}
	t.Logf("checked %d histories: 25 checkpoint pairs x 2 save orders x 10 two-crash cut pairs", cases)
}

// Exact multiplicity matters: Add(op-0) twice must not hide a lost op-1.
func assertDispatchIDs(t *testing.T, adds, canonical []operation.Operation) {
	t.Helper()
	got, want := map[operation.ID]int{}, map[operation.ID]int{}
	for _, value := range adds {
		got[value.ID]++
	}
	for _, value := range canonical {
		if !dispatchOracleTerminal(value.Status) {
			want[value.ID]++
		}
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("dispatch ID counts = %v want %v", got, want)
	}
}

func dispatchOracleTerminal(status operation.Status) bool {
	// Do not call production operationIsTerminal: mutating its gate must fail.
	return status == operation.StatusCompleted || status == operation.StatusFailed || status == operation.StatusCanceled
}

func dispatchSeed(t *testing.T) (string, *localfile.Store, tool.Registry, []operation.Operation) {
	t.Helper()
	directory := t.TempDir()
	store, err := localfile.New(directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = store.Create(t.Context(), "session-1"); err != nil {
		t.Fatal(err)
	}
	if err = store.AppendTurn(t.Context(), "session-1", session.Turn{ID: "turn-1", Type: session.TurnRegular}); err != nil {
		t.Fatal(err)
	}
	response := sessionstore.ModelResponse{TurnID: "turn-1", Response: llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "call-0", Name: tool.ViewImageName, Arguments: `{}`}}}}}
	if err = store.AppendModelResponse(t.Context(), "session-1", response); err != nil {
		t.Fatal(err)
	}
	latest := []operation.Operation{}
	for index := range 2 {
		latest = append(latest, operation.Operation{ID: operation.ID(fmt.Sprintf("op-%d", index)), Type: operation.TypeValue, Version: operation.VersionValue, Status: operation.StatusReady, State: jsontext.Value(`{"Value":0}`)})
	}
	status := sessionstore.ToolCallStatus{TurnID: "turn-1", CallID: "call-0", Status: tool.CallStatus{WaitingFor: []operation.ID{latest[0].ID, latest[1].ID}}, Operations: latest}
	if err = store.AppendToolCallStatus(t.Context(), "session-1", status); err != nil {
		t.Fatal(err)
	}
	registry := tool.NewRegistry(tool.StaticTranslators{ViewImage: operationStatusTranslator{}}, tool.ViewImageName)
	return directory, store, registry, latest
}

var errDispatchCommit = errors.New("injected dispatch persistence failure")

// Exercise both ambiguous error outcomes: old prefix and complete new record.
// In either case the live call must stop before Add. Fresh recovery may use
// whatever complete records survived. The wrapper does not fake replay logic.
type dispatchCommitFault struct {
	sessionstore.Store
	site    string
	visible bool
}

func (s dispatchCommitFault) AppendModelResponse(ctx context.Context, id session.ID, value sessionstore.ModelResponse) error {
	if s.site != "response" {
		return s.Store.AppendModelResponse(ctx, id, value)
	}
	if s.visible {
		if err := s.Store.AppendModelResponse(ctx, id, value); err != nil {
			return err
		}
	}
	return errDispatchCommit
}
func (s dispatchCommitFault) AppendToolCallStatus(ctx context.Context, id session.ID, value sessionstore.ToolCallStatus) error {
	if s.site != "status" {
		return s.Store.AppendToolCallStatus(ctx, id, value)
	}
	if s.visible {
		if err := s.Store.AppendToolCallStatus(ctx, id, value); err != nil {
			return err
		}
	}
	return errDispatchCommit
}

func TestDispatchCommitBeforeAdd(t *testing.T) {
	for _, site := range []string{"none", "response", "status"} {
		for _, visible := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/visible%v", site, visible), func(t *testing.T) {
				directory := t.TempDir()
				store, err := localfile.New(directory)
				if err != nil {
					t.Fatal(err)
				}
				if _, err = store.Create(t.Context(), "session-1"); err != nil {
					t.Fatal(err)
				}
				if err = store.AppendTurn(t.Context(), "session-1", session.Turn{ID: "turn-1", Type: session.TurnRegular}); err != nil {
					t.Fatal(err)
				}
				spec, err := operation.NewValueSpec(jsontext.Value(`0`))
				if err != nil {
					t.Fatal(err)
				}
				translator := &submittingTranslator{specs: []operation.Spec{spec, spec}}
				registry := tool.NewRegistry(tool.StaticTranslators{ViewImage: translator}, tool.ViewImageName)
				manager := newFakeOperationManager()
				manager.addError = func(value operation.Operation) error {
					// Independently reopen at each Add; compare actual committed snapshots.
					disk, err := localfile.New(directory)
					if err != nil {
						t.Fatal(err)
					}
					restored, err := disk.Resume(t.Context(), "session-1")
					if err != nil {
						t.Fatal(err)
					}
					for _, canonical := range restored.Operations {
						if reflect.DeepEqual(canonical, value) {
							return nil
						}
					}
					t.Fatalf("Add before matching committed initialization: %#v", value)
					return nil
				}
				current := New(Dependencies{SessionID: "session-1", Sessions: dispatchCommitFault{Store: store, site: site, visible: visible}, Tools: registry, Operations: manager, ContextBuilder: contextbuilder.NewBuilder()}).(*coordinator)
				if err = current.restore(t.Context()); err != nil {
					t.Fatal(err)
				}
				response := llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "call-0", Name: tool.ViewImageName, Arguments: `{}`}}}}
				err = current.processModelResponse(t.Context(), modelResponseResult{turnID: "turn-1", response: response})
				if site != "none" {
					if !errors.Is(err, errDispatchCommit) || len(manager.adds) != 0 {
						t.Fatalf("failed %s commit allowed dispatch: err=%v adds=%d", site, err, len(manager.adds))
					}
				} else if err != nil || len(manager.adds) != 2 {
					t.Fatalf("successful commit: err=%v adds=%d", err, len(manager.adds))
				}
				if site == "none" {
					committed, err := store.Resume(t.Context(), "session-1")
					if err != nil {
						t.Fatal(err)
					}
					assertDispatchIDs(t, manager.adds, committed.Operations)
				}
				// Discard all local state. Retry only from a freshly read durable prefix.
				disk, err := localfile.New(directory)
				if err != nil {
					t.Fatal(err)
				}
				restored, err := disk.Resume(t.Context(), "session-1")
				if err != nil {
					t.Fatal(err)
				}
				retryManager := newFakeOperationManager()
				retryManager.addError = manager.addError
				retry := New(Dependencies{SessionID: "session-1", Sessions: disk, Restored: restored, Tools: registry, Operations: retryManager, ContextBuilder: contextbuilder.NewBuilder()}).(*coordinator)
				if err = retry.restore(t.Context()); err != nil {
					t.Fatal(err)
				}
				if _, err = retry.scheduleToolCalls(t.Context()); err != nil {
					t.Fatal(err)
				}
				if err = retry.dispatchOperationsToManager(); err != nil {
					t.Fatal(err)
				}
				expected := 2
				if site == "response" && !visible {
					expected = 0
				}
				if len(retryManager.adds) != expected {
					t.Fatalf("recovery Add count=%d want %d", len(retryManager.adds), expected)
				}
				committed, err := disk.Resume(t.Context(), "session-1")
				if err != nil {
					t.Fatal(err)
				}
				assertDispatchIDs(t, retryManager.adds, committed.Operations)
			})
		}
	}
}
