package metadata

import (
	"encoding/json"
	"fmt"

	"declarest/internal/resource"
)

// ValidateMetadataDocument ensures the provided map matches the expected metadata schema.
func ValidateMetadataDocument(doc map[string]any) error {
	if doc == nil {
		return nil
	}
	data, err := json.Marshal(doc)
	if err != nil {
		return fmt.Errorf("encode metadata: %w", err)
	}

	var parsed resource.ResourceMetadata
	if err := json.Unmarshal(data, &parsed); err != nil {
		return fmt.Errorf("invalid metadata document: %w", err)
	}
	return nil
}
