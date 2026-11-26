package pengobatan

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/example/be_jwt_golang/backend/internal/pendaftaran"
	"github.com/example/be_jwt_golang/backend/internal/stoklog"
)

// Saving a treatment sale (the form's Save) follows the legacy data:
//   - trxjual: the sale, numbered per branch and month (no_kwi = YYMM + branch + 4 digits);
//   - trxjualtotal: totals and settlement (status 1 Close);
//   - trxjualobat / trxjualtindakan / trxjualpnjng / trxjualdiagnosa: its lines;
//   - trxstock: the branch stock goes down by the drugs sold;
//   - trxpendaftaran: the registration becomes CLOSE.
// Prices and totals are computed here from the master data; only an editable
// procedure price (reftindakan.isedit) comes from the form.

// VisitClose is the visit status (refstatustransaksi) of a billed registration.
const VisitClose = 2

// SaleStatusClose is trxjualtotal.status of a settled sale.
const SaleStatusClose = 1

var (
	ErrAlreadyBilled  = errors.New("this registration already has a treatment sale")
	ErrBranchMismatch = errors.New("the registration belongs to another branch")
)

// ItemError is a line the server cannot price or stock: unknown, not sold at
// the branch, or not enough stock.
type ItemError struct {
	Field string // obat, tindakan, penunjang, diagnosa
	ID    int
	Msg   string
}

func (e *ItemError) Error() string { return fmt.Sprintf("%s %d: %s", e.Field, e.ID, e.Msg) }

type SaveObat struct {
	IDObat int     `json:"idObat"`
	Byk    float64 `json:"byk"`
	Dosis  string  `json:"dosis"`
}

type SaveTindakan struct {
	IDTindakan int      `json:"idTindakan"`
	Byk        float64  `json:"byk"`
	Harga      *float64 `json:"harga"` // used only when the procedure's price is editable
}

type SavePenunjang struct {
	IDPenunjang int     `json:"idPenunjang"`
	Byk         float64 `json:"byk"`
}

// SaveInput is the form as sent by Save.
type SaveInput struct {
	Cabang       string          `json:"cabang"`
	TglKwi       string          `json:"tglKwi"` // YYYY-MM-DD, clinic date, today or earlier
	IDKun        int             `json:"idKun"`
	IDJenisRawat int             `json:"idJenisRawat"`
	IDJenisBayar int             `json:"idJenisBayar"`
	IDDokter     int             `json:"idDokter"`
	Obat         []SaveObat      `json:"obat"`
	Tindakan     []SaveTindakan  `json:"tindakan"`
	Penunjang    []SavePenunjang `json:"penunjang"`
	Diagnosa     []int           `json:"diagnosa"` // reficd ids
	Diskon       float64         `json:"diskon"`
	Payment      *float64        `json:"payment"` // money handed over; empty = exactly the amount due
}

// SaveResult is what the form shows after saving.
type SaveResult struct {
	ID         int     `json:"id"`
	NoKwi      string  `json:"noKwi"`
	TotObat    float64 `json:"totObat"`
	TotTndk    float64 `json:"totTindakan"`
	TotPnjng   float64 `json:"totPenunjang"`
	Total      float64 `json:"total"`
	Potongan   float64 `json:"potongan"`
	GrandTotal float64 `json:"grandTotal"`
	Pembayaran float64 `json:"pembayaran"`
	Cicilan    float64 `json:"cicilan"`
	Piutang    float64 `json:"piutang"`
	Kembalian  float64 `json:"kembalian"`
}

// roundUp500 rounds an amount up to a multiple of 500, ignoring floating-point dust.
func roundUp500(x float64) float64 {
	cents := math.Round(x*100) / 100
	return math.Ceil(cents/500) * 500
}

// obatLine prices one drug line: Jumlah = Harga x Banyak, PPN 10% when taxable,
// each rounded up to 500 ("di bulatkan").
func obatLine(harga, byk float64, taxable bool) (jumlah, ppn, billed float64) {
	jumlah = harga * byk
	if taxable {
		ppn = jumlah * 0.1
	}
	return jumlah, ppn, roundUp500(jumlah) + roundUp500(ppn)
}

// settle splits the amount due into what is paid now and what stays owed.
func settle(total, potongan float64, payment *float64) (grand, pembayaran, cicilan, piutang, kembalian float64) {
	grand = math.Max(total-potongan, 0)
	pembayaran = grand
	if payment != nil {
		pembayaran = *payment
	}
	cicilan = math.Min(pembayaran, grand)
	return grand, pembayaran, cicilan, grand - cicilan, math.Max(pembayaran-grand, 0)
}

// Validate checks the shape of the input; prices, stock and references are checked when saving.
func (in *SaveInput) Validate(today string) map[string]string {
	errs := map[string]string{}
	in.Cabang = strings.ToUpper(strings.TrimSpace(in.Cabang))
	if in.Cabang == "" {
		errs["cabang"] = "Choose a branch"
	}
	if !validDate(in.TglKwi) || in.TglKwi > today {
		errs["tglKwi"] = "Use a date up to today (YYYY-MM-DD)"
	}
	if in.IDKun <= 0 {
		errs["idKun"] = "Choose a registration (No. Daftar)"
	}
	if in.IDJenisRawat <= 0 {
		errs["idJenisRawat"] = "Choose the Pelayanan"
	}
	if in.IDJenisBayar <= 0 {
		errs["idJenisBayar"] = "Choose the Jenis Bayar"
	}
	if in.IDDokter <= 0 {
		errs["idDokter"] = "Choose the Dokter PJ"
	}
	if len(in.Obat)+len(in.Tindakan)+len(in.Penunjang) == 0 {
		errs["items"] = "Add at least one drug, procedure or examination"
	}
	seen := map[string]bool{}
	dup := func(kind string, id int) bool {
		k := fmt.Sprintf("%s/%d", kind, id)
		if seen[k] {
			return true
		}
		seen[k] = true
		return false
	}
	for i := range in.Obat {
		o := &in.Obat[i]
		o.Dosis = strings.TrimSpace(o.Dosis)
		switch {
		case o.IDObat <= 0 || o.Byk <= 0:
			errs["obat"] = "Each drug needs a quantity above 0"
		case len([]rune(o.Dosis)) > 100:
			errs["obat"] = "Dosis is at most 100 characters"
		case dup("obat", o.IDObat):
			errs["obat"] = "A drug appears twice"
		}
	}
	for _, t := range in.Tindakan {
		switch {
		case t.IDTindakan <= 0 || t.Byk <= 0:
			errs["tindakan"] = "Each procedure needs a quantity above 0"
		case t.Harga != nil && *t.Harga < 0:
			errs["tindakan"] = "A price cannot be negative"
		case dup("tindakan", t.IDTindakan):
			errs["tindakan"] = "A procedure appears twice"
		}
	}
	for _, p := range in.Penunjang {
		switch {
		case p.IDPenunjang <= 0 || p.Byk <= 0:
			errs["penunjang"] = "Each examination needs a quantity above 0"
		case dup("penunjang", p.IDPenunjang):
			errs["penunjang"] = "An examination appears twice"
		}
	}
	for _, d := range in.Diagnosa {
		if d <= 0 || dup("diagnosa", d) {
			errs["diagnosa"] = "Each diagnosis once"
		}
	}
	if in.Diskon < 0 {
		errs["diskon"] = "Cannot be negative"
	}
	if in.Payment != nil && *in.Payment < 0 {
		errs["payment"] = "Cannot be negative"
	}
	return errs
}

// Save records the sale in one transaction; see the package comment above.
func (p *PostgresRepository) Save(ctx context.Context, in SaveInput, user string) (SaveResult, error) {
	var res SaveResult
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return res, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	// The registration: locked, at this branch, billable and not billed yet.
	var regCabang string
	var status, idPas int
	err = tx.QueryRow(ctx, `SELECT COALESCE(cabang, ''), COALESCE(id_statustr, 0), COALESCE(id_pas, 0)
		FROM trxpendaftaran WHERE id = $1 FOR UPDATE`, in.IDKun).Scan(&regCabang, &status, &idPas)
	if errors.Is(err, pgx.ErrNoRows) {
		return res, ErrRegNotFound
	}
	if err != nil {
		return res, err
	}
	if regCabang != in.Cabang {
		return res, ErrBranchMismatch
	}
	if !examEditable(status) {
		return res, ErrRegNotEditable
	}
	var billed bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM trxjual WHERE id_kun = $1)`, in.IDKun).Scan(&billed); err != nil {
		return res, err
	}
	if billed {
		return res, ErrAlreadyBilled
	}

	// References.
	var ok bool
	checks := []struct {
		field, sql string
		args       []any
	}{
		{"idDokter", `SELECT EXISTS (SELECT 1 FROM refmdokterauth WHERE dokterid = $1 AND cabang = $2)`, []any{in.IDDokter, in.Cabang}},
		{"idJenisRawat", `SELECT EXISTS (SELECT 1 FROM refjenisrawat WHERE id = $1)`, []any{in.IDJenisRawat}},
		{"idJenisBayar", `SELECT EXISTS (SELECT 1 FROM refjenisbayar WHERE id = $1)`, []any{in.IDJenisBayar}},
	}
	for _, c := range checks {
		if err := tx.QueryRow(ctx, c.sql, c.args...).Scan(&ok); err != nil {
			return res, err
		}
		if !ok {
			return res, &ItemError{Field: c.field, ID: 0, Msg: "unknown or not at this branch"}
		}
	}
	var idJamkes *int
	var noJamkes *string
	if err := tx.QueryRow(ctx, `SELECT NULLIF(id_jamkes, 0), NULLIF(no_jamkes, '') FROM refmpasien WHERE id = $1`, idPas).Scan(&idJamkes, &noJamkes); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return res, err
	}

	// Drugs: branch price and stock, locked so two cashiers cannot sell the same stock twice.
	type obatRow struct {
		SaveObat
		harga, hp, ppn float64
	}
	obat := make([]obatRow, 0, len(in.Obat))
	for _, o := range in.Obat {
		var nama string
		var harga, hp, stok float64
		var taxable bool
		err := tx.QueryRow(ctx, `SELECT o.nama, COALESCE(s.hjual, 0), COALESCE(s.hp, 0), COALESCE(s.stok, 0), COALESCE(o.kena_pajak, 0) = 1
			FROM trxstock s JOIN refobat o ON o.id = s.id_obat
			WHERE s.id_obat = $1 AND s.cabang = $2 AND s.isactive = 1 FOR UPDATE OF s`, o.IDObat, in.Cabang).Scan(&nama, &harga, &hp, &stok, &taxable)
		if errors.Is(err, pgx.ErrNoRows) {
			return res, &ItemError{Field: "obat", ID: o.IDObat, Msg: "not sold at this branch"}
		}
		if err != nil {
			return res, err
		}
		if o.Byk > stok {
			return res, &ItemError{Field: "obat", ID: o.IDObat, Msg: fmt.Sprintf("%s: not enough stock (%g left, %g asked)", nama, stok, o.Byk)}
		}
		_, ppn, line := obatLine(harga, o.Byk, taxable)
		res.TotObat += line
		obat = append(obat, obatRow{SaveObat: o, harga: harga, hp: hp, ppn: ppn})
	}

	// Procedures of the Dokter PJ at the branch.
	type pricedRow struct {
		id    int
		byk   float64
		harga float64
	}
	tindakan := make([]pricedRow, 0, len(in.Tindakan))
	for _, t := range in.Tindakan {
		var harga float64
		var editable bool
		err := tx.QueryRow(ctx, `SELECT COALESCE(d.hjual, 0), COALESCE(r.isedit, 0) = 1
			FROM trxtindakan x JOIN reftindakan r ON r.id = x.id_tindakan
			JOIN reftindakandetail d ON d.id_tindakan = r.id AND d.cabang = $3
			WHERE x.id_dokter = $1 AND r.id = $2 AND COALESCE(r.isactive, 1) = 1 AND COALESCE(d.isactive, 1) = 1
			LIMIT 1`, in.IDDokter, t.IDTindakan, in.Cabang).Scan(&harga, &editable)
		if errors.Is(err, pgx.ErrNoRows) {
			return res, &ItemError{Field: "tindakan", ID: t.IDTindakan, Msg: "not a procedure of this doctor at this branch"}
		}
		if err != nil {
			return res, err
		}
		if editable && t.Harga != nil {
			harga = *t.Harga
		}
		res.TotTndk += harga * t.Byk
		tindakan = append(tindakan, pricedRow{t.IDTindakan, t.Byk, harga})
	}

	// Supporting examinations at the branch.
	penunjang := make([]pricedRow, 0, len(in.Penunjang))
	for _, x := range in.Penunjang {
		var harga float64
		err := tx.QueryRow(ctx, `SELECT COALESCE(d.hjual, 0) FROM refpenunjang r
			JOIN refpenunjangdetail d ON d.id_penunjang = r.id AND d.cabang = $2
			WHERE r.id = $1 AND COALESCE(r.isactive, 1) = 1 AND COALESCE(d.isactive, 1) = 1 LIMIT 1`, x.IDPenunjang, in.Cabang).Scan(&harga)
		if errors.Is(err, pgx.ErrNoRows) {
			return res, &ItemError{Field: "penunjang", ID: x.IDPenunjang, Msg: "not offered at this branch"}
		}
		if err != nil {
			return res, err
		}
		res.TotPnjng += harga * x.Byk
		penunjang = append(penunjang, pricedRow{x.IDPenunjang, x.Byk, harga})
	}

	for _, d := range in.Diagnosa {
		if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM reficd WHERE id = $1)`, d).Scan(&ok); err != nil {
			return res, err
		}
		if !ok {
			return res, &ItemError{Field: "diagnosa", ID: d, Msg: "unknown ICD-X code"}
		}
	}

	// Totals and settlement.
	res.Total = res.TotObat + res.TotTndk + res.TotPnjng
	if in.Diskon > res.Total {
		return res, &ItemError{Field: "diskon", Msg: "the discount is larger than the total"}
	}
	res.Potongan = in.Diskon
	res.GrandTotal, res.Pembayaran, res.Cicilan, res.Piutang, res.Kembalian = settle(res.Total, res.Potongan, in.Payment)

	// Receipt number: YYMM + branch + running number per branch and month.
	yymm := in.TglKwi[2:4] + in.TglKwi[5:7]
	prefix := yymm + in.Cabang
	if len(prefix)+4 > 11 {
		return res, fmt.Errorf("branch code %q is too long for a receipt number", in.Cabang)
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtext('trxjual.no_kwi:' || $1))`, prefix); err != nil {
		return res, err
	}
	var last int
	if err := tx.QueryRow(ctx, `SELECT COALESCE(max(substr(no_kwi, $2)::int), 0) FROM trxjual
		WHERE no_kwi LIKE $1 || '%' AND length(no_kwi) = $3 AND substr(no_kwi, $2) ~ '^[0-9]+$'`,
		prefix, len(prefix)+1, len(prefix)+4).Scan(&last); err != nil {
		return res, err
	}
	if last >= 9999 {
		return res, fmt.Errorf("receipt numbers for %s are used up", prefix)
	}
	res.NoKwi = fmt.Sprintf("%s%04d", prefix, last+1)

	// The sale and its totals.
	now := `(now() AT TIME ZONE '` + pendaftaran.ClinicTimeZone + `')`
	err = tx.QueryRow(ctx, `INSERT INTO trxjual (tgl, no_kwi, tgl_kwi, id_pas, id_jenis_byr, id_kun, id_jenis_rwt, id_dok,
			create_user, create_date, id_jamkes, no_jamkes, cabang, flow_status)
		VALUES ($1::date + `+now+`::time, $2, $1::date, $3, $4, $5, $6, $7, $8, `+now+`, $9, $10, $11, 'Close')
		RETURNING id`,
		in.TglKwi, res.NoKwi, idPas, in.IDJenisBayar, in.IDKun, in.IDJenisRawat, in.IDDokter, user, idJamkes, noJamkes, in.Cabang).Scan(&res.ID)
	if err != nil {
		return res, err
	}
	// no_close: the 2-digit year followed by the sale id (26521594), as in lab.
	noClose, err := strconv.Atoi(in.TglKwi[2:4] + strconv.Itoa(res.ID))
	if err != nil {
		return res, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO trxjualtotal (id_jual, tot_obat, tot_tndk, tot_pnjng, total, cicilan, potongan, piutang,
			no_close, tgl_close, status, point, pembayaran)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10::date, $11, 0, $12)`,
		res.ID, res.TotObat, res.TotTndk, res.TotPnjng, res.Total, res.Cicilan, res.Potongan, res.Piutang,
		noClose, in.TglKwi, SaleStatusClose, res.Pembayaran); err != nil {
		return res, err
	}

	// Lines, and the stock they use.
	for _, o := range obat {
		var idLine int
		if err := tx.QueryRow(ctx, `INSERT INTO trxjualobat (id_jual, id_obat, hjual, byk, hp, dosis, ppn) VALUES ($1, $2, $3, $4, $5, NULLIF($6, ''), $7)
			RETURNING id`, res.ID, o.IDObat, o.harga, o.Byk, o.hp, o.Dosis, o.ppn).Scan(&idLine); err != nil {
			return res, err
		}
		if _, _, err := stoklog.Move(ctx, tx, stoklog.Entry{IDObat: o.IDObat, Cabang: in.Cabang, Delta: -o.Byk,
			TableRef: stoklog.RefJual, IDRef: idLine, Remarks: "Transaksi " + res.NoKwi, User: user}); err != nil {
			return res, err
		}
	}
	for _, t := range tindakan {
		if _, err := tx.Exec(ctx, `INSERT INTO trxjualtindakan (id_jual, id_tndk, hjual, byk, id_dok) VALUES ($1, $2, $3, $4, $5)`,
			res.ID, t.id, t.harga, t.byk, in.IDDokter); err != nil {
			return res, err
		}
	}
	for _, x := range penunjang {
		if _, err := tx.Exec(ctx, `INSERT INTO trxjualpnjng (id_jual, id_pnjng, hjual, byk) VALUES ($1, $2, $3, $4)`,
			res.ID, x.id, x.harga, x.byk); err != nil {
			return res, err
		}
	}
	for _, d := range in.Diagnosa {
		if _, err := tx.Exec(ctx, `INSERT INTO trxjualdiagnosa (id_jual, icd_id) VALUES ($1, $2)`, res.ID, d); err != nil {
			return res, err
		}
	}

	// The registration is billed.
	if _, err := tx.Exec(ctx, `UPDATE trxpendaftaran SET id_statustr = $2, update_user = $3, update_date = `+now+` WHERE id = $1`,
		in.IDKun, VisitClose, user); err != nil {
		return res, err
	}
	return res, tx.Commit(ctx)
}
