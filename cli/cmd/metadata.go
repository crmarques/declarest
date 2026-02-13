package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/crmarques/declarest/metadata"
	"github.com/crmarques/declarest/openapi"
	"github.com/crmarques/declarest/resource"

	"github.com/spf13/cobra"
)

const metadataResourceOnlyFlag = "for-resource-only"
const metadataInferRecursiveFlag = "recursively"

func newMetadataCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "metadata",
		GroupID: groupUserFacing,
		Short:   "Manage resource metadata definitions in the repository",
	}

	cmd.PersistentFlags().Bool(metadataResourceOnlyFlag, false, "Treat the path as a resource when no trailing / is provided")

	cmd.AddCommand(newMetadataGetCommand())
	cmd.AddCommand(newMetadataEditCommand())
	cmd.AddCommand(newMetadataSetCommand())
	cmd.AddCommand(newMetadataUnsetCommand())
	cmd.AddCommand(newMetadataAddCommand())
	cmd.AddCommand(newMetadataUpdateResourcesCommand())
	cmd.AddCommand(newMetadataInferCommand())

	return cmd
}

func newMetadataEditCommand() *cobra.Command {
	var (
		path   string
		editor string
	)

	cmd := &cobra.Command{
		Use:   "edit <path>",
		Short: "Edit metadata using your editor with defaults prefilled",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			path, err = resolveSingleArg(cmd, path, args, "path")
			if err != nil {
				return err
			}
			path, err = normalizeMetadataInputPath(cmd, path)
			if err != nil {
				return err
			}
			if err := validateMetadataPath(cmd, path); err != nil {
				return err
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			mergedMeta, err := recon.MergedMetadata(path)
			if err != nil {
				return err
			}

			payload, err := formatMetadataOutput(mergedMeta)
			if err != nil {
				return err
			}
			payloadData, err := marshalMetadataEditPayloadWithComments(payload)
			if err != nil {
				return err
			}

			tempFile, err := os.CreateTemp("", "declarest-metadata-*.json")
			if err != nil {
				return fmt.Errorf("create temp metadata file: %w", err)
			}
			tempPath := tempFile.Name()
			if err := tempFile.Close(); err != nil {
				return fmt.Errorf("close temp metadata file: %w", err)
			}
			defer os.Remove(tempPath)

			if err := os.WriteFile(tempPath, payloadData, 0o644); err != nil {
				return fmt.Errorf("write temp metadata file: %w", err)
			}

			defaultEditor, err := metadataDefaultEditor()
			if err != nil {
				return err
			}
			editorArgs, err := resolveEditorCommand(editor, defaultEditor)
			if err != nil {
				return err
			}
			editorArgs = append(editorArgs, tempPath)
			if err := runEditor(editorArgs); err != nil {
				return err
			}

			editedData, err := os.ReadFile(tempPath)
			if err != nil {
				return fmt.Errorf("read edited metadata: %w", err)
			}
			editedMeta, err := decodeMetadataEditFile(editedData)
			if err != nil {
				return err
			}
			if err := metadata.ValidateMetadataDocument(editedMeta); err != nil {
				return fmt.Errorf("invalid metadata: %w", err)
			}

			defaultMeta, err := defaultMetadataForPath(path, recon.OpenAPISpec())
			if err != nil {
				return err
			}
			defaultPayload, err := formatMetadataOutput(defaultMeta)
			if err != nil {
				return err
			}
			defaultMap, err := payloadToMetadataMap(defaultPayload)
			if err != nil {
				return err
			}
			applyListCollectionTemplateDefaults(defaultMap)

			stripped := stripDefaultMetadata(editedMeta, defaultMap)
			if err := recon.UpdateLocalMetadata(path, func(meta map[string]any) (bool, error) {
				for key := range meta {
					delete(meta, key)
				}
				for key, value := range stripped {
					meta[key] = value
				}
				return true, nil
			}); err != nil {
				return err
			}

			successf(cmd, "updated metadata for %s", path)
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource or collection path to update (defaults to collection)")
	cmd.Flags().StringVar(&editor, "editor", "", "Editor command (defaults to vi)")

	registerResourcePathCompletion(cmd, resourceRepoPathStrategy)

	return cmd
}

func newMetadataGetCommand() *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:   "get <path>",
		Short: "Render the effective metadata for a resource or collection",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			path, err = resolveSingleArg(cmd, path, args, "path")
			if err != nil {
				return err
			}
			path, err = normalizeMetadataInputPath(cmd, path)
			if err != nil {
				return err
			}
			if err := validateMetadataPath(cmd, path); err != nil {
				return err
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			meta, err := recon.ResourceMetadata(path)
			if err != nil {
				return err
			}

			payload, err := formatMetadataOutput(meta)
			if err != nil {
				return err
			}
			data, err := json.MarshalIndent(payload, "", "  ")
			if err != nil {
				return err
			}
			data = append(data, '\n')
			if _, err := cmd.OutOrStdout().Write(data); err != nil {
				return err
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource or collection path to show metadata for (defaults to collection)")

	registerResourcePathCompletion(cmd, resourceRepoPathStrategy)

	return cmd
}

func defaultMetadataForPath(path string, spec *openapi.Spec) (resource.ResourceMetadata, error) {
	meta, isCollection := metadata.DefaultMetadataForPath(path, metadata.CollectionPathTemplate)
	meta = metadata.ApplyOpenAPIDefaults(meta, strings.TrimSpace(path), isCollection, resource.Resource{}, spec)
	return meta, nil
}

func newMetadataSetCommand() *cobra.Command {
	var (
		path      string
		attribute string
		value     string
	)

	cmd := &cobra.Command{
		Use:   "set <path>",
		Short: "Set a metadata attribute for a resource or collection",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			path, err = resolveSingleArg(cmd, path, args, "path")
			if err != nil {
				return err
			}
			path, err = normalizeMetadataInputPath(cmd, path)
			if err != nil {
				return err
			}
			if err := validateMetadataPath(cmd, path); err != nil {
				return err
			}

			attribute = strings.TrimSpace(attribute)
			if attribute == "" {
				return usageError(cmd, "--attribute is required")
			}
			if !cmd.Flags().Changed("value") {
				return usageError(cmd, "--value is required")
			}

			if isSecretInAttributesAttribute(attribute) {
				input, err := parseSecretInAttributesInput(value)
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

				if err := recon.UpdateLocalMetadata(path, func(meta map[string]any) (bool, error) {
					return setSecretInAttributes(meta, attribute, input)
				}); err != nil {
					return err
				}

				successf(cmd, "set metadata %s for %s", attribute, path)
				return nil
			}

			parsedValue, err := parseMetadataValue(value)
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

			if err := recon.UpdateLocalMetadata(path, func(meta map[string]any) (bool, error) {
				return metadata.SetMetadataAttribute(meta, attribute, parsedValue)
			}); err != nil {
				return err
			}

			successf(cmd, "set metadata %s for %s", attribute, path)
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource or collection path to update (defaults to collection)")
	cmd.Flags().StringVar(&attribute, "attribute", "", "Metadata attribute to set (dot-separated)")
	cmd.Flags().StringVar(&value, "value", "", "Metadata value (JSON literal or string)")

	registerResourcePathCompletion(cmd, resourceRepoPathStrategy)
	registerSecretAttributeValueCompletion(cmd)

	return cmd
}

func newMetadataUnsetCommand() *cobra.Command {
	var (
		path      string
		attribute string
		value     string
	)

	cmd := &cobra.Command{
		Use:   "unset <path>",
		Short: "Unset a metadata attribute for a resource or collection",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			path, err = resolveSingleArg(cmd, path, args, "path")
			if err != nil {
				return err
			}
			path, err = normalizeMetadataInputPath(cmd, path)
			if err != nil {
				return err
			}
			if err := validateMetadataPath(cmd, path); err != nil {
				return err
			}

			attribute = strings.TrimSpace(attribute)
			if attribute == "" {
				return usageError(cmd, "--attribute is required")
			}
			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			if cmd.Flags().Changed("value") {
				parsedValue, err := parseMetadataValue(value)
				if err != nil {
					return err
				}

				if err := recon.UpdateLocalMetadata(path, func(meta map[string]any) (bool, error) {
					return metadata.UnsetMetadataAttribute(meta, attribute, parsedValue)
				}); err != nil {
					return err
				}
			} else {
				if err := recon.UpdateLocalMetadata(path, func(meta map[string]any) (bool, error) {
					return metadata.DeleteMetadataAttribute(meta, attribute)
				}); err != nil {
					return err
				}
			}

			successf(cmd, "unset metadata %s for %s", attribute, path)
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource or collection path to update (defaults to collection)")
	cmd.Flags().StringVar(&attribute, "attribute", "", "Metadata attribute to unset (dot-separated)")
	cmd.Flags().StringVar(&value, "value", "", "Metadata value to remove (optional; omit to delete the attribute)")

	registerResourcePathCompletion(cmd, resourceRepoPathStrategy)
	registerSecretAttributeValueCompletion(cmd)

	return cmd
}

func newMetadataAddCommand() *cobra.Command {
	var (
		path     string
		filePath string
	)

	cmd := &cobra.Command{
		Use:   "add <path> <file>",
		Short: "Add a metadata definition from a JSON file",
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
			path, err := normalizeMetadataInputPath(cmd, path)
			if err != nil {
				return err
			}
			if err := validateMetadataPath(cmd, path); err != nil {
				return err
			}

			if filePath == "" {
				return usageError(cmd, "file path is required (use --file or positional argument)")
			}

			payload, err := os.ReadFile(filePath)
			if err != nil {
				return err
			}

			meta, err := decodeMetadataFile(payload)
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

			if err := recon.WriteLocalMetadata(path, meta); err != nil {
				return err
			}

			successf(cmd, "added metadata for %s", path)
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource or collection path to update (defaults to collection)")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to a JSON metadata file")

	registerResourcePathCompletion(cmd, resourceRepoPathStrategy)

	return cmd
}

func newMetadataUpdateResourcesCommand() *cobra.Command {
	var path string

	cmd := &cobra.Command{
		Use:   "update-resources <path>",
		Short: "Update local resources using the current metadata rules",
		RunE: func(cmd *cobra.Command, args []string) error {
			var err error
			path, err = resolveSingleArg(cmd, path, args, "path")
			if err != nil {
				return err
			}
			path, err = normalizeMetadataInputPath(cmd, path)
			if err != nil {
				return err
			}
			if err := validateMetadataPath(cmd, path); err != nil {
				return err
			}

			recon, cleanup, err := loadDefaultReconciler()
			if cleanup != nil {
				defer cleanup()
			}
			if err != nil {
				return err
			}

			results, err := recon.UpdateLocalResourcesForMetadata(path)
			if err != nil {
				return err
			}
			if len(results) == 0 {
				successf(cmd, "no resources updated for %s", path)
				return nil
			}

			for _, result := range results {
				if result.Moved && result.UpdatedPath != result.OriginalPath {
					successf(cmd, "updated resource %s (moved from %s)", result.UpdatedPath, result.OriginalPath)
					continue
				}
				successf(cmd, "updated resource %s", result.UpdatedPath)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource or collection path to update (defaults to collection)")

	registerResourcePathCompletion(cmd, resourceRepoPathStrategy)

	return cmd
}
