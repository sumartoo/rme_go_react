#!/usr/bin/env bash
# Copies the MCU examination groups (refmastermcu) from the legacy "lab" database.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-master-mcu.sh            # only into an empty table
#   LAB_PASSWORD=... scripts/import-lab-master-mcu.sh --replace  # wipe and copy again
#
# See scripts/lib/lab.sh for the optional LAB_* settings.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty refmastermcu "$REPLACE"

echo "Clearing app MCU master..."
app -c "TRUNCATE refmastermcu RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy refmastermcu "id, mcumastercode, mcumastername"
reset_identity refmastermcu
echo "Done."
