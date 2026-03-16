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

package config

const contextTemplateYAML = `# Context catalog template for declarest.
# Fill the fields you need and remove examples/comments as desired.
# Optional default editor for commands that open an editor (defaults to vi).
# defaultEditor: vi
# Optional reusable credentials.
# Referenced credentials are injected where credentialsRef is used.
credentials:
  - name: shared-basic
    username: change-me
    password: change-me
  # - name: prompt-basic
  #   username:
  #     prompt: true
  #     persistInSession: true
  #   password:
  #     prompt: true
  #     persistInSession: true
  #   # Reuse across later declarest commands in one shell session:
  #   # eval "$(declarest context session-hook bash)"

contexts:
  - name: my-context
    repository:
      # Mutually exclusive: choose exactly one repository backend.
      git:
        local:
          baseDir: /path/to/repository
          # autoInit: true

        # Optional remote configuration.
        # remote:
        #   url: https://example.com/org/repo.git
        #   branch: main
        #   provider: github
        #   autoSync: true
        #
        #   # Optional auth.
        #   auth:
        #     # Mutually exclusive: choose exactly one auth method.
        #     basic:
        #       credentialsRef:
        #         name: shared-basic
        #     # ssh:
        #     #   user: git
        #     #   privateKeyFile: /path/to/id_rsa
        #     #   passphrase: change-me
        #     #   knownHostsFile: /path/to/known_hosts
        #     #   insecureIgnoreHostKey: false
        #     # accessKey:
        #     #   token: change-me
        #
        #   # Optional TLS.
        #   tls:
        #     caCertFile: /path/to/ca.pem
        #     clientCertFile: /path/to/client.pem
        #     clientKeyFile: /path/to/client-key.pem
        #     insecureSkipVerify: false

      # filesystem:
      #   baseDir: /path/to/repository

    # Required managedServer.
    managedServer:
      http:
        url: https://example.com/api
        # healthCheck: /health
        # openapi: /path/to/openapi-or-swagger.yaml
        # If omitted and metadata.bundle is configured, declarest can fallback to bundle OpenAPI hints.

        # Optional default request headers.
        # defaultHeaders:
        #   X-Example: value

        # Optional managedServer proxy.
        # proxy:
        #   # Configure one or both proxy URLs.
        #   http: http://proxy.example.com:3128
        #   https: http://proxy.example.com:3128
        #   # Optional comma-separated bypass rules.
        #   noProxy: localhost,127.0.0.1,.svc.cluster.local
        #   # Optional proxy auth.
        #   auth:
        #     basic:
        #       credentialsRef:
        #         name: shared-basic

        # Mutually exclusive: choose exactly one auth method.
        auth:
          customHeaders:
            - header: Authorization
              prefix: Bearer
              value: change-me
          # oauth2:
          #   tokenURL: https://example.com/oauth/token
          #   grantType: client_credentials
          #   clientID: change-me
          #   clientSecret: change-me
          #   username: change-me
          #   password: change-me
          #   scope: api.read
          #   audience: https://example.com/
          # basic:
          #   credentialsRef:
          #     name: shared-basic
          # customHeaders:
          #   - header: X-API-Key
          #     value: change-me

        # Optional TLS.
        # tls:
        #   caCertFile: /path/to/ca.pem
        #   clientCertFile: /path/to/client.pem
        #   clientKeyFile: /path/to/client-key.pem
        #   insecureSkipVerify: false

    # Optional secret store.
    # secretStore:
    #   # Mutually exclusive: choose exactly one provider.
    #   file:
    #     path: /path/to/secrets.json
    #     # Mutually exclusive: choose exactly one key source.
    #     passphrase: change-me
    #     # key: base64-encoded-key
    #     # keyFile: /path/to/key.txt
    #     # passphraseFile: /path/to/passphrase.txt
    #     # Optional KDF tuning.
    #     # kdf:
    #     #   time: 1
    #     #   memory: 65536
    #     #   threads: 4
    #   # vault:
    #   #   address: https://vault.example.com
    #   #   mount: secret
    #   #   pathPrefix: declarest
    #   #   kvVersion: 2
    #   #   auth:
    #   #     # Mutually exclusive: choose exactly one vault auth method.
    #   #     token: s.xxxx
    #   #     # password:
    #   #     #   credentialsRef:
    #   #     #     name: shared-basic
    #   #     #   mount: userpass
    #   #     # appRole:
    #   #     #   roleID: role-id
    #   #     #   secretID: secret-id
    #   #     #   mount: approle
    #   #   tls:
    #   #     caCertFile: /path/to/ca.pem
    #   #     clientCertFile: /path/to/client.pem
    #   #     clientKeyFile: /path/to/client-key.pem
    #   #     insecureSkipVerify: false

    # Optional metadata source.
    # metadata:
    #   # Choose at most one metadata source.
    #   # baseDir: /path/to/metadata
    #   # bundle: keycloak-bundle:0.0.1
    #   # bundleFile: /path/to/keycloak-bundle-0.0.1.tar.gz
    #   # proxy:
    #   #   http: http://proxy.example.com:3128
    #   #   https: http://proxy.example.com:3128
    #   #   noProxy: localhost,127.0.0.1,.svc.cluster.local
    #   #   auth:
    #   #     basic:
    #   #       credentialsRef:
    #   #         name: shared-basic

    # Optional arbitrary key/value preferences.
    # preferences:
    #   env: dev
    #   owner: team-a

currentContext: my-context
`
