package pasienmcu

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

func TestNormalize(t *testing.T) {
	in := Input{Nama: " Ekowati ", Gender: "p", TglLahir: "1977-05-13", KodePerusahaan: " UMUM ", Email: "-"}
	if errs := in.Normalize("2026-11-23"); errs != nil || in.Nama != "Ekowati" || in.Gender != "P" || in.KodePerusahaan != "UMUM" {
		t.Fatalf("got %v %+v", errs, in)
	}
	errs := (&Input{}).Normalize("2026-11-23")
	for _, f := range []string{"nama", "gender", "kodePerusahaan"} {
		if errs[f] == "" {
			t.Errorf("empty %s must fail: %v", f, errs)
		}
	}
	errs = (&Input{Nama: "X", Gender: "L", KodePerusahaan: "UMUM", TglLahir: "2026-11-24", Email: "bukan email", HP: strings.Repeat("0", 16)}).Normalize("2026-11-23")
	for _, f := range []string{"tglLahir", "email", "hp"} {
		if errs[f] == "" {
			t.Errorf("bad %s must fail: %v", f, errs)
		}
	}
	if errs := (&Input{Nama: "X", Gender: "L", KodePerusahaan: "UMUM", TglLahir: "13/05/1977"}).Normalize("2026-11-23"); errs["tglLahir"] == "" {
		t.Error("a date not in YYYY-MM-DD must fail")
	}
}

type fakeRepo struct {
	items []Pasien
	last  ListParams
}

func (f *fakeRepo) List(_ context.Context, lp ListParams) (Page, error) {
	f.last = lp
	return Page{Rows: f.items, Total: len(f.items), UnfilteredTotal: len(f.items)}, nil
}
func (f *fakeRepo) Companies(context.Context) ([]Company, error) {
	return []Company{{Kode: "UMUM", Nama: "UMUM"}}, nil
}
func (f *fakeRepo) Create(_ context.Context, in Input, _ string) (Pasien, error) {
	if in.KodePerusahaan != "UMUM" {
		return Pasien{}, ErrUnknownCompany
	}
	x := Pasien{ID: 100, RM: "2611230001", Perusahaan: "UMUM", Input: in}
	f.items = append(f.items, x)
	return x, nil
}
func (f *fakeRepo) Update(_ context.Context, id int, in Input, _ string) (Pasien, error) {
	for i := range f.items {
		if f.items[i].ID == id {
			f.items[i].Input = in
			return f.items[i], nil
		}
	}
	return Pasien{}, ErrNotFound
}

func TestHandler(t *testing.T) {
	repo := &fakeRepo{items: []Pasien{{ID: 1, RM: "1902150001", Input: Input{Nama: "Engelbertho", Gender: "L", KodePerusahaan: "UMUM"}}}}
	tokens := auth.NewTokenManager("test-secret-that-is-at-least-32-characters", time.Minute)
	pass := func(next http.Handler) http.Handler { return next }
	guarded := 0
	requireMenu := func(path string) func(http.Handler) http.Handler {
		if path != PagePath {
			t.Fatalf("guarded by %q", path)
		}
		guarded++
		return pass
	}
	mux := http.NewServeMux()
	h := NewHandler(repo)
	h.now = func() time.Time { return time.Date(2026, 11, 23, 3, 0, 0, 0, time.UTC) }
	h.Routes(mux, tokens.Middleware, requireMenu)
	if guarded != 4 {
		t.Fatalf("%d routes guarded, want 4", guarded)
	}
	tok, _, _ := tokens.Generate("11111111-1111-1111-1111-111111111111", "admin@medika.com")
	do := func(method, path string, body any) (int, map[string]any) {
		var buf bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buf).Encode(body)
		}
		req := httptest.NewRequest(method, path, &buf)
		req.Header.Set("Authorization", "Bearer "+tok)
		rec := httptest.NewRecorder()
		mux.ServeHTTP(rec, req)
		out := map[string]any{}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return rec.Code, out
	}

	if code, out := do("GET", "/api/pasien-mcu?page=2&size=10&q=%20eko%20&sort=bogus", nil); code != http.StatusOK || len(out["rows"].([]any)) != 1 {
		t.Fatalf("list: %d %v", code, out)
	}
	if repo.last.Offset != 10 || repo.last.Search != "eko" || repo.last.Sort != "rm" || repo.last.Desc {
		t.Errorf("list params: %+v", repo.last)
	}
	if code, out := do("GET", "/api/pasien-mcu/options", nil); code != http.StatusOK || len(out["perusahaan"].([]any)) != 1 {
		t.Errorf("options: %d %v", code, out)
	}
	ok := map[string]any{"nama": "Rika", "gender": "P", "kodePerusahaan": "UMUM", "tglLahir": "1990-01-02"}
	if code, out := do("POST", "/api/pasien-mcu", ok); code != http.StatusCreated || out["rm"] != "2611230001" || out["perusahaan"] != "UMUM" {
		t.Fatalf("create: %d %v", code, out)
	}
	if code, out := do("POST", "/api/pasien-mcu", map[string]any{"nama": "X", "gender": "L", "kodePerusahaan": "NOPE"}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("unknown company: %d %v", code, out)
	}
	if code, out := do("POST", "/api/pasien-mcu", map[string]any{"nama": " "}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("empty: %d %v", code, out)
	}
	if code, out := do("PUT", "/api/pasien-mcu/1", map[string]any{"nama": "Engel", "gender": "L", "kodePerusahaan": "UMUM", "posisi": "Floorman"}); code != http.StatusOK || out["posisi"] != "Floorman" {
		t.Errorf("update: %d %v", code, out)
	}
	if code, _ := do("PUT", "/api/pasien-mcu/999", ok); code != http.StatusNotFound {
		t.Errorf("update missing: %d", code)
	}
}
