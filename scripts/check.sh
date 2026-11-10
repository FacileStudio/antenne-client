#!/usr/bin/env sh
#
# The repository quality gate, run by the lefthook pre-push hook.
#
#   sh scripts/check.sh    gofmt + vet + race tests (Go), typecheck + bun test (TS), filet
#
# Set TEST_DATABASE_URL to run the outbox tests against a real Postgres; they
# skip loudly without it.

set -eu

cd "$(git rev-parse --show-toplevel)"

status=0

echo "==> go"
(
  cd go
  unformatted="$(gofmt -l .)"
  if [ -n "$unformatted" ]; then
    echo "gofmt: not formatted:"
    echo "$unformatted"
    exit 1
  fi
  go vet ./...
  go test -race ./...
) || status=1

echo "==> ts"
(cd ts && bun run test) || status=1

if command -v filet >/dev/null 2>&1; then
  echo "==> filet"
  filet check || status=1
else
  echo "check: filet not on PATH, skipped" >&2
fi

if [ "$status" -ne 0 ]; then
  echo "check failed"
fi
exit "$status"
