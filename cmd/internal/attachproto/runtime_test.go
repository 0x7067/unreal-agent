package attachproto

import (
	"bytes"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

func TestRuntimeRequestValidation(t *testing.T) {
	valid := []RuntimeRequest{{Version: 1, Action: "check"}, {Version: 1, Action: "replace", ExpectedGeneration: 1}, {Version: 1, Action: "cancel", TargetID: strings.Repeat("a", 128)}}
	for _, r := range valid {
		if err := ValidateRuntimeRequest(r); err != nil {
			t.Fatalf("valid %+v: %v", r, err)
		}
	}
	bad := []RuntimeRequest{{Action: "check"}, {Version: 2, Action: "check"}, {Version: 1, Action: "unknown"}, {Version: 1, Action: "CHECK"}, {Version: 1, Action: " check"}, {Version: 1, Action: "check", ExpectedGeneration: 1}, {Version: 1, Action: "check", TargetID: "id"}, {Version: 1, Action: "replace"}, {Version: 1, Action: "replace", ExpectedGeneration: 1, TargetID: "id"}, {Version: 1, Action: "cancel"}, {Version: 1, Action: "cancel", TargetID: " id"}, {Version: 1, Action: "cancel", TargetID: "id "}, {Version: 1, Action: "cancel", TargetID: "a\x00b"}, {Version: 1, Action: "cancel", TargetID: strings.Repeat("a", 129)}, {Version: 1, Action: "cancel", TargetID: "id", ExpectedGeneration: 1}}
	for _, r := range bad {
		if ValidateRuntimeRequest(r) == nil {
			t.Errorf("accepted invalid request %+v", r)
		}
	}
}

func TestRuntimeStatusValidation(t *testing.T) {
	base := RuntimeStatus{Version: 1, Generation: 1, RunningBuild: "running", State: "ready"}
	for _, state := range []string{"checking", "replacing", "ready", "blocked", "completed", "canceled", "cancel_requested", "stopped", "failed"} {
		s := base
		s.State = state
		if err := ValidateRuntimeStatus(s); err != nil {
			t.Errorf("%s: %v", state, err)
		}
		if s.Terminal() != (state != "checking" && state != "replacing") {
			t.Errorf("terminal %s", state)
		}
	}
	for name, mutate := range map[string]func(*RuntimeStatus){"version": func(s *RuntimeStatus) { s.Version = 2 }, "state": func(s *RuntimeStatus) { s.State = "unknown" }, "generation": func(s *RuntimeStatus) { s.Generation = 0 }, "build": func(s *RuntimeStatus) { s.RunningBuild = "" }, "success error": func(s *RuntimeStatus) { s.Error = "failure" }, "running size": func(s *RuntimeStatus) { s.RunningBuild = strings.Repeat("a", 513) }, "candidate size": func(s *RuntimeStatus) { s.CandidateBuild = strings.Repeat("a", 513) }, "message size": func(s *RuntimeStatus) { s.Message = strings.Repeat("a", 1025) }, "error size": func(s *RuntimeStatus) { s.State = "failed"; s.Error = strings.Repeat("a", 1025) }, "control": func(s *RuntimeStatus) { s.Message = "bad\x1btext" }} {
		t.Run(name, func(t *testing.T) {
			s := base
			mutate(&s)
			if ValidateRuntimeStatus(s) == nil {
				t.Fatal("accepted invalid status")
			}
		})
	}
	base.RunningBuild = strings.Repeat("a", 512)
	base.CandidateBuild = strings.Repeat("b", 512)
	base.Message = strings.Repeat("m", 1024)
	if err := ValidateRuntimeStatus(base); err != nil {
		t.Fatal("exact limits", err)
	}
	base.State = "failed"
	base.Error = strings.Repeat("e", 1024)
	if err := ValidateRuntimeStatus(base); err != nil {
		t.Fatal("exact error limit", err)
	}
}

func TestRuntimeStrictCodecAndLegacyOmission(t *testing.T) {
	for _, frame := range []string{`{"Type":"runtime","Runtime":{"Version":1,"Action":"check","Unknown":true}}`, `{"Type":"runtime","Runtime":{"Version":1,"Action":"check","ExpectedGeneration":"1"}}`} {
		if _, err := ReadRequest(Scanner(strings.NewReader(frame + "\n"))); err == nil {
			t.Fatalf("accepted request %s", frame)
		}
	}
	for _, frame := range []string{`{"Type":"runtime","Runtime":{"Version":1,"State":"ready","Unknown":true}}`, `{"Type":"runtime","Runtime":{"Version":1,"State":"ready","Generation":-1}}`} {
		if _, err := ReadEvent(Scanner(strings.NewReader(frame + "\n"))); err == nil {
			t.Fatalf("accepted event %s", frame)
		}
	}
	for _, value := range []any{Request{Type: "attach", Protocol: Version}, Event{Type: "hello", Protocol: Version}, Event{Type: "status", Status: &Status{Generation: 1}}} {
		b, err := Encode(value)
		if err != nil {
			t.Fatal(err)
		}
		if bytes.Contains(b, []byte(`"Runtime"`)) {
			t.Fatalf("legacy frame includes Runtime: %s", b)
		}
	}
	request := Request{Type: "runtime", Runtime: &RuntimeRequest{Version: 1, Action: "replace", ExpectedGeneration: 7}}
	b, err := Encode(request)
	if err != nil {
		t.Fatal(err)
	}
	decoded, err := ReadRequest(Scanner(bytes.NewReader(b)))
	if err != nil || decoded.Runtime == nil || *decoded.Runtime != *request.Runtime {
		t.Fatalf("request roundtrip %+v %v", decoded, err)
	}
}

func runtimeLiveView(t *testing.T) (*View, Event) {
	t.Helper()
	e := Event{Protocol: Version, SessionID: "s", Connection: "c", Incarnation: "host", Revision: 4}
	v := NewView(e)
	header := e
	header.Type = "snapshot"
	header.Status = &Status{Generation: 1}
	header.Model = &llm.Model{}
	if err := v.Accept(header); err != nil {
		t.Fatal(err)
	}
	end := e
	end.Type = "snapshot_end"
	if err := v.Accept(end); err != nil {
		t.Fatal(err)
	}
	e.Type = "runtime"
	e.ID = "request"
	e.Runtime = &RuntimeStatus{Version: 1, Generation: 1, RunningBuild: "build", State: "ready"}
	return v, e
}

func TestRuntimeViewRepliesDoNotAdvanceCursor(t *testing.T) {
	v, e := runtimeLiveView(t)
	for range 3 {
		if err := v.Accept(e); err != nil {
			t.Fatal(err)
		}
		if v.revision != 4 || v.head != 0 || v.sequence != 0 {
			t.Fatal("runtime advanced cursor")
		}
	}
	next := e
	next.Type = "status"
	next.Runtime = nil
	next.Revision = 5
	next.Head = 0
	next.Status = &Status{Generation: 1}
	if err := v.Accept(next); err != nil {
		t.Fatal("runtime disrupted contiguous status", err)
	}
}

func TestRuntimeViewRejectsWrongBindingStateAndPayload(t *testing.T) {
	for name, mutate := range map[string]func(*Event){"protocol": func(e *Event) { e.Protocol++ }, "session": func(e *Event) { e.SessionID = "other" }, "connection": func(e *Event) { e.Connection = "other" }, "incarnation": func(e *Event) { e.Incarnation = "other" }, "revision": func(e *Event) { e.Revision++ }, "head": func(e *Event) { e.Head++ }, "item": func(e *Event) { e.Item = &sessionstore.Item{} }, "base": func(e *Event) { e.Base = &Cursor{} }, "missing": func(e *Event) { e.Runtime = nil }, "old event runtime": func(e *Event) { e.Type = "accepted" }, "bad status": func(e *Event) { e.Runtime.State = "unknown" }, "bad version": func(e *Event) { e.Runtime.Version = 2 }} {
		t.Run(name, func(t *testing.T) {
			v, e := runtimeLiveView(t)
			mutate(&e)
			if v.Accept(e) == nil {
				t.Fatal("accepted invalid runtime event")
			}
			if v.revision != 4 || v.head != 0 {
				t.Fatal("rejected event changed cursor")
			}
		})
	}
	_, e := runtimeLiveView(t)
	if NewView(e).Accept(e) == nil {
		t.Fatal("runtime accepted before complete snapshot")
	}
}

func TestRuntimeNaturalDrainStrictVersionAndNonterminalState(t *testing.T) {
	r := RuntimeRequest{Version: 1, Action: "drain", ExpectedGeneration: 1}
	if err := ValidateRuntimeRequest(r); err != nil {
		t.Fatal(err)
	}
	r.ExpectedGeneration = 0
	if ValidateRuntimeRequest(r) == nil {
		t.Fatal("unfenced queued replacement accepted")
	}
	s := RuntimeStatus{Version: 1, Generation: 1, RunningBuild: "trusted", State: "queued"}
	if err := ValidateRuntimeStatus(s); err != nil || s.Terminal() {
		t.Fatalf("queued is terminal or unsupported: %v", err)
	}
}
