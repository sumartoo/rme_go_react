// Package labprice serves Setting > Price: the price of each lab examination
// (reflab.price), listed with its Sub Master and Lab Master. Only the price is
// edited here; the rest of reflab belongs to Master Data > Lab (package lab).
// Like the original query, rows join their Sub Master and Lab Master with
// inner joins, and Non Active rows are listed too.
package labprice

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 002_create_menus.sql).
const PagePath = "/setting/price"

var ErrNotFound = errors.New("lab not found")

// Price is one lab examination with its price. Price is null when not set.
type Price struct {
	ID           int      `json:"id"` // reflab.labid
	LabMaster    string   `json:"labMaster"`
	LabSubMaster string   `json:"labSubMaster"`
	Nama         string   `json:"nama"` // reflab.labname
	Price        *float64 `json:"price"`
	IsActive     bool     `json:"isActive"` // NOT isdeleted
}

// Input is the new price; null clears it.
type Input struct {
	Price *float64 `json:"price"`
}

// Normalize checks the price is a number from 0.
func (in *Input) Normalize() httpx.FieldErrors {
	if in.Price != nil && (math.IsNaN(*in.Price) || math.IsInf(*in.Price, 0) || *in.Price < 0 || *in.Price > 1e12) {
		return httpx.FieldErrors{"price": "Price must be a number from 0"}
	}
	return nil
}

type Repository interface {
	List(ctx context.Context) ([]Price, error)
	Update(ctx context.Context, id int, price *float64, user string) (Price, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

const query = `SELECT l.labid, COALESCE(m.labmastername, ''), COALESCE(s.labsubmastername, ''), COALESCE(l.labname, ''),
	       l.price, l.isdeleted IS NOT TRUE
	FROM reflab l
	JOIN reflabsubmaster s ON s.labsubmasterid = l.labsubmasterid
	JOIN reflabmaster m ON m.labmasterid = s.labmasterid`

func scan(r pgx.CollectableRow) (Price, error) {
	var x Price
	err := r.Scan(&x.ID, &x.LabMaster, &x.LabSubMaster, &x.Nama, &x.Price, &x.IsActive)
	return x, err
}

func (p *PostgresRepository) List(ctx context.Context) ([]Price, error) {
	rows, err := p.pool.Query(ctx, query+` ORDER BY l.labid`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, scan)
	if list == nil {
		list = []Price{}
	}
	return list, err
}

// Update sets the price and records who changed it and when.
func (p *PostgresRepository) Update(ctx context.Context, id int, price *float64, user string) (Price, error) {
	var x Price
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		tag, err := tx.Exec(ctx,
			`UPDATE reflab SET price = $2, modifiedby = $3, modifieddate = now() AT TIME ZONE 'Asia/Jakarta' WHERE labid = $1`,
			id, price, user)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		rows, err := tx.Query(ctx, query+` WHERE l.labid = $1`, id)
		if err != nil {
			return err
		}
		x, err = pgx.CollectExactlyOneRow(rows, scan)
		if errors.Is(err, pgx.ErrNoRows) {
			return ErrNotFound // the row has no Sub Master or Lab Master, so it is not on this page
		}
		return err
	})
	return x, err
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
	mux.Handle("GET /api/lab-price", guard(h.List))
	mux.Handle("PUT /api/lab-price/{id}", guard(h.Update))
}

// user is what goes into modifiedby (varchar(30)).
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"price": list})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, "lab not found")
		return
	}
	var in Input
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if errs := in.Normalize(); len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return
	}
	x, err := h.repo.Update(r.Context(), id, in.Price, user(r))
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "lab not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, x)
}
