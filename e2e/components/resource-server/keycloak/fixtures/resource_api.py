#!/usr/bin/env python3
import json
import os
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from urllib.parse import urlparse

PORT = int(os.environ.get("RESOURCE_API_PORT", "8080"))
REQUIRE_AUTH = os.environ.get("RESOURCE_API_REQUIRE_AUTH", "true").lower() == "true"

STORE = {}


def parse_json(handler):
    length = int(handler.headers.get("Content-Length", "0"))
    if length <= 0:
        return {}
    raw = handler.rfile.read(length)
    if not raw:
        return {}
    return json.loads(raw.decode("utf-8"))


class Handler(BaseHTTPRequestHandler):
    def _auth_ok(self):
        if not REQUIRE_AUTH:
            return True
        header = self.headers.get("Authorization", "")
        return header.startswith("Bearer ") and len(header) > len("Bearer ")

    def _send(self, status, payload=None):
        body = b""
        if payload is not None:
            body = json.dumps(payload).encode("utf-8")
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(body)))
        self.end_headers()
        if body:
            self.wfile.write(body)

    def _path_parts(self):
        parsed = urlparse(self.path)
        return [p for p in parsed.path.split("/") if p], parsed

    def _ensure_auth(self):
        if self.path == "/healthz":
            return True
        if self._auth_ok():
            return True
        self._send(401, {"error": "missing bearer token"})
        return False

    def do_GET(self):
        parts, _ = self._path_parts()

        if self.path == "/healthz":
            self._send(200, {"status": "ok"})
            return

        if not self._ensure_auth():
            return

        if parts == ["customers"]:
            items = [STORE[k] for k in sorted(STORE.keys())]
            self._send(200, items)
            return

        if len(parts) == 2 and parts[0] == "customers":
            customer_id = parts[1]
            item = STORE.get(customer_id)
            if item is None:
                self._send(404, {"error": "not found"})
                return
            self._send(200, item)
            return

        self._send(404, {"error": "not found"})

    def do_POST(self):
        parts, _ = self._path_parts()
        if not self._ensure_auth():
            return

        if parts != ["customers"]:
            self._send(404, {"error": "not found"})
            return

        payload = parse_json(self)
        customer_id = str(payload.get("id") or len(STORE) + 1)
        payload["id"] = customer_id
        if "alias" not in payload:
            payload["alias"] = customer_id
        STORE[customer_id] = payload
        self._send(200, payload)

    def do_PUT(self):
        parts, _ = self._path_parts()
        if not self._ensure_auth():
            return

        if len(parts) != 2 or parts[0] != "customers":
            self._send(404, {"error": "not found"})
            return

        customer_id = parts[1]
        if customer_id not in STORE:
            self._send(404, {"error": "not found"})
            return

        payload = parse_json(self)
        payload["id"] = customer_id
        if "alias" not in payload:
            payload["alias"] = STORE[customer_id].get("alias", customer_id)
        STORE[customer_id] = payload
        self._send(200, payload)

    def do_DELETE(self):
        parts, _ = self._path_parts()
        if not self._ensure_auth():
            return

        if len(parts) != 2 or parts[0] != "customers":
            self._send(404, {"error": "not found"})
            return

        customer_id = parts[1]
        STORE.pop(customer_id, None)
        self._send(204, None)

    def log_message(self, fmt, *args):
        return


if __name__ == "__main__":
    server = ThreadingHTTPServer(("0.0.0.0", PORT), Handler)
    server.serve_forever()
