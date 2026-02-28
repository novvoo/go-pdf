//go:build gopdfcmd
// +build gopdfcmd

package main

import (
	"bytes"
	"fmt"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/novvoo/go-pdf/pkg/gopdf"
	pdffont "github.com/pdfcpu/pdfcpu/pkg/font"
)

type textLine struct {
	Y                float64
	MinX             float64
	MaxX             float64
	FontSize         float64
	Text             string
	Elements         []gopdf.TextElementInfo
	ElemCount        int
	FontNames        []string
	HasToUnicode     int
	IsIdentity       int
	CIDCount         int
	ReplacementCount int
	ToUnicodeHit     int
	GlyphNameHit     int
	IdentityASCIIHit int
}

func normalizeLineTextForReport(s string) string {
	compact := strings.ReplaceAll(s, " ", "")
	compact = strings.ReplaceAll(compact, "\t", "")
	if compact == "|{}" {
		return "⏟"
	}
	return s
}

func main() {
	pdfPath := "example/test_vector.pdf"
	reportPath := "example/render_vector_postscript.txt"
	outTranslatedSVGPath := "example/test_vector_postscript.svg"
	outTranslatedPDFPath := "example/test_vector_postscript.pdf"

	oldStdout := os.Stdout
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w

	var debugBuf bytes.Buffer
	gopdf.SetDebugOutput(&debugBuf)

	outputChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outputChan <- buf.String()
	}()

	var report strings.Builder
	report.WriteString("PDF Translation Report (Simplified Vector)\n")
	report.WriteString("==========================================\n")
	report.WriteString(fmt.Sprintf("Time: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	report.WriteString(fmt.Sprintf("📄 Input PDF: %s\n", pdfPath))

	if !fileExists(pdfPath) {
		report.WriteString(fmt.Sprintf("❌ Error: PDF file not found: %s\n", pdfPath))
		finishReport(reportPath, report.String(), &capturedIO{w: w, oldStdout: oldStdout, oldStderr: oldStderr, outChan: outputChan})
		return
	}

	chineseSansFont, chineseSerifFont := "Helvetica", "Helvetica"

	reader := gopdf.NewPDFReader(pdfPath)
	defer reader.Close()

	pageCount, err := reader.GetPageCount()
	if err != nil {
		report.WriteString(fmt.Sprintf("❌ Failed to get page count: %v\n", err))
		finishReport(reportPath, report.String(), &capturedIO{w: w, oldStdout: oldStdout, oldStderr: oldStderr, outChan: outputChan})
		return
	}
	report.WriteString(fmt.Sprintf("✅ Page count: %d\n", pageCount))

	const dpi = 144.0
	var vectorOverlays []gopdf.TextOverlayTopLeft

	for pageNum := 1; pageNum <= pageCount; pageNum++ {
		pageInfo, err := reader.GetPageInfo(pageNum)
		if err != nil {
			report.WriteString(fmt.Sprintf("❌ Failed to get page info (page=%d): %v\n", pageNum, err))
			continue
		}

		textElements, _ := reader.ExtractPageElements(pageNum)
		lines := groupIntoLines(textElements)
		titleThr := detectTitleFontThreshold(lines)

		report.WriteString(fmt.Sprintf("\nPage %d:\n", pageNum))
		report.WriteString(fmt.Sprintf("✅ Extracted %d text elements -> %d lines\n", len(textElements), len(lines)))

		report.WriteString("Elements (ordered by y then x):\n")
		elemsForLog := append([]gopdf.TextElementInfo(nil), textElements...)
		sort.Slice(elemsForLog, func(i, j int) bool {
			if elemsForLog[i].Y == elemsForLog[j].Y {
				return elemsForLog[i].X < elemsForLog[j].X
			}
			return elemsForLog[i].Y < elemsForLog[j].Y
		})
		maxElemLog := 400
		if len(elemsForLog) < maxElemLog {
			maxElemLog = len(elemsForLog)
		}
		for i := 0; i < maxElemLog; i++ {
			e := elemsForLog[i]
			report.WriteString(fmt.Sprintf(
				"  - elem[%03d] x=%.2f y=%.2f w=%.2f h=%.2f fs=%.2f font=%q base=%q tu=%t ident=%t cid=%d repl=%d hitTU=%d hitGN=%d hitIA=%d text=%q raw=%q\n",
				i, e.X, e.Y, e.Width, e.Height, e.FontSize, e.FontName, e.FontBaseName,
				e.HasToUnicode, e.IsIdentity, e.CIDCount, e.ReplacementCount, e.ToUnicodeHit, e.GlyphNameHit, e.IdentityASCIIHit,
				clipForLog(escapeForLog(e.Text), 160), clipForLog(escapeForLog(e.RawText), 160),
			))
		}
		if len(elemsForLog) > maxElemLog {
			report.WriteString(fmt.Sprintf("  ... omitted %d elements\n", len(elemsForLog)-maxElemLog))
		}

		report.WriteString("Lines (grouped):\n")
		for i, ln := range lines {
			raw := strings.TrimSpace(ln.Text)
			kind := classifyLineKind(ln, raw, titleThr)
			report.WriteString(fmt.Sprintf(
				"  - line[%03d] kind=%s y=%.2f x=[%.2f..%.2f] fs=%.2f elems=%d fonts=%q tu=%d ident=%d cid=%d repl=%d hitTU=%d hitGN=%d hitIA=%d text=%q\n",
				i, kind, ln.Y, ln.MinX, ln.MaxX, ln.FontSize, ln.ElemCount, strings.Join(ln.FontNames, ","),
				ln.HasToUnicode, ln.IsIdentity, ln.CIDCount, ln.ReplacementCount, ln.ToUnicodeHit, ln.GlyphNameHit, ln.IdentityASCIIHit,
				clipForLog(escapeForLog(raw), 220),
			))
			maxLineElem := 30
			if len(ln.Elements) < maxLineElem {
				maxLineElem = len(ln.Elements)
			}
			for j := 0; j < maxLineElem; j++ {
				e := ln.Elements[j]
				report.WriteString(fmt.Sprintf(
					"    * e[%02d] x=%.2f y=%.2f w=%.2f h=%.2f fs=%.2f font=%q base=%q tu=%t ident=%t cid=%d repl=%d text=%q raw=%q\n",
					j, e.X, e.Y, e.Width, e.Height, e.FontSize, e.FontName, e.FontBaseName,
					e.HasToUnicode, e.IsIdentity, e.CIDCount, e.ReplacementCount,
					clipForLog(escapeForLog(e.Text), 120), clipForLog(escapeForLog(e.RawText), 120),
				))
			}
			if len(ln.Elements) > maxLineElem {
				report.WriteString(fmt.Sprintf("    * ... omitted %d elements\n", len(ln.Elements)-maxLineElem))
			}
		}

		pageOverlays, vecLog := buildTranslationOverlays(pageNum, lines, pageInfo.Width, pageInfo.Height, chineseSansFont, chineseSerifFont)
		vectorOverlays = append(vectorOverlays, pageOverlays...)
		report.WriteString(vecLog)
	}

	basePSPath := "example/test_vector.ps"
	baseSVGPath := "example/test_vector.svg"
	_ = os.Remove(basePSPath)
	if err := reader.WritePostScriptBestEffort(basePSPath, dpi); err != nil {
		report.WriteString(fmt.Sprintf("❌ Failed to write base PostScript: %v\n", err))
		finishReport(reportPath, report.String(), &capturedIO{w: w, oldStdout: oldStdout, oldStderr: oldStderr, outChan: outputChan})
		return
	}

	report.WriteString(fmt.Sprintf("\n✅ Base PS: %s\n", basePSPath))
	report.WriteString(fmt.Sprintf("✅ Vector translated overlays: %d\n", len(vectorOverlays)))

	if b, err := os.ReadFile(basePSPath); err == nil {
		psContent := string(b)
		rep := gopdf.AnalyzePSShowFragmentation(psContent, 30)
		psNewPath := strings.Count(psContent, "\nnewpath\n")
		psCurveTo := strings.Count(psContent, " curveto\n")
		report.WriteString("\n[PS Text Check]\n")
		report.WriteString(fmt.Sprintf("- shows=%d literal=%d hex=%d\n", rep.TotalShows, rep.LiteralShows, rep.HexShows))
		report.WriteString(fmt.Sprintf("- short(len=1)=%d len=2=%d len=3=%d\n", rep.ShortShowsLen1, rep.ShortShowsLen2, rep.ShortShowsLen3))
		report.WriteString(fmt.Sprintf("- consecutive-letter-fragments=%d (body=%d math=%d)\n", rep.ConsecutiveLetters, rep.BodyConsecutiveLetters, rep.MathConsecutiveLetters))
		report.WriteString(fmt.Sprintf("- ps_paths: newpath=%d curveto=%d\n", psNewPath, psCurveTo))
		if rep.BodyConsecutiveLetters > 0 && len(rep.BodyExamples) > 0 {
			report.WriteString("  body examples:\n")
			for i := 0; i < len(rep.BodyExamples) && i < 10; i++ {
				ex := rep.BodyExamples[i]
				report.WriteString(fmt.Sprintf("  - line=%d %s\n", ex.LineNo, ex.Text))
			}
		}
		const maxBodyFragmentation = 50
		if rep.BodyConsecutiveLetters > maxBodyFragmentation {
			report.WriteString(fmt.Sprintf("❌ PS text check failed: body-fragmentation=%d (limit=%d)\n", rep.BodyConsecutiveLetters, maxBodyFragmentation))
			finishReport(reportPath, report.String(), &capturedIO{w: w, oldStdout: oldStdout, oldStderr: oldStderr, outChan: outputChan, debug: &debugBuf})
			os.Exit(2)
		}
	} else {
		report.WriteString(fmt.Sprintf("\n[PS Text Check] ❌ read PS failed: %v\n", err))
	}

	_ = os.Remove(outTranslatedSVGPath)
	_ = os.Remove(outTranslatedPDFPath)
	_ = os.Remove(baseSVGPath)

	if err := gopdf.ConvertPostScriptToSVG(basePSPath, baseSVGPath); err != nil {
		report.WriteString(fmt.Sprintf("❌ Failed to convert PS to base SVG: %v\n", err))
		finishReport(reportPath, report.String(), &capturedIO{w: w, oldStdout: oldStdout, oldStderr: oldStderr, outChan: outputChan, debug: &debugBuf})
		return
	}
	report.WriteString(fmt.Sprintf("✅ Base SVG: %s\n", baseSVGPath))

	if err := gopdf.InsertTextOverlaysIntoSVG(baseSVGPath, outTranslatedSVGPath, vectorOverlays); err != nil {
		report.WriteString(fmt.Sprintf("❌ Failed to inject translations into SVG: %v\n", err))
		finishReport(reportPath, report.String(), &capturedIO{w: w, oldStdout: oldStdout, oldStderr: oldStderr, outChan: outputChan, debug: &debugBuf})
		return
	}

	if err := gopdf.ConvertSVGToPDF(outTranslatedSVGPath, outTranslatedPDFPath); err != nil {
		report.WriteString(fmt.Sprintf("❌ Failed to convert translated SVG to PDF: %v\n", err))
		finishReport(reportPath, report.String(), &capturedIO{w: w, oldStdout: oldStdout, oldStderr: oldStderr, outChan: outputChan, debug: &debugBuf})
		return
	}

	report.WriteString(fmt.Sprintf("✅ Output SVG (editable, with translations): %s\n", outTranslatedSVGPath))
	report.WriteString(fmt.Sprintf("✅ Output PDF (from SVG): %s\n", outTranslatedPDFPath))

	finishReport(reportPath, report.String(), &capturedIO{w: w, oldStdout: oldStdout, oldStderr: oldStderr, outChan: outputChan, debug: &debugBuf})
}

func buildTranslationOverlays(pageNum int, lines []textLine, pageWidth, pageHeight float64, chineseSansFont, chineseSerifFont string) ([]gopdf.TextOverlayTopLeft, string) {
	leftMargin := 6.0
	rightMargin := 6.0
	topMargin := 6.0
	bottomMargin := 6.0

	var log strings.Builder
	var overlays []gopdf.TextOverlayTopLeft

	titleThr := detectTitleFontThreshold(lines)

	for i, ln := range lines {
		raw := strings.TrimSpace(ln.Text)
		if raw == "" {
			continue
		}

		kind := classifyLineKind(ln, raw, titleThr)
		if !shouldTranslateLineKind(kind, raw) {
			continue
		}
		if !containsEnglishLetters(raw) {
			continue
		}

		// Determine Font
		useSerif := false
		for _, fn := range ln.FontNames {
			l := strings.ToLower(fn)
			if strings.Contains(l, "times") || strings.Contains(l, "serif") || strings.Contains(l, "cmr") {
				useSerif = true
				break
			}
		}
		chineseFont := chineseSansFont
		if useSerif && chineseSerifFont != "" {
			chineseFont = chineseSerifFont
		}
		if chineseFont == "" {
			chineseFont = "Helvetica"
		}

		baseFontSize := ln.FontSize
		if baseFontSize <= 0 {
			baseFontSize = 12
		}
		if baseFontSize < 6 {
			baseFontSize = 6
		}

		ch := englishToZhPlaceholder(raw)

		minX := ln.MinX
		maxX := ln.MaxX
		if maxX <= minX {
			maxX = minX + 120
		}

		// Calculate Geometry
		x := clamp(minX, leftMargin, pageWidth-rightMargin)
		y := clamp(ln.Y, topMargin, pageHeight-bottomMargin)
		targetWidth := clamp(maxX-minX, 100, pageWidth-rightMargin-x)

		// Fit Text
		fontSize := float64(pdffont.Size(ch, chineseFont, targetWidth))
		if fontSize <= 0 {
			fontSize = baseFontSize * 0.80
		}
		if fontSize > baseFontSize {
			fontSize = baseFontSize
		}
		if fontSize < 6 {
			fontSize = 6
		}

		wrapped := wrapTextToWidth(ch, fontSize, targetWidth)
		if len(wrapped) == 0 {
			wrapped = []string{strings.TrimSpace(ch)}
		}

		// Create Overlays
		// Important: We anchor to the source line Y (ln.Y).
		// The SVG injector uses this Y to find the insertion point.
		// We calculate yy for subsequent wrapped lines based on this anchor.

		lh := fontSize * 1.15
		for li, s := range wrapped {
			// Calculate visual position for the translation lines.
			// The first line of translation (li=0) should appear immediately after the source line.
			// Subsequent lines (li>0) follow the line height.
			// Note: The injector shifts subsequent content (like the "notation" line) downwards.
			// So we position the translation in the space created by that shift.

			// Start Y = Source Line Y + Source Line Height + Padding
			yy := y + (baseFontSize * 1.2) + (float64(li) * lh)

			// Ensure we don't go off page (though SVG is infinite canvas effectively until printed)
			if yy > pageHeight-bottomMargin {
				break
			}

			overlays = append(overlays, gopdf.TextOverlayTopLeft{
				Page:      pageNum,
				Text:      s,
				X:         x,
				Y:         yy,
				FontName:  chineseFont,
				FontSize:  fontSize,
				FillColor: "0 0 0", // Black
				Opacity:   1.0,
				OnTop:     true,
				// TextLength can be used for justification if needed
			})
		}

		log.WriteString(fmt.Sprintf("- page=%d line[%03d] translate=yes x=%.2f y=%.2f fs=%.2f lines=%d raw=%q\n",
			pageNum, i, x, y, fontSize, len(wrapped), clipForLog(raw, 60)))
	}
	return overlays, log.String()
}

// --- Helpers (Copied from backup, simplified where needed) ---

type capturedIO struct {
	w         *os.File
	oldStdout *os.File
	oldStderr *os.File
	outChan   chan string
	debug     *bytes.Buffer
}

func finishReport(path string, content string, cio *capturedIO) {
	cio.w.Close()
	os.Stdout = cio.oldStdout
	os.Stderr = cio.oldStderr
	<-cio.outChan
	dir := filepath.Dir(path)
	os.MkdirAll(dir, 0755)
	os.WriteFile(path, []byte(content), 0644)
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func clipForLog(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}

func escapeForLog(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\n", "\\n")
	s = strings.ReplaceAll(s, "\t", "\\t")
	return s
}

func clamp(v, minV, maxV float64) float64 {
	if v < minV {
		return minV
	}
	if v > maxV {
		return maxV
	}
	return v
}

func groupIntoLines(elems []gopdf.TextElementInfo) []textLine {
	// Simplified bucketing
	type lineBucket struct {
		y        float64
		minX     float64
		maxX     float64
		fontSize float64
		elems    []gopdf.TextElementInfo
	}

	sorted := make([]gopdf.TextElementInfo, 0, len(elems))
	for _, e := range elems {
		if strings.TrimSpace(e.Text) == "" {
			continue
		}
		sorted = append(sorted, e)
	}
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Y == sorted[j].Y {
			return sorted[i].X < sorted[j].X
		}
		return sorted[i].Y < sorted[j].Y
	})

	var buckets []lineBucket
	for _, e := range sorted {
		placed := false
		for bi := range buckets {
			tol := buckets[bi].fontSize * 0.5 // Generous Y tolerance
			if tol < 2 {
				tol = 2
			}

			if abs(e.Y-buckets[bi].y) <= tol {
				// Horizontal check? For now, just bucket by line Y
				buckets[bi].elems = append(buckets[bi].elems, e)
				if e.X < buckets[bi].minX {
					buckets[bi].minX = e.X
				}
				if e.X+e.Width > buckets[bi].maxX {
					buckets[bi].maxX = e.X + e.Width
				}
				if e.FontSize > buckets[bi].fontSize {
					buckets[bi].fontSize = e.FontSize
				}
				// Update Y weighted avg? Or just keep first?
				// buckets[bi].y = (buckets[bi].y + e.Y) / 2 // Simple avg
				placed = true
				break
			}
		}
		if !placed {
			buckets = append(buckets, lineBucket{
				y:        e.Y,
				minX:     e.X,
				maxX:     e.X + e.Width,
				fontSize: e.FontSize,
				elems:    []gopdf.TextElementInfo{e},
			})
		}
	}

	out := make([]textLine, 0, len(buckets))
	for _, b := range buckets {
		sort.Slice(b.elems, func(i, j int) bool { return b.elems[i].X < b.elems[j].X })
		var sb strings.Builder
		prevEnd := 0.0
		prevSet := false
		prevLast := rune(0)
		prevHasLast := false
		isWordRune := func(r rune) bool {
			if r == '_' {
				return true
			}
			return unicode.IsLetter(r) || unicode.IsDigit(r)
		}
		needsSpaceAfterRune := func(prev rune, next rune) bool {
			if !isWordRune(next) {
				return false
			}
			switch prev {
			case ':', ';', ',', ')', ']', '}':
				return true
			default:
				return false
			}
		}
		fontSet := make(map[string]bool)
		var fonts []string
		hasToUnicode := 0
		isIdentity := 0
		cidCount := 0
		replCount := 0
		toUnicodeHit := 0
		glyphNameHit := 0
		identityASCIIHit := 0
		for _, e := range b.elems {
			text := e.Text
			if text == "" {
				continue
			}
			firstRune := rune(0)
			hasFirstRune := false
			for _, r := range text {
				firstRune = r
				hasFirstRune = true
				break
			}

			if prevSet {
				gap := e.X - prevEnd
				needSpace := false
				if gap > math.Max(1.5, b.fontSize*0.25) {
					needSpace = true
				} else if prevHasLast && hasFirstRune && needsSpaceAfterRune(prevLast, firstRune) && gap >= -0.5 {
					needSpace = true
				}
				if needSpace {
					sb.WriteByte(' ')
				}
			}

			sb.WriteString(e.Text)
			prevEnd = e.X + e.Width
			prevSet = true
			for _, r := range text {
				prevLast = r
				prevHasLast = true
			}
			if !fontSet[e.FontName] {
				fontSet[e.FontName] = true
				fonts = append(fonts, e.FontName)
			}
			if e.HasToUnicode {
				hasToUnicode++
			}
			if e.IsIdentity {
				isIdentity++
			}
			cidCount += e.CIDCount
			replCount += e.ReplacementCount
			toUnicodeHit += e.ToUnicodeHit
			glyphNameHit += e.GlyphNameHit
			identityASCIIHit += e.IdentityASCIIHit
		}
		out = append(out, textLine{
			Y:                b.y,
			MinX:             b.minX,
			MaxX:             b.maxX,
			FontSize:         b.fontSize,
			Text:             normalizeLineTextForReport(sb.String()),
			Elements:         b.elems,
			ElemCount:        len(b.elems),
			FontNames:        fonts,
			HasToUnicode:     hasToUnicode,
			IsIdentity:       isIdentity,
			CIDCount:         cidCount,
			ReplacementCount: replCount,
			ToUnicodeHit:     toUnicodeHit,
			GlyphNameHit:     glyphNameHit,
			IdentityASCIIHit: identityASCIIHit,
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Y < out[j].Y })
	return out
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func classifyLineKind(ln textLine, raw string, titleThr float64) string {
	if raw == "" {
		return "empty"
	}
	if isSpecialLine(raw) {
		return "special"
	}
	if looksLikeCodeLine(ln, raw) {
		return "code"
	}
	if isTitleLine(ln, titleThr) {
		return "title"
	}
	return "body"
}

func shouldTranslateLineKind(kind, raw string) bool {
	if kind == "code" || kind == "special" || kind == "empty" {
		return false
	}
	return true
}

func detectTitleFontThreshold(lines []textLine) float64 {
	var fs []float64
	for _, ln := range lines {
		if ln.FontSize > 0 {
			fs = append(fs, ln.FontSize)
		}
	}
	if len(fs) == 0 {
		return 14
	}
	sort.Float64s(fs)
	median := fs[len(fs)/2]
	thr := median * 1.4
	if thr < 14 {
		thr = 14
	}
	return thr
}

func isTitleLine(ln textLine, thr float64) bool {
	return ln.FontSize >= thr
}

// --- Text Content Helpers ---

func englishToZhPlaceholder(s string) string {
	// Simplified placeholder logic
	var b strings.Builder
	inWord := false
	wordLen := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			inWord = true
			wordLen++
		} else {
			if inWord {
				b.WriteString("译")
				if wordLen > 4 {
					b.WriteString("译")
				}
				inWord = false
				wordLen = 0
			}
			b.WriteRune(r)
		}
	}
	if inWord {
		b.WriteString("译")
		if wordLen > 4 {
			b.WriteString("译")
		}
	}
	return b.String()
}

func containsEnglishLetters(s string) bool {
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			return true
		}
	}
	return false
}

func isSpecialLine(s string) bool {
	if strings.Contains(s, "```") {
		return true
	}
	// Add more heuristics if needed
	return false
}

func looksLikeCodeLine(ln textLine, raw string) bool {
	if strings.HasPrefix(raw, "    ") || strings.HasPrefix(raw, "\t") {
		return true
	}
	for _, f := range ln.FontNames {
		l := strings.ToLower(f)
		if strings.Contains(l, "mono") || strings.Contains(l, "courier") {
			return true
		}
	}
	return false
}

func wrapTextToWidth(s string, fontSize, maxWidth float64) []string {
	if maxWidth <= 0 {
		return []string{s}
	}
	var lines []string
	runes := []rune(s)
	start := 0
	width := 0.0
	for i, r := range runes {
		w := fontSize // Simplified mono-width assumption for CJK
		if r < 256 {
			w = fontSize * 0.5
		}
		if width+w > maxWidth {
			lines = append(lines, string(runes[start:i]))
			start = i
			width = 0
		}
		width += w
	}
	if start < len(runes) {
		lines = append(lines, string(runes[start:]))
	}
	return lines
}
