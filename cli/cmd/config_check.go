package cmd

import (
	"errors"
	"fmt"

	ctx "github.com/crmarques/declarest/context"

	"github.com/spf13/cobra"
)

func newConfigCheckCommand(manager ctx.ContextManager) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Verify configuration and connectivity",
		RunE: func(cmd *cobra.Command, args []string) error {
			if manager == nil {
				reportCheck(cmd, "Load default context", errors.New("context manager is not configured"))
				return handledError{msg: "configuration check failed"}
			}

			context, err := ctx.LoadContextWithEnv(manager)
			if err != nil {
				reportCheck(cmd, "Load default context", err)
				return handledError{msg: "configuration check failed"}
			}

			recon := context.Reconciler
			if recon == nil {
				reportCheck(cmd, "Load default context", errors.New("reconciler is not configured"))
				return handledError{msg: "configuration check failed"}
			}
			defer recon.Close()

			failed := false

			reportCheck(cmd, fmt.Sprintf("Loaded context %q", context.Name), nil)

			if err := recon.CheckLocalRepositoryAccess(); err != nil {
				failed = true
				reportCheck(cmd, "Repository access", err)
			} else {
				reportCheck(cmd, "Repository access", nil)
			}

			remoteConfigured, remoteEmpty, err := recon.CheckRemoteAccess()
			if err != nil {
				failed = true
				reportCheck(cmd, "Remote repository access", err)
			} else if remoteEmpty {
				reportCheck(cmd, "Remote repository access (empty)", nil)
			} else if !remoteConfigured {
				reportCheck(cmd, "Remote repository not configured", nil)
			} else {
				reportCheck(cmd, "Remote repository access", nil)
			}

			if !recon.ManagedServerConfigured() {
				reportCheck(cmd, "Managed server not configured", nil)
			} else if err := recon.CheckManagedServerAccess(); err != nil {
				failed = true
				reportCheck(cmd, "Managed server access", err)
			} else {
				reportCheck(cmd, "Managed server access", nil)
			}

			if !recon.SecretsConfigured() {
				reportCheck(cmd, "Secret store not configured", nil)
			} else if err := recon.InitSecrets(); err != nil {
				failed = true
				reportCheck(cmd, "Secret store access", err)
			} else {
				reportCheck(cmd, "Secret store access", nil)
			}

			reportCheckStatus(cmd, "Authentication validation", checkStatusSkipped, errors.New("config check does not validate authentication"))

			if failed {
				return handledError{msg: "configuration check failed"}
			}
			return nil
		},
	}
}
