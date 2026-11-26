import { StrictMode } from "react";
import { createRoot } from "react-dom/client";
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import { AuthProvider } from "./auth/AuthContext";
import { GuestRoute, ProtectedRoute } from "./auth/ProtectedRoute";
import AppLayout from "./layout/AppLayout";
import MenuGuard from "./layout/MenuGuard";
import BranchPage from "./pages/BranchPage";
import Dashboard from "./pages/Dashboard";
import DistributorPage from "./pages/DistributorPage";
import DokterPage from "./pages/DokterPage";
import IcdPage from "./pages/IcdPage";
import JamkesPage from "./pages/JamkesPage";
import Login from "./pages/Login";
import ObatPage from "./pages/ObatPage";
import PasienPage from "./pages/PasienPage";
import PendaftaranPage from "./pages/PendaftaranPage";
import PengobatanPage from "./pages/PengobatanPage";
import PengobatanFormPage from "./pages/PengobatanFormPage";
import PenjualanObatPage from "./pages/PenjualanObatPage";
import PenjualanTindakanPage from "./pages/PenjualanTindakanPage";
import PenjualanPenunjangPage from "./pages/PenjualanPenunjangPage";
import PenjualanOTCPage from "./pages/PenjualanOTCPage";
import PasienBaruPage from "./pages/PasienBaruPage";
import RekapAsuransiPage from "./pages/RekapAsuransiPage";
import TransaksiHarianPage from "./pages/TransaksiHarianPage";
import AllStokPage from "./pages/AllStokPage";
import StokOpnamePage from "./pages/StokOpnamePage";
import StokAdjusmentPage from "./pages/StokAdjusmentPage";
import StokAlertPage from "./pages/StokAlertPage";
import AnalitikPage from "./pages/AnalitikPage";
import RiwayatPasienPage from "./pages/RiwayatPasienPage";
import McuFormPage from "./pages/mcu/McuFormPage";
import StokTracePage from "./pages/StokTracePage";
import RujukanPage from "./pages/RujukanPage";
import ReportPendaftaranPage from "./pages/ReportPendaftaranPage";
import ReportPendapatanPage from "./pages/ReportPendapatanPage";
import ReportDiagnosaPage from "./pages/ReportDiagnosaPage";
import ReportDinkesPage from "./pages/ReportDinkesPage";
import PenerimaanBarangPage from "./pages/PenerimaanBarangPage";
import PenerimaanBarangFormPage from "./pages/PenerimaanBarangFormPage";
import OrganisasiLokasiPage from "./pages/OrganisasiLokasiPage";
import PerujukOtcPage from "./pages/PerujukOtcPage";
import MasterMcuPage from "./pages/MasterMcuPage";
import PerusahaanPage from "./pages/PerusahaanPage";
import PasienMcuPage from "./pages/PasienMcuPage";
import LabMasterPage from "./pages/LabMasterPage";
import LabSubMasterPage from "./pages/LabSubMasterPage";
import LabPage from "./pages/LabPage";
import NonLabPage from "./pages/NonLabPage";
import DokterMasterPage from "./pages/DokterMasterPage";
import DokterMcuPage from "./pages/DokterMcuPage";
import LabPricePage from "./pages/LabPricePage";
import PaketPricePage from "./pages/PaketPricePage";
import SettingBarcodePage from "./pages/SettingBarcodePage";
import PrincipalPage from "./pages/PrincipalPage";
import PenunjangPage from "./pages/PenunjangPage";
import MenuPage from "./pages/MenuPage";
import Register from "./pages/Register";
import RolePage from "./pages/RolePage";
import TindakanPage from "./pages/TindakanPage";
import UserPage from "./pages/UserPage";
import "./styles.css";

// After a deploy, a tab still running the previous build asks for chunk files
// that no longer exist ("Failed to fetch dynamically imported module"). Reload
// once to pick up the new build; the guard stops a reload loop.
window.addEventListener("vite:preloadError", (event) => {
  let last = 0;
  try {
    last = Number(sessionStorage.getItem("chunkReloadAt") ?? 0);
  } catch {
    // storage unavailable: still reload once
  }
  if (Date.now() - last < 10_000) return;
  event.preventDefault();
  try {
    sessionStorage.setItem("chunkReloadAt", String(Date.now()));
  } catch {
    // ignore
  }
  window.location.reload();
});

const MCU_ROUTES: [string, string][] = [
  ["/mcu/pemeriksaan-fisik", "fisik"],
  ["/mcu/riwayat-medis", "riwayat-medis"],
  ["/mcu/pemeriksaan-lab", "lab"],
  ["/mcu/pemeriksaan-audiometri", "audiometri"],
  ["/mcu/riwayat-kesehatan-keluarga", "riwayat-keluarga"],
  ["/mcu/riwayat-khusus-wanita", "khusus-wanita"],
  ["/mcu/riwayat-opname-operasi", "opname-operasi"],
  ["/mcu/kebiasaan-pasien", "kebiasaan"],
  ["/mcu/riwayat-pekerjaan", "pekerjaan"],
  ["/mcu/anamnesa", "anamnesa"],
  ["/mcu/pemeriksaan-gigi", "gigi"],
  ["/mcu/rontgen-thorax", "rontgen"],
  ["/mcu/ekg", "ekg"],
  ["/mcu/ekg-treadmill", "treadmill"],
  ["/mcu/spirometry", "spirometri"],
  ["/mcu/usg", "usg"],
];

createRoot(document.getElementById("root")!).render(
  <StrictMode>
    <BrowserRouter>
      <AuthProvider>
        <Routes>
          <Route path="/register" element={<GuestRoute><Register /></GuestRoute>} />
          <Route path="/login" element={<GuestRoute><Login /></GuestRoute>} />
          <Route path="/" element={<Navigate to="/dashboard" replace />} />
          {/* Every page after login shares the sidebar layout. */}
          <Route element={<ProtectedRoute><AppLayout /></ProtectedRoute>}>
            <Route path="/dashboard" element={<Dashboard />} />
            {/* Built pages: only open when the user's role grants the menu item. */}
            <Route path="/management/role" element={<MenuGuard><RolePage /></MenuGuard>} />
            <Route path="/management/user" element={<MenuGuard><UserPage /></MenuGuard>} />
            <Route path="/management/cabang" element={<MenuGuard><BranchPage /></MenuGuard>} />
            <Route path="/master-data-klinik/obat" element={<MenuGuard><ObatPage /></MenuGuard>} />
            <Route path="/master-data-klinik/tindakan" element={<MenuGuard><TindakanPage /></MenuGuard>} />
            <Route path="/master-data-klinik/penunjang" element={<MenuGuard><PenunjangPage /></MenuGuard>} />
            <Route path="/master-data-klinik/pasien" element={<MenuGuard><PasienPage /></MenuGuard>} />
            <Route path="/master-data-klinik/dokter" element={<MenuGuard><DokterPage /></MenuGuard>} />
            <Route path="/master-data-klinik/jaminan-kesehatan" element={<MenuGuard><JamkesPage /></MenuGuard>} />
            <Route path="/master-data-klinik/distributor" element={<MenuGuard><DistributorPage /></MenuGuard>} />
            <Route path="/master-data-klinik/principal" element={<MenuGuard><PrincipalPage /></MenuGuard>} />
            <Route path="/master-data-klinik/icd" element={<MenuGuard><IcdPage /></MenuGuard>} />
            <Route path="/master-data/mcu" element={<MenuGuard><MasterMcuPage /></MenuGuard>} />
            <Route path="/master-data/perusahaan" element={<MenuGuard><PerusahaanPage /></MenuGuard>} />
            <Route path="/master-data/pasien" element={<MenuGuard><PasienMcuPage /></MenuGuard>} />
            <Route path="/master-data/lab-master" element={<MenuGuard><LabMasterPage /></MenuGuard>} />
            <Route path="/master-data/lab-sub-master" element={<MenuGuard><LabSubMasterPage /></MenuGuard>} />
            <Route path="/master-data/lab" element={<MenuGuard><LabPage /></MenuGuard>} />
            <Route path="/master-data/non-lab" element={<MenuGuard><NonLabPage /></MenuGuard>} />
            <Route path="/master-data/dokter-master" element={<MenuGuard><DokterMasterPage /></MenuGuard>} />
            <Route path="/master-data/dokter" element={<MenuGuard><DokterMcuPage /></MenuGuard>} />
            <Route path="/setting/price" element={<MenuGuard><LabPricePage /></MenuGuard>} />
            <Route path="/setting/paket-price" element={<MenuGuard><PaketPricePage /></MenuGuard>} />
            <Route path="/setting/barcode" element={<MenuGuard><SettingBarcodePage /></MenuGuard>} />
            <Route path="/master-data-klinik/penjual-otc" element={<MenuGuard><PerujukOtcPage /></MenuGuard>} />
            <Route path="/front-office-klinik/pendaftaran-pasien" element={<MenuGuard><PendaftaranPage /></MenuGuard>} />
            <Route path="/front-office-klinik/riwayat-pasien" element={<MenuGuard><RiwayatPasienPage /></MenuGuard>} />
            {MCU_ROUTES.map(([path, form]) => (
              <Route key={path} path={path} element={<MenuGuard><McuFormPage key={form} form={form} /></MenuGuard>} />
            ))}
            <Route path="/transaksi-klinik/pengobatan" element={<MenuGuard><PengobatanPage /></MenuGuard>} />
            <Route path="/transaksi-klinik/pengobatan/new" element={<MenuGuard><PengobatanFormPage /></MenuGuard>} />
            <Route path="/transaksi-klinik/pengobatan/edit/:id" element={<MenuGuard><PengobatanFormPage /></MenuGuard>} />
            <Route path="/transaksi-klinik/penerimaan-barang" element={<MenuGuard><PenerimaanBarangPage /></MenuGuard>} />
            <Route path="/transaksi-klinik/penerimaan-barang/new" element={<MenuGuard><PenerimaanBarangFormPage /></MenuGuard>} />
            <Route path="/transaksi-klinik/penerimaan-barang/view/:id" element={<MenuGuard><PenerimaanBarangFormPage /></MenuGuard>} />
            <Route path="/report-klinik/raw-data-pengobatan" element={<MenuGuard><PengobatanPage raw /></MenuGuard>} />
            <Route path="/report-klinik/penjualan-obat" element={<MenuGuard><PenjualanObatPage /></MenuGuard>} />
            <Route path="/report-klinik/penjualan-tindakan" element={<MenuGuard><PenjualanTindakanPage /></MenuGuard>} />
            <Route path="/report-klinik/penjualan-penunjang" element={<MenuGuard><PenjualanPenunjangPage /></MenuGuard>} />
            <Route path="/report-klinik/penjualan-otc" element={<MenuGuard><PenjualanOTCPage /></MenuGuard>} />
            <Route path="/report-klinik/penjualan-softdrink" element={<MenuGuard><PenjualanOTCPage key="softdrink" softdrink /></MenuGuard>} />
            <Route path="/report-klinik/pasien-baru" element={<MenuGuard><PasienBaruPage /></MenuGuard>} />
            <Route path="/report-klinik/rekap-asuransi" element={<MenuGuard><RekapAsuransiPage /></MenuGuard>} />
            <Route path="/report-klinik/transaksi-harian" element={<MenuGuard><TransaksiHarianPage /></MenuGuard>} />
            <Route path="/report-klinik/all-stok" element={<MenuGuard><AllStokPage /></MenuGuard>} />
            <Route path="/stok-farmasi/stok-opname" element={<MenuGuard><StokOpnamePage /></MenuGuard>} />
            <Route path="/stok-farmasi/stok-adjusment" element={<MenuGuard><StokAdjusmentPage /></MenuGuard>} />
            <Route path="/stok-farmasi/stok-alert" element={<MenuGuard><StokAlertPage /></MenuGuard>} />
            <Route path="/stok-farmasi/analitik" element={<MenuGuard><AnalitikPage /></MenuGuard>} />
            <Route path="/stok-farmasi/stok-trace" element={<MenuGuard><StokTracePage /></MenuGuard>} />
            <Route path="/report-klinik/rujukan" element={<MenuGuard><RujukanPage /></MenuGuard>} />
            <Route path="/report-klinik/pendaftaran" element={<MenuGuard><ReportPendaftaranPage /></MenuGuard>} />
            <Route path="/report-klinik/report-pendapatan" element={<MenuGuard><ReportPendapatanPage /></MenuGuard>} />
            <Route path="/report-klinik/report-diagnosa" element={<MenuGuard><ReportDiagnosaPage /></MenuGuard>} />
            <Route path="/report-klinik/report-icd-dinkes" element={<MenuGuard><ReportDinkesPage /></MenuGuard>} />
            <Route path="/master-data-satu-sehat/organisasi-lokasi" element={<MenuGuard><OrganisasiLokasiPage /></MenuGuard>} />
            {/* Routes from the menus table that have no page yet. */}
            <Route path="*" element={<MenuPage />} />
          </Route>
        </Routes>
      </AuthProvider>
    </BrowserRouter>
  </StrictMode>,
);
