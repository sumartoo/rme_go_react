import { useEffect, useState } from "react";
import { useNavigate, useParams } from "react-router-dom";
import { Info } from "lucide-react";
import { api, ApiError, PemeriksaanRecord, PengobatanPendaftaran } from "../api";
import { useAuth } from "../auth/AuthContext";
import { age, fmtDate } from "../components/dates";
import DokterLookup from "./DokterLookup";
import PemeriksaanTab from "./PemeriksaanTab";
import { PERAWAT_PATH } from "./PemeriksaanPerawatPage";
import { Card, Lookup, Row } from "./PengobatanFormPage";

type Dokter = { id: number; nama: string };

/**
 * Transaksi Klinik > Pemeriksaan Perawat > Edit (legacy
 * /TransDokter/EditPemeriksaan/{id}): the registration and patient on the left,
 * the nurse's examination and its Dokter PJ on the right. Dokter PJ is saved
 * with the examination. A closed or cancelled visit is shown read-only.
 */
export default function PemeriksaanPerawatFormPage() {
  const { id } = useParams();
  const idKun = Number(id);
  const { token, logout } = useAuth();
  const navigate = useNavigate();
  const [reg, setReg] = useState<PengobatanPendaftaran | null>(null);
  const [editable, setEditable] = useState(false);
  const [loadError, setLoadError] = useState("");
  const [dokter, setDokter] = useState<Dokter | null>(null);
  const [savedDokter, setSavedDokter] = useState<number | null>(null);
  const [dokterOpen, setDokterOpen] = useState(false);
  const [examDirty, setExamDirty] = useState(false);

  useEffect(() => {
    if (!token || !Number.isInteger(idKun) || idKun <= 0) {
      setLoadError("Pendaftaran tidak ditemukan.");
      return;
    }
    api
      .perawatPendaftaran(token, idKun)
      .then((res) => {
        setReg(res.pendaftaran);
        setEditable(res.editable);
        const p = res.pendaftaran;
        setDokter(p.idDokter !== null ? { id: p.idDokter, nama: p.dokter } : null);
      })
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
        else setLoadError(err instanceof ApiError && err.status === 404 ? "Pendaftaran tidak ditemukan." : err instanceof Error ? err.message : "Gagal memuat");
      });
  }, [token, idKun]); // eslint-disable-line react-hooks/exhaustive-deps

  // The loaded or just-saved examination is the baseline of Dokter PJ.
  const onLoaded = (r: PemeriksaanRecord | null) => {
    setSavedDokter(r?.idDokter ?? null);
    if (r?.idDokter != null) setDokter({ id: r.idDokter, nama: r.dokter });
  };
  const dirty = examDirty || (dokter?.id ?? null) !== savedDokter;

  const back = () => {
    if (dirty && editable && !window.confirm("Ada perubahan yang belum disimpan. Tinggalkan halaman ini?")) return;
    navigate(PERAWAT_PATH);
  };

  if (loadError) {
    return (
      <section className="panel">
        <p className="alert error" role="alert">
          {loadError}
        </p>
        <button type="button" className="btn-small" onClick={() => navigate(PERAWAT_PATH)}>
          Kembali
        </button>
      </section>
    );
  }
  if (!reg) return <p className="muted">Loading…</p>;


  return (
    <>
      <div className="page-head">
        <h1>Pemeriksaan Perawat · {reg.id}</h1>
        <div className="head-tools">
          <button type="button" className="btn-small" onClick={back}>
            Kembali
          </button>
        </div>
      </div>
      {!editable ? (
        <p className="alert info" role="note">
          <Info aria-hidden="true" /> Kunjungan ini sudah <strong>{reg.status || "ditutup"}</strong>: pemeriksaan hanya bisa dilihat.
        </p>
      ) : (
        dirty && (
          <p className="alert info" role="status">
            Ada perubahan yang belum disimpan. Klik <strong>Simpan Pemeriksaan</strong>.
          </p>
        )
      )}

      <div className="tx-grid tx-grid-2">
        <RegCard reg={reg} />

        <Card title="Pemeriksaan Perawat">
          <Row id="pr-dokter" label="Dokter PJ" required>
            {editable ? (
              <Lookup id="pr-dokter" placeholder={dokter ? "Dokter" : "Belum di Pilih"} value={dokter?.nama ?? ""} onSearch={() => setDokterOpen(true)} />
            ) : (
              <input id="pr-dokter" readOnly value={dokter?.nama ?? ""} />
            )}
          </Row>
          <PemeriksaanTab
            hidden={false}
            idKun={reg.id}
            idDokter={dokter?.id ?? null}
            onDirty={setExamDirty}
            base="/api/pemeriksaan-perawat"
            readOnly={!editable}
            requireDokter
            onLoaded={onLoaded}
          />
        </Card>
      </div>

      {dokterOpen && (
        <DokterLookup
          cabang={reg.cabang}
          branchName={reg.cabang}
          load={api.perawatDokter}
          onPick={(d) => {
            setDokter({ id: d.id, nama: d.nama });
            setDokterOpen(false);
          }}
          onClose={() => setDokterOpen(false)}
        />
      )}
    </>
  );
}

/** The registration and patient, read-only (left card of the examination forms). */
export function RegCard({ reg }: { reg: PengobatanPendaftaran }) {
  const umur = age(reg.tglLahir);
  return (
    <Card title="Data Pendaftaran & Data Pasien" locked>
      <Row id="pr-cabang" label="Cabang">
        <input id="pr-cabang" readOnly value={reg.cabang} />
      </Row>
      <Row id="pr-nodaftar" label="No. Daftar">
        <input id="pr-nodaftar" readOnly value={reg.id} />
      </Row>
      <Row id="pr-tgldaftar" label="Tanggal Daftar">
        <input id="pr-tgldaftar" readOnly value={reg.tglMasuk ? `${fmtDate(reg.tglMasuk.slice(0, 10))} ${reg.tglMasuk.slice(11)}` : ""} />
      </Row>
      <Row id="pr-stskun" label="Sts. Kun">
        <input id="pr-stskun" readOnly value={reg.status} />
      </Row>
      <Row id="pr-pelayanan" label="Pelayanan">
        <input id="pr-pelayanan" readOnly value={reg.pelayanan} />
      </Row>
      <Row id="pr-norm" label="No. RM">
        <input id="pr-norm" readOnly value={reg.noRm} />
      </Row>
      <Row id="pr-nama" label="Nama & Umur">
        <div className="tx-split tx-split-name">
          <input id="pr-nama" readOnly value={reg.nama} />
          <input aria-label="Umur" readOnly value={umur === null ? "" : `${umur} Th`} />
        </div>
      </Row>
      <Row id="pr-alamat" label="Alamat">
        <input id="pr-alamat" readOnly value={reg.alamat} />
      </Row>
      <Row id="pr-hp" label="Telp / HP">
        <input id="pr-hp" readOnly value={reg.hp} />
      </Row>
      <Row id="pr-pj" label="Penanggung Jawab">
        <input id="pr-pj" readOnly value={reg.penanggung} />
      </Row>
      <Row id="pr-jamkes" label="Jam Kes / No Kartu">
        <div className="tx-split">
          <input id="pr-jamkes" readOnly value={reg.jamkes} />
          <input aria-label="No Kartu" readOnly value={reg.noJamkes} />
        </div>
      </Row>
    </Card>
  );
}
