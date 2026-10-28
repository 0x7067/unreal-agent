// Package extension implements only compiled, trusted Echo implementations.
// Registry mutation is an owner API, never a model-facing operation.
package extension

import (
	"bytes"
	"context"
	"encoding/json/jsontext"
	"encoding/json/v2"
	"errors"
	"fmt"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"

	"github.com/unreallabsai/unreal-agent/harness/operation"
)

const (
	Name                                                 = "Echo"
	SchemaVersion                                        = 1
	ImplementationVersion                                = 1
	PlanType              operation.RemoteJobPlanType    = "trusted_echo"
	PlanVersion           operation.RemoteJobPlanVersion = 1
	MaxGenerations                                       = 16
	MaxCalls                                             = 32
	MaxInput                                             = 4096
)

type Descriptor struct {
	Name                  string
	Generation            uint64
	SchemaVersion         int
	ImplementationVersion int
	Implementation        string
}

func (d Descriptor) validate() error {
	if d.Name != Name || d.Generation == 0 || d.SchemaVersion != 1 || d.ImplementationVersion != 1 || (d.Implementation != "A" && d.Implementation != "B") {
		return errors.New("unsupported trusted Echo descriptor")
	}
	return nil
}

type entry struct {
	descriptor Descriptor
	leases     int
}
type Registry struct {
	mu       sync.Mutex
	entries  map[uint64]*entry
	selected uint64
	used     map[uint64]bool
}

func NewRegistry() *Registry { return &Registry{entries: map[uint64]*entry{}, used: map[uint64]bool{}} }

// Install never changes selection. Generation numbers cannot be reused, even after removal.
func (r *Registry) Install(d Descriptor) error {
	if e := d.validate(); e != nil {
		return e
	}
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries == nil {
		r.entries = map[uint64]*entry{}
		r.used = map[uint64]bool{}
	}
	if r.used[d.Generation] || len(r.used) >= MaxGenerations {
		return errors.New("generation reused or registry capacity exceeded")
	}
	r.entries[d.Generation] = &entry{descriptor: d}
	r.used[d.Generation] = true
	return nil
}
func (r *Registry) Select(g uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.entries[g] == nil {
		return errors.New("generation unavailable")
	}
	r.selected = g
	return nil
}
func (r *Registry) Selected() (Descriptor, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e := r.entries[r.selected]; e != nil {
		return e.descriptor, nil
	}
	return Descriptor{}, errors.New("no selected trusted extension")
}
func (r *Registry) Lookup(g uint64) (Descriptor, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if e := r.entries[g]; e != nil {
		return e.descriptor, nil
	}
	return Descriptor{}, errors.New("pinned generation unavailable")
}
func (r *Registry) Remove(g uint64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := r.entries[g]
	if e == nil || g == r.selected || e.leases != 0 {
		return errors.New("generation selected, in use, or unavailable")
	}
	delete(r.entries, g)
	return nil
}
func (r *Registry) acquire(p Plan) (Descriptor, func(), error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	e := r.entries[p.Generation]
	if e == nil || !p.matches(e.descriptor) {
		return Descriptor{}, nil, errors.New("pinned extension unavailable or incompatible")
	}
	e.leases++
	var once sync.Once
	return e.descriptor, func() { once.Do(func() { r.mu.Lock(); e.leases--; r.mu.Unlock() }) }, nil
}

type Plan struct {
	Name                  string
	Generation            uint64
	SchemaVersion         int
	ImplementationVersion int
	Implementation        string
	Input                 string
	OwnerTurnID           string `json:",omitzero"`
}

func (p Plan) matches(d Descriptor) bool {
	return p.Name == d.Name && p.Generation == d.Generation && p.SchemaVersion == d.SchemaVersion && p.ImplementationVersion == d.ImplementationVersion && p.Implementation == d.Implementation
}
func (p Plan) validate() error {
	if e := (Descriptor{p.Name, p.Generation, p.SchemaVersion, p.ImplementationVersion, p.Implementation}).validate(); e != nil {
		return e
	}
	if len(p.OwnerTurnID) > 128 || strings.ContainsFunc(p.OwnerTurnID, unicode.IsControl) || !utf8.ValidString(p.OwnerTurnID) {
		return errors.New("invalid exact Echo owner")
	}
	if len(p.Input) > MaxInput || !utf8.ValidString(p.Input) {
		return errors.New("Echo input must be valid UTF-8 and at most 4096 bytes")
	}
	return nil
}
func NewSpec(d Descriptor, input string, owner ...string) (operation.Spec, error) {
	if len(owner) > 1 {
		return operation.Spec{}, errors.New("one exact owner allowed")
	}
	p := Plan{Name: d.Name, Generation: d.Generation, SchemaVersion: d.SchemaVersion, ImplementationVersion: d.ImplementationVersion, Implementation: d.Implementation, Input: input}
	if len(owner) == 1 {
		p.OwnerTurnID = owner[0]
		if p.OwnerTurnID == "" {
			return operation.Spec{}, errors.New("empty foreground owner")
		}
	}

	if e := p.validate(); e != nil {
		return operation.Spec{}, e
	}
	data, e := json.Marshal(p)
	if e != nil {
		return operation.Spec{}, e
	}
	return operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: PlanType, Version: PlanVersion, Data: data})
}
func Decode(op operation.Operation) (operation.RemoteJobState, Plan, error) {
	if op.MaxOutputLength != operation.DefaultMaxOutputLength {
		return operation.RemoteJobState{}, Plan{}, errors.New("unsupported Echo output envelope")
	}
	s, e := operation.DecodeRemoteJobState(op)
	if e != nil {
		return s, Plan{}, e
	}
	var strict operation.RemoteJobState
	if e = json.Unmarshal(op.State, &strict, json.RejectUnknownMembers(true)); e != nil {
		return s, Plan{}, e
	}
	if s.Plan.Type != PlanType || s.Plan.Version != PlanVersion {
		return s, Plan{}, errors.New("unsupported Echo plan")
	}
	var p Plan
	e = json.Unmarshal(s.Plan.Data, &p, json.RejectUnknownMembers(true))
	if e == nil {
		e = p.validate()
	}
	if e != nil {
		return s, p, e
	}
	var fields map[string]jsontext.Value
	if err := json.Unmarshal(s.Plan.Data, &fields); err != nil {
		return s, p, err
	}
	for _, name := range []string{"Name", "Generation", "SchemaVersion", "ImplementationVersion", "Implementation", "Input"} {
		value, ok := fields[name]
		if !ok || bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
			return s, p, errors.New("required Echo plan field missing/null")
		}
	}
	if value, ok := fields["OwnerTurnID"]; ok && bytes.Equal(bytes.TrimSpace(value), []byte("null")) {
		return s, p, errors.New("null Echo owner")
	}
	if len(s.TerminalResult) > MaxInput+2 || s.ResultTruncated || len(s.Handle) != 0 || len(s.Subscription) != 0 || len(s.OutstandingInput) != 0 {
		return s, p, errors.New("invalid Echo checkpoint")
	}
	switch op.Status {
	case operation.StatusReady, operation.StatusAwaiting, operation.StatusCanceling:
		if s.TerminalResult != "" || s.TerminalError != "" {
			return s, p, errors.New("nonterminal Echo checkpoint has terminal output")
		}
	case operation.StatusCompleted:
		if s.TerminalResult != p.Implementation+":"+p.Input || s.TerminalError != "" {
			return s, p, errors.New("invalid stored Echo result")
		}
	case operation.StatusFailed, operation.StatusCanceled:
		if s.TerminalResult != "" {
			return s, p, errors.New("invalid failed Echo output")
		}
	default:
		return s, p, errors.New("invalid Echo status")
	}
	return s, p, nil
}

// ValidateRecovery performs no I/O. Historical terminal output never requires code lookup.
func ValidateRecovery(op operation.Operation, r *Registry) error {
	if op.Status == operation.StatusCanceling {
		return errors.New("canceling Echo requires explicit reconciliation")
	}
	_, p, e := Decode(op)
	if e != nil {
		return e
	}
	switch op.Status {
	case operation.StatusCompleted, operation.StatusFailed, operation.StatusCanceled:
		return nil
	}
	if r == nil {
		return errors.New("trusted extension registry unavailable")
	}
	d, e := r.Lookup(p.Generation)
	if e != nil {
		return e
	}
	if !p.matches(d) {
		return errors.New("pinned extension incompatible")
	}
	return nil
}

type Config struct {
	Registry      *Registry
	BeforeExecute func(context.Context, Descriptor) error
}
type call struct {
	execution sync.WaitGroup
	cancel    context.CancelFunc
	done      chan struct{}
}
type Handler struct {
	mu      sync.Mutex
	cfg     Config
	calls   map[operation.ID]*call
	updates chan operation.Operation
	workers sync.WaitGroup
	closed  bool
	stopped chan struct{}
	once    sync.Once
}

func NewHandler(c Config) (*Handler, error) {
	if c.Registry == nil {
		return nil, errors.New("registry required")
	}
	return &Handler{cfg: c, calls: map[operation.ID]*call{}, updates: make(chan operation.Operation, MaxCalls), stopped: make(chan struct{})}, nil
}
func (h *Handler) RemoteJobPlanType() operation.RemoteJobPlanType       { return PlanType }
func (h *Handler) RemoteJobPlanVersion() operation.RemoteJobPlanVersion { return PlanVersion }
func (h *Handler) RemoteJobUpdates() <-chan operation.Operation         { return h.updates }
func (h *Handler) AddRemoteJob(op operation.Operation) error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.closed {
		return errors.New("Echo handler closed")
	}
	if op.ID == "" {
		return errors.New("operation ID required")
	}
	if _, ok := h.calls[op.ID]; ok {
		return errors.New("duplicate Echo operation")
	}
	if len(h.calls) >= MaxCalls {
		return errors.New("Echo call capacity exceeded")
	}
	s, p, e := Decode(op)
	if e != nil {
		return e
	}
	if op.Status != operation.StatusReady && op.Status != operation.StatusAwaiting {
		return errors.New("Echo admission requires ready or awaiting")
	}
	d, release, e := h.cfg.Registry.acquire(p)
	if e != nil {
		return e
	}
	op.State = bytes.Clone(op.State)
	op.Idempotency = bytes.Clone(op.Idempotency)
	s.Plan.Data = bytes.Clone(s.Plan.Data)
	ctx, cancel := context.WithCancel(context.Background())
	c := &call{cancel: cancel, done: make(chan struct{})}
	h.calls[op.ID] = c
	c.execution.Add(1)
	h.workers.Add(2)
	result := make(chan operation.Operation, 1)
	go func() {
		defer h.workers.Done()
		defer c.execution.Done()
		defer close(c.done)
		defer release()
		defer cancel()
		var err error
		if h.cfg.BeforeExecute != nil {
			err = h.cfg.BeforeExecute(ctx, d)
		}
		status := operation.StatusCompleted
		s.TerminalResult = d.Implementation + ":" + p.Input
		if ctx.Err() != nil {
			status = operation.StatusCanceled
			s.TerminalResult = ""
			s.TerminalError = "Echo canceled"
		} else if err != nil {
			status = operation.StatusFailed
			s.TerminalResult = ""
			s.TerminalError = "trusted Echo execution failed"
		}
		step, e := operation.UpdateRemoteJob(op, s, status)
		if e != nil {
			panic(fmt.Sprintf("validated Echo transition failed: %v", e))
		}
		result <- *step.Operation
	}()
	go func() {
		defer h.workers.Done()
		c.execution.Wait() // actual execution join before terminal publication
		h.updates <- <-result
	}()

	return nil
}

// Cancel joins the local worker before returning. BeforeExecute must honor ctx.
func (h *Handler) CancelRemoteJob(id operation.ID, _ string) error {
	h.mu.Lock()
	c := h.calls[id]
	h.mu.Unlock()
	if c == nil {
		return errors.New("Echo call unavailable")
	}
	select {
	case <-c.done:
		return errors.New("Echo already finished; cancellation not admitted")
	default:
	}
	c.cancel()
	c.execution.Wait()
	return nil
}
func (h *Handler) Close() error {
	h.once.Do(func() {
		h.mu.Lock()
		h.closed = true
		for _, c := range h.calls {
			c.cancel()
		}
		h.mu.Unlock()
		h.workers.Wait()
		close(h.updates)
		close(h.stopped)
	})
	return nil
}
func (h *Handler) AwaitStopped(ctx context.Context) error {
	select {
	case <-h.stopped:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

var _ operation.RemoteJobHandler = (*Handler)(nil)
