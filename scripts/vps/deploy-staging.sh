#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="${1:-$ROOT_DIR/.env.staging}"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Env file not found: $ENV_FILE" >&2
  exit 1
fi

cd "$ROOT_DIR"

docker compose --env-file "$ENV_FILE" -f docker-compose.yml up -d --build --remove-orphans
docker compose --env-file "$ENV_FILE" -f docker-compose.yml ps
