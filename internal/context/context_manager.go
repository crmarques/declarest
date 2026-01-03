package context

type ContextManager interface {
	AddContext(name string, file string) error
	UpdateContext(name string, file string) error
	DeleteContext(name string) error
	RenameContext(currentName string, newName string) error
	SetDefaultContext(name string) error
	GetDefaultContext() (string, error)
	ListContexts() ([]string, error)
	LoadDefaultContext() (Context, error)
}
