// Package workerruntime hosts a resident session behind the private worker pipe
// protocol. It never loads frontend preferences, settings files, or dotenv.
package workerruntime

import (
	"context"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
	"uuid"

	"github.com/unreallabsai/unreal-agent/cmd/internal/providers"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/contextbuilder"
	"github.com/unreallabsai/unreal-agent/harness/coordinator"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/harness/tool"
)

// Probe is deliberately independent of configuration, environment, credentials,
// session storage and providers. It emits exactly one hello frame.
func Probe(output io.Writer, identity workerproto.Identity) error {
	if err := identity.Validate(); err != nil {
		return fmt.Errorf("%w: %w", ErrProtocol, err)
	}
	if output == nil {
		return fmt.Errorf("%w: output required", ErrOutput)
	}
	if err := workerproto.Write(output, workerproto.Event{Type: "hello", Identity: &identity}); err != nil {
		return fmt.Errorf("%w: %w", ErrOutput, err)
	}
	return nil
}

// Run owns one worker lifetime and joins its session writers before returning.
// Input and output must not block indefinitely without a way to interrupt them.
// For input, *os.File pipes on Linux/macOS use an interruptible private read
// descriptor when needed; the caller's descriptor stays open and blocking flags
// are restored on return. Pollable pipes use a read deadline. An *io.PipeReader
// transfers ownership of its read end to Run. For any
// other blocking reader (including a file that does not support deadlines), the
// caller MUST close/unblock the pipe on cancellation or retirement. A successful
// quiesce never reads ahead, so it does not require closing stdin to send its ack.
// Run must be the only reader/deadline owner of input. Output must remain writable
// through the final quiesced/error frame; Run never closes output or stderr.
func Run(ctx context.Context, input io.Reader, output, stderr io.Writer, getenv func(string) string, identity workerproto.Identity) error {
	return run(ctx, input, output, stderr, getenv, identity, providers.Default())
}

func run(ctx context.Context, input io.Reader, output, stderr io.Writer, getenv func(string) string, identity workerproto.Identity, available []providers.Provider) error {
	if input == nil || output == nil {
		return fmt.Errorf("%w: input and output required", ErrProtocol)
	}
	if getenv == nil {
		return fmt.Errorf("%w: getenv required", ErrProtocol)
	}
	if stderr == nil {
		stderr = io.Discard
	}
	if err := Probe(output, identity); err != nil {
		return err
	}
	runCtx, cancel := context.WithCancel(ctx)
	defer cancel()
	events := &emitter{output: output, cancel: cancel}
	input, cleanupInput, err := prepareInput(input)
	if err != nil {
		return fmt.Errorf("prepare worker input: %w", err)
	}
	defer cleanupInput()
	reader := newInputPump(input)
	request, err := reader.next(runCtx, nil)
	quiesced := false
	responseID := request.ID
	if err == nil {
		if request.Type != "start" || request.Protocol != workerproto.Version || request.Bootstrap == nil || request.Input != nil {
			err = fmt.Errorf("%w: first request must be start with protocol %d and bootstrap", ErrProtocol, workerproto.Version)
		} else if err = validateBootstrap(*request.Bootstrap); err == nil {
			quiesced, responseID, err = serve(runCtx, reader, events, stderr, getenv, *request.Bootstrap, request.ID, available)
		}
	} else if errors.Is(err, io.EOF) {
		err = fmt.Errorf("%w: missing start frame", ErrProtocol)
	}
	cancel()
	reader.join()
	if outputErr := events.failure(); outputErr != nil {
		return errors.Join(err, outputErr)
	}
	if err != nil {
		_ = events.write(workerproto.Event{Type: "error", ID: responseID, Error: err.Error()})
		return errors.Join(err, events.failure())
	}
	if quiesced {
		return events.write(workerproto.Event{Type: "quiesced", ID: responseID})
	}
	return nil
}

func validateBootstrap(b workerproto.Bootstrap) error {
	if b.SessionID == "" {
		return fmt.Errorf("%w: session ID required", ErrProtocol)
	}
	for _, c := range string(b.SessionID) {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '-') {
			return fmt.Errorf("%w: invalid session ID", ErrProtocol)
		}
	}
	if !filepath.IsAbs(b.Directory) || !filepath.IsAbs(b.Workspace) {
		return fmt.Errorf("%w: session directory and workspace must be absolute", ErrProtocol)
	}
	if strings.TrimSpace(b.Provider) == "" || strings.TrimSpace(b.Model.ID) == "" || b.MaxAttempts < 1 {
		return fmt.Errorf("%w: provider, model and positive max attempts required", ErrProtocol)
	}
	if b.Model.CompactionThreshold < 0 || b.Model.MaxOutputTokens != nil && *b.Model.MaxOutputTokens <= 0 || b.Model.ReasoningEffort != "" && !b.Model.ReasoningEffort.Valid() {
		return fmt.Errorf("%w: invalid model settings", ErrProtocol)
	}
	if b.Shell != "" && !filepath.IsAbs(b.Shell) {
		return fmt.Errorf("%w: shell must be absolute", ErrProtocol)
	}
	if len(b.SubagentCommand) > 0 && !filepath.IsAbs(b.SubagentCommand[0]) {
		return fmt.Errorf("%w: subagent executable must be absolute", ErrProtocol)
	}
	return nil
}

// canonicalDirectory resolves existing prefixes explicitly, including platform
// aliases such as /var on Darwin. The resolved target is still lease-validated.
func canonicalDirectory(path string) (string, error) {
	path = filepath.Clean(path)
	var suffix []string
	for {
		resolved, err := filepath.EvalSymlinks(path)
		if err == nil {
			for i := len(suffix) - 1; i >= 0; i-- {
				resolved = filepath.Join(resolved, suffix[i])
			}
			return resolved, nil
		}
		if !errors.Is(err, fs.ErrNotExist) {
			return "", err
		}
		parent := filepath.Dir(path)
		if parent == path {
			return "", err
		}
		suffix = append(suffix, filepath.Base(path))
		path = parent
	}
}

func serve(ctx context.Context, reader *inputPump, events *emitter, stderr io.Writer, getenv func(string) string, b workerproto.Bootstrap, startID string, available []providers.Provider) (quiesced bool, responseID string, result error) {
	responseID = startID
	directory, err := canonicalDirectory(b.Directory)
	if err != nil {
		return false, responseID, err
	}
	b.Directory = directory
	lease, err := localfile.AcquireLease(directory, b.SessionID)
	if err != nil {
		return false, responseID, err
	}
	defer func() { result = errors.Join(result, lease.Close()) }()
	store, err := localfile.New(directory)
	if err != nil {
		return false, responseID, err
	}
	restored, err := store.Resume(ctx, b.SessionID)
	if errors.Is(err, fs.ErrNotExist) {
		snapshot, createErr := store.Create(ctx, b.SessionID)
		if createErr != nil {
			return false, responseID, createErr
		}
		restored.Snapshot = snapshot
		payload, marshalErr := json.Marshal(inbox.ControlMessage{Mode: inbox.UpdateSettings, Parameters: inbox.Settings{Model: b.Model.ID, CompactionThreshold: new(b.Model.CompactionThreshold), MaxOutputTokens: b.Model.MaxOutputTokens, ReasoningEffort: b.Model.ReasoningEffort}})
		if marshalErr != nil {
			return false, responseID, marshalErr
		}
		initial := inbox.Input{ID: inbox.ID(uuid.New().String()), Kind: inbox.InputControl, Payload: payload}
		if err := store.AppendInput(ctx, b.SessionID, initial); err != nil {
			return false, responseID, err
		}
		restored.InputIDs = []inbox.ID{initial.ID}
	} else if err != nil {
		return false, responseID, err
	}
	for _, op := range restored.Operations {
		if !terminal(op.Status) {
			return false, responseID, fmt.Errorf("refuse resume: operation %q has unresolved status %q", op.ID, op.Status)
		}
	}
	// No writer runs during history transfer. Install the observer only after the
	// last typed history frame, before starting the coordinator: no gaps/duplicates.
	effectiveModel := b.Model
	after := sessionstore.BeforeFirst
	for {
		page, err := store.Items(ctx, b.SessionID, after, 256)
		if err != nil {
			return false, responseID, err
		}
		for _, item := range page.Items {
			if err := events.write(workerproto.Event{Type: "item", Item: &item}); err != nil {
				return false, responseID, err
			}
			replaySettings(&effectiveModel, item)
		}
		if !page.More {
			break
		}
		after = page.NextAfter
	}
	observer := store.AddObserver(func(_ session.ID, item sessionstore.Item) {
		_ = events.write(workerproto.Event{Type: "item", Item: &item})
	})
	defer store.RemoveObserver(observer)
	provider, err := providers.Find(available, b.Provider)
	if err != nil {
		return false, responseID, err
	}
	key, err := provider.APIKey(getenv)
	if err != nil {
		return false, responseID, err
	}
	if b.BaseURL == "" {
		b.BaseURL = provider.BaseURL
	}
	client, err := provider.NewClient(key, b.BaseURL, b.MaxAttempts, getenv)
	if err != nil {
		return false, responseID, err
	}
	calls := newJoinedClient(client)
	defer func() { calls.shutdown(); result = errors.Join(result, client.Close()) }()
	// During setup, caller cancellation must still tear down partial owners.
	// Once the coordinator starts, it owns cancellation: canceling its context
	// first would discard operation updates before their terminal states persist.
	ownerCtx, cancelOwners := context.WithCancel(context.WithoutCancel(ctx))
	defer cancelOwners()
	stopSetupCancellation := context.AfterFunc(ctx, cancelOwners)
	defer stopSetupCancellation()
	inputs, err := inbox.New(ownerCtx, restored.InputIDs)
	if err != nil {
		return false, responseID, err
	}
	// Join even partial operation setup. Once running, the coordinator's
	// graceful-stop defer below runs first, while operation updates are live.
	owners := &ownedExecution{}
	var operations operation.Manager
	defer func() {
		cancelOwners()
		if operations != nil {
			for range operations.Updates() {
			}
		}
		result = errors.Join(result, owners.wait())
		for range inputs.Output() {
		}
	}()
	operationDirectory := filepath.Join(directory, "operations", string(b.SessionID))
	if err := os.MkdirAll(operationDirectory, 0o700); err != nil {
		return false, responseID, err
	}
	if b.Shell == "" {
		b.Shell = "/bin/sh"
	}
	skills, skillErrors := tool.DiscoverSkills(filepath.Join(b.Workspace, ".harness", "skills"))
	for _, err := range skillErrors {
		_, _ = fmt.Fprintf(stderr, "Skill error: %v\n", err)
	}
	registry, manager, err := setupOperations(ownerCtx, b, effectiveModel, operationDirectory, inputs, len(skills) > 0, owners)
	operations = manager
	if err != nil {
		return false, responseID, err
	}
	for _, skill := range skills {
		if _, err := registry.RegisterSkill(skill); err != nil {
			return false, responseID, fmt.Errorf("register skill %q: %w", skill.Name, err)
		}
	}
	builder := contextbuilder.NewBuilder(skills...)
	builder.SetSystemPrompt(b.SystemPrompt)
	builder.SetModel(b.Model) // coordinator replays persisted settings over defaults
	for _, definition := range registry.StaticDefinitions() {
		builder.AddTool(definition.Tool)
	}
	current := coordinator.New(coordinator.Dependencies{SessionID: b.SessionID, Restored: restored, Sessions: store, Inbox: inputs, Operations: operations, Tools: registry, ContextBuilder: builder, LLM: calls, ToolHeartbeatInterval: 10 * time.Minute})
	if err := events.write(workerproto.Event{Type: "ready", ID: startID}); err != nil {
		return false, responseID, err
	}
	// Atomically hand cancellation responsibility from partial setup to the
	// running coordinator. A cancellation that already claimed setup cannot
	// start a coordinator on an already canceled owner context.
	if !stopSetupCancellation() || ctx.Err() != nil {
		return false, responseID, context.Cause(ctx)
	}
	ended := make(chan struct{})
	var runErr error
	go func() { defer close(ended); runErr = current.Run(ownerCtx) }()
	defer func() {
		// StopHard cancels model/tool owners and consumes their actual terminal
		// updates before returning. This is retirement only: ordinary submissions
		// and busy quiesce never join work. Never synthesize terminal operations.
		watchdog := time.AfterFunc(shutdownGrace, cancelOwners)
		defer watchdog.Stop()
		select {
		case <-ended:
		default:
			payload, _ := json.Marshal(inbox.ControlMessage{Mode: inbox.StopHard, Reason: "worker shutdown"})
			stopErr := inputs.Submit(ownerCtx, inbox.Input{ID: inbox.ID(uuid.New().String()), Kind: inbox.InputControl, Payload: payload})
			result = errors.Join(result, stopErr)
			<-ended
		}
		if !errors.Is(runErr, coordinator.ErrQuiesced) {
			result = errors.Join(result, runErr)
		}
		cancelOwners()
	}()
	for {
		request, err := reader.next(ctx, ended)
		if errors.Is(err, errCoordinatorEnded) {
			if errors.Is(runErr, coordinator.ErrQuiesced) {
				return false, responseID, errors.New("coordinator quiesced without a matching request")
			}
			return false, responseID, runErr
		}
		if errors.Is(err, io.EOF) {
			return false, responseID, nil
		}
		if err != nil {
			return false, responseID, err
		}
		responseID = request.ID
		if request.Bootstrap != nil || request.Protocol != 0 && request.Protocol != workerproto.Version {
			return false, responseID, fmt.Errorf("%w: invalid request envelope", ErrProtocol)
		}
		switch request.Type {
		case "input":
			if request.Input == nil {
				return false, responseID, fmt.Errorf("%w: missing input", ErrProtocol)
			}
			if err := inputs.Submit(ctx, *request.Input); err != nil {
				return false, responseID, err
			}
		case "quiesce":
			if request.ID == "" || request.Input != nil {
				return false, responseID, fmt.Errorf("%w: quiesce requires ID and no input", ErrProtocol)
			}
			err := current.TryQuiesce(ctx)
			if errors.Is(err, coordinator.ErrBusy) {
				if err := events.write(workerproto.Event{Type: "busy", ID: request.ID}); err != nil {
					return false, responseID, err
				}
				continue
			}
			if err != nil {
				return false, responseID, err
			}
			<-ended
			if !errors.Is(runErr, coordinator.ErrQuiesced) {
				return false, responseID, fmt.Errorf("quiesce did not retire coordinator: %w", runErr)
			}
			return true, responseID, nil
		case "stop":
			if request.Input != nil {
				return false, responseID, fmt.Errorf("%w: stop does not accept input", ErrProtocol)
			}
			return false, responseID, nil
		default:
			return false, responseID, fmt.Errorf("%w: unsupported request %q", ErrProtocol, request.Type)
		}
	}
}

// Leave room for the primitive owner's 5s termination escalation and joins
// before the host's default 20s process boundary. A host configured with a shorter grace still force-kills at
// its own boundary; neither fallback is proof that unresolved work terminated.
const shutdownGrace = 10 * time.Second

func terminal(status operation.Status) bool {
	return status == operation.StatusCompleted || status == operation.StatusFailed || status == operation.StatusCanceled || status == operation.StatusUnsupported
}
func replaySettings(model *llm.Model, item sessionstore.Item) {
	input, ok := item.Data.(inbox.Input)
	if !ok || input.Kind != inbox.InputControl {
		return
	}
	control, err := input.DecodeControlMessage()
	if err != nil || control.Mode != inbox.UpdateSettings {
		return
	}
	settings := control.Parameters.(inbox.Settings)
	if settings.Model != "" {
		model.ID = settings.Model
	}
	if settings.CompactionThreshold != nil {
		model.CompactionThreshold = *settings.CompactionThreshold
	}
	if settings.MaxOutputTokens != nil {
		model.MaxOutputTokens = settings.MaxOutputTokens
	}
	if settings.ReasoningEffort != "" {
		model.ReasoningEffort = settings.ReasoningEffort
	}
}
