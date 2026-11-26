// Package mcuhasil serves the 16 MCU result forms (MCU > Pemeriksaan Fisik,
// Riwayat Medis, Pemeriksaan Lab, ..., USG) on the MCU registrations
// (trxmcuheader). Every form stores its answers the same way (mcu_hasil_form +
// mcu_hasil_item); the catalogue forms (fisik, riwayat-medis, khusus-wanita,
// rontgen, ekg, usg) get their sections and items from the legacy examination
// catalogue (mcu_group > mcu_type > mcu_items > resulted_type), the others are
// laid out by the frontend. Each form is guarded by its own menu item.
package mcuhasil

// Form is one of the result forms.
type Form struct {
	Slug     string // URL segment and mcu_hasil_form.form
	Title    string
	PagePath string   // the menu item (004_mcu_menu.sql) that grants it
	Groups   []string // catalogue groups (mcu_group.group_type) laid out by the backend
}

// Forms are the 16 result forms, in menu order.
var Forms = []Form{
	{Slug: "fisik", Title: "Pemeriksaan Fisik", PagePath: "/mcu/pemeriksaan-fisik", Groups: []string{"G"}},
	{Slug: "riwayat-medis", Title: "Riwayat Medis", PagePath: "/mcu/riwayat-medis", Groups: []string{"B"}},
	{Slug: "lab", Title: "Pemeriksaan Lab", PagePath: "/mcu/pemeriksaan-lab"},
	{Slug: "audiometri", Title: "Pemeriksaan Audiometri", PagePath: "/mcu/pemeriksaan-audiometri"},
	{Slug: "riwayat-keluarga", Title: "Riwayat Kesehatan Keluarga", PagePath: "/mcu/riwayat-kesehatan-keluarga"},
	{Slug: "khusus-wanita", Title: "Riwayat Khusus Wanita", PagePath: "/mcu/riwayat-khusus-wanita", Groups: []string{"C", "D"}},
	{Slug: "opname-operasi", Title: "Riwayat Opname & Operasi", PagePath: "/mcu/riwayat-opname-operasi"},
	{Slug: "kebiasaan", Title: "Kebiasaan Pasien", PagePath: "/mcu/kebiasaan-pasien"},
	{Slug: "pekerjaan", Title: "Riwayat Pekerjaan", PagePath: "/mcu/riwayat-pekerjaan"},
	{Slug: "anamnesa", Title: "Anamnesa", PagePath: "/mcu/anamnesa"},
	{Slug: "gigi", Title: "Pemeriksaan Gigi", PagePath: "/mcu/pemeriksaan-gigi"},
	{Slug: "rontgen", Title: "Rontgen Thorax", PagePath: "/mcu/rontgen-thorax", Groups: []string{"R"}},
	{Slug: "ekg", Title: "EKG", PagePath: "/mcu/ekg", Groups: []string{"K"}},
	{Slug: "treadmill", Title: "EKG Treadmill", PagePath: "/mcu/ekg-treadmill"},
	{Slug: "spirometri", Title: "Spirometry", PagePath: "/mcu/spirometry"},
	{Slug: "usg", Title: "USG", PagePath: "/mcu/usg", Groups: []string{"M", "N", "O", "P", "Q"}},
}

// FormBySlug finds a form.
func FormBySlug(slug string) (Form, bool) {
	for _, f := range Forms {
		if f.Slug == slug {
			return f, true
		}
	}
	return Form{}, false
}

// Limits of one save.
const (
	MaxItems      = 800
	MaxKeyLen     = 60
	MaxValueLen   = 20000 // lists are JSON in one value
	MaxNoteLen    = 2000
	MaxKesimpulan = 4000
)

// Status values of a form.
const (
	StatusNormal   = "normal"
	StatusAbnormal = "abnormal"
)
