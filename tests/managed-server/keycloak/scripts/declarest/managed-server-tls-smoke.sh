#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
SCRIPTS_DIR="$(cd "$SCRIPT_DIR/.." && pwd)"
# shellcheck source=../lib/env.sh
source "$SCRIPTS_DIR/lib/env.sh"
# shellcheck source=../lib/logging.sh
source "$SCRIPTS_DIR/lib/logging.sh"
# shellcheck source=../lib/shell.sh
source "$SCRIPTS_DIR/lib/shell.sh"
# shellcheck source=../lib/cli.sh
source "$SCRIPTS_DIR/lib/cli.sh"

require_cmd python3
require_cmd openssl

tmpdir="$(mktemp -d)"
server_pid=""
context_added=0
context_deleted=0
context_restored=0
context_name="tls-${DECLAREST_RUN_ID:-tls}"
context_cfg="$tmpdir/context.yaml"
orig_context_name="${DECLAREST_CONTEXT_NAME:-}"

cleanup() {
    set +e
    if [[ -n "$server_pid" ]]; then
        kill "$server_pid" >/dev/null 2>&1
    fi
    if [[ $context_restored -eq 0 && -n "$orig_context_name" ]]; then
        run_cli "restore tls context (cleanup)" config use "$orig_context_name"
        context_restored=1
    fi
    if [[ $context_added -eq 1 && $context_deleted -eq 0 ]]; then
        run_cli "delete tls context (cleanup)" config delete "$context_name" --yes
        context_deleted=1
    fi
    rm -rf "$tmpdir"
}
trap cleanup EXIT

log_line "Generating mutual TLS assets"
port="$(python3 - "$tmpdir" <<'PY'
import socket, sys
tmp = sys.argv[1]
with socket.socket(socket.AF_INET, socket.SOCK_STREAM) as sock:
    sock.bind(("127.0.0.1", 0))
    print(sock.getsockname()[1])
PY
)"

cat <<'EOF' > "$tmpdir/server.cnf"
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = 127.0.0.1

[v3_req]
subjectAltName = @alt_names

[alt_names]
IP.1 = 127.0.0.1
EOF

cat <<'EOF' > "$tmpdir/client.cnf"
[req]
distinguished_name = req_distinguished_name
req_extensions = v3_req
prompt = no

[req_distinguished_name]
CN = declarest-client

[v3_req]
extendedKeyUsage = clientAuth
EOF

run_logged "create CA cert" openssl req -x509 -newkey rsa:2048 -nodes -subj "/CN=declarest-tls-ca" \
    -keyout "$tmpdir/ca.key" -out "$tmpdir/ca.pem" -days 365

run_logged "create server csr" openssl req -newkey rsa:2048 -nodes -keyout "$tmpdir/server.key" \
    -out "$tmpdir/server.csr" -config "$tmpdir/server.cnf"

run_logged "sign server cert" openssl x509 -req -in "$tmpdir/server.csr" -CA "$tmpdir/ca.pem" \
    -CAkey "$tmpdir/ca.key" -CAcreateserial -out "$tmpdir/server.pem" -days 365 \
    -extensions v3_req -extfile "$tmpdir/server.cnf"

run_logged "create client csr" openssl req -newkey rsa:2048 -nodes -keyout "$tmpdir/client.key" \
    -out "$tmpdir/client.csr" -config "$tmpdir/client.cnf"

run_logged "sign client cert" openssl x509 -req -in "$tmpdir/client.csr" -CA "$tmpdir/ca.pem" \
    -CAkey "$tmpdir/ca.key" -CAcreateserial -out "$tmpdir/client.pem" -days 365 \
    -extensions v3_req -extfile "$tmpdir/client.cnf"

log_line "Starting TLS server with client authentication on port $port"
openssl s_server -accept "127.0.0.1:$port" -cert "$tmpdir/server.pem" -key "$tmpdir/server.key" \
    -WWW -Verify 1 -CAfile "$tmpdir/ca.pem" > "$tmpdir/openssl.log" 2>&1 &
server_pid=$!
sleep 1
if ! kill -0 "$server_pid" >/dev/null 2>&1; then
    cat "$tmpdir/openssl.log"
    die "TLS server failed to start"
fi

mkdir -p "$DECLAREST_WORK_DIR/tls-repo"

cat <<EOF > "$context_cfg"
repository:
  filesystem:
    base_dir: "$DECLAREST_WORK_DIR/tls-repo"
managed_server:
  http:
    base_url: "https://127.0.0.1:$port"
    tls:
      ca_cert_file: "$tmpdir/ca.pem"
      client_cert_file: "$tmpdir/client.pem"
      client_key_file: "$tmpdir/client.key"
EOF

run_cli "register tls context" config add "$context_name" "$context_cfg" --force
context_added=1

if [[ -z "$orig_context_name" ]]; then
    die "original context name is not set"
fi

run_cli "activate tls context" config use "$context_name"

run_cli "validate tls connection" config check

run_cli "restore default context" config use "$orig_context_name"
context_restored=1

run_cli "delete tls context" config delete "$context_name" --yes
context_deleted=1

log_line "Managed server TLS smoke check completed successfully"
