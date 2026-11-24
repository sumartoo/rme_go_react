package labprice

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/example/be_jwt_golang/backend/internal/auth"
)

func f(v float64) *float64 { return &v }

func TestNormalize(t *testing.T) {
	if errs := (&Input{Price: f(95000)}).Normalize(); errs != nil {
		t.Errorf("valid price: %v", errs)
	}
	if errs := (&Input{}).Normalize(); errs != nil {
		t.Errorf("null price clears it: %v", errs)
	}
	if errs := (&Input{Price: f(-1)}).Normalize(); errs["price"] == "" {
		t.Error("negative price must fail")
	}
}

type fakeRepo struct {
	items    map[int]Price
	lastUser string
}

func (r *fakeRepo) List(context.Context) ([]Price, error) {
	out := []Price{}
	for _, x := range r.items {
		out = append(out, x)
	}
	return out, nil
}
func (r *fakeRepo) Update(_ context.Context, id int, price *float64, user string) (Price, error) {
	r.lastUser = user
	x, ok := r.items[id]
	if !ok {
		return Price{}, ErrNotFound
	}
	x.Price = price
	r.items[id] = x
	return x, nil
}

func TestHandler(t *testing.T) {
	repo := &fakeRepo{items: map[int]Price{1: {ID: 1, LabMaster: "KIMIA KLINIK", LabSubMaster: "FAAL HATI", Nama: "SGOT", Price: f(95000), IsActive: true}}}
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
	if guarded != 2 {
		t.Fatalf("%d routes guarded, want 2", guarded)
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

	if code, out := do("GET", "/api/lab-price", nil); code != http.StatusOK || len(out["price"].([]any)) != 1 {
		t.Fatalf("list: %d %v", code, out)
	}
	if code, out := do("PUT", "/api/lab-price/1", map[string]any{"price": 100000}); code != http.StatusOK || out["price"] != 100000.0 || out["nama"] != "SGOT" {
		t.Errorf("update: %d %v", code, out)
	}
	if repo.lastUser != "admin@medika.com" {
		t.Errorf("modifiedby = %q", repo.lastUser)
	}
	if code, out := do("PUT", "/api/lab-price/1", map[string]any{"price": -5}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("negative: %d %v", code, out)
	}
	if code, _ := do("PUT", "/api/lab-price/999", map[string]any{"price": 1}); code != http.StatusNotFound {
		t.Errorf("update missing: %d", code)
	}
}
