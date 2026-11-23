import { FormEvent, useCallback, useEffect, useRef, useState } from "react";
import { Pencil, Plus } from "lucide-react";
import { api, ApiError, FieldErrors, PasienMcu, PasienMcuCompany, PasienMcuInput } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { orDash } from "../components/branchView";
import { FieldShell, Modal, useModal } from "../components/FormKit";
import { fmtDate, today } from "../components/dates";

/** Rows an export may fetch (the API caps one request at this size). */
const EXPORT_LIMIT = 10000;

const GENDER: Record<string, string> = { L: "Laki-Laki", P: "Perempuan" };

/** Master Data > Pasien: the MCU patients (refpasien), paged on the server. */
export default function PasienMcuPage() {
  const { token, logout } = useAuth();
  const [companies, setCompanies] = useState<PasienMcuCompany[] | null>(null);
  const [page, setPage] = useState<{ rows: PasienMcu[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<PasienMcu | "new" | null>(null);
  const [notice, setNotice] = useState<{ kind: "success" | "info" | "error"; text: string } | null>(null);
  const lastQuery = useRef<RemoteQuery | null>(null);
  const seq = useRef(0);

  const expired = useCallback(() => logout("Your session has expired. Please log in again."), [logout]);

  useEffect(() => {
    if (!token) return;
    api
      .pasienMcuOptions(token)
      .then((o) => setCompanies(o.perusahaan))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const params = (q: RemoteQuery, size = q.pageSize, pageNo = q.page) => ({
    page: pageNo,
    size,
    q: q.search,
    sort: q.sort?.key,
    dir: q.sort?.dir,
  });

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token) return;
      lastQuery.current = q;
      const mine = ++seq.current;
      setLoading(true);
      api
        .pasienMcu(token, params(q))
        .then((res) => mine === seq.current && setPage(res))
        .catch((err) => {
          if (mine !== seq.current) return;
          if (err instanceof ApiError && err.status === 401) expired();
          else setNotice({ kind: "error", text: err instanceof Error ? err.message : "Failed to load patients" });
        })
        .finally(() => mine === seq.current && setLoading(false));
    },
    [token], // eslint-disable-line react-hooks/exhaustive-deps
  );

  const fetchAll = async (q: RemoteQuery): Promise<PasienMcu[]> => {
    if (!token) return [];
    const res = await api.pasienMcu(token, params(q, EXPORT_LIMIT, 1));
    if (res.total > EXPORT_LIMIT) {
      setNotice({
        kind: "info",
        text: `The export holds the first ${EXPORT_LIMIT.toLocaleString("en-US")} of ${res.total.toLocaleString("en-US")} matching patients. Narrow the search to export the rest.`,
      });
    }
    return res.rows;
  };

  const columns: Column<PasienMcu>[] = [
    { key: "rm", header: "No RM", value: (p) => p.rm },
    { key: "nama", header: "Nama", value: (p) => p.nama, className: "dt-wrap" },
    { key: "gender", header: "Jenis Kelamin", value: (p) => GENDER[p.gender] ?? orDash(p.gender) },
    { key: "tglLahir", header: "Tanggal Lahir", value: (p) => fmtDate(p.tglLahir) },
    { key: "telp", header: "No Telp", value: (p) => orDash(p.telp) },
    { key: "hp", header: "No Hp", value: (p) => orDash(p.hp) },
    { key: "alamat", header: "Alamat", value: (p) => orDash(p.alamat), className: "dt-wrap" },
    { key: "perusahaan", header: "Perusahaan", value: (p) => orDash(p.perusahaan || p.kodePerusahaan), className: "dt-wrap" },
    { key: "nik", header: "NIK", value: (p) => orDash(p.nik) },
    { key: "email", header: "Email", value: (p) => orDash(p.email) },
    { key: "departemen", header: "Departmen", value: (p) => orDash(p.departemen), className: "dt-wrap" },
    { key: "posisi", header: "Posisi", value: (p) => orDash(p.posisi), className: "dt-wrap" },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      className: "dt-action",
      render: (p) => (
        <button type="button" className="row-action" aria-label={`Edit ${p.rm}`} title="Edit" onClick={() => setEditing(p)}>
          <Pencil aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Pasien</h1>
        <button type="button" className="btn-add" onClick={() => setEditing("new")} disabled={!companies}>
          <Plus aria-hidden="true" /> Add
        </button>
      </div>
      {notice && (
        <p className={`alert ${notice.kind}`} role={notice.kind === "error" ? "alert" : "status"}>
          {notice.text}
        </p>
      )}
      {loadError ? (
        <p className="alert error" role="alert">
          {loadError}
        </p>
      ) : (
        <DataTable
          title="Pasien"
          columns={columns}
          rows={page?.rows ?? []}
          rowKey={(p) => p.id}
          initialSort={{ key: "rm", dir: "asc" }}
          defaultPageSize={10}
          remote={{
            total: page?.total ?? 0,
            unfilteredTotal: page?.unfilteredTotal ?? 0,
            loading: loading || !page,
            onQuery: fetchPage,
            fetchAll,
          }}
        />
      )}
      {editing && companies && (
        <PasienMcuForm
          pasien={editing === "new" ? null : editing}
          companies={companies}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(p, isNew) => {
            setEditing(null);
            setNotice({ kind: "success", text: `${p.nama} (${p.rm}) ${isNew ? "added" : "updated"}.` });
            if (isNew) lastQuery.current && fetchPage(lastQuery.current);
            else setPage((pg) => pg && { ...pg, rows: pg.rows.map((x) => (x.id === p.id ? p : x)) });
          }}
        />
      )}
    </section>
  );
}

type TextKey = Exclude<keyof PasienMcuInput, "gender" | "kodePerusahaan" | "tglLahir">;

function PasienMcuForm({
  pasien,
  companies,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  pasien: PasienMcu | null;
  companies: PasienMcuCompany[];
  onClose: () => void;
  onSaved: (p: PasienMcu, isNew: boolean) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [form, setForm] = useState<PasienMcuInput>(() => ({
    nama: pasien?.nama ?? "",
    gender: pasien?.gender ?? "",
    tglLahir: pasien?.tglLahir ?? "",
    telp: pasien?.telp ?? "",
    hp: pasien?.hp ?? "",
    alamat: pasien?.alamat ?? "",
    kodePerusahaan: pasien?.kodePerusahaan ?? "",
    nik: pasien?.nik ?? "",
    email: pasien?.email ?? "",
    departemen: pasien?.departemen ?? "",
    posisi: pasien?.posisi ?? "",
  }));
  const [errors, setErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const set = (key: keyof PasienMcuInput, value: string) => {
    setForm((f) => ({ ...f, [key]: value }));
    setErrors(({ [key]: _, ...rest }) => rest);
  };

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    const errs: FieldErrors = {};
    if (!form.nama.trim()) errs.nama = "Nama is required";
    if (!form.gender) errs.gender = "Choose Laki-Laki or Perempuan";
    if (!form.kodePerusahaan) errs.kodePerusahaan = "Perusahaan is required";
    if (Object.keys(errs).length) {
      setErrors(errs);
      return;
    }
    setBusy(true);
    setError("");
    try {
      const saved = pasien ? await api.updatePasienMcu(token, pasien.id, form) : await api.createPasienMcu(token, form);
      onSaved(saved, !pasien);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && Object.keys(err.fieldErrors).length) setErrors(err.fieldErrors);
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  // max matches the varchar size of the refpasien column.
  const field = (key: TextKey, label: string, max: number, extra: { required?: boolean; wide?: boolean; type?: string } = {}) => (
    <FieldShell id={`pasien-mcu-${key}`} label={label} error={errors[key]} required={extra.required} wide={extra.wide}>
      <input
        id={`pasien-mcu-${key}`}
        type={extra.type ?? "text"}
        value={form[key]}
        maxLength={max}
        aria-invalid={errors[key] ? "true" : undefined}
        onChange={(e) => set(key, e.target.value)}
      />
    </FieldShell>
  );

  // A patient may point to a company code that no longer exists; keep it selectable.
  const missing = form.kodePerusahaan && !companies.some((c) => c.kode === form.kodePerusahaan);

  return (
    <Modal
      dialogRef={dialogRef}
      title={pasien ? `Edit Pasien ${pasien.rm}` : "Add Pasien"}
      onClose={onClose}
      onSubmit={submit}
      error={error}
      busy={busy}
    >
      <div className="form-grid">
        {!pasien && <p className="wide-note muted">No RM is given automatically when the patient is saved.</p>}
        {field("nama", "Nama", 100, { required: true, wide: true })}
        <FieldShell id="pasien-mcu-gender" label="Jenis Kelamin" error={errors.gender} required>
          <select id="pasien-mcu-gender" value={form.gender} aria-invalid={errors.gender ? "true" : undefined} onChange={(e) => set("gender", e.target.value)}>
            <option value="">Pilih…</option>
            <option value="L">Laki-Laki</option>
            <option value="P">Perempuan</option>
          </select>
        </FieldShell>
        <FieldShell id="pasien-mcu-tglLahir" label="Tanggal Lahir" error={errors.tglLahir}>
          <input
            id="pasien-mcu-tglLahir"
            type="date"
            value={form.tglLahir}
            max={today()}
            aria-invalid={errors.tglLahir ? "true" : undefined}
            onChange={(e) => set("tglLahir", e.target.value)}
          />
        </FieldShell>
        <FieldShell id="pasien-mcu-kodePerusahaan" label="Perusahaan" error={errors.kodePerusahaan} required wide>
          <select
            id="pasien-mcu-kodePerusahaan"
            value={form.kodePerusahaan}
            aria-invalid={errors.kodePerusahaan ? "true" : undefined}
            onChange={(e) => set("kodePerusahaan", e.target.value)}
          >
            <option value="">Pilih perusahaan…</option>
            {missing && <option value={form.kodePerusahaan}>{form.kodePerusahaan}</option>}
            {companies.map((c) => (
              <option key={c.kode} value={c.kode}>
                {c.nama} ({c.kode})
              </option>
            ))}
          </select>
        </FieldShell>
        {field("alamat", "Alamat", 200, { wide: true })}
        {field("telp", "No Telp", 15)}
        {field("hp", "No Hp", 15)}
        {field("nik", "NIK", 20)}
        {field("email", "Email", 50)}
        {field("departemen", "Departmen", 50)}
        {field("posisi", "Posisi", 50)}
      </div>
    </Modal>
  );
}
