package mcudaftar

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
)

const userID = "11111111-1111-1111-1111-111111111111"

// 23:30 UTC on 18 Nov is already 19 Nov in Jakarta.
var now = time.Date(2026, 11, 18, 23, 30, 0, 0, time.UTC)

type fakeRepo struct {
	lastList ListParams
	created  Input
	updated  map[int]Input
	cancels  []string
	status   map[int]string // id -> status
}

func (f *fakeRepo) List(_ context.Context, lp ListParams) (Page, error) {
	f.lastList = lp
	return Page{Rows: []Row{}}, nil
}
func (f *fakeRepo) Kategori(context.Context) ([]Option, error) {
	return []Option{{1, "Umum"}, {2, "Perusahaan"}}, nil
}
func (f *fakeRepo) Info(context.Context) ([]Option, error) { return []Option{{6, "tlp klinik"}}, nil }
func (f *fakeRepo) Pasien(_ context.Context, search string, limit, offset int) ([]Pasien, int, error) {
	return []Pasien{{ID: 1, Nama: search}}, 1, nil
}
func (f *fakeRepo) Lab(context.Context) ([]Lab, error)       { return []Lab{}, nil }
func (f *fakeRepo) Paket(context.Context) ([]Paket, error)   { return []Paket{}, nil }
func (f *fakeRepo) NonLab(context.Context) ([]NonLab, error) { return []NonLab{}, nil }
func (f *fakeRepo) Dokter(context.Context) ([]Dokter, error) { return []Dokter{}, nil }
func (f *fakeRepo) Get(_ context.Context, id int) (Detail, error) {
	s, ok := f.status[id]
	if !ok {
		return Detail{}, ErrNotFound
	}
	return Detail{ID: id, Status: s}, nil
}
func (f *fakeRepo) Create(_ context.Context, in Input, _ string) ([]int, error) {
	if len(in.LabIDs) > 0 && in.LabIDs[0] == 999 {
		return nil, ErrUnknownLab
	}
	f.created = in
	ids := []int{}
	for i := range in.PasienIDs {
		ids = append(ids, 100+i)
	}
	return ids, nil
}
func (f *fakeRepo) Update(_ context.Context, id int, in Input, _ string) error {
	s, ok := f.status[id]
	if !ok {
		return ErrNotFound
	}
	if s != StatusNew {
		return ErrNotNew
	}
	if f.updated == nil {
		f.updated = map[int]Input{}
	}
	f.updated[id] = in
	return nil
}
func (f *fakeRepo) Cancel(_ context.Context, id int, reason, user string) error {
	if f.status[id] != StatusNew {
		return ErrNotNew
	}
	f.cancels = append(f.cancels, reason+"|"+user)
	return nil
}

type server struct {
	mux    *http.ServeMux
	tokens *auth.TokenManager
	repo   *fakeRepo
}

func newServer(t *testing.T) *server {
	t.Helper()
	repo := &fakeRepo{status: map[int]string{1: StatusNew, 2: StatusClose}}
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
	if guarded != 11 {
		t.Fatalf("%d routes guarded, want 11", guarded)
	}
	return &server{mux: mux, tokens: tokens, repo: repo}
}

func (s *server) do(t *testing.T, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var b []byte
	if body != nil {
		b, _ = json.Marshal(body)
	}
	req := httptest.NewRequest(method, path, bytes.NewReader(b))
	tok, _, _ := s.tokens.Generate(userID, "admin@medika.com")
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()
	s.mux.ServeHTTP(rec, req)
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestWhere(t *testing.T) {
	sql, args := where(ListParams{From: "2026-11-01", To: "2026-11-19", Status: "New", Search: "a_b"}, true)
	for _, want := range []string{"h.mcudate >= $1::date", "h.mcudate < $2::date + 1", "h.status = $3", "p.rm ILIKE $4", "h.id::text = $5"} {
		if !strings.Contains(sql, want) {
			t.Errorf("where missing %q: %s", want, sql)
		}
	}
	if len(args) != 5 || args[3] != `%a\_b%` {
		t.Errorf("args = %v", args)
	}
	if sql, args := where(ListParams{From: "x", To: "y", Search: "z"}, false); strings.Contains(sql, "ILIKE") || len(args) != 2 {
		t.Errorf("without search: %s %v", sql, args)
	}
}

func TestList(t *testing.T) {
	s := newServer(t)
	code, out := s.do(t, "GET", "/api/mcu-pendaftaran", nil)
	lp := s.repo.lastList
	if code != http.StatusOK || lp.From != "2026-10-21" || lp.To != "2026-11-19" || lp.Sort != "mcuDate" || !lp.Desc || lp.Limit != 10 || out["to"] != "2026-11-19" {
		t.Fatalf("default list (last 30 days in Jakarta, newest first): %d %v %+v", code, out, lp)
	}
	s.do(t, "GET", "/api/mcu-pendaftaran?from=2026-11-01&to=2026-11-02&status=Close&q=+ani+&sort=nama&dir=asc&page=2&size=20", nil)
	lp = s.repo.lastList
	if lp.Status != "Close" || lp.Search != "ani" || lp.Sort != "nama" || lp.Desc || lp.Offset != 20 {
		t.Errorf("filtered: %+v", lp)
	}
	if code, _ := s.do(t, "GET", "/api/mcu-pendaftaran?status=Open", nil); code != http.StatusBadRequest {
		t.Errorf("bad status: %d", code)
	}
	if code, _ := s.do(t, "GET", "/api/mcu-pendaftaran?from=2026-11-05&to=2026-11-01", nil); code != http.StatusBadRequest {
		t.Errorf("reversed range: %d", code)
	}
	if code, out := s.do(t, "GET", "/api/mcu-pendaftaran/options", nil); code != http.StatusOK || out["today"] != "2026-11-19" || len(out["kategori"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
}

func TestNormalize(t *testing.T) {
	ok := Input{McuDate: "2026-11-19", KategoriID: 1, PasienIDs: []int{1, 2}, PaketIDs: []int{30}, Dokter: []string{" D10 "}}
	if errs := ok.Normalize(true); len(errs) != 0 || ok.Dokter[0] != "D10" {
		t.Fatalf("valid form: %v %v", errs, ok.Dokter)
	}
	for name, in := range map[string]Input{
		"no date":      {KategoriID: 1, PasienIDs: []int{1}, PaketIDs: []int{30}},
		"no kategori":  {McuDate: "2026-11-19", PasienIDs: []int{1}, PaketIDs: []int{30}},
		"no patient":   {McuDate: "2026-11-19", KategoriID: 1, PaketIDs: []int{30}},
		"same patient": {McuDate: "2026-11-19", KategoriID: 1, PasienIDs: []int{1, 1}, PaketIDs: []int{30}},
		"no lines":     {McuDate: "2026-11-19", KategoriID: 1, PasienIDs: []int{1}},
		"same doctor":  {McuDate: "2026-11-19", KategoriID: 1, PasienIDs: []int{1}, LabIDs: []int{3}, Dokter: []string{"D1", "D1"}},
	} {
		if errs := in.Normalize(true); len(errs) == 0 {
			t.Errorf("%s: no error", name)
		}
	}
	// Editing has no patients.
	edit := Input{McuDate: "2026-11-19", KategoriID: 1, NonLabIDs: []int{5}}
	if errs := edit.Normalize(false); len(errs) != 0 {
		t.Errorf("edit: %v", errs)
	}
}

func TestCreateUpdateCancel(t *testing.T) {
	s := newServer(t)
	body := map[string]any{"mcuDate": "2026-11-19", "kategoriId": 1, "infoId": 6, "pasienIds": []int{7, 8}, "paketIds": []int{30}}
	code, out := s.do(t, "POST", "/api/mcu-pendaftaran", body)
	if code != http.StatusCreated || len(out["ids"].([]any)) != 2 || len(s.repo.created.PasienIDs) != 2 {
		t.Fatalf("create: %d %v", code, out)
	}
	if code, out := s.do(t, "POST", "/api/mcu-pendaftaran", map[string]any{"mcuDate": "2026-11-19", "kategoriId": 1, "pasienIds": []int{7}, "labIds": []int{999}}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("unknown lab: %d %v", code, out)
	}
	upd := map[string]any{"mcuDate": "2026-11-20", "kategoriId": 2, "pasienIds": []int{9}, "labIds": []int{3}}
	if code, _ := s.do(t, "PUT", "/api/mcu-pendaftaran/1", upd); code != http.StatusOK || s.repo.updated[1].PasienIDs != nil || s.repo.updated[1].McuDate != "2026-11-20" {
		t.Errorf("update New: %d %+v", code, s.repo.updated[1])
	}
	if code, _ := s.do(t, "PUT", "/api/mcu-pendaftaran/2", upd); code != http.StatusConflict {
		t.Errorf("update Close: %d, want 409", code)
	}
	if code, _ := s.do(t, "PUT", "/api/mcu-pendaftaran/99", upd); code != http.StatusNotFound {
		t.Errorf("update missing: %d, want 404", code)
	}
	if code, _ := s.do(t, "POST", "/api/mcu-pendaftaran/1/cancel", map[string]any{"reason": " "}); code != http.StatusBadRequest {
		t.Errorf("cancel without reason: %d", code)
	}
	if code, _ := s.do(t, "POST", "/api/mcu-pendaftaran/1/cancel", map[string]any{"reason": " tidak datang "}); code != http.StatusNoContent || s.repo.cancels[0] != "tidak datang|admin@medika.com" {
		t.Errorf("cancel: %d %v", code, s.repo.cancels)
	}
	if code, _ := s.do(t, "POST", "/api/mcu-pendaftaran/2/cancel", map[string]any{"reason": "x"}); code != http.StatusConflict {
		t.Errorf("cancel Close: %d, want 409", code)
	}
	if code, out := s.do(t, "GET", "/api/mcu-pendaftaran/2", nil); code != http.StatusOK || out["status"] != StatusClose {
		t.Errorf("get: %d %v", code, out)
	}
}
