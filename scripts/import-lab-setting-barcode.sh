#!/usr/bin/env bash
# Copies the MCU barcode label layout (refsettingbarcode) from the legacy "lab" database.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-setting-barcode.sh            # only into an empty table
#   LAB_PASSWORD=... scripts/import-lab-setting-barcode.sh --replace  # wipe and copy again
#
# See scripts/lib/lab.sh for the optional LAB_* settings.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty refsettingbarcode "$REPLACE"

echo "Clearing app barcode setting..."
app -c "TRUNCATE refsettingbarcode RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy refsettingbarcode 'id, "left", middle, "right", row1, row2, row3, row4, row5, row6, barcodewidth, barcodeheight, barcodeleft, barcodetop, fontsize, isactive'
reset_identity refsettingbarcode
echo "Done."
