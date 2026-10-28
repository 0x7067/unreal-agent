// Package durablechild owns durable one-shot launch reservations and socket-only
// connections to independent resident CLI hosts. A launch marker is never a PID.
package durablechild

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"time"
	"uuid"

	"github.com/unreallabsai/unreal-agent/cmd/internal/attachclient"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/session"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
)

var ErrNotSubmitted = errors.New("durable child not submitted")
var ErrUncertain = errors.New("durable child launch uncertain; automatic relaunch forbidden")
var ErrConflict = errors.New("durable child immutable plan conflict")

type Config struct {
	Directory, RuntimeDirectory, Workspace, Provider, BaseURL, Model string
	Command                                                          []string
}
type Request struct {
	ParentID               session.ID
	OperationID            operation.ID
	Name, Prompt, Lifetime string
}
type Link struct {
	Version                                                  int
	ParentID                                                 session.ID
	OperationID                                              operation.ID
	ChildID                                                  session.ID
	Key, PlanDigest, Directory, RuntimeDirectory, DeliveryID string
}
type Answer struct {
	TurnID   session.TurnID
	Sequence sessionstore.Sequence
	Text     string
}
type launchRecord struct {
	Version int
	Link    Link
	Config  Config
	Request Request
}
type Connection struct {
	link       Link
	request    Request
	client     *attachclient.Client
	readerDone chan struct{}
	launchDone chan struct{}
	launchErr  error // published by closing launchDone
}

func digest(v any) string {
	b, _ := json.Marshal(v, json.Deterministic(true))
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])
}
func plan(cfg Config, req Request) (launchRecord, error) {
	if req.ParentID == "" || req.OperationID == "" || len(cfg.Command) == 0 || !filepath.IsAbs(cfg.Command[0]) {
		return launchRecord{}, errors.New("parent, operation and absolute command required")
	}
	for _, p := range []string{cfg.Directory, cfg.RuntimeDirectory, cfg.Workspace} {
		if !filepath.IsAbs(p) {
			return launchRecord{}, errors.New("absolute directories required")
		}
	}
	cfg.Directory = filepath.Clean(cfg.Directory)
	cfg.RuntimeDirectory = filepath.Clean(cfg.RuntimeDirectory)
	cfg.Workspace = filepath.Clean(cfg.Workspace)
	key := digest([]string{string(req.ParentID), string(req.OperationID)})
	identityConfig := cfg
	// A replacement parent may have a different staged pathname. Existing child
	// execution remains pinned to the original owner; pathname is not job identity.
	identityConfig.Command = nil
	l := Link{Version: 1, ParentID: req.ParentID, OperationID: req.OperationID, ChildID: session.ID("child-" + key), Key: key, PlanDigest: digest(struct {
		Config  Config
		Request Request
	}{identityConfig, req}), Directory: filepath.Join(cfg.Directory, "durable-children", key), RuntimeDirectory: cfg.RuntimeDirectory, DeliveryID: "child-initial-" + key}
	return launchRecord{1, l, cfg, req}, nil
}

// privatePath rejects symlinks in every existing path component. Missing final
// components may be created only under the validated private store root.
func privatePath(path string, directory bool) error {
	for p := filepath.Clean(path); ; p = filepath.Dir(p) {
		st, err := os.Lstat(p)
		if err != nil {
			return err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return errors.New("symlink metadata path forbidden")
		}
		if p == path {
			if directory && !st.IsDir() {
				return errors.New("not a directory")
			}
			if st.Mode().Perm()&0077 != 0 {
				return errors.New("metadata path must be private")
			}
		}
		if p == filepath.Dir(p) {
			break
		}
	}
	return nil
}
func syncDir(path string) error {
	f, e := os.Open(path)
	if e != nil {
		return e
	}
	defer f.Close()
	return f.Sync()
}
func readRecord(path string) (launchRecord, error) {
	var r launchRecord
	if e := privatePath(path, false); e != nil {
		return r, e
	}
	f, e := os.Open(path)
	if e != nil {
		return r, e
	}
	defer f.Close()
	st, e := f.Stat()
	if e != nil {
		return r, e
	}
	if !st.Mode().IsRegular() || st.Mode().Perm() != 0600 || st.Size() > 1<<20 {
		return r, errors.New("invalid launch marker")
	}
	b, e := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if e != nil {
		return r, e
	}
	e = json.Unmarshal(b, &r, json.RejectUnknownMembers(true))
	if e != nil {
		return r, e
	}
	expected, e := plan(r.Config, r.Request)
	if e != nil || r.Version != 1 || r.Link != expected.Link {
		return r, errors.New("invalid launch identity")
	}
	return r, nil
}

// Open reserves a v1 write-before-start launch exactly once. A marker, including
// an incomplete marker after a crash, always forbids a second execution.
// On a post-launch error a nonnil connection retains the actual WaitLaunch
// capability; Close is safe without a connected transport and never stops the host.
func Open(ctx context.Context, cfg Config, req Request, allowLaunch bool) (*Connection, error) {
	r, e := plan(cfg, req)
	if e != nil {
		return nil, e
	}
	cfg = r.Config
	if e = ctx.Err(); e != nil {
		return nil, e
	}
	if e = privatePath(cfg.Directory, true); e != nil {
		return nil, e
	}
	if e = privatePath(cfg.RuntimeDirectory, true); e != nil {
		return nil, e
	}
	marker := filepath.Join(r.Link.Directory, "launch.json")
	existing, e := readRecord(marker)
	launch := false
	if errors.Is(e, os.ErrNotExist) {
		if _, err := os.Lstat(r.Link.Directory); err == nil || !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("%w: existing child directory without valid reservation", ErrUncertain)
		}
		if !allowLaunch {
			return nil, ErrNotSubmitted
		}
		for _, p := range []string{filepath.Dir(r.Link.Directory), r.Link.Directory} {
			if e = os.Mkdir(p, 0700); e != nil && !errors.Is(e, os.ErrExist) {
				return nil, e
			}
			if e = privatePath(p, true); e != nil {
				return nil, e
			}
			if e = syncDir(filepath.Dir(p)); e != nil {
				return nil, e
			}
		}
		f, err := os.OpenFile(marker, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if errors.Is(err, os.ErrExist) {
			existing, e = readRecord(marker)
		} else if err != nil {
			return nil, err
		} else {
			b, err := json.Marshal(r, json.Deterministic(true))
			if err == nil {
				var n int
				n, err = f.Write(b)
				if err == nil && n != len(b) {
					err = io.ErrShortWrite
				}
			}
			if err == nil {
				err = f.Sync()
			}
			ce := f.Close()
			if err == nil {
				err = ce
			}
			if err == nil {
				err = syncDir(r.Link.Directory)
			}
			if err != nil {
				return nil, fmt.Errorf("%w: reserve: %v", ErrUncertain, err)
			}
			launch = true
			e = nil
			existing = r
		}
	}
	if e != nil {
		return nil, fmt.Errorf("%w: marker: %v", ErrUncertain, e)
	}
	if existing.Link != r.Link {
		return nil, ErrConflict
	}
	c := &Connection{link: r.Link, request: req}
	if launch {
		logPath := filepath.Join(r.Link.Directory, "host.log")
		log, err := os.OpenFile(logPath, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
		if err != nil {
			return nil, fmt.Errorf("%w: log: %v", ErrUncertain, err)
		}
		args := append([]string(nil), cfg.Command[1:]...)
		args = append(args, "host", "--runtime-directory", cfg.RuntimeDirectory, "--session-directory", r.Link.Directory, "--session", string(r.Link.ChildID), "--workspace", cfg.Workspace, "--provider", cfg.Provider, "--base-url", cfg.BaseURL, "--model", cfg.Model, "--worker-executable", cfg.Command[0])
		cmd := exec.Command(cfg.Command[0], args...)
		if err := isolateChild(cmd); err != nil {
			_ = log.Close()
			return nil, err
		}
		cmd.Stdin = nil
		cmd.Stdout = log
		cmd.Stderr = log
		err = cmd.Start()
		_ = log.Close()
		if err != nil {
			return nil, fmt.Errorf("%w: start: %v", ErrUncertain, err)
		}
		c.launchDone = make(chan struct{})
		go func() { c.launchErr = cmd.Wait(); close(c.launchDone) }()
	}
	// Detach connection lifetime from the parent's request context. Close joins
	// only the reader; neither cancellation nor Close sends a child stop.
	deadline := time.Now().Add(5 * time.Second)
	for {
		if e = ctx.Err(); e != nil {
			return c, e
		}
		dialCtx, cancel := context.WithCancel(context.Background())
		stop := context.AfterFunc(ctx, cancel)
		timer := time.AfterFunc(time.Until(deadline), cancel)
		c.client, e = attachclient.Dial(dialCtx, cfg.RuntimeDirectory, r.Link.Directory, r.Link.ChildID)
		stop()
		timer.Stop()
		if e == nil {
			c.readerDone = make(chan struct{})
			go func() {
				defer close(c.readerDone)
				defer cancel()
				for {
					if _, err := c.client.Next(); err != nil {
						return
					}
				}
			}()
			return c, nil
		}
		cancel()
		if !time.Now().Before(deadline) {
			return c, fmt.Errorf("%w: endpoint: %v", ErrUncertain, e)
		}
		select {
		case <-ctx.Done():
			return c, ctx.Err()
		case <-time.After(50 * time.Millisecond):
		}
	}
}
func (c *Connection) Link() Link                            { return c.link }
func (c *Connection) ClientSnapshot() attachclient.Snapshot { return c.client.Snapshot() }
func (c *Connection) Close() error {
	if c.client == nil {
		return nil
	}
	e := c.client.Close()
	<-c.readerDone
	return e
}

// WaitLaunch joins only an actual command owned by this connection, never a PID.
func (c *Connection) WaitLaunch(ctx context.Context) error {
	if c.launchDone == nil {
		return errors.New("launch command not owned")
	}
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-c.launchDone:
		return c.launchErr
	}
}
func (c *Connection) SubmitInitial(ctx context.Context) error {
	b, e := json.Marshal(c.request.Prompt)
	if e != nil {
		return e
	}
	return c.client.Submit(ctx, inbox.Input{ID: inbox.ID(c.link.DeliveryID), Kind: inbox.InputExternal, Payload: b})
}

// Shutdown acknowledges admission only, not child termination or cleanup.
func (c *Connection) Shutdown(ctx context.Context) error {
	s := c.ClientSnapshot()
	if s.Header.Status == nil {
		return errors.New("missing lifecycle status")
	}
	b, e := json.Marshal(inbox.ControlMessage{Mode: inbox.ShutdownSession, Parameters: inbox.ShutdownSessionRequest{Version: 1, Scope: fmt.Sprintf("%s/%d", s.Hello.Incarnation, s.Header.Status.Generation)}})
	if e != nil {
		return e
	}
	return c.client.SubmitLifecycle(ctx, inbox.Input{ID: inbox.ID("child-shutdown-" + uuid.New().String()), Kind: inbox.InputControl, Payload: b})
}

// Answer reads canonical history, never transport completion or terminal tickets.
// The scan has bounded pages and a hard 4096-item budget per call.
func (c *Connection) Answer(ctx context.Context) (Answer, bool, error) {
	store, e := localfile.New(c.link.Directory)
	if e != nil {
		return Answer{}, false, e
	}
	var after sessionstore.Sequence
	seen := false
	regularTurns := map[session.TurnID]bool{}
	for pages := 0; pages < 32; pages++ {
		page, e := store.Items(ctx, c.link.ChildID, after, 128)
		if e != nil {
			return Answer{}, false, e
		}
		for _, item := range page.Items {
			if item.Kind == sessionstore.ItemInput {
				in, ok := item.Data.(inbox.Input)
				if ok && in.Kind == inbox.InputExternal && string(in.ID) != c.link.DeliveryID {
					return Answer{}, false, fmt.Errorf("%w: unlinked child input before answer", ErrConflict)
				}
				if ok && string(in.ID) == c.link.DeliveryID {
					var prompt string
					if e = json.Unmarshal(in.Payload, &prompt); e != nil || prompt != c.request.Prompt || in.Kind != inbox.InputExternal {
						return Answer{}, false, ErrConflict
					}
					seen = true
				}
			}
			if seen && item.Kind == sessionstore.ItemTurn {
				if turn, ok := item.Data.(session.Turn); ok && turn.Type == session.TurnRegular {
					regularTurns[turn.ID] = true
				}
			}
			if !seen || item.Kind != sessionstore.ItemModelResponse {
				continue
			}
			response, ok := item.Data.(sessionstore.ModelResponse)
			if !ok || !regularTurns[response.TurnID] || response.Response.Failure != nil || response.Response.Stop != llm.StopComplete {
				continue
			}
			text := ""
			tool := false
			for _, out := range response.Response.Output {
				switch v := out.Data.(type) {
				case llm.ToolCall:
					tool = true
				case llm.Message:
					if v.Role == llm.RoleAssistant {
						text += v.Text
					}
				}
				if len(text) > 256<<10 {
					return Answer{}, false, errors.New("child answer exceeds 256 KiB")
				}
			}
			if !tool && text != "" {
				return Answer{response.TurnID, item.Sequence, text}, true, nil
			}
		}
		if !page.More {
			return Answer{}, false, nil
		}
		if page.NextAfter <= after {
			return Answer{}, false, errors.New("nonprogressing history")
		}
		after = page.NextAfter
	}
	return Answer{}, false, errors.New("child history exceeds scan budget")
}
