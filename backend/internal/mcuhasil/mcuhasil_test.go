package mcuhasil

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

const userID = "22222222-2222-2222-2222-222222222222"

var now = time.Date(2026, 11, 24, 23, 30, 0, 0, time.UTC) // 25 Nov in Jakarta

type fakeRepo struct {
	list      ListParams
	groups    []string
	saved     map[string]Hasil
	savedBy   string
	cancelled bool
}

func (f *fakeRepo) List(_ context.Context, p ListParams) (Page, error) {
	f.list = p
	return Page{Rows: []Registrasi{{ID: 9001, Nama: "BUDI", Status: "Close"}}, Total: 1}, nil
}
func (f *fakeRepo) Registrasi(_ context.Context, id int) (Registrasi, error) {
	if id != 9001 {
		return Registrasi{}, ErrNotFound
	}
	return Registrasi{ID: 9001, Nama: "BUDI"}, nil
}
func (f *fakeRepo) Katalog(_ context.Context, groups []string) ([]Section, error) {
	f.groups = groups
	return []Section{{Kode: "06", Nama: "Mata", Items: []Item{{Key: "06.061", Nama: "Persepsi warna", Options: []Option{{Nama: "Normal", Entry: "radio"}}}}}}, nil
}
func (f *fakeRepo) Hasil(_ context.Context, id int, form string) (Hasil, error) {
	if h, ok := f.saved[form]; ok {
		return h, nil
	}
	return Hasil{Values: map[string]Value{}}, nil
}
func (f *fakeRepo) Save(_ context.Context, id int, form string, h Hasil, user string) error {
	if id != 9001 {
		return ErrNotFound
	}
	if f.cancelled {
		return ErrCancelled
	}
	f.saved[form], f.savedBy = h, user
	return nil
}
func (f *fakeRepo) LabItems(_ context.Context, id int) ([]LabItem, error) {
	return []LabItem{{LabID: 10, Nama: "HEMOGLOBIN"}}, nil
}

func setup(t *testing.T) (*http.ServeMux, *fakeRepo, *auth.TokenManager) {
	t.Helper()
	repo := &fakeRepo{saved: map[string]Hasil{}}
	tokens := auth.NewTokenManager("test-secret-that-is-at-least-32-characters", time.Minute)
	guards := map[string]int{}
	requireMenu := func(path string) func(http.Handler) http.Handler {
		guards[path]++
		return func(next http.Handler) http.Handler { return next }
	}
	h := NewHandler(repo)
	h.now = func() time.Time { return now }
	mux := http.NewServeMux()
	h.Routes(mux, tokens.Middleware, requireMenu)
	if len(guards) != 16 {
		t.Fatalf("%d menu paths guard the forms, want 16", len(guards))
	}
	for p, n := range guards {
		if n != 4 || !strings.HasPrefix(p, "/mcu/") {
			t.Errorf("%s guards %d routes", p, n)
		}
	}
	return mux, repo, tokens
}

func call(t *testing.T, mux *http.ServeMux, tok *auth.TokenManager, method, path string, body any) (int, map[string]any) {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		_ = json.NewEncoder(&buf).Encode(body)
	}
	req := httptest.NewRequest(method, path, &buf)
	req.Header.Set("Content-Type", "application/json")
	token, _, _ := tok.Generate(userID, "perawat@medika.com")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestFormsAreUnique(t *testing.T) {
	slugs, paths := map[string]bool{}, map[string]bool{}
	for _, f := range Forms {
		if slugs[f.Slug] || paths[f.PagePath] {
			t.Errorf("duplicate form %s %s", f.Slug, f.PagePath)
		}
		slugs[f.Slug], paths[f.PagePath] = true, true
	}
}

func TestList(t *testing.T) {
	mux, repo, tok := setup(t)
	code, out := call(t, mux, tok, "GET", "/api/mcu/form/fisik/registrasi?q=+budi+&size=500", nil)
	p := repo.list
	if code != http.StatusOK || p.Form != "fisik" || p.Search != "budi" || p.From != "2026-10-27" || p.To != "2026-11-25" || p.Limit != 100 || p.Cancel {
		t.Fatalf("list: %d %v %+v", code, out, p)
	}
	if code, _ := call(t, mux, tok, "GET", "/api/mcu/form/fisik/registrasi?from=2026-11-10&to=2026-11-01", nil); code != http.StatusBadRequest {
		t.Errorf("bad range: %d", code)
	}
	if code, _ := call(t, mux, tok, "GET", "/api/mcu/form/nope/registrasi", nil); code != http.StatusNotFound {
		t.Errorf("unknown form: %d", code)
	}
}

func TestKatalogAndGet(t *testing.T) {
	mux, repo, tok := setup(t)
	if code, out := call(t, mux, tok, "GET", "/api/mcu/form/khusus-wanita/katalog", nil); code != http.StatusOK || strings.Join(repo.groups, ",") != "C,D" || out["title"] != "Riwayat Khusus Wanita" {
		t.Fatalf("katalog: %d %v", code, out)
	}
	code, out := call(t, mux, tok, "GET", "/api/mcu/form/lab/9001", nil)
	if code != http.StatusOK || out["registrasi"].(map[string]any)["nama"] != "BUDI" || len(out["lab"].([]any)) != 1 {
		t.Fatalf("get lab: %d %v", code, out)
	}
	if _, out := call(t, mux, tok, "GET", "/api/mcu/form/fisik/9001", nil); out["lab"] != nil {
		t.Errorf("lab items on another form: %v", out)
	}
	if code, _ := call(t, mux, tok, "GET", "/api/mcu/form/fisik/1", nil); code != http.StatusNotFound {
		t.Errorf("missing registration: %d", code)
	}
}

func TestSave(t *testing.T) {
	mux, repo, tok := setup(t)
	body := map[string]any{"status": "abnormal", "kesimpulan": "  Buta warna parsial ", "values": map[string]any{
		"06.061": map[string]any{"value": "Buta Warna Parsial"},
		"06.079": map[string]any{"value": " ", "note": ""}, // empty: dropped
	}}
	code, out := call(t, mux, tok, "PUT", "/api/mcu/form/fisik/9001", body)
	got := repo.saved["fisik"]
	if code != http.StatusOK || got.Status != "abnormal" || got.Kesimpulan != "Buta warna parsial" || len(got.Values) != 1 || repo.savedBy != "perawat" {
		t.Fatalf("save: %d %v %+v", code, out, got)
	}
	for name, b := range map[string]map[string]any{
		"bad status": {"status": "maybe"},
		"long key":   {"values": map[string]any{strings.Repeat("k", MaxKeyLen+1): map[string]any{"value": "x"}}},
	} {
		if code, _ := call(t, mux, tok, "PUT", "/api/mcu/form/fisik/9001", b); code != http.StatusBadRequest {
			t.Errorf("%s: %d", name, code)
		}
	}
	repo.cancelled = true
	if code, _ := call(t, mux, tok, "PUT", "/api/mcu/form/fisik/9001", map[string]any{}); code != http.StatusConflict {
		t.Errorf("cancelled: %d", code)
	}
	if code, _ := call(t, mux, tok, "PUT", "/api/mcu/form/fisik/1", map[string]any{}); code != http.StatusNotFound {
		t.Errorf("missing: %d", code)
	}
}
