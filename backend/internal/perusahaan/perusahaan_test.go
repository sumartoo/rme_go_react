package perusahaan

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
	in := Input{Kode: " OMS ", Nama: " PT OMS ", Alamat: " Jakarta ", IsVat: " 1 "}
	if errs := in.Normalize(); errs != nil || in.Kode != "OMS" || in.Nama != "PT OMS" || in.Alamat != "Jakarta" || in.IsVat != "1" {
		t.Fatalf("got %v %+v", errs, in)
	}
	errs := (&Input{}).Normalize()
	if errs["kode"] == "" || errs["nama"] == "" {
		t.Errorf("empty kode and nama must fail: %v", errs)
	}
	errs = (&Input{Kode: strings.Repeat("k", 11), Nama: "X", Telepon: strings.Repeat("1", 31), IsVat: "2"}).Normalize()
	if errs["kode"] == "" || errs["telepon"] == "" || errs["isVat"] == "" {
		t.Errorf("too long fields and bad isVat must fail: %v", errs)
	}
}

type fakeRepo struct{ items map[int]Perusahaan }

func (f *fakeRepo) List(context.Context) ([]Perusahaan, error) {
	out := []Perusahaan{}
	for _, x := range f.items {
		out = append(out, x)
	}
	return out, nil
}
func (f *fakeRepo) Create(_ context.Context, in Input) (Perusahaan, error) {
	for _, x := range f.items {
		if x.Kode == in.Kode {
			return Perusahaan{}, ErrDuplicate
		}
	}
	x := Perusahaan{ID: 100 + len(f.items), Input: in}
	f.items[x.ID] = x
	return x, nil
}
func (f *fakeRepo) Update(_ context.Context, id int, in Input) (Perusahaan, error) {
	if _, ok := f.items[id]; !ok {
		return Perusahaan{}, ErrNotFound
	}
	x := Perusahaan{ID: id, Input: in}
	f.items[id] = x
	return x, nil
}

func TestHandler(t *testing.T) {
	repo := &fakeRepo{items: map[int]Perusahaan{1: {ID: 1, Input: Input{Kode: "UMUM", Nama: "UMUM"}}}}
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
	NewHandler(repo).Routes(mux, tokens.Middleware, requireMenu)
	if guarded != 3 {
		t.Fatalf("%d routes guarded, want 3", guarded)
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

	if code, out := do("GET", "/api/perusahaan", nil); code != http.StatusOK || len(out["perusahaan"].([]any)) != 1 {
		t.Fatalf("list: %d %v", code, out)
	}
	if code, out := do("POST", "/api/perusahaan", map[string]any{"kode": " OMS ", "nama": "PT OMS", "isVat": "1"}); code != http.StatusCreated || out["kode"] != "OMS" || out["isVat"] != "1" {
		t.Fatalf("create: %d %v", code, out)
	}
	if code, out := do("POST", "/api/perusahaan", map[string]any{"kode": "OMS", "nama": "Lagi"}); code != http.StatusConflict || out["errors"] == nil {
		t.Errorf("duplicate kode: %d %v", code, out)
	}
	if code, out := do("POST", "/api/perusahaan", map[string]any{"kode": " ", "nama": "X"}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("empty kode: %d %v", code, out)
	}
	if code, out := do("PUT", "/api/perusahaan/1", map[string]any{"kode": "UMUM", "nama": "Umum", "alamat": "Jakarta"}); code != http.StatusOK || out["alamat"] != "Jakarta" {
		t.Errorf("update: %d %v", code, out)
	}
	if code, _ := do("PUT", "/api/perusahaan/999", map[string]any{"kode": "X", "nama": "X"}); code != http.StatusNotFound {
		t.Errorf("update missing: %d", code)
	}
}
