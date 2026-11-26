package analitik

import (
	"context"
	"fmt"
)

// CekKualitas is one data-quality check: how many records it flags and the
// first ones, as table cells. Checks only flag; nothing is changed.
type CekKualitas struct {
	Kode       string     `json:"kode"`
	Judul      string     `json:"judul"`
	Penjelasan string     `json:"penjelasan"`
	Jumlah     int        `json:"jumlah"`
	Kolom      []string   `json:"kolom"`
	Contoh     [][]string `json:"contoh"`
}

// MaxContoh caps the example rows of a check.
const MaxContoh = 50

// cekDef is a check: its SQL selects the example cells as text, the last
// column being count(*) OVER () (the total), with $1 the branches, $2
// today's clinic date (every check references it: pgx refuses unused
// arguments) and LIMIT $3.
type cekDef struct {
	kode, judul, penjelasan string
	kolom                   []string
	sql                     string
}

var cekDefs = []cekDef{
	{
		kode: "pasien-ganda", judul: "Kandidat pasien ganda",
		penjelasan: "Pasien dengan nama dan tanggal lahir yang sama tapi nomor RM berbeda. Periksa apakah orang yang sama terdaftar dua kali.",
		kolom:      []string{"Nama", "Tgl lahir", "Jumlah RM", "No RM"},
		sql: `SELECT min(p.nama), to_char(p.tgl_lahir, 'YYYY-MM-DD'), count(*)::text,
				left(string_agg(p.no_rm, ', ' ORDER BY p.no_rm), 120), count(*) OVER ()
			FROM refmpasien p
			WHERE p.cabang = ANY($1) AND $2::date IS NOT NULL AND p.tgl_lahir IS NOT NULL AND trim(p.nama) <> ''
			GROUP BY lower(regexp_replace(trim(p.nama), '\s+', ' ', 'g')), p.tgl_lahir
			HAVING count(*) > 1
			ORDER BY count(*) DESC, min(p.nama) LIMIT $3`,
	},
	{
		kode: "kunjungan-tanpa-diagnosa", judul: "Kunjungan tanpa diagnosa (90 hari)",
		penjelasan: "Transaksi dari pendaftaran (bukan OTC) yang tidak punya diagnosa ICD. Tren penyakit dan pemetaan diagnosa-obat jadi kurang lengkap.",
		kolom:      []string{"Tanggal", "No kuitansi", "No RM", "Pasien", "Dokter", "Cabang"},
		sql: `SELECT to_char(j.tgl, 'YYYY-MM-DD HH24:MI'), COALESCE(j.no_kwi, ''), COALESCE(p.no_rm, ''), COALESCE(p.nama, ''),
				COALESCE(trim(dk.nama), ''), j.cabang, count(*) OVER ()
			FROM trxjual j
			LEFT JOIN trxjualtotal t ON t.id_jual = j.id
			LEFT JOIN refmpasien p ON p.id = j.id_pas
			LEFT JOIN refmdokter dk ON dk.id = j.id_dok
			WHERE j.cabang = ANY($1) AND j.id_kun IS NOT NULL AND j.tgl >= $2::date - 89 AND j.tgl < $2::date + 1
				AND COALESCE(t.status, 0) <> 2
				AND NOT EXISTS (SELECT 1 FROM trxjualdiagnosa x WHERE x.id_jual = j.id)
			ORDER BY j.tgl DESC LIMIT $3`,
	},
	{
		kode: "stok-negatif", judul: "Stok negatif",
		penjelasan: "Stok tercatat di bawah nol: ada pengeluaran yang tidak diimbangi penerimaan. Perlu stok opname.",
		kolom:      []string{"Obat", "Cabang", "Stok"},
		sql: `SELECT r.nama, s.cabang, s.stok::text, count(*) OVER ()
			FROM trxstock s JOIN refobat r ON r.id = s.id_obat
			WHERE s.cabang = ANY($1) AND $2::date IS NOT NULL AND s.stok < 0
			ORDER BY s.stok, r.nama LIMIT $3`,
	},
	{
		kode: "harga-tidak-wajar", judul: "Harga pokok kosong atau tidak wajar",
		penjelasan: "Obat aktif yang harga pokoknya (HP) kosong atau lebih dari 1,2× harga jual, sering karena HP diisi harga per box. Estimasi biaya order mengabaikan obat ini.",
		kolom:      []string{"Obat", "Cabang", "HP", "Harga jual"},
		sql: `SELECT r.nama, s.cabang, COALESCE(s.hp, 0)::text, COALESCE(s.hjual, 0)::text, count(*) OVER ()
			FROM trxstock s JOIN refobat r ON r.id = s.id_obat
			WHERE s.cabang = ANY($1) AND $2::date IS NOT NULL AND s.isactive = 1 AND COALESCE(r.non_aktif, 0) <> 1
				AND NOT (s.hp > 0 AND s.hjual > 0 AND s.hp <= s.hjual * 1.2)
			ORDER BY r.nama, s.cabang LIMIT $3`,
	},
	{
		kode: "kedaluwarsa-default", judul: "Tanggal kedaluwarsa & batch tidak diisi (365 hari)",
		penjelasan: "Penerimaan barang yang tanggal kedaluwarsanya tepat 1 tahun setelah tanggal terima atau nomor batch kosong/0, tanda nilai bawaan. Risiko obat kedaluwarsa belum bisa dihitung sampai data ini diisi dengan benar.",
		kolom:      []string{"Tgl terima", "No faktur", "Obat", "Batch", "Tgl kedaluwarsa", "Cabang"},
		sql: `SELECT to_char(f.tgl, 'YYYY-MM-DD'), COALESCE(f.no_fak, ''), COALESCE(r.nama, ''), COALESCE(fo.no_batch, ''),
				COALESCE(to_char(fo.tgl_exp, 'YYYY-MM-DD'), ''), COALESCE(f.cabang, ''), count(*) OVER ()
			FROM trxfakturobat fo
			JOIN trxfaktur f ON f.id = fo.id_fak
			LEFT JOIN refobat r ON r.id = fo.id_obat
			WHERE f.cabang = ANY($1) AND f.tgl >= $2::date - 364 AND f.tgl < $2::date + 1
				AND (fo.tgl_exp IS NULL OR fo.tgl_exp = (f.tgl::date + interval '1 year')::date
					OR COALESCE(trim(fo.no_batch), '') IN ('', '0', '-'))
			ORDER BY f.tgl DESC LIMIT $3`,
	},
	{
		kode: "vital-tidak-wajar", judul: "Tanda vital tidak wajar (365 hari)",
		penjelasan: "Tekanan darah, berat atau tinggi badan di luar rentang yang mungkin, biasanya salah ketik.",
		kolom:      []string{"Tanggal", "No RM", "Pasien", "TD", "BB", "TB", "Cabang"},
		sql: `SELECT to_char(j.tgl, 'YYYY-MM-DD'), COALESCE(p.no_rm, ''), COALESCE(p.nama, ''),
				COALESCE(pm.tekanan_darah_sistole::text, '') || '/' || COALESCE(pm.tekanan_darah_diastole::text, ''),
				COALESCE(pm.berat_badan::text, ''), COALESCE(pm.tinggi_badan::text, ''), j.cabang, count(*) OVER ()
			FROM trxpemeriksaan pm
			JOIN trxjual j ON j.id_kun = pm.id_kun
			LEFT JOIN refmpasien p ON p.id = j.id_pas
			WHERE j.cabang = ANY($1) AND j.tgl >= $2::date - 364 AND j.tgl < $2::date + 1
				AND ((pm.tekanan_darah_sistole > 0 AND (pm.tekanan_darah_sistole < 50 OR pm.tekanan_darah_sistole > 300))
					OR pm.tekanan_darah_diastole > 200
					OR pm.berat_badan > 300
					OR pm.tinggi_badan > 250 OR (pm.tinggi_badan > 0 AND pm.tinggi_badan < 30))
			ORDER BY j.tgl DESC LIMIT $3`,
	},
	{
		kode: "pasien-tanpa-tgl-lahir", judul: "Pasien tanpa tanggal lahir",
		penjelasan: "Tanpa tanggal lahir, usia pasien tidak bisa dihitung untuk laporan dan analisa.",
		kolom:      []string{"No RM", "Nama", "Cabang"},
		sql: `SELECT p.no_rm, p.nama, COALESCE(p.cabang, ''), count(*) OVER ()
			FROM refmpasien p
			WHERE p.cabang = ANY($1) AND $2::date IS NOT NULL AND p.tgl_lahir IS NULL
			ORDER BY p.no_rm LIMIT $3`,
	},
}

// Kualitas runs every data-quality check on the branches.
func (p *PostgresRepository) Kualitas(ctx context.Context, branches []string, today string) ([]CekKualitas, error) {
	out := make([]CekKualitas, 0, len(cekDefs))
	for _, d := range cekDefs {
		c, err := p.runCek(ctx, d, branches, today)
		if err != nil {
			return nil, fmt.Errorf("cek %s: %w", d.kode, err)
		}
		out = append(out, c)
	}
	return out, nil
}

func (p *PostgresRepository) runCek(ctx context.Context, d cekDef, branches []string, today string) (CekKualitas, error) {
	c := CekKualitas{Kode: d.kode, Judul: d.judul, Penjelasan: d.penjelasan, Kolom: d.kolom, Contoh: [][]string{}}
	rows, err := p.pool.Query(ctx, d.sql, branches, today, MaxContoh)
	if err != nil {
		return c, err
	}
	defer rows.Close()
	for rows.Next() {
		cells := make([]string, len(d.kolom))
		dest := make([]any, len(d.kolom)+1)
		for i := range cells {
			dest[i] = &cells[i]
		}
		dest[len(cells)] = &c.Jumlah
		if err := rows.Scan(dest...); err != nil {
			return c, err
		}
		c.Contoh = append(c.Contoh, cells)
	}
	return c, rows.Err()
}
