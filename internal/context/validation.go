package context

func ValidateContextConfig(cfg *ContextConfig) error {
	_, err := buildReconcilerFromConfig(cfg)
	return err
}
