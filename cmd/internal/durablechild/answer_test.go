package durablechild

import (
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

type answerHistory struct {
	t    *testing.T
	c    *Connection
	s    *localfile.Store
	last session.TurnID
	n    int
}

func newAnswerHistory(t *testing.T) *answerHistory {
	t.Helper()
	cfg, req := fixture(t)
	r, err := plan(cfg, req)
	if err != nil {
		t.Fatal(err)
	}
	s, err := localfile.New(r.Link.Directory)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Create(t.Context(), r.Link.ChildID); err != nil {
		t.Fatal(err)
	}
	return &answerHistory{t: t, c: &Connection{link: r.Link, request: req}, s: s}
}
func (h *answerHistory) input(id, prompt string) {
	h.t.Helper()
	b, err := json.Marshal(prompt)
	if err != nil {
		h.t.Fatal(err)
	}
	if err = h.s.AppendInput(h.t.Context(), h.c.link.ChildID, inbox.Input{ID: inbox.ID(id), Kind: inbox.InputExternal, Payload: b}); err != nil {
		h.t.Fatal(err)
	}
}
func (h *answerHistory) initial() { h.input(h.c.link.DeliveryID, h.c.request.Prompt) }
func (h *answerHistory) turn(kind session.TurnType) session.TurnID {
	h.t.Helper()
	h.n++
	id := session.TurnID(fmt.Sprintf("turn-%d", h.n))
	if err := h.s.AppendTurn(h.t.Context(), h.c.link.ChildID, session.Turn{ID: id, PreviousTurnID: h.last, Type: kind}); err != nil {
		h.t.Fatal(err)
	}
	h.last = id
	return id
}
func (h *answerHistory) response(kind session.TurnType, r llm.Response) session.TurnID {
	h.t.Helper()
	id := h.turn(kind)
	if err := h.s.AppendModelResponse(h.t.Context(), h.c.link.ChildID, sessionstore.ModelResponse{TurnID: id, Response: r}); err != nil {
		h.t.Fatal(err)
	}
	return id
}
func finalText(text string) llm.Response {
	return llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: text}}}}
}
func (h *answerHistory) absent() {
	h.t.Helper()
	a, found, err := h.c.Answer(h.t.Context())
	if err != nil || found || a != (Answer{}) {
		h.t.Fatalf("unexpected authority: %+v found=%v err=%v", a, found, err)
	}
}

func TestAnswerAuthorityFirstRegularFinalPinned(t *testing.T) {
	h := newAnswerHistory(t)
	h.response(session.TurnRegular, finalText("before initial"))
	h.absent()
	h.initial()
	h.absent()
	cases := []struct {
		name string
		kind session.TurnType
		r    llm.Response
	}{
		{"partial", session.TurnRegular, func() llm.Response { r := finalText("partial"); r.Stop = llm.StopReason("incomplete"); return r }()},
		{"failed", session.TurnRegular, func() llm.Response { r := finalText("failed"); r.Failure = &llm.Failure{Message: "failed"}; return r }()},
		{"tool", session.TurnRegular, func() llm.Response {
			r := finalText("tool preamble")
			r.Output = append(r.Output, llm.Item{Type: llm.ItemToolCall, Data: llm.ToolCall{CallID: "call", Name: "tool", Arguments: "{}"}})
			return r
		}()},
		{"compaction", session.TurnCompaction, finalText("summary")},
		{"empty", session.TurnRegular, finalText("")},
		{"nonassistant", session.TurnRegular, llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleUser, Text: "not an answer"}}}}},
	}
	for _, tc := range cases {
		t.Log(tc.name)
		h.response(tc.kind, tc.r)
		h.absent()
	}
	firstID := h.response(session.TurnRegular, finalText("authoritative"))
	first, found, err := h.c.Answer(t.Context())
	if err != nil || !found || first.Text != "authoritative" || first.TurnID != firstID || first.Sequence == 0 {
		t.Fatalf("first: %+v found=%v err=%v", first, found, err)
	}
	h.response(session.TurnRegular, finalText("later"))
	later, found, err := h.c.Answer(t.Context())
	if err != nil || !found || later != first {
		t.Fatalf("answer changed: %+v found=%v err=%v", later, found, err)
	}
}

func TestAnswerAuthorityExternalInput(t *testing.T) {
	for _, tc := range []struct {
		name, id, prompt string
		conflict         bool
	}{
		{"foreign", "foreign", "other", true}, {"same ID changed payload", "", "changed", true}, {"same ID same payload", "", "hello", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			h := newAnswerHistory(t)
			h.initial()
			id := tc.id
			if id == "" {
				id = h.c.link.DeliveryID
			}
			h.input(id, tc.prompt)
			h.response(session.TurnRegular, finalText("answer"))
			_, found, err := h.c.Answer(t.Context())
			if tc.conflict {
				if found || !errors.Is(err, ErrConflict) {
					t.Fatalf("found=%v err=%v", found, err)
				}
			} else if err != nil || !found {
				t.Fatalf("found=%v err=%v", found, err)
			}
		})
	}
}
func TestAnswerAuthorityTextLimit(t *testing.T) {
	for _, size := range []int{256 << 10, (256 << 10) + 1} {
		t.Run(fmt.Sprint(size), func(t *testing.T) {
			h := newAnswerHistory(t)
			h.initial()
			h.response(session.TurnRegular, finalText(strings.Repeat("x", size)))
			a, found, err := h.c.Answer(t.Context())
			if size == 256<<10 {
				if err != nil || !found || len(a.Text) != size {
					t.Fatalf("found=%v size=%d err=%v", found, len(a.Text), err)
				}
			} else if err == nil || found {
				t.Fatalf("oversized accepted: found=%v err=%v", found, err)
			}
		})
	}
}
func TestAnswerAuthorityPageBudgetRefuses(t *testing.T) {
	h := newAnswerHistory(t)
	h.initial()
	for i := 0; i < 4096; i++ {
		h.turn(session.TurnRegular)
	}
	h.response(session.TurnRegular, finalText("beyond budget"))
	_, found, err := h.c.Answer(t.Context())
	if found || err == nil || !strings.Contains(err.Error(), "budget") {
		t.Fatalf("scan did not explicitly refuse: found=%v err=%v", found, err)
	}
}

func TestAnswerRejectsForeignInputBeforeInitial(t *testing.T) {
	h := newAnswerHistory(t)
	h.input("unlinked", "other work")
	h.initial()
	h.response(session.TurnRegular, llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: "untrusted answer"}}}})
	if _, found, err := h.c.Answer(t.Context()); found || !errors.Is(err, ErrConflict) {
		t.Fatalf("foreign conversation attributed: %v %v", found, err)
	}
}
