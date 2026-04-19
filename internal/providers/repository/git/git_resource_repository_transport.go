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

package git

import (
	"context"
	"net/url"

	"github.com/crmarques/declarest/faults"
	"github.com/crmarques/declarest/internal/promptauth"
	proxyhelper "github.com/crmarques/declarest/internal/proxy"
	"github.com/go-git/go-git/v5/plumbing/transport"
	httpauth "github.com/go-git/go-git/v5/plumbing/transport/http"
	sshauth "github.com/go-git/go-git/v5/plumbing/transport/ssh"
)

func (r *GitResourceRepository) authMethod(ctx context.Context) (transport.AuthMethod, error) {
	if r.remote == nil || r.remote.Auth == nil {
		return nil, nil
	}

	auth := r.remote.Auth
	switch {
	case auth.Basic != nil:
		creds, err := promptauth.ResolveCredentials(
			r.runtime,
			ctx,
			auth.Basic.CredentialName(),
			auth.Basic.Username,
			auth.Basic.Password,
		)
		if err != nil {
			return nil, err
		}
		return &httpauth.BasicAuth{
			Username: creds.Username,
			Password: creds.Password,
		}, nil
	case auth.AccessKey != nil:
		return &httpauth.BasicAuth{
			Username: "token",
			Password: auth.AccessKey.Token,
		}, nil
	case auth.SSH != nil:
		username := auth.SSH.User
		if username == "" {
			username = "git"
		}

		sshKeys, err := sshauth.NewPublicKeysFromFile(username, auth.SSH.PrivateKeyFile, auth.SSH.Passphrase)
		if err != nil {
			return nil, faults.Auth("failed to load git ssh auth configuration", err)
		}
		return sshKeys, nil
	default:
		return nil, faults.Invalid("git remote auth configuration is invalid", nil)
	}
}

func (r *GitResourceRepository) proxyOptions(ctx context.Context) (transport.ProxyOptions, error) {
	proxyConfig, disabled, err := proxyhelper.ResolveWithRuntime("repository.git.remote.proxy", r.proxy, r.runtime)
	if err != nil {
		return transport.ProxyOptions{}, err
	}
	if disabled || !proxyConfig.HasProxy() {
		return transport.ProxyOptions{}, nil
	}

	envVars, err := proxyConfig.Env(ctx)
	if err != nil {
		return transport.ProxyOptions{}, err
	}

	proxyURL := envVars["HTTPS_PROXY"]
	if proxyURL == "" {
		proxyURL = envVars["HTTP_PROXY"]
	}
	if proxyURL == "" {
		return transport.ProxyOptions{}, nil
	}

	parsed, err := url.Parse(proxyURL)
	if err != nil {
		return transport.ProxyOptions{URL: proxyURL}, nil
	}

	opts := transport.ProxyOptions{URL: proxyURL}
	if parsed.User != nil {
		opts.Username = parsed.User.Username()
		opts.Password, _ = parsed.User.Password()
		parsed.User = nil
		opts.URL = parsed.String()
	}
	return opts, nil
}
