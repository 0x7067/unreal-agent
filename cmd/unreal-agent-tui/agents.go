package main

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"
	"uuid"

	tea "charm.land/bubbletea/v2"
	"github.com/unreallabsai/unreal-agent/cmd/internal/subagent"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/tool"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

type activityInspector interface {
	ActivitySnapshot(context.Context) (workerhost.Activity, error)
}
type activityPoll struct{}
type activityFinished struct {
	activity       workerhost.Activity
	err            error
	action, handle string
	periodic       bool
}

func (m model) activityTimer() tea.Cmd {
	if _, ok := m.inputs.(activityInspector); !ok {
		return nil
	}
	return tea.Tick(time.Second, func(time.Time) tea.Msg { return activityPoll{} })
}
func (m model) queryActivity(action, handle string, periodic bool) tea.Cmd {
	inspector, ok := m.inputs.(activityInspector)
	if !ok {
		return func() tea.Msg {
			return activityFinished{err: fmt.Errorf("live agent inspection unavailable"), action: action, handle: handle, periodic: periodic}
		}
	}
	ctx := m.ctx
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		activity, err := inspector.ActivitySnapshot(ctx)
		return activityFinished{activity: activity, err: err, action: action, handle: handle, periodic: periodic}
	}
}
func (m *model) applyActivity(msg activityFinished) {
	if msg.err != nil {
		if msg.activity.Generation != 0 && msg.activity.Generation < m.workerGeneration {
			return
		}
		m.activityError = singleLine(msg.err.Error())
		if msg.action != "" {
			m.append("Agents", "Live snapshot unavailable: "+msg.err.Error()+"\nAn unavailable query is not an idle or drain acknowledgment.")
		}
		return
	}
	if msg.activity.Generation < m.workerGeneration || msg.activity.Generation < m.activity.Generation {
		return
	}
	if msg.activity.Generation == m.activity.Generation && msg.activity.Snapshot.CapturedAt.Before(m.activity.Snapshot.CapturedAt) {
		return
	}
	if msg.activity.Snapshot.Version != 1 {
		m.activityError = "unsupported fleet snapshot version"
		if msg.action != "" {
			m.append("Agents", m.activityError)
		}
		return
	}
	m.activity = m.ensureTerminalActivity(msg.activity)
	m.activityError = ""
	if msg.action == "list" {
		var lines []string
		lines = append(lines, "Live-only activity · worker generation "+strconv.FormatUint(m.activity.Generation, 10))
		for _, agent := range m.activity.Snapshot.Agents {
			lines = append(lines, agentTarget(agent)+" · "+childActivityLine(agent.ChildActivity, m.activity.Snapshot.CapturedAt))
		}
		if len(m.activity.Snapshot.Agents) == 0 {
			lines = append(lines, "No supervised agents observed. This does not clear pending tool cards.")
		}
		if omitted := m.activity.Snapshot.OmittedAgents; omitted > 0 {
			lines = append(lines, fmt.Sprintf("%d agents omitted; absence is not idle proof", omitted))
		}
		if resources := m.activity.Resources; resources != nil {
			account := resources.Account
			lines = append(lines, fmt.Sprintf("Resources (%s): work %d / %d · management %d / %d · total %d owned · peak %d · rejected %d", account.Scope, account.WorkOwned, account.Limit, account.ManagementOwned, account.ManagementLimit, account.Owned, account.Peak, account.Rejected))
			for _, op := range resources.Operations {
				if op.Owned {
					lines = append(lines, fmt.Sprintf("  %s %s · %s · age %s · cancellation/drain distinct", op.Type, op.ID, op.Phase, activityAge(m.activity.Snapshot.CapturedAt, op.StartedAt)))
				}
			}
			if resources.Omitted > 0 {
				lines = append(lines, fmt.Sprintf("%d resource observations omitted", resources.Omitted))
			}
		}
		lines = append(lines, agentsUsage)
		m.append("Agents", strings.Join(lines, "\n"))
	}
	if msg.action == "inspect" {
		handle, generation, _ := splitAgentTarget(msg.handle)
		for _, agent := range m.activity.Snapshot.Agents {
			if agent.Handle == handle && (generation == 0 || generation == uint64(agent.Generation)) {
				detail := childActivityDetail(agent.ChildActivity, agentTarget(agent), m.activity.Snapshot.CapturedAt)
				detail += "\nPhase: " + singleLine(agent.Phase) + " · scope: " + singleLine(agent.ScopeID) + " · lifetime: " + singleLine(agent.Lifetime)
				if agent.GraphID != "" {
					detail += fmt.Sprintf("\nGraph/task: %s/%s · task generation %d", singleLine(agent.GraphID), singleLine(agent.TaskID), agent.TaskGeneration)
				}
				if agent.Purpose != "" {
					detail += "\nPurpose: " + singleLine(agent.Purpose)
				}
				m.append("Agent inspection", detail)
				return
			}
		}
		m.append("Agent inspection", "Unknown/stale scoped handle; refresh with /agents list. No action was taken.")
	}
}

// Retain monotone terminal evidence for the same scoped attempt. A new worker
// generation has a different live hub and is never merged with historical data.
func (m model) ensureTerminalActivity(next workerhost.Activity) workerhost.Activity {
	if next.Generation != m.activity.Generation {
		return next
	}
	for i := range next.Snapshot.Agents {
		a := &next.Snapshot.Agents[i]
		for _, previous := range m.activity.Snapshot.Agents {
			if a.Handle == previous.Handle && a.Generation == previous.Generation && previous.OwnerTerminalEnqueued {
				*a = previous
				break
			}
		}
	}
	return next
}
func agentTarget(a subagent.AgentActivity) string { return a.Handle + "@" + strconv.Itoa(a.Generation) }
func splitAgentTarget(target string) (string, uint64, error) {
	handle, generationText, ok := strings.Cut(target, "@")
	if !ok {
		return handle, 0, nil
	}
	generation, err := strconv.ParseUint(generationText, 10, 64)
	if err != nil || generation == 0 || handle == "" {
		return "", 0, fmt.Errorf("target must be HANDLE@GENERATION from /agents list")
	}
	return handle, generation, nil
}
func (m *model) startAgentCommand(text string) tea.Cmd {
	m.composer.Reset()
	command, err := parseAgentCommand(text)
	if err != nil {
		m.append("Agents", err.Error())
		return nil
	}
	if command.action == "list" || command.action == "inspect" {
		return m.queryActivity(command.action, command.handle, false)
	}
	handle, generation, err := splitAgentTarget(command.handle)
	if err != nil {
		m.append("Agent control", err.Error())
		return nil
	}
	if generation == 0 {
		for _, agent := range m.activity.Snapshot.Agents {
			if agent.Handle == handle {
				generation = uint64(agent.Generation)
				break
			}
		}
	}
	plan := toolsubagent.ControlPlan{Action: command.action, Handle: handle, Generation: generation, Message: command.text}
	if err := plan.Validate(); err != nil {
		m.append("Agent control", err.Error()+" · use /agents list for the exact target")
		return nil
	}
	arguments, err := json.Marshal(struct {
		Action     string `json:"action"`
		Handle     string `json:"handle"`
		Generation uint64 `json:"generation"`
		Message    string `json:"message,omitempty"`
	}{plan.Action, plan.Handle, plan.Generation, plan.Message})
	if err != nil {
		m.append("Agent control", err.Error())
		return nil
	}
	id := inbox.ID(uuid.New().String())
	// This is durable ingress, not a direct hub dispatch or session-store write.
	// Coordinator decodes the structural tool call and persists its operations
	// before any target side effect; no LLM response is required.
	payload, err := json.Marshal(inbox.ControlMessage{Mode: inbox.InvokeTool, Parameters: inbox.ToolInvocation{Call: llm.ToolCall{CallID: string(id), Name: tool.AgentControlName, Arguments: string(arguments)}}})
	if m.pendingSubmissions == nil {
		m.pendingSubmissions = make(map[inbox.ID]struct{})
	}
	m.pendingSubmissions[id] = struct{}{}
	m.sending = true
	m.lines = append(m.lines, message{label: "Agent control", text: fmt.Sprintf("%s requested for %s · admission only; not consumption or drain", command.action, command.handle), inputID: id, submission: submissionSending})
	m.renderConversation()
	m.conversation.GotoBottom()
	if m.submissionQueue == nil {
		m.submissionQueue = newSubmissionQueue(m.ctx)
	}
	result := m.submissionQueue.enqueue(inbox.Input{ID: id, Kind: inbox.InputControl, Payload: payload}, text, m.inputs, err)
	return func() tea.Msg { return <-result }
}

func (m model) activitySummary() string {
	if m.activity.Generation == 0 || m.activity.Generation < m.workerGeneration {
		return ""
	}
	agents := append([]subagent.AgentActivity(nil), m.activity.Snapshot.Agents...)
	// Cancellation/drain is most urgent, then report-with-owned-work, then oldest
	// active agent; stable handles break ties deterministically.
	slices.SortFunc(agents, func(a, b subagent.AgentActivity) int {
		ra, rb := agentDiagnosticRank(a), agentDiagnosticRank(b)
		if ra != rb {
			return ra - rb
		}
		if !a.AttemptedAt.Equal(b.AttemptedAt) {
			if a.AttemptedAt.Before(b.AttemptedAt) {
				return -1
			}
			return 1
		}
		return strings.Compare(a.Handle, b.Handle)
	})
	for _, agent := range agents {
		if !agent.OwnerTerminalEnqueued && !agent.StartFailed {
			summary := childActivityLine(agent.ChildActivity, time.Now())
			if m.activityError != "" {
				summary += " · live refresh unavailable"
			}
			return summary + " · /agents"
		}
	}
	if m.activity.Resources != nil {
		for _, op := range m.activity.Resources.Operations {
			if !op.Owned {
				continue
			}
			if op.Phase == operation.ActivityWarning || op.Phase == operation.ActivityCancellationRequested || op.Phase == operation.ActivityDraining {
				return fmt.Sprintf("%s · %s · %s · ownership retained", op.Type, strings.ReplaceAll(string(op.Phase), "_", " "), activityAge(time.Now(), op.StartedAt))
			}
		}
	}
	if m.activity.Snapshot.OmittedAgents > 0 {
		return fmt.Sprintf("Activity incomplete · %d omitted · /agents", m.activity.Snapshot.OmittedAgents)
	}
	return ""
}
func agentDiagnosticRank(a subagent.AgentActivity) int {
	if a.CancellationRequested || a.Phase == "draining" {
		return 0
	}
	if a.ReportAvailable {
		return 1
	}
	return 2
}
