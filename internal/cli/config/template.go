package config

const contextTemplateYAML = `# Context catalog template for declarest.
# Fill the fields you need and remove examples/comments as desired.
# Optional default editor for commands that open an editor (defaults to vi).
# defaultEditor: vi
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
        #     basicAuth:
        #       username: change-me
        #       password: change-me
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
        baseURL: https://example.com/api
        # healthCheck: /health
        # openapi: /path/to/openapi-or-swagger.yaml
        # If omitted and metadata.bundle is configured, declarest can fallback to bundle OpenAPI hints.

        # Optional default request headers.
        # defaultHeaders:
        #   X-Example: value

        # Optional managedServer proxy.
        # proxy:
        #   # Configure one or both proxy URLs.
        #   httpURL: http://proxy.example.com:3128
        #   httpsURL: http://proxy.example.com:3128
        #   # Optional comma-separated bypass rules.
        #   noProxy: localhost,127.0.0.1,.svc.cluster.local
        #   # Optional proxy auth.
        #   auth:
        #     username: proxy-user
        #     password: proxy-pass

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
          # basicAuth:
          #   username: change-me
          #   password: change-me
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
    #   #     #   username: vault-user
    #   #     #   password: vault-pass
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

    # Optional arbitrary key/value preferences.
    # preferences:
    #   env: dev
    #   owner: team-a

currentContext: my-context
`
