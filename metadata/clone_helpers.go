package metadata

func cloneBoolPointer(value *bool) *bool {
	if value == nil {
		return nil
	}

	cloned := *value
	return &cloned
}

func cloneStringSlice(values []string) []string {
	if values == nil {
		return nil
	}
	cloned := make([]string, len(values))
	copy(cloned, values)
	return cloned
}

func cloneStringSliceOrEmpty(values []string) []string {
	if len(values) == 0 {
		return []string{}
	}
	return cloneStringSlice(values)
}

func stringPointer(value string) *string {
	return &value
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func stringSlicePointer(values []string) *[]string {
	if values == nil {
		return nil
	}

	cloned := make([]string, len(values))
	copy(cloned, values)
	return &cloned
}

func stringMapPointer(values map[string]string) *map[string]string {
	if values == nil {
		return nil
	}

	cloned := make(map[string]string, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return &cloned
}

func headerMapWirePointer(values map[string]string) *headerMapWire {
	if values == nil {
		return nil
	}

	cloned := make(headerMapWire, len(values))
	for key, value := range values {
		cloned[key] = value
	}
	return &cloned
}
