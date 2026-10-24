//go:build darwin || linux || freebsd || openbsd || netbsd || dragonfly

package main

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/harness/operation"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

type artifactHandler struct {
	updates chan operation.Operation
	full    string
}

func (h *artifactHandler) RemoteJobPlanType() operation.RemoteJobPlanType {
	return toolsubagent.RunPlanType
}
func (h *artifactHandler) RemoteJobPlanVersion() operation.RemoteJobPlanVersion {
	return toolsubagent.PlanVersion
}
func (h *artifactHandler) RemoteJobUpdates() <-chan operation.Operation { return h.updates }
func (h *artifactHandler) CancelRemoteJob(operation.ID, string) error   { return nil }
func (h *artifactHandler) AddRemoteJob(op operation.Operation) error {
	s, e := operation.DecodeRemoteJobState(op)
	if e != nil {
		return e
	}
	s.TerminalResult = h.full
	step, e := operation.UpdateRemoteJob(op, s, operation.StatusCompleted)
	if e != nil {
		return e
	}
	h.updates <- *step.Operation
	return nil
}

func TestTaskOperationsRemoteArtifactDefaults(t *testing.T) {
	base := t.TempDir()
	full := strings.Repeat("TUI full agent result", 100)
	_, m, e := newTaskOperations(t.Context(), t.TempDir(), base, "/bin/sh", nil, false, func(context.Context) ([]operation.RemoteJobHandler, error) {
		return []operation.RemoteJobHandler{&artifactHandler{updates: make(chan operation.Operation, 1), full: full}}, nil
	})
	if e != nil {
		t.Fatal(e)
	}
	spec, e := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: toolsubagent.RunPlanType, Version: toolsubagent.PlanVersion, Data: []byte(`{}`)})
	if e != nil {
		t.Fatal(e)
	}
	op := operation.Operation{ID: "tui-agent", Type: spec.Type, Version: spec.Version, State: spec.State, Status: operation.StatusReady, MaxOutputLength: 12}
	if e = m.Add(op); e != nil {
		t.Fatal(e)
	}
	select {
	case op = <-m.Updates():
	case <-time.After(5 * time.Second):
		t.Fatal("no remote result")
	}
	s, e := operation.DecodeRemoteJobState(op)
	if e != nil {
		t.Fatal(e)
	}
	b, e := os.ReadFile(s.ResultPath)
	if e != nil || string(b) != full {
		t.Fatalf("TUI default artifact: %v %#v", e, s)
	}
}
