package main

import (
	"encoding/json/v2"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	graph "github.com/unreallabsai/unreal-agent/harness/taskgraph"
)

// Diagnostics project typed persisted observations only. No filesystem, process
// scraping, or interpretation of arbitrary result text runs on the UI thread.
func toolDiagnostic(data sessionstore.ToolCallStatus) string {
	for _, op := range data.Operations {
		if terminalOperation(op.Status) {
			continue
		}
		switch op.Type {
		case operation.TypeShell:
			state, err := operation.DecodeShellState(op)
			if err != nil {
				continue
			}
			phase := string(state.Phase)
			if op.Status == operation.StatusCanceling {
				phase = "stopping; process/output drain pending"
			}
			if phase == "" {
				phase = "queued; process start unconfirmed"
			}
			return "shell → " + phase + " · " + safeExecutableSummary(state.Input.Command)
		case operation.TypeRemoteJob:
			state, err := operation.DecodeRemoteJobState(op)
			if err != nil || state.Plan.Type != graph.RunPlanType {
				continue
			}
			// Handle is a runtime checkpoint. Deliberately decode only safe identities,
			// enums, phase timing and dispatch evidence, never task commands/prompts.
			var checkpoint struct {
				Graph struct {
					Name  string `json:"name"`
					Nodes []struct {
						Task struct {
							ID string `json:"id"`
						} `json:"task"`
						Generation uint64 `json:"generation"`
						Status     string `json:"status"`
					} `json:"nodes"`
				} `json:"graph"`
				Attempts map[string]struct {
					Generation        uint64    `json:"generation"`
					Phase             string    `json:"phase"`
					PhaseStartedAt    time.Time `json:"phase_started_at"`
					CommandDispatched bool      `json:"command_dispatched"`
				} `json:"attempts"`
				Canceling bool `json:"canceling"`
			}
			if json.Unmarshal(state.Handle, &checkpoint) != nil {
				continue
			}
			if checkpoint.Canceling {
				return "Stopping graph · waiting for scans and owned execution drain"
			}
			slices.SortFunc(checkpoint.Graph.Nodes, func(a, b struct {
				Task struct {
					ID string `json:"id"`
				} `json:"task"`
				Generation uint64 `json:"generation"`
				Status     string `json:"status"`
			}) int {
				if rankA, rankB := graphDiagnosticRank(a.Status), graphDiagnosticRank(b.Status); rankA != rankB {
					return rankA - rankB
				}
				return strings.Compare(a.Task.ID, b.Task.ID)
			})
			for _, node := range checkpoint.Graph.Nodes {
				if node.Status == graph.Accepted || node.Status == graph.Canceled {
					continue
				}
				attempt := checkpoint.Attempts[node.Task.ID]
				phase := strings.ReplaceAll(attempt.Phase, "_", " ")
				if phase == "" {
					phase = node.Status
				}
				dispatch := "command start unconfirmed"
				if !attempt.CommandDispatched {
					dispatch = "command not dispatched"
				} else {
					dispatch = "command dispatched (start not proven)"
				}
				return fmt.Sprintf("Graph %s/%s g%d · %s · %s · %s", singleLine(checkpoint.Graph.Name), singleLine(node.Task.ID), node.Generation, phase, activityAge(time.Now(), attempt.PhaseStartedAt), dispatch)
			}
			return "Graph " + singleLine(checkpoint.Graph.Name) + " · awaiting current-generation acceptance"
		}
	}
	return ""
}
func terminalOperation(status operation.Status) bool {
	return status == operation.StatusCompleted || status == operation.StatusFailed || status == operation.StatusCanceled || status == operation.StatusUnsupported
}

func graphDiagnosticRank(status string) int {
	switch status {
	case graph.Running, graph.Canceling:
		return 0
	case graph.Failed:
		return 1
	case graph.Completed:
		return 2
	case "ready":
		return 3
	default:
		return 4
	}
}
