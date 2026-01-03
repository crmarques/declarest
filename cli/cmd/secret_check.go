package cmd

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"declarest/internal/metadata"
	"declarest/internal/reconciler"
	"declarest/internal/resource"

	"github.com/spf13/cobra"
)

func newSecretCheckCommand() *cobra.Command {
	var (
		path string
		fix  bool
	)

	cmd := &cobra.Command{
		Use:   "check [path]",
		Short: "Scan resource definitions for unmapped secrets",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			path, err = resolveOptionalArg(cmd, path, args, "path")
			if err != nil {
				return err
			}
			if path != "" {
				if err := validateLogicalPath(cmd, path); err != nil {
					return err
				}
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			targets, err := secretCheckTargets(recon, path)
			if err != nil {
				return err
			}
			if len(targets) == 0 {
				return nil
			}

			findings := map[string][]string{}
			resources := map[string]resource.Resource{}
			for _, target := range targets {
				res, err := recon.GetLocalResource(target)
				if err != nil {
					return err
				}
				resources[target] = res
				mapped, err := recon.SecretPathsFor(target)
				if err != nil {
					return err
				}
				unmapped := findUnmappedSecretPaths(res, mapped, resource.IsCollectionPath(target))
				if len(unmapped) > 0 {
					findings[target] = unmapped
				}
			}

			if len(findings) == 0 {
				successf(cmd, "no unmapped secrets found")
				return nil
			}

			ordered := sortedKeys(findings)
			for _, target := range ordered {
				paths := findings[target]
				sort.Strings(paths)
				infof(cmd, "%s:", target)
				for _, attr := range paths {
					infof(cmd, "  %s", attr)
				}
			}

			if !fix {
				return nil
			}
			if recon.SecretsManager == nil {
				fmt.Fprintln(cmd.ErrOrStderr(), "Secrets manager is not configured. Configure one and rerun with --fix.")
				return handledError{msg: "secrets manager is not configured"}
			}

			fixed := 0
			for _, target := range ordered {
				res := resources[target]
				if res.Kind() == resource.KindArray {
					fmt.Fprintf(cmd.ErrOrStderr(), "Skipping %s: collection resources cannot be fixed; save items instead.\n", target)
					continue
				}
				if err := recon.UpdateLocalMetadata(target, func(meta map[string]any) (bool, error) {
					return mergeSecretInAttributes(meta, findings[target])
				}); err != nil {
					return err
				}
				if err := saveLocalResourceWithSecrets(recon, target, res, true); err != nil {
					return err
				}
				fixed++
			}

			successf(cmd, "mapped secrets for %d resource(s)", fixed)
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource path to check (optional)")
	cmd.Flags().BoolVar(&fix, "fix", false, "Map detected secrets into metadata and store values in the secrets manager")

	return cmd
}

func secretCheckTargets(recon *reconciler.DefaultReconciler, path string) ([]string, error) {
	if recon == nil {
		return nil, errors.New("reconciler is not configured")
	}
	if strings.TrimSpace(path) == "" {
		return recon.RepositoryResourcePaths(), nil
	}
	if resource.IsCollectionPath(path) {
		return recon.RepositoryPathsInCollection(path)
	}
	return []string{path}, nil
}

func mergeSecretInAttributes(meta map[string]any, attrs []string) (bool, error) {
	if len(attrs) == 0 {
		return false, nil
	}
	existing, _ := resource.GetAttrPath(meta, "resourceInfo.secretInAttributes")
	current, err := secretAttributesFromValue(existing)
	if err != nil {
		return false, err
	}

	merged := append([]string{}, current...)
	ordered := append([]string{}, attrs...)
	sort.Strings(ordered)
	for _, attr := range ordered {
		attr = strings.TrimSpace(attr)
		if attr == "" {
			continue
		}
		if !containsString(merged, attr) {
			merged = append(merged, attr)
		}
	}

	if len(merged) == len(current) {
		return false, nil
	}
	return metadata.SetMetadataAttribute(meta, "resourceInfo.secretInAttributes", merged)
}

func secretAttributesFromValue(value any) ([]string, error) {
	switch typed := value.(type) {
	case nil:
		return nil, nil
	case []string:
		return append([]string{}, typed...), nil
	case []any:
		return coerceStringSlice(typed)
	default:
		return nil, fmt.Errorf("resourceInfo.secretInAttributes must be a list of strings")
	}
}

func containsString(items []string, value string) bool {
	for _, item := range items {
		if item == value {
			return true
		}
	}
	return false
}

func sortedKeys[V any](data map[string]V) []string {
	keys := make([]string, 0, len(data))
	for key := range data {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
