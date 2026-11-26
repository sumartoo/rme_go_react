import { McuItem, McuSection } from "../../api";
import { Box, Field, FormProps, Radios, v, Values, n as noteOf, num, fmt } from "./mcuFields";

const NORMAL_RE = /^(normal|tidak|tidak ada|negatif|sinus|normo|baik|dpt mengikuti|jernih|bersih|compos)/i;

const radios = (it: McuItem) => it.options.filter((o) => o.entry === "radio" && o.nama.trim() !== "");
const texts = (it: McuItem) => it.options.filter((o) => o.entry === "text");

/** Items without answers in the catalogue (Riwayat Medis) are yes/no questions. */
export const YA_TIDAK = ["Ya", "Tidak"];

/** The answer that means "nothing found": the first one that reads as normal, else the first. */
export function normalOption(it: McuItem): string {
  const r = radios(it);
  if (r.length === 0) return it.options.length === 0 ? "Tidak" : "";
  return (r.find((o) => NORMAL_RE.test(o.nama.trim())) ?? r[0]).nama;
}

export function isAbnormal(it: McuItem, values: Values): boolean {
  const val = v(values, it.key);
  if (!val) return false;
  if (radios(it).length === 0 && it.options.length > 0) return false; // text-only
  return val !== normalOption(it);
}

function ItemRow({ it, p }: { it: McuItem; p: FormProps }) {
  const r = radios(it);
  const t = texts(it);
  const val = v(p.values, it.key);
  const yaTidak = it.options.length === 0;
  const opts = yaTidak ? YA_TIDAK : r.map((o) => o.nama);
  const unit = t.find((o) => o.nama.trim() !== "")?.nama.trim();
  return (
    <Field label={it.nama}>
      {opts.length > 0 && <Radios label={it.nama} value={val} options={opts} onChange={(x) => p.set(it.key, x)} abnormal={isAbnormal(it, p.values)} />}
      {opts.length === 0 && t.length > 0 && (
        <>
          <input className="select mcu-input" inputMode="decimal" aria-label={it.nama} value={val} onChange={(e) => p.set(it.key, e.target.value)} />
          {unit && <span className="mcu-unit">{unit}</span>}
        </>
      )}
      {(t.length > 0 && opts.length > 0) || yaTidak || (val && val !== normalOption(it) && opts.length > 0) ? (
        <input
          className="select mcu-input mcu-note"
          aria-label={`Keterangan ${it.nama}`}
          placeholder={yaTidak ? "Keterangan (tahun, pengobatan)" : "Keterangan"}
          value={noteOf(p.values, it.key)}
          onChange={(e) => p.setNote(it.key, e.target.value)}
        />
      ) : null}
    </Field>
  );
}

/** Ishihara plates (Mata 081–094) as a compact grid. */
const isIshihara = (s: McuSection, it: McuItem) => s.kode === "06" && /^\d+$/.test(it.nama.trim()) && radios(it).length >= 2;

function SectionBox({ s, p }: { s: McuSection; p: FormProps }) {
  const plates = s.items.filter((it) => isIshihara(s, it));
  const rest = s.items.filter((it) => !isIshihara(s, it));
  const allNormal = () => {
    for (const it of s.items) {
      const norm = normalOption(it);
      if (norm && !v(p.values, it.key)) p.set(it.key, norm);
    }
  };
  const abnormal = s.items.filter((it) => isAbnormal(it, p.values)).length;
  return (
    <Box
      title={`${s.nama}${abnormal ? ` · ${abnormal} temuan` : ""}`}
      actions={
        s.items.some((it) => normalOption(it)) ? (
          <button type="button" className="btn-small" onClick={allNormal} title="Isi yang masih kosong dengan jawaban normal">
            Semua normal
          </button>
        ) : undefined
      }
    >
      {s.kode === "02" && <Imt p={p} s={s} />}
      {rest.map((it) => (
        <ItemRow key={it.key} it={it} p={p} />
      ))}
      {plates.length > 0 && (
        <div className="mcu-ishihara">
          <p className="muted">Tes buta warna (Ishihara): pilih angka yang dibaca pasien. Pilihan pertama adalah jawaban normal.</p>
          <div className="mcu-plates">
            {plates.map((it) => (
              <label key={it.key} className={isAbnormal(it, p.values) ? "mcu-plate mcu-abnormal" : "mcu-plate"}>
                <span>Plat {it.nama}</span>
                <select className="select mcu-input" value={v(p.values, it.key)} onChange={(e) => p.set(it.key, e.target.value)}>
                  <option value="">—</option>
                  {[...new Set(radios(it).map((o) => o.nama))].map((o) => (
                    <option key={o}>{o}</option>
                  ))}
                </select>
              </label>
            ))}
          </div>
        </div>
      )}
    </Box>
  );
}

/** BMI from the Status Gizi section's weight and height, when present. */
function Imt({ p, s }: { p: FormProps; s: McuSection }) {
  const bb = s.items.find((it) => /berat/i.test(it.nama));
  const tb = s.items.find((it) => /tinggi/i.test(it.nama));
  if (!bb || !tb) return null;
  const w = num(p.values, bb.key);
  const h = num(p.values, tb.key);
  if (!w || !h) return null;
  const imt = w / (h / 100) ** 2;
  const kat = imt < 18.5 ? "Berat badan kurang" : imt < 23 ? "Normal" : imt < 25 ? "Berat badan lebih" : imt < 30 ? "Obesitas I" : "Obesitas II";
  return (
    <p className={imt < 18.5 || imt >= 25 ? "mcu-calc mcu-abnormal" : "mcu-calc"}>
      IMT {fmt.format(imt)} kg/m² — {kat} (kriteria Asia-Pasifik)
    </p>
  );
}

export function CatalogForm(p: FormProps) {
  if (p.sections.length === 0) return <p className="muted">Katalog pemeriksaan belum diimpor (scripts/import-lab-mcu.sh).</p>;
  let grup = "";
  return (
    <>
      {p.sections.map((s) => {
        const head = s.grup !== grup && p.sections.some((x) => x.grup !== p.sections[0].grup);
        grup = s.grup;
        return (
          <div key={s.kode}>
            {head && <h2 className="mcu-group-title">{s.grup}</h2>}
            <SectionBox s={s} p={p} />
          </div>
        );
      })}
    </>
  );
}

export function catalogFindings(values: Values, sections: McuSection[]): string[] {
  const out: string[] = [];
  for (const s of sections) {
    for (const it of s.items) {
      if (it.options.length === 0 && v(values, it.key) === "Ya") {
        const note = noteOf(values, it.key);
        out.push(`Riwayat ${it.nama}${note ? ` (${note})` : ""}`);
      } else if (isAbnormal(it, values)) {
        const note = noteOf(values, it.key);
        out.push(`${s.nama} – ${it.nama}: ${v(values, it.key)}${note ? ` (${note})` : ""}`);
      }
    }
    const sys = s.items.find((it) => /sistol/i.test(it.nama));
    const dia = s.items.find((it) => /diastol/i.test(it.nama));
    if (sys && dia) {
      const a = num(values, sys.key);
      const b = num(values, dia.key);
      if ((a !== null && a >= 140) || (b !== null && b >= 90)) out.push(`Tekanan darah ${a ?? "-"}/${b ?? "-"} mmHg (≥140/90)`);
    }
    if (s.kode === "02") {
      const bb = s.items.find((it) => /berat/i.test(it.nama));
      const tb = s.items.find((it) => /tinggi/i.test(it.nama));
      const w = bb ? num(values, bb.key) : null;
      const h = tb ? num(values, tb.key) : null;
      if (w && h) {
        const imt = w / (h / 100) ** 2;
        if (imt < 18.5 || imt >= 25) out.push(`IMT ${fmt.format(imt)} kg/m² (${imt < 18.5 ? "berat badan kurang" : imt < 30 ? "berat badan lebih/obesitas I" : "obesitas II"})`);
      }
    }
  }
  return out;
}

// ---------- USG ----------

const USG_NORMAL = "Dalam batas normal";

export function UsgForm(p: FormProps) {
  const groups = [...new Set(p.sections.map((s) => s.grup))];
  const jenis = v(p.values, "usg.jenis") || groups[0] || "";
  const organs = p.sections.filter((s) => s.grup === jenis);
  return (
    <>
      <Box title="Jenis pemeriksaan">
        <Field label="Jenis USG">
          <Radios label="Jenis USG" value={jenis} options={groups} onChange={(x) => p.set("usg.jenis", x)} />
        </Field>
        <Field label="Dokter pemeriksa">
          <input className="select mcu-input mcu-wide" value={v(p.values, "usg.dokter")} onChange={(e) => p.set("usg.dokter", e.target.value)} aria-label="Dokter pemeriksa" />
        </Field>
      </Box>
      <Box
        title={`Organ — ${jenis}`}
        actions={
          <button type="button" className="btn-small" onClick={() => organs.forEach((s) => !v(p.values, `usg.${s.kode}`) && p.set(`usg.${s.kode}`, USG_NORMAL))}>
            Semua normal
          </button>
        }
      >
        {organs.map((s) => {
          const val = v(p.values, `usg.${s.kode}`);
          return (
            <Field key={s.kode} label={s.nama}>
              <textarea
                className={`select mcu-input mcu-wide${val && val !== USG_NORMAL ? " mcu-abnormal-input" : ""}`}
                rows={2}
                aria-label={s.nama}
                value={val}
                placeholder={USG_NORMAL}
                onChange={(e) => p.set(`usg.${s.kode}`, e.target.value)}
              />
            </Field>
          );
        })}
        <Field label="Kesan USG">
          <textarea className="select mcu-input mcu-wide" rows={2} value={v(p.values, "usg.kesan")} onChange={(e) => p.set("usg.kesan", e.target.value)} aria-label="Kesan USG" />
        </Field>
      </Box>
    </>
  );
}

export function usgFindings(values: Values, sections: McuSection[]): string[] {
  const jenis = v(values, "usg.jenis") || sections[0]?.grup || "";
  const out = sections
    .filter((s) => s.grup === jenis)
    .filter((s) => {
      const val = v(values, `usg.${s.kode}`);
      return val && val !== USG_NORMAL;
    })
    .map((s) => `USG ${s.nama}: ${v(values, `usg.${s.kode}`)}`);
  const kesan = v(values, "usg.kesan");
  if (kesan) out.push(`Kesan USG: ${kesan}`);
  return out;
}
