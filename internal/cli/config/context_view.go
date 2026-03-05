package config

import (
	"fmt"

	configdomain "github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/faults"
)

func compactContextCatalogForView(catalog configdomain.ContextCatalog) configdomain.ContextCatalog {
	return configdomain.CompactContextCatalog(catalog)
}

func selectContextForView(contexts []configdomain.Context, name string) (configdomain.Context, int, error) {
	for idx, item := range contexts {
		if item.Name != name {
			continue
		}
		return configdomain.CompactContext(item), idx, nil
	}
	return configdomain.Context{}, -1, faults.NewTypedError(faults.NotFoundError, fmt.Sprintf("context %q not found", name), nil)
}
