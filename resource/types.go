package resource

type Value = any

type PayloadDescriptor struct {
	PayloadType string
	MediaType   string
	Extension   string
}

type Content struct {
	Value      Value
	Descriptor PayloadDescriptor
}

type Resource struct {
	LogicalPath        string
	CollectionPath     string
	LocalAlias         string
	RemoteID           string
	ResolvedRemotePath string
	Payload            Value
	PayloadDescriptor  PayloadDescriptor
}

type DiffEntry struct {
	ResourcePath string
	Path         string
	Operation    string
	Local        Value
	Remote       Value
}
