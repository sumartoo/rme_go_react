package pengobatan

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/jackc/pgx/v5"

	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
	"github.com/example/be_jwt_golang/backend/internal/pendaftaran"
)

// PerawatPath is Transaksi Klinik > Pemeriksaan Perawat (legacy /TransPerawat),
// seeded by 008_transaksi_klinik_menu.sql: the nurse fills in the examination
// of a poli registration and chooses its Dokter PJ.
const PerawatPath = "/transaksi-klinik/pemeriksaan-perawat"

// PerawatRow is a poli registration in the Pemeriksaan Perawat list.
type PerawatRow struct {
	RegRow
	NoAntrianPoli string `json:"noAntrianPoli"`
	KeluhanUtama  string `json:"keluhanUtama"`
}

// PerawatParams selects one page of the Pemeriksaan Perawat list.
type PerawatParams struct {
	Branches []string // the caller's branches, or the one chosen
	From, To string   // YYYY-MM-DD, inclusive, clinic dates
	IDStatus *int
	IDDokter *int // Dokter PJ (trxpemeriksaan.id_dok)
	Search   string
	Sort     string
	Desc     bool
	Limit    int
	Offset   int
}

type PerawatPage struct {
	Rows            []PerawatRow `json:"rows"`
	Total           int          `json:"total"`
	UnfilteredTotal int          `json:"unfilteredTotal"`
}

var perawatSortColumns = map[string]string{
	"noAntrianPoli": "d.no_antri_poli",
	"tglMasuk":      "d.tgl_masuk",
	"pelayanan":     "r.info",
	"dokter":        "dk.nama",
	"noRm":          "p.no_rm",
	"nama":          "p.nama",
	"umur":          "p.tgl_lahir",
	"alamat":        "p.alamat",
	"keluhanUtama":  "pm.keluhan_utama",
	"cabang":        "d.cabang",
	"status":        "s.info",
}

const perawatColumns = regColumns + `,
	COALESCE(NULLIF(trim(d.no_antri_poli_str), ''), d.no_antri_poli::text, ''), COALESCE(pm.keluhan_utama, '')`

func scanPerawat(r pgx.CollectableRow) (PerawatRow, error) {
	var x PerawatRow
	err := r.Scan(&x.ID, &x.TglMasuk, &x.IDJenisRawat, &x.Pelayanan, &x.IsPemeriksaan, &x.IDStatus, &x.Status, &x.IDPas,
		&x.NoRM, &x.Nama, &x.TglLahir, &x.Alamat, &x.HP, &x.Penanggung, &x.IDJamkes, &x.Jamkes, &x.NoJamkes, &x.Cabang,
		&x.IDDokter, &x.Dokter, &x.NoAntrianPoli, &x.KeluhanUtama)
	return x, err
}

// perawatWhere builds the list filter: poli registrations (a doctor's
// examination) only. Without withSearch it leaves out the search box.
func perawatWhere(pp PerawatParams, withSearch bool) (string, []any) {
	args := []any{pp.Branches, pp.From, pp.To}
	conds := []string{
		`d.cabang = ANY($1)`, `d.tgl_masuk >= $2::date`, `d.tgl_masuk < $3::date + 1`, `COALESCE(r.ispemeriksaan, false)`,
	}
	if pp.IDStatus != nil {
		args = append(args, *pp.IDStatus)
		conds = append(conds, fmt.Sprintf(`d.id_statustr = $%d`, len(args)))
	}
	if pp.IDDokter != nil {
		args = append(args, *pp.IDDokter)
		conds = append(conds, fmt.Sprintf(`pm.id_dok = $%d`, len(args)))
	}
	if withSearch && pp.Search != "" {
		args = append(args, "%"+escapeLike(pp.Search)+"%")
		n := len(args)
		conds = append(conds, fmt.Sprintf(`(p.nama ILIKE $%[1]d OR p.no_rm ILIKE $%[1]d OR p.alamat ILIKE $%[1]d OR r.info ILIKE $%[1]d
			OR dk.nama ILIKE $%[1]d OR pm.keluhan_utama ILIKE $%[1]d OR s.info ILIKE $%[1]d OR d.id::text = $%[2]d)`, n, n+1))
		args = append(args, pp.Search)
	}
	return strings.Join(conds, " AND "), args
}

// PerawatList pages the poli registrations at the given branches, newest first by default.
func (p *PostgresRepository) PerawatList(ctx context.Context, pp PerawatParams) (PerawatPage, error) {
	page := PerawatPage{Rows: []PerawatRow{}}
	col, ok := perawatSortColumns[pp.Sort]
	if !ok {
		col = "d.tgl_masuk"
	}
	desc := pp.Desc
	if pp.Sort == "umur" {
		desc = !desc // older birth date = higher age
	}
	dir := "ASC"
	if desc {
		dir = "DESC"
	}
	cond, args := perawatWhere(pp, true)
	n := len(args)
	rows, err := p.pool.Query(ctx, fmt.Sprintf(`%s%s WHERE %s ORDER BY %s %s NULLS LAST, d.id %s LIMIT $%d OFFSET $%d`,
		perawatColumns, regFrom, cond, col, dir, dir, n+1, n+2), append(args, pp.Limit, pp.Offset)...)
	if err != nil {
		return page, err
	}
	if page.Rows, err = pgx.CollectRows(rows, scanPerawat); err != nil {
		return page, err
	}
	if page.Rows == nil {
		page.Rows = []PerawatRow{}
	}
	if err := p.pool.QueryRow(ctx, `SELECT count(*)`+regFrom+` WHERE `+cond, args...).Scan(&page.Total); err != nil {
		return page, err
	}
	if pp.Search == "" {
		page.UnfilteredTotal = page.Total
		return page, nil
	}
	cond, args = perawatWhere(pp, false)
	err = p.pool.QueryRow(ctx, `SELECT count(*)`+regFrom+` WHERE `+cond, args...).Scan(&page.UnfilteredTotal)
	return page, err
}

// RegByID returns one registration with its patient and Dokter PJ.
func (p *PostgresRepository) RegByID(ctx context.Context, idKun int) (RegRow, error) {
	rows, err := p.pool.Query(ctx, regColumns+regFrom+` WHERE d.id = $1`, idKun)
	if err != nil {
		return RegRow{}, err
	}
	r, err := pgx.CollectExactlyOneRow(rows, scanReg)
	if errors.Is(err, pgx.ErrNoRows) {
		return RegRow{}, ErrRegNotFound
	}
	return r, err
}

// PerawatOptions returns the caller's branches, the visit statuses and today's clinic date.
func (h *Handler) PerawatOptions(w http.ResponseWriter, r *http.Request) {
	statuses, err := h.repo.StatusKunjungan(r.Context())
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"status": statuses, "branches": branches, "today": pendaftaran.ClinicToday(h.now()),
	})
}

// PerawatList pages the poli registrations at the caller's branches.
//
// Query: from, to (YYYY-MM-DD, default yesterday and today in the clinic's time
// zone), cabang, status, dokter (Dokter PJ), q, sort, dir, page, size (1..5000).
func (h *Handler) PerawatList(w http.ResponseWriter, r *http.Request) {
	branches, err := h.branches(r)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	q := r.URL.Query()
	codes := branchauth.Codes(branches)
	if c := strings.ToUpper(strings.TrimSpace(q.Get("cabang"))); c != "" {
		if !branchauth.Has(branches, c) {
			httpx.WriteError(w, http.StatusForbidden, "you are not assigned to branch "+c)
			return
		}
		codes = []string{c}
	}
	now := h.now()
	from, to := q.Get("from"), q.Get("to")
	if to == "" {
		to = pendaftaran.ClinicToday(now)
	}
	if from == "" {
		from = pendaftaran.ClinicToday(now.AddDate(0, 0, -1))
	}
	if !validDate(from) || !validDate(to) || to < from {
		httpx.WriteValidation(w, httpx.FieldErrors{"from": "Use a valid date range (YYYY-MM-DD, from ≤ to)"})
		return
	}
	sort := q.Get("sort")
	desc := q.Get("dir") != "asc"
	if _, ok := perawatSortColumns[sort]; !ok {
		sort, desc = "tglMasuk", true
	}
	size := intParam(r, "size", 10, 1, MaxPageSize)
	page := intParam(r, "page", 1, 1, 1_000_000)
	res, err := h.repo.PerawatList(r.Context(), PerawatParams{
		Branches: codes, From: from, To: to, IDStatus: optInt(r, "status"), IDDokter: optInt(r, "dokter"), Search: strings.TrimSpace(q.Get("q")),
		Sort: sort, Desc: desc, Limit: size, Offset: (page - 1) * size,
	})
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"rows": res.Rows, "total": res.Total, "unfilteredTotal": res.UnfilteredTotal, "from": from, "to": to,
	})
}

// PerawatReg returns a registration at the caller's branches with its patient
// and Dokter PJ, and whether its examination can still change:
// {"pendaftaran": {...}, "editable": bool}.
func (h *Handler) PerawatReg(w http.ResponseWriter, r *http.Request) {
	h.regWith(w, r, examEditable)
}

// regWith answers a registration and whether editable allows changing it.
func (h *Handler) regWith(w http.ResponseWriter, r *http.Request, editable func(status int) bool) {
	idKun, status, ok := h.regOfCaller(w, r)
	if !ok {
		return
	}
	reg, err := h.repo.RegByID(r.Context(), idKun)
	if errors.Is(err, ErrRegNotFound) {
		httpx.WriteError(w, http.StatusNotFound, "pendaftaran not found")
		return
	}
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"pendaftaran": reg, "editable": editable(status)})
}
