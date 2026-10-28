package workerhost

import (
	"encoding/json/v2"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

// These tests use the actual Host/runtime/provider. All state, HTTP endpoints,
// processes and joins belong to the fixture. No producer claims an internal
// after-commit/before-adoption interception: the second cut follows an observed
// committed summary and the next live request; replay loses all live adoption.
func m24Seed(t *testing.T, dir string, automatic bool) {
	t.Helper()
	s, err := localfile.New(dir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = s.Create(t.Context(), m21Session); err != nil {
		t.Fatal(err)
	}
	previous := session.TurnID("")
	for i, label := range []string{"m24-old-A", "m24-old-B"} {
		input := inbox.Input{ID: inbox.ID(label), Kind: inbox.InputExternal, Payload: []byte(fmt.Sprintf("%q", label))}
		if err = s.AppendInput(t.Context(), m21Session, input); err != nil {
			t.Fatal(err)
		}
		turn := session.Turn{ID: session.TurnID(label), PreviousTurnID: previous, Type: session.TurnRegular}
		if err = s.AppendTurn(t.Context(), m21Session, turn); err != nil {
			t.Fatal(err)
		}
		response := llm.Response{Stop: llm.StopComplete, Output: []llm.Item{{Type: llm.ItemMessage, Data: llm.Message{Role: llm.RoleAssistant, Text: label + " answer"}}}}
		tokens := int64(30000 + i*10000)
		if automatic {
			tokens += 70000
		}
		response.Usage = llm.Usage{InputTokens: tokens - 1000, OutputTokens: 1000}
		if err = s.AppendModelResponse(t.Context(), m21Session, sessionstore.ModelResponse{TurnID: turn.ID, Response: response}); err != nil {
			t.Fatal(err)
		}
		previous = turn.ID
	}
	if automatic {
		if err = s.AppendInput(t.Context(), m21Session, inbox.Input{ID: "m24-pending", Kind: inbox.InputExternal, Payload: []byte(`"m24-pending-original"`)}); err != nil {
			t.Fatal(err)
		}
	}
}

func m24Records(t *testing.T, items []sessionstore.Item) []inbox.CompactionRequestRecord {
	t.Helper()
	var out []inbox.CompactionRequestRecord
	for _, item := range items {
		if input, ok := item.Data.(inbox.Input); ok && input.Kind == inbox.InputControl {
			v, err := input.DecodeControlMessage()
			if err != nil {
				t.Fatal(err)
			}
			if v.Mode == inbox.CompactionRequest {
				out = append(out, v.Parameters.(inbox.CompactionRequestRecord))
			}
		}
	}
	return out
}

type m24HTTP struct {
	n       int64
	summary bool
	body    string
}

func m24HTTPWait(t *testing.T, ch <-chan m24HTTP, want int64) m24HTTP {
	t.Helper()
	select {
	case got := <-ch:
		if got.n != want {
			t.Fatalf("HTTP got %d want %d", got.n, want)
		}
		return got
	case <-time.After(10 * time.Second):
		t.Fatal("owned HTTP boundary missing")
		return m24HTTP{}
	}
}
func m24JoinHTTP(t *testing.T, ch <-chan int64, want int64) {
	t.Helper()
	select {
	case got := <-ch:
		if got != want {
			t.Fatalf("HTTP join %d want %d", got, want)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("owned HTTP join missing")
	}
}
func m24Server(t *testing.T, hold func(int64) bool, entered chan<- m24HTTP, joined chan<- int64, calls *atomic.Int64) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := calls.Add(1)
		defer func() { joined <- n }()
		body, err := io.ReadAll(r.Body)
		if err != nil {
			return
		}
		var request struct {
			Tools []any `json:"tools"`
		}
		if err = json.Unmarshal(body, &request); err != nil {
			t.Errorf("request: %v", err)
			return
		}
		summary := len(request.Tools) == 0
		if hold(n) {
			w.Header().Set("Content-Type", "text/event-stream")
			fmt.Fprint(w, "data: {\"type\":\"response.output_item.added\",\"output_index\":0,\"item\":{\"id\":\"partial\",\"type\":\"function_call\",\"call_id\":\"uncommitted\",\"name\":\"Bash\",\"arguments\":\"{\\\"command\\\":\\\"touch forbidden-compaction-effect\\\"}\",\"status\":\"completed\"}}\n\n")
			w.(http.Flusher).Flush()
			entered <- m24HTTP{n, summary, string(body)}
			<-r.Context().Done()
			return
		}
		entered <- m24HTTP{n, summary, string(body)}
		text := "m24-ordinary-final"
		if summary {
			text = "m24-usable-summary"
		}
		completed(w, n, assistant(n, text))
	}))
	t.Cleanup(server.Close)
	return server
}
func m24HasFinal(items []sessionstore.Item) bool {
	for _, item := range items {
		if r, ok := item.Data.(sessionstore.ModelResponse); ok {
			b, _ := json.Marshal(r.Response)
			if strings.Contains(string(b), "m24-ordinary-final") {
				return true
			}
		}
	}
	return false
}

func TestM24OwnedAutomaticSummaryRecoveryAndCommittedAdoption(t *testing.T) {
	var calls atomic.Int64
	entered := make(chan m24HTTP, 8)
	joined := make(chan int64, 8)
	server := m24Server(t, func(n int64) bool { return n == 1 || n == 3 }, entered, joined, &calls)
	cfg := m23Config(t, server.URL)
	dir := cfg.Bootstrap.Directory
	m24Seed(t, dir, true)
	h := m23Start(t, cfg)
	first := m24HTTPWait(t, entered, 1)
	if !first.summary {
		t.Fatal("expected automatic summary")
	}
	records := m24Records(t, m21ReadItems(t, dir))
	if len(records) != 1 || records[0].Request.Attempt != 1 || records[0].ManualInputID != "" || records[0].Request.Limit != 3 {
		t.Fatalf("first charge: %+v", records)
	}
	m23KillJoin(t, h, dir)
	m24JoinHTTP(t, joined, 1)
	h = m23Start(t, cfg)
	second := m24HTTPWait(t, entered, 2)
	if !second.summary {
		t.Fatal("retry changed kind")
	}
	m24JoinHTTP(t, joined, 2)
	third := m24HTTPWait(t, entered, 3)
	if third.summary || strings.Count(third.body, "m24-usable-summary") != 1 || strings.Count(third.body, "m24-pending-original") != 1 {
		t.Fatalf("adoption/pending context: %s", third.body)
	}
	items := m21ReadItems(t, dir)
	records = m24Records(t, items)
	if len(records) != 3 || records[1].Request.Action != "interrupted" || records[2].Request.Attempt != 2 || records[2].Request.RequestID != records[0].Request.RequestID || records[2].Request.TurnID == records[0].Request.TurnID {
		t.Fatalf("bounded summary retry: %+v", records)
	}
	summaries := 0
	for _, item := range items {
		if r, ok := item.Data.(sessionstore.ModelResponse); ok {
			b, _ := json.Marshal(r.Response)
			if strings.Contains(string(b), "m24-usable-summary") {
				summaries++
			}
		}
	}
	if summaries != 1 {
		t.Fatalf("committed summaries %d", summaries)
	}
	m23KillJoin(t, h, dir)
	m24JoinHTTP(t, joined, 3)
	h = m23Start(t, cfg)
	fourth := m24HTTPWait(t, entered, 4)
	if fourth.summary || strings.Count(fourth.body, "m24-usable-summary") != 1 || strings.Count(fourth.body, "m24-pending-original") != 1 {
		t.Fatalf("summary replay/working input: %s", fourth.body)
	}
	m24JoinHTTP(t, joined, 4)
	m23Wait(t, dir, m24HasFinal)
	if len(m24Records(t, m21ReadItems(t, dir))) != 3 {
		t.Fatal("committed usable summary redispatched")
	}
	m23Close(t, h, dir)
	h = m23Start(t, cfg)
	m23Close(t, h, dir)
	if calls.Load() != 4 {
		t.Fatalf("extra HTTP after adoption: %d", calls.Load())
	}
}

func TestM24OwnedManualOnlyLossesExhaustAndRemainDisabled(t *testing.T) {
	var calls atomic.Int64
	entered := make(chan m24HTTP, 8)
	joined := make(chan int64, 8)
	server := m24Server(t, func(n int64) bool { return n <= 3 }, entered, joined, &calls)
	cfg := m23Config(t, server.URL)
	dir := cfg.Bootstrap.Directory
	m24Seed(t, dir, false)
	h := m23Start(t, cfg)
	payload, err := json.Marshal(inbox.ControlMessage{Mode: inbox.Compact})
	if err != nil {
		t.Fatal(err)
	}
	if err = h.Submit(t.Context(), inbox.Input{ID: "m24-manual-origin", Kind: inbox.InputControl, Payload: payload}); err != nil {
		t.Fatal(err)
	}
	logical := ""
	for attempt := int64(1); attempt <= 3; attempt++ {
		request := m24HTTPWait(t, entered, attempt)
		if !request.summary {
			t.Fatal("manual-only retry changed kind")
		}
		records := m24Records(t, m21ReadItems(t, dir))
		last := records[len(records)-1]
		if logical == "" {
			logical = last.Request.RequestID
		}
		if last.Request.Action != "started" || last.Request.Attempt != int(attempt) || last.Request.RequestID != logical || last.Request.Limit != 3 || last.ManualInputID != "m24-manual-origin" || last.FailuresBefore != 0 {
			t.Fatalf("manual charge: %+v", last)
		}
		m23KillJoin(t, h, dir)
		m24JoinHTTP(t, joined, attempt)
		h = m23Start(t, cfg)
	}
	items := m23Wait(t, dir, func(items []sessionstore.Item) bool {
		records := m24Records(t, items)
		if len(records) == 0 || records[len(records)-1].Request.Action != "exhausted" {
			return false
		}
		// Exhaustion and disabling are separate canonical commits. Wait for
		// the latter too, rather than inspecting the gap between the writes.
		for _, item := range items {
			if input, ok := item.Data.(inbox.Input); ok && input.Kind == inbox.InputControl {
				control, err := input.DecodeControlMessage()
				if err != nil {
					t.Fatal(err)
				}
				if control.Mode == inbox.CompactionDisabled {
					return true
				}
			}
		}
		return false
	})
	disabled := 0
	for _, item := range items {
		if input, ok := item.Data.(inbox.Input); ok && input.Kind == inbox.InputControl {
			c, _ := input.DecodeControlMessage()
			if c.Mode == inbox.CompactionDisabled {
				disabled++
			}
		}
		if _, ok := item.Data.(sessionstore.ToolCallStatus); ok {
			t.Fatal("partial summary acquired tool authority")
		}
	}
	if disabled != 1 || calls.Load() != 3 {
		t.Fatalf("exhaustion disable=%d calls=%d", disabled, calls.Load())
	}
	m23Close(t, h, dir)
	h = m23Start(t, cfg)
	if calls.Load() != 3 {
		t.Fatal("restart replenished manual-only group")
	}
	m23Submit(t, h, "m24-fresh-after-exhaustion")
	ordinary := m24HTTPWait(t, entered, 4)
	if ordinary.summary || !strings.Contains(ordinary.body, "m24-old-A") || !strings.Contains(ordinary.body, "m24-old-B") {
		t.Fatalf("failed summary lost original history: %s", ordinary.body)
	}
	m24JoinHTTP(t, joined, 4)
	m23Wait(t, dir, m24HasFinal)
	m23Close(t, h, dir)
}
