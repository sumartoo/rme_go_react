package lab

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
	in := Input{IDLabSubMaster: 4, Kode: " SGOT ", Nama: "  SGOT ", Price: f(95000), Fee: f(15), IsActive: true}
	if errs := in.Normalize(); errs != nil || in.Kode != "SGOT" || in.Nama != "SGOT" {
		t.Fatalf("got %v %+v", errs, in)
	}
	errs := (&Input{}).Normalize()
	if errs["idLabSubMaster"] == "" || errs["nama"] == "" {
		t.Errorf("empty fields must fail: %v", errs)
	}
	if errs := (&Input{IDLabSubMaster: 1, Nama: "X"}).Normalize(); errs != nil {
		t.Errorf("code, price and fee are optional: %v", errs)
	}
	errs = (&Input{IDLabSubMaster: 1, Kode: strings.Repeat("k", 51), Nama: strings.Repeat("n", 51), Price: f(-1), Fee: f(101)}).Normalize()
	for _, k := range []string{"kode", "nama", "price", "fee"} {
		if errs[k] == "" {
			t.Errorf("bad %s must fail: %v", k, errs)
		}
	}
}

func TestKodeDefaultsToNama(t *testing.T) {
	if got := kode(Input{Nama: "SGOT"}); got != "SGOT" {
		t.Errorf("kode = %q", got)
	}
	if got := kode(Input{Kode: "GOT", Nama: "SGOT"}); got != "GOT" {
		t.Errorf("kode = %q", got)
	}
}

type fakeRepo struct {
	items    map[int]Lab
	lastUser string
}

func (r *fakeRepo) List(context.Context) ([]Lab, error) {
	out := []Lab{}
	for _, x := range r.items {
		out = append(out, x)
	}
	return out, nil
}
func (r *fakeRepo) SubMasters(context.Context) ([]SubMaster, error) {
	return []SubMaster{{ID: 4, Nama: "FAAL HATI", LabMaster: "KIMIA KLINIK"}}, nil
}
func (r *fakeRepo) Create(_ context.Context, in Input, user string) (Lab, error) {
	r.lastUser = user
	if in.IDLabSubMaster != 4 {
		return Lab{}, ErrUnknownSubMaster
	}
	x := Lab{ID: 100 + len(r.items), LabMaster: "KIMIA KLINIK", LabSubMaster: "FAAL HATI", Input: in}
	r.items[x.ID] = x
	return x, nil
}
func (r *fakeRepo) Update(_ context.Context, id int, in Input, user string) (Lab, error) {
	r.lastUser = user
	if in.IDLabSubMaster != 4 {
		return Lab{}, ErrUnknownSubMaster
	}
	if _, ok := r.items[id]; !ok {
		return Lab{}, ErrNotFound
	}
	x := Lab{ID: id, LabMaster: "KIMIA KLINIK", LabSubMaster: "FAAL HATI", Input: in}
	r.items[id] = x
	return x, nil
}

func TestHandler(t *testing.T) {
	repo := &fakeRepo{items: map[int]Lab{1: {ID: 1, LabMaster: "KIMIA KLINIK", LabSubMaster: "FAAL HATI", Input: Input{IDLabSubMaster: 4, Kode: "SGOT", Nama: "SGOT", IsActive: true}}}}
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

	if code, out := do("GET", "/api/lab", nil); code != http.StatusOK || len(out["lab"].([]any)) != 1 {
		t.Fatalf("list: %d %v", code, out)
	}
	if code, out := do("GET", "/api/lab/options", nil); code != http.StatusOK || len(out["labSubMaster"].([]any)) != 1 {
		t.Errorf("options: %d %v", code, out)
	}
	body := map[string]any{"idLabSubMaster": 4, "nama": " SGPT ", "price": 95000, "fee": 15, "isActive": true}
	if code, out := do("POST", "/api/lab", body); code != http.StatusCreated || out["nama"] != "SGPT" || out["labSubMaster"] != "FAAL HATI" || out["price"] != 95000.0 {
		t.Fatalf("create: %d %v", code, out)
	}
	if repo.lastUser != "admin@medika.com" {
		t.Errorf("createby = %q", repo.lastUser)
	}
	if code, out := do("POST", "/api/lab", map[string]any{"idLabSubMaster": 99, "nama": "X"}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("unknown sub master: %d %v", code, out)
	}
	if code, out := do("PUT", "/api/lab/1", map[string]any{"idLabSubMaster": 4, "nama": "SGOT", "isActive": false}); code != http.StatusOK || out["isActive"] != false || out["price"] != nil {
		t.Errorf("deactivate: %d %v", code, out)
	}
	if code, _ := do("PUT", "/api/lab/999", body); code != http.StatusNotFound {
		t.Errorf("update missing: %d", code)
	}
}
