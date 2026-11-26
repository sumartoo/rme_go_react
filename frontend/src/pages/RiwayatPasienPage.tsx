import { FormEvent, useState } from "react";
import { AlertTriangle, ArrowLeft, Search } from "lucide-react";
import { api, RiwayatKunjungan, RiwayatPasien, RiwayatPasienHit } from "../api";
import { useAuth } from "../auth/AuthContext";
import { num, rupiah, useFail } from "./analitikShared";

/**
 * Front Office Klinik > Riwayat Pasien: find a patient, then read their visits
 * at the user's branches with a structured summary — recurring diagnoses,
 * frequent drugs, vital-sign trend and allergy note. Computed from the
 * records only; nothing here is sent to an AI service.
 */
export default function RiwayatPasienPage() {
  const { token } = useAuth();
  const fail = useFail();
  const [q, setQ] = useState("");
  const [hits, setHits] = useState<{ rows: RiwayatPasienHit[]; total: number } | null>(null);
  const [searching, setSearching] = useState(false);
  const [data, setData] = useState<RiwayatPasien | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");

  const search = (e: FormEvent) => {
    e.preventDefault();
    if (!token || q.trim().length < 2) {
      setError("Ketik minimal 2 karakter (nama, No RM, NIK atau alamat).");
      return;
    }
    setSearching(true);
    setError("");
    api
      .riwayatPasienSearch(token, q.trim(), 1, 20)
      .then(setHits)
      .catch((err) => setError(fail(err, "Pencarian gagal")))
      .finally(() => setSearching(false));
  };

  const open = (id: number) => {
    if (!token) return;
    setLoading(true);
    setError("");
    api
      .riwayatPasien(token, id)
      .then(setData)
      .catch((err) => setError(fail(err, "Gagal memuat riwayat pasien")))
      .finally(() => setLoading(false));
  };

  return (
    <section className="panel">
      <div className="page-head">
        <h1>Riwayat Pasien</h1>
      </div>
      {error && (
        <p className="alert error" role="alert">
          {error}
        </p>
      )}
      {data ? (
        <Detail data={data} onBack={() => setData(null)} />
      ) : (
        <>
          <form className="an-controls" onSubmit={search} role="search">
            <label className="dt-length rp-search">
              <span>Cari pasien</span>
              <input className="select" value={q} placeholder="Nama, No RM, NIK atau alamat" onChange={(e) => setQ(e.target.value)} />
            </label>
            <button type="submit" className="btn-add" disabled={searching}>
              <Search aria-hidden="true" /> {searching ? "Mencari…" : "Cari"}
            </button>
          </form>
          {loading && <p className="muted">Memuat riwayat…</p>}
          {hits &&
            (hits.rows.length === 0 ? (
              <p className="muted">Tidak ada pasien yang cocok di cabang Anda.</p>
            ) : (
              <div className="rp-hits">
                <p className="muted">
                  {num.format(hits.total)} pasien{hits.total > hits.rows.length ? `, menampilkan ${hits.rows.length} pertama` : ""}. Pilih untuk melihat
                  riwayat.
                </p>
                <ul className="rp-hit-list">
                  {hits.rows.map((h) => (
                    <li key={h.id}>
                      <button type="button" className="rp-hit" onClick={() => open(h.id)}>
                        <strong>{h.nama}</strong>
                        <span className="muted">
                          {h.noRm} · {h.tglLahir ?? "tgl lahir -"} · {h.cabang}
                          {h.alamat ? ` · ${h.alamat}` : ""}
                        </span>
                      </button>
                    </li>
                  ))}
                </ul>
              </div>
            ))}
        </>
      )}
    </section>
  );
}

function bp(s: number | null, d: number | null) {
  return s || d ? `${s ?? "-"}/${d ?? "-"}` : "-";
}

function Detail({ data, onBack }: { data: RiwayatPasien; onBack: () => void }) {
  const p = data.pasien;
  const r = data.ringkasan;
  const alergi = p.alergi && !["-", "tidak ada", "tidak", "no", "none"].includes(p.alergi.trim().toLowerCase()) ? p.alergi : "";
  return (
    <div className="rp-detail">
      <button type="button" className="btn-add" onClick={onBack}>
        <ArrowLeft aria-hidden="true" /> Cari pasien lain
      </button>
      <div className="rp-head">
        <div>
          <h2>{p.nama}</h2>
          <p className="muted">
            {p.noRm} · {p.jenisKelamin || "-"} · {p.usia !== null ? `${p.usia} tahun` : "usia -"} ({p.tglLahir ?? "-"}) · {p.penjamin || "Umum"}
          </p>
          <p className="muted">
            {p.alamat || "-"} · {p.telp || "telp -"}
          </p>
        </div>
        {alergi ? (
          <p className="alert error rp-alergi" role="alert">
            <AlertTriangle aria-hidden="true" /> <strong>Alergi:</strong> {alergi}
          </p>
        ) : (
          <p className="muted rp-alergi">Tidak ada catatan alergi.</p>
        )}
      </div>

      <div className="an-kpis">
        <div className="an-kpi">
          <span className="an-kpi-label">Kunjungan</span>
          <span className="an-kpi-value">{num.format(r.jumlahKunjungan)}</span>
          <span className="an-kpi-sub">{r.cabang.join(", ") || "-"}</span>
        </div>
        <div className="an-kpi">
          <span className="an-kpi-label">Pertama</span>
          <span className="an-kpi-value rp-date">{r.pertama || "-"}</span>
        </div>
        <div className="an-kpi">
          <span className="an-kpi-label">Terakhir</span>
          <span className="an-kpi-value rp-date">{r.terakhir || "-"}</span>
        </div>
      </div>

      <div className="rp-grid">
        <section className="an-ai-card">
          <h2>Diagnosa</h2>
          {r.diagnosa.length === 0 ? (
            <p className="muted">Belum ada diagnosa tercatat.</p>
          ) : (
            <ul className="rp-count-list">
              {r.diagnosa.slice(0, 12).map((d) => (
                <li key={`${d.kode}|${d.nama}`}>
                  <span>
                    <strong>{d.kode || "-"}</strong> {d.nama}
                  </span>
                  <span className="muted">
                    {d.kali}× · terakhir {d.terakhir}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </section>
        <section className="an-ai-card">
          <h2>Obat tersering</h2>
          {r.obat.length === 0 ? (
            <p className="muted">Belum ada resep tercatat.</p>
          ) : (
            <ul className="rp-count-list">
              {r.obat.map((o) => (
                <li key={o.nama}>
                  <span>{o.nama}</span>
                  <span className="muted">
                    {o.kali}× · terakhir {o.terakhir}
                  </span>
                </li>
              ))}
            </ul>
          )}
        </section>
        <section className="an-ai-card">
          <h2>Tanda vital</h2>
          {r.vital.length === 0 ? (
            <p className="muted">Belum ada tanda vital tercatat.</p>
          ) : (
            <table className="table rp-vital">
              <thead>
                <tr>
                  <th>Tanggal</th>
                  <th>TD (mmHg)</th>
                  <th className="num">BB</th>
                  <th className="num">TB</th>
                  <th className="num">IMT</th>
                </tr>
              </thead>
              <tbody>
                {r.vital.map((v, i) => (
                  <tr key={i}>
                    <td>{v.tgl}</td>
                    <td className={v.sistole !== null && v.sistole >= 140 ? "rp-high" : undefined}>{bp(v.sistole, v.diastole)}</td>
                    <td className="num">{v.bb ?? "-"}</td>
                    <td className="num">{v.tb ?? "-"}</td>
                    <td className="num">{v.imt !== null ? num.format(v.imt) : "-"}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </section>
      </div>

      <h2 className="rp-timeline-title">Kunjungan ({data.kunjungan.length}{data.kunjungan.length >= data.maxKunjungan ? `, ${data.maxKunjungan} terbaru` : ""})</h2>
      <ol className="rp-timeline">
        {data.kunjungan.map((k) => (
          <Visit key={k.idJual} k={k} />
        ))}
      </ol>
      <p className="muted an-foot">
        Ringkasan dihitung langsung dari catatan kunjungan di cabang Anda (tanpa AI). Tekanan darah ≥140 ditandai. Kunjungan yang dibatalkan tidak
        dihitung di ringkasan.
      </p>
    </div>
  );
}

function Visit({ k }: { k: RiwayatKunjungan }) {
  const vital = [
    k.sistole || k.diastole ? `TD ${bp(k.sistole, k.diastole)}` : "",
    k.bb ? `BB ${k.bb} kg` : "",
    k.tb ? `TB ${k.tb} cm` : "",
    k.suhu ? `Suhu ${num.format(k.suhu)}°C` : "",
  ].filter(Boolean);
  return (
    <li className={`rp-visit${k.batal ? " rp-batal" : ""}`}>
      <header>
        <strong>{k.tgl}</strong>
        <span className="muted">
          {" "}
          · {k.cabang} · {k.dokter || "tanpa dokter"} · {k.jenisRawat || "-"}
          {k.noKwi ? ` · ${k.noKwi}` : ""}
          {k.total !== null ? ` · ${rupiah.format(k.total)}` : ""}
        </span>
        {k.batal && <span className="an-status an-kritis"> Dibatalkan</span>}
      </header>
      {(k.keluhan || k.anamnesis) && (
        <p>
          <span className="muted">Keluhan:</span> {[k.keluhan, k.anamnesis].filter(Boolean).join(" — ")}
        </p>
      )}
      {vital.length > 0 && <p className="muted">{vital.join(" · ")}</p>}
      {k.diagnosa.length > 0 && (
        <p>
          <span className="muted">Diagnosa:</span> {k.diagnosa.map((d) => `${d.kode ? `${d.kode} ` : ""}${d.nama}`).join("; ")}
        </p>
      )}
      {k.resep.length > 0 && (
        <p>
          <span className="muted">Resep:</span>{" "}
          {k.resep.map((o) => `${o.nama}${o.byk ? ` (${num.format(o.byk)})` : ""}${o.dosis ? ` ${o.dosis}` : ""}`).join("; ")}
        </p>
      )}
      {k.tindakan.length > 0 && (
        <p>
          <span className="muted">Tindakan:</span> {k.tindakan.map((t) => t.nama).join("; ")}
        </p>
      )}
      {(k.hasilDokter || k.rencana) && (
        <p>
          <span className="muted">Dokter:</span> {[k.hasilDokter, k.rencana].filter(Boolean).join(" — ")}
        </p>
      )}
    </li>
  );
}
