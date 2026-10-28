package residentcli

import (
	"context"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachclient"
	"github.com/unreallabsai/unreal-agent/cmd/internal/attachproto"
	"github.com/unreallabsai/unreal-agent/harness/session"
)

// retireHost is an explicit exclusive control client, never a process supervisor.
// Ordinary attached clients must detach first. The original trusted supervisor
// must still stop/actually wait the old host before launching a replacement.
func retireHost(ctx context.Context, args []string, output, stderr io.Writer) error {
	flags := flag.NewFlagSet("host-retire", flag.ContinueOnError)
	flags.SetOutput(stderr)
	runtime := flags.String("runtime-directory", "", "existing private runtime directory")
	state := flags.String("session-directory", "", "existing private session directory")
	sid := flags.String("session", "", "stable session identity")
	incarnation := flags.String("expected-incarnation", "", "required exact old host incarnation from its owned startup/hello")
	generation := flags.Uint64("expected-generation", 0, "required exact current worker generation")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 || *runtime == "" || *state == "" || *sid == "" || *incarnation == "" || len(*incarnation) > 128 || *generation == 0 {
		return errors.New("host-retire requires explicit directories, session, expected incarnation and nonzero generation")
	}
	control, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	c, err := attachclient.Dial(control, *runtime, *state, session.ID(*sid))
	if err != nil {
		return err
	}
	defer c.Close()
	snapshot := c.Snapshot()
	if snapshot.Hello.Incarnation != *incarnation || snapshot.Header.Status == nil || snapshot.Header.Status.Generation != *generation {
		return errors.New("host-retire owner/generation mismatch; no retirement requested")
	}
	done := make(chan struct{})
	go func() {
		defer close(done)
		for {
			if _, e := c.Next(); e != nil {
				return
			}
		}
	}()
	defer func() { _ = c.Close(); <-done }()
	status, err := c.RuntimeControl(control, attachproto.RuntimeRequest{Version: 1, Action: "retire-host", ExpectedGeneration: *generation})
	if status.Version != 0 {
		data, e := json.Marshal(status)
		if e != nil {
			return e
		}
		if _, e = fmt.Fprintln(output, string(data)); e != nil {
			return e
		}
	}
	if err != nil {
		return err
	}
	if status.State != "drained" {
		return errors.New("host retirement unconfirmed; do not launch replacement")
	}
	return nil
}
