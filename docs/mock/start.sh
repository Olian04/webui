#! /bin/bash
# Start the server and build the site.
#
#   ./start.sh            # default delay, port 8000
#   ./start.sh 0          # no delay
#   ./start.sh 1500 8000  # delay ms, then port

DELAY_MS="${1:-200}"
PORT="${2:-8000}"

node ./build.mjs

echo "Copying assets to public directory"
cp -r ./assets ./public/assets

./serve.sh ./public "$DELAY_MS" "$PORT"