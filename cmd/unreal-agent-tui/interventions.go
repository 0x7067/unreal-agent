package main

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"strconv"
	"strings"
	"uuid"

	tea "charm.land/bubbletea/v2"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

type interventionSubmitter interface {
	ControlScope() string
	SubmitIntervention(context.Context, inbox.Input) error
}

type interventionInputWriter struct{ interventionSubmitter }

func (w interventionInputWriter) Submit(ctx context.Context, in inbox.Input) error {
	return w.SubmitIntervention(ctx, in)
}

// Controls are durably admitted through the host, not dispatched directly into
// a graph or Hub. A captured scope never follows a replacement generation.
func (m *model) startStructuralControl(label, command, notice, name, arguments string) tea.Cmd {
	writer := m.inputs
	scope := ""
	if control, ok := writer.(interventionSubmitter); ok {
		scope = control.ControlScope()
		if scope == "" {
			m.append(label, "No live worker generation; nothing submitted.")
			return nil
		}
		writer = interventionInputWriter{control}
	} else if name == tool.TaskGraphName {
		m.append(label, "This connection does not support generation-bound graph controls.")
		return nil
	}
	id := inbox.ID(uuid.New().String())
	payload, err := json.Marshal(inbox.ControlMessage{Mode: inbox.InvokeTool, Parameters: inbox.ToolInvocation{Scope: scope, Call: llm.ToolCall{CallID: string(id), Name: name, Arguments: arguments}}})
	in := inbox.Input{ID: id, Kind: inbox.InputControl, Payload: payload}
	if err == nil && scope != "" {
		err = inbox.ValidateIntervention(in, scope)
	}
	if err != nil {
		m.append(label, err.Error()+" · nothing submitted")
		return nil
	}
	if m.pendingSubmissions == nil {
		m.pendingSubmissions = make(map[inbox.ID]struct{})
	}
	m.pendingSubmissions[id] = struct{}{}
	m.sending = true
	m.lines = append(m.lines, message{label: label, text: notice, inputID: id, submission: submissionSending})
	m.renderConversation()
	m.conversation.GotoBottom()
	if m.submissionQueue == nil {
		m.submissionQueue = newSubmissionQueue(m.ctx)
	}
	result := m.submissionQueue.enqueue(in, command, writer, nil)
	return func() tea.Msg { return <-result }
}

type graphIntervention struct {
	Action     string `json:"action"`
	Name       string `json:"name"`
	TaskID     string `json:"task_id,omitempty"`
	Generation uint64 `json:"generation,omitempty"`
	Evidence   string `json:"evidence,omitempty"`
}

func parseGraphCommand(text string) (graphIntervention, error) {
	const usage = "Use /graph status|cancel NAME or /graph accept|reject NAME TASK GENERATION EVIDENCE. Acceptance requires your own validation."
	var out graphIntervention
	fields := strings.Fields(text)
	if len(text) > 16000 || len(fields) < 3 || fields[0] != "/graph" {
		return out, fmt.Errorf("%s", usage)
	}
	out.Action, out.Name = fields[1], fields[2]
	switch out.Action {
	case "status", "cancel":
		if len(fields) != 3 {
			return out, fmt.Errorf("%s", usage)
		}
	case "accept", "reject":
		if len(fields) < 6 {
			return out, fmt.Errorf("%s", usage)
		}
		out.TaskID = fields[3]
		var err error
		out.Generation, err = strconv.ParseUint(fields[4], 10, 64)
		if err != nil || out.Generation == 0 {
			return out, fmt.Errorf("a positive current task generation is required")
		}
		out.Evidence = strings.Join(fields[5:], " ")
	default:
		return out, fmt.Errorf("%s", usage)
	}
	return out, nil
}

func (m *model) startGraphCommand(text string) tea.Cmd {
	args, err := parseGraphCommand(text)
	if err != nil {
		m.append("Graph control", err.Error())
		return nil
	}
	encoded, err := json.Marshal(args)
	if err != nil {
		m.append("Graph control", err.Error())
		return nil
	}
	m.composer.Reset()
	return m.startStructuralControl("Graph control", text, args.Action+" requested for "+args.Name+" · admission only; inspect the durable result", tool.TaskGraphName, string(encoded))
}
