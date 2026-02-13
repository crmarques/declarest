package cmd

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/crmarques/declarest/openapi"
	"github.com/crmarques/declarest/reconciler"
	"github.com/crmarques/declarest/resource"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newResourceAddCommand() *cobra.Command {
	var (
		path        string
		filePath    string
		fromPath    string
		fromOpenAPI string
		overrides   []string
		applyRemote bool
		force       bool
	)

	cmd := &cobra.Command{
		Use:   "add <path> [file]",
		Short: "Add a resource definition to the resource repository",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) > 2 {
				return usageError(cmd, "expected <path> [file]")
			}
			path = strings.TrimSpace(path)
			filePath = strings.TrimSpace(filePath)
			fromPath = strings.TrimSpace(fromPath)
			fromOpenAPI = strings.TrimSpace(fromOpenAPI)
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
					if fromPath != "" {
						return usageError(cmd, "cannot combine --from-path with a file argument")
					}
					if filePath != "" && filePath != argFile {
						return usageError(cmd, "file specified twice")
					}
					if filePath == "" {
						filePath = argFile
					}
				}
			}
			useOpenAPI := cmd.Flags().Changed("from-openapi")
			openAPISource := ""
			if useOpenAPI {
				if fromOpenAPI == openAPIFromContextValue || fromOpenAPI == "" {
					openAPISource = ""
				} else {
					openAPISource = fromOpenAPI
				}
			}
			if path == "" {
				return usageError(cmd, "path is required")
			}
			if err := validateLogicalPath(cmd, path); err != nil {
				return err
			}
			if filePath != "" && fromPath != "" {
				return usageError(cmd, "--file and --from-path cannot be used together")
			}
			if useOpenAPI && (filePath != "" || fromPath != "") {
				return usageError(cmd, "--from-openapi cannot be combined with --file or --from-path")
			}
			if filePath == "" && fromPath == "" && !useOpenAPI {
				return usageError(cmd, "either --file, --from-path, or --from-openapi is required")
			}
			if fromPath != "" {
				if err := validateLogicalPath(cmd, fromPath); err != nil {
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

			var res resource.Resource
			if useOpenAPI {
				res, err = resourceFromOpenAPI(recon, path, openAPISource)
			} else if fromPath != "" {
				res, err = recon.GetLocalResource(fromPath)
				if err != nil {
					return err
				}
				res, err = dropResourceID(res)
				if err != nil {
					return err
				}
			} else {
				res, err = loadResourceFromFile(filePath)
			}
			if err != nil {
				return err
			}

			if len(overrides) > 0 {
				res, err = applyResourceOverrides(res, overrides)
				if err != nil {
					return err
				}
			}

			targetPath, err := resolveAddTargetPath(recon, path, res)
			if err != nil {
				return err
			}
			if !force {
				exists, err := localResourceExists(recon, targetPath)
				if err != nil {
					return err
				}
				if exists {
					if targetPath != path {
						return fmt.Errorf("resource %s already exists in the repository (resolved from %s); use --force to overwrite", targetPath, path)
					}
					return fmt.Errorf("resource %s already exists in the repository; use --force to overwrite", targetPath)
				}
			}

			if err := saveLocalResourceWithSecrets(recon, path, res, true); err != nil {
				return err
			}

			if targetPath != path {
				successf(cmd, "added resource %s to the resource repository (resolved from %s)", targetPath, path)
			} else {
				successf(cmd, "added resource %s to the resource repository", targetPath)
			}

			if applyRemote {
				if err := recon.SaveRemoteResource(targetPath, res); err != nil {
					return wrapRemoteErrorWithDetails(err, targetPath)
				}
				successf(cmd, "applied remote resource %s", targetPath)
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Resource path to add")
	cmd.Flags().StringVar(&filePath, "file", "", "Path to a JSON or YAML resource payload file")
	cmd.Flags().StringVar(&fromPath, "from-path", "", "Resource path in the repository to copy")
	cmd.Flags().StringVar(&fromOpenAPI, "from-openapi", "", "Build the resource from an OpenAPI schema (optional spec path; defaults to context)")
	cmd.Flags().Lookup("from-openapi").NoOptDefVal = openAPIFromContextValue
	cmd.Flags().StringArrayVar(&overrides, "override", nil, "Override resource fields (key=value list, JSON object, or JSON/YAML file)")
	cmd.Flags().BoolVar(&applyRemote, "apply", false, "Apply the resource to the remote server after saving")
	cmd.Flags().BoolVar(&force, "force", false, "Overwrite the resource in the repository if it already exists")

	registerResourcePathCompletion(cmd, resourceRepoPathStrategy)

	return cmd
}

func loadResourceFromFile(path string) (resource.Resource, error) {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return resource.Resource{}, errors.New("resource file path is required")
	}

	data, err := os.ReadFile(trimmed)
	if err != nil {
		return resource.Resource{}, err
	}

	doc, err := parseResourceDocument(data, filepath.Dir(trimmed))
	if err != nil {
		return resource.Resource{}, err
	}

	if format, ok := resourceFileFormat(trimmed); ok {
		return decodeResolvedResource(doc, format)
	}

	if res, err := decodeResolvedResource(doc, "json"); err == nil {
		return res, nil
	}
	if res, err := decodeResolvedResource(doc, "yaml"); err == nil {
		return res, nil
	}

	return resource.Resource{}, fmt.Errorf("resource file %q must be valid JSON or YAML", trimmed)
}

func decodeResourceData(data []byte, format string) (resource.Resource, error) {
	switch format {
	case "yaml":
		return resource.NewResourceFromYAML(data)
	default:
		return resource.NewResourceFromJSON(data)
	}
}

func resourceFileFormat(path string) (string, bool) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".yaml", ".yml":
		return "yaml", true
	case ".json":
		return "json", true
	default:
		return "", false
	}
}

func parseResourceDocument(data []byte, baseDir string) (any, error) {
	var raw any
	if err := yaml.Unmarshal(data, &raw); err != nil {
		return nil, err
	}
	normalized, err := normalizeYAMLStructure(raw)
	if err != nil {
		return nil, err
	}
	return resolveResourceIncludes(normalized, baseDir)
}

func decodeResolvedResource(doc any, format string) (resource.Resource, error) {
	switch format {
	case "yaml":
		b, err := yaml.Marshal(doc)
		if err != nil {
			return resource.Resource{}, err
		}
		return resource.NewResourceFromYAML(b)
	default:
		b, err := json.Marshal(doc)
		if err != nil {
			return resource.Resource{}, err
		}
		return resource.NewResourceFromJSON(b)
	}
}

func resolveResourceIncludes(value any, baseDir string) (any, error) {
	switch t := value.(type) {
	case map[string]any:
		for key, item := range t {
			resolved, err := resolveResourceIncludes(item, baseDir)
			if err != nil {
				return nil, err
			}
			t[key] = resolved
		}
		return t, nil
	case []any:
		for idx, item := range t {
			resolved, err := resolveResourceIncludes(item, baseDir)
			if err != nil {
				return nil, err
			}
			t[idx] = resolved
		}
		return t, nil
	case string:
		includePath, ok := parseIncludeDirective(t)
		if !ok {
			return t, nil
		}
		return loadIncludedValue(baseDir, includePath)
	default:
		return t, nil
	}
}

func loadIncludedValue(baseDir, includePath string) (any, error) {
	resolved := includePath
	if !filepath.IsAbs(includePath) {
		resolved = filepath.Join(baseDir, includePath)
	}
	resolved = filepath.Clean(resolved)

	data, err := os.ReadFile(resolved)
	if err != nil {
		return nil, err
	}

	if _, ok := resourceFileFormat(resolved); ok {
		return parseResourceDocument(data, filepath.Dir(resolved))
	}

	return string(data), nil
}

func normalizeYAMLStructure(value any) (any, error) {
	switch t := value.(type) {
	case nil:
		return nil, nil
	case map[string]any:
		out := make(map[string]any, len(t))
		for key, val := range t {
			norm, err := normalizeYAMLStructure(val)
			if err != nil {
				return nil, err
			}
			out[key] = norm
		}
		return out, nil
	case map[any]any:
		out := make(map[string]any, len(t))
		for key, val := range t {
			ks, ok := key.(string)
			if !ok {
				return nil, fmt.Errorf("yaml key %v is not a string", key)
			}
			norm, err := normalizeYAMLStructure(val)
			if err != nil {
				return nil, err
			}
			out[ks] = norm
		}
		return out, nil
	case []any:
		out := make([]any, len(t))
		for idx, val := range t {
			norm, err := normalizeYAMLStructure(val)
			if err != nil {
				return nil, err
			}
			out[idx] = norm
		}
		return out, nil
	default:
		return t, nil
	}
}

func parseIncludeDirective(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) <= 4 || !strings.HasPrefix(trimmed, "{{") || !strings.HasSuffix(trimmed, "}}") {
		return "", false
	}
	inner := strings.TrimSpace(trimmed[2 : len(trimmed)-2])
	const includeKeyword = "include"
	if !strings.HasPrefix(inner, includeKeyword) {
		return "", false
	}
	path := strings.TrimSpace(inner[len(includeKeyword):])
	if path == "" {
		return "", false
	}
	return path, true
}

func resolveAddTargetPath(recon reconciler.AppReconciler, path string, res resource.Resource) (string, error) {
	if recon == nil {
		return "", errors.New("reconciler is not configured")
	}

	record, err := recon.ResourceRecord(path)
	if err != nil {
		return "", err
	}
	if strings.TrimSpace(record.Path) == "" {
		record.Path = path
	}

	payload := record.ReadPayload()
	processed, err := record.ApplyPayload(res, payload)
	if err != nil {
		return "", err
	}

	return record.AliasPath(processed), nil
}

func localResourceExists(recon reconciler.AppReconciler, path string) (bool, error) {
	if recon == nil {
		return false, errors.New("reconciler is not configured")
	}
	_, err := recon.GetLocalResource(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}

func applyResourceOverrides(res resource.Resource, overrides []string) (resource.Resource, error) {
	obj, err := ensureResourceObject(res)
	if err != nil {
		return resource.Resource{}, err
	}

	for _, raw := range overrides {
		override, err := parseOverride(raw)
		if err != nil {
			return resource.Resource{}, err
		}
		mergeOverride(obj, override)
	}

	return resource.NewResource(obj)
}

func ensureResourceObject(res resource.Resource) (map[string]any, error) {
	if res.V == nil {
		return map[string]any{}, nil
	}
	obj, ok := res.AsObject()
	if !ok {
		return nil, errors.New("overrides require an object resource")
	}
	return obj, nil
}

func parseOverride(raw string) (map[string]any, error) {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return nil, errors.New("override value is required")
	}

	if json.Valid([]byte(trimmed)) {
		res, err := resource.NewResourceFromJSON([]byte(trimmed))
		if err != nil {
			return nil, err
		}
		obj, ok := res.AsObject()
		if !ok {
			return nil, errors.New("override JSON must be an object")
		}
		return obj, nil
	}

	if strings.Contains(trimmed, "=") {
		pairs := splitCommaList(trimmed)
		if len(pairs) == 0 {
			return nil, errors.New("override must include at least one key=value pair")
		}
		override, err := parseOverridePairs(pairs)
		if err != nil {
			return nil, err
		}
		return override, nil
	}

	if override, ok, err := loadOverrideFile(trimmed); ok {
		return override, err
	}

	return nil, errors.New("override must be a JSON object, key=value list, or JSON/YAML file")
}

func loadOverrideFile(path string) (map[string]any, bool, error) {
	format, ok := resourceFileFormat(path)
	if !ok {
		return nil, false, nil
	}

	info, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, true, fmt.Errorf("override file %q not found", path)
		}
		return nil, true, err
	}
	if info.IsDir() {
		return nil, true, fmt.Errorf("override file %q is a directory", path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, true, err
	}

	res, err := decodeResourceData(data, format)
	if err != nil {
		return nil, true, err
	}
	obj, ok := res.AsObject()
	if !ok {
		return nil, true, fmt.Errorf("override file %q must contain a JSON or YAML object", path)
	}
	return obj, true, nil
}

func parseOverridePairs(pairs []string) (map[string]any, error) {
	override := make(map[string]any)
	for _, pair := range pairs {
		key, rawValue, ok := strings.Cut(pair, "=")
		if !ok {
			return nil, fmt.Errorf("override %q must use key=value syntax", pair)
		}
		key = strings.TrimSpace(key)
		if key == "" {
			return nil, errors.New("override key is required")
		}
		value, err := parseOverrideValue(strings.TrimSpace(rawValue))
		if err != nil {
			return nil, err
		}
		if err := setOverridePath(override, key, value); err != nil {
			return nil, err
		}
	}
	return override, nil
}

func parseOverrideValue(raw string) (any, error) {
	if raw == "" {
		return "", nil
	}
	if !json.Valid([]byte(raw)) {
		return raw, nil
	}
	dec := json.NewDecoder(strings.NewReader(raw))
	dec.UseNumber()
	var value any
	if err := dec.Decode(&value); err != nil {
		return nil, err
	}
	return value, nil
}

func setOverridePath(obj map[string]any, path string, value any) error {
	segments := strings.Split(path, ".")
	if len(segments) == 0 {
		return errors.New("override key is required")
	}
	current := obj
	for idx, segment := range segments {
		segment = strings.TrimSpace(segment)
		if segment == "" {
			return fmt.Errorf("invalid override key %q", path)
		}
		if idx == len(segments)-1 {
			current[segment] = value
			return nil
		}
		next, ok := current[segment].(map[string]any)
		if !ok {
			next = map[string]any{}
			current[segment] = next
		}
		current = next
	}
	return nil
}

func mergeOverride(dst map[string]any, src map[string]any) {
	for key, value := range src {
		srcMap, ok := value.(map[string]any)
		if !ok {
			dst[key] = value
			continue
		}
		existing, ok := dst[key].(map[string]any)
		if !ok {
			dst[key] = srcMap
			continue
		}
		mergeOverride(existing, srcMap)
	}
}

func dropResourceID(res resource.Resource) (resource.Resource, error) {
	obj, ok := res.AsObject()
	if !ok {
		return res, nil
	}

	clone := make(map[string]any, len(obj))
	for key, value := range obj {
		clone[key] = value
	}

	if _, ok := clone["id"]; ok {
		delete(clone, "id")
		return resource.NewResource(clone)
	}
	return res, nil
}

func resourceFromOpenAPI(recon reconciler.AppReconciler, logicalPath, source string) (resource.Resource, error) {
	if recon == nil {
		return resource.Resource{}, errors.New("reconciler is not configured")
	}

	spec, err := resolveOpenAPISpec(recon, source)
	if err != nil {
		return resource.Resource{}, err
	}
	return openapi.BuildResourceFromSpec(spec, logicalPath)
}
