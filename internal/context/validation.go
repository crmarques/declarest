package context

// ValidateContextConfig checks that the provided configuration can build a reconciler.
func ValidateContextConfig(cfg *ContextConfig) error {
	_, err := buildReconcilerFromConfig(cfg)
	return err
}
