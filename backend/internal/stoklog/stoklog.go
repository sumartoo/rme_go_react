// Package stoklog moves a drug's stock at a branch (trxstock.stok) and writes
// the movement to trxobat, the log Stok Farmasi > Stok Trace reads, in the
// caller's transaction. Every change of trxstock.stok goes through Move so
// the trace stays complete.
package stoklog

import (
	"context"
	"errors"
	"math"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/example/be_jwt_golang/backend/internal/pendaftaran"
)

// table_ref values, as in the legacy data.
const (
	RefJual        = "TrxJualObat"        // id_ref = trxjualobat.id
	RefFaktur      = "TrxFakturObat"      // id_ref = trxfakturobat.id
	RefAdjus       = "TrxStokAdjus"       // id_ref = trxstokadjus.id
	RefAdjusCancel = "TrxStokAdjusCancel" // id_ref = trxstokadjus.id written by a sale's cancel
)

// Tx is what Move needs from a pgx transaction.
type Tx interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
}

// Entry is one movement: Delta > 0 adds to the stock (In), < 0 takes off (Out).
type Entry struct {
	IDObat   int
	Cabang   string
	Delta    float64
	TableRef string
	IDRef    int
	Remarks  string
	User     string
}

// Move applies e.Delta to the branch stock and logs it; it returns the stock
// after the move. A drug without a stock row at the branch is left alone
// (ok false, nothing logged), as the plain UPDATE it replaces did.
func Move(ctx context.Context, tx Tx, e Entry) (stock float64, ok bool, err error) {
	err = tx.QueryRow(ctx, `UPDATE trxstock SET stok = COALESCE(stok, 0) + $1 WHERE id_obat = $2 AND cabang = $3 RETURNING stok`,
		e.Delta, e.IDObat, e.Cabang).Scan(&stock)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, false, nil
	}
	if err != nil {
		return 0, false, err
	}
	inout := "In"
	if e.Delta < 0 {
		inout = "Out"
	}
	remarks := []rune(e.Remarks)
	if len(remarks) > 300 {
		remarks = remarks[:300]
	}
	user := []rune(e.User)
	if len(user) > 20 {
		user = user[:20]
	}
	_, err = tx.Exec(ctx, `INSERT INTO trxobat (id_obat, cabang, laststock, inout, byk, stock, systemstock, id_ref, table_ref, remarks,
			createdby, createddate)
		VALUES ($1, $2, $3, $4, $5, $6, $6, $7, $8, $9, $10, (now() AT TIME ZONE '`+pendaftaran.ClinicTimeZone+`'))`,
		e.IDObat, e.Cabang, stock-e.Delta, inout, math.Abs(e.Delta), stock, e.IDRef, e.TableRef, string(remarks), string(user))
	if err != nil {
		return 0, false, err
	}
	return stock, true, nil
}
