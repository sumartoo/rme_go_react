#!/usr/bin/env bash
# Copies the MCU examination catalogue (mcu_group, mcu_type, mcu_items,
# resulted_type, phisic_resulted_type) and the MCU registrations (trxmcuheader
# with their ordered examinations, trxmcudetail, non-lab services,
# trxmcunonlab, and doctors, trxmcudoctor, plus the refmcukategori and
# refmcuinfo choices) from the legacy "lab"
# database. The MCU result forms (mcu_hasil_*) are new and are not touched.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-mcu.sh            # only into empty tables
#   LAB_PASSWORD=... scripts/import-lab-mcu.sh --replace  # wipe and copy again
#
# --replace keeps the results already entered: they point at registration ids,
# which are copied unchanged.
#
# See scripts/lib/lab.sh for the optional LAB_* settings.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty mcu_items "$REPLACE"
require_empty trxmcuheader "$REPLACE"

echo "Clearing the app's MCU catalogue and registrations..."
app -c "TRUNCATE mcu_group, mcu_type, mcu_items, resulted_type, phisic_resulted_type, trxmcuheader, trxmcudetail, trxmcunonlab, trxmcudoctor, refmcukategori, refmcuinfo"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy mcu_group "group_type, group_name, default_flag, detail_flag, sort_id, input_date, input_by"
copy mcu_type "check_up_type, group_type, name, confirm_date, expired_date"
copy_query mcu_items "check_up_type, item_id, item_description, confirm_date, expired_date" \
  "SELECT DISTINCT ON (check_up_type, item_id) check_up_type, item_id, item_description, confirm_date, expired_date FROM mcu_items ORDER BY check_up_type, item_id, confirm_date DESC NULLS LAST"
copy_query resulted_type "resulted_type, resulted_name, entry_type, group_type, confirm_date" \
  "SELECT DISTINCT ON (resulted_type) resulted_type, resulted_name, entry_type, group_type, confirm_date FROM resulted_type ORDER BY resulted_type, confirm_date DESC NULLS LAST"
copy phisic_resulted_type "resulted_type, check_up_type, item_id, sort_id"
copy trxmcuheader "id, pasienid, mcudate, status, createddate, createby, modifieddate, modifiedby, deleteddate, deletedby, price,
  resume, inviceno, invoicedate, cancelremarks, canceldate, cancelby, mcukategoriid, mcuinfoid"
copy trxmcudetail "id, headerid, paketid, labid, status, result, price"
copy trxmcunonlab "id, headerid, nonlabid, price"
copy trxmcudoctor "id, headerid, doctor, feepercentage"
copy refmcukategori "id, mcukategori"
copy refmcuinfo "id, mcuinfo"
reset_identity trxmcuheader trxmcudetail trxmcunonlab trxmcudoctor
echo "Done."
