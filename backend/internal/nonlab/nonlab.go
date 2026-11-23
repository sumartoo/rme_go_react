// Package nonlab serves Master Data > Non Lab: services sold with an MCU that
// are not lab examinations (refnonlab: name and price). There is no delete,
// like the original screen; soft-deleted rows (isdeleted) are hidden.
package nonlab

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 002_create_menus.sql).
const PagePath = "/master-data/non-lab"

var ErrNotFound = errors.New("non lab not found")

// NonLab is a refnonlab row.
type NonLab struct {
	ID int `json:"id"`
	Input
}

// Input is the editable part of refnonlab. Price is null when not set.
type Input struct {
	Nama  string   `json:"nama"`
	Price *float64 `json:"price"`
}

// Normalize trims the name and validates it against the column size (varchar(100)).
func (in *Input) Normalize() httpx.FieldErrors {
	in.Nama = strings.TrimSpace(in.Nama)
	errs := httpx.FieldErrors{}
	switch {
	case in.Nama == "":
		errs["nama"] = "Non Lab Name is required"
	case utf8.RuneCountInString(in.Nama) > 100:
		errs["nama"] = "Non Lab Name must be at most 100 characters"
	}
	if in.Price != nil && (math.IsNaN(*in.Price) || *in.Price < 0 || *in.Price > 1e12) {
		errs["price"] = "Price must be a number from 0"
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

type Repository interface {
	List(ctx context.Context) ([]NonLab, error)
	Create(ctx context.Context, in Input, user string) (NonLab, error)
	Update(ctx context.Context, id int, in Input, user string) (NonLab, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (p *PostgresRepository) List(ctx context.Context) ([]NonLab, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, COALESCE(name, ''), price FROM refnonlab WHERE isdeleted IS NOT TRUE ORDER BY id`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (NonLab, error) {
		var x NonLab
		err := r.Scan(&x.ID, &x.Nama, &x.Price)
		return x, err
	})
	if list == nil {
		list = []NonLab{}
	}
	return list, err
}

func (p *PostgresRepository) Create(ctx context.Context, in Input, user string) (NonLab, error) {
	x := NonLab{Input: in}
	err := p.pool.QueryRow(ctx,
		`INSERT INTO refnonlab (name, price, isdeleted, createby, createddate)
		 VALUES ($1, $2, false, $3, now() AT TIME ZONE 'Asia/Jakarta') RETURNING id`,
		in.Nama, in.Price, user).Scan(&x.ID)
	return x, err
}

func (p *PostgresRepository) Update(ctx context.Context, id int, in Input, user string) (NonLab, error) {
	tag, err := p.pool.Exec(ctx,
		`UPDATE refnonlab SET name = $2, price = $3, modifiedby = $4, modifieddate = now() AT TIME ZONE 'Asia/Jakarta'
		 WHERE id = $1 AND isdeleted IS NOT TRUE`,
		id, in.Nama, in.Price, user)
	if err != nil {
		return NonLab{}, err
	}
	if tag.RowsAffected() == 0 {
		return NonLab{}, ErrNotFound
	}
	return NonLab{ID: id, Input: in}, nil
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
	mux.Handle("GET /api/non-lab", guard(h.List))
	mux.Handle("POST /api/non-lab", guard(h.Create))
	mux.Handle("PUT /api/non-lab/{id}", guard(h.Update))
}

// user is what goes into createby/modifiedby (varchar(30)).
func user(r *http.Request) string {
	id, _ := auth.IdentityFrom(r.Context())
	u := []rune(id.Email)
	if len(u) > 30 {
		u = u[:30]
	}
	return string(u)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"nonLab": list})
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
	x, err := h.repo.Create(r.Context(), in, user(r))
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, x)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, "non lab not found")
		return
	}
	in, ok := decode(w, r)
	if !ok {
		return
	}
	x, err := h.repo.Update(r.Context(), id, in, user(r))
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "non lab not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, x)
}
