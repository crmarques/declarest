package cmd

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"declarest/internal/managedserver"
	"declarest/internal/reconciler"
	"declarest/internal/repository"

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
	server     serverDebugProvider
	repository repositoryDebugProvider
}

var currentDebug debugSettings
var currentDebugContext debugContext

type serverDebugProvider interface {
	DebugInfo() managedserver.ServerDebugInfo
}

type repositoryDebugProvider interface {
	DebugInfo() repository.RepositoryDebugInfo
}

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
			return debugSettings{}, fmt.Errorf("unknown verbose group %q (available: %s)", name, strings.Join(knownDebugGroups(), ", "))
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

func captureDebugContext(recon *reconciler.DefaultReconciler) {
	if recon == nil {
		return
	}
	if provider, ok := recon.ResourceServerManager.(serverDebugProvider); ok {
		currentDebugContext.server = provider
	}
	if provider, ok := recon.ResourceRepositoryManager.(repositoryDebugProvider); ok {
		currentDebugContext.repository = provider
	}
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
	for _, section := range sections {
		fmt.Fprintf(out, "  %s:\n", section.title)
		for _, item := range section.items {
			printDebugItem(out, item)
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
	if provider := currentDebugContext.server; provider != nil {
		info := provider.DebugInfo()
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
		for idx, interaction := range info.Interactions {
			prefix := fmt.Sprintf("interaction[%d]", idx+1)
			if interaction.Request != nil {
				requestLine := strings.TrimSpace(fmt.Sprintf("%s %s", interaction.Request.Method, interaction.Request.URL))
				if requestLine != "" {
					section.items = append(section.items, debugItem{key: fmt.Sprintf("%s.request", prefix), value: requestLine})
				}
				if len(interaction.Request.Headers) > 0 {
					section.items = append(section.items, debugItem{key: fmt.Sprintf("%s.request.headers", prefix), value: strings.Join(interaction.Request.Headers, "\n")})
				}
				if strings.TrimSpace(interaction.Request.Body) != "" {
					section.items = append(section.items, debugItem{key: fmt.Sprintf("%s.request.body", prefix), value: interaction.Request.Body})
				}
			}
			if interaction.Response != nil {
				statusText := interaction.Response.StatusText
				if statusText == "" {
					statusText = http.StatusText(interaction.Response.StatusCode)
				}
				if interaction.Response.StatusCode > 0 || statusText != "" {
					section.items = append(section.items, debugItem{key: fmt.Sprintf("%s.response.status", prefix), value: fmt.Sprintf("%d %s", interaction.Response.StatusCode, statusText)})
				}
				if len(interaction.Response.Headers) > 0 {
					section.items = append(section.items, debugItem{key: fmt.Sprintf("%s.response.headers", prefix), value: strings.Join(interaction.Response.Headers, "\n")})
				}
				if interaction.Response.Body != "" {
					section.items = append(section.items, debugItem{key: fmt.Sprintf("%s.response.body", prefix), value: interaction.Response.Body})
				}
			}
		}
	}
	if err != nil {
		section.items = append(section.items, debugItem{key: "error", value: err.Error()})
	}
	return section
}

func buildRepositoryDebugSection() debugSection {
	section := debugSection{title: "Repository"}
	provider := currentDebugContext.repository
	if provider == nil {
		return section
	}
	info := provider.DebugInfo()
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
