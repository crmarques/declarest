package cmd

import (
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"

	ctx "declarest/internal/context"
	"declarest/internal/managedserver"
	"declarest/internal/repository"
	"declarest/internal/secrets"
)

func runInteractiveContextSetup(manager *ctx.DefaultContextManager, prompt interactivePrompter, initialName string, force bool) error {
	name := strings.TrimSpace(initialName)
	var err error
	if name == "" {
		prompt.sectionHeader("Context name", "Give this configuration a short, unique name.")
		name, err = prompt.required("Context name: ")
		if err != nil {
			return err
		}
	}
	if err := validateContextName(name); err != nil {
		return err
	}

	if manager != nil {
		exists, err := contextExists(manager, name)
		if err != nil {
			return err
		}
		if exists && !force {
			return fmt.Errorf("context %q already exists", name)
		}
	}

	prompt.sectionHeader("Repository configuration", "Select where to store your resources.")
	repoType, err := prompt.choice("Repository type", []string{"filesystem", "git-local", "git-remote"}, "filesystem", normalizeRepoType)
	if err != nil {
		return err
	}

	repoCfg, err := promptRepositoryConfig(prompt, repoType)
	if err != nil {
		return err
	}

	cfg := &ctx.ContextConfig{Repository: repoCfg}

	managedCfg, err := promptManagedServerConfig(prompt)
	if err != nil {
		return err
	}
	cfg.ManagedServer = managedCfg

	secretsCfg, err := promptSecretsConfig(prompt)
	if err != nil {
		return err
	}
	if secretsCfg != nil {
		cfg.SecretManager = secretsCfg
	}

	if manager == nil {
		return fmt.Errorf("context manager is not configured")
	}
	if force {
		return manager.ReplaceContextConfig(name, cfg)
	}
	return manager.AddContextConfig(name, cfg)
}

func normalizeRepoType(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "filesystem", "fs":
		return "filesystem", true
	case "git-local", "gitlocal", "local":
		return "git-local", true
	case "git-remote", "gitremote", "remote":
		return "git-remote", true
	default:
		return "", false
	}
}

func promptRepositoryConfig(prompt interactivePrompter, repoType string) (*ctx.RepositoryConfig, error) {
	switch repoType {
	case "filesystem":
		baseDir, err := prompt.required("Repository base directory: ")
		if err != nil {
			return nil, err
		}
		return &ctx.RepositoryConfig{
			Filesystem: &repository.FileSystemResourceRepositoryConfig{
				BaseDir: baseDir,
			},
		}, nil
	case "git-local":
		baseDir, err := prompt.required("Local git repository base directory: ")
		if err != nil {
			return nil, err
		}
		return &ctx.RepositoryConfig{
			Git: &repository.GitResourceRepositoryConfig{
				Local: &repository.GitResourceRepositoryLocalConfig{
					BaseDir: baseDir,
				},
			},
		}, nil
	case "git-remote":
		gitCfg, err := promptGitRemoteConfig(prompt)
		if err != nil {
			return nil, err
		}
		return &ctx.RepositoryConfig{Git: gitCfg}, nil
	default:
		return nil, fmt.Errorf("unsupported repository type %q", repoType)
	}
}

func promptGitRemoteConfig(prompt interactivePrompter) (*repository.GitResourceRepositoryConfig, error) {
	prompt.sectionHeader("Remote repository details", "Tell DeclaREST where the repository lives locally and remotely.")
	baseDir, err := prompt.required("Local git repository base directory: ")
	if err != nil {
		return nil, err
	}
	remoteURL, err := prompt.required("Remote repository URL: ")
	if err != nil {
		return nil, err
	}

	cfg := &repository.GitResourceRepositoryConfig{
		Local: &repository.GitResourceRepositoryLocalConfig{
			BaseDir: baseDir,
		},
		Remote: &repository.GitResourceRepositoryRemoteConfig{
			URL: remoteURL,
		},
	}

	authType := ""
	if isSSHGitURL(remoteURL) {
		authType = "ssh"
	} else {
		prompt.sectionHeader("Remote authentication", "Pick how you authenticate when talking to the remote git endpoint.")
		authType, err = prompt.choice("Remote auth", []string{"none", "basic", "access-key"}, "none", normalizeGitAuthType)
		if err != nil {
			return nil, err
		}
	}
	switch authType {
	case "basic":
		username, err := prompt.required("Git basic auth username: ")
		if err != nil {
			return nil, err
		}
		password, err := prompt.requiredSecret("Git basic auth password: ")
		if err != nil {
			return nil, err
		}
		cfg.Remote.Auth = &repository.GitResourceRepositoryRemoteAuthConfig{
			BasicAuth: &repository.GitResourceRepositoryBasicAuthConfig{
				Username: username,
				Password: password,
			},
		}
	case "access-key":
		provider, err := prompt.choice("Git provider", []string{"github", "gitlab", "gitea", "none"}, "none", normalizeGitProviderChoice)
		if err != nil {
			return nil, err
		}
		token, err := prompt.requiredSecret("Git access token: ")
		if err != nil {
			return nil, err
		}
		cfg.Remote.Auth = &repository.GitResourceRepositoryRemoteAuthConfig{
			AccessKey: &repository.GitResourceRepositoryAccessKeyConfig{
				Token: token,
			},
		}
		if provider != "" {
			cfg.Remote.Provider = provider
		}
	case "ssh":
		user, err := prompt.optional("SSH user (leave blank to auto-detect): ")
		if err != nil {
			return nil, err
		}
		keyFile, err := prompt.required("SSH private key file: ")
		if err != nil {
			return nil, err
		}
		passphrase, err := prompt.optionalSecret("SSH key passphrase (leave blank if none): ")
		if err != nil {
			return nil, err
		}
		knownHosts, err := prompt.optional("Known hosts file (leave blank to use system default): ")
		if err != nil {
			return nil, err
		}
		insecure, err := prompt.confirm("Skip SSH host key verification?", false)
		if err != nil {
			return nil, err
		}
		cfg.Remote.Auth = &repository.GitResourceRepositoryRemoteAuthConfig{
			SSH: &repository.GitResourceRepositorySSHAuthConfig{
				User:                  strings.TrimSpace(user),
				PrivateKeyFile:        keyFile,
				Passphrase:            strings.TrimSpace(passphrase),
				KnownHostsFile:        strings.TrimSpace(knownHosts),
				InsecureIgnoreHostKey: insecure,
			},
		}
	case "none":
	default:
		return nil, fmt.Errorf("unsupported auth type %q", authType)
	}

	branch, err := prompt.optional("Remote branch (leave blank for default): ")
	if err != nil {
		return nil, err
	}
	branch = strings.TrimSpace(branch)
	if branch != "" {
		cfg.Remote.Branch = branch
	}

	autoSync, err := prompt.confirm("Enable auto-sync to remote?", true)
	if err != nil {
		return nil, err
	}
	if !autoSync {
		cfg.Remote.AutoSync = boolPtr(false)
	}

	if isHTTPSURL(remoteURL) {
		insecureTLS, err := prompt.confirm("Skip TLS verification for remote?", false)
		if err != nil {
			return nil, err
		}
		if insecureTLS {
			cfg.Remote.TLS = &repository.GitResourceRepositoryRemoteTLSConfig{
				InsecureSkipVerify: true,
			}
		}
	}

	return cfg, nil
}

func promptManagedServerConfig(prompt interactivePrompter) (*ctx.ManagedServerConfig, error) {
	prompt.sectionHeader("Managed server configuration", "Provide the HTTP endpoint plus auth details.")

	baseURL, err := prompt.required("Managed server base URL: ")
	if err != nil {
		return nil, err
	}

	httpCfg := &managedserver.HTTPResourceServerConfig{
		BaseURL: baseURL,
	}

	openapiSource, err := prompt.optional("OpenAPI spec URL or file (leave blank to skip): ")
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(openapiSource) != "" {
		httpCfg.OpenAPI = strings.TrimSpace(openapiSource)
	}

	authType, err := prompt.choice("Managed server auth", []string{"none", "basic", "bearer", "custom-header", "oauth2"}, "none", normalizeManagedAuthType)
	if err != nil {
		return nil, err
	}
	switch authType {
	case "basic":
		username, err := prompt.required("Managed server basic auth username: ")
		if err != nil {
			return nil, err
		}
		password, err := prompt.requiredSecret("Managed server basic auth password: ")
		if err != nil {
			return nil, err
		}
		httpCfg.Auth = &managedserver.HTTPResourceServerAuthConfig{
			BasicAuth: &managedserver.HTTPResourceServerBasicAuthConfig{
				Username: username,
				Password: password,
			},
		}
	case "bearer":
		token, err := prompt.requiredSecret("Managed server bearer token: ")
		if err != nil {
			return nil, err
		}
		httpCfg.Auth = &managedserver.HTTPResourceServerAuthConfig{
			BearerToken: &managedserver.HTTPResourceServerBearerTokenConfig{
				Token: token,
			},
		}
	case "custom-header":
		header, err := prompt.required("Managed server auth header name: ")
		if err != nil {
			return nil, err
		}
		token, err := prompt.requiredSecret("Managed server auth token: ")
		if err != nil {
			return nil, err
		}
		httpCfg.Auth = &managedserver.HTTPResourceServerAuthConfig{
			CustomHeader: &managedserver.HTTPResourceServerCustomHeaderConfig{
				Header: strings.TrimSpace(header),
				Token:  token,
			},
		}
	case "oauth2":
		tokenURL, err := prompt.required("OAuth2 token URL: ")
		if err != nil {
			return nil, err
		}
		grantType, err := prompt.choice("OAuth2 grant type", []string{"client_credentials", "password"}, "client_credentials", normalizeOAuthGrantType)
		if err != nil {
			return nil, err
		}
		oauthCfg := &managedserver.HTTPResourceServerOAuth2Config{
			TokenURL:  tokenURL,
			GrantType: grantType,
		}
		if grantType == "password" {
			username, err := prompt.required("OAuth2 username: ")
			if err != nil {
				return nil, err
			}
			password, err := prompt.requiredSecret("OAuth2 password: ")
			if err != nil {
				return nil, err
			}
			oauthCfg.Username = username
			oauthCfg.Password = password
		}
		clientID, err := prompt.optional("OAuth2 client ID (leave blank to skip): ")
		if err != nil {
			return nil, err
		}
		clientSecret, err := prompt.optionalSecret("OAuth2 client secret (leave blank to skip): ")
		if err != nil {
			return nil, err
		}
		scope, err := prompt.optional("OAuth2 scope (leave blank to skip): ")
		if err != nil {
			return nil, err
		}
		audience, err := prompt.optional("OAuth2 audience (leave blank to skip): ")
		if err != nil {
			return nil, err
		}
		oauthCfg.ClientID = strings.TrimSpace(clientID)
		oauthCfg.ClientSecret = strings.TrimSpace(clientSecret)
		oauthCfg.Scope = strings.TrimSpace(scope)
		oauthCfg.Audience = strings.TrimSpace(audience)
		httpCfg.Auth = &managedserver.HTTPResourceServerAuthConfig{
			OAuth2: oauthCfg,
		}
	case "none":
	default:
		return nil, fmt.Errorf("unsupported auth type %q", authType)
	}

	addHeaders, err := prompt.confirm("Add default headers?", false)
	if err != nil {
		return nil, err
	}
	if addHeaders {
		headers, err := promptHeaders(prompt)
		if err != nil {
			return nil, err
		}
		if len(headers) > 0 {
			httpCfg.DefaultHeaders = headers
		}
	}

	if isHTTPSURL(baseURL) {
		insecureTLS, err := prompt.confirm("Skip TLS verification for managed server?", false)
		if err != nil {
			return nil, err
		}
		if insecureTLS {
			httpCfg.TLS = &managedserver.HTTPResourceServerTLSConfig{
				InsecureSkipVerify: true,
			}
		}
	}

	return &ctx.ManagedServerConfig{HTTP: httpCfg}, nil
}

func promptSecretsConfig(prompt interactivePrompter) (*secrets.SecretsManagerConfig, error) {
	prompt.sectionHeader("Secret store configuration (optional)", "Use a secret store to keep sensitive values out of resources.")
	configure, err := prompt.confirm("Configure secret store?", false)
	if err != nil {
		return nil, err
	}
	if !configure {
		return nil, nil
	}

	storeType, err := prompt.choice("Secret store type", []string{"file", "vault"}, "file", normalizeSecretStoreType)
	if err != nil {
		return nil, err
	}

	switch storeType {
	case "file":
		fileCfg, err := promptFileSecretsConfig(prompt)
		if err != nil {
			return nil, err
		}
		return &secrets.SecretsManagerConfig{File: fileCfg}, nil
	case "vault":
		vaultCfg, err := promptVaultSecretsConfig(prompt)
		if err != nil {
			return nil, err
		}
		return &secrets.SecretsManagerConfig{Vault: vaultCfg}, nil
	default:
		return nil, fmt.Errorf("unsupported secret store type %q", storeType)
	}
}

func promptFileSecretsConfig(prompt interactivePrompter) (*secrets.FileSecretsManagerConfig, error) {
	prompt.sectionHeader("File secret store options", "Provide the secrets file location and key/passphrase.")
	path, err := prompt.required("Secrets file path: ")
	if err != nil {
		return nil, err
	}
	fileCfg := &secrets.FileSecretsManagerConfig{
		Path: path,
	}

	keySource, err := prompt.choice("Key source", []string{"key", "key-file", "passphrase", "passphrase-file"}, "key", normalizeKeySource)
	if err != nil {
		return nil, err
	}
	switch keySource {
	case "key":
		key, err := prompt.requiredSecret("Raw key (base64, 32 bytes): ")
		if err != nil {
			return nil, err
		}
		fileCfg.Key = key
	case "key-file":
		keyFile, err := prompt.required("Key file path: ")
		if err != nil {
			return nil, err
		}
		fileCfg.KeyFile = keyFile
	case "passphrase":
		passphrase, err := prompt.requiredSecret("Passphrase: ")
		if err != nil {
			return nil, err
		}
		fileCfg.Passphrase = passphrase
	case "passphrase-file":
		passphraseFile, err := prompt.required("Passphrase file path: ")
		if err != nil {
			return nil, err
		}
		fileCfg.PassphraseFile = passphraseFile
	default:
		return nil, fmt.Errorf("unsupported key source %q", keySource)
	}

	if keySource == "passphrase" || keySource == "passphrase-file" {
		customize, err := prompt.confirm("Customize KDF parameters?", false)
		if err != nil {
			return nil, err
		}
		if customize {
			kdf := &secrets.FileSecretsManagerKDFConfig{}
			if value, ok, err := promptOptionalUint32(prompt, "KDF time (iterations, default 3): "); err != nil {
				return nil, err
			} else if ok {
				kdf.Time = value
			}
			if value, ok, err := promptOptionalUint32(prompt, "KDF memory (KiB, default 65536): "); err != nil {
				return nil, err
			} else if ok {
				kdf.Memory = value
			}
			if value, ok, err := promptOptionalUint8(prompt, "KDF threads (default CPU-based): "); err != nil {
				return nil, err
			} else if ok {
				kdf.Threads = value
			}
			if kdf.Time != 0 || kdf.Memory != 0 || kdf.Threads != 0 {
				fileCfg.KDF = kdf
			}
		}
	}

	return fileCfg, nil
}

func promptVaultSecretsConfig(prompt interactivePrompter) (*secrets.VaultSecretsManagerConfig, error) {
	prompt.sectionHeader("Vault secret store options", "Vault connection details, auth, and TLS settings.")
	address, err := prompt.required("Vault address (https://vault.example.com): ")
	if err != nil {
		return nil, err
	}
	mount, err := prompt.optional("Vault mount (default: secret): ")
	if err != nil {
		return nil, err
	}
	pathPrefix, err := prompt.optional("Vault path prefix (optional): ")
	if err != nil {
		return nil, err
	}
	kvVersion := 2
	if raw, err := prompt.optional("Vault KV version (1 or 2, default 2): "); err != nil {
		return nil, err
	} else if strings.TrimSpace(raw) != "" {
		parsed, err := strconv.Atoi(strings.TrimSpace(raw))
		if err != nil || (parsed != 1 && parsed != 2) {
			return nil, fmt.Errorf("invalid KV version %q", raw)
		}
		kvVersion = parsed
	}

	authType, err := prompt.choice("Vault auth type", []string{"token", "password", "approle"}, "token", normalizeVaultAuthType)
	if err != nil {
		return nil, err
	}
	authCfg := &secrets.VaultSecretsManagerAuthConfig{}
	switch authType {
	case "token":
		token, err := prompt.requiredSecret("Vault token: ")
		if err != nil {
			return nil, err
		}
		authCfg.Token = token
	case "password":
		username, err := prompt.required("Vault username: ")
		if err != nil {
			return nil, err
		}
		password, err := prompt.requiredSecret("Vault password: ")
		if err != nil {
			return nil, err
		}
		authMount, err := prompt.optional("Vault password auth mount (default: userpass): ")
		if err != nil {
			return nil, err
		}
		authCfg.Password = &secrets.VaultSecretsManagerPasswordAuthConfig{
			Username: username,
			Password: password,
			Mount:    authMount,
		}
	case "approle":
		roleID, err := prompt.requiredSecret("Vault AppRole role_id: ")
		if err != nil {
			return nil, err
		}
		secretID, err := prompt.requiredSecret("Vault AppRole secret_id: ")
		if err != nil {
			return nil, err
		}
		authMount, err := prompt.optional("Vault AppRole auth mount (default: approle): ")
		if err != nil {
			return nil, err
		}
		authCfg.AppRole = &secrets.VaultSecretsManagerAppRoleAuthConfig{
			RoleID:   roleID,
			SecretID: secretID,
			Mount:    authMount,
		}
	default:
		return nil, fmt.Errorf("unsupported vault auth type %q", authType)
	}

	var tlsCfg *secrets.VaultSecretsManagerTLSConfig
	configureTLS, err := prompt.confirm("Configure mTLS for Vault?", false)
	if err != nil {
		return nil, err
	}
	if configureTLS {
		caCert, err := prompt.optional("Vault CA cert file (leave blank to skip): ")
		if err != nil {
			return nil, err
		}
		clientCert, err := prompt.optional("Vault client cert file (leave blank to skip): ")
		if err != nil {
			return nil, err
		}
		clientKey := ""
		if strings.TrimSpace(clientCert) != "" {
			clientKey, err = prompt.required("Vault client key file: ")
			if err != nil {
				return nil, err
			}
		} else {
			clientKey, err = prompt.optional("Vault client key file (leave blank to skip): ")
			if err != nil {
				return nil, err
			}
		}
		insecureTLS := false
		if isHTTPSURL(address) {
			insecureTLS, err = prompt.confirm("Skip TLS verification for Vault?", false)
			if err != nil {
				return nil, err
			}
		}
		if strings.TrimSpace(clientCert) != "" && strings.TrimSpace(clientKey) == "" {
			return nil, errors.New("vault client key file is required when providing a client certificate")
		}
		if strings.TrimSpace(clientKey) != "" && strings.TrimSpace(clientCert) == "" {
			return nil, errors.New("vault client cert file is required when providing a client key")
		}
		tlsCfg = &secrets.VaultSecretsManagerTLSConfig{
			CACertFile:         caCert,
			ClientCertFile:     clientCert,
			ClientKeyFile:      clientKey,
			InsecureSkipVerify: insecureTLS,
		}
	} else if isHTTPSURL(address) {
		insecureTLS, err := prompt.confirm("Skip TLS verification for Vault?", false)
		if err != nil {
			return nil, err
		}
		if insecureTLS {
			tlsCfg = &secrets.VaultSecretsManagerTLSConfig{
				InsecureSkipVerify: true,
			}
		}
	}

	return &secrets.VaultSecretsManagerConfig{
		Address:    address,
		Mount:      mount,
		PathPrefix: pathPrefix,
		KVVersion:  kvVersion,
		Auth:       authCfg,
		TLS:        tlsCfg,
	}, nil
}

func promptHeaders(prompt interactivePrompter) (map[string]string, error) {
	headers := map[string]string{}
	for {
		value, err := prompt.readLine("Header (key=value, leave blank to finish): ")
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			break
		}
		parts := strings.SplitN(value, "=", 2)
		if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" {
			prompt.messagef("invalid header: %s\n", value)
			continue
		}
		headers[strings.TrimSpace(parts[0])] = strings.TrimSpace(parts[1])
	}
	if len(headers) == 0 {
		return nil, nil
	}
	return headers, nil
}

func promptOptionalUint32(prompt interactivePrompter, label string) (uint32, bool, error) {
	for {
		value, err := prompt.optional(label)
		if err != nil {
			return 0, false, err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return 0, false, nil
		}
		parsed, err := strconv.ParseUint(value, 10, 32)
		if err != nil {
			prompt.messagef("invalid number: %s\n", value)
			continue
		}
		if parsed == 0 {
			prompt.messagef("value must be greater than zero\n")
			continue
		}
		return uint32(parsed), true, nil
	}
}

func promptOptionalUint8(prompt interactivePrompter, label string) (uint8, bool, error) {
	for {
		value, err := prompt.optional(label)
		if err != nil {
			return 0, false, err
		}
		value = strings.TrimSpace(value)
		if value == "" {
			return 0, false, nil
		}
		parsed, err := strconv.ParseUint(value, 10, 8)
		if err != nil {
			prompt.messagef("invalid number: %s\n", value)
			continue
		}
		if parsed == 0 {
			prompt.messagef("value must be greater than zero\n")
			continue
		}
		return uint8(parsed), true, nil
	}
}

func normalizeGitAuthType(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "none", "no":
		return "none", true
	case "basic", "basic-auth", "basic_auth":
		return "basic", true
	case "ssh":
		return "ssh", true
	case "access-key", "access_key", "token", "pat":
		return "access-key", true
	default:
		return "", false
	}
}

func normalizeGitProviderChoice(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "none", "":
		return "", true
	case "github":
		return "github", true
	case "gitlab":
		return "gitlab", true
	case "gitea":
		return "gitea", true
	default:
		return "", false
	}
}

func normalizeManagedAuthType(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "none", "no":
		return "none", true
	case "basic", "basic-auth", "basic_auth":
		return "basic", true
	case "bearer", "token", "bearer-token", "bearer_token":
		return "bearer", true
	case "custom", "custom-header", "custom_header", "header":
		return "custom-header", true
	case "oauth2", "oauth":
		return "oauth2", true
	default:
		return "", false
	}
}

func normalizeOAuthGrantType(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "client_credentials", "client-credentials", "client":
		return "client_credentials", true
	case "password":
		return "password", true
	default:
		return "", false
	}
}

func normalizeKeySource(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "key", "raw":
		return "key", true
	case "key-file", "keyfile":
		return "key-file", true
	case "passphrase", "pass":
		return "passphrase", true
	case "passphrase-file", "passphrasefile", "passfile":
		return "passphrase-file", true
	default:
		return "", false
	}
}

func normalizeSecretStoreType(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "file", "fs":
		return "file", true
	case "vault", "hashicorp", "hashicorp-vault":
		return "vault", true
	default:
		return "", false
	}
}

func normalizeVaultAuthType(raw string) (string, bool) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "token", "bearer":
		return "token", true
	case "password", "userpass", "user-pass":
		return "password", true
	case "approle", "app-role", "app_role":
		return "approle", true
	default:
		return "", false
	}
}

func isHTTPSURL(raw string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(raw)), "https://")
}

func isSSHGitURL(raw string) bool {
	trimmed := strings.TrimSpace(raw)
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "ssh://") {
		return true
	}
	if strings.HasPrefix(trimmed, "git@") {
		return true
	}
	if strings.Contains(trimmed, "@") && strings.Contains(trimmed, ":") &&
		!strings.HasPrefix(lower, "http://") && !strings.HasPrefix(lower, "https://") {
		return true
	}
	return false
}

func boolPtr(value bool) *bool {
	return &value
}
