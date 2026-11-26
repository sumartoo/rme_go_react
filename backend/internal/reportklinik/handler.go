package reportklinik

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
	"github.com/example/be_jwt_golang/backend/internal/pendaftaran"
)

// PenjualanObatPath is the menu item (Report Klinik > Penjualan Obat) that
// grants access to its endpoints.
const PenjualanObatPath = "/report-klinik/penjualan-obat"

// PenjualanTindakanPath is Report Klinik > Penjualan Tindakan.
const PenjualanTindakanPath = "/report-klinik/penjualan-tindakan"

// PenjualanPenunjangPath is Report Klinik > Penjualan Penunjang.
const PenjualanPenunjangPath = "/report-klinik/penjualan-penunjang"

// PenjualanOTCPath is Report Klinik > Penjualan OTC.
const PenjualanOTCPath = "/report-klinik/penjualan-otc"

// PenjualanSoftdrinkPath is Report Klinik > Penjualan Softdrink.
const PenjualanSoftdrinkPath = "/report-klinik/penjualan-softdrink"

// PasienBaruPath is Report Klinik > Pasien Baru.
const PasienBaruPath = "/report-klinik/pasien-baru"

// RekapAsuransiPath is Report Klinik > Rekap Asuransi.
const RekapAsuransiPath = "/report-klinik/rekap-asuransi"

// TransaksiHarianPath is Report Klinik > Transaksi Harian.
const TransaksiHarianPath = "/report-klinik/transaksi-harian"

// AllStokPath is Report Klinik > All Stok.
const AllStokPath = "/report-klinik/all-stok"

// RujukanPath is Report Klinik > Rujukan.
const RujukanPath = "/report-klinik/rujukan"

// PendaftaranPath is Report Klinik > Pendaftaran.
const PendaftaranPath = "/report-klinik/pendaftaran"

// PendapatanPath is Report Klinik > Report Pendapatan.
const PendapatanPath = "/report-klinik/report-pendapatan"

// DiagnosaPath is Report Klinik > Report Diagnosa.
const DiagnosaPath = "/report-klinik/report-diagnosa"

// DinkesPath is Report Klinik > Report ICD Dinkes.
const DinkesPath = "/report-klinik/report-icd-dinkes"

// StokOpnamePath is Stok Farmasi > Stok Opname.
const StokOpnamePath = "/stok-farmasi/stok-opname"

// StokAlertPath is Stok Farmasi > Stok Alert.
const StokAlertPath = "/stok-farmasi/stok-alert"

// StokTracePath is Stok Farmasi > Stok Trace.
const StokTracePath = "/stok-farmasi/stok-trace"

// MaxPageSize caps one request; exports ask for up to this many rows.
const MaxPageSize = 5000

type Handler struct {
	repo Repository
	now  func() time.Time
}

func NewHandler(repo Repository) *Handler {
	return &Handler{repo: repo, now: time.Now}
}

// Routes registers each report's endpoints behind its own menu item.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	guard := func(path string, f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(path)(f)) }
	mux.Handle("GET /api/report-klinik/penjualan-obat", guard(PenjualanObatPath, h.PenjualanObat))
	mux.Handle("GET /api/report-klinik/penjualan-obat/options", guard(PenjualanObatPath, h.PenjualanObatOptions))
	mux.Handle("GET /api/report-klinik/penjualan-tindakan", guard(PenjualanTindakanPath, h.PenjualanTindakan))
	mux.Handle("GET /api/report-klinik/penjualan-tindakan/options", guard(PenjualanTindakanPath, h.PeriodOptions))
	mux.Handle("GET /api/report-klinik/penjualan-penunjang", guard(PenjualanPenunjangPath, h.PenjualanPenunjang))
	mux.Handle("GET /api/report-klinik/penjualan-penunjang/options", guard(PenjualanPenunjangPath, h.PenjualanPenunjangOptions))
	mux.Handle("GET /api/report-klinik/penjualan-otc", guard(PenjualanOTCPath, h.PenjualanOTC))
	mux.Handle("GET /api/report-klinik/penjualan-otc/options", guard(PenjualanOTCPath, h.PenjualanOTCOptions))
	mux.Handle("GET /api/report-klinik/penjualan-softdrink", guard(PenjualanSoftdrinkPath, h.PenjualanSoftdrink))
	mux.Handle("GET /api/report-klinik/penjualan-softdrink/options", guard(PenjualanSoftdrinkPath, h.PenjualanOTCOptions))
	mux.Handle("GET /api/report-klinik/pasien-baru", guard(PasienBaruPath, h.PasienBaru))
	mux.Handle("GET /api/report-klinik/pasien-baru/options", guard(PasienBaruPath, h.PasienBaruOptions))
	mux.Handle("GET /api/report-klinik/rekap-asuransi", guard(RekapAsuransiPath, h.RekapAsuransi))
	mux.Handle("GET /api/report-klinik/rekap-asuransi/options", guard(RekapAsuransiPath, h.RekapAsuransiOptions))
	mux.Handle("GET /api/report-klinik/transaksi-harian", guard(TransaksiHarianPath, h.TransaksiHarian))
	mux.Handle("GET /api/report-klinik/transaksi-harian/options", guard(TransaksiHarianPath, h.TransaksiHarianOptions))
	mux.Handle("GET /api/report-klinik/all-stok", guard(AllStokPath, h.AllStok))
	mux.Handle("GET /api/report-klinik/all-stok/options", guard(AllStokPath, h.AllStokOptions))
	mux.Handle("GET /api/report-klinik/rujukan", guard(RujukanPath, h.Rujukan))
	mux.Handle("GET /api/report-klinik/rujukan/options", guard(RujukanPath, h.PeriodOptions))
	mux.Handle("GET /api/report-klinik/pendaftaran", guard(PendaftaranPath, h.Pendaftaran))
	mux.Handle("GET /api/report-klinik/pendaftaran/options", guard(PendaftaranPath, h.PendaftaranOptions))
	mux.Handle("GET /api/report-klinik/pendapatan", guard(PendapatanPath, h.Pendapatan))
	mux.Handle("GET /api/report-klinik/pendapatan/options", guard(PendapatanPath, h.PendapatanOptions))
	mux.Handle("GET /api/report-klinik/pendapatan/dokter", guard(PendapatanPath, h.PendapatanDokter))
	mux.Handle("GET /api/report-klinik/diagnosa", guard(DiagnosaPath, h.ReportDiagnosa))
	mux.Handle("GET /api/report-klinik/diagnosa/options", guard(DiagnosaPath, h.PendapatanOptions))
	mux.Handle("GET /api/report-klinik/icd-dinkes", guard(DinkesPath, h.ReportDinkes))
	mux.Handle("GET /api/report-klinik/icd-dinkes/options", guard(DinkesPath, h.PeriodOptions))
	mux.Handle("GET /api/stok-farmasi/stok-opname", guard(StokOpnamePath, h.StokOpname))
	mux.Handle("GET /api/stok-farmasi/stok-opname/options", guard(StokOpnamePath, h.StokOpnameOptions))
	mux.Handle("GET /api/stok-farmasi/stok-alert/minim-stok", guard(StokAlertPath, h.MinimStok))
	mux.Handle("GET /api/stok-farmasi/stok-alert/options", guard(StokAlertPath, h.StokOpnameOptions))
	mux.Handle("GET /api/stok-farmasi/stok-trace/obat", guard(StokTracePath, h.StokTraceObat))
	mux.Handle("GET /api/stok-farmasi/stok-trace", guard(StokTracePath, h.StokTrace))
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

// period reads the branch and date range every report shares: the caller's
// branches (or the one asked for, 403 if not theirs) and from/to (default
// today, clinic time). ok is false when an error was written.
func (h *Handler) period(w http.ResponseWriter, r *http.Request) (codes []string, from, to string, ok bool) {
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return nil, "", "", false
	}
	q := r.URL.Query()
	codes = branchauth.Codes(branches)
	if c := strings.ToUpper(strings.TrimSpace(q.Get("cabang"))); c != "" {
		if !branchauth.Has(branches, c) {
			httpx.WriteError(w, http.StatusForbidden, "you are not assigned to branch "+c)
			return nil, "", "", false
		}
		codes = []string{c}
	}
	from, to = q.Get("from"), q.Get("to")
	if from == "" {
		from = pendaftaran.ClinicToday(h.now())
	}
	if to == "" {
		to = from
	}
	if !validDate(from) || !validDate(to) || to < from {
		httpx.WriteValidation(w, httpx.FieldErrors{"from": "Use a valid date range (YYYY-MM-DD, from ≤ to)"})
		return nil, "", "", false
	}
	return codes, from, to, true
}

// PenjualanObat returns one page of drugs sold per day at the caller's
// branches, by drug name then day.
//
// Query: from, to (YYYY-MM-DD, default today), cabang, page, size (1..5000),
// q, sort (tgl, obat, jenis, satuan, harga, byk, jumlah, cabang, status), dir,
// nama, jenis (refjenisobat id), status (open, close, cancel).
func (h *Handler) PenjualanObat(w http.ResponseWriter, r *http.Request) {
	codes, from, to, ok := h.period(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
	status, ok := statusParam(w, r)
	if !ok {
		return
	}
	if len(codes) == 0 {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": []ObatRow{}, "total": 0, "unfilteredTotal": 0, "from": from, "to": to})
		return
	}
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") == "desc"
	if !ObatSortable(sort) {
		sort, desc = "obat", false
	}
	res, err := h.repo.PenjualanObat(r.Context(), ObatParams{
		Branches: codes, From: from, To: to,
		Search: strings.TrimSpace(q.Get("q")), Nama: strings.TrimSpace(q.Get("nama")), IDJenis: optInt(r, "jenis"), Status: status,
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

// PenjualanObatOptions returns the Cari panel's choices, the caller's branches and today.
func (h *Handler) PenjualanObatOptions(w http.ResponseWriter, r *http.Request) {
	jenis, err := h.repo.JenisObat(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"jenis": jenis, "branches": branches, "today": pendaftaran.ClinicToday(h.now())})
}

// statusParam reads the optional status filter; ok is false when an error was written.
func statusParam(w http.ResponseWriter, r *http.Request) (string, bool) {
	status := strings.ToLower(strings.TrimSpace(r.URL.Query().Get("status")))
	if status != "" && status != StatusOpen && status != StatusClose && status != StatusCancel {
		httpx.WriteValidation(w, httpx.FieldErrors{"status": "Use open, close or cancel"})
		return "", false
	}
	return status, true
}

// PenjualanTindakan returns one page of procedure lines sold at the caller's
// branches, newest first.
//
// Query: from, to (YYYY-MM-DD, default today), cabang, page, size (1..5000),
// q, sort (tgl, tindakan, jenis, dokter, pasien, fee, harga, byk, totalFee,
// cabang, status), dir, nama (procedure), dokter, status (open, close, cancel).
func (h *Handler) PenjualanTindakan(w http.ResponseWriter, r *http.Request) {
	codes, from, to, ok := h.period(w, r)
	if !ok {
		return
	}
	status, ok := statusParam(w, r)
	if !ok {
		return
	}
	if len(codes) == 0 {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": []TindakanRow{}, "total": 0, "unfilteredTotal": 0, "from": from, "to": to})
		return
	}
	q := r.URL.Query()
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") != "asc"
	if !TindakanSortable(sort) {
		sort, desc = "tgl", true // newest first, like the legacy report
	}
	res, err := h.repo.PenjualanTindakan(r.Context(), TindakanParams{
		Branches: codes, From: from, To: to,
		Search: strings.TrimSpace(q.Get("q")), Nama: strings.TrimSpace(q.Get("nama")), Dokter: strings.TrimSpace(q.Get("dokter")), Status: status,
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

// PeriodOptions returns what a report filtered only by branch and dates needs:
// the caller's branches and today's clinic date.
func (h *Handler) PeriodOptions(w http.ResponseWriter, r *http.Request) {
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"branches": branches, "today": pendaftaran.ClinicToday(h.now())})
}

// PenjualanPenunjang returns one page of supporting examinations sold at the
// caller's branches, newest first.
//
// Query: from, to (YYYY-MM-DD, default today), cabang, page, size (1..5000),
// q, sort (tgl, noRm, pasien, penunjang, jenis, dokter, harga, byk, cabang,
// status, createUser), dir, nama (examination), jenis (refjenispanjang id),
// status (open, close, cancel).
func (h *Handler) PenjualanPenunjang(w http.ResponseWriter, r *http.Request) {
	codes, from, to, ok := h.period(w, r)
	if !ok {
		return
	}
	status, ok := statusParam(w, r)
	if !ok {
		return
	}
	if len(codes) == 0 {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": []PenunjangRow{}, "total": 0, "unfilteredTotal": 0, "from": from, "to": to})
		return
	}
	q := r.URL.Query()
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") != "asc"
	if !PenunjangSortable(sort) {
		sort, desc = "tgl", true // newest first, like the legacy report
	}
	res, err := h.repo.PenjualanPenunjang(r.Context(), PenunjangParams{
		Branches: codes, From: from, To: to,
		Search: strings.TrimSpace(q.Get("q")), Nama: strings.TrimSpace(q.Get("nama")), IDJenis: optInt(r, "jenis"), Status: status,
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

// PenjualanPenunjangOptions returns the examination types, the caller's branches and today.
func (h *Handler) PenjualanPenunjangOptions(w http.ResponseWriter, r *http.Request) {
	jenis, err := h.repo.JenisPenunjang(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"jenis": jenis, "branches": branches, "today": pendaftaran.ClinicToday(h.now())})
}

// PenjualanOTC returns one page of OTC drug lines sold at the caller's
// branches, newest sale first.
//
// Query: from, to (YYYY-MM-DD, default today), cabang, page, size (1..5000),
// q, sort (idJual, tgl, noKwi, obat, jenis, perujuk, byk, harga, ppn, total,
// cabang, status), dir, nama (drug), perujuk (refperujukotc id), status.
func (h *Handler) PenjualanOTC(w http.ResponseWriter, r *http.Request) {
	h.penjualanJenis(w, r, JenisOTC)
}

// PenjualanSoftdrink is PenjualanOTC for the drugs of type SOFT DRINK.
func (h *Handler) PenjualanSoftdrink(w http.ResponseWriter, r *http.Request) {
	h.penjualanJenis(w, r, JenisSoftDrink)
}

// penjualanJenis pages the drug lines of one drug type (JenisOTC or JenisSoftDrink).
func (h *Handler) penjualanJenis(w http.ResponseWriter, r *http.Request, jenis string) {
	codes, from, to, ok := h.period(w, r)
	if !ok {
		return
	}
	status, ok := statusParam(w, r)
	if !ok {
		return
	}
	if len(codes) == 0 {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": []OTCRow{}, "total": 0, "unfilteredTotal": 0, "from": from, "to": to})
		return
	}
	q := r.URL.Query()
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") != "asc"
	if !OTCSortable(sort) {
		sort, desc = "idJual", true // newest sale first, like the legacy report
	}
	res, err := h.repo.PenjualanOTC(r.Context(), OTCParams{
		Branches: codes, From: from, To: to, Jenis: jenis,
		Search: strings.TrimSpace(q.Get("q")), Nama: strings.TrimSpace(q.Get("nama")), IDPerujuk: optInt(r, "perujuk"), Status: status,
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

// PenjualanOTCOptions returns the OTC referrers, the caller's branches and today.
func (h *Handler) PenjualanOTCOptions(w http.ResponseWriter, r *http.Request) {
	perujuk, err := h.repo.PerujukOTC(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"perujuk": perujuk, "branches": branches, "today": pendaftaran.ClinicToday(h.now())})
}

// PasienBaru returns one page of visits by patients registered in the period,
// at the caller's branches, newest first.
//
// Query: from, to (YYYY-MM-DD, default today), cabang, page, size (1..5000),
// q, sort (tgl, noRm, pasien, jenisRawat, tempatLahir, tglLahir, umur, alamat,
// tglDaftar, dokter, status, cabang), dir, nama, pelayanan (refjenisrawat id),
// status (open, close, cancel).
func (h *Handler) PasienBaru(w http.ResponseWriter, r *http.Request) {
	codes, from, to, ok := h.period(w, r)
	if !ok {
		return
	}
	status, ok := statusParam(w, r)
	if !ok {
		return
	}
	if len(codes) == 0 {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": []PasienBaruRow{}, "total": 0, "unfilteredTotal": 0, "from": from, "to": to})
		return
	}
	q := r.URL.Query()
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") != "asc"
	if !PasienBaruSortable(sort) {
		sort, desc = "tgl", true // newest visit first, like the legacy report
	}
	repoDesc := desc
	if sort == "umur" {
		repoDesc = !desc // age sorts by birth date the other way round
	}
	res, err := h.repo.PasienBaru(r.Context(), PasienBaruParams{
		Branches: codes, From: from, To: to,
		Search: strings.TrimSpace(q.Get("q")), Nama: strings.TrimSpace(q.Get("nama")), IDJenisRawat: optInt(r, "pelayanan"), Status: status,
		Sort: sort, Desc: repoDesc, Limit: size, Offset: (page - 1) * size,
	})
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"rows": res.Rows, "total": res.Total, "unfilteredTotal": res.UnfilteredTotal, "from": from, "to": to,
	})
}

// PasienBaruOptions returns the services, the caller's branches and today.
func (h *Handler) PasienBaruOptions(w http.ResponseWriter, r *http.Request) {
	pelayanan, err := h.repo.JenisRawat(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"pelayanan": pelayanan, "branches": branches, "today": pendaftaran.ClinicToday(h.now())})
}

// RekapAsuransi returns one page of insured sales at the caller's branches,
// newest first, each with its items per group and diagnoses. Paging counts sales.
//
// Query: from, to (YYYY-MM-DD, default today), cabang, page, size (1..5000),
// q, sort (tgl, noRm, noKartu, perusahaan, pasien, dokter, total, cabang,
// status), dir, nama (patient), asuransi (refjaminankesehatan id), status.
func (h *Handler) RekapAsuransi(w http.ResponseWriter, r *http.Request) {
	codes, from, to, ok := h.period(w, r)
	if !ok {
		return
	}
	status, ok := statusParam(w, r)
	if !ok {
		return
	}
	if len(codes) == 0 {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": []AsuransiSale{}, "total": 0, "unfilteredTotal": 0, "from": from, "to": to})
		return
	}
	q := r.URL.Query()
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") != "asc"
	if !AsuransiSortable(sort) {
		sort, desc = "tgl", true // newest first, like the legacy report
	}
	res, err := h.repo.RekapAsuransi(r.Context(), AsuransiParams{
		Branches: codes, From: from, To: to,
		Search: strings.TrimSpace(q.Get("q")), Nama: strings.TrimSpace(q.Get("nama")), IDJamkes: optInt(r, "asuransi"), Status: status,
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

// RekapAsuransiOptions returns the insurers, the caller's branches and today.
func (h *Handler) RekapAsuransiOptions(w http.ResponseWriter, r *http.Request) {
	asuransi, err := h.repo.Asuransi(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"asuransi": asuransi, "branches": branches, "today": pendaftaran.ClinicToday(h.now())})
}

// TransaksiHarian returns one page of sales at the caller's branches, newest
// first, with their cost per group, total and discount.
//
// Query: from, to (YYYY-MM-DD, default today), cabang, pelayanan
// (refjenisrawat id), page, size (1..5000), q, sort (tgl, noRm, pasien,
// pelayanan, dokter, pembayaran, obat, tindakan, penunjang, total, cabang,
// status, diskon), dir, nama (patient), pembayaran (refjenisbayar id), status.
func (h *Handler) TransaksiHarian(w http.ResponseWriter, r *http.Request) {
	codes, from, to, ok := h.period(w, r)
	if !ok {
		return
	}
	status, ok := statusParam(w, r)
	if !ok {
		return
	}
	if len(codes) == 0 {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": []HarianRow{}, "total": 0, "unfilteredTotal": 0, "from": from, "to": to})
		return
	}
	q := r.URL.Query()
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") != "asc"
	if !HarianSortable(sort) {
		sort, desc = "tgl", true // newest first
	}
	res, err := h.repo.TransaksiHarian(r.Context(), HarianParams{
		Branches: codes, From: from, To: to, IDJenisRawat: optInt(r, "pelayanan"),
		Search: strings.TrimSpace(q.Get("q")), Nama: strings.TrimSpace(q.Get("nama")), IDJenisBayar: optInt(r, "pembayaran"), Status: status,
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

// TransaksiHarianOptions returns the services, payment types, the caller's branches and today.
func (h *Handler) TransaksiHarianOptions(w http.ResponseWriter, r *http.Request) {
	pelayanan, err := h.repo.JenisRawat(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	pembayaran, err := h.repo.JenisBayar(r.Context())
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
		"pelayanan": pelayanan, "pembayaran": pembayaran, "branches": branches, "today": pendaftaran.ClinicToday(h.now()),
	})
}

// AllStok returns one page of drugs per branch at the caller's branches with
// their stock movement over the period, the dearest first.
//
// Query: from, to (YYYY-MM-DD, default today), cabang, page, size (1..5000),
// q, sort (idObat, obat, satuanBeli, isiPerBox, satuanJual, hargaBeli,
// hargaJual, jenis, principal, distributor, movingPo, stokAwal, jual,
// stokAkhir, cabang), dir, nama, jenis (refjenisobat id), movingPo
// (refkodemovingpo id).
func (h *Handler) AllStok(w http.ResponseWriter, r *http.Request) {
	codes, from, to, ok := h.period(w, r)
	if !ok {
		return
	}
	if len(codes) == 0 {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": []StokRow{}, "total": 0, "unfilteredTotal": 0, "from": from, "to": to})
		return
	}
	q := r.URL.Query()
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") != "asc"
	if !StokSortable(sort) {
		sort, desc = "hargaJual", true // the dearest first, like the legacy report
	}
	res, err := h.repo.AllStok(r.Context(), StokParams{
		Branches: codes, From: from, To: to,
		Search: strings.TrimSpace(q.Get("q")), Nama: strings.TrimSpace(q.Get("nama")), IDJenis: optInt(r, "jenis"), IDMovingPO: optInt(r, "movingPo"),
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

// AllStokOptions returns the drug types, PO moving codes, the caller's branches and today.
func (h *Handler) AllStokOptions(w http.ResponseWriter, r *http.Request) {
	jenis, err := h.repo.JenisObat(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	moving, err := h.repo.MovingPO(r.Context())
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
		"jenis": jenis, "movingPo": moving, "branches": branches, "today": pendaftaran.ClinicToday(h.now()),
	})
}

// Rujukan returns one page of the sales of external doctors at the caller's
// branches, newest first, once per supporting examination.
//
// Query: from, to (YYYY-MM-DD, default today), cabang, page, size (1..5000),
// q, sort (tgl, noRm, pasien, dokter, jenisRawat, penunjang, total, cabang,
// status), dir, nama (patient), dokter, status (open, close, cancel).
func (h *Handler) Rujukan(w http.ResponseWriter, r *http.Request) {
	codes, from, to, ok := h.period(w, r)
	if !ok {
		return
	}
	status, ok := statusParam(w, r)
	if !ok {
		return
	}
	if len(codes) == 0 {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": []RujukanRow{}, "total": 0, "unfilteredTotal": 0, "from": from, "to": to})
		return
	}
	q := r.URL.Query()
	size := intParam(r, "size", 20, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") != "asc"
	if !RujukanSortable(sort) {
		sort, desc = "tgl", true // newest first, like the legacy report
	}
	res, err := h.repo.Rujukan(r.Context(), RujukanParams{
		Branches: codes, From: from, To: to,
		Search: strings.TrimSpace(q.Get("q")), Nama: strings.TrimSpace(q.Get("nama")), Dokter: strings.TrimSpace(q.Get("dokter")), Status: status,
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

// Pendaftaran returns one page of registrations at the caller's branches,
// newest first.
//
// Query: from, to (YYYY-MM-DD, default today), cabang, page, size (1..5000),
// q, sort (id, noAntri, tgl, pelayanan, statusKun, noRm, pasien, umur, alamat,
// cabang), dir, nama (patient), pelayanan (refjenisrawat id), statusKunjungan
// (refstatustransaksi id).
func (h *Handler) Pendaftaran(w http.ResponseWriter, r *http.Request) {
	codes, from, to, ok := h.period(w, r)
	if !ok {
		return
	}
	if len(codes) == 0 {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": []PendaftaranRow{}, "total": 0, "unfilteredTotal": 0, "from": from, "to": to})
		return
	}
	q := r.URL.Query()
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") != "asc"
	if !PendaftaranSortable(sort) {
		sort, desc = "id", true // newest registration first, like the legacy report
	}
	repoDesc := desc
	if sort == "umur" {
		repoDesc = !desc // age sorts by birth date the other way round
	}
	res, err := h.repo.Pendaftaran(r.Context(), PendaftaranParams{
		Branches: codes, From: from, To: to,
		Search: strings.TrimSpace(q.Get("q")), Nama: strings.TrimSpace(q.Get("nama")),
		IDJenisRawat: optInt(r, "pelayanan"), IDStatus: optInt(r, "statusKunjungan"),
		Sort: sort, Desc: repoDesc, Limit: size, Offset: (page - 1) * size,
	})
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"rows": res.Rows, "total": res.Total, "unfilteredTotal": res.UnfilteredTotal, "from": from, "to": to,
	})
}

// PendaftaranOptions returns the services, visit statuses, the caller's branches and today.
func (h *Handler) PendaftaranOptions(w http.ResponseWriter, r *http.Request) {
	pelayanan, err := h.repo.JenisRawat(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	status, err := h.repo.StatusKunjungan(r.Context())
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
		"pelayanan": pelayanan, "statusKunjungan": status, "branches": branches, "today": pendaftaran.ClinicToday(h.now()),
	})
}

// oneBranch reads the required cabang parameter: one of the caller's branches
// (400 when missing, 403 when not theirs). ok is false when an error was written.
func (h *Handler) oneBranch(w http.ResponseWriter, r *http.Request) (string, bool) {
	c := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("cabang")))
	if c == "" {
		httpx.WriteValidation(w, httpx.FieldErrors{"cabang": "Pilih Cabang"})
		return "", false
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return "", false
	}
	if !branchauth.Has(branches, c) {
		httpx.WriteError(w, http.StatusForbidden, "you are not assigned to branch "+c)
		return "", false
	}
	return c, true
}

// Pendapatan returns one branch's sales in the period for the Report
// Pendapatan PDF, oldest first, with this request's user as Petugas.
//
// Query: cabang (required), from, to (YYYY-MM-DD, default today), pelayanan
// (refjenisrawat id), dokter (refmdokter id).
func (h *Handler) Pendapatan(w http.ResponseWriter, r *http.Request) {
	cabang, ok := h.oneBranch(w, r)
	if !ok {
		return
	}
	q := r.URL.Query()
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
	res, err := h.repo.Pendapatan(r.Context(), PendapatanParams{
		Cabang: cabang, From: from, To: to, IDJenisRawat: optInt(r, "pelayanan"), IDDokter: optInt(r, "dokter"),
	})
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	id, _ := auth.IdentityFrom(r.Context())
	petugas := id.Email
	if at := strings.IndexByte(petugas, '@'); at > 0 {
		petugas = petugas[:at] // printed as "(ato)", like the legacy user names
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"cabang": res.Cabang, "namaCabang": res.NamaCabang, "telpCabang": res.TelpCabang, "rows": res.Rows,
		"from": from, "to": to, "today": pendaftaran.ClinicToday(h.now()), "petugas": petugas,
	})
}

// PendapatanOptions returns the services (Layanan), the caller's branches and today.
func (h *Handler) PendapatanOptions(w http.ResponseWriter, r *http.Request) {
	pelayanan, err := h.repo.JenisRawat(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"pelayanan": pelayanan, "branches": branches, "today": pendaftaran.ClinicToday(h.now())})
}

// PendapatanDokter lists the doctors of the chosen branch for the Dokter dropdown.
func (h *Handler) PendapatanDokter(w http.ResponseWriter, r *http.Request) {
	cabang, ok := h.oneBranch(w, r)
	if !ok {
		return
	}
	list, err := h.repo.DokterCabang(r.Context(), cabang)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": list})
}

// ReportDiagnosa returns one page of the diagnoses of the sales at the
// caller's branches, newest sale first.
//
// Query: from, to (YYYY-MM-DD, default today), cabang, page, size (1..5000),
// q, sort (idJual, tgl, noRm, pasien, tglLahir, usia, jenisKelamin, dokter,
// jenisRawat, icd, diagnosa, cabang), dir, nama (patient), diagnosa (ICD code
// or name), pelayanan (refjenisrawat id).
func (h *Handler) ReportDiagnosa(w http.ResponseWriter, r *http.Request) {
	codes, from, to, ok := h.period(w, r)
	if !ok {
		return
	}
	if len(codes) == 0 {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": []DiagnosaRow{}, "total": 0, "unfilteredTotal": 0, "from": from, "to": to})
		return
	}
	q := r.URL.Query()
	size := intParam(r, "size", 5, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") != "asc"
	if !DiagnosaSortable(sort) {
		sort, desc = "idJual", true // newest sale first, like the legacy report
	}
	repoDesc := desc
	if sort == "usia" {
		repoDesc = !desc // age sorts by birth date the other way round
	}
	res, err := h.repo.ReportDiagnosa(r.Context(), DiagnosaParams{
		Branches: codes, From: from, To: to,
		Search: strings.TrimSpace(q.Get("q")), Nama: strings.TrimSpace(q.Get("nama")), Diagnosa: strings.TrimSpace(q.Get("diagnosa")),
		IDJenisRawat: optInt(r, "pelayanan"), Sort: sort, Desc: repoDesc, Limit: size, Offset: (page - 1) * size,
	})
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"rows": res.Rows, "total": res.Total, "unfilteredTotal": res.UnfilteredTotal, "from": from, "to": to,
	})
}

// ReportDinkes returns one page of the cases per ICD code and service at the
// caller's branches (usually one, from the toolbar's Cabang), by age group and
// sex, highest code first.
//
// Query: from, to (YYYY-MM-DD, default today), cabang, page, size (1..5000),
// q (ICD code or name, service), sort (kode, icd, jenisRawat, hari0_7,
// hari8_28, bulan1_11, tahun1_4, tahun5_9, tahun10_14, tahun15_19, tahun20_44,
// tahun45_59, tahun60, laki, perempuan, total), dir.
func (h *Handler) ReportDinkes(w http.ResponseWriter, r *http.Request) {
	codes, from, to, ok := h.period(w, r)
	if !ok {
		return
	}
	if len(codes) == 0 {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": []DinkesRow{}, "total": 0, "unfilteredTotal": 0, "from": from, "to": to})
		return
	}
	q := r.URL.Query()
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") != "asc"
	if !DinkesSortable(sort) {
		sort, desc = "kode", true // highest code first, like the legacy report
	}
	res, err := h.repo.ReportDinkes(r.Context(), DinkesParams{
		Branches: codes, From: from, To: to, Search: strings.TrimSpace(q.Get("q")),
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

// StokOpname returns one page of drugs per branch at the caller's branches with
// their stock, minimum stock and prices, by drug id.
//
// Query: cabang, page, size (1..5000), q, sort (idObat, nama, hargaJual,
// hargaBeli, beratIsi, jenis, satuanBeli, satuanJual, jumUnit, stokMin, stok,
// cabang), dir, nama, jenis (refjenisobat id), aktif (1 or 0).
func (h *Handler) StokOpname(w http.ResponseWriter, r *http.Request) {
	h.stokList(w, r, false, "idObat")
}

// MinimStok is Stok Alert's "Obat Minim Stok" tab: the Stok Opname rows whose
// stock is below the minimum (trxstock.stok_min > stok), by drug name. Same
// query parameters as StokOpname.
func (h *Handler) MinimStok(w http.ResponseWriter, r *http.Request) {
	h.stokList(w, r, true, "nama")
}

func (h *Handler) stokList(w http.ResponseWriter, r *http.Request, belowMin bool, defaultSort string) {
	codes, _, _, ok := h.period(w, r)
	if !ok {
		return
	}
	if len(codes) == 0 {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": []OpnameRow{}, "total": 0, "unfilteredTotal": 0})
		return
	}
	q := r.URL.Query()
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") == "desc"
	if !OpnameSortable(sort) {
		sort, desc = defaultSort, false
	}
	var aktif *bool
	switch q.Get("aktif") {
	case "1":
		v := true
		aktif = &v
	case "0":
		v := false
		aktif = &v
	}
	res, err := h.repo.StokOpname(r.Context(), OpnameParams{
		Branches: codes, Search: strings.TrimSpace(q.Get("q")), Nama: strings.TrimSpace(q.Get("nama")), IDJenis: optInt(r, "jenis"), Aktif: aktif, BelowMin: belowMin,
		Sort: sort, Desc: desc, Limit: size, Offset: (page - 1) * size,
	})
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": res.Rows, "total": res.Total, "unfilteredTotal": res.UnfilteredTotal})
}

// StokOpnameOptions returns the drug types and the caller's branches.
func (h *Handler) StokOpnameOptions(w http.ResponseWriter, r *http.Request) {
	jenis, err := h.repo.JenisObat(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"jenis": jenis, "branches": branches})
}

// StokTraceObat lists the drugs stocked at one of the caller's branches for
// Stok Trace's "Obat" lookup, by id. Query: cabang (required).
func (h *Handler) StokTraceObat(w http.ResponseWriter, r *http.Request) {
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
	list, err := h.repo.ObatCabang(r.Context(), c)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": list})
}

// StokTrace returns one page of the stock movements (trxobat) at one of the
// caller's branches in the period, oldest first, of one drug or of all.
//
// Query: cabang (required), obat (refobat id; empty or 0 = every drug), from,
// to (YYYY-MM-DD of createddate, default today), page, size (1..5000), q,
// sort (id, createdDate, nama, lastStock, inOut, byk, stock, createdBy,
// transaksi, remarks, cabang), dir.
func (h *Handler) StokTrace(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	c := strings.ToUpper(strings.TrimSpace(q.Get("cabang")))
	if c == "" {
		httpx.WriteValidation(w, httpx.FieldErrors{"cabang": "Pilih Cabang"})
		return
	}
	codes, from, to, ok := h.period(w, r) // checks the branch is the caller's, and the dates
	if !ok {
		return
	}
	if len(codes) != 1 {
		httpx.WriteError(w, http.StatusForbidden, "you are not assigned to branch "+c)
		return
	}
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	sort := q.Get("sort")
	desc := q.Get("dir") == "desc"
	if !TraceSortable(sort) {
		sort, desc = "id", false // in the order they happened, like the legacy page
	}
	res, err := h.repo.StokTrace(r.Context(), TraceParams{
		Cabang: codes[0], IDObat: optInt(r, "obat"), From: from, To: to,
		Search: strings.TrimSpace(q.Get("q")), Sort: sort, Desc: desc, Limit: size, Offset: (page - 1) * size,
	})
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"rows": res.Rows, "total": res.Total, "unfilteredTotal": res.UnfilteredTotal, "from": from, "to": to,
	})
}
