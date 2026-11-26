package settingbarcode

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
	if errs := (&Input{Left: f(5), Middle: f(25), BarcodeWidth: f(150), FontSize: f(6)}).Normalize(); errs != nil {
		t.Errorf("valid: %v", errs)
	}
	if errs := (&Input{}).Normalize(); errs != nil {
		t.Errorf("all numbers are optional: %v", errs)
	}
	errs := (&Input{Left: f(-1), BarcodeHeight: f(10001)}).Normalize()
	if errs["left"] == "" || errs["barcodeHeight"] == "" {
		t.Errorf("out of range must fail: %v", errs)
	}
}

type fakeRepo struct{ items map[int]Setting }

func (r *fakeRepo) List(context.Context) ([]Setting, error) {
	out := []Setting{}
	for _, x := range r.items {
		out = append(out, x)
	}
	return out, nil
}
func (r *fakeRepo) Update(_ context.Context, id int, in Input) (Setting, error) {
	if _, ok := r.items[id]; !ok {
		return Setting{}, ErrNotFound
	}
	x := Setting{ID: id, Input: in}
	r.items[id] = x
	return x, nil
}

func TestHandler(t *testing.T) {
	repo := &fakeRepo{items: map[int]Setting{1: {ID: 1, Input: Input{Left: f(5), Middle: f(25), Right: f(5), BarcodeWidth: f(150), BarcodeHeight: f(40), IsActive: true}}}}
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

	if code, out := do("GET", "/api/setting-barcode", nil); code != http.StatusOK || len(out["setting"].([]any)) != 1 {
		t.Fatalf("list: %d %v", code, out)
	}
	if code, out := do("PUT", "/api/setting-barcode/1", map[string]any{"left": 6, "barcodeWidth": 160, "fontSize": 7, "isActive": true}); code != http.StatusOK || out["left"] != 6.0 || out["middle"] != nil {
		t.Errorf("update: %d %v", code, out)
	}
	if code, out := do("PUT", "/api/setting-barcode/1", map[string]any{"left": -1}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("negative: %d %v", code, out)
	}
	if code, _ := do("PUT", "/api/setting-barcode/9", map[string]any{}); code != http.StatusNotFound {
		t.Errorf("update missing: %d", code)
	}
}
