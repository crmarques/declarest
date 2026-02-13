package resource

import "github.com/crmarques/declarest/core"

type DiffEntry struct {
	Path      string
	Operation string
	Local     core.Resource
	Remote    core.Resource
}
