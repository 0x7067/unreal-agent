// Package jobexecutor owns trusted, named execution independently of operation clients.
// Start belongs to a separately supervised executor, never to a worker/session context.
package jobexecutor

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

const Version = 1
const MaxJobs = 32
const MaxOutput = 64 << 10
const maxBody = 16 << 10

// Plan contains no commands, filesystem paths, or credentials. All execution
// authority comes from the server's named trusted configuration.
type Plan struct {
	Version       int
	OperationID   string
	JobID         string
	AttemptID     string
	ControllerID  string
	EnvironmentID string
	JobName       string
}
type Handle struct {
	Version                                                    int
	OperationID, JobID, AttemptID, ControllerID, EnvironmentID string
	Fingerprint                                                string
}
type Request struct {
	Handle Handle
	Offset int
}
type Snapshot struct {
	Handle    Handle
	State     string // reserved, running, completed, failed, canceled, unknown
	Output    []byte
	Offset    int
	Truncated bool
	ExitCode  int
	Error     string
}
type TrustedJob struct {
	Path string
	Args []string
	Dir  string
	Env  []string
}
type Config struct {
	ControllerIDs []string // optional local allowlist; mandatory for network exposure
	Directory     string
	EnvironmentID string
	Token         string // memory/header only; never persisted
	Jobs          map[string]TrustedJob
	// DropSubmitResponse is a transport fault seam, called after durable launch.
	DropSubmitResponse func(Handle) bool
}
type record struct {
	Plan      Plan
	Handle    Handle
	State     string
	Output    []byte
	Truncated bool
	ExitCode  int
	Error     string
}
type active struct {
	record record
	cmd    *exec.Cmd
	done   chan struct{}
	cancel bool
}
type Service struct {
	mu        sync.Mutex
	cfg       Config
	jobs      map[string]*active
	listener  net.Listener
	server    *http.Server
	lock      *os.File
	workers   sync.WaitGroup
	handlers  sync.WaitGroup
	serveDone chan struct{}
	closed    bool
}

func validID(s string) bool {
	if len(s) == 0 || len(s) > 128 {
		return false
	}
	for _, c := range s {
		if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || strings.ContainsRune("._-", c)) {
			return false
		}
	}
	return s != "." && s != ".."
}
func (p Plan) validate(env string) error {
	if p.Version != Version {
		return errors.New("unsupported plan version")
	}
	for _, s := range []string{p.OperationID, p.JobID, p.AttemptID, p.ControllerID, p.EnvironmentID, p.JobName} {
		if !validID(s) {
			return errors.New("invalid plan identity")
		}
	}
	if p.EnvironmentID != env {
		return errors.New("environment mismatch")
	}
	return nil
}
func identity(p Plan) Handle {
	b, _ := json.Marshal(p)
	sum := sha256.Sum256(b)
	return Handle{Version: Version, OperationID: p.OperationID, JobID: p.JobID, AttemptID: p.AttemptID, ControllerID: p.ControllerID, EnvironmentID: p.EnvironmentID, Fingerprint: hex.EncodeToString(sum[:])}
}
func key(id string) string { s := sha256.Sum256([]byte(id)); return hex.EncodeToString(s[:]) }

// Start acquires exclusive durable ownership of a private directory and Unix listener.
// Reservations from any earlier owner become unknown and can never launch again.
func Start(cfg Config) (*Service, error) {
	if !validID(cfg.EnvironmentID) || len(cfg.Token) < 24 || len(cfg.Token) > 1024 || len(cfg.Jobs) == 0 {
		return nil, errors.New("invalid trusted executor configuration")
	}
	if len(cfg.ControllerIDs) > 32 {
		return nil, errors.New("controller scope exceeds bound")
	}
	seenControllers := map[string]bool{}
	for _, id := range cfg.ControllerIDs {
		if !validID(id) || seenControllers[id] {
			return nil, errors.New("invalid controller scope")
		}
		seenControllers[id] = true
	}
	cfg.ControllerIDs = append([]string(nil), cfg.ControllerIDs...)
	for name, j := range cfg.Jobs {
		if !validID(name) || !filepath.IsAbs(j.Path) {
			return nil, errors.New("invalid trusted job")
		}
	}
	if !filepath.IsAbs(cfg.Directory) {
		return nil, errors.New("executor directory must be absolute")
	}
	if err := os.Mkdir(cfg.Directory, 0700); err != nil && !os.IsExist(err) {
		return nil, err
	}
	info, err := os.Lstat(cfg.Directory)
	if err != nil {
		return nil, err
	}
	if !info.IsDir() || info.Mode().Perm() != 0700 {
		return nil, errors.New("executor directory must be private 0700 directory")
	}
	lock, err := os.OpenFile(filepath.Join(cfg.Directory, "owner.lock"), os.O_CREATE|os.O_RDWR|syscall.O_NOFOLLOW, 0600)
	if err != nil {
		return nil, err
	}
	if err = syscall.Flock(int(lock.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		lock.Close()
		return nil, errors.New("executor already owned")
	}
	s := &Service{cfg: cfg, jobs: map[string]*active{}, lock: lock, serveDone: make(chan struct{})}
	fail := func(err error) (*Service, error) { lock.Close(); return nil, err }
	entries, err := os.ReadDir(cfg.Directory)
	if err != nil {
		return fail(err)
	}
	for _, e := range entries {
		if !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		if len(s.jobs) >= MaxJobs {
			return fail(errors.New("job capacity exceeded"))
		}
		path := filepath.Join(cfg.Directory, e.Name())
		f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NOFOLLOW, 0)
		if err != nil {
			return fail(err)
		}
		var r record
		err = decode(f, &r, MaxOutput*2+maxBody)
		f.Close()
		if err != nil {
			return fail(err)
		}
		if r.Plan.validate(cfg.EnvironmentID) != nil || r.Handle != identity(r.Plan) || e.Name() != key(r.Plan.JobID)+".json" || len(r.Output) > MaxOutput {
			return fail(errors.New("invalid durable job record"))
		}
		a := &active{record: r, done: make(chan struct{})}
		if r.State == "running" || r.State == "reserved" {
			a.record.State = "unknown"
			a.record.Error = "executor ownership interrupted; execution will not be replayed"
			if err = s.save(a); err != nil {
				return fail(err)
			}
		}
		close(a.done)
		s.jobs[r.Plan.JobID] = a
	}
	socket := filepath.Join(cfg.Directory, "executor.sock")
	if info, err := os.Lstat(socket); err == nil {
		if info.Mode()&os.ModeSocket == 0 {
			return fail(errors.New("refusing non-socket listener path"))
		}
		if err = os.Remove(socket); err != nil {
			return fail(err)
		}
	} else if !os.IsNotExist(err) {
		return fail(err)
	}
	listener, err := net.Listen("unix", socket)
	if err != nil {
		return fail(err)
	}
	if err = os.Chmod(socket, 0600); err != nil {
		listener.Close()
		return fail(err)
	}
	s.listener = listener
	s.server = &http.Server{Handler: s, ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 4096}
	go func() { defer close(s.serveDone); _ = s.server.Serve(listener) }()
	return s, nil
}
func decode(r io.Reader, v any, max int64) error {
	data, err := io.ReadAll(io.LimitReader(r, max+1))
	if err != nil {
		return err
	}
	if int64(len(data)) > max {
		return errors.New("request too large")
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.DisallowUnknownFields()
	if err = d.Decode(v); err != nil {
		return err
	}
	if d.Decode(new(any)) != io.EOF {
		return errors.New("trailing data")
	}
	return nil
}
func (s *Service) save(a *active) error {
	b, err := json.Marshal(a.record)
	if err != nil {
		return err
	}
	if len(b) > MaxOutput*2+maxBody {
		return errors.New("record too large")
	}
	f, err := os.CreateTemp(s.cfg.Directory, ".reserve-")
	if err != nil {
		return err
	}
	name := f.Name()
	defer os.Remove(name)
	if _, err = f.Write(b); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err == nil {
		err = closeErr
	}
	if err != nil {
		return err
	}
	if err = os.Rename(name, filepath.Join(s.cfg.Directory, key(a.record.Plan.JobID)+".json")); err != nil {
		return err
	}
	dir, err := os.Open(s.cfg.Directory)
	if err != nil {
		return err
	}
	defer dir.Close()
	return dir.Sync()
}
func (s *Service) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		http.Error(w, "executor unavailable", http.StatusServiceUnavailable)
		return
	}
	s.handlers.Add(1)
	s.mu.Unlock()
	defer s.handlers.Done()
	if r.Method != "POST" {
		http.Error(w, "POST required", 405)
		return
	}
	if subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+s.cfg.Token)) != 1 {
		http.Error(w, "unauthorized", 401)
		return
	}
	var snap Snapshot
	var err error
	switch r.URL.Path {
	case "/submit":
		var p Plan
		if err = decode(r.Body, &p, maxBody); err == nil {
			snap, err = s.submit(p)
		}
	case "/inspect", "/cancel":
		var req Request
		if err = decode(r.Body, &req, maxBody); err == nil {
			if r.URL.Path == "/cancel" {
				snap, err = s.cancel(r.Context(), req)
			} else {
				snap, err = s.inspect(req)
			}
		}
	default:
		http.Error(w, "unknown route", 404)
		return
	}
	if err != nil {
		http.Error(w, err.Error(), 409)
		return
	}
	if r.URL.Path == "/submit" && s.cfg.DropSubmitResponse != nil && s.cfg.DropSubmitResponse(snap.Handle) {
		if h, ok := w.(http.Hijacker); ok {
			conn, _, e := h.Hijack()
			if e == nil {
				conn.Close()
				return
			}
		}
	}
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(snap)
}
func snapshot(a *active, offset int) (Snapshot, error) {
	if offset < 0 || offset > len(a.record.Output) {
		return Snapshot{}, errors.New("invalid output offset")
	}
	r := a.record
	return Snapshot{Handle: r.Handle, State: r.State, Output: append([]byte(nil), r.Output[offset:]...), Offset: len(r.Output), Truncated: r.Truncated, ExitCode: r.ExitCode, Error: r.Error}, nil
}
func (s *Service) submit(p Plan) (Snapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed {
		return Snapshot{}, errors.New("executor unavailable")
	}
	if !s.controllerAllowed(p.ControllerID) {
		return Snapshot{}, errors.New("controller scope denied")
	}
	if err := p.validate(s.cfg.EnvironmentID); err != nil {
		return Snapshot{}, err
	}
	h := identity(p)
	if a := s.jobs[p.JobID]; a != nil {
		if a.record.Handle != h {
			return Snapshot{}, errors.New("idempotency conflict")
		}
		return snapshot(a, 0)
	}
	job, ok := s.cfg.Jobs[p.JobName]
	if !ok {
		return Snapshot{}, errors.New("trusted job not configured")
	}
	if len(s.jobs) >= MaxJobs {
		return Snapshot{}, errors.New("executor job capacity reached")
	}
	a := &active{record: record{Plan: p, Handle: h, State: "reserved", ExitCode: -1}, done: make(chan struct{})}
	if err := s.save(a); err != nil {
		return Snapshot{}, err
	}
	s.jobs[p.JobID] = a
	cmd := exec.Command(job.Path, job.Args...)
	cmd.Dir = job.Dir
	cmd.Env = append([]string{}, job.Env...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.WaitDelay = time.Second
	cmd.Stdout = &outputWriter{s: s, a: a}
	cmd.Stderr = cmd.Stdout
	a.cmd = cmd
	if err := cmd.Start(); err != nil {
		a.record.State = "failed"
		a.record.Error = "trusted command launch failed"
		_ = s.save(a)
		close(a.done)
		return snapshot(a, 0)
	}
	a.record.State = "running"
	if err := s.save(a); err != nil {
		a.record.Error = "running checkpoint failed; execution reserved"
	}
	s.workers.Add(1)
	go s.wait(a)
	return snapshot(a, 0)
}

type outputWriter struct {
	s *Service
	a *active
}

func (w *outputWriter) Write(p []byte) (int, error) {
	w.s.mu.Lock()
	defer w.s.mu.Unlock()
	n := len(p)
	room := MaxOutput - len(w.a.record.Output)
	if len(p) > room {
		p = p[:room]
		w.a.record.Truncated = true
	}
	w.a.record.Output = append(w.a.record.Output, p...)
	if err := w.s.save(w.a); err != nil {
		w.a.record.Error = "output checkpoint failed"
	}
	return n, nil
}
func (s *Service) wait(a *active) {
	defer s.workers.Done()
	err := a.cmd.Wait()
	s.mu.Lock()
	defer s.mu.Unlock()
	a.record.ExitCode = a.cmd.ProcessState.ExitCode()
	a.record.State = "completed"
	if err != nil {
		a.record.State = "failed"
	}
	if a.cancel {
		a.record.State = "canceled"
	}
	if err := s.save(a); err != nil {
		a.record.State = "unknown"
		a.record.Error = "terminal checkpoint failed"
	}
	close(a.done)
}
func (s *Service) authorized(req Request) (*active, error) {
	a := s.jobs[req.Handle.JobID]
	if a == nil || a.record.Handle != req.Handle || req.Handle.Version != Version {
		return nil, errors.New("job authority mismatch")
	}
	return a, nil
}
func (s *Service) inspect(req Request) (Snapshot, error) {
	if !s.controllerAllowed(req.Handle.ControllerID) {
		return Snapshot{}, errors.New("controller scope denied")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	a, err := s.authorized(req)
	if err != nil {
		return Snapshot{}, err
	}
	return snapshot(a, req.Offset)
}
func (s *Service) cancel(ctx context.Context, req Request) (Snapshot, error) {
	if !s.controllerAllowed(req.Handle.ControllerID) {
		return Snapshot{}, errors.New("controller scope denied")
	}
	s.mu.Lock()
	a, err := s.authorized(req)
	if err != nil {
		s.mu.Unlock()
		return Snapshot{}, err
	}
	if a.record.State == "running" {
		a.cancel = true
		_ = syscall.Kill(-a.cmd.Process.Pid, syscall.SIGKILL)
	}
	done := a.done
	s.mu.Unlock()
	select {
	case <-done:
		return s.inspect(req)
	case <-ctx.Done():
		return Snapshot{}, ctx.Err()
	}
}

// Close stops admission, cancels owned live processes, joins Wait/output workers,
// drains HTTP handlers, and releases directory ownership. Client.Close never calls it.
func (s *Service) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return errors.New("executor already closed")
	}
	s.closed = true
	for _, a := range s.jobs {
		if a.record.State == "running" {
			a.cancel = true
			_ = syscall.Kill(-a.cmd.Process.Pid, syscall.SIGKILL)
		}
	}
	s.mu.Unlock()
	s.workers.Wait()
	err := s.server.Close()
	<-s.serveDone
	s.handlers.Wait()
	s.lock.Close()
	return err
}

type Client struct {
	http      *http.Client
	transport *http.Transport
	token     string
}

func NewClient(socket, token string) *Client {
	t := &http.Transport{DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return (&net.Dialer{Timeout: time.Second}).DialContext(ctx, "unix", socket)
	}, DisableKeepAlives: true}
	return &Client{http: &http.Client{Transport: t, Timeout: 4 * time.Second}, transport: t, token: token}
}
func (c *Client) Close() { c.transport.CloseIdleConnections() }
func (c *Client) call(ctx context.Context, path string, body any) (Snapshot, error) {
	var snap Snapshot
	b, err := json.Marshal(body)
	if err != nil {
		return snap, err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "http://executor"+path, bytes.NewReader(b))
	if err != nil {
		return snap, err
	}
	req.Header.Set("Authorization", "Bearer "+c.token)
	res, err := c.http.Do(req)
	if err != nil {
		return snap, fmt.Errorf("executor unavailable or acknowledgment unknown: %w", err)
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return snap, fmt.Errorf("executor rejected request (HTTP %d)", res.StatusCode)
	}
	err = decode(res.Body, &snap, MaxOutput*2+maxBody)
	return snap, err
}

// Submit retries only the identical stable identity, at most twice. A lost
// post-launch acknowledgment reconciles to the durable reservation, never reruns.
func (c *Client) Submit(ctx context.Context, p Plan) (Snapshot, error) {
	snap, err := c.call(ctx, "/submit", p)
	if err != nil && ctx.Err() == nil {
		return c.call(ctx, "/submit", p)
	}
	return snap, err
}
func (c *Client) Inspect(ctx context.Context, h Handle, offset int) (Snapshot, error) {
	return c.call(ctx, "/inspect", Request{Handle: h, Offset: offset})
}
func (c *Client) Cancel(ctx context.Context, h Handle) (Snapshot, error) {
	return c.call(ctx, "/cancel", Request{Handle: h})
}

func (s *Service) controllerAllowed(id string) bool {
	if len(s.cfg.ControllerIDs) == 0 {
		return true
	} // private UDS historical default
	for _, allowed := range s.cfg.ControllerIDs {
		if allowed == id {
			return true
		}
	}
	return false
}
