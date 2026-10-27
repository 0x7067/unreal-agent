package main

import (
	tea "charm.land/bubbletea/v2"
	"context"
	"fmt"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"time"
)

type buildInspector interface {
	CheckInstalled(context.Context) (workerhost.Status, error)
}
type buildsFinished struct {
	status workerhost.Status
	err    error
}

func (m *model) startBuildInspection() tea.Cmd {
	m.composer.Reset()
	writer, ctx := m.inputs, m.ctx
	return func() tea.Msg {
		inspector, ok := writer.(buildInspector)
		if !ok {
			return buildsFinished{err: fmt.Errorf("installed candidate inspection unavailable")}
		}
		ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
		defer cancel()
		status, err := inspector.CheckInstalled(ctx)
		return buildsFinished{status: status, err: err}
	}
}

func (m *model) finishBuildInspection(msg buildsFinished) {
	if msg.err != nil {
		m.reloadState = "candidate inspection unavailable; running worker unchanged"
		m.append("Builds", "Installed candidate unavailable: "+msg.err.Error())
		return
	}
	m.installedBuild, m.reloadState = singleLine(msg.status.InstalledBuild), singleLine(msg.status.ReloadState)
	m.append("Builds", fmt.Sprintf("Frontend executing: %s · %s · %s\nWorker executing: %s\nInstalled candidate (checked now): %s\n%s\n/reload changes worker only at an idle boundary; frontend changes require restart.", version, commit, date, m.workerBuild, m.installedBuild, m.reloadState))
}
