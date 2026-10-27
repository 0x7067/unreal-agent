package subagent

import (
	"encoding/json/jsontext"
	"encoding/json/v2"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
)

const (
	MaxActivityChildren   = 64
	MaxActivityOperations = 32
	activityIdentityBytes = 128
	activityFrameBytes    = 1 << 20
	activityStateBytes    = 16 << 10
)

// ActivitySnapshot is a bounded, live-only diagnostic projection, not a control
// handle or a persisted/protocol type. ScopeID identifies this supervisor, not
// a global subagent namespace. No query calls the actor, child, store or filesystem.
//
// Child frames are observed before actor delivery. They can lead parent-side
// processing, and become stale when that delivery blocks. Tool states are LAST
// OBSERVED PERSISTED TOOL-CALL SNAPSHOTS, not live operation-manager state: ready
// or awaiting does not prove dispatch, and output silence does not prove a stall.
// This API retains only allowlisted executable names, never argv, raw commands,
// prompts, output, provider payloads, paths or process IDs. Purpose/resources are
// explicit bounded caller metadata, not inferred prompt text. Owner capabilities
// are opaque and separately checked by Hub. Parser/worker bounds are in io.go.
type ActivitySnapshot struct {
	ScopeID           string
	CapturedAt        time.Time
	ShutdownRequested bool // Supervisor context canceled; NOT a join.
	SupervisorJoined  bool // Actor processing ended and registered child reader/Wait goroutines joined.
	Children          []ChildActivity
	UntrackedStarts   uint64 // Cumulative attempts omitted after child slots filled; not a live-child count.
}

// ChildActivity separates text evidence, execution joins and result publication.
// None of these fields proves that the parent's operation manager persisted a
// terminal result, or that a candidate is correct/accepted. History is not loaded
// on startup; sequence zero, old/replayed frames and old generations are ignored.
type ChildActivity struct {
	ProgressEvidence                                                                                            string
	ProcessWaitReturnedAt, ReaderReturnedAt, OutputEOFObservedAt, OwnerExitProcessedAt, OwnerTerminalEnqueuedAt time.Time
	PreparationFailed                                                                                           bool
	DiagnosticCode                                                                                              string
	Inputs                                                                                                      []InputActivity
	OmittedInputs                                                                                               uint64

	Handle, ParentHandle, Purpose, Lifetime, GraphID, TaskID string
	ParentTask                                               string
	OwnedResources                                           []string
	TaskGeneration                                           uint64
	ReportAvailable                                          bool
	ReportAvailableAt                                        time.Time
	CancellationRequestedAt                                  time.Time
	LastInputID, InputBarrier                                string
	DeliveryError                                            bool

	Name, SessionID, OwnerOperationID string
	Generation                        int
	IdentityTruncated                 bool
	AttemptedAt, ProcessStartedAt     time.Time
	LastFrameAt                       time.Time
	LastSequence                      sessionstore.Sequence
	ModelTextObserved                 bool // Any nonempty model message; NOT necessarily a final report.
	ModelTextSequence                 sessionstore.Sequence
	CancellationRequested             bool // Owner CancelRemoteJob observed, not child acknowledgment or termination.
	StartFailed                       bool
	ProcessWaitReturned               bool
	OutputEOFObserved                 bool
	ReaderReturned                    bool
	OwnerExitProcessed                bool
	OwnerTerminalEnqueued             bool // Handler queue only, NOT downstream delivery or persistence.
	Operations                        []OperationActivity
	OverflowOperation                 *OperationActivity // One extra live blocker, then a sticky terminal tombstone until replaced by a distinct live ID.
	OverflowEvictions                 uint64             // Terminal overflow tombstones replaced; main inventory tombstones are never evicted.
	ObservedNonterminalOperations     int
	RelevantOperation                 *OperationActivity // Oldest retained nonterminal observation, if any.
	OmittedOperationObservations      uint64             // Cumulative, not unique or necessarily pending.
	SkippedFrames                     uint64             // Oversized frames omitted by projection only.
}

// OperationActivity retains identities and enums only, never raw operation state.
// A terminal observation is sticky for the operation ID within this attempt.
type OperationActivity struct {
	Executable  string
	Descendants []OperationActivity

	ID, TurnID, CallID string
	IdentityTruncated  bool
	Type               operation.Type
	Status             operation.Status
	ShellPhase         operation.ShellPhase
	DetailsUnavailable bool
	FirstObservedAt    time.Time
	ObservedAt         time.Time
	Sequence           sessionstore.Sequence
}

type activityProjection struct {
	mu       sync.Mutex // Protects only bounded, already-redacted data; never held over I/O or decoding.
	snapshot ActivitySnapshot
}

// ActivitySnapshot returns a detached copy. The API remains available during
// startup, blocked actor/child I/O and after shutdown, without admitting work.
func (current *Supervisor) ActivitySnapshot() ActivitySnapshot {
	result := current.activity.snapshotAt(time.Now())
	result.ShutdownRequested = current.ctx.Err() != nil
	return result
}

func (p *activityProjection) snapshotAt(now time.Time) ActivitySnapshot {
	p.mu.Lock()
	defer p.mu.Unlock()
	result := p.snapshot
	result.CapturedAt = now
	result.Children = append([]ChildActivity(nil), p.snapshot.Children...)
	for i := range result.Children {
		c := &result.Children[i]
		c.OwnedResources = append([]string(nil), c.OwnedResources...)
		c.Inputs = append([]InputActivity(nil), c.Inputs...)
		c.Operations = append([]OperationActivity(nil), c.Operations...)
		for j := range c.Operations {
			c.Operations[j].Descendants = cloneActivityPath(c.Operations[j].Descendants)
		}
		if c.OverflowOperation != nil {
			overflow := *c.OverflowOperation
			overflow.Descendants = cloneActivityPath(overflow.Descendants)
			c.OverflowOperation = &overflow
		}
		for j := range c.Operations {
			op := c.Operations[j]
			if !activityTerminal(op.Status) {
				c.ObservedNonterminalOperations++
				if c.RelevantOperation == nil {
					c.RelevantOperation = &op
				}
			}
		}
		if c.OverflowOperation != nil && !activityTerminal(c.OverflowOperation.Status) {
			c.ObservedNonterminalOperations++
			if c.RelevantOperation == nil {
				c.RelevantOperation = c.OverflowOperation
			}
		}
	}
	return result
}

func (p *activityProjection) begin(name, sessionID string, generation int, owner operation.ID, now time.Time) {
	p.mu.Lock()
	defer p.mu.Unlock()
	if len(sessionID) > activityIdentityBytes {
		p.snapshot.UntrackedStarts++
		return
	}
	var previous sessionstore.Sequence
	for i := range p.snapshot.Children {
		if p.snapshot.Children[i].SessionID == sessionID {
			previous = p.snapshot.Children[i].LastSequence
			p.snapshot.Children[i] = newChildActivity(name, sessionID, generation, owner, now)
			p.snapshot.Children[i].LastSequence = previous
			return
		}
	}
	if len(p.snapshot.Children) == MaxActivityChildren {
		p.snapshot.UntrackedStarts++
		return
	}
	p.snapshot.Children = append(p.snapshot.Children, newChildActivity(name, sessionID, generation, owner, now))
}

func newChildActivity(name, sessionID string, generation int, owner operation.ID, now time.Time) ChildActivity {
	c := ChildActivity{Generation: generation, AttemptedAt: now}
	c.Name = activityIdentity(name, &c.IdentityTruncated)
	c.SessionID = activityIdentity(sessionID, &c.IdentityTruncated)
	c.OwnerOperationID = activityIdentity(string(owner), &c.IdentityTruncated)
	return c
}

func (p *activityProjection) change(sessionID string, generation int, update func(*ChildActivity)) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.snapshot.Children {
		c := &p.snapshot.Children[i]
		if c.SessionID == sessionID && c.Generation == generation {
			update(c)
			return
		}
	}
}

func (p *activityProjection) joined() {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.snapshot.SupervisorJoined = true
}

// observeLine caps additional projection work. The preexisting reader still
// allocates whole JSONL lines and the actor still decodes them independently.
// Keep parsing/redaction outside the snapshot lock, and inspect only a fixed
// prefix of variable-length collections. No free-form payload is retained.
func (p *activityProjection) observeLine(sessionID string, generation int, line []byte, now time.Time) {
	if len(line) > activityFrameBytes {
		p.change(sessionID, generation, func(c *ChildActivity) { c.SkippedFrames++ })
		return
	}
	var item sessionstore.Item
	if json.Unmarshal(line, &item) != nil || item.Sequence == 0 {
		return
	}
	text := false
	report := false
	var ops []OperationActivity
	omitted := 0
	switch data := item.Data.(type) {
	case sessionstore.ModelResponse:
		report = data.Response.Stop == llm.StopComplete && data.Response.Failure == nil && len(data.Response.Output) <= MaxActivityOperations
		for _, output := range data.Response.Output[:min(len(data.Response.Output), MaxActivityOperations)] {
			if _, ok := output.Data.(llm.ToolCall); ok {
				report = false
			}
			if message, ok := output.Data.(llm.Message); ok && len(message.Text) != 0 {
				text = true
			}
		}
	case sessionstore.ToolCallStatus:
		for _, op := range data.Operations[:min(len(data.Operations), MaxActivityOperations)] {
			if len(op.ID) <= activityIdentityBytes {
				ops = append(ops, projectActivityOperation(data, op, item.Sequence, now))
			}
		}
		omitted = len(data.Operations) - len(ops)
	}
	p.change(sessionID, generation, func(c *ChildActivity) {
		if c.ReaderReturned || c.OwnerExitProcessed || item.Sequence <= c.LastSequence {
			return
		}
		c.LastSequence, c.LastFrameAt = item.Sequence, now
		c.ProgressEvidence = "persisted_child_session_frame"
		if text {
			c.ModelTextObserved, c.ModelTextSequence = true, item.Sequence
			if report && !c.ReportAvailable {
				c.ReportAvailable = true
				c.ReportAvailableAt = now
			}
		}
		c.OmittedOperationObservations += uint64(omitted)
		for _, op := range ops {
			found := false
			for i := range c.Operations {
				if c.Operations[i].ID == op.ID {
					if !activityTerminal(c.Operations[i].Status) {
						op.FirstObservedAt = c.Operations[i].FirstObservedAt
						c.Operations[i] = op
					}
					found = true
					break
				}
			}
			if !found {
				if len(c.Operations) < MaxActivityOperations {
					c.Operations = append(c.Operations, op)
				} else {
					switch {
					case c.OverflowOperation != nil && c.OverflowOperation.ID == op.ID:
						if !activityTerminal(c.OverflowOperation.Status) {
							op.FirstObservedAt = c.OverflowOperation.FirstObservedAt
							c.OverflowOperation = &op
						}
					case !activityTerminal(op.Status) && (c.OverflowOperation == nil || activityTerminal(c.OverflowOperation.Status)):
						if c.OverflowOperation != nil {
							c.OverflowEvictions++
						}
						c.OverflowOperation = &op
					default:
						c.OmittedOperationObservations++
					}
				}
			}
		}
	})
}

func projectActivityOperation(data sessionstore.ToolCallStatus, op operation.Operation, sequence sessionstore.Sequence, now time.Time) OperationActivity {
	return projectActivityOperationDepth(data, op, sequence, now, 0)
}
func projectActivityOperationDepth(data sessionstore.ToolCallStatus, op operation.Operation, sequence sessionstore.Sequence, now time.Time, depth int) OperationActivity {
	v := OperationActivity{FirstObservedAt: now, ObservedAt: now, Sequence: sequence}
	v.ID = activityIdentity(string(op.ID), &v.IdentityTruncated)
	v.TurnID = activityIdentity(string(data.TurnID), &v.IdentityTruncated)
	v.CallID = activityIdentity(data.CallID, &v.IdentityTruncated)
	switch op.Type {
	case operation.TypeShell, operation.TypeRemoteJob, operation.TypeValue:
		v.Type = operation.Type(strings.Clone(string(op.Type)))
	default:
		v.Type = "unknown"
	}
	switch op.Status {
	case operation.StatusReady, operation.StatusAwaiting, operation.StatusCanceling,
		operation.StatusCompleted, operation.StatusFailed, operation.StatusCanceled:
		v.Status = operation.Status(strings.Clone(string(op.Status)))
	default:
		v.Status = "unknown"
	}
	if op.Type == operation.TypeShell {
		var state struct {
			Phase operation.ShellPhase
			Input struct{ Command string }
		}
		if len(op.State) > activityStateBytes || json.Unmarshal(op.State, &state) != nil {
			v.DetailsUnavailable = true
		} else {
			v.Executable = safeExecutable(state.Input.Command)
			switch state.Phase {
			case "", operation.ShellPhaseCreateDirectory, operation.ShellPhaseCreateOut, operation.ShellPhaseCreateErr,
				operation.ShellPhaseProcess, operation.ShellPhaseReadOut, operation.ShellPhaseReadOutTail,
				operation.ShellPhaseReadErr, operation.ShellPhaseReadErrTail:
				v.ShellPhase = operation.ShellPhase(strings.Clone(string(state.Phase)))
			default:
				v.DetailsUnavailable = true
			}
		}
	}
	if op.Type == operation.TypeRemoteJob && depth < 4 && len(op.State) <= activityStateBytes {
		var remote struct {
			Plan   struct{ Type string }
			Handle jsontext.Value
		}
		if json.Unmarshal(op.State, &remote) == nil && remote.Plan.Type == "taskgraph.run" {
			var checkpoint struct {
				Version  int `json:"version"`
				Attempts map[string]struct {
					Child operation.Operation `json:"child_operation"`
				} `json:"attempts"`
			}
			if json.Unmarshal(remote.Handle, &checkpoint) == nil && checkpoint.Version == 1 {
				keys := make([]string, 0, len(checkpoint.Attempts))
				for k := range checkpoint.Attempts {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, key := range keys[:min(len(keys), MaxActivityOperations)] {
					child := checkpoint.Attempts[key].Child
					if child.ID != "" && !activityTerminal(child.Status) && len(child.ID) <= activityIdentityBytes {
						nested := projectActivityOperationDepth(data, child, sequence, now, depth+1)
						v.Descendants = append(v.Descendants, nested)
						break
					}
				}
			}
		}
	}
	return v
}

func activityTerminal(status operation.Status) bool {
	return status == operation.StatusCompleted || status == operation.StatusFailed || status == operation.StatusCanceled
}

func activityIdentity(s string, truncated *bool) string {
	if len(s) <= activityIdentityBytes {
		return strings.Clone(s)
	}
	*truncated = true
	s = s[:activityIdentityBytes]
	for !utf8.ValidString(s) {
		s = s[:len(s)-1]
	}
	return strings.Clone(s)
}

// Only an exact allowlisted first executable token is retained. No argument,
// assignment, path, prompt text or shell fragment is ever copied into status.
func safeExecutable(command string) string {
	token := strings.Fields(command)
	if len(token) == 0 {
		return ""
	}
	switch token[0] {
	case "find", "rg", "grep", "go", "git", "make", "cat", "ls", "pwd", "sleep", "python", "python3", "node", "npm", "sh", "bash":
		return token[0]
	}
	return ""
}

func (p *activityProjection) terminalEnqueued(id operation.ID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.snapshot.Children {
		c := &p.snapshot.Children[i]
		if c.OwnerOperationID == string(id) {
			c.OwnerTerminalEnqueued = true
			c.OwnerTerminalEnqueuedAt = time.Now()
		}
	}
}

func cloneActivityPath(path []OperationActivity) []OperationActivity {
	result := append([]OperationActivity(nil), path...)
	for i := range result {
		result[i].Descendants = cloneActivityPath(result[i].Descendants)
	}
	return result
}

// InputActivity records transport and persistence separately. Persistence proves
// the matching child session item, not model consumption. Late writer completion
// can never regress a persisted observation. IDs/payloads are not prompts.
type InputActivity struct {
	ID, Barrier                                                  string
	AdmittedAt, QueuedAt, WriteStartedAt, WrittenAt, PersistedAt time.Time
	CancellationRequested, CanceledBeforeWrite                   bool
}

func inputRank(barrier string) int {
	switch barrier {
	case "admitted":
		return 1
	case "queued":
		return 2
	case "write_started":
		return 3
	case "written", "write_failed":
		return 4
	case "persisted":
		return 5
	}
	return 0
}
func (p *activityProjection) input(session string, generation int, id, barrier string) {
	if id == "" || len(id) > activityIdentityBytes {
		return
	}
	now := time.Now()
	p.change(session, generation, func(c *ChildActivity) {
		index := -1
		for i := range c.Inputs {
			if c.Inputs[i].ID == id {
				index = i
				break
			}
		}
		if index < 0 {
			if len(c.Inputs) >= MaxActivityOperations {
				c.OmittedInputs++
				return
			}
			c.Inputs = append(c.Inputs, InputActivity{ID: id})
			index = len(c.Inputs) - 1
		}
		input := &c.Inputs[index]
		switch barrier {
		case "admitted":
			if input.AdmittedAt.IsZero() {
				input.AdmittedAt = now
			}
		case "queued":
			if input.QueuedAt.IsZero() {
				input.QueuedAt = now
			}
		case "write_started":
			if input.WriteStartedAt.IsZero() {
				input.WriteStartedAt = now
			}
		case "written":
			if input.WrittenAt.IsZero() {
				input.WrittenAt = now
			}
		case "persisted":
			if input.PersistedAt.IsZero() {
				input.PersistedAt = now
			}
		case "cancel_requested":
			input.CancellationRequested = true
		case "queued_frame_retracted":
			input.CancellationRequested = true
			input.CanceledBeforeWrite = true
		}
		if inputRank(barrier) > inputRank(input.Barrier) {
			input.Barrier = barrier
		}
		if c.LastInputID != id || inputRank(input.Barrier) >= inputRank(c.InputBarrier) {
			c.LastInputID = id
			c.InputBarrier = input.Barrier
		}
	})
}

func (p *activityProjection) preparationFailed(id operation.ID) {
	p.mu.Lock()
	defer p.mu.Unlock()
	for i := range p.snapshot.Children {
		c := &p.snapshot.Children[i]
		if c.OwnerOperationID == string(id) {
			c.PreparationFailed = true
			c.DiagnosticCode = "checkpoint_preparation_failed"
		}
	}
}
