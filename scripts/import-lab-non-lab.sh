#!/usr/bin/env bash
# Copies the non-lab services (refnonlab) from the legacy "lab" database.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-non-lab.sh            # only into an empty table
#   LAB_PASSWORD=... scripts/import-lab-non-lab.sh --replace  # wipe and copy again
#
# See scripts/lib/lab.sh for the optional LAB_* settings.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty refnonlab "$REPLACE"

echo "Clearing app non-lab services..."
app -c "TRUNCATE refnonlab RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy refnonlab "id, name, price, isdeleted, createddate, createby, modifieddate, modifiedby, deleteddate, deletedby"
reset_identity refnonlab
echo "Done."
