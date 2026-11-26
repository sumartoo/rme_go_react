package analitik

import (
	"context"
	"fmt"
)

// Kelompok values of Kinerja: how sales are grouped.
var kelompokExpr = map[string]string{
	"tanggal":     `to_char(j.tgl::date, 'YYYY-MM-DD')`,
	"minggu":      `to_char(date_trunc('week', j.tgl)::date, 'YYYY-MM-DD')`, // Monday of the week
	"bulan":       `to_char(j.tgl, 'YYYY-MM')`,
	"hari":        `extract(isodow FROM j.tgl)::int::text`, // 1 = Senin … 7 = Minggu
	"dokter":      `COALESCE(NULLIF(trim(dk.nama), ''), '(tanpa dokter)')`,
	"penjamin":    `COALESCE(jk.nama, 'Umum / pribadi')`,
	"jenis-rawat": `COALESCE(r.info, '(tanpa jenis rawat)')`,
	"cabang":      `COALESCE(b.nama, j.cabang)`,
}

// timeKelompok are the groupings listed in time order (the others by revenue).
var timeKelompok = map[string]bool{"tanggal": true, "minggu": true, "bulan": true, "hari": true}

var namaHari = map[string]string{"1": "Senin", "2": "Selasa", "3": "Rabu", "4": "Kamis", "5": "Jumat", "6": "Sabtu", "7": "Minggu"}

// ValidKelompok reports whether k is a Kinerja grouping.
func ValidKelompok(k string) bool {
	_, ok := kelompokExpr[k]
	return ok
}

// KinerjaRow is the sales of one group. Cancelled sales are left out.
type KinerjaRow struct {
	Kelompok   string  `json:"kelompok"`
	Transaksi  int     `json:"transaksi"` // sales, over-the-counter included
	Kunjungan  int     `json:"kunjungan"` // sales of a registered visit
	Pasien     int     `json:"pasien"`    // distinct patients
	Pendapatan float64 `json:"pendapatan"`
	Obat       float64 `json:"obat"`
	Tindakan   float64 `json:"tindakan"`
	Penunjang  float64 `json:"penunjang"`
}

// KinerjaParams selects the sales and their grouping.
type KinerjaParams struct {
	Branches []string
	From, To string // YYYY-MM-DD, inclusive
	Kelompok string // a key of kelompokExpr
}

// Terlaris is a drug or a procedure ranked by quantity or value.
type Terlaris struct {
	Nama      string  `json:"nama"`
	Satuan    string  `json:"satuan,omitempty"`
	Jumlah    float64 `json:"jumlah"`
	Nilai     float64 `json:"nilai"` // sale price × quantity
	Transaksi int     `json:"transaksi"`
}

const salesFrom = ` FROM trxjual j
	LEFT JOIN trxjualtotal t ON t.id_jual = j.id
	LEFT JOIN refmdokter dk ON dk.id = j.id_dok
	LEFT JOIN refjaminankesehatan jk ON jk.id = j.id_jamkes
	LEFT JOIN refjenisrawat r ON r.id = j.id_jenis_rwt
	LEFT JOIN refbranch b ON b.kode = j.cabang
	WHERE j.cabang = ANY($1) AND j.tgl >= $2::date AND j.tgl < $3::date + 1 AND COALESCE(t.status, 0) <> 2`

// Kinerja groups the sales of the branches over the period (at most 400 groups).
func (p *PostgresRepository) Kinerja(ctx context.Context, kp KinerjaParams) ([]KinerjaRow, error) {
	expr, ok := kelompokExpr[kp.Kelompok]
	if !ok {
		return nil, fmt.Errorf("unknown kelompok %q", kp.Kelompok)
	}
	order := "1"
	if !timeKelompok[kp.Kelompok] {
		order = "5 DESC, 1"
	}
	rows, err := p.pool.Query(ctx, `SELECT `+expr+`, count(*), count(j.id_kun), count(DISTINCT j.id_pas),
			COALESCE(sum(t.total), 0), COALESCE(sum(t.tot_obat), 0), COALESCE(sum(t.tot_tndk), 0), COALESCE(sum(t.tot_pnjng), 0)`+
		salesFrom+` GROUP BY 1 ORDER BY `+order+` LIMIT 400`, kp.Branches, kp.From, kp.To)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []KinerjaRow{}
	for rows.Next() {
		var k KinerjaRow
		if err := rows.Scan(&k.Kelompok, &k.Transaksi, &k.Kunjungan, &k.Pasien, &k.Pendapatan, &k.Obat, &k.Tindakan, &k.Penunjang); err != nil {
			return nil, err
		}
		if kp.Kelompok == "hari" {
			k.Kelompok = namaHari[k.Kelompok]
		}
		out = append(out, k)
	}
	return out, rows.Err()
}

// ObatTerlaris ranks the drugs sold over the period by quantity or value.
func (p *PostgresRepository) ObatTerlaris(ctx context.Context, branches []string, from, to string, byNilai bool, limit int) ([]Terlaris, error) {
	order := "3 DESC"
	if byNilai {
		order = "4 DESC"
	}
	return p.terlaris(ctx, `SELECT COALESCE(o.nama, '(obat ' || ob.id_obat || ')'), COALESCE(st.info, ''),
			sum(COALESCE(ob.byk, 0)), sum(COALESCE(ob.byk, 0) * COALESCE(ob.hjual, 0)), count(DISTINCT j.id)
		FROM trxjualobat ob
		JOIN trxjual j ON j.id = ob.id_jual
		LEFT JOIN trxjualtotal t ON t.id_jual = j.id
		LEFT JOIN refobat o ON o.id = ob.id_obat
		LEFT JOIN refsatuan st ON st.id = o.sat_jual
		WHERE j.cabang = ANY($1) AND j.tgl >= $2::date AND j.tgl < $3::date + 1 AND COALESCE(t.status, 0) <> 2
		GROUP BY ob.id_obat, o.nama, st.info ORDER BY `+order+` LIMIT $4`, branches, from, to, limit)
}

// TindakanTerlaris ranks the procedures over the period by count or value.
func (p *PostgresRepository) TindakanTerlaris(ctx context.Context, branches []string, from, to string, byNilai bool, limit int) ([]Terlaris, error) {
	order := "3 DESC"
	if byNilai {
		order = "4 DESC"
	}
	return p.terlaris(ctx, `SELECT COALESCE(tn.nama, '(tindakan ' || x.id_tndk || ')'), '',
			sum(COALESCE(x.byk, 1)), sum(COALESCE(x.byk, 1) * COALESCE(x.hjual, 0)), count(DISTINCT j.id)
		FROM trxjualtindakan x
		JOIN trxjual j ON j.id = x.id_jual
		LEFT JOIN trxjualtotal t ON t.id_jual = j.id
		LEFT JOIN reftindakan tn ON tn.id = x.id_tndk
		WHERE j.cabang = ANY($1) AND j.tgl >= $2::date AND j.tgl < $3::date + 1 AND COALESCE(t.status, 0) <> 2
		GROUP BY x.id_tndk, tn.nama ORDER BY `+order+` LIMIT $4`, branches, from, to, limit)
}

func (p *PostgresRepository) terlaris(ctx context.Context, sql string, args ...any) ([]Terlaris, error) {
	rows, err := p.pool.Query(ctx, sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Terlaris{}
	for rows.Next() {
		var t Terlaris
		if err := rows.Scan(&t.Nama, &t.Satuan, &t.Jumlah, &t.Nilai, &t.Transaksi); err != nil {
			return nil, err
		}
		t.Jumlah, t.Nilai = round2(t.Jumlah), round2(t.Nilai)
		out = append(out, t)
	}
	return out, rows.Err()
}

// SumKinerja adds up rows into one total (Kelompok "total"); Pasien is the
// largest group's count, since distinct patients do not add up across groups.
func SumKinerja(rows []KinerjaRow) KinerjaRow {
	t := KinerjaRow{Kelompok: "total"}
	for _, r := range rows {
		t.Transaksi += r.Transaksi
		t.Kunjungan += r.Kunjungan
		t.Pasien = max(t.Pasien, r.Pasien)
		t.Pendapatan += r.Pendapatan
		t.Obat += r.Obat
		t.Tindakan += r.Tindakan
		t.Penunjang += r.Penunjang
	}
	return t
}

// PersenBerubah is the change from before to now in percent (nil when before is 0).
func PersenBerubah(before, now float64) *float64 {
	if before == 0 {
		return nil
	}
	v := round2((now - before) / before * 100)
	return &v
}
