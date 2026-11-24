// Package paketprice serves Setting > Paket Price: MCU packages per company
// (refpriceheader) and the lab examinations in each (refpricedetail -> reflab).
// Delete hides a package (isdeleted) instead of removing it, because MCU
// transactions refer to it (trxmcudetail.paketid in lab).
package paketprice

import (
	"context"
	"errors"
	"math"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

const timeZone = "Asia/Jakarta"

var (
	ErrNotFound        = errors.New("paket not found")
	ErrUnknownCompany  = errors.New("unknown perusahaan")
	ErrUnknownKategori = errors.New("unknown kategori")
	ErrUnknownLab      = errors.New("unknown lab")
)

// Paket is a refpriceheader row as listed: with its company and category names.
type Paket struct {
	ID           int      `json:"id"`
	IDPerusahaan int      `json:"idPerusahaan"`
	Perusahaan   string   `json:"perusahaan"`
	Nama         string   `json:"nama"`
	IsPaket      bool     `json:"isPaket"`
	IDKategori   *int     `json:"idKategori"`
	Kategori     string   `json:"kategori"`
	FromDate     string   `json:"fromDate"` // YYYY-MM-DD or ""
	ToDate       string   `json:"toDate"`
	Price        *float64 `json:"price"`
	IsActive     bool     `json:"isActive"`
	JumlahLab    int      `json:"jumlahLab"`
}

// LabItem is a lab examination in a package.
type LabItem struct {
	ID           int      `json:"id"` // reflab.labid
	LabMaster    string   `json:"labMaster"`
	LabSubMaster string   `json:"labSubMaster"`
	Nama         string   `json:"nama"`
	Price        *float64 `json:"price"` // reflab.price
	IsActive     bool     `json:"isActive"`
}

// Detail is a package with its lab examinations.
type Detail struct {
	Paket
	Labs []LabItem `json:"labs"`
}

// Input is the editable part of a package. Dates are YYYY-MM-DD.
type Input struct {
	IDPerusahaan int      `json:"idPerusahaan"`
	Nama         string   `json:"nama"`
	IsPaket      bool     `json:"isPaket"`
	IDKategori   *int     `json:"idKategori"`
	FromDate     string   `json:"fromDate"`
	ToDate       string   `json:"toDate"`
	Price        *float64 `json:"price"`
	IsActive     bool     `json:"isActive"`
	LabIDs       []int    `json:"labIds"`
}

// Normalize trims and validates the input against refpriceheader, and
// deduplicates and sorts the lab ids.
func (in *Input) Normalize() httpx.FieldErrors {
	in.Nama = strings.TrimSpace(in.Nama)
	in.FromDate = strings.TrimSpace(in.FromDate)
	in.ToDate = strings.TrimSpace(in.ToDate)
	errs := httpx.FieldErrors{}
	if in.IDPerusahaan <= 0 {
		errs["idPerusahaan"] = "Perusahaan is required"
	}
	switch {
	case in.Nama == "":
		errs["nama"] = "Nama Paket is required"
	case utf8.RuneCountInString(in.Nama) > 30:
		errs["nama"] = "Nama Paket must be at most 30 characters"
	}
	if in.IDKategori != nil && *in.IDKategori <= 0 {
		in.IDKategori = nil
	}
	date := func(field, label, v string) (time.Time, bool) {
		if v == "" {
			errs[field] = label + " is required"
			return time.Time{}, false
		}
		d, err := time.Parse("2006-01-02", v)
		if err != nil {
			errs[field] = "Use the format YYYY-MM-DD"
			return time.Time{}, false
		}
		return d, true
	}
	from, okFrom := date("fromDate", "Dari Tanggal", in.FromDate)
	to, okTo := date("toDate", "Sampai Tanggal", in.ToDate)
	if okFrom && okTo && to.Before(from) {
		errs["toDate"] = "Sampai Tanggal cannot be before Dari Tanggal"
	}
	if in.Price == nil {
		errs["price"] = "Price is required"
	} else if math.IsNaN(*in.Price) || math.IsInf(*in.Price, 0) || *in.Price < 0 || *in.Price > 1e12 {
		errs["price"] = "Price must be a number from 0"
	}
	seen := map[int]bool{}
	ids := []int{}
	for _, id := range in.LabIDs {
		if id > 0 && !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Ints(ids)
	in.LabIDs = ids
	if len(ids) == 0 {
		errs["labIds"] = "Choose at least one lab examination"
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// Choice is an id and a name for a form list.
type Choice struct {
	ID   int    `json:"id"`
	Nama string `json:"nama"`
}

// Options holds the form's lists. Labs holds every examination; the form
// offers only active ones, plus inactive ones already in the package.
type Options struct {
	Perusahaan []Choice  `json:"perusahaan"`
	Kategori   []Choice  `json:"kategori"`
	Labs       []LabItem `json:"labs"`
}

type Repository interface {
	List(ctx context.Context) ([]Paket, error)
	Options(ctx context.Context) (Options, error)
	Get(ctx context.Context, id int) (Detail, error)
	Create(ctx context.Context, in Input, user string) (int, error)
	Update(ctx context.Context, id int, in Input, user string) error
	Delete(ctx context.Context, id int, user string) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// Like the original query, a package is listed only with its company (inner join).
const paketQuery = `SELECT h.id, h.companyid, COALESCE(c.companyname, ''), COALESCE(h.paketname, ''), COALESCE(h.ispaket, false),
	       h.kategoriid, COALESCE(k.category, ''), COALESCE(to_char(h.fromdate, 'YYYY-MM-DD'), ''),
	       COALESCE(to_char(h.todate, 'YYYY-MM-DD'), ''), h.price, COALESCE(h.isactive, 0) = 1,
	       (SELECT count(*) FROM refpricedetail d WHERE d.headerid = h.id)
	FROM refpriceheader h
	JOIN refcompany c ON c.companyid = h.companyid
	LEFT JOIN refcategory k ON k.id = h.kategoriid
	WHERE h.isdeleted IS NOT TRUE`

func scanPaket(r pgx.CollectableRow) (Paket, error) {
	var x Paket
	err := r.Scan(&x.ID, &x.IDPerusahaan, &x.Perusahaan, &x.Nama, &x.IsPaket, &x.IDKategori, &x.Kategori,
		&x.FromDate, &x.ToDate, &x.Price, &x.IsActive, &x.JumlahLab)
	return x, err
}

const labColumns = `l.labid, COALESCE(m.labmastername, ''), COALESCE(s.labsubmastername, ''), COALESCE(l.labname, ''),
	l.price, l.isdeleted IS NOT TRUE`

const labJoins = `reflab l
	LEFT JOIN reflabsubmaster s ON s.labsubmasterid = l.labsubmasterid
	LEFT JOIN reflabmaster m ON m.labmasterid = s.labmasterid`

func scanLab(r pgx.CollectableRow) (LabItem, error) {
	var x LabItem
	err := r.Scan(&x.ID, &x.LabMaster, &x.LabSubMaster, &x.Nama, &x.Price, &x.IsActive)
	return x, err
}

func (p *PostgresRepository) List(ctx context.Context) ([]Paket, error) {
	rows, err := p.pool.Query(ctx, paketQuery+` ORDER BY h.id DESC`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, scanPaket)
	if list == nil {
		list = []Paket{}
	}
	return list, err
}

func choices(ctx context.Context, pool *pgxpool.Pool, sql string) ([]Choice, error) {
	rows, err := pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Choice, error) {
		var c Choice
		err := r.Scan(&c.ID, &c.Nama)
		return c, err
	})
	if list == nil {
		list = []Choice{}
	}
	return list, err
}

func (p *PostgresRepository) Options(ctx context.Context) (Options, error) {
	var o Options
	var err error
	if o.Perusahaan, err = choices(ctx, p.pool, `SELECT companyid, COALESCE(companyname, '') FROM refcompany ORDER BY companyname, companyid`); err != nil {
		return o, err
	}
	if o.Kategori, err = choices(ctx, p.pool, `SELECT id, COALESCE(category, '') FROM refcategory ORDER BY id`); err != nil {
		return o, err
	}
	rows, err := p.pool.Query(ctx, `SELECT `+labColumns+` FROM `+labJoins+`
		ORDER BY m.labmastername NULLS LAST, s.labsubmastername NULLS LAST, l.labid`)
	if err != nil {
		return o, err
	}
	if o.Labs, err = pgx.CollectRows(rows, scanLab); err != nil {
		return o, err
	}
	if o.Labs == nil {
		o.Labs = []LabItem{}
	}
	return o, nil
}

func (p *PostgresRepository) Get(ctx context.Context, id int) (Detail, error) {
	var d Detail
	rows, err := p.pool.Query(ctx, paketQuery+` AND h.id = $1`, id)
	if err != nil {
		return d, err
	}
	d.Paket, err = pgx.CollectExactlyOneRow(rows, scanPaket)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNotFound
	}
	if err != nil {
		return d, err
	}
	rows, err = p.pool.Query(ctx, `SELECT `+labColumns+` FROM `+labJoins+`
		WHERE l.labid IN (SELECT d.labid FROM refpricedetail d WHERE d.headerid = $1)
		ORDER BY m.labmastername NULLS LAST, s.labsubmastername NULLS LAST, l.labid`, id)
	if err != nil {
		return d, err
	}
	if d.Labs, err = pgx.CollectRows(rows, scanLab); err != nil {
		return d, err
	}
	if d.Labs == nil {
		d.Labs = []LabItem{}
	}
	return d, nil
}

// check verifies the company, category and labs exist.
func check(ctx context.Context, tx pgx.Tx, in Input) error {
	var ok bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM refcompany WHERE companyid = $1)`, in.IDPerusahaan).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return ErrUnknownCompany
	}
	if in.IDKategori != nil {
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM refcategory WHERE id = $1)`, *in.IDKategori).Scan(&ok); err != nil {
			return err
		}
		if !ok {
			return ErrUnknownKategori
		}
	}
	var n int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM reflab WHERE labid = ANY($1)`, in.LabIDs).Scan(&n); err != nil {
		return err
	}
	if n != len(in.LabIDs) {
		return ErrUnknownLab
	}
	return nil
}

// replaceLabs makes the package's examinations exactly in.LabIDs, keeping the
// rows (and ids) of examinations that stay.
func replaceLabs(ctx context.Context, tx pgx.Tx, id int, labIDs []int) error {
	if _, err := tx.Exec(ctx, `DELETE FROM refpricedetail WHERE headerid = $1 AND NOT (labid = ANY($2))`, id, labIDs); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO refpricedetail (headerid, labid)
		SELECT $1, x FROM unnest($2::int[]) AS x
		WHERE NOT EXISTS (SELECT 1 FROM refpricedetail d WHERE d.headerid = $1 AND d.labid = x)`, id, labIDs)
	return err
}

func active(b bool) int {
	if b {
		return 1
	}
	return 0
}

func (p *PostgresRepository) Create(ctx context.Context, in Input, user string) (int, error) {
	var id int
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		if err := check(ctx, tx, in); err != nil {
			return err
		}
		err := tx.QueryRow(ctx,
			`INSERT INTO refpriceheader (companyid, paketname, ispaket, kategoriid, fromdate, todate, price, isactive,
			                             isdeleted, createby, createddate)
			 VALUES ($1, $2, $3, $4, $5::date, $6::date, $7, $8, false, $9, now() AT TIME ZONE '`+timeZone+`')
			 RETURNING id`,
			in.IDPerusahaan, in.Nama, in.IsPaket, in.IDKategori, in.FromDate, in.ToDate, in.Price, active(in.IsActive), user,
		).Scan(&id)
		if err != nil {
			return err
		}
		return replaceLabs(ctx, tx, id, in.LabIDs)
	})
	return id, err
}

func (p *PostgresRepository) Update(ctx context.Context, id int, in Input, user string) error {
	return pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		if err := check(ctx, tx, in); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx,
			`UPDATE refpriceheader SET companyid = $2, paketname = $3, ispaket = $4, kategoriid = $5, fromdate = $6::date,
			        todate = $7::date, price = $8, isactive = $9, modifiedby = $10, modifieddate = now() AT TIME ZONE '`+timeZone+`'
			 WHERE id = $1 AND isdeleted IS NOT TRUE`,
			id, in.IDPerusahaan, in.Nama, in.IsPaket, in.IDKategori, in.FromDate, in.ToDate, in.Price, active(in.IsActive), user)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return replaceLabs(ctx, tx, id, in.LabIDs)
	})
}

func (p *PostgresRepository) Delete(ctx context.Context, id int, user string) error {
	tag, err := p.pool.Exec(ctx,
		`UPDATE refpriceheader SET isdeleted = true, deletedby = $2, deleteddate = now() AT TIME ZONE '`+timeZone+`'
		 WHERE id = $1 AND isdeleted IS NOT TRUE`, id, user)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// errorFields maps repository errors to form fields.
func errorFields(err error) (httpx.FieldErrors, bool) {
	switch {
	case errors.Is(err, ErrUnknownCompany):
		return httpx.FieldErrors{"idPerusahaan": "Choose a company from the list"}, true
	case errors.Is(err, ErrUnknownKategori):
		return httpx.FieldErrors{"idKategori": "Choose a category from the list"}, true
	case errors.Is(err, ErrUnknownLab):
		return httpx.FieldErrors{"labIds": "A chosen lab examination no longer exists; reload the page"}, true
	}
	return nil, false
}
