import { FormEvent, useEffect, useState } from "react";
import { Pencil, Plus } from "lucide-react";
import { api, ApiError, FieldErrors, NonLab } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { AmountInput, FieldShell, Modal, numText, parseAmounts, useModal } from "../components/FormKit";

/** Master Data > Non Lab: services sold with an MCU that are not lab examinations (refnonlab). */
export default function NonLabPage() {
  const { token, logout } = useAuth();
  const [list, setList] = useState<NonLab[] | null>(null);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<NonLab | "new" | null>(null);
  const [saved, setSaved] = useState("");

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    api
      .nonLab(token)
      .then((res) => setList(res.nonLab))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const columns: Column<NonLab>[] = [
    { key: "id", header: "ID", value: (n) => String(n.id), sortValue: (n) => n.id },
    { key: "nama", header: "Non Lab Name", value: (n) => n.nama, className: "dt-wrap dt-grow" },
    // Plain digits, like the original screen (and so exports stay numeric).
    { key: "price", header: "Price", value: (n) => (n.price === null ? "-" : String(n.price)), sortValue: (n) => n.price ?? -1, className: "dt-num" },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      className: "dt-action",
      render: (n) => (
        <button type="button" className="row-action" aria-label={`Edit ${n.nama}`} title="Edit" onClick={() => setEditing(n)}>
          <Pencil aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Non Lab</h1>
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
        <DataTable title="Non Lab" columns={columns} rows={list} rowKey={(n) => n.id} initialSort={{ key: "id", dir: "asc" }} defaultPageSize={20} />
      )}
      {editing && (
        <NonLabForm
          nonLab={editing === "new" ? null : editing}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(n, isNew) => {
            setList((l) => (isNew ? [...(l ?? []), n] : (l ?? []).map((x) => (x.id === n.id ? n : x))));
            setEditing(null);
            setSaved(`${n.nama} ${isNew ? "added" : "updated"}.`);
          }}
        />
      )}
    </section>
  );
}

function NonLabForm({
  nonLab,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  nonLab: NonLab | null;
  onClose: () => void;
  onSaved: (n: NonLab, isNew: boolean) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [nama, setNama] = useState(nonLab?.nama ?? "");
  const [price, setPrice] = useState(numText(nonLab?.price ?? null));
  const [errors, setErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const clear = (key: string) => setErrors(({ [key]: _, ...rest }) => rest);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    const errs: FieldErrors = {};
    if (!nama.trim()) errs.nama = "Non Lab Name is required";
    const amounts = parseAmounts({ price });
    if ("bad" in amounts) errs.price = "Enter a number from 0";
    if (Object.keys(errs).length || "bad" in amounts) {
      setErrors(errs);
      return;
    }
    setBusy(true);
    setError("");
    try {
      const input = { nama, price: amounts.ok.price };
      const saved = nonLab ? await api.updateNonLab(token, nonLab.id, input) : await api.createNonLab(token, input);
      onSaved(saved, !nonLab);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && Object.keys(err.fieldErrors).length) setErrors(err.fieldErrors);
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal dialogRef={dialogRef} title={nonLab ? `Edit Non Lab #${nonLab.id}` : "Add Non Lab"} onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <FieldShell id="non-lab-nama" label="Non Lab Name" error={errors.nama} required>
        <input
          id="non-lab-nama"
          value={nama}
          maxLength={100}
          aria-invalid={errors.nama ? "true" : undefined}
          onChange={(e) => {
            setNama(e.target.value);
            clear("nama");
          }}
        />
      </FieldShell>
      <FieldShell id="non-lab-price" label="Price" error={errors.price}>
        <AmountInput
          id="non-lab-price"
          value={price}
          invalid={!!errors.price}
          onChange={(v) => {
            setPrice(v);
            clear("price");
          }}
        />
      </FieldShell>
    </Modal>
  );
}
