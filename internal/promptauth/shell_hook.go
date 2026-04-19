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

package promptauth

import (
	"strings"

	"github.com/crmarques/declarest/faults"
)

func RenderSessionHook(shell string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(shell)) {
	case "bash":
		return bashSessionHook, nil
	case "zsh":
		return zshSessionHook, nil
	default:
		return "", faults.Invalid("context session-hook shell must be bash or zsh", nil)
	}
}

const bashSessionHook = `if [ -z "${DECLAREST_PROMPT_AUTH_SESSION_ID:-}" ]; then
  export DECLAREST_PROMPT_AUTH_SESSION_ID="bash:${BASHPID:-$$}"
fi
declare -f declarest_prompt_auth_cleanup >/dev/null 2>&1 || declarest_prompt_auth_cleanup() {
  command declarest context clean --credentials-in-session >/dev/null 2>&1 || true
}
if [ -z "${__declarest_prompt_auth_bash_hook_installed:-}" ]; then
  __declarest_prompt_auth_prev_exit="$(trap -p EXIT | sed -n "s/^trap -- '\\(.*\\)' EXIT$/\\1/p")"
  declarest_prompt_auth_on_exit() {
    declarest_prompt_auth_cleanup
    if [ -n "${__declarest_prompt_auth_prev_exit:-}" ]; then
      eval "$__declarest_prompt_auth_prev_exit"
    fi
  }
  trap 'declarest_prompt_auth_on_exit' EXIT
  __declarest_prompt_auth_bash_hook_installed=1
fi
`

const zshSessionHook = `if [ -z "${DECLAREST_PROMPT_AUTH_SESSION_ID:-}" ]; then
  export DECLAREST_PROMPT_AUTH_SESSION_ID="zsh:$$"
fi
typeset -f declarest_prompt_auth_cleanup >/dev/null 2>&1 || declarest_prompt_auth_cleanup() {
  command declarest context clean --credentials-in-session >/dev/null 2>&1 || true
}
typeset -ga zshexit_functions
if (( ${zshexit_functions[(Ie)declarest_prompt_auth_cleanup]} == 0 )); then
  zshexit_functions+=(declarest_prompt_auth_cleanup)
fi
`
