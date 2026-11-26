import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { CalendarDays, Plus, RotateCcw, Search, X } from "lucide-react";
import { api, ApiError, StokAdjus, StokAdjusObat, StokAdjusOptions, StokAdjusQuery } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { ALL_BRANCHES, BranchFilter, initialBranch, NoBranchNotice } from "../components/branchView";
import { FieldShell, Modal, useModal } from "../components/FormKit";

const EXPORT_LIMIT = 5000;
const plain = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });

/** The "Pencarian" panel's fields; applied together with the search button. */
type Cari = { nama: string; ket: string; status: "" | "1" | "0" };
const NO_CARI: Cari = { nama: "", ket: "", status: "" };

type Notice = { kind: "info" | "error" | "success"; text: string } | null;

/** YYYY-MM-DD → DD/MM/YYYY, like the legacy list. */
const dmy = (d: string | null) => (d ? `${d.slice(8, 10)}/${d.slice(5, 7)}/${d.slice(0, 4)}` : "-");

/**
 * Stok Farmasi > Stok Adjusment (legacy /StokAdjusment): the stock corrections
 * at the user's branches in the period, newest first. Add records one and moves
 * the branch stock up (Masuk) or down (Keluar).
 */
export default function StokAdjusmentPage() {
  const { token, logout } = useAuth();
  const [options, setOptions] = useState<StokAdjusOptions | null>(null);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [branch, setBranch] = useState(ALL_BRANCHES);
  const [cariOpen, setCariOpen] = useState(false);
  const [draft, setDraft] = useState<Cari>(NO_CARI);
  const [cari, setCari] = useState<Cari>(NO_CARI);
  const [page, setPage] = useState<{ rows: StokAdjus[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [notice, setNotice] = useState<Notice>(null);
  const [adding, setAdding] = useState(false);
  const [reload, setReload] = useState(0);
  const seq = useRef(0);

  const expired = useCallback(() => logout("Your session has expired. Please log in again."), [logout]);

  useEffect(() => {
    if (!token) return;
    api
      .stokAdjusOptions(token)
      .then((o) => {
        setOptions(o);
        setFrom(o.defaultFrom);
        setTo(o.today);
        setBranch(initialBranch(o.branches));
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const params = (q: RemoteQuery, size = q.pageSize, pageNo = q.page): StokAdjusQuery => ({
    from,
    to,
    page: pageNo,
    size,
    q: q.search,
    sort: q.sort?.key,
    dir: q.sort?.dir,
    cabang: branch === ALL_BRANCHES ? undefined : branch,
    nama: cari.nama || undefined,
    ket: cari.ket || undefined,
    status: cari.status || undefined,
  });

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token || !from) return;
      const mine = ++seq.current;
      setLoading(true);
      api
        .stokAdjus(token, params(q))
        .then((res) => mine === seq.current && setPage(res))
        .catch((err) => {
          if (mine !== seq.current) return;
          if (err instanceof ApiError && err.status === 401) expired();
          else setNotice({ kind: "error", text: err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : "Failed to load adjustments" });
        })
        .finally(() => mine === seq.current && setLoading(false));
    },
    [token, from, to, branch, cari], // eslint-disable-line react-hooks/exhaustive-deps
  );

  const fetchAll = async (q: RemoteQuery): Promise<StokAdjus[]> => {
    if (!token) return [];
    const res = await api.stokAdjus(token, params(q, EXPORT_LIMIT, 1));
    if (res.total > EXPORT_LIMIT) {
      setNotice({
        kind: "info",
        text: `The export holds the first ${EXPORT_LIMIT.toLocaleString("en-US")} of ${res.total.toLocaleString("en-US")} rows. Narrow the period, pick a branch or use Cari to export the rest.`,
      });
    }
    return res.rows;
  };

  const applyCari = () => setCari({ ...draft, nama: draft.nama.trim(), ket: draft.ket.trim() });
  const resetCari = () => {
    setDraft(NO_CARI);
    setCari(NO_CARI);
  };

  const columns: Column<StokAdjus>[] = [
    { key: "id", header: "Kode", value: (r) => String(r.id), sortValue: (r) => r.id },
    { key: "tanggal", header: "Tanggal", value: (r) => dmy(r.tanggal), sortValue: (r) => r.tanggal ?? "" },
    { key: "idObat", header: "Kode Obat", value: (r) => String(r.idObat), sortValue: (r) => r.idObat },
    { key: "nama", header: "Nama", value: (r) => r.nama, className: "dt-wrap" },
    { key: "status", header: "Status", value: (r) => r.statusDesc, sortValue: (r) => r.status },
    { key: "ket", header: "Keterangan", value: (r) => r.ket || "-", className: "dt-wrap-sm" },
    { key: "byk", header: "Banyak", value: (r) => plain.format(r.byk), sortValue: (r) => r.byk },
    { key: "cabang", header: "Cabang", value: (r) => r.namaCabang || r.cabang, className: "dt-wrap-sm" },
  ];

  const branches = options?.branches ?? [];
  const cariCount = (cari.nama ? 1 : 0) + (cari.ket ? 1 : 0) + (cari.status ? 1 : 0);

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Stok Adjusment</h1>
        <div className="head-tools">
          <label className="date-range" title="Tanggal adjusment">
            <CalendarDays aria-hidden="true" />
            <input type="date" aria-label="Dari tanggal" value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} />
            <span aria-hidden="true">–</span>
            <input type="date" aria-label="Sampai tanggal" value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} />
          </label>
          <button type="button" className="btn-add" aria-expanded={cariOpen} aria-controls="stok-adjus-cari" onClick={() => setCariOpen((o) => !o)}>
            Cari{cariCount ? ` (${cariCount})` : ""} <Search aria-hidden="true" />
          </button>
          {branches.length > 0 && (
            <button type="button" className="btn-add" onClick={() => setAdding(true)}>
              Add <Plus aria-hidden="true" />
            </button>
          )}
        </div>
      </div>
      {cariOpen && options && (
        <form
          id="stok-adjus-cari"
          className="search-panel"
          aria-label="Pencarian"
          onSubmit={(e) => {
            e.preventDefault();
            applyCari();
          }}
        >
          <h3>
            <Search aria-hidden="true" /> Pencarian
          </h3>
          <div className="search-panel-row cols-3">
            <FieldShell id="sa-nama" label="Nama Obat">
              <input id="sa-nama" value={draft.nama} placeholder="Input nama obat" onChange={(e) => setDraft({ ...draft, nama: e.target.value })} />
            </FieldShell>
            <FieldShell id="sa-ket" label="Keterangan">
              <input id="sa-ket" value={draft.ket} placeholder="Input keterangan" onChange={(e) => setDraft({ ...draft, ket: e.target.value })} />
            </FieldShell>
            <FieldShell id="sa-status" label="Status">
              <select id="sa-status" className="select" value={draft.status} onChange={(e) => setDraft({ ...draft, status: e.target.value as Cari["status"] })}>
                <option value="">Semua</option>
                <option value="1">Masuk</option>
                <option value="0">Keluar</option>
              </select>
            </FieldShell>
            <div className="search-panel-actions">
              <button type="submit" className="icon-round" aria-label="Search adjustments" title="Search">
                <Search aria-hidden="true" />
              </button>
              <button type="button" className="icon-round" aria-label="Reset search" title="Reset" onClick={resetCari}>
                <RotateCcw aria-hidden="true" />
              </button>
            </div>
          </div>
        </form>
      )}
      {notice && (
        <p className={`alert ${notice.kind}`} role={notice.kind === "error" ? "alert" : "status"}>
          {notice.text}
          <button type="button" className="row-action alert-close" aria-label="Dismiss" onClick={() => setNotice(null)}>
            <X aria-hidden="true" />
          </button>
        </p>
      )}
      {loadError ? (
        <p className="alert error" role="alert">
          {loadError}
        </p>
      ) : !options ? (
        <p className="muted">Loading…</p>
      ) : branches.length === 0 ? (
        <NoBranchNotice what="stock adjustments" />
      ) : (
        <DataTable
          // New filters (or a new adjustment) start again from page 1.
          key={`${from}|${to}|${branch}|${cari.nama}|${cari.ket}|${cari.status}|${reload}`}
          title={`Stok Adjusment ${from}${to !== from ? " - " + to : ""}`}
          columns={columns}
          rows={page?.rows ?? []}
          rowKey={(r) => String(r.id)}
          initialSort={{ key: "id", dir: "desc" }}
          defaultPageSize={10}
          toolbarStart={<BranchFilter branches={branches} value={branch} onChange={setBranch} />}
          remote={{ total: page?.total ?? 0, unfilteredTotal: page?.unfilteredTotal ?? 0, loading: loading || !page, onQuery: fetchPage, fetchAll }}
        />
      )}
      {adding && options && (
        <AddAdjusDialog
          branches={branches}
          initialCabang={branch === ALL_BRANCHES ? branches[0]?.kode ?? "" : branch}
          today={options.today}
          onClose={() => setAdding(false)}
          onSaved={(msg) => {
            setAdding(false);
            setNotice({ kind: "success", text: msg });
            if (to < options.today) setTo(options.today);
            setReload((n) => n + 1);
          }}
        />
      )}
    </section>
  );
}

type Errors = Partial<Record<"cabang" | "tanggal" | "idObat" | "status" | "byk" | "ket", string>>;

const obatLabel = (o: StokAdjusObat) => `${o.id} - ${o.nama}`;

/** The Add form: one adjustment of one drug at one branch. */
function AddAdjusDialog({
  branches,
  initialCabang,
  today,
  onClose,
  onSaved,
}: {
  branches: { kode: string; nama: string }[];
  initialCabang: string;
  today: string;
  onClose: () => void;
  onSaved: (message: string) => void;
}) {
  const { token, logout } = useAuth();
  const dialogRef = useModal();
  const [cabang, setCabang] = useState(initialCabang);
  const [tanggal, setTanggal] = useState(today);
  const [obatList, setObatList] = useState<StokAdjusObat[] | null>(null);
  const [obatText, setObatText] = useState("");
  const [status, setStatus] = useState<"" | "1" | "0">("1");
  const [byk, setByk] = useState("1");
  const [ket, setKet] = useState("");
  const [errors, setErrors] = useState<Errors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  useEffect(() => {
    if (!token || !cabang) return;
    setObatList(null);
    setObatText("");
    api
      .stokAdjusObat(token, cabang)
      .then((res) => setObatList(res.rows))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
        else setError(err instanceof Error ? err.message : "Failed to load drugs");
      });
  }, [token, cabang]); // eslint-disable-line react-hooks/exhaustive-deps

  const byLabel = useMemo(() => new Map((obatList ?? []).map((o) => [obatLabel(o), o])), [obatList]);
  const obat = byLabel.get(obatText.trim());

  const submit = async (e: FormEvent) => {
    e.preventDefault();
    if (!token) return;
    const n = Number(byk.trim().replace(",", "."));
    const errs: Errors = {};
    if (!cabang) errs.cabang = "Pilih Cabang";
    if (!tanggal) errs.tanggal = "Isi Tanggal";
    else if (tanggal > today) errs.tanggal = "Tanggal tidak boleh setelah hari ini";
    if (!obat) errs.idObat = "Pilih obat dari daftar";
    if (!status) errs.status = "Pilih Status";
    if (!Number.isFinite(n) || n <= 0) errs.byk = "Banyak harus lebih dari 0";
    else if (status === "0" && obat && n > obat.stok) errs.byk = `Stok saat ini ${plain.format(obat.stok)}`;
    if (!ket.trim()) errs.ket = "Isi Keterangan";
    else if (ket.trim().length > 60) errs.ket = "Keterangan maksimal 60 karakter";
    setErrors(errs);
    setError("");
    if (Object.keys(errs).length > 0 || !obat) return;
    setBusy(true);
    try {
      const res = await api.saveStokAdjus(token, { cabang, tanggal, idObat: obat.id, status: status === "1" ? 1 : 0, byk: n, ket: ket.trim() });
      onSaved(`Adjusment #${res.id} tersimpan. Stok ${obat.nama} sekarang ${plain.format(res.stok)} ${obat.satuan}.`);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
      else if (err instanceof ApiError && Object.keys(err.fieldErrors).length > 0) setErrors(err.fieldErrors as Errors);
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  };

  return (
    <Modal dialogRef={dialogRef} title="Add Stok Adjusment" onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <div className="form-grid">
        <FieldShell id="adj-cabang" label="Cabang" required error={errors.cabang}>
          <select id="adj-cabang" value={cabang} onChange={(e) => setCabang(e.target.value)}>
            {branches.map((b) => (
              <option key={b.kode} value={b.kode}>
                {b.nama}
              </option>
            ))}
          </select>
        </FieldShell>
        <FieldShell id="adj-tanggal" label="Tanggal" required error={errors.tanggal}>
          <input id="adj-tanggal" type="date" value={tanggal} max={today} onChange={(e) => setTanggal(e.target.value)} />
        </FieldShell>
        <FieldShell
          id="adj-obat"
          label="Obat"
          required
          wide
          error={errors.idObat}
          hint={obat ? `${obat.jenis || "-"} · stok ${plain.format(obat.stok)} ${obat.satuan}` : obatList === null ? "Memuat obat…" : "Ketik kode atau nama, lalu pilih"}
        >
          <input
            id="adj-obat"
            list="adj-obat-list"
            value={obatText}
            disabled={obatList === null}
            placeholder="Cari obat"
            autoComplete="off"
            onChange={(e) => setObatText(e.target.value)}
          />
          <datalist id="adj-obat-list">
            {(obatList ?? []).map((o) => (
              <option key={o.id} value={obatLabel(o)} />
            ))}
          </datalist>
        </FieldShell>
        <FieldShell id="adj-status" label="Status" required error={errors.status}>
          <select id="adj-status" value={status} onChange={(e) => setStatus(e.target.value as "" | "1" | "0")}>
            <option value="1">Masuk (stok bertambah)</option>
            <option value="0">Keluar (stok berkurang)</option>
          </select>
        </FieldShell>
        <FieldShell id="adj-byk" label={`Banyak${obat?.satuan ? ` (${obat.satuan})` : ""}`} required error={errors.byk}>
          <input id="adj-byk" inputMode="decimal" value={byk} onChange={(e) => setByk(e.target.value)} />
        </FieldShell>
        <FieldShell id="adj-ket" label="Keterangan" required wide error={errors.ket}>
          <input id="adj-ket" value={ket} maxLength={60} placeholder="Alasan adjusment" onChange={(e) => setKet(e.target.value)} />
        </FieldShell>
      </div>
    </Modal>
  );
}
