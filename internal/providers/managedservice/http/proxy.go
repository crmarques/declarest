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

package http

import (
	"net/http"
	"net/url"

	"github.com/crmarques/declarest/config"
	"github.com/crmarques/declarest/internal/promptauth"
	proxyhelper "github.com/crmarques/declarest/internal/proxy"
)

func buildProxyFunc(proxyConfig *config.HTTPProxy, runtime *promptauth.Runtime) (func(*http.Request) (*url.URL, error), error) {
	cfg, disabled, err := proxyhelper.ResolveWithRuntime("managed-service.http.proxy", proxyConfig, runtime)
	if err != nil {
		return nil, err
	}
	if disabled {
		return nil, nil
	}
	return cfg.Resolver(), nil
}
