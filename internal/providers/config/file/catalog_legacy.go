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

func normalizeLegacyCatalogYAML(data []byte) ([]byte, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return data, nil
	}

	var root yaml.Node
	if err := yaml.Unmarshal(data, &root); err != nil {
		return nil, err
	}

	rewriteLegacyCatalogKeys(&root, "")
	migrateLegacyCatalogFields(&root)

	if root.Kind == yaml.DocumentNode && len(root.Content) == 1 {
		return yaml.Marshal(root.Content[0])
	}
	return yaml.Marshal(&root)
}

func rewriteLegacyCatalogKeys(node *yaml.Node, parentKey string) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range node.Content {
			rewriteLegacyCatalogKeys(child, parentKey)
		}
	case yaml.MappingNode:
		skipAliases := parentKey == "preferences" || parentKey == "defaultHeaders"
		for idx := 0; idx+1 < len(node.Content); idx += 2 {
			keyNode := node.Content[idx]
			valueNode := node.Content[idx+1]
			if !skipAliases {
				if replacement, ok := legacyCatalogKeyAliases[keyNode.Value]; ok {
					keyNode.Value = replacement
				}
			}
			rewriteLegacyCatalogKeys(valueNode, keyNode.Value)
		}
	}
}

func migrateLegacyCatalogFields(node *yaml.Node) {
	if node == nil {
		return
	}

	switch node.Kind {
	case yaml.DocumentNode, yaml.SequenceNode:
		for _, child := range node.Content {
			migrateLegacyCatalogFields(child)
		}
	case yaml.MappingNode:
		for idx := 0; idx+1 < len(node.Content); idx += 2 {
			keyNode := node.Content[idx]
			valueNode := node.Content[idx+1]
			if keyNode.Value == "contexts" && valueNode.Kind == yaml.SequenceNode {
				for _, contextNode := range valueNode.Content {
					migrateLegacyContextFields(contextNode)
				}
			}
			migrateLegacyCatalogFields(valueNode)
		}
	}
}

func migrateLegacyContextFields(contextNode *yaml.Node) {
	if contextNode == nil || contextNode.Kind != yaml.MappingNode {
		return
	}

	repositoryNode := mappingValue(contextNode, "repository")
	if repositoryNode == nil || repositoryNode.Kind != yaml.MappingNode {
		return
	}

	resourceFormatNode := mappingValue(repositoryNode, "resourceFormat")
	if resourceFormatNode == nil {
		return
	}

	resourceFormat := strings.TrimSpace(resourceFormatNode.Value)
	removeMappingEntry(repositoryNode, "resourceFormat")
	if resourceFormat == "" {
		return
	}

	preferencesNode := ensureMappingValue(contextNode, "preferences")
	if mappingValue(preferencesNode, "preferredFormat") != nil {
		return
	}
	setMappingScalar(preferencesNode, "preferredFormat", resourceFormat)
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

func removeMappingEntry(node *yaml.Node, key string) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	for idx := 0; idx+1 < len(node.Content); idx += 2 {
		if node.Content[idx].Value == key {
			node.Content = append(node.Content[:idx], node.Content[idx+2:]...)
			return
		}
	}
}

func ensureMappingValue(node *yaml.Node, key string) *yaml.Node {
	if node == nil || node.Kind != yaml.MappingNode {
		return nil
	}
	if valueNode := mappingValue(node, key); valueNode != nil {
		return valueNode
	}

	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	valueNode := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
	node.Content = append(node.Content, keyNode, valueNode)
	return valueNode
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
