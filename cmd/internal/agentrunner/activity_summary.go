package agentrunner

import (
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/activitysummary"
	"github.com/unreallabsai/unreal-agent/cmd/internal/subagent"
	"github.com/unreallabsai/unreal-agent/harness/operation"
)

func runnerActivitySummary(hub *subagent.Hub, manager *operation.LocalOperationManager) string {
	var fleet *subagent.FleetSnapshot
	if hub != nil {
		s := hub.Snapshot()
		fleet = &s
	}
	var activity *operation.ActivitySnapshot
	if manager != nil {
		s := manager.ActivitySnapshot()
		activity = &s
	}
	return activitysummary.Format(time.Now(), fleet, activity)
}
