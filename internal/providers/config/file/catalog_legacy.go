package file

import (
	"bytes"
	"strings"

	"go.yaml.in/yaml/v3"
)

var legacyCatalogKeyAliases = map[string]string{
	"access-key":               "accessKey",
	"app-role":                 "appRole",
	"auto-init":                "autoInit",
	"auto-sync":                "autoSync",
	"base-dir":                 "baseDir",
	"base-url":                 "baseURL",
	"bundle-file":              "bundleFile",
	"ca-cert-file":             "caCertFile",
	"client-cert-file":         "clientCertFile",
	"client-id":                "clientID",
	"client-key-file":          "clientKeyFile",
	"client-secret":            "clientSecret",
	"current-context":          "currentContext",
	"current-ctx":              "currentContext",
	"custom-headers":           "customHeaders",
	"default-editor":           "defaultEditor",
	"default-headers":          "defaultHeaders",
	"grant-type":               "grantType",
	"health-check":             "healthCheck",
	"http-url":                 "httpURL",
	"https-url":                "httpsURL",
	"insecure-ignore-host-key": "insecureIgnoreHostKey",
	"insecure-skip-verify":     "insecureSkipVerify",
	"key-file":                 "keyFile",
	"known-hosts-file":         "knownHostsFile",
	"kv-version":               "kvVersion",
	"managed-server":           "managedServer",
	"max-concurrent-requests":  "maxConcurrentRequests",
	"no-proxy":                 "noProxy",
	"passphrase-file":          "passphraseFile",
	"path-prefix":              "pathPrefix",
	"private-key-file":         "privateKeyFile",
	"queue-size":               "queueSize",
	"request-throttling":       "requestThrottling",
	"requests-per-second":      "requestsPerSecond",
	"resource-format":          "resourceFormat",
	"role-id":                  "roleID",
	"scope-key":                "scopeKey",
	"secret-id":                "secretID",
	"secret-store":             "secretStore",
	"token-url":                "tokenURL",
}

var legacyCatalogDetectionTokens = buildLegacyCatalogDetectionTokens()

func buildLegacyCatalogDetectionTokens() [][]byte {
	tokens := make([][]byte, 0, len(legacyCatalogKeyAliases))
	for key := range legacyCatalogKeyAliases {
		tokens = append(tokens, []byte(key+":"))
	}
	return tokens
}

func normalizeLegacyCatalogYAML(data []byte) ([]byte, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return data, nil
	}
	if !legacyCatalogMayNeedNormalization(data) {
		return data, nil
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	changed := rewriteLegacyCatalogKeys(&root, "")
	if migrateLegacyCatalogFields(&root) {
		changed = true
	}
	if !changed {
		return data, nil
	}

	if root.Kind == yaml.DocumentNode && len(root.Content) == 1 {
		return yaml.Marshal(root.Content[0])
	}
	return yaml.Marshal(&root)
}

func legacyCatalogMayNeedNormalization(data []byte) bool {
	for _, token := range legacyCatalogDetectionTokens {
		if bytes.Contains(data, token) {
			return true
		}
	}
	return false
}

func rewriteLegacyCatalogKeys(node *yaml.Node, parentKey string) bool {
	if node == nil {
		return false
	}

	changed := false
	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range node.Content {
			if rewriteLegacyCatalogKeys(child, parentKey) {
				changed = true
			}
		}
	case yaml.MappingNode:
		skipAliases := parentKey == "preferences" || parentKey == "defaultHeaders"
		for idx := 0; idx+1 < len(node.Content); idx += 2 {
			keyNode := node.Content[idx]
			valueNode := node.Content[idx+1]
			if !skipAliases {
				if replacement, ok := legacyCatalogKeyAliases[keyNode.Value]; ok {
					changed = true
					keyNode.Value = replacement
				}
			}
			if rewriteLegacyCatalogKeys(valueNode, keyNode.Value) {
				changed = true
			}
		}
	}
	return changed
}

func migrateLegacyCatalogFields(node *yaml.Node) bool {
	if node == nil {
		return false
	}

	changed := false
	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range node.Content {
			if migrateLegacyCatalogFields(child) {
				changed = true
			}
		}
	case yaml.MappingNode:
		for idx := 0; idx+1 < len(node.Content); idx += 2 {
			keyNode := node.Content[idx]
			valueNode := node.Content[idx+1]
			if keyNode.Value == "contexts" && valueNode.Kind == yaml.SequenceNode {
				for _, contextNode := range valueNode.Content {
					if migrateLegacyContextFields(contextNode) {
						changed = true
					}
				}
			}
			if migrateLegacyCatalogFields(valueNode) {
				changed = true
			}
		}
	}
	return changed
}

func migrateLegacyContextFields(contextNode *yaml.Node) bool {
	if contextNode == nil || contextNode.Kind != yaml.MappingNode {
		return false
	}

	repositoryNode := mappingValue(contextNode, "repository")
	if repositoryNode == nil || repositoryNode.Kind != yaml.MappingNode {
		return false
	}

	resourceFormatNode := mappingValue(repositoryNode, "resourceFormat")
	if resourceFormatNode == nil {
		return false
	}

	resourceFormat := strings.TrimSpace(resourceFormatNode.Value)
	changed := removeMappingEntry(repositoryNode, "resourceFormat")
	if resourceFormat == "" {
		return changed
	}

	preferencesNode, created := ensureMappingValue(contextNode, "preferences")
	if created {
		changed = true
	}
	if mappingValue(preferencesNode, "preferredFormat") != nil {
		return changed
	}
	setMappingScalar(preferencesNode, "preferredFormat", resourceFormat)
	return true
}

func mappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		if node.Content[idx].Value == key {
			return node.Content[idx+1]
		}
	}
	return nil
}

func removeMappingEntry(node *yaml.Node, key string) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}
	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		if node.Content[idx].Value == key {
			node.Content = append(node.Content[:idx], node.Content[idx+2:]...)
			return true
		}
	}
	return false
}

func ensureMappingValue(node *yaml.Node, key string) (*yaml.Node, bool) {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil, false
	}
	if valueNode := mappingValue(node, key); valueNode != nil {
		return valueNode, false
	}

	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	valueNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	node.Content = append(node.Content, keyNode, valueNode)
	return valueNode, true
}

func setMappingScalar(node *yaml.Node, key string, value string) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	if valueNode := mappingValue(node, key); valueNode != nil {
		valueNode.Kind = yaml.ScalarNode
		valueNode.Tag = "!!str"
		valueNode.Value = value
		valueNode.Content = nil
		return
	}

	node.Content = append(
		node.Content,
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key},
		&yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: value},
	)
}
