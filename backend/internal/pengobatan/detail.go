package pengobatan

import (
	"context"
	"errors"
	"math"

	"github.com/jackc/pgx/v5"

	"github.com/example/be_jwt_golang/backend/internal/pendaftaran"
	"github.com/example/be_jwt_golang/backend/internal/stoklog"
)

// DetailObat is a drug line of a saved sale, as the form's Obat tab shows it.
type DetailObat struct {
	IDObat    int     `json:"idObat"`
	Nama      string  `json:"nama"`
	Jenis     string  `json:"jenis"`
	Satuan    string  `json:"satuan"`
	Harga     float64 `json:"harga"`
	Byk       float64 `json:"byk"`
	KenaPajak bool    `json:"kenaPajak"`
	PPN       float64 `json:"ppn"`
	Dosis     string  `json:"dosis"`
}

// DetailTindakan is a procedure or supporting-examination line of a saved sale.
type DetailTindakan struct {
	ID     int     `json:"id"`
	Nama   string  `json:"nama"`
	Jenis  string  `json:"jenis"`
	Dokter string  `json:"dokter,omitempty"`
	Harga  float64 `json:"harga"`
	Byk    float64 `json:"byk"`
	IsEdit bool    `json:"isEdit"`
}

// DetailICD is a diagnosis of a saved sale.
type DetailICD struct {
	ID    int    `json:"id"`
	Kode  string `json:"kode"`
	Nama  string `json:"nama"`
	Jenis string `json:"jenis"`
}

// Detail is a saved sale as the Edit page (PengobatanDetail/Edit) shows it.
type Detail struct {
	ID           int     `json:"id"`
	NoKwi        string  `json:"noKwi"`
	TglKwi       *string `json:"tglKwi"` // YYYY-MM-DD
	Cabang       string  `json:"cabang"`
	IDJenisRawat *int    `json:"idJenisRawat"`
	IDJenisBayar *int    `json:"idJenisBayar"`
	IDDokter     *int    `json:"idDokter"`
	Dokter       string  `json:"dokter"`
	Status       string  `json:"status"` // open, close, cancel
	CancelBy     string  `json:"cancelBy"`
	CancelAt     *string `json:"cancelAt"`
	CancelReason string  `json:"cancelReason"`
	// Reg is the registration with its patient; nil when it no longer exists.
	Reg          *RegRow          `json:"reg"`
	Obat         []DetailObat     `json:"obat"`
	Tindakan     []DetailTindakan `json:"tindakan"`
	Penunjang    []DetailTindakan `json:"penunjang"`
	Diagnosa     []DetailICD      `json:"diagnosa"`
	TotObat      float64          `json:"totObat"`
	TotTindakan  float64          `json:"totTindakan"`
	TotPenunjang float64          `json:"totPenunjang"`
	Total        float64          `json:"total"`
	Potongan     float64          `json:"potongan"`
	GrandTotal   float64          `json:"grandTotal"`
	Pembayaran   float64          `json:"pembayaran"`
	Piutang      float64          `json:"piutang"`
	Kembalian    float64          `json:"kembalian"`
}

// Detail loads a saved sale with its registration, patient and lines.
func (p *PostgresRepository) Detail(ctx context.Context, id int) (Detail, error) {
	d := Detail{Obat: []DetailObat{}, Tindakan: []DetailTindakan{}, Penunjang: []DetailTindakan{}, Diagnosa: []DetailICD{}}
	var idKun *int
	err := p.pool.QueryRow(ctx, `SELECT j.id, COALESCE(j.no_kwi, ''), to_char(j.tgl_kwi, 'YYYY-MM-DD'), COALESCE(j.cabang, ''),
			j.id_jenis_rwt, j.id_jenis_byr, j.id_dok, COALESCE(trim(dk.nama), ''), j.id_kun, `+statusSQL+`,
			COALESCE(t.cancelby, ''), to_char(t.cancelat, 'YYYY-MM-DD HH24:MI:SS'), COALESCE(t.cancelreason, ''),
			COALESCE(t.tot_obat, 0), COALESCE(t.tot_tndk, 0), COALESCE(t.tot_pnjng, 0), COALESCE(t.total, 0),
			COALESCE(t.potongan, 0), COALESCE(t.pembayaran, 0), COALESCE(t.piutang, 0)
		FROM trxjual j
		LEFT JOIN trxjualtotal t ON t.id_jual = j.id
		LEFT JOIN refmdokter dk ON dk.id = j.id_dok
		WHERE j.id = $1`, id).Scan(&d.ID, &d.NoKwi, &d.TglKwi, &d.Cabang, &d.IDJenisRawat, &d.IDJenisBayar, &d.IDDokter, &d.Dokter,
		&idKun, &d.Status, &d.CancelBy, &d.CancelAt, &d.CancelReason, &d.TotObat, &d.TotTindakan, &d.TotPenunjang, &d.Total,
		&d.Potongan, &d.Pembayaran, &d.Piutang)
	if errors.Is(err, pgx.ErrNoRows) {
		return d, ErrSaleNotFound
	}
	if err != nil {
		return d, err
	}
	d.GrandTotal = math.Max(d.Total-d.Potongan, 0)
	d.Kembalian = math.Max(d.Pembayaran-d.GrandTotal, 0)

	if idKun != nil {
		rows, err := p.pool.Query(ctx, regColumns+regFrom+` WHERE d.id = $1 LIMIT 1`, *idKun)
		if err != nil {
			return d, err
		}
		regs, err := pgx.CollectRows(rows, scanReg)
		if err != nil {
			return d, err
		}
		if len(regs) == 1 {
			d.Reg = &regs[0]
		}
	}

	rows, err := p.pool.Query(ctx, `SELECT x.id_obat, COALESCE(o.nama, '(obat '||x.id_obat||')'), COALESCE(jo.info, ''), COALESCE(s.info, ''),
			COALESCE(x.hjual, 0), COALESCE(x.byk, 0), COALESCE(o.kena_pajak, 0) = 1, COALESCE(x.ppn, 0), COALESCE(x.dosis, '')
		FROM trxjualobat x
		LEFT JOIN refobat o ON o.id = x.id_obat
		LEFT JOIN refjenisobat jo ON jo.id = o.id_jenis
		LEFT JOIN refsatuan s ON s.id = o.sat_jual
		WHERE x.id_jual = $1 ORDER BY x.id`, id)
	if err != nil {
		return d, err
	}
	if d.Obat, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (DetailObat, error) {
		var o DetailObat
		err := r.Scan(&o.IDObat, &o.Nama, &o.Jenis, &o.Satuan, &o.Harga, &o.Byk, &o.KenaPajak, &o.PPN, &o.Dosis)
		return o, err
	}); err != nil {
		return d, err
	}

	scanLine := func(r pgx.CollectableRow) (DetailTindakan, error) {
		var t DetailTindakan
		err := r.Scan(&t.ID, &t.Nama, &t.Jenis, &t.Dokter, &t.Harga, &t.Byk, &t.IsEdit)
		return t, err
	}
	if rows, err = p.pool.Query(ctx, `SELECT x.id_tndk, COALESCE(r.nama, '(tindakan '||x.id_tndk||')'), COALESCE(jt.info, ''),
			COALESCE(trim(dk.nama), ''), COALESCE(x.hjual, 0), COALESCE(x.byk, 0), COALESCE(r.isedit, 0) = 1
		FROM trxjualtindakan x
		LEFT JOIN reftindakan r ON r.id = x.id_tndk
		LEFT JOIN refjenistindakan jt ON jt.id = r.id_jenis
		LEFT JOIN refmdokter dk ON dk.id = x.id_dok
		WHERE x.id_jual = $1 ORDER BY x.id`, id); err != nil {
		return d, err
	}
	if d.Tindakan, err = pgx.CollectRows(rows, scanLine); err != nil {
		return d, err
	}
	if rows, err = p.pool.Query(ctx, `SELECT x.id_pnjng, COALESCE(r.nama, '(penunjang '||x.id_pnjng||')'), COALESCE(jp.info, ''),
			'', COALESCE(x.hjual, 0), COALESCE(x.byk, 0), false
		FROM trxjualpnjng x
		LEFT JOIN refpenunjang r ON r.id = x.id_pnjng
		LEFT JOIN refjenispanjang jp ON jp.id = r.id_jenis
		WHERE x.id_jual = $1 ORDER BY x.id`, id); err != nil {
		return d, err
	}
	if d.Penunjang, err = pgx.CollectRows(rows, scanLine); err != nil {
		return d, err
	}
	if rows, err = p.pool.Query(ctx, `SELECT x.icd_id, COALESCE(i.kode_icd, ''), COALESCE(i.nama, '(ICD '||x.icd_id||')'), COALESCE(ji.info, '')
		FROM trxjualdiagnosa x
		LEFT JOIN reficd i ON i.id = x.icd_id
		LEFT JOIN refjenisicd ji ON ji.id = i.id_jenis_icd
		WHERE x.id_jual = $1 ORDER BY x.id`, id); err != nil {
		return d, err
	}
	if d.Diagnosa, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (DetailICD, error) {
		var x DetailICD
		err := r.Scan(&x.ID, &x.Kode, &x.Nama, &x.Jenis)
		return x, err
	}); err != nil {
		return d, err
	}
	// CollectRows returns nil for no rows; the API always sends lists.
	if d.Obat == nil {
		d.Obat = []DetailObat{}
	}
	if d.Tindakan == nil {
		d.Tindakan = []DetailTindakan{}
	}
	if d.Penunjang == nil {
		d.Penunjang = []DetailTindakan{}
	}
	if d.Diagnosa == nil {
		d.Diagnosa = []DetailICD{}
	}
	return d, nil
}

// SaleStatusCancel is trxjualtotal.status of a cancelled sale.
const SaleStatusCancel = 2

// ErrAlreadyCancelled is returned when cancelling a sale that is already cancelled.
var ErrAlreadyCancelled = errors.New("this sale is already cancelled")

// Cancel cancels a sale as the legacy "Cancel Pengobatan" does: trxjualtotal
// gets status 2 with who, when and why; amounts and lines are kept, and the
// registration stays CLOSE. The drugs sold go back into the branch stock,
// undoing what Save took out.
func (p *PostgresRepository) Cancel(ctx context.Context, id int, reason, user string) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	var cabang, noKwi string
	var status *int
	err = tx.QueryRow(ctx, `SELECT COALESCE(j.cabang, ''), COALESCE(j.no_kwi, ''), t.status FROM trxjual j
		LEFT JOIN trxjualtotal t ON t.id_jual = j.id WHERE j.id = $1 FOR UPDATE OF j`, id).Scan(&cabang, &noKwi, &status)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrSaleNotFound
	}
	if err != nil {
		return err
	}
	if status != nil && *status == SaleStatusCancel {
		return ErrAlreadyCancelled
	}
	now := `(now() AT TIME ZONE '` + pendaftaran.ClinicTimeZone + `')`
	if _, err := tx.Exec(ctx, `INSERT INTO trxjualtotal (id_jual, status, cancelby, cancelat, cancelreason)
		VALUES ($1, $2, $3, `+now+`, $4)
		ON CONFLICT (id_jual) DO UPDATE SET status = EXCLUDED.status, cancelby = EXCLUDED.cancelby,
			cancelat = EXCLUDED.cancelat, cancelreason = EXCLUDED.cancelreason`,
		id, SaleStatusCancel, user, reason); err != nil {
		return err
	}
	rows, err := tx.Query(ctx, `SELECT id_obat, sum(COALESCE(byk, 0)) FROM trxjualobat WHERE id_jual = $1 GROUP BY id_obat ORDER BY id_obat`, id)
	if err != nil {
		return err
	}
	type back struct {
		idObat int
		byk    float64
	}
	backs, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (back, error) {
		var b back
		err := r.Scan(&b.idObat, &b.byk)
		return b, err
	})
	if err != nil {
		return err
	}
	ket := "Cancel Transaksi " + noKwi
	if k := []rune(ket); len(k) > 60 { // trxstokadjus.ket is varchar(60)
		ket = string(k[:60])
	}
	for _, b := range backs {
		if b.byk == 0 {
			continue
		}
		var idAdjus int
		if err := tx.QueryRow(ctx, `INSERT INTO trxstokadjus (tanggal, id_obat, ket, byk, status, create_user, create_date, update_user, update_date, cabang)
			VALUES (`+now+`::date, $1, $2, $3, 1, $4, `+now+`, $4, `+now+`, $5) RETURNING id`,
			b.idObat, ket, b.byk, user, cabang).Scan(&idAdjus); err != nil {
			return err
		}
		if _, _, err := stoklog.Move(ctx, tx, stoklog.Entry{IDObat: b.idObat, Cabang: cabang, Delta: b.byk,
			TableRef: stoklog.RefAdjusCancel, IDRef: idAdjus, Remarks: ket, User: user}); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}
