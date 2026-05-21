#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
ENV_FILE="${1:-$ROOT_DIR/.env.production}"
BACKUP_ROOT="${BACKUP_ROOT:-$ROOT_DIR/backups/daily}"
BACKUP_RETENTION_DAYS="${BACKUP_RETENTION_DAYS:-30}"
TIMESTAMP="$(date +%Y%m%d_%H%M%S)"
CURRENT_STEP="initializing"

if [[ ! -f "$ENV_FILE" ]]; then
  echo "Env file not found: $ENV_FILE" >&2
  exit 1
fi

set -a
source "$ENV_FILE"
set +a

: "${DB_USER:?DB_USER is required in env file}"
: "${DB_NAME:?DB_NAME is required in env file}"

DB_BACKUP_DIR="$BACKUP_ROOT/database"
UPLOADS_BACKUP_DIR="$BACKUP_ROOT/uploads"
MANIFEST_DIR="$BACKUP_ROOT/manifests"
FAILURE_DIR="$BACKUP_ROOT/failures"

mkdir -p "$DB_BACKUP_DIR" "$UPLOADS_BACKUP_DIR" "$MANIFEST_DIR" "$FAILURE_DIR"

DB_BACKUP_FILE="$DB_BACKUP_DIR/${DB_NAME}_${TIMESTAMP}.dump"
UPLOADS_BACKUP_FILE="$UPLOADS_BACKUP_DIR/uploads_${TIMESTAMP}.tar.gz"
MANIFEST_FILE="$MANIFEST_DIR/backup_${TIMESTAMP}.txt"
FAILURE_REPORT_FILE="$FAILURE_DIR/backup_failed_${TIMESTAMP}.txt"
DB_TMP_FILE="${DB_BACKUP_FILE}.tmp"
UPLOADS_TMP_FILE="${UPLOADS_BACKUP_FILE}.tmp"

cleanup_tmp_files() {
  rm -f "$DB_TMP_FILE" "$UPLOADS_TMP_FILE"
}

send_backup_email() {
  local subject="$1"
  local body_file="$2"

  if [[ -z "${BACKUP_NOTIFY_EMAIL_TO:-}" ]]; then
    return 0
  fi

  local from_address="${BACKUP_NOTIFY_EMAIL_FROM:-erp-backup@$(hostname -f 2>/dev/null || hostname)}"
  local subject_prefix="${BACKUP_NOTIFY_SUBJECT_PREFIX:-[ERP Backup]}"

  if command -v sendmail >/dev/null 2>&1; then
    {
      echo "To: ${BACKUP_NOTIFY_EMAIL_TO}"
      echo "From: ${from_address}"
      echo "Subject: ${subject_prefix} ${subject}"
      echo "Content-Type: text/plain; charset=UTF-8"
      echo
      cat "$body_file"
    } | sendmail -t
    return 0
  fi

  if command -v mail >/dev/null 2>&1; then
    mail -s "${subject_prefix} ${subject}" -r "$from_address" "$BACKUP_NOTIFY_EMAIL_TO" < "$body_file"
    return 0
  fi

  echo "Email notification skipped: install sendmail-compatible MTA or mail command" >&2
  return 0
}

handle_exit() {
  local exit_status=$?
  cleanup_tmp_files

  if [[ "$exit_status" -ne 0 ]]; then
    {
      echo "ERP Akuntansi Daily Local Backup"
      echo "Result: FAILED"
      echo "Timestamp: $TIMESTAMP"
      echo "Failed at: $(date --iso-8601=seconds)"
      echo "Step: $CURRENT_STEP"
      echo "Env file: $ENV_FILE"
      echo "Backup root: $BACKUP_ROOT"
      echo
      echo "Please check the cron log, Docker containers, disk space, and rclone/email configuration."
    } > "$FAILURE_REPORT_FILE" || true

    if [[ "${BACKUP_NOTIFY_ON_FAILURE:-true}" == "true" ]]; then
      send_backup_email "FAILED - $(hostname)" "$FAILURE_REPORT_FILE" || true
    fi
  fi

  exit "$exit_status"
}
trap handle_exit EXIT

cd "$ROOT_DIR"

echo "[$(date --iso-8601=seconds)] Starting daily local backup"
echo "Using env file: $ENV_FILE"
echo "Backup root: $BACKUP_ROOT"

CURRENT_STEP="dumping PostgreSQL database"
docker compose --env-file "$ENV_FILE" -f docker-compose.prod.yml exec -T db \
  pg_dump -U "$DB_USER" -d "$DB_NAME" -Fc > "$DB_TMP_FILE"
mv "$DB_TMP_FILE" "$DB_BACKUP_FILE"

CURRENT_STEP="archiving uploads"
docker compose --env-file "$ENV_FILE" -f docker-compose.prod.yml exec -T backend \
  tar -C /app -czf - uploads > "$UPLOADS_TMP_FILE"
mv "$UPLOADS_TMP_FILE" "$UPLOADS_BACKUP_FILE"

CURRENT_STEP="writing local manifest"
{
  echo "ERP Akuntansi Daily Local Backup"
  echo "Timestamp: $TIMESTAMP"
  echo "Created at: $(date --iso-8601=seconds)"
  echo "Env file: $ENV_FILE"
  echo "Database backup: $DB_BACKUP_FILE"
  echo "Uploads backup: $UPLOADS_BACKUP_FILE"
  echo "Retention days: $BACKUP_RETENTION_DAYS"
  echo
  echo "File sizes:"
  ls -lh "$DB_BACKUP_FILE" "$UPLOADS_BACKUP_FILE"
  echo
  if command -v sha256sum >/dev/null 2>&1; then
    echo "SHA256:"
    sha256sum "$DB_BACKUP_FILE" "$UPLOADS_BACKUP_FILE"
  else
    echo "SHA256: sha256sum command not available"
  fi
} > "$MANIFEST_FILE"

OFFSITE_STATUS="disabled"
OFFSITE_TARGET="${BACKUP_OFFSITE_TARGET:-}"
OFFSITE_PATH=""
if [[ -n "$OFFSITE_TARGET" ]]; then
  CURRENT_STEP="uploading backup to offsite target"
  if ! command -v rclone >/dev/null 2>&1; then
    echo "rclone is required when BACKUP_OFFSITE_TARGET is configured" >&2
    exit 1
  fi

  OFFSITE_PATH="${OFFSITE_TARGET%/}/$(date +%Y/%m/%d)"
  rclone copy "$DB_BACKUP_FILE" "$OFFSITE_PATH/database/"
  rclone copy "$UPLOADS_BACKUP_FILE" "$OFFSITE_PATH/uploads/"
  OFFSITE_STATUS="uploaded"
fi

CURRENT_STEP="updating manifest with offsite status"
{
  echo
  echo "Offsite backup:"
  echo "Status: $OFFSITE_STATUS"
  if [[ -n "$OFFSITE_PATH" ]]; then
    echo "Target: $OFFSITE_PATH"
  fi
} >> "$MANIFEST_FILE"

if [[ -n "$OFFSITE_TARGET" ]]; then
  CURRENT_STEP="uploading manifest to offsite target"
  rclone copy "$MANIFEST_FILE" "$OFFSITE_PATH/manifests/"
fi

CURRENT_STEP="applying local retention"
find "$DB_BACKUP_DIR" -type f -name "*.dump" -mtime +"$BACKUP_RETENTION_DAYS" -delete
find "$UPLOADS_BACKUP_DIR" -type f -name "*.tar.gz" -mtime +"$BACKUP_RETENTION_DAYS" -delete
find "$MANIFEST_DIR" -type f -name "*.txt" -mtime +"$BACKUP_RETENTION_DAYS" -delete
find "$FAILURE_DIR" -type f -name "*.txt" -mtime +"$BACKUP_RETENTION_DAYS" -delete

echo "Database backup created: $DB_BACKUP_FILE"
echo "Uploads backup created: $UPLOADS_BACKUP_FILE"
echo "Manifest created: $MANIFEST_FILE"
if [[ -n "$OFFSITE_PATH" ]]; then
  echo "Offsite backup uploaded: $OFFSITE_PATH"
fi
echo "[$(date --iso-8601=seconds)] Daily local backup completed"

CURRENT_STEP="sending success notification"
if [[ "${BACKUP_NOTIFY_ON_SUCCESS:-true}" == "true" ]]; then
  send_backup_email "SUCCESS - $(hostname)" "$MANIFEST_FILE" || echo "Email notification failed after successful backup" >&2
fi

CURRENT_STEP="completed"
