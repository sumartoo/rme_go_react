#!/usr/bin/env bash
# Copies the doctor's prescriptions and diagnoses per registration
# (trxjualobatdokter, trxjualdiagnosadokter) from the legacy "lab" database,
# for Transaksi Klinik > Pemeriksaan Dokter.
#
# The data is medical: keep it on machines allowed to hold it, and never commit it.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-pemeriksaan-dokter.sh            # only into empty tables
#   LAB_PASSWORD=... scripts/import-lab-pemeriksaan-dokter.sh --replace  # wipe and copy again
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty trxjualobatdokter "$REPLACE"
require_empty trxjualdiagnosadokter "$REPLACE"

echo "Clearing app doctor prescription tables..."
app -c "TRUNCATE trxjualobatdokter, trxjualdiagnosadokter RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy trxjualobatdokter "id, id_kun, id_obat, byk, dosis"
copy trxjualdiagnosadokter "id, id_kun, icd_id"

reset_identity trxjualobatdokter trxjualdiagnosadokter
app -c "ANALYZE trxjualobatdokter" >/dev/null
app -c "ANALYZE trxjualdiagnosadokter" >/dev/null
echo "Done."
