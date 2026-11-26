import { useEffect, useState } from "react";
import { SquareCheck, X } from "lucide-react";
import { api, ApiError, PengobatanObat } from "../api";
import { useAuth } from "../auth/AuthContext";
import DataTable, { Column } from "../components/DataTable";
import { useModal } from "../components/FormKit";

const money = new Intl.NumberFormat("id-ID", { minimumFractionDigits: 2, maximumFractionDigits: 2 });
const qtyFmt = new Intl.NumberFormat("id-ID", { maximumFractionDigits: 2 });

/**
 * The Pengobatan form's "Data Obat" lookup: the drugs sold at the chosen branch
 * with that branch's price and stock (trxstock). Type Banyak and press Enter,
 * or tick the row, to add it to the drug list.
 */
export default function ObatLookup({
  cabang,
  branchName,
  inList,
  onPick,
  onClose,
  load = api.pengobatanObat,
  checkStock = true,
}: {
  cabang: string;
  branchName: string;
  /** Where the drugs come from; Pemeriksaan Dokter uses its own endpoint. */
  load?: (token: string, cabang: string) => Promise<{ rows: PengobatanObat[] }>;
  /** Refuse more than the branch's stock (a sale); a prescription does not check it. */
  checkStock?: boolean;
  /** Quantity of each drug already in the list, to check the stock. */
  inList: Map<number, number>;
  onPick: (o: PengobatanObat, byk: number) => void;
  onClose: () => void;
}) {
  const { token, logout } = useAuth();
  const dialogRef = useModal();
  const [rows, setRows] = useState<PengobatanObat[] | null>(null);
  const [qty, setQty] = useState<Record<number, string>>({});
  const [error, setError] = useState("");

  useEffect(() => {
    if (!token) return;
    load(token, cabang)
      .then((res) => setRows(res.rows))
      .catch((err) => {
        if (err instanceof ApiError && err.status === 401) logout("Your session has expired. Please log in again.");
        else setError(err instanceof Error ? err.message : "Failed to load drugs");
      });
  }, [token, cabang]); // eslint-disable-line react-hooks/exhaustive-deps

  const pick = (o: PengobatanObat) => {
    const raw = (qty[o.id] ?? "1").trim().replace(",", ".");
    const byk = Number(raw);
    if (!raw || !Number.isFinite(byk) || byk <= 0) {
      setError(`Banyak untuk ${o.nama} harus lebih dari 0.`);
      return;
    }
    const already = inList.get(o.id) ?? 0;
    if (checkStock && already + byk > o.stok) {
      setError(
        `Stok ${o.nama} tidak cukup: stok ${qtyFmt.format(o.stok)}${already ? `, sudah di daftar ${qtyFmt.format(already)}` : ""}, diminta ${qtyFmt.format(byk)}.`,
      );
      return;
    }
    setError("");
    onPick(o, byk);
  };

  const columns: Column<PengobatanObat>[] = [
    { key: "id", header: "ID Obat", value: (o) => String(o.id), sortValue: (o) => o.id },
    { key: "nama", header: "Nama Obat", value: (o) => o.nama, className: "dt-wrap-sm" },
    { key: "jenis", header: "Jenis", value: (o) => o.jenis || "-" },
    { key: "satuan", header: "Satuan", value: (o) => o.satuan || "-" },
    { key: "harga", header: "Harga", value: (o) => money.format(o.harga), sortValue: (o) => o.harga, className: "dt-num" },
    {
      key: "stok",
      header: "Stok",
      value: (o) => qtyFmt.format(o.stok),
      sortValue: (o) => o.stok,
      className: "dt-num",
      render: (o) => <span className={o.stok <= 0 ? "stock-out" : undefined}>{qtyFmt.format(o.stok)}</span>,
    },
    {
      key: "banyak",
      header: "Banyak",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (o) => (
        <input
          className="qty-input"
          type="number"
          min={0}
          step="any"
          inputMode="decimal"
          aria-label={`Banyak ${o.nama}`}
          value={qty[o.id] ?? "1"}
          onChange={(e) => setQty({ ...qty, [o.id]: e.target.value })}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              e.preventDefault();
              pick(o);
            }
          }}
        />
      ),
    },
    {
      key: "action",
      header: "Action",
      value: () => "",
      sortable: false,
      exportable: false,
      render: (o) => (
        <button type="button" className="row-action" aria-label={`Pilih obat ${o.nama}`} title="Pilih" onClick={() => pick(o)}>
          <SquareCheck aria-hidden="true" />
        </button>
      ),
    },
  ];

  return (
    <dialog ref={dialogRef} className="modal modal-xl lookup-dialog" aria-labelledby="obat-lookup-title" onClose={onClose}>
      <header className="modal-head">
        <h2 id="obat-lookup-title">Data Obat · {branchName}</h2>
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
        {!rows ? (
          !error && <p className="muted">Loading…</p>
        ) : (
          <DataTable
            title="Data Obat"
            columns={columns}
            rows={rows}
            rowKey={(o) => o.id}
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
