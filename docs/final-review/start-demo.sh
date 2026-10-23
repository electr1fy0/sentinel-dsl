#!/bin/sh
set -eu
ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$ROOT"
if [ "${1:-}" = "api" ]; then
  exec go run ./cmd/sentinel serve docs/final-review/fixtures/review-policy.sntl 127.0.0.1:8080
elif [ "${1:-}" = "web" ]; then
  cd web
  exec ./node_modules/.bin/vite dev --port 3000 --host 127.0.0.1
else
  printf 'Run in two terminals from the project directory:\n\n'
  printf '  sh docs/final-review/start-demo.sh api\n'
  printf '  sh docs/final-review/start-demo.sh web\n\n'
  printf 'Then open http://localhost:3000\n'
fi
