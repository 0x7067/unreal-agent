package resident

import (
	"context"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachclient"
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

func residentIntervention(t *testing.T, scope, arguments string) inbox.Input {
	t.Helper()
	return residentControl(t, "stable-intervention", inbox.InvokeTool, inbox.ToolInvocation{Scope: scope, Call: llm.ToolCall{Name: "AgentControl", CallID: "list-call", Arguments: arguments}})
}

func interventionResult(t *testing.T, state string, id inbox.ID) []sessionstore.Item {
	t.Helper()
	store, err := localfile.New(state)
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for {
		page, err := store.Items(t.Context(), "lifecycle", 0, 100)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range page.Items {
			in, ok := item.Data.(inbox.Input)
			if !ok || in.Kind != inbox.InputControl {
				continue
			}
			control, err := in.DecodeControlMessage()
			if err == nil && control.Mode == inbox.ToolInvocationFinished {
				result := control.Parameters.(inbox.ToolInvocationResult)
				if result.InputID == id {
					// This minimal worker fixture does not enable optional helpers.
					// Admission must still be durable and its terminal failure
					// explicit; an accepted ack never promises tool execution.
					if result.Status != "failed" || result.Error != "structural tool is not enabled for this session" {
						t.Fatalf("unexpected terminal intervention outcome: %+v", result)
					}
					return page.Items
				}
			}
		}
		if time.Now().After(deadline) {
			t.Fatal("intervention execution not observed")
		}
		time.Sleep(5 * time.Millisecond)
	}
}

func interventionRawAttach(t *testing.T, ctx context.Context, runtime, state string) (*net.UnixConn, attachproto.Event) {
	t.Helper()
	deadline := time.Now().Add(3 * time.Second)
	for {
		conn, err := attachproto.Dial(ctx, runtime, state, "lifecycle")
		if err != nil {
			t.Fatal(err)
		}
		_ = conn.SetDeadline(deadline)
		if err := attachproto.Write(conn, attachproto.Request{Type: "attach", ID: "attach", Protocol: attachproto.Version, SessionID: "lifecycle"}); err != nil {
			conn.Close()
			t.Fatal(err)
		}
		scanner := attachproto.Scanner(conn)
		hello, err := attachproto.ReadEvent(scanner)
		if err == nil && hello.Type == "error" && hello.Code == "busy" && time.Now().Before(deadline) {
			conn.Close()
			time.Sleep(5 * time.Millisecond)
			continue
		}
		if err != nil || hello.Type != "hello" {
			conn.Close()
			t.Fatalf("attach: %+v %v", hello, err)
		}
		for {
			e, err := attachproto.ReadEvent(scanner)
			if err != nil {
				conn.Close()
				t.Fatal(err)
			}
			if e.Type == "snapshot_end" {
				break
			}
		}
		return conn, hello
	}
}

func TestInterventionPublicProtocolStableIdentityAndGenerationFence(t *testing.T) {
	var calls atomic.Int64
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		http.Error(w, "interventions must not call provider", 500)
	}))
	t.Cleanup(provider.Close)
	_, runtime, state, _, _ := residentLifecycleFixture(t, provider, true)
	ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
	defer cancel()
	c := lifecycleDial(t, ctx, runtime, state)
	reader := faultReader(t, c)
	t.Cleanup(func() { c.Close(); <-reader })
	var advertised bool
	for _, cap := range c.Snapshot().Hello.Capabilities {
		if cap == "runtime_replacement_v1" || cap == "runtime_drain_v1" || cap == "host_retire_v1" {
			t.Fatalf("host advertises changed runtime semantics with old capability %q", cap)
		}
		advertised = advertised || cap == attachproto.CapabilityIntervention
	}
	if !advertised {
		t.Fatal("host did not advertise intervention capability")
	}
	original := residentIntervention(t, c.ControlScope(), `{"action":"list"}`)
	if err := c.SubmitIntervention(ctx, original); err != nil {
		t.Fatal(err)
	}
	interventionResult(t, state, original.ID)
	if err := c.SubmitIntervention(ctx, original); err != nil {
		t.Fatalf("stable retry: %v", err)
	}
	changed := residentIntervention(t, c.ControlScope(), `{"action":"status","handle":"missing","generation":1}`)
	var conflict *attachclient.ReplyError
	if err := c.SubmitIntervention(ctx, changed); !errors.As(err, &conflict) || conflict.Code != "conflict" || conflict.InputID != original.ID {
		t.Fatalf("changed stable payload not rejected with correlated conflict: %v", err)
	}
	items := interventionResult(t, state, original.ID)
	var count int
	for _, item := range items {
		if in, ok := item.Data.(inbox.Input); ok && in.ID == original.ID {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("stable input persisted %d times", count)
	}
	generation := c.Snapshot().Header.Status.Generation
	out, err := c.RuntimeControl(ctx, attachproto.RuntimeRequest{Version: 1, Action: "replace", ExpectedGeneration: generation})
	if err != nil || out.Generation != generation+1 || out.State != "completed" {
		t.Fatalf("replacement: %+v %v", out, err)
	}
	if c.SubmitIntervention(ctx, original) == nil {
		t.Fatal("SDK admitted old generation")
	}
	c.Close()
	// The SDK check is not the trust boundary. Bypass it with an authenticated
	// public request carrying the old scope: the server must reject before host
	// admission even though this stable ID had succeeded in the old generation.
	conn, hello := interventionRawAttach(t, ctx, runtime, state)
	defer conn.Close()
	if err := attachproto.Write(conn, attachproto.Request{Type: "intervention", ID: "stale", Protocol: attachproto.Version, SessionID: "lifecycle", Connection: hello.Connection, Input: &original}); err != nil {
		t.Fatal(err)
	}
	_, err = attachproto.ReadEvent(attachproto.Scanner(conn))
	if !errors.Is(err, io.EOF) {
		t.Fatalf("stale public input not disconnected: %v", err)
	}
	if calls.Load() != 0 {
		t.Fatalf("intervention invoked model %d times", calls.Load())
	}
}
