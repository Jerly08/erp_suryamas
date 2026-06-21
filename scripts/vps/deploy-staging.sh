#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="${1:-$ROOT_DIR/.env.staging}"
COMPOSE_FILE="$ROOT_DIR/docker-compose.yml"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Env file not found: $ENV_FILE" >&2
  exit 1
fi

cd "$ROOT_DIR"

set -a
# shellcheck disable=SC1090
source "$ENV_FILE"
set +a

derive_domain_from_url() {
  local url="${1:-}"
  local host

  [[ -z "$url" ]] && return 1

  host="${url#http://}"
  host="${host#https://}"
  host="${host%%/*}"
  host="${host%%:*}"

  case "$host" in
    ""|"localhost"|"127.0.0.1"|"SERVER_IP")
      return 1
      ;;
  esac

  printf '%s\n' "$host"
}

normalize_bool() {
  case "${1:-}" in
    1|true|TRUE|yes|YES|y|Y|on|ON)
      printf 'true\n'
      ;;
    *)
      printf 'false\n'
      ;;
  esac
}

prepend_origin_if_missing() {
  local origin="$1"
  local origins="${2:-}"

  if [[ -z "$origins" ]]; then
    printf '%s\n' "$origin"
    return 0
  fi

  case ",$origins," in
    *",$origin,"*)
      printf '%s\n' "$origins"
      ;;
    *)
      printf '%s,%s\n' "$origin" "$origins"
      ;;
  esac
}

DOMAIN="${DOMAIN:-${STAGING_DOMAIN:-}}"
if [[ -z "$DOMAIN" ]]; then
  DOMAIN="$(derive_domain_from_url "${FRONTEND_URL:-}" || true)"
fi

STAGING_PROXY_ENABLED="$(normalize_bool "${STAGING_PROXY_ENABLED:-true}")"
COMPOSE_REMOVE_ORPHANS="$(normalize_bool "${COMPOSE_REMOVE_ORPHANS:-false}")"

compose_cmd=(docker compose --env-file "$ENV_FILE" -f "$COMPOSE_FILE")
up_args=(up -d --build)

if [[ "$COMPOSE_REMOVE_ORPHANS" == "true" ]]; then
  up_args+=(--remove-orphans)
fi

if [[ "$STAGING_PROXY_ENABLED" == "true" && -n "$DOMAIN" ]]; then
  export DOMAIN
  export ACME_EMAIL="${ACME_EMAIL:-admin@$DOMAIN}"
  export CLIENT_MAX_BODY_SIZE="${CLIENT_MAX_BODY_SIZE:-50MB}"

  if [[ -z "${FRONTEND_URL:-}" || "${FRONTEND_URL:-}" == *":3003"* || "${FRONTEND_URL:-}" == *"SERVER_IP"* ]]; then
    export FRONTEND_URL="https://$DOMAIN"
  fi

  export ALLOWED_ORIGINS="$(prepend_origin_if_missing "https://$DOMAIN" "${ALLOWED_ORIGINS:-}")"
  export ENABLE_HTTPS="${ENABLE_HTTPS:-true}"
  export COOKIE_SECURE="${COOKIE_SECURE:-true}"
  export COOKIE_HTTP_ONLY="${COOKIE_HTTP_ONLY:-true}"
  export COOKIE_SAME_SITE="${COOKIE_SAME_SITE:-lax}"

  echo "Deploying staging with Caddy HTTPS proxy: https://$DOMAIN"
  compose_cmd+=(--profile proxy)
else
  echo "Deploying staging without Caddy proxy."
  echo "Set DOMAIN=staging.example.com in $ENV_FILE to enable HTTPS proxy."
fi

"${compose_cmd[@]}" "${up_args[@]}"
"${compose_cmd[@]}" ps
