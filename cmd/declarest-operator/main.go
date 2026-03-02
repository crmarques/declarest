package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/bootstrap"
	"github.com/crmarques/declarest/internal/operator"
)

func main() {
	var contextName string
	var path string
	var source string
	var showSecrets bool
	var showMetadata bool

	flag.StringVar(&contextName, "context", "", "context name")
	flag.StringVar(&path, "path", "", "resource logical path (required)")
	flag.StringVar(&source, "source", operator.SourceRemoteServer, "resource source: repository|remote-server")
	flag.BoolVar(&showSecrets, "show-secrets", false, "resolve and show secret values")
	flag.BoolVar(&showMetadata, "show-metadata", false, "include rendered metadata in output")
	flag.Parse()

	if strings.TrimSpace(path) == "" {
		_, _ = fmt.Fprintln(os.Stderr, "flag --path is required")
		os.Exit(2)
	}

	session, err := bootstrap.NewSession(
		bootstrap.BootstrapConfig{},
		config.ContextSelection{Name: strings.TrimSpace(contextName)},
	)
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, strings.TrimSpace(err.Error()))
		os.Exit(exitCodeForError(err))
	}

	result, err := operator.ReconcileOnce(context.Background(), operator.Dependencies{
		Orchestrator: session.Orchestrator,
		Contexts:     session.Contexts,
		Metadata:     session.Services.MetadataService(),
		Secrets:      session.Services.SecretProvider(),
	}, operator.ReconcileRequest{
		LogicalPath:  path,
		Source:       source,
		ContextName:  strings.TrimSpace(contextName),
		ShowSecrets:  showSecrets,
		ShowMetadata: showMetadata,
	})
	if err != nil {
		_, _ = fmt.Fprintln(os.Stderr, strings.TrimSpace(err.Error()))
		os.Exit(exitCodeForError(err))
	}

	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	if err := encoder.Encode(result); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, strings.TrimSpace(err.Error()))
		os.Exit(1)
	}
}

func exitCodeForError(err error) int {
	return faults.ExitCodeForError(err)
}
