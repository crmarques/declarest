package cmd

import (
	"errors"
	"fmt"
	"net/http"

	"declarest/internal/managedserver"
	"declarest/internal/reconciler"
	"declarest/internal/repository"

	gittransport "github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/spf13/cobra"
)

type remoteAccessChecker interface {
	CheckRemoteAccess() (bool, error)
}

type remoteSyncChecker interface {
	CheckRemoteSync() (bool, bool, error)
}

type serverAccessChecker interface {
	CheckAccess() error
}

type localRepoChecker interface {
	IsLocalRepositoryInitialized() (bool, error)
}

type checkStatus string

const (
	checkStatusOK      checkStatus = "OK"
	checkStatusFailed  checkStatus = "FAILED"
	checkStatusSkipped checkStatus = "SKIPPED"
)

func reportCheck(cmd *cobra.Command, label string, err error) bool {
	status := checkStatusOK
	if err != nil {
		status = checkStatusFailed
	}
	return reportCheckStatus(cmd, label, status, err)
}

func reportCheckStatus(cmd *cobra.Command, label string, status checkStatus, err error) bool {
	switch status {
	case checkStatusFailed:
		if err != nil {
			fmt.Fprintf(cmd.OutOrStdout(), "[FAILED] %s: %v\n", label, err)
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "[FAILED] %s\n", label)
		}
		return false
	case checkStatusSkipped:
		fmt.Fprintf(cmd.OutOrStdout(), "[SKIPPED] %s\n", label)
		return true
	default:
		fmt.Fprintf(cmd.OutOrStdout(), "[OK] %s\n", label)
		return true
	}
}

func checkRemoteAccess(manager repository.ResourceRepositoryManager) (bool, bool, error) {
	if manager == nil {
		return false, false, errors.New("resource repository manager is not configured")
	}
	if checker, ok := manager.(remoteAccessChecker); ok {
		configured, err := checker.CheckRemoteAccess()
		if err != nil && errors.Is(err, gittransport.ErrEmptyRemoteRepository) {
			return configured, true, nil
		}
		return configured, false, err
	}
	return false, false, nil
}

func checkLocalRepositoryInitialized(manager repository.ResourceRepositoryManager) (bool, bool, error) {
	if manager == nil {
		return false, false, errors.New("resource repository manager is not configured")
	}
	if checker, ok := manager.(localRepoChecker); ok {
		initialized, err := checker.IsLocalRepositoryInitialized()
		return true, initialized, err
	}
	return false, false, nil
}

func checkRemoteSync(manager repository.ResourceRepositoryManager) (bool, bool, error) {
	if manager == nil {
		return false, false, errors.New("resource repository manager is not configured")
	}
	if checker, ok := manager.(remoteSyncChecker); ok {
		return checker.CheckRemoteSync()
	}
	return false, false, nil
}

func checkManagedServerAccess(manager managedserver.ResourceServerManager) error {
	if manager == nil {
		return errors.New("resource server manager is not configured")
	}

	if checker, ok := manager.(serverAccessChecker); ok {
		return checker.CheckAccess()
	}

	if err := manager.Init(); err != nil {
		return err
	}

	spec := managedserver.RequestSpec{
		Kind: managedserver.KindHTTP,
		HTTP: &managedserver.HTTPRequestSpec{
			Path: "/",
		},
	}

	_, err := manager.ResourceExists(spec)
	if err == nil {
		return nil
	}

	var httpErr *managedserver.HTTPError
	if errors.As(err, &httpErr) && httpErr.StatusCode == http.StatusMethodNotAllowed {
		return nil
	}
	return err
}

func closeReconciler(recon *reconciler.DefaultReconciler) {
	if recon == nil {
		return
	}
	if recon.ResourceRepositoryManager != nil {
		recon.ResourceRepositoryManager.Close()
	}
	if recon.ResourceServerManager != nil {
		recon.ResourceServerManager.Close()
	}
	if recon.SecretsManager != nil {
		recon.SecretsManager.Close()
	}
}
