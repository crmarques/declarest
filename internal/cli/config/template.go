package config

const contextTemplateYAML = `# Context catalog template for declarest.
# Fill the fields you need and remove examples/comments as desired.
contexts:
  - name: my-context
    repository:
      # Optional resource file format used by repository operations: json or yaml.
      # If omitted, declarest uses the remote resource format default.
      # resource-format: yaml

      # Mutually exclusive: choose exactly one repository backend.
      git:
        local:
          base-dir: /path/to/repository

        # Optional remote configuration.
        # remote:
        #   url: https://example.com/org/repo.git
        #   branch: main
        #   provider: github
        #   auto-sync: false
        #
        #   # Optional auth.
        #   auth:
        #     # Mutually exclusive: choose exactly one auth method.
        #     basic-auth:
        #       username: change-me
        #       password: change-me
        #     # ssh:
        #     #   user: git
        #     #   private-key-file: /path/to/id_rsa
        #     #   passphrase: change-me
        #     #   known-hosts-file: /path/to/known_hosts
        #     #   insecure-ignore-host-key: false
        #     # access-key:
        #     #   token: change-me
        #
        #   # Optional TLS.
        #   tls:
        #     ca-cert-file: /path/to/ca.pem
        #     client-cert-file: /path/to/client.pem
        #     client-key-file: /path/to/client-key.pem
        #     insecure-skip-verify: false

      # filesystem:
      #   base-dir: /path/to/repository

    # Required resource-server.
    resource-server:
      http:
        base-url: https://example.com/api
        # openapi: /path/to/openapi.yaml

        # Optional default request headers.
        # default-headers:
        #   X-Example: value

        # Mutually exclusive: choose exactly one auth method.
        auth:
          bearer-token:
            token: change-me
          # oauth2:
          #   token-url: https://example.com/oauth/token
          #   grant-type: client_credentials
          #   client-id: change-me
          #   client-secret: change-me
          #   username: change-me
          #   password: change-me
          #   scope: api.read
          #   audience: https://example.com/
          # basic-auth:
          #   username: change-me
          #   password: change-me
          # custom-header:
          #   header: X-Api-Token
          #   token: change-me

        # Optional TLS.
        # tls:
        #   ca-cert-file: /path/to/ca.pem
        #   client-cert-file: /path/to/client.pem
        #   client-key-file: /path/to/client-key.pem
        #   insecure-skip-verify: false

    # Optional secret store.
    # secret-store:
    #   # Mutually exclusive: choose exactly one provider.
    #   file:
    #     path: /path/to/secrets.json
    #     # Mutually exclusive: choose exactly one key source.
    #     passphrase: change-me
    #     # key: base64-encoded-key
    #     # key-file: /path/to/key.txt
    #     # passphrase-file: /path/to/passphrase.txt
    #     # Optional KDF tuning.
    #     # kdf:
    #     #   time: 1
    #     #   memory: 65536
    #     #   threads: 4
    #   # vault:
    #   #   address: https://vault.example.com
    #   #   mount: secret
    #   #   path-prefix: declarest
    #   #   kv-version: 2
    #   #   auth:
    #   #     # Mutually exclusive: choose exactly one vault auth method.
    #   #     token: s.xxxx
    #   #     # password:
    #   #     #   username: vault-user
    #   #     #   password: vault-pass
    #   #     #   mount: userpass
    #   #     # approle:
    #   #     #   role-id: role-id
    #   #     #   secret-id: secret-id
    #   #     #   mount: approle
    #   #   tls:
    #   #     ca-cert-file: /path/to/ca.pem
    #   #     client-cert-file: /path/to/client.pem
    #   #     client-key-file: /path/to/client-key.pem
    #   #     insecure-skip-verify: false

    # Optional metadata directory override.
    # metadata:
    #   base-dir: /path/to/metadata

    # Optional arbitrary key/value preferences.
    # preferences:
    #   env: dev
    #   owner: team-a

current-ctx: my-context
`
