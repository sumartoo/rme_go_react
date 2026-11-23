// Package mastermcu serves Master Data > MCU: the MCU examination groups
// (refmastermcu: code and name). There is no delete, like the original screen.
package mastermcu

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 002_create_menus.sql).
const PagePath = "/master-data/mcu"

var ErrNotFound = errors.New("master MCU not found")

// Mcu is a refmastermcu row.
type Mcu struct {
	ID   int    `json:"id"`
	Kode string `json:"kode"`
	Nama string `json:"nama"`
}

// Input is the editable part of refmastermcu.
type Input struct {
	Kode string `json:"kode"`
	Nama string `json:"nama"`
}

// Normalize trims both fields and validates them against the column sizes
// (mcumastercode varchar(10), mcumastername varchar(30)).
func (in *Input) Normalize() httpx.FieldErrors {
	in.Kode = strings.TrimSpace(in.Kode)
	in.Nama = strings.TrimSpace(in.Nama)
	errs := httpx.FieldErrors{}
	switch {
	case in.Kode == "":
		errs["kode"] = "Kode Mcu is required"
	case utf8.RuneCountInString(in.Kode) > 10:
		errs["kode"] = "Kode Mcu must be at most 10 characters"
	}
	switch {
	case in.Nama == "":
		errs["nama"] = "Nama Mcu is required"
	case utf8.RuneCountInString(in.Nama) > 30:
		errs["nama"] = "Nama Mcu must be at most 30 characters"
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

type Repository interface {
	List(ctx context.Context) ([]Mcu, error)
	Create(ctx context.Context, in Input) (Mcu, error)
	Update(ctx context.Context, id int, in Input) (Mcu, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (p *PostgresRepository) List(ctx context.Context) ([]Mcu, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, COALESCE(mcumastercode, ''), COALESCE(mcumastername, '') FROM refmastermcu ORDER BY id`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Mcu, error) {
		var x Mcu
		err := r.Scan(&x.ID, &x.Kode, &x.Nama)
		return x, err
	})
	if list == nil {
		list = []Mcu{}
	}
	return list, err
}

func (p *PostgresRepository) Create(ctx context.Context, in Input) (Mcu, error) {
	x := Mcu{Kode: in.Kode, Nama: in.Nama}
	err := p.pool.QueryRow(ctx, `INSERT INTO refmastermcu (mcumastercode, mcumastername) VALUES ($1, $2) RETURNING id`, in.Kode, in.Nama).Scan(&x.ID)
	return x, err
}

func (p *PostgresRepository) Update(ctx context.Context, id int, in Input) (Mcu, error) {
	tag, err := p.pool.Exec(ctx, `UPDATE refmastermcu SET mcumastercode = $2, mcumastername = $3 WHERE id = $1`, id, in.Kode, in.Nama)
	if err != nil {
		return Mcu{}, err
	}
	if tag.RowsAffected() == 0 {
		return Mcu{}, ErrNotFound
	}
	return Mcu{ID: id, Kode: in.Kode, Nama: in.Nama}, nil
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
	mux.Handle("GET /api/master-mcu", guard(h.List))
	mux.Handle("POST /api/master-mcu", guard(h.Create))
	mux.Handle("PUT /api/master-mcu/{id}", guard(h.Update))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"mcu": list})
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

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	in, ok := decode(w, r)
	if !ok {
		return
	}
	x, err := h.repo.Create(r.Context(), in)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, x)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, "master MCU not found")
		return
	}
	in, ok := decode(w, r)
	if !ok {
		return
	}
	x, err := h.repo.Update(r.Context(), id, in)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "master MCU not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, x)
}
