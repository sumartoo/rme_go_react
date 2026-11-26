import { FormEvent, useCallback, useEffect, useRef, useState } from "react";
import { Link, useLocation, useNavigate } from "react-router-dom";
import { Ban, CalendarDays, Pencil, Plus, X } from "lucide-react";
import { api, ApiError, McuDaftarOptions, McuDaftarQuery, McuDaftarRow } from "../../api";
import { useAuth } from "../../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../../components/DataTable";
import { useModal } from "../../components/FormKit";
import { addDays, fmtDate } from "../../components/dates";

const EXPORT_LIMIT = 5000;
export const MCU_DAFTAR_PATH = "/mcu/pendaftaran";
const money = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });

type Notice = { kind: "info" | "error" | "success"; text: string } | null;

/**
 * MCU > Pendaftaran (legacy /Pendaftaran): the MCU registrations, newest MCU
 * date first, over the last 30 days by default. Add opens the form (several
 * patients at once); Edit opens a registration (read-only once it is no longer
 * New); Cancel cancels a New registration with a reason.
 */
export default function McuPendaftaranPage() {
  const { token, logout } = useAuth();
  const [options, setOptions] = useState<McuDaftarOptions | null>(null);
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [status, setStatus] = useState("");
  const [page, setPage] = useState<{ rows: McuDaftarRow[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  // The form sends a success message back after Add.
  const location = useLocation();
  const [notice, setNotice] = useState<Notice>(() => {
    const text = (location.state as { notice?: string } | null)?.notice;
    return text ? { kind: "success", text } : null;
  });
  const [cancelRow, setCancelRow] = useState<McuDaftarRow | null>(null);
  const [reload, setReload] = useState(0);
  const seq = useRef(0);
  const navigate = useNavigate();

  const expired = useCallback(() => logout("Your session has expired. Please log in again."), [logout]);

  useEffect(() => {
    if (!token) return;
    api
      .mcuDaftarOptions(token)
      .then((o) => {
        setOptions(o);
        setFrom(addDays(o.today, -29));
        setTo(o.today);
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const params = (q: RemoteQuery, size = q.pageSize, pageNo = q.page): McuDaftarQuery => ({
    from,
    to,
    page: pageNo,
    size,
    q: q.search,
    sort: q.sort?.key,
    dir: q.sort?.dir,
    status: status || undefined,
  });

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token || !from) return;
      const mine = ++seq.current;
      setLoading(true);
      api
        .mcuDaftarList(token, params(q))
        .then((res) => mine === seq.current && setPage(res))
        .catch((err) => {
          if (mine !== seq.current) return;
          if (err instanceof ApiError && err.status === 401) expired();
          else setNotice({ kind: "error", text: err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : "Failed to load registrations" });
        })
        .finally(() => mine === seq.current && setLoading(false));
    },
    [token, from, to, status, reload], // eslint-disable-line react-hooks/exhaustive-deps
  );

  const fetchAll = async (q: RemoteQuery): Promise<McuDaftarRow[]> => {
    if (!token) return [];
    const res = await api.mcuDaftarList(token, params(q, EXPORT_LIMIT, 1));
    if (res.total > EXPORT_LIMIT) {
      setNotice({ kind: "info", text: `The export holds the first ${EXPORT_LIMIT.toLocaleString("en-US")} of ${res.total.toLocaleString("en-US")} registrations. Narrow the dates to export the rest.` });
    }
    return res.rows;
  };

  const columns: Column<McuDaftarRow>[] = [
    { key: "id", header: "ID", value: (r) => String(r.id), sortValue: (r) => r.id, hidden: true },
    { key: "mcuDate", header: "MCU Date", value: (r) => fmtDate(r.mcuDate), sortValue: (r) => r.mcuDate },
    { key: "rm", header: "No RM", value: (r) => r.rm || "-" },
    { key: "status", header: "Status", value: (r) => r.status || "-" },
    { key: "nama", header: "Name", value: (r) => r.nama || "-", className: "dt-wrap-sm" },
    { key: "perusahaan", header: "Perusahaan", value: (r) => r.perusahaan || "-", className: "dt-wrap-sm" },
    { key: "kategori", header: "Kategori", value: (r) => r.kategori || "-", hidden: true },
    { key: "price", header: "Price", value: (r) => money.format(r.price), sortValue: (r) => r.price, className: "dt-num", hidden: true },
    { key: "createBy", header: "Dibuat Oleh", value: (r) => r.createBy || "-", hidden: true },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (r) => (
        <span className="row-actions">
          <button type="button" className="row-action" aria-label={`Edit pendaftaran ${r.id}`} title={r.status === "New" ? "Edit" : "Lihat"} onClick={() => navigate(`${MCU_DAFTAR_PATH}/edit/${r.id}`)}>
            <Pencil aria-hidden="true" />
          </button>
          {r.status === "New" && (
            <button type="button" className="row-action danger" aria-label={`Cancel pendaftaran ${r.id}`} title="Cancel" onClick={() => setCancelRow(r)}>
              <Ban aria-hidden="true" />
            </button>
          )}
        </span>
      ),
    },
  ];

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Pendaftaran MCU</h1>
        <div className="head-tools">
          <label className="date-range" title="MCU date">
            <CalendarDays aria-hidden="true" />
            <input type="date" aria-label="Dari tanggal" value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} />
            <span aria-hidden="true">–</span>
            <input type="date" aria-label="Sampai tanggal" value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} />
          </label>
          <select className="select" aria-label="Status" value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="">Semua status</option>
            {(options?.status ?? []).map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
          <Link to={`${MCU_DAFTAR_PATH}/new`} className="btn-add btn-link">
            <Plus aria-hidden="true" /> Add
          </Link>
        </div>
      </div>
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
      ) : (
        <DataTable
          key={`${from}|${to}|${status}|${reload}`}
          title={`Pendaftaran MCU ${from}${to !== from ? " - " + to : ""}`}
          columns={columns}
          rows={page?.rows ?? []}
          rowKey={(r) => r.id}
          initialSort={{ key: "mcuDate", dir: "desc" }}
          defaultPageSize={10}
          remote={{ total: page?.total ?? 0, unfilteredTotal: page?.unfilteredTotal ?? 0, loading: loading || !page, onQuery: fetchPage, fetchAll }}
        />
      )}
      {cancelRow && (
        <CancelMcuDialog
          row={cancelRow}
          onDone={(msg) => {
            setCancelRow(null);
            setNotice({ kind: "success", text: msg });
            setReload((n) => n + 1);
          }}
          onClose={() => setCancelRow(null)}
        />
      )}
    </section>
  );
}

/** "Cancel Pendaftaran": a required reason, then the registration becomes Cancel. */
function CancelMcuDialog({ row, onDone, onClose }: { row: McuDaftarRow; onDone: (msg: string) => void; onClose: () => void }) {
  const { token, logout } = useAuth();
  const dialogRef = useModal();
  const [alasan, setAlasan] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    if (!alasan.trim()) {
      setError("Isi Alasan dulu.");
      return;
    }
    setBusy(true);
    setError("");
    try {
      await api.mcuDaftarCancel(token, row.id, alasan.trim());
      onDone(`Pendaftaran MCU ${row.id} (${row.nama}) sudah di-cancel.`);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) return logout("Your session has expired. Please log in again.");
      setError(err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : "Gagal cancel pendaftaran");
    } finally {
      setBusy(false);
    }
  }

  return (
    <dialog ref={dialogRef} className="modal cancel-dialog" aria-labelledby="mcu-cancel-title" onClose={onClose}>
      <form onSubmit={submit} noValidate>
        <header className="modal-head">
          <h2 id="mcu-cancel-title">Cancel Pendaftaran MCU</h2>
          <button type="button" className="row-action" aria-label="Close" onClick={onClose}>
            <X aria-hidden="true" />
          </button>
        </header>
        <div className="modal-body">
          <p>
            Apakah pendaftaran <strong>{row.id}</strong> ({row.nama}) akan di cancel?
          </p>
          <div className="cancel-row">
            <label htmlFor="mcu-cancel-alasan">Alasan</label>
            <input id="mcu-cancel-alasan" maxLength={300} placeholder="Masukan Alasan" autoFocus value={alasan} onChange={(e) => setAlasan(e.target.value)} />
          </div>
          {error && (
            <p className="alert error" role="alert">
              {error}
            </p>
          )}
        </div>
        <footer className="modal-foot">
          <button type="button" className="btn-small" onClick={onClose}>
            Cancel
          </button>
          <button type="submit" className="btn-small btn-primary" disabled={busy}>
            {busy ? "Memproses…" : "Yes"}
          </button>
        </footer>
      </form>
    </dialog>
  );
}
