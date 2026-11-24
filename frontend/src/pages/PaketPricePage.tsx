import { FormEvent, useMemo, useEffect, useState } from "react";
import { List, Pencil, Plus, Trash2, X } from "lucide-react";
import { api, ApiError, FieldErrors, PaketPrice, PaketPriceDetail, PaketPriceInput, PaketPriceLab, PaketPriceOptions } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { orDash } from "../components/branchView";
import { AmountInput, FieldShell, Modal, numText, textNum, useModal } from "../components/FormKit";
import { fmtDate, today } from "../components/dates";

/** 94.000,00 — like the original screen's formatAmount. */
const money = new Intl.NumberFormat("id-ID", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const fmtMoney = (v: number | null) => (v === null ? "-" : money.format(v));
const statusText = (p: PaketPrice) => (p.isActive ? "Aktif" : "Non Aktif");

/** Setting > Paket Price: MCU packages per company (refpriceheader) and their lab examinations. */
export default function PaketPricePage() {
  const { token, logout } = useAuth();
  const [list, setList] = useState<PaketPrice[] | null>(null);
  const [options, setOptions] = useState<PaketPriceOptions | null>(null);
  const [loadError, setLoadError] = useState("");
  const [viewing, setViewing] = useState<PaketPriceDetail | null>(null);
  const [editing, setEditing] = useState<PaketPriceDetail | "new" | null>(null);
  const [notice, setNotice] = useState<{ kind: "success" | "error"; text: string } | null>(null);

  const expired = () => logout("Your session has expired. Please log in again.");
  const fail = (err: unknown, what: string) => {
    if (err instanceof ApiError && err.status === 401) expired();
    else setNotice({ kind: "error", text: err instanceof Error ? err.message : what });
  };

  useEffect(() => {
    if (!token) return;
    Promise.all([api.paketPrice(token), api.paketPriceOptions(token)])
      .then(([res, opts]) => {
        setList(res.paket);
        setOptions(opts);
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof Error ? err.message : "Failed to load");
      });
  }, []); // eslint-disable-line react-hooks/exhaustive-deps

  const open = async (p: PaketPrice, then: (d: PaketPriceDetail) => void) => {
    if (!token) return;
    try {
      then(await api.paketPriceDetail(token, p.id));
    } catch (err) {
      fail(err, "Failed to load the package");
    }
  };

  const remove = async (p: PaketPrice) => {
    if (!token || !window.confirm(`Hapus paket "${p.nama}" (${p.perusahaan})?`)) return;
    try {
      await api.deletePaketPrice(token, p.id);
      setList((l) => (l ?? []).filter((x) => x.id !== p.id));
      setNotice({ kind: "success", text: `Paket ${p.nama} deleted.` });
    } catch (err) {
      fail(err, "Failed to delete the package");
    }
  };

  const columns: Column<PaketPrice>[] = [
    { key: "id", header: "ID", value: (p) => String(p.id), sortValue: (p) => p.id },
    { key: "perusahaan", header: "Perusahaan", value: (p) => p.perusahaan, className: "dt-wrap" },
    { key: "nama", header: "Nama Paket", value: (p) => p.nama, className: "dt-wrap" },
    { key: "paket", header: "Paket", value: (p) => (p.isPaket ? "Ya" : "Tidak") },
    { key: "kategori", header: "Kategori", value: (p) => orDash(p.kategori) },
    { key: "fromDate", header: "Dari", value: (p) => fmtDate(p.fromDate), sortValue: (p) => p.fromDate, hidden: true },
    { key: "toDate", header: "Sampai", value: (p) => fmtDate(p.toDate), sortValue: (p) => p.toDate, hidden: true },
    { key: "jumlahLab", header: "Jumlah Lab", value: (p) => String(p.jumlahLab), sortValue: (p) => p.jumlahLab, hidden: true },
    { key: "price", header: "Price", value: (p) => fmtMoney(p.price), sortValue: (p) => p.price ?? -1, className: "dt-num" },
    {
      key: "status",
      header: "Status",
      value: statusText,
      render: (p) => <span className={p.isActive ? "status on" : "status off"}>{statusText(p)}</span>,
    },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      className: "dt-action",
      render: (p) => (
        <span className="row-actions">
          <button type="button" className="row-action" aria-label={`Detail ${p.nama}`} title="Detail" onClick={() => open(p, setViewing)}>
            <List aria-hidden="true" />
          </button>
          <button type="button" className="row-action" aria-label={`Edit ${p.nama}`} title="Edit" onClick={() => open(p, setEditing)}>
            <Pencil aria-hidden="true" />
          </button>
          <button type="button" className="row-action danger" aria-label={`Delete ${p.nama}`} title="Delete" onClick={() => remove(p)}>
            <Trash2 aria-hidden="true" />
          </button>
        </span>
      ),
    },
  ];

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Paket Price</h1>
        <button type="button" className="btn-add" onClick={() => setEditing("new")} disabled={!list || !options}>
          <Plus aria-hidden="true" /> Add
        </button>
      </div>
      {notice && (
        <p className={`alert ${notice.kind}`} role={notice.kind === "error" ? "alert" : "status"}>
          {notice.text}
        </p>
      )}
      {loadError ? (
        <p className="alert error" role="alert">
          {loadError}
        </p>
      ) : !list ? (
        <p className="muted">Loading…</p>
      ) : (
        <DataTable title="Paket Price" columns={columns} rows={list} rowKey={(p) => p.id} initialSort={{ key: "id", dir: "desc" }} defaultPageSize={10} />
      )}
      {viewing && <PaketDetailDialog paket={viewing} onClose={() => setViewing(null)} />}
      {editing && options && (
        <PaketForm
          paket={editing === "new" ? null : editing}
          options={options}
          onClose={() => setEditing(null)}
          onUnauthorized={expired}
          onSaved={(d, isNew) => {
            const { labs: _, ...row } = d;
            setList((l) => (isNew ? [row, ...(l ?? [])] : (l ?? []).map((x) => (x.id === row.id ? row : x))));
            setEditing(null);
            setNotice({ kind: "success", text: `Paket ${d.nama} ${isNew ? "added" : "updated"}.` });
          }}
        />
      )}
    </section>
  );
}

/** Read-only view of a package and its lab examinations (the ☰ action). */
function PaketDetailDialog({ paket, onClose }: { paket: PaketPriceDetail; onClose: () => void }) {
  const dialogRef = useModal();
  const total = paket.labs.reduce((s, l) => s + (l.price ?? 0), 0);
  return (
    <dialog ref={dialogRef} className="modal modal-wide" aria-labelledby="paket-detail-title" onClose={onClose}>
      <header className="modal-head">
        <h2 id="paket-detail-title">
          Paket #{paket.id} · {paket.nama}
        </h2>
        <button type="button" className="row-action" aria-label="Close" onClick={onClose}>
          <X aria-hidden="true" />
        </button>
      </header>
      <div className="modal-body">
        <dl className="form-summary">
          <dt>Perusahaan</dt>
          <dd>{paket.perusahaan}</dd>
          <dt>Paket</dt>
          <dd>{paket.isPaket ? "Ya" : "Tidak"}</dd>
          <dt>Kategori</dt>
          <dd>{orDash(paket.kategori)}</dd>
          <dt>Periode</dt>
          <dd>
            {fmtDate(paket.fromDate)} – {fmtDate(paket.toDate)}
          </dd>
          <dt>Price</dt>
          <dd>{fmtMoney(paket.price)}</dd>
          <dt>Status</dt>
          <dd>{statusText(paket)}</dd>
        </dl>
        <div className="paket-labs-table">
          <table className="dt-table">
            <thead>
              <tr>
                <th>No</th>
                <th>Lab Master</th>
                <th>Sub Master</th>
                <th>Lab</th>
                <th className="dt-num">Harga Lab</th>
              </tr>
            </thead>
            <tbody>
              {paket.labs.map((l, i) => (
                <tr key={l.id}>
                  <td>{i + 1}</td>
                  <td>{orDash(l.labMaster)}</td>
                  <td>{orDash(l.labSubMaster)}</td>
                  <td>
                    {l.nama}
                    {!l.isActive && <span className="muted"> (Non Active)</span>}
                  </td>
                  <td className="dt-num">{fmtMoney(l.price)}</td>
                </tr>
              ))}
            </tbody>
            <tfoot>
              <tr>
                <th colSpan={4}>Total harga lab ({paket.labs.length} pemeriksaan)</th>
                <th className="dt-num">{money.format(total)}</th>
              </tr>
            </tfoot>
          </table>
        </div>
      </div>
      <footer className="modal-foot">
        <button type="button" className="btn-small" onClick={onClose}>
          Close
        </button>
      </footer>
    </dialog>
  );
}

function PaketForm({
  paket,
  options,
  onClose,
  onSaved,
  onUnauthorized,
}: {
  paket: PaketPriceDetail | null;
  options: PaketPriceOptions;
  onClose: () => void;
  onSaved: (d: PaketPriceDetail, isNew: boolean) => void;
  onUnauthorized: () => void;
}) {
  const { token } = useAuth();
  const dialogRef = useModal();
  const [idPerusahaan, setIdPerusahaan] = useState(paket ? String(paket.idPerusahaan) : "");
  const [nama, setNama] = useState(paket?.nama ?? "");
  const [isPaket, setIsPaket] = useState(paket ? paket.isPaket : true);
  const [idKategori, setIdKategori] = useState(paket?.idKategori ? String(paket.idKategori) : "");
  const [fromDate, setFromDate] = useState(paket?.fromDate ?? today());
  const [toDate, setToDate] = useState(paket?.toDate ?? "");
  const [price, setPrice] = useState(numText(paket?.price ?? null));
  const [isActive, setIsActive] = useState(paket ? paket.isActive : true);
  const [labIds, setLabIds] = useState<Set<number>>(() => new Set(paket?.labs.map((l) => l.id) ?? []));
  const [errors, setErrors] = useState<FieldErrors>({});
  const [error, setError] = useState("");
  const [busy, setBusy] = useState(false);

  const clear = (key: string) => setErrors(({ [key]: _, ...rest }) => rest);

  async function submit(e: FormEvent) {
    e.preventDefault();
    if (!token) return;
    const errs: FieldErrors = {};
    if (!idPerusahaan) errs.idPerusahaan = "Perusahaan is required";
    if (!nama.trim()) errs.nama = "Nama Paket is required";
    if (!fromDate) errs.fromDate = "Dari Tanggal is required";
    if (!toDate) errs.toDate = "Sampai Tanggal is required";
    else if (fromDate && toDate < fromDate) errs.toDate = "Sampai Tanggal cannot be before Dari Tanggal";
    const p = textNum(price);
    if (p === null) errs.price = "Price is required";
    else if (Number.isNaN(p) || p < 0) errs.price = "Enter a number from 0";
    if (labIds.size === 0) errs.labIds = "Choose at least one lab examination";
    if (Object.keys(errs).length) {
      setErrors(errs);
      return;
    }
    const input: PaketPriceInput = {
      idPerusahaan: Number(idPerusahaan),
      nama,
      isPaket,
      idKategori: idKategori ? Number(idKategori) : null,
      fromDate,
      toDate,
      price: p,
      isActive,
      labIds: [...labIds],
    };
    setBusy(true);
    setError("");
    try {
      const saved = paket ? await api.updatePaketPrice(token, paket.id, input) : await api.createPaketPrice(token, input);
      onSaved(saved, !paket);
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) onUnauthorized();
      else if (err instanceof ApiError && Object.keys(err.fieldErrors).length) setErrors(err.fieldErrors);
      else setError(err instanceof Error ? err.message : "Failed to save");
    } finally {
      setBusy(false);
    }
  }

  return (
    <Modal dialogRef={dialogRef} wide title={paket ? `Edit Paket #${paket.id}` : "Add Paket"} onClose={onClose} onSubmit={submit} error={error} busy={busy}>
      <div className="form-grid">
        <FieldShell id="paket-perusahaan" label="Perusahaan" error={errors.idPerusahaan} required wide>
          <select
            id="paket-perusahaan"
            value={idPerusahaan}
            aria-invalid={errors.idPerusahaan ? "true" : undefined}
            onChange={(e) => {
              setIdPerusahaan(e.target.value);
              clear("idPerusahaan");
            }}
          >
            <option value="">Pilih perusahaan…</option>
            {options.perusahaan.map((c) => (
              <option key={c.id} value={c.id}>
                {c.nama}
              </option>
            ))}
          </select>
        </FieldShell>
        <FieldShell id="paket-nama" label="Nama Paket" error={errors.nama} required>
          <input
            id="paket-nama"
            value={nama}
            maxLength={30}
            aria-invalid={errors.nama ? "true" : undefined}
            onChange={(e) => {
              setNama(e.target.value);
              clear("nama");
            }}
          />
        </FieldShell>
        <FieldShell id="paket-kategori" label="Kategori" error={errors.idKategori}>
          <select
            id="paket-kategori"
            value={idKategori}
            onChange={(e) => {
              setIdKategori(e.target.value);
              clear("idKategori");
            }}
          >
            <option value="">-</option>
            {options.kategori.map((k) => (
              <option key={k.id} value={k.id}>
                {k.nama}
              </option>
            ))}
          </select>
        </FieldShell>
        <FieldShell id="paket-from" label="Dari Tanggal" error={errors.fromDate} required>
          <input
            id="paket-from"
            type="date"
            value={fromDate}
            max={toDate || undefined}
            aria-invalid={errors.fromDate ? "true" : undefined}
            onChange={(e) => {
              setFromDate(e.target.value);
              clear("fromDate");
            }}
          />
        </FieldShell>
        <FieldShell id="paket-to" label="Sampai Tanggal" error={errors.toDate} required>
          <input
            id="paket-to"
            type="date"
            value={toDate}
            min={fromDate || undefined}
            aria-invalid={errors.toDate ? "true" : undefined}
            onChange={(e) => {
              setToDate(e.target.value);
              clear("toDate");
            }}
          />
        </FieldShell>
        <FieldShell id="paket-price" label="Price" error={errors.price} required>
          <AmountInput
            id="paket-price"
            value={price}
            invalid={!!errors.price}
            onChange={(v) => {
              setPrice(v);
              clear("price");
            }}
          />
        </FieldShell>
        <FieldShell id="paket-ispaket" label="Paket">
          <select id="paket-ispaket" value={isPaket ? "1" : "0"} onChange={(e) => setIsPaket(e.target.value === "1")}>
            <option value="1">Ya</option>
            <option value="0">Tidak</option>
          </select>
        </FieldShell>
        <FieldShell id="paket-status" label="Status">
          <select id="paket-status" value={isActive ? "1" : "0"} onChange={(e) => setIsActive(e.target.value === "1")}>
            <option value="1">Aktif</option>
            <option value="0">Non Aktif</option>
          </select>
        </FieldShell>
      </div>
      <LabPicker
        labs={options.labs}
        selected={labIds}
        error={errors.labIds}
        onChange={(next) => {
          setLabIds(next);
          clear("labIds");
        }}
      />
    </Modal>
  );
}

/**
 * Checklist of lab examinations grouped by Lab Master and Sub Master, with a
 * search. Non Active examinations are offered only when already chosen.
 */
function LabPicker({
  labs,
  selected,
  error,
  onChange,
}: {
  labs: PaketPriceLab[];
  selected: Set<number>;
  error?: string;
  onChange: (next: Set<number>) => void;
}) {
  const [q, setQ] = useState("");
  const [onlyChosen, setOnlyChosen] = useState(false);

  const groups = useMemo(() => {
    const needle = q.trim().toLowerCase();
    const m = new Map<string, PaketPriceLab[]>();
    for (const l of labs) {
      if (!l.isActive && !selected.has(l.id)) continue;
      if (onlyChosen && !selected.has(l.id)) continue;
      const key = `${l.labMaster || "-"} › ${l.labSubMaster || "-"}`;
      if (needle && !`${key} ${l.nama}`.toLowerCase().includes(needle)) continue;
      m.set(key, [...(m.get(key) ?? []), l]);
    }
    return [...m.entries()];
  }, [labs, selected, q, onlyChosen]);

  const total = labs.reduce((s, l) => s + (selected.has(l.id) ? (l.price ?? 0) : 0), 0);

  const toggle = (ids: number[], on: boolean) => {
    const next = new Set(selected);
    for (const id of ids) (on ? next.add(id) : next.delete(id));
    onChange(next);
  };

  return (
    <fieldset className="lab-picker" aria-invalid={error ? "true" : undefined}>
      <legend>
        Pemeriksaan Lab <span aria-hidden="true">*</span>
      </legend>
      <div className="lab-picker-bar">
        <input type="search" placeholder="Cari lab…" aria-label="Cari lab" value={q} onChange={(e) => setQ(e.target.value)} />
        <label className="lab-picker-only">
          <input type="checkbox" checked={onlyChosen} onChange={(e) => setOnlyChosen(e.target.checked)} /> Hanya yang dipilih
        </label>
        <span className="lab-picker-sum" aria-live="polite">
          {selected.size} dipilih · total harga lab {money.format(total)}
        </span>
      </div>
      <div className="lab-picker-list">
        {groups.length === 0 && <p className="muted">Tidak ada pemeriksaan yang cocok.</p>}
        {groups.map(([key, items]) => {
          const ids = items.map((l) => l.id);
          const all = ids.every((id) => selected.has(id));
          return (
            <div key={key} className="lab-picker-group">
              <label className="lab-picker-head">
                <input type="checkbox" checked={all} onChange={(e) => toggle(ids, e.target.checked)} /> {key}
              </label>
              {items.map((l) => (
                <label key={l.id} className="lab-picker-item">
                  <input type="checkbox" checked={selected.has(l.id)} onChange={(e) => toggle([l.id], e.target.checked)} />
                  <span>
                    {l.nama}
                    {!l.isActive && <span className="muted"> (Non Active)</span>}
                  </span>
                  <span className="lab-picker-price">{fmtMoney(l.price)}</span>
                </label>
              ))}
            </div>
          );
        })}
      </div>
      {error && <span className="field-error">{error}</span>}
    </fieldset>
  );
}
