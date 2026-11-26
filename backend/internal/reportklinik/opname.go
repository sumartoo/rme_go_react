package reportklinik

import (
	"context"
	"fmt"
	"strings"
)

// OpnameRow is one drug at one branch of Stok Farmasi > Stok Opname (legacy
// /StokOpname): the drug master (refobat) joined with its stock and prices at
// the branch (trxstock).
type OpnameRow struct {
	IDObat     int     `json:"idObat"` // Kode
	Nama       string  `json:"nama"`
	HargaJual  float64 `json:"hargaJual"` // trxstock.hjual
	HargaBeli  float64 `json:"hargaBeli"` // trxstock.hbeli
	BeratIsi   string  `json:"beratIsi"`  // refobat.isi (text)
	IDJenis    *int    `json:"idJenis"`
	Jenis      string  `json:"jenis"`
	SatuanBeli string  `json:"satuanBeli"`
	SatuanJual string  `json:"satuanJual"`
	JumUnit    float64 `json:"jumUnit"` // refobat.qjb
	StokMin    float64 `json:"stokMin"`
	Stok       float64 `json:"stok"`
	Cabang     string  `json:"cabang"` // refbranch.kode
	NamaCabang string  `json:"namaCabang"`
	KenaPajak  bool    `json:"kenaPajak"`
	IsActive   bool    `json:"isActive"` // trxstock.isactive = 1
}

// OpnameParams selects one page of Stok Opname.
type OpnameParams struct {
	Branches []string // always applied
	Search   string   // the table's search box
	Nama     string   // Filter: drug name
	IDJenis  *int     // Filter: refjenisobat id
	Aktif    *bool    // Filter: trxstock.isactive
	BelowMin bool     // Stok Alert: only rows whose stock is below the minimum
	Sort     string
	Desc     bool
	Limit    int
	Offset   int
}

type OpnamePage struct {
	Rows            []OpnameRow `json:"rows"`
	Total           int         `json:"total"`
	UnfilteredTotal int         `json:"unfilteredTotal"`
}

var opnameSort = map[string]string{
	"idObat":     "s.id_obat",
	"nama":       "o.nama",
	"hargaJual":  "s.hjual",
	"hargaBeli":  "s.hbeli",
	"beratIsi":   "o.isi",
	"jenis":      "jo.info",
	"satuanBeli": "sb.info",
	"satuanJual": "sj.info",
	"jumUnit":    "o.qjb",
	"stokMin":    "s.stok_min",
	"stok":       "s.stok",
	"cabang":     "b.nama",
}

// OpnameSortable reports whether key is a Stok Opname sort column.
func OpnameSortable(key string) bool {
	_, ok := opnameSort[key]
	return ok
}

const opnameFrom = ` FROM trxstock s
	JOIN refobat o ON o.id = s.id_obat
	LEFT JOIN refsatuan sj ON sj.id = o.sat_jual
	LEFT JOIN refsatuan sb ON sb.id = o.sat_beli
	LEFT JOIN refjenisobat jo ON jo.id = o.id_jenis
	LEFT JOIN refbranch b ON b.kode = s.cabang`

func opnameWhere(op OpnameParams, withSearch bool) (string, []any) {
	args := []any{op.Branches}
	conds := []string{`s.cabang = ANY($1)`}
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
	if op.BelowMin {
		conds = append(conds, `COALESCE(s.stok_min, 0) > COALESCE(s.stok, 0)`)
	}
	if op.Aktif != nil {
		if *op.Aktif {
			conds = append(conds, `s.isactive = 1`)
		} else {
			conds = append(conds, `s.isactive <> 1`)
		}
	}
	if withSearch && op.Search != "" {
		args = append(args, "%"+escapeLike(op.Search)+"%")
		n := len(args)
		cond := fmt.Sprintf(`o.nama ILIKE $%[1]d OR jo.info ILIKE $%[1]d OR sj.info ILIKE $%[1]d OR sb.info ILIKE $%[1]d OR b.nama ILIKE $%[1]d`, n)
		var id int
		if _, err := fmt.Sscanf(op.Search, "%d", &id); err == nil && fmt.Sprint(id) == op.Search {
			args = append(args, id)
			cond += fmt.Sprintf(` OR s.id_obat = $%d`, len(args))
		}
		conds = append(conds, `(`+cond+`)`)
	}
	return strings.Join(conds, " AND "), args
}

func (p *PostgresRepository) StokOpname(ctx context.Context, op OpnameParams) (OpnamePage, error) {
	page := OpnamePage{Rows: []OpnameRow{}}
	col, ok := opnameSort[op.Sort]
	if !ok {
		col = "s.id_obat"
	}
	dir := "ASC"
	if op.Desc {
		dir = "DESC"
	}
	cond, args := opnameWhere(op, true)
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`SELECT s.id_obat, COALESCE(o.nama, ''), COALESCE(s.hjual, 0), COALESCE(s.hbeli, 0),
			COALESCE(o.isi, ''), o.id_jenis, COALESCE(jo.info, ''), COALESCE(sb.info, ''), COALESCE(sj.info, ''), COALESCE(o.qjb, 0),
			COALESCE(s.stok_min, 0), COALESCE(s.stok, 0), s.cabang, COALESCE(b.nama, s.cabang), o.kena_pajak = 1, s.isactive = 1,
			count(*) OVER ()
		%s WHERE %s ORDER BY %s %s NULLS LAST, s.id_obat, b.nama LIMIT $%d OFFSET $%d`,
		opnameFrom, cond, col, dir, n+1, n+2), append(args, op.Limit, op.Offset)...)
	if err != nil {
		return page, err
	}
	defer rows.Close()
	for rows.Next() {
		var x OpnameRow
		if err := rows.Scan(&x.IDObat, &x.Nama, &x.HargaJual, &x.HargaBeli, &x.BeratIsi, &x.IDJenis, &x.Jenis, &x.SatuanBeli, &x.SatuanJual,
			&x.JumUnit, &x.StokMin, &x.Stok, &x.Cabang, &x.NamaCabang, &x.KenaPajak, &x.IsActive, &page.Total); err != nil {
			return page, err
		}
		page.Rows = append(page.Rows, x)
	}
	if err := rows.Err(); err != nil {
		return page, err
	}
	// An offset past the end returns no rows, and so no window count.
	if len(page.Rows) == 0 && op.Offset > 0 {
		if err := p.pool.QueryRow(ctx, `SELECT count(*)`+opnameFrom+` WHERE `+cond, args...).Scan(&page.Total); err != nil {
			return page, err
		}
	}
	if op.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = opnameWhere(op, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*)`+opnameFrom+` WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}

// ObatCabang is a row of Stok Trace's drug lookup: a drug stocked at the branch.
type ObatCabang struct {
	ID     int     `json:"id"`
	Nama   string  `json:"nama"`
	Jenis  string  `json:"jenis"`
	Satuan string  `json:"satuan"` // the selling unit, the unit of the stock
	Stok   float64 `json:"stok"`
}

// ObatCabang lists the drugs stocked at a branch (every trxstock row), by id.
func (p *PostgresRepository) ObatCabang(ctx context.Context, branch string) ([]ObatCabang, error) {
	rows, err := p.pool.Query(ctx, `SELECT s.id_obat, COALESCE(o.nama, ''), COALESCE(jo.info, ''), COALESCE(sj.info, ''), COALESCE(s.stok, 0)
		FROM trxstock s
		JOIN refobat o ON o.id = s.id_obat
		LEFT JOIN refjenisobat jo ON jo.id = o.id_jenis
		LEFT JOIN refsatuan sj ON sj.id = o.sat_jual
		WHERE s.cabang = $1
		ORDER BY s.id_obat`, branch)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	list := []ObatCabang{}
	for rows.Next() {
		var o ObatCabang
		if err := rows.Scan(&o.ID, &o.Nama, &o.Jenis, &o.Satuan, &o.Stok); err != nil {
			return nil, err
		}
		list = append(list, o)
	}
	return list, rows.Err()
}
