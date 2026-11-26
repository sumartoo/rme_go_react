import { useState } from "react";
import { Box, Field, FormProps, RadioField, TextArea, TextInput, v, Values, num, fmt } from "./mcuFields";

// ---------- Audiometri ----------

const FREQ = [250, 500, 1000, 2000, 3000, 4000, 6000, 8000];
const EARS = [
  { k: "ka", label: "Telinga kanan", mark: "O", cls: "mcu-ear-r" },
  { k: "ki", label: "Telinga kiri", mark: "X", cls: "mcu-ear-l" },
];

/** Pure-tone average of 500, 1000, 2000 and 4000 Hz. */
export function pta(x: Values, ear: string): number | null {
  const vals = [500, 1000, 2000, 4000].map((f) => num(x, `aud.${ear}.${f}`));
  if (vals.some((a) => a === null)) return null;
  return (vals as number[]).reduce((a, b) => a + b, 0) / 4;
}

export function derajat(db: number): string {
  if (db <= 25) return "Normal";
  if (db <= 40) return "Tuli ringan";
  if (db <= 55) return "Tuli sedang";
  if (db <= 70) return "Tuli sedang-berat";
  if (db <= 90) return "Tuli berat";
  return "Tuli sangat berat";
}

/** A noise notch: 3–6 kHz at least 20 dB worse than 2 kHz, better again at 8 kHz. */
export function notch(x: Values, ear: string): boolean {
  const at2 = num(x, `aud.${ear}.2000`);
  const at8 = num(x, `aud.${ear}.8000`);
  const worst = Math.max(...[3000, 4000, 6000].map((f) => num(x, `aud.${ear}.${f}`) ?? -Infinity));
  return at2 !== null && worst !== -Infinity && worst - at2 >= 20 && (at8 === null || at8 <= worst - 10);
}

function Audiogram({ x }: { x: Values }) {
  const W = 520;
  const H = 300;
  const pad = { l: 44, r: 12, t: 22, b: 12 };
  const xs = (i: number) => pad.l + (i * (W - pad.l - pad.r)) / (FREQ.length - 1);
  const ys = (db: number) => pad.t + ((db + 10) / 130) * (H - pad.t - pad.b);
  return (
    <figure className="mcu-audiogram">
      <svg viewBox={`0 0 ${W} ${H}`} role="img" aria-label="Audiogram">
        <rect x={pad.l} y={ys(-10)} width={W - pad.l - pad.r} height={ys(25) - ys(-10)} className="mcu-aud-normal" />
        {[-10, 0, 10, 20, 30, 40, 50, 60, 70, 80, 90, 100, 110, 120].map((db) => (
          <g key={db}>
            <line x1={pad.l} x2={W - pad.r} y1={ys(db)} y2={ys(db)} className={db === 0 ? "an-axis" : "an-grid"} />
            <text x={pad.l - 6} y={ys(db)} className="an-tick" textAnchor="end" dominantBaseline="middle">
              {db}
            </text>
          </g>
        ))}
        {FREQ.map((f, i) => (
          <g key={f}>
            <line x1={xs(i)} x2={xs(i)} y1={ys(-10)} y2={ys(120)} className="an-grid" />
            <text x={xs(i)} y={12} className="an-tick" textAnchor="middle">
              {f >= 1000 ? `${f / 1000}k` : f}
            </text>
          </g>
        ))}
        {EARS.map((e) => {
          const pts = FREQ.map((f, i) => ({ i, db: num(x, `aud.${e.k}.${f}`) })).filter((p) => p.db !== null) as { i: number; db: number }[];
          return (
            <g key={e.k} className={e.cls}>
              <polyline fill="none" strokeWidth={2} points={pts.map((p) => `${xs(p.i)},${ys(p.db)}`).join(" ")} />
              {pts.map((p) =>
                e.mark === "O" ? (
                  <circle key={p.i} cx={xs(p.i)} cy={ys(p.db)} r={5} fill="var(--card)" strokeWidth={2} />
                ) : (
                  <path key={p.i} d={`M${xs(p.i) - 5},${ys(p.db) - 5} l10,10 M${xs(p.i) + 5},${ys(p.db) - 5} l-10,10`} strokeWidth={2} />
                ),
              )}
            </g>
          );
        })}
      </svg>
      <figcaption className="mcu-legend">
        <span className="mcu-ear-r">○ Kanan</span> <span className="mcu-ear-l">✕ Kiri</span> <span className="muted">dB HL (konduksi udara); area arsir = normal ≤25 dB</span>
      </figcaption>
    </figure>
  );
}

export function AudiometriForm(p: FormProps) {
  return (
    <>
      <Box title="Ambang dengar konduksi udara (dB HL)">
        <div className="mcu-scroll">
          <table className="mcu-table mcu-grid">
            <thead>
              <tr>
                <th>Frekuensi (Hz)</th>
                {FREQ.map((f) => (
                  <th key={f}>{f}</th>
                ))}
                <th>Rata-rata*</th>
              </tr>
            </thead>
            <tbody>
              {EARS.map((e) => {
                const avg = pta(p.values, e.k);
                return (
                  <tr key={e.k}>
                    <td className={e.cls}>
                      {e.mark} {e.label}
                    </td>
                    {FREQ.map((f) => {
                      const k = `aud.${e.k}.${f}`;
                      const val = num(p.values, k);
                      return (
                        <td key={f}>
                          <input
                            className={`select mcu-input mcu-num${val !== null && val > 25 ? " mcu-abnormal-input" : ""}`}
                            inputMode="numeric"
                            aria-label={`${e.label} ${f} Hz`}
                            value={v(p.values, k)}
                            onChange={(ev) => p.set(k, ev.target.value)}
                          />
                        </td>
                      );
                    })}
                    <td className={avg !== null && avg > 25 ? "mcu-abnormal" : undefined}>{avg === null ? "-" : `${fmt.format(avg)} · ${derajat(avg)}`}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
        <p className="muted">* Rata-rata 500, 1000, 2000 dan 4000 Hz. Takik pada 3–6 kHz (≥20 dB lebih buruk dari 2 kHz) ditandai sebagai kemungkinan gangguan dengar akibat bising.</p>
        {EARS.map((e) => notch(p.values, e.k) && (
          <p key={e.k} className="mcu-calc mcu-abnormal">
            {e.label}: takik 3–6 kHz — sesuai pola gangguan dengar akibat bising (NIHL)
          </p>
        ))}
        <Audiogram x={p.values} />
      </Box>
      <Box title="Pemeriksaan telinga">
        <RadioField p={p} k="aud.otoskopi" label="Otoskopi" options={["Normal", "Serumen", "Perforasi membran timpani", "Lainnya"]} normal={["Normal"]} />
        <TextInput p={p} k="aud.alat" label="Alat / ruang" wide />
      </Box>
    </>
  );
}

export function audiometriFindings(x: Values): string[] {
  const out: string[] = [];
  for (const e of EARS) {
    const avg = pta(x, e.k);
    if (avg !== null && avg > 25) out.push(`${e.label}: ${derajat(avg).toLowerCase()} (rata-rata ${fmt.format(avg)} dB)`);
    if (notch(x, e.k)) out.push(`${e.label}: takik 3–6 kHz, curiga gangguan dengar akibat bising`);
  }
  const oto = v(x, "aud.otoskopi");
  if (oto && oto !== "Normal") out.push(`Otoskopi: ${oto}`);
  return out;
}

// ---------- Spirometri ----------

const SPIRO = [
  { k: "fvc", label: "FVC (KVP)", unit: "L" },
  { k: "fev1", label: "FEV1 (VEP1)", unit: "L" },
  { k: "pef", label: "PEF (APE)", unit: "L/s" },
];

export interface SpiroCalc {
  fvcPct: number | null;
  fev1Pct: number | null;
  ratio: number | null;
  kesan: string | null;
}

export function spiroCalc(x: Values): SpiroCalc {
  const pct = (k: string) => {
    const m = num(x, `spiro.${k}`);
    const pr = num(x, `spiro.${k}.pred`);
    return m !== null && pr ? (m / pr) * 100 : null;
  };
  const fvc = num(x, "spiro.fvc");
  const fev1 = num(x, "spiro.fev1");
  const fvcPct = pct("fvc");
  const fev1Pct = pct("fev1");
  const ratio = fvc && fev1 !== null ? (fev1 / fvc) * 100 : null;
  let kesan: string | null = null;
  if (ratio !== null && fvcPct !== null && fev1Pct !== null) {
    const obs = ratio < 70;
    const res = fvcPct < 80;
    const grade = (pv: number, cut: [number, number]) => (pv >= cut[0] ? "ringan" : pv >= cut[1] ? "sedang" : "berat");
    if (obs && res) kesan = "Campuran (obstruksi dan restriksi)";
    else if (obs) kesan = `Obstruksi ${grade(fev1Pct, [60, 40])}`;
    else if (res) kesan = `Restriksi ${grade(fvcPct, [60, 50])}`;
    else kesan = "Normal";
  }
  return { fvcPct, fev1Pct, ratio, kesan };
}

export function SpirometriForm(p: FormProps) {
  const c = spiroCalc(p.values);
  return (
    <>
      <Box title="Data pasien">
        <Field label="Usia / jenis kelamin">
          <span>
            {p.reg.usia ?? "-"} tahun · {p.reg.gender || "-"}
          </span>
        </Field>
        <TextInput p={p} k="spiro.tb" label="Tinggi badan" unit="cm" type="number" />
        <TextInput p={p} k="spiro.bb" label="Berat badan" unit="kg" type="number" />
      </Box>
      <Box title="Hasil (isi nilai prediksi dari alat spirometer)">
        <div className="mcu-scroll">
          <table className="mcu-table mcu-grid">
            <thead>
              <tr>
                <th>Parameter</th>
                <th>Nilai</th>
                <th>Prediksi</th>
                <th>% prediksi</th>
              </tr>
            </thead>
            <tbody>
              {SPIRO.map((s) => {
                const m = num(p.values, `spiro.${s.k}`);
                const pr = num(p.values, `spiro.${s.k}.pred`);
                const pct = m !== null && pr ? (m / pr) * 100 : null;
                return (
                  <tr key={s.k}>
                    <td>
                      {s.label} <span className="muted">{s.unit}</span>
                    </td>
                    <td>
                      <input className="select mcu-input mcu-num" inputMode="decimal" aria-label={`${s.label} nilai`} value={v(p.values, `spiro.${s.k}`)} onChange={(e) => p.set(`spiro.${s.k}`, e.target.value)} />
                    </td>
                    <td>
                      <input
                        className="select mcu-input mcu-num"
                        inputMode="decimal"
                        aria-label={`${s.label} prediksi`}
                        value={v(p.values, `spiro.${s.k}.pred`)}
                        onChange={(e) => p.set(`spiro.${s.k}.pred`, e.target.value)}
                      />
                    </td>
                    <td className={pct !== null && pct < 80 ? "mcu-abnormal" : undefined}>{pct === null ? "-" : `${fmt.format(pct)}%`}</td>
                  </tr>
                );
              })}
              <tr>
                <td>FEV1/FVC</td>
                <td colSpan={2} className={c.ratio !== null && c.ratio < 70 ? "mcu-abnormal" : undefined}>
                  {c.ratio === null ? "-" : `${fmt.format(c.ratio)}%`}
                </td>
                <td className="muted">normal ≥70%</td>
              </tr>
            </tbody>
          </table>
        </div>
        {c.kesan && (
          <p className={c.kesan === "Normal" ? "mcu-calc" : "mcu-calc mcu-abnormal"}>
            Interpretasi otomatis: <strong>{c.kesan}</strong> <span className="muted">(FEV1/FVC &lt;70% = obstruksi; FVC &lt;80% prediksi = restriksi)</span>
          </p>
        )}
        <RadioField p={p} k="spiro.kualitas" label="Kualitas tes" options={["Baik", "Cukup", "Kurang (perlu diulang)"]} normal={["Baik", "Cukup"]} />
        <TextInput p={p} k="spiro.kesan" label="Kesan dokter" wide placeholder={c.kesan ?? ""} />
      </Box>
    </>
  );
}

export function spirometriFindings(x: Values): string[] {
  const kesan = v(x, "spiro.kesan") || spiroCalc(x).kesan;
  const out: string[] = [];
  if (kesan && !/^normal/i.test(kesan)) out.push(`Spirometri: ${kesan}`);
  if (v(x, "spiro.kualitas").startsWith("Kurang")) out.push("Spirometri perlu diulang (kualitas tes kurang)");
  return out;
}

// ---------- EKG Treadmill ----------

const TM_ROWS = ["Istirahat", "Stage 1", "Stage 2", "Stage 3", "Stage 4", "Stage 5", "Pemulihan 1'", "Pemulihan 3'", "Pemulihan 5'"];
const TM_COLS = [
  { k: "menit", label: "Waktu (mnt)" },
  { k: "hr", label: "HR (x/mnt)" },
  { k: "td", label: "TD (mmHg)" },
  { k: "ket", label: "Keluhan / perubahan ST" },
];

export function TreadmillForm(p: FormProps) {
  const usia = p.reg.usia;
  const target = usia ? 220 - usia : null;
  const hrMax = num(p.values, "tm.hrmax");
  const pct = target && hrMax ? (hrMax / target) * 100 : null;
  return (
    <>
      <Box title="Protokol">
        <RadioField p={p} k="tm.protokol" label="Protokol" options={["Bruce", "Modified Bruce", "Naughton"]} />
        <Field label="Target HR">
          <span>{target ? `${target} x/mnt (220 − usia); submaksimal 85% = ${Math.round(target * 0.85)}` : "Usia tidak diketahui"}</span>
        </Field>
      </Box>
      <Box title="Tahapan">
        <div className="mcu-scroll">
          <table className="mcu-table mcu-grid">
            <thead>
              <tr>
                <th>Tahap</th>
                {TM_COLS.map((c) => (
                  <th key={c.k}>{c.label}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {TM_ROWS.map((r, i) => (
                <tr key={r}>
                  <td>{r}</td>
                  {TM_COLS.map((c) => (
                    <td key={c.k}>
                      <input
                        className={`select mcu-input${c.k === "ket" ? "" : " mcu-num"}`}
                        aria-label={`${r} ${c.label}`}
                        value={v(p.values, `tm.${i}.${c.k}`)}
                        onChange={(e) => p.set(`tm.${i}.${c.k}`, e.target.value)}
                      />
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </Box>
      <Box title="Ringkasan">
        <TextInput p={p} k="tm.durasi" label="Durasi latihan" unit="menit" type="number" />
        <TextInput p={p} k="tm.mets" label="Kapasitas" unit="METs" type="number" />
        <TextInput p={p} k="tm.hrmax" label="HR maksimal" unit="x/mnt" type="number" />
        {pct !== null && <p className={pct < 85 ? "mcu-calc mcu-abnormal" : "mcu-calc"}>{fmt.format(pct)}% dari target HR{pct < 85 ? " — submaksimal tidak tercapai" : ""}</p>}
        <RadioField
          p={p}
          k="tm.berhenti"
          label="Alasan berhenti"
          options={["Target HR tercapai", "Kelelahan", "Nyeri dada", "Sesak napas", "Perubahan ST", "Aritmia", "Respon TD abnormal", "Lainnya"]}
          normal={["Target HR tercapai", "Kelelahan"]}
        />
        <RadioField p={p} k="tm.respon_td" label="Respon tekanan darah" options={["Normal", "Hipertensif", "Hipotensif"]} normal={["Normal"]} />
        <RadioField p={p} k="tm.st" label="Perubahan segmen ST" options={["Tidak ada", "Depresi ST ≥1 mm", "Elevasi ST"]} normal={["Tidak ada"]} />
        <RadioField p={p} k="tm.aritmia" label="Aritmia" options={["Tidak ada", "Ada"]} normal={["Tidak ada"]} />
        <RadioField
          p={p}
          k="tm.kesan"
          label="Kesan"
          options={["Respon iskemik negatif", "Respon iskemik positif", "Equivocal", "Tidak dapat disimpulkan"]}
          normal={["Respon iskemik negatif"]}
        />
        <TextArea p={p} k="tm.catatan" label="Catatan" />
      </Box>
    </>
  );
}

export function treadmillFindings(x: Values, usia: number | null): string[] {
  const out: string[] = [];
  const kesan = v(x, "tm.kesan");
  if (kesan && kesan !== "Respon iskemik negatif") out.push(`Treadmill: ${kesan.toLowerCase()}`);
  const st = v(x, "tm.st");
  if (st && st !== "Tidak ada") out.push(`Treadmill: ${st}`);
  const td = v(x, "tm.respon_td");
  if (td && td !== "Normal") out.push(`Respon TD ${td.toLowerCase()} saat treadmill`);
  if (v(x, "tm.aritmia") === "Ada") out.push("Aritmia saat treadmill");
  const hr = num(x, "tm.hrmax");
  if (usia && hr && (hr / (220 - usia)) * 100 < 85) out.push("Target HR submaksimal tidak tercapai");
  return out;
}

// ---------- Pemeriksaan Gigi ----------

const UPPER = [18, 17, 16, 15, 14, 13, 12, 11, 21, 22, 23, 24, 25, 26, 27, 28];
const LOWER = [48, 47, 46, 45, 44, 43, 42, 41, 31, 32, 33, 34, 35, 36, 37, 38];
const GIGI_STATUS = [
  { k: "", label: "Sehat", short: "" },
  { k: "D", label: "Karies", short: "D" },
  { k: "F", label: "Tambalan", short: "F" },
  { k: "M", label: "Hilang", short: "M" },
  { k: "R", label: "Sisa akar", short: "R" },
  { k: "P", label: "Gigi palsu", short: "P" },
];

export function dmft(x: Values) {
  const all = [...UPPER, ...LOWER].map((t) => v(x, `gigi.${t}`));
  const c = (k: string) => all.filter((s) => s === k).length;
  return { d: c("D"), m: c("M"), f: c("F"), r: c("R"), p: c("P") };
}

export function GigiForm(p: FormProps) {
  const [brush, setBrush] = useState("D");
  const s = dmft(p.values);
  const tooth = (t: number) => {
    const st = v(p.values, `gigi.${t}`);
    return (
      <button
        key={t}
        type="button"
        className={`mcu-tooth${st ? ` mcu-tooth-${st}` : ""}`}
        title={`Gigi ${t}: ${GIGI_STATUS.find((g) => g.k === st)?.label ?? "Sehat"}`}
        aria-label={`Gigi ${t}`}
        onClick={() => p.set(`gigi.${t}`, st === brush ? "" : brush)}
      >
        <span className="mcu-tooth-no">{t}</span>
        <span className="mcu-tooth-st">{st || "·"}</span>
      </button>
    );
  };
  return (
    <>
      <Box title="Odontogram">
        <div className="mcu-brush" role="radiogroup" aria-label="Status yang ditandai">
          <span className="muted">Pilih status lalu klik gigi:</span>
          {GIGI_STATUS.filter((g) => g.k).map((g) => (
            <label key={g.k} className={brush === g.k ? "mcu-radio on" : "mcu-radio"}>
              <input type="radio" checked={brush === g.k} onChange={() => setBrush(g.k)} />
              {g.short} · {g.label}
            </label>
          ))}
        </div>
        <div className="mcu-teeth">
          <div className="mcu-teeth-row">{UPPER.map(tooth)}</div>
          <div className="mcu-teeth-row">{LOWER.map(tooth)}</div>
        </div>
        <p className="mcu-calc">
          DMF-T {s.d + s.m + s.f} (D {s.d} · M {s.m} · F {s.f}) · Sisa akar {s.r} · Gigi palsu {s.p}
        </p>
      </Box>
      <Box title="Jaringan mulut">
        <RadioField p={p} k="gigi.karang" label="Karang gigi" options={["Tidak ada", "Sedikit", "Banyak"]} normal={["Tidak ada", "Sedikit"]} />
        <RadioField p={p} k="gigi.gusi" label="Gusi" options={["Normal", "Gingivitis", "Periodontitis"]} normal={["Normal"]} />
        <RadioField p={p} k="gigi.oklusi" label="Oklusi" options={["Normal", "Maloklusi"]} normal={["Normal"]} />
        <RadioField p={p} k="gigi.kebersihan" label="Kebersihan mulut" options={["Baik", "Sedang", "Buruk"]} normal={["Baik", "Sedang"]} />
        <TextArea p={p} k="gigi.saran" label="Saran perawatan" />
      </Box>
    </>
  );
}

export function gigiFindings(x: Values): string[] {
  const s = dmft(x);
  const out: string[] = [];
  if (s.d) out.push(`Karies ${s.d} gigi`);
  if (s.r) out.push(`Sisa akar ${s.r} gigi`);
  if (s.m) out.push(`Gigi hilang ${s.m}`);
  if (v(x, "gigi.karang") === "Banyak") out.push("Karang gigi banyak");
  const g = v(x, "gigi.gusi");
  if (g && g !== "Normal") out.push(g);
  if (v(x, "gigi.oklusi") === "Maloklusi") out.push("Maloklusi");
  if (v(x, "gigi.kebersihan") === "Buruk") out.push("Kebersihan mulut buruk");
  return out;
}

// ---------- Pemeriksaan Lab ----------

const FLAGS = ["", "N", "H", "L", "Abnormal"];

export function LabForm(p: FormProps) {
  if (p.lab.length === 0) return <p className="muted">Registrasi ini tidak memiliki pemeriksaan lab yang dipesan.</p>;
  let grup = "";
  const paket = [...new Set(p.lab.map((l) => l.paket).filter(Boolean))];
  return (
    <Box title={`Hasil pemeriksaan${paket.length ? ` · ${paket.join(", ")}` : ""}`}>
      <div className="mcu-scroll">
        <table className="mcu-table mcu-grid">
          <thead>
            <tr>
              <th>Pemeriksaan</th>
              <th>Hasil</th>
              <th>Satuan</th>
              <th>Nilai rujukan</th>
              <th>Flag</th>
            </tr>
          </thead>
          <tbody>
            {p.lab.map((l) => {
              const k = `lab.${l.labId}`;
              const head = l.grup !== grup;
              grup = l.grup;
              const flag = v(p.values, `${k}.flag`);
              return [
                head && (
                  <tr key={`g-${l.grup}-${l.labId}`} className="mcu-row-group">
                    <td colSpan={5}>{l.grup || "Lainnya"}</td>
                  </tr>
                ),
                <tr key={l.labId} className={flag && flag !== "N" ? "mcu-row-abnormal" : undefined}>
                  <td>{l.nama}</td>
                  <td>
                    <input className="select mcu-input" aria-label={`Hasil ${l.nama}`} value={v(p.values, k)} onChange={(e) => p.set(k, e.target.value)} />
                  </td>
                  <td>
                    <input className="select mcu-input mcu-num" aria-label={`Satuan ${l.nama}`} value={v(p.values, `${k}.satuan`)} onChange={(e) => p.set(`${k}.satuan`, e.target.value)} />
                  </td>
                  <td>
                    <input className="select mcu-input" aria-label={`Rujukan ${l.nama}`} value={v(p.values, `${k}.rujukan`)} onChange={(e) => p.set(`${k}.rujukan`, e.target.value)} />
                  </td>
                  <td>
                    <select className="select mcu-input" aria-label={`Flag ${l.nama}`} value={flag} onChange={(e) => p.set(`${k}.flag`, e.target.value)}>
                      {FLAGS.map((f) => (
                        <option key={f} value={f}>
                          {f || "—"}
                        </option>
                      ))}
                    </select>
                  </td>
                </tr>,
              ];
            })}
          </tbody>
        </table>
      </div>
      <p className="muted">Flag: N normal, H tinggi, L rendah. Item non-lab dalam paket (dokter, radiologi, EKG) diisi di form masing-masing.</p>
    </Box>
  );
}

export function labFindings(x: Values, lab: FormProps["lab"]): string[] {
  return lab
    .filter((l) => {
      const f = v(x, `lab.${l.labId}.flag`);
      return f && f !== "N";
    })
    .map((l) => {
      const k = `lab.${l.labId}`;
      const f = v(x, `${k}.flag`);
      return `${l.nama} ${v(x, k)}${v(x, `${k}.satuan`) ? ` ${v(x, `${k}.satuan`)}` : ""} (${f === "H" ? "tinggi" : f === "L" ? "rendah" : "abnormal"})`;
    });
}
