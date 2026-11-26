package analitik

import (
	"math"
	"sort"
)

// Predict forecasts every drug's usage and plans its stock.
func Predict(usages []Usage, horizon, leadTime int) []ObatPrediksi {
	out := make([]ObatPrediksi, 0, len(usages))
	for _, u := range usages {
		r := DailyRate(u.Series)
		used := 0.0
		for _, v := range u.Series {
			used += math.Max(0, v)
		}
		p := ObatPrediksi{
			IDObat: u.IDObat, Nama: u.Nama, Satuan: u.Satuan, Cabang: u.Cabang, NamaCabang: u.NamaCabang,
			Stok: u.Stok, StokMin: u.StokMin, AvgHarian: round2(r.AvgDaily), Terpakai: round2(used),
			Hari: r.Days, LowData: r.LowData, HargaPokok: u.HargaPokok,
			Plan: PlanStock(u.Stok, r, horizon, leadTime),
		}
		if u.HargaPokok != nil {
			cost := round2(p.SaranOrder * *u.HargaPokok)
			p.EstimasiBiaya = &cost
		}
		out = append(out, p)
	}
	SortPredictions(out)
	return out
}

// IcdTrend is one diagnosis with its weekly series and trend.
type IcdTrend struct {
	IDIcd int    `json:"idIcd"`
	Kode  string `json:"kode"`
	Nama  string `json:"nama"`
	Weeks []int  `json:"weeks"` // oldest first
	Trend
}

// Penyakit is the disease trend of a period.
type Penyakit struct {
	WeekStarts []string   `json:"weekStarts"` // first day (YYYY-MM-DD) of each week, oldest first
	Top        []IcdTrend `json:"top"`        // most visits over the period
	Lonjakan   []IcdTrend `json:"lonjakan"`   // spikes this week, any diagnosis, most extra cases first
	Kunjungan  []int      `json:"kunjungan"`  // diagnosed visits per week (a visit counts once per diagnosis)
}

// BuildPenyakit ranks the diagnoses and finds this week's spikes.
func BuildPenyakit(series []IcdSeries, today string, weeks, top int) Penyakit {
	p := Penyakit{WeekStarts: make([]string, weeks), Top: []IcdTrend{}, Lonjakan: []IcdTrend{}, Kunjungan: make([]int, weeks)}
	for i := range weeks {
		p.WeekStarts[i] = addDays(today, -((weeks-i)*7 - 1))
	}
	all := make([]IcdTrend, 0, len(series))
	for _, s := range series {
		t := IcdTrend{IDIcd: s.IDIcd, Kode: s.Kode, Nama: s.Nama, Weeks: s.Weeks, Trend: WeeklyTrend(s.Weeks)}
		all = append(all, t)
		for i, v := range s.Weeks {
			if i < weeks {
				p.Kunjungan[i] += v
			}
		}
	}
	sort.SliceStable(all, func(i, j int) bool {
		if all[i].Total != all[j].Total {
			return all[i].Total > all[j].Total
		}
		return all[i].Kode < all[j].Kode
	})
	p.Top = append(p.Top, all[:min(top, len(all))]...)
	for _, t := range all {
		if t.Lonjakan {
			p.Lonjakan = append(p.Lonjakan, t)
		}
	}
	excess := func(t IcdTrend) float64 { return float64(t.MingguIni) - t.RataSebelum }
	sort.SliceStable(p.Lonjakan, func(i, j int) bool { return excess(p.Lonjakan[i]) > excess(p.Lonjakan[j]) })
	return p
}

// SaranObat is a drug to prepare for the diagnoses expected over the horizon.
type SaranObat struct {
	IDObat    int      `json:"idObat"`
	Nama      string   `json:"nama"`
	Satuan    string   `json:"satuan"`
	Kebutuhan float64  `json:"kebutuhan"` // expected units over the horizon
	Stok      float64  `json:"stok"`      // stock at the selected branches
	Kurang    float64  `json:"kurang"`    // max(0, kebutuhan − stok), whole units
	Dari      []string `json:"dari"`      // the ICD codes the need comes from
}

// SaranStok turns the projected cases of each diagnosis into drug needs:
// projected weekly cases × horizon/7 × the units prescribed per case. A visit
// with several diagnoses adds to several, so the need is an upper estimate.
func SaranStok(resep []IcdObat, trends map[int]Trend, horizon int, stok map[int]float64) []SaranObat {
	byObat := map[int]*SaranObat{}
	for _, d := range resep {
		t, ok := trends[d.IDIcd]
		if !ok || t.ProyeksiMinggu <= 0 {
			continue
		}
		cases := t.ProyeksiMinggu * float64(horizon) / 7
		label := d.Kode
		if label == "" {
			label = d.Nama
		}
		for _, o := range d.Obat {
			if o.QtyPerKasus <= 0 {
				continue
			}
			s := byObat[o.IDObat]
			if s == nil {
				s = &SaranObat{IDObat: o.IDObat, Nama: o.Nama, Satuan: o.Satuan, Dari: []string{}}
				byObat[o.IDObat] = s
			}
			s.Kebutuhan += cases * o.QtyPerKasus
			s.Dari = append(s.Dari, label)
		}
	}
	out := make([]SaranObat, 0, len(byObat))
	for _, s := range byObat {
		s.Kebutuhan = round2(s.Kebutuhan)
		s.Stok = stok[s.IDObat]
		s.Kurang = math.Max(0, math.Ceil(s.Kebutuhan-s.Stok))
		out = append(out, *s)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].Kurang != out[j].Kurang {
			return out[i].Kurang > out[j].Kurang
		}
		if out[i].Kebutuhan != out[j].Kebutuhan {
			return out[i].Kebutuhan > out[j].Kebutuhan
		}
		return out[i].Nama < out[j].Nama
	})
	return out
}
