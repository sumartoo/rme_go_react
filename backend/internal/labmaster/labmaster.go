// Package labmaster serves Master Data > Lab Master: the groups of lab
// examinations (reflabmaster: code and name). There is no delete, like the
// original screen; soft-deleted rows (isdeleted) are hidden.
package labmaster

import (
	"context"
	"errors"
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
const PagePath = "/master-data/lab-master"

var ErrNotFound = errors.New("lab master not found")

// LabMaster is a reflabmaster row.
type LabMaster struct {
	ID   int    `json:"id"`
	Kode string `json:"kode"`
	Nama string `json:"nama"`
}

// Input is the editable part of reflabmaster.
type Input struct {
	Kode string `json:"kode"`
	Nama string `json:"nama"`
}

// Normalize trims both fields and validates them against the column sizes
// (labmastercode varchar(20), labmastername varchar(50)).
func (in *Input) Normalize() httpx.FieldErrors {
	in.Kode = strings.TrimSpace(in.Kode)
	in.Nama = strings.TrimSpace(in.Nama)
	errs := httpx.FieldErrors{}
	switch {
	case in.Kode == "":
		errs["kode"] = "Lab Master Code is required"
	case utf8.RuneCountInString(in.Kode) > 20:
		errs["kode"] = "Lab Master Code must be at most 20 characters"
	}
	switch {
	case in.Nama == "":
		errs["nama"] = "Lab Master Name is required"
	case utf8.RuneCountInString(in.Nama) > 50:
		errs["nama"] = "Lab Master Name must be at most 50 characters"
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

type Repository interface {
	List(ctx context.Context) ([]LabMaster, error)
	Create(ctx context.Context, in Input, user string) (LabMaster, error)
	Update(ctx context.Context, id int, in Input, user string) (LabMaster, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (p *PostgresRepository) List(ctx context.Context) ([]LabMaster, error) {
	rows, err := p.pool.Query(ctx, `SELECT labmasterid, COALESCE(labmastercode, ''), COALESCE(labmastername, '')
		FROM reflabmaster WHERE isdeleted IS NOT TRUE ORDER BY labmastercode, labmasterid`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (LabMaster, error) {
		var x LabMaster
		err := r.Scan(&x.ID, &x.Kode, &x.Nama)
		return x, err
	})
	if list == nil {
		list = []LabMaster{}
	}
	return list, err
}

func (p *PostgresRepository) Create(ctx context.Context, in Input, user string) (LabMaster, error) {
	x := LabMaster{Kode: in.Kode, Nama: in.Nama}
	err := p.pool.QueryRow(ctx,
		`INSERT INTO reflabmaster (labmastercode, labmastername, isdeleted, createby, createddate)
		 VALUES ($1, $2, false, $3, now() AT TIME ZONE 'Asia/Jakarta') RETURNING labmasterid`,
		in.Kode, in.Nama, user).Scan(&x.ID)
	return x, err
}

func (p *PostgresRepository) Update(ctx context.Context, id int, in Input, user string) (LabMaster, error) {
	tag, err := p.pool.Exec(ctx,
		`UPDATE reflabmaster SET labmastercode = $2, labmastername = $3, modifiedby = $4, modifieddate = now() AT TIME ZONE 'Asia/Jakarta'
		 WHERE labmasterid = $1 AND isdeleted IS NOT TRUE`,
		id, in.Kode, in.Nama, user)
	if err != nil {
		return LabMaster{}, err
	}
	if tag.RowsAffected() == 0 {
		return LabMaster{}, ErrNotFound
	}
	return LabMaster{ID: id, Kode: in.Kode, Nama: in.Nama}, nil
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
	mux.Handle("GET /api/lab-master", guard(h.List))
	mux.Handle("POST /api/lab-master", guard(h.Create))
	mux.Handle("PUT /api/lab-master/{id}", guard(h.Update))
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"labMaster": list})
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
		httpx.WriteError(w, http.StatusNotFound, "lab master not found")
		return
	}
	in, ok := decode(w, r)
	if !ok {
		return
	}
	x, err := h.repo.Update(r.Context(), id, in, user(r))
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "lab master not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, x)
}
