import { useCallback, useEffect, useRef, useState } from "react";
import { SquareCheck, X } from "lucide-react";
import { api, ApiError, McuDaftarPasien } from "../../api";
import { useAuth } from "../../auth/AuthContext";
import DataTable, { Column, RemoteQuery } from "../../components/DataTable";
import { useModal } from "../../components/FormKit";

/**
 * A lookup over a list loaded once (General, Paket MCU, Non Lab, Dokter). The
 * Pilih button adds a row; rows already chosen are disabled. It stays open so
 * several rows can be added; Close ends it.
 */
export function PickDialog<T>({
  title,
  load,
  columns,
  rowKey,
  chosen,
  onPick,
  onClose,
}: {
  title: string;
  load: (token: string) => Promise<{ rows: T[] }>;
  columns: Column<T>[];
  rowKey: (row: T) => string | number;
  chosen: Set<string | number>;
  onPick: (row: T) => void;
  onClose: () => void;
}) {
  const { token, logout } = useAuth();
  const dialogRef = useModal();
  const [rows, setRows] = useState<T[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!token) return;
    load(token)
      .then((res) => setRows(res.rows))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
        else setError(err instanceof Error ? err.message : "Gagal memuat data");
      });
  }, [token]); // eslint-disable-line react-hooks/exhaustive-deps

  const all: Column<T>[] = [
    ...columns,
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (r) => {
        const done = chosen.has(rowKey(r));
        return (
          <button type="button" className="row-action" aria-label={`Pilih ${rowKey(r)}`} title={done ? "Sudah dipilih" : "Pilih"} disabled={done} onClick={() => onPick(r)}>
            <SquareCheck aria-hidden="true" />
          </button>
        );
      },
    },
  ];

  return (
    <dialog ref={dialogRef} className="modal modal-xl lookup-dialog" aria-labelledby="mcu-pick-title" onClose={onClose}>
      <header className="modal-head">
        <h2 id="mcu-pick-title">{title}</h2>
        <button type="button" className="row-action" aria-label="Close" onClick={onClose}>
          <X aria-hidden="true" />
        </button>
      </header>
      <div className="modal-body">
        {error ? (
          <p className="alert error" role="alert">
            {error}
          </p>
        ) : !rows ? (
          <p className="muted">Loading…</p>
        ) : (
          <DataTable title={title} columns={all} rows={rows} rowKey={rowKey} pageSizes={[5, 10, 20, 50]} defaultPageSize={10} exports={false} />
        )}
      </div>
      <footer className="modal-foot">
        <span className="muted">{chosen.size} dipilih</span>
        <button type="button" className="btn-small" onClick={onClose}>
          Close
        </button>
      </footer>
    </dialog>
  );
}

/**
 * The patient lookup ("Add Pasien"): MCU patients paged and searched on the
 * server. Several patients can be added; each gets its own registration.
 */
export function PasienLookup({
  chosen,
  onPick,
  onClose,
}: {
  chosen: Set<number>;
  onPick: (p: McuDaftarPasien) => void;
  onClose: () => void;
}) {
  const { token, logout } = useAuth();
  const dialogRef = useModal();
  const [page, setPage] = useState<{ rows: McuDaftarPasien[]; total: number } | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState("");
  const seq = useRef(0);

  const fetchPage = useCallback(
    (q: RemoteQuery) => {
      if (!token) return;
      const mine = ++seq.current;
      setLoading(true);
      api
        .mcuDaftarPasien(token, { q: q.search, page: q.page, size: q.pageSize })
        .then((res) => mine === seq.current && setPage(res))
        .catch((err) => {
          if (mine !== seq.current) return;
          if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
          else setError(err instanceof Error ? err.message : "Gagal memuat pasien");
        })
        .finally(() => mine === seq.current && setLoading(false));
    },
    [token], // eslint-disable-line react-hooks/exhaustive-deps
  );

  const columns: Column<McuDaftarPasien>[] = [
    { key: "id", header: "Pasien ID", value: (p) => String(p.id), sortable: false },
    { key: "rm", header: "No RM", value: (p) => p.rm || "-", sortable: false },
    { key: "nama", header: "Nama", value: (p) => p.nama, sortable: false, className: "dt-wrap-sm" },
    { key: "kodePerusahaan", header: "Kode Perusahaan", value: (p) => p.kodePerusahaan || "-", sortable: false },
    { key: "perusahaan", header: "Perusahaan", value: (p) => p.perusahaan || "-", sortable: false, className: "dt-wrap-sm" },
    { key: "nik", header: "NIK", value: (p) => p.nik || "-", sortable: false },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (p) => (
        <button
          type="button"
          className="row-action"
          aria-label={`Pilih pasien ${p.nama}`}
          title={chosen.has(p.id) ? "Sudah dipilih" : "Pilih"}
          disabled={chosen.has(p.id)}
          onClick={() => onPick(p)}
        >
          <SquareCheck aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <dialog ref={dialogRef} className="modal modal-xl lookup-dialog" aria-labelledby="mcu-pasien-title" onClose={onClose}>
      <header className="modal-head">
        <h2 id="mcu-pasien-title">Data Pasien</h2>
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
        <DataTable
          title="Data Pasien"
          columns={columns}
          rows={page?.rows ?? []}
          rowKey={(p) => p.id}
          pageSizes={[5, 10, 20, 50]}
          defaultPageSize={10}
          exports={false}
          remote={{ total: page?.total ?? 0, unfilteredTotal: page?.total ?? 0, loading: loading || !page, onQuery: fetchPage }}
        />
      </div>
      <footer className="modal-foot">
        <span className="muted">{chosen.size} pasien dipilih</span>
        <button type="button" className="btn-small" onClick={onClose}>
          Close
        </button>
      </footer>
    </dialog>
  );
}
