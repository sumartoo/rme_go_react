import { FormEvent, useEffect, useState } from "react";
import { Pencil, Plus } from "lucide-react";
import { api, ApiError, LabMaster } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { FieldShell, Modal, useModal } from "../components/FormKit";

/** Master Data > Lab Master: the groups of lab examinations (reflabmaster). */
export default function LabMasterPage() {
  const { token, logout } = useAuth();
  const [list, setList] = useState<LabMaster[] | null>(null);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<LabMaster | "new" | null>(null);
  const [saved, setSaved] = useState("");

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    api
      .labMaster(token)
      .then((res) => setList(res.labMaster))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const columns: Column<LabMaster>[] = [
    { key: "kode", header: "Lab Master Code", value: (m) => m.kode, className: "dt-wrap" },
    { key: "nama", header: "Lab Master Name", value: (m) => m.nama, className: "dt-wrap dt-grow" },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      className: "dt-action",
      render: (m) => (
        <button type="button" className="row-action" aria-label={`Edit ${m.nama}`} title="Edit" onClick={() => setEditing(m)}>
          <Pencil aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Lab Master</h1>
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
        <DataTable title="Lab Master" columns={columns} rows={list} rowKey={(m) => m.id} initialSort={{ key: "kode", dir: "asc" }} defaultPageSize={10} />
      )}
      {editing && (
        <LabMasterForm
          labMaster={editing === "new" ? null : editing}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(m, isNew) => {
            setList((l) => (isNew ? [...(l ?? []), m] : (l ?? []).map((x) => (x.id === m.id ? m : x))));
            setEditing(null);
            setSaved(`${m.nama} ${isNew ? "added" : "updated"}.`);
          }}
        />
      )}
    </section>
  );
}

function LabMasterForm({
  labMaster,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  labMaster: LabMaster | null;
  onClose: () => void;
  onSaved: (m: LabMaster, isNew: boolean) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [kode, setKode] = useState(labMaster?.kode ?? "");
  const [nama, setNama] = useState(labMaster?.nama ?? "");
  const [fieldErrors, setFieldErrors] = useState<{ kode?: string; nama?: string }>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    const errs: { kode?: string; nama?: string } = {};
    if (!kode.trim()) errs.kode = "Lab Master Code is required";
    if (!nama.trim()) errs.nama = "Lab Master Name is required";
    if (errs.kode || errs.nama) {
      setFieldErrors(errs);
      return;
    }
    setBusy(true);
    setError("");
    try {
      const input = { kode, nama };
      const saved = labMaster ? await api.updateLabMaster(token, labMaster.id, input) : await api.createLabMaster(token, input);
      onSaved(saved, !labMaster);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && (err.fieldErrors.kode || err.fieldErrors.nama)) setFieldErrors({ kode: err.fieldErrors.kode, nama: err.fieldErrors.nama });
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal dialogRef={dialogRef} title={labMaster ? `Edit Lab Master #${labMaster.id}` : "Add Lab Master"} onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <FieldShell id="lab-master-kode" label="Lab Master Code" error={fieldErrors.kode} required>
        <input
          id="lab-master-kode"
          value={kode}
          maxLength={20}
          aria-invalid={fieldErrors.kode ? "true" : undefined}
          onChange={(e) => {
            setKode(e.target.value);
            setFieldErrors((f) => ({ ...f, kode: undefined }));
          }}
        />
      </FieldShell>
      <FieldShell id="lab-master-nama" label="Lab Master Name" error={fieldErrors.nama} required>
        <input
          id="lab-master-nama"
          value={nama}
          maxLength={50}
          aria-invalid={fieldErrors.nama ? "true" : undefined}
          onChange={(e) => {
            setNama(e.target.value);
            setFieldErrors((f) => ({ ...f, nama: undefined }));
          }}
        />
      </FieldShell>
    </Modal>
  );
}
