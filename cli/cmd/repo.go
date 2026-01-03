package cmd

import (
	"errors"
	"fmt"

	ctx "declarest/internal/context"
	"declarest/internal/reconciler"

	"github.com/spf13/cobra"
)

func newRepoCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "repo",
		GroupID: groupUserFacing,
		Short:   "Manage the resource repository",
	}

	cmd.AddCommand(newRepoInitCommand())
	cmd.AddCommand(newRepoRefreshCommand())
	cmd.AddCommand(newRepoPushCommand())
	cmd.AddCommand(newRepoResetCommand())
	cmd.AddCommand(newRepoCheckCommand())

	return cmd
}

func newRepoInitCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "init",
		Short: "Initialise the resource repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			recon, cleanup, err := loadDefaultReconcilerSkippingRepoSync()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}
			if err := recon.InitRepositoryLocal(); err != nil {
				return err
			}
			successf(cmd, "initialised local repository")
			remoteConfigured, err := recon.InitRepositoryRemoteIfEmpty()
			if err != nil {
				return err
			}
			if remoteConfigured {
				successf(cmd, "initialised remote repository")
			} else {
				successf(cmd, "remote repository not configured")
			}
			return nil
		},
	}
}

func newRepoRefreshCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "refresh",
		Short: "Fast-forward the repository from the remote branch",
		RunE: func(cmd *cobra.Command, args []string) error {
			recon, cleanup, err := loadDefaultReconcilerSkippingRepoSync()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}
			if err := recon.RefreshRepository(); err != nil {
				return err
			}
			successf(cmd, "fast-forwarded repository")
			return nil
		},
	}
}

func newRepoPushCommand() *cobra.Command {
	var (
		force bool
		yes   bool
	)

	cmd := &cobra.Command{
		Use:     "push",
		Aliases: []string{"update-remote"},
		Short:   "Push repository changes to the remote repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			if force {
				message := fmt.Sprintf("Force-push rewrites remote history. %s Continue?", impactSummary(false, true))
				if err := confirmAction(cmd, yes, message); err != nil {
					return err
				}
			}

			recon, cleanup, err := loadDefaultReconcilerSkippingRepoSync()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			if err := recon.UpdateRemoteRepositoryWithForce(force); err != nil {
				return err
			}
			if force {
				successf(cmd, "force-pushed repository changes to remote")
			} else {
				successf(cmd, "pushed repository changes to remote")
			}
			return nil
		},
	}

	cmd.Flags().BoolVar(&force, "force", false, "Force-push local repository state to remote")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompts")

	return cmd
}

func newRepoResetCommand() *cobra.Command {
	var yes bool

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Hard reset the repository to the remote branch",
		RunE: func(cmd *cobra.Command, args []string) error {
			message := fmt.Sprintf("Hard reset discards local changes and resets to the remote branch. %s Continue?", impactSummary(true, false))
			if err := confirmAction(cmd, yes, message); err != nil {
				return err
			}

			recon, cleanup, err := loadDefaultReconcilerSkippingRepoSync()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}
			if err := recon.ResetRepository(); err != nil {
				return err
			}
			successf(cmd, "hard reset repository")
			return nil
		},
	}

	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompts")

	return cmd
}

func newRepoCheckCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Verify repository connectivity and sync status",
		RunE: func(cmd *cobra.Command, args []string) error {
			manager := &ctx.DefaultContextManager{}
			context, err := manager.LoadDefaultContext()
			if err != nil {
				reportCheck(cmd, "Load default context", err)
				return handledError{msg: "repository check failed"}
			}

			recon, ok := context.Reconciler.(*reconciler.DefaultReconciler)
			if !ok {
				reportCheck(cmd, "Load default context", errors.New("unexpected reconciler type"))
				return handledError{msg: "repository check failed"}
			}
			defer closeReconciler(recon)

			failed := false

			reportCheckStatus(cmd, fmt.Sprintf("Loaded context %q", context.Name), checkStatusOK, nil)

			localAccessOk := false
			if recon.ResourceRepositoryManager == nil {
				failed = true
				reportCheckStatus(cmd, "Local repository access", checkStatusFailed, errors.New("resource repository manager is not configured"))
			} else if err := recon.ResourceRepositoryManager.Init(); err != nil {
				failed = true
				reportCheckStatus(cmd, "Local repository access", checkStatusFailed, err)
			} else {
				localAccessOk = true
				reportCheckStatus(cmd, "Local repository access", checkStatusOK, nil)
			}

			localInitOk := false
			if !localAccessOk {
				reportCheckStatus(cmd, "Local repository initialized", checkStatusSkipped, nil)
			} else {
				supported, initialized, err := checkLocalRepositoryInitialized(recon.ResourceRepositoryManager)
				if err != nil {
					failed = true
					reportCheckStatus(cmd, "Local repository initialized", checkStatusFailed, err)
				} else if !supported {
					reportCheckStatus(cmd, "Local repository initialized", checkStatusSkipped, nil)
				} else if initialized {
					localInitOk = true
					reportCheckStatus(cmd, "Local repository initialized", checkStatusOK, nil)
				} else {
					failed = true
					reportCheckStatus(cmd, "Local repository initialized", checkStatusFailed, nil)
				}
			}

			remoteConfigured, remoteEmpty, err := checkRemoteAccess(recon.ResourceRepositoryManager)
			remoteAccessErr := err != nil
			remoteAccessOk := false
			if err != nil {
				failed = true
				reportCheckStatus(cmd, "Remote repository access", checkStatusFailed, err)
			} else if !remoteConfigured {
				reportCheckStatus(cmd, "Remote repository access (not configured)", checkStatusSkipped, nil)
			} else if remoteEmpty {
				remoteAccessOk = true
				reportCheckStatus(cmd, "Remote repository access (empty)", checkStatusOK, nil)
			} else {
				remoteAccessOk = true
				reportCheckStatus(cmd, "Remote repository access", checkStatusOK, nil)
			}

			remoteInitializedOk := false
			switch {
			case !remoteConfigured:
				reportCheckStatus(cmd, "Remote repository initialized", checkStatusSkipped, nil)
			case remoteAccessErr:
				failed = true
				reportCheckStatus(cmd, "Remote repository initialized", checkStatusFailed, err)
			case remoteEmpty:
				failed = true
				reportCheckStatus(cmd, "Remote repository initialized", checkStatusFailed, nil)
			case remoteAccessOk:
				remoteInitializedOk = true
				reportCheckStatus(cmd, "Remote repository initialized", checkStatusOK, nil)
			default:
				failed = true
				reportCheckStatus(cmd, "Remote repository initialized", checkStatusFailed, errors.New("remote access failed"))
			}

			if localInitOk && remoteInitializedOk {
				_, inSync, err := checkRemoteSync(recon.ResourceRepositoryManager)
				if err != nil {
					failed = true
					reportCheckStatus(cmd, "Remote and local repositories sync check", checkStatusFailed, err)
				} else if inSync {
					reportCheckStatus(cmd, "Remote and local repositories sync check", checkStatusOK, nil)
				} else {
					failed = true
					reportCheckStatus(cmd, "Remote and local repositories sync check", checkStatusFailed, errors.New("local and remote are out of sync"))
				}
			} else {
				reportCheckStatus(cmd, "Remote and local repositories sync check", checkStatusSkipped, nil)
			}

			if failed {
				return handledError{msg: "repository check failed"}
			}
			return nil
		},
	}
}
