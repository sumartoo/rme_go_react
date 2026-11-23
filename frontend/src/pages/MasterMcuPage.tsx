import { FormEvent, useEffect, useState } from "react";
import { Pencil, Plus } from "lucide-react";
import { api, ApiError, MasterMcu } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { FieldShell, Modal, useModal } from "../components/FormKit";

/** Master Data > MCU: the MCU examination groups (refmastermcu). */
export default function MasterMcuPage() {
  const { token, logout } = useAuth();
  const [list, setList] = useState<MasterMcu[] | null>(null);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<MasterMcu | "new" | null>(null);
  const [saved, setSaved] = useState("");

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    api
      .masterMcu(token)
      .then((res) => setList(res.mcu))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const columns: Column<MasterMcu>[] = [
    { key: "id", header: "ID", value: (m) => String(m.id), sortValue: (m) => m.id },
    { key: "kode", header: "Kode Mcu", value: (m) => m.kode },
    { key: "nama", header: "Nama Mcu", value: (m) => m.nama, className: "dt-wrap dt-grow" },
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
        <h1>MCU</h1>
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
        <DataTable title="MCU" columns={columns} rows={list} rowKey={(m) => m.id} initialSort={{ key: "id", dir: "asc" }} defaultPageSize={10} />
      )}
      {editing && (
        <McuForm
          mcu={editing === "new" ? null : editing}
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

function McuForm({
  mcu,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  mcu: MasterMcu | null;
  onClose: () => void;
  onSaved: (m: MasterMcu, isNew: boolean) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [kode, setKode] = useState(mcu?.kode ?? "");
  const [nama, setNama] = useState(mcu?.nama ?? "");
  const [fieldErrors, setFieldErrors] = useState<{ kode?: string; nama?: string }>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    const errs: { kode?: string; nama?: string } = {};
    if (!kode.trim()) errs.kode = "Kode Mcu is required";
    if (!nama.trim()) errs.nama = "Nama Mcu is required";
    if (errs.kode || errs.nama) {
      setFieldErrors(errs);
      return;
    }
    setBusy(true);
    setError("");
    try {
      const input = { kode, nama };
      const saved = mcu ? await api.updateMasterMcu(token, mcu.id, input) : await api.createMasterMcu(token, input);
      onSaved(saved, !mcu);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && (err.fieldErrors.kode || err.fieldErrors.nama)) setFieldErrors({ kode: err.fieldErrors.kode, nama: err.fieldErrors.nama });
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal dialogRef={dialogRef} title={mcu ? `Edit MCU #${mcu.id}` : "Add MCU"} onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <FieldShell id="mcu-kode" label="Kode Mcu" error={fieldErrors.kode} required>
        <input
          id="mcu-kode"
          value={kode}
          maxLength={10}
          aria-invalid={fieldErrors.kode ? "true" : undefined}
          onChange={(e) => {
            setKode(e.target.value);
            setFieldErrors((f) => ({ ...f, kode: undefined }));
          }}
        />
      </FieldShell>
      <FieldShell id="mcu-nama" label="Nama Mcu" error={fieldErrors.nama} required>
        <input
          id="mcu-nama"
          value={nama}
          maxLength={30}
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
