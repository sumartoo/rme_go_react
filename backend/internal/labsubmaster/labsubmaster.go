// Package labsubmaster serves Master Data > Lab Sub Master: the subgroups of
// lab examinations (reflabsubmaster), each under a Lab Master (reflabmaster).
// There is no delete, like the original screen; soft-deleted rows (isdeleted)
// are hidden.
package labsubmaster

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
const PagePath = "/master-data/lab-sub-master"

var (
	ErrNotFound      = errors.New("lab sub master not found")
	ErrUnknownMaster = errors.New("unknown lab master")
)

// SubMaster is a reflabsubmaster row with its Lab Master's name.
type SubMaster struct {
	ID        int    `json:"id"`
	LabMaster string `json:"labMaster"` // reflabmaster.labmastername
	Input
}

// Input is the editable part of reflabsubmaster.
type Input struct {
	IDLabMaster int    `json:"idLabMaster"`
	Kode        string `json:"kode"`
	Nama        string `json:"nama"`
}

// Normalize trims the text and validates it against the column sizes
// (labsubmastercode varchar(20), labsubmastername varchar(50)).
func (in *Input) Normalize() httpx.FieldErrors {
	in.Kode = strings.TrimSpace(in.Kode)
	in.Nama = strings.TrimSpace(in.Nama)
	errs := httpx.FieldErrors{}
	if in.IDLabMaster <= 0 {
		errs["idLabMaster"] = "Lab Master is required"
	}
	switch {
	case in.Kode == "":
		errs["kode"] = "Sub Master Code is required"
	case utf8.RuneCountInString(in.Kode) > 20:
		errs["kode"] = "Sub Master Code must be at most 20 characters"
	}
	switch {
	case in.Nama == "":
		errs["nama"] = "Sub Master Name is required"
	case utf8.RuneCountInString(in.Nama) > 50:
		errs["nama"] = "Sub Master Name must be at most 50 characters"
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// LabMaster is a choice for the Lab Master field.
type LabMaster struct {
	ID   int    `json:"id"`
	Nama string `json:"nama"`
}

type Repository interface {
	List(ctx context.Context) ([]SubMaster, error)
	LabMasters(ctx context.Context) ([]LabMaster, error)
	Create(ctx context.Context, in Input, user string) (SubMaster, error)
	Update(ctx context.Context, id int, in Input, user string) (SubMaster, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

// List orders by Lab Master name, then by id (the order rows were entered),
// like the original screen.
func (p *PostgresRepository) List(ctx context.Context) ([]SubMaster, error) {
	rows, err := p.pool.Query(ctx, `SELECT s.labsubmasterid, COALESCE(s.labmasterid, 0), COALESCE(m.labmastername, ''),
		       COALESCE(s.labsubmastercode, ''), COALESCE(s.labsubmastername, '')
		FROM reflabsubmaster s LEFT JOIN reflabmaster m ON m.labmasterid = s.labmasterid
		WHERE s.isdeleted IS NOT TRUE
		ORDER BY m.labmastername NULLS LAST, s.labsubmasterid`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (SubMaster, error) {
		var x SubMaster
		err := r.Scan(&x.ID, &x.IDLabMaster, &x.LabMaster, &x.Kode, &x.Nama)
		return x, err
	})
	if list == nil {
		list = []SubMaster{}
	}
	return list, err
}

func (p *PostgresRepository) LabMasters(ctx context.Context) ([]LabMaster, error) {
	rows, err := p.pool.Query(ctx, `SELECT labmasterid, COALESCE(labmastername, '') FROM reflabmaster
		WHERE isdeleted IS NOT TRUE ORDER BY labmastername, labmasterid`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (LabMaster, error) {
		var x LabMaster
		err := r.Scan(&x.ID, &x.Nama)
		return x, err
	})
	if list == nil {
		list = []LabMaster{}
	}
	return list, err
}

// masterName returns the Lab Master's name, or ErrUnknownMaster.
func masterName(ctx context.Context, tx pgx.Tx, id int) (string, error) {
	var nama string
	err := tx.QueryRow(ctx, `SELECT COALESCE(labmastername, '') FROM reflabmaster WHERE labmasterid = $1 AND isdeleted IS NOT TRUE`, id).Scan(&nama)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUnknownMaster
	}
	return nama, err
}

func (p *PostgresRepository) Create(ctx context.Context, in Input, user string) (SubMaster, error) {
	x := SubMaster{Input: in}
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		var err error
		if x.LabMaster, err = masterName(ctx, tx, in.IDLabMaster); err != nil {
			return err
		}
		return tx.QueryRow(ctx,
			`INSERT INTO reflabsubmaster (labmasterid, labsubmastercode, labsubmastername, isdeleted, createby, createddate)
			 VALUES ($1, $2, $3, false, $4, now() AT TIME ZONE 'Asia/Jakarta') RETURNING labsubmasterid`,
			in.IDLabMaster, in.Kode, in.Nama, user).Scan(&x.ID)
	})
	return x, err
}

func (p *PostgresRepository) Update(ctx context.Context, id int, in Input, user string) (SubMaster, error) {
	x := SubMaster{ID: id, Input: in}
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		var err error
		if x.LabMaster, err = masterName(ctx, tx, in.IDLabMaster); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx,
			`UPDATE reflabsubmaster SET labmasterid = $2, labsubmastercode = $3, labsubmastername = $4,
			        modifiedby = $5, modifieddate = now() AT TIME ZONE 'Asia/Jakarta'
			 WHERE labsubmasterid = $1 AND isdeleted IS NOT TRUE`,
			id, in.IDLabMaster, in.Kode, in.Nama, user)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 0 {
			return ErrNotFound
		}
		return nil
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
	mux.Handle("GET /api/lab-sub-master", guard(h.List))
	mux.Handle("GET /api/lab-sub-master/options", guard(h.Options))
	mux.Handle("POST /api/lab-sub-master", guard(h.Create))
	mux.Handle("PUT /api/lab-sub-master/{id}", guard(h.Update))
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"labSubMaster": list})
}

// Options returns the Lab Masters for the form's Lab Master field.
func (h *Handler) Options(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.LabMasters(r.Context())
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

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "lab sub master not found")
	case errors.Is(err, ErrUnknownMaster):
		httpx.WriteValidation(w, httpx.FieldErrors{"idLabMaster": "Choose a Lab Master from the list"})
	default:
		httpx.InternalError(w, err)
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	in, ok := decode(w, r)
	if !ok {
		return
	}
	x, err := h.repo.Create(r.Context(), in, user(r))
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, x)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, "lab sub master not found")
		return
	}
	in, ok := decode(w, r)
	if !ok {
		return
	}
	x, err := h.repo.Update(r.Context(), id, in, user(r))
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, x)
}
