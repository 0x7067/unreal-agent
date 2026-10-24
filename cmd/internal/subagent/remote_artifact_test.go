//go:build darwin || linux || freebsd || openbsd || netbsd || dragonfly

package subagent

import (
	"encoding/json/v2"
	"os"
	"strings"
	"testing"

	"github.com/unreallabsai/unreal-agent/harness/operation"
	toolsubagent "github.com/unreallabsai/unreal-agent/harness/tool/subagent"
)

// Exercise the actual supervisor finish/update path without spawning a child or
// contacting an LLM. Both successful replies and failures must remain recoverable.
func TestSupervisorTerminalArtifacts(t *testing.T) {
	for _, failure := range []bool{false, true} {
		spec, e := operation.NewRemoteJobSpec(operation.RemoteJobPlan{Type: toolsubagent.RunPlanType, Version: toolsubagent.PlanVersion, Data: []byte(`{}`)})
		if e != nil {
			t.Fatal(e)
		}
		var remote operation.RemoteJobState
		if e = json.Unmarshal(spec.State, &remote); e != nil {
			t.Fatal(e)
		}
		remote.OutputDirectory = t.TempDir()
		spec.State, e = json.Marshal(remote)
		if e != nil {
			t.Fatal(e)
		}
		op := operation.Operation{ID: "child-reply", Type: spec.Type, Version: spec.Version, State: spec.State, MaxOutputLength: 10}
		full := strings.Repeat("child full reply\n", 100)
		s := &state{}
		if failure {
			s.finish(op, operation.StatusFailed, "", full)
		} else {
			s.finish(op, operation.StatusCompleted, full, "")
		}
		if len(s.runQueue) != 1 {
			t.Fatal("supervisor did not emit terminal result")
		}
		remote, e = operation.DecodeRemoteJobState(s.runQueue[0])
		if e != nil {
			t.Fatal(e)
		}
		path := remote.ResultPath
		if failure {
			path = remote.ErrorPath
		}
		b, e := os.ReadFile(path)
		if e != nil || string(b) != full {
			t.Fatalf("supervisor full output: %v", e)
		}
	}
}
