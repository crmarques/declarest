package config

import (
	"fmt"
	"strconv"
	"strings"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/spf13/cobra"
)

const resourceFormatRemoteDefaultOption = "remote-default"

func resolveCreateContextInput(
	command *cobra.Command,
	input common.InputFlags,
	prompter configPrompter,
	contextName string,
) (configdomain.Context, error) {
	if shouldUseInteractiveCreate(command, input, prompter) {
		return promptCreateContext(command, prompter, contextName)
	}

	cfg, err := decodeContextStrict(command, input)
	if err != nil {
		return configdomain.Context{}, err
	}

	if strings.TrimSpace(contextName) != "" {
		cfg.Name = strings.TrimSpace(contextName)
	}

	return cfg, nil
}

func shouldUseInteractiveCreate(command *cobra.Command, input common.InputFlags, prompter configPrompter) bool {
	if input.Payload != "" {
		return false
	}
	if common.HasPipedInput(command) {
		return false
	}
	return prompter.IsInteractive(command)
}

func promptCreateContext(command *cobra.Command, prompter configPrompter, contextName string) (configdomain.Context, error) {
	name := strings.TrimSpace(contextName)
	if name == "" {
		var err error
		name, err = promptRequiredInput(command, prompter, "Context name: ", "context name")
		if err != nil {
			return configdomain.Context{}, err
		}
	}

	resourceFormatSelection, err := prompter.Select(
		command,
		"Select resource format (optional; remote-default keeps remote resource format)",
		[]string{resourceFormatRemoteDefaultOption, configdomain.ResourceFormatYAML, configdomain.ResourceFormatJSON},
	)
	if err != nil {
		return configdomain.Context{}, err
	}
	resourceFormat := strings.TrimSpace(resourceFormatSelection)
	if resourceFormat == resourceFormatRemoteDefaultOption {
		resourceFormat = ""
	}

	repositoryType, err := prompter.Select(command, "Select repository type", []string{"filesystem", "git"})
	if err != nil {
		return configdomain.Context{}, err
	}

	contextCfg := configdomain.Context{
		Name: name,
		Repository: configdomain.Repository{
			ResourceFormat: strings.TrimSpace(resourceFormat),
		},
	}

	repositoryBaseDir, err := promptRepositoryConfig(command, prompter, &contextCfg, repositoryType)
	if err != nil {
		return configdomain.Context{}, err
	}

	metadataPrompt := fmt.Sprintf("Metadata base-dir (defaults to %s): ", repositoryBaseDir)
	metadataBaseDir, err := promptOptionalInput(command, prompter, metadataPrompt)
	if err != nil {
		return configdomain.Context{}, err
	}
	if metadataBaseDir == "" {
		metadataBaseDir = repositoryBaseDir
	}
	contextCfg.Metadata.BaseDir = metadataBaseDir

	resourceServer, err := promptResourceServer(command, prompter)
	if err != nil {
		return configdomain.Context{}, err
	}
	contextCfg.ResourceServer = resourceServer

	includeSecretStore, err := prompter.Confirm(command, "Configure secret-store?", false)
	if err != nil {
		return configdomain.Context{}, err
	}
	if includeSecretStore {
		secretStore, secretErr := promptSecretStore(command, prompter)
		if secretErr != nil {
			return configdomain.Context{}, secretErr
		}
		contextCfg.SecretStore = secretStore
	}

	includePreferences, err := prompter.Confirm(command, "Configure preferences?", false)
	if err != nil {
		return configdomain.Context{}, err
	}
	if includePreferences {
		preferences, prefErr := promptStringMap(command, prompter, "Preference")
		if prefErr != nil {
			return configdomain.Context{}, prefErr
		}
		contextCfg.Preferences = preferences
	}

	return contextCfg, nil
}

func promptRepositoryConfig(
	command *cobra.Command,
	prompter configPrompter,
	contextCfg *configdomain.Context,
	repositoryType string,
) (string, error) {
	switch strings.TrimSpace(repositoryType) {
	case "filesystem":
		baseDir, err := promptRequiredInput(command, prompter, "Repository base-dir: ", "repository base-dir")
		if err != nil {
			return "", err
		}
		contextCfg.Repository.Filesystem = &configdomain.FilesystemRepository{BaseDir: baseDir}
		return baseDir, nil
	case "git":
		baseDir, err := promptRequiredInput(command, prompter, "Git local base-dir: ", "git local base-dir")
		if err != nil {
			return "", err
		}

		autoInit, err := prompter.Confirm(command, "Enable git local auto-init?", true)
		if err != nil {
			return "", err
		}

		localConfig := configdomain.GitLocal{BaseDir: baseDir}
		if !autoInit {
			autoInitFalse := false
			localConfig.AutoInit = &autoInitFalse
		}

		repo := &configdomain.GitRepository{
			Local: localConfig,
		}

		includeRemote, err := prompter.Confirm(command, "Configure git remote?", false)
		if err != nil {
			return "", err
		}
		if includeRemote {
			remote, remoteErr := promptGitRemote(command, prompter)
			if remoteErr != nil {
				return "", remoteErr
			}
			repo.Remote = remote
		}

		contextCfg.Repository.Git = repo
		return baseDir, nil
	default:
		return "", common.ValidationError("invalid repository type selected", nil)
	}
}

func promptGitRemote(command *cobra.Command, prompter configPrompter) (*configdomain.GitRemote, error) {
	url, err := promptRequiredInput(command, prompter, "Git remote URL: ", "git remote url")
	if err != nil {
		return nil, err
	}
	branch, err := promptOptionalInput(command, prompter, "Git remote branch (optional): ")
	if err != nil {
		return nil, err
	}
	provider, err := promptOptionalInput(command, prompter, "Git remote provider (optional): ")
	if err != nil {
		return nil, err
	}

	remote := &configdomain.GitRemote{
		URL:      url,
		Branch:   branch,
		Provider: provider,
	}

	autoSync, err := prompter.Confirm(command, "Enable git remote auto-sync?", false)
	if err != nil {
		return nil, err
	}
	remote.AutoSync = autoSync

	includeAuth, err := prompter.Confirm(command, "Configure git remote auth?", false)
	if err != nil {
		return nil, err
	}
	if includeAuth {
		auth, authErr := promptGitAuth(command, prompter)
		if authErr != nil {
			return nil, authErr
		}
		remote.Auth = auth
	}

	includeTLS, err := prompter.Confirm(command, "Configure git remote TLS?", false)
	if err != nil {
		return nil, err
	}
	if includeTLS {
		tls, tlsErr := promptTLS(command, prompter)
		if tlsErr != nil {
			return nil, tlsErr
		}
		remote.TLS = tls
	}

	return remote, nil
}

func promptGitAuth(command *cobra.Command, prompter configPrompter) (*configdomain.GitAuth, error) {
	method, err := prompter.Select(command, "Select git auth method", []string{"basic-auth", "ssh", "access-key"})
	if err != nil {
		return nil, err
	}

	auth := &configdomain.GitAuth{}
	switch strings.TrimSpace(method) {
	case "basic-auth":
		username, inputErr := promptRequiredInput(command, prompter, "Git basic-auth username: ", "git basic-auth username")
		if inputErr != nil {
			return nil, inputErr
		}
		password, inputErr := promptRequiredInput(command, prompter, "Git basic-auth password: ", "git basic-auth password")
		if inputErr != nil {
			return nil, inputErr
		}
		auth.BasicAuth = &configdomain.BasicAuth{
			Username: username,
			Password: password,
		}
	case "ssh":
		user, inputErr := promptRequiredInput(command, prompter, "Git SSH user: ", "git ssh user")
		if inputErr != nil {
			return nil, inputErr
		}
		privateKeyFile, inputErr := promptRequiredInput(
			command,
			prompter,
			"Git SSH private-key-file: ",
			"git ssh private-key-file",
		)
		if inputErr != nil {
			return nil, inputErr
		}
		passphrase, inputErr := promptOptionalInput(command, prompter, "Git SSH passphrase (optional): ")
		if inputErr != nil {
			return nil, inputErr
		}
		knownHostsFile, inputErr := promptOptionalInput(command, prompter, "Git SSH known-hosts-file (optional): ")
		if inputErr != nil {
			return nil, inputErr
		}
		insecureIgnoreHostKey, inputErr := prompter.Confirm(command, "Git SSH insecure-ignore-host-key?", false)
		if inputErr != nil {
			return nil, inputErr
		}
		auth.SSH = &configdomain.SSHAuth{
			User:                  user,
			PrivateKeyFile:        privateKeyFile,
			Passphrase:            passphrase,
			KnownHostsFile:        knownHostsFile,
			InsecureIgnoreHostKey: insecureIgnoreHostKey,
		}
	case "access-key":
		token, inputErr := promptRequiredInput(command, prompter, "Git access-key token: ", "git access-key token")
		if inputErr != nil {
			return nil, inputErr
		}
		auth.AccessKey = &configdomain.AccessKeyAuth{Token: token}
	default:
		return nil, common.ValidationError("invalid git auth method selected", nil)
	}

	return auth, nil
}

func promptResourceServer(command *cobra.Command, prompter configPrompter) (*configdomain.ResourceServer, error) {
	baseURL, err := promptRequiredInput(command, prompter, "Resource-server base-url: ", "resource-server base-url")
	if err != nil {
		return nil, err
	}
	openAPI, err := promptOptionalInput(command, prompter, "Resource-server OpenAPI path/url (optional): ")
	if err != nil {
		return nil, err
	}

	server := &configdomain.HTTPServer{
		BaseURL: baseURL,
		OpenAPI: openAPI,
	}

	includeHeaders, err := prompter.Confirm(command, "Configure resource-server default headers?", false)
	if err != nil {
		return nil, err
	}
	if includeHeaders {
		headers, headerErr := promptStringMap(command, prompter, "Resource-server default header")
		if headerErr != nil {
			return nil, headerErr
		}
		server.DefaultHeaders = headers
	}

	auth, err := promptHTTPAuth(command, prompter)
	if err != nil {
		return nil, err
	}
	server.Auth = auth

	includeTLS, err := prompter.Confirm(command, "Configure resource-server TLS?", false)
	if err != nil {
		return nil, err
	}
	if includeTLS {
		tls, tlsErr := promptTLS(command, prompter)
		if tlsErr != nil {
			return nil, tlsErr
		}
		server.TLS = tls
	}

	return &configdomain.ResourceServer{HTTP: server}, nil
}

func promptHTTPAuth(command *cobra.Command, prompter configPrompter) (*configdomain.HTTPAuth, error) {
	method, err := prompter.Select(
		command,
		"Select resource-server auth method",
		[]string{"oauth2", "basic-auth", "bearer-token", "custom-header"},
	)
	if err != nil {
		return nil, err
	}

	auth := &configdomain.HTTPAuth{}
	switch strings.TrimSpace(method) {
	case "oauth2":
		tokenURL, inputErr := promptRequiredInput(
			command,
			prompter,
			"OAuth2 token-url: ",
			"oauth2 token-url",
		)
		if inputErr != nil {
			return nil, inputErr
		}
		grantType, inputErr := promptOptionalInput(
			command,
			prompter,
			fmt.Sprintf("OAuth2 grant-type (default %s): ", configdomain.OAuthClientCreds),
		)
		if inputErr != nil {
			return nil, inputErr
		}
		if grantType == "" {
			grantType = configdomain.OAuthClientCreds
		}
		clientID, inputErr := promptRequiredInput(command, prompter, "OAuth2 client-id: ", "oauth2 client-id")
		if inputErr != nil {
			return nil, inputErr
		}
		clientSecret, inputErr := promptRequiredInput(command, prompter, "OAuth2 client-secret: ", "oauth2 client-secret")
		if inputErr != nil {
			return nil, inputErr
		}
		username, inputErr := promptOptionalInput(command, prompter, "OAuth2 username (optional): ")
		if inputErr != nil {
			return nil, inputErr
		}
		password, inputErr := promptOptionalInput(command, prompter, "OAuth2 password (optional): ")
		if inputErr != nil {
			return nil, inputErr
		}
		scope, inputErr := promptOptionalInput(command, prompter, "OAuth2 scope (optional): ")
		if inputErr != nil {
			return nil, inputErr
		}
		audience, inputErr := promptOptionalInput(command, prompter, "OAuth2 audience (optional): ")
		if inputErr != nil {
			return nil, inputErr
		}
		auth.OAuth2 = &configdomain.OAuth2{
			TokenURL:     tokenURL,
			GrantType:    grantType,
			ClientID:     clientID,
			ClientSecret: clientSecret,
			Username:     username,
			Password:     password,
			Scope:        scope,
			Audience:     audience,
		}
	case "basic-auth":
		username, inputErr := promptRequiredInput(command, prompter, "Basic auth username: ", "basic auth username")
		if inputErr != nil {
			return nil, inputErr
		}
		password, inputErr := promptRequiredInput(command, prompter, "Basic auth password: ", "basic auth password")
		if inputErr != nil {
			return nil, inputErr
		}
		auth.BasicAuth = &configdomain.BasicAuth{
			Username: username,
			Password: password,
		}
	case "bearer-token":
		token, inputErr := promptRequiredInput(command, prompter, "Bearer token: ", "bearer token")
		if inputErr != nil {
			return nil, inputErr
		}
		auth.BearerToken = &configdomain.BearerTokenAuth{Token: token}
	case "custom-header":
		header, inputErr := promptRequiredInput(command, prompter, "Custom auth header name: ", "custom auth header name")
		if inputErr != nil {
			return nil, inputErr
		}
		token, inputErr := promptRequiredInput(command, prompter, "Custom auth token: ", "custom auth token")
		if inputErr != nil {
			return nil, inputErr
		}
		auth.CustomHeader = &configdomain.HeaderTokenAuth{
			Header: header,
			Token:  token,
		}
	default:
		return nil, common.ValidationError("invalid resource-server auth method selected", nil)
	}

	return auth, nil
}

func promptSecretStore(command *cobra.Command, prompter configPrompter) (*configdomain.SecretStore, error) {
	provider, err := prompter.Select(command, "Select secret-store provider", []string{"file", "vault"})
	if err != nil {
		return nil, err
	}

	store := &configdomain.SecretStore{}
	switch strings.TrimSpace(provider) {
	case "file":
		fileStore, storeErr := promptFileSecretStore(command, prompter)
		if storeErr != nil {
			return nil, storeErr
		}
		store.File = fileStore
	case "vault":
		vaultStore, storeErr := promptVaultSecretStore(command, prompter)
		if storeErr != nil {
			return nil, storeErr
		}
		store.Vault = vaultStore
	default:
		return nil, common.ValidationError("invalid secret-store provider selected", nil)
	}

	return store, nil
}

func promptFileSecretStore(command *cobra.Command, prompter configPrompter) (*configdomain.FileSecretStore, error) {
	path, err := promptRequiredInput(command, prompter, "Secret-store file path: ", "secret-store file path")
	if err != nil {
		return nil, err
	}
	keySource, err := prompter.Select(
		command,
		"Select secret-store file key source",
		[]string{"key", "key-file", "passphrase", "passphrase-file"},
	)
	if err != nil {
		return nil, err
	}

	store := &configdomain.FileSecretStore{Path: path}
	switch strings.TrimSpace(keySource) {
	case "key":
		store.Key, err = promptRequiredInput(command, prompter, "Secret-store file key: ", "secret-store file key")
	case "key-file":
		store.KeyFile, err = promptRequiredInput(
			command,
			prompter,
			"Secret-store file key-file: ",
			"secret-store file key-file",
		)
	case "passphrase":
		store.Passphrase, err = promptRequiredInput(
			command,
			prompter,
			"Secret-store file passphrase: ",
			"secret-store file passphrase",
		)
	case "passphrase-file":
		store.PassphraseFile, err = promptRequiredInput(
			command,
			prompter,
			"Secret-store file passphrase-file: ",
			"secret-store file passphrase-file",
		)
	default:
		return nil, common.ValidationError("invalid secret-store file key source selected", nil)
	}
	if err != nil {
		return nil, err
	}

	includeKDF, err := prompter.Confirm(command, "Configure secret-store file KDF parameters?", false)
	if err != nil {
		return nil, err
	}
	if includeKDF {
		kdf, kdfErr := promptKDF(command, prompter)
		if kdfErr != nil {
			return nil, kdfErr
		}
		store.KDF = kdf
	}

	return store, nil
}

func promptKDF(command *cobra.Command, prompter configPrompter) (*configdomain.KDF, error) {
	timeValue, hasTime, err := promptOptionalInt(command, prompter, "KDF time (optional integer): ", "kdf time")
	if err != nil {
		return nil, err
	}
	memoryValue, hasMemory, err := promptOptionalInt(command, prompter, "KDF memory (optional integer): ", "kdf memory")
	if err != nil {
		return nil, err
	}
	threadValue, hasThreads, err := promptOptionalInt(command, prompter, "KDF threads (optional integer): ", "kdf threads")
	if err != nil {
		return nil, err
	}

	if !hasTime && !hasMemory && !hasThreads {
		return nil, nil
	}

	kdf := &configdomain.KDF{}
	if hasTime {
		kdf.Time = timeValue
	}
	if hasMemory {
		kdf.Memory = memoryValue
	}
	if hasThreads {
		kdf.Threads = threadValue
	}

	return kdf, nil
}

func promptVaultSecretStore(command *cobra.Command, prompter configPrompter) (*configdomain.VaultSecretStore, error) {
	address, err := promptRequiredInput(command, prompter, "Vault address: ", "vault address")
	if err != nil {
		return nil, err
	}
	mount, err := promptOptionalInput(command, prompter, "Vault mount (optional): ")
	if err != nil {
		return nil, err
	}
	pathPrefix, err := promptOptionalInput(command, prompter, "Vault path-prefix (optional): ")
	if err != nil {
		return nil, err
	}
	kvVersion, hasKVVersion, err := promptOptionalInt(command, prompter, "Vault kv-version (optional integer): ", "vault kv-version")
	if err != nil {
		return nil, err
	}
	auth, err := promptVaultAuth(command, prompter)
	if err != nil {
		return nil, err
	}

	store := &configdomain.VaultSecretStore{
		Address:    address,
		Mount:      mount,
		PathPrefix: pathPrefix,
		Auth:       auth,
	}
	if hasKVVersion {
		store.KVVersion = kvVersion
	}

	includeTLS, err := prompter.Confirm(command, "Configure vault TLS?", false)
	if err != nil {
		return nil, err
	}
	if includeTLS {
		tls, tlsErr := promptTLS(command, prompter)
		if tlsErr != nil {
			return nil, tlsErr
		}
		store.TLS = tls
	}

	return store, nil
}

func promptVaultAuth(command *cobra.Command, prompter configPrompter) (*configdomain.VaultAuth, error) {
	method, err := prompter.Select(command, "Select vault auth method", []string{"token", "password", "approle"})
	if err != nil {
		return nil, err
	}

	auth := &configdomain.VaultAuth{}
	switch strings.TrimSpace(method) {
	case "token":
		token, inputErr := promptRequiredInput(command, prompter, "Vault token: ", "vault token")
		if inputErr != nil {
			return nil, inputErr
		}
		auth.Token = token
	case "password":
		username, inputErr := promptRequiredInput(command, prompter, "Vault password auth username: ", "vault password auth username")
		if inputErr != nil {
			return nil, inputErr
		}
		password, inputErr := promptRequiredInput(command, prompter, "Vault password auth password: ", "vault password auth password")
		if inputErr != nil {
			return nil, inputErr
		}
		mount, inputErr := promptOptionalInput(command, prompter, "Vault password auth mount (optional): ")
		if inputErr != nil {
			return nil, inputErr
		}
		auth.Password = &configdomain.VaultUserPasswordAuth{
			Username: username,
			Password: password,
			Mount:    mount,
		}
	case "approle":
		roleID, inputErr := promptRequiredInput(command, prompter, "Vault approle role-id: ", "vault approle role-id")
		if inputErr != nil {
			return nil, inputErr
		}
		secretID, inputErr := promptRequiredInput(command, prompter, "Vault approle secret-id: ", "vault approle secret-id")
		if inputErr != nil {
			return nil, inputErr
		}
		mount, inputErr := promptOptionalInput(command, prompter, "Vault approle mount (optional): ")
		if inputErr != nil {
			return nil, inputErr
		}
		auth.AppRole = &configdomain.VaultAppRoleAuth{
			RoleID:   roleID,
			SecretID: secretID,
			Mount:    mount,
		}
	default:
		return nil, common.ValidationError("invalid vault auth method selected", nil)
	}

	return auth, nil
}

func promptTLS(command *cobra.Command, prompter configPrompter) (*configdomain.TLS, error) {
	caCertFile, err := promptOptionalInput(command, prompter, "TLS ca-cert-file (optional): ")
	if err != nil {
		return nil, err
	}
	clientCertFile, err := promptOptionalInput(command, prompter, "TLS client-cert-file (optional): ")
	if err != nil {
		return nil, err
	}
	clientKeyFile, err := promptOptionalInput(command, prompter, "TLS client-key-file (optional): ")
	if err != nil {
		return nil, err
	}
	insecureSkipVerify, err := prompter.Confirm(command, "TLS insecure-skip-verify?", false)
	if err != nil {
		return nil, err
	}

	if caCertFile == "" && clientCertFile == "" && clientKeyFile == "" && !insecureSkipVerify {
		return nil, nil
	}

	return &configdomain.TLS{
		CACertFile:         caCertFile,
		ClientCertFile:     clientCertFile,
		ClientKeyFile:      clientKeyFile,
		InsecureSkipVerify: insecureSkipVerify,
	}, nil
}

func promptStringMap(
	command *cobra.Command,
	prompter configPrompter,
	label string,
) (map[string]string, error) {
	values := map[string]string{}
	for {
		key, err := promptOptionalInput(
			command,
			prompter,
			fmt.Sprintf("%s key (leave blank to finish): ", label),
		)
		if err != nil {
			return nil, err
		}
		if key == "" {
			break
		}
		value, err := promptRequiredInput(
			command,
			prompter,
			fmt.Sprintf("%s value: ", label),
			fmt.Sprintf("%s value", strings.ToLower(label)),
		)
		if err != nil {
			return nil, err
		}
		values[key] = value
	}

	if len(values) == 0 {
		return nil, nil
	}

	return values, nil
}

func promptRequiredInput(
	command *cobra.Command,
	prompter configPrompter,
	prompt string,
	field string,
) (string, error) {
	value, err := prompter.Input(command, prompt, true)
	if err != nil {
		return "", err
	}
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return "", common.ValidationError(fmt.Sprintf("%s is required", field), nil)
	}
	return trimmed, nil
}

func promptOptionalInput(command *cobra.Command, prompter configPrompter, prompt string) (string, error) {
	value, err := prompter.Input(command, prompt, false)
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func promptOptionalInt(
	command *cobra.Command,
	prompter configPrompter,
	prompt string,
	field string,
) (int, bool, error) {
	value, err := promptOptionalInput(command, prompter, prompt)
	if err != nil {
		return 0, false, err
	}
	if value == "" {
		return 0, false, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, false, common.ValidationError(fmt.Sprintf("invalid integer value for %s", field), err)
	}
	return parsed, true, nil
}

func selectContextForAction(
	command *cobra.Command,
	contexts configdomain.ContextService,
	prompter configPrompter,
	actionLabel string,
) (string, error) {
	items, err := contexts.List(command.Context())
	if err != nil {
		return "", err
	}
	if len(items) == 0 {
		return "", common.ValidationError("no contexts available", nil)
	}
	if !prompter.IsInteractive(command) {
		return "", common.ValidationError(fmt.Sprintf("context name is required: declarest config %s <name>", actionLabel), nil)
	}

	options := make([]string, 0, len(items))
	for _, item := range items {
		options = append(options, item.Name)
	}
	return prompter.Select(command, "Choose context", options)
}
