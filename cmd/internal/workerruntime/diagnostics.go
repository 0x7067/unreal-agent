package workerruntime

import (
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/activitysummary"
	"github.com/unreallabsai/unreal-agent/cmd/internal/subagent"
	"github.com/unreallabsai/unreal-agent/harness/operation"
)

func (o *ownedExecution) summary() string {
	var fleet *subagent.FleetSnapshot
	if o.hub != nil {
		s := o.hub.Snapshot()
		fleet = &s
	}
	var activity *operation.ActivitySnapshot
	if o.account != nil {
		activity = &operation.ActivitySnapshot{Account: o.account.Snapshot()}
	}
	return activitysummary.Format(time.Now(), fleet, activity)
}
