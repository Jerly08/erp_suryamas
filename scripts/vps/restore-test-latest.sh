#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="${1:-$ROOT_DIR/.env.production}"
BACKUP_ROOT="${BACKUP_ROOT:-$ROOT_DIR/backups/daily}"
RESTORE_TEST_IMAGE="${RESTORE_TEST_IMAGE:-postgres:15-alpine}"
RESTORE_TEST_DB="${RESTORE_TEST_DB:-restore_test}"
RESTORE_TEST_USER="${RESTORE_TEST_USER:-restore_user}"
RESTORE_TEST_PASSWORD="${RESTORE_TEST_PASSWORD:-restore_test_$(date +%s)_$RANDOM}"
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
CONTAINER_NAME="erp_restore_test_${TIMESTAMP}_$$"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Env file not found: $ENV_FILE" >&2
  exit 1
fi

set -a
source "$ENV_FILE"
set +a

DB_BACKUP_DIR="$BACKUP_ROOT/database"
UPLOADS_BACKUP_DIR="$BACKUP_ROOT/uploads"
RESTORE_TEST_DIR="$BACKUP_ROOT/restore-tests"
RESTORE_TEST_REPORT="$RESTORE_TEST_DIR/restore_test_${TIMESTAMP}.txt"

mkdir -p "$RESTORE_TEST_DIR"

latest_database_backup="$(ls -1t "$DB_BACKUP_DIR"/*.dump 2>/dev/null | head -n 1 || true)"
latest_uploads_backup="$(ls -1t "$UPLOADS_BACKUP_DIR"/*.tar.gz 2>/dev/null | head -n 1 || true)"

if [[ -z "$latest_database_backup" ]]; then
  echo "No database backup found in $DB_BACKUP_DIR" >&2
  exit 1
fi

if [[ -z "$latest_uploads_backup" ]]; then
  echo "No uploads backup found in $UPLOADS_BACKUP_DIR" >&2
  exit 1
fi

cleanup() {
  docker rm -f "$CONTAINER_NAME" >/dev/null 2>&1 || true
}
trap cleanup EXIT

echo "[$(date --iso-8601=seconds)] Starting restore test"
echo "Database backup: $latest_database_backup"
echo "Uploads backup: $latest_uploads_backup"

tar -tzf "$latest_uploads_backup" >/dev/null
uploads_entry_count="$(tar -tzf "$latest_uploads_backup" | wc -l | tr -d ' ')"

docker run -d --rm \
  --name "$CONTAINER_NAME" \
  -e POSTGRES_USER="$RESTORE_TEST_USER" \
  -e POSTGRES_PASSWORD="$RESTORE_TEST_PASSWORD" \
  -e POSTGRES_DB="$RESTORE_TEST_DB" \
  -v "$latest_database_backup:/backup/latest.dump:ro" \
  "$RESTORE_TEST_IMAGE" >/dev/null

for _ in $(seq 1 30); do
  if docker exec "$CONTAINER_NAME" pg_isready -U "$RESTORE_TEST_USER" -d "$RESTORE_TEST_DB" >/dev/null 2>&1; then
    break
  fi
  sleep 2
done

if ! docker exec "$CONTAINER_NAME" pg_isready -U "$RESTORE_TEST_USER" -d "$RESTORE_TEST_DB" >/dev/null 2>&1; then
  echo "Temporary restore database did not become ready" >&2
  exit 1
fi

docker exec -e PGPASSWORD="$RESTORE_TEST_PASSWORD" "$CONTAINER_NAME" \
  pg_restore --list /backup/latest.dump >/dev/null

docker exec -e PGPASSWORD="$RESTORE_TEST_PASSWORD" "$CONTAINER_NAME" \
  pg_restore -U "$RESTORE_TEST_USER" -d "$RESTORE_TEST_DB" \
  --no-owner --no-privileges /backup/latest.dump

restored_table_count="$(docker exec -e PGPASSWORD="$RESTORE_TEST_PASSWORD" "$CONTAINER_NAME" \
  psql -U "$RESTORE_TEST_USER" -d "$RESTORE_TEST_DB" -tAc \
  "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public';" | tr -d '[:space:]')"

if [[ -z "$restored_table_count" || "$restored_table_count" -le 0 ]]; then
  echo "Restore test failed: restored database has no public tables" >&2
  exit 1
fi

{
  echo "ERP Akuntansi Restore Test"
  echo "Timestamp: $TIMESTAMP"
  echo "Created at: $(date --iso-8601=seconds)"
  echo "Env file: $ENV_FILE"
  echo "Database backup tested: $latest_database_backup"
  echo "Uploads backup tested: $latest_uploads_backup"
  echo "Restore image: $RESTORE_TEST_IMAGE"
  echo "Restored public table count: $restored_table_count"
  echo "Uploads archive entry count: $uploads_entry_count"
  echo
  echo "File sizes:"
  ls -lh "$latest_database_backup" "$latest_uploads_backup"
  echo
  echo "Result: PASS"
} > "$RESTORE_TEST_REPORT"

echo "Restore test passed"
echo "Restored public table count: $restored_table_count"
echo "Uploads archive entry count: $uploads_entry_count"
echo "Report created: $RESTORE_TEST_REPORT"
