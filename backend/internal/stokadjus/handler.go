package stokadjus

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
	"github.com/example/be_jwt_golang/backend/internal/pendaftaran"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 010_stok_farmasi_menu.sql).
const PagePath = "/stok-farmasi/stok-adjusment"

// MaxPageSize caps one request; exports ask for up to this many rows.
const MaxPageSize = 5000

// DefaultDays is the default period, ending today (like the legacy page).
const DefaultDays = 30

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
	mux.Handle("GET /api/stok-farmasi/stok-adjusment", guard(h.List))
	mux.Handle("GET /api/stok-farmasi/stok-adjusment/options", guard(h.Options))
	mux.Handle("GET /api/stok-farmasi/stok-adjusment/obat", guard(h.Obat))
	mux.Handle("POST /api/stok-farmasi/stok-adjusment", guard(h.Save))
}

func (h *Handler) branches(r *http.Request) ([]branchauth.Branch, error) {
	id, _ := auth.IdentityFrom(r.Context())
	list, err := h.repo.UserBranches(r.Context(), id.UserID)
	if list == nil {
		list = []branchauth.Branch{}
	}
	return list, err
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

// defaultFrom is the first day of the default period ending today.
func defaultFrom(today string) string {
	d, _ := time.Parse("2006-01-02", today)
	return d.AddDate(0, 0, -(DefaultDays - 1)).Format("2006-01-02")
}

// List returns one page of adjustments at the caller's branches, newest first.
//
// Query: from, to (YYYY-MM-DD of tanggal, default the last 30 days), cabang,
// page, size (1..5000), q, sort (id, tanggal, idObat, nama, status, ket, byk,
// cabang), dir, nama, ket, status (1 Masuk, 0 Keluar).
func (h *Handler) List(w http.ResponseWriter, r *http.Request) {
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	q := r.URL.Query()
	codes := branchauth.Codes(branches)
	if c := strings.ToUpper(strings.TrimSpace(q.Get("cabang"))); c != "" {
		if !branchauth.Has(branches, c) {
			httpx.WriteError(w, http.StatusForbidden, "you are not assigned to branch "+c)
			return
		}
		codes = []string{c}
	}
	today := pendaftaran.ClinicToday(h.now())
	from, to := q.Get("from"), q.Get("to")
	if to == "" {
		to = today
	}
	if from == "" {
		from = defaultFrom(today)
		if to < from {
			from = to
		}
	}
	if !validDate(from) || !validDate(to) || to < from {
		httpx.WriteValidation(w, httpx.FieldErrors{"from": "Use a valid date range (YYYY-MM-DD, from ≤ to)"})
		return
	}
	if len(codes) == 0 {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": []Row{}, "total": 0, "unfilteredTotal": 0, "from": from, "to": to})
		return
	}
	var status *int
	switch q.Get("status") {
	case "1":
		v := StatusMasuk
		status = &v
	case "0":
		v := StatusKeluar
		status = &v
	}
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") != "asc"
	if !Sortable(sort) {
		sort, desc = "id", true // newest first, like the legacy list
	}
	res, err := h.repo.List(r.Context(), ListParams{
		Branches: codes, From: from, To: to,
		Search: strings.TrimSpace(q.Get("q")), Nama: strings.TrimSpace(q.Get("nama")), Ket: strings.TrimSpace(q.Get("ket")), Status: status,
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

// Options returns the caller's branches, today's clinic date and the default period start.
func (h *Handler) Options(w http.ResponseWriter, r *http.Request) {
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	today := pendaftaran.ClinicToday(h.now())
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"branches": branches, "today": today, "defaultFrom": defaultFrom(today)})
}

// Obat lists the drugs stocked at one of the caller's branches for the Add
// form's lookup. Query: cabang (required).
func (h *Handler) Obat(w http.ResponseWriter, r *http.Request) {
	c := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("cabang")))
	if c == "" {
		httpx.WriteValidation(w, httpx.FieldErrors{"cabang": "Pilih Cabang"})
		return
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	if !branchauth.Has(branches, c) {
		httpx.WriteError(w, http.StatusForbidden, "you are not assigned to branch "+c)
		return
	}
	list, err := h.repo.Obat(r.Context(), c)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": list})
}

// Save records an adjustment at one of the caller's branches and moves that
// branch's stock; answers 201 with the new id and the stock after it.
func (h *Handler) Save(w http.ResponseWriter, r *http.Request) {
	var in SaveInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	in.Normalize()
	if errs := in.Validate(pendaftaran.ClinicToday(h.now())); len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	if !branchauth.Has(branches, in.Cabang) {
		httpx.WriteError(w, http.StatusForbidden, "you are not assigned to branch "+in.Cabang)
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	user := id.Email
	if at := strings.IndexByte(user, '@'); at > 0 {
		user = user[:at] // like the legacy user names ("mega_mgm")
	}
	if u := []rune(user); len(u) > 20 { // create_user is varchar(20)
		user = string(u[:20])
	}
	res, err := h.repo.Save(r.Context(), in, user)
	switch {
	case err == nil:
		httpx.WriteJSON(w, http.StatusCreated, res)
	case errors.Is(err, ErrNotStocked):
		httpx.WriteValidation(w, httpx.FieldErrors{"idObat": "Obat tidak ada di stok cabang " + in.Cabang})
	case errors.Is(err, ErrShortStock):
		httpx.WriteValidation(w, httpx.FieldErrors{"byk": "Stok tidak cukup untuk dikeluarkan"})
	default:
		httpx.InternalError(w, err)
	}
}
