package stokadjus

import (
	"bytes"
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

// 23:30 UTC on 24 Nov is already 25 Nov in Jakarta.
var now = time.Date(2026, 11, 24, 23, 30, 0, 0, time.UTC)

type fakeRepo struct {
	branches map[string][]branchauth.Branch
	last     ListParams
	calls    int
	saved    []SaveInput
	savedBy  string
	saveErr  error
	obatFor  string
}

func (f *fakeRepo) UserBranches(_ context.Context, userID string) ([]branchauth.Branch, error) {
	return f.branches[userID], nil
}
func (f *fakeRepo) List(_ context.Context, p ListParams) (Page, error) {
	f.last, f.calls = p, f.calls+1
	tgl := "2026-11-24"
	return Page{Rows: []Row{{ID: 198420, Tanggal: &tgl, IDObat: 1298, Nama: "MICROPHORE 1/2 INC", Status: 1, StatusDesc: "Masuk",
		Ket: "Cancel Transaksi 2611CTM0882", Byk: 1, Cabang: "KMJ", NamaCabang: "KLINIK MITRA JATINEGARA"}}, Total: 1, UnfilteredTotal: 1}, nil
}
func (f *fakeRepo) Obat(_ context.Context, branch string) ([]Obat, error) {
	f.obatFor = branch
	return []Obat{{ID: 1298, Nama: "MICROPHORE 1/2 INC", Jenis: "ETIKAL", Satuan: "ROLL", Stok: 4}}, nil
}
func (f *fakeRepo) Save(_ context.Context, in SaveInput, user string) (SaveResult, error) {
	if f.saveErr != nil {
		return SaveResult{}, f.saveErr
	}
	f.saved, f.savedBy = append(f.saved, in), user
	return SaveResult{ID: 198421, Stok: 5}, nil
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
	guarded := 0
	requireMenu := func(path string) func(http.Handler) http.Handler {
		if path != PagePath {
			t.Fatalf("guarded by %q, want %q", path, PagePath)
		}
		guarded++
		return func(next http.Handler) http.Handler { return next }
	}
	h := NewHandler(repo)
	h.now = func() time.Time { return now }
	mux := http.NewServeMux()
	h.Routes(mux, tokens.Middleware, requireMenu)
	if guarded != 4 {
		t.Fatalf("%d routes guarded, want 4", guarded)
	}
	return &server{mux: mux, repo: repo, tok: tokens}
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
		tok, _, _ := s.tok.Generate(userID, "mega_mgm@medika.com")
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
	code, out := s.do(t, "GET", csID, "/api/stok-farmasi/stok-adjusment", nil)
	p := s.repo.last
	if code != http.StatusOK || p.From != "2026-10-27" || p.To != "2026-11-25" || strings.Join(p.Branches, ",") != "KMJ,MED" ||
		p.Sort != "id" || !p.Desc || p.Limit != 10 || p.Status != nil {
		t.Fatalf("default (last 30 days in Jakarta, newest first): %d %v %+v", code, out, p)
	}
	if rows := out["rows"].([]any); len(rows) != 1 || rows[0].(map[string]any)["statusDesc"] != "Masuk" {
		t.Errorf("rows: %v", out["rows"])
	}
	s.do(t, "GET", csID, "/api/stok-farmasi/stok-adjusment?from=2026-11-01&to=2026-11-08&cabang=kmj&nama=+alko+&ket=+cancel+&status=0&q=x&sort=byk&dir=asc&size=20&page=3", nil)
	p = s.repo.last
	if p.From != "2026-11-01" || p.To != "2026-11-08" || strings.Join(p.Branches, ",") != "KMJ" || p.Nama != "alko" || p.Ket != "cancel" ||
		p.Status == nil || *p.Status != StatusKeluar || p.Search != "x" || p.Sort != "byk" || p.Desc || p.Offset != 40 {
		t.Errorf("filtered: %+v", p)
	}
	s.do(t, "GET", csID, "/api/stok-farmasi/stok-adjusment?sort=evil&size=999999&status=9", nil)
	if p = s.repo.last; p.Sort != "id" || !p.Desc || p.Limit != MaxPageSize || p.Status != nil {
		t.Errorf("unknown sort / huge size / bad status: %+v", p)
	}
	before := s.repo.calls
	for path, want := range map[string]int{
		"/api/stok-farmasi/stok-adjusment?cabang=CTM":                    http.StatusForbidden,
		"/api/stok-farmasi/stok-adjusment?from=2026-11-10&to=2026-11-01": http.StatusBadRequest,
	} {
		if code, _ := s.do(t, "GET", csID, path, nil); code != want {
			t.Errorf("%s: %d, want %d", path, code, want)
		}
	}
	if code, out := s.do(t, "GET", loneID, "/api/stok-farmasi/stok-adjusment", nil); code != http.StatusOK || out["total"] != 0.0 || s.repo.calls != before {
		t.Errorf("rejected or branchless requests: %d %v", code, out)
	}
	code, out = s.do(t, "GET", csID, "/api/stok-farmasi/stok-adjusment/options", nil)
	if code != http.StatusOK || out["today"] != "2026-11-25" || out["defaultFrom"] != "2026-10-27" || len(out["branches"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
}

func TestObat(t *testing.T) {
	s := newServer(t)
	if code, out := s.do(t, "GET", csID, "/api/stok-farmasi/stok-adjusment/obat?cabang=kmj", nil); code != http.StatusOK || s.repo.obatFor != "KMJ" || len(out["rows"].([]any)) != 1 {
		t.Errorf("obat: %d %v", code, out)
	}
	if code, _ := s.do(t, "GET", csID, "/api/stok-farmasi/stok-adjusment/obat", nil); code != http.StatusBadRequest {
		t.Errorf("no cabang: %d", code)
	}
	if code, _ := s.do(t, "GET", csID, "/api/stok-farmasi/stok-adjusment/obat?cabang=CTM", nil); code != http.StatusForbidden {
		t.Errorf("other branch: %d", code)
	}
}

func TestSave(t *testing.T) {
	s := newServer(t)
	good := map[string]any{"cabang": " kmj ", "tanggal": "2026-11-25", "idObat": 1298, "status": 1, "byk": 1, "ket": " input "}
	code, out := s.do(t, "POST", csID, "/api/stok-farmasi/stok-adjusment", good)
	if code != http.StatusCreated || out["id"] != 198421.0 || len(s.repo.saved) != 1 || s.repo.savedBy != "mega_mgm" {
		t.Fatalf("save: %d %v %v", code, out, s.repo.saved)
	}
	if in := s.repo.saved[0]; in.Cabang != "KMJ" || in.Ket != "input" || *in.Status != StatusMasuk {
		t.Errorf("normalized: %+v", in)
	}
	code, out = s.do(t, "POST", csID, "/api/stok-farmasi/stok-adjusment", map[string]any{"tanggal": "2026-11-26", "byk": 0, "status": 5, "ket": strings.Repeat("x", 61)})
	errs, _ := out["errors"].(map[string]any)
	if code != http.StatusBadRequest || len(s.repo.saved) != 1 {
		t.Fatalf("invalid: %d %v", code, out)
	}
	for _, f := range []string{"cabang", "tanggal", "idObat", "status", "byk", "ket"} {
		if errs != nil && errs[f] == nil {
			t.Errorf("no error for %s: %v", f, errs)
		}
	}
	bad := map[string]any{"cabang": "CTM", "tanggal": "2026-11-25", "idObat": 1298, "status": 0, "byk": 1, "ket": "t"}
	if code, _ := s.do(t, "POST", csID, "/api/stok-farmasi/stok-adjusment", bad); code != http.StatusForbidden {
		t.Errorf("other branch: %d", code)
	}
	s.repo.saveErr = ErrShortStock
	if code, _ := s.do(t, "POST", csID, "/api/stok-farmasi/stok-adjusment", good); code != http.StatusBadRequest {
		t.Errorf("short stock: %d", code)
	}
}
