package pasienmcu

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 002_create_menus.sql).
const PagePath = "/master-data/pasien"

// MaxPageSize caps one request; exports ask for up to this many rows.
const MaxPageSize = 10000

type Handler struct {
	repo Repository
	now  func() time.Time
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo, now: time.Now}
}

// Routes registers the endpoints for callers whose role can open PagePath.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	guard := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(PagePath)(f)) }
	mux.Handle("GET /api/pasien-mcu", guard(h.List))
	mux.Handle("GET /api/pasien-mcu/options", guard(h.Options))
	mux.Handle("POST /api/pasien-mcu", guard(h.Create))
	mux.Handle("PUT /api/pasien-mcu/{id}", guard(h.Update))
}

// today is the clinic's date, YYYY-MM-DD.
func (h *Handler) today() string {
	loc, err := time.LoadLocation(ClinicTimeZone)
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	return h.now().In(loc).Format("2006-01-02")
}

// user is what goes into createdby/modifyby (varchar(50)).
func user(r *http.Request) string {
	id, _ := auth.IdentityFrom(r.Context())
	u := []rune(id.Email)
	if len(u) > 50 {
		u = u[:50]
	}
	return string(u)
}

func intParam(r *http.Request, name string, def, lo, hi int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil {
		return def
	}
	return max(lo, min(v, hi))
}

// List returns one page of patients.
//
// Query: page (from 1), size (1..10000), q (search No RM, name, NIK, address or
// company), sort (rm, nama, gender, tglLahir, telp, hp, alamat, perusahaan,
// nik, email, departemen, posisi; default rm), dir (asc|desc, default asc).
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	if _, ok := sortColumns[sort]; !ok {
		sort = "rm"
	}
	res, err := h.repo.List(r.Context(), ListParams{
		Search: strings.TrimSpace(q.Get("q")),
		Sort:   sort,
		Desc:   q.Get("dir") == "desc",
		Limit:  size,
		Offset: (page - 1) * size,
	})
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// Options returns the companies for the Perusahaan field.
func (h *Handler) Options(w http.ResponseWriter, r *http.Request) {
	companies, err := h.repo.Companies(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"perusahaan": companies})
}

func (h *Handler) decode(w http.ResponseWriter, r *http.Request) (Input, bool) {
	var in Input
	if !httpx.DecodeJSON(w, r, &in) {
		return in, false
	}
	if errs := in.Normalize(h.today()); len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return in, false
	}
	return in, true
}

func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "pasien not found")
	case errors.Is(err, ErrUnknownCompany):
		httpx.WriteValidation(w, httpx.FieldErrors{"kodePerusahaan": "Choose a company from the list"})
	case errors.Is(err, ErrRMExhausted):
		httpx.WriteError(w, http.StatusConflict, "no free No RM left today")
	default:
		httpx.InternalError(w, err)
	}
}

func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	in, ok := h.decode(w, r)
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
		httpx.WriteError(w, http.StatusNotFound, "pasien not found")
		return
	}
	in, ok := h.decode(w, r)
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
