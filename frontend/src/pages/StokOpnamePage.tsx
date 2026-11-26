import { useCallback, useEffect, useRef, useState } from "react";
import { Filter, RotateCcw, Search, X } from "lucide-react";
import { api, ApiError, StokOpname, StokOpnameOptions, StokOpnameQuery } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { ALL_BRANCHES, BranchFilter, initialBranch, NoBranchNotice } from "../components/branchView";
import { FieldShell } from "../components/FormKit";

const EXPORT_LIMIT = 5000;
const money = new Intl.NumberFormat("id-ID", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const plain = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });

/** The "Filter" panel's fields; applied together with the search button. */
type Cari = { nama: string; jenis: string; aktif: "" | "1" | "0" };
const NO_CARI: Cari = { nama: "", jenis: "", aktif: "" };

type Notice = { kind: "info" | "error"; text: string } | null;

/**
 * Stok Farmasi > Stok Opname (legacy /StokOpname): every drug at the user's
 * branches with its sale price, units, minimum stock and current stock, by
 * drug code. Data: refobat joined with trxstock, refsatuan, refjenisobat and
 * refbranch, limited to the branches the user is assigned to.
 */
export default function StokOpnamePage() {
  const { token, logout } = useAuth();
  const [options, setOptions] = useState<StokOpnameOptions | null>(null);
  const [branch, setBranch] = useState(ALL_BRANCHES);
  const [cariOpen, setCariOpen] = useState(false);
  const [draft, setDraft] = useState<Cari>(NO_CARI);
  const [cari, setCari] = useState<Cari>(NO_CARI);
  const [page, setPage] = useState<{ rows: StokOpname[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [notice, setNotice] = useState<Notice>(null);
  const seq = useRef(0);

  const expired = useCallback(() => logout("Your session has expired. Please log in again."), [logout]);

  useEffect(() => {
    if (!token) return;
    api
      .stokOpnameOptions(token)
      .then((o) => {
        setOptions(o);
        setBranch(initialBranch(o.branches));
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const params = (q: RemoteQuery, size = q.pageSize, pageNo = q.page): StokOpnameQuery => ({
    page: pageNo,
    size,
    q: q.search,
    sort: q.sort?.key,
    dir: q.sort?.dir,
    cabang: branch === ALL_BRANCHES ? undefined : branch,
    nama: cari.nama || undefined,
    jenis: cari.jenis ? Number(cari.jenis) : undefined,
    aktif: cari.aktif || undefined,
  });

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token || !options) return;
      const mine = ++seq.current;
      setLoading(true);
      api
        .stokOpname(token, params(q))
        .then((res) => mine === seq.current && setPage(res))
        .catch((err) => {
          if (mine !== seq.current) return;
          if (err instanceof ApiError && err.status === 401) expired();
          else setNotice({ kind: "error", text: err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : "Failed to load the stock" });
        })
        .finally(() => mine === seq.current && setLoading(false));
    },
    [token, options, branch, cari], // eslint-disable-line react-hooks/exhaustive-deps
  );

  const fetchAll = async (q: RemoteQuery): Promise<StokOpname[]> => {
    if (!token) return [];
    const res = await api.stokOpname(token, params(q, EXPORT_LIMIT, 1));
    if (res.total > EXPORT_LIMIT) {
      setNotice({
        kind: "info",
        text: `The export holds the first ${EXPORT_LIMIT.toLocaleString("en-US")} of ${res.total.toLocaleString("en-US")} rows. Pick a branch or use Filter to export the rest.`,
      });
    }
    return res.rows;
  };

  const applyCari = () => setCari({ ...draft, nama: draft.nama.trim() });
  const resetCari = () => {
    setDraft(NO_CARI);
    setCari(NO_CARI);
  };

  const num = (key: "jumUnit" | "stokMin" | "stok", header: string): Column<StokOpname> => ({
    key,
    header,
    value: (r) => plain.format(r[key]),
    sortValue: (r) => r[key],
    className: "dt-num",
  });
  const columns: Column<StokOpname>[] = [
    { key: "idObat", header: "Kode", value: (r) => String(r.idObat), sortValue: (r) => r.idObat },
    { key: "nama", header: "Nama", value: (r) => r.nama, className: "dt-wrap" },
    { key: "hargaJual", header: "Harga Jual", value: (r) => money.format(r.hargaJual), sortValue: (r) => r.hargaJual, className: "dt-num" },
    { key: "beratIsi", header: "Berat Isi", value: (r) => r.beratIsi || "-" },
    { key: "jenis", header: "Jenis Obat", value: (r) => r.jenis || "-" },
    { key: "satuanBeli", header: "Satuan Beli", value: (r) => r.satuanBeli || "-" },
    { key: "satuanJual", header: "Satuan Jual", value: (r) => r.satuanJual || "-" },
    num("jumUnit", "Jum Unit"),
    num("stokMin", "Stok Min"),
    num("stok", "Stok"),
    { key: "cabang", header: "Cabang", value: (r) => r.namaCabang || r.cabang, className: "dt-wrap-sm" },
  ];

  const branches = options?.branches ?? [];
  const cariCount = (cari.nama ? 1 : 0) + (cari.jenis ? 1 : 0) + (cari.aktif ? 1 : 0);

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Stok Opname</h1>
        <div className="head-tools">
          <button type="button" className="btn-add" aria-expanded={cariOpen} aria-controls="stok-opname-filter" onClick={() => setCariOpen((o) => !o)}>
            Filter{cariCount ? ` (${cariCount})` : ""} <Filter aria-hidden="true" />
          </button>
        </div>
      </div>
      {cariOpen && options && (
        <form
          id="stok-opname-filter"
          className="search-panel"
          aria-label="Filter"
          onSubmit={(e) => {
            e.preventDefault();
            applyCari();
          }}
        >
          <h3>
            <Filter aria-hidden="true" /> Filter
          </h3>
          <div className="search-panel-row cols-3">
            <FieldShell id="so-nama" label="Nama Obat">
              <input id="so-nama" value={draft.nama} placeholder="Input nama obat" onChange={(e) => setDraft({ ...draft, nama: e.target.value })} />
            </FieldShell>
            <FieldShell id="so-jenis" label="Jenis Obat">
              <select id="so-jenis" className="select" value={draft.jenis} onChange={(e) => setDraft({ ...draft, jenis: e.target.value })}>
                <option value="">Semua jenis</option>
                {options.jenis.map((o) => (
                  <option key={o.id} value={o.id}>
                    {o.name}
                  </option>
                ))}
              </select>
            </FieldShell>
            <FieldShell id="so-aktif" label="Status">
              <select id="so-aktif" className="select" value={draft.aktif} onChange={(e) => setDraft({ ...draft, aktif: e.target.value as Cari["aktif"] })}>
                <option value="">Semua</option>
                <option value="1">Aktif</option>
                <option value="0">Tidak aktif</option>
              </select>
            </FieldShell>
            <div className="search-panel-actions">
              <button type="submit" className="icon-round" aria-label="Apply filter" title="Search">
                <Search aria-hidden="true" />
              </button>
              <button type="button" className="icon-round" aria-label="Reset filter" title="Reset" onClick={resetCari}>
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
        <NoBranchNotice what="stock" />
      ) : (
        <DataTable
          // New filters start again from page 1.
          key={`${branch}|${cari.nama}|${cari.jenis}|${cari.aktif}`}
          title="Stok Opname"
          columns={columns}
          rows={page?.rows ?? []}
          rowKey={(r) => `${r.cabang}|${r.idObat}`}
          initialSort={{ key: "idObat", dir: "asc" }}
          defaultPageSize={10}
          toolbarStart={<BranchFilter branches={branches} value={branch} onChange={setBranch} />}
          remote={{ total: page?.total ?? 0, unfilteredTotal: page?.unfilteredTotal ?? 0, loading: loading || !page, onQuery: fetchPage, fetchAll }}
        />
      )}
    </section>
  );
}
