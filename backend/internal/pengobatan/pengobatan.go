// Package pengobatan serves Transaksi Klinik > Pengobatan: the list of treatment
// sales (trxjual) with their totals and settlement status (trxjualtotal). A user
// only sees the sales of the branches they are mapped to (refbranchauth).
// Report Klinik > Raw Data Pengobatan is the same list with the patient's
// address, birth date and sex, the insurer and the cashier.
//
// It replaces the legacy spVTrxJualByPeriodeAuth8 procedure and its
// VTrxJualWithAddress view with one parameterized query that joins only what
// the list shows.
package pengobatan

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/icd"
)

// Settlement statuses (trxjualtotal.status), shown as Status.
const (
	StatusOpen   = "open"   // anything but 1 or 2, also a sale without a total
	StatusClose  = "close"  // 1
	StatusCancel = "cancel" // 2
)

// Row is a sale as listed.
type Row struct {
	ID             int     `json:"id"`
	Tgl            *string `json:"tgl"` // "YYYY-MM-DD HH:MM:SS", clinic time
	NoKwi          string  `json:"noKwi"`
	IDPas          *int    `json:"idPas"`
	NoRM           string  `json:"noRm"`
	Pasien         string  `json:"pasien"` // "" when the patient no longer exists
	Dokter         string  `json:"dokter"`
	IDJenisRawat   *int    `json:"idJenisRawat"`
	JenisRawat     string  `json:"jenisRawat"`
	IDJenisBayar   *int    `json:"idJenisBayar"`
	Pembayaran     string  `json:"pembayaran"`
	TotalObat      float64 `json:"totalObat"`
	TotalTindakan  float64 `json:"totalTindakan"`
	TotalPenunjang float64 `json:"totalPenunjang"`
	Potongan       float64 `json:"potongan"`
	Total          float64 `json:"total"`
	Cabang         string  `json:"cabang"`
	Status         string  `json:"status"` // open, close or cancel
	CancelBy       string  `json:"cancelBy"`
	CancelAt       *string `json:"cancelAt"`
	CancelReason   string  `json:"cancelReason"`
	// Raw Data Pengobatan columns.
	Alamat       string  `json:"alamat"`
	RT           string  `json:"rt"`
	RW           string  `json:"rw"`
	Kelurahan    string  `json:"kelurahan"`
	TglLahir     *string `json:"tglLahir"` // YYYY-MM-DD
	JenisKelamin string  `json:"jenisKelamin"`
	Asuransi     string  `json:"asuransi"` // the sale's insurer (trxjual.id_jamkes)
	CreateUser   string  `json:"createUser"`
}

const statusSQL = `CASE t.status WHEN 1 THEN 'close' WHEN 2 THEN 'cancel' ELSE 'open' END`

// sortColumns are the sortable list columns by API key (SQL is fixed, never from the request).
var sortColumns = map[string]string{
	"id":         "j.id",
	"tgl":        "j.tgl",
	"noKwi":      "j.no_kwi",
	"pasien":     "p.nama",
	"dokter":     "dk.nama",
	"jenisRawat": "r.info",
	"pembayaran": "b.info",
	"total":      "t.total",
	"cabang":     "j.cabang",
	"status":     statusSQL,
	// Raw Data Pengobatan
	"noRm":         "p.no_rm",
	"alamat":       "p.alamat",
	"kelurahan":    "kl.info",
	"tglLahir":     "p.tgl_lahir",
	"jenisKelamin": "g.info",
	"asuransi":     "jk.nama",
	"potongan":     "t.potongan",
	"createUser":   "j.create_user",
}

// ListParams selects one page of sales.
type ListParams struct {
	Branches        []string // always applied
	From, To        string   // YYYY-MM-DD, inclusive, clinic dates
	Search          string   // the table's search box
	Nama            string   // "Pencarian" panel: patient name
	IDJenisRawat    *int     // "Pencarian" panel: Jenis Pelayanan
	IDStatusKunjung *int     // "Pencarian" panel: status of the linked registration (refstatustransaksi)
	IDJenisBayar    *int     // Filter: Pembayaran
	Status          string   // Filter: open, close, cancel or ""
	Sort            string
	Desc            bool
	Limit           int
	Offset          int
}

type Page struct {
	Rows            []Row `json:"rows"`
	Total           int   `json:"total"`
	UnfilteredTotal int   `json:"unfilteredTotal"`
}

type Option struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

type Repository interface {
	UserBranches(ctx context.Context, userID string) ([]branchauth.Branch, error)
	List(ctx context.Context, p ListParams) (Page, error)
	JenisRawat(ctx context.Context) ([]Option, error)
	StatusKunjungan(ctx context.Context) ([]Option, error)
	JenisBayar(ctx context.Context) ([]Option, error)
	Pendaftaran(ctx context.Context, p RegParams) (RegPage, error)
	ProsesStatus(ctx context.Context) ([]Option, error)
	Dokter(ctx context.Context, branch string) ([]Dokter, error)
	Obat(ctx context.Context, branch string) ([]Obat, error)
	Tindakan(ctx context.Context, branch string, dokterID int) ([]Tindakan, error)
	Penunjang(ctx context.Context, branch string) ([]Penunjang, error)
	ICD(ctx context.Context, lp icd.ListParams) (icd.Page, error)
	RegBranch(ctx context.Context, idKun int) (cabang string, status int, err error)
	GetPemeriksaan(ctx context.Context, idKun int) (*PemeriksaanRecord, error)
	SavePemeriksaan(ctx context.Context, idKun int, in Pemeriksaan, user string) error
	PerawatList(ctx context.Context, p PerawatParams) (PerawatPage, error)
	RegByID(ctx context.Context, idKun int) (RegRow, error)
	DokterAtBranches(ctx context.Context, branches []string) ([]Dokter, error)
	GetResep(ctx context.Context, idKun int) (Resep, error)
	SaveResep(ctx context.Context, idKun int, in ResepInput) error
	Save(ctx context.Context, in SaveInput, user string) (SaveResult, error)
	Kuitansi(ctx context.Context, id int) (Kuitansi, error)
	Cetak(ctx context.Context, id int, user string) (Kuitansi, error)
	Nota(ctx context.Context, id int) (Nota, error)
	Detail(ctx context.Context, id int) (Detail, error)
	Cancel(ctx context.Context, id int, reason, user string) error
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

const fromJoins = ` FROM trxjual j
	LEFT JOIN trxjualtotal t ON t.id_jual = j.id
	LEFT JOIN refmpasien p ON p.id = j.id_pas
	LEFT JOIN refmdokter dk ON dk.id = j.id_dok
	LEFT JOIN refjenisrawat r ON r.id = j.id_jenis_rwt
	LEFT JOIN refjenisbayar b ON b.id = j.id_jenis_byr
	LEFT JOIN refkelurahan kl ON kl.id = p.id_kel
	LEFT JOIN refgender g ON g.id = p.id_gender
	LEFT JOIN refjaminankesehatan jk ON jk.id = j.id_jamkes`

const rowColumns = `SELECT j.id, to_char(j.tgl, 'YYYY-MM-DD HH24:MI:SS'), COALESCE(j.no_kwi, ''), j.id_pas,
	COALESCE(p.no_rm, ''), COALESCE(p.nama, ''), COALESCE(dk.nama, ''), j.id_jenis_rwt, COALESCE(r.info, ''),
	j.id_jenis_byr, COALESCE(b.info, ''), COALESCE(t.tot_obat, 0), COALESCE(t.tot_tndk, 0), COALESCE(t.tot_pnjng, 0),
	COALESCE(t.potongan, 0), COALESCE(t.total, 0), COALESCE(j.cabang, ''), ` + statusSQL + `,
	COALESCE(t.cancelby, ''), to_char(t.cancelat, 'YYYY-MM-DD HH24:MI:SS'), COALESCE(t.cancelreason, ''),
	COALESCE(p.alamat, ''), COALESCE(p.rt, ''), COALESCE(p.rw, ''), COALESCE(kl.info, ''), to_char(p.tgl_lahir, 'YYYY-MM-DD'),
	COALESCE(g.info, ''), COALESCE(jk.nama, ''), COALESCE(j.create_user, '')`

func scanRow(r pgx.CollectableRow) (Row, error) {
	var x Row
	err := r.Scan(&x.ID, &x.Tgl, &x.NoKwi, &x.IDPas, &x.NoRM, &x.Pasien, &x.Dokter, &x.IDJenisRawat, &x.JenisRawat,
		&x.IDJenisBayar, &x.Pembayaran, &x.TotalObat, &x.TotalTindakan, &x.TotalPenunjang, &x.Potongan, &x.Total,
		&x.Cabang, &x.Status, &x.CancelBy, &x.CancelAt, &x.CancelReason, &x.Alamat, &x.RT, &x.RW, &x.Kelurahan,
		&x.TglLahir, &x.JenisKelamin, &x.Asuransi, &x.CreateUser)
	return x, err
}

// where builds the filter shared by the page and its counts. Without
// withSearch it leaves out the search box, for the "filtered from" total.
func where(lp ListParams, withSearch bool) (string, []any) {
	args := []any{lp.Branches, lp.From, lp.To}
	conds := []string{`j.cabang = ANY($1)`, `j.tgl >= $2::date`, `j.tgl < $3::date + 1`}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if lp.Nama != "" {
		add(`p.nama ILIKE $%d`, "%"+escapeLike(lp.Nama)+"%")
	}
	if lp.IDJenisRawat != nil {
		add(`j.id_jenis_rwt = $%d`, *lp.IDJenisRawat)
	}
	if lp.IDStatusKunjung != nil {
		add(`EXISTS (SELECT 1 FROM trxpendaftaran d WHERE d.id = j.id_kun AND d.id_statustr = $%d)`, *lp.IDStatusKunjung)
	}
	if lp.IDJenisBayar != nil {
		add(`j.id_jenis_byr = $%d`, *lp.IDJenisBayar)
	}
	if lp.Status != "" {
		add(statusSQL+` = $%d`, lp.Status)
	}
	if withSearch && lp.Search != "" {
		args = append(args, "%"+escapeLike(lp.Search)+"%")
		n := len(args)
		cond := fmt.Sprintf(`j.no_kwi ILIKE $%[1]d OR p.nama ILIKE $%[1]d OR p.no_rm ILIKE $%[1]d OR dk.nama ILIKE $%[1]d`, n)
		var id int
		if _, err := fmt.Sscanf(lp.Search, "%d", &id); err == nil && fmt.Sprint(id) == lp.Search {
			args = append(args, id)
			cond += fmt.Sprintf(` OR j.id = $%d`, len(args))
		}
		conds = append(conds, `(`+cond+`)`)
	}
	return strings.Join(conds, " AND "), args
}

func (p *PostgresRepository) List(ctx context.Context, lp ListParams) (Page, error) {
	page := Page{Rows: []Row{}}
	col, ok := sortColumns[lp.Sort]
	if !ok {
		col = "j.id"
	}
	dir := "ASC"
	if lp.Desc {
		dir = "DESC"
	}
	cond, args := where(lp, true)
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`%s%s WHERE %s ORDER BY %s %s NULLS LAST, j.id %s LIMIT $%d OFFSET $%d`,
		rowColumns, fromJoins, cond, col, dir, dir, n+1, n+2), append(args, lp.Limit, lp.Offset)...)
	if err != nil {
		return page, err
	}
	if page.Rows, err = pgx.CollectRows(rows, scanRow); err != nil {
		return page, err
	}
	if page.Rows == nil {
		page.Rows = []Row{}
	}
	if err := p.pool.QueryRow(ctx, `SELECT count(*)`+fromJoins+` WHERE `+cond, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	if lp.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = where(lp, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*)`+fromJoins+` WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}

func (p *PostgresRepository) options(ctx context.Context, sql string) ([]Option, error) {
	rows, err := p.pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Option, error) {
		var o Option
		err := r.Scan(&o.ID, &o.Name)
		return o, err
	})
	if list == nil {
		list = []Option{}
	}
	return list, err
}

func (p *PostgresRepository) JenisRawat(ctx context.Context) ([]Option, error) {
	return p.options(ctx, `SELECT id, COALESCE(info, '') FROM refjenisrawat ORDER BY info, id`)
}

func (p *PostgresRepository) StatusKunjungan(ctx context.Context) ([]Option, error) {
	return p.options(ctx, `SELECT id, COALESCE(info, '') FROM refstatustransaksi ORDER BY id`)
}

// JenisBayar lists the payment types in use (flag 1); flag 2 rows repeat TUNAI and HUTANG and are unused.
func (p *PostgresRepository) JenisBayar(ctx context.Context) ([]Option, error) {
	return p.options(ctx, `SELECT id, COALESCE(info, '') FROM refjenisbayar WHERE flag = 1 ORDER BY id`)
}
