package riwayatpasien

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/example/be_jwt_golang/backend/internal/auth"
	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/pendaftaran"
)

const (
	csID   = "22222222-2222-2222-2222-222222222222" // mapped to KMJ and MED
	loneID = "33333333-3333-3333-3333-333333333333" // mapped to no branch
)

func ip(v int) *int { return &v }

func TestSummarize(t *testing.T) {
	visits := []Kunjungan{ // newest first
		{Tgl: "2026-11-20 09:00", Cabang: "KMJ", Sistole: ip(150), Diastole: ip(95), BB: ip(80), TB: ip(170),
			Diagnosa: []Item{{Kode: "I10", Nama: "HIPERTENSI"}}, Resep: []Item{{Nama: "AMLODIPIN"}, {Nama: "AMLODIPIN"}}},
		{Tgl: "2026-10-01 10:00", Cabang: "KMJ", Batal: true, Diagnosa: []Item{{Kode: "Z00", Nama: "BATAL"}}},
		{Tgl: "2026-09-15 08:00", Cabang: "MED", Sistole: ip(160), Diastole: ip(100),
			Diagnosa: []Item{{Kode: "I10", Nama: "HIPERTENSI"}, {Kode: "J06", Nama: "ISPA"}}, Resep: []Item{{Nama: "AMLODIPIN"}, {Nama: "PARASETAMOL"}}},
	}
	s := Summarize(visits)
	if s.JumlahKunjungan != 2 || s.Pertama != "2026-09-15" || s.Terakhir != "2026-11-20" || strings.Join(s.Cabang, ",") != "KMJ,MED" {
		t.Fatalf("counts: %+v", s)
	}
	if len(s.Diagnosa) != 2 || s.Diagnosa[0].Kode != "I10" || s.Diagnosa[0].Kali != 2 || s.Diagnosa[0].Terakhir != "2026-11-20" {
		t.Errorf("diagnosa (cancelled visit left out): %+v", s.Diagnosa)
	}
	if s.Obat[0].Nama != "AMLODIPIN" || s.Obat[0].Kali != 2 { // twice in one visit counts once
		t.Errorf("obat: %+v", s.Obat)
	}
	if len(s.Vital) != 2 || s.Vital[0].Tgl != "2026-09-15" || s.Vital[1].IMT == nil || *s.Vital[1].IMT != 27.7 {
		t.Errorf("vital (oldest first, BMI 80/1.7²): %+v", s.Vital)
	}
	if e := Summarize(nil); e.JumlahKunjungan != 0 || e.Diagnosa == nil || e.Vital == nil {
		t.Errorf("empty: %+v", e)
	}
}

type fakeRepo struct {
	branches map[string][]branchauth.Branch
	searchQ  string
	searchIn []string
	gotFor   []string
}

func (f *fakeRepo) UserBranches(_ context.Context, userID string) ([]branchauth.Branch, error) {
	return f.branches[userID], nil
}
func (f *fakeRepo) SearchPasien(_ context.Context, branches []string, q string, limit, offset int) (pendaftaran.PasienPage, error) {
	f.searchQ, f.searchIn = q, branches
	return pendaftaran.PasienPage{Rows: []pendaftaran.PasienHit{{ID: 7, NoRM: "0001", Nama: "BUDI"}}, Total: 1}, nil
}
func (f *fakeRepo) Riwayat(_ context.Context, branches []string, id int) (Riwayat, error) {
	f.gotFor = branches
	if id != 7 {
		return Riwayat{}, ErrNotFound
	}
	return Riwayat{Pasien: Pasien{ID: 7, NoRM: "0001", Nama: "BUDI", Alergi: "AMOXICILLIN"},
		Kunjungan: []Kunjungan{{Tgl: "2026-11-20 09:00", Cabang: "KMJ", Diagnosa: []Item{{Kode: "I10", Nama: "HIPERTENSI"}}, Resep: []Item{}, Tindakan: []Item{}}}}, nil
}

func newServer(t *testing.T) (*http.ServeMux, *fakeRepo, *auth.TokenManager) {
	t.Helper()
	repo := &fakeRepo{branches: map[string][]branchauth.Branch{csID: {{Kode: "KMJ"}, {Kode: "MED"}}}}
	tokens := auth.NewTokenManager("test-secret-that-is-at-least-32-characters", time.Minute)
	guarded := 0
	requireMenu := func(path string) func(http.Handler) http.Handler {
		if path != PagePath {
			t.Fatalf("guarded by %q", path)
		}
		guarded++
		return func(next http.Handler) http.Handler { return next }
	}
	mux := http.NewServeMux()
	NewHandler(repo).Routes(mux, tokens.Middleware, requireMenu)
	if guarded != 2 {
		t.Fatalf("%d routes guarded, want 2", guarded)
	}
	return mux, repo, tokens
}

func get(t *testing.T, mux *http.ServeMux, tok *auth.TokenManager, userID, path string) (int, map[string]any) {
	t.Helper()
	req := httptest.NewRequest(http.MethodGet, path, nil)
	token, _, _ := tok.Generate(userID, "cs@medika.com")
	req.Header.Set("Authorization", "Bearer "+token)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	out := map[string]any{}
	_ = json.Unmarshal(rec.Body.Bytes(), &out)
	return rec.Code, out
}

func TestHandler(t *testing.T) {
	mux, repo, tok := newServer(t)
	code, out := get(t, mux, tok, csID, "/api/riwayat-pasien/pasien?q=+budi+")
	if code != http.StatusOK || repo.searchQ != "budi" || strings.Join(repo.searchIn, ",") != "KMJ,MED" || out["total"] != float64(1) {
		t.Fatalf("search: %d %v", code, out)
	}
	if code, _ := get(t, mux, tok, csID, "/api/riwayat-pasien/pasien?q=b"); code != http.StatusBadRequest {
		t.Errorf("short query: %d", code)
	}
	code, out = get(t, mux, tok, csID, "/api/riwayat-pasien/7")
	if code != http.StatusOK || out["pasien"].(map[string]any)["alergi"] != "AMOXICILLIN" ||
		out["ringkasan"].(map[string]any)["jumlahKunjungan"] != float64(1) || len(out["kunjungan"].([]any)) != 1 {
		t.Fatalf("get: %d %v", code, out)
	}
	for path, user := range map[string]string{"/api/riwayat-pasien/8": csID, "/api/riwayat-pasien/x": csID, "/api/riwayat-pasien/7": loneID} {
		if code, _ := get(t, mux, tok, user, path); code != http.StatusNotFound {
			t.Errorf("%s as %s: %d", path, user, code)
		}
	}
}
