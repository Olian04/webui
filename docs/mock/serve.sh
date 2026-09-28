#!/bin/bash
# Static server with an artificial delay on HTML documents.
# The delay is the wait before the next page, so the .scroll dim is visible.
# Assets are not delayed.
#
#   ./serve.sh                     # ., default delay, port 8000
#   ./serve.sh public              # serve public/
#   ./serve.sh . 0                 # no delay
#   ./serve.sh . 1500 8000         # dir, delay ms, then port

DIR="${1:-.}"
DELAY_MS="${2:-200}"
PORT="${3:-8000}"

cd "$DIR" || exit 1

exec python - "$DELAY_MS" "$PORT" <<'PY'
import sys, time
from http.server import ThreadingHTTPServer, SimpleHTTPRequestHandler

delay = float(sys.argv[1]) / 1000
port = int(sys.argv[2])

class Handler(SimpleHTTPRequestHandler):
    def send_head(self):
        path = self.path.split("?", 1)[0]
        if delay and (path.endswith(".html") or path.endswith("/")):
            time.sleep(delay)
        return super().send_head()

    def end_headers(self):
        path = self.path.split("?", 1)[0]
        if path.endswith(".html") or path.endswith("/"):
            self.send_header("Cache-Control", "no-store")
        super().end_headers()

class Server(ThreadingHTTPServer):
    daemon_threads = True
    block_on_close = False

print(f"Serving HTTP on port {port} (html delay {sys.argv[1]}ms)", flush=True)
Server.allow_reuse_address = True
server = Server(("", port), Handler)
try:
    server.serve_forever()
except KeyboardInterrupt:
    server.server_close()
PY
