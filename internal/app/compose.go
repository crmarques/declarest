package app

import (
	ctxfile "github.com/crmarques/declarest/internal/adapters/ctx/file"
	reconnoop "github.com/crmarques/declarest/internal/adapters/noop/reconciler"
)

func NewContainer() Container {
	// Real adapters are composed incrementally; reconciler remains noop until implemented.
	return Container{
		Contexts:   ctxfile.NewManager(""),
		Reconciler: &reconnoop.Reconciler{},
	}
}
