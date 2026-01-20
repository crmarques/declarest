package cmd

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/crmarques/declarest/managedserver"
	"github.com/crmarques/declarest/reconciler"
	"github.com/crmarques/declarest/repository"

	"github.com/spf13/cobra"
)

const (
	debugGroupAll        = "all"
	debugGroupNetwork    = "network"
	debugGroupRepository = "repository"
	debugGroupResource   = "resource"
)

var debugGroupOrder = []string{
	debugGroupNetwork,
	debugGroupRepository,
	debugGroupResource,
}

type debugSettings struct {
	enabled bool
	groups  map[string]bool
}

type debugContext struct {
	serverInfo     func() (managedserver.ServerDebugInfo, bool)
	repositoryInfo func() (repository.RepositoryDebugInfo, bool)
}

var currentDebug debugSettings
var currentDebugContext debugContext

func configureDebugSettings(cmd *cobra.Command) error {
	resetDebugContext()

	debugValue, err := lookupStringFlag(cmd, "debug")
	if err != nil {
		return err
	}

	settings, err := parseDebugSettings(debugValue)
	if err != nil {
		return usageError(cmd, err.Error())
	}
	currentDebug = settings
	return nil
}

func parseDebugSettings(value string) (debugSettings, error) {
	raw := strings.TrimSpace(value)
	if raw == "" {
		return debugSettings{}, nil
	}

	groups := map[string]bool{}
	for _, entry := range splitDebugGroups(raw) {
		name := strings.ToLower(strings.TrimSpace(entry))
		if name == "" {
			continue
		}
		if name == debugGroupAll {
			return debugSettings{
				enabled: true,
				groups:  debugGroupSet(),
			}, nil
		}
		if !isKnownDebugGroup(name) {
			return debugSettings{}, fmt.Errorf("unknown debug group %q (available: %s)", name, strings.Join(knownDebugGroups(), ", "))
		}
		groups[name] = true
	}
	if len(groups) == 0 {
		return debugSettings{}, nil
	}
	return debugSettings{
		enabled: true,
		groups:  groups,
	}, nil
}

func splitDebugGroups(raw string) []string {
	parts := strings.FieldsFunc(raw, func(r rune) bool {
		return r == ',' || r == ';'
	})
	return parts
}

func knownDebugGroups() []string {
	return []string{
		debugGroupAll,
		debugGroupNetwork,
		debugGroupRepository,
		debugGroupResource,
	}
}

func debugGroupSet() map[string]bool {
	groups := map[string]bool{}
	for _, group := range debugGroupOrder {
		groups[group] = true
	}
	return groups
}

func isKnownDebugGroup(group string) bool {
	for _, name := range knownDebugGroups() {
		if name == group {
			return true
		}
	}
	return false
}

func debugEnabled(group string) bool {
	if !currentDebug.enabled {
		return false
	}
	return currentDebug.groups[group]
}

func captureDebugContext(recon reconciler.AppReconciler) {
	if recon == nil {
		return
	}
	currentDebugContext.serverInfo = recon.ServerDebugInfo
	currentDebugContext.repositoryInfo = recon.RepositoryDebugInfo
}

func resetDebugContext() {
	currentDebugContext = debugContext{}
}

func ReportDebug(err error, out io.Writer) {
	if out == nil || !currentDebug.enabled {
		return
	}

	sections := []debugSection{}

	if debugEnabled(debugGroupNetwork) {
		if section := buildNetworkDebugSection(err); section.hasItems() {
			sections = append(sections, section)
		}
	}
	if debugEnabled(debugGroupRepository) {
		if section := buildRepositoryDebugSection(); section.hasItems() {
			sections = append(sections, section)
		}
	}

	if len(sections) == 0 && err == nil {
		return
	}

	fmt.Fprintln(out, "Debug info:")
	for idx, section := range sections {
		fmt.Fprintf(out, "  %s:\n", section.title)
		for _, item := range section.items {
			printDebugItem(out, item)
		}
		if idx < len(sections)-1 {
			fmt.Fprintln(out)
		}
	}
}

func printDebugItem(out io.Writer, item debugItem) {
	if strings.Contains(item.value, "\n") {
		fmt.Fprintf(out, "    %s:\n", item.key)
		for _, line := range strings.Split(item.value, "\n") {
			line = strings.TrimSpace(line)
			if line == "" {
				continue
			}
			fmt.Fprintf(out, "      %s\n", line)
		}
		return
	}
	fmt.Fprintf(out, "    %s: %s\n", item.key, item.value)
}

type debugItem struct {
	key   string
	value string
}

type debugSection struct {
	title string
	items []debugItem
}

func (s debugSection) hasItems() bool {
	return len(s.items) > 0
}

func buildNetworkDebugSection(err error) debugSection {
	section := debugSection{title: "Network"}
	if getter := currentDebugContext.serverInfo; getter != nil {
		info, ok := getter()
		if !ok {
			return section
		}
		if strings.TrimSpace(info.Type) != "" {
			section.items = append(section.items, debugItem{key: "type", value: info.Type})
		}
		if strings.TrimSpace(info.BaseURL) != "" {
			section.items = append(section.items, debugItem{key: "base_url", value: info.BaseURL})
		}
		if strings.TrimSpace(info.AuthMethod) != "" {
			section.items = append(section.items, debugItem{key: "auth", value: info.AuthMethod})
		}
		if info.TLSInsecureSkipVerify != nil {
			section.items = append(section.items, debugItem{key: "tls_insecure_skip_verify", value: fmt.Sprintf("%t", *info.TLSInsecureSkipVerify)})
		}
		if len(info.DefaultHeaders) > 0 {
			section.items = append(section.items, debugItem{key: "default_headers", value: strings.Join(info.DefaultHeaders, ", ")})
		}
		if strings.TrimSpace(info.OpenAPI) != "" {
			section.items = append(section.items, debugItem{key: "openapi", value: info.OpenAPI})
		}
		if len(info.Interactions) == 0 && (info.LastRequest != nil || info.LastResponse != nil) {
			appendHTTPInteractionDebugItems(&section, "last_interaction", managedserver.HTTPInteraction{
				Request:  info.LastRequest,
				Response: info.LastResponse,
			})
		}
		for idx, interaction := range info.Interactions {
			prefix := fmt.Sprintf("interaction[%d]", idx+1)
			appendHTTPInteractionDebugItems(&section, prefix, interaction)
		}
	}
	if err != nil {
		section.items = append(section.items, debugItem{key: "error", value: err.Error()})
	}
	return section
}

func appendHTTPInteractionDebugItems(section *debugSection, prefix string, interaction managedserver.HTTPInteraction) {
	appendHTTPRequestDebugItems(section, fmt.Sprintf("%s.request", prefix), interaction.Request)
	appendHTTPResponseDebugItems(section, fmt.Sprintf("%s.response", prefix), interaction.Response)
}

func appendHTTPRequestDebugItems(section *debugSection, baseKey string, request *managedserver.HTTPRequestDebugInfo) {
	if request == nil {
		return
	}
	if line := strings.TrimSpace(fmt.Sprintf("%s %s", request.Method, request.URL)); line != "" {
		section.items = append(section.items, debugItem{key: baseKey, value: line})
	}
	if len(request.Headers) > 0 {
		section.items = append(section.items, debugItem{key: baseKey + ".headers", value: strings.Join(request.Headers, "\n")})
	}
	if strings.TrimSpace(request.Body) != "" {
		section.items = append(section.items, debugItem{key: baseKey + ".body", value: request.Body})
	}
}

func appendHTTPResponseDebugItems(section *debugSection, baseKey string, response *managedserver.HTTPResponseDebugInfo) {
	if response == nil {
		return
	}
	statusText := strings.TrimSpace(response.StatusText)
	if statusText == "" {
		statusText = http.StatusText(response.StatusCode)
	}
	if response.StatusCode > 0 || statusText != "" {
		section.items = append(section.items, debugItem{key: baseKey + ".status", value: fmt.Sprintf("%d %s", response.StatusCode, statusText)})
	}
	if len(response.Headers) > 0 {
		section.items = append(section.items, debugItem{key: baseKey + ".headers", value: strings.Join(response.Headers, "\n")})
	}
	if strings.TrimSpace(response.Body) != "" {
		section.items = append(section.items, debugItem{key: baseKey + ".body", value: response.Body})
	}
}

func buildRepositoryDebugSection() debugSection {
	section := debugSection{title: "Repository"}
	getter := currentDebugContext.repositoryInfo
	if getter == nil {
		return section
	}
	info, ok := getter()
	if !ok {
		return section
	}
	if strings.TrimSpace(info.Type) != "" {
		section.items = append(section.items, debugItem{key: "type", value: info.Type})
	}
	if strings.TrimSpace(info.BaseDir) != "" {
		section.items = append(section.items, debugItem{key: "root", value: info.BaseDir})
	}
	if strings.TrimSpace(info.ResourceFormat) != "" {
		section.items = append(section.items, debugItem{key: "resource_format", value: info.ResourceFormat})
	}
	if strings.TrimSpace(info.RemoteURL) != "" {
		section.items = append(section.items, debugItem{key: "remote_url", value: info.RemoteURL})
	}
	if strings.TrimSpace(info.RemoteBranch) != "" {
		section.items = append(section.items, debugItem{key: "remote_branch", value: info.RemoteBranch})
	}
	if strings.TrimSpace(info.RemoteProvider) != "" {
		section.items = append(section.items, debugItem{key: "remote_provider", value: info.RemoteProvider})
	}
	if strings.TrimSpace(info.RemoteAuth) != "" {
		section.items = append(section.items, debugItem{key: "remote_auth", value: info.RemoteAuth})
	}
	if info.RemoteAutoSync != nil {
		section.items = append(section.items, debugItem{key: "remote_auto_sync", value: fmt.Sprintf("%t", *info.RemoteAutoSync)})
	}
	if info.RemoteTLSInsecureSkipVerify != nil {
		section.items = append(section.items, debugItem{key: "remote_tls_insecure_skip_verify", value: fmt.Sprintf("%t", *info.RemoteTLSInsecureSkipVerify)})
	}
	return section
}

func redactURL(raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return ""
	}
	parsed, err := url.Parse(trimmed)
	if err != nil {
		return trimmed
	}
	parsed.User = nil
	parsed.RawQuery = ""
	parsed.Fragment = ""
	return parsed.String()
}

func lookupStringFlag(cmd *cobra.Command, name string) (string, error) {
	if cmd == nil {
		return "", nil
	}
	if cmd.Flags().Lookup(name) != nil {
		return cmd.Flags().GetString(name)
	}
	if cmd.InheritedFlags().Lookup(name) != nil {
		return cmd.InheritedFlags().GetString(name)
	}
	return "", nil
}
