#!/usr/bin/env bash
# Copies procedures (reftindakan + refjenistindakan) and their prices per branch
# (reftindakandetail) from the legacy "lab" database into the app database.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-tindakan.sh            # only into empty tables
#   LAB_PASSWORD=... scripts/import-lab-tindakan.sh --replace  # wipe app procedure data first
#
# See scripts/lib/lab.sh for the optional LAB_* settings. Duplicate price rows
# for the same procedure and branch are imported once (the lowest id), and rows
# for branches missing from the app are skipped.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty reftindakan "$REPLACE"

echo "Clearing app procedure tables..."
# CASCADE also empties the doctors' procedures (trxtindakan): re-run import-lab-dokter.sh afterwards.
app -c "TRUNCATE reftindakandetail, reftindakan, refjenistindakan RESTART IDENTITY CASCADE"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy refjenistindakan "id, info"
copy reftindakan "id, nama, id_jenis, hbeli, hjual, create_user, create_date, update_user, update_date,
                  sts_ctk_krtu, isedit, isactive, hjual_jamkes"

cols="id, id_tindakan, cabang, hbeli, hjual, hjual_jamkes, isactive"
copy_query reftindakandetail "$cols" \
  "SELECT DISTINCT ON (id_tindakan, cabang) $cols FROM reftindakandetail
   WHERE cabang IN ($(app_branches)) ORDER BY id_tindakan, cabang, id"

reset_identity refjenistindakan reftindakan reftindakandetail
echo "Done."
