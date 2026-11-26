package penerimaan

import (
	"context"
	"errors"
	"fmt"
	"math"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/example/be_jwt_golang/backend/internal/httpx"
	"github.com/example/be_jwt_golang/backend/internal/pendaftaran"
	"github.com/example/be_jwt_golang/backend/internal/stoklog"
)

// Saving a goods receipt (the Add form's Save) follows the legacy data:
//   - trxfaktur: tgl = now, tgl_fak and tgl_jtp = Tanggal GR, id_supp 0,
//     id_jenis_byr 1 (TUNAI), the user as create_user and update_user;
//   - trxfakturobat: a line per drug with the branch's buying price
//     (trxstock.hbeli), tot_qjb = byk, batch, expiry date and discount (%);
//   - trxfakturtotal: total = Σ hbeli × byk × (1 − disc/100), fully paid
//     (cicilan = total, potongan 0, utang 0), status 1, no_close = YYMM + id;
//   - trxstock: the branch stock goes up by the quantity received.
// Prices come from the stock rows, never from the form.

// Distributor is a row of the "Data Distributor" lookup.
type Distributor struct {
	ID     int    `json:"id"`
	Nama   string `json:"nama"`
	Alamat string `json:"alamat"`
}

// Obat is a row of the "Data Obat" lookup: a drug stocked at the branch.
type Obat struct {
	ID     int     `json:"id"`
	Nama   string  `json:"nama"`
	Jenis  string  `json:"jenis"`
	Satuan string  `json:"satuan"` // the selling unit, the unit of the stock
	Harga  float64 `json:"harga"`  // trxstock.hbeli
	Stok   float64 `json:"stok"`
}

type SaveLine struct {
	IDObat  int     `json:"idObat"`
	NoBatch string  `json:"noBatch"`
	Byk     float64 `json:"byk"`
	TglExp  string  `json:"tglExp"` // YYYY-MM-DD
	Disc    float64 `json:"disc"`   // percent
}

type SaveInput struct {
	Cabang        string     `json:"cabang"`
	IDDistributor int        `json:"idDistributor"`
	NoGR          string     `json:"noGr"`
	TglGR         string     `json:"tglGr"` // YYYY-MM-DD
	Obat          []SaveLine `json:"obat"`
}

type SaveResult struct {
	ID      int     `json:"id"` // No Trans
	Total   float64 `json:"total"`
	NoClose *int    `json:"noClose"`
}

// MaxLines caps the drug lines of one receipt.
const MaxLines = 300

var (
	ErrUnknownDistributor = errors.New("unknown distributor")
)

// ItemError is a line the server cannot receive: the drug is not stocked at the branch.
type ItemError struct {
	IDObat int
	Msg    string
}

func (e *ItemError) Error() string { return fmt.Sprintf("obat %d: %s", e.IDObat, e.Msg) }

// Normalize trims the text and fills the defaults (No Batch "0").
func (in *SaveInput) Normalize() {
	in.Cabang = strings.ToUpper(strings.TrimSpace(in.Cabang))
	in.NoGR = strings.TrimSpace(in.NoGR)
	in.TglGR = strings.TrimSpace(in.TglGR)
	for i := range in.Obat {
		l := &in.Obat[i]
		l.NoBatch = strings.TrimSpace(l.NoBatch)
		if l.NoBatch == "" {
			l.NoBatch = "0"
		}
		l.TglExp = strings.TrimSpace(l.TglExp)
	}
}

// Validate checks the form; today is the clinic date (Tanggal GR may not be later).
func (in SaveInput) Validate(today string) httpx.FieldErrors {
	errs := httpx.FieldErrors{}
	if in.Cabang == "" {
		errs["cabang"] = "Pilih Cabang"
	}
	if in.IDDistributor <= 0 {
		errs["idDistributor"] = "Pilih Distributor"
	}
	switch n := utf8.RuneCountInString(in.NoGR); {
	case n == 0:
		errs["noGr"] = "Isi No GR"
	case n > 15:
		errs["noGr"] = "No GR maksimal 15 karakter"
	}
	if d, err := time.Parse("2006-01-02", in.TglGR); err != nil {
		errs["tglGr"] = "Tanggal GR tidak valid"
	} else if d.Format("2006-01-02") > today {
		errs["tglGr"] = "Tanggal GR tidak boleh setelah hari ini"
	}
	switch {
	case len(in.Obat) == 0:
		errs["obat"] = "Tambahkan minimal satu obat"
	case len(in.Obat) > MaxLines:
		errs["obat"] = fmt.Sprintf("Maksimal %d obat", MaxLines)
	}
	for i, l := range in.Obat {
		field := fmt.Sprintf("obat[%d]", i)
		switch {
		case l.IDObat <= 0:
			errs[field] = "Obat tidak valid"
		case !(l.Byk > 0) || math.IsInf(l.Byk, 0):
			errs[field] = "Banyak harus lebih dari 0"
		case l.Disc < 0 || l.Disc > 100 || math.IsNaN(l.Disc):
			errs[field] = "Disc % harus antara 0 dan 100"
		case utf8.RuneCountInString(l.NoBatch) > 20:
			errs[field] = "No Batch maksimal 20 karakter"
		}
		if _, err := time.Parse("2006-01-02", l.TglExp); err != nil && errs[field] == "" {
			errs[field] = "Exp Date tidak valid"
		}
	}
	return errs
}

func (p *PostgresRepository) Distributors(ctx context.Context) ([]Distributor, error) {
	rows, err := p.pool.Query(ctx, `SELECT id, COALESCE(trim(distributor), ''), COALESCE(trim(alamat), '') FROM refdistributorobat ORDER BY id`)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Distributor, error) {
		var d Distributor
		err := r.Scan(&d.ID, &d.Nama, &d.Alamat)
		return d, err
	})
	if list == nil {
		list = []Distributor{}
	}
	return list, err
}

// Obat lists the drugs stocked at a branch (every trxstock row), by id like
// the legacy lookup, with the buying price and stock.
func (p *PostgresRepository) Obat(ctx context.Context, branch string) ([]Obat, error) {
	rows, err := p.pool.Query(ctx, `SELECT s.id_obat, COALESCE(o.nama, ''), COALESCE(jo.info, ''), COALESCE(sj.info, ''), COALESCE(s.hbeli, 0),
			COALESCE(s.stok, 0)
		FROM trxstock s
		JOIN refobat o ON o.id = s.id_obat
		LEFT JOIN refjenisobat jo ON jo.id = o.id_jenis
		LEFT JOIN refsatuan sj ON sj.id = o.sat_jual
		WHERE s.cabang = $1
		ORDER BY s.id_obat`, branch)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Obat, error) {
		var o Obat
		err := r.Scan(&o.ID, &o.Nama, &o.Jenis, &o.Satuan, &o.Harga, &o.Stok)
		return o, err
	})
	if list == nil {
		list = []Obat{}
	}
	return list, err
}

// noClose is the legacy settlement number: YYMM of the entry date followed by
// the receipt id (2611 + 13809 = 261113809), or nil when it would not fit.
func noClose(tgl time.Time, id int) *int {
	n, err := strconv.ParseInt(tgl.Format("0601")+strconv.Itoa(id), 10, 32)
	if err != nil {
		return nil
	}
	v := int(n)
	return &v
}

// Save stores a receipt with its lines and total and adds the drugs to the
// branch stock, all in one transaction. user goes to create_user.
func (p *PostgresRepository) Save(ctx context.Context, in SaveInput, user string) (SaveResult, error) {
	var res SaveResult
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return res, err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	var ok bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM refdistributorobat WHERE id = $1)`, in.IDDistributor).Scan(&ok); err != nil {
		return res, err
	}
	if !ok {
		return res, ErrUnknownDistributor
	}

	// Price each line from the branch stock (locked until the stock update).
	prices := make([]float64, len(in.Obat))
	for i, l := range in.Obat {
		err := tx.QueryRow(ctx, `SELECT COALESCE(hbeli, 0) FROM trxstock WHERE id_obat = $1 AND cabang = $2 FOR UPDATE`,
			l.IDObat, in.Cabang).Scan(&prices[i])
		if errors.Is(err, pgx.ErrNoRows) {
			return res, &ItemError{IDObat: l.IDObat, Msg: "tidak ada di stok cabang " + in.Cabang}
		}
		if err != nil {
			return res, err
		}
		res.Total += prices[i] * l.Byk * (1 - l.Disc/100)
	}

	now := `(now() AT TIME ZONE '` + pendaftaran.ClinicTimeZone + `')`
	var tgl time.Time
	if err := tx.QueryRow(ctx, `INSERT INTO trxfaktur (tgl, tgl_jtp, no_fak, tgl_fak, id_supp, id_jenis_byr, create_user, create_date,
			update_user, update_date, cabang, id_distributor)
		VALUES (`+now+`, $1::date, $2, $1::date, 0, 1, $3, `+now+`, $3, `+now+`, $4, $5)
		RETURNING id, tgl`, in.TglGR, in.NoGR, user, in.Cabang, in.IDDistributor).Scan(&res.ID, &tgl); err != nil {
		return res, err
	}
	for i, l := range in.Obat {
		var idLine int
		if err := tx.QueryRow(ctx, `INSERT INTO trxfakturobat (id_fak, id_obat, no_batch, tgl_exp, disc, hbeli, byk, tot_qjb)
			VALUES ($1, $2, $3, $4::date, $5, $6, $7, $7) RETURNING id`, res.ID, l.IDObat, l.NoBatch, l.TglExp, l.Disc, prices[i], l.Byk).Scan(&idLine); err != nil {
			return res, err
		}
		if _, _, err := stoklog.Move(ctx, tx, stoklog.Entry{IDObat: l.IDObat, Cabang: in.Cabang, Delta: l.Byk,
			TableRef: stoklog.RefFaktur, IDRef: idLine, Remarks: "Good Receive " + in.Cabang, User: user}); err != nil {
			return res, err
		}
	}
	res.NoClose = noClose(tgl, res.ID)
	if _, err := tx.Exec(ctx, `INSERT INTO trxfakturtotal (id_fak, total, cicilan, potongan, utang, no_close, tgl_close, status)
		VALUES ($1, $2, $2, 0, 0, $3, $4::date, 1)`, res.ID, res.Total, res.NoClose, in.TglGR); err != nil {
		return res, err
	}
	return res, tx.Commit(ctx)
}
