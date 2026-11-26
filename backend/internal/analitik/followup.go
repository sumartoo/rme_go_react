package analitik

import (
	"context"
	"fmt"
	"strings"
)

// KelompokKronis is a group of chronic diagnoses followed up, by ICD-10 code.
type KelompokKronis struct {
	Kode  string `json:"kode"`
	Nama  string `json:"nama"`
	Regex string `json:"-"` // on reficd.kode_icd
}

// Kronis are the chronic groups whose patients are expected back for control.
var Kronis = []KelompokKronis{
	{Kode: "hipertensi", Nama: "Hipertensi (I10–I15)", Regex: `^I1[0-5]`},
	{Kode: "diabetes", Nama: "Diabetes melitus (E10–E14)", Regex: `^E1[0-4]`},
	{Kode: "dislipidemia", Nama: "Dislipidemia (E78)", Regex: `^E78`},
	{Kode: "asma", Nama: "Asma (J45)", Regex: `^J45`},
}

// FollowUpRow is a chronic patient who has not been back.
type FollowUpRow struct {
	IDPasien      int    `json:"idPasien"`
	NoRM          string `json:"noRm"`
	Nama          string `json:"nama"`
	Telp          string `json:"telp"`
	Kelompok      string `json:"kelompok"` // a KelompokKronis.Kode
	KodeICD       string `json:"kodeIcd"`  // the last diagnosis of the group
	Terakhir      string `json:"terakhir"` // YYYY-MM-DD of the last visit
	HariSejak     int    `json:"hariSejak"`
	JumlahKontrol int    `json:"jumlahKontrol"` // visit days with the diagnosis in the last year
	Cabang        string `json:"cabang"`
}

// FollowUpParams selects the patients to follow up.
type FollowUpParams struct {
	Branches []string
	Today    string // YYYY-MM-DD
	Hari     int    // no visit for at least this many days
	Kelompok string // a KelompokKronis.Kode, or empty for all
	Limit    int
}

// FollowUpPage is a page of patients to follow up and their total.
type FollowUpPage struct {
	Rows  []FollowUpRow `json:"rows"`
	Total int           `json:"total"`
}

// kronisCase is the SQL CASE naming the chronic group of an ICD code.
func kronisCase(groups []KelompokKronis) string {
	var b strings.Builder
	b.WriteString("CASE")
	for _, g := range groups {
		fmt.Fprintf(&b, " WHEN i.kode_icd ~ '%s' THEN '%s'", g.Regex, g.Kode) // constants, not input
	}
	b.WriteString(" END")
	return b.String()
}

// FollowUp lists patients with a chronic diagnosis on at least two visit days
// in the last year whose last visit (for any reason) is at least Hari days
// ago: the patients who did not come back for control.
func (p *PostgresRepository) FollowUp(ctx context.Context, fp FollowUpParams) (FollowUpPage, error) {
	page := FollowUpPage{Rows: []FollowUpRow{}}
	groups := Kronis
	if fp.Kelompok != "" {
		groups = nil
		for _, g := range Kronis {
			if g.Kode == fp.Kelompok {
				groups = []KelompokKronis{g}
			}
		}
		if groups == nil {
			return page, fmt.Errorf("unknown kelompok %q", fp.Kelompok)
		}
	}
	rows, err := p.pool.Query(ctx, `WITH dx AS (
			SELECT j.id_pas, j.cabang, j.tgl, i.kode_icd, `+kronisCase(groups)+` AS kelompok
			FROM trxjualdiagnosa x
			JOIN trxjual j ON j.id = x.id_jual
			JOIN reficd i ON i.id = x.icd_id
			LEFT JOIN trxjualtotal t ON t.id_jual = j.id
			WHERE j.cabang = ANY($1) AND j.tgl >= $2::date - 364 AND j.tgl < $2::date + 1 AND COALESCE(t.status, 0) <> 2
				AND j.id_pas IS NOT NULL),
		per AS (
			SELECT id_pas, kelompok, count(DISTINCT tgl::date) AS kontrol,
				(array_agg(kode_icd ORDER BY tgl DESC))[1] AS kode
			FROM dx WHERE kelompok IS NOT NULL
			GROUP BY id_pas, kelompok
			HAVING count(DISTINCT tgl::date) >= 2),
		akhir AS (
			SELECT j.id_pas, max(j.tgl) AS terakhir, (array_agg(j.cabang ORDER BY j.tgl DESC))[1] AS cabang
			FROM trxjual j JOIN (SELECT DISTINCT id_pas FROM per) q ON q.id_pas = j.id_pas
			WHERE j.tgl < $2::date + 1
			GROUP BY j.id_pas)
		SELECT pa.id, pa.no_rm, pa.nama, COALESCE(NULLIF(trim(pa.hp), ''), NULLIF(trim(pa.telp), ''), ''),
			per.kelompok, per.kode, to_char(a.terakhir, 'YYYY-MM-DD'), ($2::date - a.terakhir::date), per.kontrol, a.cabang,
			count(*) OVER ()
		FROM per
		JOIN akhir a ON a.id_pas = per.id_pas
		JOIN refmpasien pa ON pa.id = per.id_pas
		WHERE a.terakhir < $2::date - $3::int + 1 AND a.cabang = ANY($1)
		ORDER BY a.terakhir DESC, pa.nama
		LIMIT $4`, fp.Branches, fp.Today, fp.Hari, fp.Limit)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var r FollowUpRow
		if err := rows.Scan(&r.IDPasien, &r.NoRM, &r.Nama, &r.Telp, &r.Kelompok, &r.KodeICD, &r.Terakhir, &r.HariSejak,
			&r.JumlahKontrol, &r.Cabang, &page.Total); err != nil {
			return page, err
		}
		page.Rows = append(page.Rows, r)
	}
	return page, rows.Err()
}
