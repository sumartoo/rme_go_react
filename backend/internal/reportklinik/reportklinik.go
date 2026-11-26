// Package reportklinik serves the Report Klinik pages. Each report is one
// parameterized query over the sale tables, limited to the caller's branches
// (refbranchauth) and guarded by its own menu item.
//
// Penjualan Obat (legacy /PenjualanObat): the drugs sold per day, grouped by
// drug, price, branch and settlement status, with the quantities summed.
//
// Penjualan Tindakan (legacy /PenjualanTindakan): every procedure line sold,
// with its doctor, patient and the doctor's fee.
//
// Penjualan Penunjang (legacy /PenjualanPenunjang): every supporting
// examination sold, with the patient, the sale's doctor and its cashier.
//
// Penjualan OTC (legacy /PenjualanOTC) and Penjualan Softdrink (legacy
// /PenjualanSoftdrink): every drug line of type OTC, or SOFT DRINK, with the
// sale's OTC referrer, PPN and total.
//
// Pasien Baru (legacy /PasienBaru): the visits (sales) of patients registered
// in the period.
//
// Rekap Asuransi (legacy /ReportAsuransi): the insured sales with their items
// per group, diagnoses and total.
//
// Transaksi Harian (legacy /ReportHarian): every sale with its cost per group,
// total and discount.
//
// All Stok (legacy /ReportAllStock): every drug at every branch with its
// stock at the start and end of the period and the drugs sold in it.
//
// Rujukan (legacy /ReportRujukan): the sales of external doctors, once per
// supporting examination.
//
// Pendaftaran (legacy /ReportPendaftaranKlinik): the registrations with their
// queue number, service, visit status and patient.
//
// Report Pendapatan (legacy /ReportPendapatan): one branch's sales in a period,
// downloaded as the "Laporan Pendapatan Klinik" PDF.
//
// Report Diagnosa (legacy /ReportDiagnosa): every diagnosis of a sale with the
// patient, the prescription and the visit's main complaint.
//
// Report ICD Dinkes (legacy /ReportDinkes): the cases per ICD code and service,
// by age group and sex, for the health office (Dinas Kesehatan).
package reportklinik

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/example/be_jwt_golang/backend/internal/branchauth"
)

// Settlement statuses (trxjualtotal.status), as on the Pengobatan list.
const (
	StatusOpen   = "open"   // anything but 1 or 2, also a sale without a total
	StatusClose  = "close"  // 1
	StatusCancel = "cancel" // 2
)

const statusSQL = `CASE t.status WHEN 1 THEN 'close' WHEN 2 THEN 'cancel' ELSE 'open' END`

type Option struct {
	ID   int    `json:"id"`
	Name string `json:"name"`
}

// ObatRow is one line of Penjualan Obat: a drug sold on one day at one price
// and branch, with the sales' status; Banyak is the quantity summed.
type ObatRow struct {
	Tgl    string  `json:"tgl"` // YYYY-MM-DD, clinic date
	IDObat int     `json:"idObat"`
	Obat   string  `json:"obat"`
	Jenis  string  `json:"jenis"`  // refjenisobat
	Satuan string  `json:"satuan"` // the selling unit
	Harga  float64 `json:"harga"`
	Byk    float64 `json:"byk"`
	Jumlah float64 `json:"jumlah"` // Harga × Banyak
	Cabang string  `json:"cabang"`
	Status string  `json:"status"` // open, close or cancel
}

// ObatParams selects one page of Penjualan Obat.
type ObatParams struct {
	Branches []string // always applied
	From, To string   // YYYY-MM-DD, inclusive, clinic dates
	Search   string   // the table's search box
	Nama     string   // Cari: drug name
	IDJenis  *int     // Cari: refjenisobat id
	Status   string   // Cari: open, close, cancel or ""
	Sort     string
	Desc     bool
	Limit    int
	Offset   int
}

type ObatPage struct {
	Rows            []ObatRow `json:"rows"`
	Total           int       `json:"total"`
	UnfilteredTotal int       `json:"unfilteredTotal"`
}

type Repository interface {
	UserBranches(ctx context.Context, userID string) ([]branchauth.Branch, error)
	JenisObat(ctx context.Context) ([]Option, error)
	PenjualanObat(ctx context.Context, p ObatParams) (ObatPage, error)
	PenjualanTindakan(ctx context.Context, p TindakanParams) (TindakanPage, error)
	JenisPenunjang(ctx context.Context) ([]Option, error)
	PenjualanPenunjang(ctx context.Context, p PenunjangParams) (PenunjangPage, error)
	PerujukOTC(ctx context.Context) ([]Option, error)
	PenjualanOTC(ctx context.Context, p OTCParams) (OTCPage, error)
	JenisRawat(ctx context.Context) ([]Option, error)
	PasienBaru(ctx context.Context, p PasienBaruParams) (PasienBaruPage, error)
	Asuransi(ctx context.Context) ([]Option, error)
	RekapAsuransi(ctx context.Context, p AsuransiParams) (AsuransiPage, error)
	JenisBayar(ctx context.Context) ([]Option, error)
	TransaksiHarian(ctx context.Context, p HarianParams) (HarianPage, error)
	MovingPO(ctx context.Context) ([]Option, error)
	AllStok(ctx context.Context, p StokParams) (StokPage, error)
	Rujukan(ctx context.Context, p RujukanParams) (RujukanPage, error)
	StatusKunjungan(ctx context.Context) ([]Option, error)
	Pendaftaran(ctx context.Context, p PendaftaranParams) (PendaftaranPage, error)
	Pendapatan(ctx context.Context, p PendapatanParams) (Pendapatan, error)
	DokterCabang(ctx context.Context, cabang string) ([]Option, error)
	ReportDiagnosa(ctx context.Context, p DiagnosaParams) (DiagnosaPage, error)
	ReportDinkes(ctx context.Context, p DinkesParams) (DinkesPage, error)
	StokOpname(ctx context.Context, p OpnameParams) (OpnamePage, error)
	ObatCabang(ctx context.Context, branch string) ([]ObatCabang, error)
	StokTrace(ctx context.Context, p TraceParams) (TracePage, error)
}

type PostgresRepository struct {
	pool *pgxpool.Pool
}

func NewPostgresRepository(pool *pgxpool.Pool) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (p *PostgresRepository) UserBranches(ctx context.Context, userID string) ([]branchauth.Branch, error) {
	return branchauth.ForUser(ctx, p.pool, userID)
}

func (p *PostgresRepository) JenisObat(ctx context.Context) ([]Option, error) {
	return p.optionList(ctx, `SELECT id, COALESCE(info, '') FROM refjenisobat ORDER BY info, id`)
}

// optionList runs a query of (id, name) rows.
func (p *PostgresRepository) optionList(ctx context.Context, sql string) ([]Option, error) {
	rows, err := p.pool.Query(ctx, sql)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Option, error) {
		var o Option
		err := r.Scan(&o.ID, &o.Name)
		return o, err
	})
	if list == nil {
		list = []Option{}
	}
	return list, err
}

func escapeLike(s string) string {
	return strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`).Replace(s)
}

// obatSort are the sortable columns of the grouped rows (fixed SQL, never from the request).
var obatSort = map[string]string{
	"tgl":    "g.tgl",
	"obat":   "g.obat",
	"jenis":  "g.jenis",
	"satuan": "g.satuan",
	"harga":  "g.harga",
	"byk":    "g.byk",
	"jumlah": "g.jumlah",
	"cabang": "g.cabang_nama",
	"status": "g.status",
}

// ObatSortable reports whether key is a Penjualan Obat sort column.
func ObatSortable(key string) bool {
	_, ok := obatSort[key]
	return ok
}

// obatGrouped is the grouped query; withSearch adds the search box on the grouped rows.
func obatGrouped(op ObatParams, withSearch bool) (string, []any) {
	args := []any{op.Branches, op.From, op.To}
	conds := []string{`j.cabang = ANY($1)`, `j.tgl >= $2::date`, `j.tgl < $3::date + 1`}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if op.Nama != "" {
		add(`o.nama ILIKE $%d`, "%"+escapeLike(op.Nama)+"%")
	}
	if op.IDJenis != nil {
		add(`o.id_jenis = $%d`, *op.IDJenis)
	}
	if op.Status != "" {
		add(statusSQL+` = $%d`, op.Status)
	}
	sql := `SELECT j.tgl::date AS tgl, x.id_obat, COALESCE(o.nama, '(obat ' || x.id_obat || ')') AS obat,
			COALESCE(jo.info, '') AS jenis, COALESCE(s.info, '') AS satuan, COALESCE(x.hjual, 0) AS harga,
			sum(COALESCE(x.byk, 0)) AS byk, sum(COALESCE(x.hjual, 0) * COALESCE(x.byk, 0)) AS jumlah,
			COALESCE(j.cabang, '') AS cabang, COALESCE(b.nama, j.cabang, '') AS cabang_nama, ` + statusSQL + ` AS status
		FROM trxjual j
		JOIN trxjualobat x ON x.id_jual = j.id
		LEFT JOIN trxjualtotal t ON t.id_jual = j.id
		LEFT JOIN refobat o ON o.id = x.id_obat
		LEFT JOIN refjenisobat jo ON jo.id = o.id_jenis
		LEFT JOIN refsatuan s ON s.id = o.sat_jual
		LEFT JOIN refbranch b ON b.kode = j.cabang
		WHERE ` + strings.Join(conds, " AND ") + `
		GROUP BY 1, 2, 3, 4, 5, 6, 9, 10, 11`
	search := ""
	if withSearch && op.Search != "" {
		args = append(args, "%"+escapeLike(op.Search)+"%")
		search = fmt.Sprintf(` WHERE g.obat ILIKE $%[1]d OR g.jenis ILIKE $%[1]d OR g.satuan ILIKE $%[1]d OR g.cabang_nama ILIKE $%[1]d`, len(args))
	}
	return `FROM (` + sql + `) g` + search, args
}

func (p *PostgresRepository) PenjualanObat(ctx context.Context, op ObatParams) (ObatPage, error) {
	page := ObatPage{Rows: []ObatRow{}}
	col, ok := obatSort[op.Sort]
	if !ok {
		col = "g.obat"
	}
	dir := "ASC"
	if op.Desc {
		dir = "DESC"
	}
	from, args := obatGrouped(op, true)
	n := len(args)
	// Ties: drug, then day, then branch, as the legacy report lists them.
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`SELECT to_char(g.tgl, 'YYYY-MM-DD'), g.id_obat, g.obat, g.jenis, g.satuan, g.harga, g.byk, g.jumlah,
			g.cabang, g.status, count(*) OVER ()
		%s ORDER BY %s %s NULLS LAST, g.obat, g.tgl, g.cabang_nama, g.harga LIMIT $%d OFFSET $%d`, from, col, dir, n+1, n+2),
		append(args, op.Limit, op.Offset)...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var x ObatRow
		if err := rows.Scan(&x.Tgl, &x.IDObat, &x.Obat, &x.Jenis, &x.Satuan, &x.Harga, &x.Byk, &x.Jumlah, &x.Cabang, &x.Status, &page.Total); err != nil {
			return page, err
		}
		page.Rows = append(page.Rows, x)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	// An offset past the end returns no rows, and so no window count.
	if len(page.Rows) == 0 && op.Offset > 0 {
		if err := p.pool.QueryRow(ctx, `SELECT count(*) `+from, args...).Scan(&page.Total); err != nil {
			return page, err
		}
	}
	if op.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	from, args = obatGrouped(op, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*) `+from, args...).Scan(&page.UnfilteredTotal)
	return page, err
}
