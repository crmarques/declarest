package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"net/http"
	"os"
	"strings"

	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/reconciler"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/secrets"

	"github.com/spf13/cobra"
)

func newResourceCreateCommand() *cobra.Command {
	var (
		path string
		all  bool
		sync bool
	)

	cmd := &cobra.Command{
		Use:   "create <path>",
		Short: "Create the remote resource using the repository definition",
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath, err := resolvePathOrAll(cmd, path, all, args)
			if err != nil {
				return err
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			paths := []string{targetPath}
			if all {
				paths, err = recon.RepositoryResourcePathsWithErrors()
				if err != nil {
					return err
				}
				if len(paths) == 0 {
					return nil
				}
			}

			for _, target := range paths {
				data, err := recon.GetLocalResource(target)
				if err != nil {
					return err
				}

				if err := recon.CreateRemoteResource(target, data); err != nil {
					return wrapRemoteErrorWithDetails(err, target)
				}

				if sync {
					if err := syncLocalResource(recon, target); err != nil {
						return err
					}
				}

				if debugEnabled(debugGroupResource) {
					successf(cmd, "created remote resource %s", target)
					_ = printResourceJSON(cmd, data)
				} else {
					successf(cmd, "created remote resource %s", target)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource path to create")
	cmd.Flags().BoolVar(&all, "all", false, "Create all resources from the resource repository")
	cmd.Flags().BoolVar(&sync, "sync", false, "After creating, fetch the remote resource and save it in the resource repository")

	registerResourcePathCompletion(cmd, resourceRepoPathStrategy)
	return cmd
}

func newResourceUpdateCommand() *cobra.Command {
	var (
		path string
		all  bool
		sync bool
	)

	cmd := &cobra.Command{
		Use:   "update <path>",
		Short: "Update the remote resource using the repository definition",
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath, err := resolvePathOrAll(cmd, path, all, args)
			if err != nil {
				return err
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			paths := []string{targetPath}
			if all {
				paths, err = recon.RepositoryResourcePathsWithErrors()
				if err != nil {
					return err
				}
				if len(paths) == 0 {
					return nil
				}
			}

			for _, target := range paths {
				data, err := recon.GetLocalResource(target)
				if err != nil {
					return err
				}

				if err := recon.UpdateRemoteResource(target, data); err != nil {
					return wrapRemoteErrorWithDetails(err, target)
				}

				if sync {
					if err := syncLocalResource(recon, target); err != nil {
						return err
					}
				}
				if debugEnabled(debugGroupResource) {
					successf(cmd, "updated remote resource %s", target)
					_ = printResourceJSON(cmd, data)
				} else {
					successf(cmd, "updated remote resource %s", target)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource path to update")
	cmd.Flags().BoolVar(&all, "all", false, "Update all resources from the resource repository")
	cmd.Flags().BoolVar(&sync, "sync", false, "After updating, fetch the remote resource and save it in the resource repository")

	registerResourcePathCompletion(cmd, resourceRepoPathStrategy)

	return cmd
}

func newResourceApplyCommand() *cobra.Command {
	var (
		path string
		all  bool
		sync bool
	)

	cmd := &cobra.Command{
		Use:   "apply <path>",
		Short: "Create or update the remote resource using the repository definition",
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath, err := resolvePathOrAll(cmd, path, all, args)
			if err != nil {
				return err
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			paths := []string{targetPath}
			if all {
				paths, err = recon.RepositoryResourcePathsWithErrors()
				if err != nil {
					return err
				}
				if len(paths) == 0 {
					return nil
				}
			}

			for _, target := range paths {
				data, err := recon.GetLocalResource(target)
				if err != nil {
					return err
				}

				if err := recon.SaveRemoteResource(target, data); err != nil {
					return wrapRemoteErrorWithDetails(err, target)
				}

				if sync {
					if err := syncLocalResource(recon, target); err != nil {
						return err
					}
					if res, err := recon.GetLocalResource(target); err == nil {
						data = res
					}
				}

				if debugEnabled(debugGroupResource) {
					successf(cmd, "applied remote resource %s", target)
					_ = printResourceJSON(cmd, data)
				} else {
					successf(cmd, "applied remote resource %s", target)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource path to apply")
	cmd.Flags().BoolVar(&all, "all", false, "Apply all resources from the resource repository")
	cmd.Flags().BoolVar(&sync, "sync", false, "After applying, fetch the remote resource and save it in the resource repository")

	registerResourcePathCompletion(cmd, resourceRepoPathStrategy)

	return cmd
}

func newResourceDeleteCommand() *cobra.Command {
	var (
		path         string
		all          bool
		repo         bool
		remote       bool
		resourceList bool
		allItems     bool
		yes          bool
	)

	cmd := &cobra.Command{
		Use:   "delete <path>",
		Short: "Delete resources from the resource repository, remote resources, or both",
		RunE: func(cmd *cobra.Command, args []string) error {
			targetPath, err := resolvePathOrAll(cmd, path, all, args)
			if err != nil {
				return err
			}

			isCollection := !all && resource.IsCollectionPath(targetPath)
			resourceListChanged := cmd.Flags().Changed("resource-list")

			if all {
				if resourceListChanged || allItems {
					return usageError(cmd, "--resource-list and --all-items require --path")
				}
			} else {
				if resourceListChanged && !isCollection {
					return usageError(cmd, "--resource-list requires a collection path")
				}
				if allItems && !isCollection {
					return usageError(cmd, "--all-items requires a collection path")
				}
				if (resourceListChanged || allItems) && !repo {
					return usageError(cmd, "--resource-list and --all-items require --repo")
				}
				if repo && isCollection && !resourceListChanged {
					resourceList = true
				}
				if repo && isCollection && !resourceList && !allItems && !remote {
					return usageError(cmd, "no delete targets specified for collection path")
				}
			}

			if err := ensureDeleteTargets(cmd, remote, repo); err != nil {
				return err
			}

			confirmMessage := resourceDeleteConfirmationMessage(targetPath, all, isCollection, repo, remote, resourceList, allItems)
			if err := confirmAction(cmd, yes, confirmMessage); err != nil {
				return err
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			paths := []string{targetPath}
			if all {
				paths, err = recon.RepositoryResourcePathsWithErrors()
				if err != nil {
					return err
				}
				if len(paths) == 0 {
					return nil
				}
			}

			for _, target := range paths {
				deletedLocal := false
				deletedRemote := false

				if remote {
					if err := recon.DeleteRemoteResource(target); err != nil {
						return wrapRemoteErrorWithDetails(err, target)
					}
					deletedRemote = true
				}

				if repo {
					if !isCollection || resourceList {
						if err := recon.DeleteLocalResource(target); err != nil {
							return err
						}
						deletedLocal = true
					}
				}

				switch {
				case deletedLocal && deletedRemote:
					successf(cmd, "deleted resource %s from the resource repository and remote resource", target)
				case deletedRemote:
					successf(cmd, "deleted remote resource %s", target)
				case deletedLocal:
					successf(cmd, "deleted resource %s from the resource repository", target)
				}
			}

			if allItems {
				itemPaths, err := recon.RepositoryPathsInCollection(targetPath)
				if err != nil {
					return err
				}
				base := strings.TrimRight(resource.NormalizePath(targetPath), "/")
				for _, item := range itemPaths {
					if item == base {
						continue
					}
					if err := recon.DeleteLocalResource(item); err != nil {
						return err
					}
					successf(cmd, "deleted resource %s from the resource repository", item)
				}
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource path to delete")
	cmd.Flags().BoolVar(&all, "all", false, "Delete all resources from the resource repository")
	cmd.Flags().BoolVar(&repo, "repo", true, "Delete from the resource repository (default unless --remote is set)")
	cmd.Flags().BoolVar(&remote, "remote", false, "Delete remote resources")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompts")
	cmd.Flags().BoolVar(&resourceList, "resource-list", false, "When used with --repo on a collection path, delete the collection list entry from the resource repository")
	cmd.Flags().BoolVar(&allItems, "all-items", false, "When used with --repo on a collection path, delete all saved collection items from the resource repository")

	registerResourcePathCompletion(cmd, resourceDeletePathStrategy)

	return cmd
}

func resourceDeleteConfirmationMessage(targetPath string, all, isCollection, repo, remote, resourceList, allItems bool) string {
	target := "resource"
	switch {
	case all:
		target = "all resources"
	case isCollection && resourceList && allItems:
		target = fmt.Sprintf("collection %s and all items under it", targetPath)
	case isCollection && allItems:
		target = fmt.Sprintf("all items under collection %s", targetPath)
	case isCollection && resourceList:
		target = fmt.Sprintf("collection entry %s", targetPath)
	case isCollection:
		target = fmt.Sprintf("collection %s", targetPath)
	default:
		target = fmt.Sprintf("resource %s", targetPath)
	}
	return fmt.Sprintf("Delete %s. %s Continue?", target, impactSummary(repo, remote))
}

func ensureDeleteTargets(cmd *cobra.Command, remote, repo bool) error {
	if !remote && !repo {
		return usageError(cmd, "at least one of --remote or --repo must be true")
	}
	return nil
}

func resolvePathOrAll(cmd *cobra.Command, path string, all bool, args []string) (string, error) {
	trimmed, err := resolveOptionalArg(cmd, path, args, "path")
	if err != nil {
		return "", err
	}
	if all {
		if strings.TrimSpace(trimmed) != "" {
			return "", usageError(cmd, "--all cannot be used with --path")
		}
		return "", nil
	}
	if strings.TrimSpace(trimmed) == "" {
		return "", usageError(cmd, "path is required unless --all is set")
	}
	if err := validateLogicalPath(cmd, trimmed); err != nil {
		return "", err
	}
	return trimmed, nil
}

func warnUnmappedSecrets(cmd *cobra.Command, path string, res resource.Resource, secretPaths []string) {
	unmapped := secrets.FindUnmappedSecretPaths(res, secretPaths, resource.IsCollectionPath(path))
	if len(unmapped) == 0 {
		return
	}
	fmt.Fprintf(cmd.ErrOrStderr(), "Warning: potential secrets in %s are not mapped to resourceInfo.secretInAttributes:\n", path)
	for _, attr := range unmapped {
		fmt.Fprintf(cmd.ErrOrStderr(), "  - %s\n", attr)
	}
	fmt.Fprintln(cmd.ErrOrStderr(), "Run `declarest secret check` to review or `declarest secret check --fix` to map and store them.")
}

func syncLocalResource(recon reconciler.AppReconciler, path string) error {
	res, err := recon.GetRemoteResource(path)
	if err != nil {
		if managedserver.IsNotFoundError(err) {
			return nil
		}
		return wrapRemoteErrorWithDetails(err, path)
	}
	return saveLocalResourceWithSecrets(recon, path, res, true)
}

func ensureRepositoryOverwriteAllowed(recon reconciler.AppReconciler, path string, data resource.Resource, force bool) error {
	if force || recon == nil {
		return nil
	}
	targetPath := path
	resolvedTargetPath, err := resolveAddTargetPath(recon, path, data)
	if err != nil {
		return err
	}
	if strings.TrimSpace(resolvedTargetPath) != "" {
		targetPath = resolvedTargetPath
	}

	_, err = recon.GetLocalResource(targetPath)
	if err == nil {
		if resource.NormalizePath(targetPath) != resource.NormalizePath(path) {
			return fmt.Errorf("resource %s already exists in the resource repository (resolved from %s); use --force to overwrite", targetPath, path)
		}
		return fmt.Errorf("resource %s already exists in the resource repository; use --force to override", targetPath)
	}
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, os.ErrNotExist) {
		return nil
	}
	return err
}

func newResourceDiffCommand() *cobra.Command {
	var (
		path string
		fail bool
	)

	cmd := &cobra.Command{
		Use:   "diff <path>",
		Short: "Show the reconcile diff for a resource",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			path, err = resolveSingleArg(cmd, path, args, "path")
			if err != nil {
				return err
			}
			if err := validateLogicalPath(cmd, path); err != nil {
				return err
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			patch, err := recon.DiffResource(path)
			if err != nil {
				return wrapRemoteErrorWithDetails(err, path)
			}

			if len(patch) == 0 {
				successf(cmd, "resource %s is in sync", path)
				return nil
			}

			if err := PrintPatchSummary(cmd, patch); err != nil {
				return err
			}
			if fail {
				return fmt.Errorf("resource %s is out of sync", path)
			}
			successf(cmd, "diff generated for %s", path)
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource path to diff")
	registerResourcePathCompletion(cmd, resourceRepoPathStrategy)
	cmd.Flags().BoolVar(&fail, "fail", false, "Exit with error if the resource is not in sync")
	return cmd
}

func printResourceJSON(cmd *cobra.Command, res resource.Resource) error {
	data, err := res.MarshalJSON()
	if err != nil {
		return err
	}

	var buf bytes.Buffer
	if err := json.Indent(&buf, data, "", "  "); err != nil {
		return err
	}
	buf.WriteByte('\n')

	_, err = cmd.OutOrStdout().Write(buf.Bytes())
	return err
}

func PrintPatchSummary(cmd *cobra.Command, patch resource.ResourcePatch) error {
	for _, op := range patch {
		verb := strings.ToLower(strings.TrimSpace(op.Op))
		if verb == "" {
			verb = "change"
		}
		if strings.TrimSpace(op.Path) == "" {
			fmt.Fprintln(cmd.OutOrStdout(), verb)
			continue
		}
		fmt.Fprintf(cmd.OutOrStdout(), "%s %s\n", verb, op.Path)
	}
	return nil
}

func wrapRemoteErrorWithDetails(err error, path string) error {
	var httpErr *managedserver.HTTPError
	if errors.As(err, &httpErr) {
		status := httpErr.Status()
		if status == 0 {
			status = http.StatusInternalServerError
		}
		statusText := http.StatusText(status)
		if statusText == "" {
			statusText = "Unknown"
		}
		if managedserver.IsNotFoundError(err) {
			return fmt.Errorf("remote resource %s not found (HTTP %d %s)", path, status, statusText)
		}
		return fmt.Errorf("remote server returned %d %s for %s", status, statusText, path)
	}
	return err
}
