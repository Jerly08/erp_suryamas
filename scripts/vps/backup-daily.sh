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
SUCCESS_REPORT_FILE="$MANIFEST_DIR/backup_email_${TIMESTAMP}.txt"
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

format_file_size() {
  local file_path="$1"

  if [[ ! -f "$file_path" ]]; then
    echo "-"
    return 0
  fi

  ls -lh "$file_path" | awk '{print $5}'
}

format_backup_time() {
  date '+%Y-%m-%d %H:%M:%S %Z'
}

format_offsite_status() {
  case "${OFFSITE_STATUS:-disabled}" in
    uploaded)
      echo "berhasil diupload"
      ;;
    disabled)
      echo "tidak dikonfigurasi"
      ;;
    *)
      echo "${OFFSITE_STATUS}"
      ;;
  esac
}

write_success_email_report() {
  local database_size
  local uploads_size
  local offsite_status_label

  database_size="$(format_file_size "$DB_BACKUP_FILE")"
  uploads_size="$(format_file_size "$UPLOADS_BACKUP_FILE")"
  offsite_status_label="$(format_offsite_status)"

  {
    echo "Backup ERP Avery berhasil"
    echo
    echo "Halo,"
    echo
    echo "Backup harian ERP Avery sudah selesai dibuat dengan aman."
    echo
    echo "Ringkasan:"
    echo "- ID backup: $TIMESTAMP"
    echo "- Waktu selesai: $(format_backup_time)"
    echo "- Database: berhasil (${database_size})"
    echo "- File upload: berhasil (${uploads_size})"
    echo "- Retensi lokal: ${BACKUP_RETENTION_DAYS} hari"
    echo "- Offsite backup: ${offsite_status_label}"
    echo
    echo "Cakupan backup:"
    echo "- Full database PostgreSQL"
    echo "- Semua file upload aplikasi di /app/uploads"
    echo
    echo "Detail teknis, checksum, dan lokasi file backup tersimpan di manifest server."
    echo
    echo "Pesan ini tidak perlu dibalas."
  } > "$SUCCESS_REPORT_FILE"
}

handle_exit() {
  local exit_status=$?
  cleanup_tmp_files

  if [[ "$exit_status" -ne 0 ]]; then
    {
      echo "Backup ERP Avery gagal"
      echo
      echo "Halo,"
      echo
      echo "Backup harian ERP Avery belum berhasil diselesaikan."
      echo
      echo "Ringkasan:"
      echo "- ID backup: $TIMESTAMP"
      echo "- Waktu gagal: $(format_backup_time)"
      echo "- Tahap gagal: $CURRENT_STEP"
      echo "- Env file: $ENV_FILE"
      echo "- Backup root: $BACKUP_ROOT"
      echo
      echo "Mohon cek log cron, status container Docker, kapasitas disk, dan konfigurasi rclone/email."
      echo
      echo "Pesan ini tidak perlu dibalas."
    } > "$FAILURE_REPORT_FILE" || true

    if [[ "${BACKUP_NOTIFY_ON_FAILURE:-true}" == "true" ]]; then
      send_backup_email "GAGAL - $(hostname)" "$FAILURE_REPORT_FILE" || true
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
  write_success_email_report
  send_backup_email "BERHASIL - $(hostname)" "$SUCCESS_REPORT_FILE" || echo "Email notification failed after successful backup" >&2
fi

CURRENT_STEP="completed"
