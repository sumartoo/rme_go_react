package mcuhasil

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrNotFound  = errors.New("registrasi MCU not found")
	ErrCancelled = errors.New("registrasi MCU dibatalkan")
)

// Registrasi is an MCU registration with its patient.
type Registrasi struct {
	ID         int     `json:"id"`
	Tanggal    string  `json:"tanggal"` // YYYY-MM-DD
	Status     string  `json:"status"`  // New, Close, Cancel
	PasienID   int     `json:"pasienId"`
	RM         string  `json:"rm"`
	Nama       string  `json:"nama"`
	Gender     string  `json:"gender"`
	TglLahir   *string `json:"tglLahir"`
	Usia       *int    `json:"usia"` // years on the MCU day
	Perusahaan string  `json:"perusahaan"`
	Departemen string  `json:"departemen"`
	Jabatan    string  `json:"jabatan"`
	FormStatus *string `json:"formStatus"` // this form's status, null when not filled yet
}

// ListParams selects registrations.
type ListParams struct {
	Form     string
	Search   string // name, RM, NIK or company
	From, To string // YYYY-MM-DD of mcudate
	Cancel   bool   // include cancelled registrations
	Limit    int
	Offset   int
}

type Page struct {
	Rows  []Registrasi `json:"rows"`
	Total int          `json:"total"`
}

// Option is an answer a catalogue item offers.
type Option struct {
	Nama  string `json:"nama"`
	Entry string `json:"entry"` // "radio" or "text" (a text box, Nama is its unit/label)
}

// Item is a catalogue question.
type Item struct {
	Key     string   `json:"key"` // check_up_type.item_id
	Nama    string   `json:"nama"`
	Options []Option `json:"options"`
}

// Section is a catalogue section (mcu_type) with its items.
type Section struct {
	Kode  string `json:"kode"` // check_up_type
	Nama  string `json:"nama"`
	Grup  string `json:"grup"` // mcu_group name
	Items []Item `json:"items"`
}

// Value is one answer.
type Value struct {
	Value string `json:"value"`
	Note  string `json:"note"`
}

// Hasil is a form's saved result.
type Hasil struct {
	Status     string           `json:"status"`
	Kesimpulan string           `json:"kesimpulan"`
	UpdatedBy  string           `json:"updatedBy"`
	UpdatedAt  *time.Time       `json:"updatedAt"`
	Values     map[string]Value `json:"values"`
}

// LabItem is an examination ordered with the registration (Pemeriksaan Lab).
type LabItem struct {
	LabID int    `json:"labId"` // its answer is stored under "lab.<labId>"
	Kode  string `json:"kode"`
	Nama  string `json:"nama"`
	Grup  string `json:"grup"`  // lab sub master
	Paket string `json:"paket"` // the package it came with, empty when ordered alone
}

type Repository interface {
	List(ctx context.Context, p ListParams) (Page, error)
	Registrasi(ctx context.Context, id int) (Registrasi, error)
	Katalog(ctx context.Context, groups []string) ([]Section, error)
	Hasil(ctx context.Context, id int, form string) (Hasil, error)
	Save(ctx context.Context, id int, form string, h Hasil, user string) error
	LabItems(ctx context.Context, id int) ([]LabItem, error)
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

const regSelect = `SELECT h.id, to_char(h.mcudate, 'YYYY-MM-DD'), COALESCE(h.status, ''), COALESCE(h.pasienid, 0),
	COALESCE(p.rm, ''), COALESCE(p.pasienname, ''), COALESCE(p.gender, ''), to_char(p.birthdaydate, 'YYYY-MM-DD'),
	date_part('year', age(h.mcudate::date, p.birthdaydate::date))::int,
	COALESCE(c.companyname, p.companycode, ''), COALESCE(p.departmen, ''), COALESCE(p.position, ''), f.status`

func regFrom(form string) (string, []any) {
	return ` FROM trxmcuheader h
	LEFT JOIN refpasien p ON p.pasienid = h.pasienid
	LEFT JOIN LATERAL (SELECT rc.companyname FROM refcompany rc WHERE rc.companycode = p.companycode ORDER BY rc.companyid LIMIT 1) c ON true
	LEFT JOIN mcu_hasil_form f ON f.header_id = h.id AND f.form = $1`, []any{form}
}

func scanReg(row pgx.Row, extra ...any) (Registrasi, error) {
	var r Registrasi
	err := row.Scan(append([]any{&r.ID, &r.Tanggal, &r.Status, &r.PasienID, &r.RM, &r.Nama, &r.Gender, &r.TglLahir, &r.Usia,
		&r.Perusahaan, &r.Departemen, &r.Jabatan, &r.FormStatus}, extra...)...)
	return r, err
}

// List returns registrations by MCU date, newest first, with this form's status.
func (p *PostgresRepository) List(ctx context.Context, lp ListParams) (Page, error) {
	page := Page{Rows: []Registrasi{}}
	from, args := regFrom(lp.Form)
	conds := []string{`h.mcudate >= $2::date`, `h.mcudate < $3::date + 1`}
	args = append(args, lp.From, lp.To)
	if !lp.Cancel {
		conds = append(conds, `COALESCE(h.status, '') <> 'Cancel'`)
	}
	if lp.Search != "" {
		args = append(args, "%"+escapeLike(lp.Search)+"%")
		n := len(args)
		conds = append(conds, fmt.Sprintf(`(p.pasienname ILIKE $%[1]d OR p.rm ILIKE $%[1]d OR p.nik ILIKE $%[1]d OR c.companyname ILIKE $%[1]d OR h.id::text = $%[2]d)`, n, n+1))
		args = append(args, lp.Search)
	}
	args = append(args, lp.Limit, lp.Offset)
	rows, err := p.pool.Query(ctx, regSelect+`, count(*) OVER ()`+from+` WHERE `+strings.Join(conds, " AND ")+
		fmt.Sprintf(` ORDER BY h.mcudate DESC, h.id DESC LIMIT $%d OFFSET $%d`, len(args)-1, len(args)), args...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		r, err := scanReg(rows, &page.Total)
		if err != nil {
			return page, err
		}
		page.Rows = append(page.Rows, r)
	}
	return page, rows.Err()
}

// Registrasi reads one registration (FormStatus left nil).
func (p *PostgresRepository) Registrasi(ctx context.Context, id int) (Registrasi, error) {
	from, args := regFrom("")
	r, err := scanReg(p.pool.QueryRow(ctx, regSelect+from+` WHERE h.id = $2`, append(args, id)...))
	if errors.Is(err, pgx.ErrNoRows) {
		return r, ErrNotFound
	}
	return r, err
}

// Katalog lays out the catalogue groups: their sections, items and answers.
// Expired items (expired_date in the past) are left out.
func (p *PostgresRepository) Katalog(ctx context.Context, groups []string) ([]Section, error) {
	out := []Section{}
	if len(groups) == 0 {
		return out, nil
	}
	rows, err := p.pool.Query(ctx, `SELECT trim(t.check_up_type), COALESCE(t.name, ''), COALESCE(g.group_name, ''),
			trim(i.item_id), COALESCE(i.item_description, ''),
			COALESCE(trim(r.resulted_name), ''), COALESCE(r.entry_type, '')
		FROM mcu_type t
		JOIN mcu_group g ON g.group_type = t.group_type
		LEFT JOIN mcu_items i ON i.check_up_type = t.check_up_type AND (i.expired_date IS NULL OR i.expired_date >= current_date)
		LEFT JOIN phisic_resulted_type pr ON pr.check_up_type = i.check_up_type AND pr.item_id = i.item_id
		LEFT JOIN resulted_type r ON r.resulted_type = pr.resulted_type
		WHERE t.group_type = ANY($1) AND (t.expired_date IS NULL OR t.expired_date >= current_date)
		ORDER BY g.sort_id, t.group_type, t.check_up_type, i.item_id, pr.sort_id, pr.resulted_type`, groups)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var kode, nama, grup, itemID, itemNama, opt, entry string
		var itemIDN, itemNamaN, optN, entryN *string
		if err := rows.Scan(&kode, &nama, &grup, &itemIDN, &itemNamaN, &optN, &entryN); err != nil {
			return nil, err
		}
		if itemIDN != nil {
			itemID = *itemIDN
		}
		if itemNamaN != nil {
			itemNama = *itemNamaN
		}
		if optN != nil {
			opt = *optN
		}
		if entryN != nil {
			entry = *entryN
		}
		if len(out) == 0 || out[len(out)-1].Kode != kode {
			out = append(out, Section{Kode: kode, Nama: nama, Grup: grup, Items: []Item{}})
		}
		s := &out[len(out)-1]
		if itemID == "" {
			continue
		}
		key := kode + "." + itemID
		if len(s.Items) == 0 || s.Items[len(s.Items)-1].Key != key {
			s.Items = append(s.Items, Item{Key: key, Nama: itemNama, Options: []Option{}})
		}
		it := &s.Items[len(s.Items)-1]
		if entry != "" && !hasOption(it.Options, opt, entry) {
			it.Options = append(it.Options, Option{Nama: opt, Entry: entry})
		}
	}
	return out, rows.Err()
}

// hasOption drops repeated answers (the legacy data lists some twice).
func hasOption(opts []Option, nama, entry string) bool {
	for _, o := range opts {
		if o.Nama == nama && o.Entry == entry {
			return true
		}
	}
	return false
}

// Hasil reads a form's saved result (empty when not filled yet).
func (p *PostgresRepository) Hasil(ctx context.Context, id int, form string) (Hasil, error) {
	h := Hasil{Values: map[string]Value{}}
	var at time.Time
	err := p.pool.QueryRow(ctx, `SELECT status, kesimpulan, updated_by, updated_at FROM mcu_hasil_form WHERE header_id = $1 AND form = $2`,
		id, form).Scan(&h.Status, &h.Kesimpulan, &h.UpdatedBy, &at)
	if errors.Is(err, pgx.ErrNoRows) {
		return h, nil
	}
	if err != nil {
		return h, err
	}
	h.UpdatedAt = &at
	rows, err := p.pool.Query(ctx, `SELECT item_key, value, note FROM mcu_hasil_item WHERE header_id = $1 AND form = $2`, id, form)
	if err != nil {
		return h, err
	}
	defer rows.Close()
	for rows.Next() {
		var k string
		var v Value
		if err := rows.Scan(&k, &v.Value, &v.Note); err != nil {
			return h, err
		}
		h.Values[k] = v
	}
	return h, rows.Err()
}

// Save replaces a form's result in one transaction.
func (p *PostgresRepository) Save(ctx context.Context, id int, form string, h Hasil, user string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck
	var status string
	err = tx.QueryRow(ctx, `SELECT COALESCE(status, '') FROM trxmcuheader WHERE id = $1 FOR SHARE`, id).Scan(&status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if status == "Cancel" {
		return ErrCancelled
	}
	if _, err := tx.Exec(ctx, `INSERT INTO mcu_hasil_form (header_id, form, status, kesimpulan, updated_by, updated_at)
		VALUES ($1, $2, $3, $4, $5, now())
		ON CONFLICT (header_id, form) DO UPDATE SET status = $3, kesimpulan = $4, updated_by = $5, updated_at = now()`,
		id, form, h.Status, h.Kesimpulan, user); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM mcu_hasil_item WHERE header_id = $1 AND form = $2`, id, form); err != nil {
		return err
	}
	if len(h.Values) > 0 {
		rows := make([][]any, 0, len(h.Values))
		for k, v := range h.Values {
			rows = append(rows, []any{id, form, k, v.Value, v.Note})
		}
		if _, err := tx.CopyFrom(ctx, pgx.Identifier{"mcu_hasil_item"}, []string{"header_id", "form", "item_key", "value", "note"},
			pgx.CopyFromRows(rows)); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// LabItems lists the examinations ordered with the registration: those
// ordered one by one and the contents of the ordered packages
// (refpricedetail), each examination once.
func (p *PostgresRepository) LabItems(ctx context.Context, id int) ([]LabItem, error) {
	rows, err := p.pool.Query(ctx, `SELECT * FROM (
			SELECT DISTINCT ON (x.labid) x.labid, COALESCE(l.labcode, ''), COALESCE(l.labname, '(lab ' || x.labid || ')'),
				COALESCE(s.labsubmastername, ''), x.paket
			FROM trxmcudetail d
			CROSS JOIN LATERAL (
				SELECT d.labid AS labid, ''::text AS paket WHERE COALESCE(d.labid, 0) <> 0
				UNION ALL
				SELECT pd.labid, COALESCE(ph.paketname, '')
				FROM refpricedetail pd LEFT JOIN refpriceheader ph ON ph.id = pd.headerid
				WHERE COALESCE(d.paketid, 0) <> 0 AND pd.headerid = d.paketid AND COALESCE(pd.labid, 0) <> 0
			) x
			LEFT JOIN reflab l ON l.labid = x.labid
			LEFT JOIN reflabsubmaster s ON s.labsubmasterid = l.labsubmasterid
			WHERE d.headerid = $1
			ORDER BY x.labid, x.paket DESC) t
		ORDER BY 4, 3`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []LabItem{}
	for rows.Next() {
		var l LabItem
		if err := rows.Scan(&l.LabID, &l.Kode, &l.Nama, &l.Grup, &l.Paket); err != nil {
			return nil, err
		}
		out = append(out, l)
	}
	return out, rows.Err()
}
