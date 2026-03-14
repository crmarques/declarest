package diff

import "github.com/crmarques/declarest/resource"

type Document struct {
	ResourcePath string
	Local        resource.Content
	Remote       resource.Content
	Entries      []resource.DiffEntry
}
