package paketprice

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/example/be_jwt_golang/backend/internal/auth"
)

func f(v float64) *float64 { return &v }
func i(v int) *int         { return &v }

func valid() Input {
	return Input{IDPerusahaan: 1, Nama: " 4 IN 1 ", IsPaket: true, IDKategori: i(3), FromDate: "2026-11-13", ToDate: "2026-12-13",
		Price: f(94000), IsActive: true, LabIDs: []int{26, 17, 26, 0, 31}}
}

func TestNormalize(t *testing.T) {
	in := valid()
	if errs := in.Normalize(); errs != nil {
		t.Fatalf("valid input: %v", errs)
	}
	if in.Nama != "4 IN 1" || !reflect.DeepEqual(in.LabIDs, []int{17, 26, 31}) {
		t.Errorf("normalized: %+v", in)
	}
	zero := valid()
	zero.IDKategori = i(0)
	if errs := zero.Normalize(); errs != nil || zero.IDKategori != nil {
		t.Errorf("kategori 0 means none: %v %v", errs, zero.IDKategori)
	}
	errs := (&Input{}).Normalize()
	for _, k := range []string{"idPerusahaan", "nama", "fromDate", "toDate", "price", "labIds"} {
		if errs[k] == "" {
			t.Errorf("empty %s must fail: %v", k, errs)
		}
	}
	bad := valid()
	bad.Nama = strings.Repeat("n", 31)
	bad.ToDate = "2026-11-12"
	bad.Price = f(-1)
	errs = bad.Normalize()
	for _, k := range []string{"nama", "toDate", "price"} {
		if errs[k] == "" {
			t.Errorf("bad %s must fail: %v", k, errs)
		}
	}
	format := valid()
	format.FromDate = "13/11/2026"
	if errs := format.Normalize(); errs["fromDate"] == "" {
		t.Error("a date not in YYYY-MM-DD must fail")
	}
}

type fakeRepo struct {
	items    map[int]Detail
	lastUser string
}

func (r *fakeRepo) List(context.Context) ([]Paket, error) {
	out := []Paket{}
	for _, d := range r.items {
		out = append(out, d.Paket)
	}
	return out, nil
}
func (r *fakeRepo) Options(context.Context) (Options, error) {
	return Options{Perusahaan: []Choice{{ID: 1, Nama: "UMUM"}}, Kategori: []Choice{{ID: 3, Nama: "MCU Kategori 3"}}, Labs: []LabItem{{ID: 17, Nama: "CHOLESTEROL TOTAL"}}}, nil
}
func (r *fakeRepo) Get(_ context.Context, id int) (Detail, error) {
	d, ok := r.items[id]
	if !ok {
		return Detail{}, ErrNotFound
	}
	return d, nil
}
func (r *fakeRepo) save(id int, in Input) {
	labs := []LabItem{}
	for _, l := range in.LabIDs {
		labs = append(labs, LabItem{ID: l})
	}
	r.items[id] = Detail{Paket: Paket{ID: id, IDPerusahaan: in.IDPerusahaan, Perusahaan: "UMUM", Nama: in.Nama, Price: in.Price, IsActive: in.IsActive, JumlahLab: len(labs)}, Labs: labs}
}
func (r *fakeRepo) Create(_ context.Context, in Input, user string) (int, error) {
	r.lastUser = user
	if in.IDPerusahaan != 1 {
		return 0, ErrUnknownCompany
	}
	id := 400 + len(r.items)
	r.save(id, in)
	return id, nil
}
func (r *fakeRepo) Update(_ context.Context, id int, in Input, user string) error {
	r.lastUser = user
	if _, ok := r.items[id]; !ok {
		return ErrNotFound
	}
	for _, l := range in.LabIDs {
		if l > 1000 {
			return ErrUnknownLab
		}
	}
	r.save(id, in)
	return nil
}
func (r *fakeRepo) Delete(_ context.Context, id int, user string) error {
	r.lastUser = user
	if _, ok := r.items[id]; !ok {
		return ErrNotFound
	}
	delete(r.items, id)
	return nil
}

func TestHandler(t *testing.T) {
	repo := &fakeRepo{items: map[int]Detail{}}
	seed := valid()
	seed.Normalize()
	repo.save(364, seed)
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
	if guarded != 6 {
		t.Fatalf("%d routes guarded, want 6", guarded)
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

	if code, out := do("GET", "/api/paket-price", nil); code != http.StatusOK || len(out["paket"].([]any)) != 1 {
		t.Fatalf("list: %d %v", code, out)
	}
	if code, out := do("GET", "/api/paket-price/options", nil); code != http.StatusOK || len(out["labs"].([]any)) != 1 || len(out["kategori"].([]any)) != 1 {
		t.Errorf("options: %d %v", code, out)
	}
	if code, out := do("GET", "/api/paket-price/364", nil); code != http.StatusOK || len(out["labs"].([]any)) != 3 {
		t.Errorf("get: %d %v", code, out)
	}
	if code, _ := do("GET", "/api/paket-price/999", nil); code != http.StatusNotFound {
		t.Errorf("get missing: %d", code)
	}
	body := map[string]any{"idPerusahaan": 1, "nama": "MCU BARU", "isPaket": true, "idKategori": 1, "fromDate": "2026-12-01",
		"toDate": "2026-12-31", "price": 500000, "isActive": true, "labIds": []int{17, 26}}
	code, out := do("POST", "/api/paket-price", body)
	if code != http.StatusCreated || out["nama"] != "MCU BARU" || len(out["labs"].([]any)) != 2 {
		t.Fatalf("create: %d %v", code, out)
	}
	if repo.lastUser != "admin@medika.com" {
		t.Errorf("createby = %q", repo.lastUser)
	}
	body["idPerusahaan"] = 99
	if code, out := do("POST", "/api/paket-price", body); code != http.StatusBadRequest || out["errors"].(map[string]any)["idPerusahaan"] == nil {
		t.Errorf("unknown company: %d %v", code, out)
	}
	body["idPerusahaan"] = 1
	body["labIds"] = []int{}
	if code, out := do("POST", "/api/paket-price", body); code != http.StatusBadRequest || out["errors"].(map[string]any)["labIds"] == nil {
		t.Errorf("no labs: %d %v", code, out)
	}
	body["labIds"] = []int{17, 2000}
	if code, out := do("PUT", "/api/paket-price/364", body); code != http.StatusBadRequest || out["errors"].(map[string]any)["labIds"] == nil {
		t.Errorf("unknown lab: %d %v", code, out)
	}
	body["labIds"] = []int{17}
	body["isActive"] = false
	if code, out := do("PUT", "/api/paket-price/364", body); code != http.StatusOK || out["isActive"] != false || out["jumlahLab"] != 1.0 {
		t.Errorf("update: %d %v", code, out)
	}
	if code, _ := do("PUT", "/api/paket-price/999", body); code != http.StatusNotFound {
		t.Errorf("update missing: %d", code)
	}
	if code, _ := do("DELETE", "/api/paket-price/364", nil); code != http.StatusNoContent {
		t.Errorf("delete: %d", code)
	}
	if code, _ := do("DELETE", "/api/paket-price/364", nil); code != http.StatusNotFound {
		t.Errorf("delete again: %d", code)
	}
}
