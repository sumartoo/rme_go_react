import { FormEvent, useEffect, useState } from "react";
import { Pencil } from "lucide-react";
import { api, ApiError, LabPrice } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { AmountInput, FieldShell, Modal, numText, textNum, useModal } from "../components/FormKit";

/** 95.000,00 — like the original screen's formatAmount. */
const money = new Intl.NumberFormat("id-ID", { minimumFractionDigits: 2, maximumFractionDigits: 2 });

/** Setting > Price: the price of each lab examination (reflab.price). */
export default function LabPricePage() {
  const { token, logout } = useAuth();
  const [list, setList] = useState<LabPrice[] | null>(null);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<LabPrice | null>(null);
  const [saved, setSaved] = useState("");

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    api
      .labPrice(token)
      .then((res) => setList(res.price))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const columns: Column<LabPrice>[] = [
    { key: "id", header: "Lab ID", value: (p) => String(p.id), sortValue: (p) => p.id },
    { key: "labMaster", header: "Lab Master", value: (p) => p.labMaster, className: "dt-wrap" },
    { key: "labSubMaster", header: "Sub Master Name", value: (p) => p.labSubMaster, className: "dt-wrap" },
    { key: "nama", header: "Lab", value: (p) => p.nama, className: "dt-wrap dt-grow" },
    { key: "price", header: "Price", value: (p) => (p.price === null ? "-" : money.format(p.price)), sortValue: (p) => p.price ?? -1, className: "dt-num" },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      className: "dt-action",
      render: (p) => (
        <button type="button" className="row-action" aria-label={`Edit price of ${p.nama}`} title="Edit" onClick={() => setEditing(p)}>
          <Pencil aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Price</h1>
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
        <DataTable title="Price" columns={columns} rows={list} rowKey={(p) => p.id} initialSort={{ key: "id", dir: "asc" }} defaultPageSize={10} />
      )}
      {editing && (
        <PriceForm
          row={editing}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(p) => {
            setList((l) => (l ?? []).map((x) => (x.id === p.id ? p : x)));
            setEditing(null);
            setSaved(`Price of ${p.nama} updated to ${p.price === null ? "-" : money.format(p.price)}.`);
          }}
        />
      )}
    </section>
  );
}

function PriceForm({
  row,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  row: LabPrice;
  onClose: () => void;
  onSaved: (p: LabPrice) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [price, setPrice] = useState(numText(row.price));
  const [priceError, setPriceError] = useState("");
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    const v = textNum(price);
    if (v !== null && (Number.isNaN(v) || v < 0)) {
      setPriceError("Enter a number from 0");
      return;
    }
    setBusy(true);
    setError("");
    try {
      onSaved(await api.updateLabPrice(token, row.id, v));
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && err.fieldErrors.price) setPriceError(err.fieldErrors.price);
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal dialogRef={dialogRef} title={`Edit Price #${row.id}`} onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <dl className="form-summary">
        <dt>Lab Master</dt>
        <dd>{row.labMaster}</dd>
        <dt>Sub Master</dt>
        <dd>{row.labSubMaster}</dd>
        <dt>Lab</dt>
        <dd>{row.nama}</dd>
      </dl>
      <FieldShell id="lab-price" label="Price" error={priceError} hint="Empty = no price">
        <AmountInput
          id="lab-price"
          value={price}
          invalid={!!priceError}
          onChange={(v) => {
            setPrice(v);
            setPriceError("");
          }}
        />
      </FieldShell>
    </Modal>
  );
}
