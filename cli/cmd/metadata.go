package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"strings"

	"declarest/internal/metadata"
	"declarest/internal/resource"

	"github.com/spf13/cobra"
)

const metadataResourceOnlyFlag = "for-resource-only"

func newMetadataCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "metadata",
		GroupID: groupUserFacing,
		Short:   "Manage resource metadata definitions in the repository",
	}

	cmd.PersistentFlags().Bool(metadataResourceOnlyFlag, false, "Treat the path as a resource when no trailing / is provided")

	cmd.AddCommand(newMetadataGetCommand())
	cmd.AddCommand(newMetadataSetCommand())
	cmd.AddCommand(newMetadataUnsetCommand())
	cmd.AddCommand(newMetadataAddCommand())
	cmd.AddCommand(newMetadataUpdateResourcesCommand())

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

	return cmd
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
				return usageError(cmd, "--file is required")
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

	return cmd
}

func parseMetadataValue(raw string) (any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return "", nil
	}
	if !json.Valid([]byte(trimmed)) {
		return raw, nil
	}

	dec := json.NewDecoder(strings.NewReader(trimmed))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func decodeMetadataFile(data []byte) (map[string]any, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return nil, errors.New("metadata file is empty")
	}

	var meta map[string]any
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	if err := dec.Decode(&meta); err != nil {
		return nil, err
	}
	if meta == nil {
		meta = map[string]any{}
	}
	return meta, nil
}

type secretListInput struct {
	values  []string
	replace bool
}

func formatMetadataOutput(meta resource.ResourceMetadata) (any, error) {
	output := metadataOutput{
		OperationInfo: meta.OperationInfo,
	}
	if meta.ResourceInfo == nil {
		values := []string{}
		output.ResourceInfo = &resourceInfoOutput{
			SecretInAttributes: &values,
		}
		return output, nil
	}

	outInfo := &resourceInfoOutput{
		IDFromAttribute:    meta.ResourceInfo.IDFromAttribute,
		AliasFromAttribute: meta.ResourceInfo.AliasFromAttribute,
		CollectionPath:     meta.ResourceInfo.CollectionPath,
	}
	values := meta.ResourceInfo.SecretInAttributes
	if values == nil {
		values = []string{}
	} else {
		values = append([]string{}, values...)
	}
	outInfo.SecretInAttributes = &values
	output.ResourceInfo = outInfo
	return output, nil
}

type metadataOutput struct {
	ResourceInfo  *resourceInfoOutput             `json:"resourceInfo,omitempty"`
	OperationInfo *resource.OperationInfoMetadata `json:"operationInfo,omitempty"`
}

type resourceInfoOutput struct {
	IDFromAttribute    string    `json:"idFromAttribute,omitempty"`
	AliasFromAttribute string    `json:"aliasFromAttribute,omitempty"`
	CollectionPath     string    `json:"collectionPath,omitempty"`
	SecretInAttributes *[]string `json:"secretInAttributes,omitempty"`
}

func isSecretInAttributesAttribute(attribute string) bool {
	return strings.EqualFold(strings.TrimSpace(attribute), "resourceInfo.secretInAttributes")
}

func parseSecretInAttributesInput(raw string) (secretListInput, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return secretListInput{}, errors.New("secretInAttributes value is required")
	}

	if json.Valid([]byte(trimmed)) {
		dec := json.NewDecoder(strings.NewReader(trimmed))
		dec.UseNumber()
		var parsed any
		if err := dec.Decode(&parsed); err != nil {
			return secretListInput{}, err
		}
		switch value := parsed.(type) {
		case []any:
			if len(value) == 0 {
				return secretListInput{values: []string{}, replace: true}, nil
			}
			values, err := coerceStringSlice(value)
			if err != nil {
				return secretListInput{}, err
			}
			return secretListInput{values: values, replace: true}, nil
		case []string:
			return secretListInput{values: value, replace: true}, nil
		case string:
			if strings.Contains(value, ",") {
				values := splitCommaList(value)
				if len(values) == 0 {
					return secretListInput{}, errors.New("secretInAttributes must include at least one value")
				}
				return secretListInput{values: values, replace: true}, nil
			}
			value = strings.TrimSpace(value)
			if value == "" {
				return secretListInput{}, errors.New("secretInAttributes must include at least one value")
			}
			return secretListInput{values: []string{value}, replace: false}, nil
		default:
			return secretListInput{}, errors.New("secretInAttributes must be a JSON array or comma-separated string")
		}
	}

	values := splitCommaList(trimmed)
	if len(values) == 0 {
		return secretListInput{}, errors.New("secretInAttributes must include at least one value")
	}
	replace := strings.Contains(trimmed, ",")
	return secretListInput{values: values, replace: replace}, nil
}

func setSecretInAttributes(meta map[string]any, attribute string, input secretListInput) (bool, error) {
	if input.replace {
		return metadata.SetMetadataAttribute(meta, attribute, input.values)
	}
	if len(input.values) == 0 {
		return metadata.SetMetadataAttribute(meta, attribute, []string{})
	}
	existing, ok := resource.GetAttrPath(meta, attribute)
	if ok && isArrayValue(existing) {
		return metadata.SetMetadataAttribute(meta, attribute, input.values[0])
	}
	return metadata.SetMetadataAttribute(meta, attribute, input.values)
}

func splitCommaList(raw string) []string {
	parts := strings.Split(raw, ",")
	values := make([]string, 0, len(parts))
	for _, part := range parts {
		value := strings.TrimSpace(part)
		if value == "" {
			continue
		}
		values = append(values, value)
	}
	return values
}

func coerceStringSlice(values []any) ([]string, error) {
	out := make([]string, 0, len(values))
	for _, value := range values {
		text, ok := value.(string)
		if !ok {
			return nil, errors.New("secretInAttributes entries must be strings")
		}
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			return nil, errors.New("secretInAttributes entries must be non-empty")
		}
		out = append(out, trimmed)
	}
	return out, nil
}

func isArrayValue(value any) bool {
	switch value.(type) {
	case []any, []string:
		return true
	default:
		return false
	}
}

func metadataForResourceOnly(cmd *cobra.Command) bool {
	if cmd == nil {
		return false
	}
	if cmd.Flags().Lookup(metadataResourceOnlyFlag) != nil {
		value, err := cmd.Flags().GetBool(metadataResourceOnlyFlag)
		if err == nil {
			return value
		}
	}
	if cmd.InheritedFlags().Lookup(metadataResourceOnlyFlag) != nil {
		value, err := cmd.InheritedFlags().GetBool(metadataResourceOnlyFlag)
		if err == nil {
			return value
		}
	}
	return false
}

func normalizeMetadataInputPath(cmd *cobra.Command, path string) (string, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "", usageError(cmd, "path is required")
	}
	normalized := normalizeMetadataPath(trimmed, metadataForResourceOnly(cmd))
	return normalized, nil
}

func normalizeMetadataPath(path string, forResourceOnly bool) string {
	trimmed := strings.TrimSpace(path)
	isCollection := resource.IsCollectionPath(trimmed)
	normalized := resource.NormalizePath(trimmed)
	if isCollection || !forResourceOnly {
		if normalized != "/" {
			return normalized + "/"
		}
	}
	return normalized
}

func validateMetadataPath(cmd *cobra.Command, path string) error {
	if err := resource.ValidateMetadataPath(path); err != nil {
		return usageError(cmd, err.Error())
	}
	return nil
}
