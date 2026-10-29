package residentcli

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json/v2"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/cmd/internal/durablejob"
	"github.com/unreallabsai/unreal-agent/cmd/internal/jobexecutor"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerhost"
	"github.com/unreallabsai/unreal-agent/cmd/internal/workerproto"
	"github.com/unreallabsai/unreal-agent/harness/inbox"
	"github.com/unreallabsai/unreal-agent/harness/llm"
	"github.com/unreallabsai/unreal-agent/harness/operation"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore"
	"github.com/unreallabsai/unreal-agent/harness/sessionstore/localfile"
	"github.com/unreallabsai/unreal-agent/internal/testtmp"
)

// Actual verified TLS environment, independently owned service/job and real worker A/B.
// The service fault drops acknowledgments only after durable reservation and launch.
func TestM45VerifiedRemoteRecoveryAndNoLocalFallback(t *testing.T) {
	root, err := testtmp.MkdirTemp("j45.")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(root) })
	path := func(name string) string { return filepath.Join(root, name) }
	ctx, cancel := context.WithTimeout(t.Context(), 40*time.Second)
	defer cancel()
	wait := func(label string, f func() bool) {
		t.Helper()
		for !f() {
			select {
			case <-ctx.Done():
				t.Fatalf("%s: %v", label, ctx.Err())
			case <-time.After(5 * time.Millisecond):
			}
		}
	}
	for _, dir := range []string{"work", "state", "home"} {
		if err := os.Mkdir(path(dir), 0700); err != nil {
			t.Fatal(err)
		}
	}
	writeConfig := func(name string, value any) {
		t.Helper()
		b, e := json.Marshal(value)
		if e != nil {
			t.Fatal(e)
		}
		if e = os.WriteFile(path(name), b, 0600); e != nil {
			t.Fatal(e)
		}
	}
	token := "private-m45-scoped-executor-credential"
	t.Setenv("HOME", path("home"))
	t.Setenv("XDG_CONFIG_HOME", path("home"))
	t.Setenv("XDG_STATE_HOME", path("home"))
	t.Setenv("OPENAI_API_KEY", "fake")
	t.Setenv("UNREAL_HARNESS_LLM_API_KEY", "fake")
	t.Setenv(durablejob.TokenEnvironment, token)
	command := "printf x >> " + path("effect") + "; printf first; while [ ! -e " + path("release") + " ]; do /bin/sleep 0.01; done; printf second"

	cert, roots, caPEM := m45Certificate(t)
	if err = os.WriteFile(path("ca.pem"), caPEM, 0600); err != nil {
		t.Fatal(err)
	}
	var dropped atomic.Bool
	var drop atomic.Bool
	drop.Store(true)
	service, err := jobexecutor.Start(jobexecutor.Config{Directory: path("executor"), EnvironmentID: "private-tests", Token: token, ControllerIDs: []string{durablejob.Controller("private-job")}, Jobs: map[string]jobexecutor.TrustedJob{"tests": {Path: "/bin/sh", Args: []string{"-c", command}, Dir: path("work"), Env: []string{"PATH=/usr/bin:/bin"}}}, DropSubmitResponse: func(jobexecutor.Handle) bool {
		if drop.Load() {
			dropped.Store(true)
			return true
		}
		return false
	}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = service.Close() })
	endpoint, err := service.ListenTLS("127.0.0.1:0", cert)
	if err != nil {
		t.Fatal(err)
	}
	address := endpoint.Address()
	t.Cleanup(func() { _ = endpoint.Close() })
	runtimeConfig := durablejob.RuntimeConfig{Version: 1, EnvironmentID: "private-tests", TLSAddress: address, TLSServerName: "executor.test", CAFile: path("ca.pem"), Jobs: []string{"tests"}}
	writeConfig("runtime.json", runtimeConfig)
	t.Setenv(durablejob.ConfigEnvironment, path("runtime.json"))
	var calls atomic.Int64
	var scrubbed atomic.Bool
	provider := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestBody, _ := io.ReadAll(r.Body)
		n := calls.Add(1)
		if n == 4 {
			var request struct {
				Input []map[string]any `json:"input"`
			}
			if e := json.Unmarshal(requestBody, &request); e == nil {
				for _, item := range request.Input {
					if item["type"] == "function_call_output" && item["call_id"] == "scrub-call" {
						if strings.Contains(fmt.Sprint(item["output"]), "SCRUBBED_EXECUTOR_AUTH") {
							scrubbed.Store(true)
						}
					}
				}
			}
		}

		output := []map[string]any{{"type": "message", "id": "answer", "role": "assistant", "content": []map[string]any{{"type": "output_text", "text": "job completed"}}}}
		if n == 3 {
			output = []map[string]any{{"type": "function_call", "id": "scrub-call", "call_id": "scrub-call", "name": "Bash", "arguments": `{"command":"test -z \"$UNREAL_AGENT_EXECUTOR_TOKEN\" && printf SCRUBBED_EXECUTOR_AUTH","description":"Check private child environment"}`}}
		}
		if n == 1 {
			output = []map[string]any{{"type": "function_call", "id": "job-call", "call_id": "job-call", "name": "RunTrustedJob", "arguments": `{"job":"tests"}`}}
		}
		body, _ := json.Marshal(map[string]any{"type": "response.completed", "response": map[string]any{"id": fmt.Sprint("response-", n), "status": "completed", "output": output, "usage": map[string]int{"input_tokens": 1, "output_tokens": 1}}})
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = fmt.Fprintf(w, "data: %s\n\n", body)
	}))
	defer provider.Close()
	binary, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	cfg := workerhost.Config{Command: []string{binary, "worker"}, Bootstrap: workerproto.Bootstrap{SessionID: "private-job", Directory: path("state"), Workspace: path("work"), Provider: "openai", BaseURL: provider.URL, Model: llm.Model{ID: "private-model", CompactionThreshold: 64000}, MaxAttempts: 2, Shell: "/bin/sh"}}
	first, err := workerhost.Start(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = first.Close(context.Background()) })
	input := inbox.Input{ID: "original-job-input", Kind: inbox.InputExternal, Payload: []byte(`"run private tests"`)}
	if err = first.Submit(ctx, input); err != nil {
		t.Fatal(err)
	}
	wait("postlaunch response dropped and effect", func() bool {
		b, e := os.ReadFile(path("effect"))
		return dropped.Load() && e == nil && string(b) == "x"
	})
	store, err := localfile.New(path("state"))
	if err != nil {
		t.Fatal(err)
	}
	find := func() operation.Operation {
		t.Helper()
		r, e := store.Resume(ctx, "private-job")
		if e != nil {
			t.Fatal(e)
		}
		for _, op := range r.Operations {
			if op.Type == operation.TypeRemoteJob {
				return op
			}
		}
		history, e := sessionstore.ReadLifecycle(ctx, store, "private-job")
		if e != nil {
			t.Fatal(e)
		}
		for _, op := range history.RemoteOperations {
			if op.Type == operation.TypeRemoteJob {
				return op
			}
		}
		return operation.Operation{}
	}
	wait("committed job intent", func() bool { return find().ID != "" })
	original := find().ID
	if err = first.Kill(); err != nil {
		t.Fatal(err)
	}
	select {
	case e := <-first.Done():
		if e == nil {
			t.Fatal("crashed worker falsely successful")
		}
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}
	if err = first.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if _, e := os.Stat(path("release")); !errors.Is(e, os.ErrNotExist) {
		t.Fatal("job gate unexpectedly released")
	}

	// Both runtime and TLS transport are gone, but the separately owned job survives.
	if err = endpoint.Close(); err != nil {
		t.Fatal(err)
	}
	remote, err := jobexecutor.NewTLSClient(address, token, "executor.test", roots)
	if err != nil {
		t.Fatal(err)
	}
	defer remote.Close()
	var recorded struct{ Plan jobexecutor.Plan }
	entries, err := os.ReadDir(path("executor"))
	if err != nil {
		t.Fatal(err)
	}
	reservations := 0
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			reservations++
			data, e := os.ReadFile(path("executor/" + entry.Name()))
			if e != nil {
				t.Fatal(e)
			}
			if e = json.Unmarshal(data, &recorded); e != nil {
				t.Fatal(e)
			}
		}
	}
	if reservations != 1 || recorded.Plan.ControllerID != durablejob.Controller("private-job") || recorded.Plan.EnvironmentID != "private-tests" {
		t.Fatalf("remote reservation scope %+v count%d", recorded, reservations)
	}
	if _, err = remote.Submit(ctx, recorded.Plan); err == nil {
		t.Fatal("unavailable remote executor falsely succeeded")
	}
	effect, e := os.ReadFile(path("effect"))
	if e != nil || string(effect) != "x" {
		t.Fatalf("remote loss caused local rerun %q %v", effect, e)
	}
	drop.Store(false)
	endpoint, err = service.ListenTLS(address, cert)
	if err != nil {
		t.Fatal(err)
	}
	unauthorized, err := jobexecutor.NewTLSClient(address, "wrong-token-not-authorized-at-all", "executor.test", roots)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = unauthorized.Submit(ctx, recorded.Plan); err == nil {
		t.Fatal("unauthorized remote submission accepted")
	}
	unauthorized.Close()
	bad := recorded.Plan
	bad.EnvironmentID = "another-environment"
	if _, err = remote.Submit(ctx, bad); err == nil {
		t.Fatal("environment mismatch accepted")
	}
	bad = recorded.Plan
	bad.ControllerID = "untrusted-controller"
	bad.JobID = "untrusted-new-job"
	if _, err = remote.Submit(ctx, bad); err == nil {
		t.Fatal("new untrusted controller scope accepted")
	}
	snap, err := remote.Submit(ctx, recorded.Plan)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Handle.JobID != recorded.Plan.JobID || snap.State != "running" {
		t.Fatalf("duplicate did not reattach %+v", snap)
	}
	second, err := workerhost.Start(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = second.Close(context.Background()) })
	if err = os.WriteFile(path("release"), []byte("explicit finish"), 0600); err != nil {
		t.Fatal(err)
	}
	wait("same original operation terminal after reattach", func() bool { op := find(); return op.ID == original && op.Status == operation.StatusCompleted })
	op := find()
	state, err := operation.DecodeRemoteJobState(op)
	if err != nil {
		t.Fatal(err)
	}
	if err = durablejob.TerminalCleanupProof(op, durablejob.RuntimeConfig{EnvironmentID: "private-tests", Jobs: []string{"tests"}}.Authority("private-job")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(state.TerminalResult, "firstsecond") {
		t.Fatalf("output lost: %q", state.TerminalResult)
	}
	if err = second.Submit(ctx, input); err != nil {
		t.Fatal(err)
	}
	wait("final model result", func() bool { return calls.Load() == 2 })
	scrub := inbox.Input{ID: "verify-native-child-authority", Kind: inbox.InputExternal, Payload: []byte(`"check scoped native child environment"`)}
	if err = second.Submit(ctx, scrub); err != nil {
		t.Fatal(err)
	}
	wait("native shell cannot inherit executor credential", func() bool {
		data, e := os.ReadFile(path("state/private-job.session.jsonl"))
		return e == nil && strings.Contains(string(data), "SCRUBBED_EXECUTOR_AUTH") && calls.Load() == 4 && scrubbed.Load()
	})
	if err = second.Close(ctx); err != nil {
		t.Fatal(err)
	}
	effect, err = os.ReadFile(path("effect"))
	if err != nil || string(effect) != "x" {
		t.Fatalf("duplicate effect %q %v", effect, err)
	}
	canonical, err := os.ReadFile(path("state/private-job.session.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(canonical), token) || strings.Contains(string(canonical), address) || strings.Contains(string(canonical), path("ca.pem")) {
		t.Fatal("credential/transport path leaked to canonical history")
	}

	final, err := remote.Inspect(ctx, snap.Handle, 0)
	if err != nil {
		t.Fatal(err)
	}
	if final.State != "completed" || string(final.Output) != "firstsecond" {
		t.Fatalf("durable remote output reference lost %+v", final)
	}
	if err = endpoint.Close(); err != nil {
		t.Fatal(err)
	}
	if err = service.Close(); err != nil {
		t.Fatal(err)
	}
}

func m45Certificate(t *testing.T) (tls.Certificate, *x509.CertPool, []byte) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(45), Subject: pkix.Name{CommonName: "executor.test"}, DNSNames: []string{"executor.test"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	caPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der})
	keyDER, err := x509.MarshalPKCS8PrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := tls.X509KeyPair(caPEM, pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: keyDER}))
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	if !roots.AppendCertsFromPEM(caPEM) {
		t.Fatal("certificate roots")
	}
	return cert, roots, caPEM
}
