// Package durablejob is an unwired adapter to an independently supervised executor.
// Closing this client never cancels executor-owned work.
package durablejob

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	stdjson "encoding/json"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/jobexecutor"
	"github.com/unreallabsai/unreal-agent/harness/operation"
)

const PlanType operation.RemoteJobPlanType = "durable_executor_job"
const Version = 1

// Plan is trusted selection only; paths, commands and credentials are forbidden.
type Plan struct {
	Version                              int
	EnvironmentID, ControllerID, JobName string
}

// Checkpoint is the complete adapter-owned RemoteJobState.Handle schema.
// Output contains bytes (base64 in JSON), so partial UTF-8 is preserved.
type Checkpoint struct {
	Version         int
	Handle          jobexecutor.Handle
	Offset          int
	Output          []byte
	Truncated       bool
	Phase           string
	Failures        int
	CancelRequested bool
}
type Config struct {
	Client                      *jobexecutor.Client
	EnvironmentID, ControllerID string
	Jobs                        []string
	PollInterval                time.Duration
	RequestTimeout              time.Duration
	MaxFailures                 int
}
type entry struct {
	op     operation.Operation
	state  operation.RemoteJobState
	plan   jobexecutor.Plan
	cp     Checkpoint
	cancel chan struct{}
	once   sync.Once
	live   bool
}
type Handler struct {
	cfg     Config
	ctx     context.Context
	stop    context.CancelFunc
	mu      sync.Mutex
	entries map[operation.ID]*entry
	closed  bool
	workers sync.WaitGroup
	updates chan operation.Operation
	done    chan struct{}
}

var _ operation.RemoteJobHandler = (*Handler)(nil)

func valid(s string) bool {
	if s == "" || len(s) > 128 || s == "." || s == ".." {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("._-", c)) {
			return false
		}
	}
	return true
}
func New(ctx context.Context, cfg Config) (*Handler, error) {
	if cfg.Client == nil || !valid(cfg.EnvironmentID) || !valid(cfg.ControllerID) || len(cfg.Jobs) == 0 {
		return nil, errors.New("invalid adapter authority")
	}
	for _, j := range cfg.Jobs {
		if !valid(j) {
			return nil, errors.New("invalid trusted job name")
		}
	}
	cfg.Jobs = append([]string(nil), cfg.Jobs...)
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 100 * time.Millisecond
	}
	if cfg.RequestTimeout == 0 {
		cfg.RequestTimeout = 5 * time.Second
	}
	if cfg.MaxFailures == 0 {
		cfg.MaxFailures = 3
	}
	if cfg.PollInterval < 10*time.Millisecond || cfg.PollInterval > time.Minute || cfg.RequestTimeout < time.Millisecond || cfg.RequestTimeout > 10*time.Second || cfg.MaxFailures < 1 || cfg.MaxFailures > 8 {
		return nil, errors.New("invalid bounded retry policy")
	}
	ctx, stop := context.WithCancel(ctx)
	h := &Handler{cfg: cfg, ctx: ctx, stop: stop, entries: make(map[operation.ID]*entry), updates: make(chan operation.Operation), done: make(chan struct{})}
	go func() {
		<-ctx.Done()
		h.mu.Lock()
		h.closed = true
		h.mu.Unlock()
		h.workers.Wait()
		cfg.Client.Close()
		close(h.updates)
		close(h.done)
	}()
	return h, nil
}
func (*Handler) RemoteJobPlanType() operation.RemoteJobPlanType       { return PlanType }
func (*Handler) RemoteJobPlanVersion() operation.RemoteJobPlanVersion { return Version }
func (h *Handler) RemoteJobUpdates() <-chan operation.Operation       { return h.updates }

// Close joins only adapter requests and goroutines, not independent jobs.
func (h *Handler) Close() error { h.stop(); return h.AwaitStopped(context.Background()) }
func (h *Handler) AwaitStopped(ctx context.Context) error {
	select {
	case <-h.done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
func derive(id operation.ID, p Plan) jobexecutor.Plan {
	b, _ := stdjson.Marshal(struct {
		ID   operation.ID
		Plan Plan
	}{id, p})
	sum := sha256.Sum256(b)
	key := hex.EncodeToString(sum[:])
	return jobexecutor.Plan{Version: jobexecutor.Version, OperationID: "op-" + key, JobID: "job-" + key, AttemptID: "attempt-" + key, ControllerID: p.ControllerID, EnvironmentID: p.EnvironmentID, JobName: p.JobName}
}
func authority(p jobexecutor.Plan) jobexecutor.Handle {
	b, _ := stdjson.Marshal(p)
	sum := sha256.Sum256(b)
	return jobexecutor.Handle{Version: jobexecutor.Version, OperationID: p.OperationID, JobID: p.JobID, AttemptID: p.AttemptID, ControllerID: p.ControllerID, EnvironmentID: p.EnvironmentID, Fingerprint: hex.EncodeToString(sum[:])}
}

// ValidateRecovery is pure: no requests, admission or goroutines. Configuration
// must be the same explicitly trusted environment/controller/job selection.
func ValidateRecovery(op operation.Operation, cfg Config) error {
	if cfg.MaxFailures == 0 {
		cfg.MaxFailures = 3
	}
	_, err := (&Handler{cfg: cfg}).prepare(op)
	return err
}

func (h *Handler) prepare(op operation.Operation) (*entry, error) {
	if op.ID == "" || len(op.ID) > 1024 || len(op.State) > 256<<10 {
		return nil, errors.New("invalid operation identity or state size")
	}
	if op.Status != operation.StatusReady && op.Status != operation.StatusAwaiting && op.Status != operation.StatusCanceling {
		return nil, errors.New("operation is not live")
	}
	s, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		return nil, err
	}
	// Strict outer schema as well as strict adapter-owned plan/checkpoint.
	if err = json.Unmarshal(op.State, &s, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if s.Plan.Type != PlanType || s.Plan.Version != Version || len(s.Plan.Data) > 4096 {
		return nil, errors.New("unsupported adapter plan")
	}
	var p Plan
	if err = json.Unmarshal(s.Plan.Data, &p, json.RejectUnknownMembers(true)); err != nil {
		return nil, err
	}
	if p.Version != Version || p.EnvironmentID != h.cfg.EnvironmentID || p.ControllerID != h.cfg.ControllerID || !valid(p.JobName) {
		return nil, errors.New("adapter plan authority mismatch")
	}
	allowed := false
	for _, j := range h.cfg.Jobs {
		allowed = allowed || j == p.JobName
	}
	if !allowed {
		return nil, errors.New("untrusted job name")
	}
	ep := derive(op.ID, p)
	cp := Checkpoint{Version: Version, Handle: authority(ep), Phase: "submitting"}
	if len(s.Handle) > 0 {
		if len(s.Handle) > 128<<10 {
			return nil, errors.New("checkpoint too large")
		}
		var saved Checkpoint
		if err = json.Unmarshal(s.Handle, &saved, json.RejectUnknownMembers(true)); err != nil {
			return nil, err
		}
		var fields map[string]jsontext.Value
		if err = json.Unmarshal(s.Handle, &fields); err != nil {
			return nil, err
		}
		for _, name := range []string{"Version", "Handle", "Offset", "Output", "Truncated", "Phase", "Failures", "CancelRequested"} {
			value, ok := fields[name]
			if !ok {
				return nil, fmt.Errorf("missing checkpoint field %s", name)
			}
			if name != "Output" && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
				return nil, fmt.Errorf("null checkpoint field %s", name)
			}
		}
		cp = saved
		limit := min(op.MaxOutputLength, jobexecutor.MaxOutput)
		if cp.Version != Version || cp.Handle != authority(ep) || len(cp.Output) != min(cp.Offset, limit) || cp.Offset > jobexecutor.MaxOutput || cp.Offset < 0 || (cp.Offset > len(cp.Output) && !cp.Truncated) || cp.Failures < 0 || cp.Failures > h.cfg.MaxFailures {
			return nil, errors.New("invalid checkpoint identity or output")
		}
		switch cp.Phase {
		case "submitting", "reserved", "running", "unknown", "unavailable":
		default:
			return nil, errors.New("unsupported live checkpoint phase")
		}
	}
	if len(s.OutstandingInput) > 0 || len(s.Subscription) > 0 {
		return nil, errors.New("unsupported adapter input/subscription")
	}
	if op.Status == operation.StatusCanceling {
		cp.CancelRequested = true
	}
	// Clone caller-owned bytes: manager and adapter never share mutable state.
	op.State = bytes.Clone(op.State)
	op.Idempotency = bytes.Clone(op.Idempotency)
	s.Plan.Data = bytes.Clone(s.Plan.Data)
	e := &entry{op: op, state: s, plan: ep, cp: cp, cancel: make(chan struct{}), live: true}
	return e, nil
}
func (h *Handler) AddRemoteJob(op operation.Operation) error {
	e, err := h.prepare(op)
	if err != nil {
		return err
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed || h.ctx.Err() != nil {
		return errors.New("adapter closed")
	}
	if _, ok := h.entries[op.ID]; ok {
		return errors.New("operation already admitted")
	}
	if len(h.entries) >= jobexecutor.MaxJobs {
		return errors.New("adapter capacity reached")
	}
	h.entries[op.ID] = e
	h.workers.Add(1)
	go func() { defer h.workers.Done(); h.run(e) }()
	return nil
}
func (h *Handler) CancelRemoteJob(id operation.ID, _ string) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return errors.New("adapter closed")
	}
	e := h.entries[id]
	if e == nil || !e.live {
		return errors.New("unknown operation")
	}
	// No caller-supplied handle is accepted: only the validated committed binding.
	e.once.Do(func() { close(e.cancel) })
	return nil
}

// encodeState deliberately retains the exact Plan.Data bytes for manager fences.
func encodeState(s operation.RemoteJobState) ([]byte, error) {
	raw := s.Plan.Data
	s.Plan.Data = jsontext.Value(`null`)
	b, err := json.Marshal(s)
	if err != nil {
		return nil, err
	}
	p, err := json.Marshal(s.Plan)
	if err != nil {
		return nil, err
	}
	typ, _ := json.Marshal(s.Plan.Type)
	replacement := []byte(`"Plan":{"Type":` + string(typ) + `,"Version":` + strconv.Itoa(int(s.Plan.Version)) + `,"Data":` + string(raw) + `}`)
	needle := append([]byte(`"Plan":`), p...)
	if bytes.Count(b, needle) != 1 {
		return nil, errors.New("cannot preserve plan bytes")
	}
	return bytes.Replace(b, needle, replacement, 1), nil
}
func (h *Handler) publish(e *entry, status operation.Status) bool {
	b, err := json.Marshal(e.cp)
	if err != nil {
		return false
	}
	e.state.Handle = b
	e.state.NextInspectionAt = time.Now().Add(h.cfg.PollInterval)
	e.state.TerminalResult = strings.ToValidUTF8(string(e.cp.Output), "�")
	e.state.ResultBytes = len(e.cp.Output)
	e.state.ResultTruncated = e.cp.Truncated
	b, err = encodeState(e.state)
	if err != nil {
		return false
	}
	e.op.State = b
	e.op.Status = status
	out := e.op
	out.State = bytes.Clone(b)
	out.Idempotency = bytes.Clone(out.Idempotency)
	select {
	case h.updates <- out:
		return true
	case <-h.ctx.Done():
		return false
	}
}
func (h *Handler) retire(e *entry) { h.mu.Lock(); e.live = false; h.mu.Unlock() }
func (h *Handler) run(e *entry) {
	defer h.retire(e)
	if e.cp.Failures >= h.cfg.MaxFailures {
		h.retire(e)
		status := operation.StatusAwaiting
		if e.cp.CancelRequested {
			status = operation.StatusCanceling
		}
		h.publish(e, status)
		return
	}
	// A fresh/recovered submit always uses the same full stable identity. Submit
	// returns output from zero; subsequent inspections request the saved offset.
	submitted := e.cp.Phase != "submitting" && e.cp.Phase != "unavailable"
	for {
		select {
		case <-h.ctx.Done():
			return
		default:
		}
		select {
		case <-e.cancel:
			e.cp.CancelRequested = true
		default:
		}
		ctx, cancel := context.WithTimeout(h.ctx, h.cfg.RequestTimeout)
		var snap jobexecutor.Snapshot
		var err error
		full := false
		if !submitted {
			snap, err = h.cfg.Client.Submit(ctx, e.plan)
			full = true
		} else if e.cp.CancelRequested {
			snap, err = h.cfg.Client.Cancel(ctx, e.cp.Handle)
			full = true
		} else {
			snap, err = h.cfg.Client.Inspect(ctx, e.cp.Handle, e.cp.Offset)
		}
		cancel()
		status := operation.StatusAwaiting
		if e.cp.CancelRequested {
			status = operation.StatusCanceling
		}
		if err != nil {
			e.cp.Failures++
			e.cp.Phase = "unavailable"
			e.state.TerminalError = "executor unavailable or acknowledgment unknown"
			if e.cp.Failures >= h.cfg.MaxFailures {
				h.retire(e)
			}
			if !h.publish(e, status) || e.cp.Failures >= h.cfg.MaxFailures {
				return
			}
		} else {
			if snap.Handle != e.cp.Handle || snap.Offset < e.cp.Offset || snap.Offset > jobexecutor.MaxOutput {
				e.cp.Phase = "unknown"
				e.state.TerminalError = "executor authority or offset mismatch"
				h.retire(e)
				h.publish(e, status)
				return
			}
			delta := snap.Output
			if full {
				if len(delta) < e.cp.Offset || !bytes.Equal(delta[:len(e.cp.Output)], e.cp.Output) {
					e.cp.Phase = "unknown"
					e.state.TerminalError = "executor output prefix mismatch"
					h.retire(e)
					h.publish(e, status)
					return
				}
				delta = delta[e.cp.Offset:]
			}
			if snap.Offset != e.cp.Offset+len(delta) {
				e.cp.Phase = "unknown"
				e.state.TerminalError = "executor output offset mismatch"
				h.retire(e)
				h.publish(e, status)
				return
			}
			room := min(e.op.MaxOutputLength, jobexecutor.MaxOutput) - len(e.cp.Output)
			if len(delta) > room {
				delta = delta[:room]
				e.cp.Truncated = true
			}
			e.cp.Output = append(e.cp.Output, delta...)
			e.cp.Offset = snap.Offset
			e.cp.Truncated = e.cp.Truncated || snap.Truncated
			e.cp.Failures = 0
			e.cp.Phase = snap.State
			e.state.TerminalError = ""
			submitted = true
			terminal := false
			switch snap.State {
			case "completed":
				if e.cp.CancelRequested {
					status = operation.StatusFailed
					e.state.TerminalError = "job completed before cancellation acknowledgment"
				} else {
					status = operation.StatusCompleted
				}
				terminal = true
			case "failed":
				status = operation.StatusFailed
				e.state.TerminalError = fmt.Sprintf("executor exit %d: %s", snap.ExitCode, snap.Error)
				terminal = true
			case "canceled":
				status = operation.StatusCanceled
				terminal = true
			case "unknown":
				e.state.TerminalError = "executor ownership unknown; no replay"
				h.retire(e)
				h.publish(e, status)
				return
			case "reserved", "running":
			default:
				e.cp.Phase = "unknown"
				e.state.TerminalError = "unsupported executor state"
				h.retire(e)
				h.publish(e, status)
				return
			}
			if terminal {
				h.retire(e)
			}
			if !h.publish(e, status) || terminal {
				return
			}
		}
		wake := e.cancel
		if e.cp.CancelRequested {
			wake = nil
		}
		timer := time.NewTimer(h.cfg.PollInterval)
		select {
		case <-timer.C:
		case <-h.ctx.Done():
			timer.Stop()
			return
		case <-wake:
			timer.Stop()
			e.cp.CancelRequested = true
		}
	}
}

// TerminalCleanupProof requires canonical terminal operation plus an executor
// terminal acknowledgment with exact stable authority, not a local handler join.
// It is pure and never infers process completion from an unavailable transport.
func TerminalCleanupProof(op operation.Operation, cfg Config) error {
	if op.Status != operation.StatusCompleted && op.Status != operation.StatusFailed && op.Status != operation.StatusCanceled {
		return errors.New("job not terminal")
	}
	state, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		return err
	}
	var cp Checkpoint
	if err = json.Unmarshal(state.Handle, &cp, json.RejectUnknownMembers(true)); err != nil {
		return err
	}
	if cp.Phase != "completed" && cp.Phase != "failed" && cp.Phase != "canceled" {
		return errors.New("executor terminal acknowledgment unavailable")
	}
	if op.Status == operation.StatusCanceled && cp.Phase != "canceled" {
		return errors.New("executor cancellation not acknowledged")
	}
	// Reuse the strict authority/required-field/output validator on an inert copy.
	var fields map[string]jsontext.Value
	if err = json.Unmarshal(state.Handle, &fields); err != nil {
		return err
	}
	for _, name := range []string{"Version", "Handle", "Offset", "Output", "Truncated", "Phase", "Failures", "CancelRequested"} {
		value, ok := fields[name]
		if !ok || (name != "Output" && bytes.Equal(bytes.TrimSpace(value), []byte("null"))) {
			return errors.New("missing/null terminal checkpoint field")
		}
	}
	cp.Phase = "running"
	state.Handle, err = json.Marshal(cp)
	if err != nil {
		return err
	}
	op.State, err = json.Marshal(state)
	if err != nil {
		return err
	}
	op.Status = operation.StatusAwaiting
	return ValidateRecovery(op, cfg)
}

// TerminalSelfProof validates stored identity without trusting a current default
// selection. The caller must separately attest the exact configured handler join.
func TerminalSelfProof(op operation.Operation) error {
	state, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		return err
	}
	var p Plan
	if err = json.Unmarshal(state.Plan.Data, &p, json.RejectUnknownMembers(true)); err != nil {
		return err
	}
	if !valid(p.EnvironmentID) || !valid(p.ControllerID) || !valid(p.JobName) {
		return errors.New("invalid terminal job identity")
	}
	return TerminalCleanupProof(op, Config{EnvironmentID: p.EnvironmentID, ControllerID: p.ControllerID, Jobs: []string{p.JobName}})
}
