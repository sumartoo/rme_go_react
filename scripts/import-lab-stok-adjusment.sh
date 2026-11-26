#!/usr/bin/env bash
# Copies the stock adjustments (trxstokadjus) from the legacy "lab" database.
# Needs the drugs and branches first (import-lab-obat.sh).
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-stok-adjusment.sh            # only into an empty table
#   LAB_PASSWORD=... scripts/import-lab-stok-adjusment.sh --replace  # wipe and copy again
#
# See scripts/lib/lab.sh for the optional LAB_* settings. Adjustments of drugs
# that no longer exist, or of branches missing from the app, are skipped.
# Only the history is copied: trxstock.stok already holds their effect.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty trxstokadjus "$REPLACE"

echo "Clearing app stock adjustments..."
app -c "TRUNCATE trxstokadjus RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
ids=$(app -At -c "SELECT COALESCE(string_agg(id::text, ','), '0') FROM refobat")
copy trxstokadjus "id, tanggal, id_obat, ket, byk, status, create_user, create_date, update_user, update_date, cabang" \
  "WHERE id_obat IN ($ids) AND cabang IN ($(app_branches))"
reset_identity trxstokadjus
echo "Done."
