package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"

	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/openapi"
	"github.com/crmarques/declarest/resource"

	"github.com/spf13/cobra"
)

func newAdHocCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "ad-hoc",
		GroupID: groupUserFacing,
		Short:   "Send ad-hoc HTTP requests with optional metadata context",
	}

	for _, definition := range []struct {
		name   string
		method string
		short  string
	}{
		{"get", http.MethodGet, "Fetch a resource or collection without touching the repository"},
		{"post", http.MethodPost, "Create data using the metadata-aware collection endpoint"},
		{"put", http.MethodPut, "Update data using the metadata-aware resource endpoint"},
		{"patch", http.MethodPatch, "Patch data using the metadata-aware resource endpoint"},
		{"delete", http.MethodDelete, "Delete a resource using its metadata-aware endpoint"},
	} {
		cmd.AddCommand(newAdHocMethodCommand(definition.name, definition.method, definition.short))
	}

	return cmd
}

func newAdHocMethodCommand(name, method, short string) *cobra.Command {
	var (
		path           string
		headers        []string
		payload        string
		defaultHeaders bool
	)

	cmd := &cobra.Command{
		Use:   fmt.Sprintf("%s <path>", name),
		Short: short,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAdHocRequest(cmd, method, path, headers, defaultHeaders, payload, args)
		},
	}

	cmd.Flags().StringVar(&path, "path", "", "Logical resource or collection path")
	cmd.Flags().StringArrayVar(&headers, "header", nil, "Add a request header (Name: value or Name=value)")
	cmd.Flags().BoolVar(&defaultHeaders, "default-headers", false, "Ensure Accept/Content-Type defaults are applied")
	cmd.Flags().StringVar(&payload, "payload", "", "Request payload string or @file path (methods with bodies only)")

	if name == "get" {
		registerResourcePathCompletion(cmd, resourceGetPathStrategy)
	} else {
		registerResourcePathCompletion(cmd, resourceRemotePathStrategy)
	}

	return cmd
}

func runAdHocRequest(cmd *cobra.Command, method, pathFlag string, headerFlags []string, defaultHeaders bool, payloadFlag string, args []string) error {
	path, err := resolveSingleArg(cmd, pathFlag, args, "path")
	if err != nil {
		return err
	}
	if err := validateLogicalPath(cmd, path); err != nil {
		return err
	}

	recon, cleanup, err := loadDefaultReconcilerSkippingRepoSync()
	if cleanup != nil {
		defer cleanup()
	}
	if err != nil {
		return err
	}

	spec := recon.OpenAPISpec()

	record, err := recon.ResourceRecord(path)
	if err != nil {
		return err
	}

	methodUpper := strings.ToUpper(strings.TrimSpace(method))
	if methodUpper == "" {
		methodUpper = http.MethodGet
	}

	isCollection := resource.IsCollectionPath(path)
	opMetadata := selectAdHocOperation(record, methodUpper, isCollection)
	if opMetadata == nil {
		opMetadata = &resource.OperationMetadata{}
	}
	op := cloneAdHocOperationMetadata(opMetadata)
	op.HTTPMethod = methodUpper

	targetPath := path
	if resolved, err := record.ResolveOperationPath(path, op, isCollection); err != nil {
		return err
	} else if strings.TrimSpace(resolved) != "" {
		targetPath = resolved
	}

	headers := record.HeadersFor(op, path, isCollection)
	applyOpenAPIHeaders(headers, spec, targetPath, methodUpper)
	acceptValues := takeAdHocHeaderValues(headers, "Accept")
	contentType := firstAdHocHeaderValue(headers, "Content-Type")

	if err := applyUserHeaders(cmd, headers, &acceptValues, &contentType, headerFlags); err != nil {
		return err
	}

	if defaultHeaders {
		if len(acceptValues) == 0 {
			acceptValues = []string{"application/json"}
		}
		if contentType == "" && resource.MethodSupportsBody(methodUpper) {
			contentType = "application/json"
		}
	}

	var acceptHeader string
	if len(acceptValues) > 0 {
		acceptHeader = strings.Join(acceptValues, ", ")
	}

	httpSpec := &managedserver.HTTPRequestSpec{
		Method:      methodUpper,
		Path:        targetPath,
		Headers:     headers,
		Query:       record.QueryFor(op),
		Accept:      acceptHeader,
		ContentType: contentType,
	}

	payloadBytes, payloadSource, err := loadAdHocPayload(payloadFlag)
	if err != nil {
		return err
	}
	if payloadSource != "" {
		infof(cmd, "Payload loaded from %s", payloadSource)
	} else if strings.TrimSpace(payloadFlag) != "" {
		infof(cmd, "Payload provided as literal string; use @path to load from a file")
	}

	resp, err := recon.ExecuteHTTPRequest(httpSpec, payloadBytes)
	if err != nil {
		return err
	}

	if err := writeAdHocResponse(cmd, resp); err != nil {
		return err
	}

	if !noStatusOutput {
		fmt.Fprintf(cmd.ErrOrStderr(), "[OK] %s %s %d\n", methodUpper, targetPath, resp.StatusCode)
	}

	return nil
}

func writeAdHocResponse(cmd *cobra.Command, resp *managedserver.HTTPResponse) error {
	if resp == nil || len(resp.Body) == 0 {
		return nil
	}
	if shouldFormatAdHocJSON(resp) {
		res, err := resource.NewResourceFromJSON(resp.Body)
		if err == nil {
			return printResourceJSON(cmd, res)
		}
	}
	if _, err := cmd.OutOrStdout().Write(resp.Body); err != nil {
		return fmt.Errorf("failed to write response: %w", err)
	}
	return nil
}

func shouldFormatAdHocJSON(resp *managedserver.HTTPResponse) bool {
	if resp == nil || len(resp.Body) == 0 {
		return false
	}
	if resp.Header != nil {
		contentType := strings.ToLower(resp.Header.Get("Content-Type"))
		if strings.Contains(contentType, "json") {
			return true
		}
	}
	trimmed := bytes.TrimSpace(resp.Body)
	if len(trimmed) == 0 {
		return false
	}
	return json.Valid(trimmed)
}

func selectAdHocOperation(record resource.ResourceRecord, method string, isCollection bool) *resource.OperationMetadata {
	switch method {
	case http.MethodPost:
		return record.CreateOperation()
	case http.MethodPut, http.MethodPatch:
		return record.UpdateOperation()
	case http.MethodDelete:
		return record.DeleteOperation()
	default:
		return record.ReadOperation(isCollection)
	}
}

func cloneAdHocOperationMetadata(src *resource.OperationMetadata) *resource.OperationMetadata {
	if src == nil {
		return &resource.OperationMetadata{}
	}

	clone := *src

	if src.URL != nil {
		urlClone := *src.URL
		urlClone.QueryStrings = append([]string{}, src.URL.QueryStrings...)
		clone.URL = &urlClone
	}

	if src.HTTPHeaders != nil {
		clone.HTTPHeaders = append(resource.HeaderList{}, src.HTTPHeaders...)
	}

	if src.Payload != nil {
		payloadClone := *src.Payload
		payloadClone.FilterAttributes = append([]string{}, src.Payload.FilterAttributes...)
		payloadClone.SuppressAttributes = append([]string{}, src.Payload.SuppressAttributes...)
		clone.Payload = &payloadClone
	}

	return &clone
}

func applyUserHeaders(cmd *cobra.Command, headers map[string][]string, acceptValues *[]string, contentType *string, headerFlags []string) error {
	for _, raw := range headerFlags {
		name, value, err := parseAdHocHeader(raw)
		if err != nil {
			return usageError(cmd, err.Error())
		}
		switch strings.ToLower(name) {
		case "accept":
			*acceptValues = []string{value}
		case "content-type":
			*contentType = value
		default:
			key := http.CanonicalHeaderKey(name)
			headers[key] = append(headers[key], value)
		}
	}
	return nil
}

func parseAdHocHeader(raw string) (string, string, error) {
	if name, value, ok := resource.SplitHeaderLine(raw); ok {
		return name, value, nil
	}
	parts := strings.SplitN(raw, "=", 2)
	if len(parts) == 2 {
		name := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])
		if name != "" && value != "" {
			return name, value, nil
		}
	}
	return "", "", fmt.Errorf("headers must be in the form Name: value or Name=value")
}

func loadAdHocPayload(raw string) ([]byte, string, error) {
	value := strings.TrimSpace(raw)
	if value == "" {
		return nil, "", nil
	}
	if strings.HasPrefix(value, "@") {
		path := strings.TrimSpace(strings.TrimPrefix(value, "@"))
		if path == "" {
			return nil, "", fmt.Errorf("payload file path is empty")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, "", fmt.Errorf("failed to read payload file %q: %w", path, err)
		}
		return data, path, nil
	}
	return []byte(value), "", nil
}

func takeAdHocHeaderValues(headers map[string][]string, name string) []string {
	if len(headers) == 0 {
		return nil
	}
	var values []string
	for key, list := range headers {
		if !strings.EqualFold(key, name) {
			continue
		}
		if len(list) > 0 {
			values = append(values, list...)
		}
		delete(headers, key)
	}
	return values
}

func firstAdHocHeaderValue(headers map[string][]string, name string) string {
	values := takeAdHocHeaderValues(headers, name)
	if len(values) == 0 {
		return ""
	}
	return values[0]
}

func applyOpenAPIHeaders(headers map[string][]string, spec *openapi.Spec, rawPath, method string) {
	if spec == nil || headers == nil {
		return
	}
	path := resource.NormalizePath(rawPath)
	if path == "" {
		path = "/"
	}
	item := spec.MatchPath(path)
	if item == nil {
		return
	}
	op := item.Operation(strings.ToLower(strings.TrimSpace(method)))
	if op == nil {
		return
	}
	for name, value := range op.HeaderParameters {
		key := http.CanonicalHeaderKey(strings.TrimSpace(name))
		if key == "" {
			continue
		}
		val := strings.TrimSpace(value)
		if val == "" {
			continue
		}
		if _, exists := headers[key]; exists {
			continue
		}
		headers[key] = []string{val}
	}
}
