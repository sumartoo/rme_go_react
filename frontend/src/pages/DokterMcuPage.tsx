import { FormEvent, useEffect, useState } from "react";
import { Pencil, Plus } from "lucide-react";
import { api, ApiError, DokterMcu, DokterMcuMasterOption, FieldErrors } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { orDash } from "../components/branchView";
import { AmountInput, FieldShell, Modal, numText, parseAmounts, useModal } from "../components/FormKit";

/** Master Data > Dokter: the MCU doctors (refdoctor), separate from the clinic doctors. */
export default function DokterMcuPage() {
  const { token, logout } = useAuth();
  const [list, setList] = useState<DokterMcu[] | null>(null);
  const [masters, setMasters] = useState<DokterMcuMasterOption[] | null>(null);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<DokterMcu | "new" | null>(null);
  const [saved, setSaved] = useState("");

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    Promise.all([api.dokterMcu(token), api.dokterMcuOptions(token)])
      .then(([res, opts]) => {
        setList(res.dokter);
        setMasters(opts.dokterMaster);
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const columns: Column<DokterMcu>[] = [
    { key: "kode", header: "Dokter Code", value: (d) => d.kode },
    { key: "nama", header: "Dokter Name", value: (d) => d.nama, className: "dt-wrap" },
    { key: "dokterMaster", header: "Dokter Master Name", value: (d) => orDash(d.dokterMaster), className: "dt-wrap" },
    { key: "fee", header: "Percentage Fee", value: (d) => (d.fee === null ? "-" : String(d.fee)), sortValue: (d) => d.fee ?? -1 },
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
        <h1>Dokter</h1>
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
        <DataTable title="Dokter" columns={columns} rows={list} rowKey={(d) => d.kode} initialSort={{ key: "kode", dir: "asc" }} defaultPageSize={10} />
      )}
      {editing && masters && (
        <DokterMcuForm
          dokter={editing === "new" ? null : editing}
          masters={masters}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(d, isNew) => {
            setList((l) => (isNew ? [...(l ?? []), d] : (l ?? []).map((x) => (x.kode === d.kode ? d : x))));
            setEditing(null);
            setSaved(`${d.nama} ${isNew ? "added" : "updated"}.`);
          }}
        />
      )}
    </section>
  );
}

function DokterMcuForm({
  dokter,
  masters,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  dokter: DokterMcu | null;
  masters: DokterMcuMasterOption[];
  onClose: () => void;
  onSaved: (d: DokterMcu, isNew: boolean) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [kode, setKode] = useState(dokter?.kode ?? "");
  const [nama, setNama] = useState(dokter?.nama ?? "");
  const [idDokterMaster, setIdDokterMaster] = useState(dokter?.idDokterMaster ? String(dokter.idDokterMaster) : "");
  const [fee, setFee] = useState(numText(dokter?.fee ?? null));
  const [errors, setErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const clear = (key: string) => setErrors(({ [key]: _, ...rest }) => rest);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    const errs: FieldErrors = {};
    if (!dokter && !kode.trim()) errs.kode = "Dokter Code is required";
    if (!nama.trim()) errs.nama = "Dokter Name is required";
    if (!idDokterMaster) errs.idDokterMaster = "Dokter Master is required";
    const amounts = parseAmounts({ fee });
    if ("bad" in amounts) errs.fee = "Enter a number from 0";
    else if (amounts.ok.fee !== null && amounts.ok.fee > 100) errs.fee = "Percentage Fee must be between 0 and 100";
    if (Object.keys(errs).length || "bad" in amounts) {
      setErrors(errs);
      return;
    }
    setBusy(true);
    setError("");
    try {
      const input = { nama, idDokterMaster: Number(idDokterMaster), fee: amounts.ok.fee };
      const saved = dokter ? await api.updateDokterMcu(token, dokter.kode, input) : await api.createDokterMcu(token, { kode, ...input });
      onSaved(saved, !dokter);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && Object.keys(err.fieldErrors).length) setErrors(err.fieldErrors);
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal dialogRef={dialogRef} title={dokter ? `Edit Dokter ${dokter.kode}` : "Add Dokter"} onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <div className="form-grid">
        <FieldShell id="dokter-mcu-kode" label="Dokter Code" error={errors.kode} required={!dokter} hint={dokter ? "The code cannot be changed" : undefined}>
          <input
            id="dokter-mcu-kode"
            value={kode}
            maxLength={20}
            readOnly={!!dokter}
            aria-invalid={errors.kode ? "true" : undefined}
            onChange={(e) => {
              setKode(e.target.value);
              clear("kode");
            }}
          />
        </FieldShell>
        <FieldShell id="dokter-mcu-fee" label="Percentage Fee" error={errors.fee}>
          <AmountInput
            id="dokter-mcu-fee"
            value={fee}
            invalid={!!errors.fee}
            onChange={(v) => {
              setFee(v);
              clear("fee");
            }}
          />
        </FieldShell>
        <FieldShell id="dokter-mcu-nama" label="Dokter Name" error={errors.nama} required wide>
          <input
            id="dokter-mcu-nama"
            value={nama}
            maxLength={50}
            aria-invalid={errors.nama ? "true" : undefined}
            onChange={(e) => {
              setNama(e.target.value);
              clear("nama");
            }}
          />
        </FieldShell>
        <FieldShell id="dokter-mcu-master" label="Dokter Master" error={errors.idDokterMaster} required wide>
          <select
            id="dokter-mcu-master"
            value={idDokterMaster}
            aria-invalid={errors.idDokterMaster ? "true" : undefined}
            onChange={(e) => {
              setIdDokterMaster(e.target.value);
              clear("idDokterMaster");
            }}
          >
            <option value="">Pilih Dokter Master…</option>
            {masters.map((m) => (
              <option key={m.id} value={m.id}>
                {m.nama}
              </option>
            ))}
          </select>
        </FieldShell>
      </div>
    </Modal>
  );
}
