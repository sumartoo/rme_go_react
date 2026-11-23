import { FormEvent, useEffect, useState } from "react";
import { Pencil, Plus } from "lucide-react";
import { api, ApiError, FieldErrors, LabSubMaster, LabSubMasterOption } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { orDash } from "../components/branchView";
import { FieldShell, Modal, useModal } from "../components/FormKit";

/** Master Data > Lab Sub Master: subgroups of lab examinations under a Lab Master (reflabsubmaster). */
export default function LabSubMasterPage() {
  const { token, logout } = useAuth();
  const [list, setList] = useState<LabSubMaster[] | null>(null);
  const [masters, setMasters] = useState<LabSubMasterOption[] | null>(null);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<LabSubMaster | "new" | null>(null);
  const [saved, setSaved] = useState("");

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    Promise.all([api.labSubMaster(token), api.labSubMasterOptions(token)])
      .then(([res, opts]) => {
        setList(res.labSubMaster);
        setMasters(opts.labMaster);
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const columns: Column<LabSubMaster>[] = [
    { key: "labMaster", header: "Lab Master", value: (s) => orDash(s.labMaster), className: "dt-wrap" },
    { key: "kode", header: "Sub Master Code", value: (s) => s.kode, className: "dt-wrap" },
    { key: "nama", header: "Sub Master Name", value: (s) => s.nama, className: "dt-wrap dt-grow" },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      className: "dt-action",
      render: (s) => (
        <button type="button" className="row-action" aria-label={`Edit ${s.nama}`} title="Edit" onClick={() => setEditing(s)}>
          <Pencil aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Lab Sub Master</h1>
        <button type="button" className="btn-add" onClick={() => setEditing("new")} disabled={!list || !masters}>
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
        // Rows arrive ordered by Lab Master, then entry order; the sort is stable, so that order holds within a Lab Master.
        <DataTable title="Lab Sub Master" columns={columns} rows={list} rowKey={(s) => s.id} initialSort={{ key: "labMaster", dir: "asc" }} defaultPageSize={10} />
      )}
      {editing && masters && (
        <LabSubMasterForm
          subMaster={editing === "new" ? null : editing}
          masters={masters}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(s, isNew) => {
            setList((l) => (isNew ? [...(l ?? []), s] : (l ?? []).map((x) => (x.id === s.id ? s : x))));
            setEditing(null);
            setSaved(`${s.nama} ${isNew ? "added" : "updated"}.`);
          }}
        />
      )}
    </section>
  );
}

function LabSubMasterForm({
  subMaster,
  masters,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  subMaster: LabSubMaster | null;
  masters: LabSubMasterOption[];
  onClose: () => void;
  onSaved: (s: LabSubMaster, isNew: boolean) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [idLabMaster, setIdLabMaster] = useState(subMaster?.idLabMaster ? String(subMaster.idLabMaster) : "");
  const [kode, setKode] = useState(subMaster?.kode ?? "");
  const [nama, setNama] = useState(subMaster?.nama ?? "");
  const [errors, setErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const clear = (key: string) => setErrors(({ [key]: _, ...rest }) => rest);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    const errs: FieldErrors = {};
    if (!idLabMaster) errs.idLabMaster = "Lab Master is required";
    if (!kode.trim()) errs.kode = "Sub Master Code is required";
    if (!nama.trim()) errs.nama = "Sub Master Name is required";
    if (Object.keys(errs).length) {
      setErrors(errs);
      return;
    }
    setBusy(true);
    setError("");
    try {
      const input = { idLabMaster: Number(idLabMaster), kode, nama };
      const saved = subMaster ? await api.updateLabSubMaster(token, subMaster.id, input) : await api.createLabSubMaster(token, input);
      onSaved(saved, !subMaster);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && Object.keys(err.fieldErrors).length) setErrors(err.fieldErrors);
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal dialogRef={dialogRef} title={subMaster ? `Edit Lab Sub Master #${subMaster.id}` : "Add Lab Sub Master"} onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <FieldShell id="lab-sub-master-master" label="Lab Master" error={errors.idLabMaster} required>
        <select
          id="lab-sub-master-master"
          value={idLabMaster}
          aria-invalid={errors.idLabMaster ? "true" : undefined}
          onChange={(e) => {
            setIdLabMaster(e.target.value);
            clear("idLabMaster");
          }}
        >
          <option value="">Pilih Lab Master…</option>
          {masters.map((m) => (
            <option key={m.id} value={m.id}>
              {m.nama}
            </option>
          ))}
        </select>
      </FieldShell>
      <FieldShell id="lab-sub-master-kode" label="Sub Master Code" error={errors.kode} required>
        <input
          id="lab-sub-master-kode"
          value={kode}
          maxLength={20}
          aria-invalid={errors.kode ? "true" : undefined}
          onChange={(e) => {
            setKode(e.target.value);
            clear("kode");
          }}
        />
      </FieldShell>
      <FieldShell id="lab-sub-master-nama" label="Sub Master Name" error={errors.nama} required>
        <input
          id="lab-sub-master-nama"
          value={nama}
          maxLength={50}
          aria-invalid={errors.nama ? "true" : undefined}
          onChange={(e) => {
            setNama(e.target.value);
            clear("nama");
          }}
        />
      </FieldShell>
    </Modal>
  );
}
