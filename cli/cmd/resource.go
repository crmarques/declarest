package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"

	"declarest/internal/managedserver"
	"declarest/internal/reconciler"
	"declarest/internal/resource"
	"declarest/internal/secrets"

	"github.com/spf13/cobra"
)

func newResourceCommand() *cobra.Command {
	var resourceVerbose bool

	cmd := &cobra.Command{
		Use:     "resource",
		GroupID: groupUserFacing,
		Short:   "Operate on resources stored in the resource repository",
	}

	cmd.PersistentFlags().BoolVar(&resourceVerbose, "verbose", false, "Print detailed information for debugging")

	cmd.AddCommand(newResourceGetCommand(resourceVerbose))
	cmd.AddCommand(newResourceAddCommand())
	cmd.AddCommand(newResourceCreateCommand(resourceVerbose))
	cmd.AddCommand(newResourceUpdateCommand(resourceVerbose))
	cmd.AddCommand(newResourceApplyCommand(resourceVerbose))
	cmd.AddCommand(newResourceDeleteCommand(resourceVerbose))
	cmd.AddCommand(newResourceDiffCommand(resourceVerbose))
	cmd.AddCommand(newResourceListCommand())

	return cmd
}

func newResourceGetCommand(verbose bool) *cobra.Command {
	var (
		path        string
		print       bool
		fromRepo    bool
		save        bool
		saveAsOne   bool
		withSecrets bool
		force       bool
	)

	cmd := &cobra.Command{
		Use:   "get <path>",
		Short: "Fetch a resource from the remote server (or repository)",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			path, err = resolveSingleArg(cmd, path, args, "path")
			if err != nil {
				return err
			}
			if err := validateLogicalPath(cmd, path); err != nil {
				return err
			}
			if fromRepo && (save || saveAsOne) {
				return usageError(cmd, "--from-repo cannot be combined with --save or --save-as-one-resource")
			}
			if saveAsOne && !save {
				return usageError(cmd, "--save-as-one-resource requires --save")
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			var res resource.Resource
			if fromRepo {
				res, err = recon.GetLocalResource(path)
			} else {
				res, err = recon.GetRemoteResource(path)
			}
			if err != nil {
				if fromRepo {
					return err
				}
				return wrapRemoteErrorWithDetails(err, path, verbose)
			}

			secretPaths, err := recon.SecretPathsFor(path)
			if err != nil {
				return err
			}
			unmapped := findUnmappedSecretPaths(res, secretPaths, resource.IsCollectionPath(path))
			if len(unmapped) > 0 {
				fmt.Fprintf(cmd.ErrOrStderr(), "Warning: potential secrets in %s are not mapped to resourceInfo.secretInAttributes:\n", path)
				for _, attr := range unmapped {
					fmt.Fprintf(cmd.ErrOrStderr(), "  - %s\n", attr)
				}
				fmt.Fprintln(cmd.ErrOrStderr(), "Run `declarest secret check` to review or `declarest secret check --fix` to map and store them.")
			}

			saveMode := save && !fromRepo
			saveCollectionItems := saveMode && resource.IsCollectionPath(path) && !saveAsOne

			if saveMode && withSecrets && !force {
				return fmt.Errorf("refusing to save plaintext secrets without --force (saving secrets in the repository has security implications)")
			}

			output := res
			if withSecrets {
				if fromRepo {
					secretPaths, err := secretPathsFor(recon, path)
					if err != nil {
						return err
					}
					hasPlaceholders, err := secrets.HasSecretPlaceholders(res, secretPaths)
					if err != nil {
						return err
					}
					if hasPlaceholders {
						output, err = recon.ResolveResourceSecrets(path, res)
						if err != nil {
							return err
						}
					}
				}
			} else {
				output, err = recon.MaskResourceSecrets(path, res, false)
				if err != nil {
					return err
				}
			}

			printOutput := print
			if saveMode && !cmd.Flags().Changed("print") {
				printOutput = false
			}

			if printOutput {
				if err := printResourceJSON(cmd, output); err != nil {
					return err
				}
			}

			if saveCollectionItems {
				items, ok := res.AsArray()
				if !ok {
					return usageError(cmd, "collection paths require a collection payload; use --save-as-one-resource to save the full response")
				}
				var resources []resource.Resource
				for _, item := range items {
					r, err := resource.NewResource(item)
					if err != nil {
						return err
					}
					resources = append(resources, r)
				}
				if err := saveLocalCollectionItemsWithSecrets(recon, path, resources, !withSecrets); err != nil {
					return err
				}
				successf(cmd, "fetched remote collection %s and saved %d items in the resource repository", path, len(resources))
				return nil
			}

			if saveMode {
				if err := saveLocalResourceWithSecrets(recon, path, res, !withSecrets); err != nil {
					return err
				}
				successf(cmd, "fetched remote resource %s and saved in the resource repository", path)
				return nil
			}

			if fromRepo {
				successf(cmd, "loaded resource %s from the repository", path)
			} else {
				successf(cmd, "fetched remote resource %s", path)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource path to read")
	cmd.Flags().BoolVar(&print, "print", true, "Print the resource payload to stdout")
	cmd.Flags().BoolVar(&fromRepo, "from-repo", false, "Read the resource from the resource repository")
	cmd.Flags().BoolVar(&save, "save", false, "Persist the remote resource into the resource repository (collection paths save items by default)")
	cmd.Flags().BoolVar(&saveAsOne, "save-as-one-resource", false, "Save a fetched collection as a single resource repository entry")
	cmd.Flags().BoolVar(&withSecrets, "with-secrets", false, "Include secrets in output (resolves repo placeholders via the secret store)")
	cmd.Flags().BoolVar(&force, "force", false, "Allow saving plaintext secrets into the resource repository")

	return cmd
}

func newResourceAddCommand() *cobra.Command {
	var (
		path     string
		filePath string
	)

	cmd := &cobra.Command{
		Use:   "add <path> <file>",
		Short: "Add a resource definition to the resource repository from a file",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 2 {
				return usageError(cmd, "expected <path> <file>")
			}
			path = strings.TrimSpace(path)
			filePath = strings.TrimSpace(filePath)
			if len(args) > 0 {
				argPath := strings.TrimSpace(args[0])
				if argPath != "" {
					if path != "" && path != argPath {
						return usageError(cmd, "path specified twice")
					}
					if path == "" {
						path = argPath
					}
				}
			}
			if len(args) > 1 {
				argFile := strings.TrimSpace(args[1])
				if argFile != "" {
					if filePath != "" && filePath != argFile {
						return usageError(cmd, "file specified twice")
					}
					if filePath == "" {
						filePath = argFile
					}
				}
			}
			if path == "" {
				return usageError(cmd, "path is required")
			}
			if err := validateLogicalPath(cmd, path); err != nil {
				return err
			}

			if filePath == "" {
				return usageError(cmd, "file is required")
			}

			payload, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}
			res, err := resource.NewResourceFromJSON(payload)
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

			if err := saveLocalResourceWithSecrets(recon, path, res, true); err != nil {
				return err
			}
			successf(cmd, "added resource %s to the resource repository", path)
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource path to add")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to a JSON resource payload file")

	return cmd
}

func newResourceListCommand() *cobra.Command {
	var (
		path       string
		listRepo   bool
		listRemote bool
	)

	cmd := &cobra.Command{
		Use:   "list [path]",
		Short: "List resource paths from the resource repository or remote server",
		Long:  "List resource paths from the resource repository or remote server. When --remote is set without --path, DeclaREST enumerates collection paths from the resource repository to drive remote listing.",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			path, err = resolveOptionalArg(cmd, path, args, "path")
			if err != nil {
				return err
			}

			repoChanged := cmd.Flags().Changed("repo")
			if listRemote && listRepo && !repoChanged {
				listRepo = false
			}
			if listRepo && listRemote {
				return usageError(cmd, "--repo and --remote cannot be used together")
			}
			if !listRepo && !listRemote {
				return usageError(cmd, "at least one of --repo or --remote must be true")
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			var paths []string
			if path != "" {
				if err := validateLogicalPath(cmd, path); err != nil {
					return err
				}
				if listRemote {
					paths, err = recon.ListRemoteResourcePaths(path)
				} else {
					paths, err = recon.RepositoryPathsInCollection(path)
				}
			} else {
				if listRemote {
					paths, err = recon.ListRemoteResourcePathsFromLocal()
				} else {
					paths = recon.RepositoryResourcePaths()
				}
			}
			if err != nil {
				return err
			}

			if len(paths) == 0 {
				return nil
			}

			for _, path := range paths {
				infof(cmd, "%s", path)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Collection path to list (optional)")
	cmd.Flags().BoolVar(&listRepo, "repo", true, "List resources from the resource repository (default)")
	cmd.Flags().BoolVar(&listRemote, "remote", false, "List resources from the remote server (uses resource repository collection metadata when --path is omitted)")

	return cmd
}

func newResourceCreateCommand(verbose bool) *cobra.Command {
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
				paths = recon.RepositoryResourcePaths()
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
					return wrapRemoteErrorWithDetails(err, target, verbose)
				}

				if sync {
					if err := syncLocalResource(recon, target, verbose); err != nil {
						return err
					}
				}

				if verbose {
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
	return cmd
}

func newResourceUpdateCommand(verbose bool) *cobra.Command {
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
				paths = recon.RepositoryResourcePaths()
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
					return wrapRemoteErrorWithDetails(err, target, verbose)
				}

				if sync {
					if err := syncLocalResource(recon, target, verbose); err != nil {
						return err
					}
				}
				if verbose {
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
	return cmd
}

func newResourceApplyCommand(verbose bool) *cobra.Command {
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
				paths = recon.RepositoryResourcePaths()
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
					return wrapRemoteErrorWithDetails(err, target, verbose)
				}

				if sync {
					if err := syncLocalResource(recon, target, verbose); err != nil {
						return err
					}
					if res, err := recon.GetLocalResource(target); err == nil {
						data = res
					}
				}

				if verbose {
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
	return cmd
}

func newResourceDeleteCommand(verbose bool) *cobra.Command {
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
				paths = recon.RepositoryResourcePaths()
				if len(paths) == 0 {
					return nil
				}
			}

			for _, target := range paths {
				deletedLocal := false
				deletedRemote := false

				if remote {
					if err := recon.DeleteRemoteResource(target); err != nil {
						return wrapRemoteErrorWithDetails(err, target, verbose)
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
	cmd.Flags().BoolVar(&repo, "repo", true, "Delete from the resource repository (default)")
	cmd.Flags().BoolVar(&remote, "remote", false, "Delete remote resources")
	cmd.Flags().BoolVar(&repo, "local", true, "Delete from the resource repository (default)")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompts")
	cmd.Flags().BoolVar(&yes, "force", false, "DEPRECATED: use --yes")
	cmd.Flags().BoolVar(&resourceList, "resource-list", false, "When used with --repo on a collection path, delete the collection list entry from the resource repository")
	cmd.Flags().BoolVar(&allItems, "all-items", false, "When used with --repo on a collection path, delete all saved collection items from the resource repository")
	_ = cmd.Flags().MarkHidden("local")
	_ = cmd.Flags().MarkHidden("force")

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

func syncLocalResource(recon *reconciler.DefaultReconciler, path string, verbose bool) error {
	res, err := recon.GetRemoteResource(path)
	if err != nil {
		if managedserver.IsNotFoundError(err) {
			return nil
		}
		return wrapRemoteErrorWithDetails(err, path, verbose)
	}
	return saveLocalResourceWithSecrets(recon, path, res, true)
}

func newResourceDiffCommand(verbose bool) *cobra.Command {
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
				if verbose {
					return wrapRemoteErrorWithDetails(err, path, verbose)
				}
				return err
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

func wrapRemoteErrorWithDetails(err error, path string, verbose bool) error {
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
			if verbose {
				return fmt.Errorf("remote resource %s not found (HTTP %d %s, url: %s, body: %s)", path, status, statusText, httpErr.URL, string(httpErr.Body))
			}
			return fmt.Errorf("remote resource %s not found (HTTP %d %s)", path, status, statusText)
		}
		if verbose {
			return fmt.Errorf("remote server returned %d %s for %s (url: %s, body: %s)", status, statusText, path, httpErr.URL, string(httpErr.Body))
		}
		return fmt.Errorf("remote server returned %d %s for %s", status, statusText, path)
	}
	return err
}
