#!/usr/bin/env bash
# Run Spike A against Caddy in Docker (host Go harness → container admin API).
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
COMPOSE_DIR="$ROOT/docker/spike-caddy"
LAST_GOOD="${TMPDIR:-/tmp}/webor-caddy-last-good.json"
ADMIN="${ADMIN:-http://127.0.0.1:2019}"
PROBE="${PROBE:-http://127.0.0.1:8080/}"

cd "$COMPOSE_DIR"
echo "==> starting Caddy"
docker compose up -d --pull missing
trap 'docker compose down >/dev/null 2>&1 || true' EXIT

echo "==> waiting for admin API"
for _ in $(seq 1 30); do
  if curl -sf "$ADMIN/config/" >/dev/null; then
    break
  fi
  sleep 0.5
done
curl -sf "$ADMIN/config/" >/dev/null

echo "==> probe baseline"
curl -sf "$PROBE" | grep -q webor-spike-a-ok

echo "==> apply good config"
(cd "$ROOT" && go run ./cmd/spike-caddy \
  -admin "$ADMIN" \
  -config docker/spike-caddy/good.json \
  -probe "$PROBE" \
  -last-good "$LAST_GOOD")

echo "==> apply bad config (expect failure)"
set +e
(cd "$ROOT" && go run ./cmd/spike-caddy \
  -admin "$ADMIN" \
  -config docker/spike-caddy/bad.json \
  -probe "$PROBE" \
  -last-good "$LAST_GOOD")
bad_status=$?
set -e
if [[ "$bad_status" -eq 0 ]]; then
  echo "expected bad config apply to fail" >&2
  exit 1
fi

echo "==> site still serving good response"
curl -sf "$PROBE" | grep -q webor-spike-a-ok

echo "OK: Spike A docker smoke test passed"
