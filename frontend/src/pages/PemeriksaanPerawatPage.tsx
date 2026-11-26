import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { useNavigate } from "react-router-dom";
import { CalendarDays, Eye, Pencil, X } from "lucide-react";
import { api, ApiError, DokterPeriksaOptions, PerawatQuery, PerawatRow } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { ALL_BRANCHES, BranchFilter, initialBranch, NoBranchNotice } from "../components/branchView";
import { addDays, age, fmtDate } from "../components/dates";

const EXPORT_LIMIT = 5000;
export const PERAWAT_PATH = "/transaksi-klinik/pemeriksaan-perawat";
export const DOKTER_PATH = "/transaksi-klinik/pemeriksaan-dokter";
export const PROSES_OBAT_PATH = "/transaksi-klinik/proses-obat";
/** Visit status PAYMENT (refstatustransaksi): Proses Obat starts filtered on it. */
const PAYMENT = "5";

type Notice = { kind: "info" | "error"; text: string } | null;

/**
 * Transaksi Klinik > Pemeriksaan Perawat (legacy /TransPerawat): the poli
 * registrations at the user's branches, newest first. Edit opens the nurse's
 * examination, where the Dokter PJ is chosen.
 *
 * With `dokter` it is Transaksi Klinik > Pemeriksaan Dokter (legacy
 * /TransDokter): the same list with a Dokter PJ filter and the birth date; Edit
 * opens the examination with the doctor's prescription and diagnoses.
 *
 * With `prosesObat` it is Transaksi Klinik > Proses Obat (legacy /ProsesObat):
 * the registrations waiting for their drugs (PAYMENT by default), each opened
 * read-only with its examination, prescription and diagnoses.
 */
export default function PemeriksaanPerawatPage({ dokter: dokterMode = false, prosesObat = false }: { dokter?: boolean; prosesObat?: boolean }) {
  const { token, logout } = useAuth();
  const [options, setOptions] = useState<(Omit<DokterPeriksaOptions, "dokter"> & Partial<Pick<DokterPeriksaOptions, "dokter">>) | null>(null);
  const [dokter, setDokter] = useState("");
  const basePath = prosesObat ? PROSES_OBAT_PATH : dokterMode ? DOKTER_PATH : PERAWAT_PATH;
  const title = prosesObat ? "Proses Obat" : dokterMode ? "Pemeriksaan Dokter" : "Pemeriksaan Perawat";
  const list = prosesObat ? api.prosesObatList : dokterMode ? api.dokterPeriksaList : api.perawatList;
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [branch, setBranch] = useState(ALL_BRANCHES);
  const [status, setStatus] = useState(prosesObat ? PAYMENT : "");
  const [page, setPage] = useState<{ rows: PerawatRow[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [notice, setNotice] = useState<Notice>(null);
  const seq = useRef(0);
  const navigate = useNavigate();

  const expired = useCallback(() => logout("Your session has expired. Please log in again."), [logout]);

  useEffect(() => {
    if (!token) return;
    (prosesObat ? api.prosesObatOptions(token) : dokterMode ? api.dokterPeriksaOptions(token) : api.perawatOptions(token))
      .then((o) => {
        setOptions(o);
        // Default: yesterday and today (H-1 to H), like the legacy screen.
        setFrom(addDays(o.today, -1));
        setTo(o.today);
        setBranch(initialBranch(o.branches));
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const branchName = useMemo(() => new Map((options?.branches ?? []).map((b) => [b.kode, b.nama])), [options]);
  const params = (q: RemoteQuery, size = q.pageSize, pageNo = q.page): PerawatQuery => ({
    from,
    to,
    page: pageNo,
    size,
    q: q.search,
    sort: q.sort?.key,
    dir: q.sort?.dir,
    cabang: branch === ALL_BRANCHES ? undefined : branch,
    status: status === "" ? undefined : Number(status),
    dokter: dokter === "" ? undefined : Number(dokter),
  });

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token || !from) return;
      const mine = ++seq.current;
      setLoading(true);
      list(token, params(q))
        .then((res) => mine === seq.current && setPage(res))
        .catch((err) => {
          if (mine !== seq.current) return;
          if (err instanceof ApiError && err.status === 401) expired();
          else setNotice({ kind: "error", text: err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : "Failed to load registrations" });
        })
        .finally(() => mine === seq.current && setLoading(false));
    },
    [token, from, to, branch, status, dokter], // eslint-disable-line react-hooks/exhaustive-deps
  );

  const fetchAll = async (q: RemoteQuery): Promise<PerawatRow[]> => {
    if (!token) return [];
    const res = await list(token, params(q, EXPORT_LIMIT, 1));
    if (res.total > EXPORT_LIMIT) {
      setNotice({ kind: "info", text: `The export holds the first ${EXPORT_LIMIT.toLocaleString("en-US")} of ${res.total.toLocaleString("en-US")} registrations. Narrow the dates to export the rest.` });
    }
    return res.rows;
  };

  const columns: Column<PerawatRow>[] = [
    ...(prosesObat ? [] : [{ key: "noAntrianPoli", header: "No Antrian Poli", value: (r: PerawatRow) => r.noAntrianPoli || "-" }]),
    { key: "tglMasuk", header: "Tanggal Waktu", value: (r) => r.tglMasuk ?? "-", className: "dt-wrap-sm" },
    { key: "pelayanan", header: "Pelayanan", value: (r) => r.pelayanan || "-" },
    { key: "dokter", header: "Dokter PJ", value: (r) => r.dokter || "-", className: "dt-wrap-sm" },
    { key: "noRm", header: "No RM", value: (r) => r.noRm || "-" },
    { key: "nama", header: "Pasien", value: (r) => r.nama || "-", className: "dt-wrap-sm" },
    { key: "umur", header: "Usia", value: (r) => String(age(r.tglLahir) ?? "-") },
    ...(dokterMode ? [{ key: "tglLahir", header: "Tanggal Lahir", value: (r: PerawatRow) => fmtDate(r.tglLahir), sortable: false }] : []),
    { key: "alamat", header: "Alamat", value: (r) => r.alamat || "-", className: "dt-wrap-sm" },
    ...(prosesObat ? [] : [{ key: "keluhanUtama", header: "Keluhan Utama", value: (r: PerawatRow) => r.keluhanUtama || "-", className: "dt-wrap-sm" }]),
    { key: "cabang", header: "Cabang", value: (r) => branchName.get(r.cabang) ?? r.cabang, className: "dt-wrap-sm" },
    { key: "status", header: "Status", value: (r) => r.status || "-" },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (r) => (
        <span className="row-actions">
          <button
            type="button"
            className="row-action"
            aria-label={`${prosesObat ? "View" : "Edit"} pemeriksaan ${r.id}`}
            title={prosesObat ? "View" : "Edit"}
            onClick={() => navigate(`${basePath}/${prosesObat ? "view" : "edit"}/${r.id}`)}
          >
            {prosesObat ? <Eye aria-hidden="true" /> : <Pencil aria-hidden="true" />}
          </button>
        </span>
      ),
    },
  ];

  const branches = options?.branches ?? [];

  return (
    <section className="panel">
      <div className="page-head">
        <h1>{title}</h1>
        <div className="head-tools">
          <label className="date-range" title="Tanggal daftar">
            <CalendarDays aria-hidden="true" />
            <input type="date" aria-label="Dari tanggal" value={from} max={to || undefined} onChange={(e) => setFrom(e.target.value)} />
            <span aria-hidden="true">–</span>
            <input type="date" aria-label="Sampai tanggal" value={to} min={from || undefined} onChange={(e) => setTo(e.target.value)} />
          </label>
          {dokterMode && (
            <select className="select" aria-label="Dokter" value={dokter} onChange={(e) => setDokter(e.target.value)}>
              <option value="">Semua dokter</option>
              {(options?.dokter ?? []).map((d) => (
                <option key={d.id} value={d.id}>
                  {d.nama}
                </option>
              ))}
            </select>
          )}
          <select className="select" aria-label="Status" value={status} onChange={(e) => setStatus(e.target.value)}>
            <option value="">Semua status</option>
            {(options?.status ?? []).map((o) => (
              <option key={o.id} value={o.id}>
                {o.name}
              </option>
            ))}
          </select>
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
      ) : branches.length === 0 ? (
        <NoBranchNotice what="registrations" />
      ) : (
        <DataTable
          // New filters start again from page 1.
          key={`${from}|${to}|${branch}|${status}|${dokter}`}
          title={`${title} ${from}${to !== from ? " - " + to : ""}`}
          columns={columns}
          rows={page?.rows ?? []}
          rowKey={(r) => r.id}
          initialSort={{ key: "tglMasuk", dir: "desc" }}
          defaultPageSize={10}
          toolbarStart={<BranchFilter branches={branches} value={branch} onChange={setBranch} />}
          remote={{ total: page?.total ?? 0, unfilteredTotal: page?.unfilteredTotal ?? 0, loading: loading || !page, onQuery: fetchPage, fetchAll }}
        />
      )}
    </section>
  );
}
