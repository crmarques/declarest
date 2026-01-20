package context

import (
	"errors"

	"github.com/crmarques/declarest/reconciler"
)

type Context struct {
	Name       string
	Reconciler reconciler.AppReconciler
}

func (c *Context) Init() error {
	if c == nil {
		return errors.New("context is nil")
	}
	if c.Reconciler == nil {
		return errors.New("context reconciler is not configured")
	}
	return c.Reconciler.Init()
}
