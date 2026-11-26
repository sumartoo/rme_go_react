package mcuhasil

import (
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
	"github.com/example/be_jwt_golang/backend/internal/pendaftaran"
)

// DefaultDays is the default period of the registration list, ending today.
const DefaultDays = 30

type Handler struct {
	repo Repository
	now  func() time.Time
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo, now: time.Now}
}

// Routes registers each form's endpoints behind its own menu item.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	for _, f := range Forms {
		f := f
		guard := func(fn func(http.ResponseWriter, *http.Request, Form)) http.Handler {
			return requireAuth(requireMenu(f.PagePath)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fn(w, r, f) })))
		}
		base := "/api/mcu/form/" + f.Slug
		mux.Handle("GET "+base+"/registrasi", guard(h.List))
		mux.Handle("GET "+base+"/katalog", guard(h.Katalog))
		mux.Handle("GET "+base+"/{id}", guard(h.Get))
		mux.Handle("PUT "+base+"/{id}", guard(h.Save))
	}
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

// List returns the MCU registrations of a period with this form's status.
//
// Query: from, to (YYYY-MM-DD, default the last 30 days), q (name, RM, NIK,
// company or registration id), cancel (1 to include cancelled), page, size (1..100).
func (h *Handler) List(w http.ResponseWriter, r *http.Request, f Form) {
	q := r.URL.Query()
	today := pendaftaran.ClinicToday(h.now())
	from, to := q.Get("from"), q.Get("to")
	if to == "" {
		to = today
	}
	if from == "" {
		d, _ := time.Parse("2006-01-02", to)
		from = d.AddDate(0, 0, -(DefaultDays - 1)).Format("2006-01-02")
	}
	if !validDate(from) || !validDate(to) || to < from {
		httpx.WriteValidation(w, httpx.FieldErrors{"from": "Use a valid date range (YYYY-MM-DD, from ≤ to)"})
		return
	}
	size := intParam(r, "size", 10, 1, 100)
	page := intParam(r, "page", 1, 1, 1_000_000)
	res, err := h.repo.List(r.Context(), ListParams{
		Form: f.Slug, Search: strings.TrimSpace(q.Get("q")), From: from, To: to, Cancel: q.Get("cancel") == "1",
		Limit: size, Offset: (page - 1) * size,
	})
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": res.Rows, "total": res.Total, "from": from, "to": to, "today": today})
}

// Katalog returns the form's catalogue sections (empty for frontend-laid forms).
func (h *Handler) Katalog(w http.ResponseWriter, r *http.Request, f Form) {
	sections, err := h.repo.Katalog(r.Context(), f.Groups)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"form": f.Slug, "title": f.Title, "sections": sections})
}

func pathID(r *http.Request) (int, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	return id, err == nil && id > 0
}

// Get returns the registration and the form's saved result; the lab form also
// gets the examinations ordered with the registration.
func (h *Handler) Get(w http.ResponseWriter, r *http.Request, f Form) {
	id, ok := pathID(r)
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "registrasi MCU not found")
		return
	}
	reg, err := h.repo.Registrasi(r.Context(), id)
	if errors.Is(err, ErrNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "registrasi MCU not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	hasil, err := h.repo.Hasil(r.Context(), id, f.Slug)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	out := map[string]any{"registrasi": reg, "hasil": hasil}
	if f.Slug == "lab" {
		items, err := h.repo.LabItems(r.Context(), id)
		if err != nil {
			httpx.InternalError(w, err)
			return
		}
		out["lab"] = items
	}
	httpx.WriteJSON(w, http.StatusOK, out)
}

// SaveInput is a form's result as sent.
type SaveInput struct {
	Status     string           `json:"status"`
	Kesimpulan string           `json:"kesimpulan"`
	Values     map[string]Value `json:"values"`
}

// Validate trims the input and checks its limits. Empty answers are dropped.
func (in *SaveInput) Validate() httpx.FieldErrors {
	errs := httpx.FieldErrors{}
	in.Status = strings.TrimSpace(in.Status)
	in.Kesimpulan = strings.TrimSpace(in.Kesimpulan)
	if in.Status != "" && in.Status != StatusNormal && in.Status != StatusAbnormal {
		errs["status"] = "Status harus normal atau abnormal"
	}
	if utf8.RuneCountInString(in.Kesimpulan) > MaxKesimpulan {
		errs["kesimpulan"] = "Kesimpulan terlalu panjang (maksimal 4000 karakter)"
	}
	clean := map[string]Value{}
	for k, v := range in.Values {
		k = strings.TrimSpace(k)
		v.Value, v.Note = strings.TrimSpace(v.Value), strings.TrimSpace(v.Note)
		if v.Value == "" && v.Note == "" {
			continue
		}
		if k == "" || len(k) > MaxKeyLen || utf8.RuneCountInString(v.Value) > MaxValueLen || utf8.RuneCountInString(v.Note) > MaxNoteLen {
			errs["values"] = "Isian tidak valid atau terlalu panjang"
			break
		}
		clean[k] = v
	}
	if len(clean) > MaxItems {
		errs["values"] = "Terlalu banyak isian"
	}
	in.Values = clean
	return errs
}

// Save replaces the form's result. A cancelled registration answers 409.
//
// Body: {status: normal|abnormal|"", kesimpulan, values: {key: {value, note}}}.
func (h *Handler) Save(w http.ResponseWriter, r *http.Request, f Form) {
	id, ok := pathID(r)
	if !ok {
		httpx.WriteError(w, http.StatusNotFound, "registrasi MCU not found")
		return
	}
	var in SaveInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if errs := in.Validate(); len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return
	}
	ident, _ := auth.IdentityFrom(r.Context())
	user := ident.Email
	if at := strings.IndexByte(user, '@'); at > 0 {
		user = user[:at]
	}
	err := h.repo.Save(r.Context(), id, f.Slug, Hasil{Status: in.Status, Kesimpulan: in.Kesimpulan, Values: in.Values}, user)
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.WriteError(w, http.StatusNotFound, "registrasi MCU not found")
	case errors.Is(err, ErrCancelled):
		httpx.WriteError(w, http.StatusConflict, "Registrasi MCU ini sudah dibatalkan")
	case err != nil:
		httpx.InternalError(w, err)
	default:
		hasil, err := h.repo.Hasil(r.Context(), id, f.Slug)
		if err != nil {
			httpx.InternalError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"hasil": hasil})
	}
}
