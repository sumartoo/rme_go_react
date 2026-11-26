import { ReactNode, useEffect, useState } from "react";
import { Save } from "lucide-react";
import { api, ApiError, PemeriksaanBase, PemeriksaanInput, PemeriksaanRecord } from "../api";
import { useAuth } from "../auth/AuthContext";

/**
 * The Pengobatan form's Pemeriksaan tab: the examination of the picked
 * registration, loaded from and saved to trxpemeriksaan (one row per
 * registration). Dokter PJ is saved with it. Stays mounted while other tabs
 * show, so unsaved typing is kept.
 */

type TextKey =
  | "keluhanUtama"
  | "anamnesis"
  | "pemeriksaanFisik"
  | "rekapHasilPenunjang"
  | "hasilPenunjang"
  | "rencanaPenatalaksanaan"
  | "hasilPemeriksaanDokter"
  | "tindakanDokter";
type NumKey = "sistole" | "diastole" | "beratBadan" | "tinggiBadan" | "suhu";
type Draft = Record<TextKey | NumKey, string>;

const EMPTY: Draft = {
  keluhanUtama: "",
  anamnesis: "",
  pemeriksaanFisik: "",
  rekapHasilPenunjang: "",
  hasilPenunjang: "",
  rencanaPenatalaksanaan: "",
  hasilPemeriksaanDokter: "",
  tindakanDokter: "",
  sistole: "",
  diastole: "",
  beratBadan: "",
  tinggiBadan: "",
  suhu: "",
};

const TEXTS: { key: TextKey; label: string; max: number }[] = [
  { key: "keluhanUtama", label: "Keluhan Utama", max: 300 },
  { key: "anamnesis", label: "Anamnesis", max: 300 },
  { key: "pemeriksaanFisik", label: "Pemeriksaan Fisik", max: 300 },
  { key: "rekapHasilPenunjang", label: "Rekap Hasil Penunjang", max: 300 },
  { key: "hasilPenunjang", label: "Hasil Penunjang", max: 300 },
  { key: "rencanaPenatalaksanaan", label: "Rencana Penatalaksanaan", max: 300 },
  { key: "hasilPemeriksaanDokter", label: "Hasil Pemeriksaan Dokter", max: 400 },
  { key: "tindakanDokter", label: "Tindakan Dokter", max: 400 },
];

const str = (v: number | null | undefined) => (v === null || v === undefined ? "" : String(v));

function toDraft(r: PemeriksaanRecord): Draft {
  return {
    keluhanUtama: r.keluhanUtama,
    anamnesis: r.anamnesis,
    pemeriksaanFisik: r.pemeriksaanFisik,
    rekapHasilPenunjang: r.rekapHasilPenunjang,
    hasilPenunjang: r.hasilPenunjang,
    rencanaPenatalaksanaan: r.rencanaPenatalaksanaan,
    hasilPemeriksaanDokter: r.hasilPemeriksaanDokter,
    tindakanDokter: r.tindakanDokter,
    sistole: str(r.sistole),
    diastole: str(r.diastole),
    beratBadan: str(r.beratBadan),
    tinggiBadan: str(r.tinggiBadan),
    suhu: str(r.suhu),
  };
}

/** "" -> null; otherwise a number (whole when `whole`), or undefined when invalid. */
function parseNum(s: string, whole: boolean): number | null | undefined {
  const t = s.trim().replace(",", ".");
  if (!t) return null;
  const n = Number(t);
  if (!Number.isFinite(n) || (whole && !Number.isInteger(n))) return undefined;
  return n;
}

function ExamRow({ id, label, children }: { id: string; label: string; children: ReactNode }) {
  return (
    <div className="tx-row">
      <label htmlFor={id}>{label}</label>
      <div className="tx-ctrl">{children}</div>
    </div>
  );
}

export default function PemeriksaanTab({
  hidden,
  idKun,
  idDokter,
  onDirty,
  base = "/api/pengobatan",
  readOnly = false,
  requireDokter = false,
  onLoaded,
  section = "all",
}: {
  hidden: boolean;
  /** The picked registration (trxpendaftaran id); null when none is picked. */
  idKun: number | null;
  /** Dokter PJ, saved with the examination. */
  idDokter: number | null;
  /** Reports whether there is typing that is not saved yet. */
  onDirty?: (dirty: boolean) => void;
  /** Which API loads and saves it (each is guarded by its own menu). */
  base?: PemeriksaanBase;
  /** The visit is closed or cancelled: show the examination without saving. */
  readOnly?: boolean;
  /** Refuse to save until a Dokter PJ is chosen. */
  requireDokter?: boolean;
  /** Called with the loaded or saved examination (null when none is saved yet). */
  onLoaded?: (r: PemeriksaanRecord | null) => void;
  /** Which fields show: the nurse's (complaints and vital signs), the doctor's, or all. Save always sends all of them. */
  section?: "all" | "perawat" | "dokter";
}) {
  const { token, logout } = useAuth();
  const [draft, setDraft] = useState<Draft>(EMPTY);
  const [saved, setSaved] = useState<PemeriksaanRecord | null>(null);
  const [loading, setLoading] = useState(false);
  const [busy, setBusy] = useState(false);
  const [msg, setMsg] = useState<{ kind: "success" | "error" | "info"; text: string } | null>(null);
  const [fieldErrors, setFieldErrors] = useState<Record<string, string>>({});

  const expired = () => logout("Your session has expired. Please log in again.");

  // Unsaved when the draft differs from what was loaded or last saved.
  const baseline = saved ? toDraft(saved) : EMPTY;
  const dirty = idKun !== null && (Object.keys(EMPTY) as (keyof Draft)[]).some((k) => draft[k].trim() !== baseline[k].trim());
  useEffect(() => onDirty?.(dirty), [dirty]); // eslint-disable-line react-hooks/exhaustive-deps

  // A new registration loads its examination (or starts empty).
  useEffect(() => {
    setDraft(EMPTY);
    setSaved(null);
    setMsg(null);
    setFieldErrors({});
    if (!token || idKun === null) return;
    let cancelled = false;
    setLoading(true);
    api
      .getPemeriksaan(token, idKun, base)
      .then((res) => {
        if (cancelled) return;
        setSaved(res.pemeriksaan);
        onLoaded?.(res.pemeriksaan);
        if (res.pemeriksaan) setDraft(toDraft(res.pemeriksaan));
        else setMsg({ kind: "info", text: "Belum ada pemeriksaan untuk pendaftaran ini." });
      })
      .catch((err) => {
        if (cancelled) return;
        if (err instanceof ApiError && err.status === 401) expired();
        else setMsg({ kind: "error", text: err instanceof Error ? err.message : "Gagal memuat pemeriksaan" });
      })
      .finally(() => !cancelled && setLoading(false));
    return () => {
      cancelled = true;
    };
  }, [token, idKun]); // eslint-disable-line react-hooks/exhaustive-deps

  const set = (k: keyof Draft) => (e: { target: { value: string } }) => setDraft({ ...draft, [k]: e.target.value });

  async function save() {
    if (!token || idKun === null || readOnly) return;
    if (requireDokter && idDokter === null) {
      setMsg({ kind: "error", text: "Pilih Dokter PJ dulu." });
      return;
    }
    const nums: Partial<Record<NumKey, number | null>> = {};
    const errs: Record<string, string> = {};
    for (const [k, whole] of [
      ["sistole", true],
      ["diastole", true],
      ["beratBadan", true],
      ["tinggiBadan", true],
      ["suhu", false],
    ] as [NumKey, boolean][]) {
      const v = parseNum(draft[k], whole);
      if (v === undefined) errs[k] = whole ? "Angka bulat" : "Angka";
      else nums[k] = v;
    }
    if (Object.keys(errs).length) {
      setFieldErrors(errs);
      setMsg({ kind: "error", text: "Periksa isian yang ditandai." });
      return;
    }
    const input: PemeriksaanInput = {
      keluhanUtama: draft.keluhanUtama,
      anamnesis: draft.anamnesis,
      pemeriksaanFisik: draft.pemeriksaanFisik,
      rekapHasilPenunjang: draft.rekapHasilPenunjang,
      hasilPenunjang: draft.hasilPenunjang,
      rencanaPenatalaksanaan: draft.rencanaPenatalaksanaan,
      hasilPemeriksaanDokter: draft.hasilPemeriksaanDokter,
      tindakanDokter: draft.tindakanDokter,
      sistole: nums.sistole ?? null,
      diastole: nums.diastole ?? null,
      beratBadan: nums.beratBadan ?? null,
      tinggiBadan: nums.tinggiBadan ?? null,
      suhu: nums.suhu ?? null,
      idDokter,
    };
    setBusy(true);
    setFieldErrors({});
    try {
      const res = await api.savePemeriksaan(token, idKun, input, base);
      setSaved(res.pemeriksaan);
      onLoaded?.(res.pemeriksaan);
      if (res.pemeriksaan) setDraft(toDraft(res.pemeriksaan));
      setMsg({ kind: "success", text: `Pemeriksaan pendaftaran ${idKun} tersimpan.` });
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) return expired();
      if (err instanceof ApiError && Object.keys(err.fieldErrors).length) {
        setFieldErrors(err.fieldErrors);
        setMsg({ kind: "error", text: "Periksa isian yang ditandai." });
      } else {
        setMsg({ kind: "error", text: err instanceof Error ? err.message : "Gagal menyimpan pemeriksaan" });
      }
    } finally {
      setBusy(false);
    }
  }

  const err = (k: string) => fieldErrors[k] && <span className="field-error">{fieldErrors[k]}</span>;
  const disabled = idKun === null || loading || busy || readOnly;
  const numInput = (k: NumKey, label: string, step: string) => (
    <input
      id={`tx-${k}`}
      type="number"
      min={0}
      step={step}
      inputMode="decimal"
      aria-label={label}
      placeholder={label}
      aria-invalid={fieldErrors[k] ? "true" : undefined}
      value={draft[k]}
      onChange={set(k)}
      disabled={disabled}
    />
  );

  return (
    <div className="tx-exam" hidden={hidden}>
      <div className="tx-exam-head">
        <p className="muted">
          {idKun === null
            ? "Pilih No. Daftar dulu: pemeriksaan disimpan per pendaftaran."
            : loading
              ? "Memuat pemeriksaan…"
              : saved?.updatedAt
                ? `Terakhir disimpan ${saved.updatedAt}${saved.updatedBy ? ` oleh ${saved.updatedBy}` : ""}${saved.dokter ? ` · Dokter PJ ${saved.dokter}` : ""}`
                : `Pendaftaran ${idKun}`}
        </p>
        <button type="button" className="btn-small btn-primary" onClick={save} disabled={disabled}>
          <Save aria-hidden="true" /> {busy ? "Menyimpan…" : "Simpan Pemeriksaan"}
        </button>
      </div>
      {msg && (
        <p className={`alert ${msg.kind}`} role={msg.kind === "error" ? "alert" : "status"}>
          {msg.text}
        </p>
      )}
      {section !== "dokter" && TEXTS.slice(0, 2).map((t) => (
        <ExamRow key={t.key} id={`tx-${t.key}`} label={t.label}>
          <textarea id={`tx-${t.key}`} rows={2} maxLength={t.max} value={draft[t.key]} onChange={set(t.key)} disabled={disabled} />
          {err(t.key)}
        </ExamRow>
      ))}
      {section !== "dokter" && (
      <>
      <ExamRow id="tx-sistole" label="Tekanan Darah">
        <div className="tx-vitals">
          {numInput("sistole", "Sistole", "1")}
          <span>/</span>
          {numInput("diastole", "Diastole", "1")}
          <span className="unit">mmHg</span>
        </div>
        {err("sistole")}
        {err("diastole")}
      </ExamRow>
      <ExamRow id="tx-beratBadan" label="Berat / Tinggi">
        <div className="tx-vitals">
          {numInput("beratBadan", "Berat", "1")}
          <span className="unit">kg</span>
          {numInput("tinggiBadan", "Tinggi", "1")}
          <span className="unit">cm</span>
        </div>
        {err("beratBadan")}
        {err("tinggiBadan")}
      </ExamRow>
      <ExamRow id="tx-suhu" label="Suhu Badan">
        <div className="tx-vitals">
          {numInput("suhu", "Suhu", "0.1")}
          <span className="unit">°C</span>
        </div>
        {err("suhu")}
      </ExamRow>
      </>
      )}
      {section !== "perawat" && TEXTS.slice(2).map((t) => (
        <ExamRow key={t.key} id={`tx-${t.key}`} label={t.label}>
          <textarea id={`tx-${t.key}`} rows={2} maxLength={t.max} value={draft[t.key]} onChange={set(t.key)} disabled={disabled} />
          {err(t.key)}
        </ExamRow>
      ))}
    </div>
  );
}
