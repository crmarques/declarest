package cmd

import (
	"errors"
	"fmt"
	"strings"
	"sync"

	ctx "declarest/internal/context"
	"declarest/internal/reconciler"
	"declarest/internal/resource"

	"github.com/spf13/cobra"
)

type handledError struct {
	msg string
}

func (handledError) handledMarker() {}

func (e handledError) Error() string {
	return e.msg
}

type handled interface {
	handledMarker()
}

type loadReconcilerOptions struct {
	skipRepoSync bool
}

func loadDefaultReconciler() (*reconciler.DefaultReconciler, func(), error) {
	return loadDefaultReconcilerWithOptions(loadReconcilerOptions{})
}

func loadDefaultReconcilerSkippingRepoSync() (*reconciler.DefaultReconciler, func(), error) {
	return loadDefaultReconcilerWithOptions(loadReconcilerOptions{skipRepoSync: true})
}

func loadDefaultReconcilerWithOptions(opts loadReconcilerOptions) (*reconciler.DefaultReconciler, func(), error) {
	manager := &ctx.DefaultContextManager{}
	context, err := manager.LoadDefaultContext()
	if err != nil {
		return nil, nil, err
	}

	actual, ok := context.Reconciler.(*reconciler.DefaultReconciler)
	if !ok {
		return nil, nil, errors.New("unexpected reconciler type")
	}
	if opts.skipRepoSync {
		actual.SkipRepositorySync = true
	}
	captureDebugContext(actual)

	if err := context.Init(); err != nil {
		return nil, nil, err
	}

	var once sync.Once
	cleanup := func() {
		once.Do(func() {
			if actual.ResourceRepositoryManager != nil {
				actual.ResourceRepositoryManager.Close()
			}
			if actual.ResourceServerManager != nil {
				actual.ResourceServerManager.Close()
			}
			if actual.SecretsManager != nil {
				actual.SecretsManager.Close()
			}
		})
	}

	return actual, cleanup, nil
}

func usageError(cmd *cobra.Command, message string) error {
	msg := strings.TrimSpace(message)
	if msg == "" {
		msg = "invalid command usage"
	}

	cmd.SilenceUsage = true
	cmd.SilenceErrors = true

	fmt.Fprint(cmd.ErrOrStderr(), cmd.UsageString())

	return handledError{msg: msg}
}

func validateLogicalPath(cmd *cobra.Command, path string) error {
	if err := resource.ValidateLogicalPath(path); err != nil {
		return usageError(cmd, err.Error())
	}
	return nil
}

func successf(cmd *cobra.Command, format string, args ...any) {
	if noStatusOutput {
		return
	}
	fmt.Fprintf(cmd.OutOrStdout(), "[OK] "+format+"\n", args...)
}

func infof(cmd *cobra.Command, format string, args ...any) {
	fmt.Fprintf(cmd.OutOrStdout(), format+"\n", args...)
}

func confirmAction(cmd *cobra.Command, skipPrompt bool, message string) error {
	if skipPrompt {
		return nil
	}
	prompt := newPrompter(cmd.InOrStdin(), cmd.ErrOrStderr())
	confirmed, err := prompt.confirm(message, false)
	if err != nil {
		return err
	}
	if !confirmed {
		fmt.Fprintln(cmd.ErrOrStderr(), "Aborted.")
		return handledError{msg: "operation cancelled"}
	}
	return nil
}

func impactSummary(repo, remote bool) string {
	return fmt.Sprintf("Repository: %s. Remote: %s.", yesNo(repo), yesNo(remote))
}

func yesNo(value bool) string {
	if value {
		return "yes"
	}
	return "no"
}

func resolveOptionalArg(cmd *cobra.Command, value string, args []string, label string) (string, error) {
	if len(args) > 1 {
		return "", usageError(cmd, fmt.Sprintf("expected <%s>", label))
	}
	value = strings.TrimSpace(value)
	if len(args) == 1 {
		arg := strings.TrimSpace(args[0])
		if arg != "" {
			if value != "" && value != arg {
				return "", usageError(cmd, fmt.Sprintf("%s specified twice", label))
			}
			if value == "" {
				value = arg
			}
		}
	}
	return value, nil
}

func IsHandledError(err error) bool {
	if err == nil {
		return false
	}
	var h handled
	return errors.As(err, &h)
}
