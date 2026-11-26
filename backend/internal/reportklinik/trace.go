package reportklinik

import (
	"context"
	"fmt"
	"strings"
)

// TraceRow is one stock movement of Stok Farmasi > Stok Trace (legacy
// /StokTrace): a trxobat line with its drug and branch.
type TraceRow struct {
	ID          int     `json:"id"`
	IDObat      int     `json:"idObat"`
	Nama        string  `json:"nama"`
	Cabang      string  `json:"cabang"`
	NamaCabang  string  `json:"namaCabang"`
	LastStock   float64 `json:"lastStock"` // Stock Lama
	InOut       string  `json:"inOut"`     // In / Out
	Byk         float64 `json:"byk"`       // Banyak
	Stock       float64 `json:"stock"`
	SystemStock float64 `json:"systemStock"`
	TableRef    string  `json:"tableRef"`
	Transaksi   string  `json:"transaksi"` // table_ref as the legacy page names it
	Remarks     string  `json:"remarks"`   // Keterangan
	CreatedBy   string  `json:"createdBy"` // User
	CreatedDate string  `json:"createdDate"`
}

// TraceParams selects one page of Stok Trace.
type TraceParams struct {
	Cabang   string // one branch, always applied
	IDObat   *int   // nil: every drug at the branch
	From, To string // YYYY-MM-DD, inclusive, of createddate
	Search   string
	Sort     string
	Desc     bool
	Limit    int
	Offset   int
}

type TracePage struct {
	Rows            []TraceRow `json:"rows"`
	Total           int        `json:"total"`
	UnfilteredTotal int        `json:"unfilteredTotal"`
}

// traceTransaksi names table_ref like the legacy query; other values (the
// legacy TrxJualObatDokter) are shown as they are.
const traceTransaksi = `CASE a.table_ref
		WHEN 'TrxStokAdjus' THEN 'Stock Adjusment'
		WHEN 'TrxStokAdjusCancel' THEN 'Cancel Penjualan Obat'
		WHEN 'TrxFakturObat' THEN 'Penerimaan Obat'
		WHEN 'TrxJualObat' THEN 'Penjualan Obat'
		ELSE COALESCE(a.table_ref, '') END`

var traceSort = map[string]string{
	"id":          "a.id",
	"createdDate": "a.createddate",
	"nama":        "o.nama",
	"lastStock":   "a.laststock",
	"inOut":       "a.inout",
	"byk":         "a.byk",
	"stock":       "a.stock",
	"createdBy":   "a.createdby",
	"transaksi":   traceTransaksi,
	"remarks":     "a.remarks",
	"cabang":      "b.nama",
}

// TraceSortable reports whether key is a Stok Trace sort column.
func TraceSortable(key string) bool {
	_, ok := traceSort[key]
	return ok
}

const traceFrom = ` FROM trxobat a
	JOIN refobat o ON o.id = a.id_obat
	JOIN refbranch b ON b.kode = a.cabang`

func traceWhere(tp TraceParams, withSearch bool) (string, []any) {
	args := []any{tp.Cabang, tp.From, tp.To}
	conds := []string{`a.cabang = $1`, `a.createddate >= $2::date`, `a.createddate < $3::date + 1`}
	add := func(cond string, v any) {
		args = append(args, v)
		conds = append(conds, fmt.Sprintf(cond, len(args)))
	}
	if tp.IDObat != nil {
		add(`a.id_obat = $%d`, *tp.IDObat)
	}
	if withSearch && tp.Search != "" {
		args = append(args, "%"+escapeLike(tp.Search)+"%")
		conds = append(conds, fmt.Sprintf(`(o.nama ILIKE $%[1]d OR a.remarks ILIKE $%[1]d OR a.createdby ILIKE $%[1]d OR a.inout ILIKE $%[1]d
			OR (`+traceTransaksi+`) ILIKE $%[1]d)`, len(args)))
	}
	return strings.Join(conds, " AND "), args
}

func (p *PostgresRepository) StokTrace(ctx context.Context, tp TraceParams) (TracePage, error) {
	page := TracePage{Rows: []TraceRow{}}
	col, ok := traceSort[tp.Sort]
	if !ok {
		col = "a.id"
	}
	dir := "ASC"
	if tp.Desc {
		dir = "DESC"
	}
	cond, args := traceWhere(tp, true)
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`SELECT a.id, a.id_obat, COALESCE(o.nama, ''), a.cabang, COALESCE(b.nama, a.cabang),
			COALESCE(a.laststock, 0), COALESCE(a.inout, ''), COALESCE(a.byk, 0), COALESCE(a.stock, 0), COALESCE(a.systemstock, 0),
			COALESCE(a.table_ref, ''), `+traceTransaksi+`, COALESCE(a.remarks, ''), COALESCE(a.createdby, ''),
			to_char(a.createddate, 'YYYY-MM-DD HH24:MI:SS.MS'), count(*) OVER ()
		%s WHERE %s ORDER BY %s %s NULLS LAST, a.id LIMIT $%d OFFSET $%d`, traceFrom, cond, col, dir, n+1, n+2),
		append(args, tp.Limit, tp.Offset)...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var x TraceRow
		if err := rows.Scan(&x.ID, &x.IDObat, &x.Nama, &x.Cabang, &x.NamaCabang, &x.LastStock, &x.InOut, &x.Byk, &x.Stock, &x.SystemStock,
			&x.TableRef, &x.Transaksi, &x.Remarks, &x.CreatedBy, &x.CreatedDate, &page.Total); err != nil {
			return page, err
		}
		page.Rows = append(page.Rows, x)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	// An offset past the end returns no rows, and so no window count.
	if len(page.Rows) == 0 && tp.Offset > 0 {
		if err := p.pool.QueryRow(ctx, `SELECT count(*)`+traceFrom+` WHERE `+cond, args...).Scan(&page.Total); err != nil {
			return page, err
		}
	}
	if tp.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = traceWhere(tp, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*)`+traceFrom+` WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}
