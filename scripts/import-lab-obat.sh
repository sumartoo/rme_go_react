#!/usr/bin/env bash
# Copies the drug master (refobat + lookups) and stock per branch (trxstock)
# from the legacy "lab" database into the app database.
#
# Usage (from the project root, with the stack running):
#   LAB_PASSWORD=... scripts/import-lab-obat.sh            # only into empty tables
#   LAB_PASSWORD=... scripts/import-lab-obat.sh --replace  # wipe app drug data first
#
# See scripts/lib/lab.sh for the optional LAB_* settings. Stock rows for drugs
# that no longer exist in lab, or for branches missing from the app, are skipped.
set -euo pipefail
cd "$(dirname "$0")/.."
source scripts/lib/lab.sh
REPLACE=0
[[ "${1:-}" == "--replace" ]] && REPLACE=1

require_empty refobat "$REPLACE"

echo "Clearing app drug tables..."
app -c "TRUNCATE trxobat, trxstokadjus, trxstock, refobat, refjenisobat, refsatuan, reflogoobat, refprodusen, refkodemovingobat,
        refkodemovingpo, refdistributorobat, refprincipalobat, refgolonganobat RESTART IDENTITY"

echo "Copying from $LAB_USER@$LAB_HOST:$LAB_PORT/$LAB_DB ..."
copy refjenisobat       "id, info, sts_hmlm"
copy refsatuan          "id, info"
copy reflogoobat        "id, info"
copy refprodusen        "id, nama, alamat, telp, fax, create_user, create_date, update_user, update_date"
copy refkodemovingobat  "id, kodemoving"
copy refkodemovingpo    "id, kodemovingpo"
copy refdistributorobat "id, distributor, alamat, telp, fax, cp, telp_cp, create_user, create_date, update_user, update_date"
copy refprincipalobat   "id, principal"
copy refgolonganobat    "id, golonganobat"
copy refobat "id, nama, isi, id_jenis, id_logo, id_prod, sat_beli, sat_jual, qjb, hbeli, hjual, hp, non_aktif,
              create_user, create_date, update_user, update_date, id_moving, id_distributor, id_principal,
              id_golongan_obat, komposisi, id_moving_po, kena_pajak, isi_per_box"

ids=$(app -At -c "SELECT COALESCE(string_agg(id::text, ','), '0') FROM refobat")
copy trxstock "id_obat, stok, stok_min, cabang, hbeli, hp, hjual, isactive" \
  "WHERE id_obat IN ($ids) AND cabang IN ($(app_branches))"

reset_identity refjenisobat refsatuan reflogoobat refprodusen refkodemovingobat refkodemovingpo \
  refdistributorobat refprincipalobat refgolonganobat refobat
echo "Done."
