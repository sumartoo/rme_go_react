export type FieldErrors = Record<string, string>;

export class ApiError extends Error {
  constructor(
    public status: number,
    message: string,
    public fieldErrors: FieldErrors = {},
  ) {
    super(message);
  }
}

export interface TokenResponse {
  token: string;
  expiresAt: string;
}

export interface MeResponse {
  id: string;
  email: string;
  message: string;
}

/** One node of the sidebar menu, configured in the `menus` table. */
export interface MenuItem {
  id: number;
  title: string;
  icon: string;
  /** Route to open, or null for a group that only expands its children. */
  path: string | null;
  children: MenuItem[];
}

export interface Role {
  id: number;
  name: string;
  /** Every active menu, including menus added later. */
  allAccess: boolean;
  /** Granted menus. A group id grants everything under it. */
  menuIds: number[];
  userCount: number;
}

export type RoleInput = Pick<Role, "name" | "allAccess" | "menuIds">;

export interface UserWithRole {
  id: string;
  email: string;
  roleId: number | null;
  /** Branch codes the user belongs to (refbranchauth); a user can have several. */
  branches: string[];
}

export interface BranchOption {
  kode: string;
  nama: string;
  isActive: boolean;
}

/** Clinic branch (Management > Cabang): a row of refbranch. "" stands for NULL. */
export interface Branch {
  id: number;
  /** Primary key; cannot change after creation. */
  kode: string;
  nama: string;
  alamat: string;
  telp: string;
  /** kode_kwt; may differ from kode (e.g. MED uses BTA). */
  kodeKwt: string;
  npwp: string;
  namaNpwp: string;
  dbHbee: string;
  kodeFasyankes: string;
  namaFasyankes: string;
  clientKey: string;
  /** The secret key itself is never sent to the browser. */
  secretKeySet: boolean;
  /** Last 4 characters of the secret key, or "" */
  secretKeyHint: string;
  organizationId: string;
  isActive: boolean;
  /** Own clinic (true) or partner (false). */
  isInternal: boolean;
}

export type BranchInput = Omit<Branch, "id" | "secretKeySet" | "secretKeyHint"> & {
  /** On update, "" keeps the stored key. */
  secretKey: string;
};

/** A trxstock row: one drug's stock and prices at one branch. */
export interface ObatStock {
  cabang: string;
  stok: number | null;
  stokMin: number | null;
  hbeli: number | null;
  hp: number | null;
  hjual: number | null;
  isActive: boolean;
}

/** A refobat row. Lookup columns are ids; names come from api.obatOptions. */
export interface Obat {
  id: number;
  nama: string;
  isi: string;
  isiPerBox: string;
  idJenis: number | null;
  idLogo: number | null;
  idProd: number | null;
  satBeli: number | null;
  satJual: number | null;
  idMoving: number | null;
  idMovingPo: number | null;
  idDistributor: number | null;
  idPrincipal: number | null;
  idGolongan: number | null;
  qjb: number | null;
  hbeli: number | null;
  hjual: number | null;
  hp: number | null;
  nonAktif: boolean;
  kenaPajak: boolean;
  komposisi: string;
  updatedBy: string;
  updatedAt: string | null;
  /** Only for the branches the user is mapped to. */
  stocks: ObatStock[];
}

export type ObatInput = Omit<Obat, "id" | "updatedBy" | "updatedAt" | "stocks">;
export type ObatStockInput = Omit<ObatStock, "cabang">;

export interface LookupOption {
  id: number;
  name: string;
}

export interface ObatOptions {
  jenis: LookupOption[];
  satuan: LookupOption[];
  logo: LookupOption[];
  produsen: LookupOption[];
  moving: LookupOption[];
  movingPo: LookupOption[];
  distributor: LookupOption[];
  principal: LookupOption[];
  golongan: LookupOption[];
}

/** A reftindakandetail row: one procedure's prices at one branch. */
export interface TindakanPrice {
  cabang: string;
  hbeli: number | null;
  hjual: number | null;
  /** Price for health-insurance (jaminan kesehatan) patients. */
  hjualJamkes: number | null;
  isActive: boolean;
}

/** A reftindakan row. */
export interface Tindakan {
  id: number;
  nama: string;
  idJenis: number | null;
  hbeli: number | null;
  hjual: number | null;
  hjualJamkes: number | null;
  isActive: boolean;
  isEdit: boolean;
  cetakKartu: boolean;
  updatedBy: string;
  updatedAt: string | null;
  /** Only for the branches the user is mapped to. */
  prices: TindakanPrice[];
}

export type TindakanInput = Omit<Tindakan, "id" | "updatedBy" | "updatedAt" | "prices">;
export type TindakanPriceInput = Omit<TindakanPrice, "cabang">;

/** A refpenunjangdetail row: one examination's prices at one branch (lab has no purchase price). */
export interface PenunjangPrice {
  cabang: string;
  hjual: number | null;
  /** Price for health-insurance (jaminan kesehatan) patients. */
  hjualJamkes: number | null;
  isActive: boolean;
}

/** A refpenunjang row (supporting examination: lab, radiology, ...). */
export interface Penunjang {
  id: number;
  nama: string;
  idJenis: number | null;
  hjual: number | null;
  hjualJamkes: number | null;
  isActive: boolean;
  updatedBy: string;
  updatedAt: string | null;
  /** Only for the branches the user is mapped to. */
  prices: PenunjangPrice[];
}

export type PenunjangInput = Omit<Penunjang, "id" | "updatedBy" | "updatedAt" | "prices">;
export type PenunjangPriceInput = Omit<PenunjangPrice, "cabang">;

/** A patient as listed (refmpasien). Dates are YYYY-MM-DD. */
export interface PasienRow {
  id: number;
  noRm: string;
  nama: string;
  tmptLahir: string;
  tglLahir: string | null;
  alamat: string;
  rt: string;
  rw: string;
  cabang: string;
}

/** The full patient record for the edit form. */
export interface Pasien extends PasienRow {
  idKel: number | null;
  kelurahan: string;
  kecamatan: string;
  kabupaten: string;
  provinsi: string;
  telp: string;
  hp: string;
  idGender: number | null;
  idGolDarah: number | null;
  idAgama: number | null;
  idStatusKwn: number | null;
  /** First visit date. */
  tglKun: string | null;
  penanggung: string;
  idJamkes: number | null;
  noJamkes: string;
  nik: string;
  alergi: string;
  ihsId: string;
  updatedBy: string;
  updatedAt: string | null;
}

export type PasienInput = Pick<
  Pasien,
  | "nama" | "tmptLahir" | "tglLahir" | "alamat" | "idKel" | "rt" | "rw" | "telp" | "hp" | "idGender" | "idGolDarah"
  | "idAgama" | "idStatusKwn" | "tglKun" | "penanggung" | "idJamkes" | "noJamkes" | "cabang" | "nik" | "alergi"
>;

export interface PasienOption {
  id: number;
  name: string;
  isActive: boolean;
}

export interface PasienOptions {
  gender: PasienOption[];
  golDarah: PasienOption[];
  agama: PasienOption[];
  statusKwn: PasienOption[];
  jamkes: PasienOption[];
  branches: { kode: string; nama: string }[];
}

export interface KelurahanHit {
  id: number;
  kelurahan: string;
  kecamatan: string;
  kabupaten: string;
  provinsi: string;
}

export interface PasienPage {
  rows: PasienRow[];
  total: number;
  unfilteredTotal: number;
  branches: { kode: string; nama: string }[];
}

/** A doctor (refmdokter) with the codes of its practice branches. */
export interface Dokter {
  id: number;
  nama: string;
  idJenis: number | null;
  tglMasuk: string | null;
  tmptLahir: string;
  tglLahir: string | null;
  alamat: string;
  telp: string;
  hp: string;
  pendidikan: string;
  spesialis: string;
  idGender: number | null;
  idAgama: number | null;
  idGolDarah: number | null;
  idStatusKwn: number | null;
  nik: string;
  isActive: boolean;
  cabangPraktek: string[];
  updatedBy: string;
  updatedAt: string | null;
}

export type DokterInput = Omit<Dokter, "id" | "cabangPraktek" | "updatedBy" | "updatedAt">;

export interface DokterOptions {
  jenis: LookupOption[];
  gender: LookupOption[];
  agama: LookupOption[];
  golDarah: LookupOption[];
  statusKwn: LookupOption[];
}

export interface DokterTindakanItem {
  id: number;
  nama: string;
  jenis: string;
}

export interface DokterTindakanLists {
  assigned: DokterTindakanItem[];
  available: DokterTindakanItem[];
}

export interface DokterCabangLists {
  assigned: { kode: string; nama: string }[];
  available: { kode: string; nama: string }[];
  /** Branches the user may add or remove (their own). */
  editable: string[];
}

/** A health insurer (refjaminankesehatan). */
export interface Jamkes {
  id: number;
  nama: string;
  alamat: string;
  telp: string;
  fax: string;
  /** refjaminankesehatantype (PT, CV, Non PT CV, ...). */
  idType: number | null;
  isActive: boolean;
  /** status_limit 1 = Limit, 2 = No Limit. */
  hasLimit: boolean;
  limit: number | null;
  updatedBy: string;
  updatedAt: string | null;
}

export type JamkesInput = Omit<Jamkes, "id" | "updatedBy" | "updatedAt">;

/** A PKS (agreement) term of an insurer; every term of the master list is returned. */
export interface JamkesTerm {
  detailId: number;
  description: string;
  value: string;
  shown: boolean;
  saved: boolean;
}

export interface JamkesItem {
  no: number;
  value: string;
  shown: boolean;
}

/** A drug distributor (refdistributorobat). */
export interface Distributor {
  id: number;
  nama: string;
  alamat: string;
  telp: string;
  fax: string;
  /** Contact person. */
  cp: string;
  /** Contact person's phone. */
  telpCp: string;
  updatedBy: string;
  updatedAt: string | null;
}

export type DistributorInput = Pick<Distributor, "nama" | "alamat" | "telp" | "fax" | "cp" | "telpCp">;

/** A drug principal (refprincipalobat). */
export interface Principal {
  id: number;
  nama: string;
}

/** An ICD-X diagnosis code (reficd). */
export interface Icd {
  id: number;
  kode: string;
  nama: string;
  idJenis: number;
  /** refjenisicd name (GIGI, THT, ...). */
  jenis: string;
  updatedBy: string;
  updatedAt: string | null;
}

export interface IcdInput {
  kode: string;
  nama: string;
  idJenis: number | null;
}

/** The editable part of an MCU patient (refpasien). gender is "L", "P" or ""; tglLahir is YYYY-MM-DD or "". */
export interface PasienMcuInput {
  nama: string;
  gender: string;
  tglLahir: string;
  telp: string;
  hp: string;
  alamat: string;
  kodePerusahaan: string;
  nik: string;
  email: string;
  departemen: string;
  posisi: string;
}

/** An MCU patient (refpasien) with its company's name. */
export interface PasienMcu extends PasienMcuInput {
  id: number;
  rm: string;
  perusahaan: string;
}

export interface PasienMcuCompany {
  kode: string;
  nama: string;
}

/** A client company (refcompany). isVat is "1" (PPN), "0" (no PPN) or "" (not set). */
export interface PerusahaanInput {
  kode: string;
  nama: string;
  alamat: string;
  contactPerson: string;
  email: string;
  telepon: string;
  fax: string;
  isVat: string;
}

export interface Perusahaan extends PerusahaanInput {
  id: number;
}

/** A group of lab examinations (reflabmaster). */
export interface LabMaster {
  id: number;
  kode: string;
  nama: string;
}

export interface LabMasterInput {
  kode: string;
  nama: string;
}

/** The editable part of an MCU doctor (refdoctor). fee is feepercentage. */
export interface DokterMcuInput {
  nama: string;
  idDokterMaster: number;
  fee: number | null;
}

/** An MCU doctor (refdoctor); kode (doctorcode) is the key and is not edited. */
export interface DokterMcu extends DokterMcuInput {
  kode: string;
  dokterMaster: string;
}

export interface DokterMcuMasterOption {
  id: number;
  nama: string;
}

/** A kind of MCU doctor (refdoctormaster). */
export interface DokterMaster {
  id: number;
  nama: string;
}

/** A service sold with an MCU that is not a lab examination (refnonlab). */
export interface NonLabInput {
  nama: string;
  price: number | null;
}

export interface NonLab extends NonLabInput {
  id: number;
}

/** Setting > Barcode: the MCU barcode label layout (refsettingbarcode). null = not set. */
export interface SettingBarcodeInput {
  left: number | null;
  middle: number | null;
  right: number | null;
  row1: number | null;
  row2: number | null;
  row3: number | null;
  row4: number | null;
  row5: number | null;
  row6: number | null;
  barcodeWidth: number | null;
  barcodeHeight: number | null;
  barcodeLeft: number | null;
  barcodeTop: number | null;
  fontSize: number | null;
  isActive: boolean;
}

export interface SettingBarcode extends SettingBarcodeInput {
  id: number;
}

/** Setting > Paket Price: an MCU package (refpriceheader) as listed. Dates are YYYY-MM-DD. */
export interface PaketPrice {
  id: number;
  idPerusahaan: number;
  perusahaan: string;
  nama: string;
  isPaket: boolean;
  idKategori: number | null;
  kategori: string;
  fromDate: string;
  toDate: string;
  price: number | null;
  isActive: boolean;
  jumlahLab: number;
}

/** A lab examination in (or offered for) a package. */
export interface PaketPriceLab {
  id: number;
  labMaster: string;
  labSubMaster: string;
  nama: string;
  price: number | null;
  isActive: boolean;
}

export interface PaketPriceDetail extends PaketPrice {
  labs: PaketPriceLab[];
}

export interface PaketPriceInput {
  idPerusahaan: number;
  nama: string;
  isPaket: boolean;
  idKategori: number | null;
  fromDate: string;
  toDate: string;
  price: number | null;
  isActive: boolean;
  labIds: number[];
}

export interface PaketPriceOptions {
  perusahaan: { id: number; nama: string }[];
  kategori: { id: number; nama: string }[];
  labs: PaketPriceLab[];
}

/** Setting > Price: a lab examination's price (reflab.price) with its Sub Master and Lab Master. */
export interface LabPrice {
  id: number;
  labMaster: string;
  labSubMaster: string;
  nama: string;
  price: number | null;
  isActive: boolean;
}

/** A lab examination (reflab). id is labid ("Lab Code"); isActive is NOT isdeleted. */
export interface LabInput {
  idLabSubMaster: number;
  kode: string;
  nama: string;
  price: number | null;
  fee: number | null;
  isActive: boolean;
}

export interface Lab extends LabInput {
  id: number;
  labMaster: string;
  labSubMaster: string;
}

export interface LabSubMasterChoice {
  id: number;
  nama: string;
  labMaster: string;
}

/** A subgroup of lab examinations (reflabsubmaster) with its Lab Master's name. */
export interface LabSubMasterInput {
  idLabMaster: number;
  kode: string;
  nama: string;
}

export interface LabSubMaster extends LabSubMasterInput {
  id: number;
  labMaster: string;
}

export interface LabSubMasterOption {
  id: number;
  nama: string;
}

/** An MCU examination group (refmastermcu). */
export interface MasterMcu {
  id: number;
  kode: string;
  nama: string;
}

export interface MasterMcuInput {
  kode: string;
  nama: string;
}

/** A person who refers over-the-counter sales (refperujukotc). */
export interface PerujukOtc {
  id: number;
  nama: string;
}

/** SATUSEHAT Organization and Location of a branch's poli (reforganizationlocation). */
export interface OrganisasiLokasi {
  id: number;
  cabang: string;
  namaCabang: string;
  idJenisRawat: number | null;
  poli: string;
  organisasi: string;
  lokasi: string;
  /** SATUSEHAT ids (UUID); "" until registered. */
  organisasiId: string;
  lokasiId: string;
  updatedBy: string;
  updatedAt: string | null;
}

export type OrganisasiLokasiInput = Pick<OrganisasiLokasi, "organisasi" | "lokasi" | "organisasiId" | "lokasiId">;

/** A visit registration (trxpendaftaran) with its patient. */
export interface Pendaftaran {
  id: number;
  noAntri: number | null;
  /** e.g. U015; null for services without a poli queue. */
  noAntriPoli: string | null;
  /** "YYYY-MM-DD HH:MM:SS", clinic time. */
  tglMasuk: string | null;
  idJenis: number | null;
  pelayanan: string;
  idStatus: number | null;
  status: string;
  idPas: number | null;
  /** "" when the patient no longer exists. */
  noRm: string;
  nama: string;
  tglLahir: string | null;
  alamat: string;
  cabang: string;
  pakaiKartu: boolean;
  totalPoint: number | null;
  cancelBy: string;
  cancelAt: string | null;
  cancelReason: string;
}

export interface PendaftaranService {
  id: number;
  name: string;
  needQueue: boolean;
  firstCode: string;
  isExamination: boolean;
}

export interface PendaftaranOptions {
  pelayanan: PendaftaranService[];
  status: LookupOption[];
  branches: { kode: string; nama: string }[];
  /** Today in the clinic's time zone. */
  today: string;
}

export interface PendaftaranPasien {
  id: number;
  noRm: string;
  nama: string;
  tglLahir: string | null;
  alamat: string;
  nik: string;
  hp: string;
  penanggung: string;
  jamkes: string;
  cabang: string;
}

/** Visit status ids (refstatustransaksi). */
export const STATUS_DAFTAR = 3;

/** Settlement status of a sale (trxjualtotal.status: 1 close, 2 cancel, else open). */
export type PengobatanStatus = "open" | "close" | "cancel";

/** Transaksi Klinik > Pengobatan: one treatment sale (trxjual + trxjualtotal). */
export interface Pengobatan {
  id: number;
  /** "YYYY-MM-DD HH:MM:SS", clinic time. */
  tgl: string | null;
  noKwi: string;
  idPas: number | null;
  noRm: string;
  /** Empty when the patient no longer exists. */
  pasien: string;
  dokter: string;
  idJenisRawat: number | null;
  jenisRawat: string;
  idJenisBayar: number | null;
  pembayaran: string;
  totalObat: number;
  totalTindakan: number;
  totalPenunjang: number;
  potongan: number;
  total: number;
  cabang: string;
  status: PengobatanStatus;
  cancelBy: string;
  cancelAt: string | null;
  cancelReason: string;
  /** Report Klinik > Raw Data Pengobatan columns. */
  alamat: string;
  rt: string;
  rw: string;
  kelurahan: string;
  tglLahir: string | null;
  jenisKelamin: string;
  /** The sale's insurer. */
  asuransi: string;
  /** Dibuat Oleh: who created the sale. */
  createUser: string;
}

export interface PengobatanOptions {
  pelayanan: LookupOption[];
  statusKunjungan: LookupOption[];
  pembayaran: LookupOption[];
  /** Visit statuses a treatment can still be recorded for (DAFTAR, PROSES, PAYMENT). */
  proses: LookupOption[];
  branches: { kode: string; nama: string }[];
  /** Today in the clinic's time zone. */
  today: string;
}

/** A registration offered by the Pengobatan form's "Data Pendaftaran" lookup. */
export interface PengobatanPendaftaran {
  id: number;
  tglMasuk: string | null;
  idJenisRawat: number | null;
  pelayanan: string;
  isPemeriksaan: boolean;
  idStatus: number | null;
  status: string;
  idPas: number | null;
  noRm: string;
  nama: string;
  tglLahir: string | null;
  alamat: string;
  hp: string;
  penanggung: string;
  idJamkes: number | null;
  jamkes: string;
  noJamkes: string;
  cabang: string;
  /** Dokter PJ; null when not chosen yet. */
  idDokter: number | null;
  dokter: string;
}

/** A doctor in the Pengobatan form's Dokter PJ lookup. */
export interface PengobatanDokter {
  id: number;
  nama: string;
  /** refjenisdokter; empty when not set. */
  jenis: string;
}

/** A drug in the Pengobatan form's "Data Obat" lookup, with the branch price and stock. */
/** A branch the logged-in user is mapped to (refbranchauth). */
export interface MyBranch {
  kode: string;
  nama: string;
}

/** A procedure in the Pengobatan form's "Data Tindakan Dokter" lookup, priced for the branch. */
export interface PengobatanTindakan {
  id: number;
  nama: string;
  jenis: string;
  harga: number;
  /** The price may be changed in the form. */
  isEdit: boolean;
}

/** A supporting examination in the Penunjang tab's lookup, priced for the branch. */
export interface PengobatanPenunjang {
  id: number;
  nama: string;
  jenis: string;
  harga: number;
}

/** An ICD-X code in the Diagnosa tab's "Data Diagnosa" lookup. */
export interface PengobatanIcd {
  id: number;
  kode: string;
  nama: string;
  jenis: string;
}

/** The examination of a registration (trxpemeriksaan), as sent to be saved. */
export interface PemeriksaanInput {
  keluhanUtama: string;
  anamnesis: string;
  sistole: number | null;
  diastole: number | null;
  beratBadan: number | null;
  tinggiBadan: number | null;
  suhu: number | null;
  pemeriksaanFisik: string;
  rekapHasilPenunjang: string;
  hasilPenunjang: string;
  rencanaPenatalaksanaan: string;
  hasilPemeriksaanDokter: string;
  tindakanDokter: string;
  /** Dokter PJ. */
  idDokter: number | null;
}

/** A saved examination, with who last changed it. */
export interface PemeriksaanRecord extends PemeriksaanInput {
  dokter: string;
  updatedBy: string;
  updatedAt: string | null;
}

/** MCU > Pendaftaran: a registration in the list. */
export interface McuDaftarRow {
  id: number;
  mcuDate: string | null;
  rm: string;
  status: string;
  nama: string;
  perusahaan: string;
  kategori: string;
  price: number;
  createBy: string;
}

export interface McuDaftarQuery {
  from: string;
  to: string;
  page: number;
  size: number;
  q?: string;
  sort?: string;
  dir?: string;
  status?: string;
}

export interface McuDaftarOptions {
  kategori: LookupOption[];
  info: LookupOption[];
  status: string[];
  today: string;
}

/** An MCU patient in the registration form's lookup. */
export interface McuDaftarPasien {
  id: number;
  rm: string;
  nama: string;
  kodePerusahaan: string;
  perusahaan: string;
  nik: string;
}

/** A General lab examination with its price. */
export interface McuDaftarLab {
  id: number;
  master: string;
  subMaster: string;
  nama: string;
  price: number;
}

/** An MCU package with its company and price. */
export interface McuDaftarPaket {
  id: number;
  nama: string;
  perusahaan: string;
  kategori: string;
  price: number;
}

export interface McuDaftarNonLab {
  id: number;
  nama: string;
  price: number;
}

export interface McuDaftarDokter {
  kode: string;
  nama: string;
  master: string;
  feePercentage: number;
}

/** An ordered line of a saved registration. */
export interface McuDaftarLine {
  id: number;
  nama: string;
  /** General: master / sub master; Paket: perusahaan. */
  extra: string;
  price: number;
}

export interface McuDaftarDetail {
  id: number;
  mcuDate: string | null;
  status: string;
  kategoriId: number | null;
  infoId: number | null;
  price: number;
  createBy: string;
  createdDate: string | null;
  cancelRemarks: string;
  pasien: McuDaftarPasien;
  lab: McuDaftarLine[];
  paket: McuDaftarLine[];
  nonLab: McuDaftarLine[];
  dokter: McuDaftarDokter[];
}

/** The registration form as sent; the server prices the lines. */
export interface McuDaftarInput {
  mcuDate: string;
  kategoriId: number;
  infoId: number | null;
  /** Create only: one registration per patient. */
  pasienIds?: number[];
  labIds: number[];
  paketIds: number[];
  nonLabIds: number[];
  dokter: string[];
}

/** Which API serves an examination: each is guarded by its own menu. */
export type PemeriksaanBase = "/api/pengobatan" | "/api/pemeriksaan-perawat" | "/api/pemeriksaan-dokter" | "/api/proses-obat";

/** Transaksi Klinik > Pemeriksaan Perawat filter choices. */
export interface PerawatOptions {
  status: LookupOption[];
  branches: { kode: string; nama: string }[];
  /** Today in the clinic's time zone. */
  today: string;
}

/** A poli registration in the Pemeriksaan Perawat list. */
export interface PerawatRow extends PengobatanPendaftaran {
  noAntrianPoli: string;
  keluhanUtama: string;
}

export interface PerawatQuery {
  from: string;
  to: string;
  page: number;
  size: number;
  q?: string;
  sort?: string;
  dir?: string;
  cabang?: string;
  status?: number;
  /** Dokter PJ (Pemeriksaan Dokter only). */
  dokter?: number;
}

/** Transaksi Klinik > Pemeriksaan Dokter filter choices. */
export interface DokterPeriksaOptions extends PerawatOptions {
  /** Active doctors of your branches, for the Dokter filter. */
  dokter: PengobatanDokter[];
}

/** A drug the doctor prescribed for a registration. */
export interface ResepObat {
  idObat: number;
  nama: string;
  satuan: string;
  byk: number;
  dosis: string;
}

/** A diagnosis the doctor recorded for a registration. */
export interface ResepDiagnosa {
  icdId: number;
  kode: string;
  nama: string;
}

/** The doctor's prescription and diagnoses of a registration. */
export interface Resep {
  obat: ResepObat[];
  diagnosa: ResepDiagnosa[];
}

/** Replaces the prescription and diagnoses of a registration. */
export interface ResepInput {
  obat: { idObat: number; byk: number; dosis: string }[];
  diagnosa: number[];
}

/** The Pengobatan form as sent by Save; the server prices it from the master data. */
export interface PengobatanSaveInput {
  cabang: string;
  tglKwi: string;
  idKun: number;
  idJenisRawat: number;
  idJenisBayar: number;
  idDokter: number;
  obat: { idObat: number; byk: number; dosis: string }[];
  /** harga only for procedures whose price is editable. */
  tindakan: { idTindakan: number; byk: number; harga: number | null }[];
  penunjang: { idPenunjang: number; byk: number }[];
  diagnosa: number[];
  diskon: number;
  /** Money handed over; null = exactly the amount due. */
  payment: number | null;
}

/** A saved sale as the Edit page shows it (GET /api/pengobatan/detail/{id}). */
export interface PengobatanDetail {
  id: number;
  noKwi: string;
  tglKwi: string | null;
  cabang: string;
  idJenisRawat: number | null;
  idJenisBayar: number | null;
  idDokter: number | null;
  dokter: string;
  status: PengobatanStatus;
  cancelBy: string;
  cancelAt: string | null;
  cancelReason: string;
  /** The registration with its patient; null when it no longer exists. */
  reg: PengobatanPendaftaran | null;
  obat: { idObat: number; nama: string; jenis: string; satuan: string; harga: number; byk: number; kenaPajak: boolean; ppn: number; dosis: string }[];
  tindakan: PengobatanDetailLine[];
  penunjang: PengobatanDetailLine[];
  diagnosa: PengobatanIcd[];
  totObat: number;
  totTindakan: number;
  totPenunjang: number;
  total: number;
  potongan: number;
  grandTotal: number;
  pembayaran: number;
  piutang: number;
  kembalian: number;
}

export interface PengobatanDetailLine {
  id: number;
  nama: string;
  jenis: string;
  dokter?: string;
  harga: number;
  byk: number;
  isEdit: boolean;
}

export interface PengobatanSaveResult {
  id: number;
  noKwi: string;
  totObat: number;
  totTindakan: number;
  totPenunjang: number;
  total: number;
  potongan: number;
  grandTotal: number;
  pembayaran: number;
  cicilan: number;
  piutang: number;
  kembalian: number;
}

/** The receipt data of a saved sale ("Kwitansi"). */
export interface Kuitansi {
  id: number;
  noKwi: string;
  tglKwi: string | null;
  cabang: string;
  namaCabang: string;
  noRm: string;
  pasien: string;
  biayaObat: number;
  biayaTindakan: number;
  biayaPenunjang: number;
  potongan: number;
  /** Total minus discount: the amount on the receipt. */
  jumlah: number;
  /** Empty until the first print. */
  noCetak: string;
  printedBy: string;
  printedAt: string | null;
  /** Set by cetakKuitansi: this print's server time and user. */
  printedNow?: string;
  printedUser?: string;
}

/** A priced line on the printed bill. */
export interface NotaLine {
  nama: string;
  satuan?: string;
  byk: number;
  harga: number;
  jumlah: number;
  ppn?: number;
  dosis?: string;
}

/** A saved sale with its lines, for the Print menu. */
export interface Nota {
  id: number;
  noKwi: string;
  tgl: string | null;
  cabang: string;
  namaCabang: string;
  alamatCabang: string;
  telpCabang: string;
  /** The company the branch is registered under. */
  namaPt: string;
  /** Print number, set at the first print. */
  noCetak: string;
  idKun: number | null;
  noRm: string;
  pasien: string;
  alamatPasien: string;
  penanggung: string;
  /** The patient's points balance. */
  point: number;
  dokter: string;
  pelayanan: string;
  jenisBayar: string;
  jamkes: string;
  kasir: string;
  obat: NotaLine[];
  tindakan: NotaLine[];
  penunjang: NotaLine[];
  diagnosa: string[];
  totObat: number;
  totTindakan: number;
  totPenunjang: number;
  total: number;
  potongan: number;
  grandTotal: number;
  pembayaran: number;
  piutang: number;
  kembalian: number;
  status: PengobatanStatus;
}

export interface PengobatanObat {
  id: number;
  nama: string;
  jenis: string;
  satuan: string;
  harga: number;
  stok: number;
  kenaPajak: boolean;
}

export interface PengobatanPendaftaranQuery {
  cabang: string;
  from: string;
  to: string;
  page: number;
  size: number;
  nama?: string;
  pelayanan?: number;
  status?: number;
  q?: string;
  sort?: string;
  dir?: string;
}

export interface PengobatanQuery {
  from: string;
  to: string;
  page: number;
  size: number;
  q?: string;
  sort?: string;
  dir?: string;
  cabang?: string;
  nama?: string;
  pelayanan?: number;
  statusKunjungan?: number;
  pembayaran?: number;
  status?: PengobatanStatus;
}

/** Report Klinik > Penjualan Obat: a drug sold on one day at one price and branch; byk is summed. */
export interface PenjualanObat {
  tgl: string;
  idObat: number;
  obat: string;
  jenis: string;
  satuan: string;
  harga: number;
  byk: number;
  /** Harga × Banyak. */
  jumlah: number;
  cabang: string;
  status: PengobatanStatus;
}

export interface PenjualanObatQuery {
  from: string;
  to: string;
  page: number;
  size: number;
  q?: string;
  sort?: string;
  dir?: string;
  cabang?: string;
  nama?: string;
  jenis?: number;
  status?: PengobatanStatus;
}

export interface PenjualanObatOptions {
  jenis: { id: number; name: string }[];
  branches: { kode: string; nama: string }[];
  today: string;
}

/** Report Klinik > Penjualan Tindakan: one procedure line sold, with the doctor's fee. */
export interface PenjualanTindakan {
  id: number;
  idJual: number;
  tgl: string;
  tindakan: string;
  jenis: string;
  dokter: string;
  pasien: string;
  /** The branch's doctor fee for the procedure (master data). */
  fee: number;
  /** The price sold at. */
  harga: number;
  byk: number;
  /** Fee × Banyak. */
  totalFee: number;
  cabang: string;
  status: PengobatanStatus;
}

export interface PenjualanTindakanQuery {
  from: string;
  to: string;
  page: number;
  size: number;
  q?: string;
  sort?: string;
  dir?: string;
  cabang?: string;
  nama?: string;
  dokter?: string;
  status?: PengobatanStatus;
}

/** Report Klinik > Penjualan Penunjang: one supporting examination sold. */
export interface PenjualanPenunjang {
  id: number;
  idJual: number;
  tgl: string;
  noRm: string;
  pasien: string;
  penunjang: string;
  jenis: string;
  /** The sale's Dokter PJ. */
  dokter: string;
  harga: number;
  byk: number;
  cabang: string;
  status: PengobatanStatus;
  /** Dibuat Oleh: who created the sale. */
  createUser: string;
}

export interface PenjualanPenunjangQuery {
  from: string;
  to: string;
  page: number;
  size: number;
  q?: string;
  sort?: string;
  dir?: string;
  cabang?: string;
  nama?: string;
  jenis?: number;
  status?: PengobatanStatus;
}

/** Report Klinik > Penjualan OTC: one drug line of type OTC, with the sale's referrer. */
export interface PenjualanOTC {
  id: number;
  /** No Trans. */
  idJual: number;
  tgl: string;
  noKwi: string;
  obat: string;
  jenis: string;
  perujuk: string;
  byk: number;
  harga: number;
  ppn: number;
  /** Harga × Banyak + PPN. */
  total: number;
  cabang: string;
  status: PengobatanStatus;
}

export interface PenjualanOTCQuery {
  from: string;
  to: string;
  page: number;
  size: number;
  q?: string;
  sort?: string;
  dir?: string;
  cabang?: string;
  nama?: string;
  perujuk?: number;
  status?: PengobatanStatus;
}

export interface PenjualanOTCOptions {
  perujuk: { id: number; name: string }[];
  branches: { kode: string; nama: string }[];
  today: string;
}

/** Report Klinik > Pasien Baru: one visit (sale) of a patient registered in the period. */
export interface PasienBaru {
  idJual: number;
  /** Tanggal Kunjungan, "YYYY-MM-DD HH:MM:SS". */
  tgl: string;
  noRm: string;
  pasien: string;
  jenisRawat: string;
  tempatLahir: string;
  tglLahir: string | null;
  /** Age in years on the visit day. */
  umur: number | null;
  alamat: string;
  /** The day the patient was registered. */
  tglDaftar: string;
  dokter: string;
  status: PengobatanStatus;
  cabang: string;
}

export interface PasienBaruQuery {
  from: string;
  to: string;
  page: number;
  size: number;
  q?: string;
  sort?: string;
  dir?: string;
  cabang?: string;
  nama?: string;
  pelayanan?: number;
  status?: PengobatanStatus;
}

export interface PasienBaruOptions {
  pelayanan: { id: number; name: string }[];
  branches: { kode: string; nama: string }[];
  today: string;
}

/** An item of an insured sale (Rekap Asuransi). */
export interface RekapAsuransiLine {
  nama: string;
  byk: number;
  harga: number;
  /** Harga × Banyak. */
  jumlah: number;
}

/** Report Klinik > Rekap Asuransi: one insured sale with its items per group. */
export interface RekapAsuransi {
  idJual: number;
  tgl: string;
  noRm: string;
  /** Card Number. */
  noKartu: string;
  /** The insurer. */
  perusahaan: string;
  pasien: string;
  dokter: string;
  diagnosa: string[];
  obat: RekapAsuransiLine[];
  tindakan: RekapAsuransiLine[];
  penunjang: RekapAsuransiLine[];
  total: number;
  cabang: string;
  status: PengobatanStatus;
}

export interface RekapAsuransiQuery {
  from: string;
  to: string;
  page: number;
  size: number;
  q?: string;
  sort?: string;
  dir?: string;
  cabang?: string;
  nama?: string;
  asuransi?: number;
  status?: PengobatanStatus;
}

export interface RekapAsuransiOptions {
  asuransi: { id: number; name: string }[];
  branches: { kode: string; nama: string }[];
  today: string;
}

/** Report Klinik > Transaksi Harian: one sale with its cost per group. */
export interface TransaksiHarian {
  idJual: number;
  tgl: string;
  noRm: string;
  pasien: string;
  pelayanan: string;
  dokter: string;
  pembayaran: string;
  obat: number;
  tindakan: number;
  penunjang: number;
  total: number;
  cabang: string;
  status: PengobatanStatus;
  diskon: number;
}

export interface TransaksiHarianQuery {
  from: string;
  to: string;
  page: number;
  size: number;
  q?: string;
  sort?: string;
  dir?: string;
  cabang?: string;
  pelayanan?: number;
  nama?: string;
  pembayaran?: number;
  status?: PengobatanStatus;
}

export interface TransaksiHarianOptions {
  pelayanan: { id: number; name: string }[];
  pembayaran: { id: number; name: string }[];
  branches: { kode: string; nama: string }[];
  today: string;
}

/** Report Klinik > All Stok: one drug at one branch with its stock movement over the period. */
export interface AllStok {
  /** Kode. */
  idObat: number;
  obat: string;
  satuanBeli: string;
  isiPerBox: string;
  satuanJual: string;
  hargaBeli: number;
  hargaJual: number;
  jenis: string;
  principal: string;
  distributor: string;
  /** Kode Moving PO. */
  movingPo: string;
  stokAwal: number;
  jual: number;
  stokAkhir: number;
  cabang: string;
}

export interface AllStokQuery {
  from: string;
  to: string;
  page: number;
  size: number;
  q?: string;
  sort?: string;
  dir?: string;
  cabang?: string;
  nama?: string;
  jenis?: number;
  movingPo?: number;
}

export interface AllStokOptions {
  jenis: { id: number; name: string }[];
  movingPo: { id: number; name: string }[];
  branches: { kode: string; nama: string }[];
  today: string;
}

/** Stok Farmasi > Stok Opname: one drug at one branch with its stock and prices. */
export interface StokOpname {
  idObat: number;
  nama: string;
  hargaJual: number;
  hargaBeli: number;
  beratIsi: string;
  idJenis: number | null;
  jenis: string;
  satuanBeli: string;
  satuanJual: string;
  jumUnit: number;
  stokMin: number;
  stok: number;
  cabang: string;
  namaCabang: string;
  kenaPajak: boolean;
  isActive: boolean;
}

export interface StokOpnameQuery {
  page: number;
  size: number;
  q?: string;
  sort?: string;
  dir?: string;
  cabang?: string;
  nama?: string;
  jenis?: number;
  aktif?: "1" | "0";
}

export interface StokOpnameOptions {
  jenis: { id: number; name: string }[];
  branches: { kode: string; nama: string }[];
}

/** Stok Farmasi > Stok Adjusment: a manual stock correction. status 1 = Masuk, 0 = Keluar. */
export interface StokAdjus {
  id: number;
  tanggal: string | null;
  idObat: number;
  nama: string;
  status: 0 | 1;
  statusDesc: string;
  ket: string;
  byk: number;
  cabang: string;
  namaCabang: string;
}

export interface StokAdjusQuery {
  from: string;
  to: string;
  page: number;
  size: number;
  q?: string;
  sort?: string;
  dir?: string;
  cabang?: string;
  nama?: string;
  ket?: string;
  status?: "1" | "0";
}

export interface StokAdjusOptions {
  branches: { kode: string; nama: string }[];
  today: string;
  defaultFrom: string;
}

export interface StokAdjusObat {
  id: number;
  nama: string;
  jenis: string;
  satuan: string;
  stok: number;
}

/** Stok Farmasi > Stok Trace: one stock movement (trxobat). */
export interface StokTrace {
  id: number;
  idObat: number;
  nama: string;
  cabang: string;
  namaCabang: string;
  lastStock: number;
  inOut: string;
  byk: number;
  stock: number;
  systemStock: number;
  tableRef: string;
  transaksi: string;
  remarks: string;
  createdBy: string;
  createdDate: string; // "YYYY-MM-DD HH:MM:SS.mmm"
}

export interface StokTraceQuery {
  cabang: string;
  obat?: number;
  from: string;
  to: string;
  page: number;
  size: number;
  q?: string;
  sort?: string;
  dir?: string;
}

export interface StokAdjusSaveInput {
  cabang: string;
  tanggal: string;
  idObat: number;
  status: 0 | 1;
  byk: number;
  ket: string;
}

/** Report Klinik > Rujukan: a sale of an external doctor, once per supporting examination. */
export interface Rujukan {
  idJual: number;
  /** The examination line; null for a sale without one. */
  idLine: number | null;
  tgl: string;
  noRm: string;
  pasien: string;
  /** Dokter Eksternal. */
  dokter: string;
  jenisRawat: string;
  penunjang: string;
  /** The examination's Harga × Banyak; 0 without one. */
  total: number;
  cabang: string;
  status: PengobatanStatus;
}

export interface RujukanQuery {
  from: string;
  to: string;
  page: number;
  size: number;
  q?: string;
  sort?: string;
  dir?: string;
  cabang?: string;
  nama?: string;
  dokter?: string;
  status?: PengobatanStatus;
}

/** Report Klinik > Pendaftaran: one registration. */
export interface ReportPendaftaran {
  /** No Pendaftaran. */
  id: number;
  noAntri: number | null;
  /** "YYYY-MM-DD HH:MM:SS". */
  tgl: string | null;
  pelayanan: string;
  /** Status Kun, e.g. CLOSE, DAFTAR. */
  statusKun: string;
  noRm: string;
  pasien: string;
  /** Age in years on the registration day. */
  umur: number | null;
  alamat: string;
  cabang: string;
}

export interface ReportPendaftaranQuery {
  from: string;
  to: string;
  page: number;
  size: number;
  q?: string;
  sort?: string;
  dir?: string;
  cabang?: string;
  nama?: string;
  pelayanan?: number;
  statusKunjungan?: number;
}

export interface ReportPendaftaranOptions {
  pelayanan: { id: number; name: string }[];
  statusKunjungan: { id: number; name: string }[];
  branches: { kode: string; nama: string }[];
  today: string;
}

/** Report Klinik > Report Pendapatan: one branch's sales in a period, for the PDF. */
export interface ReportPendapatan {
  cabang: string;
  namaCabang: string;
  telpCabang: string;
  from: string;
  to: string;
  /** Tanggal Posting: today, clinic time. */
  today: string;
  /** The caller (e-mail without its domain). */
  petugas: string;
  rows: {
    idJual: number;
    /** "YYYY-MM-DD HH:MM:SS.mmm". */
    tgl: string;
    noKwi: string;
    pasien: string;
    obat: number;
    tindakan: number;
    penunjang: number;
    total: number;
    potongan: number;
    /** The patient was registered in the period. */
    baru: boolean;
  }[];
}

export interface ReportPendapatanOptions {
  pelayanan: { id: number; name: string }[];
  branches: { kode: string; nama: string }[];
  today: string;
}

/** Report Klinik > Report Diagnosa: one diagnosis of a sale. */
export interface ReportDiagnosa {
  id: number;
  idJual: number;
  /** "YYYY-MM-DD HH:MM:SS". */
  tgl: string;
  noRm: string;
  pasien: string;
  tglLahir: string | null;
  /** Age in years on the visit day. */
  usia: number | null;
  jenisKelamin: string;
  dokter: string;
  jenisRawat: string;
  /** ICD.X code. */
  icd: string;
  diagnosa: string;
  /** "DRUG (qty) , DRUG (qty)". */
  resep: string;
  /** Keluhan Utama of the visit's examination. */
  keluhan: string;
  cabang: string;
}

export interface ReportDiagnosaQuery {
  from: string;
  to: string;
  page: number;
  size: number;
  q?: string;
  sort?: string;
  dir?: string;
  cabang?: string;
  nama?: string;
  diagnosa?: string;
  pelayanan?: number;
}

/** Report Klinik > Report ICD Dinkes: the cases of one ICD code and service, by age group and sex. */
export interface ReportDinkes {
  kode: string;
  icd: string;
  jenisRawat: string;
  hari0_7: number;
  hari8_28: number;
  bulan1_11: number;
  tahun1_4: number;
  tahun5_9: number;
  tahun10_14: number;
  tahun15_19: number;
  tahun20_44: number;
  tahun45_59: number;
  /** Over 59 years. */
  tahun60: number;
  laki: number;
  perempuan: number;
  total: number;
}

export interface ReportDinkesQuery {
  from: string;
  to: string;
  cabang: string;
  page: number;
  size: number;
  q?: string;
  sort?: string;
  dir?: string;
}

/** Transaksi Klinik > Penerimaan Barang: one goods receipt (trxfaktur). */
export interface PenerimaanBarang {
  /** No. */
  id: number;
  /** No GR (trxfaktur.no_fak). */
  noGr: string;
  /** Tgl Trans: the entry date. */
  tglTrans: string | null;
  /** Tgl GR: the receipt date. */
  tglGr: string | null;
  /** Empty when the receipt has no supplier. */
  supplier: string;
  total: number;
  cabang: string;
}

export interface PenerimaanBarangQuery {
  from: string;
  to: string;
  page: number;
  size: number;
  q?: string;
  sort?: string;
  dir?: string;
  cabang?: string;
  noGr?: string;
  /** refsupplier id; 0 = no supplier. */
  supplier?: number;
}

export interface PenerimaanBarangOptions {
  suppliers: { id: number; name: string }[];
  branches: { kode: string; nama: string }[];
  today: string;
}

/** A row of the receipt form's "Data Distributor" lookup. */
export interface PenerimaanDistributor {
  id: number;
  nama: string;
  alamat: string;
}

/** A row of the receipt form's "Data Obat" lookup: a drug stocked at the branch. */
export interface PenerimaanObat {
  id: number;
  nama: string;
  jenis: string;
  /** The selling unit (the unit of the stock). */
  satuan: string;
  /** The branch's buying price (trxstock.hbeli). */
  harga: number;
  stok: number;
}

export interface PenerimaanSaveInput {
  cabang: string;
  idDistributor: number;
  noGr: string;
  tglGr: string;
  obat: { idObat: number; noBatch: string; byk: number; tglExp: string; disc: number }[];
}

/** A saved goods receipt for the read-only View page. */
export interface PenerimaanDetail {
  id: number;
  noGr: string;
  /** "YYYY-MM-DD HH:MM:SS". */
  tglTrans: string | null;
  tglGr: string | null;
  cabang: string;
  idDistributor: number | null;
  distributor: string;
  supplier: string;
  createUser: string;
  total: number;
  obat: { idObat: number; nama: string; jenis: string; satuan: string; harga: number; noBatch: string; byk: number; tglExp: string | null; disc: number }[];
}

export interface PenerimaanSaveResult {
  /** No Trans. */
  id: number;
  total: number;
  noClose: number | null;
}

/** Fields of the patient "Pencarian" panel in the registration form. */
export type PasienSearchField = "norm" | "nama" | "umur" | "alamat" | "jamkes";

/** Stok Farmasi > Analitik. */
export type AnalitikStatus = "habis" | "kritis" | "perlu-order" | "aman" | "tidak-bergerak";

export interface AnalitikOptions {
  branches: { kode: string; nama: string }[];
  today: string;
  /** false when the backend has no DEEPSEEK_API_KEY. */
  aiEnabled: boolean;
  historyDays: number;
  horizon: number;
  leadTime: number;
  weeks: number;
}

/** Usage forecast and order advice for one drug at one branch. */
export interface AnalitikObat {
  idObat: number;
  nama: string;
  satuan: string;
  cabang: string;
  namaCabang: string;
  stok: number;
  stokMin: number;
  avgHarian: number;
  terpakai: number;
  hari: number;
  lowData: boolean;
  kebutuhan: number;
  safetyStock: number;
  rop: number;
  hargaPokok: number | null;
  estimasiBiaya: number | null;
  saranOrder: number;
  habisDalamHari: number | null;
  status: AnalitikStatus;
}

export interface AnalitikObatResult {
  rows: AnalitikObat[];
  total: number;
  from: string;
  to: string;
  horizon: number;
  leadTime: number;
  ringkas: Partial<Record<AnalitikStatus, number>>;
  biayaOrder: number;
  orderTanpaHarga: number;
}

export interface AnalitikIcdTrend {
  idIcd: number;
  kode: string;
  nama: string;
  weeks: number[];
  total: number;
  mingguIni: number;
  rataSebelum: number;
  rasio: number;
  lonjakan: boolean;
  proyeksiMinggu: number;
}

export interface AnalitikPenyakit {
  weekStarts: string[];
  kunjungan: number[];
  top: AnalitikIcdTrend[];
  lonjakan: AnalitikIcdTrend[];
}

export interface AnalitikResep {
  idObat: number;
  nama: string;
  satuan: string;
  kunjungan: number;
  qty: number;
  support: number;
  qtyPerKasus: number;
}

export interface AnalitikDiagnosa {
  idIcd: number;
  kode: string;
  nama: string;
  kunjungan: number;
  proyeksiMinggu: number;
  lonjakan: boolean;
  obat: AnalitikResep[];
}

export interface AnalitikSaran {
  idObat: number;
  nama: string;
  satuan: string;
  kebutuhan: number;
  stok: number;
  kurang: number;
  dari: string[];
}

export interface AnalitikDiagnosaObatResult {
  diagnosa: AnalitikDiagnosa[];
  saran: AnalitikSaran[];
  from: string;
  to: string;
  horizon: number;
}

export interface AnalitikChatMessage {
  role: "user" | "assistant";
  content: string;
}

export interface AnalitikKinerja {
  kelompok: string;
  transaksi: number;
  kunjungan: number;
  pasien: number;
  pendapatan: number;
  obat: number;
  tindakan: number;
  penunjang: number;
}

export interface AnalitikKPI {
  obatHabis: number;
  obatKritis: number;
  obatPerluOrder: number;
  biayaOrder: number;
  orderTanpaHarga: number;
  lonjakan: number;
  transaksi7: number;
  transaksi7Lalu: number;
  transaksiDelta: number | null;
  pendapatan30: number;
  pendapatan30Lalu: number;
  pendapatanDelta: number | null;
  tidakBergerak: number;
}

export interface AnalitikInsight {
  level: "kritis" | "perhatian" | "info";
  judul: string;
  detail: string;
}

export interface AnalitikCommand {
  kpi: AnalitikKPI;
  insights: AnalitikInsight[];
  mingguan: AnalitikKinerja[];
  today: string;
}

export interface AnalitikCekKualitas {
  kode: string;
  judul: string;
  penjelasan: string;
  jumlah: number;
  kolom: string[];
  contoh: string[][];
}

export interface AnalitikFollowUp {
  idPasien: number;
  noRm: string;
  nama: string;
  telp: string;
  kelompok: string;
  kodeIcd: string;
  terakhir: string;
  hariSejak: number;
  jumlahKontrol: number;
  cabang: string;
}

export interface IcdKandidat {
  id: number;
  kode: string;
  nama: string;
  jenis: string;
  alasan: string;
  sumber: "ai" | "pencarian";
}

/** Front Office Klinik > Riwayat Pasien. */
export interface RiwayatItem {
  kode?: string;
  nama: string;
  byk?: number | null;
  dosis?: string;
}

export interface RiwayatKunjungan {
  idJual: number;
  tgl: string;
  noKwi: string;
  cabang: string;
  dokter: string;
  jenisRawat: string;
  batal: boolean;
  total: number | null;
  keluhan: string;
  anamnesis: string;
  sistole: number | null;
  diastole: number | null;
  bb: number | null;
  tb: number | null;
  suhu: number | null;
  hasilDokter: string;
  rencana: string;
  diagnosa: RiwayatItem[];
  resep: RiwayatItem[];
  tindakan: RiwayatItem[];
}

export interface RiwayatJumlah {
  kode?: string;
  nama: string;
  kali: number;
  terakhir: string;
}

export interface RiwayatVital {
  tgl: string;
  sistole: number | null;
  diastole: number | null;
  bb: number | null;
  tb: number | null;
  imt: number | null;
}

export interface RiwayatPasien {
  pasien: {
    id: number;
    noRm: string;
    nama: string;
    tglLahir: string | null;
    usia: number | null;
    jenisKelamin: string;
    alamat: string;
    telp: string;
    alergi: string;
    penjamin: string;
    cabang: string;
  };
  ringkasan: {
    jumlahKunjungan: number;
    pertama: string;
    terakhir: string;
    cabang: string[];
    diagnosa: RiwayatJumlah[];
    obat: RiwayatJumlah[];
    vital: RiwayatVital[];
  };
  kunjungan: RiwayatKunjungan[];
  maxKunjungan: number;
}

export interface RiwayatPasienHit {
  id: number;
  noRm: string;
  nama: string;
  tglLahir: string | null;
  alamat: string;
  nik: string;
  hp: string;
  cabang: string;
}

/** MCU result forms (MCU > Pemeriksaan Fisik … USG). */
export interface McuRegistrasi {
  id: number;
  tanggal: string;
  status: string;
  pasienId: number;
  rm: string;
  nama: string;
  gender: string;
  tglLahir: string | null;
  usia: number | null;
  perusahaan: string;
  departemen: string;
  jabatan: string;
  formStatus: string | null;
}

export interface McuOption {
  nama: string;
  entry: "radio" | "text" | string;
}

export interface McuItem {
  key: string;
  nama: string;
  options: McuOption[];
}

export interface McuSection {
  kode: string;
  nama: string;
  grup: string;
  items: McuItem[];
}

export interface McuValue {
  value: string;
  note: string;
}

export interface McuHasil {
  status: string;
  kesimpulan: string;
  updatedBy: string;
  updatedAt: string | null;
  values: Record<string, McuValue>;
}

export interface McuLabItem {
  labId: number;
  kode: string;
  nama: string;
  grup: string;
  paket: string;
}

async function request<T>(path: string, options: { method?: string; body?: unknown; token?: string | null } = {}): Promise<T> {
  const headers: Record<string, string> = {};
  if (options.body !== undefined) headers["Content-Type"] = "application/json";
  if (options.token) headers["Authorization"] = `Bearer ${options.token}`;

  const res = await fetch(path, {
    method: options.method ?? "GET",
    headers,
    body: options.body !== undefined ? JSON.stringify(options.body) : undefined,
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) {
    throw new ApiError(res.status, data.error ?? `Request failed (${res.status})`, data.errors ?? {});
  }
  return data as T;
}

export const api = {
  register: (email: string, password: string, confirmPassword: string) =>
    request<{ id: string; email: string }>("/api/auth/register", {
      method: "POST",
      body: { email, password, confirmPassword },
    }),
  login: (email: string, password: string) =>
    request<TokenResponse>("/api/auth/login", { method: "POST", body: { email, password } }),
  refresh: (token: string) => request<TokenResponse>("/api/auth/refresh", { method: "POST", token }),
  me: (token: string) => request<MeResponse>("/api/me", { token }),
  myBranches: (token: string) => request<{ branches: MyBranch[] }>("/api/me/branches", { token }),
  /** Menus the caller's role can open, and the role's name. */
  menus: (token: string) => request<{ menus: MenuItem[]; role: string | null }>("/api/menus", { token }),

  // Management > Role
  roles: (token: string) => request<{ roles: Role[]; menus: MenuItem[] }>("/api/roles", { token }),
  createRole: (token: string, input: RoleInput) => request<Role>("/api/roles", { method: "POST", body: input, token }),
  updateRole: (token: string, id: number, input: RoleInput) =>
    request<Role>(`/api/roles/${id}`, { method: "PUT", body: input, token }),
  deleteRole: (token: string, id: number) => request<unknown>(`/api/roles/${id}`, { method: "DELETE", token }),

  // Management > User
  users: (token: string) =>
    request<{ users: UserWithRole[]; roles: { id: number; name: string }[]; branches: BranchOption[] }>("/api/users", { token }),
  setUserRole: (token: string, userId: string, roleId: number | null) =>
    request<{ id: string; roleId: number | null }>(`/api/users/${userId}/role`, { method: "PUT", body: { roleId }, token }),
  /** The menu tree a user gets from their role (what their sidebar shows). */
  userMenus: (token: string, userId: string) =>
    request<{ role: string | null; allAccess: boolean; menus: MenuItem[] }>(`/api/users/${userId}/menus`, { token }),
  setUserBranches: (token: string, userId: string, branches: string[]) =>
    request<{ id: string; branches: string[] }>(`/api/users/${userId}/branches`, { method: "PUT", body: { branches }, token }),

  // Master Data Klinik > Obat
  obat: (token: string) => request<{ obat: Obat[]; branches: { kode: string; nama: string }[] }>("/api/obat", { token }),
  obatOptions: (token: string) => request<ObatOptions>("/api/obat/options", { token }),
  createObat: (token: string, input: ObatInput) => request<Obat>("/api/obat", { method: "POST", body: input, token }),
  updateObat: (token: string, id: number, input: ObatInput) =>
    request<Obat>(`/api/obat/${id}`, { method: "PUT", body: input, token }),
  setObatStock: (token: string, id: number, cabang: string, input: ObatStockInput) =>
    request<Obat>(`/api/obat/${id}/stock/${encodeURIComponent(cabang)}`, { method: "PUT", body: input, token }),

  // Master Data Klinik > Tindakan
  tindakan: (token: string) =>
    request<{ tindakan: Tindakan[]; branches: { kode: string; nama: string }[] }>("/api/tindakan", { token }),
  tindakanOptions: (token: string) => request<{ jenis: LookupOption[] }>("/api/tindakan/options", { token }),
  createTindakan: (token: string, input: TindakanInput) =>
    request<Tindakan>("/api/tindakan", { method: "POST", body: input, token }),
  updateTindakan: (token: string, id: number, input: TindakanInput) =>
    request<Tindakan>(`/api/tindakan/${id}`, { method: "PUT", body: input, token }),
  setTindakanPrice: (token: string, id: number, cabang: string, input: TindakanPriceInput) =>
    request<Tindakan>(`/api/tindakan/${id}/price/${encodeURIComponent(cabang)}`, { method: "PUT", body: input, token }),

  // Master Data Klinik > Penunjang
  penunjang: (token: string) =>
    request<{ penunjang: Penunjang[]; branches: { kode: string; nama: string }[] }>("/api/penunjang", { token }),
  penunjangOptions: (token: string) => request<{ jenis: LookupOption[] }>("/api/penunjang/options", { token }),
  createPenunjang: (token: string, input: PenunjangInput) =>
    request<Penunjang>("/api/penunjang", { method: "POST", body: input, token }),
  updatePenunjang: (token: string, id: number, input: PenunjangInput) =>
    request<Penunjang>(`/api/penunjang/${id}`, { method: "PUT", body: input, token }),
  setPenunjangPrice: (token: string, id: number, cabang: string, input: PenunjangPriceInput) =>
    request<Penunjang>(`/api/penunjang/${id}/price/${encodeURIComponent(cabang)}`, { method: "PUT", body: input, token }),

  // Master Data Klinik > Pasien (server-side paging)
  pasien: (token: string, params: { page: number; size: number; q?: string; sort?: string; dir?: string; cabang?: string }) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<PasienPage>(`/api/pasien?${qs}`, { token });
  },
  pasienOptions: (token: string) => request<PasienOptions>("/api/pasien/options", { token }),
  pasienDetail: (token: string, id: number) => request<Pasien>(`/api/pasien/${id}`, { token }),
  searchKelurahan: (token: string, q: string) =>
    request<{ kelurahan: KelurahanHit[] }>(`/api/pasien/kelurahan?q=${encodeURIComponent(q)}`, { token }),
  createPasien: (token: string, input: PasienInput) => request<Pasien>("/api/pasien", { method: "POST", body: input, token }),
  updatePasien: (token: string, id: number, input: PasienInput) =>
    request<Pasien>(`/api/pasien/${id}`, { method: "PUT", body: input, token }),

  // Master Data Klinik > Dokter
  dokter: (token: string) => request<{ dokter: Dokter[]; branches: { kode: string; nama: string }[] }>("/api/dokter", { token }),
  dokterOptions: (token: string) => request<DokterOptions>("/api/dokter/options", { token }),
  createDokter: (token: string, input: DokterInput) => request<Dokter>("/api/dokter", { method: "POST", body: input, token }),
  updateDokter: (token: string, id: number, input: DokterInput) =>
    request<Dokter>(`/api/dokter/${id}`, { method: "PUT", body: input, token }),
  dokterTindakan: (token: string, id: number) => request<DokterTindakanLists>(`/api/dokter/${id}/tindakan`, { token }),
  changeDokterTindakan: (token: string, id: number, change: { add?: number[]; remove?: number[] }) =>
    request<DokterTindakanLists>(`/api/dokter/${id}/tindakan`, { method: "PATCH", body: change, token }),
  dokterCabang: (token: string, id: number) => request<DokterCabangLists>(`/api/dokter/${id}/cabang`, { token }),
  changeDokterCabang: (token: string, id: number, change: { add?: string[]; remove?: string[] }) =>
    request<DokterCabangLists>(`/api/dokter/${id}/cabang`, { method: "PATCH", body: change, token }),

  // Master Data Klinik > Jaminan Kesehatan
  jamkes: (token: string) => request<{ jamkes: Jamkes[]; types: LookupOption[] }>("/api/jamkes", { token }),
  createJamkes: (token: string, input: JamkesInput) => request<Jamkes>("/api/jamkes", { method: "POST", body: input, token }),
  updateJamkes: (token: string, id: number, input: JamkesInput) =>
    request<Jamkes>(`/api/jamkes/${id}`, { method: "PUT", body: input, token }),
  jamkesPks: (token: string, id: number) => request<{ terms: JamkesTerm[] }>(`/api/jamkes/${id}/pks`, { token }),
  saveJamkesPks: (token: string, id: number, terms: { detailId: number; value: string; shown: boolean }[]) =>
    request<{ terms: JamkesTerm[] }>(`/api/jamkes/${id}/pks`, { method: "PUT", body: { terms }, token }),
  jamkesItems: (token: string, id: number) => request<{ items: JamkesItem[] }>(`/api/jamkes/${id}/items`, { token }),
  saveJamkesItems: (token: string, id: number, items: { value: string; shown: boolean }[]) =>
    request<{ items: JamkesItem[] }>(`/api/jamkes/${id}/items`, { method: "PUT", body: { items }, token }),

  // Master Data Klinik > Distributor
  distributor: (token: string) => request<{ distributor: Distributor[] }>("/api/distributor", { token }),
  createDistributor: (token: string, input: DistributorInput) =>
    request<Distributor>("/api/distributor", { method: "POST", body: input, token }),
  updateDistributor: (token: string, id: number, input: DistributorInput) =>
    request<Distributor>(`/api/distributor/${id}`, { method: "PUT", body: input, token }),

  // Master Data Klinik > Principal
  principal: (token: string) => request<{ principal: Principal[] }>("/api/principal", { token }),
  createPrincipal: (token: string, nama: string) => request<Principal>("/api/principal", { method: "POST", body: { nama }, token }),
  updatePrincipal: (token: string, id: number, nama: string) =>
    request<Principal>(`/api/principal/${id}`, { method: "PUT", body: { nama }, token }),

  // Master Data Klinik > ICD (server-side paging)
  icd: (token: string, params: { page: number; size: number; q?: string; sort?: string; dir?: string }) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: Icd[]; total: number; unfilteredTotal: number }>(`/api/icd?${qs}`, { token });
  },
  icdOptions: (token: string) => request<{ jenis: LookupOption[] }>("/api/icd/options", { token }),
  createIcd: (token: string, input: IcdInput) => request<Icd>("/api/icd", { method: "POST", body: input, token }),
  updateIcd: (token: string, id: number, input: IcdInput) => request<Icd>(`/api/icd/${id}`, { method: "PUT", body: input, token }),

  // Master Data > Pasien (MCU patients)
  pasienMcu: (token: string, params: { page: number; size: number; q?: string; sort?: string; dir?: string }) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: PasienMcu[]; total: number; unfilteredTotal: number }>(`/api/pasien-mcu?${qs}`, { token });
  },
  pasienMcuOptions: (token: string) => request<{ perusahaan: PasienMcuCompany[] }>("/api/pasien-mcu/options", { token }),
  createPasienMcu: (token: string, input: PasienMcuInput) => request<PasienMcu>("/api/pasien-mcu", { method: "POST", body: input, token }),
  updatePasienMcu: (token: string, id: number, input: PasienMcuInput) =>
    request<PasienMcu>(`/api/pasien-mcu/${id}`, { method: "PUT", body: input, token }),
  // Master Data > Perusahaan
  perusahaan: (token: string) => request<{ perusahaan: Perusahaan[] }>("/api/perusahaan", { token }),
  createPerusahaan: (token: string, input: PerusahaanInput) => request<Perusahaan>("/api/perusahaan", { method: "POST", body: input, token }),
  updatePerusahaan: (token: string, id: number, input: PerusahaanInput) =>
    request<Perusahaan>(`/api/perusahaan/${id}`, { method: "PUT", body: input, token }),
  // Master Data > Lab Master
  labMaster: (token: string) => request<{ labMaster: LabMaster[] }>("/api/lab-master", { token }),
  createLabMaster: (token: string, input: LabMasterInput) => request<LabMaster>("/api/lab-master", { method: "POST", body: input, token }),
  updateLabMaster: (token: string, id: number, input: LabMasterInput) =>
    request<LabMaster>(`/api/lab-master/${id}`, { method: "PUT", body: input, token }),
  // Master Data > Dokter (MCU doctors)
  dokterMcu: (token: string) => request<{ dokter: DokterMcu[] }>("/api/dokter-mcu", { token }),
  dokterMcuOptions: (token: string) => request<{ dokterMaster: DokterMcuMasterOption[] }>("/api/dokter-mcu/options", { token }),
  createDokterMcu: (token: string, input: DokterMcuInput & { kode: string }) =>
    request<DokterMcu>("/api/dokter-mcu", { method: "POST", body: input, token }),
  updateDokterMcu: (token: string, kode: string, input: DokterMcuInput) =>
    request<DokterMcu>(`/api/dokter-mcu/${encodeURIComponent(kode)}`, { method: "PUT", body: input, token }),
  // Master Data > Dokter Master
  dokterMaster: (token: string) => request<{ dokterMaster: DokterMaster[] }>("/api/dokter-master", { token }),
  createDokterMaster: (token: string, nama: string) => request<DokterMaster>("/api/dokter-master", { method: "POST", body: { nama }, token }),
  updateDokterMaster: (token: string, id: number, nama: string) =>
    request<DokterMaster>(`/api/dokter-master/${id}`, { method: "PUT", body: { nama }, token }),
  // Master Data > Non Lab
  nonLab: (token: string) => request<{ nonLab: NonLab[] }>("/api/non-lab", { token }),
  createNonLab: (token: string, input: NonLabInput) => request<NonLab>("/api/non-lab", { method: "POST", body: input, token }),
  updateNonLab: (token: string, id: number, input: NonLabInput) => request<NonLab>(`/api/non-lab/${id}`, { method: "PUT", body: input, token }),
  // Setting > Barcode
  settingBarcode: (token: string) => request<{ setting: SettingBarcode[] }>("/api/setting-barcode", { token }),
  updateSettingBarcode: (token: string, id: number, input: SettingBarcodeInput) =>
    request<SettingBarcode>(`/api/setting-barcode/${id}`, { method: "PUT", body: input, token }),
  // Setting > Paket Price
  paketPrice: (token: string) => request<{ paket: PaketPrice[] }>("/api/paket-price", { token }),
  paketPriceOptions: (token: string) => request<PaketPriceOptions>("/api/paket-price/options", { token }),
  paketPriceDetail: (token: string, id: number) => request<PaketPriceDetail>(`/api/paket-price/${id}`, { token }),
  createPaketPrice: (token: string, input: PaketPriceInput) =>
    request<PaketPriceDetail>("/api/paket-price", { method: "POST", body: input, token }),
  updatePaketPrice: (token: string, id: number, input: PaketPriceInput) =>
    request<PaketPriceDetail>(`/api/paket-price/${id}`, { method: "PUT", body: input, token }),
  deletePaketPrice: (token: string, id: number) => request<unknown>(`/api/paket-price/${id}`, { method: "DELETE", token }),
  // Setting > Price
  labPrice: (token: string) => request<{ price: LabPrice[] }>("/api/lab-price", { token }),
  updateLabPrice: (token: string, id: number, price: number | null) =>
    request<LabPrice>(`/api/lab-price/${id}`, { method: "PUT", body: { price }, token }),
  // Master Data > Lab
  lab: (token: string) => request<{ lab: Lab[] }>("/api/lab", { token }),
  labOptions: (token: string) => request<{ labSubMaster: LabSubMasterChoice[] }>("/api/lab/options", { token }),
  createLab: (token: string, input: LabInput) => request<Lab>("/api/lab", { method: "POST", body: input, token }),
  updateLab: (token: string, id: number, input: LabInput) => request<Lab>(`/api/lab/${id}`, { method: "PUT", body: input, token }),
  // Master Data > Lab Sub Master
  labSubMaster: (token: string) => request<{ labSubMaster: LabSubMaster[] }>("/api/lab-sub-master", { token }),
  labSubMasterOptions: (token: string) => request<{ labMaster: LabSubMasterOption[] }>("/api/lab-sub-master/options", { token }),
  createLabSubMaster: (token: string, input: LabSubMasterInput) => request<LabSubMaster>("/api/lab-sub-master", { method: "POST", body: input, token }),
  updateLabSubMaster: (token: string, id: number, input: LabSubMasterInput) =>
    request<LabSubMaster>(`/api/lab-sub-master/${id}`, { method: "PUT", body: input, token }),
  // Master Data > MCU
  masterMcu: (token: string) => request<{ mcu: MasterMcu[] }>("/api/master-mcu", { token }),
  createMasterMcu: (token: string, input: MasterMcuInput) => request<MasterMcu>("/api/master-mcu", { method: "POST", body: input, token }),
  updateMasterMcu: (token: string, id: number, input: MasterMcuInput) =>
    request<MasterMcu>(`/api/master-mcu/${id}`, { method: "PUT", body: input, token }),
  // Master Data Klinik > Penjual OTC (Perujuk OTC)
  perujukOtc: (token: string) => request<{ perujuk: PerujukOtc[] }>("/api/perujuk-otc", { token }),
  createPerujukOtc: (token: string, nama: string) => request<PerujukOtc>("/api/perujuk-otc", { method: "POST", body: { nama }, token }),
  updatePerujukOtc: (token: string, id: number, nama: string) =>
    request<PerujukOtc>(`/api/perujuk-otc/${id}`, { method: "PUT", body: { nama }, token }),

  // Master Data Satu Sehat > Organisasi & Lokasi
  organisasiLokasi: (token: string) =>
    request<{ rows: OrganisasiLokasi[]; branches: { kode: string; nama: string }[] }>("/api/organisasi-lokasi", { token }),
  updateOrganisasiLokasi: (token: string, id: number, input: OrganisasiLokasiInput) =>
    request<OrganisasiLokasi>(`/api/organisasi-lokasi/${id}`, { method: "PUT", body: input, token }),

  // Transaksi Klinik > Pengobatan (server-side paging)
  pengobatan: (token: string, params: PengobatanQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: Pengobatan[]; total: number; unfilteredTotal: number }>(`/api/pengobatan?${qs}`, { token });
  },
  pengobatanOptions: (token: string) => request<PengobatanOptions>("/api/pengobatan/options", { token }),
  // Report Klinik > Raw Data Pengobatan: the same list with patient columns, guarded by the report menu.
  rawDataPengobatan: (token: string, params: PengobatanQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: Pengobatan[]; total: number; unfilteredTotal: number }>(`/api/report-klinik/raw-data-pengobatan?${qs}`, { token });
  },
  rawDataPengobatanOptions: (token: string) => request<PengobatanOptions>("/api/report-klinik/raw-data-pengobatan/options", { token }),
  penjualanObat: (token: string, params: PenjualanObatQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: PenjualanObat[]; total: number; unfilteredTotal: number }>(`/api/report-klinik/penjualan-obat?${qs}`, { token });
  },
  penjualanObatOptions: (token: string) => request<PenjualanObatOptions>("/api/report-klinik/penjualan-obat/options", { token }),
  penjualanTindakan: (token: string, params: PenjualanTindakanQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: PenjualanTindakan[]; total: number; unfilteredTotal: number }>(`/api/report-klinik/penjualan-tindakan?${qs}`, { token });
  },
  penjualanPenunjang: (token: string, params: PenjualanPenunjangQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: PenjualanPenunjang[]; total: number; unfilteredTotal: number }>(`/api/report-klinik/penjualan-penunjang?${qs}`, { token });
  },
  /** Same shape as Penjualan Obat's options: jenis (here refjenispanjang), branches, today. */
  penjualanPenunjangOptions: (token: string) => request<PenjualanObatOptions>("/api/report-klinik/penjualan-penunjang/options", { token }),
  penjualanOTC: (token: string, params: PenjualanOTCQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: PenjualanOTC[]; total: number; unfilteredTotal: number }>(`/api/report-klinik/penjualan-otc?${qs}`, { token });
  },
  penjualanOTCOptions: (token: string) => request<PenjualanOTCOptions>("/api/report-klinik/penjualan-otc/options", { token }),
  /** Penjualan Softdrink: Penjualan OTC for the drugs of type SOFT DRINK. */
  penjualanSoftdrink: (token: string, params: PenjualanOTCQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: PenjualanOTC[]; total: number; unfilteredTotal: number }>(`/api/report-klinik/penjualan-softdrink?${qs}`, { token });
  },
  penjualanSoftdrinkOptions: (token: string) => request<PenjualanOTCOptions>("/api/report-klinik/penjualan-softdrink/options", { token }),
  pasienBaru: (token: string, params: PasienBaruQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: PasienBaru[]; total: number; unfilteredTotal: number }>(`/api/report-klinik/pasien-baru?${qs}`, { token });
  },
  pasienBaruOptions: (token: string) => request<PasienBaruOptions>("/api/report-klinik/pasien-baru/options", { token }),
  rekapAsuransi: (token: string, params: RekapAsuransiQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: RekapAsuransi[]; total: number; unfilteredTotal: number }>(`/api/report-klinik/rekap-asuransi?${qs}`, { token });
  },
  rekapAsuransiOptions: (token: string) => request<RekapAsuransiOptions>("/api/report-klinik/rekap-asuransi/options", { token }),
  transaksiHarian: (token: string, params: TransaksiHarianQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: TransaksiHarian[]; total: number; unfilteredTotal: number }>(`/api/report-klinik/transaksi-harian?${qs}`, { token });
  },
  transaksiHarianOptions: (token: string) => request<TransaksiHarianOptions>("/api/report-klinik/transaksi-harian/options", { token }),
  allStok: (token: string, params: AllStokQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: AllStok[]; total: number; unfilteredTotal: number }>(`/api/report-klinik/all-stok?${qs}`, { token });
  },
  allStokOptions: (token: string) => request<AllStokOptions>("/api/report-klinik/all-stok/options", { token }),
  stokOpname: (token: string, params: StokOpnameQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: StokOpname[]; total: number; unfilteredTotal: number }>(`/api/stok-farmasi/stok-opname?${qs}`, { token });
  },
  stokOpnameOptions: (token: string) => request<StokOpnameOptions>("/api/stok-farmasi/stok-opname/options", { token }),
  minimStok: (token: string, params: StokOpnameQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: StokOpname[]; total: number; unfilteredTotal: number }>(`/api/stok-farmasi/stok-alert/minim-stok?${qs}`, { token });
  },
  stokAlertOptions: (token: string) => request<StokOpnameOptions>("/api/stok-farmasi/stok-alert/options", { token }),
  stokTrace: (token: string, params: StokTraceQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: StokTrace[]; total: number; unfilteredTotal: number; from: string; to: string }>(`/api/stok-farmasi/stok-trace?${qs}`, { token });
  },
  stokTraceObat: (token: string, cabang: string) =>
    request<{ rows: StokAdjusObat[] }>(`/api/stok-farmasi/stok-trace/obat?cabang=${encodeURIComponent(cabang)}`, { token }),
  rujukan: (token: string, params: RujukanQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: Rujukan[]; total: number; unfilteredTotal: number }>(`/api/report-klinik/rujukan?${qs}`, { token });
  },
  rujukanOptions: (token: string) =>
    request<{ branches: { kode: string; nama: string }[]; today: string }>("/api/report-klinik/rujukan/options", { token }),
  reportPendaftaran: (token: string, params: ReportPendaftaranQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: ReportPendaftaran[]; total: number; unfilteredTotal: number }>(`/api/report-klinik/pendaftaran?${qs}`, { token });
  },
  reportPendaftaranOptions: (token: string) => request<ReportPendaftaranOptions>("/api/report-klinik/pendaftaran/options", { token }),
  reportPendapatan: (token: string, params: { cabang: string; from: string; to: string; pelayanan?: number; dokter?: number }) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<ReportPendapatan>(`/api/report-klinik/pendapatan?${qs}`, { token });
  },
  reportPendapatanOptions: (token: string) => request<ReportPendapatanOptions>("/api/report-klinik/pendapatan/options", { token }),
  reportPendapatanDokter: (token: string, cabang: string) =>
    request<{ rows: { id: number; name: string }[] }>(`/api/report-klinik/pendapatan/dokter?cabang=${encodeURIComponent(cabang)}`, { token }),
  reportDiagnosa: (token: string, params: ReportDiagnosaQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: ReportDiagnosa[]; total: number; unfilteredTotal: number }>(`/api/report-klinik/diagnosa?${qs}`, { token });
  },
  /** Same shape as Report Pendapatan's options: services, branches, today. */
  reportDiagnosaOptions: (token: string) => request<ReportPendapatanOptions>("/api/report-klinik/diagnosa/options", { token }),
  reportDinkes: (token: string, params: ReportDinkesQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: ReportDinkes[]; total: number; unfilteredTotal: number }>(`/api/report-klinik/icd-dinkes?${qs}`, { token });
  },
  reportDinkesOptions: (token: string) =>
    request<{ branches: { kode: string; nama: string }[]; today: string }>("/api/report-klinik/icd-dinkes/options", { token }),
  penerimaanBarang: (token: string, params: PenerimaanBarangQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: PenerimaanBarang[]; total: number; unfilteredTotal: number }>(`/api/penerimaan-barang?${qs}`, { token });
  },
  penerimaanBarangOptions: (token: string) => request<PenerimaanBarangOptions>("/api/penerimaan-barang/options", { token }),
  penerimaanDistributor: (token: string) => request<{ rows: PenerimaanDistributor[] }>("/api/penerimaan-barang/distributor", { token }),
  penerimaanObat: (token: string, cabang: string) =>
    request<{ rows: PenerimaanObat[] }>(`/api/penerimaan-barang/obat?cabang=${encodeURIComponent(cabang)}`, { token }),
  penerimaanDetail: (token: string, id: number) => request<PenerimaanDetail>(`/api/penerimaan-barang/detail/${id}`, { token }),
  savePenerimaan: (token: string, input: PenerimaanSaveInput) =>
    request<PenerimaanSaveResult>("/api/penerimaan-barang", { method: "POST", body: input, token }),
  stokAdjus: (token: string, params: StokAdjusQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: StokAdjus[]; total: number; unfilteredTotal: number; from: string; to: string }>(`/api/stok-farmasi/stok-adjusment?${qs}`, { token });
  },
  stokAdjusOptions: (token: string) => request<StokAdjusOptions>("/api/stok-farmasi/stok-adjusment/options", { token }),
  stokAdjusObat: (token: string, cabang: string) =>
    request<{ rows: StokAdjusObat[] }>(`/api/stok-farmasi/stok-adjusment/obat?cabang=${encodeURIComponent(cabang)}`, { token }),
  saveStokAdjus: (token: string, input: StokAdjusSaveInput) =>
    request<{ id: number; stok: number }>("/api/stok-farmasi/stok-adjusment", { method: "POST", body: input, token }),
  penjualanTindakanOptions: (token: string) =>
    request<{ branches: { kode: string; nama: string }[]; today: string }>("/api/report-klinik/penjualan-tindakan/options", { token }),
  pengobatanDokter: (token: string, cabang: string) =>
    request<{ rows: PengobatanDokter[] }>(`/api/pengobatan/dokter?cabang=${encodeURIComponent(cabang)}`, { token }),
  pengobatanTindakan: (token: string, cabang: string, dokter: number) =>
    request<{ rows: PengobatanTindakan[] }>(`/api/pengobatan/tindakan?cabang=${encodeURIComponent(cabang)}&dokter=${dokter}`, { token }),
  pengobatanPenunjang: (token: string, cabang: string) =>
    request<{ rows: PengobatanPenunjang[] }>(`/api/pengobatan/penunjang?cabang=${encodeURIComponent(cabang)}`, { token }),
  pengobatanIcd: (token: string, params: { q?: string; page: number; size: number; sort?: string; dir?: string }) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: PengobatanIcd[]; total: number; unfilteredTotal: number }>(`/api/pengobatan/icd?${qs}`, { token });
  },
  // base: /api/pengobatan (Pengobatan form) or /api/pemeriksaan-perawat (Pemeriksaan Perawat), each guarded by its own menu.
  getPemeriksaan: (token: string, idKun: number, base: PemeriksaanBase = "/api/pengobatan") =>
    request<{ pemeriksaan: PemeriksaanRecord | null }>(`${base}/pemeriksaan/${idKun}`, { token }),
  savePemeriksaan: (token: string, idKun: number, input: PemeriksaanInput, base: PemeriksaanBase = "/api/pengobatan") =>
    request<{ pemeriksaan: PemeriksaanRecord | null }>(`${base}/pemeriksaan/${idKun}`, { method: "PUT", body: input, token }),
  // MCU > Pendaftaran.
  mcuDaftarOptions: (token: string) => request<McuDaftarOptions>("/api/mcu-pendaftaran/options", { token }),
  mcuDaftarList: (token: string, params: McuDaftarQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: McuDaftarRow[]; total: number; unfilteredTotal: number }>(`/api/mcu-pendaftaran?${qs}`, { token });
  },
  mcuDaftarPasien: (token: string, params: { q?: string; page: number; size: number }) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: McuDaftarPasien[]; total: number }>(`/api/mcu-pendaftaran/pasien?${qs}`, { token });
  },
  mcuDaftarLab: (token: string) => request<{ rows: McuDaftarLab[] }>("/api/mcu-pendaftaran/lab", { token }),
  mcuDaftarPaket: (token: string) => request<{ rows: McuDaftarPaket[] }>("/api/mcu-pendaftaran/paket", { token }),
  mcuDaftarNonLab: (token: string) => request<{ rows: McuDaftarNonLab[] }>("/api/mcu-pendaftaran/non-lab", { token }),
  mcuDaftarDokter: (token: string) => request<{ rows: McuDaftarDokter[] }>("/api/mcu-pendaftaran/dokter", { token }),
  mcuDaftarGet: (token: string, id: number) => request<McuDaftarDetail>(`/api/mcu-pendaftaran/${id}`, { token }),
  mcuDaftarCreate: (token: string, input: McuDaftarInput) =>
    request<{ ids: number[] }>("/api/mcu-pendaftaran", { method: "POST", body: input, token }),
  mcuDaftarUpdate: (token: string, id: number, input: McuDaftarInput) =>
    request<McuDaftarDetail>(`/api/mcu-pendaftaran/${id}`, { method: "PUT", body: input, token }),
  mcuDaftarCancel: (token: string, id: number, reason: string) =>
    request<void>(`/api/mcu-pendaftaran/${id}/cancel`, { method: "POST", body: { reason }, token }),
  // Transaksi Klinik > Pemeriksaan Perawat.
  perawatOptions: (token: string) => request<PerawatOptions>("/api/pemeriksaan-perawat/options", { token }),
  perawatList: (token: string, params: PerawatQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: PerawatRow[]; total: number; unfilteredTotal: number }>(`/api/pemeriksaan-perawat?${qs}`, { token });
  },
  perawatPendaftaran: (token: string, idKun: number) =>
    request<{ pendaftaran: PengobatanPendaftaran; editable: boolean }>(`/api/pemeriksaan-perawat/pendaftaran/${idKun}`, { token }),
  perawatDokter: (token: string, cabang: string) =>
    request<{ rows: PengobatanDokter[] }>(`/api/pemeriksaan-perawat/dokter?cabang=${encodeURIComponent(cabang)}`, { token }),
  // Transaksi Klinik > Pemeriksaan Dokter.
  dokterPeriksaOptions: (token: string) => request<DokterPeriksaOptions>("/api/pemeriksaan-dokter/options", { token }),
  dokterPeriksaList: (token: string, params: PerawatQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: PerawatRow[]; total: number; unfilteredTotal: number }>(`/api/pemeriksaan-dokter?${qs}`, { token });
  },
  dokterPeriksaPendaftaran: (token: string, idKun: number, base: PemeriksaanBase = "/api/pemeriksaan-dokter") =>
    request<{ pendaftaran: PengobatanPendaftaran; editable: boolean }>(`${base}/pendaftaran/${idKun}`, { token }),
  dokterPeriksaDokter: (token: string, cabang: string) =>
    request<{ rows: PengobatanDokter[] }>(`/api/pemeriksaan-dokter/dokter?cabang=${encodeURIComponent(cabang)}`, { token }),
  dokterPeriksaObat: (token: string, cabang: string) =>
    request<{ rows: PengobatanObat[] }>(`/api/pemeriksaan-dokter/obat?cabang=${encodeURIComponent(cabang)}`, { token }),
  dokterPeriksaIcd: (token: string, params: { q?: string; page: number; size: number; sort?: string; dir?: string }) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: PengobatanIcd[]; total: number; unfilteredTotal: number }>(`/api/pemeriksaan-dokter/icd?${qs}`, { token });
  },
  getResep: (token: string, idKun: number, base: PemeriksaanBase = "/api/pemeriksaan-dokter") => request<Resep>(`${base}/resep/${idKun}`, { token }),
  // Transaksi Klinik > Proses Obat (view only).
  prosesObatOptions: (token: string) => request<PerawatOptions>("/api/proses-obat/options", { token }),
  prosesObatList: (token: string, params: PerawatQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: PerawatRow[]; total: number; unfilteredTotal: number }>(`/api/proses-obat?${qs}`, { token });
  },
  saveResep: (token: string, idKun: number, input: ResepInput) =>
    request<Resep>(`/api/pemeriksaan-dokter/resep/${idKun}`, { method: "PUT", body: input, token }),
  savePengobatan: (token: string, input: PengobatanSaveInput) =>
    request<PengobatanSaveResult>("/api/pengobatan", { method: "POST", body: input, token }),
  kuitansi: (token: string, id: number) => request<Kuitansi>(`/api/pengobatan/kuitansi/${id}`, { token }),
  cetakKuitansi: (token: string, id: number) => request<Kuitansi>(`/api/pengobatan/kuitansi/${id}/cetak`, { method: "POST", token }),
  nota: (token: string, id: number) => request<Nota>(`/api/pengobatan/nota/${id}`, { token }),
  pengobatanDetail: (token: string, id: number) => request<PengobatanDetail>(`/api/pengobatan/detail/${id}`, { token }),
  cancelPengobatan: (token: string, id: number, alasan: string) =>
    request<PengobatanDetail>(`/api/pengobatan/detail/${id}/cancel`, { method: "POST", body: { alasan }, token }),
  pengobatanObat: (token: string, cabang: string) =>
    request<{ rows: PengobatanObat[] }>(`/api/pengobatan/obat?cabang=${encodeURIComponent(cabang)}`, { token }),
  pengobatanPendaftaran: (token: string, params: PengobatanPendaftaranQuery) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: PengobatanPendaftaran[]; total: number; unfilteredTotal: number }>(`/api/pengobatan/pendaftaran?${qs}`, { token });
  },

  // Front Office Klinik > Pendaftaran Pasien (server-side paging)
  pendaftaran: (
    token: string,
    params: { from: string; to: string; page: number; size: number; q?: string; sort?: string; dir?: string; cabang?: string; pelayanan?: number; status?: number },
  ) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: Pendaftaran[]; total: number; unfilteredTotal: number }>(`/api/pendaftaran?${qs}`, { token });
  },
  pendaftaranOptions: (token: string) => request<PendaftaranOptions>("/api/pendaftaran/options", { token }),
  pendaftaranPasien: (token: string, params: { q?: string; by?: PasienSearchField | ""; key?: string; page: number; size: number }) => {
    const qs = new URLSearchParams({ page: String(params.page), size: String(params.size), q: params.q ?? "", by: params.by ?? "", key: params.key ?? "" });
    return request<{ rows: PendaftaranPasien[]; total: number; unfilteredTotal: number }>(`/api/pendaftaran/pasien?${qs}`, { token });
  },
  createPendaftaran: (token: string, input: { tanggal: string; idPas: number; idJenis: number; cabang: string; pakaiKartu: boolean }) =>
    request<Pendaftaran>("/api/pendaftaran", { method: "POST", body: input, token }),
  updatePendaftaran: (token: string, id: number, input: { idJenis: number; pakaiKartu: boolean }) =>
    request<Pendaftaran>(`/api/pendaftaran/${id}`, { method: "PUT", body: input, token }),
  cancelPendaftaran: (token: string, id: number, reason: string) =>
    request<Pendaftaran>(`/api/pendaftaran/${id}/cancel`, { method: "POST", body: { reason }, token }),

  // Management > Cabang
  branches: (token: string) => request<{ branches: Branch[] }>("/api/branches", { token }),
  createBranch: (token: string, input: BranchInput) =>
    request<Branch>("/api/branches", { method: "POST", body: input, token }),
  updateBranch: (token: string, kode: string, input: BranchInput) =>
    request<Branch>(`/api/branches/${encodeURIComponent(kode)}`, { method: "PUT", body: input, token }),
  // Stok Farmasi > Analitik
  analitikOptions: (token: string) => request<AnalitikOptions>("/api/analitik/options", { token }),
  analitikObat: (token: string, params: { cabang?: string; horizon?: number; leadTime?: number; q?: string; status?: string }) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<AnalitikObatResult>(`/api/analitik/obat?${qs}`, { token });
  },
  analitikPenyakit: (token: string, params: { cabang?: string; weeks?: number; top?: number }) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<AnalitikPenyakit>(`/api/analitik/penyakit?${qs}`, { token });
  },
  analitikDiagnosaObat: (token: string, params: { cabang?: string; icd?: string; horizon?: number }) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<AnalitikDiagnosaObatResult>(`/api/analitik/diagnosa-obat?${qs}`, { token });
  },
  analitikRingkasan: (token: string, input: { cabang?: string; refresh?: boolean }) =>
    request<{ text: string; cached: boolean; today: string }>("/api/analitik/ringkasan", { method: "POST", body: input, token }),
  analitikChat: (token: string, input: { cabang?: string; messages: AnalitikChatMessage[] }) =>
    request<{ message: AnalitikChatMessage }>("/api/analitik/chat", { method: "POST", body: input, token }),
  analitikCommand: (token: string, cabang?: string) =>
    request<AnalitikCommand>(`/api/analitik/command${cabang ? `?cabang=${encodeURIComponent(cabang)}` : ""}`, { token }),
  analitikKinerja: (token: string, params: { cabang?: string; dari?: string; sampai?: string; kelompok?: string }) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: AnalitikKinerja[]; total: AnalitikKinerja; dari: string; sampai: string; kelompok: string }>(`/api/analitik/kinerja?${qs}`, { token });
  },
  analitikKualitas: (token: string, cabang?: string) =>
    request<{ checks: AnalitikCekKualitas[]; maxContoh: number }>(`/api/analitik/kualitas-data${cabang ? `?cabang=${encodeURIComponent(cabang)}` : ""}`, { token }),
  analitikFollowUp: (token: string, params: { cabang?: string; hari?: number; kelompok?: string }) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: AnalitikFollowUp[]; total: number; hari: number; kelompok: { kode: string; nama: string }[] }>(`/api/analitik/follow-up?${qs}`, { token });
  },
  saranIcd: (token: string, deskripsi: string) =>
    request<{ ai: boolean; aiError: string; kandidat: IcdKandidat[] }>("/api/analitik/saran-icd", { method: "POST", body: { deskripsi }, token }),

  // Front Office Klinik > Riwayat Pasien
  riwayatPasienSearch: (token: string, q: string, page = 1, size = 10) =>
    request<{ rows: RiwayatPasienHit[]; total: number }>(`/api/riwayat-pasien/pasien?q=${encodeURIComponent(q)}&page=${page}&size=${size}`, { token }),
  riwayatPasien: (token: string, id: number) => request<RiwayatPasien>(`/api/riwayat-pasien/${id}`, { token }),
  // MCU result forms
  mcuRegistrasi: (token: string, form: string, params: { q?: string; from?: string; to?: string; page?: number; size?: number; cancel?: string }) => {
    const qs = new URLSearchParams();
    for (const [k, v] of Object.entries(params)) if (v !== undefined && v !== "") qs.set(k, String(v));
    return request<{ rows: McuRegistrasi[]; total: number; from: string; to: string; today: string }>(`/api/mcu/form/${form}/registrasi?${qs}`, { token });
  },
  mcuKatalog: (token: string, form: string) => request<{ form: string; title: string; sections: McuSection[] }>(`/api/mcu/form/${form}/katalog`, { token }),
  mcuHasil: (token: string, form: string, id: number) =>
    request<{ registrasi: McuRegistrasi; hasil: McuHasil; lab?: McuLabItem[] }>(`/api/mcu/form/${form}/${id}`, { token }),
  mcuSave: (token: string, form: string, id: number, input: { status: string; kesimpulan: string; values: Record<string, McuValue> }) =>
    request<{ hasil: McuHasil }>(`/api/mcu/form/${form}/${id}`, { method: "PUT", body: input, token }),
};
