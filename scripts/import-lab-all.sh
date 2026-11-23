#!/usr/bin/env bash
# Runs every scripts/import-lab-*.sh in dependency order, so the app database
# gets all the data from the legacy "lab" database in one go.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-all.sh            # only into empty tables
#   LAB_PASSWORD=... scripts/import-lab-all.sh --replace  # wipe and copy everything again
#
# --replace empties the app's copies first, so rows entered in the app since
# the last import are lost. Stops at the first import that fails.
# See scripts/lib/lab.sh for the optional LAB_* settings and DATABASE_URL.
set -euo pipefail
cd "$(dirname "$0")/.."
: "${LAB_PASSWORD:?Set LAB_PASSWORD to the password of the lab database}"

# Order matters: dokter needs pasien and tindakan (whose --replace empties the
# doctors' procedures), jamkes needs pasien, pengobatan needs pasien, dokter and
# pendaftaran, and the detail lines need the sales.
for s in obat tindakan penunjang icd perujuk-otc pasien jamkes dokter \
         pendaftaran organisasi-lokasi pemeriksaan pengobatan pengobatan-detail \
         penerimaan-barang; do
  echo "===== import-lab-$s ====="
  bash "scripts/import-lab-$s.sh" "$@"
done
echo "All imports done."
