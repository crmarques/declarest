package common

import (
	"github.com/crmarques/declarest/ctx"
	"github.com/crmarques/declarest/reconciler"
)

type CommandWiring struct {
	Reconciler reconciler.Reconciler
	Contexts   ctx.Manager
}
