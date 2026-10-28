package resident

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerruntime"
)

// The test executable is a thin fixture around the real worker runtime, not a
// fake worker. Probe intentionally does not load credentials or provider state.
func TestMain(m *testing.M) {
	identity := workerproto.Identity{Protocol: workerproto.Version, SessionVersion: workerproto.SessionVersion, Build: "fixture"}
	if len(os.Args) == 2 {
		var err error
		switch os.Args[1] {
		case "worker-probe":
			err = workerruntime.Probe(os.Stdout, identity)
		case "worker":
			// Private test-only fault injection. Probe deliberately never touches
			// this directory, and all other fixtures retain the default path.
			root := os.Getenv("UA_RESIDENT_FAILURE_TEST_ROOT")
			gated := false
			if root != "" {
				if _, e := os.Stat(filepath.Join(root, "startup-fail")); e == nil {
					joined, _ := os.ReadFile(filepath.Join(root, "host-wait"))
					if e := os.WriteFile(filepath.Join(root, "candidate-attempt"), append([]byte("host-wait="), joined...), 0600); e != nil {
						os.Exit(24)
					}
					os.Exit(23)
				}
				_, e := os.Stat(filepath.Join(root, "retired"))
				gated = os.IsNotExist(e)
			}
			err = workerruntime.Run(context.Background(), os.Stdin, os.Stdout, os.Stderr, func(key string) string {
				switch key {
				case "OPENAI_API_KEY", "UNREAL_HARNESS_LLM_API_KEY":
					return "fake-loopback-only"
				}
				return ""
			}, identity)
			if gated {
				// Run has actually returned: worker leases and managers joined.
				if e := os.WriteFile(filepath.Join(root, "retired"), []byte(fmt.Sprint(os.Getpid())), 0600); e != nil {
					os.Exit(24)
				}
				deadline := time.Now().Add(15 * time.Second)
				for {
					if _, e := os.Stat(filepath.Join(root, "allow-exit")); e == nil {
						break
					}
					if time.Now().After(deadline) {
						os.Exit(25)
					}
					time.Sleep(5 * time.Millisecond)
				}
			}
		default:
			os.Exit(m.Run())
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		os.Exit(0)
	}
	os.Exit(m.Run())
}
