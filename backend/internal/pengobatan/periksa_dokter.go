package pengobatan

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strconv"
	"strings"
	"unicode/utf8"

	"github.com/jackc/pgx/v5"

	"github.com/example/be_jwt_golang/backend/internal/branchauth"
	"github.com/example/be_jwt_golang/backend/internal/httpx"
	"github.com/example/be_jwt_golang/backend/internal/pendaftaran"
)

// DokterPath is Transaksi Klinik > Pemeriksaan Dokter (legacy /TransDokter),
// seeded by 008_transaksi_klinik_menu.sql: the doctor updates the nurse's and
// doctor's examination and records the prescription and diagnoses of a poli
// registration.
const DokterPath = "/transaksi-klinik/pemeriksaan-dokter"

// ProsesObatPath is Transaksi Klinik > Proses Obat (legacy /ProsesObat): the
// poli registrations waiting for their drugs, each opened read-only with its
// examination, prescription and diagnoses.
const ProsesObatPath = "/transaksi-klinik/proses-obat"

// MaxResepLines caps the drugs and the diagnoses of one registration.
const MaxResepLines = 50

var (
	ErrUnknownObat = errors.New("unknown obat")
	ErrUnknownICD  = errors.New("unknown icd")
)

// DokterStatuses are the visit statuses (refstatustransaksi) the doctor can
// still update the examination of: DAFTAR, PROSES, PAYMENT and also CLOSE, so a
// doctor can complete the notes after the patient has paid, like the legacy
// screen. A cancelled visit cannot change.
var DokterStatuses = []int{2, 3, 4, 5}

func dokterEditable(status int) bool { return slices.Contains(DokterStatuses, status) }

// ResepObat is a drug the doctor prescribed for a registration (trxjualobatdokter).
type ResepObat struct {
	IDObat int     `json:"idObat"`
	Nama   string  `json:"nama"`   // "" when the drug no longer exists
	Satuan string  `json:"satuan"` // sale unit
	Byk    float64 `json:"byk"`
	Dosis  string  `json:"dosis"`
}

// ResepDiagnosa is a diagnosis the doctor recorded (trxjualdiagnosadokter).
type ResepDiagnosa struct {
	ICDID int    `json:"icdId"`
	Kode  string `json:"kode"`
	Nama  string `json:"nama"`
}

// Resep is the doctor's prescription and diagnoses of one registration.
type Resep struct {
	Obat     []ResepObat     `json:"obat"`
	Diagnosa []ResepDiagnosa `json:"diagnosa"`
}

// ResepInput replaces a registration's prescription and diagnoses.
type ResepInput struct {
	Obat []struct {
		IDObat int     `json:"idObat"`
		Byk    float64 `json:"byk"`
		Dosis  string  `json:"dosis"`
	} `json:"obat"`
	Diagnosa []int `json:"diagnosa"` // reficd ids
}

// Validate trims the doses and checks the lines.
func (in *ResepInput) Validate() httpx.FieldErrors {
	errs := httpx.FieldErrors{}
	if len(in.Obat) > MaxResepLines {
		errs["obat"] = "At most " + strconv.Itoa(MaxResepLines) + " drugs"
	}
	for i := range in.Obat {
		o := &in.Obat[i]
		o.Dosis = strings.TrimSpace(o.Dosis)
		field := "obat[" + strconv.Itoa(i) + "]"
		switch {
		case o.IDObat <= 0:
			errs[field] = "Choose a drug"
		case !(o.Byk > 0) || o.Byk > 10000:
			errs[field] = "Banyak must be between 0 and 10000"
		case utf8.RuneCountInString(o.Dosis) > 100:
			errs[field] = "Dosis is at most 100 characters"
		}
	}
	if len(in.Diagnosa) > MaxResepLines {
		errs["diagnosa"] = "At most " + strconv.Itoa(MaxResepLines) + " diagnoses"
	}
	seen := map[int]bool{}
	for _, id := range in.Diagnosa {
		if id <= 0 {
			errs["diagnosa"] = "Choose a diagnosis"
		} else if seen[id] {
			errs["diagnosa"] = "Each diagnosis only once"
		}
		seen[id] = true
	}
	return errs
}

// ProsesObatReg is PerawatReg for Proses Obat, which only views: editable is always false.
func (h *Handler) ProsesObatReg(w http.ResponseWriter, r *http.Request) {
	h.regWith(w, r, func(int) bool { return false })
}

// GetResep returns the doctor's prescription and diagnoses of a registration, in entry order.
func (p *PostgresRepository) GetResep(ctx context.Context, idKun int) (Resep, error) {
	res := Resep{Obat: []ResepObat{}, Diagnosa: []ResepDiagnosa{}}
	rows, err := p.pool.Query(ctx,
		`SELECT r.id_obat, COALESCE(o.nama, ''), COALESCE(s.info, ''), COALESCE(r.byk, 0), COALESCE(r.dosis, '')
		 FROM trxjualobatdokter r
		 LEFT JOIN refobat o ON o.id = r.id_obat
		 LEFT JOIN refsatuan s ON s.id = o.sat_jual
		 WHERE r.id_kun = $1 ORDER BY r.id`, idKun)
	if err != nil {
		return res, err
	}
	if res.Obat, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (ResepObat, error) {
		var x ResepObat
		err := r.Scan(&x.IDObat, &x.Nama, &x.Satuan, &x.Byk, &x.Dosis)
		return x, err
	}); err != nil {
		return res, err
	}
	rows, err = p.pool.Query(ctx,
		`SELECT r.icd_id, COALESCE(i.kode_icd, ''), COALESCE(i.nama, '')
		 FROM trxjualdiagnosadokter r LEFT JOIN reficd i ON i.id = r.icd_id
		 WHERE r.id_kun = $1 ORDER BY r.id`, idKun)
	if err != nil {
		return res, err
	}
	if res.Diagnosa, err = pgx.CollectRows(rows, func(r pgx.CollectableRow) (ResepDiagnosa, error) {
		var x ResepDiagnosa
		err := r.Scan(&x.ICDID, &x.Kode, &x.Nama)
		return x, err
	}); err != nil {
		return res, err
	}
	if res.Obat == nil {
		res.Obat = []ResepObat{}
	}
	if res.Diagnosa == nil {
		res.Diagnosa = []ResepDiagnosa{}
	}
	return res, nil
}

// SaveResep replaces the registration's prescription and diagnoses in one transaction.
func (p *PostgresRepository) SaveResep(ctx context.Context, idKun int, in ResepInput) error {
	tx, err := p.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx) //nolint:errcheck // no-op after Commit

	obatIDs := make([]int, 0, len(in.Obat))
	for _, o := range in.Obat {
		obatIDs = append(obatIDs, o.IDObat)
	}
	var missing int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM unnest($1::int[]) AS x(id) WHERE NOT EXISTS (SELECT 1 FROM refobat o WHERE o.id = x.id)`, obatIDs).Scan(&missing); err != nil {
		return err
	}
	if missing > 0 {
		return ErrUnknownObat
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM unnest($1::int[]) AS x(id) WHERE NOT EXISTS (SELECT 1 FROM reficd i WHERE i.id = x.id)`, in.Diagnosa).Scan(&missing); err != nil {
		return err
	}
	if missing > 0 {
		return ErrUnknownICD
	}
	if _, err := tx.Exec(ctx, `DELETE FROM trxjualobatdokter WHERE id_kun = $1`, idKun); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM trxjualdiagnosadokter WHERE id_kun = $1`, idKun); err != nil {
		return err
	}
	for _, o := range in.Obat {
		if _, err := tx.Exec(ctx, `INSERT INTO trxjualobatdokter (id_kun, id_obat, byk, dosis) VALUES ($1, $2, $3, $4)`,
			idKun, o.IDObat, o.Byk, o.Dosis); err != nil {
			return err
		}
	}
	for _, id := range in.Diagnosa {
		if _, err := tx.Exec(ctx, `INSERT INTO trxjualdiagnosadokter (id_kun, icd_id) VALUES ($1, $2)`, idKun, id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

// DokterAtBranches lists the active doctors mapped to any of the branches, for
// the list's Dokter filter.
func (p *PostgresRepository) DokterAtBranches(ctx context.Context, branches []string) ([]Dokter, error) {
	rows, err := p.pool.Query(ctx,
		`SELECT d.id, trim(COALESCE(d.nama, '')), COALESCE(j.info, '')
		 FROM refmdokter d
		 LEFT JOIN refjenisdokter j ON j.id = d.id_jenisdok
		 WHERE COALESCE(d.status, 1) = 1
		   AND EXISTS (SELECT 1 FROM refmdokterauth a WHERE a.dokterid = d.id AND a.cabang = ANY($1))
		 ORDER BY trim(COALESCE(d.nama, '')), d.id`, branches)
	if err != nil {
		return nil, err
	}
	list, err := pgx.CollectRows(rows, func(r pgx.CollectableRow) (Dokter, error) {
		var d Dokter
		err := r.Scan(&d.ID, &d.Nama, &d.Jenis)
		return d, err
	})
	if list == nil {
		list = []Dokter{}
	}
	return list, err
}

// DokterOptions returns the caller's branches, their doctors, the visit statuses and today's clinic date.
func (h *Handler) DokterOptions(w http.ResponseWriter, r *http.Request) {
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
	dokter, err := h.repo.DokterAtBranches(r.Context(), branchauth.Codes(branches))
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{
		"status": statuses, "branches": branches, "dokter": dokter, "today": pendaftaran.ClinicToday(h.now()),
	})
}

// DokterReg is PerawatReg with the doctor's rule: a closed visit can still change.
func (h *Handler) DokterReg(w http.ResponseWriter, r *http.Request) {
	h.regWith(w, r, dokterEditable)
}

// GetResep returns the doctor's prescription and diagnoses of a registration at the caller's branches.
func (h *Handler) GetResep(w http.ResponseWriter, r *http.Request) {
	idKun, _, ok := h.regOfCaller(w, r)
	if !ok {
		return
	}
	res, err := h.repo.GetResep(r.Context(), idKun)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// SaveResep replaces the doctor's prescription and diagnoses of a registration
// at the caller's branches. A cancelled visit answers 409.
func (h *Handler) SaveResep(w http.ResponseWriter, r *http.Request) {
	idKun, status, ok := h.regOfCaller(w, r)
	if !ok {
		return
	}
	var in ResepInput
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	if errs := in.Validate(); len(errs) > 0 {
		httpx.WriteValidation(w, errs)
		return
	}
	if !dokterEditable(status) {
		httpx.WriteError(w, http.StatusConflict, "the visit is cancelled")
		return
	}
	err := h.repo.SaveResep(r.Context(), idKun, in)
	switch {
	case errors.Is(err, ErrUnknownObat):
		httpx.WriteValidation(w, httpx.FieldErrors{"obat": "Unknown drug"})
		return
	case errors.Is(err, ErrUnknownICD):
		httpx.WriteValidation(w, httpx.FieldErrors{"diagnosa": "Unknown diagnosis"})
		return
	case err != nil:
		httpx.InternalError(w, err)
		return
	}
	res, err := h.repo.GetResep(r.Context(), idKun)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}
