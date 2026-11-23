package mastermcu

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
	in := Input{Kode: " GNR ", Nama: "  General "}
	if errs := in.Normalize(); errs != nil || in.Kode != "GNR" || in.Nama != "General" {
		t.Fatalf("got %v %q %q", errs, in.Kode, in.Nama)
	}
	errs := (&Input{}).Normalize()
	if errs["kode"] == "" || errs["nama"] == "" {
		t.Errorf("empty fields must fail: %v", errs)
	}
	errs = (&Input{Kode: strings.Repeat("k", 11), Nama: strings.Repeat("n", 31)}).Normalize()
	if errs["kode"] == "" || errs["nama"] == "" {
		t.Errorf("too long fields must fail: %v", errs)
	}
}

type fakeRepo struct{ items map[int]Mcu }

func (f *fakeRepo) List(context.Context) ([]Mcu, error) {
	out := []Mcu{}
	for _, x := range f.items {
		out = append(out, x)
	}
	return out, nil
}
func (f *fakeRepo) Create(_ context.Context, in Input) (Mcu, error) {
	x := Mcu{ID: 100 + len(f.items), Kode: in.Kode, Nama: in.Nama}
	f.items[x.ID] = x
	return x, nil
}
func (f *fakeRepo) Update(_ context.Context, id int, in Input) (Mcu, error) {
	if _, ok := f.items[id]; !ok {
		return Mcu{}, ErrNotFound
	}
	x := Mcu{ID: id, Kode: in.Kode, Nama: in.Nama}
	f.items[id] = x
	return x, nil
}

func TestHandler(t *testing.T) {
	repo := &fakeRepo{items: map[int]Mcu{1: {ID: 1, Kode: "GNR", Nama: "General"}}}
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

	if code, out := do("GET", "/api/master-mcu", nil); code != http.StatusOK || len(out["mcu"].([]any)) != 1 {
		t.Fatalf("list: %d %v", code, out)
	}
	if code, out := do("POST", "/api/master-mcu", map[string]any{"kode": " FSK ", "nama": " Fisik "}); code != http.StatusCreated || out["kode"] != "FSK" || out["nama"] != "Fisik" {
		t.Fatalf("create: %d %v", code, out)
	}
	if code, out := do("POST", "/api/master-mcu", map[string]any{"kode": " ", "nama": "X"}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("empty kode: %d %v", code, out)
	}
	if code, out := do("PUT", "/api/master-mcu/1", map[string]any{"kode": "GEN", "nama": "Umum"}); code != http.StatusOK || out["kode"] != "GEN" {
		t.Errorf("update: %d %v", code, out)
	}
	if code, _ := do("PUT", "/api/master-mcu/999", map[string]any{"kode": "X", "nama": "X"}); code != http.StatusNotFound {
		t.Errorf("update missing: %d", code)
	}
}
