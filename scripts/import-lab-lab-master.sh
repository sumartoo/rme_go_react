#!/usr/bin/env bash
# Copies the lab examination groups (reflabmaster) from the legacy "lab" database.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-lab-master.sh            # only into an empty table
#   LAB_PASSWORD=... scripts/import-lab-lab-master.sh --replace  # wipe and copy again
#
# See scripts/lib/lab.sh for the optional LAB_* settings.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty reflabmaster "$REPLACE"

echo "Clearing app lab master..."
app -c "TRUNCATE reflabmaster RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy reflabmaster "labmasterid, labmastercode, labmastername, isdeleted, createddate, createby, modifieddate, modifiedby, deleteddate, deletedby"
# reset_identity assumes an "id" column; reflabmaster's is labmasterid.
app -c "SELECT setval(pg_get_serial_sequence('reflabmaster', 'labmasterid'), COALESCE((SELECT max(labmasterid) FROM reflabmaster), 0) + 1, false)" >/dev/null
echo "Done."
