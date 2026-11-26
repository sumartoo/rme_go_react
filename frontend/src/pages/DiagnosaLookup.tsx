import { FormEvent, useCallback, useRef, useState } from "react";
import { Sparkles, SquareCheck, X } from "lucide-react";
import { api, ApiError, IcdKandidat, PengobatanIcd } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../components/DataTable";
import { useModal } from "../components/FormKit";

/**
 * The Diagnosa tab's "Data Diagnosa" lookup: the ICD-X codes (18.5k), paged and
 * searched on the server, by ID like the original screen. Ticking a code adds it.
 */
export default function DiagnosaLookup({
  chosen,
  onPick,
  onClose,
  load = api.pengobatanIcd,
}: {
  /** Where the codes come from; Pemeriksaan Dokter uses its own endpoint. */
  load?: typeof api.pengobatanIcd;
  /** Codes already in the list; picking one again is refused. */
  chosen: Set<number>;
  onPick: (d: PengobatanIcd) => void;
  onClose: () => void;
}) {
  const { token, logout } = useAuth();
  const dialogRef = useModal();
  const [page, setPage] = useState<{ rows: PengobatanIcd[]; total: number; unfilteredTotal: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const seq = useRef(0);

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token) return;
      const mine = ++seq.current;
      setLoading(true);
      load(token, { q: q.search, page: q.page, size: q.pageSize, sort: q.sort?.key, dir: q.sort?.dir })
        .then((res) => mine === seq.current && setPage(res))
        .catch((err) => {
          if (mine !== seq.current) return;
          if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
          else setError(err instanceof Error ? err.message : "Failed to load ICD-X codes");
        })
        .finally(() => mine === seq.current && setLoading(false));
    },
    [token], // eslint-disable-line react-hooks/exhaustive-deps
  );

  const pick = (d: PengobatanIcd) => {
    if (chosen.has(d.id)) {
      setError(`${d.kode} ${d.nama} sudah ada di daftar diagnosa.`);
      return;
    }
    setError("");
    onPick(d);
  };

  const columns: Column<PengobatanIcd>[] = [
    { key: "id", header: "ID", value: (d) => String(d.id) },
    { key: "kode", header: "Kode ICD-X", value: (d) => d.kode },
    { key: "nama", header: "Nama Penyakit", value: (d) => d.nama, className: "dt-wrap-sm" },
    { key: "jenis", header: "Jenis ICD-X", value: (d) => d.jenis || "-" },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (d) => (
        <button
          type="button"
          className="row-action"
          aria-label={`Pilih diagnosa ${d.kode}`}
          title={chosen.has(d.id) ? "Sudah dipilih" : "Pilih"}
          disabled={chosen.has(d.id)}
          onClick={() => pick(d)}
        >
          <SquareCheck aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <dialog ref={dialogRef} className="modal modal-xl lookup-dialog" aria-labelledby="icd-lookup-title" onClose={onClose}>
      <header className="modal-head">
        <h2 id="icd-lookup-title">Data Diagnosa</h2>
        <button type="button" className="row-action" aria-label="Close" onClick={onClose}>
          <X aria-hidden="true" />
        </button>
      </header>
      <div className="modal-body">
        {error && (
          <p className="alert error" role="alert">
            {error}
          </p>
        )}
        <SaranIcd chosen={chosen} onPick={pick} />
        <DataTable
          title="Data Diagnosa"
          columns={columns}
          rows={page?.rows ?? []}
          rowKey={(d) => d.id}
          initialSort={{ key: "id", dir: "asc" }}
          pageSizes={[5, 10, 20, 50]}
          defaultPageSize={10}
          exports={false}
          remote={{ total: page?.total ?? 0, unfilteredTotal: page?.unfilteredTotal ?? 0, loading: loading || !page, onQuery: fetchPage }}
        />
      </div>
      <footer className="modal-foot">
        <button type="button" className="btn-small" onClick={onClose}>
          Close
        </button>
      </footer>
    </dialog>
  );
}

/**
 * "Saran ICD": the doctor describes the case in words and gets candidate codes
 * from the ICD master — the AI's (with a reason) when the backend has an AI
 * key, then plain search matches. Only the description is sent, and the
 * doctor still picks the code.
 */
function SaranIcd({ chosen, onPick }: { chosen: Set<number>; onPick: (d: PengobatanIcd) => void }) {
  const { token, logout } = useAuth();
  const [open, setOpen] = useState(false);
  const [text, setText] = useState("");
  const [busy, setBusy] = useState(false);
  const [res, setRes] = useState<{ ai: boolean; aiError: string; kandidat: IcdKandidat[] } | null>(null);
  const [error, setError] = useState("");

  const ask = (e: FormEvent) => {
    e.preventDefault();
    if (!token || text.trim().length < 3) {
      setError("Tulis deskripsi klinis minimal 3 karakter.");
      return;
    }
    setBusy(true);
    setError("");
    api
      .saranIcd(token, text.trim())
      .then(setRes)
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
        else setError(err instanceof ApiError ? Object.values(err.fieldErrors)[0] ?? err.message : "Gagal mencari saran ICD");
      })
      .finally(() => setBusy(false));
  };

  if (!open) {
    return (
      <p>
        <button type="button" className="btn-add" onClick={() => setOpen(true)}>
          <Sparkles aria-hidden="true" /> Saran ICD dari deskripsi
        </button>
      </p>
    );
  }
  return (
    <section className="icd-saran" aria-label="Saran ICD">
      <form onSubmit={ask} className="icd-saran-form">
        <textarea
          className="select"
          rows={2}
          maxLength={500}
          value={text}
          placeholder="Deskripsi klinis, mis. batuk pilek 3 hari, demam, nyeri tenggorokan"
          aria-label="Deskripsi klinis"
          onChange={(e) => setText(e.target.value)}
        />
        <button type="submit" className="btn-add" disabled={busy}>
          <Sparkles aria-hidden="true" /> {busy ? "Mencari…" : "Cari saran"}
        </button>
      </form>
      {error && (
        <p className="alert error" role="alert">
          {error}
        </p>
      )}
      {res && (
        <>
          {res.aiError && <p className="muted">{res.aiError}</p>}
          {res.kandidat.length === 0 ? (
            <p className="muted">Tidak ada kode yang cocok. Coba deskripsi lain atau cari di tabel di bawah.</p>
          ) : (
            <ul className="icd-saran-list">
              {res.kandidat.map((k) => (
                <li key={k.id}>
                  <button
                    type="button"
                    className="icd-saran-item"
                    disabled={chosen.has(k.id)}
                    onClick={() => onPick({ id: k.id, kode: k.kode, nama: k.nama, jenis: k.jenis })}
                  >
                    <span>
                      <strong>{k.kode}</strong> {k.nama}
                      {k.sumber === "ai" && <span className="chip icd-ai-chip">AI</span>}
                    </span>
                    {k.alasan && <span className="muted">{k.alasan}</span>}
                    {chosen.has(k.id) && <span className="muted">Sudah dipilih</span>}
                  </button>
                </li>
              ))}
            </ul>
          )}
          <p className="muted an-foot">
            Saran untuk ditinjau dokter, bukan diagnosis. {res.ai ? "Hanya teks deskripsi yang dikirim ke AI, tanpa identitas pasien." : ""}
          </p>
        </>
      )}
    </section>
  );
}
