package resource

type Value = any

type Resource struct {
	LogicalPath        string
	CollectionPath     string
	LocalAlias         string
	RemoteID           string
	ResolvedRemotePath string
	Payload            Value
}

type DiffEntry struct {
	ResourcePath string
	Path         string
	Operation    string
	Local        Value
	Remote       Value
}
