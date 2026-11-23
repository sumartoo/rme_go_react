// Package doktermaster serves Master Data > Dokter Master: the kinds of MCU
// doctor (refdoctormaster: id and name). There is no delete, like the
// original screen.
package doktermaster

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
const PagePath = "/master-data/dokter-master"

var ErrNotFound = errors.New("dokter master not found")

// DokterMaster is a refdoctormaster row.
type DokterMaster struct {
	ID   int    `json:"id"`
	Nama string `json:"nama"`
}

// Input is the editable part of refdoctormaster.
type Input struct {
	Nama string `json:"nama"`
}

// Normalize trims the name and validates it against the column size (varchar(50)).
func (in *Input) Normalize() httpx.FieldErrors {
	in.Nama = strings.TrimSpace(in.Nama)
	switch {
	case in.Nama == "":
		return httpx.FieldErrors{"nama": "Dokter Master Name is required"}
	case utf8.RuneCountInString(in.Nama) > 50:
		return httpx.FieldErrors{"nama": "Dokter Master Name must be at most 50 characters"}
	}
	return nil
}

type Repository interface {
	List(ctx context.Context) ([]DokterMaster, error)
	Create(ctx context.Context, in Input) (DokterMaster, error)
	Update(ctx context.Context, id int, in Input) (DokterMaster, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (p *PostgresRepository) List(ctx context.Context) ([]DokterMaster, error) {
	rows, err := p.pool.Query(ctx, `SELECT doctormaster, COALESCE(doctormastername, '') FROM refdoctormaster ORDER BY doctormastername, doctormaster`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (DokterMaster, error) {
		var x DokterMaster
		err := r.Scan(&x.ID, &x.Nama)
		return x, err
	})
	if list == nil {
		list = []DokterMaster{}
	}
	return list, err
}

func (p *PostgresRepository) Create(ctx context.Context, in Input) (DokterMaster, error) {
	x := DokterMaster{Nama: in.Nama}
	err := p.pool.QueryRow(ctx, `INSERT INTO refdoctormaster (doctormastername) VALUES ($1) RETURNING doctormaster`, in.Nama).Scan(&x.ID)
	return x, err
}

func (p *PostgresRepository) Update(ctx context.Context, id int, in Input) (DokterMaster, error) {
	tag, err := p.pool.Exec(ctx, `UPDATE refdoctormaster SET doctormastername = $2 WHERE doctormaster = $1`, id, in.Nama)
	if err != nil {
		return DokterMaster{}, err
	}
	if tag.RowsAffected() == 0 {
		return DokterMaster{}, ErrNotFound
	}
	return DokterMaster{ID: id, Nama: in.Nama}, nil
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
	mux.Handle("GET /api/dokter-master", guard(h.List))
	mux.Handle("POST /api/dokter-master", guard(h.Create))
	mux.Handle("PUT /api/dokter-master/{id}", guard(h.Update))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"dokterMaster": list})
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
		httpx.WriteError(w, http.StatusNotFound, "dokter master not found")
		return
	}
	in, ok := decode(w, r)
	if !ok {
		return
	}
	x, err := h.repo.Update(r.Context(), id, in)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "dokter master not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, x)
}
