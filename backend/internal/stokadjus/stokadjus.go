// Package stokadjus serves Stok Farmasi > Stok Adjusment (legacy
// /StokAdjusment): the manual stock corrections (trxstokadjus) at the user's
// branches, and the Add form that records one and moves the branch stock
// (trxstock.stok) up (Masuk) or down (Keluar). A user only sees, and adjusts,
// the branches they are mapped to (refbranchauth).
package stokadjus

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
	"github.com/example/be_jwt_golang/backend/internal/pendaftaran"
	"github.com/example/be_jwt_golang/backend/internal/stoklog"
)

// Status values of trxstokadjus.status.
const (
	StatusKeluar = 0 // stock goes down
	StatusMasuk  = 1 // stock goes up
)

// Row is one adjustment as listed.
type Row struct {
	ID         int     `json:"id"`      // Kode
	Tanggal    *string `json:"tanggal"` // YYYY-MM-DD
	IDObat     int     `json:"idObat"`
	Nama       string  `json:"nama"`
	Status     int     `json:"status"`     // 1 Masuk, 0 Keluar
	StatusDesc string  `json:"statusDesc"` // "Masuk" / "Keluar"
	Ket        string  `json:"ket"`
	Byk        float64 `json:"byk"`
	Cabang     string  `json:"cabang"`
	NamaCabang string  `json:"namaCabang"`
}

// ListParams selects one page of adjustments.
type ListParams struct {
	Branches []string // always applied
	From, To string   // YYYY-MM-DD, inclusive, of tanggal
	Search   string   // the table's search box
	Nama     string   // Cari: drug name
	Ket      string   // Cari: keterangan
	Status   *int     // Cari: 1 Masuk, 0 Keluar
	Sort     string
	Desc     bool
	Limit    int
	Offset   int
}

type Page struct {
	Rows            []Row `json:"rows"`
	Total           int   `json:"total"`
	UnfilteredTotal int   `json:"unfilteredTotal"`
}

// Obat is a row of the Add form's drug lookup: a drug stocked at the branch.
type Obat struct {
	ID     int     `json:"id"`
	Nama   string  `json:"nama"`
	Jenis  string  `json:"jenis"`
	Satuan string  `json:"satuan"` // the selling unit, the unit of the stock
	Stok   float64 `json:"stok"`
}

// SaveInput is the Add form.
type SaveInput struct {
	Cabang  string  `json:"cabang"`
	Tanggal string  `json:"tanggal"` // YYYY-MM-DD
	IDObat  int     `json:"idObat"`
	Status  *int    `json:"status"` // 1 Masuk, 0 Keluar
	Byk     float64 `json:"byk"`
	Ket     string  `json:"ket"`
}

type SaveResult struct {
	ID   int     `json:"id"`
	Stok float64 `json:"stok"` // the branch stock after the adjustment
}

var (
	ErrNotStocked = errors.New("obat tidak ada di stok cabang ini")
	ErrShortStock = errors.New("stok tidak cukup")
)

type Repository interface {
	UserBranches(ctx context.Context, userID string) ([]branchauth.Branch, error)
	List(ctx context.Context, p ListParams) (Page, error)
	Obat(ctx context.Context, branch string) ([]Obat, error)
	Save(ctx context.Context, in SaveInput, user string) (SaveResult, error)
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

// sortColumns are the sortable columns (fixed SQL, never from the request).
var sortColumns = map[string]string{
	"id":      "a.id",
	"tanggal": "a.tanggal",
	"idObat":  "a.id_obat",
	"nama":    "o.nama",
	"status":  "a.status",
	"ket":     "a.ket",
	"byk":     "a.byk",
	"cabang":  "b.nama",
}

// Sortable reports whether key is a sort column.
func Sortable(key string) bool {
	_, ok := sortColumns[key]
	return ok
}

const fromJoins = ` FROM trxstokadjus a
	JOIN refobat o ON o.id = a.id_obat
	JOIN refbranch b ON b.kode = a.cabang`

func where(lp ListParams, withSearch bool) (string, []any) {
	args := []any{lp.Branches, lp.From, lp.To}
	conds := []string{`a.cabang = ANY($1)`, `a.tanggal >= $2::date`, `a.tanggal < $3::date + 1`}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if lp.Nama != "" {
		add(`o.nama ILIKE $%d`, "%"+escapeLike(lp.Nama)+"%")
	}
	if lp.Ket != "" {
		add(`a.ket ILIKE $%d`, "%"+escapeLike(lp.Ket)+"%")
	}
	if lp.Status != nil {
		add(`COALESCE(a.status, 0) = $%d`, *lp.Status)
	}
	if withSearch && lp.Search != "" {
		args = append(args, "%"+escapeLike(lp.Search)+"%")
		n := len(args)
		cond := fmt.Sprintf(`o.nama ILIKE $%[1]d OR a.ket ILIKE $%[1]d OR b.nama ILIKE $%[1]d
			OR (CASE WHEN a.status = 1 THEN 'Masuk' ELSE 'Keluar' END) ILIKE $%[1]d`, n)
		var id int
		if _, err := fmt.Sscanf(lp.Search, "%d", &id); err == nil && fmt.Sprint(id) == lp.Search {
			args = append(args, id)
			cond += fmt.Sprintf(` OR a.id = $%[1]d OR a.id_obat = $%[1]d`, len(args))
		}
		conds = append(conds, `(`+cond+`)`)
	}
	return strings.Join(conds, " AND "), args
}

func (p *PostgresRepository) List(ctx context.Context, lp ListParams) (Page, error) {
	page := Page{Rows: []Row{}}
	col, ok := sortColumns[lp.Sort]
	if !ok {
		col = "a.id"
	}
	dir := "ASC"
	if lp.Desc {
		dir = "DESC"
	}
	cond, args := where(lp, true)
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`SELECT a.id, to_char(a.tanggal, 'YYYY-MM-DD'), a.id_obat, COALESCE(o.nama, ''),
			COALESCE(a.status, 0), COALESCE(a.ket, ''), COALESCE(a.byk, 0), a.cabang, COALESCE(b.nama, a.cabang), count(*) OVER ()
		%s WHERE %s ORDER BY %s %s NULLS LAST, a.id %s LIMIT $%d OFFSET $%d`, fromJoins, cond, col, dir, dir, n+1, n+2),
		append(args, lp.Limit, lp.Offset)...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var x Row
		if err := rows.Scan(&x.ID, &x.Tanggal, &x.IDObat, &x.Nama, &x.Status, &x.Ket, &x.Byk, &x.Cabang, &x.NamaCabang, &page.Total); err != nil {
			return page, err
		}
		x.StatusDesc = StatusDesc(x.Status)
		page.Rows = append(page.Rows, x)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	// An offset past the end returns no rows, and so no window count.
	if len(page.Rows) == 0 && lp.Offset > 0 {
		if err := p.pool.QueryRow(ctx, `SELECT count(*)`+fromJoins+` WHERE `+cond, args...).Scan(&page.Total); err != nil {
			return page, err
		}
	}
	if lp.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = where(lp, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*)`+fromJoins+` WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}

// StatusDesc is the legacy label: 1 Masuk, anything else Keluar.
func StatusDesc(status int) string {
	if status == StatusMasuk {
		return "Masuk"
	}
	return "Keluar"
}

// Obat lists the drugs stocked at a branch (every trxstock row), by id, with the stock.
func (p *PostgresRepository) Obat(ctx context.Context, branch string) ([]Obat, error) {
	rows, err := p.pool.Query(ctx, `SELECT s.id_obat, COALESCE(o.nama, ''), COALESCE(jo.info, ''), COALESCE(sj.info, ''), COALESCE(s.stok, 0)
		FROM trxstock s
		JOIN refobat o ON o.id = s.id_obat
		LEFT JOIN refjenisobat jo ON jo.id = o.id_jenis
		LEFT JOIN refsatuan sj ON sj.id = o.sat_jual
		WHERE s.cabang = $1
		ORDER BY s.id_obat`, branch)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Obat, error) {
		var o Obat
		err := r.Scan(&o.ID, &o.Nama, &o.Jenis, &o.Satuan, &o.Stok)
		return o, err
	})
	if list == nil {
		list = []Obat{}
	}
	return list, err
}

// Normalize trims the text.
func (in *SaveInput) Normalize() {
	in.Cabang = strings.ToUpper(strings.TrimSpace(in.Cabang))
	in.Tanggal = strings.TrimSpace(in.Tanggal)
	in.Ket = strings.TrimSpace(in.Ket)
}

// Validate checks the form; today is the clinic date (Tanggal may not be later).
func (in SaveInput) Validate(today string) httpx.FieldErrors {
	errs := httpx.FieldErrors{}
	if in.Cabang == "" {
		errs["cabang"] = "Pilih Cabang"
	}
	if d, err := time.Parse("2006-01-02", in.Tanggal); err != nil {
		errs["tanggal"] = "Tanggal tidak valid"
	} else if d.Format("2006-01-02") > today {
		errs["tanggal"] = "Tanggal tidak boleh setelah hari ini"
	}
	if in.IDObat <= 0 {
		errs["idObat"] = "Pilih Obat"
	}
	if in.Status == nil || (*in.Status != StatusMasuk && *in.Status != StatusKeluar) {
		errs["status"] = "Pilih Status (Masuk atau Keluar)"
	}
	if !(in.Byk > 0) || math.IsInf(in.Byk, 0) {
		errs["byk"] = "Banyak harus lebih dari 0"
	}
	switch n := utf8.RuneCountInString(in.Ket); {
	case n == 0:
		errs["ket"] = "Isi Keterangan"
	case n > 60:
		errs["ket"] = "Keterangan maksimal 60 karakter"
	}
	return errs
}

// Save records an adjustment and moves the branch stock in one transaction:
// Masuk adds byk, Keluar takes it off (never below 0). user goes to create_user.
func (p *PostgresRepository) Save(ctx context.Context, in SaveInput, user string) (SaveResult, error) {
	var res SaveResult
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return res, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	var stok float64
	err = tx.QueryRow(ctx, `SELECT COALESCE(stok, 0) FROM trxstock WHERE id_obat = $1 AND cabang = $2 FOR UPDATE`, in.IDObat, in.Cabang).Scan(&stok)
	if errors.Is(err, pgx.ErrNoRows) {
		return res, ErrNotStocked
	}
	if err != nil {
		return res, err
	}
	delta := in.Byk
	if *in.Status == StatusKeluar {
		if in.Byk > stok {
			return res, ErrShortStock
		}
		delta = -in.Byk
	}
	now := `(now() AT TIME ZONE '` + pendaftaran.ClinicTimeZone + `')`
	if err := tx.QueryRow(ctx, `INSERT INTO trxstokadjus (tanggal, id_obat, ket, byk, status, create_user, create_date, update_user, update_date, cabang)
		VALUES ($1::date, $2, $3, $4, $5, $6, `+now+`, $6, `+now+`, $7) RETURNING id`,
		in.Tanggal, in.IDObat, in.Ket, in.Byk, *in.Status, user, in.Cabang).Scan(&res.ID); err != nil {
		return res, err
	}
	if res.Stok, _, err = stoklog.Move(ctx, tx, stoklog.Entry{IDObat: in.IDObat, Cabang: in.Cabang, Delta: delta,
		TableRef: stoklog.RefAdjus, IDRef: res.ID, Remarks: in.Ket, User: user}); err != nil {
		return res, err
	}
	return res, tx.Commit(ctx)
}
