#!/usr/bin/env bash
# Copies the kinds of MCU doctor (refdoctormaster) from the legacy "lab" database.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-dokter-master.sh            # only into an empty table
#   LAB_PASSWORD=... scripts/import-lab-dokter-master.sh --replace  # wipe and copy again
#
# See scripts/lib/lab.sh for the optional LAB_* settings.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty refdoctormaster "$REPLACE"

echo "Clearing app dokter master..."
app -c "TRUNCATE refdoctormaster RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy refdoctormaster "doctormaster, doctormastername"
# reset_identity assumes an "id" column; refdoctormaster's is doctormaster.
app -c "SELECT setval(pg_get_serial_sequence('refdoctormaster', 'doctormaster'), COALESCE((SELECT max(doctormaster) FROM refdoctormaster), 0) + 1, false)" >/dev/null
echo "Done."
