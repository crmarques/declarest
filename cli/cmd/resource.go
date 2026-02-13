package cmd

import (
	"fmt"
	"github.com/crmarques/declarest/resource"
	"github.com/crmarques/declarest/secrets"

	"github.com/spf13/cobra"
)

func newResourceCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "resource",
		GroupID: groupUserFacing,
		Short:   "Operate on resources stored in the resource repository",
	}

	cmd.AddCommand(newResourceGetCommand())
	cmd.AddCommand(newResourceSaveCommand())
	cmd.AddCommand(newResourceExplainCommand())
	cmd.AddCommand(newResourceTemplateCommand())
	cmd.AddCommand(newResourceAddCommand())
	cmd.AddCommand(newResourceCreateCommand())
	cmd.AddCommand(newResourceUpdateCommand())
	cmd.AddCommand(newResourceApplyCommand())
	cmd.AddCommand(newResourceDeleteCommand())
	cmd.AddCommand(newResourceDiffCommand())
	cmd.AddCommand(newResourceListCommand())

	return cmd
}

func newResourceGetCommand() *cobra.Command {
	var (
		path        string
		print       bool
		fromRepo    bool
		withSecrets bool
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
				return wrapRemoteErrorWithDetails(err, path)
			}

			secretPaths, err := recon.SecretPathsFor(path)
			if err != nil {
				return err
			}
			warnUnmappedSecrets(cmd, path, res, secretPaths)

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

			if print {
				if err := printResourceJSON(cmd, output); err != nil {
					return err
				}
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
	cmd.Flags().BoolVar(&fromRepo, "repo", false, "Read the resource from the resource repository")
	cmd.Flags().BoolVar(&withSecrets, "with-secrets", false, "Include secrets in output (resolves repo placeholders via the secret store)")

	registerResourcePathCompletion(cmd, resourceGetPathStrategy)

	return cmd
}

func newResourceSaveCommand() *cobra.Command {
	var (
		path          string
		print         bool
		withSecrets   bool
		asOneResource bool
		force         bool
	)

	cmd := &cobra.Command{
		Use:   "save <path>",
		Short: "Fetch a remote resource and persist it in the resource repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			path, err = resolveSingleArg(cmd, path, args, "path")
			if err != nil {
				return err
			}
			if err := validateLogicalPath(cmd, path); err != nil {
				return err
			}
			if asOneResource && !resource.IsCollectionPath(path) {
				return usageError(cmd, "--as-one-resource requires a collection path")
			}
			if withSecrets && !force {
				return fmt.Errorf("refusing to save plaintext secrets without --force (saving secrets in the repository has security implications)")
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			res, err := recon.GetRemoteResource(path)
			if err != nil {
				return wrapRemoteErrorWithDetails(err, path)
			}

			secretPaths, err := recon.SecretPathsFor(path)
			if err != nil {
				return err
			}
			warnUnmappedSecrets(cmd, path, res, secretPaths)

			output := res
			if !withSecrets {
				output, err = recon.MaskResourceSecrets(path, res, false)
				if err != nil {
					return err
				}
			}

			if print {
				if err := printResourceJSON(cmd, output); err != nil {
					return err
				}
			}

			storeSecrets := !withSecrets

			if resource.IsCollectionPath(path) {
				if asOneResource {
					if err := ensureRepositoryOverwriteAllowed(recon, path, res, force); err != nil {
						return err
					}
					if err := saveLocalResourceWithSecrets(recon, path, res, storeSecrets); err != nil {
						return err
					}
					successf(cmd, "fetched remote collection %s and saved in the resource repository", path)
					return nil
				}

				items, ok := res.AsArray()
				if !ok {
					return usageError(cmd, "collection paths require a collection payload; use --as-one-resource to save the full response")
				}
				var resources []resource.Resource
				for _, item := range items {
					r, err := resource.NewResource(item)
					if err != nil {
						return err
					}
					resources = append(resources, r)
				}
				if err := saveLocalCollectionItemsWithSecrets(recon, path, resources, storeSecrets); err != nil {
					return err
				}
				successf(cmd, "fetched remote collection %s and saved %d items in the resource repository", path, len(resources))
				return nil
			}

			if err := ensureRepositoryOverwriteAllowed(recon, path, res, force); err != nil {
				return err
			}
			if err := saveLocalResourceWithSecrets(recon, path, res, storeSecrets); err != nil {
				return err
			}
			successf(cmd, "fetched remote resource %s and saved in the resource repository", path)
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource path to save")
	cmd.Flags().BoolVar(&print, "print", false, "Print the resource payload to stdout")
	cmd.Flags().BoolVar(&withSecrets, "with-secrets", false, "Include secrets in output (saving plaintext secrets requires --force)")
	cmd.Flags().BoolVar(&asOneResource, "as-one-resource", false, "Save a fetched collection as a single resource repository entry")
	cmd.Flags().BoolVar(&force, "force", false, "Allow saving plaintext secrets or overriding existing definitions in the resource repository")

	registerResourcePathCompletion(cmd, resourceRemotePathStrategy)

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
					paths, err = recon.RepositoryResourcePathsWithErrors()
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

	registerResourcePathCompletion(cmd, resourceListPathStrategy)

	return cmd
}
