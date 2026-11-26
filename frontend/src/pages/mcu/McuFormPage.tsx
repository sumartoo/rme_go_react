import { useCallback, useEffect, useRef, useState } from "react";
import { ArrowLeft, ClipboardList, Save, Wand2 } from "lucide-react";
import { api, ApiError, McuHasil, McuLabItem, McuRegistrasi, McuSection } from "../../api";
import { useAuth } from "../../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../../components/DataTable";
import { CatalogForm, catalogFindings, UsgForm, usgFindings } from "./mcuCatalog";
import {
  AnamnesaForm,
  anamnesaFindings,
  KebiasaanForm,
  kebiasaanFindings,
  KeluargaForm,
  keluargaFindings,
  OpnameForm,
  opnameFindings,
  PekerjaanForm,
  pekerjaanFindings,
} from "./mcuCustom";
import { FormProps, Radios, Values } from "./mcuFields";
import {
  AudiometriForm,
  audiometriFindings,
  GigiForm,
  gigiFindings,
  LabForm,
  labFindings,
  SpirometriForm,
  spirometriFindings,
  TreadmillForm,
  treadmillFindings,
} from "./mcuMeasure";

interface FormDef {
  title: string;
  catalog?: boolean; // loads the catalogue sections
  Body: (p: FormProps) => JSX.Element;
  findings: (values: Values, p: { sections: McuSection[]; lab: McuLabItem[]; reg: McuRegistrasi }) => string[];
}

/** The 16 MCU result forms, by slug (backend mcuhasil.Forms). */
export const MCU_FORMS: Record<string, FormDef> = {
  fisik: { title: "Pemeriksaan Fisik", catalog: true, Body: CatalogForm, findings: (x, c) => catalogFindings(x, c.sections) },
  "riwayat-medis": { title: "Riwayat Medis", catalog: true, Body: CatalogForm, findings: (x, c) => catalogFindings(x, c.sections) },
  lab: { title: "Pemeriksaan Lab", Body: LabForm, findings: (x, c) => labFindings(x, c.lab) },
  audiometri: { title: "Pemeriksaan Audiometri", Body: AudiometriForm, findings: (x) => audiometriFindings(x) },
  "riwayat-keluarga": { title: "Riwayat Kesehatan Keluarga", Body: KeluargaForm, findings: (x) => keluargaFindings(x) },
  "khusus-wanita": { title: "Riwayat Khusus Wanita", catalog: true, Body: CatalogForm, findings: (x, c) => catalogFindings(x, c.sections) },
  "opname-operasi": { title: "Riwayat Opname & Operasi", Body: OpnameForm, findings: (x) => opnameFindings(x) },
  kebiasaan: { title: "Kebiasaan Pasien", Body: KebiasaanForm, findings: (x) => kebiasaanFindings(x) },
  pekerjaan: { title: "Riwayat Pekerjaan", Body: PekerjaanForm, findings: (x) => pekerjaanFindings(x) },
  anamnesa: { title: "Anamnesa", Body: AnamnesaForm, findings: (x) => anamnesaFindings(x) },
  gigi: { title: "Pemeriksaan Gigi", Body: GigiForm, findings: (x) => gigiFindings(x) },
  rontgen: { title: "Rontgen Thorax", catalog: true, Body: CatalogForm, findings: (x, c) => catalogFindings(x, c.sections) },
  ekg: { title: "EKG", catalog: true, Body: CatalogForm, findings: (x, c) => catalogFindings(x, c.sections) },
  treadmill: { title: "EKG Treadmill", Body: TreadmillForm, findings: (x, c) => treadmillFindings(x, c.reg.usia) },
  spirometri: { title: "Spirometry", Body: SpirometriForm, findings: (x) => spirometriFindings(x) },
  usg: { title: "USG", catalog: true, Body: UsgForm, findings: (x, c) => usgFindings(x, c.sections) },
};

function useFail() {
  const { logout } = useAuth();
  return useCallback(
    (err: unknown, fallback: string) => {
      if (err instanceof ApiError && err.status === 401) {
        logout("Your session has expired. Please log in again.");
        return "";
      }
      return err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : fallback;
    },
    [logout],
  );
}

const STATUS_LABEL: Record<string, string> = { normal: "Normal", abnormal: "Abnormal", "": "Terisi" };

/**
 * One MCU result form (MCU > Pemeriksaan Fisik … USG): pick an MCU
 * registration, fill the form, conclude and save. Every form shares this page;
 * the body comes from MCU_FORMS.
 */
export default function McuFormPage({ form }: { form: string }) {
  const def = MCU_FORMS[form];
  const [selected, setSelected] = useState<number | null>(null);
  if (!def) return <p className="alert error">Form tidak dikenal.</p>;
  return (
    <section className="panel">
      <div className="page-head">
        <h1>{def.title}</h1>
      </div>
      {selected === null ? <RegList form={form} onPick={setSelected} /> : <Editor key={selected} form={form} def={def} id={selected} onBack={() => setSelected(null)} />}
    </section>
  );
}

function RegList({ form, onPick }: { form: string; onPick: (id: number) => void }) {
  const { token } = useAuth();
  const fail = useFail();
  const [from, setFrom] = useState("");
  const [to, setTo] = useState("");
  const [page, setPage] = useState<{ rows: McuRegistrasi[]; total: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const seq = useRef(0);

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token) return;
      const mine = ++seq.current;
      setLoading(true);
      api
        .mcuRegistrasi(token, form, { q: q.search, page: q.page, size: q.pageSize, from: from || undefined, to: to || undefined })
        .then((r) => {
          if (mine !== seq.current) return;
          setPage(r);
          if (!from) setFrom(r.from);
          if (!to) setTo(r.to);
        })
        .catch((err) => mine === seq.current && setError(fail(err, "Gagal memuat registrasi MCU")))
        .finally(() => mine === seq.current && setLoading(false));
    },
    [token, form, from, to, fail],
  );

  const columns: Column<McuRegistrasi>[] = [
    { key: "tanggal", header: "Tanggal", value: (r) => r.tanggal, sortable: false },
    { key: "id", header: "No. reg", value: (r) => String(r.id), sortable: false },
    { key: "nama", header: "Nama", value: (r) => r.nama, className: "dt-wrap", sortable: false },
    { key: "rm", header: "RM", value: (r) => r.rm, sortable: false },
    { key: "perusahaan", header: "Perusahaan", value: (r) => r.perusahaan || "-", className: "dt-wrap-sm", sortable: false },
    { key: "usia", header: "Usia / JK", value: (r) => `${r.usia ?? "-"} · ${r.gender || "-"}`, sortable: false },
    { key: "status", header: "Registrasi", value: (r) => r.status, sortable: false },
    {
      key: "formStatus",
      header: "Form ini",
      value: (r) => (r.formStatus === null ? "Belum diisi" : STATUS_LABEL[r.formStatus] ?? r.formStatus),
      render: (r) =>
        r.formStatus === null ? (
          <span className="muted">Belum diisi</span>
        ) : (
          <span className={`mcu-badge mcu-badge-${r.formStatus || "isi"}`}>{STATUS_LABEL[r.formStatus] ?? r.formStatus}</span>
        ),
      sortable: false,
    },
    {
      key: "aksi",
      header: "",
      value: () => "",
      exportable: false,
      sortable: false,
      render: (r) => (
        <button type="button" className="btn-small" onClick={() => onPick(r.id)}>
          <ClipboardList aria-hidden="true" /> Isi
        </button>
      ),
    },
  ];

  return (
    <>
      {error && (
        <p className="alert error" role="alert">
          {error}
        </p>
      )}
      <DataTable
        key={`${from}|${to}`}
        title="Registrasi MCU"
        columns={columns}
        rows={page?.rows ?? []}
        rowKey={(r) => r.id}
        defaultPageSize={10}
        exports={false}
        toolbarStart={
          <span className="mcu-dates">
            <label className="dt-length">
              <span>Dari</span>
              <input type="date" className="select" value={from} onChange={(e) => setFrom(e.target.value)} />
            </label>
            <label className="dt-length">
              <span>Sampai</span>
              <input type="date" className="select" value={to} onChange={(e) => setTo(e.target.value)} />
            </label>
          </span>
        }
        remote={{ total: page?.total ?? 0, unfilteredTotal: page?.total ?? 0, loading: loading || !page, onQuery: fetchPage }}
      />
    </>
  );
}

const sectionCache: Record<string, McuSection[]> = {};

function Editor({ form, def, id, onBack }: { form: string; def: FormDef; id: number; onBack: () => void }) {
  const { token } = useAuth();
  const fail = useFail();
  const [reg, setReg] = useState<McuRegistrasi | null>(null);
  const [sections, setSections] = useState<McuSection[]>(sectionCache[form] ?? []);
  const [lab, setLab] = useState<McuLabItem[]>([]);
  const [values, setValues] = useState<Values>({});
  const [status, setStatus] = useState("");
  const [kesimpulan, setKesimpulan] = useState("");
  const [saved, setSaved] = useState<McuHasil | null>(null);
  const [dirty, setDirty] = useState(false);
  const [busy, setBusy] = useState(false);
  const [notice, setNotice] = useState<{ kind: "success" | "error"; text: string } | null>(null);

  useEffect(() => {
    if (!token) return;
    const loads: Promise<unknown>[] = [
      api.mcuHasil(token, form, id).then((r) => {
        setReg(r.registrasi);
        setLab(r.lab ?? []);
        setValues(r.hasil.values ?? {});
        setStatus(r.hasil.status);
        setKesimpulan(r.hasil.kesimpulan);
        setSaved(r.hasil.updatedAt ? r.hasil : null);
      }),
    ];
    if (def.catalog && !sectionCache[form]) {
      loads.push(
        api.mcuKatalog(token, form).then((k) => {
          sectionCache[form] = k.sections;
          setSections(k.sections);
        }),
      );
    }
    Promise.all(loads).catch((err) => setNotice({ kind: "error", text: fail(err, "Gagal memuat form") }));
  }, [token, form, id]); // eslint-disable-line react-hooks/exhaustive-deps

  useEffect(() => {
    if (!dirty) return;
    const warn = (e: BeforeUnloadEvent) => e.preventDefault();
    window.addEventListener("beforeunload", warn);
    return () => window.removeEventListener("beforeunload", warn);
  }, [dirty]);

  const set = (key: string, value: string, note?: string) => {
    setDirty(true);
    setValues((cur) => ({ ...cur, [key]: { value, note: note ?? cur[key]?.note ?? "" } }));
  };
  const setNote = (key: string, note: string) => {
    setDirty(true);
    setValues((cur) => ({ ...cur, [key]: { value: cur[key]?.value ?? "", note } }));
  };

  if (!reg) {
    return notice ? (
      <>
        <p className={`alert ${notice.kind}`} role="alert">
          {notice.text}
        </p>
        <button type="button" className="btn-add" onClick={onBack}>
          <ArrowLeft aria-hidden="true" /> Kembali
        </button>
      </>
    ) : (
      <p className="muted">Memuat…</p>
    );
  }

  const ctx = { sections, lab, reg };
  const findings = def.findings(values, ctx);
  const props: FormProps = { values, set, setNote, reg, sections, lab };
  const cancelled = reg.status === "Cancel";

  const autoConclude = () => {
    setDirty(true);
    if (findings.length === 0) {
      setStatus("normal");
      setKesimpulan("Dalam batas normal.");
    } else {
      setStatus("abnormal");
      setKesimpulan(findings.map((f) => `- ${f}`).join("\n"));
    }
  };

  const save = () => {
    if (!token) return;
    setBusy(true);
    setNotice(null);
    api
      .mcuSave(token, form, id, { status, kesimpulan, values })
      .then((r) => {
        setSaved(r.hasil);
        setDirty(false);
        setNotice({ kind: "success", text: "Tersimpan." });
      })
      .catch((err) => setNotice({ kind: "error", text: fail(err, "Gagal menyimpan") }))
      .finally(() => setBusy(false));
  };

  const back = () => {
    if (dirty && !window.confirm("Perubahan belum disimpan. Tinggalkan form ini?")) return;
    onBack();
  };

  return (
    <div className="mcu-editor">
      <div className="mcu-patient">
        <button type="button" className="btn-add" onClick={back}>
          <ArrowLeft aria-hidden="true" /> Daftar registrasi
        </button>
        <div>
          <strong>{reg.nama}</strong>
          <span className="muted">
            {" "}
            · No. reg {reg.id} · RM {reg.rm} · {reg.usia ?? "-"} th · {reg.gender || "-"} · {reg.tanggal}
          </span>
          <div className="muted">
            {reg.perusahaan || "-"}
            {reg.departemen ? ` · ${reg.departemen}` : ""}
            {reg.jabatan ? ` · ${reg.jabatan}` : ""}
          </div>
        </div>
        {cancelled && <p className="alert error">Registrasi ini dibatalkan; hasil tidak bisa disimpan.</p>}
      </div>

      <fieldset className="mcu-body" disabled={cancelled}>
        <def.Body {...props} />
      </fieldset>

      <section className="mcu-box mcu-conclude">
        <header className="mcu-box-head">
          <h3>Kesimpulan</h3>
          <button type="button" className="btn-small" onClick={autoConclude} disabled={cancelled} title="Susun dari temuan di atas">
            <Wand2 aria-hidden="true" /> Susun otomatis
          </button>
        </header>
        <p className="muted">
          {findings.length === 0 ? "Belum ada temuan abnormal pada isian." : `${findings.length} temuan: ${findings.slice(0, 3).join("; ")}${findings.length > 3 ? "; …" : ""}`}
        </p>
        <div className="mcu-field">
          <span className="mcu-label">Status</span>
          <div className="mcu-control">
            <Radios
              label="Status"
              value={status === "normal" ? "Normal" : status === "abnormal" ? "Abnormal" : ""}
              options={["Normal", "Abnormal"]}
              onChange={(x) => {
                setDirty(true);
                setStatus(x === "Normal" ? "normal" : x === "Abnormal" ? "abnormal" : "");
              }}
              abnormal={status === "abnormal"}
            />
          </div>
        </div>
        <textarea
          className="select mcu-input mcu-wide"
          rows={4}
          maxLength={4000}
          aria-label="Kesimpulan"
          value={kesimpulan}
          disabled={cancelled}
          onChange={(e) => {
            setDirty(true);
            setKesimpulan(e.target.value);
          }}
        />
        {notice && (
          <p className={`alert ${notice.kind}`} role={notice.kind === "error" ? "alert" : "status"}>
            {notice.text}
          </p>
        )}
        <div className="mcu-actions">
          <button type="button" className="btn-add btn-primary-add" onClick={save} disabled={busy || cancelled}>
            <Save aria-hidden="true" /> {busy ? "Menyimpan…" : "Simpan"}
          </button>
          {saved?.updatedAt && (
            <span className="muted">
              Terakhir disimpan {new Date(saved.updatedAt).toLocaleString("id-ID")} oleh {saved.updatedBy || "-"}
            </span>
          )}
          {dirty && <span className="mcu-dirty">Ada perubahan belum disimpan</span>}
        </div>
      </section>
    </div>
  );
}
