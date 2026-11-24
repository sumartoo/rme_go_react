package paketprice

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 002_create_menus.sql).
const PagePath = "/setting/paket-price"

type Handler struct {
	repo Repository
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo}
}

// Routes registers the endpoints for callers whose role can open PagePath.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	guard := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(PagePath)(f)) }
	mux.Handle("GET /api/paket-price", guard(h.List))
	mux.Handle("GET /api/paket-price/options", guard(h.Options))
	mux.Handle("GET /api/paket-price/{id}", guard(h.Get))
	mux.Handle("POST /api/paket-price", guard(h.Create))
	mux.Handle("PUT /api/paket-price/{id}", guard(h.Update))
	mux.Handle("DELETE /api/paket-price/{id}", guard(h.Delete))
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

func pathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, "paket not found")
		return 0, false
	}
	return id, true
}

func writeRepoError(w http.ResponseWriter, err error) {
	if errs, ok := errorFields(err); ok {
		httpx.WriteValidation(w, errs)
		return
	}
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "paket not found")
		return
	}
	httpx.InternalError(w, err)
}

func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.List(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"paket": list})
}

func (h *Handler) Options(w http.ResponseWriter, r *http.Request) {
	o, err := h.repo.Options(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, o)
}

func (h *Handler) Get(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	d, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, d)
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

// Create returns the new package with its examinations.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	in, ok := decode(w, r)
	if !ok {
		return
	}
	id, err := h.repo.Create(r.Context(), in, user(r))
	if err != nil {
		writeRepoError(w, err)
		return
	}
	d, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, d)
}

// Update replaces the package and its examinations and returns them.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	in, ok := decode(w, r)
	if !ok {
		return
	}
	if err := h.repo.Update(r.Context(), id, in, user(r)); err != nil {
		writeRepoError(w, err)
		return
	}
	d, err := h.repo.Get(r.Context(), id)
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, d)
}

// Delete hides the package (isdeleted); transactions keep referring to it.
func (h *Handler) Delete(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	if err := h.repo.Delete(r.Context(), id, user(r)); err != nil {
		writeRepoError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
