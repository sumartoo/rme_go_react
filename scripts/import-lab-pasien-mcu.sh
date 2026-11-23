#!/usr/bin/env bash
# Copies the MCU patients (refpasien) from the legacy "lab" database. The
# clinic patients (refmpasien) are imported by import-lab-pasien.sh.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-pasien-mcu.sh            # only into an empty table
#   LAB_PASSWORD=... scripts/import-lab-pasien-mcu.sh --replace  # wipe and copy again
#
# See scripts/lib/lab.sh for the optional LAB_* settings.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty refpasien "$REPLACE"

echo "Clearing app MCU patients..."
app -c "TRUNCATE refpasien RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy refpasien "pasienid, pasienname, companycode, nik, departmen, position, rm, address, gender, hp, telp, mail,
  birthdaydate, isdeleted, createdby, createddate, modifyby, modifydate, deletedby, deleteddate"
# reset_identity assumes an "id" column; refpasien's is pasienid.
app -c "SELECT setval(pg_get_serial_sequence('refpasien', 'pasienid'), COALESCE((SELECT max(pasienid) FROM refpasien), 0) + 1, false)" >/dev/null
echo "Done."
