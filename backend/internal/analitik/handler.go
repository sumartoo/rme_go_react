package analitik

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
	"github.com/example/be_jwt_golang/backend/internal/pendaftaran"
)

// PagePath is the menu item that grants access to these endpoints
// (seeded by 055_analitik_menu.sql).
const PagePath = "/stok-farmasi/analitik"

const (
	// HistoryDays is the usage history behind the drug forecasts and the
	// window of the diagnosis-drug associations.
	HistoryDays = 90
	// DefaultHorizon and DefaultLeadTime are the forecast horizon and the
	// days an order takes to arrive.
	DefaultHorizon  = 30
	DefaultLeadTime = 7
	DefaultWeeks    = 12
	DefaultTop      = 10
	// Chat limits: turns kept, characters per question, and characters kept of
	// each earlier answer (longer answers are cut, not rejected).
	MaxChatMessages = 20
	MaxChatChars    = 2000
	MaxAnswerChars  = 6000
)

type Handler struct {
	repo Repository
	ai   Assistant // nil: the AI endpoints answer 503
	now  func() time.Time

	mu    sync.Mutex
	cache map[string]string // ringkasan per clinic day and branches
}

// NewHandler serves the analytics; ai may be nil when no API key is set.
func NewHandler(repo Repository, ai Assistant) *Handler {
	return &Handler{repo: repo, ai: ai, now: time.Now, cache: map[string]string{}}
}

// Routes registers the endpoints for callers whose role can open PagePath.
func (h *Handler) Routes(mux *http.ServeMux, requireAuth func(http.Handler) http.Handler, requireMenu func(string) func(http.Handler) http.Handler) {
	guard := func(f http.HandlerFunc) http.Handler { return requireAuth(requireMenu(PagePath)(f)) }
	mux.Handle("GET /api/analitik/options", guard(h.Options))
	mux.Handle("GET /api/analitik/obat", guard(h.Obat))
	mux.Handle("GET /api/analitik/penyakit", guard(h.Penyakit))
	mux.Handle("GET /api/analitik/diagnosa-obat", guard(h.DiagnosaObat))
	mux.Handle("POST /api/analitik/ringkasan", guard(h.Ringkasan))
	mux.Handle("POST /api/analitik/chat", guard(h.Chat))
	mux.Handle("GET /api/analitik/command", guard(h.Command))
	mux.Handle("GET /api/analitik/kinerja", guard(h.Kinerja))
	mux.Handle("GET /api/analitik/kualitas-data", guard(h.KualitasData))
	mux.Handle("GET /api/analitik/follow-up", guard(h.FollowUpHandler))
	// ICD suggestions serve the Pengobatan form, so they follow its menu item.
	mux.Handle("POST /api/analitik/saran-icd", requireAuth(requireMenu(PengobatanPagePath)(http.HandlerFunc(h.SaranICD))))
}

func (h *Handler) branches(r *http.Request) ([]branchauth.Branch, error) {
	id, _ := auth.IdentityFrom(r.Context())
	list, err := h.repo.UserBranches(r.Context(), id.UserID)
	if list == nil {
		list = []branchauth.Branch{}
	}
	return list, err
}

// scope resolves the cabang parameter to branch codes the caller may read:
// one branch, or all of theirs when empty. It writes the error itself.
func (h *Handler) scope(w http.ResponseWriter, r *http.Request, cabang string) ([]string, string, bool) {
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return nil, "", false
	}
	c := strings.ToUpper(strings.TrimSpace(cabang))
	if c == "" {
		names := make([]string, len(branches))
		for i, b := range branches {
			names[i] = b.Nama
		}
		return branchauth.Codes(branches), "semua cabang (" + strings.Join(names, ", ") + ")", true
	}
	for _, b := range branches {
		if b.Kode == c {
			return []string{c}, b.Nama + " (" + c + ")", true
		}
	}
	httpx.WriteError(w, http.StatusForbidden, "you are not assigned to branch "+c)
	return nil, "", false
}

func intParam(r *http.Request, name string, def, lo, hi int) int {
	v, err := strconv.Atoi(r.URL.Query().Get(name))
	if err != nil {
		return def
	}
	return min(max(v, lo), hi)
}

func clamp(v *int, def, lo, hi int) int {
	if v == nil {
		return def
	}
	return min(max(*v, lo), hi)
}

func (h *Handler) today() string { return pendaftaran.ClinicToday(h.now()) }

// Options returns the caller's branches, today's clinic date, the defaults
// and whether the AI assistant is configured.
func (h *Handler) Options(w http.ResponseWriter, r *http.Request) {
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"branches": branches, "today": h.today(), "aiEnabled": h.ai != nil,
		"historyDays": HistoryDays, "horizon": DefaultHorizon, "leadTime": DefaultLeadTime, "weeks": DefaultWeeks,
	})
}

// ObatResult is the drug forecast of the caller's branches.
type ObatResult struct {
	Rows     []ObatPrediksi `json:"rows"`
	Total    int            `json:"total"` // rows before limit
	From     string         `json:"from"`  // history window
	To       string         `json:"to"`
	Horizon  int            `json:"horizon"`
	LeadTime int            `json:"leadTime"`
	Ringkas  map[string]int `json:"ringkas"` // rows per status
	// BiayaOrder sums estimasiBiaya of the drugs to order (habis, kritis,
	// perlu-order); OrderTanpaHarga counts those without a plausible cost.
	BiayaOrder      float64 `json:"biayaOrder"`
	OrderTanpaHarga int     `json:"orderTanpaHarga"`
}

func (h *Handler) prediksi(ctx context.Context, codes []string, horizon, leadTime int, nama, status string, limit int) (ObatResult, error) {
	to := h.today()
	from := addDays(to, -(HistoryDays - 1))
	res := ObatResult{Rows: []ObatPrediksi{}, From: from, To: to, Horizon: horizon, LeadTime: leadTime, Ringkas: map[string]int{}}
	if len(codes) == 0 {
		return res, nil
	}
	usages, err := h.repo.Usage(ctx, UsageParams{Branches: codes, From: from, To: to, Search: nama})
	if err != nil {
		return res, err
	}
	for _, p := range Predict(usages, horizon, leadTime) {
		res.Ringkas[p.Status]++
		if p.SaranOrder > 0 && p.Status != StatusAman && p.Status != StatusDiam {
			if p.EstimasiBiaya != nil {
				res.BiayaOrder += *p.EstimasiBiaya
			} else {
				res.OrderTanpaHarga++
			}
		}
		if status != "" && p.Status != status {
			continue
		}
		res.Total++
		if limit <= 0 || len(res.Rows) < limit {
			res.Rows = append(res.Rows, p)
		}
	}
	res.BiayaOrder = round2(res.BiayaOrder)
	return res, nil
}

// Obat forecasts the usage of every active drug at the caller's branches,
// most urgent first.
//
// Query: cabang (default all of the caller's), horizon (7..90 days, default
// 30), leadTime (1..60 days, default 7), q (drug name), status.
func (h *Handler) Obat(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	codes, _, ok := h.scope(w, r, q.Get("cabang"))
	if !ok {
		return
	}
	res, err := h.prediksi(r.Context(), codes,
		intParam(r, "horizon", DefaultHorizon, 7, 90), intParam(r, "leadTime", DefaultLeadTime, 1, 60),
		strings.TrimSpace(q.Get("q")), strings.TrimSpace(q.Get("status")), 0)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) penyakit(ctx context.Context, codes []string, weeks, top int) (Penyakit, error) {
	today := h.today()
	if len(codes) == 0 {
		return BuildPenyakit(nil, today, weeks, top), nil
	}
	series, err := h.repo.IcdWeekly(ctx, codes, today, weeks)
	if err != nil {
		return Penyakit{}, err
	}
	return BuildPenyakit(series, today, weeks, top), nil
}

// Penyakit returns the weekly disease trend of the caller's branches.
//
// Query: cabang, weeks (4..26, default 12), top (1..20, default 10).
func (h *Handler) Penyakit(w http.ResponseWriter, r *http.Request) {
	codes, _, ok := h.scope(w, r, r.URL.Query().Get("cabang"))
	if !ok {
		return
	}
	res, err := h.penyakit(r.Context(), codes, intParam(r, "weeks", DefaultWeeks, 4, 26), intParam(r, "top", DefaultTop, 1, 20))
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// DiagnosaObatRow is a diagnosis with its prescriptions and trend.
type DiagnosaObatRow struct {
	IcdObat
	ProyeksiMinggu float64 `json:"proyeksiMinggu"`
	Lonjakan       bool    `json:"lonjakan"`
}

// DiagnosaObatResult maps diagnoses to drugs and the stock they call for.
type DiagnosaObatResult struct {
	Diagnosa []DiagnosaObatRow `json:"diagnosa"`
	Saran    []SaranObat       `json:"saran"`
	From     string            `json:"from"`
	To       string            `json:"to"`
	Horizon  int               `json:"horizon"`
}

func (h *Handler) diagnosaObat(ctx context.Context, codes []string, icd string, horizon int) (DiagnosaObatResult, error) {
	to := h.today()
	from := addDays(to, -(HistoryDays - 1))
	res := DiagnosaObatResult{Diagnosa: []DiagnosaObatRow{}, Saran: []SaranObat{}, From: from, To: to, Horizon: horizon}
	if len(codes) == 0 {
		return res, nil
	}
	resep, err := h.repo.Resep(ctx, ResepParams{Branches: codes, From: from, To: to, ICD: icd, LimitIcd: 15, LimitObat: 10})
	if err != nil {
		return res, err
	}
	series, err := h.repo.IcdWeekly(ctx, codes, to, DefaultWeeks)
	if err != nil {
		return res, err
	}
	trends := map[int]Trend{}
	for _, s := range series {
		trends[s.IDIcd] = WeeklyTrend(s.Weeks)
	}
	ids := []int{}
	seen := map[int]bool{}
	for _, d := range resep {
		t := trends[d.IDIcd]
		res.Diagnosa = append(res.Diagnosa, DiagnosaObatRow{IcdObat: d, ProyeksiMinggu: t.ProyeksiMinggu, Lonjakan: t.Lonjakan})
		for _, o := range d.Obat {
			if !seen[o.IDObat] {
				seen[o.IDObat] = true
				ids = append(ids, o.IDObat)
			}
		}
	}
	stok, err := h.repo.Stok(ctx, codes, ids)
	if err != nil {
		return res, err
	}
	res.Saran = SaranStok(resep, trends, horizon, stok)
	return res, nil
}

// DiagnosaObat lists the drugs prescribed with the most frequent diagnoses
// (or those matching icd) and the stock their projected cases call for.
//
// Query: cabang, icd (code or name), horizon (7..90 days, default 30).
func (h *Handler) DiagnosaObat(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	codes, _, ok := h.scope(w, r, q.Get("cabang"))
	if !ok {
		return
	}
	res, err := h.diagnosaObat(r.Context(), codes, strings.TrimSpace(q.Get("icd")), intParam(r, "horizon", DefaultHorizon, 7, 90))
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

func (h *Handler) aiReady(w http.ResponseWriter) bool {
	if h.ai == nil {
		httpx.WriteError(w, http.StatusServiceUnavailable, "AI belum dikonfigurasi (DEEPSEEK_API_KEY kosong)")
		return false
	}
	return true
}

func (h *Handler) aiError(w http.ResponseWriter, err error) {
	if errors.Is(err, ErrRefused) {
		httpx.WriteError(w, http.StatusUnprocessableEntity, err.Error())
		return
	}
	log.Printf("analitik: AI request failed: %v", err)
	httpx.WriteError(w, http.StatusBadGateway, "Layanan AI sedang tidak dapat dihubungi, coba lagi nanti")
}

func konteks(today, label string) string {
	return "Tanggal hari ini: " + today + ". Cabang yang dianalisa: " + label + "."
}

// Ringkasan asks the assistant for an executive summary of the current
// numbers. The summary is kept for the clinic day; refresh asks again.
//
// Body: {cabang, refresh}.
func (h *Handler) Ringkasan(w http.ResponseWriter, r *http.Request) {
	if !h.aiReady(w) {
		return
	}
	var in struct {
		Cabang  string `json:"cabang"`
		Refresh bool   `json:"refresh"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	codes, label, ok := h.scope(w, r, in.Cabang)
	if !ok {
		return
	}
	if len(codes) == 0 {
		httpx.WriteError(w, http.StatusForbidden, "you are not assigned to any branch")
		return
	}
	today := h.today()
	key := today + "|" + strings.Join(codes, ",")
	h.mu.Lock()
	cached, hit := h.cache[key]
	h.mu.Unlock()
	if hit && !in.Refresh {
		httpx.WriteJSON(w, http.StatusOK, map[string]any{"text": cached, "cached": true, "today": today})
		return
	}
	data, err := h.ringkasanData(r.Context(), codes)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	text, err := h.ai.Ringkasan(r.Context(), konteks(today, label), data)
	if err != nil {
		h.aiError(w, err)
		return
	}
	h.mu.Lock()
	h.cache[key] = text
	h.mu.Unlock()
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"text": text, "cached": false, "today": today})
}

// ringkasanData is the JSON the summary is written from: the most urgent
// drugs, the disease trend and the drugs the trend calls for.
func (h *Handler) ringkasanData(ctx context.Context, codes []string) (string, error) {
	obat, err := h.prediksi(ctx, codes, DefaultHorizon, DefaultLeadTime, "", "", 0)
	if err != nil {
		return "", err
	}
	urgent := []ObatPrediksi{}
	for _, p := range obat.Rows {
		if p.Status == StatusHabis || p.Status == StatusKritis || p.Status == StatusOrder {
			urgent = append(urgent, p)
		}
		if len(urgent) == 25 {
			break
		}
	}
	sakit, err := h.penyakit(ctx, codes, DefaultWeeks, DefaultTop)
	if err != nil {
		return "", err
	}
	do, err := h.diagnosaObat(ctx, codes, "", DefaultHorizon)
	if err != nil {
		return "", err
	}
	cmd, err := h.command(ctx, codes)
	if err != nil {
		return "", err
	}
	b, err := json.Marshal(map[string]any{
		"kpi":               cmd.KPI,
		"insight":           cmd.Insights,
		"kinerjaMingguan":   cmd.Mingguan,
		"prediksiObat":      map[string]any{"jumlahPerStatus": obat.Ringkas, "palingMendesak": urgent, "horizonHari": DefaultHorizon, "leadTimeHari": DefaultLeadTime, "riwayat": obat.From + " s/d " + obat.To, "biayaOrder": obat.BiayaOrder, "orderTanpaHarga": obat.OrderTanpaHarga},
		"trenPenyakit":      sakit,
		"saranStokDariTren": do.Saran[:min(15, len(do.Saran))],
	})
	return string(b), err
}

// Chat answers the last user message of the conversation; the assistant
// reads the analytics of the caller's branches through its tools.
//
// Body: {cabang, messages: [{role: user|assistant, content}]}, ending with a
// user message.
func (h *Handler) Chat(w http.ResponseWriter, r *http.Request) {
	if !h.aiReady(w) {
		return
	}
	var in struct {
		Cabang   string        `json:"cabang"`
		Messages []ChatMessage `json:"messages"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if len(in.Messages) > MaxChatMessages {
		in.Messages = in.Messages[len(in.Messages)-MaxChatMessages:]
	}
	// The conversation must alternate, starting and ending with the user.
	for len(in.Messages) > 0 && in.Messages[0].Role != "user" {
		in.Messages = in.Messages[1:]
	}
	if len(in.Messages) == 0 || in.Messages[len(in.Messages)-1].Role != "user" {
		httpx.WriteValidation(w, httpx.FieldErrors{"messages": "Tulis pertanyaan"})
		return
	}
	for i, m := range in.Messages {
		want := "user"
		if i%2 == 1 {
			want = "assistant"
		}
		m.Content = strings.TrimSpace(m.Content)
		if m.Role == "assistant" {
			if r := []rune(m.Content); len(r) > MaxAnswerChars {
				m.Content = string(r[:MaxAnswerChars]) + " …"
			}
		}
		if m.Role != want || m.Content == "" || (m.Role == "user" && utf8.RuneCountInString(m.Content) > MaxChatChars) {
			httpx.WriteValidation(w, httpx.FieldErrors{"messages": "Pesan tidak valid (pertanyaan maksimal 2000 karakter)"})
			return
		}
		in.Messages[i] = m
	}
	codes, label, ok := h.scope(w, r, in.Cabang)
	if !ok {
		return
	}
	if len(codes) == 0 {
		httpx.WriteError(w, http.StatusForbidden, "you are not assigned to any branch")
		return
	}
	text, err := h.ai.Chat(r.Context(), konteks(h.today(), label), in.Messages, h.toolRunner(codes))
	if err != nil {
		h.aiError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"message": ChatMessage{Role: "assistant", Content: text}})
}

// toolRunner runs the assistant's tools, always on codes (the branches the
// caller chose and may read), whatever the model asks for.
func (h *Handler) toolRunner(codes []string) ToolFunc {
	return func(ctx context.Context, name string, input json.RawMessage) (string, error) {
		var in struct {
			Nama     string `json:"nama"`
			Status   string `json:"status"`
			Horizon  *int   `json:"horizon"`
			Limit    *int   `json:"limit"`
			Weeks    *int   `json:"weeks"`
			Top      *int   `json:"top"`
			ICD      string `json:"icd"`
			Dari     string `json:"dari"`
			Sampai   string `json:"sampai"`
			Kelompok string `json:"kelompok"`
			Urut     string `json:"urut"`
		}
		if len(input) > 0 {
			if err := json.Unmarshal(input, &in); err != nil {
				return "", errors.New("input tidak valid")
			}
		}
		var out any
		var err error
		switch name {
		case ToolPrediksiObat:
			out, err = h.prediksi(ctx, codes, clamp(in.Horizon, DefaultHorizon, 7, 90), DefaultLeadTime,
				strings.TrimSpace(in.Nama), strings.TrimSpace(in.Status), clamp(in.Limit, 20, 1, 50))
		case ToolTrenPenyakit:
			out, err = h.penyakit(ctx, codes, clamp(in.Weeks, DefaultWeeks, 4, 26), clamp(in.Top, DefaultTop, 1, 20))
		case ToolDiagnosaObat:
			var res DiagnosaObatResult
			res, err = h.diagnosaObat(ctx, codes, strings.TrimSpace(in.ICD), clamp(in.Horizon, DefaultHorizon, 7, 90))
			res.Saran = res.Saran[:min(20, len(res.Saran))]
			out = res
		case ToolKinerja, ToolObatTerlaris, ToolTindakanTerlaris:
			dari, sampai, msg := h.periode(strings.TrimSpace(in.Dari), strings.TrimSpace(in.Sampai), 30)
			if msg != "" {
				return "", errors.New(msg)
			}
			switch name {
			case ToolKinerja:
				kelompok := strings.TrimSpace(in.Kelompok)
				if kelompok == "" {
					kelompok = "bulan"
				}
				if !ValidKelompok(kelompok) {
					return "", errors.New("kelompok tidak dikenal: " + kelompok)
				}
				var res KinerjaResult
				res, err = h.kinerja(ctx, codes, dari, sampai, kelompok)
				res.Rows = res.Rows[:min(60, len(res.Rows))]
				out = res
			case ToolObatTerlaris:
				var rows []Terlaris
				rows, err = h.repo.ObatTerlaris(ctx, codes, dari, sampai, in.Urut == "nilai", clamp(in.Limit, 15, 1, 50))
				out = map[string]any{"dari": dari, "sampai": sampai, "urut": in.Urut, "rows": rows}
			default:
				var rows []Terlaris
				rows, err = h.repo.TindakanTerlaris(ctx, codes, dari, sampai, in.Urut == "nilai", clamp(in.Limit, 15, 1, 50))
				out = map[string]any{"dari": dari, "sampai": sampai, "urut": in.Urut, "rows": rows}
			}
		default:
			return "", errors.New("tool tidak dikenal: " + name)
		}
		if err != nil {
			log.Printf("analitik: tool %s: %v", name, err)
			return "", errors.New("gagal membaca data")
		}
		b, err := json.Marshal(out)
		return string(b), err
	}
}
