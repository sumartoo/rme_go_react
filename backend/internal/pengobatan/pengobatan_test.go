package pengobatan

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
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
	csID   = "22222222-2222-2222-2222-222222222222" // mapped to MED and KMJ
	loneID = "33333333-3333-3333-3333-333333333333" // mapped to no branch
)

func ptr[T any](v T) *T { return &v }

// 23:30 UTC on 18 Nov is already 19 Nov in Jakarta.
var now = time.Date(2026, 11, 18, 23, 30, 0, 0, time.UTC)

func TestWhere(t *testing.T) {
	sql, args := where(ListParams{
		Branches: []string{"MED"}, From: "2026-11-18", To: "2026-11-19", Search: "522830", Nama: "obat",
		IDJenisRawat: ptr(2), IDStatusKunjung: ptr(2), IDJenisBayar: ptr(1), Status: StatusCancel,
	}, true)
	for _, want := range []string{
		"j.cabang = ANY($1)", "j.tgl >= $2::date", "j.tgl < $3::date + 1", "p.nama ILIKE $4", "j.id_jenis_rwt = $5",
		"d.id = j.id_kun AND d.id_statustr = $6", "j.id_jenis_byr = $7", "ELSE 'open' END = $8", "j.no_kwi ILIKE $9", "j.id = $10",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("where missing %q: %s", want, sql)
		}
	}
	if len(args) != 10 || args[3] != "%obat%" || args[7] != "cancel" {
		t.Errorf("args = %v", args)
	}
	// The "filtered from" count keeps the panel filters and drops only the search box.
	sql, args = where(ListParams{Branches: []string{"MED"}, From: "2026-11-18", To: "2026-11-18", Search: "x", Nama: "a_b"}, false)
	if strings.Contains(sql, "no_kwi") || len(args) != 4 || args[3] != `%a\_b%` {
		t.Errorf("without search: %s %v", sql, args)
	}
}

type fakeRepo struct {
	userBranches  map[string][]branchauth.Branch
	lastList      ListParams
	lists         int
	lastReg       RegParams
	regs          int
	lastDokter    string
	lastObat      string
	lastTindakan  string
	lastPenunjang string
	lastICD       icd.ListParams
	lastPerawat   PerawatParams
	resep         map[int]ResepInput
	regs2         map[int][2]any // idKun -> {cabang, status}
	saved         map[int]Pemeriksaan
	savedBy       string
	saleIn        SaveInput
	saleErr       error
	sales         int
	prints        []string
	cancels       []string // "id reason user"
	cancelled     bool
}

func (f *fakeRepo) UserBranches(_ context.Context, userID string) ([]branchauth.Branch, error) {
	return f.userBranches[userID], nil
}
func (f *fakeRepo) List(_ context.Context, p ListParams) (Page, error) {
	f.lastList, f.lists = p, f.lists+1
	return Page{Rows: []Row{{ID: 1, Status: StatusClose}}, Total: 1, UnfilteredTotal: 1}, nil
}
func (f *fakeRepo) JenisRawat(context.Context) ([]Option, error) {
	return []Option{{ID: 2, Name: "POLI UMUM"}}, nil
}
func (f *fakeRepo) StatusKunjungan(context.Context) ([]Option, error) {
	return []Option{{ID: 2, Name: "CLOSE"}}, nil
}
func (f *fakeRepo) JenisBayar(context.Context) ([]Option, error) {
	return []Option{{ID: 1, Name: "TUNAI"}}, nil
}
func (f *fakeRepo) Pendaftaran(_ context.Context, p RegParams) (RegPage, error) {
	f.lastReg, f.regs = p, f.regs+1
	return RegPage{Rows: []RegRow{}}, nil
}
func (f *fakeRepo) Dokter(_ context.Context, branch string) ([]Dokter, error) {
	f.lastDokter = branch
	if branch == "MED" {
		return []Dokter{{ID: 1, Nama: "NON DOKTER"}, {ID: 6, Nama: "dr. A", Jenis: "DOKTER UMUM"}}, nil
	}
	return []Dokter{}, nil
}
func (f *fakeRepo) Obat(_ context.Context, branch string) ([]Obat, error) {
	f.lastObat = branch
	return []Obat{{ID: 1, Nama: "ACESSORIES", Harga: 175000, Stok: 505}}, nil
}
func (f *fakeRepo) Tindakan(_ context.Context, branch string, dokterID int) ([]Tindakan, error) {
	f.lastTindakan = fmt.Sprintf("%s/%d", branch, dokterID)
	return []Tindakan{{ID: 41, Nama: "TINDAKAN DOKTER GIGI", Harga: 1750000, IsEdit: true}}, nil
}
func (f *fakeRepo) Penunjang(_ context.Context, branch string) ([]Penunjang, error) {
	f.lastPenunjang = branch
	return []Penunjang{{ID: 1, Nama: "ASAM URAT", Jenis: "LAB MOHC", Harga: 77000}}, nil
}
func (f *fakeRepo) ICD(_ context.Context, lp icd.ListParams) (icd.Page, error) {
	f.lastICD = lp
	return icd.Page{Rows: []icd.ICD{{ID: 1, Kode: "K04.3", Nama: "ABNORMAL HARD TISSUE FORMATION IN PULP", Jenis: "GIGI"}}, Total: 1, UnfilteredTotal: 1}, nil
}
func (f *fakeRepo) RegBranch(_ context.Context, idKun int) (string, int, error) {
	r, ok := f.regs2[idKun]
	if !ok {
		return "", 0, ErrRegNotFound
	}
	return r[0].(string), r[1].(int), nil
}
func (f *fakeRepo) GetPemeriksaan(_ context.Context, idKun int) (*PemeriksaanRecord, error) {
	p, ok := f.saved[idKun]
	if !ok {
		return nil, nil
	}
	return &PemeriksaanRecord{Pemeriksaan: p, UpdatedBy: f.savedBy}, nil
}
func (f *fakeRepo) PerawatList(_ context.Context, p PerawatParams) (PerawatPage, error) {
	f.lastPerawat = p
	return PerawatPage{Rows: []PerawatRow{}}, nil
}
func (f *fakeRepo) RegByID(_ context.Context, idKun int) (RegRow, error) {
	r, ok := f.regs2[idKun]
	if !ok {
		return RegRow{}, ErrRegNotFound
	}
	return RegRow{ID: idKun, Cabang: r[0].(string)}, nil
}
func (f *fakeRepo) DokterAtBranches(_ context.Context, branches []string) ([]Dokter, error) {
	return []Dokter{{ID: 5, Nama: "dr. A"}}, nil
}
func (f *fakeRepo) GetResep(_ context.Context, idKun int) (Resep, error) {
	res := Resep{Obat: []ResepObat{}, Diagnosa: []ResepDiagnosa{}}
	if in, ok := f.resep[idKun]; ok {
		for _, o := range in.Obat {
			res.Obat = append(res.Obat, ResepObat{IDObat: o.IDObat, Byk: o.Byk, Dosis: o.Dosis})
		}
		for _, id := range in.Diagnosa {
			res.Diagnosa = append(res.Diagnosa, ResepDiagnosa{ICDID: id})
		}
	}
	return res, nil
}
func (f *fakeRepo) SaveResep(_ context.Context, idKun int, in ResepInput) error {
	for _, o := range in.Obat {
		if o.IDObat == 999 {
			return ErrUnknownObat
		}
	}
	if f.resep == nil {
		f.resep = map[int]ResepInput{}
	}
	f.resep[idKun] = in
	return nil
}
func (f *fakeRepo) SavePemeriksaan(_ context.Context, idKun int, in Pemeriksaan, user string) error {
	if in.IDDokter != nil && *in.IDDokter == 999 {
		return ErrUnknownDokter
	}
	if f.saved == nil {
		f.saved = map[int]Pemeriksaan{}
	}
	f.saved[idKun], f.savedBy = in, user
	return nil
}
func (f *fakeRepo) Save(_ context.Context, in SaveInput, _ string) (SaveResult, error) {
	f.saleIn, f.sales = in, f.sales+1
	if f.saleErr != nil {
		return SaveResult{}, f.saleErr
	}
	return SaveResult{ID: 600001, NoKwi: "2611KMJ0001", Total: 175000, GrandTotal: 175000}, nil
}
func (f *fakeRepo) Kuitansi(_ context.Context, id int) (Kuitansi, error) {
	switch id {
	case 522954:
		return Kuitansi{ID: id, NoKwi: "2611KMJ0438", Cabang: "KMJ", Jumlah: 50000}, nil
	case 522000:
		return Kuitansi{ID: id, NoKwi: "2611CTM0001", Cabang: "CTM"}, nil
	}
	return Kuitansi{}, ErrSaleNotFound
}
func (f *fakeRepo) Cetak(ctx context.Context, id int, user string) (Kuitansi, error) {
	f.prints = append(f.prints, user)
	k, _ := f.Kuitansi(ctx, id)
	k.NoCetak, k.PrintedUser = "KMJ-26-11-00001", user
	return k, nil
}
func (f *fakeRepo) Nota(_ context.Context, id int) (Nota, error) {
	return Nota{ID: id, NoKwi: "2611KMJ0438", Obat: []NotaLine{{Nama: "MICROGINON", Byk: 1, Harga: 30000, Jumlah: 30000}}}, nil
}
func (f *fakeRepo) Detail(_ context.Context, id int) (Detail, error) {
	status := "close"
	if f.cancelled {
		status = "cancel"
	}
	return Detail{ID: id, NoKwi: "2611KMJ0438", Cabang: "KMJ", Status: status, Obat: []DetailObat{{IDObat: 366, Nama: "MINYAK KAPAK", Byk: 1, Harga: 18000}},
		Tindakan: []DetailTindakan{}, Penunjang: []DetailTindakan{}, Diagnosa: []DetailICD{}}, nil
}
func (f *fakeRepo) Cancel(_ context.Context, id int, reason, user string) error {
	if f.cancelled {
		return ErrAlreadyCancelled
	}
	f.cancelled = true
	f.cancels = append(f.cancels, fmt.Sprintf("%d %s %s", id, reason, user))
	return nil
}
func (f *fakeRepo) ProsesStatus(context.Context) ([]Option, error) {
	return []Option{{ID: 3, Name: "DAFTAR"}, {ID: 4, Name: "PROSES"}, {ID: 5, Name: "PAYMENT"}}, nil
}

type server struct {
	mux    *http.ServeMux
	tokens *auth.TokenManager
	repo   *fakeRepo
}

func newServer(t *testing.T) *server {
	t.Helper()
	repo := &fakeRepo{userBranches: map[string][]branchauth.Branch{
		csID: {{Kode: "KMJ", Nama: "KLINIK MITRA JATINEGARA"}, {Kode: "MED", Nama: "KLINIK MEDIKA"}},
	}}
	tokens := auth.NewTokenManager("test-secret-that-is-at-least-32-characters", time.Minute)
	guarded := map[string]int{}
	requireMenu := func(path string) func(http.Handler) http.Handler {
		if path != PagePath && path != ReportPath && path != PerawatPath && path != DokterPath && path != ProsesObatPath {
			t.Fatalf("guarded by %q", path)
		}
		guarded[path]++
		return func(next http.Handler) http.Handler { return next }
	}
	h := NewHandler(repo)
	h.now = func() time.Time { return now }
	mux := http.NewServeMux()
	h.Routes(mux, tokens.Middleware, requireMenu)
	// Raw Data Pengobatan (the list and its options) is guarded by the report menu.
	if guarded[PagePath] != 16 || guarded[ReportPath] != 2 || guarded[PerawatPath] != 6 || guarded[DokterPath] != 10 || guarded[ProsesObatPath] != 5 {
		t.Fatalf("routes guarded: %v", guarded)
	}
	return &server{mux: mux, tokens: tokens, repo: repo}
}

func (s *server) get(t *testing.T, userID, path string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("GET", path, nil)
	if userID != "" {
		tok, _, _ := s.tokens.Generate(userID, "cs@medika.com")
		req.Header.Set("Authorization", "Bearer "+tok)
	}
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestList(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/pengobatan")
	lp := s.repo.lastList
	if code != http.StatusOK || out["from"] != "2026-11-19" || lp.From != "2026-11-19" || lp.To != "2026-11-19" ||
		strings.Join(lp.Branches, ",") != "KMJ,MED" || lp.Sort != "id" || !lp.Desc || lp.Limit != 10 {
		t.Fatalf("default list (today in Jakarta, newest first): %d %v %+v", code, out, lp)
	}
	s.get(t, csID, "/api/pengobatan?from=2026-11-01&to=2026-11-08&cabang=med&nama=+siti+&pelayanan=2&statusKunjungan=2&pembayaran=7&status=Cancel&q=2611&sort=total&dir=asc&size=20&page=3")
	lp = s.repo.lastList
	if lp.From != "2026-11-01" || lp.To != "2026-11-08" || strings.Join(lp.Branches, ",") != "MED" || lp.Nama != "siti" ||
		*lp.IDJenisRawat != 2 || *lp.IDStatusKunjung != 2 || *lp.IDJenisBayar != 7 || lp.Status != StatusCancel ||
		lp.Search != "2611" || lp.Sort != "total" || lp.Desc || lp.Offset != 40 {
		t.Errorf("filtered list: %+v", lp)
	}
	s.get(t, csID, "/api/pengobatan?sort=drop+table&size=999999")
	if lp = s.repo.lastList; lp.Sort != "id" || !lp.Desc || lp.Limit != MaxPageSize {
		t.Errorf("unknown sort / huge size: %+v", lp)
	}

	before := s.repo.lists
	for path, want := range map[string]int{
		"/api/pengobatan?cabang=CTM":                    http.StatusForbidden,
		"/api/pengobatan?from=2026-11-10&to=2026-11-01": http.StatusBadRequest,
		"/api/pengobatan?from=18-11-2026":               http.StatusBadRequest,
		"/api/pengobatan?status=paid":                   http.StatusBadRequest,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if s.repo.lists != before {
		t.Errorf("rejected requests reached the repository")
	}
	if code, out := s.get(t, loneID, "/api/pengobatan"); code != http.StatusOK || out["total"] != 0.0 || s.repo.lists != before {
		t.Errorf("user without branches: %d %v", code, out)
	}
	if code, _ := s.get(t, "", "/api/pengobatan"); code != http.StatusUnauthorized {
		t.Errorf("no token: %d", code)
	}
}

func TestOptions(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/pengobatan/options")
	if code != http.StatusOK || out["today"] != "2026-11-19" || len(out["branches"].([]any)) != 2 ||
		len(out["pelayanan"].([]any)) != 1 || len(out["statusKunjungan"].([]any)) != 1 || len(out["pembayaran"].([]any)) != 1 ||
		len(out["proses"].([]any)) != 3 {
		t.Errorf("options: %d %v", code, out)
	}
	if code, out := s.get(t, loneID, "/api/pengobatan/options"); code != http.StatusOK || out["branches"] == nil {
		t.Errorf("options without branches must return an empty list: %d %v", code, out)
	}
}

func TestRegWhere(t *testing.T) {
	sql, args := regWhere(RegParams{Branch: "CTM", From: "2026-11-18", To: "2026-11-19", Nama: "lola", IDJenisRawat: ptr(2), Search: "518586"}, true)
	for _, want := range []string{
		"d.cabang = $1", "d.tgl_masuk >= $2::date", "d.tgl_masuk < $3::date + 1", "d.id_statustr = ANY($4)",
		"NOT EXISTS (SELECT 1 FROM trxjual j WHERE j.id_kun = d.id)", "p.nama ILIKE $5", "d.id_jenis_rwt = $6", "d.id = $8",
	} {
		if !strings.Contains(sql, want) {
			t.Errorf("regWhere missing %q: %s", want, sql)
		}
	}
	if st, ok := args[3].([]int); !ok || len(st) != 3 {
		t.Errorf("default statuses = %v", args[3])
	}
	_, args = regWhere(RegParams{Branch: "CTM", From: "2026-11-18", To: "2026-11-19", IDStatus: ptr(4)}, true)
	if st := args[3].([]int); len(st) != 1 || st[0] != 4 {
		t.Errorf("status filter = %v", st)
	}
}

func TestPendaftaranLookup(t *testing.T) {
	s := newServer(t)
	// 23:30 UTC on 18 Nov is 19 Nov in Jakarta: the default range is 18-19 Nov.
	code, out := s.get(t, csID, "/api/pengobatan/pendaftaran?cabang=kmj")
	rp := s.repo.lastReg
	if code != http.StatusOK || rp.Branch != "KMJ" || rp.From != "2026-11-18" || rp.To != "2026-11-19" || rp.Sort != "id" || !rp.Desc || rp.Limit != 5 || rp.IDStatus != nil {
		t.Fatalf("default lookup: %d %v %+v", code, out, rp)
	}
	s.get(t, csID, "/api/pengobatan/pendaftaran?cabang=MED&from=2026-11-01&to=2026-11-08&nama=+lola+&pelayanan=2&status=4&q=48-01&sort=nama&dir=asc&size=10&page=2")
	rp = s.repo.lastReg
	if rp.Branch != "MED" || rp.From != "2026-11-01" || rp.Nama != "lola" || *rp.IDJenisRawat != 2 || *rp.IDStatus != 4 || rp.Search != "48-01" || rp.Sort != "nama" || rp.Desc || rp.Offset != 10 {
		t.Errorf("filtered lookup: %+v", rp)
	}
	before := s.repo.regs
	for path, want := range map[string]int{
		"/api/pengobatan/pendaftaran":                                          http.StatusBadRequest, // no branch
		"/api/pengobatan/pendaftaran?cabang=CTM":                               http.StatusForbidden,
		"/api/pengobatan/pendaftaran?cabang=KMJ&status=2":                      http.StatusBadRequest, // CLOSE is not billable
		"/api/pengobatan/pendaftaran?cabang=KMJ&from=2026-11-19&to=2026-11-01": http.StatusBadRequest,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if code, _ := s.get(t, loneID, "/api/pengobatan/pendaftaran?cabang=KMJ"); code != http.StatusForbidden {
		t.Errorf("user without branches: %d", code)
	}
	if s.repo.regs != before {
		t.Errorf("rejected requests reached the repository")
	}
}

func TestDokterLookup(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/pengobatan/dokter?cabang=med")
	rows, _ := out["rows"].([]any)
	if code != http.StatusOK || s.repo.lastDokter != "MED" || len(rows) != 2 {
		t.Fatalf("doctors of MED: %d %v", code, out)
	}
	if code, out := s.get(t, csID, "/api/pengobatan/dokter?cabang=KMJ"); code != http.StatusOK || out["rows"] == nil {
		t.Errorf("a branch without doctors must return an empty list: %d %v", code, out)
	}
	s.repo.lastDokter = ""
	for path, want := range map[string]int{
		"/api/pengobatan/dokter":            http.StatusBadRequest,
		"/api/pengobatan/dokter?cabang=CTM": http.StatusForbidden,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if s.repo.lastDokter != "" {
		t.Errorf("rejected requests reached the repository")
	}
}

func TestObatLookup(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/pengobatan/obat?cabang=kmj")
	rows, _ := out["rows"].([]any)
	if code != http.StatusOK || s.repo.lastObat != "KMJ" || len(rows) != 1 {
		t.Fatalf("drugs of KMJ: %d %v", code, out)
	}
	s.repo.lastObat = ""
	for path, want := range map[string]int{
		"/api/pengobatan/obat":            http.StatusBadRequest,
		"/api/pengobatan/obat?cabang=CTM": http.StatusForbidden,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if s.repo.lastObat != "" {
		t.Errorf("rejected requests reached the repository")
	}
}

func TestTindakanLookup(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/pengobatan/tindakan?cabang=med&dokter=13")
	rows, _ := out["rows"].([]any)
	if code != http.StatusOK || s.repo.lastTindakan != "MED/13" || len(rows) != 1 {
		t.Fatalf("procedures of doctor 13 at MED: %d %v", code, out)
	}
	s.repo.lastTindakan = ""
	for path, want := range map[string]int{
		"/api/pengobatan/tindakan?cabang=MED":           http.StatusBadRequest, // no doctor
		"/api/pengobatan/tindakan?cabang=MED&dokter=x":  http.StatusBadRequest,
		"/api/pengobatan/tindakan?dokter=13":            http.StatusBadRequest, // no branch
		"/api/pengobatan/tindakan?cabang=CTM&dokter=13": http.StatusForbidden,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if s.repo.lastTindakan != "" {
		t.Errorf("rejected requests reached the repository")
	}
}

func TestPenunjangLookup(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/pengobatan/penunjang?cabang=med")
	rows, _ := out["rows"].([]any)
	if code != http.StatusOK || s.repo.lastPenunjang != "MED" || len(rows) != 1 {
		t.Fatalf("examinations at MED: %d %v", code, out)
	}
	s.repo.lastPenunjang = ""
	for path, want := range map[string]int{
		"/api/pengobatan/penunjang":            http.StatusBadRequest,
		"/api/pengobatan/penunjang?cabang=CTM": http.StatusForbidden,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if s.repo.lastPenunjang != "" {
		t.Errorf("rejected requests reached the repository")
	}
}

func TestICDLookup(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/pengobatan/icd")
	lp := s.repo.lastICD
	if code != http.StatusOK || out["total"] != 1.0 || lp.Sort != "id" || lp.Desc || lp.Limit != 10 || lp.Offset != 0 {
		t.Fatalf("default ICD page (by id, 10 rows): %d %v %+v", code, out, lp)
	}
	s.get(t, csID, "/api/pengobatan/icd?q=+gingivitis+&sort=nama&dir=desc&size=20&page=3")
	if lp = s.repo.lastICD; lp.Search != "gingivitis" || lp.Sort != "nama" || !lp.Desc || lp.Limit != 20 || lp.Offset != 40 {
		t.Errorf("filtered ICD page: %+v", lp)
	}
	s.get(t, csID, "/api/pengobatan/icd?sort=drop&size=9999")
	if lp = s.repo.lastICD; lp.Sort != "id" || lp.Limit != 100 {
		t.Errorf("unknown sort / huge size: %+v", lp)
	}
	// Codes are not per branch: a user without branches may still look them up.
	if code, _ := s.get(t, loneID, "/api/pengobatan/icd"); code != http.StatusOK {
		t.Errorf("user without branches: %d", code)
	}
}

func (s *server) put(t *testing.T, userID, path, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest("PUT", path, strings.NewReader(body))
	tok, _, _ := s.tokens.Generate(userID, "a-very-long-email-address@medika.com")
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestPemeriksaan(t *testing.T) {
	s := newServer(t)
	s.repo.regs2 = map[int][2]any{
		518586: {"KMJ", 3}, // DAFTAR at the caller's branch
		518001: {"KMJ", 2}, // CLOSE
		518999: {"CTM", 3}, // another branch
	}
	// Nothing saved yet.
	if code, out := s.get(t, csID, "/api/pengobatan/pemeriksaan/518586"); code != http.StatusOK || out["pemeriksaan"] != nil {
		t.Fatalf("empty examination: %d %v", code, out)
	}
	body := `{"keluhanUtama":"  demam 3 hari ","sistole":120,"diastole":80,"beratBadan":60,"tinggiBadan":165,"suhu":38.5,"idDokter":13}`
	code, out := s.put(t, csID, "/api/pengobatan/pemeriksaan/518586", body)
	saved := s.repo.saved[518586]
	if code != http.StatusOK || saved.KeluhanUtama != "demam 3 hari" || *saved.Sistole != 120 || *saved.Suhu != 38.5 || *saved.IDDokter != 13 {
		t.Fatalf("save: %d %v %+v", code, out, saved)
	}
	if s.repo.savedBy != "a-very-long-email-ad" {
		t.Errorf("user must be cut to 20 characters: %q", s.repo.savedBy)
	}
	if code, out := s.get(t, csID, "/api/pengobatan/pemeriksaan/518586"); code != http.StatusOK || out["pemeriksaan"] == nil {
		t.Errorf("saved examination not returned: %d %v", code, out)
	}
	for name, c := range map[string]struct {
		path, body string
		want       int
	}{
		"closed visit":    {"/api/pengobatan/pemeriksaan/518001", "{}", http.StatusConflict},
		"other branch":    {"/api/pengobatan/pemeriksaan/518999", "{}", http.StatusNotFound},
		"unknown visit":   {"/api/pengobatan/pemeriksaan/1", "{}", http.StatusNotFound},
		"bad id":          {"/api/pengobatan/pemeriksaan/x", "{}", http.StatusNotFound},
		"too long":        {"/api/pengobatan/pemeriksaan/518586", `{"keluhanUtama":"` + strings.Repeat("a", 301) + `"}`, http.StatusBadRequest},
		"bad temperature": {"/api/pengobatan/pemeriksaan/518586", `{"suhu":60}`, http.StatusBadRequest},
		"bad pressure":    {"/api/pengobatan/pemeriksaan/518586", `{"sistole":-1}`, http.StatusBadRequest},
		"unknown doctor":  {"/api/pengobatan/pemeriksaan/518586", `{"idDokter":999}`, http.StatusBadRequest},
		"bad json":        {"/api/pengobatan/pemeriksaan/518586", `{`, http.StatusBadRequest},
	} {
		if code, out := s.put(t, csID, c.path, c.body); code != c.want {
			t.Errorf("%s: %d %v, want %d", name, code, out, c.want)
		}
	}
	if code, _ := s.get(t, csID, "/api/pengobatan/pemeriksaan/518999"); code != http.StatusNotFound {
		t.Errorf("other branch read: %d", code)
	}
}

func TestSaleRules(t *testing.T) {
	// Drugs: each line (and its PPN) rounded up to 500.
	for _, c := range []struct {
		harga, byk          float64
		tax                 bool
		jumlah, ppn, billed float64
	}{
		{175000, 1, false, 175000, 0, 175000},
		{4000, 6, true, 24000, 2400, 24000 + 2500},
		{5406, 1, false, 5406, 0, 5500},
		{0.1 * 3, 1000, false, 300, 0, 500}, // floating-point dust is ignored
		{110010, 1, false, 110010, 0, 110500},
	} {
		j, p, b := obatLine(c.harga, c.byk, c.tax)
		if math.Abs(j-c.jumlah) > 1e-6 || math.Abs(p-c.ppn) > 1e-6 || b != c.billed {
			t.Errorf("obatLine(%v, %v, %v) = %v %v %v, want %v %v %v", c.harga, c.byk, c.tax, j, p, b, c.jumlah, c.ppn, c.billed)
		}
	}
	// Settlement, as in lab: cash handed over, settled part, receivable, change.
	pay := func(v float64) *float64 { return &v }
	for _, c := range []struct {
		total, potongan                 float64
		payment                         *float64
		grand, bayar, cicil, utang, kbl float64
	}{
		{302500, 60500, nil, 242000, 242000, 242000, 0, 0},        // paid exactly
		{25000, 0, pay(250000), 25000, 250000, 25000, 0, 225000},  // cash with change
		{285000, 0, pay(0), 285000, 0, 0, 285000, 0},              // insurer pays later
		{286000, 0, pay(281000), 286000, 281000, 281000, 5000, 0}, // partly paid
	} {
		g, b, ci, u, k := settle(c.total, c.potongan, c.payment)
		if g != c.grand || b != c.bayar || ci != c.cicil || u != c.utang || k != c.kbl {
			t.Errorf("settle(%v, %v) = %v %v %v %v %v", c.total, c.potongan, g, b, ci, u, k)
		}
	}
}

func validSale() SaveInput {
	return SaveInput{
		Cabang: "kmj", TglKwi: "2026-11-19", IDKun: 518586, IDJenisRawat: 2, IDJenisBayar: 1, IDDokter: 13,
		Obat: []SaveObat{{IDObat: 1, Byk: 1, Dosis: " 3 x 1 "}}, Diagnosa: []int{5},
	}
}

func TestSaveValidate(t *testing.T) {
	in := validSale()
	if errs := in.Validate("2026-11-19"); len(errs) != 0 || in.Cabang != "KMJ" || in.Obat[0].Dosis != "3 x 1" {
		t.Fatalf("valid sale: %v %+v", errs, in)
	}
	for name, c := range map[string]struct {
		mut   func(*SaveInput)
		field string
	}{
		"future date":       {func(i *SaveInput) { i.TglKwi = "2026-11-20" }, "tglKwi"},
		"no visit":          {func(i *SaveInput) { i.IDKun = 0 }, "idKun"},
		"no doctor":         {func(i *SaveInput) { i.IDDokter = 0 }, "idDokter"},
		"no items":          {func(i *SaveInput) { i.Obat = nil }, "items"},
		"zero quantity":     {func(i *SaveInput) { i.Obat[0].Byk = 0 }, "obat"},
		"drug twice":        {func(i *SaveInput) { i.Obat = append(i.Obat, SaveObat{IDObat: 1, Byk: 2}) }, "obat"},
		"long dosis":        {func(i *SaveInput) { i.Obat[0].Dosis = strings.Repeat("x", 101) }, "obat"},
		"negative price":    {func(i *SaveInput) { h := -1.0; i.Tindakan = []SaveTindakan{{IDTindakan: 41, Byk: 1, Harga: &h}} }, "tindakan"},
		"diagnosis twice":   {func(i *SaveInput) { i.Diagnosa = []int{5, 5} }, "diagnosa"},
		"negative discount": {func(i *SaveInput) { i.Diskon = -1 }, "diskon"},
	} {
		in := validSale()
		c.mut(&in)
		if errs := in.Validate("2026-11-19"); errs[c.field] == "" {
			t.Errorf("%s: want an error on %s, got %v", name, c.field, errs)
		}
	}
}

func TestSave(t *testing.T) {
	s := newServer(t)
	post := func(body any) (int, map[string]any) {
		t.Helper()
		b, _ := json.Marshal(body)
		req := httptest.NewRequest("POST", "/api/pengobatan", bytes.NewReader(b))
		tok, _, _ := s.tokens.Generate(csID, "cs@medika.com")
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		s.mux.ServeHTTP(rec, req)
		out := map[string]any{}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}
	// 23:30 UTC on 18 Nov is 19 Nov in Jakarta, so the 19th is "today".
	code, out := post(validSale())
	if code != http.StatusCreated || out["noKwi"] != "2611KMJ0001" || s.repo.saleIn.Cabang != "KMJ" {
		t.Fatalf("save: %d %v", code, out)
	}
	other := validSale()
	other.Cabang = "CTM"
	if code, _ := post(other); code != http.StatusForbidden {
		t.Errorf("other branch: %d", code)
	}
	bad := validSale()
	bad.Obat = nil
	before := s.repo.sales
	if code, _ := post(bad); code != http.StatusBadRequest || s.repo.sales != before {
		t.Errorf("invalid sale must not reach the repository: %d", code)
	}
	for err, want := range map[error]int{
		ErrRegNotFound:    http.StatusBadRequest,
		ErrBranchMismatch: http.StatusBadRequest,
		ErrRegNotEditable: http.StatusConflict,
		ErrAlreadyBilled:  http.StatusConflict,
		&ItemError{Field: "obat", ID: 1, Msg: "X: not enough stock (0 left, 1 asked)"}:              http.StatusConflict,
		&ItemError{Field: "tindakan", ID: 41, Msg: "not a procedure of this doctor at this branch"}: http.StatusBadRequest,
		errors.New("db down"): http.StatusInternalServerError,
	} {
		s.repo.saleErr = err
		if code, out := post(validSale()); code != want {
			t.Errorf("%v: %d %v, want %d", err, code, out, want)
		}
	}
}

func TestKuitansi(t *testing.T) {
	s := newServer(t)
	if code, out := s.get(t, csID, "/api/pengobatan/kuitansi/522954"); code != http.StatusOK || out["noKwi"] != "2611KMJ0438" || out["jumlah"] != 50000.0 {
		t.Fatalf("receipt: %d %v", code, out)
	}
	for path, want := range map[string]int{
		"/api/pengobatan/kuitansi/522000": http.StatusNotFound, // another branch
		"/api/pengobatan/kuitansi/1":      http.StatusNotFound,
		"/api/pengobatan/kuitansi/x":      http.StatusNotFound,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	req := httptest.NewRequest("POST", "/api/pengobatan/kuitansi/522954/cetak", nil)
	tok, _, _ := s.tokens.Generate(csID, "cs.kmj@medika.com")
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || len(s.repo.prints) != 1 || s.repo.prints[0] != "cs.kmj" {
		t.Fatalf("print: %d %s %v (the user is printed without the domain)", rec.Code, rec.Body, s.repo.prints)
	}
	req = httptest.NewRequest("POST", "/api/pengobatan/kuitansi/522000/cetak", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec = httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound || len(s.repo.prints) != 1 {
		t.Errorf("another branch's print: %d", rec.Code)
	}
}

func TestNota(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/pengobatan/nota/522954")
	if code != http.StatusOK || out["noKwi"] != "2611KMJ0438" || len(out["obat"].([]any)) != 1 {
		t.Fatalf("nota: %d %v", code, out)
	}
	if code, _ := s.get(t, csID, "/api/pengobatan/nota/522000"); code != http.StatusNotFound {
		t.Errorf("another branch's nota: %d", code)
	}
}

func (s *server) send(t *testing.T, method, userID, path, body string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	tok, _, _ := s.tokens.Generate(userID, "a-very-long-email-address-for-cancel@medika.com")
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestDetail(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/pengobatan/detail/522954")
	if code != http.StatusOK || out["noKwi"] != "2611KMJ0438" || len(out["obat"].([]any)) != 1 {
		t.Fatalf("detail: %d %v", code, out)
	}
	for path, want := range map[string]int{
		"/api/pengobatan/detail/522000": http.StatusNotFound, // another branch
		"/api/pengobatan/detail/1":      http.StatusNotFound,
		"/api/pengobatan/detail/abc":    http.StatusNotFound,
	} {
		if code, _ := s.get(t, csID, path); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
}

func TestCancel(t *testing.T) {
	s := newServer(t)
	for body, want := range map[string]int{
		`{"alasan":"   "}`: http.StatusBadRequest,
		`{}`:               http.StatusBadRequest,
		`{"alasan":"` + strings.Repeat("x", 101) + `"}`: http.StatusBadRequest,
		`not json`: http.StatusBadRequest,
	} {
		if code, _ := s.send(t, "POST", csID, "/api/pengobatan/detail/522954/cancel", body); code != want {
			t.Errorf("%s: %d, want %d", body, code, want)
		}
	}
	if len(s.repo.cancels) != 0 {
		t.Fatalf("invalid input cancelled: %v", s.repo.cancels)
	}
	if code, _ := s.send(t, "POST", csID, "/api/pengobatan/detail/522000/cancel", `{"alasan":"SALAH INPUT"}`); code != http.StatusNotFound {
		t.Errorf("another branch's sale: %d", code)
	}
	code, out := s.send(t, "POST", csID, "/api/pengobatan/detail/522954/cancel", `{"alasan":"  SALAH INPUT "}`)
	if code != http.StatusOK || out["status"] != "cancel" {
		t.Fatalf("cancel: %d %v", code, out)
	}
	// The user is the e-mail without its domain, cut to cancelby's 30 characters.
	if got := s.repo.cancels; len(got) != 1 || got[0] != "522954 SALAH INPUT a-very-long-email-address-for-" {
		t.Errorf("cancel call: %q", got)
	}
	if code, _ := s.send(t, "POST", csID, "/api/pengobatan/detail/522954/cancel", `{"alasan":"LAGI"}`); code != http.StatusConflict {
		t.Errorf("second cancel: %d", code)
	}
}

func TestRawDataPengobatan(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/report-klinik/raw-data-pengobatan?from=2026-11-01&to=2026-11-08&sort=jenisKelamin&dir=asc&status=cancel")
	if code != http.StatusOK || out["rows"] == nil {
		t.Fatalf("raw data: %d %v", code, out)
	}
	if lp := s.repo.lastList; lp.Sort != "jenisKelamin" || lp.Status != "cancel" || lp.From != "2026-11-01" || len(lp.Branches) != 2 {
		t.Errorf("list params: %+v", lp)
	}
	if code, out := s.get(t, csID, "/api/report-klinik/raw-data-pengobatan/options"); code != http.StatusOK || out["branches"] == nil {
		t.Errorf("options: %d %v", code, out)
	}
	if code, _ := s.get(t, "", "/api/report-klinik/raw-data-pengobatan"); code != http.StatusUnauthorized {
		t.Errorf("no token: %d", code)
	}
}

func TestPerawatWhere(t *testing.T) {
	sql, args := perawatWhere(PerawatParams{Branches: []string{"MED"}, From: "2026-11-18", To: "2026-11-19", IDStatus: ptr(4), Search: "a%"}, true)
	for _, want := range []string{"d.cabang = ANY($1)", "d.tgl_masuk >= $2::date", "d.tgl_masuk < $3::date + 1",
		"COALESCE(r.ispemeriksaan, false)", "d.id_statustr = $4", "p.nama ILIKE $5", "d.id::text = $6"} {
		if !strings.Contains(sql, want) {
			t.Errorf("perawatWhere missing %q: %s", want, sql)
		}
	}
	if len(args) != 6 || args[4] != `%a\%%` || args[5] != "a%" {
		t.Errorf("args = %v", args)
	}
	if sql, args := perawatWhere(PerawatParams{Branches: []string{"MED"}, From: "x", To: "y", Search: "z"}, false); strings.Contains(sql, "ILIKE") || len(args) != 3 {
		t.Errorf("without search: %s %v", sql, args)
	}
}

func TestPerawat(t *testing.T) {
	s := newServer(t)
	code, out := s.get(t, csID, "/api/pemeriksaan-perawat")
	pp := s.repo.lastPerawat
	if code != http.StatusOK || pp.From != "2026-11-18" || pp.To != "2026-11-19" || strings.Join(pp.Branches, ",") != "KMJ,MED" ||
		pp.Sort != "tglMasuk" || !pp.Desc || pp.Limit != 10 || out["to"] != "2026-11-19" {
		t.Fatalf("default list (yesterday and today in Jakarta, newest first): %d %v %+v", code, out, pp)
	}
	s.get(t, csID, "/api/pemeriksaan-perawat?cabang=med&status=3&q=+siti+&sort=nama&dir=asc&page=2")
	pp = s.repo.lastPerawat
	if strings.Join(pp.Branches, ",") != "MED" || *pp.IDStatus != 3 || pp.Search != "siti" || pp.Sort != "nama" || pp.Desc || pp.Offset != 10 {
		t.Errorf("filtered list: %+v", pp)
	}
	if code, _ := s.get(t, csID, "/api/pemeriksaan-perawat?cabang=XYZ"); code != http.StatusForbidden {
		t.Errorf("other branch: %d, want 403", code)
	}
	if code, out := s.get(t, csID, "/api/pemeriksaan-perawat/options"); code != http.StatusOK || out["today"] != "2026-11-19" {
		t.Errorf("options: %d %v", code, out)
	}
	s.repo.regs2 = map[int][2]any{7: {"MED", 4}, 8: {"MED", 6}, 9: {"CTM", 4}}
	if code, out := s.get(t, csID, "/api/pemeriksaan-perawat/pendaftaran/7"); code != http.StatusOK || out["editable"] != true {
		t.Errorf("reg 7: %d %v", code, out)
	}
	if code, out := s.get(t, csID, "/api/pemeriksaan-perawat/pendaftaran/8"); code != http.StatusOK || out["editable"] != false {
		t.Errorf("closed reg 8: %d %v", code, out)
	}
	if code, _ := s.get(t, csID, "/api/pemeriksaan-perawat/pendaftaran/9"); code != http.StatusNotFound {
		t.Errorf("other branch's reg: %d, want 404", code)
	}
}

func (s *server) sendJSON(t *testing.T, method, path string, body any) (int, map[string]any) {
	t.Helper()
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	tok, _, _ := s.tokens.Generate(csID, "cs@medika.com")
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestPemeriksaanDokter(t *testing.T) {
	s := newServer(t)
	s.repo.regs2 = map[int][2]any{7: {"MED", 2}, 8: {"MED", 6}, 9: {"CTM", 4}}
	s.get(t, csID, "/api/pemeriksaan-dokter?dokter=5")
	if p := s.repo.lastPerawat; p.IDDokter == nil || *p.IDDokter != 5 {
		t.Errorf("dokter filter: %+v", p)
	}
	if code, out := s.get(t, csID, "/api/pemeriksaan-dokter/options"); code != http.StatusOK || len(out["dokter"].([]any)) != 1 {
		t.Errorf("options: %d %v", code, out)
	}
	// A closed visit can still change for the doctor, not for the nurse; a cancelled one cannot.
	if code, out := s.get(t, csID, "/api/pemeriksaan-dokter/pendaftaran/7"); code != http.StatusOK || out["editable"] != true {
		t.Errorf("closed reg for the doctor: %d %v", code, out)
	}
	if code, out := s.get(t, csID, "/api/pemeriksaan-perawat/pendaftaran/7"); code != http.StatusOK || out["editable"] != false {
		t.Errorf("closed reg for the nurse: %d %v", code, out)
	}
	if code, _ := s.sendJSON(t, "PUT", "/api/pemeriksaan-dokter/pemeriksaan/7", map[string]any{"keluhanUtama": "pusing"}); code != http.StatusOK {
		t.Errorf("doctor saves a closed visit: %d", code)
	}
	if code, _ := s.sendJSON(t, "PUT", "/api/pemeriksaan-perawat/pemeriksaan/7", map[string]any{"keluhanUtama": "pusing"}); code != http.StatusConflict {
		t.Errorf("nurse saves a closed visit: %d, want 409", code)
	}

	body := map[string]any{"obat": []map[string]any{{"idObat": 3, "byk": 10, "dosis": " 3x1 "}}, "diagnosa": []int{4, 6}}
	code, out := s.sendJSON(t, "PUT", "/api/pemeriksaan-dokter/resep/7", body)
	if code != http.StatusOK || len(out["obat"].([]any)) != 1 || len(out["diagnosa"].([]any)) != 2 || s.repo.resep[7].Obat[0].Dosis != "3x1" {
		t.Fatalf("save resep: %d %v", code, out)
	}
	if code, out := s.get(t, csID, "/api/pemeriksaan-dokter/resep/7"); code != http.StatusOK || len(out["obat"].([]any)) != 1 {
		t.Errorf("get resep: %d %v", code, out)
	}
	if code, _ := s.sendJSON(t, "PUT", "/api/pemeriksaan-dokter/resep/8", body); code != http.StatusConflict {
		t.Errorf("cancelled visit: %d, want 409", code)
	}
	if code, _ := s.sendJSON(t, "PUT", "/api/pemeriksaan-dokter/resep/9", body); code != http.StatusNotFound {
		t.Errorf("other branch: %d, want 404", code)
	}
	for _, bad := range []map[string]any{
		{"obat": []map[string]any{{"idObat": 3, "byk": 0}}},
		{"obat": []map[string]any{{"idObat": 0, "byk": 1}}},
		{"diagnosa": []int{4, 4}},
	} {
		if code, _ := s.sendJSON(t, "PUT", "/api/pemeriksaan-dokter/resep/7", bad); code != http.StatusBadRequest {
			t.Errorf("%v: %d, want 400", bad, code)
		}
	}
	if code, out := s.sendJSON(t, "PUT", "/api/pemeriksaan-dokter/resep/7", map[string]any{"obat": []map[string]any{{"idObat": 999, "byk": 1}}}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("unknown drug: %d %v", code, out)
	}
}

func TestProsesObat(t *testing.T) {
	s := newServer(t)
	s.repo.regs2 = map[int][2]any{7: {"MED", 5}, 9: {"CTM", 5}}
	s.get(t, csID, "/api/proses-obat?status=5")
	if p := s.repo.lastPerawat; p.IDStatus == nil || *p.IDStatus != 5 {
		t.Errorf("status filter: %+v", p)
	}
	// View only: never editable, and there is no PUT.
	if code, out := s.get(t, csID, "/api/proses-obat/pendaftaran/7"); code != http.StatusOK || out["editable"] != false {
		t.Errorf("reg 7: %d %v", code, out)
	}
	if code, _ := s.get(t, csID, "/api/proses-obat/resep/7"); code != http.StatusOK {
		t.Errorf("resep 7: %d", code)
	}
	if code, _ := s.get(t, csID, "/api/proses-obat/resep/9"); code != http.StatusNotFound {
		t.Errorf("other branch: %d, want 404", code)
	}
	if code, _ := s.sendJSON(t, "PUT", "/api/proses-obat/resep/7", map[string]any{}); code != http.StatusMethodNotAllowed {
		t.Errorf("PUT: %d, want 405", code)
	}
}
