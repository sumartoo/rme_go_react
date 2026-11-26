import { FormEvent, useCallback, useEffect, useMemo, useRef, useState } from "react";
import { CalendarDays, Search, SquareCheck, X } from "lucide-react";
import { api, ApiError, MyBranch, StokAdjusObat, StokTrace, StokTraceQuery } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { useModal } from "../components/FormKit";

const EXPORT_LIMIT = 5000;
const plain = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });

/** YYYY-MM-DD of a date in the clinic's time zone. */
function clinicDay(d: Date): string {
  return new Intl.DateTimeFormat("en-CA", { timeZone: "Asia/Jakarta" }).format(d);
}

/** "YYYY-MM-DD HH:MM:SS.mmm" → "DD/MM/YYYY HH:MM:SS". */
const tanggal = (s: string) => (s ? `${s.slice(8, 10)}/${s.slice(5, 7)}/${s.slice(0, 4)} ${s.slice(11, 19)}` : "-");

/** The search the table shows; set by Cari. */
type Applied = { cabang: string; obat: StokAdjusObat; from: string; to: string };

const HEADERS = ["Tanggal", "Obat", "Stock Lama", "In Out", "Banyak", "Stock", "User", "Transaksi", "Keterangan", "Cabang"];

/**
 * Stok Farmasi > Stok Trace (legacy /StokTrace): pick a branch, a drug and a
 * period, then Cari lists the drug's stock movements (trxobat) in the order
 * they happened, with the stock before and after each one.
 */
export default function StokTracePage() {
  const { token, logout } = useAuth();
  const [branches, setBranches] = useState<MyBranch[] | null>(null);
  const [cabang, setCabang] = useState("");
  const [obat, setObat] = useState<StokAdjusObat | null>(null);
  const [lookupOpen, setLookupOpen] = useState(false);
  const today = useMemo(() => clinicDay(new Date()), []);
  const [from, setFrom] = useState(() => {
    const d = new Date(`${today}T00:00:00Z`);
    d.setUTCDate(d.getUTCDate() - 29);
    return d.toISOString().slice(0, 10);
  });
  const [to, setTo] = useState(today);
  const [errors, setErrors] = useState<{ cabang?: string; obat?: string; from?: string }>({});
  const [loadError, setLoadError] = useState("");
  const [notice, setNotice] = useState<{ kind: "info" | "error"; text: string } | null>(null);
  const [applied, setApplied] = useState<Applied | null>(null);
  const [searchNo, setSearchNo] = useState(0);
  const [page, setPage] = useState<{ rows: StokTrace[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const seq = useRef(0);

  const expired = useCallback(() => logout("Your session has expired. Please log in again."), [logout]);

  useEffect(() => {
    if (!token) return;
    api
      .myBranches(token)
      .then((res) => {
        setBranches(res.branches);
        if (res.branches.length === 1) setCabang(res.branches[0].kode);
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load branches");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const changeCabang = (kode: string) => {
    setCabang(kode);
    setObat(null); // the drug list depends on the branch
    setErrors({});
  };

  const openLookup = () => {
    if (!cabang) return setErrors({ cabang: "Pilih Cabang dulu" });
    setLookupOpen(true);
  };

  const cari = (e: FormEvent) => {
    e.preventDefault();
    const errs: typeof errors = {};
    if (!cabang) errs.cabang = "Pilih Cabang";
    if (!obat) errs.obat = "Pilih Obat";
    if (!from || !to || to < from) errs.from = "Periode tidak valid";
    setErrors(errs);
    if (Object.keys(errs).length > 0 || !obat) return;
    setNotice(null);
    setPage(null);
    setApplied({ cabang, obat, from, to });
    setSearchNo((n) => n + 1); // Cari again reloads, even with the same values
  };

  const params = (a: Applied, q: RemoteQuery, size = q.pageSize, pageNo = q.page): StokTraceQuery => ({
    cabang: a.cabang,
    obat: a.obat.id,
    from: a.from,
    to: a.to,
    page: pageNo,
    size,
    q: q.search,
    sort: q.sort?.key,
    dir: q.sort?.dir,
  });

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token || !applied) return;
      const mine = ++seq.current;
      setLoading(true);
      api
        .stokTrace(token, params(applied, q))
        .then((res) => mine === seq.current && setPage(res))
        .catch((err) => {
          if (mine !== seq.current) return;
          if (err instanceof ApiError && err.status === 401) expired();
          else setNotice({ kind: "error", text: err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : "Failed to load the trace" });
        })
        .finally(() => mine === seq.current && setLoading(false));
    },
    [token, applied], // eslint-disable-line react-hooks/exhaustive-deps
  );

  const fetchAll = async (q: RemoteQuery): Promise<StokTrace[]> => {
    if (!token || !applied) return [];
    const res = await api.stokTrace(token, params(applied, q, EXPORT_LIMIT, 1));
    if (res.total > EXPORT_LIMIT) {
      setNotice({
        kind: "info",
        text: `The export holds the first ${EXPORT_LIMIT.toLocaleString("en-US")} of ${res.total.toLocaleString("en-US")} rows. Narrow the period to export the rest.`,
      });
    }
    return res.rows;
  };

  const num = (key: "lastStock" | "byk" | "stock", header: string): Column<StokTrace> => ({
    key,
    header,
    value: (r) => plain.format(r[key]),
    sortValue: (r) => r[key],
  });
  const columns: Column<StokTrace>[] = [
    { key: "createdDate", header: "Tanggal", value: (r) => tanggal(r.createdDate), sortValue: (r) => r.createdDate },
    { key: "nama", header: "Obat", value: (r) => r.nama, className: "dt-wrap" },
    num("lastStock", "Stock Lama"),
    { key: "inOut", header: "In Out", value: (r) => r.inOut || "-" },
    num("byk", "Banyak"),
    num("stock", "Stock"),
    { key: "createdBy", header: "User", value: (r) => r.createdBy || "-" },
    { key: "transaksi", header: "Transaksi", value: (r) => r.transaksi || "-" },
    { key: "remarks", header: "Keterangan", value: (r) => r.remarks || "-", className: "dt-wrap-sm" },
    { key: "cabang", header: "Cabang", value: (r) => r.namaCabang || r.cabang, className: "dt-wrap-sm" },
  ];

  return (
    <section className="panel">
      <form className="page-head trace-head" onSubmit={cari} noValidate>
        <h1>Stok Trace</h1>
        <div className="head-tools trace-tools">
          <div className="field trace-field">
            <label htmlFor="trace-cabang">
              Cabang<span className="req"> *</span>
            </label>
            <select
              id="trace-cabang"
              className="select"
              value={cabang}
              disabled={!branches}
              aria-invalid={!!errors.cabang}
              onChange={(e) => changeCabang(e.target.value)}
            >
              <option value="">-- Pilih Cabang --</option>
              {(branches ?? []).map((b) => (
                <option key={b.kode} value={b.kode}>
                  {b.nama}
                </option>
              ))}
            </select>
            {errors.cabang && <span className="field-error">{errors.cabang}</span>}
          </div>
          <div className="field trace-field">
            <label htmlFor="trace-obat">
              Obat<span className="req"> *</span>
            </label>
            <div className="trace-lookup">
              <input
                id="trace-obat"
                readOnly
                placeholder="Obat"
                value={obat ? `${obat.id} - ${obat.nama}` : ""}
                title={obat ? `${obat.id} - ${obat.nama}` : undefined}
                aria-invalid={!!errors.obat}
                onClick={openLookup}
              />
              <button type="button" className="trace-lookup-btn" aria-label="Cari obat" title="Cari obat" onClick={openLookup}>
                <Search aria-hidden="true" />
              </button>
            </div>
            {errors.obat && <span className="field-error">{errors.obat}</span>}
          </div>
          <div className="field trace-field">
            <label className="date-range" title="Periode">
              <CalendarDays aria-hidden="true" />
              <input type="date" aria-label="Dari tanggal" value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} />
              <span aria-hidden="true">–</span>
              <input type="date" aria-label="Sampai tanggal" value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} />
            </label>
            {errors.from && <span className="field-error">{errors.from}</span>}
          </div>
          <button type="submit" className="btn-add">
            Cari <Search aria-hidden="true" />
          </button>
        </div>
      </form>
      {loadError && (
        <p className="alert error" role="alert">
          {loadError}
        </p>
      )}
      {notice && (
        <p className={`alert ${notice.kind}`} role={notice.kind === "error" ? "alert" : "status"}>
          {notice.text}
          <button type="button" className="row-action alert-close" aria-label="Dismiss" onClick={() => setNotice(null)}>
            <X aria-hidden="true" />
          </button>
        </p>
      )}
      {applied ? (
        <DataTable
          key={searchNo}
          title={`Stok Trace ${applied.obat.nama} ${applied.from}${applied.to !== applied.from ? " - " + applied.to : ""}`}
          columns={columns}
          rows={page?.rows ?? []}
          rowKey={(r) => String(r.id)}
          initialSort={{ key: "createdDate", dir: "asc" }}
          defaultPageSize={25}
          remote={{ total: page?.total ?? 0, unfilteredTotal: page?.unfilteredTotal ?? 0, loading: loading || !page, onQuery: fetchPage, fetchAll }}
        />
      ) : (
        <div className="table-wrap">
          <table className="data-table trace-table">
            <thead>
              <tr>
                {HEADERS.map((h) => (
                  <th key={h} scope="col">
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              <tr>
                <td colSpan={HEADERS.length} className="muted trace-empty">
                  Pilih Cabang dan Obat, lalu klik Cari.
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      )}
      {lookupOpen && (
        <ObatLookup
          cabang={cabang}
          branchName={branches?.find((b) => b.kode === cabang)?.nama ?? cabang}
          onPick={(o) => {
            setObat(o);
            setErrors((e) => ({ ...e, obat: undefined }));
            setLookupOpen(false);
          }}
          onClose={() => setLookupOpen(false)}
        />
      )}
    </section>
  );
}

/** The "Obat" lookup: the drugs stocked at the chosen branch, filtered as you type. */
function ObatLookup({
  cabang,
  branchName,
  onPick,
  onClose,
}: {
  cabang: string;
  branchName: string;
  onPick: (o: StokAdjusObat) => void;
  onClose: () => void;
}) {
  const { token, logout } = useAuth();
  const dialogRef = useModal();
  const [rows, setRows] = useState<StokAdjusObat[] | null>(null);
  const [q, setQ] = useState("");
  const [error, setError] = useState("");

  useEffect(() => {
    if (!token) return;
    api
      .stokTraceObat(token, cabang)
      .then((res) => setRows(res.rows))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
        else setError(err instanceof Error ? err.message : "Failed to load drugs");
      });
  }, [token, cabang]); // eslint-disable-line react-hooks/exhaustive-deps

  const shown = useMemo(() => {
    const s = q.trim().toLowerCase();
    const list = rows ?? [];
    const hit = s ? list.filter((o) => String(o.id) === s || o.nama.toLowerCase().includes(s) || o.jenis.toLowerCase().includes(s)) : list;
    return hit.slice(0, 200);
  }, [rows, q]);

  return (
    <dialog ref={dialogRef} className="modal modal-wide" aria-labelledby="trace-lookup-title" onClose={onClose}>
      <header className="modal-head">
        <h2 id="trace-lookup-title">Data Obat · {branchName}</h2>
        <button type="button" className="row-action" aria-label="Close" onClick={onClose}>
          <X aria-hidden="true" />
        </button>
      </header>
      <div className="modal-body">
        <input autoFocus value={q} placeholder="Cari kode, nama atau jenis obat" aria-label="Cari obat" onChange={(e) => setQ(e.target.value)} />
        {error ? (
          <p className="alert error" role="alert">
            {error}
          </p>
        ) : rows === null ? (
          <p className="muted">Loading…</p>
        ) : (
          <div className="table-wrap">
            <table className="data-table">
              <thead>
                <tr>
                  <th scope="col">Kode</th>
                  <th scope="col">Nama</th>
                  <th scope="col">Jenis</th>
                  <th scope="col">Satuan</th>
                  <th scope="col">Stok</th>
                  <th scope="col">
                    <span className="visually-hidden">Pilih</span>
                  </th>
                </tr>
              </thead>
              <tbody>
                {shown.length === 0 ? (
                  <tr>
                    <td colSpan={6} className="muted">
                      Tidak ada obat.
                    </td>
                  </tr>
                ) : (
                  shown.map((o) => (
                    <tr key={o.id}>
                      <td>{o.id}</td>
                      <td>{o.nama}</td>
                      <td>{o.jenis || "-"}</td>
                      <td>{o.satuan || "-"}</td>
                      <td>{plain.format(o.stok)}</td>
                      <td>
                        <button type="button" className="row-action" aria-label={`Pilih ${o.nama}`} title="Pilih" onClick={() => onPick(o)}>
                          <SquareCheck aria-hidden="true" />
                        </button>
                      </td>
                    </tr>
                  ))
                )}
              </tbody>
            </table>
            {(rows?.length ?? 0) > shown.length && shown.length === 200 && <p className="muted">200 obat pertama ditampilkan; ketik untuk mempersempit.</p>}
          </div>
        )}
      </div>
    </dialog>
  );
}
