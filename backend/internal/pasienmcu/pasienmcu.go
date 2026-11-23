// Package pasienmcu serves Master Data > Pasien: the MCU patients (refpasien),
// with their company (refcompany). The list is paged, searched and sorted in
// the database. There is no delete, like the original screen; soft-deleted
// rows (isdeleted) are hidden.
package pasienmcu

import (
	"context"
	"errors"
	"fmt"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// ClinicTimeZone decides the date in a new No RM.
const ClinicTimeZone = "Asia/Jakarta"

var (
	ErrNotFound       = errors.New("pasien not found")
	ErrRMExhausted    = errors.New("no free No RM left today")
	ErrUnknownCompany = errors.New("unknown perusahaan")
)

// Pasien is a refpasien row with its company's name.
type Pasien struct {
	ID         int    `json:"id"`
	RM         string `json:"rm"`
	Perusahaan string `json:"perusahaan"` // refcompany.companyname
	Input
}

// Input is the editable part of refpasien. Gender is "L", "P" or "" (a few
// rows in lab); TglLahir is YYYY-MM-DD or "".
type Input struct {
	Nama           string `json:"nama"`
	Gender         string `json:"gender"`
	TglLahir       string `json:"tglLahir"`
	Telp           string `json:"telp"`
	HP             string `json:"hp"`
	Alamat         string `json:"alamat"`
	KodePerusahaan string `json:"kodePerusahaan"`
	NIK            string `json:"nik"`
	Email          string `json:"email"`
	Departemen     string `json:"departemen"`
	Posisi         string `json:"posisi"`
}

// Normalize trims text and validates it against the refpasien column sizes.
// today is the clinic's date (YYYY-MM-DD); a birth date may not be after it.
func (in *Input) Normalize(today string) httpx.FieldErrors {
	for _, f := range []*string{&in.Nama, &in.Gender, &in.TglLahir, &in.Telp, &in.HP, &in.Alamat, &in.KodePerusahaan, &in.NIK, &in.Email, &in.Departemen, &in.Posisi} {
		*f = strings.TrimSpace(*f)
	}
	in.Gender = strings.ToUpper(in.Gender)
	errs := httpx.FieldErrors{}
	if in.Nama == "" {
		errs["nama"] = "Nama is required"
	}
	if in.Gender != "L" && in.Gender != "P" {
		errs["gender"] = "Choose Laki-Laki or Perempuan"
	}
	if in.KodePerusahaan == "" {
		errs["kodePerusahaan"] = "Perusahaan is required"
	}
	if in.TglLahir != "" {
		d, err := time.Parse("2006-01-02", in.TglLahir)
		switch {
		case err != nil:
			errs["tglLahir"] = "Use the format YYYY-MM-DD"
		case d.Format("2006-01-02") > today:
			errs["tglLahir"] = "Tanggal Lahir cannot be in the future"
		}
	}
	// "-" is what lab stores for "none"; anything else must look like an address.
	if in.Email != "" && in.Email != "-" {
		if a, err := mail.ParseAddress(in.Email); err != nil || a.Address != in.Email {
			errs["email"] = "Enter a valid email address"
		}
	}
	maxLen := func(field, label, v string, n int) {
		if _, done := errs[field]; !done && utf8.RuneCountInString(v) > n {
			errs[field] = fmt.Sprintf("%s must be at most %d characters", label, n)
		}
	}
	maxLen("nama", "Nama", in.Nama, 100)
	maxLen("telp", "No Telp", in.Telp, 15)
	maxLen("hp", "No Hp", in.HP, 15)
	maxLen("alamat", "Alamat", in.Alamat, 200)
	maxLen("kodePerusahaan", "Perusahaan", in.KodePerusahaan, 20)
	maxLen("nik", "NIK", in.NIK, 20)
	maxLen("email", "Email", in.Email, 50)
	maxLen("departemen", "Departemen", in.Departemen, 50)
	maxLen("posisi", "Posisi", in.Posisi, 50)
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// ListParams selects one page of patients.
type ListParams struct {
	Search string
	Sort   string // a key of sortColumns
	Desc   bool
	Limit  int
	Offset int
}

// Page is one page of patients. UnfilteredTotal ignores the search.
type Page struct {
	Rows            []Pasien `json:"rows"`
	Total           int      `json:"total"`
	UnfilteredTotal int      `json:"unfilteredTotal"`
}

// Company is a choice for the Perusahaan field.
type Company struct {
	Kode string `json:"kode"`
	Nama string `json:"nama"`
}

// sortColumns maps the API's sort keys to SQL.
var sortColumns = map[string]string{
	"rm":         "p.rm",
	"nama":       "p.pasienname",
	"gender":     "p.gender",
	"tglLahir":   "p.birthdaydate",
	"telp":       "p.telp",
	"hp":         "p.hp",
	"alamat":     "p.address",
	"perusahaan": "c.companyname",
	"nik":        "p.nik",
	"email":      "p.mail",
	"departemen": "p.departmen",
	"posisi":     "p.position",
}

type Repository interface {
	List(ctx context.Context, lp ListParams) (Page, error)
	Companies(ctx context.Context) ([]Company, error)
	Create(ctx context.Context, in Input, user string) (Pasien, error)
	Update(ctx context.Context, id int, in Input, user string) (Pasien, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// escapeLike makes s match literally inside ILIKE.
func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

const from = `refpasien p LEFT JOIN refcompany c ON c.companycode = p.companycode`

// where hides soft-deleted rows and, with a search, matches No RM, name, NIK,
// address or company name.
func where(search string) (string, []any) {
	sql := `p.isdeleted IS NOT TRUE`
	if search == "" {
		return sql, nil
	}
	return sql + ` AND (p.rm ILIKE $1 OR p.pasienname ILIKE $1 OR p.nik ILIKE $1 OR p.address ILIKE $1 OR c.companyname ILIKE $1)`,
		[]any{"%" + escapeLike(search) + "%"}
}

const columns = `p.pasienid, p.rm, COALESCE(p.pasienname, ''), COALESCE(p.gender, ''),
	COALESCE(to_char(p.birthdaydate, 'YYYY-MM-DD'), ''), COALESCE(p.telp, ''), COALESCE(p.hp, ''),
	COALESCE(p.address, ''), COALESCE(p.companycode, ''), COALESCE(c.companyname, ''), COALESCE(p.nik, ''),
	COALESCE(p.mail, ''), COALESCE(p.departmen, ''), COALESCE(p.position, '')`

func scan(r pgx.CollectableRow) (Pasien, error) {
	var x Pasien
	err := r.Scan(&x.ID, &x.RM, &x.Nama, &x.Gender, &x.TglLahir, &x.Telp, &x.HP, &x.Alamat,
		&x.KodePerusahaan, &x.Perusahaan, &x.NIK, &x.Email, &x.Departemen, &x.Posisi)
	return x, err
}

func (p *PostgresRepository) List(ctx context.Context, lp ListParams) (Page, error) {
	page := Page{Rows: []Pasien{}}
	col, ok := sortColumns[lp.Sort]
	if !ok {
		col = "p.rm"
	}
	dir := "ASC"
	if lp.Desc {
		dir = "DESC"
	}
	cond, args := where(lp.Search)
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(
		`SELECT %s FROM %s WHERE %s ORDER BY %s %s NULLS LAST, p.pasienid %s LIMIT $%d OFFSET $%d`,
		columns, from, cond, col, dir, dir, n+1, n+2), append(args, lp.Limit, lp.Offset)...)
	if err != nil {
		return page, err
	}
	if page.Rows, err = pgx.CollectRows(rows, scan); err != nil {
		return page, err
	}
	if page.Rows == nil {
		page.Rows = []Pasien{}
	}
	if err := p.pool.QueryRow(ctx, `SELECT count(*) FROM `+from+` WHERE `+cond, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	if lp.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = where("")
	err = p.pool.QueryRow(ctx, `SELECT count(*) FROM `+from+` WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}

func (p *PostgresRepository) Companies(ctx context.Context) ([]Company, error) {
	rows, err := p.pool.Query(ctx, `SELECT companycode, COALESCE(companyname, '') FROM refcompany WHERE companycode IS NOT NULL ORDER BY companyname, companycode`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Company, error) {
		var c Company
		err := r.Scan(&c.Kode, &c.Nama)
		return c, err
	})
	if list == nil {
		list = []Company{}
	}
	return list, err
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// companyName returns the company's name, or ErrUnknownCompany.
func companyName(ctx context.Context, q interface {
	QueryRow(context.Context, string, ...any) pgx.Row
}, kode string) (string, error) {
	var nama string
	err := q.QueryRow(ctx, `SELECT COALESCE(companyname, '') FROM refcompany WHERE companycode = $1`, kode).Scan(&nama)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUnknownCompany
	}
	return nama, err
}

// Create gives the patient the next No RM of today: YYMMDD plus a 4-digit
// sequence, like lab. A transaction-scoped advisory lock serialises numbering.
func (p *PostgresRepository) Create(ctx context.Context, in Input, user string) (Pasien, error) {
	x := Pasien{Input: in}
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		var err error
		if x.Perusahaan, err = companyName(ctx, tx, in.KodePerusahaan); err != nil {
			return err
		}
		if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('refpasien.rm'))`); err != nil {
			return err
		}
		var prefix string
		var last int
		err = tx.QueryRow(ctx,
			`SELECT to_char(now() AT TIME ZONE '`+ClinicTimeZone+`', 'YYMMDD') AS prefix,
			        COALESCE((SELECT max(right(rm, 4)::int) FROM refpasien
			                  WHERE rm ~ '^[0-9]{10}$' AND left(rm, 6) = to_char(now() AT TIME ZONE '`+ClinicTimeZone+`', 'YYMMDD')), 0)`,
		).Scan(&prefix, &last)
		if err != nil {
			return err
		}
		if last >= 9999 {
			return ErrRMExhausted
		}
		x.RM = fmt.Sprintf("%s%04d", prefix, last+1)
		return tx.QueryRow(ctx,
			`INSERT INTO refpasien (rm, pasienname, gender, birthdaydate, telp, hp, address, companycode, nik, mail,
			                        departmen, position, isdeleted, createdby, createddate)
			 VALUES ($1, $2, $3, $4::date, $5, $6, $7, $8, $9, $10, $11, $12, false, $13, now() AT TIME ZONE '`+ClinicTimeZone+`')
			 RETURNING pasienid`,
			x.RM, in.Nama, in.Gender, nullIfEmpty(in.TglLahir), nullIfEmpty(in.Telp), nullIfEmpty(in.HP), nullIfEmpty(in.Alamat),
			in.KodePerusahaan, nullIfEmpty(in.NIK), nullIfEmpty(in.Email), nullIfEmpty(in.Departemen), nullIfEmpty(in.Posisi), user,
		).Scan(&x.ID)
	})
	return x, err
}

func (p *PostgresRepository) Update(ctx context.Context, id int, in Input, user string) (Pasien, error) {
	x := Pasien{ID: id, Input: in}
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		var err error
		if x.Perusahaan, err = companyName(ctx, tx, in.KodePerusahaan); err != nil {
			return err
		}
		err = tx.QueryRow(ctx,
			`UPDATE refpasien SET pasienname = $2, gender = $3, birthdaydate = $4::date, telp = $5, hp = $6, address = $7,
			        companycode = $8, nik = $9, mail = $10, departmen = $11, position = $12,
			        modifyby = $13, modifydate = now() AT TIME ZONE '`+ClinicTimeZone+`'
			 WHERE pasienid = $1 AND isdeleted IS NOT TRUE RETURNING rm`,
			id, in.Nama, in.Gender, nullIfEmpty(in.TglLahir), nullIfEmpty(in.Telp), nullIfEmpty(in.HP), nullIfEmpty(in.Alamat),
			in.KodePerusahaan, nullIfEmpty(in.NIK), nullIfEmpty(in.Email), nullIfEmpty(in.Departemen), nullIfEmpty(in.Posisi), user,
		).Scan(&x.RM)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound
		}
		return err
	})
	return x, err
}
