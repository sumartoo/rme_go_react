// Package perusahaan serves Master Data > Perusahaan: the client companies
// (refcompany). There is no delete, like the original screen.
package perusahaan

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 002_create_menus.sql).
const PagePath = "/master-data/perusahaan"

var (
	ErrNotFound  = errors.New("perusahaan not found")
	ErrDuplicate = errors.New("kode perusahaan already exists")
)

// Perusahaan is a refcompany row.
type Perusahaan struct {
	ID int `json:"id"`
	Input
}

// Input is the editable part of refcompany. IsVat is "1" (PPN), "0" (no PPN)
// or "" (not set; most rows in lab).
type Input struct {
	Kode          string `json:"kode"`
	Nama          string `json:"nama"`
	Alamat        string `json:"alamat"`
	ContactPerson string `json:"contactPerson"`
	Email         string `json:"email"`
	Telepon       string `json:"telepon"`
	Fax           string `json:"fax"`
	IsVat         string `json:"isVat"`
}

// Normalize trims text and validates it against the refcompany column sizes.
func (in *Input) Normalize() httpx.FieldErrors {
	for _, f := range []*string{&in.Kode, &in.Nama, &in.Alamat, &in.ContactPerson, &in.Email, &in.Telepon, &in.Fax, &in.IsVat} {
		*f = strings.TrimSpace(*f)
	}
	errs := httpx.FieldErrors{}
	if in.Kode == "" {
		errs["kode"] = "Kode Perusahaan is required"
	}
	if in.Nama == "" {
		errs["nama"] = "Nama Perusahaan is required"
	}
	maxLen := func(field, label, v string, n int) {
		if _, done := errs[field]; !done && utf8.RuneCountInString(v) > n {
			errs[field] = fmt.Sprintf("%s must be at most %d characters", label, n)
		}
	}
	maxLen("kode", "Kode Perusahaan", in.Kode, 10)
	maxLen("nama", "Nama Perusahaan", in.Nama, 255)
	maxLen("alamat", "Alamat", in.Alamat, 255)
	maxLen("contactPerson", "Contact Person", in.ContactPerson, 100)
	maxLen("email", "Email", in.Email, 100)
	maxLen("telepon", "Telepon", in.Telepon, 30)
	maxLen("fax", "Fax", in.Fax, 30)
	if in.IsVat != "" && in.IsVat != "0" && in.IsVat != "1" {
		errs["isVat"] = "Choose a value from the list"
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

type Repository interface {
	List(ctx context.Context) ([]Perusahaan, error)
	Create(ctx context.Context, in Input) (Perusahaan, error)
	Update(ctx context.Context, id int, in Input) (Perusahaan, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (p *PostgresRepository) List(ctx context.Context) ([]Perusahaan, error) {
	rows, err := p.pool.Query(ctx, `SELECT companyid, COALESCE(companycode, ''), COALESCE(companyname, ''),
		COALESCE(companyaddress, ''), COALESCE(companycontacperson, ''), COALESCE(companymail, ''),
		COALESCE(companyphone, ''), COALESCE(companyfax, ''), COALESCE(isvat, '')
		FROM refcompany ORDER BY companycode, companyid`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Perusahaan, error) {
		var x Perusahaan
		err := r.Scan(&x.ID, &x.Kode, &x.Nama, &x.Alamat, &x.ContactPerson, &x.Email, &x.Telepon, &x.Fax, &x.IsVat)
		return x, err
	})
	if list == nil {
		list = []Perusahaan{}
	}
	return list, err
}

func nullIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func (p *PostgresRepository) Create(ctx context.Context, in Input) (Perusahaan, error) {
	x := Perusahaan{Input: in}
	err := p.pool.QueryRow(ctx,
		`INSERT INTO refcompany (companycode, companyname, companyaddress, companycontacperson, companymail, companyphone, companyfax, isvat)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, $8) RETURNING companyid`,
		in.Kode, in.Nama, nullIfEmpty(in.Alamat), nullIfEmpty(in.ContactPerson), nullIfEmpty(in.Email),
		nullIfEmpty(in.Telepon), nullIfEmpty(in.Fax), nullIfEmpty(in.IsVat),
	).Scan(&x.ID)
	return x, translate(err)
}

func (p *PostgresRepository) Update(ctx context.Context, id int, in Input) (Perusahaan, error) {
	tag, err := p.pool.Exec(ctx,
		`UPDATE refcompany SET companycode = $2, companyname = $3, companyaddress = $4, companycontacperson = $5,
		 companymail = $6, companyphone = $7, companyfax = $8, isvat = $9 WHERE companyid = $1`,
		id, in.Kode, in.Nama, nullIfEmpty(in.Alamat), nullIfEmpty(in.ContactPerson), nullIfEmpty(in.Email),
		nullIfEmpty(in.Telepon), nullIfEmpty(in.Fax), nullIfEmpty(in.IsVat))
	if err != nil {
		return Perusahaan{}, translate(err)
	}
	if tag.RowsAffected() == 0 {
		return Perusahaan{}, ErrNotFound
	}
	return Perusahaan{ID: id, Input: in}, nil
}

func translate(err error) error {
	var pgErr *pgconn.PgError
	if errors.As(err, &pgErr) && pgErr.ConstraintName == "refcompany_companycode_key" {
		return ErrDuplicate
	}
	return err
}

// ---------- HTTP ----------

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// Routes registers the endpoints for callers whose role can open PagePath.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	guard := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(PagePath)(f)) }
	mux.Handle("GET /api/perusahaan", guard(h.List))
	mux.Handle("POST /api/perusahaan", guard(h.Create))
	mux.Handle("PUT /api/perusahaan/{id}", guard(h.Update))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"perusahaan": list})
}

func decode(w http.ResponseWriter, r *http.Request) (Input, bool) {
	var in Input
	if !httpx.DecodeJSON(w, r, &in) {
		return in, false
	}
	if errs := in.Normalize(); len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return in, false
	}
	return in, true
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "perusahaan not found")
	case errors.Is(err, ErrDuplicate):
		httpx.WriteJSON(w, http.StatusConflict, map[string]any{
			"error": "kode perusahaan already exists", "errors": httpx.FieldErrors{"kode": "Kode Perusahaan is already used"},
		})
	default:
		httpx.InternalError(w, err)
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	in, ok := decode(w, r)
	if !ok {
		return
	}
	x, err := h.repo.Create(r.Context(), in)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, x)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, "perusahaan not found")
		return
	}
	in, ok := decode(w, r)
	if !ok {
		return
	}
	x, err := h.repo.Update(r.Context(), id, in)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, x)
}
