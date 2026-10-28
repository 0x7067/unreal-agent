package workerhost

import (
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
)

func TestScheduledReloadDoesNotCancelRetainedBackgroundOwner(t *testing.T) {
	var calls atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.Copy(io.Discard, r.Body)
		id := calls.Add(1)
		switch id {
		case 1:
			completed(w, id, toolCall(id, "Agent", map[string]any{"name": "retained", "prompt": "work", "lifetime": "background", "purpose": "reload proof", "parent_task": "reload", "owned_resources": []string{"retained-live", "retained-effect", "retained-release"}}))
		case 2:
			output := append(assistant(id, "Candidate report, owned shell remains live."), toolCall(id, "Bash", map[string]any{"command": "printf x >> retained-effect; touch retained-live; while [ ! -f retained-release ]; do sleep .02; done; printf z >> retained-effect"})...)
			completed(w, id, output)
		default:
			completed(w, id, assistant(id, "root idle"))
		}
	}))
	t.Cleanup(server.Close)
	h, records, workspace := realHost(t, server, true)
	t.Cleanup(func() { _ = os.WriteFile(filepath.Join(workspace, "retained-release"), nil, 0600) })
	before, pid := generation(h)
	send(t, h, "start-retained", "start retained work")
	waitFile(t, filepath.Join(workspace, "retained-live"))
	waitResponses(t, records, 2)
	activity, err := h.ActivitySnapshot(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if len(activity.Snapshot.Agents) != 1 {
		t.Fatalf("missing exact retained identity: %+v", activity.Snapshot)
	}
	child := activity.Snapshot.Agents[0]
	reloaded := make(chan error, 1)
	go func() { reloaded <- h.Reload(t.Context(), nil) }()
	waitIdleStop(t, records)
	// A scoped status passes the ordered lane, establishing that the worker is
	// responsive after arming while the retained owner still blocks retirement.
	args, _ := json.Marshal(map[string]any{"action": "status", "handle": child.Handle, "generation": child.Generation})
	payload, _ := json.Marshal(inbox.ControlMessage{Mode: inbox.InvokeTool, Parameters: inbox.ToolInvocation{Scope: h.ControlScope(), Call: llm.ToolCall{Name: "AgentControl", CallID: "retained-status", Arguments: string(args)}}})
	if err := h.SubmitIntervention(t.Context(), inbox.Input{ID: "retained-status", Kind: inbox.InputControl, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	waitInvocation(t, records, "retained-status")
	select {
	case err := <-reloaded:
		t.Fatalf("retained owner retired without intervention: %v", err)
	default:
	}
	if _, next := generation(h); next != pid {
		t.Fatal("retained owner generation replaced")
	}
	// Explicit user cancellation is allowed; reload itself never invents it.
	args, _ = json.Marshal(map[string]any{"action": "stop", "handle": child.Handle, "generation": child.Generation})
	payload, _ = json.Marshal(inbox.ControlMessage{Mode: inbox.InvokeTool, Parameters: inbox.ToolInvocation{Scope: h.ControlScope(), Call: llm.ToolCall{Name: "AgentControl", CallID: "retained-stop", Arguments: string(args)}}})
	if err := h.SubmitIntervention(t.Context(), inbox.Input{ID: "retained-stop", Kind: inbox.InputControl, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-reloaded:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("retained owner drain did not wake coordinator")
	}
	if after, next := generation(h); after != before+1 || next == pid {
		t.Fatal("replacement did not follow owner joins")
	}
	data, err := os.ReadFile(filepath.Join(workspace, "retained-effect"))
	if err != nil || string(data) != "x" {
		t.Fatalf("owner canceled/replayed incorrectly: %q %v", data, err)
	}
}
