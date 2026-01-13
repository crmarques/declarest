package cmd

import (
	"io"

	"github.com/spf13/cobra"
)

const configTemplateYAML = `repository:
  # Resource file format: json (default) or yaml.
  resource_format: json
  # Choose exactly one repository type: filesystem or git.
  git:
    local:
      base_dir: /path/to/repo
#  ### remote git repo is optional
#  remote:
#    url: https://example.com/org/repo.git
#    branch: main
#    provider: github
#    auto_sync: true
#    auth:
#      ### Choose exactly one auth method: basic_auth, ssh, access_key.
#      basic_auth:
#        username: change-me
#        password: change-me
#      ssh:
#        user: git
#        private_key_file: /path/to/id_rsa
#        passphrase: change-me
#        known_hosts_file: /path/to/known_hosts
#        insecure_ignore_host_key: false
#      access_key:
#        token: change-me
#    tls:
#      insecure_skip_verify: false
# filesystem:
#   base_dir: /path/to/repo

managed_server:
  http:
    base_url: https://example.com/api
#   openapi: /path/to/openapi.yaml
#   default_headers:
#     X-Example: value
    auth:
#     ### Choose exactly one auth method: oauth2, basic_auth, bearer_token, custom_header.
      oauth2:
        token_url: https://example.com/oauth/token
        grant_type: client_credentials
        client_id: change-me
        client_secret: change-me
#       username: change-me
#       password: change-me
#       scope: api.read
#       audience: https://example.com/
#     basic_auth:
#       username: change-me
#       password: change-me
#     bearer_token:
#       token: change-me
#     custom_header:
#       header: X-Example-Token
#       token: change-me
#   tls:
#     insecure_skip_verify: false

secret_store:
# ### Choose exactly one: file or vault.
  file:
    path: /path/to/secrets.json
#   ### Choose exactly one of: key, key_file, passphrase, passphrase_file.
    passphrase: change-me
#    key: base64-encoded-key
#    key_file: /path/to/key.txt
#    passphrase_file: /path/to/passphrase.txt
#    kdf:
#      time: 1
#      memory: 65536
#      threads: 4
# vault:
#   address: https://vault.example.com
#   mount: secret
#   path_prefix: declarest
#   kv_version: 2
#   auth:
#     token: s.xxxx
#     # password:
#     #   username: vault-user
#     #   password: vault-pass
#     #   mount: userpass
#     # approle:
#     #   role_id: role-id
#     #   secret_id: secret-id
#     #   mount: approle
#   # mTLS is optional; enabled when client cert/key files are provided.
#   tls:
#     ca_cert_file: /path/to/ca.pem
#     client_cert_file: /path/to/client.pem
#     client_key_file: /path/to/client-key.pem
#     insecure_skip_verify: false
metadata:
# metadata files default to the repository base directory when unset.
  base_dir: /path/to/metadata
`

func newConfigPrintTemplateCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "print-template",
		Short: "Print a full context configuration file",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := io.WriteString(cmd.OutOrStdout(), configTemplateYAML)
			return err
		},
	}

	return cmd
}
