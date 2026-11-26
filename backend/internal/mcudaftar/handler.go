package mcudaftar

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 004_mcu_menu.sql).
const PagePath = "/mcu/pendaftaran"

// MaxPageSize caps one request; exports ask for up to this many rows.
const MaxPageSize = 5000

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
	mux.Handle("GET /api/mcu-pendaftaran", guard(h.List))
	mux.Handle("GET /api/mcu-pendaftaran/options", guard(h.Options))
	mux.Handle("GET /api/mcu-pendaftaran/pasien", guard(h.Pasien))
	mux.Handle("GET /api/mcu-pendaftaran/lab", guard(h.Lab))
	mux.Handle("GET /api/mcu-pendaftaran/paket", guard(h.Paket))
	mux.Handle("GET /api/mcu-pendaftaran/non-lab", guard(h.NonLab))
	mux.Handle("GET /api/mcu-pendaftaran/dokter", guard(h.Dokter))
	mux.Handle("GET /api/mcu-pendaftaran/{id}", guard(h.Get))
	mux.Handle("POST /api/mcu-pendaftaran", guard(h.Create))
	mux.Handle("PUT /api/mcu-pendaftaran/{id}", guard(h.Update))
	mux.Handle("POST /api/mcu-pendaftaran/{id}/cancel", guard(h.Cancel))
}

// today is the clinic's date, YYYY-MM-DD.
func (h *Handler) today() string {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		loc = time.FixedZone("WIB", 7*3600)
	}
	return h.now().In(loc).Format("2006-01-02")
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

func intParam(r *http.Request, name string, def, lo, hi int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil {
		return def
	}
	return min(max(v, lo), hi)
}

func validDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// List pages the registrations, newest MCU date first.
//
// Query: from, to (YYYY-MM-DD on the MCU date, default the last 30 days),
// status (New, Close, Cancel), q, sort, dir, page, size (1..5000).
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	today := h.today()
	from, to := q.Get("from"), q.Get("to")
	if to == "" {
		to = today
	}
	if from == "" {
		t, _ := time.Parse("2006-01-02", today)
		from = t.AddDate(0, 0, -29).Format("2006-01-02")
	}
	if !validDate(from) || !validDate(to) || to < from {
		httpx.WriteValidation(w, httpx.FieldErrors{"from": "Use a valid date range (YYYY-MM-DD, from ≤ to)"})
		return
	}
	status := strings.TrimSpace(q.Get("status"))
	if status != "" && status != StatusNew && status != StatusClose && status != StatusCancel {
		httpx.WriteValidation(w, httpx.FieldErrors{"status": "Use New, Close or Cancel"})
		return
	}
	sort := q.Get("sort")
	desc := q.Get("dir") != "asc"
	if _, ok := sortColumns[sort]; !ok {
		sort, desc = "mcuDate", true
	}
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	res, err := h.repo.List(r.Context(), ListParams{
		From: from, To: to, Status: status, Search: strings.TrimSpace(q.Get("q")),
		Sort: sort, Desc: desc, Limit: size, Offset: (page - 1) * size,
	})
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"rows": res.Rows, "total": res.Total, "unfilteredTotal": res.UnfilteredTotal, "from": from, "to": to,
	})
}

// Options returns the MCU Kategori and MCU Info choices and today's clinic date.
func (h *Handler) Options(w http.ResponseWriter, r *http.Request) {
	kategori, err := h.repo.Kategori(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	info, err := h.repo.Info(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"kategori": kategori, "info": info, "status": []string{StatusNew, StatusClose, StatusCancel}, "today": h.today(),
	})
}

// Pasien pages the MCU patients for the form's lookup. Query: q, page, size (1..100).
func (h *Handler) Pasien(w http.ResponseWriter, r *http.Request) {
	size := intParam(r, "size", 10, 1, 100)
	page := intParam(r, "page", 1, 1, 1_000_000)
	rows, total, err := h.repo.Pasien(r.Context(), strings.TrimSpace(r.URL.Query().Get("q")), size, (page-1)*size)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": rows, "total": total})
}

func writeRows[T any](w http.ResponseWriter, rows []T, err error) {
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": rows})
}

func (h *Handler) Lab(w http.ResponseWriter, r *http.Request) {
	rows, err := h.repo.Lab(r.Context())
	writeRows(w, rows, err)
}

func (h *Handler) Paket(w http.ResponseWriter, r *http.Request) {
	rows, err := h.repo.Paket(r.Context())
	writeRows(w, rows, err)
}

func (h *Handler) NonLab(w http.ResponseWriter, r *http.Request) {
	rows, err := h.repo.NonLab(r.Context())
	writeRows(w, rows, err)
}

func (h *Handler) Dokter(w http.ResponseWriter, r *http.Request) {
	rows, err := h.repo.Dokter(r.Context())
	writeRows(w, rows, err)
}

func pathID(w http.ResponseWriter, r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, ErrNotFound.Error())
		return 0, false
	}
	return id, true
}

// writeRepoError maps the repository's errors to responses.
func writeRepoError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, err.Error())
	case errors.Is(err, ErrNotNew):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrUnknownPasien):
		httpx.WriteValidation(w, httpx.FieldErrors{"pasienIds": "Unknown patient"})
	case errors.Is(err, ErrUnknownLab):
		httpx.WriteValidation(w, httpx.FieldErrors{"labIds": "Unknown examination"})
	case errors.Is(err, ErrUnknownPaket):
		httpx.WriteValidation(w, httpx.FieldErrors{"paketIds": "Unknown package"})
	case errors.Is(err, ErrUnknownNonLab):
		httpx.WriteValidation(w, httpx.FieldErrors{"nonLabIds": "Unknown non lab service"})
	case errors.Is(err, ErrUnknownDokter):
		httpx.WriteValidation(w, httpx.FieldErrors{"dokter": "Unknown doctor"})
	case errors.Is(err, ErrUnknownChoice):
		httpx.WriteValidation(w, httpx.FieldErrors{"kategoriId": "Unknown kategori or info"})
	default:
		httpx.InternalError(w, err)
	}
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

// Create registers every patient of the form: 201 {"ids": [...]}.
func (h *Handler) Create(w http.ResponseWriter, r *http.Request) {
	var in Input
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if errs := in.Normalize(true); len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return
	}
	ids, err := h.repo.Create(r.Context(), in, user(r))
	if err != nil {
		writeRepoError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusCreated, map[string]any{"ids": ids})
}

// Update replaces a New registration's date, choices and lines; pasienIds is ignored.
func (h *Handler) Update(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var in Input
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	in.PasienIDs = nil
	if errs := in.Normalize(false); len(errs) > 0 {
		httpx.WriteValidation(w, errs)
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

// Cancel cancels a New registration. Body: {"reason": "..."} (required, ≤ 300 characters).
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	id, ok := pathID(w, r)
	if !ok {
		return
	}
	var body struct {
		Reason string `json:"reason"`
	}
	if !httpx.DecodeJSON(w, r, &body) {
		return
	}
	reason := strings.TrimSpace(body.Reason)
	if reason == "" || utf8.RuneCountInString(reason) > 300 {
		httpx.WriteValidation(w, httpx.FieldErrors{"reason": "Give a reason of at most 300 characters"})
		return
	}
	if err := h.repo.Cancel(r.Context(), id, reason, user(r)); err != nil {
		writeRepoError(w, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
