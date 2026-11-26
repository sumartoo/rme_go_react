package docs

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func get(t *testing.T, mux *http.ServeMux, path string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, path, nil))
	return rec
}

func TestRegisterServesUIAndSpec(t *testing.T) {
	mux := http.NewServeMux()
	Register(mux)

	if rec := get(t, mux, "/swagger"); rec.Code != http.StatusMovedPermanently || rec.Header().Get("Location") != "/swagger/" {
		t.Fatalf("/swagger: got %d Location=%q", rec.Code, rec.Header().Get("Location"))
	}

	rec := get(t, mux, "/swagger/")
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "SwaggerUIBundle") {
		t.Fatalf("/swagger/: got %d", rec.Code)
	}

	rec = get(t, mux, "/swagger/openapi.yaml")
	if rec.Code != http.StatusOK {
		t.Fatalf("/swagger/openapi.yaml: got %d", rec.Code)
	}
	// Every route registered in main.go (auth, menu, role, branch) must be documented.
	for _, p := range []string{
		"/api/auth/register:", "/api/auth/login:", "/api/auth/refresh:", "/api/me:", "/api/menus:",
		"/api/roles:", "/api/roles/{id}:", "/api/users:", "/api/users/{id}/role:", "/api/users/{id}/branches:", "/api/users/{id}/menus:", "/api/branches:", "/api/branches/{kode}:", "/api/obat:", "/api/obat/options:", "/api/obat/{id}:", "/api/obat/{id}/stock/{cabang}:", "/api/tindakan:", "/api/tindakan/options:", "/api/tindakan/{id}:", "/api/tindakan/{id}/price/{cabang}:", "/api/penunjang:", "/api/penunjang/options:", "/api/penunjang/{id}:", "/api/penunjang/{id}/price/{cabang}:", "/api/pasien:", "/api/pasien/options:", "/api/pasien/kelurahan:", "/api/pasien/{id}:", "/api/dokter:", "/api/dokter/options:", "/api/dokter/{id}:", "/api/dokter/{id}/tindakan:", "/api/dokter/{id}/cabang:", "/api/jamkes:", "/api/jamkes/{id}:", "/api/jamkes/{id}/pks:", "/api/jamkes/{id}/items:", "/api/distributor:", "/api/distributor/{id}:", "/api/principal:", "/api/principal/{id}:", "/api/icd:", "/api/icd/options:", "/api/icd/{id}:", "/api/pasien-mcu:", "/api/pasien-mcu/options:", "/api/pasien-mcu/{id}:", "/api/perusahaan:", "/api/perusahaan/{id}:", "/api/dokter-mcu:", "/api/dokter-mcu/options:", "/api/dokter-mcu/{kode}:", "/api/dokter-master:", "/api/dokter-master/{id}:", "/api/non-lab:", "/api/non-lab/{id}:", "/api/setting-barcode:", "/api/setting-barcode/{id}:", "/api/paket-price:", "/api/paket-price/options:", "/api/paket-price/{id}:", "/api/lab-price:", "/api/lab-price/{id}:", "/api/lab:", "/api/lab/options:", "/api/lab/{id}:", "/api/lab-master:", "/api/lab-master/{id}:", "/api/lab-sub-master:", "/api/lab-sub-master/options:", "/api/lab-sub-master/{id}:", "/api/master-mcu:", "/api/master-mcu/{id}:", "/api/perujuk-otc:", "/api/perujuk-otc/{id}:", "/api/pendaftaran:", "/api/pendaftaran/options:", "/api/pendaftaran/pasien:", "/api/pendaftaran/{id}:", "/api/pendaftaran/{id}/cancel:", "/api/organisasi-lokasi:", "/api/organisasi-lokasi/{id}:", "/api/pengobatan:", "/api/pengobatan/options:", "/api/pengobatan/pendaftaran:", "/api/pengobatan/dokter:", "/api/pengobatan/obat:", "/api/pengobatan/tindakan:", "/api/pengobatan/penunjang:", "/api/pengobatan/icd:", "/api/pengobatan/kuitansi/{id}:", "/api/pengobatan/kuitansi/{id}/cetak:", "/api/pengobatan/nota/{id}:", "/api/report-klinik/raw-data-pengobatan:", "/api/report-klinik/raw-data-pengobatan/options:", "/api/report-klinik/penjualan-obat:", "/api/report-klinik/penjualan-obat/options:", "/api/report-klinik/penjualan-tindakan:", "/api/report-klinik/penjualan-tindakan/options:", "/api/report-klinik/penjualan-penunjang:", "/api/report-klinik/penjualan-penunjang/options:", "/api/report-klinik/penjualan-otc:", "/api/report-klinik/penjualan-otc/options:", "/api/report-klinik/penjualan-softdrink:", "/api/report-klinik/penjualan-softdrink/options:", "/api/report-klinik/pasien-baru:", "/api/report-klinik/pasien-baru/options:", "/api/report-klinik/rekap-asuransi:", "/api/report-klinik/rekap-asuransi/options:", "/api/report-klinik/transaksi-harian:", "/api/report-klinik/transaksi-harian/options:", "/api/report-klinik/all-stok:", "/api/report-klinik/all-stok/options:", "/api/stok-farmasi/stok-opname:", "/api/stok-farmasi/stok-trace/obat:", "/api/stok-farmasi/stok-trace:", "/api/stok-farmasi/stok-alert/minim-stok:", "/api/stok-farmasi/stok-alert/options:", "/api/stok-farmasi/stok-opname/options:", "/api/stok-farmasi/stok-adjusment:", "/api/stok-farmasi/stok-adjusment/options:", "/api/stok-farmasi/stok-adjusment/obat:", "/api/report-klinik/rujukan:", "/api/report-klinik/rujukan/options:", "/api/report-klinik/pendaftaran:", "/api/report-klinik/pendaftaran/options:", "/api/report-klinik/pendapatan:", "/api/report-klinik/pendapatan/options:", "/api/report-klinik/pendapatan/dokter:", "/api/report-klinik/diagnosa:", "/api/report-klinik/diagnosa/options:", "/api/report-klinik/icd-dinkes:", "/api/report-klinik/icd-dinkes/options:", "/api/penerimaan-barang:", "/api/penerimaan-barang/options:", "/api/penerimaan-barang/distributor:", "/api/penerimaan-barang/obat:", "/api/penerimaan-barang/detail/{id}:", "/api/pengobatan/detail/{id}:", "/api/pengobatan/detail/{id}/cancel:", "/api/pengobatan/pemeriksaan/{idKun}:", "/api/pemeriksaan-perawat:", "/api/mcu-pendaftaran:", "/api/mcu-pendaftaran/options:", "/api/mcu-pendaftaran/pasien:", "/api/mcu-pendaftaran/lab:", "/api/mcu-pendaftaran/paket:", "/api/mcu-pendaftaran/non-lab:", "/api/mcu-pendaftaran/dokter:", "/api/mcu-pendaftaran/{id}:", "/api/mcu-pendaftaran/{id}/cancel:", "/api/proses-obat:", "/api/proses-obat/options:", "/api/proses-obat/pendaftaran/{idKun}:", "/api/proses-obat/pemeriksaan/{idKun}:", "/api/proses-obat/resep/{idKun}:", "/api/pemeriksaan-dokter:", "/api/pemeriksaan-dokter/options:", "/api/pemeriksaan-dokter/dokter:", "/api/pemeriksaan-dokter/obat:", "/api/pemeriksaan-dokter/icd:", "/api/pemeriksaan-dokter/pendaftaran/{idKun}:", "/api/pemeriksaan-dokter/pemeriksaan/{idKun}:", "/api/pemeriksaan-dokter/resep/{idKun}:", "/api/pemeriksaan-perawat/options:", "/api/pemeriksaan-perawat/dokter:", "/api/pemeriksaan-perawat/pendaftaran/{idKun}:", "/api/pemeriksaan-perawat/pemeriksaan/{idKun}:", "/api/me/branches:", "/api/complaints:", "/api/complaints/summary:", "/api/complaints/{id}:", "/api/complaints/{id}/status:", "/api/complaints/{id}/retry:", "/api/n8n/requests:", "/api/n8n/complaints/callback:", "/api/analitik/options:", "/api/analitik/obat:", "/api/analitik/penyakit:", "/api/analitik/diagnosa-obat:", "/api/analitik/ringkasan:", "/api/analitik/chat:", "/api/analitik/command:", "/api/analitik/kinerja:", "/api/analitik/kualitas-data:", "/api/analitik/follow-up:", "/api/analitik/saran-icd:", "/api/riwayat-pasien/pasien:", "/api/riwayat-pasien/{id}:", "/api/mcu/form/{form}/registrasi:", "/api/mcu/form/{form}/katalog:", "/api/mcu/form/{form}/{id}:", "/healthz:",
	} {
		if !strings.Contains(rec.Body.String(), p) {
			t.Errorf("spec is missing path %s", strings.TrimSuffix(p, ":"))
		}
	}
}

func TestNotMountedMeansNotFound(t *testing.T) {
	mux := http.NewServeMux() // SWAGGER_ENABLED=false: Register is never called
	if rec := get(t, mux, "/swagger/"); rec.Code != http.StatusNotFound {
		t.Fatalf("got %d, want 404", rec.Code)
	}
}
