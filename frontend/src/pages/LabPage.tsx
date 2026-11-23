import { FormEvent, useEffect, useMemo, useState } from "react";
import { Pencil, Plus } from "lucide-react";
import { api, ApiError, FieldErrors, Lab, LabSubMasterChoice } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { orDash } from "../components/branchView";
import { AmountInput, FieldShell, Modal, numText, parseAmounts, useModal } from "../components/FormKit";

const statusText = (l: Lab) => (l.isActive ? "Active" : "Non Active");

/** Master Data > Lab: the lab examinations (reflab), under a Sub Master and Lab Master. */
export default function LabPage() {
  const { token, logout } = useAuth();
  const [list, setList] = useState<Lab[] | null>(null);
  const [subMasters, setSubMasters] = useState<LabSubMasterChoice[] | null>(null);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<Lab | "new" | null>(null);
  const [saved, setSaved] = useState("");

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    Promise.all([api.lab(token), api.labOptions(token)])
      .then(([res, opts]) => {
        setList(res.lab);
        setSubMasters(opts.labSubMaster);
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const columns: Column<Lab>[] = [
    { key: "id", header: "Lab Code", value: (l) => String(l.id), sortValue: (l) => l.id },
    { key: "labMaster", header: "Lab Master", value: (l) => orDash(l.labMaster), className: "dt-wrap" },
    { key: "labSubMaster", header: "Sub Master Name", value: (l) => orDash(l.labSubMaster), className: "dt-wrap" },
    { key: "nama", header: "Lab", value: (l) => l.nama, className: "dt-wrap dt-grow" },
    {
      key: "status",
      header: "Status",
      value: statusText,
      render: (l) => <span className={l.isActive ? "status on" : "status off"}>{statusText(l)}</span>,
    },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      className: "dt-action",
      render: (l) => (
        <button type="button" className="row-action" aria-label={`Edit ${l.nama}`} title="Edit" onClick={() => setEditing(l)}>
          <Pencil aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Lab</h1>
        <button type="button" className="btn-add" onClick={() => setEditing("new")} disabled={!list || !subMasters}>
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
        <DataTable title="Lab" columns={columns} rows={list} rowKey={(l) => l.id} initialSort={{ key: "id", dir: "asc" }} defaultPageSize={10} />
      )}
      {editing && subMasters && (
        <LabForm
          lab={editing === "new" ? null : editing}
          subMasters={subMasters}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(l, isNew) => {
            setList((ls) => (isNew ? [...(ls ?? []), l] : (ls ?? []).map((x) => (x.id === l.id ? l : x))));
            setEditing(null);
            setSaved(`${l.nama} ${isNew ? "added" : "updated"}.`);
          }}
        />
      )}
    </section>
  );
}

function LabForm({
  lab,
  subMasters,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  lab: Lab | null;
  subMasters: LabSubMasterChoice[];
  onClose: () => void;
  onSaved: (l: Lab, isNew: boolean) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [idLabSubMaster, setIdLabSubMaster] = useState(lab?.idLabSubMaster ? String(lab.idLabSubMaster) : "");
  const [kode, setKode] = useState(lab?.kode ?? "");
  const [nama, setNama] = useState(lab?.nama ?? "");
  const [price, setPrice] = useState(numText(lab?.price ?? null));
  // A new examination starts with lab's default fee of 15.
  const [fee, setFee] = useState(lab ? numText(lab.fee) : "15");
  const [isActive, setIsActive] = useState(lab ? lab.isActive : true);
  const [errors, setErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  // The Sub Master list grouped under each Lab Master.
  const groups = useMemo(() => {
    const m = new Map<string, LabSubMasterChoice[]>();
    for (const s of subMasters) m.set(s.labMaster || "-", [...(m.get(s.labMaster || "-") ?? []), s]);
    return [...m.entries()];
  }, [subMasters]);
  // The row's Sub Master may be soft-deleted; keep it selectable.
  const missing = lab && idLabSubMaster && !subMasters.some((s) => String(s.id) === idLabSubMaster);

  const clear = (key: string) => setErrors(({ [key]: _, ...rest }) => rest);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    const errs: FieldErrors = {};
    if (!idLabSubMaster) errs.idLabSubMaster = "Sub Master is required";
    if (!nama.trim()) errs.nama = "Lab is required";
    const amounts = parseAmounts({ price, fee });
    if ("bad" in amounts) errs[amounts.bad] = "Enter a number from 0";
    if (Object.keys(errs).length || "bad" in amounts) {
      setErrors(errs);
      return;
    }
    setBusy(true);
    setError("");
    try {
      const input = { idLabSubMaster: Number(idLabSubMaster), kode, nama, price: amounts.ok.price, fee: amounts.ok.fee, isActive };
      const saved = lab ? await api.updateLab(token, lab.id, input) : await api.createLab(token, input);
      onSaved(saved, !lab);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && Object.keys(err.fieldErrors).length) setErrors(err.fieldErrors);
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal dialogRef={dialogRef} title={lab ? `Edit Lab #${lab.id}` : "Add Lab"} onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <div className="form-grid">
        <FieldShell id="lab-sub-master" label="Sub Master" error={errors.idLabSubMaster} required wide>
          <select
            id="lab-sub-master"
            value={idLabSubMaster}
            aria-invalid={errors.idLabSubMaster ? "true" : undefined}
            onChange={(e) => {
              setIdLabSubMaster(e.target.value);
              clear("idLabSubMaster");
            }}
          >
            <option value="">Pilih Sub Master…</option>
            {missing && (
              <option value={idLabSubMaster}>
                {lab.labMaster} / {lab.labSubMaster}
              </option>
            )}
            {groups.map(([master, subs]) => (
              <optgroup key={master} label={master}>
                {subs.map((s) => (
                  <option key={s.id} value={s.id}>
                    {s.nama}
                  </option>
                ))}
              </optgroup>
            ))}
          </select>
        </FieldShell>
        <FieldShell id="lab-nama" label="Lab" error={errors.nama} required wide>
          <input
            id="lab-nama"
            value={nama}
            maxLength={50}
            aria-invalid={errors.nama ? "true" : undefined}
            onChange={(e) => {
              setNama(e.target.value);
              clear("nama");
            }}
          />
        </FieldShell>
        <FieldShell id="lab-kode" label="Kode" error={errors.kode} hint="Empty = same as Lab" wide>
          <input
            id="lab-kode"
            value={kode}
            maxLength={50}
            aria-invalid={errors.kode ? "true" : undefined}
            onChange={(e) => {
              setKode(e.target.value);
              clear("kode");
            }}
          />
        </FieldShell>
        <FieldShell id="lab-price" label="Price" error={errors.price}>
          <AmountInput
            id="lab-price"
            value={price}
            invalid={!!errors.price}
            onChange={(v) => {
              setPrice(v);
              clear("price");
            }}
          />
        </FieldShell>
        <FieldShell id="lab-fee" label="Fee" error={errors.fee}>
          <AmountInput
            id="lab-fee"
            value={fee}
            invalid={!!errors.fee}
            onChange={(v) => {
              setFee(v);
              clear("fee");
            }}
          />
        </FieldShell>
        <FieldShell id="lab-status" label="Status">
          <select id="lab-status" value={isActive ? "1" : "0"} onChange={(e) => setIsActive(e.target.value === "1")}>
            <option value="1">Active</option>
            <option value="0">Non Active</option>
          </select>
        </FieldShell>
      </div>
    </Modal>
  );
}
