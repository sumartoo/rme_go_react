import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Info, Plus, Save, Trash2 } from "lucide-react";
import { api, ApiError, PemeriksaanRecord, PengobatanPendaftaran, Resep, ResepDiagnosa, ResepObat } from "../api";
import { useAuth } from "../auth/AuthContext";
import DiagnosaLookup from "./DiagnosaLookup";
import DokterLookup from "./DokterLookup";
import ObatLookup from "./ObatLookup";
import PemeriksaanTab from "./PemeriksaanTab";
import { DOKTER_PATH, PROSES_OBAT_PATH } from "./PemeriksaanPerawatPage";
import { RegCard } from "./PemeriksaanPerawatFormPage";
import { Card, Lookup, Row } from "./PengobatanFormPage";

type Dokter = { id: number; nama: string };
type Tab = "perawat" | "dokter" | "obat" | "diagnosa";
const TABS: { key: Tab; label: string }[] = [
  { key: "perawat", label: "Pemeriksaan Perawat" },
  { key: "dokter", label: "Pemeriksaan Dokter" },
  { key: "obat", label: "Obat" },
  { key: "diagnosa", label: "Diagnosa" },
];

/** An editable prescription line: Banyak stays text while typing. */
type ObatLine = Omit<ResepObat, "byk"> & { byk: string };
const toLines = (r: Resep): ObatLine[] => r.obat.map((o) => ({ ...o, byk: String(o.byk) }));

/** What a prescription saves, to compare with what was loaded. */
function resepKey(obat: ObatLine[], diagnosa: ResepDiagnosa[]): string {
  return JSON.stringify([obat.map((o) => [o.idObat, o.byk.trim(), o.dosis.trim()]), diagnosa.map((d) => d.icdId)]);
}

/**
 * Transaksi Klinik > Pemeriksaan Dokter > Edit (legacy
 * /TransDokter/EditPemeriksaan/{id}): the registration and patient on the left;
 * on the right the nurse's and doctor's examination with the Dokter PJ, and the
 * doctor's prescription (Obat) and diagnoses (Diagnosa). The examination and
 * the prescription each have their own Save. Only a cancelled visit is read-only.
 *
 * With `view` it is Transaksi Klinik > Proses Obat > View: the same page
 * read-only, starting on the Obat tab, through the Proses Obat endpoints.
 */
export default function PemeriksaanDokterFormPage({ view = false }: { view?: boolean }) {
  const base = view ? "/api/proses-obat" : "/api/pemeriksaan-dokter";
  const listPath = view ? PROSES_OBAT_PATH : DOKTER_PATH;
  const { id } = useParams();
  const idKun = Number(id);
  const { token, logout } = useAuth();
  const navigate = useNavigate();
  const [reg, setReg] = useState<PengobatanPendaftaran | null>(null);
  const [editable, setEditable] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [tab, setTab] = useState<Tab>(view ? "obat" : "perawat");
  const [dokter, setDokter] = useState<Dokter | null>(null);
  const [savedDokter, setSavedDokter] = useState<number | null>(null);
  const [dokterOpen, setDokterOpen] = useState(false);
  const [examDirty, setExamDirty] = useState(false);
  // Prescription and diagnoses.
  const [obat, setObat] = useState<ObatLine[]>([]);
  const [diagnosa, setDiagnosa] = useState<ResepDiagnosa[]>([]);
  const [savedResep, setSavedResep] = useState(resepKey([], []));
  const [resepLoaded, setResepLoaded] = useState(false);
  const [resepBusy, setResepBusy] = useState(false);
  const [resepMsg, setResepMsg] = useState<{ kind: "success" | "error"; text: string } | null>(null);
  const [obatOpen, setObatOpen] = useState(false);
  const [diagnosaOpen, setDiagnosaOpen] = useState(false);

  const expired = () => logout("Your session has expired. Please log in again.");

  useEffect(() => {
    if (!token || !Number.isInteger(idKun) || idKun <= 0) {
      setLoadError("Pendaftaran tidak ditemukan.");
      return;
    }
    Promise.all([api.dokterPeriksaPendaftaran(token, idKun, base), api.getResep(token, idKun, base)])
      .then(([r, resep]) => {
        setReg(r.pendaftaran);
        setEditable(r.editable);
        setDokter(r.pendaftaran.idDokter !== null ? { id: r.pendaftaran.idDokter, nama: r.pendaftaran.dokter } : null);
        applyResep(resep);
        setResepLoaded(true);
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) expired();
        else setLoadError(err instanceof ApiError && err.status === 404 ? "Pendaftaran tidak ditemukan." : err instanceof Error ? err.message : "Gagal memuat");
      });
  }, [token, idKun]); // eslint-disable-line react-hooks/exhaustive-deps

  function applyResep(r: Resep) {
    const lines = toLines(r);
    setObat(lines);
    setDiagnosa(r.diagnosa);
    setSavedResep(resepKey(lines, r.diagnosa));
  }

  const onLoaded = (r: PemeriksaanRecord | null) => {
    setSavedDokter(r?.idDokter ?? null);
    if (r?.idDokter != null) setDokter({ id: r.idDokter, nama: r.dokter });
  };
  const examChanged = examDirty || (dokter?.id ?? null) !== savedDokter;
  const resepChanged = resepLoaded && resepKey(obat, diagnosa) !== savedResep;

  const back = () => {
    if ((examChanged || resepChanged) && editable && !window.confirm("Ada perubahan yang belum disimpan. Tinggalkan halaman ini?")) return;
    navigate(listPath);
  };

  async function saveResep() {
    if (!token || !reg) return;
    const lines: { idObat: number; byk: number; dosis: string }[] = [];
    for (const o of obat) {
      const byk = Number(o.byk.trim().replace(",", "."));
      if (!o.byk.trim() || !Number.isFinite(byk) || byk <= 0) {
        setTab("obat");
        setResepMsg({ kind: "error", text: `Banyak untuk ${o.nama || o.idObat} harus lebih dari 0.` });
        return;
      }
      lines.push({ idObat: o.idObat, byk, dosis: o.dosis.trim() });
    }
    setResepBusy(true);
    setResepMsg(null);
    try {
      const res = await api.saveResep(token, reg.id, { obat: lines, diagnosa: diagnosa.map((d) => d.icdId) });
      applyResep(res);
      setResepMsg({ kind: "success", text: `Obat (${res.obat.length}) dan diagnosa (${res.diagnosa.length}) tersimpan.` });
    } catch (err) {
      if (err instanceof ApiError && err.status === 401) return expired();
      const field = err instanceof ApiError ? Object.values(err.fieldErrors)[0] : undefined;
      setResepMsg({ kind: "error", text: field ?? (err instanceof Error ? err.message : "Gagal menyimpan obat dan diagnosa") });
    } finally {
      setResepBusy(false);
    }
  }

  if (loadError) {
    return (
      <section className="panel">
        <p className="alert error" role="alert">
          {loadError}
        </p>
        <button type="button" className="btn-small" onClick={() => navigate(listPath)}>
          Kembali
        </button>
      </section>
    );
  }
  if (!reg) return <p className="muted">Loading…</p>;

  const setLine = (i: number, patch: Partial<ObatLine>) => setObat((list) => list.map((o, j) => (j === i ? { ...o, ...patch } : o)));
  const resepBar = (
    <div className="tx-exam-head">
      <p className="muted">{resepChanged ? "Ada perubahan obat/diagnosa yang belum disimpan." : "Obat dan diagnosa disimpan bersama."}</p>
      <button type="button" className="btn-small btn-primary" onClick={saveResep} disabled={!editable || resepBusy}>
        <Save aria-hidden="true" /> {resepBusy ? "Menyimpan…" : "Simpan Obat & Diagnosa"}
      </button>
    </div>
  );

  return (
    <>
      <div className="page-head">
        <h1>
          {view ? "Proses Obat" : "Pemeriksaan Dokter"} · {reg.id}
        </h1>
        <div className="head-tools">
          <button type="button" className="btn-small" onClick={back}>
            Kembali
          </button>
        </div>
      </div>
      {view ? (
        <p className="alert info" role="note">
          <Info aria-hidden="true" /> Tampilan saja: pemeriksaan, obat dan diagnosa diubah lewat Pemeriksaan Dokter.
        </p>
      ) : !editable && (
        <p className="alert info" role="note">
          <Info aria-hidden="true" /> Kunjungan ini sudah <strong>{reg.status || "dibatalkan"}</strong>: pemeriksaan hanya bisa dilihat.
        </p>
      )}

      <div className="tx-grid tx-grid-2">
        <RegCard reg={reg} />

        <Card title="Data Pemeriksaan Perawat & Dokter">
          <Row id="pd-dokter" label="Dokter PJ" required>
            {editable ? (
              <Lookup id="pd-dokter" placeholder={dokter ? "Dokter" : "Belum di Pilih"} value={dokter?.nama ?? ""} onSearch={() => setDokterOpen(true)} />
            ) : (
              <input id="pd-dokter" readOnly value={dokter?.nama ?? ""} />
            )}
          </Row>
          <div className="tabs" role="tablist" aria-label="Pemeriksaan">
            {TABS.map((t) => (
              <button
                key={t.key}
                type="button"
                role="tab"
                id={`pd-tab-${t.key}`}
                aria-selected={tab === t.key}
                aria-controls={`pd-panel-${t.key}`}
                className={tab === t.key ? "tab active" : "tab"}
                onClick={() => setTab(t.key)}
              >
                {t.label}
                {(t.key === "perawat" || t.key === "dokter") && examChanged ? " •" : ""}
                {(t.key === "obat" || t.key === "diagnosa") && resepChanged ? " •" : ""}
              </button>
            ))}
          </div>
          <div role="tabpanel" id={`pd-panel-${tab}`} aria-labelledby={`pd-tab-${tab}`} className="tx-tabpanel">
            {/* One examination for both tabs, so switching keeps the typing and Save sends all fields. */}
            <PemeriksaanTab
              hidden={tab !== "perawat" && tab !== "dokter"}
              section={tab === "dokter" ? "dokter" : "perawat"}
              idKun={reg.id}
              idDokter={dokter?.id ?? null}
              onDirty={setExamDirty}
              base={base}
              readOnly={!editable}
              requireDokter
              onLoaded={onLoaded}
            />

            {(tab === "obat" || tab === "diagnosa") && !view && (
              <>
                {resepBar}
                {resepMsg && (
                  <p className={`alert ${resepMsg.kind}`} role={resepMsg.kind === "error" ? "alert" : "status"}>
                    {resepMsg.text}
                  </p>
                )}
              </>
            )}

            {tab === "obat" && (
              <>
                <div className="tx-items-head">
                  <button type="button" className="btn-add" onClick={() => setObatOpen(true)} disabled={!editable}>
                    <Plus aria-hidden="true" /> Add Obat
                  </button>
                  <p className="tx-sum tx-sum-right" aria-live="polite">
                    Jumlah obat : {obat.length}
                  </p>
                </div>
                <div className="table-wrap">
                  <table className="data-table tx-items">
                    <thead>
                      <tr>
                        <th>ID</th>
                        <th>Nama Obat</th>
                        <th>Satuan</th>
                        <th>Banyak</th>
                        <th>Dosis</th>
                        <th>Action</th>
                      </tr>
                    </thead>
                    <tbody>
                      {obat.length === 0 ? (
                        <tr>
                          <td colSpan={6} className="dt-empty">
                            Belum ada obat.
                          </td>
                        </tr>
                      ) : (
                        obat.map((o, i) => (
                          <tr key={`${o.idObat}-${i}`}>
                            <td>{o.idObat}</td>
                            <td>{o.nama || "(obat sudah tidak ada)"}</td>
                            <td>{o.satuan || "-"}</td>
                            <td>
                              <input
                                className="qty-input"
                                type="number"
                                min={0}
                                step="any"
                                inputMode="decimal"
                                aria-label={`Banyak ${o.nama}`}
                                value={o.byk}
                                disabled={!editable}
                                onChange={(e) => setLine(i, { byk: e.target.value })}
                              />
                            </td>
                            <td>
                              <input
                                aria-label={`Dosis ${o.nama}`}
                                placeholder="mis. 3x1"
                                maxLength={100}
                                value={o.dosis}
                                disabled={!editable}
                                onChange={(e) => setLine(i, { dosis: e.target.value })}
                              />
                            </td>
                            <td className="tx-actions">
                              <button
                                type="button"
                                className="row-action danger"
                                aria-label={`Hapus obat ${o.nama}`}
                                title="Hapus"
                                disabled={!editable}
                                onClick={() => setObat((list) => list.filter((_, j) => j !== i))}
                              >
                                <Trash2 aria-hidden="true" />
                              </button>
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </>
            )}

            {tab === "diagnosa" && (
              <>
                <div className="tx-items-head">
                  <button type="button" className="btn-add" onClick={() => setDiagnosaOpen(true)} disabled={!editable}>
                    <Plus aria-hidden="true" /> Add Diagnosa
                  </button>
                  <p className="tx-sum tx-sum-right" aria-live="polite">
                    Jumlah diagnosa : {diagnosa.length}
                  </p>
                </div>
                <div className="table-wrap">
                  <table className="data-table tx-items">
                    <thead>
                      <tr>
                        <th>ID</th>
                        <th>Kode ICD-X</th>
                        <th>Nama Penyakit</th>
                        <th>Action</th>
                      </tr>
                    </thead>
                    <tbody>
                      {diagnosa.length === 0 ? (
                        <tr>
                          <td colSpan={4} className="dt-empty">
                            Belum ada diagnosa.
                          </td>
                        </tr>
                      ) : (
                        diagnosa.map((d) => (
                          <tr key={d.icdId}>
                            <td>{d.icdId}</td>
                            <td>{d.kode || "-"}</td>
                            <td>{d.nama || "(kode sudah tidak ada)"}</td>
                            <td className="tx-actions">
                              <button
                                type="button"
                                className="row-action danger"
                                aria-label={`Hapus diagnosa ${d.kode}`}
                                title="Hapus"
                                disabled={!editable}
                                onClick={() => setDiagnosa((list) => list.filter((x) => x.icdId !== d.icdId))}
                              >
                                <Trash2 aria-hidden="true" />
                              </button>
                            </td>
                          </tr>
                        ))
                      )}
                    </tbody>
                  </table>
                </div>
              </>
            )}
          </div>
        </Card>
      </div>

      {dokterOpen && (
        <DokterLookup
          cabang={reg.cabang}
          branchName={reg.cabang}
          load={api.dokterPeriksaDokter}
          onPick={(d) => {
            setDokter({ id: d.id, nama: d.nama });
            setDokterOpen(false);
          }}
          onClose={() => setDokterOpen(false)}
        />
      )}
      {obatOpen && (
        <ObatLookup
          cabang={reg.cabang}
          branchName={reg.cabang}
          inList={new Map(obat.map((o) => [o.idObat, Number(o.byk) || 0]))}
          load={api.dokterPeriksaObat}
          checkStock={false}
          onPick={(o, byk) => {
            setObat((list) => {
              const i = list.findIndex((x) => x.idObat === o.id);
              if (i >= 0) return list.map((x, j) => (j === i ? { ...x, byk: String((Number(x.byk) || 0) + byk) } : x));
              return [...list, { idObat: o.id, nama: o.nama, satuan: o.satuan, byk: String(byk), dosis: "" }];
            });
            setObatOpen(false);
          }}
          onClose={() => setObatOpen(false)}
        />
      )}
      {diagnosaOpen && (
        <DiagnosaLookup
          chosen={new Set(diagnosa.map((d) => d.icdId))}
          load={api.dokterPeriksaIcd}
          onPick={(d) => {
            setDiagnosa((list) => [...list, { icdId: d.id, kode: d.kode, nama: d.nama }]);
            setDiagnosaOpen(false);
          }}
          onClose={() => setDiagnosaOpen(false)}
        />
      )}
    </>
  );
}
