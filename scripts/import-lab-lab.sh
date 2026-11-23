#!/usr/bin/env bash
# Copies the lab examinations (reflab) from the legacy "lab" database. Run
# import-lab-lab-master.sh and import-lab-lab-sub-master.sh too, so the Lab
# Master and Sub Master names show.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-lab.sh            # only into an empty table
#   LAB_PASSWORD=... scripts/import-lab-lab.sh --replace  # wipe and copy again
#
# See scripts/lib/lab.sh for the optional LAB_* settings.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty reflab "$REPLACE"

echo "Clearing app lab examinations..."
app -c "TRUNCATE reflab RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy reflab "labid, labsubmasterid, labcode, labname, isdeleted, createddate, createby, modifieddate, modifiedby,
  deleteddate, deletedby, price, fee"
# reset_identity assumes an "id" column; reflab's is labid.
app -c "SELECT setval(pg_get_serial_sequence('reflab', 'labid'), COALESCE((SELECT max(labid) FROM reflab), 0) + 1, false)" >/dev/null
echo "Done."
