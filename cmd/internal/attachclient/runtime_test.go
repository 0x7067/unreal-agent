package attachclient

import (
	"context"
	"errors"
	"io"
	"net"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
)

func runtimeGreeting(conn *net.UnixConn) error {
	if _, err := attachproto.ReadRequest(attachproto.Scanner(conn)); err != nil {
		return err
	}
	for _, kind := range []string{"hello", "snapshot", "snapshot_end"} {
		e := attachproto.Event{Type: kind, ID: "attach", Protocol: 1, SessionID: "test", Connection: "connection", Incarnation: "host", Capabilities: []string{"fresh_snapshot", "history_watch", "submit_external", attachproto.CapabilityRuntime}}
		if kind == "snapshot" {
			e.Model = &llm.Model{ID: "fake"}
			e.Status = &attachproto.Status{Generation: 1}
		}
		if err := attachproto.Write(conn, e); err != nil {
			return err
		}
	}
	return nil
}
func TestRuntimeCapabilityBeforeWrite(t *testing.T) {
	c := &Client{}
	if _, err := c.RuntimeControl(t.Context(), attachproto.RuntimeRequest{Version: 1, Action: "check"}); err == nil {
		t.Fatal("missing capability accepted")
	}
}
func TestRuntimeProgressAndConcurrentSubmit(t *testing.T) {
	hostErr := make(chan error, 1)
	root, state, _ := fakeHost(t, func(conn *net.UnixConn) {
		if err := runtimeGreeting(conn); err != nil {
			hostErr <- err
			return
		}
		scanner := attachproto.Scanner(conn)
		r, err := attachproto.ReadRequest(scanner)
		if err != nil {
			hostErr <- err
			return
		}
		if r.Type != "runtime" || r.Runtime == nil || r.Input != nil || r.Cursor != nil || r.ID == "" || r.Connection != "connection" {
			hostErr <- errors.New("invalid runtime envelope")
			return
		}
		emit := func(state string) error {
			generation := uint64(1)
			if state == "completed" {
				generation = 2
			}
			return attachproto.Write(conn, attachproto.Event{Type: "runtime", Protocol: 1, SessionID: "test", Connection: "connection", Incarnation: "host", ID: r.ID, Runtime: &attachproto.RuntimeStatus{Version: 1, State: state, Generation: generation, RunningBuild: "old"}})
		}
		if err := emit("checking"); err != nil {
			hostErr <- err
			return
		}
		s, err := attachproto.ReadRequest(scanner)
		if err != nil {
			hostErr <- err
			return
		}
		if s.Type != "submit" {
			hostErr <- errors.New("submit blocked or malformed")
			return
		}
		err = attachproto.Write(conn, attachproto.Event{Type: "accepted", Protocol: 1, SessionID: "test", Connection: "connection", Incarnation: "host", ID: s.ID, InputID: s.Input.ID})
		if err == nil {
			err = emit("completed")
		}
		hostErr <- err
		_, _ = io.Copy(io.Discard, conn)
	})
	ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
	defer cancel()
	c, err := Dial(ctx, root, state, "test")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	result := make(chan runtimeResult, 1)
	go func() {
		s, e := c.RuntimeControl(ctx, attachproto.RuntimeRequest{Version: 1, Action: "replace", ExpectedGeneration: 1})
		result <- runtimeResult{s, e}
	}()
	e, err := c.Next()
	if err != nil || e.Runtime == nil || e.Runtime.State != "checking" {
		t.Fatalf("progress: %+v %v", e, err)
	}
	submitted := make(chan error, 1)
	go func() {
		submitted <- c.Submit(ctx, inbox.Input{ID: "stable", Kind: inbox.InputExternal, Payload: []byte(`"hello"`)})
	}()
	if e, err = c.Next(); err != nil || e.Type != "accepted" {
		t.Fatalf("submit reply: %+v %v", e, err)
	}
	if err := <-submitted; err != nil {
		t.Fatal(err)
	}
	if e, err = c.Next(); err != nil || e.Runtime == nil || e.Runtime.State != "completed" {
		t.Fatalf("terminal: %+v %v", e, err)
	}
	if r := <-result; r.err != nil || r.status.State != "completed" {
		t.Fatal(r)
	}
	if scope := c.ControlScope(); scope != "host/2" {
		t.Fatalf("runtime scope did not advance: %s", scope)
	}
	if err := <-hostErr; err != nil {
		t.Fatal(err)
	}
}
func TestRuntimeUncertainNoRetry(t *testing.T) {
	for _, mode := range []string{"lost", "unknown", "wrong_connection", "close", "cancel", "blocked"} {
		t.Run(mode, func(t *testing.T) {
			seen := make(chan struct{})
			root, state, _ := fakeHost(t, func(conn *net.UnixConn) {
				if runtimeGreeting(conn) != nil {
					return
				}
				r, err := attachproto.ReadRequest(attachproto.Scanner(conn))
				if err != nil {
					return
				}
				close(seen)
				if mode == "lost" {
					return
				}
				if mode == "close" || mode == "cancel" {
					_, _ = io.Copy(io.Discard, conn)
					return
				}
				e := attachproto.Event{Type: "runtime", Protocol: 1, SessionID: "test", Connection: "connection", Incarnation: "host", ID: r.ID, Runtime: &attachproto.RuntimeStatus{Version: 1, State: "blocked", Message: "busy"}}
				if mode == "unknown" {
					e.ID = "unrelated"
				}
				if mode == "wrong_connection" {
					e.Connection = "stale"
				}
				_ = attachproto.Write(conn, e)
				// Any subsequent bytes would be an unauthorized retry.
				b := make([]byte, 1)
				if n, _ := conn.Read(b); n != 0 {
					t.Error("automatic retry")
				}
			})
			ctx, cancel := context.WithTimeout(t.Context(), 3*time.Second)
			defer cancel()
			c, err := Dial(ctx, root, state, "test")
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			controlCtx, stop := context.WithCancel(ctx)
			defer stop()
			result := make(chan runtimeResult, 1)
			go func() {
				s, e := c.RuntimeControl(controlCtx, attachproto.RuntimeRequest{Version: 1, Action: "check"})
				result <- runtimeResult{s, e}
			}()
			<-seen
			if mode == "close" {
				c.Close()
			}
			if mode == "cancel" {
				stop()
			}
			_, _ = c.Next()
			r := <-result
			if mode == "blocked" {
				var re *RuntimeError
				if !errors.As(r.err, &re) || re.Status.State != "blocked" || r.status.State != "blocked" {
					t.Fatal(r)
				}
			} else if !errors.Is(r.err, ErrRuntimeUncertain) || errors.Is(r.err, ErrUncertain) {
				t.Fatal(r)
			}
		})
	}
}

func TestControlScopeTrustedMonotonicAdvance(t *testing.T) {
	for _, mode := range []string{"status", "wrong_connection", "uncorrelated_runtime"} {
		t.Run(mode, func(t *testing.T) {
			root, state, _ := fakeHost(t, func(conn *net.UnixConn) {
				if lifecycleGreeting(conn, attachproto.CapabilitySessionShutdown) != nil {
					return
				}
				e := attachproto.Event{Type: "status", Protocol: 1, SessionID: "test", Connection: "connection", Incarnation: "host", Revision: 1, Status: &attachproto.Status{Generation: 8}}
				if mode == "wrong_connection" {
					e.Connection = "other"
				}
				if mode == "uncorrelated_runtime" {
					e.Type, e.ID, e.Revision, e.Status = "runtime", "unknown", 0, nil
					e.Runtime = &attachproto.RuntimeStatus{Version: 1, State: "checking", Generation: 100}
				}
				if attachproto.Write(conn, e) != nil {
					return
				}
				if mode == "status" {
					// A later trusted status can describe an older generation; it must not
					// move the lifecycle admission fence backward.
					e.Revision, e.Status.Generation = 2, 6
					if attachproto.Write(conn, e) != nil {
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
			if scope := c.ControlScope(); scope != "host/7" {
				t.Fatal(scope)
			}
			_, err = c.Next()
			if mode != "status" {
				if err == nil || c.ControlScope() != "host/7" {
					t.Fatalf("untrusted scope advance: %s, %v", c.ControlScope(), err)
				}
				return
			}
			if err != nil || c.ControlScope() != "host/8" {
				t.Fatalf("status scope: %s, %v", c.ControlScope(), err)
			}
			if _, err := c.Next(); err != nil {
				t.Fatal(err)
			}
			if scope := c.ControlScope(); scope != "host/8" {
				t.Fatalf("scope regressed: %s", scope)
			}
			if err := c.SubmitLifecycle(ctx, lifecycleInput("shutdown_session")); err == nil {
				t.Fatal("stale lifecycle scope admitted")
			}
		})
	}
}

func TestRuntimeNaturalDrainRequiresSeparateCapabilityBeforeWrite(t *testing.T) {
	c := &Client{capabilities: map[string]bool{attachproto.CapabilityRuntime: true}}
	if _, err := c.RuntimeControl(t.Context(), attachproto.RuntimeRequest{Version: 1, Action: "drain", ExpectedGeneration: 1}); err == nil {
		t.Fatal("old runtime capability silently implied natural drain")
	}
}
