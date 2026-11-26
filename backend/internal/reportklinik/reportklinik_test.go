package reportklinik

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/branchauth"
)

const (
	csID   = "22222222-2222-2222-2222-222222222222" // mapped to MED and KMJ
	loneID = "33333333-3333-3333-3333-333333333333" // mapped to no branch
)

// 23:30 UTC on 18 Nov is already 19 Nov in Jakarta.
var now = time.Date(2026, 11, 18, 23, 30, 0, 0, time.UTC)

type fakeRepo struct {
	branches map[string][]branchauth.Branch
	last     ObatParams
	lastT    TindakanParams
	lastP    PenunjangParams
	lastO    OTCParams
	lastB    PasienBaruParams
	lastA    AsuransiParams
	lastH    HarianParams
	lastS    StokParams
	lastR    RujukanParams
	lastD    PendaftaranParams
	lastI    PendapatanParams
	lastDok  string
	lastX    DiagnosaParams
	lastK    DinkesParams
	lastSO   OpnameParams
	obatFor  string
	lastTr   TraceParams
	calls    int
}

func (f *fakeRepo) UserBranches(_ context.Context, userID string) ([]branchauth.Branch, error) {
	return f.branches[userID], nil
}
func (f *fakeRepo) JenisObat(context.Context) ([]Option, error) {
	return []Option{{ID: 1, Name: "ETIKAL"}, {ID: 2, Name: "OTC"}}, nil
}
func (f *fakeRepo) PenjualanObat(_ context.Context, p ObatParams) (ObatPage, error) {
	f.last, f.calls = p, f.calls+1
	return ObatPage{Rows: []ObatRow{{Tgl: "2026-11-19", IDObat: 9, Obat: "AKITA TAB", Jenis: "ETIKAL", Satuan: "TABLET", Harga: 2000, Byk: 20, Jumlah: 40000, Cabang: "KMJ", Status: StatusClose}},
		Total: 1, UnfilteredTotal: 1}, nil
}

func (f *fakeRepo) PenjualanTindakan(_ context.Context, p TindakanParams) (TindakanPage, error) {
	f.lastT, f.calls = p, f.calls+1
	return TindakanPage{Rows: []TindakanRow{{ID: 1, IDJual: 522954, Tgl: "2026-11-20", Tindakan: "JASA KONSULTASI", Jenis: "Konsultasi", Dokter: "dr. A",
		Pasien: "JOANNA", Fee: 21500, Harga: 42500, Byk: 1, TotalFee: 21500, Cabang: "KMJ", Status: StatusClose}}, Total: 1, UnfilteredTotal: 1}, nil
}

func (f *fakeRepo) JenisPenunjang(context.Context) ([]Option, error) {
	return []Option{{ID: 15, Name: "LAB MOHC"}, {ID: 21, Name: "LABORATORIUM CV"}}, nil
}
func (f *fakeRepo) PenjualanPenunjang(_ context.Context, p PenunjangParams) (PenunjangPage, error) {
	f.lastP, f.calls = p, f.calls+1
	return PenunjangPage{Rows: []PenunjangRow{{ID: 1, IDJual: 522954, Tgl: "2026-11-20", NoRM: "89-01-30", Pasien: "M GOZALI", Penunjang: "PROMO 3 IN 1",
		Jenis: "LABORATORIUM CV", Dokter: "dr. FEBRIMA J 2", Harga: 69000, Byk: 1, Cabang: "KMJ", Status: StatusClose, CreateUser: "siti_ctm"}}, Total: 1, UnfilteredTotal: 1}, nil
}

func (f *fakeRepo) PerujukOTC(context.Context) ([]Option, error) {
	return []Option{{ID: 3, Name: "DELA"}, {ID: 7, Name: "Mega (MGM)"}}, nil
}
func (f *fakeRepo) PenjualanOTC(_ context.Context, p OTCParams) (OTCPage, error) {
	f.lastO, f.calls = p, f.calls+1
	return OTCPage{Rows: []OTCRow{{ID: 1, IDJual: 522968, Tgl: "2026-11-20", NoKwi: "2611CTM0727", Obat: "RAMOLIT (OTC)", Jenis: "OTC", Perujuk: "DELA",
		Byk: 4, Harga: 2000, Total: 8000, Cabang: "KMJ", Status: StatusClose}}, Total: 1, UnfilteredTotal: 1}, nil
}

func (f *fakeRepo) JenisRawat(context.Context) ([]Option, error) {
	return []Option{{ID: 1, Name: "POLI UMUM"}, {ID: 2, Name: "OBAT BEBAS"}}, nil
}
func (f *fakeRepo) PasienBaru(_ context.Context, p PasienBaruParams) (PasienBaruPage, error) {
	f.lastB, f.calls = p, f.calls+1
	umur := 47
	return PasienBaruPage{Rows: []PasienBaruRow{{IDJual: 522954, Tgl: "2026-11-19 23:41:05", NoRM: "89-01-26", Pasien: "DAUMI", JenisRawat: "POLI UMUM",
		Umur: &umur, TglDaftar: "2026-11-19", Status: StatusClose, Cabang: "KMJ"}}, Total: 1, UnfilteredTotal: 1}, nil
}

func (f *fakeRepo) Asuransi(context.Context) ([]Option, error) {
	return []Option{{ID: 5, Name: "GARDA MEDIKA"}, {ID: 9, Name: "MEDITAP"}}, nil
}
func (f *fakeRepo) RekapAsuransi(_ context.Context, p AsuransiParams) (AsuransiPage, error) {
	f.lastA, f.calls = p, f.calls+1
	return AsuransiPage{Rows: []AsuransiSale{{IDJual: 522954, Tgl: "2026-11-17", NoRM: "43-01-64", NoKartu: "M/00178350", Perusahaan: "GARDA MEDIKA",
		Pasien: "MELVA", Dokter: "DR AIDANI", Diagnosa: []string{"VASOMOTOR AND ALLERGIC RHINITIS"},
		Obat:     []AsuransiLine{{Nama: "MALTIRON PLUS", Byk: 10, Harga: 5000, Jumlah: 50000}},
		Tindakan: []AsuransiLine{}, Penunjang: []AsuransiLine{}, Total: 50000, Cabang: "KMJ", Status: StatusClose}}, Total: 1, UnfilteredTotal: 1}, nil
}

func (f *fakeRepo) JenisBayar(context.Context) ([]Option, error) {
	return []Option{{ID: 1, Name: "TUNAI"}, {ID: 9, Name: "NON TUNAI"}}, nil
}
func (f *fakeRepo) TransaksiHarian(_ context.Context, p HarianParams) (HarianPage, error) {
	f.lastH, f.calls = p, f.calls+1
	return HarianPage{Rows: []HarianRow{{IDJual: 522954, Tgl: "2026-11-20", NoRM: "89-01-31", Pasien: "RISWALDY", Pelayanan: "POLI UMUM", Dokter: "dr. FEBRIMA J 2",
		Pembayaran: "TUNAI", Obat: 105000, Penunjang: 70000, Total: 175000, Cabang: "KMJ", Status: StatusClose}}, Total: 1, UnfilteredTotal: 1}, nil
}

func (f *fakeRepo) StokOpname(_ context.Context, p OpnameParams) (OpnamePage, error) {
	f.lastSO, f.calls = p, f.calls+1
	return OpnamePage{Rows: []OpnameRow{{IDObat: 3031, Nama: "ATORVASTATIN 10 MG (OTC)", HargaJual: 33000, BeratIsi: "10", Jenis: "OTC", SatuanBeli: "STRIP",
		SatuanJual: "STRIP", JumUnit: 1, StokMin: 2, Stok: 0, Cabang: "MED", NamaCabang: "KLINIK MEDIKA", IsActive: true}}, Total: 1, UnfilteredTotal: 1}, nil
}

func (f *fakeRepo) ObatCabang(_ context.Context, branch string) ([]ObatCabang, error) {
	f.obatFor = branch
	return []ObatCabang{{ID: 3031, Nama: "ATORVASTATIN 10 MG (OTC)", Jenis: "OTC", Satuan: "STRIP", Stok: 0}}, nil
}

func (f *fakeRepo) StokTrace(_ context.Context, p TraceParams) (TracePage, error) {
	f.lastTr, f.calls = p, f.calls+1
	return TracePage{Rows: []TraceRow{{ID: 914021, IDObat: 1553, Nama: "PARACETAMOL", Cabang: "MED", NamaCabang: "KLINIK MEDIKA", LastStock: 5,
		InOut: "Out", Byk: 1, Stock: 4, SystemStock: 4, TableRef: "TrxJualObat", Transaksi: "Penjualan Obat", Remarks: "Transaksi 2611MED0768",
		CreatedBy: "naylul", CreatedDate: "2026-11-19 23:32:10.773"}}, Total: 1, UnfilteredTotal: 1}, nil
}

func (f *fakeRepo) MovingPO(context.Context) ([]Option, error) {
	return []Option{{ID: 1, Name: "-"}, {ID: 4, Name: "SM"}}, nil
}
func (f *fakeRepo) AllStok(_ context.Context, p StokParams) (StokPage, error) {
	f.lastS, f.calls = p, f.calls+1
	return StokPage{Rows: []StokRow{{IDObat: 2857, Obat: "VAKSIN HEXAXIM", SatuanBeli: "BOX", IsiPerBox: "1", SatuanJual: "AMPUL", HargaBeli: 793500,
		HargaJual: 1225000, Jenis: "ETIKAL", Distrib: "APOTIK MIRAKEL", MovingPO: "SM", StokAwal: 3, Jual: 1, StokAkhir: 2, Cabang: "KMJ"}}, Total: 1, UnfilteredTotal: 1}, nil
}

func (f *fakeRepo) Rujukan(_ context.Context, p RujukanParams) (RujukanPage, error) {
	f.lastR, f.calls = p, f.calls+1
	line := 7
	return RujukanPage{Rows: []RujukanRow{
		{IDJual: 522954, Tgl: "2026-11-15", NoRM: "33-01-84", Pasien: "FARREN", Dokter: "dr. SITTI ATIKA", JenisRawat: "LABORATORIUM", IDLine: &line,
			Penunjang: "GOLONGAN DARAH ABO + RHESUS", Total: 55000, Cabang: "KMJ", Status: StatusClose},
		{IDJual: 522955, Tgl: "2026-11-15", NoRM: "04-00-03", Pasien: "OBAT BEBAS", Dokter: "dr. SITTI ATIKA", JenisRawat: "OBAT BEBAS", Cabang: "KMJ", Status: StatusClose},
	}, Total: 2, UnfilteredTotal: 2}, nil
}

func (f *fakeRepo) StatusKunjungan(context.Context) ([]Option, error) {
	return []Option{{ID: 2, Name: "CLOSE"}, {ID: 3, Name: "DAFTAR"}}, nil
}
func (f *fakeRepo) Pendaftaran(_ context.Context, p PendaftaranParams) (PendaftaranPage, error) {
	f.lastD, f.calls = p, f.calls+1
	antri, umur, tgl := 10, 26, "2026-11-20 14:24:58"
	return PendaftaranPage{Rows: []PendaftaranRow{{ID: 518790, NoAntri: &antri, Tgl: &tgl, Pelayanan: "POLI UMUM", StatusKun: "CLOSE", NoRM: "77-00-16",
		Pasien: "ARYA", Umur: &umur, Alamat: "JMA", Cabang: "KMJ"}}, Total: 1, UnfilteredTotal: 1}, nil
}

func (f *fakeRepo) Pendapatan(_ context.Context, p PendapatanParams) (Pendapatan, error) {
	f.lastI, f.calls = p, f.calls+1
	return Pendapatan{Cabang: p.Cabang, NamaCabang: "KLINIK MITRA JATINEGARA", TelpCabang: "Telp +62 13-8865335 Fax-", Rows: []PendapatanRow{
		{IDJual: 1, Tgl: "2026-11-19 00:13:04.517", NoKwi: "2611KMJ0695", Pasien: "Obat Bebas", Obat: 149000, Total: 149000, Baru: true},
	}}, nil
}
func (f *fakeRepo) DokterCabang(_ context.Context, cabang string) ([]Option, error) {
	f.lastDok, f.calls = cabang, f.calls+1
	return []Option{{ID: 1, Name: "NON DOKTER"}, {ID: 10, Name: "DR HUMAIRAH"}}, nil
}

func (f *fakeRepo) ReportDiagnosa(_ context.Context, p DiagnosaParams) (DiagnosaPage, error) {
	f.lastX, f.calls = p, f.calls+1
	usia := 44
	return DiagnosaPage{Rows: []DiagnosaRow{{ID: 1, IDJual: 522990, Tgl: "2026-11-20 14:30:38", NoRM: "51-00-50", Pasien: "RETNO", Usia: &usia,
		JenisKelamin: "Perempuan", Dokter: "dr. Pitje", JenisRawat: "POLI UMUM", ICD: "J06", Diagnosa: "ISPA", Resep: "SANMOL (8)", Keluhan: "BATUK",
		Cabang: "KMJ"}}, Total: 1, UnfilteredTotal: 1}, nil
}

func (f *fakeRepo) ReportDinkes(_ context.Context, p DinkesParams) (DinkesPage, error) {
	f.lastK, f.calls = p, f.calls+1
	return DinkesPage{Rows: []DinkesRow{{Kode: "R51", ICD: "HEADACHE (CEPHALGIA)", JenisRawat: "POLI UMUM", Tahun20_44: 2, Laki: 1, Perempuan: 1, Total: 2}},
		Total: 1, UnfilteredTotal: 1}, nil
}

type server struct {
	mux  *http.ServeMux
	repo *fakeRepo
	tok  *auth.TokenManager
}

func newServer(t *testing.T) *server {
	t.Helper()
	repo := &fakeRepo{branches: map[string][]branchauth.Branch{
		csID: {{Kode: "KMJ", Nama: "KLINIK MITRA JATINEGARA"}, {Kode: "MED", Nama: "KLINIK MEDIKA"}},
	}}
	tokens := auth.NewTokenManager("test-secret-that-is-at-least-32-characters", time.Minute)
	guarded := map[string]int{}
	requireMenu := func(path string) func(http.Handler) http.Handler {
		guarded[path]++
		return func(next http.Handler) http.Handler { return next }
	}
	h := NewHandler(repo)
	h.now = func() time.Time { return now }
	mux := http.NewServeMux()
	h.Routes(mux, tokens.Middleware, requireMenu)
	// Each report's endpoints are guarded by its own menu item.
	if len(guarded) != 17 || guarded[StokTracePath] != 2 || guarded[StokOpnamePath] != 2 || guarded[StokAlertPath] != 2 || guarded[PenjualanObatPath] != 2 || guarded[PenjualanTindakanPath] != 2 || guarded[PenjualanPenunjangPath] != 2 ||
		guarded[PenjualanOTCPath] != 2 || guarded[PasienBaruPath] != 2 || guarded[RekapAsuransiPath] != 2 || guarded[TransaksiHarianPath] != 2 ||
		guarded[AllStokPath] != 2 || guarded[RujukanPath] != 2 || guarded[PenjualanSoftdrinkPath] != 2 ||
		guarded[PendaftaranPath] != 2 || guarded[PendapatanPath] != 3 || guarded[DiagnosaPath] != 2 || guarded[DinkesPath] != 2 {
		t.Fatalf("routes guarded: %v, want 2 by each report's menu", guarded)
	}
	return &server{mux: mux, repo: repo, tok: tokens}
}

func (s *server) get(t *testing.T, userID, path string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	if userID != "" {
		tok, _, _ := s.tok.Generate(userID, "cs@medika.com")
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestPenjualanObat(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/report-klinik/penjualan-obat")
	p := s.repo.last
	if code != http.StatusOK || out["from"] != "2026-11-19" || p.From != "2026-11-19" || p.To != "2026-11-19" ||
		strings.Join(p.Branches, ",") != "KMJ,MED" || p.Sort != "obat" || p.Desc || p.Limit != 10 {
		t.Fatalf("default (today in Jakarta, by drug name): %d %v %+v", code, out, p)
	}
	if rows := out["rows"].([]any); len(rows) != 1 || rows[0].(map[string]any)["jumlah"] != 40000.0 {
		t.Errorf("rows: %v", out["rows"])
	}
	s.get(t, csID, "/api/report-klinik/penjualan-obat?from=2026-11-01&to=2026-11-08&cabang=kmj&nama=+akita+&jenis=2&status=Cancel&q=tab&sort=byk&dir=desc&size=25&page=3")
	p = s.repo.last
	if p.From != "2026-11-01" || p.To != "2026-11-08" || strings.Join(p.Branches, ",") != "KMJ" || p.Nama != "akita" ||
		p.IDJenis == nil || *p.IDJenis != 2 || p.Status != StatusCancel || p.Search != "tab" || p.Sort != "byk" || !p.Desc || p.Offset != 50 {
		t.Errorf("filtered: %+v", p)
	}
	s.get(t, csID, "/api/report-klinik/penjualan-obat?sort=drop+table&size=999999&jenis=x")
	if p = s.repo.last; p.Sort != "obat" || p.Desc || p.Limit != MaxPageSize || p.IDJenis != nil {
		t.Errorf("unknown sort / huge size / bad jenis: %+v", p)
	}

	before := s.repo.calls
	for path, want := range map[string]int{
		"/api/report-klinik/penjualan-obat?cabang=CTM":                    http.StatusForbidden,
		"/api/report-klinik/penjualan-obat?from=2026-11-10&to=2026-11-01": http.StatusBadRequest,
		"/api/report-klinik/penjualan-obat?from=18-11-2026":               http.StatusBadRequest,
		"/api/report-klinik/penjualan-obat?status=paid":                   http.StatusBadRequest,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if s.repo.calls != before {
		t.Errorf("rejected requests reached the repository")
	}
	// A user without branches gets an empty page, not everyone's data.
	if code, out := s.get(t, loneID, "/api/report-klinik/penjualan-obat"); code != http.StatusOK || out["total"] != 0.0 || s.repo.calls != before {
		t.Errorf("user without branches: %d %v", code, out)
	}
	if code, _ := s.get(t, "", "/api/report-klinik/penjualan-obat"); code != http.StatusUnauthorized {
		t.Errorf("no token: %d", code)
	}
}

func TestPenjualanObatOptions(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/report-klinik/penjualan-obat/options")
	if code != http.StatusOK || out["today"] != "2026-11-19" || len(out["jenis"].([]any)) != 2 || len(out["branches"].([]any)) != 2 {
		t.Fatalf("options: %d %v", code, out)
	}
}

func TestPenjualanTindakan(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/report-klinik/penjualan-tindakan")
	p := s.repo.lastT
	if code != http.StatusOK || p.From != "2026-11-19" || strings.Join(p.Branches, ",") != "KMJ,MED" || p.Sort != "tgl" || !p.Desc || p.Limit != 10 {
		t.Fatalf("default (today, newest first): %d %v %+v", code, out, p)
	}
	if rows := out["rows"].([]any); len(rows) != 1 || rows[0].(map[string]any)["totalFee"] != 21500.0 {
		t.Errorf("rows: %v", out["rows"])
	}
	s.get(t, csID, "/api/report-klinik/penjualan-tindakan?from=2026-11-01&to=2026-11-08&cabang=med&nama=+konsul+&dokter=+febrima+&status=close&q=x&sort=totalFee&dir=asc&size=20&page=2")
	p = s.repo.lastT
	if p.From != "2026-11-01" || strings.Join(p.Branches, ",") != "MED" || p.Nama != "konsul" || p.Dokter != "febrima" ||
		p.Status != StatusClose || p.Search != "x" || p.Sort != "totalFee" || p.Desc || p.Offset != 20 {
		t.Errorf("filtered: %+v", p)
	}
	s.get(t, csID, "/api/report-klinik/penjualan-tindakan?sort=evil")
	if p = s.repo.lastT; p.Sort != "tgl" || !p.Desc {
		t.Errorf("unknown sort: %+v", p)
	}
	before := s.repo.calls
	for path, want := range map[string]int{
		"/api/report-klinik/penjualan-tindakan?cabang=CTM":      http.StatusForbidden,
		"/api/report-klinik/penjualan-tindakan?from=2026-13-01": http.StatusBadRequest,
		"/api/report-klinik/penjualan-tindakan?status=lunas":    http.StatusBadRequest,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if code, out := s.get(t, loneID, "/api/report-klinik/penjualan-tindakan"); code != http.StatusOK || out["total"] != 0.0 || s.repo.calls != before {
		t.Errorf("rejected or branchless requests: %d %v (calls %d, before %d)", code, out, s.repo.calls, before)
	}
	code, out = s.get(t, csID, "/api/report-klinik/penjualan-tindakan/options")
	if code != http.StatusOK || out["today"] != "2026-11-19" || len(out["branches"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
}

func TestPenjualanPenunjang(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/report-klinik/penjualan-penunjang")
	p := s.repo.lastP
	if code != http.StatusOK || p.From != "2026-11-19" || strings.Join(p.Branches, ",") != "KMJ,MED" || p.Sort != "tgl" || !p.Desc || p.Limit != 10 {
		t.Fatalf("default (today, newest first): %d %v %+v", code, out, p)
	}
	if rows := out["rows"].([]any); len(rows) != 1 || rows[0].(map[string]any)["createUser"] != "siti_ctm" || rows[0].(map[string]any)["noRm"] != "89-01-30" {
		t.Errorf("rows: %v", out["rows"])
	}
	s.get(t, csID, "/api/report-klinik/penjualan-penunjang?from=2026-11-01&to=2026-11-08&cabang=kmj&nama=+promo+&jenis=21&status=cancel&q=x&sort=createUser&dir=asc&size=20&page=2")
	p = s.repo.lastP
	if p.From != "2026-11-01" || strings.Join(p.Branches, ",") != "KMJ" || p.Nama != "promo" || p.IDJenis == nil || *p.IDJenis != 21 ||
		p.Status != StatusCancel || p.Search != "x" || p.Sort != "createUser" || p.Desc || p.Offset != 20 {
		t.Errorf("filtered: %+v", p)
	}
	before := s.repo.calls
	for path, want := range map[string]int{
		"/api/report-klinik/penjualan-penunjang?cabang=CTM":    http.StatusForbidden,
		"/api/report-klinik/penjualan-penunjang?to=2026-01-01": http.StatusBadRequest,
		"/api/report-klinik/penjualan-penunjang?status=paid":   http.StatusBadRequest,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if code, out := s.get(t, loneID, "/api/report-klinik/penjualan-penunjang"); code != http.StatusOK || out["total"] != 0.0 || s.repo.calls != before {
		t.Errorf("rejected or branchless requests: %d %v", code, out)
	}
	code, out = s.get(t, csID, "/api/report-klinik/penjualan-penunjang/options")
	if code != http.StatusOK || out["today"] != "2026-11-19" || len(out["jenis"].([]any)) != 2 || len(out["branches"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
}

func TestPenjualanOTC(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/report-klinik/penjualan-otc")
	p := s.repo.lastO
	if code != http.StatusOK || p.From != "2026-11-19" || strings.Join(p.Branches, ",") != "KMJ,MED" || p.Sort != "idJual" || !p.Desc || p.Limit != 10 {
		t.Fatalf("default (today, newest sale first): %d %v %+v", code, out, p)
	}
	if rows := out["rows"].([]any); len(rows) != 1 || rows[0].(map[string]any)["total"] != 8000.0 || rows[0].(map[string]any)["perujuk"] != "DELA" {
		t.Errorf("rows: %v", out["rows"])
	}
	s.get(t, csID, "/api/report-klinik/penjualan-otc?from=2026-11-01&to=2026-11-08&cabang=med&nama=+vicks+&perujuk=7&status=cancel&q=2611&sort=total&dir=asc&size=20&page=2")
	p = s.repo.lastO
	if p.From != "2026-11-01" || strings.Join(p.Branches, ",") != "MED" || p.Nama != "vicks" || p.IDPerujuk == nil || *p.IDPerujuk != 7 ||
		p.Status != StatusCancel || p.Search != "2611" || p.Sort != "total" || p.Desc || p.Offset != 20 {
		t.Errorf("filtered: %+v", p)
	}
	before := s.repo.calls
	for path, want := range map[string]int{
		"/api/report-klinik/penjualan-otc?cabang=CTM":   http.StatusForbidden,
		"/api/report-klinik/penjualan-otc?from=x":       http.StatusBadRequest,
		"/api/report-klinik/penjualan-otc?status=lunas": http.StatusBadRequest,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if code, out := s.get(t, loneID, "/api/report-klinik/penjualan-otc"); code != http.StatusOK || out["total"] != 0.0 || s.repo.calls != before {
		t.Errorf("rejected or branchless requests: %d %v", code, out)
	}
	code, out = s.get(t, csID, "/api/report-klinik/penjualan-otc/options")
	if code != http.StatusOK || out["today"] != "2026-11-19" || len(out["perujuk"].([]any)) != 2 || len(out["branches"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
}

func TestPasienBaru(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/report-klinik/pasien-baru")
	p := s.repo.lastB
	if code != http.StatusOK || p.From != "2026-11-19" || strings.Join(p.Branches, ",") != "KMJ,MED" || p.Sort != "tgl" || !p.Desc || p.Limit != 10 {
		t.Fatalf("default (today, newest first): %d %v %+v", code, out, p)
	}
	if rows := out["rows"].([]any); len(rows) != 1 || rows[0].(map[string]any)["umur"] != 47.0 {
		t.Errorf("rows: %v", out["rows"])
	}
	s.get(t, csID, "/api/report-klinik/pasien-baru?from=2026-11-01&to=2026-11-08&cabang=med&nama=+daumi+&pelayanan=1&status=close&q=x&sort=pasien&dir=asc&size=20&page=2")
	p = s.repo.lastB
	if p.From != "2026-11-01" || strings.Join(p.Branches, ",") != "MED" || p.Nama != "daumi" || p.IDJenisRawat == nil || *p.IDJenisRawat != 1 ||
		p.Status != StatusClose || p.Search != "x" || p.Sort != "pasien" || p.Desc || p.Offset != 20 {
		t.Errorf("filtered: %+v", p)
	}
	// Oldest first by age is youngest birth date last: the birth-date sort flips.
	s.get(t, csID, "/api/report-klinik/pasien-baru?sort=umur&dir=asc")
	if p = s.repo.lastB; p.Sort != "umur" || !p.Desc {
		t.Errorf("umur asc should sort birth dates desc: %+v", p)
	}
	before := s.repo.calls
	for path, want := range map[string]int{
		"/api/report-klinik/pasien-baru?cabang=CTM":  http.StatusForbidden,
		"/api/report-klinik/pasien-baru?from=bad":    http.StatusBadRequest,
		"/api/report-klinik/pasien-baru?status=paid": http.StatusBadRequest,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if code, out := s.get(t, loneID, "/api/report-klinik/pasien-baru"); code != http.StatusOK || out["total"] != 0.0 || s.repo.calls != before {
		t.Errorf("rejected or branchless requests: %d %v", code, out)
	}
	code, out = s.get(t, csID, "/api/report-klinik/pasien-baru/options")
	if code != http.StatusOK || out["today"] != "2026-11-19" || len(out["pelayanan"].([]any)) != 2 || len(out["branches"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
}

func TestRekapAsuransi(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/report-klinik/rekap-asuransi")
	p := s.repo.lastA
	if code != http.StatusOK || p.From != "2026-11-19" || strings.Join(p.Branches, ",") != "KMJ,MED" || p.Sort != "tgl" || !p.Desc || p.Limit != 10 {
		t.Fatalf("default (today, newest first): %d %v %+v", code, out, p)
	}
	rows := out["rows"].([]any)
	if len(rows) != 1 || len(rows[0].(map[string]any)["obat"].([]any)) != 1 || rows[0].(map[string]any)["tindakan"] == nil {
		t.Errorf("rows (items always lists): %v", out["rows"])
	}
	s.get(t, csID, "/api/report-klinik/rekap-asuransi?from=2026-11-14&to=2026-11-20&cabang=kmj&nama=+melva+&asuransi=5&status=close&q=x&sort=perusahaan&dir=asc&size=20&page=2")
	p = s.repo.lastA
	if p.From != "2026-11-14" || strings.Join(p.Branches, ",") != "KMJ" || p.Nama != "melva" || p.IDJamkes == nil || *p.IDJamkes != 5 ||
		p.Status != StatusClose || p.Search != "x" || p.Sort != "perusahaan" || p.Desc || p.Offset != 20 {
		t.Errorf("filtered: %+v", p)
	}
	before := s.repo.calls
	for path, want := range map[string]int{
		"/api/report-klinik/rekap-asuransi?cabang=CTM":   http.StatusForbidden,
		"/api/report-klinik/rekap-asuransi?to=bad":       http.StatusBadRequest,
		"/api/report-klinik/rekap-asuransi?status=lunas": http.StatusBadRequest,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if code, out := s.get(t, loneID, "/api/report-klinik/rekap-asuransi"); code != http.StatusOK || out["total"] != 0.0 || s.repo.calls != before {
		t.Errorf("rejected or branchless requests: %d %v", code, out)
	}
	code, out = s.get(t, csID, "/api/report-klinik/rekap-asuransi/options")
	if code != http.StatusOK || out["today"] != "2026-11-19" || len(out["asuransi"].([]any)) != 2 || len(out["branches"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
}

func TestTransaksiHarian(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/report-klinik/transaksi-harian")
	p := s.repo.lastH
	if code != http.StatusOK || p.From != "2026-11-19" || strings.Join(p.Branches, ",") != "KMJ,MED" || p.Sort != "tgl" || !p.Desc ||
		p.Limit != 10 || p.IDJenisRawat != nil {
		t.Fatalf("default (today, all services, newest first): %d %v %+v", code, out, p)
	}
	if rows := out["rows"].([]any); len(rows) != 1 || rows[0].(map[string]any)["total"] != 175000.0 {
		t.Errorf("rows: %v", out["rows"])
	}
	s.get(t, csID, "/api/report-klinik/transaksi-harian?from=2026-11-01&to=2026-11-08&cabang=kmj&pelayanan=2&nama=+riswaldy+&pembayaran=9&status=cancel&q=x&sort=diskon&dir=asc&size=20&page=2")
	p = s.repo.lastH
	if p.From != "2026-11-01" || strings.Join(p.Branches, ",") != "KMJ" || p.IDJenisRawat == nil || *p.IDJenisRawat != 2 || p.Nama != "riswaldy" ||
		p.IDJenisBayar == nil || *p.IDJenisBayar != 9 || p.Status != StatusCancel || p.Search != "x" || p.Sort != "diskon" || p.Desc || p.Offset != 20 {
		t.Errorf("filtered: %+v", p)
	}
	before := s.repo.calls
	for path, want := range map[string]int{
		"/api/report-klinik/transaksi-harian?cabang=CTM":  http.StatusForbidden,
		"/api/report-klinik/transaksi-harian?from=x":      http.StatusBadRequest,
		"/api/report-klinik/transaksi-harian?status=paid": http.StatusBadRequest,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if code, out := s.get(t, loneID, "/api/report-klinik/transaksi-harian"); code != http.StatusOK || out["total"] != 0.0 || s.repo.calls != before {
		t.Errorf("rejected or branchless requests: %d %v", code, out)
	}
	code, out = s.get(t, csID, "/api/report-klinik/transaksi-harian/options")
	if code != http.StatusOK || out["today"] != "2026-11-19" || len(out["pelayanan"].([]any)) != 2 || len(out["pembayaran"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
}

func TestAllStok(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/report-klinik/all-stok")
	p := s.repo.lastS
	if code != http.StatusOK || p.From != "2026-11-19" || p.To != "2026-11-19" || strings.Join(p.Branches, ",") != "KMJ,MED" ||
		p.Sort != "hargaJual" || !p.Desc || p.Limit != 10 {
		t.Fatalf("default (today, dearest first): %d %v %+v", code, out, p)
	}
	if rows := out["rows"].([]any); len(rows) != 1 || rows[0].(map[string]any)["stokAwal"] != 3.0 || rows[0].(map[string]any)["distributor"] != "APOTIK MIRAKEL" {
		t.Errorf("rows: %v", out["rows"])
	}
	s.get(t, csID, "/api/report-klinik/all-stok?from=2026-11-01&to=2026-11-08&cabang=med&nama=+vaksin+&jenis=61&movingPo=4&q=2857&sort=stokAkhir&dir=asc&size=25&page=3")
	p = s.repo.lastS
	if p.From != "2026-11-01" || p.To != "2026-11-08" || strings.Join(p.Branches, ",") != "MED" || p.Nama != "vaksin" || p.IDJenis == nil || *p.IDJenis != 61 ||
		p.IDMovingPO == nil || *p.IDMovingPO != 4 || p.Search != "2857" || p.Sort != "stokAkhir" || p.Desc || p.Offset != 50 {
		t.Errorf("filtered: %+v", p)
	}
	before := s.repo.calls
	for path, want := range map[string]int{
		"/api/report-klinik/all-stok?cabang=CTM":                    http.StatusForbidden,
		"/api/report-klinik/all-stok?from=2026-11-10&to=2026-11-01": http.StatusBadRequest,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if code, out := s.get(t, loneID, "/api/report-klinik/all-stok"); code != http.StatusOK || out["total"] != 0.0 || s.repo.calls != before {
		t.Errorf("rejected or branchless requests: %d %v", code, out)
	}
	code, out = s.get(t, csID, "/api/report-klinik/all-stok/options")
	if code != http.StatusOK || out["today"] != "2026-11-19" || len(out["jenis"].([]any)) != 2 || len(out["movingPo"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
}

func TestStokOpname(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/stok-farmasi/stok-opname")
	p := s.repo.lastSO
	if code != http.StatusOK || strings.Join(p.Branches, ",") != "KMJ,MED" || p.Sort != "idObat" || p.Desc || p.Limit != 10 || p.Aktif != nil {
		t.Fatalf("default (by kode): %d %v %+v", code, out, p)
	}
	if rows := out["rows"].([]any); len(rows) != 1 || rows[0].(map[string]any)["satuanJual"] != "STRIP" || rows[0].(map[string]any)["stokMin"] != 2.0 {
		t.Errorf("rows: %v", out["rows"])
	}
	s.get(t, csID, "/api/stok-farmasi/stok-opname?cabang=med&nama=+ator+&jenis=3&aktif=0&q=3031&sort=stok&dir=desc&size=25&page=2")
	p = s.repo.lastSO
	if strings.Join(p.Branches, ",") != "MED" || p.Nama != "ator" || p.IDJenis == nil || *p.IDJenis != 3 || p.Aktif == nil || *p.Aktif ||
		p.Search != "3031" || p.Sort != "stok" || !p.Desc || p.Offset != 25 {
		t.Errorf("filtered: %+v", p)
	}
	before := s.repo.calls
	if code, _ := s.get(t, csID, "/api/stok-farmasi/stok-opname?cabang=CTM"); code != http.StatusForbidden {
		t.Errorf("other branch: %d, want 403", code)
	}
	if code, out := s.get(t, loneID, "/api/stok-farmasi/stok-opname"); code != http.StatusOK || out["total"] != 0.0 || s.repo.calls != before {
		t.Errorf("rejected or branchless requests: %d %v", code, out)
	}
	code, out = s.get(t, csID, "/api/stok-farmasi/stok-opname/options")
	if code != http.StatusOK || len(out["jenis"].([]any)) != 2 || len(out["branches"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
}

func TestMinimStok(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/stok-farmasi/stok-alert/minim-stok")
	p := s.repo.lastSO
	if code != http.StatusOK || !p.BelowMin || p.Sort != "nama" || p.Desc || strings.Join(p.Branches, ",") != "KMJ,MED" {
		t.Fatalf("default (below minimum, by name): %d %v %+v", code, out, p)
	}
	s.get(t, csID, "/api/stok-farmasi/stok-opname")
	if s.repo.lastSO.BelowMin {
		t.Errorf("stok opname must list every row")
	}
	if code, _ := s.get(t, csID, "/api/stok-farmasi/stok-alert/minim-stok?cabang=CTM"); code != http.StatusForbidden {
		t.Errorf("other branch: %d", code)
	}
	if code, out := s.get(t, csID, "/api/stok-farmasi/stok-alert/options"); code != http.StatusOK || len(out["branches"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
}

func TestStokTraceObat(t *testing.T) {
	s := newServer(t)
	if code, out := s.get(t, csID, "/api/stok-farmasi/stok-trace/obat?cabang=med"); code != http.StatusOK || s.repo.obatFor != "MED" || len(out["rows"].([]any)) != 1 {
		t.Errorf("obat: %d %v", code, out)
	}
	if code, _ := s.get(t, csID, "/api/stok-farmasi/stok-trace/obat"); code != http.StatusBadRequest {
		t.Errorf("no cabang: %d", code)
	}
	if code, _ := s.get(t, csID, "/api/stok-farmasi/stok-trace/obat?cabang=CTM"); code != http.StatusForbidden {
		t.Errorf("other branch: %d", code)
	}
}

func TestStokTrace(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/stok-farmasi/stok-trace?cabang=med&obat=1553&from=2026-10-27&to=2026-11-19")
	p := s.repo.lastTr
	if code != http.StatusOK || p.Cabang != "MED" || p.IDObat == nil || *p.IDObat != 1553 || p.From != "2026-10-27" || p.To != "2026-11-19" ||
		p.Sort != "id" || p.Desc || p.Limit != 10 {
		t.Fatalf("default (oldest first): %d %v %+v", code, out, p)
	}
	if rows := out["rows"].([]any); len(rows) != 1 || rows[0].(map[string]any)["transaksi"] != "Penjualan Obat" || rows[0].(map[string]any)["lastStock"] != 5.0 {
		t.Errorf("rows: %v", out["rows"])
	}
	s.get(t, csID, "/api/stok-farmasi/stok-trace?cabang=KMJ&obat=0&q=x&sort=stock&dir=desc&size=25&page=2")
	if p = s.repo.lastTr; p.Cabang != "KMJ" || p.IDObat != nil || p.From != "2026-11-19" || p.Search != "x" || p.Sort != "stock" || !p.Desc || p.Offset != 25 {
		t.Errorf("every drug, today: %+v", p)
	}
	before := s.repo.calls
	for path, want := range map[string]int{
		"/api/stok-farmasi/stok-trace":                                          http.StatusBadRequest,
		"/api/stok-farmasi/stok-trace?cabang=CTM":                               http.StatusForbidden,
		"/api/stok-farmasi/stok-trace?cabang=MED&from=2026-11-10&to=2026-11-01": http.StatusBadRequest,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if code, _ := s.get(t, loneID, "/api/stok-farmasi/stok-trace?cabang=MED"); code != http.StatusForbidden || s.repo.calls != before {
		t.Errorf("branchless user: %d", code)
	}
}

func TestRujukan(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/report-klinik/rujukan")
	p := s.repo.lastR
	if code != http.StatusOK || p.From != "2026-11-19" || strings.Join(p.Branches, ",") != "KMJ,MED" || p.Sort != "tgl" || !p.Desc || p.Limit != 20 {
		t.Fatalf("default (today, newest first, 20 a page): %d %v %+v", code, out, p)
	}
	rows := out["rows"].([]any)
	if len(rows) != 2 || rows[0].(map[string]any)["idLine"] != 7.0 || rows[1].(map[string]any)["idLine"] != nil || rows[1].(map[string]any)["total"] != 0.0 {
		t.Errorf("rows (a sale without examinations has idLine null and total 0): %v", rows)
	}
	s.get(t, csID, "/api/report-klinik/rujukan?from=2026-10-22&to=2026-11-20&cabang=kmj&nama=+candra+&dokter=+sitti+&status=close&q=x&sort=total&dir=asc&size=10&page=3")
	p = s.repo.lastR
	if p.From != "2026-10-22" || strings.Join(p.Branches, ",") != "KMJ" || p.Nama != "candra" || p.Dokter != "sitti" ||
		p.Status != StatusClose || p.Search != "x" || p.Sort != "total" || p.Desc || p.Offset != 20 {
		t.Errorf("filtered: %+v", p)
	}
	before := s.repo.calls
	for path, want := range map[string]int{
		"/api/report-klinik/rujukan?cabang=CTM":  http.StatusForbidden,
		"/api/report-klinik/rujukan?from=bad":    http.StatusBadRequest,
		"/api/report-klinik/rujukan?status=paid": http.StatusBadRequest,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if code, out := s.get(t, loneID, "/api/report-klinik/rujukan"); code != http.StatusOK || out["total"] != 0.0 || s.repo.calls != before {
		t.Errorf("rejected or branchless requests: %d %v", code, out)
	}
	code, out = s.get(t, csID, "/api/report-klinik/rujukan/options")
	if code != http.StatusOK || out["today"] != "2026-11-19" || len(out["branches"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
}

func TestPenjualanSoftdrink(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/report-klinik/penjualan-softdrink?nama=teh&perujuk=7")
	p := s.repo.lastO
	if code != http.StatusOK || p.Jenis != JenisSoftDrink || p.Nama != "teh" || p.IDPerujuk == nil || *p.IDPerujuk != 7 || p.Sort != "idJual" || !p.Desc {
		t.Fatalf("softdrink: %d %v %+v", code, out, p)
	}
	// The OTC report asks for OTC drugs.
	s.get(t, csID, "/api/report-klinik/penjualan-otc")
	if p = s.repo.lastO; p.Jenis != JenisOTC {
		t.Errorf("otc: %+v", p)
	}
	if code, _ := s.get(t, csID, "/api/report-klinik/penjualan-softdrink?cabang=CTM"); code != http.StatusForbidden {
		t.Errorf("another branch: %d", code)
	}
	if code, out := s.get(t, csID, "/api/report-klinik/penjualan-softdrink/options"); code != http.StatusOK || len(out["perujuk"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
}

func TestPendaftaran(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/report-klinik/pendaftaran")
	p := s.repo.lastD
	if code != http.StatusOK || p.From != "2026-11-19" || strings.Join(p.Branches, ",") != "KMJ,MED" || p.Sort != "id" || !p.Desc || p.Limit != 10 {
		t.Fatalf("default (today, newest first): %d %v %+v", code, out, p)
	}
	if rows := out["rows"].([]any); len(rows) != 1 || rows[0].(map[string]any)["noAntri"] != 10.0 || rows[0].(map[string]any)["statusKun"] != "CLOSE" {
		t.Errorf("rows: %v", out["rows"])
	}
	s.get(t, csID, "/api/report-klinik/pendaftaran?from=2026-11-01&to=2026-11-08&cabang=kmj&nama=+arya+&pelayanan=2&statusKunjungan=3&q=518790&sort=pasien&dir=asc&size=20&page=2")
	p = s.repo.lastD
	if p.From != "2026-11-01" || strings.Join(p.Branches, ",") != "KMJ" || p.Nama != "arya" || p.IDJenisRawat == nil || *p.IDJenisRawat != 2 ||
		p.IDStatus == nil || *p.IDStatus != 3 || p.Search != "518790" || p.Sort != "pasien" || p.Desc || p.Offset != 20 {
		t.Errorf("filtered: %+v", p)
	}
	s.get(t, csID, "/api/report-klinik/pendaftaran?sort=umur&dir=asc")
	if p = s.repo.lastD; p.Sort != "umur" || !p.Desc {
		t.Errorf("umur asc should sort birth dates desc: %+v", p)
	}
	before := s.repo.calls
	for path, want := range map[string]int{
		"/api/report-klinik/pendaftaran?cabang=CTM": http.StatusForbidden,
		"/api/report-klinik/pendaftaran?from=bad":   http.StatusBadRequest,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if code, out := s.get(t, loneID, "/api/report-klinik/pendaftaran"); code != http.StatusOK || out["total"] != 0.0 || s.repo.calls != before {
		t.Errorf("rejected or branchless requests: %d %v", code, out)
	}
	code, out = s.get(t, csID, "/api/report-klinik/pendaftaran/options")
	if code != http.StatusOK || len(out["pelayanan"].([]any)) != 2 || len(out["statusKunjungan"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
}

func TestPendapatan(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/report-klinik/pendapatan?cabang=kmj&from=2026-11-19&to=2026-11-20&pelayanan=2&dokter=10")
	p := s.repo.lastI
	if code != http.StatusOK || p.Cabang != "KMJ" || p.From != "2026-11-19" || p.To != "2026-11-20" || p.IDJenisRawat == nil || *p.IDJenisRawat != 2 ||
		p.IDDokter == nil || *p.IDDokter != 10 {
		t.Fatalf("pendapatan: %d %v %+v", code, out, p)
	}
	// Petugas is the e-mail without its domain; Tanggal Posting is today in Jakarta.
	if out["petugas"] != "cs" || out["today"] != "2026-11-19" || out["namaCabang"] != "KLINIK MITRA JATINEGARA" || len(out["rows"].([]any)) != 1 {
		t.Errorf("response: %v", out)
	}
	// Without dates the period is today in Jakarta.
	if code, _ := s.get(t, csID, "/api/report-klinik/pendapatan?cabang=KMJ"); code != http.StatusOK || s.repo.lastI.From != "2026-11-19" || s.repo.lastI.To != "2026-11-19" {
		t.Errorf("default period: %d %+v", code, s.repo.lastI)
	}
	before := s.repo.calls
	for path, want := range map[string]int{
		"/api/report-klinik/pendapatan":                                          http.StatusBadRequest, // Cabang is required
		"/api/report-klinik/pendapatan?cabang=CTM":                               http.StatusForbidden,
		"/api/report-klinik/pendapatan?cabang=KMJ&from=2026-11-20&to=2026-11-19": http.StatusBadRequest,
		"/api/report-klinik/pendapatan/dokter":                                   http.StatusBadRequest,
		"/api/report-klinik/pendapatan/dokter?cabang=CTM":                        http.StatusForbidden,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if s.repo.calls != before {
		t.Errorf("rejected requests reached the repository")
	}
	code, out = s.get(t, csID, "/api/report-klinik/pendapatan/dokter?cabang=med")
	if code != http.StatusOK || s.repo.lastDok != "MED" || len(out["rows"].([]any)) != 2 {
		t.Errorf("dokter: %d %v", code, out)
	}
	code, out = s.get(t, csID, "/api/report-klinik/pendapatan/options")
	if code != http.StatusOK || len(out["pelayanan"].([]any)) != 2 || len(out["branches"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
}

func TestReportDiagnosa(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/report-klinik/diagnosa")
	p := s.repo.lastX
	if code != http.StatusOK || p.From != "2026-11-19" || strings.Join(p.Branches, ",") != "KMJ,MED" || p.Sort != "idJual" || !p.Desc || p.Limit != 5 {
		t.Fatalf("default (today, newest sale first, 5 a page): %d %v %+v", code, out, p)
	}
	if rows := out["rows"].([]any); len(rows) != 1 || rows[0].(map[string]any)["icd"] != "J06" || rows[0].(map[string]any)["resep"] != "SANMOL (8)" {
		t.Errorf("rows: %v", out["rows"])
	}
	s.get(t, csID, "/api/report-klinik/diagnosa?from=2026-11-01&to=2026-11-08&cabang=kmj&nama=+retno+&diagnosa=+J06+&pelayanan=2&q=x&sort=icd&dir=asc&size=20&page=2")
	p = s.repo.lastX
	if p.From != "2026-11-01" || strings.Join(p.Branches, ",") != "KMJ" || p.Nama != "retno" || p.Diagnosa != "J06" || p.IDJenisRawat == nil ||
		*p.IDJenisRawat != 2 || p.Search != "x" || p.Sort != "icd" || p.Desc || p.Offset != 20 {
		t.Errorf("filtered: %+v", p)
	}
	s.get(t, csID, "/api/report-klinik/diagnosa?sort=usia&dir=asc")
	if p = s.repo.lastX; p.Sort != "usia" || !p.Desc {
		t.Errorf("usia asc should sort birth dates desc: %+v", p)
	}
	before := s.repo.calls
	for path, want := range map[string]int{
		"/api/report-klinik/diagnosa?cabang=CTM": http.StatusForbidden,
		"/api/report-klinik/diagnosa?from=bad":   http.StatusBadRequest,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if code, out := s.get(t, loneID, "/api/report-klinik/diagnosa"); code != http.StatusOK || out["total"] != 0.0 || s.repo.calls != before {
		t.Errorf("rejected or branchless requests: %d %v", code, out)
	}
	if code, out := s.get(t, csID, "/api/report-klinik/diagnosa/options"); code != http.StatusOK || len(out["pelayanan"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
}

func TestReportDinkes(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/report-klinik/icd-dinkes?cabang=kmj")
	p := s.repo.lastK
	if code != http.StatusOK || p.From != "2026-11-19" || strings.Join(p.Branches, ",") != "KMJ" || p.Sort != "kode" || !p.Desc || p.Limit != 10 {
		t.Fatalf("default (today, one branch, highest code first): %d %v %+v", code, out, p)
	}
	if rows := out["rows"].([]any); len(rows) != 1 || rows[0].(map[string]any)["tahun20_44"] != 2.0 || rows[0].(map[string]any)["total"] != 2.0 {
		t.Errorf("rows: %v", out["rows"])
	}
	s.get(t, csID, "/api/report-klinik/icd-dinkes?from=2026-11-01&to=2026-11-08&q=+J06+&sort=total&dir=asc&size=20&page=2")
	p = s.repo.lastK
	if p.From != "2026-11-01" || strings.Join(p.Branches, ",") != "KMJ,MED" || p.Search != "J06" || p.Sort != "total" || p.Desc || p.Offset != 20 {
		t.Errorf("filtered: %+v", p)
	}
	before := s.repo.calls
	for path, want := range map[string]int{
		"/api/report-klinik/icd-dinkes?cabang=CTM": http.StatusForbidden,
		"/api/report-klinik/icd-dinkes?from=bad":   http.StatusBadRequest,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if code, out := s.get(t, loneID, "/api/report-klinik/icd-dinkes"); code != http.StatusOK || out["total"] != 0.0 || s.repo.calls != before {
		t.Errorf("rejected or branchless requests: %d %v", code, out)
	}
	if code, out := s.get(t, csID, "/api/report-klinik/icd-dinkes/options"); code != http.StatusOK || len(out["branches"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
}
