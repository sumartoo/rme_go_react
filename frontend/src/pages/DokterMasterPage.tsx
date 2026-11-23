import { FormEvent, useEffect, useState } from "react";
import { Pencil, Plus } from "lucide-react";
import { api, ApiError, DokterMaster } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { FieldShell, Modal, useModal } from "../components/FormKit";

/** Master Data > Dokter Master: the kinds of MCU doctor (refdoctormaster). */
export default function DokterMasterPage() {
  const { token, logout } = useAuth();
  const [list, setList] = useState<DokterMaster[] | null>(null);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<DokterMaster | "new" | null>(null);
  const [saved, setSaved] = useState("");

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    api
      .dokterMaster(token)
      .then((res) => setList(res.dokterMaster))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const columns: Column<DokterMaster>[] = [
    { key: "nama", header: "Dokter Master Name", value: (d) => d.nama, className: "dt-wrap dt-grow" },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      className: "dt-action",
      render: (d) => (
        <button type="button" className="row-action" aria-label={`Edit ${d.nama}`} title="Edit" onClick={() => setEditing(d)}>
          <Pencil aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Dokter Master</h1>
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
        <DataTable title="Dokter Master" columns={columns} rows={list} rowKey={(d) => d.id} initialSort={{ key: "nama", dir: "asc" }} defaultPageSize={10} />
      )}
      {editing && (
        <DokterMasterForm
          dokterMaster={editing === "new" ? null : editing}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(d, isNew) => {
            setList((l) => (isNew ? [...(l ?? []), d] : (l ?? []).map((x) => (x.id === d.id ? d : x))));
            setEditing(null);
            setSaved(`${d.nama} ${isNew ? "added" : "updated"}.`);
          }}
        />
      )}
    </section>
  );
}

function DokterMasterForm({
  dokterMaster,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  dokterMaster: DokterMaster | null;
  onClose: () => void;
  onSaved: (d: DokterMaster, isNew: boolean) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [nama, setNama] = useState(dokterMaster?.nama ?? "");
  const [nameError, setNameError] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    if (!nama.trim()) {
      setNameError("Dokter Master Name is required");
      return;
    }
    setBusy(true);
    setError("");
    try {
      const saved = dokterMaster ? await api.updateDokterMaster(token, dokterMaster.id, nama) : await api.createDokterMaster(token, nama);
      onSaved(saved, !dokterMaster);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && err.fieldErrors.nama) setNameError(err.fieldErrors.nama);
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal
      dialogRef={dialogRef}
      title={dokterMaster ? `Edit Dokter Master #${dokterMaster.id}` : "Add Dokter Master"}
      onClose={onClose}
      onSubmit={submit}
      error={error}
      busy={busy}
    >
      <FieldShell id="dokter-master-nama" label="Dokter Master Name" error={nameError} required>
        <input
          id="dokter-master-nama"
          value={nama}
          maxLength={50}
          aria-invalid={nameError ? "true" : undefined}
          onChange={(e) => {
            setNama(e.target.value);
            setNameError("");
          }}
        />
      </FieldShell>
    </Modal>
  );
}
