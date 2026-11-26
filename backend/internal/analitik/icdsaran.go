package analitik

import (
	"context"
	"log"
	"net/http"
	"strings"
	"unicode/utf8"

	"github.com/example/be_jwt_golang/backend/internal/httpx"
	"github.com/example/be_jwt_golang/backend/internal/icd"
)

// PengobatanPagePath is the Pengobatan form's menu item (pengobatan.PagePath):
// the ICD suggestions serve that form, so its users may call them.
const PengobatanPagePath = "/transaksi-klinik/pengobatan"

// MaxDeskripsi caps the clinical description sent for ICD suggestions.
const MaxDeskripsi = 500

// ICDSaran is a code the assistant proposes, with its reason.
type ICDSaran struct {
	Kode   string `json:"kode"`
	Alasan string `json:"alasan"`
}

// ICDKandidat is a reficd entry offered to the doctor.
type ICDKandidat struct {
	ID     int    `json:"id"`
	Kode   string `json:"kode"`
	Nama   string `json:"nama"`
	Jenis  string `json:"jenis"`
	Alasan string `json:"alasan"` // from the assistant; empty for search hits
	Sumber string `json:"sumber"` // "ai" or "pencarian"
}

// ICDByKode finds the reficd entries of codes (exact code, else the code's
// category before the dot, e.g. J06 for J06.9), in the order of codes.
func (p *PostgresRepository) ICDByKode(ctx context.Context, codes []string) (map[string]icd.ICD, error) {
	out := map[string]icd.ICD{}
	if len(codes) == 0 {
		return out, nil
	}
	cats := make([]string, len(codes))
	for i, c := range codes {
		cats[i], _, _ = strings.Cut(c, ".")
	}
	rows, err := p.pool.Query(ctx, `SELECT i.id, i.kode_icd, COALESCE(i.nama, ''), COALESCE(j.info, '')
		FROM reficd i LEFT JOIN refjenisicd j ON j.id = i.id_jenis_icd
		WHERE upper(i.kode_icd) = ANY($1) OR upper(i.kode_icd) = ANY($2)`, codes, cats)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	found := map[string]icd.ICD{}
	for rows.Next() {
		var d icd.ICD
		if err := rows.Scan(&d.ID, &d.Kode, &d.Nama, &d.Jenis); err != nil {
			return nil, err
		}
		found[strings.ToUpper(d.Kode)] = d
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for i, c := range codes {
		if d, ok := found[c]; ok {
			out[c] = d
		} else if d, ok := found[cats[i]]; ok {
			out[c] = d
		}
	}
	return out, nil
}

// SearchICD searches reficd by code or name (the ICD master's own search).
func (p *PostgresRepository) SearchICD(ctx context.Context, q string, limit int) ([]icd.ICD, error) {
	page, err := icd.NewPostgresRepository(p.pool).List(ctx, icd.ListParams{Search: q, Limit: limit})
	return page.Rows, err
}

// searchTerms picks the words of a description worth searching the ICD names
// for (longest first): short words and digits match too much.
func searchTerms(deskripsi string, n int) []string {
	seen := map[string]bool{}
	var words []string
	for _, w := range strings.FieldsFunc(strings.ToLower(deskripsi), func(r rune) bool {
		return !(r >= 'a' && r <= 'z')
	}) {
		if utf8.RuneCountInString(w) >= 5 && !seen[w] {
			seen[w] = true
			words = append(words, w)
		}
	}
	// longest first, stable
	for i := 1; i < len(words); i++ {
		for j := i; j > 0 && len(words[j]) > len(words[j-1]); j-- {
			words[j], words[j-1] = words[j-1], words[j]
		}
	}
	return words[:min(n, len(words))]
}

// SaranICD suggests ICD-10 codes for a clinical description: the assistant's
// candidates that exist in reficd (others are dropped), then matches of the
// ICD master's search. Without an assistant only the search runs. Only the
// description is sent to the assistant.
//
// Body: {deskripsi}.
func (h *Handler) SaranICD(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Deskripsi string `json:"deskripsi"`
	}
	if !httpx.DecodeJSON(w, r, &in) {
		return
	}
	d := strings.TrimSpace(in.Deskripsi)
	if n := utf8.RuneCountInString(d); n < 3 || n > MaxDeskripsi {
		httpx.WriteValidation(w, httpx.FieldErrors{"deskripsi": "Tulis deskripsi klinis (3–500 karakter)"})
		return
	}
	out := []ICDKandidat{}
	seen := map[int]bool{}
	aiError := ""
	if h.ai != nil {
		saran, err := h.ai.SaranICD(r.Context(), d)
		if err != nil {
			log.Printf("analitik: ICD suggestion failed: %v", err)
			aiError = "Saran AI tidak tersedia saat ini; menampilkan hasil pencarian."
		} else {
			codes := make([]string, 0, len(saran))
			for _, s := range saran {
				codes = append(codes, strings.ToUpper(strings.TrimSpace(s.Kode)))
			}
			found, err := h.repo.ICDByKode(r.Context(), codes)
			if err != nil {
				httpx.InternalError(w, err)
				return
			}
			for i, s := range saran {
				if c, ok := found[codes[i]]; ok && !seen[c.ID] {
					seen[c.ID] = true
					out = append(out, ICDKandidat{ID: c.ID, Kode: c.Kode, Nama: c.Nama, Jenis: c.Jenis, Alasan: strings.TrimSpace(s.Alasan), Sumber: "ai"})
				}
			}
		}
	}
	// Word search matches loosely ("nyeri" finds every kind of pain), so once
	// the assistant has found candidates only a few search hits follow.
	limit := 12
	if len(out) > 0 {
		limit = len(out) + 3
	}
	for _, term := range searchTerms(d, 3) {
		if len(out) >= limit {
			break
		}
		hits, err := h.repo.SearchICD(r.Context(), term, 5)
		if err != nil {
			httpx.InternalError(w, err)
			return
		}
		for _, c := range hits {
			if !seen[c.ID] && len(out) < limit {
				seen[c.ID] = true
				out = append(out, ICDKandidat{ID: c.ID, Kode: c.Kode, Nama: c.Nama, Jenis: c.Jenis, Sumber: "pencarian"})
			}
		}
	}
	httpx.WriteJSON(w, http.StatusOK, map[string]any{"ai": h.ai != nil && aiError == "", "aiError": aiError, "kandidat": out})
}
