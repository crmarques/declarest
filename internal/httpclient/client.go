// Copyright 2026 Carlos Marques
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package httpclient

import (
	"net/http"
	"time"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/promptauth"
	proxyhelper "github.com/crmarques/declarest/internal/proxy"
)

// Options configures the shared HTTP client factory.
//
// Timeout defaults to 30 seconds when zero.
//
// TLS + TLSScope are forwarded to BuildTLSConfig; the scope prefix is used in
// error messages surfaced by TLS parsing.
//
// Proxy + ProxyScope + ProxyRuntime drive proxy resolution via the shared
// proxy helper; nil fields produce no proxy.
//
// BaseTransport allows callers with bespoke dialers (operator artifact cache)
// to supply their own transport skeleton; a clone is made before mutation.
type Options struct {
	Timeout       time.Duration
	TLS           *config.TLS
	TLSScope      string
	Proxy         *config.HTTPProxy
	ProxyScope    string
	ProxyRuntime  *promptauth.Runtime
	BaseTransport *http.Transport
}

const defaultTimeout = 30 * time.Second

// Build returns an *http.Client configured for TLS, proxy, and optional base
// transport. The returned client's Transport field is a *http.Transport owned
// by the caller and safe to mutate further (e.g., managedservice/http sets
// its own Proxy function after auth resolution).
func Build(opts Options) (*http.Client, error) {
	transport := opts.BaseTransport
	if transport == nil {
		transport = http.DefaultTransport.(*http.Transport).Clone()
	} else {
		transport = transport.Clone()
	}
	transport.Proxy = nil

	tlsConfig, err := BuildTLSConfig(opts.TLS, opts.TLSScope)
	if err != nil {
		return nil, err
	}
	transport.TLSClientConfig = tlsConfig

	proxyConfig, disabled, err := proxyhelper.ResolveWithRuntime(opts.ProxyScope, opts.Proxy, opts.ProxyRuntime)
	if err != nil {
		return nil, err
	}
	if !disabled {
		if resolver := proxyConfig.Resolver(); resolver != nil {
			transport.Proxy = resolver
		}
	}

	timeout := opts.Timeout
	if timeout == 0 {
		timeout = defaultTimeout
	}

	return &http.Client{
		Timeout:   timeout,
		Transport: transport,
	}, nil
}
