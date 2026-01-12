package secrets

import "errors"

// ErrSecretStoreNotConfigured indicates that no secret store configuration is available.
var ErrSecretStoreNotConfigured = errors.New("secret store is not configured")

// ErrSecretStoreNotInitialized indicates that initialization (e.g., `declarest secret init`)
// has not been performed for the configured secret store.
var ErrSecretStoreNotInitialized = errors.New("secret store is not initialized")
