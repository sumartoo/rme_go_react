// Package mcudaftar serves MCU > Pendaftaran (legacy /Pendaftaran): the MCU
// registrations (trxmcuheader) with what is ordered for them, General lab
// examinations and Paket MCU (trxmcudetail), Non Lab services (trxmcunonlab),
// and their doctors (trxmcudoctor).
//
// One form registers one or more patients: each gets its own registration with
// the same lines. Prices come from the master data (reflab, refpriceheader,
// refnonlab), never from the client, and the registration's price is their sum.
// A registration changes or is cancelled only while its status is New.
package mcudaftar

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// Registration statuses (trxmcuheader.status), as in lab.
const (
	StatusNew    = "New"
	StatusClose  = "Close"
	StatusCancel = "Cancel"
)

// MaxPasien caps the patients of one form; MaxLines the lines of each kind.
const (
	MaxPasien = 200
	MaxLines  = 100
)

var (
	ErrNotFound      = errors.New("pendaftaran mcu not found")
	ErrNotNew        = errors.New("only a New registration can change")
	ErrUnknownPasien = errors.New("unknown pasien")
	ErrUnknownLab    = errors.New("unknown lab")
	ErrUnknownPaket  = errors.New("unknown paket")
	ErrUnknownNonLab = errors.New("unknown non lab")
	ErrUnknownDokter = errors.New("unknown dokter")
	ErrUnknownChoice = errors.New("unknown kategori or info")
)

// Row is a registration in the list.
type Row struct {
	ID         int     `json:"id"`
	McuDate    *string `json:"mcuDate"` // YYYY-MM-DD
	RM         string  `json:"rm"`
	Status     string  `json:"status"`
	Nama       string  `json:"nama"`
	Perusahaan string  `json:"perusahaan"`
	Kategori   string  `json:"kategori"`
	Price      float64 `json:"price"`
	CreateBy   string  `json:"createBy"`
}

// ListParams selects one page of registrations.
type ListParams struct {
	From, To string // YYYY-MM-DD, inclusive, on mcudate
	Status   string // New, Close, Cancel or "" for all
	Search   string
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

// Option is a choice of a select.
type Option struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// Pasien is an MCU patient in the form's lookup (refpasien).
type Pasien struct {
	ID             int    `json:"id"`
	RM             string `json:"rm"`
	Nama           string `json:"nama"`
	KodePerusahaan string `json:"kodePerusahaan"`
	Perusahaan     string `json:"perusahaan"`
	NIK            string `json:"nik"`
}

// Lab is a General examination (reflab) with its master and sub master names.
type Lab struct {
	ID        int     `json:"id"`
	Master    string  `json:"master"`
	SubMaster string  `json:"subMaster"`
	Nama      string  `json:"nama"`
	Price     float64 `json:"price"`
}

// Paket is an MCU package (refpriceheader.ispaket) with its company.
type Paket struct {
	ID         int     `json:"id"`
	Nama       string  `json:"nama"`
	Perusahaan string  `json:"perusahaan"`
	Kategori   string  `json:"kategori"`
	Price      float64 `json:"price"`
}

// NonLab is a non-lab service (refnonlab).
type NonLab struct {
	ID    int     `json:"id"`
	Nama  string  `json:"nama"`
	Price float64 `json:"price"`
}

// Dokter is an MCU doctor (refdoctor) with its master group.
type Dokter struct {
	Kode          string  `json:"kode"`
	Nama          string  `json:"nama"`
	Master        string  `json:"master"`
	FeePercentage float64 `json:"feePercentage"`
}

// Input is the form as sent: the patients (create only) and the ordered lines.
type Input struct {
	McuDate    string   `json:"mcuDate"` // YYYY-MM-DD
	KategoriID int      `json:"kategoriId"`
	InfoID     *int     `json:"infoId"`
	PasienIDs  []int    `json:"pasienIds"`
	LabIDs     []int    `json:"labIds"`
	PaketIDs   []int    `json:"paketIds"`
	NonLabIDs  []int    `json:"nonLabIds"`
	Dokter     []string `json:"dokter"` // refdoctor.doctorcode
}

// Normalize trims and checks the form. withPasien is true when creating.
func (in *Input) Normalize(withPasien bool) httpx.FieldErrors {
	errs := httpx.FieldErrors{}
	in.McuDate = strings.TrimSpace(in.McuDate)
	if _, err := time.Parse("2006-01-02", in.McuDate); err != nil {
		errs["mcuDate"] = "Choose the MCU date"
	}
	if in.KategoriID <= 0 {
		errs["kategoriId"] = "Choose the MCU kategori"
	}
	if in.InfoID != nil && *in.InfoID <= 0 {
		in.InfoID = nil
	}
	ids := func(field, what string, list []int, max int) {
		if len(list) > max {
			errs[field] = "At most " + strconv.Itoa(max) + " " + what
			return
		}
		seen := map[int]bool{}
		for _, id := range list {
			if id <= 0 {
				errs[field] = "Choose a " + what
				return
			}
			if seen[id] {
				errs[field] = "Each " + what + " only once"
				return
			}
			seen[id] = true
		}
	}
	if withPasien {
		if len(in.PasienIDs) == 0 {
			errs["pasienIds"] = "Add at least one patient"
		}
		ids("pasienIds", "patient", in.PasienIDs, MaxPasien)
	}
	ids("labIds", "examination", in.LabIDs, MaxLines)
	ids("paketIds", "package", in.PaketIDs, MaxLines)
	ids("nonLabIds", "non lab service", in.NonLabIDs, MaxLines)
	if len(in.LabIDs)+len(in.PaketIDs)+len(in.NonLabIDs) == 0 {
		errs["labIds"] = "Add a General examination, a Paket MCU or a Non Lab service"
	}
	if len(in.Dokter) > MaxLines {
		errs["dokter"] = "At most " + strconv.Itoa(MaxLines) + " doctors"
	}
	seen := map[string]bool{}
	for i, k := range in.Dokter {
		k = strings.TrimSpace(k)
		in.Dokter[i] = k
		switch {
		case k == "" || utf8.RuneCountInString(k) > 20:
			errs["dokter"] = "Choose a doctor"
		case seen[k]:
			errs["dokter"] = "Each doctor only once"
		}
		seen[k] = true
	}
	return errs
}

// Line is an ordered item of a saved registration.
type Line struct {
	ID    int     `json:"id"` // labid, paketid or nonlabid
	Nama  string  `json:"nama"`
	Extra string  `json:"extra"` // General: master / sub master; Paket: perusahaan
	Price float64 `json:"price"`
}

// Detail is a saved registration with its patient and lines.
type Detail struct {
	ID            int      `json:"id"`
	McuDate       *string  `json:"mcuDate"`
	Status        string   `json:"status"`
	KategoriID    *int     `json:"kategoriId"`
	InfoID        *int     `json:"infoId"`
	Price         float64  `json:"price"`
	CreateBy      string   `json:"createBy"`
	CreatedDate   *string  `json:"createdDate"`
	CancelRemarks string   `json:"cancelRemarks"`
	Pasien        Pasien   `json:"pasien"`
	Lab           []Line   `json:"lab"`
	Paket         []Line   `json:"paket"`
	NonLab        []Line   `json:"nonLab"`
	Dokter        []Dokter `json:"dokter"`
}

type Repository interface {
	List(ctx context.Context, lp ListParams) (Page, error)
	Kategori(ctx context.Context) ([]Option, error)
	Info(ctx context.Context) ([]Option, error)
	Pasien(ctx context.Context, search string, limit, offset int) ([]Pasien, int, error)
	Lab(ctx context.Context) ([]Lab, error)
	Paket(ctx context.Context) ([]Paket, error)
	NonLab(ctx context.Context) ([]NonLab, error)
	Dokter(ctx context.Context) ([]Dokter, error)
	Get(ctx context.Context, id int) (Detail, error)
	Create(ctx context.Context, in Input, user string) ([]int, error)
	Update(ctx context.Context, id int, in Input, user string) error
	Cancel(ctx context.Context, id int, reason, user string) error
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

const listFrom = ` FROM trxmcuheader h
	LEFT JOIN refpasien p ON p.pasienid = h.pasienid
	LEFT JOIN LATERAL (SELECT rc.companyname FROM refcompany rc WHERE rc.companycode = p.companycode ORDER BY rc.companyid LIMIT 1) c ON true
	LEFT JOIN refmcukategori k ON k.id = h.mcukategoriid`

var sortColumns = map[string]string{
	"id":         "h.id",
	"mcuDate":    "h.mcudate",
	"rm":         "p.rm",
	"status":     "h.status",
	"nama":       "p.pasienname",
	"perusahaan": "c.companyname",
	"kategori":   "k.mcukategori",
	"price":      "h.price",
	"createBy":   "h.createby",
}

func where(lp ListParams, withSearch bool) (string, []any) {
	args := []any{lp.From, lp.To}
	conds := []string{`h.mcudate >= $1::date`, `h.mcudate < $2::date + 1`}
	if lp.Status != "" {
		args = append(args, lp.Status)
		conds = append(conds, fmt.Sprintf(`h.status = $%d`, len(args)))
	}
	if withSearch && lp.Search != "" {
		args = append(args, "%"+escapeLike(lp.Search)+"%", lp.Search)
		n := len(args)
		conds = append(conds, fmt.Sprintf(`(p.rm ILIKE $%[1]d OR p.pasienname ILIKE $%[1]d OR c.companyname ILIKE $%[1]d OR h.status ILIKE $%[1]d OR h.id::text = $%[2]d)`, n-1, n))
	}
	return strings.Join(conds, " AND "), args
}

func (p *PostgresRepository) List(ctx context.Context, lp ListParams) (Page, error) {
	page := Page{Rows: []Row{}}
	col, ok := sortColumns[lp.Sort]
	if !ok {
		col = "h.mcudate"
	}
	dir := "ASC"
	if lp.Desc {
		dir = "DESC"
	}
	cond, args := where(lp, true)
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`SELECT h.id, to_char(h.mcudate, 'YYYY-MM-DD'), COALESCE(p.rm, ''), COALESCE(h.status, ''),
		COALESCE(p.pasienname, ''), COALESCE(c.companyname, ''), COALESCE(k.mcukategori, ''), COALESCE(h.price, 0), COALESCE(h.createby, '')
		%s WHERE %s ORDER BY %s %s NULLS LAST, h.id %s LIMIT $%d OFFSET $%d`, listFrom, cond, col, dir, dir, n+1, n+2),
		append(args, lp.Limit, lp.Offset)...)
	if err != nil {
		return page, err
	}
	if page.Rows, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (Row, error) {
		var x Row
		err := r.Scan(&x.ID, &x.McuDate, &x.RM, &x.Status, &x.Nama, &x.Perusahaan, &x.Kategori, &x.Price, &x.CreateBy)
		return x, err
	}); err != nil {
		return page, err
	}
	if page.Rows == nil {
		page.Rows = []Row{}
	}
	if err := p.pool.QueryRow(ctx, `SELECT count(*)`+listFrom+` WHERE `+cond, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	if lp.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = where(lp, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*)`+listFrom+` WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}

func (p *PostgresRepository) options(ctx context.Context, sql string) ([]Option, error) {
	rows, err := p.pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Option])
	if list == nil {
		list = []Option{}
	}
	return list, err
}

func (p *PostgresRepository) Kategori(ctx context.Context) ([]Option, error) {
	return p.options(ctx, `SELECT id, COALESCE(mcukategori, '') FROM refmcukategori ORDER BY id`)
}

func (p *PostgresRepository) Info(ctx context.Context) ([]Option, error) {
	return p.options(ctx, `SELECT id, COALESCE(mcuinfo, '') FROM refmcuinfo ORDER BY id`)
}

const pasienFrom = ` FROM refpasien p
	LEFT JOIN LATERAL (SELECT rc.companyname FROM refcompany rc WHERE rc.companycode = p.companycode ORDER BY rc.companyid LIMIT 1) c ON true
	WHERE p.isdeleted IS NOT TRUE`

// Pasien pages the MCU patients, matching No RM, name, NIK or company.
func (p *PostgresRepository) Pasien(ctx context.Context, search string, limit, offset int) ([]Pasien, int, error) {
	cond, args := "", []any{}
	if search != "" {
		args = append(args, "%"+escapeLike(search)+"%")
		cond = ` AND (p.rm ILIKE $1 OR p.pasienname ILIKE $1 OR p.nik ILIKE $1 OR c.companyname ILIKE $1 OR p.companycode ILIKE $1)`
	}
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`SELECT p.pasienid, COALESCE(p.rm, ''), COALESCE(p.pasienname, ''), COALESCE(p.companycode, ''),
		COALESCE(c.companyname, ''), COALESCE(p.nik, '')%s%s ORDER BY p.pasienid DESC LIMIT $%d OFFSET $%d`, pasienFrom, cond, n+1, n+2),
		append(args, limit, offset)...)
	if err != nil {
		return nil, 0, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Pasien])
	if err != nil {
		return nil, 0, err
	}
	if list == nil {
		list = []Pasien{}
	}
	var total int
	err = p.pool.QueryRow(ctx, `SELECT count(*)`+pasienFrom+cond, args...).Scan(&total)
	return list, total, err
}

func (p *PostgresRepository) Lab(ctx context.Context) ([]Lab, error) {
	rows, err := p.pool.Query(ctx, `SELECT l.labid, COALESCE(m.labmastername, ''), COALESCE(s.labsubmastername, ''), COALESCE(l.labname, ''), COALESCE(l.price, 0)
		FROM reflab l
		LEFT JOIN reflabsubmaster s ON s.labsubmasterid = l.labsubmasterid
		LEFT JOIN reflabmaster m ON m.labmasterid = s.labmasterid
		WHERE l.isdeleted IS NOT TRUE ORDER BY l.labid`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Lab])
	if list == nil {
		list = []Lab{}
	}
	return list, err
}

// Paket lists the active MCU packages, newest first.
func (p *PostgresRepository) Paket(ctx context.Context) ([]Paket, error) {
	rows, err := p.pool.Query(ctx, `SELECT h.id, COALESCE(h.paketname, ''), COALESCE(c.companyname, ''), COALESCE(k.category, ''), COALESCE(h.price, 0)
		FROM refpriceheader h
		LEFT JOIN refcompany c ON c.companyid = h.companyid
		LEFT JOIN refcategory k ON k.id = h.kategoriid
		WHERE h.ispaket IS TRUE AND h.isdeleted IS NOT TRUE AND COALESCE(h.isactive, 1) = 1
		ORDER BY h.id DESC`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Paket])
	if list == nil {
		list = []Paket{}
	}
	return list, err
}

func (p *PostgresRepository) NonLab(ctx context.Context) ([]NonLab, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, COALESCE(name, ''), COALESCE(price, 0) FROM refnonlab WHERE isdeleted IS NOT TRUE ORDER BY id`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[NonLab])
	if list == nil {
		list = []NonLab{}
	}
	return list, err
}

func (p *PostgresRepository) Dokter(ctx context.Context) ([]Dokter, error) {
	rows, err := p.pool.Query(ctx, `SELECT d.doctorcode, COALESCE(d.doctorname, ''), COALESCE(m.doctormastername, ''), COALESCE(d.feepercentage, 0)
		FROM refdoctor d LEFT JOIN refdoctormaster m ON m.doctormaster = d.doctormaster
		ORDER BY d.doctorcode`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Dokter])
	if list == nil {
		list = []Dokter{}
	}
	return list, err
}

func (p *PostgresRepository) Get(ctx context.Context, id int) (Detail, error) {
	d := Detail{Lab: []Line{}, Paket: []Line{}, NonLab: []Line{}, Dokter: []Dokter{}}
	err := p.pool.QueryRow(ctx, `SELECT h.id, to_char(h.mcudate, 'YYYY-MM-DD'), COALESCE(h.status, ''), h.mcukategoriid, h.mcuinfoid,
		COALESCE(h.price, 0), COALESCE(h.createby, ''), to_char(h.createddate, 'YYYY-MM-DD HH24:MI:SS'), COALESCE(h.cancelremarks, ''),
		COALESCE(p.pasienid, 0), COALESCE(p.rm, ''), COALESCE(p.pasienname, ''), COALESCE(p.companycode, ''), COALESCE(c.companyname, ''), COALESCE(p.nik, '')
		FROM trxmcuheader h
		LEFT JOIN refpasien p ON p.pasienid = h.pasienid
		LEFT JOIN LATERAL (SELECT rc.companyname FROM refcompany rc WHERE rc.companycode = p.companycode ORDER BY rc.companyid LIMIT 1) c ON true
		WHERE h.id = $1`, id).Scan(&d.ID, &d.McuDate, &d.Status, &d.KategoriID, &d.InfoID, &d.Price, &d.CreateBy, &d.CreatedDate, &d.CancelRemarks,
		&d.Pasien.ID, &d.Pasien.RM, &d.Pasien.Nama, &d.Pasien.KodePerusahaan, &d.Pasien.Perusahaan, &d.Pasien.NIK)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrNotFound
	}
	if err != nil {
		return d, err
	}
	lines := func(sql string) ([]Line, error) {
		rows, err := p.pool.Query(ctx, sql, id)
		if err != nil {
			return nil, err
		}
		list, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Line])
		if list == nil {
			list = []Line{}
		}
		return list, err
	}
	if d.Lab, err = lines(`SELECT t.labid, COALESCE(l.labname, ''), concat_ws(' / ', NULLIF(m.labmastername, ''), NULLIF(s.labsubmastername, '')), COALESCE(t.price, 0)
		FROM trxmcudetail t
		LEFT JOIN reflab l ON l.labid = t.labid
		LEFT JOIN reflabsubmaster s ON s.labsubmasterid = l.labsubmasterid
		LEFT JOIN reflabmaster m ON m.labmasterid = s.labmasterid
		WHERE t.headerid = $1 AND COALESCE(t.paketid, 0) = 0 AND COALESCE(t.labid, 0) <> 0 ORDER BY t.id`); err != nil {
		return d, err
	}
	if d.Paket, err = lines(`SELECT t.paketid, COALESCE(ph.paketname, ''), COALESCE(c.companyname, ''), COALESCE(t.price, 0)
		FROM trxmcudetail t
		LEFT JOIN refpriceheader ph ON ph.id = t.paketid
		LEFT JOIN refcompany c ON c.companyid = ph.companyid
		WHERE t.headerid = $1 AND COALESCE(t.paketid, 0) <> 0 ORDER BY t.id`); err != nil {
		return d, err
	}
	if d.NonLab, err = lines(`SELECT t.nonlabid, COALESCE(n.name, ''), '', COALESCE(t.price, 0)
		FROM trxmcunonlab t LEFT JOIN refnonlab n ON n.id = t.nonlabid
		WHERE t.headerid = $1 ORDER BY t.id`); err != nil {
		return d, err
	}
	rows, err := p.pool.Query(ctx, `SELECT t.doctor, COALESCE(d.doctorname, ''), COALESCE(m.doctormastername, ''), COALESCE(t.feepercentage, 0)
		FROM trxmcudoctor t
		LEFT JOIN refdoctor d ON d.doctorcode = t.doctor
		LEFT JOIN refdoctormaster m ON m.doctormaster = d.doctormaster
		WHERE t.headerid = $1 ORDER BY t.id`, id)
	if err != nil {
		return d, err
	}
	if d.Dokter, err = pgx.CollectRows(rows, pgx.RowToStructByPos[Dokter]); err != nil {
		return d, err
	}
	if d.Dokter == nil {
		d.Dokter = []Dokter{}
	}
	return d, nil
}

// priced is a line with the price the master data gives it.
type priced struct {
	id    int
	price float64
}

// prices reads each id's price from the master table, in the given order, or
// returns errUnknown when one is missing (or deleted).
func prices(ctx context.Context, tx pgx.Tx, sql string, ids []int, errUnknown error) ([]priced, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	rows, err := tx.Query(ctx, sql, ids)
	if err != nil {
		return nil, err
	}
	got := map[int]float64{}
	for rows.Next() {
		var id int
		var price float64
		if err := rows.Scan(&id, &price); err != nil {
			rows.Close()
			return nil, err
		}
		got[id] = price
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}
	out := make([]priced, 0, len(ids))
	for _, id := range ids {
		price, ok := got[id]
		if !ok {
			return nil, errUnknown
		}
		out = append(out, priced{id, price})
	}
	return out, nil
}

// lines are the priced lines of one form, shared by every patient.
type lines struct {
	lab, paket, nonLab []priced
	dokter             []Dokter
	total              float64
}

func loadLines(ctx context.Context, tx pgx.Tx, in Input) (lines, error) {
	var l lines
	var err error
	if l.lab, err = prices(ctx, tx, `SELECT labid, COALESCE(price, 0) FROM reflab WHERE labid = ANY($1) AND isdeleted IS NOT TRUE`, in.LabIDs, ErrUnknownLab); err != nil {
		return l, err
	}
	if l.paket, err = prices(ctx, tx, `SELECT id, COALESCE(price, 0) FROM refpriceheader WHERE id = ANY($1) AND ispaket IS TRUE AND isdeleted IS NOT TRUE`, in.PaketIDs, ErrUnknownPaket); err != nil {
		return l, err
	}
	if l.nonLab, err = prices(ctx, tx, `SELECT id, COALESCE(price, 0) FROM refnonlab WHERE id = ANY($1) AND isdeleted IS NOT TRUE`, in.NonLabIDs, ErrUnknownNonLab); err != nil {
		return l, err
	}
	for _, list := range [][]priced{l.lab, l.paket, l.nonLab} {
		for _, x := range list {
			l.total += x.price
		}
	}
	if len(in.Dokter) > 0 {
		rows, err := tx.Query(ctx, `SELECT doctorcode, COALESCE(doctorname, ''), '', COALESCE(feepercentage, 0) FROM refdoctor WHERE doctorcode = ANY($1)`, in.Dokter)
		if err != nil {
			return l, err
		}
		found, err := pgx.CollectRows(rows, pgx.RowToStructByPos[Dokter])
		if err != nil {
			return l, err
		}
		byCode := map[string]Dokter{}
		for _, d := range found {
			byCode[d.Kode] = d
		}
		for _, k := range in.Dokter {
			d, ok := byCode[k]
			if !ok {
				return l, ErrUnknownDokter
			}
			l.dokter = append(l.dokter, d)
		}
	}
	return l, nil
}

func checkChoices(ctx context.Context, tx pgx.Tx, in Input) error {
	var ok bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM refmcukategori WHERE id = $1)
		AND ($2::int IS NULL OR EXISTS (SELECT 1 FROM refmcuinfo WHERE id = $2))`, in.KategoriID, in.InfoID).Scan(&ok); err != nil {
		return err
	}
	if !ok {
		return ErrUnknownChoice
	}
	return nil
}

func writeLines(ctx context.Context, tx pgx.Tx, headerID int, l lines) error {
	for _, x := range l.paket {
		if _, err := tx.Exec(ctx, `INSERT INTO trxmcudetail (headerid, paketid, labid, status, result, price) VALUES ($1, $2, 0, '', '', $3)`, headerID, x.id, x.price); err != nil {
			return err
		}
	}
	for _, x := range l.lab {
		if _, err := tx.Exec(ctx, `INSERT INTO trxmcudetail (headerid, paketid, labid, status, result, price) VALUES ($1, 0, $2, '', '', $3)`, headerID, x.id, x.price); err != nil {
			return err
		}
	}
	for _, x := range l.nonLab {
		if _, err := tx.Exec(ctx, `INSERT INTO trxmcunonlab (headerid, nonlabid, price) VALUES ($1, $2, $3)`, headerID, x.id, x.price); err != nil {
			return err
		}
	}
	for _, d := range l.dokter {
		if _, err := tx.Exec(ctx, `INSERT INTO trxmcudoctor (headerid, doctor, feepercentage) VALUES ($1, $2, $3)`, headerID, d.Kode, d.FeePercentage); err != nil {
			return err
		}
	}
	return nil
}

// now is the clinic's wall clock, as lab stores it.
const sqlNow = `(now() AT TIME ZONE 'Asia/Jakarta')`

// Create registers each patient with the same lines, in one transaction.
func (p *PostgresRepository) Create(ctx context.Context, in Input, user string) ([]int, error) {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit
	if err := checkChoices(ctx, tx, in); err != nil {
		return nil, err
	}
	var missing int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM unnest($1::int[]) x(id)
		WHERE NOT EXISTS (SELECT 1 FROM refpasien p WHERE p.pasienid = x.id AND p.isdeleted IS NOT TRUE)`, in.PasienIDs).Scan(&missing); err != nil {
		return nil, err
	}
	if missing > 0 {
		return nil, ErrUnknownPasien
	}
	l, err := loadLines(ctx, tx, in)
	if err != nil {
		return nil, err
	}
	ids := make([]int, 0, len(in.PasienIDs))
	for _, pasien := range in.PasienIDs {
		var id int
		if err := tx.QueryRow(ctx, `INSERT INTO trxmcuheader (pasienid, mcudate, status, createddate, createby, price, mcukategoriid, mcuinfoid)
			VALUES ($1, $2::date, $3, `+sqlNow+`, $4, $5, $6, $7) RETURNING id`,
			pasien, in.McuDate, StatusNew, user, l.total, in.KategoriID, in.InfoID).Scan(&id); err != nil {
			return nil, err
		}
		if err := writeLines(ctx, tx, id, l); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, tx.Commit(ctx)
}

// lockNew locks a registration and checks it is still New.
func lockNew(ctx context.Context, tx pgx.Tx, id int) error {
	var status string
	err := tx.QueryRow(ctx, `SELECT COALESCE(status, '') FROM trxmcuheader WHERE id = $1 FOR UPDATE`, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status != StatusNew {
		return ErrNotNew
	}
	return nil
}

// Update replaces a New registration's date, choices and lines (the patient stays).
func (p *PostgresRepository) Update(ctx context.Context, id int, in Input, user string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := lockNew(ctx, tx, id); err != nil {
		return err
	}
	if err := checkChoices(ctx, tx, in); err != nil {
		return err
	}
	l, err := loadLines(ctx, tx, in)
	if err != nil {
		return err
	}
	for _, t := range []string{"trxmcudetail", "trxmcunonlab", "trxmcudoctor"} {
		if _, err := tx.Exec(ctx, `DELETE FROM `+t+` WHERE headerid = $1`, id); err != nil {
			return err
		}
	}
	if err := writeLines(ctx, tx, id, l); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE trxmcuheader SET mcudate = $2::date, mcukategoriid = $3, mcuinfoid = $4, price = $5,
		modifieddate = `+sqlNow+`, modifiedby = $6 WHERE id = $1`, id, in.McuDate, in.KategoriID, in.InfoID, l.total, user); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// Cancel marks a New registration as Cancel with the reason.
func (p *PostgresRepository) Cancel(ctx context.Context, id int, reason, user string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	if err := lockNew(ctx, tx, id); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `UPDATE trxmcuheader SET status = $2, cancelremarks = $3, canceldate = `+sqlNow+`, cancelby = $4 WHERE id = $1`,
		id, StatusCancel, reason, user); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
