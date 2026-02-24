package brfintel

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"regexp"
	"strings"
	"time"

	pdf "github.com/ledongthuc/pdf"

	"k2MarketingAi/internal/llm"
)

// ──────────────────────────────────────────────────────────────────────
// PDF Upload + OCR Handler
//
// Accepts a BRF annual report PDF, extracts text (searchable PDF),
// or falls back to Gemini Vision multimodal for scanned/image PDFs.
// ──────────────────────────────────────────────────────────────────────

const (
	maxPDFBytes = 20 * 1024 * 1024 // 20 MB
	maxPDFPages = 50
	maxPDFChars = 25000
)

// AnalyzePDF handles POST /api/brf-intel/analyze-pdf.
// Accepts multipart form with:
//   - file: PDF file
//   - brf_name: required
//   - org_number: optional
//   - municipality: optional
//   - city: optional
func (h Handler) AnalyzePDF(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(w, r)
	if !ok {
		return
	}

	if err := r.ParseMultipartForm(maxPDFBytes + (1 << 20)); err != nil {
		http.Error(w, fmt.Sprintf("ogiltig uppladdning: %v", err), http.StatusBadRequest)
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		http.Error(w, "fil saknas (välj en PDF)", http.StatusBadRequest)
		return
	}
	defer file.Close()

	data, err := io.ReadAll(io.LimitReader(file, maxPDFBytes+1))
	if err != nil {
		http.Error(w, "kunde inte läsa filen", http.StatusBadRequest)
		return
	}
	if len(data) == 0 {
		http.Error(w, "filen var tom", http.StatusBadRequest)
		return
	}
	if len(data) > maxPDFBytes {
		http.Error(w, "filen är för stor (max 20 MB)", http.StatusBadRequest)
		return
	}

	brfName := strings.TrimSpace(r.FormValue("brf_name"))
	if brfName == "" {
		brfName = "Okänd förening"
	}

	req := AnalyzeRequest{
		BRFName:      brfName,
		OrgNumber:    strings.TrimSpace(r.FormValue("org_number")),
		Municipality: strings.TrimSpace(r.FormValue("municipality")),
		City:         strings.TrimSpace(r.FormValue("city")),
		FileName:     header.Filename,
	}

	log.Printf("brfintel: PDF upload file=%s size=%d brf=%s", header.Filename, len(data), brfName)

	// ── Step 1: Try text extraction from searchable PDF ──
	text, pageCount, extractErr := extractPDFTextLocal(data, maxPDFPages)
	text = strings.TrimSpace(text)

	// Evaluate text quality: we need both sufficient volume AND financial content.
	// A scanned PDF often yields small fragments (headers, footers) but no real data.
	charsPerPage := 0
	if pageCount > 0 {
		charsPerPage = len(text) / pageCount
	}
	textUsable := len(text) > 500 && charsPerPage > 300 && hasFinanceSignals(text)

	if textUsable {
		log.Printf("brfintel: text extraction OK chars=%d pages=%d charsPerPage=%d financeSignals=true", len(text), pageCount, charsPerPage)
		if len(text) > maxPDFChars {
			text = text[:maxPDFChars]
		}
		req.RawText = text
	} else {
		// ── Step 2: Text is absent or garbage → Gemini Vision multimodal OCR ──
		reason := "unknown"
		if len(text) <= 500 {
			reason = fmt.Sprintf("too_few_chars(%d)", len(text))
		} else if charsPerPage <= 300 {
			reason = fmt.Sprintf("low_density(%d_chars/%d_pages=%d_cpp)", len(text), pageCount, charsPerPage)
		} else {
			reason = fmt.Sprintf("no_finance_signals(chars=%d)", len(text))
		}
		log.Printf("brfintel: text quality poor (%s extractErr=%v), trying Gemini Vision OCR", reason, extractErr)

		visionText, visionErr := geminiPDFOCR(r.Context(), h.Analyzer.llm, data)
		if visionErr != nil {
			log.Printf("brfintel: Gemini Vision OCR failed: %v", visionErr)
			// If we had SOME text, use it as a last resort rather than failing completely
			if len(text) > 200 {
				log.Printf("brfintel: falling back to low-quality extracted text (%d chars)", len(text))
				if len(text) > maxPDFChars {
					text = text[:maxPDFChars]
				}
				req.RawText = text
			} else {
				http.Error(w, fmt.Sprintf("kunde inte läsa PDF:en. Varken textextraktion eller AI-OCR lyckades. Försök med en bättre skannad PDF.\nDetalj: %v", visionErr), http.StatusBadRequest)
				return
			}
		} else {
			visionText = strings.TrimSpace(visionText)
			if len(visionText) < 100 {
				http.Error(w, "PDF:en verkar inte innehålla tillräcklig text. Kontrollera att det är en årsredovisning.", http.StatusBadRequest)
				return
			}
			log.Printf("brfintel: Gemini Vision OCR OK chars=%d", len(visionText))
			if len(visionText) > maxPDFChars {
				visionText = visionText[:maxPDFChars]
			}
			req.RawText = visionText
		}
	}

	// ── Step 3: Run BRF Intelligence Pipeline ──
	report, err := h.Analyzer.Analyze(r.Context(), req, user.ID)
	if err != nil {
		status := http.StatusInternalServerError
		if strings.Contains(err.Error(), "ingen data") {
			status = http.StatusBadRequest
		}
		http.Error(w, fmt.Sprintf("analys misslyckades: %v", err), status)
		return
	}

	// Add source document metadata
	report.SourceDocuments = append(report.SourceDocuments, SourceDocument{
		FileName:  header.Filename,
		PageCount: pageCount,
		CharCount: len(req.RawText),
		InputKind: "pdf-upload",
	})

	// Persist
	if h.BRFStore != nil {
		if err := h.BRFStore.SaveBRFReport(r.Context(), report); err != nil {
			log.Printf("brfintel: failed to save report %s: %v", report.ID, err)
		}
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(AnalyzeResponse{Report: report})
}

// ──────────────────────────────────────────────────────────────────────
// Local PDF text extraction (for searchable PDFs)
// Uses github.com/ledongthuc/pdf — same library as listings
// ──────────────────────────────────────────────────────────────────────

func extractPDFTextLocal(data []byte, maxPages int) (string, int, error) {
	reader, err := pdf.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		return "", 0, fmt.Errorf("PDF parse error: %w", err)
	}

	pages := reader.NumPage()
	readPages := pages
	if maxPages > 0 && pages > maxPages {
		readPages = maxPages
	}

	var b strings.Builder
	for i := 1; i <= readPages; i++ {
		p := reader.Page(i)
		if p.V.IsNull() {
			continue
		}

		text, perr := p.GetPlainText(nil)
		if perr != nil || strings.TrimSpace(text) == "" {
			// Try row-based extraction
			rows, rerr := p.GetTextByRow()
			if rerr == nil && len(rows) > 0 {
				var rowBuf strings.Builder
				for _, row := range rows {
					for _, cell := range row.Content {
						rowBuf.WriteString(cell.S)
					}
					rowBuf.WriteString("\n")
				}
				text = rowBuf.String()
			}
		}

		if strings.TrimSpace(text) != "" {
			b.WriteString(text)
			b.WriteString("\n\n")
		}
	}

	return b.String(), readPages, nil
}

// ──────────────────────────────────────────────────────────────────────
// Gemini Vision OCR — send raw PDF bytes to Gemini multimodal
//
// Gemini 1.5/2.0+ supports inline PDF data. We send the full PDF
// and ask the model to extract all text, preserving tables and numbers.
// ──────────────────────────────────────────────────────────────────────

func geminiPDFOCR(ctx context.Context, llmClient llm.Client, pdfData []byte) (string, error) {
	if llmClient == nil {
		return "", fmt.Errorf("LLM client saknas")
	}

	gemini, ok := llmClient.(*llm.GeminiClient)
	if !ok {
		return "", fmt.Errorf("Gemini Vision OCR kräver GeminiClient")
	}

	modelCtx := llm.WithModel(ctx, "gemini-2.5-flash")

	// Build multimodal request with inline PDF
	encoded := base64.StdEncoding.EncodeToString(pdfData)

	prompt := `Du är en OCR-specialist. Extrahera ALL text från detta PDF-dokument.

Regler:
- Behåll ALLA siffror exakt som de står
- Behåll tabeller i läsbar form (en rad per rad, kolumner separerade med tab eller mellanslag)
- Behåll rubriker och styckeindelning
- Inkludera alla sidor
- Svara ENBART med den extraherade texten, ingen kommentar eller inledning
- Om dokumentet är en årsredovisning, var extra noga med att extrahera:
  * Resultaträkning / resultatrapport
  * Balansräkning  
  * Nyckeltal
  * Förvaltningsberättelse
  * Skulder och tillgångar`

	payload := map[string]any{
		"contents": []map[string]any{
			{
				"role": "user",
				"parts": []map[string]any{
					{"text": prompt},
					{
						"inline_data": map[string]string{
							"mime_type": "application/pdf",
							"data":      encoded,
						},
					},
				},
			},
		},
		"generationConfig": map[string]any{
			"temperature":     0.1,
			"maxOutputTokens": 8192,
		},
	}

	result, err := gemini.RawRequest(modelCtx, payload)
	if err != nil {
		return "", fmt.Errorf("Gemini Vision OCR: %w", err)
	}

	return strings.TrimSpace(result), nil
}

// ──────────────────────────────────────────────────────────────────────
// RecentReports handles GET /api/brf-intel/recent — returns recent
// reports with scores for the dashboard widget.
// ──────────────────────────────────────────────────────────────────────
func (h Handler) RecentReports(w http.ResponseWriter, r *http.Request) {
	user, ok := userFromRequest(w, r)
	if !ok {
		return
	}

	if h.BRFStore == nil {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode([]BRFReport{})
		return
	}

	reports, err := h.BRFStore.ListBRFReports(r.Context(), user.ID)
	if err != nil {
		http.Error(w, "kunde inte hämta rapporter", http.StatusInternalServerError)
		return
	}

	// Return max 10 recent
	if len(reports) > 10 {
		reports = reports[:10]
	}

	type briefReport struct {
		ID        string    `json:"id"`
		BRFName   string    `json:"brf_name"`
		Score     int       `json:"score"`
		Grade     string    `json:"grade"`
		RiskCount int       `json:"risk_count"`
		CreatedAt time.Time `json:"created_at"`
	}

	var brief []briefReport
	for _, rpt := range reports {
		brief = append(brief, briefReport{
			ID:        rpt.ID,
			BRFName:   rpt.BRFName,
			Score:     rpt.Score.Total,
			Grade:     rpt.Score.Grade,
			RiskCount: len(rpt.Risks),
			CreatedAt: rpt.CreatedAt,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(brief)
}

// ──────────────────────────────────────────────────────────────────────
// Text quality detection
//
// Checks extracted text for financial signals typical of Swedish BRF
// annual reports. Without these, the LLM will return all zeros.
// ──────────────────────────────────────────────────────────────────────

func hasFinanceSignals(text string) bool {
	lower := strings.ToLower(text)
	if lower == "" {
		return false
	}

	// If text is mostly signing log, it's not useful
	if strings.Contains(lower, "signeringslogg") || strings.Contains(lower, "signeringslog") {
		if !strings.Contains(lower, "balansräkning") && !strings.Contains(lower, "resultaträkning") {
			return false
		}
	}

	strongSignals := []string{
		"balansräkning",
		"resultaträkning",
		"kassaflödesanalys",
		"förvaltningsberättelse",
		"rörelseresultat",
		"räntekost",
		"avskriv",
		"likvida medel",
		"skulder",
		"tillgångar",
		"eget kapital",
		"noter",
		"årsavgifter",
		"omsättning",
		"rörelseintäkter",
		"driftskostnader",
		"förvaltningskostnader",
		"underhållsfond",
		"reparationsfond",
		"yttre fond",
		"nettoomsättning",
		"fastighetsavgift",
		"fastighetsskatt",
	}
	for _, sig := range strongSignals {
		if strings.Contains(lower, sig) {
			return true
		}
	}

	// Numeric money patterns (e.g. 1 234 567 kr, 3,4 Mkr, 850 tkr)
	moneyRe := regexp.MustCompile(`(?i)\b\d{1,3}(?:[ .]\d{3})+(?:[.,]\d+)?\s*(kr|tkr|mkr)\b|\b\d+[.,]?\d*\s*(kr|tkr|mkr)\b`)
	if moneyRe.MatchString(lower) {
		return true
	}

	// At least 30 digits + currency abbreviation
	digitCount := 0
	for _, ch := range lower {
		if ch >= '0' && ch <= '9' {
			digitCount++
		}
	}
	if digitCount >= 30 && (strings.Contains(lower, "kr") || strings.Contains(lower, "tkr") || strings.Contains(lower, "mkr")) {
		return true
	}

	return false
}
