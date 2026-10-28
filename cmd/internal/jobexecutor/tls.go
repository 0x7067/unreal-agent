package jobexecutor

import (
	"bytes"
	"context"
	"crypto"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"net"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// TLSEndpoint owns an explicitly enabled network listener, its HTTP connections,
// and handlers. It does not own executor jobs. Close it before Service.Close.
// HTTPS is the only protocol on this listener; it has no redirect/cleartext port.
type TLSEndpoint struct {
	address     string
	server      *http.Server
	serveDone   chan struct{}
	connections sync.WaitGroup
	handlers    sync.WaitGroup
	once        sync.Once
	err         error
}

func tcpAddress(address string, listening bool) error {
	host, port, err := net.SplitHostPort(address)
	if err != nil || host == "" || strings.TrimSpace(host) != host {
		return errors.New("explicit TCP host:port required")
	}
	// Require a numeric port, not service-name lookup or an implicit destination.
	p, err := strconv.Atoi(port)
	if err != nil || p < 0 || p > 65535 || (!listening && p == 0) {
		return errors.New("invalid TCP port")
	}
	return nil
}

// ListenTLS opts into a second listener using precisely the existing Service
// handler, including its bearer-token, plan, identity and environment checks.
// Endpoint lifecycle is separate: caller must Close it before Service.Close.
func (s *Service) ListenTLS(address string, cert tls.Certificate) (*TLSEndpoint, error) {
	if err := tcpAddress(address, true); err != nil {
		return nil, err
	}
	if len(cert.Certificate) == 0 || cert.PrivateKey == nil {
		return nil, errors.New("TLS certificate and private key required")
	}
	// X509KeyPair checks public/private key agreement and supported key type.
	// tls.Certificate may contain a signer without serializable private material;
	// validate the leaf and key agreement via the signer's public key instead.
	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, errors.New("invalid TLS leaf certificate")
	}
	if time.Now().Before(leaf.NotBefore) || !time.Now().Before(leaf.NotAfter) {
		return nil, errors.New("TLS certificate is not currently valid")
	}
	if err = validateTLSKey(leaf, cert); err != nil {
		return nil, err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.closed || s.server == nil || len(s.cfg.ControllerIDs) == 0 {
		return nil, errors.New("executor unavailable")
	}
	listener, err := net.Listen("tcp", address)
	if err != nil {
		return nil, err
	}
	ep := &TLSEndpoint{address: listener.Addr().String(), serveDone: make(chan struct{})}
	ep.server = &http.Server{
		Handler: http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ep.handlers.Add(1)
			defer ep.handlers.Done()
			s.server.Handler.ServeHTTP(w, r)
		}),
		ReadHeaderTimeout: 2 * time.Second, ReadTimeout: 3 * time.Second, WriteTimeout: 5 * time.Second, IdleTimeout: 5 * time.Second, MaxHeaderBytes: 4096,
		TLSConfig: &tls.Config{MinVersion: tls.VersionTLS13, Certificates: []tls.Certificate{cert}, NextProtos: []string{"http/1.1"}},
		ConnState: func(_ net.Conn, state http.ConnState) {
			switch state {
			case http.StateNew:
				ep.connections.Add(1)
			case http.StateClosed, http.StateHijacked:
				ep.connections.Done()
			}
		},
	}
	go func() {
		defer close(ep.serveDone)
		_ = ep.server.Serve(tls.NewListener(limitNetworkListener(listener, 32), ep.server.TLSConfig))
	}()
	return ep, nil
}
func (e *TLSEndpoint) Address() string { return e.address }

// Close closes the listener and HTTP/TLS readers, cancels requests and joins all
// owned connections and handlers. Executor-owned jobs are deliberately untouched.
func (e *TLSEndpoint) Close() error {
	e.once.Do(func() { e.err = e.server.Close(); <-e.serveDone; e.connections.Wait(); e.handlers.Wait() })
	return e.err
}

// NewTLSClient uses the existing bounded executor protocol over verified TLS.
// The HTTP protocol's internal fixed URL is never a routing/fallback authority:
// every dial is TLS to the explicitly supplied address. Redirects are rejected.
func NewTLSClient(address, token, serverName string, roots *x509.CertPool) (*Client, error) {
	if err := tcpAddress(address, false); err != nil {
		return nil, err
	}
	if roots == nil || len(roots.Subjects()) == 0 || serverName == "" || strings.TrimSpace(serverName) != serverName || strings.ContainsAny(serverName, "/\\\x00\r\n") || len(token) < 24 || len(token) > 1024 || strings.ContainsAny(token, "\r\n") {
		return nil, errors.New("explicit TLS roots, server identity and valid token required")
	}
	cfg := &tls.Config{MinVersion: tls.VersionTLS13, RootCAs: roots.Clone(), ServerName: serverName, NextProtos: []string{"http/1.1"}}
	dialer := &tls.Dialer{NetDialer: &net.Dialer{Timeout: time.Second}, Config: cfg}
	transport := &http.Transport{DisableKeepAlives: true, DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
		return dialer.DialContext(ctx, "tcp", address)
	}, ResponseHeaderTimeout: 3 * time.Second}
	return &Client{http: &http.Client{Transport: transport, Timeout: 4 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return errors.New("executor redirects forbidden") }}, transport: transport, token: token}, nil
}

func validateTLSKey(leaf *x509.Certificate, cert tls.Certificate) error {
	signer, ok := cert.PrivateKey.(crypto.Signer)
	if !ok {
		return errors.New("unsupported TLS private key")
	}
	a, err := x509.MarshalPKIXPublicKey(leaf.PublicKey)
	if err != nil {
		return err
	}
	b, err := x509.MarshalPKIXPublicKey(signer.Public())
	if err != nil {
		return err
	}
	if !bytes.Equal(a, b) {
		return errors.New("TLS private key does not match certificate")
	}
	return nil
}
