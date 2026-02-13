package app

import (
	"github.com/crmarques/declarest/ctx"
	"github.com/crmarques/declarest/internal/cli/common"
	"github.com/crmarques/declarest/reconciler"
)

type Container struct {
	Contexts   ctx.Manager
	Reconciler reconciler.Reconciler
}

func (c Container) CommandWiring() common.CommandWiring {
	return common.CommandWiring{
		Reconciler: c.Reconciler,
		Contexts:   c.Contexts,
	}
}
