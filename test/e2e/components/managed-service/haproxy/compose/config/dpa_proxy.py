#!/usr/bin/env python3
"""Version-aware reverse proxy for HAProxy Data Plane API.

Forwards every request to the upstream Data Plane API. For mutating
methods (POST, PUT, DELETE, PATCH) it fetches the current configuration
version from ``/v3/services/haproxy/configuration/version`` and injects
``version=<N>`` and ``skip_reload=true`` into the query string unless the
caller already supplied ``version`` or ``transaction_id``.

This keeps HAProxy's strict version/transaction requirement transparent
to declarest's static metadata while preserving all other request
semantics (headers, body, status, response headers).
"""

import http.client
import http.server
import os
import socketserver
import sys
import threading
import urllib.parse

MUTATING_METHODS = {"POST", "PUT", "DELETE", "PATCH"}
HOP_BY_HOP_HEADERS = {
    "connection",
    "keep-alive",
    "proxy-authenticate",
    "proxy-authorization",
    "te",
    "trailers",
    "transfer-encoding",
    "upgrade",
}


class ProxyHandler(http.server.BaseHTTPRequestHandler):
    protocol_version = "HTTP/1.1"
    upstream_host = "127.0.0.1"
    upstream_port = 5556

    def log_message(self, fmt, *args):
        sys.stderr.write("[dpa-proxy] " + (fmt % args) + "\n")

    def do_GET(self):
        self._proxy("GET")

    def do_POST(self):
        self._proxy("POST")

    def do_PUT(self):
        self._proxy("PUT")

    def do_DELETE(self):
        self._proxy("DELETE")

    def do_PATCH(self):
        self._proxy("PATCH")

    def do_HEAD(self):
        self._proxy("HEAD")

    def do_OPTIONS(self):
        self._proxy("OPTIONS")

    def _current_version(self, auth_header):
        try:
            conn = http.client.HTTPConnection(
                self.upstream_host, self.upstream_port, timeout=10
            )
            headers = {"Accept": "application/json"}
            if auth_header:
                headers["Authorization"] = auth_header
            conn.request(
                "GET",
                "/v3/services/haproxy/configuration/version",
                headers=headers,
            )
            resp = conn.getresponse()
            body = resp.read()
            conn.close()
        except OSError:
            return None
        if resp.status != 200:
            return None
        try:
            return int(body.decode().strip())
        except ValueError:
            return None

    def _proxy(self, method):
        parsed = urllib.parse.urlsplit(self.path)
        query_pairs = urllib.parse.parse_qsl(
            parsed.query, keep_blank_values=True
        )
        query_keys = {key for key, _ in query_pairs}

        body_len = int(self.headers.get("Content-Length", "0") or "0")
        body = self.rfile.read(body_len) if body_len else b""

        if (
            method in MUTATING_METHODS
            and "version" not in query_keys
            and "transaction_id" not in query_keys
        ):
            version = self._current_version(self.headers.get("Authorization"))
            if version is not None:
                query_pairs.append(("version", str(version)))
                if "skip_reload" not in query_keys and "force_reload" not in query_keys:
                    query_pairs.append(("skip_reload", "true"))

        new_query = urllib.parse.urlencode(query_pairs, doseq=True)
        new_path = urllib.parse.urlunsplit(
            ("", "", parsed.path, new_query, "")
        )

        forwarded_headers = {}
        for name, value in self.headers.items():
            lname = name.lower()
            if lname in HOP_BY_HOP_HEADERS or lname in ("host", "content-length"):
                continue
            forwarded_headers[name] = value
        if body:
            forwarded_headers["Content-Length"] = str(len(body))

        try:
            conn = http.client.HTTPConnection(
                self.upstream_host, self.upstream_port, timeout=60
            )
            conn.request(
                method,
                new_path,
                body=body if body else None,
                headers=forwarded_headers,
            )
            resp = conn.getresponse()
            resp_body = resp.read()
        except OSError as err:
            conn_error = str(err).encode()
            self.send_response_only(502)
            self.send_header("Content-Type", "text/plain")
            self.send_header("Content-Length", str(len(conn_error)))
            self.send_header("Connection", "close")
            self.end_headers()
            self.wfile.write(conn_error)
            return

        self.send_response_only(resp.status)
        for name, value in resp.getheaders():
            if name.lower() in HOP_BY_HOP_HEADERS or name.lower() == "content-length":
                continue
            self.send_header(name, value)
        self.send_header("Content-Length", str(len(resp_body)))
        self.send_header("Connection", "close")
        self.end_headers()
        self.wfile.write(resp_body)
        conn.close()


class ThreadingHTTPServer(socketserver.ThreadingMixIn, http.server.HTTPServer):
    daemon_threads = True
    allow_reuse_address = True


def main():
    listen_port = int(sys.argv[1]) if len(sys.argv) > 1 else 5555
    upstream_host = sys.argv[2] if len(sys.argv) > 2 else "127.0.0.1"
    upstream_port = int(sys.argv[3]) if len(sys.argv) > 3 else 5556
    ProxyHandler.upstream_host = upstream_host
    ProxyHandler.upstream_port = upstream_port
    server = ThreadingHTTPServer(("0.0.0.0", listen_port), ProxyHandler)
    sys.stderr.write(
        f"[dpa-proxy] listening on 0.0.0.0:{listen_port} -> "
        f"{upstream_host}:{upstream_port}\n"
    )
    sys.stderr.flush()
    server.serve_forever()


if __name__ == "__main__":
    main()
