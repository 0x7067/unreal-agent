package main

import (
	"fmt"
	"strings"
)

const agentsUsage = "/agents list · /agents inspect HANDLE · /agents message HANDLE TEXT · /agents stop HANDLE · /builds"

type agentCommand struct{ action, handle, text string }

// Names are deliberately not accepted as aliases: only the opaque scoped handle
// returned by the live snapshot authorizes a target, including its generation.
func parseAgentCommand(text string) (agentCommand, error) {
	rest := strings.TrimSpace(strings.TrimPrefix(strings.TrimSpace(text), "/agents"))
	if rest == "" {
		return agentCommand{action: "list"}, nil
	}
	action, rest, _ := strings.Cut(rest, " ")
	rest = strings.TrimSpace(rest)
	switch action {
	case "list":
		if rest != "" {
			return agentCommand{}, fmt.Errorf("%s", agentsUsage)
		}
		return agentCommand{action: action}, nil
	case "inspect", "stop":
		if rest == "" || len(strings.Fields(rest)) != 1 {
			return agentCommand{}, fmt.Errorf("%s", agentsUsage)
		}
		return agentCommand{action: action, handle: rest}, nil
	case "message":
		handle, body, _ := strings.Cut(rest, " ")
		body = strings.TrimSpace(body)
		if handle == "" || body == "" {
			return agentCommand{}, fmt.Errorf("%s", agentsUsage)
		}
		if len(body) > 16000 {
			return agentCommand{}, fmt.Errorf("agent message exceeds 16000 bytes")
		}
		return agentCommand{action: action, handle: handle, text: body}, nil
	default:
		return agentCommand{}, fmt.Errorf("%s", agentsUsage)
	}
}
