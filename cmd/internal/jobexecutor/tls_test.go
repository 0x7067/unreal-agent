package jobexecutor

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/unreallabsai/unreal-agent/internal/testtmp"
)

func tlsFixtureCertificate(t *testing.T) (tls.Certificate, *x509.CertPool) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{SerialNumber: big.NewInt(1), Subject: pkix.Name{CommonName: "executor.test"}, DNSNames: []string{"executor.test"}, NotBefore: time.Now().Add(-time.Minute), NotAfter: time.Now().Add(time.Hour), KeyUsage: x509.KeyUsageDigitalSignature, ExtKeyUsage: []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth}, BasicConstraintsValid: true}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	leaf, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	roots := x509.NewCertPool()
	roots.AddCert(leaf)
	return tls.Certificate{Certificate: [][]byte{der}, PrivateKey: key, Leaf: leaf}, roots
}
func tlsFixtureService(t *testing.T) (*Service, string) {
	t.Helper()
	root, err := testtmp.MkdirTemp("jt-")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.RemoveAll(root) })
	effect := filepath.Join(root, "effect")
	s, err := Start(Config{ControllerIDs: []string{"owner"}, Directory: filepath.Join(root, "executor"), EnvironmentID: "tls-test", Token: "tls-test-token-at-least-24-characters", Jobs: map[string]TrustedJob{"effect": {Path: "/bin/sh", Args: []string{"-c", "echo one >> " + effect + "; printf tls-output"}, Dir: root, Env: []string{"PATH=/usr/bin:/bin"}}}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s, effect
}
func tlsFixtureClient(t *testing.T, address, token, name string, roots *x509.CertPool) *Client {
	t.Helper()
	c, err := NewTLSClient(address, token, name, roots)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(c.Close)
	return c
}
func TestTLSRemoteExecutorAuthenticatedAndIdempotent(t *testing.T) {
	s, effect := tlsFixtureService(t)
	cert, roots := tlsFixtureCertificate(t)
	ep, err := s.ListenTLS("127.0.0.1:0", cert)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { ep.Close() })
	c := tlsFixtureClient(t, ep.Address(), s.cfg.Token, "executor.test", roots)
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	plan := Plan{Version: Version, OperationID: "op", JobID: "job", AttemptID: "attempt", ControllerID: "owner", EnvironmentID: "tls-test", JobName: "effect"}
	first, err := c.Submit(ctx, plan)
	if err != nil {
		t.Fatal(err)
	}
	second, err := c.Submit(ctx, plan)
	if err != nil || second.Handle != first.Handle {
		t.Fatalf("duplicate identity: %+v %v", second, err)
	}
	var terminal Snapshot
	for {
		terminal, err = c.Inspect(ctx, first.Handle, 0)
		if err != nil {
			t.Fatal(err)
		}
		if terminal.State == "completed" || terminal.State == "failed" {
			break
		}
		select {
		case <-ctx.Done():
			t.Fatal(ctx.Err())
		case <-time.After(10 * time.Millisecond):
		}
	}
	if terminal.State != "completed" || string(terminal.Output) != "tls-output" || terminal.ExitCode != 0 {
		t.Fatalf("terminal acknowledgment %+v", terminal)
	}
	b, err := os.ReadFile(effect)
	if err != nil || string(b) != "one\n" {
		t.Fatalf("actual execution count %q %v", b, err)
	}
	for _, tc := range []struct {
		name, token, server string
		roots               *x509.CertPool
	}{
		{"wrong-name", s.cfg.Token, "wrong.test", roots},
		{"wrong-root", s.cfg.Token, "executor.test", func() *x509.CertPool { _, r := tlsFixtureCertificate(t); return r }()},
		{"wrong-token", "wrong-token-at-least-24-characters", "executor.test", roots},
	} {
		t.Run(tc.name, func(t *testing.T) {
			bad := tlsFixtureClient(t, ep.Address(), tc.token, tc.server, tc.roots)
			if _, e := bad.Inspect(ctx, first.Handle, 0); e == nil {
				t.Fatal("unauthorized client accepted")
			}
		})
	}
	wrongEnv := plan
	wrongEnv.EnvironmentID = "other"
	wrongEnv.JobID = "other"
	if _, err = c.Submit(ctx, wrongEnv); err == nil {
		t.Fatal("wrong environment accepted")
	}
	stale := first.Handle
	stale.ControllerID = "stale"
	if _, err = c.Cancel(ctx, stale); err == nil {
		t.Fatal("stale controller accepted")
	}
	// An idle TLS reader must not prevent endpoint Close from joining.
	idle, err := tls.DialWithDialer(&net.Dialer{Timeout: time.Second}, "tcp", ep.Address(), &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots, ServerName: "executor.test"})
	if err != nil {
		t.Fatal(err)
	}
	defer idle.Close()
	if err = ep.Close(); err != nil {
		t.Fatal(err)
	}
	if err = ep.Close(); err != nil {
		t.Fatal(err)
	}
	idle.SetReadDeadline(time.Now().Add(time.Second))
	if _, err = idle.Read(make([]byte, 1)); err == nil {
		t.Fatal("endpoint reader not closed")
	}
	if _, err = c.Inspect(ctx, first.Handle, 0); err == nil || !strings.Contains(err.Error(), "unavailable") {
		t.Fatalf("disconnect not explicit: %v", err)
	}
	// Network shutdown must not retire the independently owned service/UDS.
	local := NewClient(filepath.Join(s.cfg.Directory, "executor.sock"), s.cfg.Token)
	defer local.Close()
	if snap, e := local.Inspect(ctx, first.Handle, 0); e != nil || snap.State != "completed" {
		t.Fatalf("endpoint closed executor: %+v %v", snap, e)
	}
}
func TestTLSConfigurationRejected(t *testing.T) {
	s, _ := tlsFixtureService(t)
	cert, roots := tlsFixtureCertificate(t)
	for _, address := range []string{"", ":0", "http://127.0.0.1:1", "127.0.0.1:http", "127.0.0.1:65536"} {
		if ep, e := s.ListenTLS(address, cert); e == nil {
			ep.Close()
			t.Fatalf("accepted address %q", address)
		}
	}
	if ep, e := s.ListenTLS("127.0.0.1:0", tls.Certificate{}); e == nil {
		ep.Close()
		t.Fatal("missing certificate accepted")
	}
	mismatch, _ := tlsFixtureCertificate(t)
	mismatch.PrivateKey = cert.PrivateKey
	if ep, e := s.ListenTLS("127.0.0.1:0", mismatch); e == nil {
		ep.Close()
		t.Fatal("mismatched private key accepted")
	}
	for _, cfg := range []struct {
		address, name string
		roots         *x509.CertPool
	}{{"127.0.0.1:0", "executor.test", roots}, {"127.0.0.1:1", "", roots}, {"127.0.0.1:1", "executor.test", nil}, {"127.0.0.1:1", "executor.test", x509.NewCertPool()}} {
		if c, e := NewTLSClient(cfg.address, s.cfg.Token, cfg.name, cfg.roots); e == nil {
			c.Close()
			t.Fatal("invalid TLS client accepted")
		}
	}
}
