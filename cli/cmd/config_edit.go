package cmd

import (
	"bytes"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"reflect"
	"strings"

	ctx "declarest/internal/context"
	"declarest/internal/managedserver"
	"declarest/internal/repository"
	"declarest/internal/secrets"

	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func newConfigEditCommand(manager *ctx.DefaultContextManager) *cobra.Command {
	var (
		name   string
		editor string
	)

	cmd := &cobra.Command{
		Use:   "edit <name>",
		Short: "Edit a context configuration using your editor with defaults prefilled",
		RunE: func(cmd *cobra.Command, args []string) error {
			if manager == nil {
				return errors.New("context manager is not configured")
			}

			var err error
			name, err = resolveSingleArg(cmd, name, args, "name")
			if err != nil {
				return err
			}
			if err := validateContextName(name); err != nil {
				return usageError(cmd, err.Error())
			}

			existing, exists, err := manager.GetContextConfig(name)
			if err != nil {
				return err
			}

			initialConfig := existing
			if !exists {
				if info, statErr := os.Stat(name); statErr == nil && !info.IsDir() {
					cfg, readErr := readContextConfigFile(name)
					if readErr != nil {
						return readErr
					}
					initialConfig = cfg
				} else if statErr != nil && !errors.Is(statErr, fs.ErrNotExist) {
					return fmt.Errorf("stat %q: %w", name, statErr)
				}
			}

			payload := defaultConfigEditPayload()
			applyContextConfigToPayload(&payload, initialConfig)

			payloadData, err := marshalConfigEditPayloadWithComments(payload)
			if err != nil {
				return err
			}

			tempFile, err := os.CreateTemp("", "declarest-config-*.yaml")
			if err != nil {
				return fmt.Errorf("create temp config file: %w", err)
			}
			tempPath := tempFile.Name()
			if err := tempFile.Close(); err != nil {
				return fmt.Errorf("close temp config file: %w", err)
			}
			defer os.Remove(tempPath)

			if err := os.WriteFile(tempPath, payloadData, 0o644); err != nil {
				return fmt.Errorf("write temp config file: %w", err)
			}

			editorArgs, err := resolveEditorCommand(editor)
			if err != nil {
				return err
			}
			editorArgs = append(editorArgs, tempPath)
			if err := runEditor(editorArgs); err != nil {
				return err
			}

			editedData, err := os.ReadFile(tempPath)
			if err != nil {
				return fmt.Errorf("read edited config: %w", err)
			}
			editedConfig, err := decodeContextEditFile(editedData)
			if err != nil {
				return err
			}
			editedConfig = normalizeContextConfig(editedConfig)

			if err := ctx.ValidateContextConfig(editedConfig); err != nil {
				return err
			}

			if exists {
				normalizedExisting := normalizeContextConfig(existing)
				if reflect.DeepEqual(normalizedExisting, editedConfig) {
					successf(cmd, "no updates detected for %s", name)
					return nil
				}
				if err := manager.ReplaceContextConfig(name, editedConfig); err != nil {
					return err
				}
				successf(cmd, "updated config for %s", name)
				return nil
			}
			if err := manager.AddContextConfig(name, editedConfig); err != nil {
				return err
			}
			successf(cmd, "added config for %s", name)
			return nil
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "Context identifier to edit")
	cmd.Flags().StringVar(&editor, "editor", "", "Editor command (defaults to vi)")

	registerContextNameArgumentCompletion(cmd, manager, true, 0)
	registerContextNameFlagCompletion(cmd, manager, "name")

	return cmd
}

type configEditPayload struct {
	Repository    repositoryEditConfig    `yaml:"repository"`
	ManagedServer managedServerEditConfig `yaml:"managed_server"`
	SecretStore   secretStoreEditConfig   `yaml:"secret_store"`
}

type repositoryEditConfig struct {
	ResourceFormat string                         `yaml:"resource_format"`
	Git            gitRepositoryEditConfig        `yaml:"git"`
	Filesystem     filesystemRepositoryEditConfig `yaml:"filesystem"`
}

type gitRepositoryEditConfig struct {
	Local  gitLocalEditConfig  `yaml:"local"`
	Remote gitRemoteEditConfig `yaml:"remote"`
}

type gitLocalEditConfig struct {
	BaseDir string `yaml:"base_dir"`
}

type gitRemoteEditConfig struct {
	URL      string                  `yaml:"url"`
	Branch   string                  `yaml:"branch"`
	Provider string                  `yaml:"provider"`
	AutoSync bool                    `yaml:"auto_sync"`
	Auth     gitRemoteAuthEditConfig `yaml:"auth"`
	TLS      gitRemoteTLSEditConfig  `yaml:"tls"`
}

type gitRemoteAuthEditConfig struct {
	BasicAuth gitRemoteBasicAuthEditConfig `yaml:"basic_auth"`
	SSH       gitRemoteSSHEditConfig       `yaml:"ssh"`
	AccessKey gitRemoteAccessKeyEditConfig `yaml:"access_key"`
}

type gitRemoteBasicAuthEditConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type gitRemoteSSHEditConfig struct {
	User                  string `yaml:"user"`
	PrivateKeyFile        string `yaml:"private_key_file"`
	Passphrase            string `yaml:"passphrase"`
	KnownHostsFile        string `yaml:"known_hosts_file"`
	InsecureIgnoreHostKey bool   `yaml:"insecure_ignore_host_key"`
}

type gitRemoteAccessKeyEditConfig struct {
	Token string `yaml:"token"`
}

type gitRemoteTLSEditConfig struct {
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`
}

type filesystemRepositoryEditConfig struct {
	BaseDir string `yaml:"base_dir"`
}

type managedServerEditConfig struct {
	HTTP httpManagedServerEditConfig `yaml:"http"`
}

type httpManagedServerEditConfig struct {
	BaseURL        string             `yaml:"base_url"`
	OpenAPI        string             `yaml:"openapi"`
	DefaultHeaders map[string]string  `yaml:"default_headers"`
	Auth           httpAuthEditConfig `yaml:"auth"`
	TLS            httpTLSEditConfig  `yaml:"tls"`
}

type httpAuthEditConfig struct {
	OAuth2       httpOAuth2EditConfig       `yaml:"oauth2"`
	CustomHeader httpCustomHeaderEditConfig `yaml:"custom_header"`
	BasicAuth    httpBasicAuthEditConfig    `yaml:"basic_auth"`
	BearerToken  httpBearerTokenEditConfig  `yaml:"bearer_token"`
}

type httpOAuth2EditConfig struct {
	TokenURL     string `yaml:"token_url"`
	GrantType    string `yaml:"grant_type"`
	ClientID     string `yaml:"client_id"`
	ClientSecret string `yaml:"client_secret"`
	Username     string `yaml:"username"`
	Password     string `yaml:"password"`
	Scope        string `yaml:"scope"`
	Audience     string `yaml:"audience"`
}

type httpBasicAuthEditConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

type httpBearerTokenEditConfig struct {
	Token string `yaml:"token"`
}

type httpCustomHeaderEditConfig struct {
	Header string `yaml:"header"`
	Token  string `yaml:"token"`
}

type httpTLSEditConfig struct {
	InsecureSkipVerify bool `yaml:"insecure_skip_verify"`
}

type secretStoreEditConfig struct {
	File  fileSecretsEditConfig  `yaml:"file"`
	Vault vaultSecretsEditConfig `yaml:"vault"`
}

type fileSecretsEditConfig struct {
	Path           string                   `yaml:"path"`
	Key            string                   `yaml:"key"`
	KeyFile        string                   `yaml:"key_file"`
	Passphrase     string                   `yaml:"passphrase"`
	PassphraseFile string                   `yaml:"passphrase_file"`
	KDF            fileSecretsKDFEditConfig `yaml:"kdf"`
}

type fileSecretsKDFEditConfig struct {
	Time    uint32 `yaml:"time"`
	Memory  uint32 `yaml:"memory"`
	Threads uint8  `yaml:"threads"`
}

type vaultSecretsEditConfig struct {
	Address    string              `yaml:"address"`
	Mount      string              `yaml:"mount"`
	PathPrefix string              `yaml:"path_prefix"`
	KVVersion  int                 `yaml:"kv_version"`
	Auth       vaultAuthEditConfig `yaml:"auth"`
	TLS        vaultTLSEditConfig  `yaml:"tls"`
}

type vaultAuthEditConfig struct {
	Token    string                  `yaml:"token"`
	Password vaultPasswordEditConfig `yaml:"password"`
	AppRole  vaultAppRoleEditConfig  `yaml:"approle"`
}

type vaultPasswordEditConfig struct {
	Username string `yaml:"username"`
	Password string `yaml:"password"`
	Mount    string `yaml:"mount"`
}

type vaultAppRoleEditConfig struct {
	RoleID   string `yaml:"role_id"`
	SecretID string `yaml:"secret_id"`
	Mount    string `yaml:"mount"`
}

type vaultTLSEditConfig struct {
	CACertFile         string `yaml:"ca_cert_file"`
	ClientCertFile     string `yaml:"client_cert_file"`
	ClientKeyFile      string `yaml:"client_key_file"`
	InsecureSkipVerify bool   `yaml:"insecure_skip_verify"`
}

func defaultConfigEditPayload() configEditPayload {
	return configEditPayload{
		Repository: repositoryEditConfig{
			ResourceFormat: string(repository.ResourceFormatJSON),
			Git: gitRepositoryEditConfig{
				Local: gitLocalEditConfig{},
				Remote: gitRemoteEditConfig{
					AutoSync: true,
					Auth:     gitRemoteAuthEditConfig{},
					TLS:      gitRemoteTLSEditConfig{},
				},
			},
			Filesystem: filesystemRepositoryEditConfig{},
		},
		ManagedServer: managedServerEditConfig{
			HTTP: httpManagedServerEditConfig{
				DefaultHeaders: map[string]string{},
				Auth:           httpAuthEditConfig{},
				TLS:            httpTLSEditConfig{},
			},
		},
		SecretStore: secretStoreEditConfig{
			File:  fileSecretsEditConfig{KDF: fileSecretsKDFEditConfig{}},
			Vault: vaultSecretsEditConfig{Auth: vaultAuthEditConfig{}, TLS: vaultTLSEditConfig{}},
		},
	}
}

func applyContextConfigToPayload(payload *configEditPayload, cfg *ctx.ContextConfig) {
	if payload == nil || cfg == nil {
		return
	}

	if repo := cfg.Repository; repo != nil {
		if strings.TrimSpace(repo.ResourceFormat) != "" {
			payload.Repository.ResourceFormat = repo.ResourceFormat
		}
		if repo.Git != nil {
			if repo.Git.Local != nil {
				payload.Repository.Git.Local.BaseDir = repo.Git.Local.BaseDir
			}
			if repo.Git.Remote != nil {
				remote := repo.Git.Remote
				payload.Repository.Git.Remote.URL = remote.URL
				payload.Repository.Git.Remote.Branch = remote.Branch
				payload.Repository.Git.Remote.Provider = remote.Provider
				if remote.AutoSync != nil {
					payload.Repository.Git.Remote.AutoSync = *remote.AutoSync
				}
				if remote.Auth != nil {
					if remote.Auth.BasicAuth != nil {
						payload.Repository.Git.Remote.Auth.BasicAuth.Username = remote.Auth.BasicAuth.Username
						payload.Repository.Git.Remote.Auth.BasicAuth.Password = remote.Auth.BasicAuth.Password
					}
					if remote.Auth.SSH != nil {
						payload.Repository.Git.Remote.Auth.SSH.User = remote.Auth.SSH.User
						payload.Repository.Git.Remote.Auth.SSH.PrivateKeyFile = remote.Auth.SSH.PrivateKeyFile
						payload.Repository.Git.Remote.Auth.SSH.Passphrase = remote.Auth.SSH.Passphrase
						payload.Repository.Git.Remote.Auth.SSH.KnownHostsFile = remote.Auth.SSH.KnownHostsFile
						payload.Repository.Git.Remote.Auth.SSH.InsecureIgnoreHostKey = remote.Auth.SSH.InsecureIgnoreHostKey
					}
					if remote.Auth.AccessKey != nil {
						payload.Repository.Git.Remote.Auth.AccessKey.Token = remote.Auth.AccessKey.Token
					}
				}
				if remote.TLS != nil {
					payload.Repository.Git.Remote.TLS.InsecureSkipVerify = remote.TLS.InsecureSkipVerify
				}
			}
		}
		if repo.Filesystem != nil {
			payload.Repository.Filesystem.BaseDir = repo.Filesystem.BaseDir
		}
	}

	if server := cfg.ManagedServer; server != nil && server.HTTP != nil {
		httpCfg := server.HTTP
		payload.ManagedServer.HTTP.BaseURL = httpCfg.BaseURL
		payload.ManagedServer.HTTP.OpenAPI = httpCfg.OpenAPI
		if len(httpCfg.DefaultHeaders) > 0 {
			payload.ManagedServer.HTTP.DefaultHeaders = copyStringMap(httpCfg.DefaultHeaders)
		}
		if httpCfg.Auth != nil {
			if httpCfg.Auth.OAuth2 != nil {
				payload.ManagedServer.HTTP.Auth.OAuth2.TokenURL = httpCfg.Auth.OAuth2.TokenURL
				payload.ManagedServer.HTTP.Auth.OAuth2.GrantType = httpCfg.Auth.OAuth2.GrantType
				payload.ManagedServer.HTTP.Auth.OAuth2.ClientID = httpCfg.Auth.OAuth2.ClientID
				payload.ManagedServer.HTTP.Auth.OAuth2.ClientSecret = httpCfg.Auth.OAuth2.ClientSecret
				payload.ManagedServer.HTTP.Auth.OAuth2.Username = httpCfg.Auth.OAuth2.Username
				payload.ManagedServer.HTTP.Auth.OAuth2.Password = httpCfg.Auth.OAuth2.Password
				payload.ManagedServer.HTTP.Auth.OAuth2.Scope = httpCfg.Auth.OAuth2.Scope
				payload.ManagedServer.HTTP.Auth.OAuth2.Audience = httpCfg.Auth.OAuth2.Audience
			}
			if httpCfg.Auth.BasicAuth != nil {
				payload.ManagedServer.HTTP.Auth.BasicAuth.Username = httpCfg.Auth.BasicAuth.Username
				payload.ManagedServer.HTTP.Auth.BasicAuth.Password = httpCfg.Auth.BasicAuth.Password
			}
			if httpCfg.Auth.BearerToken != nil {
				payload.ManagedServer.HTTP.Auth.BearerToken.Token = httpCfg.Auth.BearerToken.Token
			}
			if httpCfg.Auth.CustomHeader != nil {
				payload.ManagedServer.HTTP.Auth.CustomHeader.Header = httpCfg.Auth.CustomHeader.Header
				payload.ManagedServer.HTTP.Auth.CustomHeader.Token = httpCfg.Auth.CustomHeader.Token
			}
		}
		if httpCfg.TLS != nil {
			payload.ManagedServer.HTTP.TLS.InsecureSkipVerify = httpCfg.TLS.InsecureSkipVerify
		}
	}

	if secretsCfg := cfg.SecretManager; secretsCfg != nil {
		if secretsCfg.File != nil {
			payload.SecretStore.File.Path = secretsCfg.File.Path
			payload.SecretStore.File.Key = secretsCfg.File.Key
			payload.SecretStore.File.KeyFile = secretsCfg.File.KeyFile
			payload.SecretStore.File.Passphrase = secretsCfg.File.Passphrase
			payload.SecretStore.File.PassphraseFile = secretsCfg.File.PassphraseFile
			if secretsCfg.File.KDF != nil {
				payload.SecretStore.File.KDF.Time = secretsCfg.File.KDF.Time
				payload.SecretStore.File.KDF.Memory = secretsCfg.File.KDF.Memory
				payload.SecretStore.File.KDF.Threads = secretsCfg.File.KDF.Threads
			}
		}
		if secretsCfg.Vault != nil {
			payload.SecretStore.Vault.Address = secretsCfg.Vault.Address
			payload.SecretStore.Vault.Mount = secretsCfg.Vault.Mount
			payload.SecretStore.Vault.PathPrefix = secretsCfg.Vault.PathPrefix
			payload.SecretStore.Vault.KVVersion = secretsCfg.Vault.KVVersion
			if secretsCfg.Vault.Auth != nil {
				payload.SecretStore.Vault.Auth.Token = secretsCfg.Vault.Auth.Token
				if secretsCfg.Vault.Auth.Password != nil {
					payload.SecretStore.Vault.Auth.Password.Username = secretsCfg.Vault.Auth.Password.Username
					payload.SecretStore.Vault.Auth.Password.Password = secretsCfg.Vault.Auth.Password.Password
					payload.SecretStore.Vault.Auth.Password.Mount = secretsCfg.Vault.Auth.Password.Mount
				}
				if secretsCfg.Vault.Auth.AppRole != nil {
					payload.SecretStore.Vault.Auth.AppRole.RoleID = secretsCfg.Vault.Auth.AppRole.RoleID
					payload.SecretStore.Vault.Auth.AppRole.SecretID = secretsCfg.Vault.Auth.AppRole.SecretID
					payload.SecretStore.Vault.Auth.AppRole.Mount = secretsCfg.Vault.Auth.AppRole.Mount
				}
			}
			if secretsCfg.Vault.TLS != nil {
				payload.SecretStore.Vault.TLS.CACertFile = secretsCfg.Vault.TLS.CACertFile
				payload.SecretStore.Vault.TLS.ClientCertFile = secretsCfg.Vault.TLS.ClientCertFile
				payload.SecretStore.Vault.TLS.ClientKeyFile = secretsCfg.Vault.TLS.ClientKeyFile
				payload.SecretStore.Vault.TLS.InsecureSkipVerify = secretsCfg.Vault.TLS.InsecureSkipVerify
			}
		}
	}
}

func copyStringMap(input map[string]string) map[string]string {
	if input == nil {
		return nil
	}
	out := make(map[string]string, len(input))
	for key, value := range input {
		out[key] = value
	}
	return out
}

func decodeContextEditFile(data []byte) (*ctx.ContextConfig, error) {
	if len(bytes.TrimSpace(data)) == 0 {
		return &ctx.ContextConfig{}, nil
	}
	var cfg ctx.ContextConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse context config: %w", err)
	}
	return &cfg, nil
}

func readContextConfigFile(path string) (*ctx.ContextConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read context config %q: %w", path, err)
	}
	var cfg ctx.ContextConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse context config %q: %w", path, err)
	}
	return &cfg, nil
}

func normalizeContextConfig(cfg *ctx.ContextConfig) *ctx.ContextConfig {
	if cfg == nil {
		return &ctx.ContextConfig{}
	}
	normalized := *cfg
	normalized.Repository = normalizeRepositoryConfig(cfg.Repository)
	normalized.ManagedServer = normalizeManagedServerConfig(cfg.ManagedServer)
	normalized.SecretManager = normalizeSecretsConfig(cfg.SecretManager)
	return &normalized
}

func normalizeRepositoryConfig(cfg *ctx.RepositoryConfig) *ctx.RepositoryConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.ResourceFormat = normalizeResourceFormatValue(cfg.ResourceFormat)
	normalized.Git = normalizeGitRepositoryConfig(cfg.Git)
	normalized.Filesystem = normalizeFilesystemRepositoryConfig(cfg.Filesystem)
	if normalized.ResourceFormat == "" && normalized.Git == nil && normalized.Filesystem == nil {
		return nil
	}
	return &normalized
}

func normalizeResourceFormatValue(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return ""
	}
	parsed, err := repository.ParseResourceFormat(trimmed)
	if err != nil {
		return trimmed
	}
	if parsed == repository.ResourceFormatJSON {
		return ""
	}
	return string(parsed)
}

func normalizeGitRepositoryConfig(cfg *repository.GitResourceRepositoryConfig) *repository.GitResourceRepositoryConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Local = normalizeGitLocalConfig(cfg.Local)
	normalized.Remote = normalizeGitRemoteConfig(cfg.Remote)
	if normalized.Local == nil && normalized.Remote == nil {
		return nil
	}
	return &normalized
}

func normalizeGitLocalConfig(cfg *repository.GitResourceRepositoryLocalConfig) *repository.GitResourceRepositoryLocalConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.BaseDir = strings.TrimSpace(cfg.BaseDir)
	if normalized.BaseDir == "" {
		return nil
	}
	return &normalized
}

func normalizeGitRemoteConfig(cfg *repository.GitResourceRepositoryRemoteConfig) *repository.GitResourceRepositoryRemoteConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.URL = strings.TrimSpace(cfg.URL)
	normalized.Branch = strings.TrimSpace(cfg.Branch)
	normalized.Provider = strings.TrimSpace(cfg.Provider)
	if normalized.AutoSync != nil && *normalized.AutoSync {
		normalized.AutoSync = nil
	}
	normalized.Auth = normalizeGitRemoteAuthConfig(cfg.Auth)
	normalized.TLS = normalizeGitRemoteTLSConfig(cfg.TLS)
	if normalized.URL == "" && normalized.Branch == "" && normalized.Provider == "" &&
		normalized.AutoSync == nil && normalized.Auth == nil && normalized.TLS == nil {
		return nil
	}
	return &normalized
}

func normalizeGitRemoteAuthConfig(cfg *repository.GitResourceRepositoryRemoteAuthConfig) *repository.GitResourceRepositoryRemoteAuthConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.BasicAuth = normalizeGitRemoteBasicAuth(cfg.BasicAuth)
	normalized.SSH = normalizeGitRemoteSSH(cfg.SSH)
	normalized.AccessKey = normalizeGitRemoteAccessKey(cfg.AccessKey)
	if normalized.BasicAuth == nil && normalized.SSH == nil && normalized.AccessKey == nil {
		return nil
	}
	return &normalized
}

func normalizeGitRemoteBasicAuth(cfg *repository.GitResourceRepositoryBasicAuthConfig) *repository.GitResourceRepositoryBasicAuthConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Username = strings.TrimSpace(cfg.Username)
	normalized.Password = strings.TrimSpace(cfg.Password)
	if normalized.Username == "" && normalized.Password == "" {
		return nil
	}
	return &normalized
}

func normalizeGitRemoteSSH(cfg *repository.GitResourceRepositorySSHAuthConfig) *repository.GitResourceRepositorySSHAuthConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.User = strings.TrimSpace(cfg.User)
	normalized.PrivateKeyFile = strings.TrimSpace(cfg.PrivateKeyFile)
	normalized.Passphrase = strings.TrimSpace(cfg.Passphrase)
	normalized.KnownHostsFile = strings.TrimSpace(cfg.KnownHostsFile)
	if normalized.User == "" && normalized.PrivateKeyFile == "" && normalized.Passphrase == "" &&
		normalized.KnownHostsFile == "" && !normalized.InsecureIgnoreHostKey {
		return nil
	}
	return &normalized
}

func normalizeGitRemoteAccessKey(cfg *repository.GitResourceRepositoryAccessKeyConfig) *repository.GitResourceRepositoryAccessKeyConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Token = strings.TrimSpace(cfg.Token)
	if normalized.Token == "" {
		return nil
	}
	return &normalized
}

func normalizeGitRemoteTLSConfig(cfg *repository.GitResourceRepositoryRemoteTLSConfig) *repository.GitResourceRepositoryRemoteTLSConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	if !normalized.InsecureSkipVerify {
		return nil
	}
	return &normalized
}

func normalizeFilesystemRepositoryConfig(cfg *repository.FileSystemResourceRepositoryConfig) *repository.FileSystemResourceRepositoryConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.BaseDir = strings.TrimSpace(cfg.BaseDir)
	if normalized.BaseDir == "" {
		return nil
	}
	return &normalized
}

func normalizeManagedServerConfig(cfg *ctx.ManagedServerConfig) *ctx.ManagedServerConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.HTTP = normalizeHTTPServerConfig(cfg.HTTP)
	if normalized.HTTP == nil {
		return nil
	}
	return &normalized
}

func normalizeHTTPServerConfig(cfg *managedserver.HTTPResourceServerConfig) *managedserver.HTTPResourceServerConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.BaseURL = strings.TrimSpace(cfg.BaseURL)
	normalized.OpenAPI = strings.TrimSpace(cfg.OpenAPI)
	if len(cfg.DefaultHeaders) == 0 {
		normalized.DefaultHeaders = nil
	} else {
		normalized.DefaultHeaders = copyStringMap(cfg.DefaultHeaders)
	}
	normalized.Auth = normalizeHTTPAuthConfig(cfg.Auth)
	normalized.TLS = normalizeHTTPTLSConfig(cfg.TLS)
	if normalized.BaseURL == "" && normalized.OpenAPI == "" && len(normalized.DefaultHeaders) == 0 &&
		normalized.Auth == nil && normalized.TLS == nil {
		return nil
	}
	return &normalized
}

func normalizeHTTPAuthConfig(cfg *managedserver.HTTPResourceServerAuthConfig) *managedserver.HTTPResourceServerAuthConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.OAuth2 = normalizeHTTPOAuth2Config(cfg.OAuth2)
	normalized.BasicAuth = normalizeHTTPBasicAuthConfig(cfg.BasicAuth)
	normalized.BearerToken = normalizeHTTPBearerConfig(cfg.BearerToken)
	normalized.CustomHeader = normalizeHTTPCustomHeaderConfig(cfg.CustomHeader)
	if normalized.OAuth2 == nil && normalized.BasicAuth == nil &&
		normalized.BearerToken == nil && normalized.CustomHeader == nil {
		return nil
	}
	return &normalized
}

func normalizeHTTPOAuth2Config(cfg *managedserver.HTTPResourceServerOAuth2Config) *managedserver.HTTPResourceServerOAuth2Config {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.TokenURL = strings.TrimSpace(cfg.TokenURL)
	normalized.GrantType = strings.TrimSpace(cfg.GrantType)
	normalized.ClientID = strings.TrimSpace(cfg.ClientID)
	normalized.ClientSecret = strings.TrimSpace(cfg.ClientSecret)
	normalized.Username = strings.TrimSpace(cfg.Username)
	normalized.Password = strings.TrimSpace(cfg.Password)
	normalized.Scope = strings.TrimSpace(cfg.Scope)
	normalized.Audience = strings.TrimSpace(cfg.Audience)
	if normalized.TokenURL == "" && normalized.GrantType == "" && normalized.ClientID == "" &&
		normalized.ClientSecret == "" && normalized.Username == "" && normalized.Password == "" &&
		normalized.Scope == "" && normalized.Audience == "" {
		return nil
	}
	return &normalized
}

func normalizeHTTPBasicAuthConfig(cfg *managedserver.HTTPResourceServerBasicAuthConfig) *managedserver.HTTPResourceServerBasicAuthConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Username = strings.TrimSpace(cfg.Username)
	normalized.Password = strings.TrimSpace(cfg.Password)
	if normalized.Username == "" && normalized.Password == "" {
		return nil
	}
	return &normalized
}

func normalizeHTTPBearerConfig(cfg *managedserver.HTTPResourceServerBearerTokenConfig) *managedserver.HTTPResourceServerBearerTokenConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Token = strings.TrimSpace(cfg.Token)
	if normalized.Token == "" {
		return nil
	}
	return &normalized
}

func normalizeHTTPCustomHeaderConfig(cfg *managedserver.HTTPResourceServerCustomHeaderConfig) *managedserver.HTTPResourceServerCustomHeaderConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Header = strings.TrimSpace(cfg.Header)
	normalized.Token = strings.TrimSpace(cfg.Token)
	if normalized.Header == "" && normalized.Token == "" {
		return nil
	}
	return &normalized
}

func normalizeHTTPTLSConfig(cfg *managedserver.HTTPResourceServerTLSConfig) *managedserver.HTTPResourceServerTLSConfig {
	if cfg == nil {
		return nil
	}
	if !cfg.InsecureSkipVerify {
		return nil
	}
	normalized := *cfg
	return &normalized
}

func normalizeSecretsConfig(cfg *secrets.SecretsManagerConfig) *secrets.SecretsManagerConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.File = normalizeFileSecretsConfig(cfg.File)
	normalized.Vault = normalizeVaultSecretsConfig(cfg.Vault)
	if normalized.File == nil && normalized.Vault == nil {
		return nil
	}
	return &normalized
}

func normalizeFileSecretsConfig(cfg *secrets.FileSecretsManagerConfig) *secrets.FileSecretsManagerConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Path = strings.TrimSpace(cfg.Path)
	normalized.Key = strings.TrimSpace(cfg.Key)
	normalized.KeyFile = strings.TrimSpace(cfg.KeyFile)
	normalized.Passphrase = strings.TrimSpace(cfg.Passphrase)
	normalized.PassphraseFile = strings.TrimSpace(cfg.PassphraseFile)
	normalized.KDF = normalizeFileKDFConfig(cfg.KDF)
	if normalized.Path == "" && normalized.Key == "" && normalized.KeyFile == "" &&
		normalized.Passphrase == "" && normalized.PassphraseFile == "" && normalized.KDF == nil {
		return nil
	}
	return &normalized
}

func normalizeFileKDFConfig(cfg *secrets.FileSecretsManagerKDFConfig) *secrets.FileSecretsManagerKDFConfig {
	if cfg == nil {
		return nil
	}
	if cfg.Time == 0 && cfg.Memory == 0 && cfg.Threads == 0 {
		return nil
	}
	normalized := *cfg
	return &normalized
}

func normalizeVaultSecretsConfig(cfg *secrets.VaultSecretsManagerConfig) *secrets.VaultSecretsManagerConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Address = strings.TrimSpace(cfg.Address)
	normalized.Mount = strings.TrimSpace(cfg.Mount)
	normalized.PathPrefix = strings.TrimSpace(cfg.PathPrefix)
	normalized.Auth = normalizeVaultAuthConfig(cfg.Auth)
	normalized.TLS = normalizeVaultTLSConfig(cfg.TLS)
	if normalized.Address == "" && normalized.Mount == "" && normalized.PathPrefix == "" &&
		normalized.KVVersion == 0 && normalized.Auth == nil && normalized.TLS == nil {
		return nil
	}
	return &normalized
}

func normalizeVaultAuthConfig(cfg *secrets.VaultSecretsManagerAuthConfig) *secrets.VaultSecretsManagerAuthConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Token = strings.TrimSpace(cfg.Token)
	normalized.Password = normalizeVaultPasswordConfig(cfg.Password)
	normalized.AppRole = normalizeVaultAppRoleConfig(cfg.AppRole)
	if normalized.Token == "" && normalized.Password == nil && normalized.AppRole == nil {
		return nil
	}
	return &normalized
}

func normalizeVaultPasswordConfig(cfg *secrets.VaultSecretsManagerPasswordAuthConfig) *secrets.VaultSecretsManagerPasswordAuthConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.Username = strings.TrimSpace(cfg.Username)
	normalized.Password = strings.TrimSpace(cfg.Password)
	normalized.Mount = strings.TrimSpace(cfg.Mount)
	if normalized.Username == "" && normalized.Password == "" && normalized.Mount == "" {
		return nil
	}
	return &normalized
}

func normalizeVaultAppRoleConfig(cfg *secrets.VaultSecretsManagerAppRoleAuthConfig) *secrets.VaultSecretsManagerAppRoleAuthConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.RoleID = strings.TrimSpace(cfg.RoleID)
	normalized.SecretID = strings.TrimSpace(cfg.SecretID)
	normalized.Mount = strings.TrimSpace(cfg.Mount)
	if normalized.RoleID == "" && normalized.SecretID == "" && normalized.Mount == "" {
		return nil
	}
	return &normalized
}

func normalizeVaultTLSConfig(cfg *secrets.VaultSecretsManagerTLSConfig) *secrets.VaultSecretsManagerTLSConfig {
	if cfg == nil {
		return nil
	}
	normalized := *cfg
	normalized.CACertFile = strings.TrimSpace(cfg.CACertFile)
	normalized.ClientCertFile = strings.TrimSpace(cfg.ClientCertFile)
	normalized.ClientKeyFile = strings.TrimSpace(cfg.ClientKeyFile)
	if normalized.CACertFile == "" && normalized.ClientCertFile == "" &&
		normalized.ClientKeyFile == "" && !normalized.InsecureSkipVerify {
		return nil
	}
	return &normalized
}

const configEditTemplateComment = "=====\nThis template shows all available options and fills currently unset attributes with defaults. \nOnce you save this file, all attributes that still match the defaults together with comments will be removed from final file.\n=====\n"

func marshalConfigEditPayloadWithComments(payload configEditPayload) ([]byte, error) {
	data, err := yaml.Marshal(payload)
	if err != nil {
		return nil, err
	}

	var doc yaml.Node
	if err := yaml.Unmarshal(data, &doc); err != nil {
		return nil, err
	}
	if len(doc.Content) == 0 {
		return nil, fmt.Errorf("config template is empty")
	}

	root := doc.Content[0]
	root.HeadComment = configEditTemplateComment
	annotateYAMLNode(root, "", configEditComments)

	var buf bytes.Buffer
	encoder := yaml.NewEncoder(&buf)
	encoder.SetIndent(2)
	if err := encoder.Encode(root); err != nil {
		_ = encoder.Close()
		return nil, err
	}
	if err := encoder.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func annotateYAMLNode(node *yaml.Node, path string, comments map[string]string) {
	if node == nil || node.Kind != yaml.MappingNode {
		return
	}
	for idx := 0; idx < len(node.Content)-1; idx += 2 {
		keyNode := node.Content[idx]
		valueNode := node.Content[idx+1]
		childPath := joinYAMLPath(path, keyNode.Value)
		if comment, ok := comments[childPath]; ok {
			keyNode.HeadComment = comment
		}
		annotateYAMLNode(valueNode, childPath, comments)
	}
}

func joinYAMLPath(parent, child string) string {
	if parent == "" {
		return child
	}
	return parent + "." + child
}

var configEditComments = map[string]string{
	"repository":                                              "Repository settings determine where and how resources are persisted.",
	"repository.resource_format":                              "Preferred format for saved resources (json or yaml); omit for json.",
	"repository.git":                                          "Git-backed repository configuration.",
	"repository.git.local":                                    "Local clone configuration.",
	"repository.git.local.base_dir":                           "Local directory used to store fetched resources.",
	"repository.git.remote":                                   "Remote Git synchronization settings.",
	"repository.git.remote.url":                               "Remote Git URL.",
	"repository.git.remote.branch":                            "Branch tracked for synchronization operations.",
	"repository.git.remote.provider":                          "Optional Git hosting provider hint.",
	"repository.git.remote.auto_sync":                         "Auto-sync the local repo when operations run; true keeps it up to date.",
	"repository.git.remote.auth":                              "Authentication method for Git pushes/pulls.",
	"repository.git.remote.auth.basic_auth":                   "Use basic auth for Git operations.",
	"repository.git.remote.auth.basic_auth.username":          "Username for Git basic authentication.",
	"repository.git.remote.auth.basic_auth.password":          "Password for Git basic authentication.",
	"repository.git.remote.auth.ssh":                          "Use SSH keypair authentication.",
	"repository.git.remote.auth.ssh.user":                     "SSH username.",
	"repository.git.remote.auth.ssh.private_key_file":         "Path to an SSH private key.",
	"repository.git.remote.auth.ssh.passphrase":               "Passphrase for the SSH private key.",
	"repository.git.remote.auth.ssh.known_hosts_file":         "Known hosts file used for SSH validation.",
	"repository.git.remote.auth.ssh.insecure_ignore_host_key": "Skip known-host checks for SSH (use cautiously).",
	"repository.git.remote.auth.access_key":                   "Use a provider access token for Git authentication.",
	"repository.git.remote.auth.access_key.token":             "Personal access token for the Git provider.",
	"repository.git.remote.tls":                               "TLS overrides for the Git remote.",
	"repository.git.remote.tls.insecure_skip_verify":          "Skip TLS verification for the Git remote certificate.",
	"repository.filesystem":                                   "Filesystem-backed resource repository configuration.",
	"repository.filesystem.base_dir":                          "Directory storing resources when using the filesystem backend.",
	"managed_server":                                          "Managed server options configure how DeclaREST calls the API.",
	"managed_server.http":                                     "HTTP settings for managed server requests.",
	"managed_server.http.base_url":                            "Base API URL used for all resource operations.",
	"managed_server.http.openapi":                             "Path or URL to the OpenAPI spec used for metadata inference.",
	"managed_server.http.default_headers":                     "Headers sent on every request (values may reference templates).",
	"managed_server.http.auth":                                "Select one authentication method for the managed server.",
	"managed_server.http.auth.oauth2":                         "OAuth2 credentials for server access.",
	"managed_server.http.auth.oauth2.token_url":               "Token endpoint URL for OAuth2.",
	"managed_server.http.auth.oauth2.grant_type":              "OAuth2 grant type (e.g., client_credentials).",
	"managed_server.http.auth.oauth2.client_id":               "OAuth2 client ID.",
	"managed_server.http.auth.oauth2.client_secret":           "OAuth2 client secret.",
	"managed_server.http.auth.oauth2.username":                "Username for password grant.",
	"managed_server.http.auth.oauth2.password":                "Password for password grant.",
	"managed_server.http.auth.oauth2.scope":                   "Space-separated OAuth2 scopes.",
	"managed_server.http.auth.oauth2.audience":                "Audience claim for OAuth2.",
	"managed_server.http.auth.basic_auth":                     "Basic auth credentials for the server.",
	"managed_server.http.auth.basic_auth.username":            "Username for basic authentication.",
	"managed_server.http.auth.basic_auth.password":            "Password for basic authentication.",
	"managed_server.http.auth.bearer_token":                   "Bearer token authentication.",
	"managed_server.http.auth.bearer_token.token":             "Token used in the Authorization header.",
	"managed_server.http.auth.custom_header":                  "Custom header authentication.",
	"managed_server.http.auth.custom_header.header":           "Header name to send for the custom auth.",
	"managed_server.http.auth.custom_header.token":            "Header value for the custom auth method.",
	"managed_server.http.tls":                                 "TLS overrides for the HTTP server.",
	"managed_server.http.tls.insecure_skip_verify":            "Skip TLS certificate verification for the managed server.",
	"secret_store":                                            "Secret store settings for sensitive values.",
	"secret_store.file":                                       "File-based secret store configuration.",
	"secret_store.file.path":                                  "Path to the encrypted secrets file.",
	"secret_store.file.key":                                   "Base64-encoded AES key used to encrypt the file.",
	"secret_store.file.key_file":                              "File path containing the AES key.",
	"secret_store.file.passphrase":                            "Passphrase for the encryption key.",
	"secret_store.file.passphrase_file":                       "Path to a file with the passphrase.",
	"secret_store.file.kdf":                                   "Key derivation parameters for the file store.",
	"secret_store.file.kdf.time":                              "Time parameter controlling KDF iterations.",
	"secret_store.file.kdf.memory":                            "Memory parameter for the KDF.",
	"secret_store.file.kdf.threads":                           "Thread count used by the KDF.",
	"secret_store.vault":                                      "HashiCorp Vault configuration.",
	"secret_store.vault.address":                              "Vault server address (including protocol).",
	"secret_store.vault.mount":                                "Vault KV mount path.",
	"secret_store.vault.path_prefix":                          "Prefix inside the mount where secrets reside.",
	"secret_store.vault.kv_version":                           "KV engine version (1 or 2).",
	"secret_store.vault.auth":                                 "Vault authentication method.",
	"secret_store.vault.auth.token":                           "Vault token for authentication.",
	"secret_store.vault.auth.password":                        "Username/password authentication for Vault.",
	"secret_store.vault.auth.password.username":               "Username for Vault password auth.",
	"secret_store.vault.auth.password.password":               "Password for Vault password auth.",
	"secret_store.vault.auth.password.mount":                  "Mount point for password authentication.",
	"secret_store.vault.auth.approle":                         "AppRole authentication details.",
	"secret_store.vault.auth.approle.role_id":                 "Role ID used for AppRole auth.",
	"secret_store.vault.auth.approle.secret_id":               "Secret ID used for AppRole auth.",
	"secret_store.vault.auth.approle.mount":                   "Mount path for the AppRole backend.",
	"secret_store.vault.tls":                                  "TLS overrides for Vault.",
	"secret_store.vault.tls.ca_cert_file":                     "CA certificate used to validate Vault.",
	"secret_store.vault.tls.client_cert_file":                 "Client certificate for Vault.",
	"secret_store.vault.tls.client_key_file":                  "Client private key for Vault TLS.",
	"secret_store.vault.tls.insecure_skip_verify":             "Skip TLS verification when talking to Vault.",
}
