import { FormEvent, KeyboardEvent, MouseEvent, ReactNode, useEffect, useMemo, useRef, useState } from "react";
import { AlertTriangle, Bot, CheckCircle2, CircleSlash, Info, OctagonAlert, RefreshCw, ShoppingCart, TrendingUp } from "lucide-react";
import {
  AnalitikDiagnosaObatResult,
  AnalitikIcdTrend,
  AnalitikObat,
  AnalitikObatResult,
  AnalitikOptions,
  AnalitikPenyakit,
  AnalitikStatus,
  api,
} from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { ALL_BRANCHES, BranchFilter, initialBranch, NoBranchNotice } from "../components/branchView";
import { CommandTab, FollowUpTab, KualitasTab } from "./AnalitikOps";
import { AiDisabledNotice, AiText, ChatBox, num, rupiah, useFail } from "./analitikShared";

const pct = new Intl.NumberFormat("id-ID", { style: "percent", maximumFractionDigits: 0 });

type TabId = "command" | "obat" | "penyakit" | "diagnosa" | "kualitas" | "followup" | "asisten";
const TABS: { id: TabId; label: string }[] = [
  { id: "command", label: "Command Center" },
  { id: "obat", label: "Prediksi Obat" },
  { id: "penyakit", label: "Tren Penyakit" },
  { id: "diagnosa", label: "Diagnosa → Obat" },
  { id: "kualitas", label: "Kualitas Data" },
  { id: "followup", label: "Follow-up Pasien" },
  { id: "asisten", label: "Asisten AI" },
];

const STATUS: Record<AnalitikStatus, { label: string; icon: ReactNode }> = {
  habis: { label: "Habis", icon: <OctagonAlert aria-hidden="true" /> },
  kritis: { label: "Kritis", icon: <AlertTriangle aria-hidden="true" /> },
  "perlu-order": { label: "Perlu order", icon: <ShoppingCart aria-hidden="true" /> },
  aman: { label: "Aman", icon: <CheckCircle2 aria-hidden="true" /> },
  "tidak-bergerak": { label: "Tidak bergerak", icon: <CircleSlash aria-hidden="true" /> },
};
const STATUS_ORDER: AnalitikStatus[] = ["habis", "kritis", "perlu-order", "aman", "tidak-bergerak"];

function StatusBadge({ status }: { status: AnalitikStatus }) {
  return (
    <span className={`an-status an-${status}`}>
      {STATUS[status].icon}
      {STATUS[status].label}
    </span>
  );
}

/**
 * Stok Farmasi > Analitik: drug usage forecasts, weekly disease trends, the
 * drugs prescribed per diagnosis and an optional AI assistant (DeepSeek). The
 * numbers are computed by the backend; the assistant only explains them.
 */
export default function AnalitikPage() {
  const { token } = useAuth();
  const fail = useFail();
  const [tab, setTab] = useState<TabId>("command");
  const [options, setOptions] = useState<AnalitikOptions | null>(null);
  const [branch, setBranch] = useState(ALL_BRANCHES);
  const [loadError, setLoadError] = useState("");

  useEffect(() => {
    if (!token) return;
    api
      .analitikOptions(token)
      .then((o) => {
        setOptions(o);
        setBranch(initialBranch(o.branches));
      })
      .catch((err) => setLoadError(fail(err, "Failed to load")));
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const onTabKey = (e: KeyboardEvent<HTMLButtonElement>) => {
    const i = TABS.findIndex((t) => t.id === tab);
    const next = e.key === "ArrowRight" ? (i + 1) % TABS.length : e.key === "ArrowLeft" ? (i + TABS.length - 1) % TABS.length : -1;
    if (next < 0) return;
    e.preventDefault();
    setTab(TABS[next].id);
    document.getElementById(`analitik-tab-${TABS[next].id}`)?.focus();
  };

  const cabang = branch === ALL_BRANCHES ? undefined : branch;

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Analitik Stok &amp; Penyakit</h1>
        {options && <BranchFilter branches={options.branches} value={branch} onChange={setBranch} />}
      </div>
      <p className="alert info an-note" role="note">
        <Info aria-hidden="true" /> Prediksi dihitung dari riwayat {options?.historyDays ?? 90} hari terakhir dengan metode sederhana, jadi angka ini
        bersifat indikatif dan belum memperhitungkan pola musiman.
      </p>
      <div className="tabs" role="tablist" aria-label="Analitik">
        {TABS.map((t) => (
          <button
            key={t.id}
            id={`analitik-tab-${t.id}`}
            type="button"
            role="tab"
            aria-selected={tab === t.id}
            aria-controls={`analitik-panel-${t.id}`}
            tabIndex={tab === t.id ? 0 : -1}
            className={tab === t.id ? "tab active" : "tab"}
            onClick={() => setTab(t.id)}
            onKeyDown={onTabKey}
          >
            {t.label}
          </button>
        ))}
      </div>
      <div id={`analitik-panel-${tab}`} role="tabpanel" aria-labelledby={`analitik-tab-${tab}`} className="tab-panel">
        {loadError ? (
          <p className="alert error" role="alert">
            {loadError}
          </p>
        ) : !options ? (
          <p className="muted">Loading…</p>
        ) : options.branches.length === 0 ? (
          <NoBranchNotice what="analytics" />
        ) : tab === "command" ? (
          <CommandTab key={branch} cabang={cabang} options={options} />
        ) : tab === "kualitas" ? (
          <KualitasTab key={branch} cabang={cabang} />
        ) : tab === "followup" ? (
          <FollowUpTab key={branch} cabang={cabang} />
        ) : tab === "obat" ? (
          <ObatTab key={branch} cabang={cabang} options={options} />
        ) : tab === "penyakit" ? (
          <PenyakitTab key={branch} cabang={cabang} options={options} />
        ) : tab === "diagnosa" ? (
          <DiagnosaTab key={branch} cabang={cabang} />
        ) : (
          <AsistenTab key={branch} cabang={cabang} options={options} />
        )}
      </div>
    </section>
  );
}

// ---------- Prediksi Obat ----------

function ObatTab({ cabang, options }: { cabang?: string; options: AnalitikOptions }) {
  const { token } = useAuth();
  const fail = useFail();
  const [horizon, setHorizon] = useState(options.horizon);
  const [leadTime, setLeadTime] = useState(options.leadTime);
  const [status, setStatus] = useState<AnalitikStatus | "">("");
  const [res, setRes] = useState<AnalitikObatResult | null>(null);
  const [error, setError] = useState("");
  const seq = useRef(0);

  useEffect(() => {
    if (!token) return;
    const mine = ++seq.current;
    setError("");
    api
      .analitikObat(token, { cabang, horizon, leadTime })
      .then((r) => mine === seq.current && setRes(r))
      .catch((err) => mine === seq.current && setError(fail(err, "Gagal memuat prediksi")));
  }, [token, cabang, horizon, leadTime, fail]);

  const rows = useMemo(() => (res ? (status ? res.rows.filter((r) => r.status === status) : res.rows) : []), [res, status]);
  const n = (key: keyof AnalitikObat, header: string, unit = false): Column<AnalitikObat> => ({
    key,
    header,
    value: (r) => num.format(r[key] as number) + (unit && r.satuan ? ` ${r.satuan}` : ""),
    sortValue: (r) => r[key] as number,
    className: "num",
  });
  const columns: Column<AnalitikObat>[] = [
    { key: "nama", header: "Obat", value: (r) => r.nama, className: "dt-wrap" },
    { key: "cabang", header: "Cabang", value: (r) => r.cabang },
    n("stok", "Stok", true),
    n("avgHarian", "Pakai/hari"),
    {
      key: "habisDalamHari",
      header: "Habis dalam",
      value: (r) => (r.habisDalamHari === null ? "-" : `${num.format(r.habisDalamHari)} hari`),
      sortValue: (r) => r.habisDalamHari ?? Number.POSITIVE_INFINITY,
      className: "num",
    },
    n("kebutuhan", `Kebutuhan ${horizon} hr`),
    n("safetyStock", "Safety stock"),
    n("rop", "Reorder point"),
    n("saranOrder", "Saran order", true),
    {
      key: "estimasiBiaya",
      header: "Est. biaya",
      value: (r) => (r.estimasiBiaya === null ? (r.saranOrder > 0 ? "harga ?" : "-") : rupiah.format(r.estimasiBiaya)),
      sortValue: (r) => r.estimasiBiaya ?? -1,
      className: "num",
    },
    {
      key: "status",
      header: "Status",
      value: (r) => STATUS[r.status].label + (r.lowData ? " (data < 30 hari)" : ""),
      render: (r) => (
        <>
          <StatusBadge status={r.status} />
          {r.lowData && <span className="muted an-low"> data &lt; 30 hari</span>}
        </>
      ),
      sortValue: (r) => STATUS_ORDER.indexOf(r.status),
    },
  ];

  return (
    <>
      <div className="an-controls">
        <label className="dt-length">
          <span>Horizon</span>
          <select className="select" value={horizon} onChange={(e) => setHorizon(Number(e.target.value))}>
            {[14, 30, 60, 90].map((d) => (
              <option key={d} value={d}>
                {d} hari
              </option>
            ))}
          </select>
        </label>
        <label className="dt-length">
          <span>Lead time order</span>
          <select className="select" value={leadTime} onChange={(e) => setLeadTime(Number(e.target.value))}>
            {[3, 5, 7, 10, 14, 21, 30].map((d) => (
              <option key={d} value={d}>
                {d} hari
              </option>
            ))}
          </select>
        </label>
      </div>
      {error && (
        <p className="alert error" role="alert">
          {error}
        </p>
      )}
      {res && (
        <div className="an-tiles" role="group" aria-label="Filter status">
          {STATUS_ORDER.map((s) => (
            <button
              key={s}
              type="button"
              className={`an-tile an-${s}${status === s ? " active" : ""}`}
              aria-pressed={status === s}
              onClick={() => setStatus(status === s ? "" : s)}
            >
              <span className="an-tile-value">{(res.ringkas[s] ?? 0).toLocaleString("id-ID")}</span>
              <span className="an-tile-label">
                {STATUS[s].icon}
                {STATUS[s].label}
              </span>
            </button>
          ))}
        </div>
      )}
      {res && (
        <p className="an-cost" role="status">
          Perkiraan biaya order (habis, kritis, perlu order): <strong>{rupiah.format(res.biayaOrder)}</strong>
          {res.orderTanpaHarga > 0 && (
            <span className="muted"> · {num.format(res.orderTanpaHarga)} obat belum dihitung karena harga pokoknya kosong atau tidak wajar</span>
          )}
        </p>
      )}
      {!res && !error ? (
        <p className="muted">Menghitung prediksi…</p>
      ) : (
        res && (
          <DataTable
            title={`Prediksi obat (${res.from} s/d ${res.to})`}
            columns={columns}
            rows={rows}
            rowKey={(r) => `${r.cabang}|${r.idObat}`}
            defaultPageSize={25}
          />
        )
      )}
      <p className="muted an-foot">
        Pakai/hari = rata-rata tertimbang pemakaian (penjualan dikurangi pembatalan), hari terbaru lebih berbobot. Safety stock = 1,65 ×
        simpangan harian × √lead time. Reorder point = pakai/hari × lead time + safety stock (pesan ulang saat stok turun ke angka ini). Saran
        order = kebutuhan + safety stock − stok. Est. biaya = saran order × harga pokok cabang.
      </p>
    </>
  );
}

// ---------- Tren Penyakit ----------

/** Validated categorical slots 1–5 (light / dark), assigned in fixed order. */
const SERIES = ["var(--an-s1)", "var(--an-s2)", "var(--an-s3)", "var(--an-s4)", "var(--an-s5)"];

function shortDate(d: string) {
  const [, m, day] = d.split("-");
  return `${Number(day)}/${Number(m)}`;
}

function TrendChart({ data }: { data: AnalitikPenyakit }) {
  const series = data.top.slice(0, 5);
  const [hover, setHover] = useState<number | null>(null);
  const W = 640;
  const H = 240;
  const pad = { l: 36, r: 12, t: 12, b: 28 };
  const n = data.weekStarts.length;
  const max = Math.max(1, ...series.flatMap((s) => s.weeks));
  const step = Math.max(1, Math.ceil(max / 4));
  const top = step * 4;
  const x = (i: number) => pad.l + (n <= 1 ? 0 : (i * (W - pad.l - pad.r)) / (n - 1));
  const y = (v: number) => pad.t + (H - pad.t - pad.b) * (1 - v / top);
  if (series.length === 0) return <p className="muted">Belum ada diagnosa pada periode ini.</p>;

  const onMove = (e: MouseEvent<SVGSVGElement>) => {
    const r = e.currentTarget.getBoundingClientRect();
    const px = ((e.clientX - r.left) / r.width) * W;
    const i = Math.round(((px - pad.l) / (W - pad.l - pad.r)) * (n - 1));
    setHover(Math.min(n - 1, Math.max(0, i)));
  };

  return (
    <figure className="an-chart">
      <figcaption className="an-chart-title">Kunjungan per minggu, 5 diagnosa teratas</figcaption>
      <ul className="an-legend">
        {series.map((s, i) => (
          <li key={s.idIcd}>
            <span className="an-swatch" style={{ background: SERIES[i] }} aria-hidden="true" />
            {s.kode || s.nama} <span className="muted">{s.nama !== s.kode ? s.nama : ""}</span>
          </li>
        ))}
      </ul>
      <div className="an-plot">
        <svg viewBox={`0 0 ${W} ${H}`} role="img" aria-label="Grafik kunjungan per minggu" onMouseMove={onMove} onMouseLeave={() => setHover(null)}>
          {[0, 1, 2, 3, 4].map((k) => (
            <g key={k}>
              <line x1={pad.l} x2={W - pad.r} y1={y(k * step)} y2={y(k * step)} className={k === 0 ? "an-axis" : "an-grid"} />
              <text x={pad.l - 6} y={y(k * step)} className="an-tick" textAnchor="end" dominantBaseline="middle">
                {k * step}
              </text>
            </g>
          ))}
          {data.weekStarts.map((d, i) =>
            i % Math.ceil(n / 8) === 0 || i === n - 1 ? (
              <text key={d} x={x(i)} y={H - 8} className="an-tick" textAnchor="middle">
                {shortDate(d)}
              </text>
            ) : null,
          )}
          {hover !== null && <line x1={x(hover)} x2={x(hover)} y1={pad.t} y2={H - pad.b} className="an-cross" />}
          {series.map((s, si) => (
            <polyline
              key={s.idIcd}
              fill="none"
              stroke={SERIES[si]}
              strokeWidth={2}
              strokeLinejoin="round"
              strokeLinecap="round"
              points={s.weeks.map((v, i) => `${x(i)},${y(v)}`).join(" ")}
            />
          ))}
          {hover !== null &&
            series.map((s, si) => <circle key={s.idIcd} cx={x(hover)} cy={y(s.weeks[hover])} r={4} fill={SERIES[si]} className="an-dot" />)}
        </svg>
        {hover !== null && (
          <div className="an-tooltip" style={{ left: `${(x(hover) / W) * 100}%` }} role="status">
            <strong>Minggu {shortDate(data.weekStarts[hover])}</strong>
            {series
              .map((s, si) => ({ s, si, v: s.weeks[hover] }))
              .sort((a, b) => b.v - a.v)
              .map(({ s, si, v }) => (
                <span key={s.idIcd} className="an-tip-row">
                  <span className="an-swatch" style={{ background: SERIES[si] }} aria-hidden="true" />
                  {s.kode || s.nama}
                  <b>{v}</b>
                </span>
              ))}
          </div>
        )}
      </div>
    </figure>
  );
}

function PenyakitTab({ cabang, options }: { cabang?: string; options: AnalitikOptions }) {
  const { token } = useAuth();
  const fail = useFail();
  const [weeks, setWeeks] = useState(options.weeks);
  const [data, setData] = useState<AnalitikPenyakit | null>(null);
  const [error, setError] = useState("");
  const seq = useRef(0);

  useEffect(() => {
    if (!token) return;
    const mine = ++seq.current;
    setError("");
    api
      .analitikPenyakit(token, { cabang, weeks, top: 10 })
      .then((d) => mine === seq.current && setData(d))
      .catch((err) => mine === seq.current && setError(fail(err, "Gagal memuat tren penyakit")));
  }, [token, cabang, weeks, fail]);

  const columns: Column<AnalitikIcdTrend>[] = [
    { key: "kode", header: "ICD", value: (r) => r.kode || "-" },
    { key: "nama", header: "Diagnosa", value: (r) => r.nama, className: "dt-wrap" },
    { key: "total", header: `Total ${weeks} mgg`, value: (r) => num.format(r.total), sortValue: (r) => r.total, className: "num" },
    { key: "mingguIni", header: "7 hari terakhir", value: (r) => num.format(r.mingguIni), sortValue: (r) => r.mingguIni, className: "num" },
    { key: "rataSebelum", header: "Rata 4 mgg sebelumnya", value: (r) => num.format(r.rataSebelum), sortValue: (r) => r.rataSebelum, className: "num" },
    { key: "proyeksiMinggu", header: "Proyeksi minggu depan", value: (r) => num.format(r.proyeksiMinggu), sortValue: (r) => r.proyeksiMinggu, className: "num" },
    {
      key: "lonjakan",
      header: "Lonjakan",
      value: (r) => (r.lonjakan ? `Ya (${num.format(r.rasio)}×)` : "-"),
      render: (r) =>
        r.lonjakan ? (
          <span className="an-status an-kritis">
            <TrendingUp aria-hidden="true" /> {num.format(r.rasio)}×
          </span>
        ) : (
          "-"
        ),
      sortValue: (r) => (r.lonjakan ? r.rasio : 0),
    },
  ];

  return (
    <>
      <div className="an-controls">
        <label className="dt-length">
          <span>Periode</span>
          <select className="select" value={weeks} onChange={(e) => setWeeks(Number(e.target.value))}>
            {[8, 12, 16, 26].map((w) => (
              <option key={w} value={w}>
                {w} minggu
              </option>
            ))}
          </select>
        </label>
      </div>
      {error && (
        <p className="alert error" role="alert">
          {error}
        </p>
      )}
      {!data ? (
        !error && <p className="muted">Memuat…</p>
      ) : (
        <>
          {data.lonjakan.length > 0 ? (
            <div className="alert error an-spikes" role="status">
              <strong>
                <TrendingUp aria-hidden="true" /> Lonjakan 7 hari terakhir:
              </strong>{" "}
              {data.lonjakan
                .slice(0, 6)
                .map((t) => `${t.kode || t.nama} ${t.nama !== t.kode ? `(${t.nama}) ` : ""}${t.mingguIni} kasus vs rata-rata ${num.format(t.rataSebelum)}`)
                .join(" · ")}
            </div>
          ) : (
            <p className="alert success" role="status">
              Tidak ada lonjakan diagnosa dalam 7 hari terakhir.
            </p>
          )}
          <TrendChart data={data} />
          <DataTable title="Diagnosa teratas" columns={columns} rows={data.top} rowKey={(r) => r.idIcd} defaultPageSize={10} />
          <p className="muted an-foot">
            Minggu = 7 hari yang berakhir hari ini (minggu terakhir bisa belum lengkap). Lonjakan: ≥ 5 kasus, ≥ 1,5× dan ≥ 3 kasus di atas
            rata-rata 4 minggu sebelumnya. Kunjungan yang dibatalkan tidak dihitung.
          </p>
        </>
      )}
    </>
  );
}

// ---------- Diagnosa → Obat ----------

function DiagnosaTab({ cabang }: { cabang?: string }) {
  const { token } = useAuth();
  const fail = useFail();
  const [icdInput, setIcdInput] = useState("");
  const [icd, setIcd] = useState("");
  const [horizon, setHorizon] = useState(30);
  const [data, setData] = useState<AnalitikDiagnosaObatResult | null>(null);
  const [error, setError] = useState("");
  const seq = useRef(0);

  useEffect(() => {
    if (!token) return;
    const mine = ++seq.current;
    setError("");
    setData(null);
    api
      .analitikDiagnosaObat(token, { cabang, icd, horizon })
      .then((d) => mine === seq.current && setData(d))
      .catch((err) => mine === seq.current && setError(fail(err, "Gagal memuat data")));
  }, [token, cabang, icd, horizon, fail]);

  const onSearch = (e: FormEvent) => {
    e.preventDefault();
    setIcd(icdInput.trim());
  };

  type Saran = AnalitikDiagnosaObatResult["saran"][number];
  const saranCols: Column<Saran>[] = [
    { key: "nama", header: "Obat", value: (r) => r.nama, className: "dt-wrap" },
    { key: "kebutuhan", header: `Kebutuhan ${horizon} hr`, value: (r) => `${num.format(r.kebutuhan)} ${r.satuan}`, sortValue: (r) => r.kebutuhan, className: "num" },
    { key: "stok", header: "Stok", value: (r) => num.format(r.stok), sortValue: (r) => r.stok, className: "num" },
    {
      key: "kurang",
      header: "Kurang",
      value: (r) => num.format(r.kurang),
      render: (r) => (r.kurang > 0 ? <span className="an-status an-kritis">{num.format(r.kurang)}</span> : <span className="muted">0</span>),
      sortValue: (r) => r.kurang,
      className: "num",
    },
    { key: "dari", header: "Dari diagnosa", value: (r) => r.dari.join(", "), className: "dt-wrap-sm" },
  ];

  return (
    <>
      <form className="an-controls" onSubmit={onSearch}>
        <label className="dt-length">
          <span>Diagnosa (ICD)</span>
          <input className="select" value={icdInput} placeholder="mis. J06 atau ISPA" onChange={(e) => setIcdInput(e.target.value)} />
        </label>
        <button type="submit" className="btn-add">
          Cari
        </button>
        <label className="dt-length">
          <span>Horizon saran</span>
          <select className="select" value={horizon} onChange={(e) => setHorizon(Number(e.target.value))}>
            {[7, 14, 30, 60].map((d) => (
              <option key={d} value={d}>
                {d} hari
              </option>
            ))}
          </select>
        </label>
      </form>
      {error && (
        <p className="alert error" role="alert">
          {error}
        </p>
      )}
      {!data ? (
        !error && <p className="muted">Memuat…</p>
      ) : (
        <>
          {data.saran.length > 0 && (
            <DataTable
              title="Saran stok dari proyeksi kasus"
              columns={saranCols}
              rows={data.saran}
              rowKey={(r) => r.idObat}
              initialSort={{ key: "kurang", dir: "desc" }}
              defaultPageSize={10}
            />
          )}
          {data.diagnosa.length === 0 ? (
            <p className="muted">Tidak ada diagnosa yang cocok pada {data.from} s/d {data.to}.</p>
          ) : (
            <div className="an-dx-list">
              {data.diagnosa.map((d) => (
                <article key={d.idIcd} className="an-dx">
                  <header>
                    <strong>{d.kode || "-"}</strong> {d.nama}
                    <span className="muted">
                      {" "}
                      · {d.kunjungan} kunjungan · proyeksi {num.format(d.proyeksiMinggu)}/minggu
                    </span>
                    {d.lonjakan && (
                      <span className="an-status an-kritis">
                        <TrendingUp aria-hidden="true" /> Lonjakan
                      </span>
                    )}
                  </header>
                  {d.obat.length === 0 ? (
                    <p className="muted">Tidak ada resep obat.</p>
                  ) : (
                    <ul className="an-rx">
                      {d.obat.map((o) => (
                        <li key={o.idObat}>
                          <span className="an-rx-name">{o.nama}</span>
                          <span className="an-bar" aria-hidden="true">
                            <span style={{ width: `${Math.round(o.support * 100)}%` }} />
                          </span>
                          <span className="an-rx-val">
                            {pct.format(o.support)} kunjungan · {num.format(o.qtyPerKasus)} {o.satuan}/kasus
                          </span>
                        </li>
                      ))}
                    </ul>
                  )}
                </article>
              ))}
            </div>
          )}
          <p className="muted an-foot">
            Data {data.from} s/d {data.to}. Ini asosiasi resep (obat yang diresepkan pada kunjungan yang sama), bukan anjuran terapi. Kunjungan
            dengan beberapa diagnosa dihitung untuk masing-masing, jadi saran stok adalah perkiraan atas.
          </p>
        </>
      )}
    </>
  );
}

// ---------- Asisten AI ----------

const SUGGESTIONS = [
  "Obat apa saja yang harus segera dipesan minggu ini?",
  "Penyakit apa yang sedang naik, dan obat apa yang perlu disiapkan?",
  "Obat apa yang biasa diresepkan untuk ISPA?",
  "Berapa pendapatan bulan ini dibanding bulan lalu?",
];

function AsistenTab({ cabang, options }: { cabang?: string; options: AnalitikOptions }) {
  const { token } = useAuth();
  const fail = useFail();
  const [summary, setSummary] = useState<{ text: string; cached: boolean } | null>(null);
  const [summaryBusy, setSummaryBusy] = useState(false);
  const [summaryError, setSummaryError] = useState("");

  if (!options.aiEnabled) return <AiDisabledNotice />;

  const ringkas = (refresh: boolean) => {
    if (!token) return;
    setSummaryBusy(true);
    setSummaryError("");
    api
      .analitikRingkasan(token, { cabang, refresh })
      .then(setSummary)
      .catch((err) => setSummaryError(fail(err, "Gagal membuat ringkasan")))
      .finally(() => setSummaryBusy(false));
  };

  return (
    <div className="an-ai">
      <section className="an-ai-card" aria-labelledby="an-ringkasan">
        <div className="an-ai-head">
          <h2 id="an-ringkasan">Ringkasan eksekutif</h2>
          <button type="button" className="btn-add" disabled={summaryBusy} onClick={() => ringkas(!!summary)}>
            {summary ? <RefreshCw aria-hidden="true" /> : <Bot aria-hidden="true" />}
            {summaryBusy ? "Membuat…" : summary ? "Buat ulang" : "Buat ringkasan"}
          </button>
        </div>
        {summaryError && (
          <p className="alert error" role="alert">
            {summaryError}
          </p>
        )}
        {summary ? (
          <>
            <AiText text={summary.text} />
            {summary.cached && <p className="muted">Ringkasan hari ini yang sudah dibuat sebelumnya.</p>}
          </>
        ) : (
          !summaryBusy && <p className="muted">Ringkasan kondisi stok, tren penyakit, kinerja dan tindakan yang disarankan untuk cabang yang dipilih.</p>
        )}
      </section>

      <section className="an-ai-card" aria-labelledby="an-chat">
        <h2 id="an-chat">Tanya asisten</h2>
        <ChatBox cabang={cabang} suggestions={SUGGESTIONS} placeholder="Tanya tentang stok obat, tren penyakit, kunjungan atau pendapatan…" />
      </section>
    </div>
  );
}
