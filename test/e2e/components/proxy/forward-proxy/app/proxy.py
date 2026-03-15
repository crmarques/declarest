import base64
import os
import select
import socket
import socketserver
import threading
import time
import urllib.error
import urllib.request
from http.server import BaseHTTPRequestHandler, HTTPServer


ACCESS_LOG = os.environ.get("FORWARD_PROXY_ACCESS_LOG", "")
AUTH_MODE = os.environ.get("FORWARD_PROXY_AUTH_MODE", "none").strip().lower()
AUTH_USERNAME = os.environ.get("FORWARD_PROXY_AUTH_USERNAME", "")
AUTH_PASSWORD = os.environ.get("FORWARD_PROXY_AUTH_PASSWORD", "")
HOP_BY_HOP = {
    "connection",
    "keep-alive",
    "proxy-authenticate",
    "proxy-authorization",
    "te",
    "trailer",
    "transfer-encoding",
    "upgrade",
}
LOG_LOCK = threading.Lock()


def write_log(message: str) -> None:
    if not ACCESS_LOG:
        return
    os.makedirs(os.path.dirname(ACCESS_LOG), exist_ok=True)
    with LOG_LOCK:
        with open(ACCESS_LOG, "a", encoding="utf-8") as handle:
            handle.write(message + "\n")


def build_target_url(handler: BaseHTTPRequestHandler) -> str:
    if handler.path.startswith("http://") or handler.path.startswith("https://"):
        return handler.path

    host = handler.headers.get("Host", "").strip()
    if not host:
        raise ValueError("missing Host header")
    return f"http://{host}{handler.path}"


class ProxyHandler(BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    server_version = "DeclarestE2EProxy/1.0"

    def log_message(self, format: str, *args) -> None:
        return

    def do_CONNECT(self) -> None:
        started = time.time()
        target = self.path
        if target == "__health" or target == "/__health":
            self._send_health()
            return
        if not self._authorize(target):
            return
        host, port = self._connect_target(target)
        status = 502
        try:
            upstream = socket.create_connection((host, port), timeout=30)
            self.send_response(200, "Connection Established")
            self.send_header("Connection", "close")
            self.end_headers()
            self.close_connection = True
            self._relay(upstream)
            status = 200
        except OSError as exc:
            body = f"proxy CONNECT failed: {exc}\n".encode("utf-8")
            self.send_response(502, "Bad Gateway")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
        finally:
            write_log(self._log_line(target, status, started))

    def do_DELETE(self) -> None:
        self._handle_http()

    def do_GET(self) -> None:
        self._handle_http()

    def do_HEAD(self) -> None:
        self._handle_http()

    def do_OPTIONS(self) -> None:
        self._handle_http()

    def do_PATCH(self) -> None:
        self._handle_http()

    def do_POST(self) -> None:
        self._handle_http()

    def do_PUT(self) -> None:
        self._handle_http()

    def _handle_http(self) -> None:
        started = time.time()
        target = self.path
        if target == "/__health":
            self._send_health()
            write_log(self._log_line(target, 204, started))
            return
        if not self._authorize(target):
            return

        try:
            target = build_target_url(self)
        except ValueError as exc:
            body = f"{exc}\n".encode("utf-8")
            self.send_response(400, "Bad Request")
            self.send_header("Content-Length", str(len(body)))
            self.end_headers()
            self.wfile.write(body)
            write_log(self._log_line(self.path, 400, started))
            return

        content_length = int(self.headers.get("Content-Length", "0") or "0")
        body = self.rfile.read(content_length) if content_length > 0 else None
        headers = {
            key: value
            for key, value in self.headers.items()
            if key.lower() not in HOP_BY_HOP
        }
        request = urllib.request.Request(target, data=body, headers=headers, method=self.command)
        opener = urllib.request.build_opener(urllib.request.ProxyHandler({}))

        try:
            with opener.open(request, timeout=30) as response:
                payload = response.read()
                self._write_response(response.status, response.reason, response.headers.items(), payload)
                write_log(self._log_line(target, response.status, started))
        except urllib.error.HTTPError as exc:
            payload = exc.read()
            self._write_response(exc.code, exc.reason, exc.headers.items(), payload)
            write_log(self._log_line(target, exc.code, started))
        except Exception as exc:
            payload = f"proxy request failed: {exc}\n".encode("utf-8")
            self.send_response(502, "Bad Gateway")
            self.send_header("Content-Length", str(len(payload)))
            self.end_headers()
            self.wfile.write(payload)
            write_log(self._log_line(target, 502, started))

    def _authorize(self, target: str) -> bool:
        if AUTH_MODE != "basic":
            return True

        header = self.headers.get("Proxy-Authorization", "")
        if not header.startswith("Basic "):
            self._proxy_auth_required(target)
            return False

        try:
            decoded = base64.b64decode(header.split(" ", 1)[1], validate=True).decode("utf-8")
        except Exception:
            self._proxy_auth_required(target)
            return False

        username, _, password = decoded.partition(":")
        if username != AUTH_USERNAME or password != AUTH_PASSWORD:
            self._proxy_auth_required(target)
            return False
        return True

    def _proxy_auth_required(self, target: str) -> None:
        started = time.time()
        payload = b"proxy authentication required\n"
        self.send_response(407, "Proxy Authentication Required")
        self.send_header("Proxy-Authenticate", 'Basic realm="Declarest E2E Proxy"')
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        self.wfile.write(payload)
        write_log(self._log_line(target, 407, started))

    def _send_health(self) -> None:
        self.send_response(204, "No Content")
        self.send_header("Content-Length", "0")
        self.end_headers()

    def _connect_target(self, target: str) -> tuple[str, int]:
        host, separator, port = target.partition(":")
        if not separator:
            return host, 443
        return host, int(port)

    def _relay(self, upstream: socket.socket) -> None:
        with upstream:
            upstream.setblocking(False)
            self.connection.setblocking(False)
            sockets = [self.connection, upstream]
            while True:
                readable, _, exceptional = select.select(sockets, [], sockets, 1.0)
                if exceptional:
                    return
                if not readable:
                    continue
                for source in readable:
                    data = source.recv(8192)
                    if not data:
                        return
                    target = upstream if source is self.connection else self.connection
                    target.sendall(data)

    def _write_response(self, status: int, reason: str, headers, payload: bytes) -> None:
        self.send_response(status, reason)
        for key, value in headers:
            if key.lower() in HOP_BY_HOP or key.lower() == "content-length":
                continue
            self.send_header(key, value)
        self.send_header("Content-Length", str(len(payload)))
        self.end_headers()
        if self.command != "HEAD" and payload:
            self.wfile.write(payload)

    def _log_line(self, target: str, status: int, started: float) -> str:
        elapsed_ms = int((time.time() - started) * 1000)
        return f"{time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())} method={self.command} target={target} status={status} elapsed_ms={elapsed_ms}"


class ThreadedHTTPServer(socketserver.ThreadingMixIn, HTTPServer):
    daemon_threads = True


def main() -> None:
    bind_host = os.environ.get("FORWARD_PROXY_BIND_HOST", "0.0.0.0")
    bind_port = int(os.environ.get("FORWARD_PROXY_BIND_PORT", "3128"))
    server = ThreadedHTTPServer((bind_host, bind_port), ProxyHandler)
    write_log(
        f"{time.strftime('%Y-%m-%dT%H:%M:%SZ', time.gmtime())} event=start bind={bind_host}:{bind_port} auth_mode={AUTH_MODE or 'none'}"
    )
    server.serve_forever()


if __name__ == "__main__":
    main()
