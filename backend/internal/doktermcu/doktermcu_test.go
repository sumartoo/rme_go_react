package doktermcu

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
	in := Input{Nama: "  Dr Diar Garnida ", IDDokterMaster: 1, Fee: f(10)}
	if errs := in.Normalize(); errs != nil || in.Nama != "Dr Diar Garnida" {
		t.Fatalf("got %v %+v", errs, in)
	}
	if errs := (&Input{Nama: "X", IDDokterMaster: 1}).Normalize(); errs != nil {
		t.Errorf("fee is optional: %v", errs)
	}
	errs := (&Input{}).Normalize()
	if errs["nama"] == "" || errs["idDokterMaster"] == "" {
		t.Errorf("empty fields must fail: %v", errs)
	}
	errs = (&Input{Nama: strings.Repeat("n", 51), IDDokterMaster: 1, Fee: f(101)}).Normalize()
	if errs["nama"] == "" || errs["fee"] == "" {
		t.Errorf("long name and fee over 100 must fail: %v", errs)
	}
	if k, e := NormalizeKode(" D24 "); k != "D24" || e != "" {
		t.Errorf("kode = %q %q", k, e)
	}
	if _, e := NormalizeKode(" "); e == "" {
		t.Error("empty kode must fail")
	}
	if _, e := NormalizeKode(strings.Repeat("k", 21)); e == "" {
		t.Error("kode longer than 20 must fail")
	}
}

var masters = map[int]string{1: "DOKTER UMUM", 5: "DOKTER GIGI"}

type fakeRepo struct{ items map[string]Dokter }

func (r *fakeRepo) List(context.Context) ([]Dokter, error) {
	out := []Dokter{}
	for _, x := range r.items {
		out = append(out, x)
	}
	return out, nil
}
func (r *fakeRepo) Masters(context.Context) ([]Master, error) {
	return []Master{{ID: 5, Nama: "DOKTER GIGI"}, {ID: 1, Nama: "DOKTER UMUM"}}, nil
}
func (r *fakeRepo) Create(_ context.Context, kode string, in Input) (Dokter, error) {
	m, ok := masters[in.IDDokterMaster]
	if !ok {
		return Dokter{}, ErrUnknownMaster
	}
	if _, ok := r.items[kode]; ok {
		return Dokter{}, ErrDuplicate
	}
	x := Dokter{Kode: kode, DokterMaster: m, Input: in}
	r.items[kode] = x
	return x, nil
}
func (r *fakeRepo) Update(_ context.Context, kode string, in Input) (Dokter, error) {
	m, ok := masters[in.IDDokterMaster]
	if !ok {
		return Dokter{}, ErrUnknownMaster
	}
	if _, ok := r.items[kode]; !ok {
		return Dokter{}, ErrNotFound
	}
	x := Dokter{Kode: kode, DokterMaster: m, Input: in}
	r.items[kode] = x
	return x, nil
}

func TestHandler(t *testing.T) {
	repo := &fakeRepo{items: map[string]Dokter{"D01": {Kode: "D01", DokterMaster: "DOKTER GIGI", Input: Input{Nama: "Drg. Lilis", IDDokterMaster: 5, Fee: f(10)}}}}
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

	if code, out := do("GET", "/api/dokter-mcu", nil); code != http.StatusOK || len(out["dokter"].([]any)) != 1 {
		t.Fatalf("list: %d %v", code, out)
	}
	if code, out := do("GET", "/api/dokter-mcu/options", nil); code != http.StatusOK || len(out["dokterMaster"].([]any)) != 2 {
		t.Errorf("options: %d %v", code, out)
	}
	if code, out := do("POST", "/api/dokter-mcu", map[string]any{"kode": " D24 ", "nama": " Dr Baru ", "idDokterMaster": 1, "fee": 10}); code != http.StatusCreated || out["kode"] != "D24" || out["dokterMaster"] != "DOKTER UMUM" {
		t.Fatalf("create: %d %v", code, out)
	}
	if code, out := do("POST", "/api/dokter-mcu", map[string]any{"kode": "D01", "nama": "X", "idDokterMaster": 1}); code != http.StatusConflict || out["errors"] == nil {
		t.Errorf("duplicate kode: %d %v", code, out)
	}
	if code, out := do("POST", "/api/dokter-mcu", map[string]any{"kode": " ", "nama": "X", "idDokterMaster": 1}); code != http.StatusBadRequest || out["errors"].(map[string]any)["kode"] == nil {
		t.Errorf("empty kode: %d %v", code, out)
	}
	if code, out := do("POST", "/api/dokter-mcu", map[string]any{"kode": "D25", "nama": "X", "idDokterMaster": 99}); code != http.StatusBadRequest || out["errors"] == nil {
		t.Errorf("unknown master: %d %v", code, out)
	}
	if code, out := do("PUT", "/api/dokter-mcu/D01", map[string]any{"nama": "Drg. Lilis S.", "idDokterMaster": 5, "fee": 12.5}); code != http.StatusOK || out["fee"] != 12.5 || out["kode"] != "D01" {
		t.Errorf("update: %d %v", code, out)
	}
	if code, _ := do("PUT", "/api/dokter-mcu/D99", map[string]any{"nama": "X", "idDokterMaster": 1}); code != http.StatusNotFound {
		t.Errorf("update missing: %d", code)
	}
}
