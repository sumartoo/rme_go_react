// Package lab serves Master Data > Lab: the lab examinations (reflab), each
// under a Lab Sub Master (reflabsubmaster) and so a Lab Master (reflabmaster).
// isdeleted is the Status (true = Non Active); inactive rows stay listed and
// can be activated again. There is no delete, like the original screen.
package lab

import (
	"context"
	"errors"
	"fmt"
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
const PagePath = "/master-data/lab"

const timeZone = "Asia/Jakarta"

var (
	ErrNotFound         = errors.New("lab not found")
	ErrUnknownSubMaster = errors.New("unknown lab sub master")
)

// Lab is a reflab row with its Sub Master's and Lab Master's names.
type Lab struct {
	ID           int    `json:"id"` // reflab.labid, shown as "Lab Code"
	LabMaster    string `json:"labMaster"`
	LabSubMaster string `json:"labSubMaster"`
	Input
}

// Input is the editable part of reflab. Price and Fee are null when not set.
type Input struct {
	IDLabSubMaster int      `json:"idLabSubMaster"`
	Kode           string   `json:"kode"` // reflab.labcode
	Nama           string   `json:"nama"` // reflab.labname
	Price          *float64 `json:"price"`
	Fee            *float64 `json:"fee"`
	IsActive       bool     `json:"isActive"`
}

// Normalize trims the text and validates it against the reflab columns.
func (in *Input) Normalize() httpx.FieldErrors {
	in.Kode = strings.TrimSpace(in.Kode)
	in.Nama = strings.TrimSpace(in.Nama)
	errs := httpx.FieldErrors{}
	if in.IDLabSubMaster <= 0 {
		errs["idLabSubMaster"] = "Sub Master is required"
	}
	switch {
	case in.Nama == "":
		errs["nama"] = "Lab is required"
	case utf8.RuneCountInString(in.Nama) > 50:
		errs["nama"] = "Lab must be at most 50 characters"
	}
	if utf8.RuneCountInString(in.Kode) > 50 {
		errs["kode"] = "Kode must be at most 50 characters"
	}
	amount := func(field, label string, v *float64, max float64) {
		if v != nil && (math.IsNaN(*v) || *v < 0 || *v > max) {
			errs[field] = fmt.Sprintf("%s must be between 0 and %s", label, strconv.FormatFloat(max, 'f', -1, 64))
		}
	}
	amount("price", "Price", in.Price, 1e12)
	amount("fee", "Fee", in.Fee, 100)
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// SubMaster is a choice for the Sub Master field.
type SubMaster struct {
	ID        int    `json:"id"`
	Nama      string `json:"nama"`
	LabMaster string `json:"labMaster"`
}

type Repository interface {
	List(ctx context.Context) ([]Lab, error)
	SubMasters(ctx context.Context) ([]SubMaster, error)
	Create(ctx context.Context, in Input, user string) (Lab, error)
	Update(ctx context.Context, id int, in Input, user string) (Lab, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (p *PostgresRepository) List(ctx context.Context) ([]Lab, error) {
	rows, err := p.pool.Query(ctx, `SELECT l.labid, COALESCE(l.labsubmasterid, 0), COALESCE(m.labmastername, ''),
		       COALESCE(s.labsubmastername, ''), COALESCE(l.labcode, ''), COALESCE(l.labname, ''), l.price, l.fee,
		       l.isdeleted IS NOT TRUE
		FROM reflab l
		LEFT JOIN reflabsubmaster s ON s.labsubmasterid = l.labsubmasterid
		LEFT JOIN reflabmaster m ON m.labmasterid = s.labmasterid
		ORDER BY l.labid`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Lab, error) {
		var x Lab
		err := r.Scan(&x.ID, &x.IDLabSubMaster, &x.LabMaster, &x.LabSubMaster, &x.Kode, &x.Nama, &x.Price, &x.Fee, &x.IsActive)
		return x, err
	})
	if list == nil {
		list = []Lab{}
	}
	return list, err
}

func (p *PostgresRepository) SubMasters(ctx context.Context) ([]SubMaster, error) {
	rows, err := p.pool.Query(ctx, `SELECT s.labsubmasterid, COALESCE(s.labsubmastername, ''), COALESCE(m.labmastername, '')
		FROM reflabsubmaster s LEFT JOIN reflabmaster m ON m.labmasterid = s.labmasterid
		WHERE s.isdeleted IS NOT TRUE
		ORDER BY m.labmastername NULLS LAST, s.labsubmastername, s.labsubmasterid`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (SubMaster, error) {
		var x SubMaster
		err := r.Scan(&x.ID, &x.Nama, &x.LabMaster)
		return x, err
	})
	if list == nil {
		list = []SubMaster{}
	}
	return list, err
}

// names returns the Sub Master's and its Lab Master's names, or ErrUnknownSubMaster.
func names(ctx context.Context, tx pgx.Tx, id int) (master, sub string, err error) {
	err = tx.QueryRow(ctx, `SELECT COALESCE(m.labmastername, ''), COALESCE(s.labsubmastername, '')
		FROM reflabsubmaster s LEFT JOIN reflabmaster m ON m.labmasterid = s.labmasterid
		WHERE s.labsubmasterid = $1 AND s.isdeleted IS NOT TRUE`, id).Scan(&master, &sub)
	if errors.Is(err, pgx.ErrNoRows) {
		err = ErrUnknownSubMaster
	}
	return master, sub, err
}

// kode is what goes into labcode: the name when no code is given, as in most of lab.
func kode(in Input) string {
	if in.Kode == "" {
		return in.Nama
	}
	return in.Kode
}

func (p *PostgresRepository) Create(ctx context.Context, in Input, user string) (Lab, error) {
	in.Kode = kode(in)
	x := Lab{Input: in}
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		var err error
		if x.LabMaster, x.LabSubMaster, err = names(ctx, tx, in.IDLabSubMaster); err != nil {
			return err
		}
		return tx.QueryRow(ctx,
			`INSERT INTO reflab (labsubmasterid, labcode, labname, price, fee, isdeleted, createby, createddate, deletedby, deleteddate)
			 VALUES ($1, $2, $3, $4, $5, NOT $6, $7, now() AT TIME ZONE '`+timeZone+`',
			         CASE WHEN $6 THEN NULL ELSE $7 END, CASE WHEN $6 THEN NULL ELSE now() AT TIME ZONE '`+timeZone+`' END)
			 RETURNING labid`,
			in.IDLabSubMaster, in.Kode, in.Nama, in.Price, in.Fee, in.IsActive, user).Scan(&x.ID)
	})
	return x, err
}

// Update records who deactivated a row and when (deletedby/deleteddate), and
// clears them when it is activated again.
func (p *PostgresRepository) Update(ctx context.Context, id int, in Input, user string) (Lab, error) {
	in.Kode = kode(in)
	x := Lab{ID: id, Input: in}
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		var err error
		if x.LabMaster, x.LabSubMaster, err = names(ctx, tx, in.IDLabSubMaster); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx,
			`UPDATE reflab SET labsubmasterid = $2, labcode = $3, labname = $4, price = $5, fee = $6,
			        modifiedby = $8, modifieddate = now() AT TIME ZONE '`+timeZone+`',
			        deletedby   = CASE WHEN $7 THEN NULL WHEN isdeleted IS TRUE THEN deletedby ELSE $8 END,
			        deleteddate = CASE WHEN $7 THEN NULL WHEN isdeleted IS TRUE THEN deleteddate ELSE now() AT TIME ZONE '`+timeZone+`' END,
			        isdeleted = NOT $7
			 WHERE labid = $1`,
			id, in.IDLabSubMaster, in.Kode, in.Nama, in.Price, in.Fee, in.IsActive, user)
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
	mux.Handle("GET /api/lab", guard(h.List))
	mux.Handle("GET /api/lab/options", guard(h.Options))
	mux.Handle("POST /api/lab", guard(h.Create))
	mux.Handle("PUT /api/lab/{id}", guard(h.Update))
}

// user is what goes into createby/modifiedby/deletedby (varchar(30)).
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
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"lab": list})
}

// Options returns the Sub Masters (with their Lab Master) for the form.
func (h *Handler) Options(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.SubMasters(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"labSubMaster": list})
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
		httpx.WriteError(w, http.StatusNotFound, "lab not found")
	case errors.Is(err, ErrUnknownSubMaster):
		httpx.WriteValidation(w, httpx.FieldErrors{"idLabSubMaster": "Choose a Sub Master from the list"})
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
		httpx.WriteError(w, http.StatusNotFound, "lab not found")
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
