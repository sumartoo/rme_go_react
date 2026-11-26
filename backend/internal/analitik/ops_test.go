package analitik

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/example/be_jwt_golang/backend/internal/pengobatan"
)

func TestPengobatanPagePathMatches(t *testing.T) {
	if PengobatanPagePath != pengobatan.PagePath {
		t.Fatalf("PengobatanPagePath %q, pengobatan.PagePath %q", PengobatanPagePath, pengobatan.PagePath)
	}
}

func TestPlanROPAndCost(t *testing.T) {
	p := PlanStock(100, Rate{AvgDaily: 2, StdDev: 1, Days: 60}, 30, 7)
	if p.ROP != 18.37 { // 2×7 + 4.37
		t.Errorf("ROP: %+v", p)
	}
	hp := 500.0
	rows := Predict([]Usage{
		{IDObat: 1, Nama: "A", Stok: 0, HargaPokok: &hp, Series: flat(60, 2)},
		{IDObat: 2, Nama: "B", Stok: 0, Series: flat(60, 1)},
	}, 30, 7)
	if rows[0].EstimasiBiaya == nil || *rows[0].EstimasiBiaya != rows[0].SaranOrder*500 || rows[1].EstimasiBiaya != nil {
		t.Errorf("cost: %+v / %+v", rows[0], rows[1])
	}
}

func TestObatCostTotals(t *testing.T) {
	s := newServer(t, false)
	_, out := s.do(t, "GET", csID, "/api/analitik/obat", nil)
	// The fake drugs have no cost price: the one to order is counted as unpriced.
	if out["biayaOrder"] != float64(0) || out["orderTanpaHarga"] != float64(1) {
		t.Errorf("totals: %v %v", out["biayaOrder"], out["orderTanpaHarga"])
	}
	rows := out["rows"].([]any)
	if _, ok := rows[0].(map[string]any)["rop"]; !ok {
		t.Errorf("rop missing: %v", rows[0])
	}
}

func TestBuildInsights(t *testing.T) {
	delta := -50.0
	k := KPI{ObatHabis: 2, ObatKritis: 1, ObatPerluOrder: 3, BiayaOrder: 1_250_000, OrderTanpaHarga: 2, TidakBergerak: 9,
		Transaksi7: 70, Transaksi7Lalu: 140, TransaksiDelta: &delta}
	obat := []ObatPrediksi{
		{Nama: "LARIS", Cabang: "KMJ", AvgHarian: 3, Plan: Plan{Status: StatusHabis}},
		{Nama: "JARANG", Cabang: "KMJ", AvgHarian: 0.1, Plan: Plan{Status: StatusHabis}},
	}
	sakit := Penyakit{Lonjakan: []IcdTrend{{Kode: "A90", Nama: "DBD", Trend: Trend{MingguIni: 9, RataSebelum: 2, Rasio: 4.5}}}}
	got := BuildInsights(k, obat, sakit)
	var titles []string
	for _, i := range got {
		titles = append(titles, i.Level+":"+i.Judul)
	}
	want := "kritis:2 obat habis|kritis:1 obat kritis|perhatian:Lonjakan A90 DBD|perhatian:Transaksi turun 50%|info:6 obat perlu dipesan|info:9 item stok tidak bergerak"
	if strings.Join(titles, "|") != want {
		t.Fatalf("insights:\n%s\nwant\n%s", strings.Join(titles, "|"), want)
	}
	if !strings.Contains(got[0].Detail, "LARIS (KMJ, 3.0/hari)") || strings.Contains(got[0].Detail, "JARANG") {
		t.Errorf("habis detail: %s", got[0].Detail)
	}
	if !strings.Contains(got[4].Detail, "Rp1.250.000") || !strings.Contains(got[4].Detail, "2 obat belum dihitung") {
		t.Errorf("order detail: %s", got[4].Detail)
	}
	if got := BuildInsights(KPI{}, nil, Penyakit{}); len(got) != 0 {
		t.Errorf("nothing to report: %+v", got)
	}
}

func TestCommand(t *testing.T) {
	s := newServer(t, false)
	code, out := s.do(t, "GET", csID, "/api/analitik/command?cabang=KMJ", nil)
	if code != http.StatusOK {
		t.Fatalf("command: %d %v", code, out)
	}
	k := out["kpi"].(map[string]any)
	if k["transaksi7"] != float64(70) || k["transaksi7Lalu"] != float64(140) || k["transaksiDelta"] != float64(-50) ||
		k["pendapatan30"] != float64(14_000_000) || k["obatKritis"] != float64(1) {
		t.Errorf("kpi: %v", k)
	}
	if len(out["insights"].([]any)) == 0 || len(out["mingguan"].([]any)) != 1 {
		t.Errorf("insights/mingguan: %v", out)
	}
	last := s.repo.kinerja[len(s.repo.kinerja)-1]
	if last.Kelompok != "minggu" || last.From != "2026-09-07" || strings.Join(last.Branches, ",") != "KMJ" {
		t.Errorf("weekly query (12 weeks from a Monday): %+v", last)
	}
}

func TestKinerjaEndpoint(t *testing.T) {
	s := newServer(t, false)
	code, out := s.do(t, "GET", csID, "/api/analitik/kinerja?kelompok=dokter&dari=2026-01-01&sampai=2026-03-31", nil)
	p := s.repo.kinerja[len(s.repo.kinerja)-1]
	if code != http.StatusOK || p.Kelompok != "dokter" || p.From != "2026-01-01" || p.To != "2026-03-31" ||
		out["total"].(map[string]any)["pendapatan"] != float64(7_000_000) {
		t.Fatalf("kinerja: %d %v %+v", code, out, p)
	}
	s.do(t, "GET", csID, "/api/analitik/kinerja", nil)
	if p = s.repo.kinerja[len(s.repo.kinerja)-1]; p.Kelompok != "minggu" || p.From != "2026-09-03" || p.To != "2026-11-25" {
		t.Errorf("default (84 days by week): %+v", p)
	}
	for path, want := range map[string]int{
		"/api/analitik/kinerja?kelompok=evil":                     http.StatusBadRequest,
		"/api/analitik/kinerja?dari=2025-01-01&sampai=2026-03-31": http.StatusBadRequest, // > 366 days
		"/api/analitik/kinerja?dari=2026-03-31&sampai=2026-01-01": http.StatusBadRequest,
		"/api/analitik/kinerja?cabang=CTM":                        http.StatusForbidden,
	} {
		if code, _ := s.do(t, "GET", csID, path, nil); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
}

func TestBusinessTools(t *testing.T) {
	s := newServer(t, true)
	s.ai.tool, s.ai.toolIn = ToolKinerja, `{"kelompok": "penjamin", "dari": "2026-10-01", "sampai": "2026-10-31"}`
	code, _ := s.do(t, "POST", csID, "/api/analitik/chat", map[string]any{"cabang": "MED", "messages": []ChatMessage{{Role: "user", Content: "Pendapatan per penjamin?"}}})
	p := s.repo.kinerja[len(s.repo.kinerja)-1]
	if code != http.StatusOK || p.Kelompok != "penjamin" || p.From != "2026-10-01" || strings.Join(p.Branches, ",") != "MED" ||
		!strings.Contains(s.ai.toolOut, `"pendapatan":7000000`) {
		t.Errorf("kinerja tool: %d %+v %s", code, p, s.ai.toolOut)
	}
	s.ai.tool, s.ai.toolIn = ToolObatTerlaris, `{"urut": "nilai"}`
	s.do(t, "POST", csID, "/api/analitik/chat", map[string]any{"messages": []ChatMessage{{Role: "user", Content: "Obat terlaris?"}}})
	if !strings.Contains(s.ai.toolOut, "PARASETAMOL") || !strings.Contains(s.ai.toolOut, `"dari":"2026-10-27"`) {
		t.Errorf("obat tool (default last 30 days): %s", s.ai.toolOut)
	}
	run := s.repoHandler().toolRunner([]string{"KMJ"})
	if _, err := run(t.Context(), ToolKinerja, []byte(`{"kelompok": "evil"}`)); err == nil {
		t.Error("unknown kelompok accepted")
	}
	if _, err := run(t.Context(), ToolTindakanTerlaris, []byte(`{"dari": "2024-01-01", "sampai": "2026-01-01"}`)); err == nil {
		t.Error("period over 366 days accepted")
	}
}

// repoHandler builds a handler on the server's fake repository (for direct tool calls).
func (s *server) repoHandler() *Handler {
	h := NewHandler(s.repo, nil)
	h.now = func() time.Time { return now }
	return h
}

func TestKualitasAndFollowUp(t *testing.T) {
	s := newServer(t, false)
	code, out := s.do(t, "GET", csID, "/api/analitik/kualitas-data?cabang=MED", nil)
	if code != http.StatusOK || strings.Join(s.repo.kualitas, ",") != "MED" || len(out["checks"].([]any)) != 1 {
		t.Fatalf("kualitas: %d %v", code, out)
	}
	code, out = s.do(t, "GET", csID, "/api/analitik/follow-up?hari=5&kelompok=diabetes", nil)
	f := s.repo.followup
	if code != http.StatusOK || f.Hari != 14 || f.Kelompok != "diabetes" || f.Today != "2026-11-25" || out["total"] != float64(1) {
		t.Fatalf("follow-up (hari clamped to 14): %d %v %+v", code, out, f)
	}
	if code, _ := s.do(t, "GET", csID, "/api/analitik/follow-up?kelompok=flu", nil); code != http.StatusBadRequest {
		t.Errorf("unknown kelompok: %d", code)
	}
	if code, out := s.do(t, "GET", loneID, "/api/analitik/kualitas-data", nil); code != http.StatusOK || len(out["checks"].([]any)) != 0 {
		t.Errorf("no branch: %d %v", code, out)
	}
}

func TestSaranICD(t *testing.T) {
	// Without AI: search only.
	s := newServer(t, false)
	code, out := s.do(t, "POST", csID, "/api/analitik/saran-icd", map[string]any{"deskripsi": "batuk pilek demam tiga hari"})
	if code != http.StatusOK || out["ai"] != false || len(out["kandidat"].([]any)) != 2 || len(s.repo.searched) == 0 {
		t.Fatalf("search only: %d %v %v", code, out, s.repo.searched)
	}
	// With AI: valid codes first (with reason), unknown codes dropped, then search hits without duplicates.
	s = newServer(t, true)
	s.ai.saran = []ICDSaran{{Kode: "j06.9", Alasan: "ISPA"}, {Kode: "ZZZ.9", Alasan: "tidak ada"}}
	code, out = s.do(t, "POST", csID, "/api/analitik/saran-icd", map[string]any{"deskripsi": "batuk pilek demam"})
	k := out["kandidat"].([]any)
	first := k[0].(map[string]any)
	if code != http.StatusOK || out["ai"] != true || len(k) != 2 || first["kode"] != "J06" || first["sumber"] != "ai" || first["alasan"] != "ISPA" ||
		k[1].(map[string]any)["kode"] != "J20" || strings.Join(s.repo.icdCodes, ",") != "J06.9,ZZZ.9" || s.ai.saranIn != "batuk pilek demam" {
		t.Fatalf("with AI: %d %v %v", code, out, s.repo.icdCodes)
	}
	for _, d := range []string{"", "ab", strings.Repeat("x", MaxDeskripsi+1)} {
		if code, _ := s.do(t, "POST", csID, "/api/analitik/saran-icd", map[string]any{"deskripsi": d}); code != http.StatusBadRequest {
			t.Errorf("deskripsi %d chars: %d", len(d), code)
		}
	}
}

func TestSearchTerms(t *testing.T) {
	got := searchTerms("Batuk, pilek; demam 3 hari, nyeri tenggorokan & batuk", 3)
	if strings.Join(got, ",") != "tenggorokan,batuk,pilek" {
		t.Errorf("terms: %v", got)
	}
}
