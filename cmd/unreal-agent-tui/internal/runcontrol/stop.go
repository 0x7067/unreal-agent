package runcontrol

import (
	"context"
	"encoding/json/v2"
	"fmt"
	"uuid"

	"github.com/unreallabsai/unreal-agent/harness/inbox"
)

func Stop(ctx context.Context, inputs inbox.Writer, done <-chan error, reason string) error {
	select {
	case err := <-done:
		return err
	default:
	}
	payload, err := json.Marshal(inbox.ControlMessage{Mode: inbox.StopHard, Reason: reason})
	if err != nil {
		return err
	}
	if err := inputs.Submit(ctx, inbox.Input{ID: inbox.ID(uuid.New().String()), Kind: inbox.InputControl, Payload: payload}); err != nil {
		return fmt.Errorf("request session stop: %w", err)
	}
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		return fmt.Errorf("wait for session stop: %w", context.Cause(ctx))
	}
}
