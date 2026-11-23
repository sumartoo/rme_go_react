import { FormEvent, useEffect, useState } from "react";
import { Pencil, Plus } from "lucide-react";
import { api, ApiError, FieldErrors, Perusahaan, PerusahaanInput } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { orDash } from "../components/branchView";
import { FieldShell, Modal, useModal } from "../components/FormKit";

/** Master Data > Perusahaan: the client companies (refcompany). */
export default function PerusahaanPage() {
  const { token, logout } = useAuth();
  const [list, setList] = useState<Perusahaan[] | null>(null);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<Perusahaan | "new" | null>(null);
  const [saved, setSaved] = useState("");

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    api
      .perusahaan(token)
      .then((res) => setList(res.perusahaan))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load companies");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const columns: Column<Perusahaan>[] = [
    { key: "kode", header: "Kode Perusahaan", value: (p) => p.kode },
    { key: "nama", header: "Nama Perusahaan", value: (p) => p.nama, className: "dt-wrap" },
    { key: "alamat", header: "Alamat", value: (p) => orDash(p.alamat), className: "dt-wrap dt-grow" },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      className: "dt-action",
      render: (p) => (
        <button type="button" className="row-action" aria-label={`Edit ${p.nama}`} title="Edit" onClick={() => setEditing(p)}>
          <Pencil aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Perusahaan</h1>
        <button type="button" className="btn-add" onClick={() => setEditing("new")} disabled={!list}>
          <Plus aria-hidden="true" /> Add
        </button>
      </div>
      {saved && (
        <p className="alert success" role="status">
          {saved}
        </p>
      )}
      {loadError ? (
        <p className="alert error" role="alert">
          {loadError}
        </p>
      ) : !list ? (
        <p className="muted">Loading…</p>
      ) : (
        <DataTable title="Perusahaan" columns={columns} rows={list} rowKey={(p) => p.id} initialSort={{ key: "kode", dir: "asc" }} defaultPageSize={10} />
      )}
      {editing && (
        <PerusahaanForm
          perusahaan={editing === "new" ? null : editing}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(p, isNew) => {
            setList((l) => (isNew ? [...(l ?? []), p] : (l ?? []).map((x) => (x.id === p.id ? p : x))));
            setEditing(null);
            setSaved(`${p.nama} ${isNew ? "added" : "updated"}.`);
          }}
        />
      )}
    </section>
  );
}

type TextKey = Exclude<keyof PerusahaanInput, "isVat">;

function PerusahaanForm({
  perusahaan,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  perusahaan: Perusahaan | null;
  onClose: () => void;
  onSaved: (p: Perusahaan, isNew: boolean) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [form, setForm] = useState<PerusahaanInput>(() => ({
    kode: perusahaan?.kode ?? "",
    nama: perusahaan?.nama ?? "",
    alamat: perusahaan?.alamat ?? "",
    contactPerson: perusahaan?.contactPerson ?? "",
    email: perusahaan?.email ?? "",
    telepon: perusahaan?.telepon ?? "",
    fax: perusahaan?.fax ?? "",
    isVat: perusahaan?.isVat ?? "",
  }));
  const [errors, setErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    const errs: FieldErrors = {};
    if (!form.kode.trim()) errs.kode = "Kode Perusahaan is required";
    if (!form.nama.trim()) errs.nama = "Nama Perusahaan is required";
    if (Object.keys(errs).length) {
      setErrors(errs);
      return;
    }
    setBusy(true);
    setError("");
    try {
      const saved = perusahaan ? await api.updatePerusahaan(token, perusahaan.id, form) : await api.createPerusahaan(token, form);
      onSaved(saved, !perusahaan);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && Object.keys(err.fieldErrors).length) setErrors(err.fieldErrors);
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  const clear = (key: keyof PerusahaanInput) => setErrors(({ [key]: _, ...rest }) => rest);

  // max matches the varchar size of the refcompany column.
  const field = (key: TextKey, label: string, max: number, extra: { required?: boolean; wide?: boolean; type?: string } = {}) => (
    <FieldShell id={`perusahaan-${key}`} label={label} error={errors[key]} required={extra.required} wide={extra.wide}>
      <input
        id={`perusahaan-${key}`}
        type={extra.type ?? "text"}
        value={form[key]}
        maxLength={max}
        aria-invalid={errors[key] ? "true" : undefined}
        onChange={(e) => {
          setForm((f) => ({ ...f, [key]: e.target.value }));
          clear(key);
        }}
      />
    </FieldShell>
  );

  return (
    <Modal
      dialogRef={dialogRef}
      title={perusahaan ? `Edit Perusahaan ${perusahaan.kode}` : "Add Perusahaan"}
      onClose={onClose}
      onSubmit={submit}
      error={error}
      busy={busy}
    >
      <div className="form-grid">
        {field("kode", "Kode Perusahaan", 10, { required: true })}
        {field("nama", "Nama Perusahaan", 255, { required: true })}
        {field("alamat", "Alamat", 255, { wide: true })}
        {field("contactPerson", "Contact Person", 100)}
        {field("email", "Email", 100)}
        {field("telepon", "Telepon", 30)}
        {field("fax", "Fax", 30)}
        <FieldShell id="perusahaan-isVat" label="PPN" error={errors.isVat}>
          <select
            id="perusahaan-isVat"
            value={form.isVat}
            onChange={(e) => {
              setForm((f) => ({ ...f, isVat: e.target.value }));
              clear("isVat");
            }}
          >
            <option value="">-</option>
            <option value="1">Ya</option>
            <option value="0">Tidak</option>
          </select>
        </FieldShell>
      </div>
    </Modal>
  );
}
