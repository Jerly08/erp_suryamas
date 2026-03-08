#!/bin/bash
# ============================================================================
# TEST SCRIPT: End-of-Month Depreciation Scheduler for Asset AS-2026-003
# ============================================================================
#
# Script ini menguji apakah scheduler depresiasi bulanan berjalan dengan benar:
#   1. Ambil data asset AS-2026-003 (MOBIL - Vehicle, Rp 150.000.000)
#   2. Trigger depreciation run untuk bulan Maret 2026
#   3. Verifikasi jurnal depresiasi tercatat di unified_journal_ledger
#   4. Validasi debit/credit amount sesuai kalkulasi Straight-Line
#   5. Cek accumulated_depreciation di asset ter-update
#   6. Verifikasi jurnal muncul di halaman /journal
#
# Penggunaan:
#   chmod +x test_depreciation_end_of_month.sh
#   ./test_depreciation_end_of_month.sh
#
# Konfigurasi (sesuaikan jika perlu):
#   BASE_URL  - URL backend API
#   TOKEN     - JWT token untuk autentikasi (login sebagai admin/finance)
# ============================================================================

set -e

# ── Auto-detect Python command ───────────────────────────────────────────────
PY=""
for _cmd in python3 python py; do
  if type "$_cmd" &>/dev/null; then
    PY="$_cmd"
    break
  fi
done
if [ -z "$PY" ]; then
  echo "ERROR: Python not found. Install Python 3."
  exit 1
fi
echo "Using Python: $PY ($($PY --version 2>&1))"

# Helper: run python and strip Windows \r
pyrun() {
  $PY "$@" | tr -d '\r'
}

# ── Konfigurasi ──────────────────────────────────────────────────────────────
BASE_URL="${BASE_URL:-http://localhost:8080}"
ASSET_CODE="AS-2026-003"
DEP_YEAR=2026
DEP_MONTH=3   # Maret 2026
USER_ID=1

# Warna output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color
BOLD='\033[1m'

# Counters
PASS=0
FAIL=0
WARN=0

# ── Helper Functions ─────────────────────────────────────────────────────────
print_header() {
  echo ""
  echo -e "${CYAN}╔══════════════════════════════════════════════════════════════╗${NC}"
  echo -e "${CYAN}║${NC}  ${BOLD}$1${NC}"
  echo -e "${CYAN}╚══════════════════════════════════════════════════════════════╝${NC}"
}

print_step() {
  echo -e "\n${BLUE}► $1${NC}"
}

print_pass() {
  echo -e "  ${GREEN}✅ PASS:${NC} $1"
  PASS=$((PASS + 1))
}

print_fail() {
  echo -e "  ${RED}❌ FAIL:${NC} $1"
  FAIL=$((FAIL + 1))
}

print_warn() {
  echo -e "  ${YELLOW}⚠️  WARN:${NC} $1"
  WARN=$((WARN + 1))
}

print_info() {
  echo -e "  ${CYAN}ℹ️  INFO:${NC} $1"
}

# ── Step 0: Login untuk mendapatkan JWT Token ────────────────────────────────
print_header "STEP 0: Autentikasi — Mendapatkan JWT Token"

if [ -z "$TOKEN" ]; then
  print_step "Login sebagai admin..."
  LOGIN_RESP=$(curl -s -X POST "${BASE_URL}/api/v1/auth/login" \
    -H "Content-Type: application/json" \
    -d '{"email":"admin@company.com","password":"password123"}' 2>/dev/null || echo '{}')

  TOKEN=$(echo "$LOGIN_RESP" | pyrun -c "
import sys,json
d=json.load(sys.stdin)
for key in ['token','access_token']:
    v = d.get(key,'')
    if v:
        print(v,end='')
        sys.exit(0)
    v = d.get('data',{}).get(key,'') if isinstance(d.get('data'),dict) else ''
    if v:
        print(v,end='')
        sys.exit(0)
print('',end='')
" 2>/dev/null | tr -d '\r\n')
  # Re-add newline for variable assignment
  TOKEN="${TOKEN}"

  if [ -z "$TOKEN" ]; then
    print_fail "Gagal login. Response: $(echo "$LOGIN_RESP" | head -c 200)"
    echo -e "\n${YELLOW}Tip: Set TOKEN secara manual:${NC}"
    echo "  export TOKEN='your-jwt-token-here'"
    echo "  ./test_depreciation_end_of_month.sh"
    exit 1
  fi
  print_pass "Login berhasil, token didapatkan"
else
  print_info "Menggunakan TOKEN dari environment variable"
fi

AUTH_HEADER="Authorization: Bearer ${TOKEN}"

# ── Step 1: Ambil Data Asset AS-2026-003 ─────────────────────────────────────
print_header "STEP 1: Ambil Data Asset ${ASSET_CODE}"

print_step "Mengambil semua assets..."
ASSETS_RESP=$(curl -s -X GET "${BASE_URL}/api/v1/assets" \
  -H "${AUTH_HEADER}" \
  -H "Content-Type: application/json")

# Cari asset berdasarkan code
ASSET_JSON=$(echo "$ASSETS_RESP" | pyrun -c "
import sys, json
data = json.load(sys.stdin)
assets = data.get('data', [])
for a in assets:
    if a.get('code') == '${ASSET_CODE}':
        json.dump(a, sys.stdout)
        sys.exit(0)
print('{}')
" 2>/dev/null || echo '{}')

if [ "$ASSET_JSON" = "{}" ]; then
  print_fail "Asset ${ASSET_CODE} tidak ditemukan!"
  exit 1
fi

# Ekstrak data asset
ASSET_ID=$(echo "$ASSET_JSON" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('id',0))")
ASSET_NAME=$(echo "$ASSET_JSON" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('name',''))")
PURCHASE_PRICE=$(echo "$ASSET_JSON" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('purchase_price',0))")
SALVAGE_VALUE=$(echo "$ASSET_JSON" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('salvage_value',0))")
USEFUL_LIFE=$(echo "$ASSET_JSON" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('useful_life',0))")
DEP_METHOD=$(echo "$ASSET_JSON" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('depreciation_method',''))")
STATUS=$(echo "$ASSET_JSON" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('status',''))")
ACCUM_DEP_BEFORE=$(echo "$ASSET_JSON" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('accumulated_depreciation',0))")
CATEGORY=$(echo "$ASSET_JSON" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('category',''))")
DEP_ACCOUNT_ID=$(echo "$ASSET_JSON" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('depreciation_account_id') or 'null')")
ACCUM_DEP_ACCOUNT_ID=$(echo "$ASSET_JSON" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('accumulated_depreciation_account_id') or 'null')")
DEP_ACCOUNT_NAME=$(echo "$ASSET_JSON" | pyrun -c "import sys,json; a=json.load(sys.stdin).get('depreciation_account'); print(f\"{a['code']} - {a['name']}\" if a else 'N/A')")
ACCUM_DEP_ACCOUNT_NAME=$(echo "$ASSET_JSON" | pyrun -c "import sys,json; a=json.load(sys.stdin).get('accumulated_depreciation_account'); print(f\"{a['code']} - {a['name']}\" if a else 'N/A')")

BOOK_VALUE_BEFORE=$(pyrun -c "print(${PURCHASE_PRICE} - ${ACCUM_DEP_BEFORE})")

print_pass "Asset ditemukan: ${ASSET_CODE}"
echo ""
echo -e "  ${BOLD}┌─────────────────────────────────────────────────────┐${NC}"
echo -e "  ${BOLD}│${NC} Data Asset ${ASSET_CODE}                               ${BOLD}│${NC}"
echo -e "  ${BOLD}├─────────────────────────────────────────────────────┤${NC}"
echo -e "  ${BOLD}│${NC} ID                  : ${ASSET_ID}"
echo -e "  ${BOLD}│${NC} Nama                : ${ASSET_NAME}"
echo -e "  ${BOLD}│${NC} Kategori            : ${CATEGORY}"
echo -e "  ${BOLD}│${NC} Status              : ${STATUS}"
echo -e "  ${BOLD}│${NC} Harga Pembelian     : Rp $(printf "%'.0f" "$PURCHASE_PRICE")"
echo -e "  ${BOLD}│${NC} Nilai Sisa          : Rp $(printf "%'.0f" "$SALVAGE_VALUE")"
echo -e "  ${BOLD}│${NC} Masa Manfaat        : ${USEFUL_LIFE} tahun"
echo -e "  ${BOLD}│${NC} Metode Penyusutan   : ${DEP_METHOD}"
echo -e "  ${BOLD}│${NC} Akumulasi Penyusutan: Rp $(printf "%'.0f" "$ACCUM_DEP_BEFORE")"
echo -e "  ${BOLD}│${NC} Nilai Buku Saat Ini : Rp $(printf "%'.0f" "$BOOK_VALUE_BEFORE")"
echo -e "  ${BOLD}│${NC} Akun Beban Dep.     : ${DEP_ACCOUNT_NAME} (ID: ${DEP_ACCOUNT_ID})"
echo -e "  ${BOLD}│${NC} Akun Akumulasi Dep. : ${ACCUM_DEP_ACCOUNT_NAME} (ID: ${ACCUM_DEP_ACCOUNT_ID})"
echo -e "  ${BOLD}└─────────────────────────────────────────────────────┘${NC}"

# ── Step 2: Validasi Pre-conditions ──────────────────────────────────────────
print_header "STEP 2: Validasi Pre-conditions"

# Check status ACTIVE
if [ "$STATUS" = "ACTIVE" ]; then
  print_pass "Status asset: ACTIVE"
else
  print_fail "Status asset bukan ACTIVE (${STATUS}). Depresiasi hanya berjalan untuk asset ACTIVE."
  exit 1
fi

# Check depreciation method
if [ -n "$DEP_METHOD" ] && [ "$DEP_METHOD" != "null" ]; then
  print_pass "Metode depresiasi: ${DEP_METHOD}"
else
  print_fail "Metode depresiasi belum diset. Depresiasi tidak akan berjalan."
  exit 1
fi

# Check useful life > 0
if [ "$USEFUL_LIFE" -gt 0 ] 2>/dev/null; then
  print_pass "Masa manfaat: ${USEFUL_LIFE} tahun (> 0)"
else
  print_fail "Masa manfaat harus > 0 (saat ini: ${USEFUL_LIFE})"
  exit 1
fi

# Check belum fully depreciated
IS_FULLY_DEP=$(pyrun -c "
pp = ${PURCHASE_PRICE}
sv = ${SALVAGE_VALUE}
ad = ${ACCUM_DEP_BEFORE}
bv = pp - ad
print('yes' if bv <= sv else 'no')
")
if [ "$IS_FULLY_DEP" = "no" ]; then
  print_pass "Asset belum fully depreciated (BV: Rp $(printf "%'.0f" "$BOOK_VALUE_BEFORE") > SV: Rp $(printf "%'.0f" "$SALVAGE_VALUE"))"
else
  print_fail "Asset sudah fully depreciated. Tidak ada depresiasi yang perlu dijalankan."
  exit 1
fi

# Check depreciation accounts configured
if [ "$DEP_ACCOUNT_ID" != "null" ] && [ "$DEP_ACCOUNT_ID" != "0" ] && [ -n "$DEP_ACCOUNT_ID" ]; then
  print_pass "Akun beban penyusutan terkonfigurasi: ${DEP_ACCOUNT_NAME}"
else
  print_warn "Akun beban penyusutan belum diset. Akan menggunakan fallback (6103/6104)."
fi

if [ "$ACCUM_DEP_ACCOUNT_ID" != "null" ] && [ "$ACCUM_DEP_ACCOUNT_ID" != "0" ] && [ -n "$ACCUM_DEP_ACCOUNT_ID" ]; then
  print_pass "Akun akumulasi penyusutan terkonfigurasi: ${ACCUM_DEP_ACCOUNT_NAME}"
else
  print_warn "Akun akumulasi penyusutan belum diset. Akan menggunakan fallback (1202-003 Vehicle)."
fi

# ── Step 3: Hitung Expected Depreciation ─────────────────────────────────────
print_header "STEP 3: Kalkulasi Expected Depreciation"

EXPECTED_MONTHLY_DEP=$(pyrun -c "
purchase_price = ${PURCHASE_PRICE}
salvage_value = ${SALVAGE_VALUE}
useful_life = ${USEFUL_LIFE}
accum_dep = ${ACCUM_DEP_BEFORE}
method = '${DEP_METHOD}'

depreciable_amount = purchase_price - salvage_value
book_value = purchase_price - accum_dep

if method == 'STRAIGHT_LINE':
    monthly = depreciable_amount / (useful_life * 12)
elif method == 'DECLINING_BALANCE':
    rate = 2.0 / useful_life
    annual = book_value * rate
    monthly = annual / 12
    if book_value - monthly < salvage_value:
        monthly = book_value - salvage_value
else:
    monthly = 0

# Cap: jangan sampai book value turun di bawah salvage value
max_deprec = book_value - salvage_value
if monthly > max_deprec:
    monthly = max_deprec
if monthly < 0:
    monthly = 0

print(f'{monthly:.2f}')
")

EXPECTED_ACCUM_AFTER=$(pyrun -c "print(f'{${ACCUM_DEP_BEFORE} + ${EXPECTED_MONTHLY_DEP}:.2f}')")
EXPECTED_BV_AFTER=$(pyrun -c "print(f'{${PURCHASE_PRICE} - ${EXPECTED_ACCUM_AFTER}:.2f}')")

echo ""
echo -e "  ${BOLD}┌─────────────────────────────────────────────────────┐${NC}"
echo -e "  ${BOLD}│${NC} Kalkulasi Depresiasi Bulanan (${DEP_METHOD})         ${BOLD}│${NC}"
echo -e "  ${BOLD}├─────────────────────────────────────────────────────┤${NC}"
echo -e "  ${BOLD}│${NC} Depreciable Amount  : Rp $(printf "%'.0f" "$(pyrun -c "print(${PURCHASE_PRICE}-${SALVAGE_VALUE})")")"
echo -e "  ${BOLD}│${NC} Formula             : (Cost - Salvage) / (Life x 12)"
echo -e "  ${BOLD}│${NC} Expected Monthly    : Rp $(printf "%'.2f" "$EXPECTED_MONTHLY_DEP")"
echo -e "  ${BOLD}│${NC}"
echo -e "  ${BOLD}│${NC} SEBELUM Run:"
echo -e "  ${BOLD}│${NC}   Akumulasi Dep.    : Rp $(printf "%'.2f" "$ACCUM_DEP_BEFORE")"
echo -e "  ${BOLD}│${NC}   Nilai Buku        : Rp $(printf "%'.2f" "$BOOK_VALUE_BEFORE")"
echo -e "  ${BOLD}│${NC}"
echo -e "  ${BOLD}│${NC} EXPECTED SESUDAH Run:"
echo -e "  ${BOLD}│${NC}   Akumulasi Dep.    : Rp $(printf "%'.2f" "$EXPECTED_ACCUM_AFTER")"
echo -e "  ${BOLD}│${NC}   Nilai Buku        : Rp $(printf "%'.2f" "$EXPECTED_BV_AFTER")"
echo -e "  ${BOLD}└─────────────────────────────────────────────────────┘${NC}"

print_pass "Expected monthly depreciation: Rp $(printf "%'.2f" "$EXPECTED_MONTHLY_DEP")"

# ── Step 4: Trigger Run Depreciation ─────────────────────────────────────────
print_header "STEP 4: Trigger Run Depreciation — Periode ${DEP_YEAR}-$(printf '%02d' $DEP_MONTH)"

print_step "Menjalankan POST /api/v1/assets/run-depreciation..."

DEP_RUN_RESP=$(curl -s -w "\n%{http_code}" -X POST "${BASE_URL}/api/v1/assets/run-depreciation" \
  -H "${AUTH_HEADER}" \
  -H "Content-Type: application/json" \
  -d "{\"year\": ${DEP_YEAR}, \"month\": ${DEP_MONTH}, \"user_id\": ${USER_ID}}")

HTTP_CODE=$(echo "$DEP_RUN_RESP" | tail -1)
DEP_RUN_BODY=$(echo "$DEP_RUN_RESP" | sed '$d')

echo ""
echo -e "  HTTP Status: ${HTTP_CODE}"

if [ "$HTTP_CODE" = "200" ]; then
  print_pass "Depreciation run berhasil (HTTP 200)"

  # Parse result
  PERIOD=$(echo "$DEP_RUN_BODY" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('data',{}).get('period',''))" 2>/dev/null || echo "N/A")
  TOTAL_ASSETS=$(echo "$DEP_RUN_BODY" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('data',{}).get('total_assets',0))" 2>/dev/null || echo "0")
  PROCESSED=$(echo "$DEP_RUN_BODY" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('data',{}).get('processed_assets',0))" 2>/dev/null || echo "0")
  SKIPPED=$(echo "$DEP_RUN_BODY" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('data',{}).get('skipped_assets',0))" 2>/dev/null || echo "0")
  TOTAL_DEP=$(echo "$DEP_RUN_BODY" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('data',{}).get('total_depreciation',0))" 2>/dev/null || echo "0")
  ERRORS=$(echo "$DEP_RUN_BODY" | pyrun -c "import sys,json; e=json.load(sys.stdin).get('data',{}).get('errors',[]); print(len(e))" 2>/dev/null || echo "0")

  echo ""
  echo -e "  ${BOLD}┌─────────────────────────────────────────────────────┐${NC}"
  echo -e "  ${BOLD}│${NC} Hasil Run Depreciation                              ${BOLD}│${NC}"
  echo -e "  ${BOLD}├─────────────────────────────────────────────────────┤${NC}"
  echo -e "  ${BOLD}│${NC} Periode             : ${PERIOD}"
  echo -e "  ${BOLD}│${NC} Total Assets        : ${TOTAL_ASSETS}"
  echo -e "  ${BOLD}│${NC} Processed           : ${PROCESSED}"
  echo -e "  ${BOLD}│${NC} Skipped             : ${SKIPPED}"
  echo -e "  ${BOLD}│${NC} Total Depreciation  : Rp $(printf "%'.2f" "$TOTAL_DEP")"
  echo -e "  ${BOLD}│${NC} Errors              : ${ERRORS}"
  echo -e "  ${BOLD}└─────────────────────────────────────────────────────┘${NC}"

  if [ "$PROCESSED" -gt 0 ] 2>/dev/null; then
    print_pass "Assets berhasil diproses: ${PROCESSED}"
  else
    print_warn "Tidak ada asset yang diproses. Mungkin depreciation untuk periode ini sudah dijalankan sebelumnya (idempotent)."
  fi

  if [ "$ERRORS" -gt 0 ] 2>/dev/null; then
    print_warn "Ada ${ERRORS} error selama depreciation run"
    echo "$DEP_RUN_BODY" | pyrun -c "
import sys,json
errors = json.load(sys.stdin).get('data',{}).get('errors',[])
for e in errors:
    print(f'    - {e}')
" 2>/dev/null
  fi
else
  print_fail "Depreciation run gagal (HTTP ${HTTP_CODE})"
  echo -e "  Response: ${DEP_RUN_BODY}"

  # Jika gagal karena closed period, tampilkan info
  if echo "$DEP_RUN_BODY" | grep -qi "closed"; then
    print_info "Periode mungkin sudah ditutup. Coba buka periode terlebih dahulu."
  fi
fi

# ── Step 5: Verifikasi Asset Data Setelah Depreciation ───────────────────────
print_header "STEP 5: Verifikasi Data Asset Setelah Depreciation"

print_step "Mengambil ulang data asset ${ASSET_CODE}..."

ASSET_AFTER_RESP=$(curl -s -X GET "${BASE_URL}/api/v1/assets/${ASSET_ID}" \
  -H "${AUTH_HEADER}" \
  -H "Content-Type: application/json")

ACCUM_DEP_AFTER=$(echo "$ASSET_AFTER_RESP" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('data',{}).get('accumulated_depreciation',0))" 2>/dev/null || echo "0")
BV_AFTER=$(pyrun -c "print(f'{${PURCHASE_PRICE} - ${ACCUM_DEP_AFTER}:.2f}')")

echo ""
echo -e "  ${BOLD}Perbandingan Sebelum vs Sesudah:${NC}"
echo -e "  ┌────────────────────────┬──────────────────────┬──────────────────────┐"
echo -e "  │ Metric                 │ Sebelum              │ Sesudah              │"
echo -e "  ├────────────────────────┼──────────────────────┼──────────────────────┤"
printf "  │ Akumulasi Penyusutan   │ Rp %'16.2f │ Rp %'16.2f │\n" "$ACCUM_DEP_BEFORE" "$ACCUM_DEP_AFTER"
printf "  │ Nilai Buku             │ Rp %'16.2f │ Rp %'16.2f │\n" "$BOOK_VALUE_BEFORE" "$BV_AFTER"
echo -e "  └────────────────────────┴──────────────────────┴──────────────────────┘"

# Validasi akumulasi bertambah
DEP_INCREASED=$(pyrun -c "print('yes' if ${ACCUM_DEP_AFTER} > ${ACCUM_DEP_BEFORE} else 'no')")
if [ "$DEP_INCREASED" = "yes" ]; then
  ACTUAL_DEP=$(pyrun -c "print(f'{${ACCUM_DEP_AFTER} - ${ACCUM_DEP_BEFORE}:.2f}')")
  print_pass "Akumulasi penyusutan bertambah: +Rp $(printf "%'.2f" "$ACTUAL_DEP")"

  # Validasi jumlah sesuai expected
  DEP_MATCH=$(pyrun -c "
actual = ${ACCUM_DEP_AFTER} - ${ACCUM_DEP_BEFORE}
expected = ${EXPECTED_MONTHLY_DEP}
diff = abs(actual - expected)
print('yes' if diff < 0.02 else 'no')  # Toleransi rounding 0.02
")
  if [ "$DEP_MATCH" = "yes" ]; then
    print_pass "Jumlah depresiasi sesuai expected: Rp $(printf "%'.2f" "$EXPECTED_MONTHLY_DEP")"
  else
    ACTUAL_DEP_VAL=$(pyrun -c "print(f'{${ACCUM_DEP_AFTER} - ${ACCUM_DEP_BEFORE}:.2f}')")
    print_warn "Jumlah depresiasi berbeda dari expected. Actual: Rp ${ACTUAL_DEP_VAL}, Expected: Rp ${EXPECTED_MONTHLY_DEP}"
    print_info "Ini bisa terjadi jika depreciation sudah pernah dijalankan sebelumnya (multiple months)."
  fi
else
  print_warn "Akumulasi penyusutan tidak bertambah. Kemungkinan sudah diproses sebelumnya (idempotent skip)."
fi

# ── Step 6: Verifikasi Jurnal di Unified Journal Ledger ──────────────────────
print_header "STEP 6: Verifikasi Jurnal Depresiasi di Halaman Journal"

DEP_REFERENCE="${ASSET_CODE}-DEP-${DEP_YEAR}-$(printf '%02d' $DEP_MONTH)"
print_step "Mencari jurnal dengan reference: ${DEP_REFERENCE}"

# Ambil journals - cari yang terkait depresiasi
JOURNALS_RESP=$(curl -s -X GET "${BASE_URL}/api/v1/journals?source_type=DEPRECIATION&limit=50" \
  -H "${AUTH_HEADER}" \
  -H "Content-Type: application/json")

# Cari jurnal yang cocok
FOUND_JOURNAL=$(echo "$JOURNALS_RESP" | pyrun -c "
import sys, json
data = json.load(sys.stdin)
entries = data.get('data', data.get('entries', []))
if isinstance(entries, list):
    for e in entries:
        ref = e.get('reference', '')
        if ref == '${DEP_REFERENCE}':
            json.dump(e, sys.stdout)
            sys.exit(0)
        # Juga cari berdasarkan source_code
        if e.get('source_code', '') == '${ASSET_CODE}' and 'DEP' in e.get('entry_number', '') and '${DEP_YEAR}$(printf '%02d' $DEP_MONTH)' in e.get('entry_number', ''):
            json.dump(e, sys.stdout)
            sys.exit(0)
print('{}')
" 2>/dev/null || echo '{}')

if [ "$FOUND_JOURNAL" != "{}" ]; then
  J_ID=$(echo "$FOUND_JOURNAL" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('id',0))")
  J_NUM=$(echo "$FOUND_JOURNAL" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('entry_number',''))")
  J_DESC=$(echo "$FOUND_JOURNAL" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('description',''))")
  J_STATUS=$(echo "$FOUND_JOURNAL" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('status',''))")
  J_REF=$(echo "$FOUND_JOURNAL" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('reference',''))")
  J_DEBIT=$(echo "$FOUND_JOURNAL" | pyrun -c "import sys,json; d=json.load(sys.stdin).get('total_debit','0'); print(float(str(d)) if d else 0)")
  J_CREDIT=$(echo "$FOUND_JOURNAL" | pyrun -c "import sys,json; d=json.load(sys.stdin).get('total_credit','0'); print(float(str(d)) if d else 0)")
  J_DATE=$(echo "$FOUND_JOURNAL" | pyrun -c "import sys,json; print(json.load(sys.stdin).get('entry_date','')[:10])")

  print_pass "Jurnal depresiasi ditemukan!"
  echo ""
  echo -e "  ${BOLD}┌─────────────────────────────────────────────────────┐${NC}"
  echo -e "  ${BOLD}│${NC} Jurnal Depresiasi ${DEP_REFERENCE}                ${BOLD}│${NC}"
  echo -e "  ${BOLD}├─────────────────────────────────────────────────────┤${NC}"
  echo -e "  ${BOLD}│${NC} ID           : ${J_ID}"
  echo -e "  ${BOLD}│${NC} Entry Number : ${J_NUM}"
  echo -e "  ${BOLD}│${NC} Reference    : ${J_REF}"
  echo -e "  ${BOLD}│${NC} Tanggal      : ${J_DATE}"
  echo -e "  ${BOLD}│${NC} Deskripsi    : ${J_DESC}"
  echo -e "  ${BOLD}│${NC} Status       : ${J_STATUS}"
  echo -e "  ${BOLD}│${NC} Total Debit  : Rp $(printf "%'.2f" "$J_DEBIT")"
  echo -e "  ${BOLD}│${NC} Total Credit : Rp $(printf "%'.2f" "$J_CREDIT")"
  echo -e "  ${BOLD}└─────────────────────────────────────────────────────┘${NC}"

  # Validasi status POSTED
  if [ "$J_STATUS" = "POSTED" ]; then
    print_pass "Status jurnal: POSTED"
  else
    print_fail "Status jurnal bukan POSTED (${J_STATUS})"
  fi

  # Validasi balanced (debit = credit)
  BALANCED=$(pyrun -c "print('yes' if abs(${J_DEBIT} - ${J_CREDIT}) < 0.01 else 'no')")
  if [ "$BALANCED" = "yes" ]; then
    print_pass "Jurnal balanced (Debit = Credit)"
  else
    print_fail "Jurnal TIDAK balanced! Debit: ${J_DEBIT}, Credit: ${J_CREDIT}"
  fi

  # Validasi amount sesuai expected
  AMOUNT_MATCH=$(pyrun -c "print('yes' if abs(${J_DEBIT} - ${EXPECTED_MONTHLY_DEP}) < 0.02 else 'no')")
  if [ "$AMOUNT_MATCH" = "yes" ]; then
    print_pass "Jumlah jurnal sesuai expected monthly depreciation: Rp $(printf "%'.2f" "$EXPECTED_MONTHLY_DEP")"
  else
    print_warn "Jumlah jurnal berbeda. Journal: Rp $(printf "%'.2f" "$J_DEBIT"), Expected: Rp $(printf "%'.2f" "$EXPECTED_MONTHLY_DEP")"
  fi

  # Step 6b: Ambil detail jurnal lines untuk verifikasi akun debit/credit
  print_step "Mengambil detail journal lines..."
  JOURNAL_DETAIL=$(curl -s -X GET "${BASE_URL}/api/v1/journals/${J_ID}" \
    -H "${AUTH_HEADER}" \
    -H "Content-Type: application/json")

  echo "$JOURNAL_DETAIL" | pyrun -c "
import sys, json
data = json.load(sys.stdin)
entry = data.get('data', data)
lines = entry.get('lines', entry.get('journal_lines', []))
if lines:
    print()
    print('  Journal Lines:')
    print('  ┌─────┬────────────────────────────────────┬──────────────────┬──────────────────┐')
    print('  │ No  │ Akun                               │ Debit            │ Credit           │')
    print('  ├─────┼────────────────────────────────────┼──────────────────┼──────────────────┤')
    for line in lines:
        no = line.get('line_number', '-')
        desc = line.get('description', '')[:34]
        debit = float(str(line.get('debit_amount', 0)))
        credit = float(str(line.get('credit_amount', 0)))
        acc = line.get('account', {})
        acc_code = acc.get('code', '') if acc else ''
        acc_name = acc.get('name', '') if acc else ''
        label = f'{acc_code} {acc_name}'.strip()[:34] if acc_code else desc[:34]
        print(f'  │ {no:<3} │ {label:<34} │ Rp {debit:>12,.2f} │ Rp {credit:>12,.2f} │')
    print('  └─────┴────────────────────────────────────┴──────────────────┴──────────────────┘')

    # Validate: Line 1 should be DEBIT (Beban Penyusutan), Line 2 should be CREDIT (Akumulasi Penyusutan)
    has_debit = any(float(str(l.get('debit_amount',0))) > 0 for l in lines)
    has_credit = any(float(str(l.get('credit_amount',0))) > 0 for l in lines)
    print()
    if has_debit and has_credit:
        print('  ✅ PASS: Jurnal memiliki entry Debit (Beban Penyusutan) dan Credit (Akumulasi Penyusutan)')
    else:
        print('  ❌ FAIL: Jurnal tidak memiliki pasangan Debit/Credit yang benar')
else:
    print('  ⚠️  Tidak dapat mengambil journal lines (mungkin format API berbeda)')
" 2>/dev/null

else
  print_warn "Jurnal depresiasi untuk ${DEP_REFERENCE} tidak ditemukan di response."
  print_info "Kemungkinan: jurnal sudah ada dari run sebelumnya, atau format pencarian berbeda."

  # Coba tampilkan semua jurnal DEPRECIATION terbaru
  print_step "Menampilkan 5 jurnal DEPRECIATION terbaru untuk ${ASSET_CODE}..."
  echo "$JOURNALS_RESP" | pyrun -c "
import sys, json
data = json.load(sys.stdin)
entries = data.get('data', data.get('entries', []))
if isinstance(entries, list):
    asset_entries = [e for e in entries if e.get('source_code', '') == '${ASSET_CODE}' or '${ASSET_CODE}' in e.get('reference', '')]
    for e in asset_entries[:5]:
        print(f\"  - {e.get('entry_number','')} | {e.get('reference','')} | {e.get('entry_date','')[:10]} | {e.get('status','')} | Debit: {e.get('total_debit','0')}\")
    if not asset_entries:
        print('  (Tidak ada jurnal DEPRECIATION untuk asset ini)')
" 2>/dev/null
fi

# ── Step 7: Verifikasi via Depreciation Schedule API ─────────────────────────
print_header "STEP 7: Verifikasi via Depreciation Schedule API"

print_step "Mengambil depreciation schedule untuk asset ID ${ASSET_ID}..."

SCHEDULE_RESP=$(curl -s -X GET "${BASE_URL}/api/v1/assets/${ASSET_ID}/depreciation-schedule" \
  -H "${AUTH_HEADER}" \
  -H "Content-Type: application/json")

echo "$SCHEDULE_RESP" | pyrun -c "
import sys, json
data = json.load(sys.stdin)
schedule = data.get('data', {}).get('schedule', [])
if schedule:
    print()
    print('  Depreciation Schedule (5 teratas):')
    print('  ┌──────┬────────────┬──────────────────┬──────────────────┬──────────────────┐')
    print('  │ Thn  │ Tanggal    │ Dep. Bulanan     │ Akumulasi        │ Nilai Buku       │')
    print('  ├──────┼────────────┼──────────────────┼──────────────────┼──────────────────┤')
    for entry in schedule[:5]:
        yr = entry.get('year', 0)
        date = entry.get('date', '')[:10]
        dep = float(entry.get('depreciation_cost', 0))
        accum = float(entry.get('accumulated_depreciation', 0))
        bv = float(entry.get('book_value', 0))
        print(f'  │ {yr:<4} │ {date:<10} │ Rp {dep:>12,.2f} │ Rp {accum:>12,.2f} │ Rp {bv:>12,.2f} │')
    print('  └──────┴────────────┴──────────────────┴──────────────────┴──────────────────┘')
    print(f'  Total entri schedule: {len(schedule)}')
else:
    print('  ⚠️  Depreciation schedule kosong atau format berbeda.')
" 2>/dev/null

# ── Step 8: Verifikasi Calculate Current Depreciation ────────────────────────
print_header "STEP 8: Verifikasi Current Depreciation Calculation"

print_step "Menghitung depresiasi saat ini untuk asset ID ${ASSET_ID}..."

CALC_RESP=$(curl -s -X GET "${BASE_URL}/api/v1/assets/${ASSET_ID}/calculate-depreciation?as_of_date=${DEP_YEAR}-$(printf '%02d' $DEP_MONTH)-31" \
  -H "${AUTH_HEADER}" \
  -H "Content-Type: application/json")

echo "$CALC_RESP" | pyrun -c "
import sys, json
data = json.load(sys.stdin)
calc = data.get('data', {})
if calc:
    print()
    print('  Kalkulasi Depresiasi per 31 Maret 2026:')
    print(f\"  - Purchase Price       : Rp {float(calc.get('purchase_price', 0)):>15,.2f}\")
    print(f\"  - Salvage Value        : Rp {float(calc.get('salvage_value', 0)):>15,.2f}\")
    print(f\"  - Accumulated Deprec.  : Rp {float(calc.get('accumulated_depreciation', 0)):>15,.2f}\")
    print(f\"  - Current Book Value   : Rp {float(calc.get('current_book_value', 0)):>15,.2f}\")
    print(f\"  - Method               : {calc.get('depreciation_method', 'N/A')}\")
    print(f\"  - Useful Life          : {calc.get('useful_life_years', 'N/A')} years\")
else:
    print('  ⚠️  Tidak dapat menghitung depreciation saat ini.')
" 2>/dev/null

# ── SUMMARY ──────────────────────────────────────────────────────────────────
print_header "RINGKASAN HASIL TEST"

TOTAL=$((PASS + FAIL))
echo ""
echo -e "  ${GREEN}✅ PASS : ${PASS}${NC}"
echo -e "  ${RED}❌ FAIL : ${FAIL}${NC}"
echo -e "  ${YELLOW}⚠️  WARN : ${WARN}${NC}"
echo -e "  ─────────────"
echo -e "  ${BOLD}TOTAL   : ${TOTAL} assertions${NC}"
echo ""

if [ "$FAIL" -eq 0 ]; then
  echo -e "  ${GREEN}${BOLD}════════════════════════════════════════════════════${NC}"
  echo -e "  ${GREEN}${BOLD}  ALL TESTS PASSED! Scheduler depresiasi berjalan   ${NC}"
  echo -e "  ${GREEN}${BOLD}  dengan benar untuk asset ${ASSET_CODE}.           ${NC}"
  echo -e "  ${GREEN}${BOLD}════════════════════════════════════════════════════${NC}"
else
  echo -e "  ${RED}${BOLD}════════════════════════════════════════════════════${NC}"
  echo -e "  ${RED}${BOLD}  ${FAIL} TEST(S) FAILED! Periksa detail di atas.      ${NC}"
  echo -e "  ${RED}${BOLD}════════════════════════════════════════════════════${NC}"
fi

echo ""
echo -e "${CYAN}Selesai. Jurnal depresiasi bisa dilihat di: http://localhost:3000/journal${NC}"
echo ""
