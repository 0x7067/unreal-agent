package durablejob

import (
	"crypto/sha256"
	"crypto/x509"
	"encoding/hex"
	"encoding/json/v2"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"syscall"

	"github.com/unreallabsai/unreal-agent/cmd/internal/jobexecutor"
	"github.com/unreallabsai/unreal-agent/harness/session"
)

const ConfigEnvironment = "UNREAL_AGENT_JOB_CONFIG"
const TokenEnvironment = "UNREAL_AGENT_EXECUTOR_TOKEN"

// RuntimeConfig is read only from an explicit trusted startup environment.
// Credentials stay in memory; paths/credentials are never operation plan data.
type RuntimeConfig struct {
	Version       int
	EnvironmentID string
	Socket        string
	TLSAddress    string
	TLSServerName string
	CAFile        string
	Jobs          []string
}

// ReadPrivateConfig refuses links, permissive modes, oversized and unknown data.
// Same-UID concurrent replacement of trusted configuration is outside this model.
func ReadPrivateFile(path string) ([]byte, error) {
	if !filepath.IsAbs(path) || filepath.Clean(path) != path {
		return nil, errors.New("trusted file must be absolute and clean")
	}
	for current := path; current != "/"; current = filepath.Dir(current) {
		st, err := os.Lstat(current)
		if err != nil {
			return nil, err
		}
		if st.Mode()&os.ModeSymlink != 0 {
			return nil, errors.New("trusted file parent links refused")
		}
	}
	st, err := os.Lstat(path)
	if err != nil {
		return nil, err
	}
	if !st.Mode().IsRegular() || st.Mode().Perm() != 0600 || st.Size() > 64<<10 {
		return nil, errors.New("trusted file must be regular private0600 and bounded")
	}
	if owner, ok := st.Sys().(*syscall.Stat_t); !ok || int(owner.Uid) != os.Geteuid() {
		return nil, errors.New("trusted file owner mismatch")
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	opened, err := f.Stat()
	if err != nil || !os.SameFile(st, opened) {
		return nil, errors.New("trusted file changed during open")
	}
	data, err := io.ReadAll(io.LimitReader(f, (64<<10)+1))
	if err != nil {
		return nil, err
	}
	if len(data) > 64<<10 {
		return nil, errors.New("trusted file oversized")
	}
	return data, nil
}
func ReadPrivateConfig(path string, out any) error {
	data, err := ReadPrivateFile(path)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, out, json.RejectUnknownMembers(true))
}

func Controller(id session.ID) string {
	sum := sha256.Sum256([]byte(id))
	return "session-" + hex.EncodeToString(sum[:16])
}
func (c RuntimeConfig) Validate() error {
	if c.Version != Version || !valid(c.EnvironmentID) || len(c.Jobs) == 0 || len(c.Jobs) > 16 {
		return errors.New("invalid trusted job configuration")
	}
	if c.TLSAddress != "" {
		host, port, err := net.SplitHostPort(c.TLSAddress)
		if err != nil || host == "" || port == "" || c.Socket != "" || c.TLSServerName == "" || !filepath.IsAbs(c.CAFile) {
			return errors.New("invalid explicit TLS environment configuration")
		}
	} else if !filepath.IsAbs(c.Socket) || len(c.Socket) > 100 || c.TLSServerName != "" || c.CAFile != "" {
		return errors.New("invalid private Unix environment configuration")
	}
	seen := map[string]bool{}
	for _, j := range c.Jobs {
		if !valid(j) || seen[j] {
			return errors.New("invalid/duplicate trusted job name")
		}
		seen[j] = true
	}
	return nil
}
func (c RuntimeConfig) Authority(id session.ID) Config {
	return Config{EnvironmentID: c.EnvironmentID, ControllerID: Controller(id), Jobs: append([]string(nil), c.Jobs...)}
}
func (c RuntimeConfig) Client(token string) (*jobexecutor.Client, error) {
	if err := c.Validate(); err != nil {
		return nil, err
	}
	if len(token) < 24 || len(token) > 1024 {
		return nil, errors.New("missing scoped executor credential")
	}
	if c.TLSAddress != "" {
		pem, err := ReadPrivateFile(c.CAFile)
		if err != nil {
			return nil, err
		}
		roots := x509.NewCertPool()
		if !roots.AppendCertsFromPEM(pem) {
			return nil, errors.New("TLS environment CA invalid")
		}
		return jobexecutor.NewTLSClient(c.TLSAddress, token, c.TLSServerName, roots)
	}
	return jobexecutor.NewClient(c.Socket, token), nil
}
