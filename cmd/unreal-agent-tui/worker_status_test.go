package main

import (
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"testing"

	"github.com/charmbracelet/x/ansi"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
)

func TestWorkerHandoffStatusRenderingEventOrders(t *testing.T) {
	handoff := workerhost.Status{Message: "Old worker exited; starting replacement", Stopped: true, Err: nil}
	ready := workerhost.Status{Generation: 2, Message: "Worker ready"}
	for _, palette := range []string{"github-dark", "turbo-vision"} {
		for _, order := range []struct {
			name   string
			events []any
		}{
			{"finished-handoff-ready", []any{reloadFinished{}, handoff, ready}},
			{"handoff-finished-ready", []any{handoff, reloadFinished{}, ready}},
			{"handoff-ready-finished", []any{handoff, ready, reloadFinished{}}},
		} {
			t.Run(palette+"/"+order.name, func(t *testing.T) {
				m := toolsModel(t, palette, 160, 40, 0)
				m = submissionUpdate(m, workerhost.Status{Generation: 1, Message: "Worker ready"})
				if m.startReload() == nil || !m.reloading {
					t.Fatal("reload did not start")
				}
				// Commands are deliberately not run: vary the ordering of the
				// command result and host status events without process timing.
				finished := false
				message := "Worker ready"
				for i, event := range order.events {
					m = submissionUpdate(m, event)
					switch event := event.(type) {
					case reloadFinished:
						finished = true
					case workerhost.Status:
						message = event.Message
					}
					if m.reloading == finished || m.reloadError != "" || m.ended {
						t.Fatalf("event %d: reloading=%v finished=%v reloadError=%q ended=%v", i, m.reloading, finished, m.reloadError, m.ended)
					}
					if finished {
						// In particular, a successful stopped handoff exposed after
						// reload completion must not inherit the error foreground.
						assertWorkerStatusForeground(t, m, message, m.theme.StatusForeground)
					} else {
						assertWorkerStatusForeground(t, m, "Reloading worker · checking candidate and idle boundary", m.theme.StatusAccent)
					}
				}
				if m.workerStopped || m.workerStatusError || m.workerGeneration != 2 {
					t.Fatalf("ready did not reset lifecycle state: stopped=%v error=%v generation=%d", m.workerStopped, m.workerStatusError, m.workerGeneration)
				}
				assertWorkerStatusForeground(t, m, ready.Message, m.theme.StatusForeground)
			})
		}
	}
}

func TestWorkerStatusErrorRenderingAndGenerationReset(t *testing.T) {
	for _, palette := range []string{"github-dark", "turbo-vision"} {
		for _, stopped := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/stopped=%v", palette, stopped), func(t *testing.T) {
				m := toolsModel(t, palette, 160, 40, 0)
				failure := workerhost.Status{Generation: 1, Message: "Worker failed", Stopped: stopped, Err: errors.New("synthetic worker failure")}
				m = submissionUpdate(m, failure)
				assertWorkerStatusForeground(t, m, "Worker failed · synthetic worker failure", m.theme.StatusError)

				m = submissionUpdate(m, workerhost.Status{Generation: 2, Message: "Worker ready"})
				if m.workerStatusError || m.workerStopped || m.workerGeneration != 2 {
					t.Fatal("new ready status did not clear the old error")
				}
				assertWorkerStatusForeground(t, m, "Worker ready", m.theme.StatusForeground)

				m = submissionUpdate(m, workerhost.Status{Generation: 1, Message: "Late old worker failure", Stopped: true, Err: errors.New("stale error")})
				if m.workerStatus != "Worker ready" || m.workerStatusError || m.workerStopped || m.workerGeneration != 2 {
					t.Fatal("stale old-generation error changed newer ready status")
				}
				assertWorkerStatusForeground(t, m, "Worker ready", m.theme.StatusForeground)
				if strings.Contains(ansi.Strip(m.View().Content), "stale error") {
					t.Fatal("stale error leaked into the rendered view")
				}
			})
		}
	}
}

func TestReloadFinishedErrorStatusRendering(t *testing.T) {
	for _, palette := range []string{"github-dark", "turbo-vision"} {
		t.Run(palette, func(t *testing.T) {
			m := toolsModel(t, palette, 160, 40, 0)
			m = submissionUpdate(m, workerhost.Status{Generation: 1, Message: "Worker ready"})
			if m.startReload() == nil {
				t.Fatal("reload did not start")
			}
			m = submissionUpdate(m, reloadFinished{err: errors.New("synthetic reload rejection")})
			if m.reloading || m.reloadError != "synthetic reload rejection" {
				t.Fatalf("reload completion state: reloading=%v error=%q", m.reloading, m.reloadError)
			}
			assertWorkerStatusForeground(t, m, "Reload failed · synthetic reload rejection", m.theme.StatusError)
		})
	}
}

// Inspect foregrounds in the actual View, not merely the model's error flag or
// the presence of an error-colored escape elsewhere on the screen. The probe
// also requires explicit, distinct colors: stripped/color-disabled output must
// fail rather than turn both expected styles into an equal empty string.
func assertWorkerStatusForeground(t *testing.T, m model, message, color string) {
	t.Helper()
	probe := "foreground-probe"
	normal := workerStatusForegrounds(t, textStyle(m.theme.StatusForeground).Render(probe), probe)[0]
	errorColor := workerStatusForegrounds(t, textStyle(m.theme.StatusError).Render(probe), probe)[0]
	if normal == "" || errorColor == "" || normal == errorColor {
		t.Fatalf("normal/error style probes must have distinct explicit foregrounds: normal=%q error=%q", normal, errorColor)
	}
	want := workerStatusForegrounds(t, textStyle(color).Render(probe), probe)[0]
	if want == "" {
		t.Fatal("expected style has no explicit foreground")
	}
	for i, got := range workerStatusForegrounds(t, m.View().Content, message) {
		if got != want {
			t.Fatalf("status %q byte %d foreground=%q, want %q (%s)", message, i, got, want, color)
		}
	}
}

var workerStatusSGR = regexp.MustCompile(`\x1b\[[0-9;]*m`)

// Track the semicolon SGR foreground forms emitted by the renderer, including
// resets and combined foreground/background sequences. Keep one foreground per
// byte so assertions cover the entire message, even across nested style resets.
func workerStatusForegrounds(t *testing.T, rendered, message string) []string {
	t.Helper()
	var plain strings.Builder
	var foregrounds []string
	foreground := ""
	appendText := func(text string) {
		plain.WriteString(text)
		for range len(text) {
			foregrounds = append(foregrounds, foreground)
		}
	}
	end := 0
	for _, loc := range workerStatusSGR.FindAllStringIndex(rendered, -1) {
		appendText(rendered[end:loc[0]])
		params := strings.Split(rendered[loc[0]+2:loc[1]-1], ";")
		for i := 0; i < len(params); i++ {
			code := 0 // An empty SGR parameter means reset.
			if params[i] != "" {
				var err error
				code, err = strconv.Atoi(params[i])
				if err != nil {
					t.Fatal(err)
				}
			}
			switch {
			case code == 0 || code == 39:
				foreground = ""
			case code >= 30 && code <= 37 || code >= 90 && code <= 97:
				foreground = params[i]
			case code == 38 || code == 48 || code == 58:
				// Consume background/underline colors too, so an RGB component
				// of 0 or 39 cannot accidentally reset the foreground.
				count := 0
				if i+1 < len(params) {
					switch params[i+1] {
					case "2":
						count = 4
					case "5":
						count = 2
					}
				}
				if count == 0 || i+count >= len(params) {
					t.Fatalf("unsupported color SGR %q", rendered[loc[0]:loc[1]])
				}
				if code == 38 {
					foreground = strings.Join(params[i:i+count+1], ";")
				}
				i += count
			}
		}
		end = loc[1]
	}
	appendText(rendered[end:])
	text := plain.String()
	if message == "" || strings.Count(text, message) != 1 {
		t.Fatalf("expected exactly one visible status %q in:\n%s", message, ansi.Strip(rendered))
	}
	start := strings.Index(text, message)
	return foregrounds[start : start+len(message)]
}
