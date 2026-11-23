package labsubmaster

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
	in := Input{IDLabMaster: 4, Kode: " FAAL HATI ", Nama: "  Faal Hati "}
	if errs := in.Normalize(); errs != nil || in.Kode != "FAAL HATI" || in.Nama != "Faal Hati" {
		t.Fatalf("got %v %+v", errs, in)
	}
	errs := (&Input{}).Normalize()
	if errs["idLabMaster"] == "" || errs["kode"] == "" || errs["nama"] == "" {
		t.Errorf("empty fields must fail: %v", errs)
	}
	errs = (&Input{IDLabMaster: 1, Kode: strings.Repeat("k", 21), Nama: strings.Repeat("n", 51)}).Normalize()
	if errs["kode"] == "" || errs["nama"] == "" {
		t.Errorf("too long fields must fail: %v", errs)
	}
}

var masters = map[int]string{1: "BIOMOLEKULER", 4: "KIMIA KLINIK"}

type fakeRepo struct {
	items    map[int]SubMaster
	lastUser string
}

func (f *fakeRepo) List(context.Context) ([]SubMaster, error) {
	out := []SubMaster{}
	for _, x := range f.items {
		out = append(out, x)
	}
	return out, nil
}
func (f *fakeRepo) LabMasters(context.Context) ([]LabMaster, error) {
	return []LabMaster{{ID: 1, Nama: "BIOMOLEKULER"}, {ID: 4, Nama: "KIMIA KLINIK"}}, nil
}
func (f *fakeRepo) Create(_ context.Context, in Input, user string) (SubMaster, error) {
	f.lastUser = user
	m, ok := masters[in.IDLabMaster]
	if !ok {
		return SubMaster{}, ErrUnknownMaster
	}
	x := SubMaster{ID: 100 + len(f.items), LabMaster: m, Input: in}
	f.items[x.ID] = x
	return x, nil
}
func (f *fakeRepo) Update(_ context.Context, id int, in Input, user string) (SubMaster, error) {
	f.lastUser = user
	m, ok := masters[in.IDLabMaster]
	if !ok {
		return SubMaster{}, ErrUnknownMaster
	}
	if _, ok := f.items[id]; !ok {
		return SubMaster{}, ErrNotFound
	}
	x := SubMaster{ID: id, LabMaster: m, Input: in}
	f.items[id] = x
	return x, nil
}

func TestHandler(t *testing.T) {
	repo := &fakeRepo{items: map[int]SubMaster{1: {ID: 1, LabMaster: "KIMIA KLINIK", Input: Input{IDLabMaster: 4, Kode: "FAAL HATI", Nama: "FAAL HATI"}}}}
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

	if code, out := do("GET", "/api/lab-sub-master", nil); code != http.StatusOK || len(out["labSubMaster"].([]any)) != 1 {
		t.Fatalf("list: %d %v", code, out)
	}
	if code, out := do("GET", "/api/lab-sub-master/options", nil); code != http.StatusOK || len(out["labMaster"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
	if code, out := do("POST", "/api/lab-sub-master", map[string]any{"idLabMaster": 1, "kode": " PCR ", "nama": "Swab PCR"}); code != http.StatusCreated || out["kode"] != "PCR" || out["labMaster"] != "BIOMOLEKULER" {
		t.Fatalf("create: %d %v", code, out)
	}
	if repo.lastUser != "admin@medika.com" {
		t.Errorf("createby = %q", repo.lastUser)
	}
	if code, out := do("POST", "/api/lab-sub-master", map[string]any{"idLabMaster": 99, "kode": "X", "nama": "X"}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("unknown master: %d %v", code, out)
	}
	if code, out := do("POST", "/api/lab-sub-master", map[string]any{"kode": " ", "nama": "X"}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("empty: %d %v", code, out)
	}
	if code, out := do("PUT", "/api/lab-sub-master/1", map[string]any{"idLabMaster": 1, "kode": "FH", "nama": "Faal Hati"}); code != http.StatusOK || out["labMaster"] != "BIOMOLEKULER" {
		t.Errorf("update: %d %v", code, out)
	}
	if code, _ := do("PUT", "/api/lab-sub-master/999", map[string]any{"idLabMaster": 1, "kode": "X", "nama": "X"}); code != http.StatusNotFound {
		t.Errorf("update missing: %d", code)
	}
}
