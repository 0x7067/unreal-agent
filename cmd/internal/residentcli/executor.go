package residentcli

import (
	"context"
	"crypto/tls"
	"encoding/json/v2"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"github.com/unreallabsai/unreal-agent/cmd/internal/durablejob"
	"github.com/unreallabsai/unreal-agent/cmd/internal/jobexecutor"
)

// A separately supervised trusted executor. It is never started by a worker,
// translator, context builder or model-selected path.
func runExecutor(ctx context.Context, args []string, getenv func(string) string, out, stderr io.Writer) error {
	flags := flag.NewFlagSet("job-executor", flag.ContinueOnError)
	flags.SetOutput(stderr)
	path := flags.String("config", "", "explicit private0600 trusted executor configuration")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *path == "" || flags.NArg() != 0 {
		return errors.New("job-executor requires --config")
	}
	var config struct {
		Version                       int
		Directory, EnvironmentID      string
		Jobs                          map[string]jobexecutor.TrustedJob
		ControllerIDs                 []string
		TLSAddress, CertFile, KeyFile string
	}
	if err := durablejob.ReadPrivateConfig(*path, &config); err != nil {
		return err
	}
	if config.Version != 1 {
		return errors.New("unsupported executor config version")
	}
	service, err := jobexecutor.Start(jobexecutor.Config{Directory: config.Directory, EnvironmentID: config.EnvironmentID, Token: getenv(durablejob.TokenEnvironment), Jobs: config.Jobs, ControllerIDs: config.ControllerIDs})
	if err != nil {
		return err
	}
	defer service.Close()
	var endpoint *jobexecutor.TLSEndpoint
	address := ""
	if config.TLSAddress != "" {
		certPEM, e := durablejob.ReadPrivateFile(config.CertFile)
		if e != nil {
			return e
		}
		keyPEM, e := durablejob.ReadPrivateFile(config.KeyFile)
		if e != nil {
			return e
		}
		cert, e := tls.X509KeyPair(certPEM, keyPEM)
		if e != nil {
			return e
		}
		endpoint, e = service.ListenTLS(config.TLSAddress, cert)
		if e != nil {
			return e
		}
		defer endpoint.Close() // drain transport before service/owned executor jobs
		address = endpoint.Address()
	} else if config.CertFile != "" || config.KeyFile != "" {
		return errors.New("TLS credentials without explicit network listener")
	}

	data, err := json.Marshal(struct {
		Version               int
		EnvironmentID, Socket string
		TLSAddress            string `json:",omitzero"`
		PID                   int
	}{1, config.EnvironmentID, config.Directory + "/executor.sock", address, os.Getpid()})
	if err != nil {
		return err
	}
	if _, err = fmt.Fprintln(out, string(data)); err != nil {
		return err
	}
	<-ctx.Done()
	if endpoint != nil {
		if err := endpoint.Close(); err != nil {
			return err
		}
	}
	return service.Close()
}
