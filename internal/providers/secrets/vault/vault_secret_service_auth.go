package vault

import (
	"strings"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
)

func buildVaultAuthConfig(cfg config.VaultAuth) (vaultAuthConfig, error) {
	setCount := countSet(
		strings.TrimSpace(cfg.Token) != "",
		cfg.Password != nil,
		cfg.AppRole != nil,
		cfg.Prompt != nil,
	)
	if setCount != 1 {
		return vaultAuthConfig{}, faults.NewValidationError("secret-store.vault.auth must define exactly one of token, password, approle, prompt", nil)
	}

	if strings.TrimSpace(cfg.Token) != "" {
		return vaultAuthConfig{
			mode:  vaultAuthToken,
			token: strings.TrimSpace(cfg.Token),
		}, nil
	}

	if cfg.Password != nil {
		if strings.TrimSpace(cfg.Password.Username) == "" || strings.TrimSpace(cfg.Password.Password) == "" {
			return vaultAuthConfig{}, faults.NewValidationError("secret-store.vault.auth.password requires username and password", nil)
		}
		copied := *cfg.Password
		return vaultAuthConfig{
			mode:     vaultAuthUserPass,
			userPass: &copied,
		}, nil
	}

	if cfg.AppRole != nil {
		if strings.TrimSpace(cfg.AppRole.RoleID) == "" || strings.TrimSpace(cfg.AppRole.SecretID) == "" {
			return vaultAuthConfig{}, faults.NewValidationError("secret-store.vault.auth.approle requires role-id and secret-id", nil)
		}
		copied := *cfg.AppRole
		return vaultAuthConfig{
			mode:    vaultAuthAppRole,
			appRole: &copied,
		}, nil
	}

	if cfg.Prompt != nil {
		copied := *cfg.Prompt
		copied.Mount = strings.TrimSpace(copied.Mount)
		return vaultAuthConfig{
			mode:   vaultAuthPrompt,
			prompt: &copied,
		}, nil
	}

	return vaultAuthConfig{}, faults.NewValidationError("secret-store.vault.auth is invalid", nil)
}

func countSet(values ...bool) int {
	count := 0
	for _, value := range values {
		if value {
			count++
		}
	}
	return count
}
