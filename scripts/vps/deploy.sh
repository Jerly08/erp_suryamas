#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="${1:-$ROOT_DIR/.env.production}"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Env file not found: $ENV_FILE" >&2
  exit 1
fi

cd "$ROOT_DIR"

compose_cmd=(docker compose --env-file "$ENV_FILE" -f docker-compose.prod.yml)

"${compose_cmd[@]}" pull
"${compose_cmd[@]}" up -d --no-build --remove-orphans
"${compose_cmd[@]}" ps
