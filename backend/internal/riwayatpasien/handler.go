package riwayatpasien

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
	"github.com/example/be_jwt_golang/backend/internal/pendaftaran"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 007_front_office_klinik_menu.sql).
const PagePath = "/front-office-klinik/riwayat-pasien"

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// Routes registers the endpoints for callers whose role can open PagePath.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	guard := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(PagePath)(f)) }
	mux.Handle("GET /api/riwayat-pasien/pasien", guard(h.Search))
	mux.Handle("GET /api/riwayat-pasien/{id}", guard(h.Get))
}

func (h *Handler) codes(w http.ResponseWriter, r *http.Request) ([]string, bool) {
	id, _ := auth.IdentityFrom(r.Context())
	list, err := h.repo.UserBranches(r.Context(), id.UserID)
	if err != nil {
		httpx.InternalError(w, err)
		return nil, false
	}
	return branchauth.Codes(list), true
}

func intParam(r *http.Request, name string, def, lo, hi int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil {
		return def
	}
	return min(max(v, lo), hi)
}

// Search finds patients of the caller's branches by name, No RM, NIK or address.
//
// Query: q (at least 2 characters), page, size (1..50, default 10).
func (h *Handler) Search(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	if len([]rune(q)) < 2 {
		httpx.WriteValidation(w, httpx.FieldErrors{"q": "Ketik minimal 2 karakter"})
		return
	}
	codes, ok := h.codes(w, r)
	if !ok {
		return
	}
	if len(codes) == 0 {
		httpx.WriteJSON(w, http.StatusOK, pendaftaran.PasienPage{Rows: []pendaftaran.PasienHit{}})
		return
	}
	size := intParam(r, "size", 10, 1, 50)
	page := intParam(r, "page", 1, 1, 1_000_000)
	res, err := h.repo.SearchPasien(r.Context(), codes, q, size, (page-1)*size)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// Get returns the patient's visits at the caller's branches and their
// summary. A patient of other branches only answers 404.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, "pasien not found")
		return
	}
	codes, ok := h.codes(w, r)
	if !ok {
		return
	}
	if len(codes) == 0 {
		httpx.WriteError(w, http.StatusNotFound, "pasien not found")
		return
	}
	res, err := h.repo.Riwayat(r.Context(), codes, id)
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "pasien not found")
	case err != nil:
		httpx.InternalError(w, err)
	default:
		httpx.WriteJSON(w, http.StatusOK, map[string]any{
			"pasien": res.Pasien, "ringkasan": Summarize(res.Kunjungan), "kunjungan": res.Kunjungan, "maxKunjungan": MaxKunjungan,
		})
	}
}
