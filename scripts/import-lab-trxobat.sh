#!/usr/bin/env bash
# Copies the stock movement log (trxobat, read by Stok Trace) from the legacy
# "lab" database. Needs the drugs and branches first (import-lab-obat.sh).
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-trxobat.sh            # only into an empty table
#   LAB_PASSWORD=... scripts/import-lab-trxobat.sh --replace  # wipe and copy again
#
# See scripts/lib/lab.sh for the optional LAB_* settings. Lines of drugs that
# no longer exist, or of branches missing from the app, are skipped.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty trxobat "$REPLACE"

echo "Clearing app stock movement log..."
app -c "TRUNCATE trxobat RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
ids=$(app -At -c "SELECT COALESCE(string_agg(id::text, ','), '0') FROM refobat")
copy trxobat "id, id_obat, cabang, laststock, inout, byk, stock, systemstock, id_ref, table_ref, remarks, createdby, createddate" \
  "WHERE id_obat IN ($ids) AND cabang IN ($(app_branches))"
reset_identity trxobat
echo "Done."
