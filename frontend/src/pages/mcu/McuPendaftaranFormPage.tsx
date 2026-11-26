import { ReactNode, useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Info, Plus, Save, Trash2, UserPlus } from "lucide-react";
import {
  api,
  ApiError,
  McuDaftarDetail,
  McuDaftarDokter,
  McuDaftarLab,
  McuDaftarNonLab,
  McuDaftarOptions,
  McuDaftarPaket,
  McuDaftarPasien,
} from "../../api";
import { useAuth } from "../../auth/AuthContext";
import { Card } from "../PengobatanFormPage";
import { MCU_DAFTAR_PATH } from "./McuPendaftaranPage";
import { PasienLookup, PickDialog } from "./mcuDaftarLookups";

const money = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });

/** An ordered line on the form: id, what it shows and its price. */
type Line = { id: number; nama: string; extra: string; price: number };
type Lookup = "pasien" | "lab" | "paket" | "nonLab" | "dokter" | null;

const sum = (lines: Line[]) => lines.reduce((t, l) => t + l.price, 0);

/** A small table with a delete button per row and an empty message. */
function Items<T>({
  head,
  rows,
  cells,
  rowKey,
  empty,
  onRemove,
  locked,
}: {
  head: string[];
  rows: T[];
  cells: (r: T) => ReactNode[];
  rowKey: (r: T) => string | number;
  empty: string;
  onRemove?: (r: T) => void;
  locked: boolean;
}) {
  const withAction = !!onRemove && !locked;
  return (
    <div className="table-wrap">
      <table className="data-table tx-items">
        <thead>
          <tr>
            {head.map((h) => (
              <th key={h}>{h}</th>
            ))}
            {withAction && <th>Action</th>}
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 ? (
            <tr>
              <td colSpan={head.length + (withAction ? 1 : 0)} className="dt-empty">
                {empty}
              </td>
            </tr>
          ) : (
            rows.map((r) => (
              <tr key={rowKey(r)}>
                {cells(r).map((c, i) => (
                  <td key={i}>{c}</td>
                ))}
                {withAction && (
                  <td className="tx-actions">
                    <button type="button" className="row-action danger" aria-label={`Hapus ${rowKey(r)}`} title="Hapus" onClick={() => onRemove!(r)}>
                      <Trash2 aria-hidden="true" />
                    </button>
                  </td>
                )}
              </tr>
            ))
          )}
        </tbody>
      </table>
    </div>
  );
}

/**
 * MCU > Pendaftaran > Add and Edit (legacy /Pendaftaran/New): Tgl MCU, MCU
 * Kategori and MCU Info, the patients, and what is ordered for them, General
 * lab examinations, Paket MCU, Non Lab services and Dokter. Add registers every
 * patient with the same lines (Total = Sub Total × patients). Edit changes one
 * registration while it is New, and shows it read-only afterwards.
 */
export default function McuPendaftaranFormPage() {
  const { id } = useParams();
  const editId = id ? Number(id) : null;
  const { token, logout } = useAuth();
  const navigate = useNavigate();
  const [options, setOptions] = useState<McuDaftarOptions | null>(null);
  const [detail, setDetail] = useState<McuDaftarDetail | null>(null);
  const [loadError, setLoadError] = useState("");
  const [mcuDate, setMcuDate] = useState("");
  const [kategori, setKategori] = useState("");
  const [info, setInfo] = useState("");
  const [pasien, setPasien] = useState<McuDaftarPasien[]>([]);
  const [lab, setLab] = useState<Line[]>([]);
  const [paket, setPaket] = useState<Line[]>([]);
  const [nonLab, setNonLab] = useState<Line[]>([]);
  const [dokter, setDokter] = useState<McuDaftarDokter[]>([]);
  const [lookup, setLookup] = useState<Lookup>(null);
  const [saving, setSaving] = useState(false);
  const [error, setError] = useState("");
  const [saved, setSaved] = useState("");

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token) return;
    Promise.all([api.mcuDaftarOptions(token), editId ? api.mcuDaftarGet(token, editId) : Promise.resolve(null)])
      .then(([o, d]) => {
        setOptions(o);
        setMcuDate(o.today);
        if (d) fill(d);
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else if (err instanceof ApiError && err.status === 404) setLoadError(`Pendaftaran MCU ${id} tidak ditemukan.`);
        else setLoadError(err instanceof Error ? err.message : "Gagal memuat");
      });
  }, [editId]); // eslint-disable-line react-hooks/exhaustive-deps

  function fill(d: McuDaftarDetail) {
    setDetail(d);
    setMcuDate(d.mcuDate ?? "");
    setKategori(d.kategoriId ? String(d.kategoriId) : "");
    setInfo(d.infoId ? String(d.infoId) : "");
    setPasien(d.pasien.id ? [d.pasien] : []);
    setLab(d.lab);
    setPaket(d.paket);
    setNonLab(d.nonLab);
    setDokter(d.dokter);
  }

  const locked = !!detail && detail.status !== "New";
  const subTotal = sum(lab) + sum(paket) + sum(nonLab);
  const jumlahPasien = pasien.length;
  const missing = [
    !mcuDate && "Tgl MCU",
    !kategori && "MCU Kategori",
    jumlahPasien === 0 && "Pasien",
    lab.length + paket.length + nonLab.length === 0 && "General / Paket MCU / Non Lab",
  ].filter(Boolean) as string[];

  async function save() {
    if (!token || locked) return;
    if (missing.length) {
      setError(`Lengkapi dulu: ${missing.join(", ")}.`);
      return;
    }
    const input = {
      mcuDate,
      kategoriId: Number(kategori),
      infoId: info ? Number(info) : null,
      labIds: lab.map((l) => l.id),
      paketIds: paket.map((l) => l.id),
      nonLabIds: nonLab.map((l) => l.id),
      dokter: dokter.map((d) => d.kode),
    };
    setSaving(true);
    setError("");
    try {
      if (editId) {
        const d = await api.mcuDaftarUpdate(token, editId, input);
        fill(d);
        setSaved(`Pendaftaran MCU ${editId} tersimpan.`);
      } else {
        const res = await api.mcuDaftarCreate(token, { ...input, pasienIds: pasien.map((p) => p.id) });
        navigate(MCU_DAFTAR_PATH, { replace: true, state: { notice: `${res.ids.length} pendaftaran MCU tersimpan (ID ${res.ids.join(", ")}).` } });
      }
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) return expired();
      setError(err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : "Gagal menyimpan");
    } finally {
      setSaving(false);
    }
  }

  const back = () => {
    const dirty = !locked && (pasien.length > 0 || subTotal > 0 || dokter.length > 0);
    if (!editId && dirty && !window.confirm("Data belum disimpan. Tinggalkan halaman ini?")) return;
    navigate(MCU_DAFTAR_PATH);
  };

  if (loadError) {
    return (
      <section className="panel">
        <p className="alert error" role="alert">
          {loadError}
        </p>
        <button type="button" className="btn-small" onClick={() => navigate(MCU_DAFTAR_PATH)}>
          Kembali
        </button>
      </section>
    );
  }
  if (!options) return <p className="muted">Loading…</p>;

  const addBtn = (label: string, which: Lookup, icon: ReactNode = <Plus aria-hidden="true" />) =>
    !locked && (
      <button type="button" className="btn-add" onClick={() => setLookup(which)}>
        {icon} {label}
      </button>
    );
  const ids = (lines: Line[]) => new Set<string | number>(lines.map((l) => l.id));
  const remove = (set: (f: (l: Line[]) => Line[]) => void) => (r: Line) => set((list) => list.filter((x) => x.id !== r.id));

  return (
    <>
      <div className="page-head">
        <h1>{editId ? `Pendaftaran MCU ${editId}` : "Add Pendaftaran MCU"}</h1>
        <div className="head-tools">
          <button type="button" className="btn-small" onClick={back}>
            {locked || saved ? "Kembali" : "Cancel"}
          </button>
          {!locked && (
            <button type="button" className="btn-small btn-primary" onClick={save} disabled={saving} title={missing.length ? `Lengkapi: ${missing.join(", ")}` : "Simpan"}>
              {saving ? "Saving…" : "Save"} <Save aria-hidden="true" />
            </button>
          )}
        </div>
      </div>
      {locked && (
        <p className="alert info" role="note">
          <Info aria-hidden="true" /> Status <strong>{detail!.status}</strong>: pendaftaran hanya bisa dilihat.
          {detail!.cancelRemarks ? ` Alasan cancel: ${detail!.cancelRemarks}.` : ""}
        </p>
      )}
      {error && (
        <p className="alert error" role="alert">
          {error}
        </p>
      )}
      {saved && (
        <p className="alert success" role="status">
          {saved}
        </p>
      )}

      <Card title="Data Pasien" locked={locked}>
        <div className="tx-items-head mcu-daftar-head">
          <label>
            Tgl MCU<span className="req"> *</span>
            <input type="date" value={mcuDate} onChange={(e) => setMcuDate(e.target.value)} />
          </label>
          <label>
            MCU Kategori<span className="req"> *</span>
            <select className="select" value={kategori} onChange={(e) => setKategori(e.target.value)}>
              <option value="">-- Pilih Kategori --</option>
              {options.kategori.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.name}
                </option>
              ))}
            </select>
          </label>
          <label>
            MCU Info
            <select className="select" value={info} onChange={(e) => setInfo(e.target.value)}>
              <option value="">-- Pilih Info MCU --</option>
              {options.info.map((o) => (
                <option key={o.id} value={o.id}>
                  {o.name}
                </option>
              ))}
            </select>
          </label>
          {!editId && addBtn("Add Pasien", "pasien", <UserPlus aria-hidden="true" />)}
        </div>
        <Items
          head={["Pasien ID", "No RM", "Nama", "Kode Perusahaan", "Perusahaan", "NIK"]}
          rows={pasien}
          rowKey={(p) => p.id}
          cells={(p) => [p.id, p.rm || "-", p.nama, p.kodePerusahaan || "-", p.perusahaan || "-", p.nik || "-"]}
          empty="Belum ada pasien."
          onRemove={editId ? undefined : (p) => setPasien((list) => list.filter((x) => x.id !== p.id))}
          locked={locked}
        />
      </Card>

      <div className="tx-grid tx-grid-2">
        <Card title="General" locked={locked}>
          <div className="tx-items-head">
            {addBtn("Add General", "lab")}
            <p className="tx-sum">Total General Price : {money.format(sum(lab))}</p>
          </div>
          <Items
            head={["Lab ID", "Master / Sub Master", "Lab Name", "Price"]}
            rows={lab}
            rowKey={(l) => l.id}
            cells={(l) => [l.id, l.extra || "-", l.nama, money.format(l.price)]}
            empty="Belum ada pemeriksaan."
            onRemove={remove(setLab)}
            locked={locked}
          />
        </Card>
        <Card title="Paket Mcu" locked={locked}>
          <div className="tx-items-head">
            {addBtn("Add Paket Mcu", "paket")}
            <p className="tx-sum">Total Paket Mcu Price : {money.format(sum(paket))}</p>
          </div>
          <Items
            head={["Paket ID", "Nama Paket", "Perusahaan", "Price"]}
            rows={paket}
            rowKey={(l) => l.id}
            cells={(l) => [l.id, l.nama, l.extra || "-", money.format(l.price)]}
            empty="Belum ada paket."
            onRemove={remove(setPaket)}
            locked={locked}
          />
        </Card>
        <Card title="Non Lab" locked={locked}>
          <div className="tx-items-head">
            {addBtn("Add Non Lab", "nonLab")}
            <p className="tx-sum">Total Non Lab Price : {money.format(sum(nonLab))}</p>
          </div>
          <Items
            head={["ID", "Non Lab Nama", "Price"]}
            rows={nonLab}
            rowKey={(l) => l.id}
            cells={(l) => [l.id, l.nama, money.format(l.price)]}
            empty="Belum ada non lab."
            onRemove={remove(setNonLab)}
            locked={locked}
          />
        </Card>
        <Card title="Dokter" locked={locked}>
          <div className="tx-items-head">{addBtn("Add Dokter", "dokter")}</div>
          <Items
            head={["Code Dokter", "Dokter Name", "Dokter Master", "Fee %"]}
            rows={dokter}
            rowKey={(d) => d.kode}
            cells={(d) => [d.kode, d.nama || "-", d.master || "-", money.format(d.feePercentage)]}
            empty="Belum ada dokter."
            onRemove={(d) => setDokter((list) => list.filter((x) => x.kode !== d.kode))}
            locked={locked}
          />
        </Card>
      </div>

      <div className="mcu-daftar-total" aria-live="polite">
        <span>
          Jumlah Pasien : <strong>{jumlahPasien}</strong>
        </span>
        <span>
          Sub Total Price : <strong>{money.format(subTotal)}</strong>
        </span>
        <span>
          Total Price : <strong>{money.format(subTotal * jumlahPasien)}</strong>
        </span>
      </div>

      {lookup === "pasien" && (
        <PasienLookup chosen={new Set(pasien.map((p) => p.id))} onPick={(p) => setPasien((list) => [...list, p])} onClose={() => setLookup(null)} />
      )}
      {lookup === "lab" && (
        <PickDialog<McuDaftarLab>
          title="Data General"
          load={api.mcuDaftarLab}
          rowKey={(l) => l.id}
          chosen={ids(lab)}
          columns={[
            { key: "id", header: "Lab ID", value: (l) => String(l.id), sortValue: (l) => l.id },
            { key: "master", header: "Master Name", value: (l) => l.master || "-" },
            { key: "subMaster", header: "Sub Master Name", value: (l) => l.subMaster || "-", className: "dt-wrap-sm" },
            { key: "nama", header: "Lab Name", value: (l) => l.nama, className: "dt-wrap-sm" },
            { key: "price", header: "Price", value: (l) => money.format(l.price), sortValue: (l) => l.price, className: "dt-num" },
          ]}
          onPick={(l) => setLab((list) => [...list, { id: l.id, nama: l.nama, extra: [l.master, l.subMaster].filter(Boolean).join(" / "), price: l.price }])}
          onClose={() => setLookup(null)}
        />
      )}
      {lookup === "paket" && (
        <PickDialog<McuDaftarPaket>
          title="Data Paket MCU"
          load={api.mcuDaftarPaket}
          rowKey={(p) => p.id}
          chosen={ids(paket)}
          columns={[
            { key: "id", header: "Paket ID", value: (p) => String(p.id), sortValue: (p) => p.id },
            { key: "nama", header: "Nama Paket", value: (p) => p.nama, className: "dt-wrap-sm" },
            { key: "perusahaan", header: "Perusahaan", value: (p) => p.perusahaan || "-", className: "dt-wrap-sm" },
            { key: "kategori", header: "Kategori", value: (p) => p.kategori || "-" },
            { key: "price", header: "Price", value: (p) => money.format(p.price), sortValue: (p) => p.price, className: "dt-num" },
          ]}
          onPick={(p) => setPaket((list) => [...list, { id: p.id, nama: p.nama, extra: p.perusahaan, price: p.price }])}
          onClose={() => setLookup(null)}
        />
      )}
      {lookup === "nonLab" && (
        <PickDialog<McuDaftarNonLab>
          title="Data Non Lab"
          load={api.mcuDaftarNonLab}
          rowKey={(n) => n.id}
          chosen={ids(nonLab)}
          columns={[
            { key: "id", header: "ID", value: (n) => String(n.id), sortValue: (n) => n.id },
            { key: "nama", header: "Non Lab Nama", value: (n) => n.nama },
            { key: "price", header: "Price", value: (n) => money.format(n.price), sortValue: (n) => n.price, className: "dt-num" },
          ]}
          onPick={(n) => setNonLab((list) => [...list, { id: n.id, nama: n.nama, extra: "", price: n.price }])}
          onClose={() => setLookup(null)}
        />
      )}
      {lookup === "dokter" && (
        <PickDialog<McuDaftarDokter>
          title="Data Dokter MCU"
          load={api.mcuDaftarDokter}
          rowKey={(d) => d.kode}
          chosen={new Set(dokter.map((d) => d.kode))}
          columns={[
            { key: "kode", header: "Code Dokter", value: (d) => d.kode },
            { key: "nama", header: "Dokter Name", value: (d) => d.nama },
            { key: "master", header: "Dokter Master", value: (d) => d.master || "-" },
            { key: "fee", header: "Fee %", value: (d) => money.format(d.feePercentage), sortValue: (d) => d.feePercentage, className: "dt-num" },
          ]}
          onPick={(d) => setDokter((list) => [...list, d])}
          onClose={() => setLookup(null)}
        />
      )}
    </>
  );
}
