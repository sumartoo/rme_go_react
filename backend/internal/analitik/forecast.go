// Package analitik serves Stok Farmasi > Analitik: statistical forecasts of
// drug usage per branch, weekly disease (ICD) trends with spike detection, the
// drugs usually prescribed per diagnosis, and an optional AI assistant (DeepSeek)
// that explains those numbers. The numbers are computed here, in Go and SQL;
// the assistant only reads them, as aggregates without patient data.
//
// The methods are deliberately simple and explainable: an exponentially
// weighted daily rate over the last 90 days, a safety stock from its spread,
// and week-over-week comparisons. No seasonality is modelled.
package analitik

import (
	"math"
	"sort"
)

const (
	// Alpha weighs recent days in the daily usage rate (0.1: the last ~10
	// days count most, older days still matter).
	Alpha = 0.1
	// WeeklyAlpha weighs recent weeks in the weekly case projection.
	WeeklyAlpha = 0.4
	// ServiceZ is the z-score of the safety stock (1.65 ≈ 95% of lead times covered).
	ServiceZ = 1.65
	// MinDays is the history below which a forecast is flagged lowData.
	MinDays = 30
)

// Rate is the usage rate of one drug at one branch, from its daily series.
type Rate struct {
	AvgDaily float64 // exponentially weighted units per day
	StdDev   float64 // spread of the daily usage
	Days     int     // days of history used
	LowData  bool    // fewer than MinDays days
}

// DailyRate weighs a daily usage series, oldest first. Negative days (more
// cancelled than sold) count as zero.
func DailyRate(series []float64) Rate {
	n := len(series)
	r := Rate{Days: n, LowData: n < MinDays}
	if n == 0 {
		return r
	}
	clean := make([]float64, n)
	for i, v := range series {
		clean[i] = math.Max(0, v)
	}
	// Start the level at the mean of the first week, then smooth.
	k := min(7, n)
	level := mean(clean[:k])
	for _, v := range clean[k:] {
		level = Alpha*v + (1-Alpha)*level
	}
	r.AvgDaily = level
	r.StdDev = stddev(clean)
	return r
}

// Status of a drug's stock against its forecast.
const (
	StatusHabis  = "habis"          // no stock left, still used
	StatusKritis = "kritis"         // runs out before a new order arrives
	StatusOrder  = "perlu-order"    // will not last the horizon
	StatusAman   = "aman"           // lasts the horizon
	StatusDiam   = "tidak-bergerak" // not used in the history
)

// statusRank orders the statuses from most to least urgent.
var statusRank = map[string]int{StatusHabis: 0, StatusKritis: 1, StatusOrder: 2, StatusAman: 3, StatusDiam: 4}

// Plan is what a rate means for the stock at hand.
type Plan struct {
	Kebutuhan      float64  `json:"kebutuhan"`      // forecast usage over the horizon
	SafetyStock    float64  `json:"safetyStock"`    // buffer for the lead time
	ROP            float64  `json:"rop"`            // reorder point: order when the stock falls to it
	SaranOrder     float64  `json:"saranOrder"`     // units to order now (whole units)
	HabisDalamHari *float64 `json:"habisDalamHari"` // days until the stock runs out; null when unused
	Status         string   `json:"status"`
}

// PlanStock turns a rate into a recommendation for a stock over horizon days,
// with orders arriving after leadTime days.
func PlanStock(stok float64, r Rate, horizon, leadTime int) Plan {
	p := Plan{
		Kebutuhan:   round2(r.AvgDaily * float64(horizon)),
		SafetyStock: round2(ServiceZ * r.StdDev * math.Sqrt(float64(leadTime))),
	}
	// ROP = daily usage × lead time + safety stock.
	p.ROP = round2(r.AvgDaily*float64(leadTime) + p.SafetyStock)
	if r.AvgDaily <= 0 {
		p.Status = StatusDiam
		return p
	}
	days := math.Max(0, stok) / r.AvgDaily
	days = round2(days)
	p.HabisDalamHari = &days
	p.SaranOrder = math.Max(0, math.Ceil(p.Kebutuhan+p.SafetyStock-math.Max(0, stok)))
	switch {
	case stok <= 0:
		p.Status = StatusHabis
	case days < float64(leadTime):
		p.Status = StatusKritis
	case p.SaranOrder > 0:
		p.Status = StatusOrder
	default:
		p.Status = StatusAman
	}
	return p
}

// SortPredictions puts the most urgent drugs first: by status, then by the
// days until they run out, then the most used (e.g. among drugs out of stock),
// then by name.
func SortPredictions(rows []ObatPrediksi) {
	sort.SliceStable(rows, func(i, j int) bool {
		a, b := rows[i], rows[j]
		if ra, rb := statusRank[a.Status], statusRank[b.Status]; ra != rb {
			return ra < rb
		}
		da, db := math.Inf(1), math.Inf(1)
		if a.HabisDalamHari != nil {
			da = *a.HabisDalamHari
		}
		if b.HabisDalamHari != nil {
			db = *b.HabisDalamHari
		}
		if da != db {
			return da < db
		}
		if a.AvgHarian != b.AvgHarian {
			return a.AvgHarian > b.AvgHarian
		}
		if a.Nama != b.Nama {
			return a.Nama < b.Nama
		}
		return a.Cabang < b.Cabang
	})
}

// Trend summarises a weekly case series, oldest week first.
type Trend struct {
	Total          int     `json:"total"`
	MingguIni      int     `json:"mingguIni"`      // the last 7 days
	RataSebelum    float64 `json:"rataSebelum"`    // mean of the 4 weeks before
	Rasio          float64 `json:"rasio"`          // mingguIni / rataSebelum (0 when there is no baseline)
	Lonjakan       bool    `json:"lonjakan"`       // a spike, see IsSpike
	ProyeksiMinggu float64 `json:"proyeksiMinggu"` // expected cases next week
}

// WeeklyTrend computes the totals, the spike flag and next week's projection.
func WeeklyTrend(weeks []int) Trend {
	var t Trend
	n := len(weeks)
	if n == 0 {
		return t
	}
	for _, v := range weeks {
		t.Total += v
	}
	t.MingguIni = weeks[n-1]
	if n > 1 {
		prev := weeks[max(0, n-5) : n-1]
		sum := 0
		for _, v := range prev {
			sum += v
		}
		t.RataSebelum = round2(float64(sum) / float64(len(prev)))
	}
	if t.RataSebelum > 0 {
		t.Rasio = round2(float64(t.MingguIni) / t.RataSebelum)
	}
	t.Lonjakan = IsSpike(t.MingguIni, t.RataSebelum)
	level := float64(weeks[0])
	for _, v := range weeks[1:] {
		level = WeeklyAlpha*float64(v) + (1-WeeklyAlpha)*level
	}
	t.ProyeksiMinggu = round2(level)
	return t
}

// IsSpike reports a week well above its baseline: at least 5 cases, at least
// 1.5 times the mean of the weeks before, and at least 3 cases more.
func IsSpike(week int, baseline float64) bool {
	w := float64(week)
	return week >= 5 && w >= 1.5*baseline && w-baseline >= 3
}

func mean(v []float64) float64 {
	if len(v) == 0 {
		return 0
	}
	s := 0.0
	for _, x := range v {
		s += x
	}
	return s / float64(len(v))
}

func stddev(v []float64) float64 {
	if len(v) < 2 {
		return 0
	}
	m := mean(v)
	s := 0.0
	for _, x := range v {
		s += (x - m) * (x - m)
	}
	return math.Sqrt(s / float64(len(v)-1))
}

func round2(v float64) float64 { return math.Round(v*100) / 100 }
