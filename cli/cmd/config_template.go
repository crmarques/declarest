package cmd

import (
	"io"

	"github.com/spf13/cobra"
)

const configTemplateYAML = `repository:
  # Choose exactly one repository type: filesystem or git.
  git:
    local:
      base_dir: /path/to/repo
#   ### remote git repo is optional
#   remote:
#     url: https://example.com/org/repo.git
#     branch: main
#     provider: github
#     auto_sync: true
#     auth:
#       ### Choose exactly one auth method: basic_auth, ssh, access_key.
#       basic_auth:
#         username: change-me
#         password: change-me
#       ssh:
#         user: git
#         private_key_file: /path/to/id_rsa
#         passphrase: change-me
#         known_hosts_file: /path/to/known_hosts
#         insecure_ignore_host_key: false
#       access_key:
#         token: change-me
#     tls:
#       insecure_skip_verify: false
# filesystem:
#    base_dir: /path/to/repo

managed_server:
  http:
    base_url: https://example.com/api
#   default_headers:
#     X-Example: value
    auth:
#     ### Choose exactly one auth method: oauth2, basic_auth, bearer_token.
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
#   tls:
#     insecure_skip_verify: false

secret_manager:
# ### Choose exactly one: file or vault (to be implemented).
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
#    ### to be implemented
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
