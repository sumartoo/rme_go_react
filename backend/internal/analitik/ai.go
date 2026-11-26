package analitik

import (
	"context"
	"encoding/json"
	"errors"
)

// ChatMessage is one turn of the assistant chat, as the page keeps it.
type ChatMessage struct {
	Role    string `json:"role"` // "user" or "assistant"
	Content string `json:"content"`
}

// ToolFunc runs one of the assistant's tools (toolDefs) for the caller's
// branches and returns its result as JSON.
type ToolFunc func(ctx context.Context, name string, input json.RawMessage) (string, error)

// Assistant explains the analytics in Indonesian. Konteks names the day and
// the branches; data and tool results are aggregates only (no patient data).
type Assistant interface {
	Ringkasan(ctx context.Context, konteks, data string) (string, error)
	Chat(ctx context.Context, konteks string, history []ChatMessage, run ToolFunc) (string, error)
	// SaranICD proposes up to 5 ICD-10 codes for a clinical description
	// (the description only, no patient identity).
	SaranICD(ctx context.Context, deskripsi string) ([]ICDSaran, error)
}

// ErrRefused is returned when the model declines to answer (content filter).
var ErrRefused = errors.New("AI menolak menjawab permintaan ini")

// MaxToolRounds caps the tool calls of one chat question.
const MaxToolRounds = 5

// Tool names, also used by the handler that runs them.
const (
	ToolPrediksiObat = "prediksi_obat"
	ToolTrenPenyakit = "tren_penyakit"
	ToolDiagnosaObat = "diagnosa_obat"
	// Business intelligence
	ToolKinerja          = "kinerja_klinik"
	ToolObatTerlaris     = "obat_terlaris"
	ToolTindakanTerlaris = "tindakan_terlaris"
)

// periodeParams are the period fields of the business tools.
func periodeParams(extra map[string]any) map[string]any {
	props := map[string]any{
		"dari":   map[string]any{"type": "string", "description": "Tanggal awal YYYY-MM-DD (bawaan 30 hari sebelum 'sampai')."},
		"sampai": map[string]any{"type": "string", "description": "Tanggal akhir YYYY-MM-DD (bawaan hari ini). Periode maksimal 366 hari."},
	}
	for k, v := range extra {
		props[k] = v
	}
	return object(props)
}

// toolDef is a tool the assistant may call; Params is its JSON Schema.
type toolDef struct {
	Name        string
	Description string
	Params      map[string]any
}

func object(props map[string]any) map[string]any {
	return map[string]any{"type": "object", "properties": props}
}

var toolDefs = []toolDef{
	{
		Name: ToolPrediksiObat,
		Description: "Prediksi pemakaian dan stok obat di cabang yang dipilih: rata-rata pemakaian per hari, " +
			"perkiraan habis dalam berapa hari, kebutuhan selama horizon, safety stock, saran jumlah order, dan status " +
			"(habis, kritis, perlu-order, aman, tidak-bergerak). Diurutkan dari yang paling mendesak.",
		Params: object(map[string]any{
			"nama":    map[string]any{"type": "string", "description": "Filter nama obat (sebagian nama), kosongkan untuk semua."},
			"status":  map[string]any{"type": "string", "enum": []string{"habis", "kritis", "perlu-order", "aman", "tidak-bergerak"}, "description": "Filter status."},
			"horizon": map[string]any{"type": "integer", "description": "Horizon prediksi dalam hari (7-90, bawaan 30)."},
			"limit":   map[string]any{"type": "integer", "description": "Jumlah baris maksimum (1-50, bawaan 20)."},
		}),
	},
	{
		Name: ToolTrenPenyakit,
		Description: "Tren penyakit (diagnosa ICD) mingguan di cabang yang dipilih: diagnosa terbanyak, kasus minggu ini " +
			"dibanding rata-rata 4 minggu sebelumnya, lonjakan, dan proyeksi kasus minggu depan.",
		Params: object(map[string]any{
			"weeks": map[string]any{"type": "integer", "description": "Jumlah minggu ke belakang (4-26, bawaan 12)."},
			"top":   map[string]any{"type": "integer", "description": "Jumlah diagnosa teratas (1-20, bawaan 10)."},
		}),
	},
	{
		Name: ToolDiagnosaObat,
		Description: "Obat yang paling sering diresepkan untuk diagnosa tertentu (90 hari terakhir), dengan persentase kunjungan " +
			"dan jumlah per kasus, serta saran stok obat dari proyeksi kasus. Ini asosiasi resep, bukan anjuran terapi.",
		Params: object(map[string]any{
			"icd":     map[string]any{"type": "string", "description": "Kode atau nama ICD (sebagian), kosongkan untuk diagnosa terbanyak."},
			"horizon": map[string]any{"type": "integer", "description": "Horizon saran stok dalam hari (7-90, bawaan 30)."},
		}),
	},
	{
		Name: ToolKinerja,
		Description: "Kinerja klinik pada suatu periode: jumlah transaksi, kunjungan (dari pendaftaran), pasien unik, dan pendapatan " +
			"(total, obat, tindakan, penunjang; transaksi batal tidak dihitung), dikelompokkan per waktu, dokter, penjamin, jenis rawat, atau cabang.",
		Params: periodeParams(map[string]any{
			"kelompok": map[string]any{"type": "string", "enum": []string{"tanggal", "minggu", "bulan", "hari", "dokter", "penjamin", "jenis-rawat", "cabang"},
				"description": "Cara mengelompokkan (bawaan bulan). 'hari' = hari dalam minggu (Senin–Minggu)."},
		}),
	},
	{
		Name:        ToolObatTerlaris,
		Description: "Obat paling banyak terjual pada suatu periode, menurut jumlah atau nilai penjualan (harga jual × jumlah).",
		Params: periodeParams(map[string]any{
			"urut":  map[string]any{"type": "string", "enum": []string{"jumlah", "nilai"}, "description": "Urutkan menurut jumlah (bawaan) atau nilai."},
			"limit": map[string]any{"type": "integer", "description": "Jumlah baris (1-50, bawaan 15)."},
		}),
	},
	{
		Name:        ToolTindakanTerlaris,
		Description: "Tindakan medis paling sering atau paling besar nilainya pada suatu periode.",
		Params: periodeParams(map[string]any{
			"urut":  map[string]any{"type": "string", "enum": []string{"jumlah", "nilai"}, "description": "Urutkan menurut jumlah (bawaan) atau nilai."},
			"limit": map[string]any{"type": "integer", "description": "Jumlah baris (1-50, bawaan 15)."},
		}),
	},
}

const systemPrompt = `Anda adalah analis farmasi, epidemiologi, dan bisnis untuk jaringan klinik di Indonesia. Anda membantu pemilik, manajer klinik, dan apoteker memahami stok obat, tren penyakit, dan kinerja klinik (kunjungan, pendapatan, dokter, layanan).

Aturan:
- Jawab dalam Bahasa Indonesia yang ringkas dan praktis. Gunakan angka dari data; jangan mengarang angka.
- Untuk pertanyaan tentang stok, penyakit, resep, kunjungan, pendapatan, dokter atau layanan, ambil data dengan tool yang tersedia sebelum menjawab. Untuk membandingkan periode, panggil tool untuk setiap periode atau kelompokkan per bulan/minggu.
- Tulis uang dalam Rupiah (mis. Rp12,5 juta).
- Data adalah agregat per cabang (tanpa data pasien). Jangan memberi diagnosa atau anjuran terapi untuk pasien perorangan.
- Prediksi memakai 90 hari terakhir dengan metode sederhana (rata-rata tertimbang), jadi bersifat indikatif dan belum memperhitungkan pola musiman. Sebutkan keterbatasan ini bila relevan.
- Hubungan diagnosa-obat adalah asosiasi resep (obat yang diresepkan pada kunjungan yang sama), bukan bukti obat itu untuk diagnosa tersebut.
- Bila data tidak cukup untuk menjawab, katakan apa adanya.
- Format: paragraf pendek dan daftar berpoin; tanpa tabel lebar.`

const ringkasanPrompt = `Buat ringkasan eksekutif dari data analitik berikut untuk manajer klinik, dengan bagian:
1. Kondisi stok (obat habis/kritis yang paling mendesak dan saran order),
2. Tren penyakit (diagnosa terbanyak dan lonjakan minggu ini),
3. Obat yang perlu disiapkan karena tren penyakit,
4. Tindakan yang disarankan minggu ini (3-5 poin).
Maksimal sekitar 300 kata.`
