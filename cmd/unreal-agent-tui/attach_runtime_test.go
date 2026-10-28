package main

import (
	"context"
	"image"
	"reflect"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachclient"
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
)

func TestAttachedRuntimeStatusMapping(t *testing.T) {
	s := attachproto.RuntimeStatus{Version: 1, Generation: 7, RunningBuild: "running", CandidateBuild: "candidate", State: "failed", Message: "message", Error: "failure"}
	got := runtimeWorkerStatus(s)
	if got.Generation != 7 || got.Build != "running" || got.InstalledBuild != "candidate" || got.ReloadState != "failed" || got.Message != "message" || got.Stopped || got.Err == nil || got.Err.Error() != "failure" {
		t.Fatalf("mapping %+v", got)
	}
	s.State = "stopped"
	s.Error = ""
	got = runtimeWorkerStatus(s)
	if !got.Stopped || got.Err != nil {
		t.Fatalf("stopped mapping %+v", got)
	}
}

func TestAttachedRuntimeWriterGenerationAndCandidateRefusal(t *testing.T) {
	w := attachedWriter(nil, attachclient.Snapshot{Header: attachproto.Event{Status: &attachproto.Status{Generation: 4}}}).(*attachedRuntimeWriter)
	for _, g := range []uint64{2, 0, 4, 9, 3} {
		w.advance(g)
	}
	if w.generation.Load() != 9 {
		t.Fatal("generation regressed", w.generation.Load())
	}
	// A nil client makes any accidental transport call fail immediately.
	if err := w.Reload(context.Background(), []string{"untrusted"}); err == nil {
		t.Fatal("frontend selected executable")
	}
}

func TestAttachedRuntimeMetadataPreservesLocalViewAndFencesLifecycle(t *testing.T) {
	m, _ := lifecycleTestModel(t)
	w := &attachedRuntimeWriter{}
	w.generation.Store(1)
	m.inputs = w
	for range 60 {
		m.append("Agent", strings.Repeat("transcript ", 20))
	}
	m.composer.SetValue("unsent draft")
	m.conversation.SetYOffset(7)
	m.selection.capture(m.conversation)
	m.selection.anchor = image.Pt(1, 1)
	m.selection.head = image.Pt(5, 2)
	beforeLines := append([]message(nil), m.lines...)
	beforeSelection := m.selection
	offset := m.conversation.YOffset()
	lifecycle := m.attachedLifecycle()
	scope := lifecycle.scope
	m.applyAttachedEvent(attachproto.Event{Type: "runtime", Runtime: &attachproto.RuntimeStatus{Version: 1, Generation: 2, RunningBuild: "new-running", CandidateBuild: "next-candidate", State: "completed", Message: "ready"}})
	if m.workerGeneration != 2 || m.workerBuild != "new-running" || m.installedBuild != "next-candidate" || m.reloadState != "completed" || w.generation.Load() != 2 {
		t.Fatalf("runtime metadata missing: generation=%d build=%q candidate=%q state=%q", m.workerGeneration, m.workerBuild, m.installedBuild, m.reloadState)
	}
	if m.composer.Value() != "unsent draft" || m.conversation.YOffset() != offset || !reflect.DeepEqual(m.lines, beforeLines) || !reflect.DeepEqual(m.selection, beforeSelection) {
		t.Fatal("metadata changed draft, scroll, transcript or selection")
	}
	if lifecycle.scope != scope || lifecycle.generation != 1 {
		t.Fatal("runtime notification rebound lifecycle authority")
	}
	if handled, cmd := m.startAttachedLifecycle("/shutdown"); !handled || cmd != nil {
		t.Fatal("new generation lifecycle was not reconnect-fenced")
	}
	m.applyAttachedEvent(attachproto.Event{Type: "runtime", Runtime: &attachproto.RuntimeStatus{Version: 1, Generation: 1, RunningBuild: "old", CandidateBuild: "old-candidate", State: "failed", Message: "old", Error: "stale"}})
	if m.workerGeneration != 2 || m.workerBuild != "new-running" || m.installedBuild != "next-candidate" || m.reloadState != "completed" || m.workerStatusError || w.generation.Load() != 2 {
		t.Fatal("stale runtime reply rolled state backwards")
	}
	m.applyAttachedEvent(attachproto.Event{Type: "status", Status: &attachproto.Status{Generation: 3, Build: "third"}})
	if w.generation.Load() != 3 || m.workerGeneration != 3 {
		t.Fatal("ordinary status did not advance runtime writer")
	}
}

func TestAttachedRuntimeCommandsPassOnlyAdvertisedConnectedBridge(t *testing.T) {
	for _, command := range []string{"/builds", "/reload"} {
		m, _ := lifecycleTestModel(t)
		m.inputs = &attachedRuntimeWriter{enabled: true}
		if cmd := m.startSubmission(command); cmd == nil {
			t.Fatalf("%s was blocked by legacy attach guard", command)
		}
	}
	for _, command := range []string{"/reload /untrusted/path", "/builds /extra", "/compact"} {
		m, _ := lifecycleTestModel(t)
		m.inputs = &attachedRuntimeWriter{enabled: true}
		m.composer.SetValue(command)
		if cmd := m.startSubmission(command); cmd != nil {
			t.Fatalf("unexpected attached host command %s", command)
		}
		if m.composer.Value() != command {
			t.Fatal("refused command lost draft")
		}
	}
	for _, connected := range []bool{true, false} {
		m, _ := lifecycleTestModel(t)
		m.inputs = &attachedRuntimeWriter{}
		m.attachConnected = connected
		if cmd := m.startSubmission("/reload"); cmd != nil {
			t.Fatal("unadvertised capability passed attach guard")
		}
	}
}

func TestAttachedUpgradeTracksOnlyOriginalRequestUntilSettled(t *testing.T) {
	w := &attachedRuntimeWriter{}
	w.trackRuntime(attachproto.Event{ID: "original", Runtime: &attachproto.RuntimeStatus{State: "queued"}})
	w.trackRuntime(attachproto.Event{ID: "cancel-reply", Runtime: &attachproto.RuntimeStatus{State: "cancel_requested"}})
	if w.pendingID != "original" {
		t.Fatal("cancel admission cleared original pending operation")
	}
	w.trackRuntime(attachproto.Event{ID: "original", Runtime: &attachproto.RuntimeStatus{State: "canceled"}})
	if w.pendingID != "" {
		t.Fatal("settled original remained pending")
	}
	for _, connected := range []bool{true, false} {
		m, _ := lifecycleTestModel(t)
		m.inputs = &attachedRuntimeWriter{}
		m.attachConnected = connected
		m.composer.SetValue("draft")
		if m.startAttachedUpgrade(false) != nil || m.startAttachedUpgrade(true) != nil {
			t.Fatal("unsupported upgrade sent")
		}
		if m.composer.Value() != "draft" {
			t.Fatal("unsupported upgrade lost draft")
		}
	}
	m, _ := lifecycleTestModel(t)
	m.inputs = &attachedRuntimeWriter{drainEnabled: true}
	m.attachConnected = true
	if m.startSubmission("/upgrade") == nil || !m.reloading {
		t.Fatal("advertised upgrade not admitted locally")
	}
}
