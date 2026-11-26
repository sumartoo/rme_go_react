// Package riwayatpasien serves Front Office Klinik > Riwayat Pasien: a patient's
// visits at the user's branches (examination, vital signs, diagnoses,
// prescriptions, procedures) and a structured summary of them — recurring
// diagnoses, frequent drugs, the blood pressure and weight trend and the
// allergy note. The summary is computed here; no AI and no external service
// sees patient data.
package riwayatpasien

import (
	"context"
	"errors"
	"sort"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/pendaftaran"
)

// ErrNotFound is returned for a patient the user may not see.
var ErrNotFound = errors.New("pasien not found")

// MaxKunjungan caps the visits read for one patient (newest first).
const MaxKunjungan = 200

// Pasien is the patient's identity.
type Pasien struct {
	ID           int     `json:"id"`
	NoRM         string  `json:"noRm"`
	Nama         string  `json:"nama"`
	TglLahir     *string `json:"tglLahir"`
	Usia         *int    `json:"usia"`
	JenisKelamin string  `json:"jenisKelamin"`
	Alamat       string  `json:"alamat"`
	Telp         string  `json:"telp"`
	Alergi       string  `json:"alergi"`
	Penjamin     string  `json:"penjamin"`
	Cabang       string  `json:"cabang"`
}

// Item is a diagnosis, drug or procedure of a visit.
type Item struct {
	Kode  string   `json:"kode,omitempty"`
	Nama  string   `json:"nama"`
	Byk   *float64 `json:"byk,omitempty"`
	Dosis string   `json:"dosis,omitempty"`
}

// Kunjungan is one sale of the patient with its examination.
type Kunjungan struct {
	IDJual      int      `json:"idJual"`
	Tgl         string   `json:"tgl"` // YYYY-MM-DD HH:MM
	NoKwi       string   `json:"noKwi"`
	Cabang      string   `json:"cabang"`
	Dokter      string   `json:"dokter"`
	JenisRawat  string   `json:"jenisRawat"`
	Batal       bool     `json:"batal"`
	Total       *float64 `json:"total"`
	Keluhan     string   `json:"keluhan"`
	Anamnesis   string   `json:"anamnesis"`
	Sistole     *int     `json:"sistole"`
	Diastole    *int     `json:"diastole"`
	BB          *int     `json:"bb"`
	TB          *int     `json:"tb"`
	Suhu        *float64 `json:"suhu"`
	HasilDokter string   `json:"hasilDokter"`
	Rencana     string   `json:"rencana"`
	Diagnosa    []Item   `json:"diagnosa"`
	Resep       []Item   `json:"resep"`
	Tindakan    []Item   `json:"tindakan"`
}

// Riwayat is what the repository reads for one patient.
type Riwayat struct {
	Pasien    Pasien      `json:"pasien"`
	Kunjungan []Kunjungan `json:"kunjungan"` // newest first, at the caller's branches
}

type Repository interface {
	UserBranches(ctx context.Context, userID string) ([]branchauth.Branch, error)
	SearchPasien(ctx context.Context, branches []string, q string, limit, offset int) (pendaftaran.PasienPage, error)
	Riwayat(ctx context.Context, branches []string, id int) (Riwayat, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
	reg  *pendaftaran.PostgresRepository
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool, reg: pendaftaran.NewPostgresRepository(pool)}
}

func (p *PostgresRepository) UserBranches(ctx context.Context, userID string) ([]branchauth.Branch, error) {
	return branchauth.ForUser(ctx, p.pool, userID)
}

// SearchPasien is the Pendaftaran patient search (patients of the branches).
func (p *PostgresRepository) SearchPasien(ctx context.Context, branches []string, q string, limit, offset int) (pendaftaran.PasienPage, error) {
	return p.reg.SearchPasien(ctx, branches, pendaftaran.PasienSearch{Q: q}, limit, offset)
}

// Riwayat reads a patient registered at, or with visits at, one of the branches.
func (p *PostgresRepository) Riwayat(ctx context.Context, branches []string, id int) (Riwayat, error) {
	r := Riwayat{Kunjungan: []Kunjungan{}}
	err := p.pool.QueryRow(ctx, `SELECT p.id, p.no_rm, p.nama, to_char(p.tgl_lahir, 'YYYY-MM-DD'),
			date_part('year', age(current_date, p.tgl_lahir))::int, COALESCE(g.info, ''), COALESCE(p.alamat, ''),
			COALESCE(NULLIF(trim(p.hp), ''), NULLIF(trim(p.telp), ''), ''), COALESCE(trim(p.alergi), ''),
			COALESCE(jk.nama, NULLIF(p.penanggung, ''), ''), COALESCE(p.cabang, '')
		FROM refmpasien p
		LEFT JOIN refgender g ON g.id = p.id_gender
		LEFT JOIN refjaminankesehatan jk ON jk.id = p.id_jamkes
		WHERE p.id = $1 AND (p.cabang = ANY($2) OR EXISTS (SELECT 1 FROM trxjual j WHERE j.id_pas = p.id AND j.cabang = ANY($2)))`,
		id, branches).Scan(&r.Pasien.ID, &r.Pasien.NoRM, &r.Pasien.Nama, &r.Pasien.TglLahir, &r.Pasien.Usia, &r.Pasien.JenisKelamin,
		&r.Pasien.Alamat, &r.Pasien.Telp, &r.Pasien.Alergi, &r.Pasien.Penjamin, &r.Pasien.Cabang)
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	if err != nil {
		return r, err
	}
	rows, err := p.pool.Query(ctx, `SELECT j.id, to_char(j.tgl, 'YYYY-MM-DD HH24:MI'), COALESCE(j.no_kwi, ''), COALESCE(j.cabang, ''),
			COALESCE(trim(dk.nama), ''), COALESCE(rw.info, ''), COALESCE(t.status, 0) = 2, t.total,
			COALESCE(pm.keluhan_utama, ''), COALESCE(pm.anamnesis, ''),
			NULLIF(pm.tekanan_darah_sistole, 0), NULLIF(pm.tekanan_darah_diastole, 0), NULLIF(pm.berat_badan, 0),
			NULLIF(pm.tinggi_badan, 0), NULLIF(pm.suhu, 0), COALESCE(pm.hasil_pemeriksaan_dokter, ''), COALESCE(pm.rencana_penatalaksanaan, '')
		FROM trxjual j
		LEFT JOIN trxjualtotal t ON t.id_jual = j.id
		LEFT JOIN refmdokter dk ON dk.id = j.id_dok
		LEFT JOIN refjenisrawat rw ON rw.id = j.id_jenis_rwt
		LEFT JOIN trxpemeriksaan pm ON pm.id_kun = j.id_kun
		WHERE j.id_pas = $1 AND j.cabang = ANY($2)
		ORDER BY j.tgl DESC, j.id DESC LIMIT $3`, id, branches, MaxKunjungan)
	if err != nil {
		return r, err
	}
	index := map[int]int{}
	ids := []int{}
	for rows.Next() {
		k := Kunjungan{Diagnosa: []Item{}, Resep: []Item{}, Tindakan: []Item{}}
		if err := rows.Scan(&k.IDJual, &k.Tgl, &k.NoKwi, &k.Cabang, &k.Dokter, &k.JenisRawat, &k.Batal, &k.Total,
			&k.Keluhan, &k.Anamnesis, &k.Sistole, &k.Diastole, &k.BB, &k.TB, &k.Suhu, &k.HasilDokter, &k.Rencana); err != nil {
			rows.Close()
			return r, err
		}
		index[k.IDJual] = len(r.Kunjungan)
		ids = append(ids, k.IDJual)
		r.Kunjungan = append(r.Kunjungan, k)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return r, err
	}
	if len(ids) == 0 {
		return r, nil
	}
	lines := []struct {
		sql string
		add func(k *Kunjungan, it Item)
	}{
		{`SELECT x.id_jual, COALESCE(i.kode_icd, ''), COALESCE(i.nama, '(ICD ' || x.icd_id || ')'), NULL::float8, ''
			FROM trxjualdiagnosa x LEFT JOIN reficd i ON i.id = x.icd_id WHERE x.id_jual = ANY($1) ORDER BY x.id`,
			func(k *Kunjungan, it Item) { k.Diagnosa = append(k.Diagnosa, it) }},
		{`SELECT ob.id_jual, '', COALESCE(o.nama, '(obat ' || ob.id_obat || ')'), ob.byk, COALESCE(ob.dosis, '')
			FROM trxjualobat ob LEFT JOIN refobat o ON o.id = ob.id_obat WHERE ob.id_jual = ANY($1) ORDER BY ob.id`,
			func(k *Kunjungan, it Item) { k.Resep = append(k.Resep, it) }},
		{`SELECT x.id_jual, '', COALESCE(tn.nama, '(tindakan ' || x.id_tndk || ')'), x.byk, ''
			FROM trxjualtindakan x LEFT JOIN reftindakan tn ON tn.id = x.id_tndk WHERE x.id_jual = ANY($1) ORDER BY x.id`,
			func(k *Kunjungan, it Item) { k.Tindakan = append(k.Tindakan, it) }},
	}
	for _, l := range lines {
		rows, err := p.pool.Query(ctx, l.sql, ids)
		if err != nil {
			return r, err
		}
		for rows.Next() {
			var idJual int
			var it Item
			if err := rows.Scan(&idJual, &it.Kode, &it.Nama, &it.Byk, &it.Dosis); err != nil {
				rows.Close()
				return r, err
			}
			if i, ok := index[idJual]; ok {
				l.add(&r.Kunjungan[i], it)
			}
		}
		rows.Close()
		if err := rows.Err(); err != nil {
			return r, err
		}
	}
	return r, nil
}

// Jumlah is a diagnosis or drug with how often it appears.
type Jumlah struct {
	Kode     string `json:"kode,omitempty"`
	Nama     string `json:"nama"`
	Kali     int    `json:"kali"`     // visits with it
	Terakhir string `json:"terakhir"` // YYYY-MM-DD of the last one
}

// Vital is one visit's vital signs, for the trend.
type Vital struct {
	Tgl      string   `json:"tgl"`
	Sistole  *int     `json:"sistole"`
	Diastole *int     `json:"diastole"`
	BB       *int     `json:"bb"`
	TB       *int     `json:"tb"`
	IMT      *float64 `json:"imt"` // body-mass index when weight and height are known
}

// Ringkasan summarises the visits: counts and trends only, computed from the
// records (nothing inferred).
type Ringkasan struct {
	JumlahKunjungan int      `json:"jumlahKunjungan"` // not cancelled
	Pertama         string   `json:"pertama"`
	Terakhir        string   `json:"terakhir"`
	Cabang          []string `json:"cabang"`
	Diagnosa        []Jumlah `json:"diagnosa"` // most frequent first
	Obat            []Jumlah `json:"obat"`     // most frequent first, at most 10
	Vital           []Vital  `json:"vital"`    // last 10 visits with a vital sign, oldest first
}

// Summarize builds the summary of the visits (newest first). Cancelled
// visits are left out.
func Summarize(visits []Kunjungan) Ringkasan {
	s := Ringkasan{Cabang: []string{}, Diagnosa: []Jumlah{}, Obat: []Jumlah{}, Vital: []Vital{}}
	dx := map[string]*Jumlah{}
	rx := map[string]*Jumlah{}
	cabang := map[string]bool{}
	count := func(m map[string]*Jumlah, key string, j Jumlah, day string) {
		if m[key] == nil {
			j.Terakhir = day
			m[key] = &j
		}
		m[key].Kali++
	}
	for _, k := range visits {
		if k.Batal {
			continue
		}
		day := k.Tgl
		if len(day) >= 10 {
			day = day[:10]
		}
		s.JumlahKunjungan++
		if s.Terakhir == "" {
			s.Terakhir = day
		}
		s.Pertama = day
		if !cabang[k.Cabang] {
			cabang[k.Cabang] = true
			s.Cabang = append(s.Cabang, k.Cabang)
		}
		seen := map[string]bool{}
		for _, d := range k.Diagnosa {
			key := d.Kode + "|" + d.Nama
			if !seen[key] {
				seen[key] = true
				count(dx, key, Jumlah{Kode: d.Kode, Nama: d.Nama}, day)
			}
		}
		for _, o := range k.Resep {
			if !seen["rx|"+o.Nama] {
				seen["rx|"+o.Nama] = true
				count(rx, o.Nama, Jumlah{Nama: o.Nama}, day)
			}
		}
		if len(s.Vital) < 10 && (k.Sistole != nil || k.BB != nil) {
			v := Vital{Tgl: day, Sistole: k.Sistole, Diastole: k.Diastole, BB: k.BB, TB: k.TB}
			if k.BB != nil && k.TB != nil && *k.TB >= 50 && *k.TB <= 250 {
				m := float64(*k.TB) / 100
				imt := float64(int(float64(*k.BB)/(m*m)*10+0.5)) / 10
				v.IMT = &imt
			}
			s.Vital = append(s.Vital, v)
		}
	}
	for i, j := 0, len(s.Vital)-1; i < j; i, j = i+1, j-1 {
		s.Vital[i], s.Vital[j] = s.Vital[j], s.Vital[i]
	}
	s.Diagnosa = ranked(dx, 0)
	s.Obat = ranked(rx, 10)
	return s
}

func ranked(m map[string]*Jumlah, limit int) []Jumlah {
	out := make([]Jumlah, 0, len(m))
	for _, j := range m {
		out = append(out, *j)
	}
	sort.SliceStable(out, func(a, b int) bool {
		if out[a].Kali != out[b].Kali {
			return out[a].Kali > out[b].Kali
		}
		if out[a].Terakhir != out[b].Terakhir {
			return out[a].Terakhir > out[b].Terakhir
		}
		return out[a].Nama < out[b].Nama
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out
}
