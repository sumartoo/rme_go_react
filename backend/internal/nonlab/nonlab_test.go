package nonlab

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

func f(v float64) *float64 { return &v }

func TestNormalize(t *testing.T) {
	in := Input{Nama: "  Sameday ", Price: f(200000)}
	if errs := in.Normalize(); errs != nil || in.Nama != "Sameday" {
		t.Fatalf("got %v %+v", errs, in)
	}
	if errs := (&Input{Nama: "X"}).Normalize(); errs != nil {
		t.Errorf("price is optional: %v", errs)
	}
	errs := (&Input{}).Normalize()
	if errs["nama"] == "" {
		t.Errorf("empty name must fail: %v", errs)
	}
	errs = (&Input{Nama: strings.Repeat("n", 101), Price: f(-1)}).Normalize()
	if errs["nama"] == "" || errs["price"] == "" {
		t.Errorf("long name and negative price must fail: %v", errs)
	}
}

type fakeRepo struct {
	items    map[int]NonLab
	lastUser string
}

func (r *fakeRepo) List(context.Context) ([]NonLab, error) {
	out := []NonLab{}
	for _, x := range r.items {
		out = append(out, x)
	}
	return out, nil
}
func (r *fakeRepo) Create(_ context.Context, in Input, user string) (NonLab, error) {
	r.lastUser = user
	x := NonLab{ID: 100 + len(r.items), Input: in}
	r.items[x.ID] = x
	return x, nil
}
func (r *fakeRepo) Update(_ context.Context, id int, in Input, user string) (NonLab, error) {
	r.lastUser = user
	if _, ok := r.items[id]; !ok {
		return NonLab{}, ErrNotFound
	}
	x := NonLab{ID: id, Input: in}
	r.items[id] = x
	return x, nil
}

func TestHandler(t *testing.T) {
	repo := &fakeRepo{items: map[int]NonLab{1: {ID: 1, Input: Input{Nama: "Sameday", Price: f(200000)}}}}
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

	if code, out := do("GET", "/api/non-lab", nil); code != http.StatusOK || len(out["nonLab"].([]any)) != 1 {
		t.Fatalf("list: %d %v", code, out)
	}
	if code, out := do("POST", "/api/non-lab", map[string]any{"nama": " Home Service ", "price": 75000}); code != http.StatusCreated || out["nama"] != "Home Service" || out["price"] != 75000.0 {
		t.Fatalf("create: %d %v", code, out)
	}
	if repo.lastUser != "admin@medika.com" {
		t.Errorf("createby = %q", repo.lastUser)
	}
	if code, out := do("POST", "/api/non-lab", map[string]any{"nama": " "}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("empty name: %d %v", code, out)
	}
	if code, out := do("PUT", "/api/non-lab/1", map[string]any{"nama": "Sameday", "price": 250000}); code != http.StatusOK || out["price"] != 250000.0 {
		t.Errorf("update: %d %v", code, out)
	}
	if code, _ := do("PUT", "/api/non-lab/999", map[string]any{"nama": "X"}); code != http.StatusNotFound {
		t.Errorf("update missing: %d", code)
	}
}
