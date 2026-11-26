package analitik

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"math"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/icd"
)

const (
	csID   = "22222222-2222-2222-2222-222222222222" // mapped to KMJ and MED
	loneID = "33333333-3333-3333-3333-333333333333" // mapped to no branch
)

// 23:30 UTC on 24 Nov is already 25 Nov in Jakarta.
var now = time.Date(2026, 11, 24, 23, 30, 0, 0, time.UTC)

func flat(n int, v float64) []float64 {
	s := make([]float64, n)
	for i := range s {
		s[i] = v
	}
	return s
}

func TestDailyRate(t *testing.T) {
	r := DailyRate(flat(60, 4))
	if math.Abs(r.AvgDaily-4) > 1e-9 || r.StdDev != 0 || r.Days != 60 || r.LowData {
		t.Fatalf("steady 4/day: %+v", r)
	}
	if r := DailyRate(nil); r.AvgDaily != 0 || !r.LowData {
		t.Errorf("empty: %+v", r)
	}
	// Usage that rose from 2 to 10 a day: the rate follows the recent days.
	rising := append(flat(40, 2), flat(20, 10)...)
	if r := DailyRate(rising); r.AvgDaily < 8 || r.AvgDaily > 10 {
		t.Errorf("rising usage weighs recent days: %+v", r)
	}
	// A day with more cancelled than sold counts as zero.
	if r := DailyRate([]float64{-5, 0, 0}); r.AvgDaily != 0 || !r.LowData {
		t.Errorf("negative day: %+v", r)
	}
}

func TestPlanStock(t *testing.T) {
	steady := Rate{AvgDaily: 2, StdDev: 1, Days: 60}
	p := PlanStock(100, steady, 30, 7)
	// need 60, safety 1.65*1*sqrt(7)=4.37, stock 100: lasts 50 days.
	if p.Status != StatusAman || p.SaranOrder != 0 || p.Kebutuhan != 60 || p.SafetyStock != 4.37 || *p.HabisDalamHari != 50 {
		t.Errorf("enough stock: %+v", p)
	}
	if p = PlanStock(40, steady, 30, 7); p.Status != StatusOrder || p.SaranOrder != 25 { // ceil(60+4.37-40)
		t.Errorf("short of the horizon: %+v", p)
	}
	if p = PlanStock(10, steady, 30, 7); p.Status != StatusKritis || *p.HabisDalamHari != 5 {
		t.Errorf("runs out before an order arrives: %+v", p)
	}
	if p = PlanStock(-3, steady, 30, 7); p.Status != StatusHabis || *p.HabisDalamHari != 0 || p.SaranOrder != 65 {
		t.Errorf("no stock: %+v", p)
	}
	if p = PlanStock(10, Rate{Days: 60}, 30, 7); p.Status != StatusDiam || p.HabisDalamHari != nil || p.SaranOrder != 0 {
		t.Errorf("unused: %+v", p)
	}
}

func TestPredictSortsMostUrgentFirst(t *testing.T) {
	rows := Predict([]Usage{
		{IDObat: 1, Nama: "AMAN", Cabang: "KMJ", Stok: 1000, Series: flat(60, 1)},
		{IDObat: 2, Nama: "DIAM", Cabang: "KMJ", Stok: 5, Series: flat(60, 0)},
		{IDObat: 3, Nama: "HABIS", Cabang: "KMJ", Stok: 0, Series: flat(60, 1)},
		{IDObat: 4, Nama: "KRITIS", Cabang: "KMJ", Stok: 3, Series: flat(60, 1)},
		{IDObat: 5, Nama: "ORDER", Cabang: "KMJ", Stok: 20, Series: flat(60, 1)},
	}, 30, 7)
	var got []string
	for _, r := range rows {
		got = append(got, r.Nama)
	}
	if strings.Join(got, ",") != "HABIS,KRITIS,ORDER,AMAN,DIAM" {
		t.Errorf("order: %v", got)
	}
	if rows[0].Terpakai != 60 || rows[0].Hari != 60 {
		t.Errorf("used: %+v", rows[0])
	}
	// Among drugs out of stock, the most used comes first.
	rows = Predict([]Usage{
		{IDObat: 1, Nama: "A JARANG", Stok: 0, Series: flat(60, 0.1)},
		{IDObat: 2, Nama: "B LARIS", Stok: 0, Series: flat(60, 3)},
	}, 30, 7)
	if rows[0].Nama != "B LARIS" {
		t.Errorf("out of stock order: %s first", rows[0].Nama)
	}
}

func TestWeeklyTrendAndSpike(t *testing.T) {
	tr := WeeklyTrend([]int{4, 5, 4, 3, 4, 12})
	if tr.Total != 32 || tr.MingguIni != 12 || tr.RataSebelum != 4 || tr.Rasio != 3 || !tr.Lonjakan {
		t.Errorf("spike: %+v", tr)
	}
	if tr.ProyeksiMinggu <= 4 || tr.ProyeksiMinggu >= 12 {
		t.Errorf("projection between baseline and spike: %+v", tr)
	}
	for _, c := range []struct {
		week     int
		baseline float64
		want     bool
	}{
		{4, 0, false},   // too few cases
		{6, 4.5, false}, // not 1.5×
		{6, 2, true},
		{5, 2.5, false}, // 2× but only 2.5 more
	} {
		if got := IsSpike(c.week, c.baseline); got != c.want {
			t.Errorf("IsSpike(%d, %v) = %v", c.week, c.baseline, got)
		}
	}
	if tr := WeeklyTrend(nil); tr.Total != 0 || tr.Lonjakan {
		t.Errorf("empty: %+v", tr)
	}
}

func TestBuildPenyakit(t *testing.T) {
	p := BuildPenyakit([]IcdSeries{
		{IDIcd: 1, Kode: "J06.9", Nama: "ISPA", Weeks: []int{10, 10, 10, 10}},
		{IDIcd: 2, Kode: "A90", Nama: "DBD", Weeks: []int{1, 1, 1, 9}},
		{IDIcd: 3, Kode: "K30", Nama: "Dispepsia", Weeks: []int{2, 2, 2, 2}},
	}, "2026-11-25", 4, 2)
	if len(p.Top) != 2 || p.Top[0].Kode != "J06.9" || p.Top[1].Kode != "A90" {
		t.Errorf("top: %+v", p.Top)
	}
	if len(p.Lonjakan) != 1 || p.Lonjakan[0].Kode != "A90" {
		t.Errorf("spikes: %+v", p.Lonjakan)
	}
	if strings.Join(p.WeekStarts, ",") != "2026-10-29,2026-11-05,2026-11-12,2026-11-19" {
		t.Errorf("week starts: %v", p.WeekStarts)
	}
	if p.Kunjungan[3] != 21 {
		t.Errorf("visits: %v", p.Kunjungan)
	}
}

func TestSaranStok(t *testing.T) {
	resep := []IcdObat{
		{IDIcd: 1, Kode: "J06.9", Kunjungan: 40, Obat: []ObatResep{
			{IDObat: 10, Nama: "PARASETAMOL", QtyPerKasus: 10},
			{IDObat: 11, Nama: "AMBROXOL", QtyPerKasus: 5},
		}},
		{IDIcd: 2, Kode: "A90", Kunjungan: 4, Obat: []ObatResep{{IDObat: 10, Nama: "PARASETAMOL", QtyPerKasus: 10}}},
		{IDIcd: 3, Kode: "K30", Obat: []ObatResep{{IDObat: 12, Nama: "ANTASIDA", QtyPerKasus: 3}}}, // no trend: skipped
	}
	trends := map[int]Trend{1: {ProyeksiMinggu: 7}, 2: {ProyeksiMinggu: 1.4}}
	s := SaranStok(resep, trends, 14, map[int]float64{10: 100, 11: 500})
	// PARASETAMOL: (14 + 2.8 cases) × 10 = 168, stock 100 → 68 short.
	if len(s) != 2 || s[0].IDObat != 10 || s[0].Kebutuhan != 168 || s[0].Kurang != 68 || strings.Join(s[0].Dari, ",") != "J06.9,A90" {
		t.Fatalf("saran: %+v", s)
	}
	if s[1].IDObat != 11 || s[1].Kebutuhan != 70 || s[1].Kurang != 0 {
		t.Errorf("enough stock: %+v", s[1])
	}
}

// ---------- handler ----------

type fakeRepo struct {
	branches  map[string][]branchauth.Branch
	usage     UsageParams
	weeklyFor []string
	weeks     int
	resep     ResepParams
	kinerja   []KinerjaParams
	kualitas  []string
	followup  FollowUpParams
	icdCodes  []string
	searched  []string
}

func (f *fakeRepo) UserBranches(_ context.Context, userID string) ([]branchauth.Branch, error) {
	return f.branches[userID], nil
}
func (f *fakeRepo) Usage(_ context.Context, p UsageParams) ([]Usage, error) {
	f.usage = p
	return []Usage{
		{IDObat: 1, Nama: "AMOXICILLIN 500", Satuan: "TAB", Cabang: "KMJ", Stok: 5, Series: flat(60, 2)},
		{IDObat: 2, Nama: "VITAMIN C", Satuan: "TAB", Cabang: "KMJ", Stok: 900, Series: flat(60, 1)},
	}, nil
}
func (f *fakeRepo) IcdWeekly(_ context.Context, branches []string, _ string, weeks int) ([]IcdSeries, error) {
	f.weeklyFor, f.weeks = branches, weeks
	w := make([]int, weeks)
	w[weeks-1] = 10
	return []IcdSeries{{IDIcd: 1, Kode: "J06.9", Nama: "ISPA", Weeks: w}}, nil
}
func (f *fakeRepo) Resep(_ context.Context, p ResepParams) ([]IcdObat, error) {
	f.resep = p
	return []IcdObat{{IDIcd: 1, Kode: "J06.9", Nama: "ISPA", Kunjungan: 10, Obat: []ObatResep{{IDObat: 1, Nama: "AMOXICILLIN 500", Kunjungan: 8, Qty: 80, Support: 0.8, QtyPerKasus: 8}}}}, nil
}
func (f *fakeRepo) Stok(_ context.Context, _ []string, ids []int) (map[int]float64, error) {
	return map[int]float64{1: 5}, nil
}
func (f *fakeRepo) Kinerja(_ context.Context, p KinerjaParams) ([]KinerjaRow, error) {
	f.kinerja = append(f.kinerja, p)
	if p.Kelompok == "tanggal" {
		// 10 sales a day for the last 7 days, 20 a day the 7 before; Rp1jt a day.
		var rows []KinerjaRow
		for i := 0; i < 14; i++ {
			n := 10
			if i >= 7 {
				n = 20
			}
			rows = append(rows, KinerjaRow{Kelompok: addDays("2026-11-25", -i), Transaksi: n, Pendapatan: 1_000_000})
		}
		return rows, nil
	}
	return []KinerjaRow{{Kelompok: "2026-11-23", Transaksi: 70, Kunjungan: 60, Pasien: 55, Pendapatan: 7_000_000}}, nil
}
func (f *fakeRepo) ObatTerlaris(_ context.Context, _ []string, from, to string, byNilai bool, limit int) ([]Terlaris, error) {
	return []Terlaris{{Nama: "PARASETAMOL", Jumlah: 100, Nilai: 50000}}, nil
}
func (f *fakeRepo) TindakanTerlaris(_ context.Context, _ []string, from, to string, byNilai bool, limit int) ([]Terlaris, error) {
	return []Terlaris{{Nama: "NEBULIZER", Jumlah: 3, Nilai: 150000}}, nil
}
func (f *fakeRepo) Kualitas(_ context.Context, branches []string, today string) ([]CekKualitas, error) {
	f.kualitas = branches
	return []CekKualitas{{Kode: "stok-negatif", Judul: "Stok negatif", Jumlah: 1, Kolom: []string{"Obat"}, Contoh: [][]string{{"X"}}}}, nil
}
func (f *fakeRepo) FollowUp(_ context.Context, p FollowUpParams) (FollowUpPage, error) {
	f.followup = p
	return FollowUpPage{Rows: []FollowUpRow{{NoRM: "001", Nama: "BUDI", Kelompok: "hipertensi", HariSejak: 40}}, Total: 1}, nil
}
func (f *fakeRepo) ICDByKode(_ context.Context, codes []string) (map[string]icd.ICD, error) {
	f.icdCodes = codes
	return map[string]icd.ICD{"J06.9": {ID: 345, Kode: "J06", Nama: "ISPA"}}, nil
}
func (f *fakeRepo) SearchICD(_ context.Context, q string, limit int) ([]icd.ICD, error) {
	f.searched = append(f.searched, q)
	return []icd.ICD{{ID: 345, Kode: "J06", Nama: "ISPA"}, {ID: 400, Kode: "J20", Nama: "BRONKITIS AKUT"}}, nil
}

type fakeAI struct {
	konteks string
	data    string
	history []ChatMessage
	calls   int
	tool    string // the tool Chat calls, if any
	toolOut string
	toolIn  string // input for the tool, default {"limit": 1, "status": "kritis"}
	err     error
	saran   []ICDSaran
	saranIn string
}

func (f *fakeAI) SaranICD(_ context.Context, deskripsi string) ([]ICDSaran, error) {
	f.saranIn = deskripsi
	return f.saran, f.err
}

func (f *fakeAI) Ringkasan(_ context.Context, konteks, data string) (string, error) {
	f.calls++
	f.konteks, f.data = konteks, data
	return "Ringkasan stok.", f.err
}
func (f *fakeAI) Chat(ctx context.Context, konteks string, history []ChatMessage, run ToolFunc) (string, error) {
	f.calls++
	f.konteks, f.history = konteks, history
	if f.tool != "" {
		in := f.toolIn
		if in == "" {
			in = `{"limit": 1, "status": "kritis"}`
		}
		out, err := run(ctx, f.tool, json.RawMessage(in))
		if err != nil {
			return "", err
		}
		f.toolOut = out
	}
	return "Jawaban.", f.err
}

type server struct {
	mux  *http.ServeMux
	repo *fakeRepo
	ai   *fakeAI
	tok  *auth.TokenManager
}

func newServer(t *testing.T, withAI bool) *server {
	t.Helper()
	repo := &fakeRepo{branches: map[string][]branchauth.Branch{
		csID: {{Kode: "KMJ", Nama: "KLINIK MITRA JATINEGARA"}, {Kode: "MED", Nama: "KLINIK MEDIKA"}},
	}}
	ai := &fakeAI{}
	var assistant Assistant
	if withAI {
		assistant = ai
	}
	tokens := auth.NewTokenManager("test-secret-that-is-at-least-32-characters", time.Minute)
	guarded := map[string]int{}
	requireMenu := func(path string) func(http.Handler) http.Handler {
		if path != PagePath && path != PengobatanPagePath {
			t.Fatalf("guarded by %q", path)
		}
		guarded[path]++
		return func(next http.Handler) http.Handler { return next }
	}
	h := NewHandler(repo, assistant)
	h.now = func() time.Time { return now }
	mux := http.NewServeMux()
	h.Routes(mux, tokens.Middleware, requireMenu)
	if guarded[PagePath] != 10 || guarded[PengobatanPagePath] != 1 {
		t.Fatalf("routes guarded: %v, want 10 by %s and 1 by %s", guarded, PagePath, PengobatanPagePath)
	}
	return &server{mux: mux, repo: repo, ai: ai, tok: tokens}
}

func (s *server) do(t *testing.T, method, userID, path string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	if userID != "" {
		tok, _, _ := s.tok.Generate(userID, "apoteker@medika.com")
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestOptions(t *testing.T) {
	s := newServer(t, false)
	code, out := s.do(t, "GET", csID, "/api/analitik/options", nil)
	if code != http.StatusOK || out["today"] != "2026-11-25" || out["aiEnabled"] != false || len(out["branches"].([]any)) != 2 {
		t.Fatalf("options: %d %v", code, out)
	}
	if code, _ := s.do(t, "GET", "", "/api/analitik/options", nil); code != http.StatusUnauthorized {
		t.Errorf("no token: %d", code)
	}
}

func TestObat(t *testing.T) {
	s := newServer(t, false)
	code, out := s.do(t, "GET", csID, "/api/analitik/obat", nil)
	u := s.repo.usage
	if code != http.StatusOK || u.From != "2026-08-28" || u.To != "2026-11-25" || strings.Join(u.Branches, ",") != "KMJ,MED" {
		t.Fatalf("default (90 days, all branches): %d %v %+v", code, out, u)
	}
	rows := out["rows"].([]any)
	if len(rows) != 2 || rows[0].(map[string]any)["status"] != StatusKritis || out["horizon"] != float64(30) || out["leadTime"] != float64(7) {
		t.Errorf("rows: %v", out)
	}
	s.do(t, "GET", csID, "/api/analitik/obat?cabang=kmj&q=+amox+&horizon=999&leadTime=0", nil)
	if u = s.repo.usage; strings.Join(u.Branches, ",") != "KMJ" || u.Search != "amox" {
		t.Errorf("filtered: %+v", u)
	}
	_, out = s.do(t, "GET", csID, "/api/analitik/obat?status=aman&horizon=999", nil)
	if out["total"] != float64(1) || out["horizon"] != float64(90) {
		t.Errorf("status filter / clamped horizon: %v", out)
	}
	if code, _ := s.do(t, "GET", csID, "/api/analitik/obat?cabang=CTM", nil); code != http.StatusForbidden {
		t.Errorf("other branch: %d", code)
	}
	if code, out := s.do(t, "GET", loneID, "/api/analitik/obat", nil); code != http.StatusOK || len(out["rows"].([]any)) != 0 {
		t.Errorf("no branch: %d %v", code, out)
	}
}

func TestPenyakitAndDiagnosaObat(t *testing.T) {
	s := newServer(t, false)
	code, out := s.do(t, "GET", csID, "/api/analitik/penyakit?cabang=MED&weeks=2&top=50", nil)
	if code != http.StatusOK || s.repo.weeks != 4 || strings.Join(s.repo.weeklyFor, ",") != "MED" || len(out["top"].([]any)) != 1 {
		t.Fatalf("penyakit (weeks clamped to 4): %d %v", code, out)
	}
	code, out = s.do(t, "GET", csID, "/api/analitik/diagnosa-obat?icd=+J06+", nil)
	if code != http.StatusOK || s.repo.resep.ICD != "J06" || s.repo.resep.From != "2026-08-28" {
		t.Fatalf("diagnosa-obat: %d %v %+v", code, out, s.repo.resep)
	}
	saran := out["saran"].([]any)
	if len(saran) != 1 || saran[0].(map[string]any)["kurang"].(float64) <= 0 {
		t.Errorf("saran: %v", saran)
	}
}

func TestAIDisabled(t *testing.T) {
	s := newServer(t, false)
	for _, path := range []string{"/api/analitik/ringkasan", "/api/analitik/chat"} {
		if code, _ := s.do(t, "POST", csID, path, map[string]any{}); code != http.StatusServiceUnavailable {
			t.Errorf("%s without API key: %d", path, code)
		}
	}
}

func TestRingkasanIsCachedPerDay(t *testing.T) {
	s := newServer(t, true)
	code, out := s.do(t, "POST", csID, "/api/analitik/ringkasan", map[string]any{"cabang": "kmj"})
	if code != http.StatusOK || out["text"] != "Ringkasan stok." || out["cached"] != false || s.ai.calls != 1 {
		t.Fatalf("first: %d %v", code, out)
	}
	if !strings.Contains(s.ai.konteks, "2026-11-25") || !strings.Contains(s.ai.konteks, "KMJ") ||
		!strings.Contains(s.ai.data, "AMOXICILLIN") || !strings.Contains(s.ai.data, "J06.9") {
		t.Errorf("context/data: %q %q", s.ai.konteks, s.ai.data)
	}
	if _, out = s.do(t, "POST", csID, "/api/analitik/ringkasan", map[string]any{"cabang": "KMJ"}); out["cached"] != true || s.ai.calls != 1 {
		t.Errorf("second: %v", out)
	}
	if _, out = s.do(t, "POST", csID, "/api/analitik/ringkasan", map[string]any{"cabang": "KMJ", "refresh": true}); out["cached"] != false || s.ai.calls != 2 {
		t.Errorf("refresh: %v", out)
	}
	s.ai.err = errors.New("boom")
	if code, _ := s.do(t, "POST", csID, "/api/analitik/ringkasan", map[string]any{"cabang": "MED"}); code != http.StatusBadGateway {
		t.Errorf("AI down: %d", code)
	}
	s.ai.err = ErrRefused
	if code, _ := s.do(t, "POST", csID, "/api/analitik/ringkasan", map[string]any{"cabang": "MED"}); code != http.StatusUnprocessableEntity {
		t.Errorf("refused: %d", code)
	}
	if code, _ := s.do(t, "POST", loneID, "/api/analitik/ringkasan", map[string]any{}); code != http.StatusForbidden {
		t.Errorf("no branch: %d", code)
	}
}

func TestChat(t *testing.T) {
	s := newServer(t, true)
	s.ai.tool = ToolPrediksiObat
	msgs := []ChatMessage{
		{Role: "assistant", Content: "Halo"}, // a leading assistant turn is dropped
		{Role: "user", Content: " Obat apa yang kritis? "},
	}
	code, out := s.do(t, "POST", csID, "/api/analitik/chat", map[string]any{"cabang": "KMJ", "messages": msgs})
	if code != http.StatusOK || out["message"].(map[string]any)["content"] != "Jawaban." {
		t.Fatalf("chat: %d %v", code, out)
	}
	if len(s.ai.history) != 1 || s.ai.history[0].Content != "Obat apa yang kritis?" {
		t.Errorf("history: %+v", s.ai.history)
	}
	// The tool ran on the chosen branch, with the model's filters.
	if strings.Join(s.repo.usage.Branches, ",") != "KMJ" || !strings.Contains(s.ai.toolOut, `"status":"kritis"`) || strings.Contains(s.ai.toolOut, "VITAMIN") {
		t.Errorf("tool: %+v %s", s.repo.usage, s.ai.toolOut)
	}
	for name, body := range map[string]any{
		"empty":            map[string]any{"messages": []ChatMessage{}},
		"ends with answer": map[string]any{"messages": []ChatMessage{{Role: "user", Content: "a"}, {Role: "assistant", Content: "b"}}},
		"not alternating":  map[string]any{"messages": []ChatMessage{{Role: "user", Content: "a"}, {Role: "user", Content: "b"}}},
		"too long":         map[string]any{"messages": []ChatMessage{{Role: "user", Content: strings.Repeat("x", MaxChatChars+1)}}},
	} {
		if code, _ := s.do(t, "POST", csID, "/api/analitik/chat", body); code != http.StatusBadRequest {
			t.Errorf("%s: %d", name, code)
		}
	}
	// A long earlier answer is cut, not rejected.
	long := []ChatMessage{{Role: "user", Content: "a"}, {Role: "assistant", Content: strings.Repeat("y", MaxAnswerChars+500)}, {Role: "user", Content: "lagi"}}
	if code, out := s.do(t, "POST", csID, "/api/analitik/chat", map[string]any{"messages": long}); code != http.StatusOK ||
		len([]rune(s.ai.history[1].Content)) != MaxAnswerChars+2 {
		t.Errorf("long answer in history: %d %v", code, out)
	}
	if code, _ := s.do(t, "POST", csID, "/api/analitik/chat", map[string]any{"cabang": "CTM", "messages": msgs}); code != http.StatusForbidden {
		t.Errorf("other branch: %d", code)
	}
}

func TestToolRunnerRejectsUnknownTool(t *testing.T) {
	h := NewHandler(&fakeRepo{}, nil)
	h.now = func() time.Time { return now }
	run := h.toolRunner([]string{"KMJ"})
	if _, err := run(context.Background(), "hapus_data", nil); err == nil {
		t.Error("unknown tool accepted")
	}
	if _, err := run(context.Background(), ToolTrenPenyakit, json.RawMessage(`{bad`)); err == nil {
		t.Error("bad input accepted")
	}
	out, err := run(context.Background(), ToolDiagnosaObat, json.RawMessage(`{"icd": "J06"}`))
	if err != nil || !strings.Contains(out, "AMOXICILLIN") {
		t.Errorf("diagnosa_obat: %v %s", err, out)
	}
}
