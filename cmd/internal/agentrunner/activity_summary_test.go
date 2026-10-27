package agentrunner

import (
	"context"
	"encoding/json/jsontext"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/unreallabsai/unreal-agent/harness/executionbudget"
	"github.com/unreallabsai/unreal-agent/harness/operation"
)

type summaryRemoteHandler struct {
	updates chan operation.Operation
	added   chan struct{}
}

func (*summaryRemoteHandler) RemoteJobPlanType() operation.RemoteJobPlanType       { return "summary-test" }
func (*summaryRemoteHandler) RemoteJobPlanVersion() operation.RemoteJobPlanVersion { return 1 }
func (h *summaryRemoteHandler) RemoteJobUpdates() <-chan operation.Operation       { return h.updates }
func (h *summaryRemoteHandler) AddRemoteJob(operation.Operation) error             { close(h.added); return nil }
func (*summaryRemoteHandler) CancelRemoteJob(operation.ID, string) error           { return nil }

func TestRunnerHeartbeatSummaryUsesOnlyRedactedSnapshots(t *testing.T) {
	ctx, cancel := context.WithCancel(t.Context())
	h := &summaryRemoteHandler{updates: make(chan operation.Operation), added: make(chan struct{})}
	account := executionbudget.NewAccount(executionbudget.Limits{MaxOperations: 2})
	manager := operation.NewLocalOperationManagerWithOptions(ctx, operation.LocalOperationManagerOptions{ResourceAccount: account}, h)
	defer func() {
		cancel()
		close(h.updates)
		for range manager.Updates() {
		}
	}()
	spec, err := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: "summary-test", Version: 1, Data: jsontext.Value(`{"Prompt":"provider-secret","Command":"find / --private-argument"}`)})
	if err != nil {
		t.Fatal(err)
	}
	op := operation.Operation{ID: "live-review", Type: spec.Type, Version: spec.Version, MaxOutputLength: spec.MaxOutputLength, State: spec.State, Status: operation.StatusReady}
	if err := manager.Add(op); err != nil {
		t.Fatal(err)
	}
	<-h.added
	text := runnerActivitySummary(nil, manager)
	if len(text) > 4096 || !utf8.ValidString(text) || !strings.Contains(text, "scope=in_process_operation_envelopes") || !strings.Contains(text, "owned=1") {
		t.Fatalf("summary=%q", text)
	}
	for _, private := range []string{"provider-secret", "find /", "private-argument", "Prompt", "Command"} {
		if strings.Contains(text, private) {
			t.Fatalf("private data %q in summary %q", private, text)
		}
	}
}

func TestSummaryLabelsAreBoundedAndCannotInjectLines(t *testing.T) {
	label := summaryLabel(strings.Repeat("界", 100) + "\nforged status")
	if len(label) > 128 || !utf8.ValidString(label) || strings.ContainsAny(label, "\n\r") {
		t.Fatalf("label=%q", label)
	}
	if summaryLabel("a\nb\x00c") != "a b c" {
		t.Fatal("control characters not normalized")
	}
}
