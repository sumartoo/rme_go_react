import { FormEvent, useEffect, useState } from "react";
import { Pencil } from "lucide-react";
import { api, ApiError, FieldErrors, SettingBarcode, SettingBarcodeInput } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { AmountInput, FieldShell, Modal, numText, parseAmounts, useModal } from "../components/FormKit";

type NumKey = Exclude<keyof SettingBarcodeInput, "isActive">;

const num = (v: number | null) => (v === null ? "-" : String(v));
const numCol = (key: NumKey, header: string, hidden = false): Column<SettingBarcode> => ({
  key,
  header,
  value: (s) => num(s[key]),
  sortValue: (s) => s[key],
  hidden,
});

/** Setting > Barcode: the MCU barcode label layout (refsettingbarcode). */
export default function SettingBarcodePage() {
  const { token, logout } = useAuth();
  const [list, setList] = useState<SettingBarcode[] | null>(null);
  const [loadError, setLoadError] = useState("");
  const [editing, setEditing] = useState<SettingBarcode | null>(null);
  const [saved, setSaved] = useState("");

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    api
      .settingBarcode(token)
      .then((res) => setList(res.setting))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const columns: Column<SettingBarcode>[] = [
    { key: "id", header: "ID", value: (s) => String(s.id), sortValue: (s) => s.id },
    numCol("left", "Left"),
    numCol("middle", "Middle"),
    numCol("right", "Right"),
    numCol("barcodeWidth", "Width"),
    numCol("barcodeHeight", "Height"),
    numCol("barcodeLeft", "Barcode Left", true),
    numCol("barcodeTop", "Barcode Top", true),
    numCol("fontSize", "Font Size", true),
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      className: "dt-action",
      render: (s) => (
        <button type="button" className="row-action" aria-label={`Edit setting ${s.id}`} title="Edit" onClick={() => setEditing(s)}>
          <Pencil aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Barcode</h1>
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
        <DataTable title="Setting Barcode" columns={columns} rows={list} rowKey={(s) => s.id} initialSort={{ key: "id", dir: "asc" }} defaultPageSize={10} />
      )}
      {editing && (
        <SettingBarcodeForm
          setting={editing}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(s) => {
            setList((l) => (l ?? []).map((x) => (x.id === s.id ? s : x)));
            setEditing(null);
            setSaved(`Barcode setting #${s.id} updated.`);
          }}
        />
      )}
    </section>
  );
}

const GROUPS: { title: string; fields: [NumKey, string][] }[] = [
  { title: "Margin", fields: [["left", "Left"], ["middle", "Middle"], ["right", "Right"]] },
  { title: "Posisi baris", fields: [["row1", "Row 1"], ["row2", "Row 2"], ["row3", "Row 3"], ["row4", "Row 4"], ["row5", "Row 5"], ["row6", "Row 6"]] },
  {
    title: "Barcode",
    fields: [["barcodeWidth", "Width"], ["barcodeHeight", "Height"], ["barcodeLeft", "Left"], ["barcodeTop", "Top"], ["fontSize", "Font Size"]],
  },
];
const NUM_KEYS = GROUPS.flatMap((g) => g.fields.map(([k]) => k));

function SettingBarcodeForm({
  setting,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  setting: SettingBarcode;
  onClose: () => void;
  onSaved: (s: SettingBarcode) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [vals, setVals] = useState<Record<NumKey, string>>(
    () => Object.fromEntries(NUM_KEYS.map((k) => [k, numText(setting[k])])) as Record<NumKey, string>,
  );
  const [isActive, setIsActive] = useState(setting.isActive);
  const [errors, setErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    const parsed = parseAmounts(vals);
    if ("bad" in parsed) {
      setErrors({ [parsed.bad]: "Enter a number from 0" });
      return;
    }
    setBusy(true);
    setError("");
    try {
      onSaved(await api.updateSettingBarcode(token, setting.id, { ...parsed.ok, isActive }));
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && Object.keys(err.fieldErrors).length) setErrors(err.fieldErrors);
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal dialogRef={dialogRef} wide title={`Edit Setting Barcode #${setting.id}`} onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      {GROUPS.map((g) => (
        <fieldset key={g.title} className="form-group">
          <legend>{g.title}</legend>
          <div className="form-grid form-grid-3">
            {g.fields.map(([key, label]) => (
              <FieldShell key={key} id={`barcode-${key}`} label={label} error={errors[key]}>
                <AmountInput
                  id={`barcode-${key}`}
                  value={vals[key]}
                  invalid={!!errors[key]}
                  onChange={(v) => {
                    setVals((s) => ({ ...s, [key]: v }));
                    setErrors(({ [key]: _, ...rest }) => rest);
                  }}
                />
              </FieldShell>
            ))}
          </div>
        </fieldset>
      ))}
      <FieldShell id="barcode-active" label="Status">
        <select id="barcode-active" value={isActive ? "1" : "0"} onChange={(e) => setIsActive(e.target.value === "1")}>
          <option value="1">Aktif</option>
          <option value="0">Non Aktif</option>
        </select>
      </FieldShell>
    </Modal>
  );
}
