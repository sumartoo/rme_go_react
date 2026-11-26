package pengobatan

import (
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
	"github.com/example/be_jwt_golang/backend/internal/icd"
	"github.com/example/be_jwt_golang/backend/internal/pendaftaran"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 008_transaksi_klinik_menu.sql).
const PagePath = "/transaksi-klinik/pengobatan"

// ReportPath is Report Klinik > Raw Data Pengobatan: the same list with patient
// columns, for roles that may open the report without the Pengobatan menu.
const ReportPath = "/report-klinik/raw-data-pengobatan"

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
	report := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(ReportPath)(f)) }
	mux.Handle("GET /api/report-klinik/raw-data-pengobatan", report(h.List))
	mux.Handle("GET /api/report-klinik/raw-data-pengobatan/options", report(h.Options))
	mux.Handle("GET /api/pengobatan", guard(h.List))
	mux.Handle("GET /api/pengobatan/options", guard(h.Options))
	mux.Handle("POST /api/pengobatan", guard(h.Save))
	mux.Handle("GET /api/pengobatan/kuitansi/{id}", guard(h.Kuitansi))
	mux.Handle("POST /api/pengobatan/kuitansi/{id}/cetak", guard(h.Cetak))
	mux.Handle("GET /api/pengobatan/nota/{id}", guard(h.Nota))
	mux.Handle("GET /api/pengobatan/detail/{id}", guard(h.Detail))
	mux.Handle("POST /api/pengobatan/detail/{id}/cancel", guard(h.Cancel))
	mux.Handle("GET /api/pengobatan/pendaftaran", guard(h.Pendaftaran))
	mux.Handle("GET /api/pengobatan/dokter", guard(h.Dokter))
	mux.Handle("GET /api/pengobatan/obat", guard(h.Obat))
	mux.Handle("GET /api/pengobatan/tindakan", guard(h.Tindakan))
	mux.Handle("GET /api/pengobatan/penunjang", guard(h.Penunjang))
	mux.Handle("GET /api/pengobatan/icd", guard(h.ICD))
	mux.Handle("GET /api/pengobatan/pemeriksaan/{idKun}", guard(h.GetPemeriksaan))
	mux.Handle("PUT /api/pengobatan/pemeriksaan/{idKun}", guard(h.SavePemeriksaan))

	// Transaksi Klinik > Pemeriksaan Perawat: the same examination and Dokter PJ
	// lookup, for roles that may open that page without the Pengobatan menu.
	perawat := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(PerawatPath)(f)) }
	mux.Handle("GET /api/pemeriksaan-perawat", perawat(h.PerawatList))
	mux.Handle("GET /api/pemeriksaan-perawat/options", perawat(h.PerawatOptions))
	mux.Handle("GET /api/pemeriksaan-perawat/dokter", perawat(h.Dokter))
	mux.Handle("GET /api/pemeriksaan-perawat/pendaftaran/{idKun}", perawat(h.PerawatReg))
	mux.Handle("GET /api/pemeriksaan-perawat/pemeriksaan/{idKun}", perawat(h.GetPemeriksaan))
	mux.Handle("PUT /api/pemeriksaan-perawat/pemeriksaan/{idKun}", perawat(h.SavePemeriksaan))

	// Transaksi Klinik > Pemeriksaan Dokter: the examination (a closed visit can
	// still change), the doctor's prescription and diagnoses, and their lookups.
	dokter := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(DokterPath)(f)) }
	mux.Handle("GET /api/pemeriksaan-dokter", dokter(h.PerawatList))
	mux.Handle("GET /api/pemeriksaan-dokter/options", dokter(h.DokterOptions))
	mux.Handle("GET /api/pemeriksaan-dokter/dokter", dokter(h.Dokter))
	mux.Handle("GET /api/pemeriksaan-dokter/obat", dokter(h.Obat))
	mux.Handle("GET /api/pemeriksaan-dokter/icd", dokter(h.ICD))
	mux.Handle("GET /api/pemeriksaan-dokter/pendaftaran/{idKun}", dokter(h.DokterReg))
	mux.Handle("GET /api/pemeriksaan-dokter/pemeriksaan/{idKun}", dokter(h.GetPemeriksaan))
	mux.Handle("PUT /api/pemeriksaan-dokter/pemeriksaan/{idKun}", dokter(h.SavePemeriksaanDokter))
	mux.Handle("GET /api/pemeriksaan-dokter/resep/{idKun}", dokter(h.GetResep))
	mux.Handle("PUT /api/pemeriksaan-dokter/resep/{idKun}", dokter(h.SaveResep))

	// Transaksi Klinik > Proses Obat: the same list, and a read-only view of the
	// examination, prescription and diagnoses.
	proses := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(ProsesObatPath)(f)) }
	mux.Handle("GET /api/proses-obat", proses(h.PerawatList))
	mux.Handle("GET /api/proses-obat/options", proses(h.PerawatOptions))
	mux.Handle("GET /api/proses-obat/pendaftaran/{idKun}", proses(h.ProsesObatReg))
	mux.Handle("GET /api/proses-obat/pemeriksaan/{idKun}", proses(h.GetPemeriksaan))
	mux.Handle("GET /api/proses-obat/resep/{idKun}", proses(h.GetResep))
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

func optInt(r *http.Request, name string) *int {
	v, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil || v <= 0 {
		return nil
	}
	return &v
}

func validDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// List returns one page of sales at the caller's branches, newest first.
//
// Query: from, to (YYYY-MM-DD, default today in the clinic's time zone),
// page, size (1..5000), q, sort, dir, cabang, nama, pelayanan,
// statusKunjungan, pembayaran, status (open, close, cancel).
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
	from, to := q.Get("from"), q.Get("to")
	if from == "" {
		from = pendaftaran.ClinicToday(h.now())
	}
	if to == "" {
		to = from
	}
	if !validDate(from) || !validDate(to) || to < from {
		httpx.WriteValidation(w, httpx.FieldErrors{"from": "Use a valid date range (YYYY-MM-DD, from ≤ to)"})
		return
	}
	status := strings.ToLower(strings.TrimSpace(q.Get("status")))
	if status != "" && status != StatusOpen && status != StatusClose && status != StatusCancel {
		httpx.WriteValidation(w, httpx.FieldErrors{"status": "Use open, close or cancel"})
		return
	}
	if len(codes) == 0 {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": []Row{}, "total": 0, "unfilteredTotal": 0, "from": from, "to": to})
		return
	}
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") != "asc"
	if _, ok := sortColumns[sort]; !ok {
		sort, desc = "id", true // newest first, like the original screen
	}
	res, err := h.repo.List(r.Context(), ListParams{
		Branches: codes, From: from, To: to,
		Search: strings.TrimSpace(q.Get("q")), Nama: strings.TrimSpace(q.Get("nama")),
		IDJenisRawat: optInt(r, "pelayanan"), IDStatusKunjung: optInt(r, "statusKunjungan"),
		IDJenisBayar: optInt(r, "pembayaran"), Status: status,
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

// Options returns the filter choices, the caller's branches and today's clinic date.
func (h *Handler) Options(w http.ResponseWriter, r *http.Request) {
	jenisRawat, err := h.repo.JenisRawat(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	statusKunjungan, err := h.repo.StatusKunjungan(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	jenisBayar, err := h.repo.JenisBayar(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	proses, err := h.repo.ProsesStatus(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"pelayanan": jenisRawat, "statusKunjungan": statusKunjungan, "pembayaran": jenisBayar, "proses": proses,
		"branches": branches, "today": pendaftaran.ClinicToday(h.now()),
	})
}

// Pendaftaran pages the registrations of one branch that a treatment can still
// be recorded for (the form's "Data Pendaftaran" lookup), newest first.
//
// Query: cabang (required, one of the caller's branches), from, to (YYYY-MM-DD,
// default yesterday and today in the clinic's time zone), nama, pelayanan,
// status (3 DAFTAR, 4 PROSES or 5 PAYMENT), q, sort, dir, page, size (1..100).
func (h *Handler) Pendaftaran(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	cabang, ok := h.lookupBranch(w, r)
	if !ok {
		return
	}
	now := h.now()
	from, to := q.Get("from"), q.Get("to")
	if to == "" {
		to = pendaftaran.ClinicToday(now)
	}
	if from == "" {
		from = pendaftaran.ClinicToday(now.AddDate(0, 0, -1))
	}
	if !validDate(from) || !validDate(to) || to < from {
		httpx.WriteValidation(w, httpx.FieldErrors{"from": "Use a valid date range (YYYY-MM-DD, from ≤ to)"})
		return
	}
	status := optInt(r, "status")
	if status != nil && !slices.Contains(BillableStatuses, *status) {
		httpx.WriteValidation(w, httpx.FieldErrors{"status": "Use DAFTAR (3), PROSES (4) or PAYMENT (5)"})
		return
	}
	size := intParam(r, "size", 5, 1, 100)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") != "asc"
	if _, ok := regSortColumns[sort]; !ok {
		sort, desc = "id", true
	}
	res, err := h.repo.Pendaftaran(r.Context(), RegParams{
		Branch: cabang, From: from, To: to, Nama: strings.TrimSpace(q.Get("nama")),
		IDJenisRawat: optInt(r, "pelayanan"), IDStatus: status, Search: strings.TrimSpace(q.Get("q")),
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

// lookupBranch reads the required cabang of a form lookup and checks that the
// caller is mapped to it. It writes the error response and returns false otherwise.
func (h *Handler) lookupBranch(w http.ResponseWriter, r *http.Request) (string, bool) {
	cabang := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("cabang")))
	if cabang == "" {
		httpx.WriteValidation(w, httpx.FieldErrors{"cabang": "Choose a branch first"})
		return "", false
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return "", false
	}
	if !branchauth.Has(branches, cabang) {
		httpx.WriteError(w, http.StatusForbidden, "you are not assigned to branch "+cabang)
		return "", false
	}
	return cabang, true
}

// Dokter lists the active doctors mapped to one of the caller's branches (the
// form's Dokter PJ lookup). Query: cabang (required).
func (h *Handler) Dokter(w http.ResponseWriter, r *http.Request) {
	cabang, ok := h.lookupBranch(w, r)
	if !ok {
		return
	}
	list, err := h.repo.Dokter(r.Context(), cabang)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": list})
}

// Obat lists the drugs sold at one of the caller's branches, with that branch's
// price and stock (the form's "Data Obat" lookup). Query: cabang (required).
func (h *Handler) Obat(w http.ResponseWriter, r *http.Request) {
	cabang, ok := h.lookupBranch(w, r)
	if !ok {
		return
	}
	list, err := h.repo.Obat(r.Context(), cabang)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": list})
}

// Tindakan lists the procedures of the Dokter PJ offered at one of the caller's
// branches, with that branch's price (the form's "Data Tindakan Dokter" lookup).
// Query: cabang and dokter (both required).
func (h *Handler) Tindakan(w http.ResponseWriter, r *http.Request) {
	dokter := optInt(r, "dokter")
	if dokter == nil {
		httpx.WriteValidation(w, httpx.FieldErrors{"dokter": "Choose the Dokter PJ first"})
		return
	}
	cabang, ok := h.lookupBranch(w, r)
	if !ok {
		return
	}
	list, err := h.repo.Tindakan(r.Context(), cabang, *dokter)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": list})
}

// Penunjang lists the supporting examinations offered at one of the caller's
// branches, with that branch's price (the Penunjang tab's lookup).
// Query: cabang (required).
func (h *Handler) Penunjang(w http.ResponseWriter, r *http.Request) {
	cabang, ok := h.lookupBranch(w, r)
	if !ok {
		return
	}
	list, err := h.repo.Penunjang(r.Context(), cabang)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": list})
}

// icdSorts are the sortable columns of the diagnosis lookup.
var icdSorts = map[string]bool{"id": true, "kode": true, "nama": true, "jenis": true}

// ICD pages the ICD-X codes for the Diagnosa tab's lookup ("Data Diagnosa"),
// by id like the original screen. The codes are not per branch.
// Query: q, sort (id, kode, nama, jenis), dir, page, size (1..100).
func (h *Handler) ICD(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	size := intParam(r, "size", 10, 1, 100)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") == "desc"
	if !icdSorts[sort] {
		sort, desc = "id", false
	}
	res, err := h.repo.ICD(r.Context(), icd.ListParams{
		Search: strings.TrimSpace(q.Get("q")), Sort: sort, Desc: desc, Limit: size, Offset: (page - 1) * size,
	})
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// regOfCaller loads the registration in the path and checks it belongs to one
// of the caller's branches. Another branch's registration answers 404, so its
// existence is not revealed. It writes the error response and returns false otherwise.
func (h *Handler) regOfCaller(w http.ResponseWriter, r *http.Request) (idKun, status int, ok bool) {
	idKun, err := strconv.Atoi(r.PathValue("idKun"))
	if err != nil || idKun <= 0 {
		httpx.WriteError(w, http.StatusNotFound, "pendaftaran not found")
		return 0, 0, false
	}
	cabang, status, err := h.repo.RegBranch(r.Context(), idKun)
	if errors.Is(err, ErrRegNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "pendaftaran not found")
		return 0, 0, false
	}
	if err != nil {
		httpx.InternalError(w, err)
		return 0, 0, false
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return 0, 0, false
	}
	if !branchauth.Has(branches, cabang) {
		httpx.WriteError(w, http.StatusNotFound, "pendaftaran not found")
		return 0, 0, false
	}
	return idKun, status, true
}

// GetPemeriksaan returns the examination of a registration at the caller's
// branches: {"pemeriksaan": {...}} or {"pemeriksaan": null} when none is saved yet.
func (h *Handler) GetPemeriksaan(w http.ResponseWriter, r *http.Request) {
	idKun, _, ok := h.regOfCaller(w, r)
	if !ok {
		return
	}
	rec, err := h.repo.GetPemeriksaan(r.Context(), idKun)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"pemeriksaan": rec})
}

// SavePemeriksaan inserts or updates the examination of a registration at the
// caller's branches (trxpemeriksaan). Only while the visit is DAFTAR, PROSES or
// PAYMENT; a closed or cancelled visit answers 409.
func (h *Handler) SavePemeriksaan(w http.ResponseWriter, r *http.Request) {
	h.savePemeriksaanWith(w, r, examEditable)
}

// SavePemeriksaanDokter is SavePemeriksaan for Pemeriksaan Dokter, where a
// closed visit can still change (not a cancelled one).
func (h *Handler) SavePemeriksaanDokter(w http.ResponseWriter, r *http.Request) {
	h.savePemeriksaanWith(w, r, dokterEditable)
}

func (h *Handler) savePemeriksaanWith(w http.ResponseWriter, r *http.Request, editable func(status int) bool) {
	idKun, status, ok := h.regOfCaller(w, r)
	if !ok {
		return
	}
	var in Pemeriksaan
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if errs := in.Validate(); len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return
	}
	if !editable(status) {
		httpx.WriteError(w, http.StatusConflict, ErrRegNotEditable.Error())
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	user := []rune(id.Email)
	if len(user) > 20 { // create_user / update_user are varchar(20)
		user = user[:20]
	}
	err := h.repo.SavePemeriksaan(r.Context(), idKun, in, string(user))
	if errors.Is(err, ErrUnknownDokter) {
		httpx.WriteValidation(w, httpx.FieldErrors{"idDokter": "Unknown doctor"})
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	rec, err := h.repo.GetPemeriksaan(r.Context(), idKun)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"pemeriksaan": rec})
}

// Save records a treatment sale from the form (see save.go): the sale, its
// totals and lines, the stock it uses, and the registration becomes CLOSE.
// Answers 201 with the receipt number and the computed totals.
func (h *Handler) Save(w http.ResponseWriter, r *http.Request) {
	var in SaveInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
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
	user := []rune(id.Email)
	if len(user) > 20 { // create_user is varchar(20)
		user = user[:20]
	}
	res, err := h.repo.Save(r.Context(), in, string(user))
	var item *ItemError
	switch {
	case err == nil:
		httpx.WriteJSON(w, http.StatusCreated, res)
	case errors.Is(err, ErrRegNotFound):
		httpx.WriteValidation(w, httpx.FieldErrors{"idKun": "Unknown registration"})
	case errors.Is(err, ErrBranchMismatch):
		httpx.WriteValidation(w, httpx.FieldErrors{"idKun": "The registration belongs to another branch"})
	case errors.Is(err, ErrRegNotEditable), errors.Is(err, ErrAlreadyBilled):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	case errors.As(err, &item):
		if strings.Contains(item.Msg, "stock") {
			httpx.WriteError(w, http.StatusConflict, item.Msg)
			return
		}
		httpx.WriteValidation(w, httpx.FieldErrors{item.Field: item.Error()})
	default:
		httpx.InternalError(w, err)
	}
}

// saleOfCaller loads a sale's receipt and checks it belongs to one of the
// caller's branches; another branch's sale answers 404.
func (h *Handler) saleOfCaller(w http.ResponseWriter, r *http.Request) (Kuitansi, bool) {
	id, err := strconv.Atoi(r.PathValue("id"))
	if err != nil || id <= 0 {
		httpx.WriteError(w, http.StatusNotFound, "pengobatan not found")
		return Kuitansi{}, false
	}
	k, err := h.repo.Kuitansi(r.Context(), id)
	if errors.Is(err, ErrSaleNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "pengobatan not found")
		return k, false
	}
	if err != nil {
		httpx.InternalError(w, err)
		return k, false
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return k, false
	}
	if !branchauth.Has(branches, k.Cabang) {
		httpx.WriteError(w, http.StatusNotFound, "pengobatan not found")
		return k, false
	}
	return k, true
}

// Kuitansi returns the receipt data of a sale at the caller's branches.
func (h *Handler) Kuitansi(w http.ResponseWriter, r *http.Request) {
	if k, ok := h.saleOfCaller(w, r); ok {
		httpx.WriteJSON(w, http.StatusOK, k)
	}
}

// Cetak records the receipt print (the first one gets a print number in
// trxjualcetak) and returns the receipt with this print's time and user.
func (h *Handler) Cetak(w http.ResponseWriter, r *http.Request) {
	k, ok := h.saleOfCaller(w, r)
	if !ok {
		return
	}
	res, err := h.repo.Cetak(r.Context(), k.ID, shortUser(r, 50))
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// Nota returns a sale with its lines for the Print menu (A5, A5 bold, thermal,
// summary), for a sale at the caller's branches.
func (h *Handler) Nota(w http.ResponseWriter, r *http.Request) {
	k, ok := h.saleOfCaller(w, r)
	if !ok {
		return
	}
	n, err := h.repo.Nota(r.Context(), k.ID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, n)
}

// shortUser is the caller as the legacy tables name users: the e-mail without
// its domain ("ato"), cut to max runes.
func shortUser(r *http.Request, max int) string {
	id, _ := auth.IdentityFrom(r.Context())
	user := id.Email
	if at := strings.IndexByte(user, '@'); at > 0 {
		user = user[:at]
	}
	if u := []rune(user); len(u) > max {
		user = string(u[:max])
	}
	return user
}

// Detail returns a saved sale for the Edit page, for a sale at the caller's branches.
func (h *Handler) Detail(w http.ResponseWriter, r *http.Request) {
	k, ok := h.saleOfCaller(w, r)
	if !ok {
		return
	}
	d, err := h.repo.Detail(r.Context(), k.ID)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, d)
}

// CancelInput is the "Cancel Pengobatan" dialog.
type CancelInput struct {
	Alasan string `json:"alasan"`
}

// Cancel cancels a sale at the caller's branches with a reason (max 100
// characters, trxjualtotal.cancelreason). An already cancelled sale answers 409.
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	k, ok := h.saleOfCaller(w, r)
	if !ok {
		return
	}
	var in CancelInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	in.Alasan = strings.TrimSpace(in.Alasan)
	switch n := len([]rune(in.Alasan)); {
	case n == 0:
		httpx.WriteValidation(w, httpx.FieldErrors{"alasan": "Alasan is required"})
		return
	case n > 100:
		httpx.WriteValidation(w, httpx.FieldErrors{"alasan": "Alasan must be at most 100 characters"})
		return
	}
	err := h.repo.Cancel(r.Context(), k.ID, in.Alasan, shortUser(r, 30)) // cancelby is varchar(30)
	switch {
	case err == nil:
		d, err := h.repo.Detail(r.Context(), k.ID)
		if err != nil {
			httpx.InternalError(w, err)
			return
		}
		httpx.WriteJSON(w, http.StatusOK, d)
	case errors.Is(err, ErrAlreadyCancelled):
		httpx.WriteError(w, http.StatusConflict, err.Error())
	case errors.Is(err, ErrSaleNotFound):
		httpx.WriteError(w, http.StatusNotFound, "pengobatan not found")
	default:
		httpx.InternalError(w, err)
	}
}
