package analitik

import (
	"context"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/icd"
)

// ObatPrediksi is the forecast of one drug at one branch.
type ObatPrediksi struct {
	IDObat        int      `json:"idObat"`
	Nama          string   `json:"nama"`
	Satuan        string   `json:"satuan"` // the selling unit, the unit of the stock
	Cabang        string   `json:"cabang"`
	NamaCabang    string   `json:"namaCabang"`
	Stok          float64  `json:"stok"`
	StokMin       float64  `json:"stokMin"`
	AvgHarian     float64  `json:"avgHarian"`     // weighted units used per day
	HargaPokok    *float64 `json:"hargaPokok"`    // cost per stock unit; null when the master price is not plausible
	EstimasiBiaya *float64 `json:"estimasiBiaya"` // saranOrder × hargaPokok
	Terpakai      float64  `json:"terpakai"`      // units used over the history
	Hari          int      `json:"hari"`          // days of history
	LowData       bool     `json:"lowData"`
	Plan
}

// Usage is the daily usage of one drug at one branch, as the repository reads it.
type Usage struct {
	IDObat     int
	Nama       string
	Satuan     string
	Cabang     string
	NamaCabang string
	Stok       float64
	StokMin    float64
	HargaPokok *float64  // trxstock.hp when plausible (0 < hp ≤ 1.2 × hjual), else nil
	Series     []float64 // units used per day, oldest first, from the branch's first logged day
}

// UsageParams selects the drugs and the history window.
type UsageParams struct {
	Branches []string
	From, To string // YYYY-MM-DD, inclusive, clinic dates
	Search   string // drug name
}

// IcdSeries is the weekly number of visits with one diagnosis.
type IcdSeries struct {
	IDIcd int
	Kode  string
	Nama  string
	Weeks []int // oldest first; the last is the 7 days ending today
}

// ObatResep is a drug prescribed in visits with a diagnosis.
type ObatResep struct {
	IDObat      int     `json:"idObat"`
	Nama        string  `json:"nama"`
	Satuan      string  `json:"satuan"`
	Kunjungan   int     `json:"kunjungan"`   // visits with the diagnosis that got the drug
	Qty         float64 `json:"qty"`         // units in those visits
	Support     float64 `json:"support"`     // Kunjungan / the diagnosis' visits (0..1)
	QtyPerKasus float64 `json:"qtyPerKasus"` // Qty / the diagnosis' visits
}

// IcdObat is a diagnosis with the drugs most often prescribed with it.
type IcdObat struct {
	IDIcd     int         `json:"idIcd"`
	Kode      string      `json:"kode"`
	Nama      string      `json:"nama"`
	Kunjungan int         `json:"kunjungan"` // visits with the diagnosis
	Obat      []ObatResep `json:"obat"`
}

// ResepParams selects the diagnoses and the window.
type ResepParams struct {
	Branches  []string
	From, To  string // YYYY-MM-DD, inclusive
	ICD       string // ICD code or name; empty: the most frequent diagnoses
	LimitIcd  int
	LimitObat int
}

type Repository interface {
	UserBranches(ctx context.Context, userID string) ([]branchauth.Branch, error)
	Usage(ctx context.Context, p UsageParams) ([]Usage, error)
	IcdWeekly(ctx context.Context, branches []string, today string, weeks int) ([]IcdSeries, error)
	Resep(ctx context.Context, p ResepParams) ([]IcdObat, error)
	Stok(ctx context.Context, branches []string, ids []int) (map[int]float64, error)
	Kinerja(ctx context.Context, p KinerjaParams) ([]KinerjaRow, error)
	ObatTerlaris(ctx context.Context, branches []string, from, to string, byNilai bool, limit int) ([]Terlaris, error)
	TindakanTerlaris(ctx context.Context, branches []string, from, to string, byNilai bool, limit int) ([]Terlaris, error)
	Kualitas(ctx context.Context, branches []string, today string) ([]CekKualitas, error)
	FollowUp(ctx context.Context, p FollowUpParams) (FollowUpPage, error)
	ICDByKode(ctx context.Context, codes []string) (map[string]icd.ICD, error)
	SearchICD(ctx context.Context, q string, limit int) ([]icd.ICD, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (p *PostgresRepository) UserBranches(ctx context.Context, userID string) ([]branchauth.Branch, error) {
	return branchauth.ForUser(ctx, p.pool, userID)
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

func addDays(day string, n int) string {
	d, _ := time.Parse("2006-01-02", day)
	return d.AddDate(0, 0, n).Format("2006-01-02")
}

func daysBetween(from, to string) int {
	a, _ := time.Parse("2006-01-02", from)
	b, _ := time.Parse("2006-01-02", to)
	return int(b.Sub(a).Hours() / 24)
}

// Usage reads the daily usage of every active drug stocked at the branches.
// Usage is what sales took out (TrxJualObat, Out) less what cancelled sales
// put back (TrxStokAdjusCancel, In); receipts and manual adjustments are not
// usage. A branch's series starts at its first logged movement in the window,
// so a branch whose log is younger than the window is not diluted by zeros.
func (p *PostgresRepository) Usage(ctx context.Context, up UsageParams) ([]Usage, error) {
	starts := map[string]string{}
	rows, err := p.pool.Query(ctx, `SELECT cabang, to_char(min(createddate)::date, 'YYYY-MM-DD') FROM trxobat
		WHERE cabang = ANY($1) AND createddate >= $2::date AND createddate < $3::date + 1 GROUP BY cabang`,
		up.Branches, up.From, up.To)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var c, d string
		if err := rows.Scan(&c, &d); err != nil {
			rows.Close()
			return nil, err
		}
		starts[c] = d
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	args := []any{up.Branches, up.From, up.To}
	search := ""
	if up.Search != "" {
		args = append(args, "%"+escapeLike(up.Search)+"%")
		search = ` AND r.nama ILIKE $4`
	}
	rows, err = p.pool.Query(ctx, `WITH pakai AS (
			SELECT o.cabang, o.id_obat, to_char(o.createddate::date, 'YYYY-MM-DD') AS d,
				sum(CASE WHEN o.inout = 'Out' THEN o.byk ELSE -o.byk END) AS q
			FROM trxobat o
			WHERE o.cabang = ANY($1) AND o.createddate >= $2::date AND o.createddate < $3::date + 1
				AND ((o.table_ref = 'TrxJualObat' AND o.inout = 'Out') OR (o.table_ref = 'TrxStokAdjusCancel' AND o.inout = 'In'))
			GROUP BY 1, 2, 3)
		SELECT s.id_obat, r.nama, COALESCE(st.info, ''), s.cabang, COALESCE(b.nama, s.cabang),
			COALESCE(s.stok, 0), COALESCE(s.stok_min, 0),
			CASE WHEN s.hp > 0 AND s.hjual > 0 AND s.hp <= s.hjual * 1.2 THEN s.hp END, u.d, COALESCE(u.q, 0)
		FROM trxstock s
		JOIN refobat r ON r.id = s.id_obat
		LEFT JOIN refsatuan st ON st.id = r.sat_jual
		LEFT JOIN refbranch b ON b.kode = s.cabang
		LEFT JOIN pakai u ON u.cabang = s.cabang AND u.id_obat = s.id_obat
		WHERE s.cabang = ANY($1) AND s.isactive = 1 AND COALESCE(r.non_aktif, 0) <> 1`+search+`
		ORDER BY s.cabang, s.id_obat, u.d`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Usage
	var cur *Usage
	for rows.Next() {
		var u Usage
		var d *string
		var q float64
		if err := rows.Scan(&u.IDObat, &u.Nama, &u.Satuan, &u.Cabang, &u.NamaCabang, &u.Stok, &u.StokMin, &u.HargaPokok, &d, &q); err != nil {
			return nil, err
		}
		if cur == nil || cur.IDObat != u.IDObat || cur.Cabang != u.Cabang {
			start, ok := starts[u.Cabang]
			n := 0
			if ok {
				n = daysBetween(start, up.To) + 1
			}
			u.Series = make([]float64, n)
			out = append(out, u)
			cur = &out[len(out)-1]
		}
		if d != nil {
			if i := daysBetween(starts[cur.Cabang], *d); i >= 0 && i < len(cur.Series) {
				cur.Series[i] += q
			}
		}
	}
	return out, rows.Err()
}

// IcdWeekly counts, per diagnosis, the visits (sales, cancelled ones left out)
// in each of the last weeks: 7-day buckets ending today, oldest first.
func (p *PostgresRepository) IcdWeekly(ctx context.Context, branches []string, today string, weeks int) ([]IcdSeries, error) {
	from := addDays(today, -(weeks*7 - 1))
	rows, err := p.pool.Query(ctx, `SELECT x.icd_id, COALESCE(i.kode_icd, ''), COALESCE(i.nama, '(ICD ' || x.icd_id || ')'),
			($3::date - j.tgl::date) / 7 AS w, count(DISTINCT j.id)
		FROM trxjualdiagnosa x
		JOIN trxjual j ON j.id = x.id_jual
		LEFT JOIN trxjualtotal t ON t.id_jual = j.id
		LEFT JOIN reficd i ON i.id = x.icd_id
		WHERE j.cabang = ANY($1) AND j.tgl >= $2::date AND j.tgl < $3::date + 1 AND COALESCE(t.status, 0) <> 2 AND x.icd_id IS NOT NULL
		GROUP BY 1, 2, 3, 4
		ORDER BY 1`, branches, from, today)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []IcdSeries
	for rows.Next() {
		var s IcdSeries
		var w, n int
		if err := rows.Scan(&s.IDIcd, &s.Kode, &s.Nama, &w, &n); err != nil {
			return nil, err
		}
		if len(out) == 0 || out[len(out)-1].IDIcd != s.IDIcd {
			s.Weeks = make([]int, weeks)
			out = append(out, s)
		}
		if w >= 0 && w < weeks {
			out[len(out)-1].Weeks[weeks-1-w] += n
		}
	}
	return out, rows.Err()
}

// Resep lists, for the most frequent diagnoses (or those matching ICD), the
// drugs prescribed in the same visits. A visit with two diagnoses counts for
// both: this is an association, not which drug treats which diagnosis.
func (p *PostgresRepository) Resep(ctx context.Context, rp ResepParams) ([]IcdObat, error) {
	args := []any{rp.Branches, rp.From, rp.To, rp.LimitIcd, rp.LimitObat}
	filter := ""
	if rp.ICD != "" {
		args = append(args, "%"+escapeLike(rp.ICD)+"%")
		filter = ` AND (i.kode_icd ILIKE $6 OR i.nama ILIKE $6)`
	}
	rows, err := p.pool.Query(ctx, `WITH v AS (
			SELECT DISTINCT x.icd_id, j.id AS id_jual
			FROM trxjualdiagnosa x
			JOIN trxjual j ON j.id = x.id_jual
			LEFT JOIN trxjualtotal t ON t.id_jual = j.id
			LEFT JOIN reficd i ON i.id = x.icd_id
			WHERE j.cabang = ANY($1) AND j.tgl >= $2::date AND j.tgl < $3::date + 1 AND COALESCE(t.status, 0) <> 2
				AND x.icd_id IS NOT NULL`+filter+`),
		top AS (SELECT icd_id, count(*) AS kunjungan FROM v GROUP BY icd_id ORDER BY count(*) DESC, icd_id LIMIT $4),
		rx AS (
			SELECT v.icd_id, ob.id_obat, count(DISTINCT v.id_jual) AS n, sum(COALESCE(ob.byk, 0)) AS qty,
				row_number() OVER (PARTITION BY v.icd_id ORDER BY count(DISTINCT v.id_jual) DESC, ob.id_obat) AS rk
			FROM v JOIN top ON top.icd_id = v.icd_id
			JOIN trxjualobat ob ON ob.id_jual = v.id_jual
			WHERE ob.id_obat IS NOT NULL
			GROUP BY v.icd_id, ob.id_obat)
		SELECT top.icd_id, COALESCE(i.kode_icd, ''), COALESCE(i.nama, '(ICD ' || top.icd_id || ')'), top.kunjungan,
			rx.id_obat, COALESCE(r.nama, '(obat ' || rx.id_obat || ')'), COALESCE(st.info, ''), COALESCE(rx.n, 0), COALESCE(rx.qty, 0)
		FROM top
		LEFT JOIN reficd i ON i.id = top.icd_id
		LEFT JOIN rx ON rx.icd_id = top.icd_id AND rx.rk <= $5
		LEFT JOIN refobat r ON r.id = rx.id_obat
		LEFT JOIN refsatuan st ON st.id = r.sat_jual
		ORDER BY top.kunjungan DESC, top.icd_id, rx.rk`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []IcdObat{}
	for rows.Next() {
		var d IcdObat
		var idObat *int
		var o ObatResep
		if err := rows.Scan(&d.IDIcd, &d.Kode, &d.Nama, &d.Kunjungan, &idObat, &o.Nama, &o.Satuan, &o.Kunjungan, &o.Qty); err != nil {
			return nil, err
		}
		if len(out) == 0 || out[len(out)-1].IDIcd != d.IDIcd {
			d.Obat = []ObatResep{}
			out = append(out, d)
		}
		if idObat != nil {
			last := &out[len(out)-1]
			o.IDObat = *idObat
			if last.Kunjungan > 0 {
				o.Support = round2(float64(o.Kunjungan) / float64(last.Kunjungan))
				o.QtyPerKasus = round2(o.Qty / float64(last.Kunjungan))
			}
			last.Obat = append(last.Obat, o)
		}
	}
	return out, rows.Err()
}

// Stok sums the stock of the drugs over the branches.
func (p *PostgresRepository) Stok(ctx context.Context, branches []string, ids []int) (map[int]float64, error) {
	out := map[int]float64{}
	if len(ids) == 0 {
		return out, nil
	}
	rows, err := p.pool.Query(ctx, `SELECT id_obat, sum(COALESCE(stok, 0)) FROM trxstock
		WHERE cabang = ANY($1) AND id_obat = ANY($2) GROUP BY id_obat`, branches, ids)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var id int
		var s float64
		if err := rows.Scan(&id, &s); err != nil {
			return nil, err
		}
		out[id] = s
	}
	return out, rows.Err()
}
