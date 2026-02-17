#!/usr/bin/env python3
import base64
import glob
import json
import os
import secrets
import ssl
import threading
import time
from http import HTTPStatus
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from socket import socket
from typing import Any
from urllib.parse import parse_qs, unquote, urlsplit


def _env_bool(name: str, default: bool) -> bool:
    raw = os.getenv(name)
    if raw is None or raw.strip() == "":
        return default
    normalized = raw.strip().lower()
    if normalized in ("1", "true", "yes", "on"):
        return True
    if normalized in ("0", "false", "no", "off"):
        return False
    raise ValueError(f"{name} must be true or false")


def _env_positive_int(name: str, default: int) -> int:
    raw = os.getenv(name)
    if raw is None or raw.strip() == "":
        return default
    try:
        value = int(raw)
    except ValueError as err:
        raise ValueError(f"{name} must be a positive integer") from err
    if value <= 0:
        raise ValueError(f"{name} must be a positive integer")
    return value


def _env_first(names: list[str], default: str = "") -> str:
    for name in names:
        raw = os.getenv(name)
        if raw is not None and raw.strip() != "":
            return raw.strip()
    return default


def _env_bool_first(names: list[str], default: bool) -> bool:
    for name in names:
        raw = os.getenv(name)
        if raw is None or raw.strip() == "":
            continue
        return _env_bool(name, default)
    return default


def _env_positive_int_first(names: list[str], default: int) -> int:
    for name in names:
        raw = os.getenv(name)
        if raw is None or raw.strip() == "":
            continue
        return _env_positive_int(name, default)
    return default


try:
    ENABLE_OAUTH2 = _env_bool_first(["ENABLE_OAUTH2", "SIMPLE_API_SERVER_ENABLE_OAUTH2"], True)
    ENABLE_BASIC_AUTH = _env_bool_first(["ENABLE_BASIC_AUTH", "SIMPLE_API_SERVER_ENABLE_BASIC_AUTH"], False)
    ENABLE_MTLS = _env_bool_first(["ENABLE_MTLS", "SIMPLE_API_SERVER_ENABLE_MTLS"], False)
    TOKEN_TTL_SECONDS = _env_positive_int_first(["TOKEN_TTL_SECONDS", "SIMPLE_API_SERVER_TOKEN_TTL_SECONDS"], 3600)
    BIND_HOST = _env_first(["BIND_HOST", "SIMPLE_API_SERVER_BIND_HOST"], "0.0.0.0")
    BIND_PORT = _env_positive_int_first(["BIND_PORT", "SIMPLE_API_SERVER_BIND_PORT"], 8080)
except ValueError as error:
    raise SystemExit(str(error))

CLIENT_ID = _env_first(["CLIENT_ID", "SIMPLE_API_SERVER_CLIENT_ID"], "declarest-e2e-client")
CLIENT_SECRET = _env_first(["CLIENT_SECRET", "SIMPLE_API_SERVER_CLIENT_SECRET"], "declarest-e2e-secret")
BASIC_AUTH_USERNAME = _env_first(["BASIC_AUTH_USERNAME", "SIMPLE_API_SERVER_BASIC_AUTH_USERNAME"], CLIENT_ID)
BASIC_AUTH_PASSWORD = _env_first(["BASIC_AUTH_PASSWORD", "SIMPLE_API_SERVER_BASIC_AUTH_PASSWORD"], CLIENT_SECRET)

CERTS_DIR = _env_first(["CERTS_DIR", "SIMPLE_API_SERVER_CERTS_DIR"], "/etc/simple-api-server/certs")
TLS_CERT_FILE = _env_first(["TLS_CERT_FILE", "SIMPLE_API_SERVER_TLS_CERT_FILE"], f"{CERTS_DIR}/server/server.crt")
TLS_KEY_FILE = _env_first(["TLS_KEY_FILE", "SIMPLE_API_SERVER_TLS_KEY_FILE"], f"{CERTS_DIR}/server/server.key")
MTLS_CLIENT_CERT_DIR = _env_first(
    ["MTLS_CLIENT_CERT_DIR", "SIMPLE_API_SERVER_MTLS_CLIENT_CERT_DIR"],
    f"{CERTS_DIR}/clients/allowed",
)
MTLS_CLIENT_CERT_FILES = _env_first(["MTLS_CLIENT_CERT_FILES", "SIMPLE_API_SERVER_MTLS_CLIENT_CERT_FILES"], "")

STORE_LOCK = threading.Lock()
STORE: dict[str, Any] = {}

TOKENS_LOCK = threading.Lock()
TOKENS: dict[str, float] = {}


def _normalize_path(raw_path: str) -> str:
    path = unquote(urlsplit(raw_path).path or "/")
    if not path.startswith("/"):
        path = "/" + path
    if path != "/":
        path = path.rstrip("/")
    return path or "/"


def _parent_path(path: str) -> str:
    if path == "/":
        return "/"
    parent = path.rsplit("/", 1)[0]
    return parent or "/"


def _json_dumps(payload: Any) -> bytes:
    return json.dumps(payload, separators=(",", ":"), sort_keys=True).encode("utf-8")


def _collect_mtls_client_certs() -> list[str]:
    if MTLS_CLIENT_CERT_FILES.strip() != "":
        entries = [item.strip() for item in MTLS_CLIENT_CERT_FILES.split(",") if item.strip() != ""]
        cert_files = entries
    else:
        cert_files = []
        for pattern in ("*.crt", "*.pem", "*.cer"):
            cert_files.extend(sorted(glob.glob(os.path.join(MTLS_CLIENT_CERT_DIR, pattern))))

    unique_files: list[str] = []
    seen: set[str] = set()
    for cert_file in cert_files:
        if cert_file in seen:
            continue
        seen.add(cert_file)
        if not os.path.isfile(cert_file):
            print(f"simple-api-server mTLS warning: certificate file not found: {cert_file}")
            continue
        unique_files.append(cert_file)

    return unique_files


def _build_mtls_context() -> ssl.SSLContext:
    context = ssl.SSLContext(ssl.PROTOCOL_TLS_SERVER)
    context.minimum_version = ssl.TLSVersion.TLSv1_2
    context.load_cert_chain(certfile=TLS_CERT_FILE, keyfile=TLS_KEY_FILE)

    client_certs = _collect_mtls_client_certs()
    if len(client_certs) == 0:
        print("simple-api-server mTLS notice: no trusted client certificates configured; client access is denied")
    else:
        cert_bundle_parts: list[str] = []
        for cert_file in client_certs:
            with open(cert_file, "r", encoding="utf-8") as file_handle:
                cert_bundle_parts.append(file_handle.read())
        cert_bundle = "\n".join(cert_bundle_parts)
        context.load_verify_locations(cadata=cert_bundle)
    context.verify_mode = ssl.CERT_REQUIRED
    return context


class DynamicMTLSThreadingHTTPServer(ThreadingHTTPServer):
    def get_request(self) -> tuple[socket, Any]:
        request, client_address = super().get_request()
        try:
            tls_context = _build_mtls_context()
            return tls_context.wrap_socket(request, server_side=True), client_address
        except (OSError, ssl.SSLError, ValueError):
            request.close()
            raise


class SimpleAPIHandler(BaseHTTPRequestHandler):
    server_version = "SimpleAPIServer/1.0"
    protocol_version = "HTTP/1.1"

    def __getattr__(self, name: str):
        if name.startswith("do_"):
            return self._handle_request
        raise AttributeError(name)

    def do_GET(self):
        self._handle_request()

    def do_POST(self):
        self._handle_request()

    def do_PUT(self):
        self._handle_request()

    def do_PATCH(self):
        self._handle_request()

    def do_DELETE(self):
        self._handle_request()

    def do_HEAD(self):
        self._handle_request()

    def do_OPTIONS(self):
        self._handle_request()

    def log_message(self, fmt: str, *args: object) -> None:
        print(f"{self.log_date_time_string()} {self.command} {self.path} - {fmt % args}")

    def _write_json(self, status: int, payload: Any, extra_headers: dict[str, str] | None = None) -> None:
        body = _json_dumps(payload)
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        if extra_headers:
            for key, value in extra_headers.items():
                self.send_header(key, value)
        self.end_headers()
        if self.command != "HEAD":
            self.wfile.write(body)

    def _write_empty(self, status: int, extra_headers: dict[str, str] | None = None) -> None:
        self.send_response(status)
        self.send_header("Content-Length", "0")
        if extra_headers:
            for key, value in extra_headers.items():
                self.send_header(key, value)
        self.end_headers()

    def _oauth_error(self, status: int, code: str, description: str, challenge: str | None = None) -> None:
        headers: dict[str, str] = {}
        if challenge:
            headers["WWW-Authenticate"] = challenge
        self._write_json(
            status,
            {"error": code, "error_description": description},
            headers,
        )

    def _read_raw_body(self) -> bytes:
        content_length = self.headers.get("Content-Length", "").strip()
        if content_length == "":
            return b""
        try:
            length = int(content_length)
        except ValueError:
            raise ValueError("invalid Content-Length header")
        if length < 0:
            raise ValueError("invalid Content-Length header")
        if length == 0:
            return b""
        return self.rfile.read(length)

    def _read_json_body(self) -> Any:
        raw = self._read_raw_body()
        if len(raw) == 0:
            return {}
        try:
            return json.loads(raw.decode("utf-8"))
        except (UnicodeDecodeError, json.JSONDecodeError) as err:
            raise ValueError("request body must be valid JSON") from err

    def _parse_basic_auth_header(self) -> tuple[str, str]:
        auth_header = self.headers.get("Authorization", "").strip()
        if not auth_header.lower().startswith("basic "):
            return "", ""

        encoded = auth_header[6:].strip()
        if encoded == "":
            raise ValueError("invalid basic authorization header")

        try:
            decoded = base64.b64decode(encoded, validate=True).decode("utf-8")
        except (ValueError, UnicodeDecodeError):
            raise ValueError("invalid basic authorization header")

        if ":" not in decoded:
            raise ValueError("invalid basic authorization header")

        username, password = decoded.split(":", 1)
        return username, password

    def _parse_client_credentials(self, form_values: dict[str, list[str]]) -> tuple[str, str]:
        form_client_id = form_values.get("client_id", [""])[0]
        form_client_secret = form_values.get("client_secret", [""])[0]

        basic_client_id, basic_client_secret = self._parse_basic_auth_header()

        if basic_client_id:
            if form_client_id and form_client_id != basic_client_id:
                raise ValueError("client_id conflicts between basic auth and form body")
            if form_client_secret and form_client_secret != basic_client_secret:
                raise ValueError("client_secret conflicts between basic auth and form body")
            return basic_client_id, basic_client_secret

        return form_client_id, form_client_secret

    def _validate_bearer_token(self) -> bool:
        auth_header = self.headers.get("Authorization", "").strip()
        if not auth_header.lower().startswith("bearer "):
            self._oauth_error(
                HTTPStatus.UNAUTHORIZED,
                "invalid_token",
                "missing bearer token",
                'Bearer realm="simple-api-server", error="invalid_token"',
            )
            return False

        token = auth_header[7:].strip()
        if token == "":
            self._oauth_error(
                HTTPStatus.UNAUTHORIZED,
                "invalid_token",
                "missing bearer token",
                'Bearer realm="simple-api-server", error="invalid_token"',
            )
            return False

        with TOKENS_LOCK:
            expires_at = TOKENS.get(token)
            now = time.time()
            if expires_at is None or expires_at <= now:
                TOKENS.pop(token, None)
                self._oauth_error(
                    HTTPStatus.UNAUTHORIZED,
                    "invalid_token",
                    "token is invalid or expired",
                    'Bearer realm="simple-api-server", error="invalid_token"',
                )
                return False

        return True

    def _validate_basic_auth(self) -> bool:
        try:
            username, password = self._parse_basic_auth_header()
        except ValueError:
            self._write_json(
                HTTPStatus.UNAUTHORIZED,
                {"error": "invalid_request", "error_description": "invalid basic authorization header"},
                {"WWW-Authenticate": 'Basic realm="simple-api-server"'},
            )
            return False

        if username == "" or password == "":
            self._write_json(
                HTTPStatus.UNAUTHORIZED,
                {"error": "invalid_client", "error_description": "missing basic authorization credentials"},
                {"WWW-Authenticate": 'Basic realm="simple-api-server"'},
            )
            return False

        if username != BASIC_AUTH_USERNAME or password != BASIC_AUTH_PASSWORD:
            self._write_json(
                HTTPStatus.UNAUTHORIZED,
                {"error": "invalid_client", "error_description": "basic authorization credentials are invalid"},
                {"WWW-Authenticate": 'Basic realm="simple-api-server"'},
            )
            return False

        return True

    def _validate_request_security(self) -> bool:
        if not ENABLE_OAUTH2 and not ENABLE_BASIC_AUTH:
            return True

        auth_header = self.headers.get("Authorization", "").strip()
        auth_lower = auth_header.lower()

        if ENABLE_OAUTH2 and ENABLE_BASIC_AUTH:
            if auth_lower.startswith("bearer "):
                return self._validate_bearer_token()
            if auth_lower.startswith("basic "):
                return self._validate_basic_auth()

            self._write_json(
                HTTPStatus.UNAUTHORIZED,
                {
                    "error": "invalid_request",
                    "error_description": "authorization header must use Bearer or Basic scheme",
                },
                {
                    "WWW-Authenticate": 'Bearer realm="simple-api-server", error="invalid_token", '
                    'Basic realm="simple-api-server"'
                },
            )
            return False

        if ENABLE_OAUTH2:
            return self._validate_bearer_token()
        if ENABLE_BASIC_AUTH:
            return self._validate_basic_auth()

        return True

    def _handle_token(self) -> None:
        if self.command != "POST":
            self._write_json(
                HTTPStatus.METHOD_NOT_ALLOWED,
                {"error": "invalid_request", "error_description": "/token only accepts POST"},
                {"Allow": "POST"},
            )
            return

        try:
            raw = self._read_raw_body()
        except ValueError as err:
            self._oauth_error(HTTPStatus.BAD_REQUEST, "invalid_request", str(err))
            return

        try:
            form_values = parse_qs(raw.decode("utf-8"), keep_blank_values=True)
        except UnicodeDecodeError:
            self._oauth_error(HTTPStatus.BAD_REQUEST, "invalid_request", "token request body must be UTF-8 form data")
            return
        grant_type = form_values.get("grant_type", [""])[0]
        if grant_type != "client_credentials":
            self._oauth_error(
                HTTPStatus.BAD_REQUEST,
                "unsupported_grant_type",
                "grant_type must be client_credentials",
            )
            return

        try:
            client_id, client_secret = self._parse_client_credentials(form_values)
        except ValueError as err:
            self._oauth_error(HTTPStatus.BAD_REQUEST, "invalid_request", str(err))
            return

        if client_id != CLIENT_ID or client_secret != CLIENT_SECRET:
            self._oauth_error(
                HTTPStatus.UNAUTHORIZED,
                "invalid_client",
                "client credentials are invalid",
                'Basic realm="simple-api-server"',
            )
            return

        token = secrets.token_urlsafe(32)
        expires_at = time.time() + TOKEN_TTL_SECONDS
        with TOKENS_LOCK:
            TOKENS[token] = expires_at

        response = {
            "access_token": token,
            "token_type": "Bearer",
            "expires_in": TOKEN_TTL_SECONDS,
        }
        scope = form_values.get("scope", [""])[0]
        if scope:
            response["scope"] = scope

        self._write_json(HTTPStatus.OK, response, {"Cache-Control": "no-store", "Pragma": "no-cache"})

    def _handle_get(self, normalized_path: str) -> None:
        if normalized_path == "/health":
            self._write_json(HTTPStatus.OK, {"status": "ok"})
            return

        with STORE_LOCK:
            if normalized_path in STORE:
                self._write_json(HTTPStatus.OK, STORE[normalized_path])
                return

            items = [
                STORE[path]
                for path in sorted(STORE.keys())
                if _parent_path(path) == normalized_path
            ]

        if len(items) == 0:
            self._write_json(
                HTTPStatus.NOT_FOUND,
                {
                    "error": "not_found",
                    "error_description": f"resource path not found: {normalized_path}",
                },
            )
            return

        self._write_json(HTTPStatus.OK, items)

    def _handle_delete(self, normalized_path: str) -> None:
        with STORE_LOCK:
            existed = normalized_path in STORE
            if existed:
                del STORE[normalized_path]

        if not existed:
            self._write_json(
                HTTPStatus.NOT_FOUND,
                {
                    "error": "not_found",
                    "error_description": f"resource path not found: {normalized_path}",
                },
            )
            return

        self._write_empty(HTTPStatus.NO_CONTENT)

    def _handle_write(self, normalized_path: str) -> None:
        try:
            payload = self._read_json_body()
        except ValueError as err:
            self._write_json(HTTPStatus.BAD_REQUEST, {"error": "invalid_request", "error_description": str(err)})
            return

        with STORE_LOCK:
            existed = normalized_path in STORE
            STORE[normalized_path] = payload

        if self.command == "POST" and not existed:
            self._write_json(HTTPStatus.CREATED, payload)
            return

        self._write_json(HTTPStatus.OK, payload)

    def _handle_options(self) -> None:
        self._write_empty(
            HTTPStatus.NO_CONTENT,
            {"Allow": "GET,HEAD,POST,PUT,PATCH,DELETE,OPTIONS"},
        )

    def _handle_request(self) -> None:
        normalized_path = _normalize_path(self.path)

        if normalized_path == "/token":
            if not ENABLE_OAUTH2:
                self._write_json(
                    HTTPStatus.NOT_FOUND,
                    {"error": "not_found", "error_description": "oauth2 token endpoint is disabled"},
                )
                return
            self._handle_token()
            return

        if not self._validate_request_security():
            return

        if self.command in ("GET", "HEAD"):
            self._handle_get(normalized_path)
            return
        if self.command == "DELETE":
            self._handle_delete(normalized_path)
            return
        if self.command == "OPTIONS":
            self._handle_options()
            return

        self._handle_write(normalized_path)


def run() -> None:
    scheme = "http"
    if ENABLE_MTLS:
        try:
            _build_mtls_context()
        except (OSError, ValueError, ssl.SSLError) as error:
            raise SystemExit(f"failed to configure mTLS: {error}") from error
        server: ThreadingHTTPServer = DynamicMTLSThreadingHTTPServer((BIND_HOST, BIND_PORT), SimpleAPIHandler)
        scheme = "https"
    else:
        server = ThreadingHTTPServer((BIND_HOST, BIND_PORT), SimpleAPIHandler)

    print(
        "simple-api-server listening on "
        f"{scheme}://{BIND_HOST}:{BIND_PORT} "
        f"(basic-auth={'enabled' if ENABLE_BASIC_AUTH else 'disabled'}, "
        f"oauth2={'enabled' if ENABLE_OAUTH2 else 'disabled'}, "
        f"mtls={'enabled' if ENABLE_MTLS else 'disabled'})"
    )
    server.serve_forever()


if __name__ == "__main__":
    run()
