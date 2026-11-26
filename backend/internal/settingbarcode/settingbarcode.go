// Package settingbarcode serves Setting > Barcode: the layout of the MCU
// barcode label (refsettingbarcode; lab holds one row). Rows are only edited,
// like the original screen: there is no add or delete.
package settingbarcode

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 002_create_menus.sql).
const PagePath = "/setting/barcode"

// maxValue bounds every size and position.
const maxValue = 10000

var ErrNotFound = errors.New("setting barcode not found")

// Setting is a refsettingbarcode row.
type Setting struct {
	ID int `json:"id"`
	Input
}

// Input is the editable part of refsettingbarcode. A null number is "not set".
type Input struct {
	Left          *float64 `json:"left"`
	Middle        *float64 `json:"middle"`
	Right         *float64 `json:"right"`
	Row1          *float64 `json:"row1"`
	Row2          *float64 `json:"row2"`
	Row3          *float64 `json:"row3"`
	Row4          *float64 `json:"row4"`
	Row5          *float64 `json:"row5"`
	Row6          *float64 `json:"row6"`
	BarcodeWidth  *float64 `json:"barcodeWidth"`
	BarcodeHeight *float64 `json:"barcodeHeight"`
	BarcodeLeft   *float64 `json:"barcodeLeft"`
	BarcodeTop    *float64 `json:"barcodeTop"`
	FontSize      *float64 `json:"fontSize"`
	IsActive      bool     `json:"isActive"`
}

// fields lists the numbers with their JSON names, for validation.
func (in *Input) fields() []struct {
	name string
	v    *float64
} {
	return []struct {
		name string
		v    *float64
	}{
		{"left", in.Left}, {"middle", in.Middle}, {"right", in.Right},
		{"row1", in.Row1}, {"row2", in.Row2}, {"row3", in.Row3}, {"row4", in.Row4}, {"row5", in.Row5}, {"row6", in.Row6},
		{"barcodeWidth", in.BarcodeWidth}, {"barcodeHeight", in.BarcodeHeight},
		{"barcodeLeft", in.BarcodeLeft}, {"barcodeTop", in.BarcodeTop}, {"fontSize", in.FontSize},
	}
}

// Normalize checks every number is from 0 to maxValue.
func (in *Input) Normalize() httpx.FieldErrors {
	errs := httpx.FieldErrors{}
	for _, f := range in.fields() {
		if f.v != nil && (math.IsNaN(*f.v) || *f.v < 0 || *f.v > maxValue) {
			errs[f.name] = "Enter a number from 0 to 10000"
		}
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

type Repository interface {
	List(ctx context.Context) ([]Setting, error)
	Update(ctx context.Context, id int, in Input) (Setting, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (p *PostgresRepository) List(ctx context.Context) ([]Setting, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, "left", middle, "right", row1, row2, row3, row4, row5, row6,
		       barcodewidth, barcodeheight, barcodeleft, barcodetop, fontsize, isactive = 1
		FROM refsettingbarcode ORDER BY id`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Setting, error) {
		var x Setting
		err := r.Scan(&x.ID, &x.Left, &x.Middle, &x.Right, &x.Row1, &x.Row2, &x.Row3, &x.Row4, &x.Row5, &x.Row6,
			&x.BarcodeWidth, &x.BarcodeHeight, &x.BarcodeLeft, &x.BarcodeTop, &x.FontSize, &x.IsActive)
		return x, err
	})
	if list == nil {
		list = []Setting{}
	}
	return list, err
}

func (p *PostgresRepository) Update(ctx context.Context, id int, in Input) (Setting, error) {
	active := 0
	if in.IsActive {
		active = 1
	}
	tag, err := p.pool.Exec(ctx,
		`UPDATE refsettingbarcode SET "left" = $2, middle = $3, "right" = $4, row1 = $5, row2 = $6, row3 = $7, row4 = $8,
		        row5 = $9, row6 = $10, barcodewidth = $11, barcodeheight = $12, barcodeleft = $13, barcodetop = $14,
		        fontsize = $15, isactive = $16
		 WHERE id = $1`,
		id, in.Left, in.Middle, in.Right, in.Row1, in.Row2, in.Row3, in.Row4, in.Row5, in.Row6,
		in.BarcodeWidth, in.BarcodeHeight, in.BarcodeLeft, in.BarcodeTop, in.FontSize, active)
	if err != nil {
		return Setting{}, err
	}
	if tag.RowsAffected() == 0 {
		return Setting{}, ErrNotFound
	}
	return Setting{ID: id, Input: in}, nil
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
	mux.Handle("GET /api/setting-barcode", guard(h.List))
	mux.Handle("PUT /api/setting-barcode/{id}", guard(h.Update))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"setting": list})
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, "setting barcode not found")
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
	x, err := h.repo.Update(r.Context(), id, in)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "setting barcode not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, x)
}
