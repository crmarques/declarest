package cmd

import (
	"errors"
	"fmt"

	ctx "declarest/internal/context"
	"declarest/internal/reconciler"

	"github.com/spf13/cobra"
)

func newConfigCheckCommand(manager *ctx.DefaultContextManager) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Verify configuration and connectivity",
		RunE: func(cmd *cobra.Command, args []string) error {
			if manager == nil {
				reportCheck(cmd, "Load default context", errors.New("context manager is not configured"))
				return handledError{msg: "configuration check failed"}
			}

			context, err := manager.LoadDefaultContext()
			if err != nil {
				reportCheck(cmd, "Load default context", err)
				return handledError{msg: "configuration check failed"}
			}

			recon, ok := context.Reconciler.(*reconciler.DefaultReconciler)
			if !ok {
				reportCheck(cmd, "Load default context", errors.New("unexpected reconciler type"))
				return handledError{msg: "configuration check failed"}
			}
			defer closeReconciler(recon)

			failed := false

			reportCheck(cmd, fmt.Sprintf("Loaded context %q", context.Name), nil)

			if recon.ResourceRepositoryManager == nil {
				failed = true
				reportCheck(cmd, "Repository access", errors.New("resource repository manager is not configured"))
			} else if err := recon.ResourceRepositoryManager.Init(); err != nil {
				failed = true
				reportCheck(cmd, "Repository access", err)
			} else {
				reportCheck(cmd, "Repository access", nil)
			}

			remoteConfigured, remoteEmpty, err := checkRemoteAccess(recon.ResourceRepositoryManager)
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

			if recon.ResourceServerManager == nil {
				reportCheck(cmd, "Managed server not configured", nil)
			} else if err := checkManagedServerAccess(recon.ResourceServerManager); err != nil {
				failed = true
				reportCheck(cmd, "Managed server access", err)
			} else {
				reportCheck(cmd, "Managed server access", nil)
			}

			if recon.SecretsManager == nil {
				reportCheck(cmd, "Secret store not configured", nil)
			} else if err := recon.SecretsManager.Init(); err != nil {
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
