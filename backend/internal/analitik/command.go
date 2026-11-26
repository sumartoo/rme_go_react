package analitik

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/example/be_jwt_golang/backend/internal/httpx"
)

// MaxPeriodeHari caps the period of the business queries.
const MaxPeriodeHari = 366

// KPI are the headline numbers of the Command Center.
type KPI struct {
	ObatHabis        int      `json:"obatHabis"`
	ObatKritis       int      `json:"obatKritis"`
	ObatPerluOrder   int      `json:"obatPerluOrder"`
	BiayaOrder       float64  `json:"biayaOrder"`      // Rp, drugs with a plausible cost price only
	OrderTanpaHarga  int      `json:"orderTanpaHarga"` // drugs to order whose cost is unknown
	Lonjakan         int      `json:"lonjakan"`        // diagnoses spiking this week
	Transaksi7       int      `json:"transaksi7"`      // sales in the last 7 days
	Transaksi7Lalu   int      `json:"transaksi7Lalu"`  // the 7 days before
	TransaksiDelta   *float64 `json:"transaksiDelta"`  // percent
	Pendapatan30     float64  `json:"pendapatan30"`    // Rp, last 30 days
	Pendapatan30Lalu float64  `json:"pendapatan30Lalu"`
	PendapatanDelta  *float64 `json:"pendapatanDelta"` // percent
	TidakBergerak    int      `json:"tidakBergerak"`
}

// Insight is one thing worth attention, from fixed rules over the numbers.
type Insight struct {
	Level  string `json:"level"` // "kritis", "perhatian" or "info"
	Judul  string `json:"judul"`
	Detail string `json:"detail"`
}

// CommandResult is the Command Center of the caller's branches.
type CommandResult struct {
	KPI      KPI          `json:"kpi"`
	Insights []Insight    `json:"insights"`
	Mingguan []KinerjaRow `json:"mingguan"` // sales per week, last 12 weeks
	Today    string       `json:"today"`
}

func rupiah(v float64) string {
	s := fmt.Sprintf("%.0f", v)
	var b strings.Builder
	for i, c := range s {
		if i > 0 && (len(s)-i)%3 == 0 {
			b.WriteByte('.')
		}
		b.WriteRune(c)
	}
	return "Rp" + b.String()
}

func plural(n int, what string) string { return fmt.Sprintf("%d %s", n, what) }

// BuildInsights turns the numbers into a short list, most serious first.
func BuildInsights(k KPI, obat []ObatPrediksi, sakit Penyakit) []Insight {
	out := []Insight{}
	if k.ObatHabis > 0 {
		var laris []string
		for _, p := range obat {
			if p.Status == StatusHabis && p.AvgHarian >= 1 && len(laris) < 3 {
				laris = append(laris, fmt.Sprintf("%s (%s, %.1f/hari)", p.Nama, p.Cabang, p.AvgHarian))
			}
		}
		d := "Stok 0 padahal masih dipakai dalam 90 hari terakhir."
		if len(laris) > 0 {
			d += " Yang paling laku: " + strings.Join(laris, "; ") + "."
		}
		out = append(out, Insight{Level: "kritis", Judul: plural(k.ObatHabis, "obat habis"), Detail: d})
	}
	if k.ObatKritis > 0 {
		out = append(out, Insight{Level: "kritis", Judul: plural(k.ObatKritis, "obat kritis"),
			Detail: "Diperkirakan habis sebelum pesanan baru datang (lead time 7 hari)."})
	}
	for _, t := range sakit.Lonjakan[:min(3, len(sakit.Lonjakan))] {
		name := t.Kode
		if t.Nama != "" && t.Nama != t.Kode {
			name += " " + t.Nama
		}
		out = append(out, Insight{Level: "perhatian", Judul: fmt.Sprintf("Lonjakan %s", name),
			Detail: fmt.Sprintf("%d kasus dalam 7 hari terakhir, %.1f× rata-rata 4 minggu sebelumnya (%.1f).", t.MingguIni, t.Rasio, t.RataSebelum)})
	}
	if k.TransaksiDelta != nil && (*k.TransaksiDelta <= -15 || *k.TransaksiDelta >= 15) {
		arah := "naik"
		if *k.TransaksiDelta < 0 {
			arah = "turun"
		}
		out = append(out, Insight{Level: "perhatian", Judul: fmt.Sprintf("Transaksi %s %.0f%%", arah, abs(*k.TransaksiDelta)),
			Detail: fmt.Sprintf("%d transaksi dalam 7 hari terakhir dibanding %d pada 7 hari sebelumnya. Bila turun tajam, periksa juga apakah semua transaksi sudah tercatat.", k.Transaksi7, k.Transaksi7Lalu)})
	}
	if k.ObatPerluOrder+k.ObatHabis+k.ObatKritis > 0 {
		d := fmt.Sprintf("Perkiraan biaya order untuk 30 hari ke depan: %s.", rupiah(k.BiayaOrder))
		if k.OrderTanpaHarga > 0 {
			d += fmt.Sprintf(" %d obat belum dihitung karena harga pokoknya kosong atau tidak wajar.", k.OrderTanpaHarga)
		}
		out = append(out, Insight{Level: "info", Judul: plural(k.ObatHabis+k.ObatKritis+k.ObatPerluOrder, "obat perlu dipesan"), Detail: d})
	}
	if k.TidakBergerak > 0 {
		out = append(out, Insight{Level: "info", Judul: plural(k.TidakBergerak, "item stok tidak bergerak"),
			Detail: "Tidak terjual dalam 90 hari terakhir. Pertimbangkan evaluasi agar modal tidak tertahan."})
	}
	return out
}

func abs(v float64) float64 {
	if v < 0 {
		return -v
	}
	return v
}

func (h *Handler) command(ctx context.Context, codes []string) (CommandResult, error) {
	today := h.today()
	res := CommandResult{Insights: []Insight{}, Mingguan: []KinerjaRow{}, Today: today}
	if len(codes) == 0 {
		return res, nil
	}
	obat, err := h.prediksi(ctx, codes, DefaultHorizon, DefaultLeadTime, "", "", 0)
	if err != nil {
		return res, err
	}
	k := KPI{
		ObatHabis: obat.Ringkas[StatusHabis], ObatKritis: obat.Ringkas[StatusKritis], ObatPerluOrder: obat.Ringkas[StatusOrder],
		BiayaOrder: obat.BiayaOrder, OrderTanpaHarga: obat.OrderTanpaHarga, TidakBergerak: obat.Ringkas[StatusDiam],
	}
	sakit, err := h.penyakit(ctx, codes, DefaultWeeks, DefaultTop)
	if err != nil {
		return res, err
	}
	k.Lonjakan = len(sakit.Lonjakan)
	harian, err := h.repo.Kinerja(ctx, KinerjaParams{Branches: codes, From: addDays(today, -59), To: today, Kelompok: "tanggal"})
	if err != nil {
		return res, err
	}
	for _, r := range harian {
		switch age := daysBetween(r.Kelompok, today); {
		case age < 7:
			k.Transaksi7 += r.Transaksi
		case age < 14:
			k.Transaksi7Lalu += r.Transaksi
		}
		if age := daysBetween(r.Kelompok, today); age < 30 {
			k.Pendapatan30 += r.Pendapatan
		} else {
			k.Pendapatan30Lalu += r.Pendapatan
		}
	}
	k.Pendapatan30, k.Pendapatan30Lalu = round2(k.Pendapatan30), round2(k.Pendapatan30Lalu)
	k.TransaksiDelta = PersenBerubah(float64(k.Transaksi7Lalu), float64(k.Transaksi7))
	k.PendapatanDelta = PersenBerubah(k.Pendapatan30Lalu, k.Pendapatan30)
	res.KPI = k
	res.Insights = BuildInsights(k, obat.Rows, sakit)
	if res.Mingguan, err = h.repo.Kinerja(ctx, KinerjaParams{Branches: codes, From: mondayWeeksAgo(today, 11), To: today, Kelompok: "minggu"}); err != nil {
		return res, err
	}
	return res, nil
}

// mondayWeeksAgo is the Monday of the week n weeks before today's week.
func mondayWeeksAgo(today string, n int) string {
	d, _ := time.Parse("2006-01-02", today)
	back := (int(d.Weekday()) + 6) % 7 // days since Monday
	return d.AddDate(0, 0, -back-7*n).Format("2006-01-02")
}

// Command returns the Command Center: KPIs, insights and weekly sales.
//
// Query: cabang.
func (h *Handler) Command(w http.ResponseWriter, r *http.Request) {
	codes, _, ok := h.scope(w, r, r.URL.Query().Get("cabang"))
	if !ok {
		return
	}
	res, err := h.command(r.Context(), codes)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// periode validates a business period (default the last `def` days).
func (h *Handler) periode(dari, sampai string, def int) (string, string, string) {
	today := h.today()
	if sampai == "" {
		sampai = today
	}
	if dari == "" {
		dari = addDays(sampai, -(def - 1))
	}
	if !validDate(dari) || !validDate(sampai) || sampai < dari {
		return "", "", "Periode tidak valid (YYYY-MM-DD, dari ≤ sampai)"
	}
	if daysBetween(dari, sampai) >= MaxPeriodeHari {
		return "", "", fmt.Sprintf("Periode maksimal %d hari", MaxPeriodeHari)
	}
	return dari, sampai, ""
}

func validDate(s string) bool {
	_, err := time.Parse("2006-01-02", s)
	return err == nil
}

// KinerjaResult is the sales of a period, grouped.
type KinerjaResult struct {
	Rows     []KinerjaRow `json:"rows"`
	Total    KinerjaRow   `json:"total"`
	Dari     string       `json:"dari"`
	Sampai   string       `json:"sampai"`
	Kelompok string       `json:"kelompok"`
}

func (h *Handler) kinerja(ctx context.Context, codes []string, dari, sampai, kelompok string) (KinerjaResult, error) {
	res := KinerjaResult{Rows: []KinerjaRow{}, Dari: dari, Sampai: sampai, Kelompok: kelompok}
	if len(codes) == 0 {
		res.Total = SumKinerja(nil)
		return res, nil
	}
	rows, err := h.repo.Kinerja(ctx, KinerjaParams{Branches: codes, From: dari, To: sampai, Kelompok: kelompok})
	if err != nil {
		return res, err
	}
	res.Rows, res.Total = rows, SumKinerja(rows)
	return res, nil
}

// Kinerja returns the clinic's sales grouped by time, doctor, payer, service
// or branch. Cancelled sales are left out.
//
// Query: cabang, dari, sampai (YYYY-MM-DD, default the last 84 days, at most
// 366 days), kelompok (tanggal, minggu, bulan, hari, dokter, penjamin,
// jenis-rawat, cabang; default minggu).
func (h *Handler) Kinerja(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	codes, _, ok := h.scope(w, r, q.Get("cabang"))
	if !ok {
		return
	}
	kelompok := q.Get("kelompok")
	if kelompok == "" {
		kelompok = "minggu"
	}
	if !ValidKelompok(kelompok) {
		httpx.WriteValidation(w, httpx.FieldErrors{"kelompok": "Kelompok tidak dikenal"})
		return
	}
	dari, sampai, msg := h.periode(q.Get("dari"), q.Get("sampai"), 84)
	if msg != "" {
		httpx.WriteValidation(w, httpx.FieldErrors{"dari": msg})
		return
	}
	res, err := h.kinerja(r.Context(), codes, dari, sampai, kelompok)
	if err != nil {
		httpx.InternalError(w, err)
		return
	}
	httpx.WriteJSON(w, http.StatusOK, res)
}

// KualitasData runs the data-quality checks on the caller's branches.
//
// Query: cabang.
func (h *Handler) KualitasData(w http.ResponseWriter, r *http.Request) {
	codes, _, ok := h.scope(w, r, r.URL.Query().Get("cabang"))
	if !ok {
		return
	}
	checks := []CekKualitas{}
	if len(codes) > 0 {
		var err error
		if checks, err = h.repo.Kualitas(r.Context(), codes, h.today()); err != nil {
			httpx.InternalError(w, err)
			return
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"checks": checks, "maxContoh": MaxContoh})
}

// FollowUpHandler lists chronic patients who have not been back.
//
// Query: cabang, hari (14..180, default 35), kelompok (hipertensi, diabetes,
// dislipidemia, asma; default all).
func (h *Handler) FollowUpHandler(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	codes, _, ok := h.scope(w, r, q.Get("cabang"))
	if !ok {
		return
	}
	kelompok := strings.TrimSpace(q.Get("kelompok"))
	valid := kelompok == ""
	for _, g := range Kronis {
		valid = valid || g.Kode == kelompok
	}
	if !valid {
		httpx.WriteValidation(w, httpx.FieldErrors{"kelompok": "Kelompok tidak dikenal"})
		return
	}
	hari := intParam(r, "hari", 35, 14, 180)
	page := FollowUpPage{Rows: []FollowUpRow{}}
	if len(codes) > 0 {
		var err error
		page, err = h.repo.FollowUp(r.Context(), FollowUpParams{Branches: codes, Today: h.today(), Hari: hari, Kelompok: kelompok, Limit: 2000})
		if err != nil {
			httpx.InternalError(w, err)
			return
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"rows": page.Rows, "total": page.Total, "hari": hari, "kelompok": Kronis})
}
