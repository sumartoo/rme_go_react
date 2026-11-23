#!/usr/bin/env bash
# Copies the client companies (refcompany) from the legacy "lab" database.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-perusahaan.sh            # only into an empty table
#   LAB_PASSWORD=... scripts/import-lab-perusahaan.sh --replace  # wipe and copy again
#
# See scripts/lib/lab.sh for the optional LAB_* settings.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty refcompany "$REPLACE"

echo "Clearing app companies..."
app -c "TRUNCATE refcompany RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy refcompany "companyid, companycode, companyname, companyaddress, companycontacperson, companymail, companyphone, companyfax, isvat"
# reset_identity assumes an "id" column; refcompany's is companyid.
app -c "SELECT setval(pg_get_serial_sequence('refcompany', 'companyid'), COALESCE((SELECT max(companyid) FROM refcompany), 0) + 1, false)" >/dev/null
echo "Done."
