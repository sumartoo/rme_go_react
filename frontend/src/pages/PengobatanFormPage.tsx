import { ReactNode, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Check, ChevronDown, ChevronUp, FilePlus2, HeartPulse, Info, Pencil, Plus, Printer, Receipt, Save, Search, Trash2, X } from "lucide-react";
import {
  api,
  ApiError,
  PengobatanDetail,
  PengobatanIcd,
  PengobatanObat,
  PengobatanOptions,
  PengobatanPendaftaran,
  PengobatanSaveResult,
  PengobatanTindakan,
} from "../api";
import DiagnosaLookup from "./DiagnosaLookup";
import PemeriksaanTab from "./PemeriksaanTab";
import KuitansiDialog from "./KuitansiDialog";
import { NOTA_FORMATS, NotaFormat, printNota } from "./notaPdf";
import { openPrintTab } from "../components/printPdf";
import { useAuth } from "../auth/AuthContext";
import { useModal } from "../components/FormKit";
import { age, fmtDate } from "../components/dates";
import DokterLookup from "./DokterLookup";
import ObatLookup from "./ObatLookup";
import { jumlah, ObatLine, obatTotals, ppn, TindakanLine, tindakanJumlah, tindakanTotal } from "./pengobatanCalc";
import TindakanLookup, { parseAmount } from "./TindakanLookup";
import PendaftaranLookup from "./PendaftaranLookup";

/**
 * Transaksi Klinik > Pengobatan > Add and Edit: the treatment form, laid out
 * like the legacy PengobatanDetail/New and /Edit/{id} screens. Edit shows a
 * saved sale read-only, with Kuitansi and Print.
 */

const LIST_PATH = "/transaksi-klinik/pengobatan";
const money = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });

type Tab = "obat" | "tindakan" | "penunjang" | "diagnosa" | "pemeriksaan";
const TABS: { key: Tab; label: string }[] = [
  { key: "obat", label: "Obat" },
  { key: "tindakan", label: "Tindakan" },
  { key: "penunjang", label: "Penunjang" },
  { key: "diagnosa", label: "Diagnosa" },
  { key: "pemeriksaan", label: "Pemeriksaan" },
];

/** Card with a coloured header that can be collapsed, like the legacy panels. */
export function Card({ title, children, className, locked = false }: { title: string; children: ReactNode; className?: string; locked?: boolean }) {
  const [open, setOpen] = useState(true);
  return (
    <section className={["tx-card", className].filter(Boolean).join(" ")}>
      <header className="tx-card-head">
        <h2>
          <HeartPulse aria-hidden="true" /> {title}
        </h2>
        <button type="button" className="tx-card-tool" aria-expanded={open} aria-label={open ? `Collapse ${title}` : `Expand ${title}`} onClick={() => setOpen((o) => !o)}>
          {open ? <ChevronUp aria-hidden="true" /> : <ChevronDown aria-hidden="true" />}
        </button>
      </header>
      {open && (
        <div className="tx-card-body">
          {/* A saved sale: the controls are read-only, the card can still collapse. */}
          <fieldset className="tx-fieldset" disabled={locked}>
            {children}
          </fieldset>
        </div>
      )}
    </section>
  );
}

/** Label on the left, control(s) on the right. */
export function Row({ id, label, required, children }: { id?: string; label: string; required?: boolean; children: ReactNode }) {
  return (
    <div className="tx-row">
      <label htmlFor={id}>
        {label}
        {required && <span className="req"> *</span>}
      </label>
      <div className="tx-ctrl">{children}</div>
    </div>
  );
}

/** Read-only input with a search button that opens a lookup. */
export function Lookup({ id, placeholder, value = "", onSearch }: { id: string; placeholder: string; value?: string; onSearch: () => void }) {
  return (
    <div className="tx-lookup">
      <input id={id} readOnly placeholder={placeholder} value={value} />
      <button type="button" className="tx-lookup-btn" aria-label={`Cari ${placeholder}`} title="Cari" onClick={onSearch}>
        <Search aria-hidden="true" />
      </button>
    </div>
  );
}

const num = (s: string) => (Number.isFinite(Number(s)) ? Number(s) : 0);

/**
 * The "Pembayaran" dialog shown by Save: Jumlah (the bill), Diskon and Payment.
 * Diskon applies the discount to the form; Bayar saves the sale. A Payment of 0
 * or empty means the patient pays exactly the amount due, as the legacy data does.
 */
function PembayaranDialog({
  jumlah,
  diskon: initialDiskon,
  busy,
  error,
  onApplyDiskon,
  onBayar,
  onClose,
}: {
  jumlah: number;
  diskon: number;
  busy: boolean;
  error: string;
  onApplyDiskon: (diskon: number) => void;
  onBayar: (diskon: number, payment: number | null) => void;
  onClose: () => void;
}) {
  const dialogRef = useModal();
  const [diskon, setDiskon] = useState(initialDiskon ? String(initialDiskon) : "0");
  const [payment, setPayment] = useState("0");
  const [msg, setMsg] = useState("");
  const d = Number(diskon.replace(",", "."));
  const p = Number(payment.replace(",", "."));
  const validDiskon = Number.isFinite(d) && d >= 0 && d <= jumlah;
  const validPayment = Number.isFinite(p) && p >= 0;
  const grand = validDiskon ? jumlah - d : jumlah;
  const paid = validPayment && p > 0 ? p : grand;

  const check = () => {
    if (!validDiskon) return setMsg(`Diskon harus antara 0 dan ${plain.format(jumlah)}.`), false;
    if (!validPayment) return setMsg("Payment tidak valid."), false;
    setMsg("");
    return true;
  };
  const bayar = () => check() && onBayar(d, p > 0 ? p : null);

  return (
    <dialog ref={dialogRef} className="modal pay-dialog" aria-labelledby="pay-title" onClose={onClose}>
      <form
        onSubmit={(e) => {
          e.preventDefault();
          bayar();
        }}
      >
        <header className="modal-head">
          <h2 id="pay-title">Pembayaran</h2>
          <button type="button" className="row-action" aria-label="Close" onClick={onClose}>
            <X aria-hidden="true" />
          </button>
        </header>
        <div className="modal-body pay-body">
          <label htmlFor="pay-jumlah">Jumlah</label>
          <input id="pay-jumlah" readOnly className="num" value={money2.format(jumlah)} />
          <label htmlFor="pay-diskon">Diskon</label>
          <input
            id="pay-diskon"
            type="number"
            min={0}
            max={jumlah}
            step="any"
            inputMode="decimal"
            className="num"
            aria-invalid={validDiskon ? undefined : "true"}
            value={diskon}
            onFocus={(e) => e.target.select()}
            onChange={(e) => setDiskon(e.target.value)}
          />
          <label htmlFor="pay-payment">Payment</label>
          <input
            id="pay-payment"
            type="number"
            min={0}
            step="any"
            inputMode="decimal"
            className="num"
            autoFocus
            aria-invalid={validPayment ? undefined : "true"}
            aria-describedby="pay-hint"
            value={payment}
            onFocus={(e) => e.target.select()}
            onChange={(e) => setPayment(e.target.value)}
          />
          <p id="pay-hint" className="pay-summary" aria-live="polite">
            Grand Total <strong>{money2.format(grand)}</strong>
            {paid > grand && (
              <>
                {" "}
                · Kembalian <strong>{money2.format(paid - grand)}</strong>
              </>
            )}
            {paid < grand && (
              <>
                {" "}
                · Sisa (piutang) <strong>{money2.format(grand - paid)}</strong>
              </>
            )}
            {!(p > 0) && <span className="muted"> · Payment 0 = dibayar pas</span>}
          </p>
          {(msg || error) && (
            <p className="alert error pay-error" role="alert">
              {msg || error}
            </p>
          )}
        </div>
        <footer className="modal-foot">
          <button type="button" className="btn-small" onClick={onClose}>
            Close
          </button>
          <button type="button" className="btn-add" onClick={() => check() && onApplyDiskon(d)} title="Terapkan diskon ke form">
            Diskon
          </button>
          <button type="submit" className="btn-add" disabled={busy}>
            {busy ? "Menyimpan…" : "Bayar $"}
          </button>
        </footer>
      </form>
    </dialog>
  );
}
const plain = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });
const money2 = new Intl.NumberFormat("id-ID", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

/**
 * The Tindakan and Penunjang tabs' list: Banyak (and Harga when editable) can be
 * changed in place, a line can be removed. Penunjang lines have no doctor column.
 */
function TindakanTable({
  lines,
  showDokter = true,
  empty = "Belum ada tindakan.",
  onChange,
  onRemove,
  onError,
}: {
  lines: TindakanLine[];
  showDokter?: boolean;
  empty?: string;
  onChange: (idTindakan: number, harga: number, byk: number) => void;
  onRemove: (idTindakan: number) => void;
  onError: (msg: string) => void;
}) {
  const [editing, setEditing] = useState<{ id: number; harga: string; byk: string } | null>(null);
  const save = (l: TindakanLine) => {
    if (!editing) return;
    const byk = Number(editing.byk.trim().replace(",", "."));
    if (!Number.isFinite(byk) || byk <= 0) return onError(`Banyak untuk ${l.nama} harus lebih dari 0.`);
    const harga = l.isEdit ? parseAmount(editing.harga) : l.harga;
    if (!Number.isFinite(harga) || harga < 0) return onError(`Harga untuk ${l.nama} tidak valid.`);
    onChange(l.idTindakan, harga, byk);
    setEditing(null);
  };
  const cols = showDokter
    ? ["ID", "Tindakan", "Jenis", "Dokter", "Harga", "Banyak", "Jumlah", "Action"]
    : ["ID", "Penunjang", "Jenis", "Harga", "Banyak", "Jumlah", "Action"];
  return (
    <div className="table-wrap">
      <table className="data-table tx-items">
        <thead>
          <tr>
            {cols.map((c) => (
              <th key={c} className={["Harga", "Banyak", "Jumlah"].includes(c) ? "dt-num" : undefined}>
                {c}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {lines.length === 0 ? (
            <tr>
              <td colSpan={cols.length} className="dt-empty">
                {empty}
              </td>
            </tr>
          ) : (
            lines.map((l) => {
              const edit = editing?.id === l.idTindakan ? editing : null;
              return (
                <tr key={l.idTindakan}>
                  <td>{l.idTindakan}</td>
                  <td>{l.nama}</td>
                  <td>{l.jenis || "-"}</td>
                  {showDokter && <td>{l.dokter}</td>}
                  <td className="dt-num">
                    {edit && l.isEdit ? (
                      <input
                        className="price-input"
                        inputMode="decimal"
                        aria-label={`Harga ${l.nama}`}
                        value={edit.harga}
                        onChange={(e) => setEditing({ ...edit, harga: e.target.value })}
                        onKeyDown={(e) => e.key === "Enter" && save(l)}
                      />
                    ) : (
                      plain.format(l.harga)
                    )}
                  </td>
                  <td className="dt-num">
                    {edit ? (
                      <input
                        className="qty-input"
                        type="number"
                        min={0}
                        step="any"
                        aria-label={`Banyak ${l.nama}`}
                        value={edit.byk}
                        onChange={(e) => setEditing({ ...edit, byk: e.target.value })}
                        onKeyDown={(e) => e.key === "Enter" && save(l)}
                      />
                    ) : (
                      plain.format(l.byk)
                    )}
                  </td>
                  <td className="dt-num">{plain.format(tindakanJumlah(l))}</td>
                  <td className="tx-actions">
                    {edit ? (
                      <>
                        <button type="button" className="row-action" aria-label={`Simpan ${l.nama}`} title="Simpan" onClick={() => save(l)}>
                          <Check aria-hidden="true" />
                        </button>
                        <button type="button" className="row-action" aria-label={`Batal ubah ${l.nama}`} title="Batal" onClick={() => setEditing(null)}>
                          <X aria-hidden="true" />
                        </button>
                      </>
                    ) : (
                      <>
                        <button type="button" className="row-action danger" aria-label={`Hapus ${l.nama}`} title="Hapus" onClick={() => onRemove(l.idTindakan)}>
                          <Trash2 aria-hidden="true" />
                        </button>
                        <button
                          type="button"
                          className="row-action"
                          aria-label={`Ubah ${l.nama}`}
                          title={l.isEdit ? "Ubah harga / banyak" : "Ubah banyak"}
                          onClick={() => setEditing({ id: l.idTindakan, harga: plain.format(l.harga), byk: String(l.byk) })}
                        >
                          <Pencil aria-hidden="true" />
                        </button>
                      </>
                    )}
                  </td>
                </tr>
              );
            })
          )}
        </tbody>
      </table>
    </div>
  );
}

/** The Obat tab's list: Banyak and Dosis can be edited in place, a line can be removed. */
function ObatTable({
  lines,
  onChange,
  onRemove,
  onError,
}: {
  lines: ObatLine[];
  onChange: (idObat: number, byk: number, dosis: string) => void;
  onRemove: (idObat: number) => void;
  onError: (msg: string) => void;
}) {
  const [editing, setEditing] = useState<{ idObat: number; byk: string; dosis: string } | null>(null);
  const save = (l: ObatLine) => {
    if (!editing) return;
    const byk = Number(editing.byk.trim().replace(",", "."));
    if (!Number.isFinite(byk) || byk <= 0) return onError(`Banyak untuk ${l.nama} harus lebih dari 0.`);
    if (byk > l.stok) return onError(`Stok ${l.nama} tidak cukup: stok ${plain.format(l.stok)}, diminta ${plain.format(byk)}.`);
    onChange(l.idObat, byk, editing.dosis.trim());
    setEditing(null);
  };
  const cols = ["ID", "Obat", "Jenis", "Satuan", "Harga", "Banyak", "Jumlah", "PPN", "Dosis", "Action"];
  return (
    <div className="table-wrap">
      <table className="data-table tx-items">
        <thead>
          <tr>
            {cols.map((c) => (
              <th key={c} className={["Harga", "Banyak", "Jumlah", "PPN"].includes(c) ? "dt-num" : undefined}>
                {c}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {lines.length === 0 ? (
            <tr>
              <td colSpan={cols.length} className="dt-empty">
                Belum ada obat.
              </td>
            </tr>
          ) : (
            lines.map((l) => {
              const edit = editing?.idObat === l.idObat ? editing : null;
              return (
                <tr key={l.idObat}>
                  <td>{l.idObat}</td>
                  <td>{l.nama}</td>
                  <td>{l.jenis || "-"}</td>
                  <td>{l.satuan || "-"}</td>
                  <td className="dt-num">{plain.format(l.harga)}</td>
                  <td className="dt-num">
                    {edit ? (
                      <input
                        className="qty-input"
                        type="number"
                        min={0}
                        step="any"
                        aria-label={`Banyak ${l.nama}`}
                        value={edit.byk}
                        onChange={(e) => setEditing({ ...edit, byk: e.target.value })}
                        onKeyDown={(e) => e.key === "Enter" && save(l)}
                      />
                    ) : (
                      plain.format(l.byk)
                    )}
                  </td>
                  <td className="dt-num">{plain.format(jumlah(l))}</td>
                  <td className="dt-num">{plain.format(ppn(l))}</td>
                  <td>
                    {edit ? (
                      <input
                        className="dosis-input"
                        aria-label={`Dosis ${l.nama}`}
                        placeholder="3 x 1"
                        maxLength={100}
                        value={edit.dosis}
                        onChange={(e) => setEditing({ ...edit, dosis: e.target.value })}
                        onKeyDown={(e) => e.key === "Enter" && save(l)}
                      />
                    ) : (
                      l.dosis
                    )}
                  </td>
                  <td className="tx-actions">
                    {edit ? (
                      <>
                        <button type="button" className="row-action" aria-label={`Simpan ${l.nama}`} title="Simpan" onClick={() => save(l)}>
                          <Check aria-hidden="true" />
                        </button>
                        <button type="button" className="row-action" aria-label={`Batal ubah ${l.nama}`} title="Batal" onClick={() => setEditing(null)}>
                          <X aria-hidden="true" />
                        </button>
                      </>
                    ) : (
                      <>
                        <button type="button" className="row-action danger" aria-label={`Hapus ${l.nama}`} title="Hapus" onClick={() => onRemove(l.idObat)}>
                          <Trash2 aria-hidden="true" />
                        </button>
                        <button
                          type="button"
                          className="row-action"
                          aria-label={`Ubah ${l.nama}`}
                          title="Ubah banyak / dosis"
                          onClick={() => setEditing({ idObat: l.idObat, byk: String(l.byk), dosis: l.dosis })}
                        >
                          <Pencil aria-hidden="true" />
                        </button>
                      </>
                    )}
                  </td>
                </tr>
              );
            })
          )}
        </tbody>
      </table>
    </div>
  );
}

export default function PengobatanFormPage() {
  const { token, logout } = useAuth();
  const navigate = useNavigate();
  // Edit page: /transaksi-klinik/pengobatan/edit/:id.
  const { id } = useParams();
  const saleId = id ? Number(id) : null;
  const [options, setOptions] = useState<PengobatanOptions | null>(null);
  const [detail, setDetail] = useState<PengobatanDetail | null>(null);
  const [loadError, setLoadError] = useState("");
  const [formKey, setFormKey] = useState(0);

  useEffect(() => {
    if (!token) return;
    setLoadError("");
    setDetail(null);
    Promise.all([api.pengobatanOptions(token), saleId ? api.pengobatanDetail(token, saleId) : Promise.resolve(null)])
      .then(([o, d]) => {
        setOptions(o);
        setDetail(d);
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
        else if (err instanceof ApiError && err.status === 404) setLoadError(`Transaksi ${id} tidak ditemukan atau bukan dari cabang Anda.`);
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, [saleId]); // eslint-disable-line react-hooks/exhaustive-deps

  const heading = saleId ? "Edit Pengobatan" : "Add Pengobatan";
  return (
    <section className="panel tx-page">
      {loadError ? (
        <>
          <h1>{heading}</h1>
          <p className="alert error" role="alert">
            {loadError}
          </p>
        </>
      ) : !options || (saleId && !detail) ? (
        <>
          <h1>{heading}</h1>
          <p className="muted">Loading…</p>
        </>
      ) : (
        <PengobatanForm
          key={saleId ? `edit-${saleId}` : formKey}
          options={options}
          detail={detail}
          onNew={() => (saleId ? navigate(`${LIST_PATH}/new`) : setFormKey((k) => k + 1))}
          onCancel={() => navigate(LIST_PATH)}
        />
      )}
    </section>
  );
}

/** The form's lines from a saved sale. */
function linesOf(d: PengobatanDetail) {
  return {
    obat: d.obat.map(
      (o): ObatLine => ({ idObat: o.idObat, nama: o.nama, jenis: o.jenis, satuan: o.satuan, harga: o.harga, byk: o.byk, kenaPajak: o.kenaPajak, stok: o.byk, dosis: o.dosis }),
    ),
    tindakan: d.tindakan.map(
      (t): TindakanLine => ({ idTindakan: t.id, nama: t.nama, jenis: t.jenis, dokter: t.dokter || d.dokter, harga: t.harga, isEdit: t.isEdit, byk: t.byk }),
    ),
    penunjang: d.penunjang.map((t): TindakanLine => ({ idTindakan: t.id, nama: t.nama, jenis: t.jenis, dokter: "", harga: t.harga, isEdit: false, byk: t.byk })),
  };
}

function PengobatanForm({
  options,
  detail,
  onNew,
  onCancel,
}: {
  options: PengobatanOptions;
  /** A saved sale (the Edit page): the form shows it read-only. */
  detail: PengobatanDetail | null;
  onNew: () => void;
  onCancel: () => void;
}) {
  const { token, logout } = useAuth();
  const isEdit = detail !== null;
  const initial = detail ? linesOf(detail) : null;
  const [tab, setTab] = useState<Tab>("obat");
  const [notice, setNotice] = useState("");
  const [payment, setPayment] = useState("");
  const [diskon, setDiskon] = useState(detail?.potongan ? String(detail.potongan) : "");
  const [tukarPoint, setTukarPoint] = useState("");
  const defaultPelayanan = String(options.pelayanan.find((o) => o.name === "POLI UMUM")?.id ?? options.pelayanan[0]?.id ?? "");
  const defaultBayar = options.pembayaran.find((o) => o.name === "TUNAI")?.id ?? options.pembayaran[0]?.id ?? "";
  const [cabang, setCabang] = useState(detail?.cabang ?? (options.branches.length === 1 ? options.branches[0].kode : ""));
  const [pelayanan, setPelayanan] = useState(detail?.idJenisRawat != null ? String(detail.idJenisRawat) : defaultPelayanan);
  const [bayar, setBayar] = useState(detail?.idJenisBayar != null ? String(detail.idJenisBayar) : String(defaultBayar));
  const [tglKwi, setTglKwi] = useState(detail?.tglKwi ?? options.today);
  const [reg, setReg] = useState<PengobatanPendaftaran | null>(detail?.reg ?? null);
  const [dokter, setDokter] = useState<{ id: number; nama: string } | null>(
    detail?.idDokter != null ? { id: detail.idDokter, nama: detail.dokter } : null,
  );
  const [lookupOpen, setLookupOpen] = useState(false);
  const [dokterOpen, setDokterOpen] = useState(false);
  const [obatOpen, setObatOpen] = useState(false);
  const [obat, setObat] = useState<ObatLine[]>(initial?.obat ?? []);
  const [tindakanOpen, setTindakanOpen] = useState(false);
  const [tindakan, setTindakan] = useState<TindakanLine[]>(initial?.tindakan ?? []);
  const [penunjangOpen, setPenunjangOpen] = useState(false);
  const [penunjang, setPenunjang] = useState<TindakanLine[]>(initial?.penunjang ?? []);
  // Diagnoses are ICD-X codes; they are not per branch and carry no price.
  const [diagnosaOpen, setDiagnosaOpen] = useState(false);
  const [diagnosa, setDiagnosa] = useState<PengobatanIcd[]>(detail?.diagnosa ?? []);
  const notYet = (what: string) => setNotice(`Pencarian ${what} belum tersedia di mockup ini.`);

  // Both lookups list the data of one branch.
  const needBranch = (what: string) => {
    if (cabang) return true;
    setNotice(`Pilih Cabang dulu: data ${what} ditampilkan per cabang.`);
    document.getElementById("tx-cabang")?.focus();
    return false;
  };
  const openLookup = () => {
    if (!needBranch("pendaftaran")) return;
    setNotice("");
    setLookupOpen(true);
  };
  const openDokter = () => {
    if (!needBranch("dokter")) return;
    setNotice("");
    setDokterOpen(true);
  };
  const openObat = () => {
    if (!needBranch("obat")) return;
    setNotice("");
    setObatOpen(true);
  };
  // Procedures are the Dokter PJ's, priced for the branch: both must be chosen first.
  const openTindakan = () => {
    if (!needBranch("tindakan")) return;
    if (!dokter) {
      setNotice("Pilih Dokter PJ dulu: daftar tindakan mengikuti dokter.");
      document.querySelector<HTMLButtonElement>('[aria-label="Cari Dokter"], [aria-label="Cari Belum di Pilih"]')?.focus();
      return;
    }
    setNotice("");
    setTindakanOpen(true);
  };
  // Picking a procedure already in the list adds to its quantity (the price picked last wins).
  const addTindakan = (t: PengobatanTindakan, harga: number, byk: number) => {
    setTindakan((list) =>
      list.some((l) => l.idTindakan === t.id)
        ? list.map((l) => (l.idTindakan === t.id ? { ...l, harga, byk: l.byk + byk } : l))
        : [...list, { idTindakan: t.id, nama: t.nama, jenis: t.jenis, dokter: dokter?.nama ?? "", harga, isEdit: t.isEdit, byk }],
    );
    setTindakanOpen(false);
  };
  // Supporting examinations depend on the branch only.
  const openPenunjang = () => {
    if (!needBranch("penunjang")) return;
    setNotice("");
    setPenunjangOpen(true);
  };
  const addPenunjang = (t: PengobatanTindakan, harga: number, byk: number) => {
    setPenunjang((list) =>
      list.some((l) => l.idTindakan === t.id)
        ? list.map((l) => (l.idTindakan === t.id ? { ...l, harga, byk: l.byk + byk } : l))
        : [...list, { idTindakan: t.id, nama: t.nama, jenis: t.jenis, dokter: "", harga, isEdit: false, byk }],
    );
    setPenunjangOpen(false);
  };
  // Every procedure line belongs to the sale's doctor: a new Dokter PJ empties the list.
  const changeDokter = (d: { id: number; nama: string } | null) => {
    if (tindakan.length > 0 && d?.id !== dokter?.id) {
      setTindakan([]);
      setNotice("Dokter PJ diganti: daftar tindakan dikosongkan (tindakan mengikuti dokter).");
    }
    setDokter(d);
  };
  // Picking a drug already in the list adds to its quantity.
  const addObat = (o: PengobatanObat, byk: number) => {
    setObat((list) =>
      list.some((l) => l.idObat === o.id)
        ? list.map((l) => (l.idObat === o.id ? { ...l, byk: l.byk + byk } : l))
        : [...list, { idObat: o.id, nama: o.nama, jenis: o.jenis, satuan: o.satuan, harga: o.harga, byk, kenaPajak: o.kenaPajak, stok: o.stok, dosis: "" }],
    );
    setObatOpen(false);
  };
  const pick = (r: PengobatanPendaftaran) => {
    setReg(r);
    if (r.idJenisRawat !== null) setPelayanan(String(r.idJenisRawat));
    setNotice("");
    changeDokter(r.idDokter !== null ? { id: r.idDokter, nama: r.dokter } : null);
    setLookupOpen(false);
  };
  const changeCabang = (kode: string) => {
    setCabang(kode);
    // A registration and its doctor belong to one branch: picking another branch clears them.
    if (reg && reg.cabang !== kode) {
      setReg(null);
      setDokter(null);
      setPelayanan(defaultPelayanan);
      setNotice("Cabang diganti: data pendaftaran, pasien dan Dokter PJ dikosongkan.");
    } else if (dokter && kode !== cabang) {
      setDokter(null);
      setNotice("Cabang diganti: Dokter PJ dikosongkan.");
    }
    // Drug and procedure prices (and drug stock) are per branch.
    if (kode !== cabang && (obat.length > 0 || tindakan.length > 0 || penunjang.length > 0)) {
      const what = [obat.length > 0 && "obat", tindakan.length > 0 && "tindakan", penunjang.length > 0 && "penunjang"]
        .filter(Boolean)
        .join(", ");
      setObat([]);
      setTindakan([]);
      setPenunjang([]);
      setNotice((n) => (n ? `${n} Daftar ${what} juga dikosongkan (harga per cabang).` : `Cabang diganti: daftar ${what} dikosongkan (harga per cabang).`));
    }
  };
  const umur = reg ? age(reg.tglLahir) : null;

  // Drugs (rounded) plus procedures and supporting examinations (not rounded).
  const totals = obatTotals(obat);
  const totalTindakan = tindakanTotal(tindakan);
  const totalPenunjang = tindakanTotal(penunjang);
  // A saved sale shows its stored total; a new one the total of the lines.
  const total = detail ? detail.total : totals.bill + totalTindakan + totalPenunjang;
  const grandTotal = Math.max(total - num(diskon) - num(tukarPoint), 0);
  // An empty Payment means the patient pays exactly the amount due (as the legacy data does).
  const paid = payment.trim() === "" ? grandTotal : num(payment);
  const sisa = Math.max(grandTotal - paid, 0);
  const kembalian = Math.max(paid - grandTotal, 0);

  // ---------- Save ----------
  const [saving, setSaving] = useState(false);
  const [saved, setSaved] = useState<PengobatanSaveResult | null>(
    detail
      ? {
          id: detail.id,
          noKwi: detail.noKwi,
          totObat: detail.totObat,
          totTindakan: detail.totTindakan,
          totPenunjang: detail.totPenunjang,
          total: detail.total,
          potongan: detail.potongan,
          grandTotal: detail.grandTotal,
          pembayaran: detail.pembayaran,
          cicilan: Math.min(detail.pembayaran, detail.grandTotal),
          piutang: detail.piutang,
          kembalian: detail.kembalian,
        }
      : null,
  );
  const [saveError, setSaveError] = useState("");
  const [examDirty, setExamDirty] = useState(false);
  const locked = saved !== null;

  // What is still missing before the sale can be saved (checked again by the server).
  const missing = [
    !cabang && "Cabang",
    !reg && "No. Daftar",
    !pelayanan && "Pelayanan",
    !bayar && "Jenis Bayar",
    !dokter && "Dokter PJ",
    obat.length + tindakan.length + penunjang.length === 0 && "minimal satu obat, tindakan atau penunjang",
  ].filter(Boolean) as string[];

  // Save first checks the form, then asks for the payment (the "Pembayaran" dialog); Bayar saves.
  const [payOpen, setPayOpen] = useState(false);
  const [kwtOpen, setKwtOpen] = useState(false);
  // Print menu: records the print, loads the saved sale with its lines and downloads the chosen format.
  const [printOpen, setPrintOpen] = useState(false);
  const [printing, setPrinting] = useState(false);
  // Fetch the PDF code now, so Print keeps working if a deploy replaces the chunks later.
  useEffect(() => {
    Promise.all([import("jspdf"), import("jspdf-autotable")]).catch(() => {});
  }, []);
  async function printAs(format: NotaFormat) {
    if (!token || !saved) return;
    // Open the print tab now, during the click: popup blockers allow it only here.
    const tab = openPrintTab();
    setPrintOpen(false);
    setPrinting(true);
    try {
      // Record the print first: the first one gets the print number shown on the nota.
      const cetak = await api.cetakKuitansi(token, saved.id);
      const nota = await api.nota(token, saved.id);
      await printNota(nota, format, cetak.printedUser ?? cetak.printedBy, cetak.printedNow ?? "", tab);
      setNotice(`${NOTA_FORMATS.find((f) => f.value === format)?.label} untuk ${saved.noKwi} ${tab ? "dibuka untuk dicetak" : "diunduh"}.`);
    } catch (err) {
      tab?.close();
      if (err instanceof ApiError && err.status === 401) return logout("Your session has expired. Please log in again.");
      setSaveError(err instanceof Error ? err.message : "Gagal mencetak nota");
    } finally {
      setPrinting(false);
    }
  }
  function startSave() {
    if (!token || locked) return;
    setSaveError("");
    if (missing.length) {
      setSaveError(`Lengkapi dulu: ${missing.join(", ")}.`);
      return;
    }
    if (examDirty) {
      setTab("pemeriksaan");
      setSaveError("Pemeriksaan belum disimpan: klik Simpan Pemeriksaan dulu, karena setelah transaksi disimpan kunjungan ditutup.");
      return;
    }
    setPayOpen(true);
  }

  /** Saves with the discount and payment from the dialog; payment null = exactly the amount due. */
  async function save(diskonValue: number, paymentValue: number | null): Promise<boolean> {
    if (!token || locked) return false;
    setDiskon(diskonValue ? String(diskonValue) : "");
    setPayment(paymentValue === null ? "" : String(paymentValue));
    setSaveError("");
    setSaving(true);
    try {
      const res = await api.savePengobatan(token, {
        cabang,
        tglKwi,
        idKun: reg!.id,
        idJenisRawat: Number(pelayanan),
        idJenisBayar: Number(bayar),
        idDokter: dokter!.id,
        obat: obat.map((l) => ({ idObat: l.idObat, byk: l.byk, dosis: l.dosis })),
        tindakan: tindakan.map((l) => ({ idTindakan: l.idTindakan, byk: l.byk, harga: l.isEdit ? l.harga : null })),
        penunjang: penunjang.map((l) => ({ idPenunjang: l.idTindakan, byk: l.byk })),
        diagnosa: diagnosa.map((d) => d.id),
        diskon: diskonValue,
        payment: paymentValue,
      });
      setSaved(res);
      setPayOpen(false);
      setNotice("");
      if (Math.abs(res.grandTotal - Math.max(total - diskonValue, 0)) > 0.5) {
        // The server prices from the master data; tell the cashier when the screen was stale.
        setNotice(`Catatan: total dihitung ulang server dengan harga terbaru menjadi ${money2.format(res.grandTotal)}.`);
      }
      return true;
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) {
        logout("Your session has expired. Please log in again.");
        return false;
      }
      setSaveError(err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : "Gagal menyimpan transaksi");
      return false;
    } finally {
      setSaving(false);
    }
  }

  return (
    <>
      <div className="page-head">
        <h1>{isEdit ? `Edit Pengobatan ${saved?.noKwi ?? ""}` : saved ? `Pengobatan ${saved.noKwi}` : "Add Pengobatan"}</h1>
        <div className="head-tools">
          <button type="button" className="btn-small" onClick={onCancel}>
            {saved ? "Kembali" : "Cancel"}
          </button>
          {!isEdit && (
            <button type="button" className="btn-small" title="Form baru" onClick={onNew}>
              New <FilePlus2 aria-hidden="true" />
            </button>
          )}
          <button type="button" className="btn-add" disabled={!saved} title={saved ? "Cetak kwitansi" : "Tersedia setelah disimpan"} onClick={() => setKwtOpen(true)}>
            Kuitansi <Receipt aria-hidden="true" />
          </button>
          <div className="filter-menu print-menu">
            <button
              type="button"
              className="btn-add"
              disabled={!saved || printing}
              aria-haspopup="menu"
              aria-expanded={printOpen}
              title={saved ? "Cetak nota" : "Tersedia setelah disimpan"}
              onClick={() => setPrintOpen((o) => !o)}
            >
              {printing ? "Mencetak…" : "Print"} <ChevronDown aria-hidden="true" />
            </button>
            {printOpen && saved && (
              <ul className="print-pop" role="menu" aria-label="Format cetak">
                {NOTA_FORMATS.map((f) => (
                  <li key={f.value} role="none">
                    <button type="button" role="menuitem" onClick={() => printAs(f.value)}>
                      <Printer aria-hidden="true" /> {f.label}
                    </button>
                  </li>
                ))}
              </ul>
            )}
          </div>
          {!isEdit && (
            <button
              type="button"
              className="btn-small btn-primary"
              onClick={startSave}
              disabled={saving || locked}
              title={locked ? "Sudah disimpan" : missing.length ? `Lengkapi: ${missing.join(", ")}` : "Simpan transaksi"}
            >
              {saving ? "Saving…" : locked ? "Saved" : "Save"} <Save aria-hidden="true" />
            </button>
          )}
        </div>
      </div>
      {detail?.status === "cancel" ? (
        <p className="alert error" role="status">
          Transaksi ini sudah di-<strong>cancel</strong>
          {detail.cancelBy ? ` oleh ${detail.cancelBy}` : ""}
          {detail.cancelAt ? ` pada ${detail.cancelAt}` : ""}
          {detail.cancelReason ? ` · Alasan: ${detail.cancelReason}` : ""}.
        </p>
      ) : isEdit ? (
        <p className="alert info" role="note">
          <Info aria-hidden="true" /> Transaksi tersimpan ditampilkan apa adanya (tidak bisa diubah). Untuk membatalkan, gunakan Cancel di daftar Pengobatan.
          {!detail?.reg && " Data pendaftaran transaksi ini sudah tidak ada."}
        </p>
      ) : saved ? (
        <p className="alert success" role="status">
          Transaksi tersimpan: No. Kuitansi <strong>{saved.noKwi}</strong> · Grand Total {money2.format(saved.grandTotal)}
          {saved.piutang > 0 ? ` · Piutang ${money2.format(saved.piutang)}` : ""}
          {saved.kembalian > 0 ? ` · Kembalian ${money2.format(saved.kembalian)}` : ""}. Kunjungan ditutup (CLOSE). Klik Kuitansi untuk mencetak, atau New untuk transaksi berikutnya.
        </p>
      ) : (
        <p className="alert info tx-mock-note" role="note">
          <Info aria-hidden="true" /> Pencarian No. RM, Jam Kes dan Perujuk OTC belum tersedia.
        </p>
      )}
      {saveError && (
        <p className="alert error" role="alert">
          {saveError}
          <button type="button" className="row-action alert-close" aria-label="Dismiss" onClick={() => setSaveError("")}>
            <X aria-hidden="true" />
          </button>
        </p>
      )}
      {notice && (
        <p className="alert info" role="status">
          {notice}
          <button type="button" className="row-action alert-close" aria-label="Dismiss" onClick={() => setNotice("")}>
            <X aria-hidden="true" />
          </button>
        </p>
      )}

      <fieldset className="tx-fieldset tx-head" disabled={locked}>
        <div className="tx-row">
          <label htmlFor="tx-cabang">
            Cabang<span className="req"> *</span>
          </label>
          <select id="tx-cabang" className="select" value={cabang} onChange={(e) => changeCabang(e.target.value)}>
            <option value="">-- Pilih Cabang --</option>
            {options.branches.map((b) => (
              <option key={b.kode} value={b.kode}>
                {b.nama || b.kode}
              </option>
            ))}
          </select>
        </div>
        <div className="tx-row">
          <label htmlFor="tx-nokwi">No. Kuitansi</label>
          <input id="tx-nokwi" readOnly placeholder="Otomatis saat disimpan" value={saved?.noKwi ?? ""} />
        </div>
        <div className="tx-row">
          <label htmlFor="tx-tglkwi">Tgl Kuitansi</label>
          <input id="tx-tglkwi" type="date" value={tglKwi} max={options.today} onChange={(e) => setTglKwi(e.target.value || options.today)} />
        </div>
      </fieldset>

      <div className="tx-grid tx-grid-2">
        <Card title="Data Pendaftaran" locked={locked}>
          <Row id="tx-nodaftar" label="No. Daftar" required>
            <Lookup id="tx-nodaftar" placeholder="No Pendaftaran" value={reg ? String(reg.id) : ""} onSearch={openLookup} />
          </Row>
          <Row id="tx-tgldaftar" label="Tanggal Daftar">
            <input id="tx-tgldaftar" readOnly value={reg?.tglMasuk ? `${fmtDate(reg.tglMasuk.slice(0, 10))} ${reg.tglMasuk.slice(11)}` : ""} />
          </Row>
          <Row id="tx-stskun" label="Sts. Kun">
            <input id="tx-stskun" readOnly value={reg?.status ?? ""} />
          </Row>
          <Row id="tx-pelayanan" label="Pelayanan" required>
            <select id="tx-pelayanan" className="select" value={pelayanan} onChange={(e) => setPelayanan(e.target.value)}>
              {options.pelayanan.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.name}
                </option>
              ))}
            </select>
          </Row>
          <Row id="tx-bayar" label="Jenis Bayar" required>
            <select id="tx-bayar" className="select" value={bayar} onChange={(e) => setBayar(e.target.value)}>
              {options.pembayaran.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.name}
                </option>
              ))}
            </select>
          </Row>
          <Row id="tx-dokter" label="Dokter PJ" required>
            <Lookup id="tx-dokter" placeholder={reg && !dokter ? "Belum di Pilih" : "Dokter"} value={dokter?.nama ?? ""} onSearch={openDokter} />
          </Row>
        </Card>

        <Card title="Data Pasien" locked={locked}>
          <Row id="tx-norm" label="No. RM">
            <Lookup id="tx-norm" placeholder="No RM" value={reg?.noRm ?? ""} onSearch={() => notYet("pasien")} />
          </Row>
          <Row id="tx-nama" label="Nama & Umur">
            <div className="tx-split tx-split-name">
              <input id="tx-nama" readOnly value={reg?.nama ?? ""} />
              <input aria-label="Umur" readOnly value={umur === null ? "" : `${umur} Th`} />
            </div>
          </Row>
          <Row id="tx-alamat" label="Alamat">
            <input id="tx-alamat" readOnly value={reg?.alamat ?? ""} />
          </Row>
          <Row id="tx-hp" label="Telp / HP">
            <input id="tx-hp" readOnly value={reg?.hp ?? ""} />
          </Row>
          <Row id="tx-pj" label="Penanggung Jawab">
            <input id="tx-pj" readOnly value={reg?.penanggung ?? ""} />
          </Row>
          <Row id="tx-jamkes" label="Jam Kes / No Kartu">
            <div className="tx-split">
              <Lookup id="tx-jamkes" placeholder="Jamkes" value={reg?.jamkes ?? ""} onSearch={() => notYet("jaminan kesehatan")} />
              <Lookup id="tx-nokartu" placeholder="No Kartu" value={reg?.noJamkes ?? ""} onSearch={() => notYet("kartu jaminan")} />
            </div>
          </Row>
        </Card>
      </div>

      <div className="tx-grid tx-grid-main">
        <Card title="Daftar Pengobatan & Pemeriksaan">
          <div className="tabs" role="tablist" aria-label="Daftar pengobatan">
            {TABS.map((t) => (
              <button
                key={t.key}
                type="button"
                role="tab"
                id={`tx-tab-${t.key}`}
                aria-selected={tab === t.key}
                aria-controls={`tx-panel-${t.key}`}
                className={tab === t.key ? "tab active" : "tab"}
                onClick={() => setTab(t.key)}
              >
                {t.label}
              </button>
            ))}
          </div>
          <fieldset role="tabpanel" id={`tx-panel-${tab}`} aria-labelledby={`tx-tab-${tab}`} className="tx-tabpanel tx-fieldset" disabled={locked}>
            {tab === "obat" && (
              <>
                <div className="tx-items-head">
                  <div>
                    <button type="button" className="btn-add" onClick={openObat}>
                      <Plus aria-hidden="true" /> Add Obat
                    </button>
                    <p className="tx-sum" aria-live="polite">
                      Total Kena Diskon : {plain.format(totals.kenaDiskon)} di bulatkan : {plain.format(totals.kenaDiskonRounded)}
                    </p>
                  </div>
                  <div className="tx-sum tx-sum-right" aria-live="polite">
                    <p>
                      Total : {plain.format(totals.total)} di bulatkan : {plain.format(totals.totalRounded)}
                    </p>
                    <p>
                      PPN : {plain.format(totals.ppn)} di bulatkan : {plain.format(totals.ppnRounded)}
                    </p>
                    <p>
                      PPN Kena Diskon : {plain.format(totals.ppnKenaDiskon)} di bulatkan : {plain.format(totals.ppnKenaDiskonRounded)}
                    </p>
                  </div>
                </div>
                <ObatTable
                  lines={obat}
                  onChange={(id, byk, dosis) => setObat((list) => list.map((l) => (l.idObat === id ? { ...l, byk, dosis } : l)))}
                  onRemove={(id) => setObat((list) => list.filter((l) => l.idObat !== id))}
                  onError={setNotice}
                />
                <div className="tx-otc">
                  <label htmlFor="tx-otc">Perujuk Obat Bebas / OTC</label>
                  <Lookup id="tx-otc" placeholder="Perujuk OTC" onSearch={() => notYet("perujuk OTC")} />
                </div>
              </>
            )}
            {tab === "tindakan" && (
              <>
                <div className="tx-items-head">
                  <button type="button" className="btn-add" onClick={openTindakan}>
                    <Plus aria-hidden="true" /> Add Tindakan
                  </button>
                  <p className="tx-sum tx-sum-right" aria-live="polite">
                    Total : {plain.format(totalTindakan)}
                  </p>
                </div>
                <TindakanTable
                  lines={tindakan}
                  onChange={(id, harga, byk) => setTindakan((list) => list.map((l) => (l.idTindakan === id ? { ...l, harga, byk } : l)))}
                  onRemove={(id) => setTindakan((list) => list.filter((l) => l.idTindakan !== id))}
                  onError={setNotice}
                />
              </>
            )}
            {tab === "penunjang" && (
              <>
                <div className="tx-items-head">
                  <button type="button" className="btn-add" onClick={openPenunjang}>
                    <Plus aria-hidden="true" /> Add Penunjang
                  </button>
                  <p className="tx-sum tx-sum-right" aria-live="polite">
                    Total : {plain.format(totalPenunjang)}
                  </p>
                </div>
                <TindakanTable
                  lines={penunjang}
                  showDokter={false}
                  empty="Belum ada pemeriksaan penunjang."
                  onChange={(id, harga, byk) => setPenunjang((list) => list.map((l) => (l.idTindakan === id ? { ...l, harga, byk } : l)))}
                  onRemove={(id) => setPenunjang((list) => list.filter((l) => l.idTindakan !== id))}
                  onError={setNotice}
                />
              </>
            )}
            {tab === "diagnosa" && (
              <>
                <div className="tx-items-head">
                  <button type="button" className="btn-add" onClick={() => setDiagnosaOpen(true)}>
                    <Plus aria-hidden="true" /> Add Diagnosa
                  </button>
                  <p className="tx-sum tx-sum-right" aria-live="polite">
                    Jumlah diagnosa : {diagnosa.length}
                  </p>
                </div>
                <div className="table-wrap">
                  <table className="data-table tx-items">
                    <thead>
                      <tr>
                        <th>ID</th>
                        <th>Kode ICD-X</th>
                        <th>Nama Penyakit</th>
                        <th>Jenis ICD-X</th>
                        <th>Action</th>
                      </tr>
                    </thead>
                    <tbody>
                      {diagnosa.length === 0 ? (
                        <tr>
                          <td colSpan={5} className="dt-empty">
                            Belum ada diagnosa.
                          </td>
                        </tr>
                      ) : (
                        diagnosa.map((d) => (
                          <tr key={d.id}>
                            <td>{d.id}</td>
                            <td>{d.kode}</td>
                            <td>{d.nama}</td>
                            <td>{d.jenis || "-"}</td>
                            <td className="tx-actions">
                              <button
                                type="button"
                                className="row-action danger"
                                aria-label={`Hapus diagnosa ${d.kode}`}
                                title="Hapus"
                                onClick={() => setDiagnosa((list) => list.filter((x) => x.id !== d.id))}
                              >
                                <Trash2 aria-hidden="true" />
                              </button>
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </>
            )}
            <PemeriksaanTab hidden={tab !== "pemeriksaan"} idKun={reg?.id ?? null} idDokter={dokter?.id ?? null} onDirty={setExamDirty} />
          </fieldset>
        </Card>

        <Card title="Data Pembayaran" className="tx-pay" locked={locked}>
          <output className="tx-total" aria-label="Total tagihan">
            {money2.format(total)}
          </output>
          <Row id="tx-payment" label="Payment">
            <input
              id="tx-payment"
              readOnly
              className="num"
              title="Diisi di dialog Pembayaran saat Save"
              value={money.format(saved ? saved.pembayaran : payment.trim() === "" ? 0 : num(payment))}
            />
          </Row>
          <Row id="tx-diskon" label="Diskon">
            <input id="tx-diskon" readOnly className="num" title="Diisi di dialog Pembayaran saat Save" value={money.format(num(diskon))} />
          </Row>
          <Row id="tx-point" label="Tukar Point">
            <input
              id="tx-point"
              type="number"
              min={0}
              step="any"
              inputMode="decimal"
              className="num"
              placeholder="0"
              disabled
              title="Tukar point belum dipakai (di data lama selalu 0)."
              value={tukarPoint}
              onChange={(e) => setTukarPoint(e.target.value)}
            />
          </Row>
          <Row id="tx-grand" label="Grand Total">
            <input id="tx-grand" readOnly className="num" value={money.format(saved ? saved.grandTotal : grandTotal)} />
          </Row>
          <Row id="tx-sisa" label="Sisa">
            <input id="tx-sisa" readOnly className="num" value={money.format(saved ? saved.piutang : sisa)} />
          </Row>
          <Row id="tx-kembali" label="Kembalian">
            <input id="tx-kembali" readOnly className="num" value={money.format(saved ? saved.kembalian : kembalian)} />
          </Row>
        </Card>
      </div>
      {kwtOpen && saved && <KuitansiDialog saleId={saved.id} onClose={() => setKwtOpen(false)} />}
      {payOpen && (
        <PembayaranDialog
          jumlah={total}
          diskon={num(diskon)}
          busy={saving}
          error={saveError}
          onApplyDiskon={(d) => setDiskon(d ? String(d) : "")}
          onBayar={(d, p) => void save(d, p)}
          onClose={() => setPayOpen(false)}
        />
      )}
      {lookupOpen && <PendaftaranLookup cabang={cabang} options={options} onPick={pick} onClose={() => setLookupOpen(false)} />}
      {dokterOpen && (
        <DokterLookup
          cabang={cabang}
          branchName={options.branches.find((b) => b.kode === cabang)?.nama ?? cabang}
          onPick={(d) => {
            changeDokter({ id: d.id, nama: d.nama });
            setDokterOpen(false);
          }}
          onClose={() => setDokterOpen(false)}
        />
      )}
      {tindakanOpen && dokter && (
        <TindakanLookup
          title={`Data Tindakan Dokter : ${dokter.nama}`}
          load={(token) => api.pengobatanTindakan(token, cabang, dokter.id)}
          onPick={addTindakan}
          onClose={() => setTindakanOpen(false)}
        />
      )}
      {diagnosaOpen && (
        <DiagnosaLookup
          chosen={new Set(diagnosa.map((d) => d.id))}
          onPick={(d) => {
            setDiagnosa((list) => [...list, d]);
            setDiagnosaOpen(false);
          }}
          onClose={() => setDiagnosaOpen(false)}
        />
      )}
      {penunjangOpen && (
        <TindakanLookup
          title="Data Tindakan"
          load={(token) => api.pengobatanPenunjang(token, cabang).then((res) => ({ rows: res.rows.map((p) => ({ ...p, isEdit: false })) }))}
          onPick={addPenunjang}
          onClose={() => setPenunjangOpen(false)}
        />
      )}
      {obatOpen && (
        <ObatLookup
          cabang={cabang}
          branchName={options.branches.find((b) => b.kode === cabang)?.nama ?? cabang}
          inList={new Map(obat.map((l) => [l.idObat, l.byk]))}
          onPick={addObat}
          onClose={() => setObatOpen(false)}
        />
      )}
    </>
  );
}
