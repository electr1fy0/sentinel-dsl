#!/bin/sh
set -eu
ROOT=$(CDPATH='' cd -- "$(dirname -- "$0")/../.." && pwd)
cd "$ROOT"
DEMO_BIN=$(mktemp "${TMPDIR:-/tmp}/sentinel-demo.XXXXXX")
trap 'rm -f "$DEMO_BIN"' EXIT HUP INT TERM
go build -o "$DEMO_BIN" ./cmd/sentinel
FIXTURES=docs/final-review/fixtures
expect_error() {
  printf '\nExpected error: %s\n' "$1"
  if "$DEMO_BIN" check "$FIXTURES/$1.sntl"; then
    printf 'FAIL: invalid policy was accepted\n' >&2
    exit 1
  fi
}
printf 'SentinelDSL final review | Ayush Pandey | 24BIT0394\n'
expect_error syntax-error
printf '\nCorrected syntax\n'
"$DEMO_BIN" check "$FIXTURES/syntax-recovered.sntl"
expect_error semantic-error
printf '\nCorrected semantics\n'
"$DEMO_BIN" check "$FIXTURES/semantic-recovered.sntl"
printf '\nExpected regex error\n'
if "$DEMO_BIN" automata build '(a|b'; then
  printf 'FAIL: unclosed group was accepted\n' >&2
  exit 1
fi
printf '\nCorrected regex and minimization\n'
"$DEMO_BIN" automata build '(a|b)*'
"$DEMO_BIN" automata minimize '(a|b)*'
"$DEMO_BIN" automata test '(a|b)*' abba
for request in allowed denied default-denied; do
  printf '\nDecision trace: %s\n' "$request"
  "$DEMO_BIN" explain "$FIXTURES/review-policy.sntl" "$FIXTURES/$request.json"
done
printf '\nVerify the browser API handler in-process (no network needed)\n'
go run ./docs/final-review/capture
printf '\nAll demonstration commands completed.\n'
