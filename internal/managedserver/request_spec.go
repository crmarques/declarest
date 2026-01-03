package managedserver

import "time"

type RequestKind string

const (
	KindHTTP  RequestKind = "http"
	KindGRPC  RequestKind = "grpc"
	KindOther RequestKind = "other"
)

type RequestSpec struct {
	Kind    RequestKind      `json:"kind"`
	Timeout time.Duration    `json:"timeout"`
	HTTP    *HTTPRequestSpec `json:"http,omitempty"`
	GRPC    *GRPCRequestSpec `json:"grpc,omitempty"`
}

type HTTPRequestSpec struct {
	Method      string              `json:"method"`
	Path        string              `json:"path"`
	Query       map[string][]string `json:"query,omitempty"`
	Headers     map[string][]string `json:"headers,omitempty"`
	ContentType string              `json:"contentType,omitempty"`
	Accept      string              `json:"accept,omitempty"`
}

type GRPCRequestSpec struct {
	Service   string              `json:"service"`
	Method    string              `json:"method"`
	Metadata  map[string][]string `json:"metadata,omitempty"`
	Authority string              `json:"authority,omitempty"`
}
