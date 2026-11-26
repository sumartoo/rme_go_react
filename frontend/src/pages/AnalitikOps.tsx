import { MouseEvent, ReactNode, useEffect, useRef, useState } from "react";
import { AlertTriangle, ChevronDown, ChevronRight, Info, OctagonAlert, TrendingDown, TrendingUp } from "lucide-react";
import { AnalitikCekKualitas, AnalitikCommand, AnalitikFollowUp, AnalitikInsight, AnalitikKinerja, AnalitikOptions, api } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { AiDisabledNotice, ChatBox, num, rupiah, useFail } from "./analitikShared";

const juta = (v: number) => (Math.abs(v) >= 1e6 ? `Rp${num.format(v / 1e6)} jt` : rupiah.format(v));

function Delta({ value, goodWhenUp = true }: { value: number | null; goodWhenUp?: boolean }) {
  if (value === null) return <span className="muted">tidak ada pembanding</span>;
  const up = value >= 0;
  const good = up === goodWhenUp;
  return (
    <span className={`an-delta ${good ? "an-delta-good" : "an-delta-bad"}`}>
      {up ? <TrendingUp aria-hidden="true" /> : <TrendingDown aria-hidden="true" />}
      {up ? "+" : ""}
      {num.format(value)}%
    </span>
  );
}

function Kpi({ label, value, sub, tone }: { label: string; value: string; sub?: ReactNode; tone?: "kritis" | "perhatian" }) {
  return (
    <div className={`an-kpi${tone ? ` an-kpi-${tone}` : ""}`}>
      <span className="an-kpi-label">{label}</span>
      <span className="an-kpi-value">{value}</span>
      {sub && <span className="an-kpi-sub">{sub}</span>}
    </div>
  );
}

const INSIGHT_ICON: Record<AnalitikInsight["level"], ReactNode> = {
  kritis: <OctagonAlert aria-hidden="true" />,
  perhatian: <AlertTriangle aria-hidden="true" />,
  info: <Info aria-hidden="true" />,
};

/** Weekly revenue bars; transactions in the tooltip (one measure per axis). */
function RevenueChart({ rows }: { rows: AnalitikKinerja[] }) {
  const [hover, setHover] = useState<number | null>(null);
  if (rows.length === 0) return <p className="muted">Belum ada transaksi pada periode ini.</p>;
  const W = 640;
  const H = 200;
  const pad = { l: 52, r: 8, t: 10, b: 26 };
  const max = Math.max(1, ...rows.map((r) => r.pendapatan));
  const step = Math.ceil(max / 4 / 1e6) * 1e6 || 1;
  const top = step * 4;
  const bw = (W - pad.l - pad.r) / rows.length;
  const y = (v: number) => pad.t + (H - pad.t - pad.b) * (1 - v / top);
  const onMove = (e: MouseEvent<SVGSVGElement>) => {
    const r = e.currentTarget.getBoundingClientRect();
    const px = ((e.clientX - r.left) / r.width) * W;
    const i = Math.floor((px - pad.l) / bw);
    setHover(i >= 0 && i < rows.length ? i : null);
  };
  const label = (k: string) => {
    const [, m, d] = k.split("-");
    return `${Number(d)}/${Number(m)}`;
  };
  return (
    <figure className="an-chart">
      <figcaption className="an-chart-title">Pendapatan per minggu (12 minggu terakhir)</figcaption>
      <div className="an-plot">
        <svg viewBox={`0 0 ${W} ${H}`} role="img" aria-label="Grafik pendapatan per minggu" onMouseMove={onMove} onMouseLeave={() => setHover(null)}>
          {[0, 1, 2, 3, 4].map((k) => (
            <g key={k}>
              <line x1={pad.l} x2={W - pad.r} y1={y(k * step)} y2={y(k * step)} className={k === 0 ? "an-axis" : "an-grid"} />
              <text x={pad.l - 6} y={y(k * step)} className="an-tick" textAnchor="end" dominantBaseline="middle">
                {num.format((k * step) / 1e6)} jt
              </text>
            </g>
          ))}
          {rows.map((r, i) => {
            const x = pad.l + i * bw + 2;
            const h = Math.max(0, y(0) - y(r.pendapatan));
            return (
              <g key={r.kelompok}>
                <path
                  d={`M${x},${y(0)} v${-Math.max(0, h - 4)} q0,-4 4,-4 h${Math.max(0, bw - 12)} q4,0 4,4 v${Math.max(0, h - 4)} z`}
                  className={hover === i ? "an-bar-fill an-bar-hover" : "an-bar-fill"}
                />
                {(i % Math.ceil(rows.length / 6) === 0 || i === rows.length - 1) && (
                  <text x={x + (bw - 4) / 2} y={H - 8} className="an-tick" textAnchor="middle">
                    {label(r.kelompok)}
                  </text>
                )}
              </g>
            );
          })}
        </svg>
        {hover !== null && (
          <div className="an-tooltip" style={{ left: `${((pad.l + (hover + 0.5) * bw) / W) * 100}%` }} role="status">
            <strong>Minggu mulai {label(rows[hover].kelompok)}</strong>
            <span className="an-tip-row">
              Pendapatan <b>{rupiah.format(rows[hover].pendapatan)}</b>
            </span>
            <span className="an-tip-row">
              Transaksi <b>{num.format(rows[hover].transaksi)}</b>
            </span>
            <span className="an-tip-row">
              Pasien <b>{num.format(rows[hover].pasien)}</b>
            </span>
          </div>
        )}
      </div>
    </figure>
  );
}

const CC_SUGGESTIONS = [
  "Obat apa yang perlu dibeli minggu ini, dan berapa perkiraan biayanya?",
  "Bagaimana pendapatan 3 bulan terakhir per bulan?",
  "Dokter dengan kunjungan terbanyak bulan ini?",
  "Hari apa kunjungan paling sepi?",
];

export function CommandTab({ cabang, options }: { cabang?: string; options: AnalitikOptions }) {
  const { token } = useAuth();
  const fail = useFail();
  const [data, setData] = useState<AnalitikCommand | null>(null);
  const [error, setError] = useState("");
  const seq = useRef(0);

  useEffect(() => {
    if (!token) return;
    const mine = ++seq.current;
    setError("");
    api
      .analitikCommand(token, cabang)
      .then((d) => mine === seq.current && setData(d))
      .catch((err) => mine === seq.current && setError(fail(err, "Gagal memuat Command Center")));
  }, [token, cabang, fail]);

  if (error)
    return (
      <p className="alert error" role="alert">
        {error}
      </p>
    );
  if (!data) return <p className="muted">Menghitung…</p>;
  const k = data.kpi;
  return (
    <div className="an-cc">
      <div className="an-kpis">
        <Kpi label="Obat habis" value={num.format(k.obatHabis)} tone={k.obatHabis > 0 ? "kritis" : undefined} sub={`${num.format(k.obatKritis)} kritis`} />
        <Kpi
          label="Perkiraan biaya order 30 hari"
          value={juta(k.biayaOrder)}
          sub={k.orderTanpaHarga > 0 ? `${num.format(k.orderTanpaHarga)} obat tanpa harga valid` : `${num.format(k.obatPerluOrder)} perlu order`}
        />
        <Kpi label="Lonjakan penyakit (7 hari)" value={num.format(k.lonjakan)} tone={k.lonjakan > 0 ? "perhatian" : undefined} />
        <Kpi label="Transaksi 7 hari" value={num.format(k.transaksi7)} sub={<Delta value={k.transaksiDelta} />} />
        <Kpi label="Pendapatan 30 hari" value={juta(k.pendapatan30)} sub={<Delta value={k.pendapatanDelta} />} />
        <Kpi label="Stok tidak bergerak" value={num.format(k.tidakBergerak)} sub="item, 90 hari" />
      </div>
      <section className="an-ai-card" aria-labelledby="an-insight">
        <h2 id="an-insight">Perlu perhatian</h2>
        {data.insights.length === 0 ? (
          <p className="muted">Tidak ada hal mendesak.</p>
        ) : (
          <ul className="an-insights">
            {data.insights.map((i) => (
              <li key={i.judul} className={`an-insight an-insight-${i.level}`}>
                {INSIGHT_ICON[i.level]}
                <div>
                  <strong>{i.judul}</strong>
                  <p>{i.detail}</p>
                </div>
              </li>
            ))}
          </ul>
        )}
      </section>
      <RevenueChart rows={data.mingguan} />
      <section className="an-ai-card" aria-labelledby="an-cc-chat">
        <h2 id="an-cc-chat">Tanyakan sesuatu kepada AI</h2>
        {options.aiEnabled ? (
          <ChatBox cabang={cabang} suggestions={CC_SUGGESTIONS} placeholder="Tanya tentang stok, penyakit, kunjungan atau pendapatan…" />
        ) : (
          <AiDisabledNotice />
        )}
      </section>
      <p className="muted an-foot">
        Data per {data.today}. Perkiraan biaya memakai harga pokok (HP) cabang yang wajar; obat dengan HP kosong atau lebih dari 1,2× harga jual
        tidak dihitung (lihat tab Kualitas Data).
      </p>
    </div>
  );
}

// ---------- Kualitas Data ----------

export function KualitasTab({ cabang }: { cabang?: string }) {
  const { token } = useAuth();
  const fail = useFail();
  const [checks, setChecks] = useState<AnalitikCekKualitas[] | null>(null);
  const [error, setError] = useState("");
  const [open, setOpen] = useState<string | null>(null);

  useEffect(() => {
    if (!token) return;
    let live = true;
    api
      .analitikKualitas(token, cabang)
      .then((r) => live && setChecks(r.checks))
      .catch((err) => live && setError(fail(err, "Gagal menjalankan pemeriksaan")));
    return () => {
      live = false;
    };
  }, [token, cabang, fail]);

  if (error)
    return (
      <p className="alert error" role="alert">
        {error}
      </p>
    );
  if (!checks) return <p className="muted">Memeriksa data{cabang ? "" : " semua cabang (bisa beberapa detik)"}…</p>;
  return (
    <>
      <p className="muted">Pemeriksaan hanya menandai data yang perlu dicek. Tidak ada data yang diubah.</p>
      <div className="an-qc-list">
        {checks.map((c) => (
          <section key={c.kode} className={`an-qc ${c.jumlah > 0 ? "an-qc-bad" : "an-qc-ok"}`}>
            <button
              type="button"
              className="an-qc-head"
              aria-expanded={open === c.kode}
              disabled={c.jumlah === 0}
              onClick={() => setOpen(open === c.kode ? null : c.kode)}
            >
              {c.jumlah > 0 ? open === c.kode ? <ChevronDown aria-hidden="true" /> : <ChevronRight aria-hidden="true" /> : <span className="an-qc-dot" />}
              <span className="an-qc-title">{c.judul}</span>
              <span className="an-qc-count">{c.jumlah === 0 ? "Bersih" : num.format(c.jumlah)}</span>
            </button>
            <p className="muted an-qc-desc">{c.penjelasan}</p>
            {open === c.kode && c.contoh.length > 0 && (
              <div className="an-qc-table">
                <table className="table">
                  <thead>
                    <tr>
                      {c.kolom.map((k) => (
                        <th key={k}>{k}</th>
                      ))}
                    </tr>
                  </thead>
                  <tbody>
                    {c.contoh.map((row, i) => (
                      <tr key={i}>
                        {row.map((cell, j) => (
                          <td key={j}>{cell || "-"}</td>
                        ))}
                      </tr>
                    ))}
                  </tbody>
                </table>
                {c.jumlah > c.contoh.length && (
                  <p className="muted">
                    Menampilkan {c.contoh.length} dari {num.format(c.jumlah)}.
                  </p>
                )}
              </div>
            )}
          </section>
        ))}
      </div>
    </>
  );
}

// ---------- Follow-up ----------

export function FollowUpTab({ cabang }: { cabang?: string }) {
  const { token } = useAuth();
  const fail = useFail();
  const [hari, setHari] = useState(35);
  const [kelompok, setKelompok] = useState("");
  const [res, setRes] = useState<{ rows: AnalitikFollowUp[]; total: number; kelompok: { kode: string; nama: string }[] } | null>(null);
  const [error, setError] = useState("");
  const seq = useRef(0);

  useEffect(() => {
    if (!token) return;
    const mine = ++seq.current;
    setError("");
    setRes(null);
    api
      .analitikFollowUp(token, { cabang, hari, kelompok })
      .then((r) => mine === seq.current && setRes(r))
      .catch((err) => mine === seq.current && setError(fail(err, "Gagal memuat daftar follow-up")));
  }, [token, cabang, hari, kelompok, fail]);

  const namaKelompok = (k: string) => res?.kelompok.find((g) => g.kode === k)?.nama ?? k;
  const columns: Column<AnalitikFollowUp>[] = [
    { key: "noRm", header: "No RM", value: (r) => r.noRm },
    { key: "nama", header: "Pasien", value: (r) => r.nama, className: "dt-wrap" },
    { key: "telp", header: "Telp / HP", value: (r) => r.telp || "-" },
    { key: "kelompok", header: "Kondisi", value: (r) => `${namaKelompok(r.kelompok)} · ${r.kodeIcd}`, className: "dt-wrap-sm" },
    { key: "terakhir", header: "Kunjungan terakhir", value: (r) => r.terakhir },
    { key: "hariSejak", header: "Hari sejak", value: (r) => num.format(r.hariSejak), sortValue: (r) => r.hariSejak, className: "num" },
    { key: "jumlahKontrol", header: "Kontrol (1 th)", value: (r) => num.format(r.jumlahKontrol), sortValue: (r) => r.jumlahKontrol, className: "num" },
    { key: "cabang", header: "Cabang", value: (r) => r.cabang },
  ];

  return (
    <>
      <div className="an-controls">
        <label className="dt-length">
          <span>Tidak kembali ≥</span>
          <select className="select" value={hari} onChange={(e) => setHari(Number(e.target.value))}>
            {[30, 35, 45, 60, 90, 180].map((d) => (
              <option key={d} value={d}>
                {d} hari
              </option>
            ))}
          </select>
        </label>
        <label className="dt-length">
          <span>Kondisi</span>
          <select className="select" value={kelompok} onChange={(e) => setKelompok(e.target.value)}>
            <option value="">Semua kronis</option>
            {(res?.kelompok ?? []).map((g) => (
              <option key={g.kode} value={g.kode}>
                {g.nama}
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
      {!res ? (
        !error && <p className="muted">Memuat…</p>
      ) : (
        <DataTable
          title={`Pasien kronis yang perlu ditindaklanjuti (${num.format(res.total)})`}
          columns={columns}
          rows={res.rows}
          rowKey={(r) => `${r.idPasien}|${r.kelompok}`}
          initialSort={{ key: "hariSejak", dir: "asc" }}
          defaultPageSize={25}
        />
      )}
      <p className="muted an-foot">
        Pasien dengan diagnosa kronis pada minimal 2 hari kunjungan dalam setahun terakhir, yang kunjungan terakhirnya (untuk keperluan apa pun)
        sudah lewat dari batas hari di atas. Daftar ini hanya tampil di aplikasi dan tidak dikirim ke AI.
      </p>
    </>
  );
}
