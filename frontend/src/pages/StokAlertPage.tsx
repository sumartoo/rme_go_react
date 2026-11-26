import { KeyboardEvent, useCallback, useEffect, useRef, useState } from "react";
import { Info, X } from "lucide-react";
import { api, ApiError, StokOpname, StokOpnameOptions, StokOpnameQuery } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { ALL_BRANCHES, BranchFilter, initialBranch, NoBranchNotice } from "../components/branchView";

const EXPORT_LIMIT = 5000;
const plain = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });

type TabId = "minim" | "kadaluarsa" | "jatuh-tempo";
const TABS: { id: TabId; label: string }[] = [
  { id: "minim", label: "Obat Minim Stok" },
  { id: "kadaluarsa", label: "Obat Stok Kadaluarsa" },
  { id: "jatuh-tempo", label: "Jatuh Tempo Pembelian" },
];

/**
 * Stok Farmasi > Stok Alert (legacy /StokAlert). "Obat Minim Stok" lists the
 * drugs at the user's branches whose stock is below the minimum
 * (trxstock.stok_min > stok), by name. The other two tabs are not built yet.
 */
export default function StokAlertPage() {
  const { token, logout } = useAuth();
  const [tab, setTab] = useState<TabId>("minim");
  const [options, setOptions] = useState<StokOpnameOptions | null>(null);
  const [loadError, setLoadError] = useState("");

  useEffect(() => {
    if (!token) return;
    api
      .stokAlertOptions(token)
      .then(setOptions)
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const onTabKey = (e: KeyboardEvent<HTMLButtonElement>) => {
    const i = TABS.findIndex((t) => t.id === tab);
    const next = e.key === "ArrowRight" ? (i + 1) % TABS.length : e.key === "ArrowLeft" ? (i + TABS.length - 1) % TABS.length : -1;
    if (next < 0) return;
    e.preventDefault();
    setTab(TABS[next].id);
    document.getElementById(`stok-alert-tab-${TABS[next].id}`)?.focus();
  };

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Stok Alert</h1>
      </div>
      <div className="tabs" role="tablist" aria-label="Stok Alert">
        {TABS.map((t) => (
          <button
            key={t.id}
            id={`stok-alert-tab-${t.id}`}
            type="button"
            role="tab"
            aria-selected={tab === t.id}
            aria-controls={`stok-alert-panel-${t.id}`}
            tabIndex={tab === t.id ? 0 : -1}
            className={tab === t.id ? "tab active" : "tab"}
            onClick={() => setTab(t.id)}
            onKeyDown={onTabKey}
          >
            {t.label}
          </button>
        ))}
      </div>
      <div id={`stok-alert-panel-${tab}`} role="tabpanel" aria-labelledby={`stok-alert-tab-${tab}`} className="tab-panel">
        {loadError ? (
          <p className="alert error" role="alert">
            {loadError}
          </p>
        ) : !options ? (
          <p className="muted">Loading…</p>
        ) : options.branches.length === 0 ? (
          <NoBranchNotice what="stock" />
        ) : tab === "minim" ? (
          <MinimStokTab options={options} />
        ) : (
          <p className="alert info" role="note">
            <Info aria-hidden="true" /> Tab {TABS.find((t) => t.id === tab)?.label} belum tersedia.
          </p>
        )}
      </div>
    </section>
  );
}

function MinimStokTab({ options }: { options: StokOpnameOptions }) {
  const { token, logout } = useAuth();
  const [branch, setBranch] = useState(() => initialBranch(options.branches));
  const [page, setPage] = useState<{ rows: StokOpname[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [notice, setNotice] = useState<{ kind: "info" | "error"; text: string } | null>(null);
  const seq = useRef(0);

  const expired = useCallback(() => logout("Your session has expired. Please log in again."), [logout]);

  const params = (q: RemoteQuery, size = q.pageSize, pageNo = q.page): StokOpnameQuery => ({
    page: pageNo,
    size,
    q: q.search,
    sort: q.sort?.key,
    dir: q.sort?.dir,
    cabang: branch === ALL_BRANCHES ? undefined : branch,
  });

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token) return;
      const mine = ++seq.current;
      setLoading(true);
      api
        .minimStok(token, params(q))
        .then((res) => mine === seq.current && setPage(res))
        .catch((err) => {
          if (mine !== seq.current) return;
          if (err instanceof ApiError && err.status === 401) expired();
          else setNotice({ kind: "error", text: err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : "Failed to load the stock" });
        })
        .finally(() => mine === seq.current && setLoading(false));
    },
    [token, branch], // eslint-disable-line react-hooks/exhaustive-deps
  );

  const fetchAll = async (q: RemoteQuery): Promise<StokOpname[]> => {
    if (!token) return [];
    const res = await api.minimStok(token, params(q, EXPORT_LIMIT, 1));
    if (res.total > EXPORT_LIMIT) {
      setNotice({
        kind: "info",
        text: `The export holds the first ${EXPORT_LIMIT.toLocaleString("en-US")} of ${res.total.toLocaleString("en-US")} rows. Pick a branch to export the rest.`,
      });
    }
    return res.rows;
  };

  const num = (key: "stokMin" | "stok" | "jumUnit", header: string): Column<StokOpname> => ({
    key,
    header,
    value: (r) => plain.format(r[key]),
    sortValue: (r) => r[key],
  });
  const columns: Column<StokOpname>[] = [
    { key: "idObat", header: "Kode", value: (r) => String(r.idObat), sortValue: (r) => r.idObat },
    { key: "nama", header: "Nama Obat", value: (r) => r.nama, className: "dt-wrap" },
    { key: "beratIsi", header: "Berat Isi", value: (r) => r.beratIsi || "-" },
    { key: "jenis", header: "Jenis Obat", value: (r) => r.jenis || "-" },
    { key: "satuanBeli", header: "Satuan Beli", value: (r) => r.satuanBeli || "-" },
    num("stokMin", "Stok Min"),
    num("stok", "Stok"),
    num("jumUnit", "Qjb"),
    { key: "cabang", header: "Cabang", value: (r) => r.namaCabang || r.cabang, className: "dt-wrap-sm" },
  ];

  return (
    <>
      {notice && (
        <p className={`alert ${notice.kind}`} role={notice.kind === "error" ? "alert" : "status"}>
          {notice.text}
          <button type="button" className="row-action alert-close" aria-label="Dismiss" onClick={() => setNotice(null)}>
            <X aria-hidden="true" />
          </button>
        </p>
      )}
      <DataTable
        key={branch}
        title="Obat Minim Stok"
        columns={columns}
        rows={page?.rows ?? []}
        rowKey={(r) => `${r.cabang}|${r.idObat}`}
        initialSort={{ key: "nama", dir: "asc" }}
        defaultPageSize={10}
        toolbarStart={<BranchFilter branches={options.branches} value={branch} onChange={setBranch} />}
        remote={{ total: page?.total ?? 0, unfilteredTotal: page?.unfilteredTotal ?? 0, loading: loading || !page, onQuery: fetchPage, fetchAll }}
      />
    </>
  );
}
