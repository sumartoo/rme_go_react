// Package doktermcu serves Master Data > Dokter: the MCU doctors (refdoctor),
// each of a kind (refdoctormaster), with their fee percentage. They are
// separate from the clinic doctors (package dokter, refdokter). The doctor
// code is the primary key, so it is set once and not edited. There is no
// delete, like the original screen.
package doktermcu

import (
	"context"
	"errors"
	"math"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 002_create_menus.sql).
const PagePath = "/master-data/dokter"

var (
	ErrNotFound      = errors.New("dokter not found")
	ErrDuplicate     = errors.New("dokter code already exists")
	ErrUnknownMaster = errors.New("unknown dokter master")
)

// Dokter is a refdoctor row with its Dokter Master's name.
type Dokter struct {
	Kode         string `json:"kode"`         // refdoctor.doctorcode
	DokterMaster string `json:"dokterMaster"` // refdoctormaster.doctormastername
	Input
}

// Input is the editable part of refdoctor. Fee is null when not set.
type Input struct {
	Nama           string   `json:"nama"`
	IDDokterMaster int      `json:"idDokterMaster"`
	Fee            *float64 `json:"fee"` // feepercentage
}

// Normalize trims the name and validates the fields against refdoctor.
func (in *Input) Normalize() httpx.FieldErrors {
	in.Nama = strings.TrimSpace(in.Nama)
	errs := httpx.FieldErrors{}
	switch {
	case in.Nama == "":
		errs["nama"] = "Dokter Name is required"
	case utf8.RuneCountInString(in.Nama) > 50:
		errs["nama"] = "Dokter Name must be at most 50 characters"
	}
	if in.IDDokterMaster <= 0 {
		errs["idDokterMaster"] = "Dokter Master is required"
	}
	if in.Fee != nil && (math.IsNaN(*in.Fee) || *in.Fee < 0 || *in.Fee > 100) {
		errs["fee"] = "Percentage Fee must be between 0 and 100"
	}
	if len(errs) == 0 {
		return nil
	}
	return errs
}

// NormalizeKode trims a new doctor's code and validates it (varchar(20)).
func NormalizeKode(kode string) (string, string) {
	kode = strings.TrimSpace(kode)
	switch {
	case kode == "":
		return kode, "Dokter Code is required"
	case utf8.RuneCountInString(kode) > 20:
		return kode, "Dokter Code must be at most 20 characters"
	}
	return kode, ""
}

// Master is a choice for the Dokter Master field.
type Master struct {
	ID   int    `json:"id"`
	Nama string `json:"nama"`
}

type Repository interface {
	List(ctx context.Context) ([]Dokter, error)
	Masters(ctx context.Context) ([]Master, error)
	Create(ctx context.Context, kode string, in Input) (Dokter, error)
	Update(ctx context.Context, kode string, in Input) (Dokter, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (p *PostgresRepository) List(ctx context.Context) ([]Dokter, error) {
	rows, err := p.pool.Query(ctx, `SELECT d.doctorcode, COALESCE(d.doctorname, ''), COALESCE(d.doctormaster, 0),
		       COALESCE(m.doctormastername, ''), d.feepercentage
		FROM refdoctor d LEFT JOIN refdoctormaster m ON m.doctormaster = d.doctormaster
		ORDER BY d.doctorcode`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Dokter, error) {
		var x Dokter
		err := r.Scan(&x.Kode, &x.Nama, &x.IDDokterMaster, &x.DokterMaster, &x.Fee)
		return x, err
	})
	if list == nil {
		list = []Dokter{}
	}
	return list, err
}

func (p *PostgresRepository) Masters(ctx context.Context) ([]Master, error) {
	rows, err := p.pool.Query(ctx, `SELECT doctormaster, COALESCE(doctormastername, '') FROM refdoctormaster ORDER BY doctormastername, doctormaster`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Master, error) {
		var x Master
		err := r.Scan(&x.ID, &x.Nama)
		return x, err
	})
	if list == nil {
		list = []Master{}
	}
	return list, err
}

// masterName returns the Dokter Master's name, or ErrUnknownMaster.
func masterName(ctx context.Context, tx pgx.Tx, id int) (string, error) {
	var nama string
	err := tx.QueryRow(ctx, `SELECT COALESCE(doctormastername, '') FROM refdoctormaster WHERE doctormaster = $1`, id).Scan(&nama)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrUnknownMaster
	}
	return nama, err
}

func (p *PostgresRepository) Create(ctx context.Context, kode string, in Input) (Dokter, error) {
	x := Dokter{Kode: kode, Input: in}
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		var err error
		if x.DokterMaster, err = masterName(ctx, tx, in.IDDokterMaster); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO refdoctor (doctorcode, doctorname, doctormaster, feepercentage) VALUES ($1, $2, $3, $4)`,
			kode, in.Nama, in.IDDokterMaster, in.Fee)
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.ConstraintName == "refdoctor_pkey" {
			return ErrDuplicate
		}
		return err
	})
	return x, err
}

func (p *PostgresRepository) Update(ctx context.Context, kode string, in Input) (Dokter, error) {
	x := Dokter{Kode: kode, Input: in}
	err := pgx.BeginFunc(ctx, p.pool, func(tx pgx.Tx) error {
		var err error
		if x.DokterMaster, err = masterName(ctx, tx, in.IDDokterMaster); err != nil {
			return err
		}
		tag, err := tx.Exec(ctx, `UPDATE refdoctor SET doctorname = $2, doctormaster = $3, feepercentage = $4 WHERE doctorcode = $1`,
			kode, in.Nama, in.IDDokterMaster, in.Fee)
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
	mux.Handle("GET /api/dokter-mcu", guard(h.List))
	mux.Handle("GET /api/dokter-mcu/options", guard(h.Options))
	mux.Handle("POST /api/dokter-mcu", guard(h.Create))
	mux.Handle("PUT /api/dokter-mcu/{kode}", guard(h.Update))
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"dokter": list})
}

// Options returns the Dokter Masters for the form.
func (h *Handler) Options(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.Masters(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"dokterMaster": list})
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "dokter not found")
	case errors.Is(err, ErrDuplicate):
		httpx.WriteJSON(w, http.StatusConflict, map[string]any{
			"error": "dokter code already exists", "errors": httpx.FieldErrors{"kode": "Dokter Code is already used"},
		})
	case errors.Is(err, ErrUnknownMaster):
		httpx.WriteValidation(w, httpx.FieldErrors{"idDokterMaster": "Choose a Dokter Master from the list"})
	default:
		httpx.InternalError(w, err)
	}
}

// Create takes the new doctor's code ("kode") along with the editable fields.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Kode string `json:"kode"`
		Input
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	errs := body.Input.Normalize()
	kode, kodeErr := NormalizeKode(body.Kode)
	if kodeErr != "" {
		if errs == nil {
			errs = httpx.FieldErrors{}
		}
		errs["kode"] = kodeErr
	}
	if len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return
	}
	x, err := h.repo.Create(r.Context(), kode, body.Input)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, x)
}

func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	kode := r.PathValue("kode")
	var in Input
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if errs := in.Normalize(); len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return
	}
	x, err := h.repo.Update(r.Context(), kode, in)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, x)
}
