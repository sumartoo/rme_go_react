import { useEffect, useState } from "react";
import { SquareCheck, X } from "lucide-react";
import { api, ApiError, PengobatanDokter } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { useModal } from "../components/FormKit";

/**
 * The Pengobatan form's Dokter PJ lookup ("Data Dokter"): the active doctors
 * mapped to the chosen branch (refmdokterauth). Picking one sets Dokter PJ.
 */
export default function DokterLookup({
  cabang,
  branchName,
  onPick,
  onClose,
  load = api.pengobatanDokter,
}: {
  cabang: string;
  branchName: string;
  onPick: (d: PengobatanDokter) => void;
  onClose: () => void;
  /** Where the doctors come from; Pemeriksaan Perawat uses its own endpoint. */
  load?: (token: string, cabang: string) => Promise<{ rows: PengobatanDokter[] }>;
}) {
  const { token, logout } = useAuth();
  const dialogRef = useModal();
  const [rows, setRows] = useState<PengobatanDokter[] | null>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    if (!token) return;
    load(token, cabang)
      .then((res) => setRows(res.rows))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
        else setError(err instanceof Error ? err.message : "Failed to load doctors");
      });
  }, [token, cabang]); // eslint-disable-line react-hooks/exhaustive-deps

  const columns: Column<PengobatanDokter>[] = [
    { key: "id", header: "Kode", value: (d) => String(d.id), sortValue: (d) => d.id },
    { key: "nama", header: "Nama Dokter", value: (d) => d.nama },
    { key: "jenis", header: "Jenis", value: (d) => d.jenis || "-" },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (d) => (
        <button type="button" className="row-action" aria-label={`Pilih dokter ${d.nama}`} title="Pilih" onClick={() => onPick(d)}>
          <SquareCheck aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <dialog ref={dialogRef} className="modal modal-xl lookup-dialog" aria-labelledby="dokter-lookup-title" onClose={onClose}>
      <header className="modal-head">
        <h2 id="dokter-lookup-title">Data Dokter · {branchName}</h2>
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
          <DataTable
            title="Data Dokter"
            columns={columns}
            rows={rows}
            rowKey={(d) => d.id}
            initialSort={{ key: "id", dir: "asc" }}
            pageSizes={[5, 10, 20, 50]}
            defaultPageSize={5}
            exports={false}
          />
        )}
      </div>
      <footer className="modal-foot">
        <button type="button" className="btn-small" onClick={onClose}>
          Close
        </button>
      </footer>
    </dialog>
  );
}
