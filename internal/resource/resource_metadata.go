package resource

type ResourceMetadata struct {
	ResourceInfo  *ResourceInfoMetadata  `mapstructure:"resourceInfo" yaml:"resourceInfo,omitempty" json:"resourceInfo,omitempty"`
	OperationInfo *OperationInfoMetadata `mapstructure:"operationInfo" yaml:"operationInfo,omitempty" json:"operationInfo,omitempty"`
}

type ResourceInfoMetadata struct {
	IDFromAttribute    string   `mapstructure:"idFromAttribute" yaml:"idFromAttribute,omitempty" json:"idFromAttribute,omitempty"`
	AliasFromAttribute string   `mapstructure:"aliasFromAttribute" yaml:"aliasFromAttribute,omitempty" json:"aliasFromAttribute,omitempty"`
	CollectionPath     string   `mapstructure:"collectionPath" yaml:"collectionPath,omitempty" json:"collectionPath,omitempty"`
	SecretInAttributes []string `mapstructure:"secretInAttributes" yaml:"secretInAttributes,omitempty" json:"secretInAttributes,omitempty"`
}

type OperationInfoMetadata struct {
	GetResource      *OperationMetadata `mapstructure:"getResource" yaml:"getResource,omitempty" json:"getResource,omitempty"`
	CreateResource   *OperationMetadata `mapstructure:"createResource" yaml:"createResource,omitempty" json:"createResource,omitempty"`
	UpdateResource   *OperationMetadata `mapstructure:"updateResource" yaml:"updateResource,omitempty" json:"updateResource,omitempty"`
	DeleteResource   *OperationMetadata `mapstructure:"deleteResource" yaml:"deleteResource,omitempty" json:"deleteResource,omitempty"`
	ListCollection   *OperationMetadata `mapstructure:"listCollection" yaml:"listCollection,omitempty" json:"listCollection,omitempty"`
	CompareResources *CompareMetadata   `mapstructure:"compareResources" yaml:"compareResources,omitempty" json:"compareResources,omitempty"`
}

type OperationMetadata struct {
	URL         *OperationURLMetadata   `mapstructure:"url" yaml:"url,omitempty" json:"url,omitempty"`
	HTTPMethod  string                  `mapstructure:"httpMethod" yaml:"httpMethod,omitempty" json:"httpMethod,omitempty"`
	HTTPHeaders HeaderList              `mapstructure:"httpHeaders" yaml:"httpHeaders,omitempty" json:"httpHeaders,omitempty"`
	Payload     *OperationPayloadConfig `mapstructure:"payload" yaml:"payload,omitempty" json:"payload,omitempty"`
	JQFilter    string                  `mapstructure:"jqFilter" yaml:"jqFilter,omitempty" json:"jqFilter,omitempty"`
}

type OperationURLMetadata struct {
	Path         string   `mapstructure:"path" yaml:"path,omitempty" json:"path,omitempty"`
	QueryStrings []string `mapstructure:"queryStrings" yaml:"queryStrings,omitempty" json:"queryStrings,omitempty"`
}

type OperationPayloadConfig struct {
	SuppressAttributes []string `mapstructure:"suppressAttributes" yaml:"suppressAttributes,omitempty" json:"suppressAttributes,omitempty"`
	FilterAttributes   []string `mapstructure:"filterAttributes" yaml:"filterAttributes,omitempty" json:"filterAttributes,omitempty"`
	JQExpression       string   `mapstructure:"jqExpression" yaml:"jqExpression,omitempty" json:"jqExpression,omitempty"`
}

type CompareMetadata struct {
	IgnoreAttributes   []string `mapstructure:"ignoreAttributes" yaml:"ignoreAttributes,omitempty" json:"ignoreAttributes,omitempty"`
	SuppressAttributes []string `mapstructure:"suppressAttributes" yaml:"suppressAttributes,omitempty" json:"suppressAttributes,omitempty"`
	FilterAttributes   []string `mapstructure:"filterAttributes" yaml:"filterAttributes,omitempty" json:"filterAttributes,omitempty"`
	JQExpression       string   `mapstructure:"jqExpression" yaml:"jqExpression,omitempty" json:"jqExpression,omitempty"`
}
