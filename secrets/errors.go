package secrets

import "errors"

var ErrSecretStoreNotConfigured = errors.New("secret store is not configured")

var ErrSecretStoreNotInitialized = errors.New("secret store is not initialized")
