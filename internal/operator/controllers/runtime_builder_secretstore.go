package controllers

import (
	"context"
	"path/filepath"

	declarestv1alpha1 "github.com/crmarques/declarest/api/v1alpha1"
	"github.com/crmarques/declarest/config"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func populateSecretStoreConfig(
	ctx context.Context,
	reader client.Reader,
	namespace string,
	secretStore *declarestv1alpha1.SecretStore,
	resolvedContext *config.Context,
	cacheDir string,
	cleanup *cleanupRegistry,
) error {
	resolvedContext.SecretStore = &config.SecretStore{}

	if secretStore.Spec.File != nil {
		fileConfig := &config.FileSecretStore{
			Path: secretStore.Spec.File.Path,
		}
		if secretStore.Spec.File.Encryption.KeyRef != nil {
			value, err := readSecretValue(ctx, reader, namespace, secretStore.Spec.File.Encryption.KeyRef)
			if err != nil {
				return err
			}
			fileConfig.Key = value
		}
		if secretStore.Spec.File.Encryption.PassphraseRef != nil {
			value, err := readSecretValue(ctx, reader, namespace, secretStore.Spec.File.Encryption.PassphraseRef)
			if err != nil {
				return err
			}
			fileConfig.Passphrase = value
		}
		resolvedContext.SecretStore.File = fileConfig
		return nil
	}

	vault := secretStore.Spec.Vault
	vaultConfig := &config.VaultSecretStore{
		Address:    vault.Address,
		Mount:      vault.Mount,
		PathPrefix: vault.PathPrefix,
		KVVersion:  vault.KVVersion,
	}
	vaultAuth := &config.VaultAuth{}
	if vault.Auth.Token != nil && vault.Auth.Token.SecretRef != nil {
		token, err := readSecretValue(ctx, reader, namespace, vault.Auth.Token.SecretRef)
		if err != nil {
			return err
		}
		vaultAuth.Token = token
	}
	if vault.Auth.Userpass != nil {
		username, err := readSecretValue(ctx, reader, namespace, vault.Auth.Userpass.UsernameRef)
		if err != nil {
			return err
		}
		password, err := readSecretValue(ctx, reader, namespace, vault.Auth.Userpass.PasswordRef)
		if err != nil {
			return err
		}
		vaultAuth.Password = &config.VaultUserPasswordAuth{Username: username, Password: password, Mount: vault.Auth.Userpass.Mount}
	}
	if vault.Auth.AppRole != nil {
		roleID, err := readSecretValue(ctx, reader, namespace, vault.Auth.AppRole.RoleIDRef)
		if err != nil {
			return err
		}
		secretID, err := readSecretValue(ctx, reader, namespace, vault.Auth.AppRole.SecretIDRef)
		if err != nil {
			return err
		}
		vaultAuth.AppRole = &config.VaultAppRoleAuth{RoleID: roleID, SecretID: secretID, Mount: vault.Auth.AppRole.Mount}
	}
	vaultConfig.Auth = vaultAuth

	if vault.TLS != nil {
		tlsConfig := &config.TLS{InsecureSkipVerify: vault.TLS.InsecureSkipVerify}
		if vault.TLS.CACertRef != nil {
			value, err := readSecretValue(ctx, reader, namespace, vault.TLS.CACertRef)
			if err != nil {
				return err
			}
			path, err := writeSecretValueToFileWithCleanup(cleanup, filepath.Join(cacheDir, "vault-tls"), "ca-cert", value)
			if err != nil {
				return err
			}
			tlsConfig.CACertFile = path
		}
		if vault.TLS.ClientCertRef != nil {
			value, err := readSecretValue(ctx, reader, namespace, vault.TLS.ClientCertRef)
			if err != nil {
				return err
			}
			path, err := writeSecretValueToFileWithCleanup(cleanup, filepath.Join(cacheDir, "vault-tls"), "client-cert", value)
			if err != nil {
				return err
			}
			tlsConfig.ClientCertFile = path
		}
		if vault.TLS.ClientKeyRef != nil {
			value, err := readSecretValue(ctx, reader, namespace, vault.TLS.ClientKeyRef)
			if err != nil {
				return err
			}
			path, err := writeSecretValueToFileWithCleanup(cleanup, filepath.Join(cacheDir, "vault-tls"), "client-key", value)
			if err != nil {
				return err
			}
			tlsConfig.ClientKeyFile = path
		}
		vaultConfig.TLS = tlsConfig
	}
	if vault.Proxy != nil {
		proxyConfig := &config.HTTPProxy{HTTPURL: vault.Proxy.HTTPURL, HTTPSURL: vault.Proxy.HTTPSURL, NoProxy: vault.Proxy.NoProxy}
		if vault.Proxy.Auth != nil {
			username, err := readSecretValue(ctx, reader, namespace, vault.Proxy.Auth.UsernameRef)
			if err != nil {
				return err
			}
			password, err := readSecretValue(ctx, reader, namespace, vault.Proxy.Auth.PasswordRef)
			if err != nil {
				return err
			}
			proxyConfig.Auth = &config.ProxyAuth{Username: username, Password: password}
		}
		vaultConfig.Proxy = proxyConfig
	}
	resolvedContext.SecretStore.Vault = vaultConfig
	return nil
}
