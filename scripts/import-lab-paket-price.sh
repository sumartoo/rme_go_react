#!/usr/bin/env bash
# Copies the MCU packages from the legacy "lab" database: the categories
# (refcategory), the packages (refpriceheader) and their lab examinations
# (refpricedetail). Run import-lab-perusahaan.sh and import-lab-lab.sh too, so
# the company and examination names show.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-paket-price.sh            # only into empty tables
#   LAB_PASSWORD=... scripts/import-lab-paket-price.sh --replace  # wipe and copy again
#
# See scripts/lib/lab.sh for the optional LAB_* settings.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty refpriceheader "$REPLACE"
require_empty refpricedetail "$REPLACE"

echo "Clearing app packages..."
app -c "TRUNCATE refpricedetail, refpriceheader, refcategory RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy refcategory "id, category, note"
copy refpriceheader "id, companyid, paketname, ispaket, fromdate, todate, price, isdeleted, createddate, createby,
  modifieddate, modifiedby, deleteddate, deletedby, kategoriid, fee, isactive"
copy refpricedetail "id, headerid, labid, iscek, price, percentage1, percentage2, percentage3"
reset_identity refcategory refpriceheader refpricedetail
echo "Done."
