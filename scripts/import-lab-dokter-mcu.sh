#!/usr/bin/env bash
# Copies the MCU doctors (refdoctor) from the legacy "lab" database. Run
# import-lab-dokter-master.sh too, so the Dokter Master names show. The clinic
# doctors (refdokter) are imported by import-lab-dokter.sh.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-dokter-mcu.sh            # only into an empty table
#   LAB_PASSWORD=... scripts/import-lab-dokter-mcu.sh --replace  # wipe and copy again
#
# See scripts/lib/lab.sh for the optional LAB_* settings.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty refdoctor "$REPLACE"

echo "Clearing app MCU doctors..."
app -c "TRUNCATE refdoctor"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy refdoctor "doctorcode, doctorname, doctormaster, feepercentage"
echo "Done."
