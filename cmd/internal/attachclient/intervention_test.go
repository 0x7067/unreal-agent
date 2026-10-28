package attachclient

import (
	"bytes"
	"context"
	"encoding/json/v2"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
)

func interventionInput(t *testing.T, tool, arguments, scope string) inbox.Input {
	t.Helper()
	payload, err := json.Marshal(inbox.ControlMessage{Mode: inbox.InvokeTool, Parameters: inbox.ToolInvocation{Scope: scope, Call: llm.ToolCall{Name: tool, CallID: "call", Arguments: arguments}}})
	if err != nil {
		t.Fatal(err)
	}
	return inbox.Input{ID: "stable-intervention", Kind: inbox.InputControl, Payload: payload}
}

func TestInterventionRejectsBeforeWrite(t *testing.T) {
	for _, tc := range []struct {
		name  string
		caps  []string
		input inbox.Input
	}{
		{"missing capability", nil, interventionInput(t, "AgentControl", `{"action":"list"}`, "host/7")},
		{"unknown capability", []string{"reload_intervention.v99"}, interventionInput(t, "AgentControl", `{"action":"list"}`, "host/7")},
		{"wrong scope", []string{attachproto.CapabilityIntervention}, interventionInput(t, "AgentControl", `{"action":"list"}`, "host/6")},
		{"launch", []string{attachproto.CapabilityIntervention}, interventionInput(t, "AgentControl", `{"action":"launch","goal":"work"}`, "host/7")},
		{"launch fields", []string{attachproto.CapabilityIntervention}, interventionInput(t, "AgentControl", `{"action":"list","goal":"work"}`, "host/7")},
		{"retry", []string{attachproto.CapabilityIntervention}, interventionInput(t, "TaskGraph", `{"action":"retry","name":"graph"}`, "host/7")},
	} {
		t.Run(tc.name, func(t *testing.T) {
			observed := make(chan error, 1)
			root, state, done := fakeHost(t, func(conn *net.UnixConn) {
				if err := lifecycleGreeting(conn, tc.caps...); err != nil {
					observed <- err
					return
				}
				_, err := attachproto.ReadRequest(attachproto.Scanner(conn))
				observed <- err
			})
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			c, err := Dial(ctx, root, state, "test")
			if err != nil {
				t.Fatal(err)
			}
			if c.SubmitIntervention(ctx, tc.input) == nil {
				t.Error("accepted invalid intervention")
			}
			c.Close()
			if err = <-observed; !errors.Is(err, io.EOF) {
				t.Fatalf("unexpected RPC: %v", err)
			}
			<-done
		})
	}
}

func TestInterventionStableInputLostAckAndCorrelatedConflict(t *testing.T) {
	original := interventionInput(t, "AgentControl", `{"action":"list"}`, "host/7")
	var previousID string
	for _, outcome := range []string{"lost", "accepted", "conflict"} {
		t.Run(outcome, func(t *testing.T) {
			observed := make(chan attachproto.Request, 1)
			root, state, done := fakeHost(t, func(conn *net.UnixConn) {
				if lifecycleGreeting(conn, attachproto.CapabilityIntervention) != nil {
					return
				}
				r, err := attachproto.ReadRequest(attachproto.Scanner(conn))
				if err != nil {
					return
				}
				observed <- r
				if outcome != "lost" {
					e := lifecycleReply(r)
					if outcome == "conflict" {
						e.Type, e.Code, e.Error = "error", "conflict", "stable ID payload changed"
					}
					_ = attachproto.Write(conn, e)
					_, _ = io.Copy(io.Discard, conn)
				}
			})
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			c, err := Dial(ctx, root, state, "test")
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			next := make(chan error, 1)
			go func() { _, err := c.Next(); next <- err }()
			before := original.Payload.Clone()
			err = c.SubmitIntervention(ctx, original)
			switch outcome {
			case "lost":
				if !errors.Is(err, ErrUncertain) {
					t.Fatalf("lost ack: %v", err)
				}
			case "accepted":
				if err != nil {
					t.Fatal(err)
				}
			case "conflict":
				var rejection *ReplyError
				if !errors.As(err, &rejection) || rejection.Code != "conflict" || rejection.InputID != original.ID {
					t.Fatalf("uncorrelated conflict: %v", err)
				}
			}
			r := <-observed
			if r.Type != "intervention" || r.ID == "" || r.ID == previousID || r.ID == string(original.ID) || r.Input == nil || r.Input.ID != original.ID || r.Input.Kind != original.Kind || !bytes.Equal(r.Input.Payload, original.Payload) {
				t.Fatalf("unstable input/correlation: %+v", r)
			}
			if !bytes.Equal(original.Payload, before) {
				t.Fatal("caller payload mutated")
			}
			previousID = r.ID
			c.Close()
			<-next
			<-done
		})
	}
}

func TestInterventionSharesSerializedSubmissionAndNext(t *testing.T) {
	requests := make(chan attachproto.Request, 3)
	root, state, done := fakeHost(t, func(conn *net.UnixConn) {
		if lifecycleGreeting(conn, attachproto.CapabilityIntervention, attachproto.CapabilitySessionShutdown) != nil {
			return
		}
		scanner := attachproto.Scanner(conn)
		for range 3 {
			r, err := attachproto.ReadRequest(scanner)
			if err != nil {
				return
			}
			requests <- r
			if attachproto.Write(conn, lifecycleReply(r)) != nil {
				return
			}
		}
		_, _ = io.Copy(io.Discard, conn)
	})
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	c, err := Dial(ctx, root, state, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	next := make(chan error, 1)
	go func() {
		for range 3 {
			if _, err := c.Next(); err != nil {
				next <- err
				return
			}
		}
		next <- nil
	}()
	in := interventionInput(t, "AgentControl", `{"action":"list"}`, "host/7")
	results := make(chan error, 3)
	go func() { results <- c.SubmitIntervention(ctx, in) }()
	go func() { results <- c.SubmitLifecycle(ctx, lifecycleInput("shutdown_session")) }()
	go func() {
		results <- c.Submit(ctx, inbox.Input{ID: "external", Kind: inbox.InputExternal, Payload: []byte(`"text"`)})
	}()
	for range 3 {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
	if err := <-next; err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for range 3 {
		r := <-requests
		if seen[r.Type] || seen[r.ID] {
			t.Fatalf("duplicate request: %+v", r)
		}
		seen[r.Type], seen[r.ID] = true, true
	}
	if c.Submit(ctx, in) == nil {
		t.Fatal("ordinary submit admitted intervention")
	}
	// Revalidate under the serialized submit path, so a queued old-generation
	// intervention cannot write after Next learns a newer trusted generation.
	c.mu.Lock()
	c.advanceControlScopeLocked(8)
	c.mu.Unlock()
	if c.submit(ctx, "intervention", in) == nil {
		t.Fatal("serialized admission accepted stale generation")
	}
	c.Close()
	<-done
}

func TestRuntimeV1CapabilitiesRejectBeforeWrite(t *testing.T) {
	for _, action := range []string{"check", "replace", "drain", "retire-host"} {
		t.Run(action, func(t *testing.T) {
			observed := make(chan error, 1)
			root, state, done := fakeHost(t, func(conn *net.UnixConn) {
				if err := lifecycleGreeting(conn, "runtime_replacement_v1", "runtime_drain_v1", "host_retire_v1"); err != nil {
					observed <- err
					return
				}
				_, err := attachproto.ReadRequest(attachproto.Scanner(conn))
				observed <- err
			})
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			c, err := Dial(ctx, root, state, "test")
			if err != nil {
				t.Fatal(err)
			}
			request := attachproto.RuntimeRequest{Version: 1, Action: action}
			if action != "check" {
				request.ExpectedGeneration = 7
			}
			if _, err := c.RuntimeControl(ctx, request); err == nil {
				t.Error("old capability authorized changed scheduled semantics")
			}
			c.Close()
			if err := <-observed; !errors.Is(err, io.EOF) {
				t.Fatalf("unexpected RPC to old host: %v", err)
			}
			<-done
		})
	}
}
