#!/usr/bin/env bash
# Copies the lab examination subgroups (reflabsubmaster) from the legacy "lab"
# database. Run import-lab-lab-master.sh too, so the Lab Master names show.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-lab-sub-master.sh            # only into an empty table
#   LAB_PASSWORD=... scripts/import-lab-lab-sub-master.sh --replace  # wipe and copy again
#
# See scripts/lib/lab.sh for the optional LAB_* settings.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty reflabsubmaster "$REPLACE"

echo "Clearing app lab sub master..."
app -c "TRUNCATE reflabsubmaster RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy reflabsubmaster "labsubmasterid, labmasterid, labsubmastercode, labsubmastername, isdeleted, createddate, createby, modifieddate, modifiedby, deleteddate, deletedby"
# reset_identity assumes an "id" column; reflabsubmaster's is labsubmasterid.
app -c "SELECT setval(pg_get_serial_sequence('reflabsubmaster', 'labsubmasterid'), COALESCE((SELECT max(labsubmasterid) FROM reflabsubmaster), 0) + 1, false)" >/dev/null
echo "Done."
