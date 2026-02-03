package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"

	"github.com/novvoo/go-pdf/pkg/gopdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	pdffont "github.com/pdfcpu/pdfcpu/pkg/font"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
	xdraw "golang.org/x/image/draw"
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

type flowLine struct {
	Text           string
	FontSize       float64
	Translate      bool
	Special        bool
	ParagraphBreak bool
}

type contentShift struct {
	Needles []string
	Dy      float64
}

type pageEditState struct {
	contentRef       types.IndirectRef
	baseContent      []byte
	headContent      []byte
	baseShift        float64
	movedOutBottom   float64
	movedInBottom    float64
	originalBottomY  float64
	translationShift float64
	width            float64
	height           float64
}

func main() {
	pdfPath := "example/test_vector.pdf"
	reportPath := "example/render_vector_translated.txt"
	outTranslatedPSPath := "example/test_vector_translated.ps"
	outTranslatedSVGPath := "example/test_vector_translated.svg"
	outTranslatedPDFPath := "example/test_vector_translated.pdf"

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
	report.WriteString("PDF Translation Report\n")
	report.WriteString("======================\n")
	report.WriteString(fmt.Sprintf("Time: %s\n\n", time.Now().Format("2006-01-02 15:04:05")))
	report.WriteString(fmt.Sprintf("📄 Input PDF: %s\n", pdfPath))
	report.WriteString(fmt.Sprintf("📄 Output PS (base): %s\n", outTranslatedPSPath))
	report.WriteString(fmt.Sprintf("📄 Output SVG (editable, with translations): %s\n", outTranslatedSVGPath))
	report.WriteString(fmt.Sprintf("📄 Output PDF (from SVG): %s\n", outTranslatedPDFPath))
	report.WriteString("\n")

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
			finishReport(reportPath, report.String(), &capturedIO{w: w, oldStdout: oldStdout, oldStderr: oldStderr, outChan: outputChan})
			return
		}

		textElements, _ := reader.ExtractPageElements(pageNum)
		lines := groupIntoLines(textElements)
		report.WriteString(fmt.Sprintf("\nPage %d:\n", pageNum))
		report.WriteString(fmt.Sprintf("✅ Extracted %d text elements -> %d lines\n", len(textElements), len(lines)))
		report.WriteString(extractionDiagnostics(textElements, lines))
		report.WriteString(docCountReportForSource(lines))

		pageVectorOverlays, vecLog := buildTranslationOverlaysOnSource(pageNum, lines, pageInfo.Width, pageInfo.Height, chineseSansFont, chineseSerifFont)
		vectorOverlays = append(vectorOverlays, pageVectorOverlays...)
		report.WriteString(vecLog)
	}

	basePSPath := "example/test_vector.ps"
	_ = os.Remove(basePSPath)
	if err := reader.WritePostScriptBestEffort(basePSPath, dpi); err != nil {
		report.WriteString(fmt.Sprintf("❌ Failed to write base PostScript: %v\n", err))
		finishReport(reportPath, report.String(), &capturedIO{w: w, oldStdout: oldStdout, oldStderr: oldStderr, outChan: outputChan})
		return
	}

	if b, err := os.ReadFile(basePSPath); err == nil {
		_ = os.Remove(outTranslatedPSPath)
		_ = os.WriteFile(outTranslatedPSPath, b, 0644)
	}

	report.WriteString(fmt.Sprintf("\n✅ Base PS: %s\n", basePSPath))
	report.WriteString(fmt.Sprintf("✅ Output PS (base copy): %s\n", outTranslatedPSPath))
	report.WriteString(fmt.Sprintf("✅ Vector translated overlays: %d\n", len(vectorOverlays)))

	_ = os.Remove(outTranslatedSVGPath)
	_ = os.Remove(outTranslatedPDFPath)

	tmpBaseSVG, err := os.CreateTemp("", "gopdf_base_ps_*.svg")
	if err != nil {
		report.WriteString(fmt.Sprintf("❌ Failed to create temp base SVG: %v\n", err))
		finishReport(reportPath, report.String(), &capturedIO{w: w, oldStdout: oldStdout, oldStderr: oldStderr, outChan: outputChan, debug: &debugBuf})
		return
	}
	tmpBaseSVGPath := tmpBaseSVG.Name()
	tmpBaseSVG.Close()
	defer os.Remove(tmpBaseSVGPath)

	if err := gopdf.ConvertPostScriptToSVG(outTranslatedPSPath, tmpBaseSVGPath); err != nil {
		report.WriteString(fmt.Sprintf("❌ Failed to convert PS to base SVG: %v\n", err))
		finishReport(reportPath, report.String(), &capturedIO{w: w, oldStdout: oldStdout, oldStderr: oldStderr, outChan: outputChan, debug: &debugBuf})
		return
	}
	if err := gopdf.InsertTextOverlaysIntoSVG(tmpBaseSVGPath, outTranslatedSVGPath, vectorOverlays); err != nil {
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

func normalizeToSize(img image.Image, targetW, targetH int) image.Image {
	if img == nil {
		return img
	}
	if targetW <= 0 || targetH <= 0 {
		return img
	}
	b := img.Bounds()
	if b.Dx() == targetW && b.Dy() == targetH {
		return img
	}
	dst := image.NewRGBA(image.Rect(0, 0, targetW, targetH))
	draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)
	xdraw.CatmullRom.Scale(dst, dst.Bounds(), img, b, xdraw.Over, nil)
	return dst
}

type reflowEvent struct {
	origYpx int
	dyPx    int
}

type translationPlan struct {
	baseOrigYpx   int
	xPt           float64
	fontSizePt    float64
	lineOffsetsPx []int
	lines         []string
}

func planTranslationsForPage(pageNum int, lines []textLine, pageWidth, pageHeight float64, chineseFont string, scale float64) ([]translationPlan, []reflowEvent) {
	leftMargin := 6.0
	rightMargin := 6.0

	pageHpx := int(math.Round(pageHeight * scale))
	pageOffsetPx := (pageNum - 1) * pageHpx

	var plans []translationPlan
	var events []reflowEvent

	for _, ln := range lines {
		raw := strings.TrimSpace(ln.Text)
		if raw == "" {
			continue
		}
		if !containsEnglishLetters(raw) || isSpecialLine(raw) {
			continue
		}

		baseFontSize := ln.FontSize
		if baseFontSize <= 0 {
			baseFontSize = 12
		}
		if baseFontSize < 6 {
			baseFontSize = 6
		}
		baseLineHeight := baseFontSize * 1.20

		x := clamp(ln.MinX, leftMargin, pageWidth-rightMargin)
		lineWidth := ln.MaxX - ln.MinX
		if lineWidth < 80 {
			lineWidth = 200
		}
		targetWidth := clamp(lineWidth, 120, pageWidth-rightMargin-x)

		ch := englishToZhPlaceholder(raw)
		zhFontSize := baseFontSize * 0.92
		if zhFontSize > baseFontSize {
			zhFontSize = baseFontSize
		}
		if zhFontSize < 6 {
			zhFontSize = 6
		}
		zhLineHeight := zhFontSize * 1.25
		zhLines := wrapTextToWidth(ch, zhFontSize, targetWidth)
		if len(zhLines) == 0 {
			zhLines = []string{strings.TrimSpace(ch)}
		}

		yAfter := ln.Y + baseLineHeight*0.95
		baseOrigYpx := pageOffsetPx + int(math.Round(yAfter*scale))

		dyPt := float64(len(zhLines))*zhLineHeight + zhLineHeight*0.35
		dyPx := int(math.Round(dyPt * scale))
		if dyPx < 1 {
			dyPx = 1
		}

		events = append(events, reflowEvent{origYpx: baseOrigYpx, dyPx: dyPx})

		lineOffsetsPx := make([]int, 0, len(zhLines))
		for i := range zhLines {
			lineOffsetsPx = append(lineOffsetsPx, int(math.Round(float64(i)*zhLineHeight*scale)))
		}

		plans = append(plans, translationPlan{
			baseOrigYpx:   baseOrigYpx,
			xPt:           x,
			fontSizePt:    zhFontSize,
			lineOffsetsPx: lineOffsetsPx,
			lines:         append([]string(nil), zhLines...),
		})
	}

	_ = chineseFont
	return plans, events
}

func normalizeReflowEvents(events []reflowEvent) []reflowEvent {
	var out []reflowEvent
	for _, e := range events {
		if e.dyPx <= 0 {
			continue
		}
		out = append(out, e)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].origYpx == out[j].origYpx {
			return out[i].dyPx < out[j].dyPx
		}
		return out[i].origYpx < out[j].origYpx
	})
	merged := make([]reflowEvent, 0, len(out))
	for _, e := range out {
		if len(merged) == 0 || merged[len(merged)-1].origYpx != e.origYpx {
			merged = append(merged, e)
			continue
		}
		merged[len(merged)-1].dyPx += e.dyPx
	}
	return merged
}

type reflowSegment struct {
	origStart int
	origEnd   int
	shift     int
}

func buildReflowedDocument(origImages []image.Image, pageWidth, pageHeight float64, scale float64, events []reflowEvent, plans []translationPlan, chineseFont string) ([]image.Image, []gopdf.TextOverlayTopLeft, string) {
	var log strings.Builder
	if len(origImages) == 0 {
		return nil, nil, ""
	}

	b := origImages[0].Bounds()
	pageWpx := b.Dx()
	pageHpx := b.Dy()
	origTapeH := len(origImages) * pageHpx

	filteredEvents := make([]reflowEvent, 0, len(events))
	for _, e := range events {
		if e.origYpx <= 0 || e.origYpx >= origTapeH {
			continue
		}
		filteredEvents = append(filteredEvents, e)
	}
	filteredEvents = normalizeReflowEvents(filteredEvents)

	totalDy := 0
	for _, e := range filteredEvents {
		totalDy += e.dyPx
	}
	finalTapeH := origTapeH + totalDy
	outPageCount := int(math.Ceil(float64(finalTapeH) / float64(pageHpx)))
	if outPageCount < 1 {
		outPageCount = 1
	}

	log.WriteString("\nReflow (raster-stable):\n")
	log.WriteString(fmt.Sprintf("- dpi=%.0f scale=%.4f\n", 72.0*scale, scale))
	log.WriteString(fmt.Sprintf("- origPages=%d outPages=%d pagePx=%dx%d totalInsertPx=%d\n", len(origImages), outPageCount, pageWpx, pageHpx, totalDy))

	var segs []reflowSegment
	prev := 0
	shift := 0
	for _, e := range filteredEvents {
		if e.origYpx < prev {
			continue
		}
		segs = append(segs, reflowSegment{origStart: prev, origEnd: e.origYpx, shift: shift})
		shift += e.dyPx
		prev = e.origYpx
	}
	segs = append(segs, reflowSegment{origStart: prev, origEnd: origTapeH, shift: shift})

	outImages := make([]image.Image, 0, outPageCount)
	for p := 0; p < outPageCount; p++ {
		dst := image.NewRGBA(image.Rect(0, 0, pageWpx, pageHpx))
		draw.Draw(dst, dst.Bounds(), &image.Uniform{C: color.White}, image.Point{}, draw.Src)

		pageStart := p * pageHpx
		pageEnd := pageStart + pageHpx
		for _, s := range segs {
			finalStart := s.origStart + s.shift
			finalEnd := s.origEnd + s.shift
			if finalEnd <= pageStart || finalStart >= pageEnd {
				continue
			}
			startFinal := finalStart
			if startFinal < pageStart {
				startFinal = pageStart
			}
			endFinal := finalEnd
			if endFinal > pageEnd {
				endFinal = pageEnd
			}
			startOrig := s.origStart + (startFinal - finalStart)
			endOrig := startOrig + (endFinal - startFinal)
			copyOrigTapeStrip(dst, origImages, startOrig, endOrig, startFinal-pageStart)
		}
		outImages = append(outImages, dst)
	}

	sort.Slice(plans, func(i, j int) bool { return plans[i].baseOrigYpx < plans[j].baseOrigYpx })
	var overlays []gopdf.TextOverlayTopLeft
	ei := 0
	cumDyBefore := 0
	for _, p := range plans {
		for ei < len(filteredEvents) && filteredEvents[ei].origYpx < p.baseOrigYpx {
			cumDyBefore += filteredEvents[ei].dyPx
			ei++
		}
		baseFinalY := p.baseOrigYpx + cumDyBefore
		for i, s := range p.lines {
			finalY := baseFinalY + p.lineOffsetsPx[i]
			if finalY < 0 {
				continue
			}
			page := finalY/pageHpx + 1
			if page < 1 || page > outPageCount {
				continue
			}
			yInPagePt := float64(finalY%pageHpx) / scale
			overlays = append(overlays, gopdf.TextOverlayTopLeft{
				Page:      page,
				Text:      s,
				X:         p.xPt,
				Y:         yInPagePt,
				FontName:  chineseFont,
				FontSize:  p.fontSizePt,
				FillColor: "0.70 0.00 0.00",
				Opacity:   1,
				OnTop:     true,
			})
		}
	}

	_ = pageWidth
	_ = pageHeight
	return outImages, overlays, log.String()
}

func copyOrigTapeStrip(dst *image.RGBA, pages []image.Image, origStart, origEnd, dstY int) {
	if origEnd <= origStart {
		return
	}
	if len(pages) == 0 {
		return
	}
	pageH := pages[0].Bounds().Dy()
	pageW := pages[0].Bounds().Dx()

	y := origStart
	for y < origEnd {
		pageIndex := y / pageH
		if pageIndex < 0 || pageIndex >= len(pages) {
			return
		}
		yInPage := y - pageIndex*pageH
		chunk := origEnd - y
		if chunk > pageH-yInPage {
			chunk = pageH - yInPage
		}

		src := pages[pageIndex]
		r := image.Rect(0, yInPage, pageW, yInPage+chunk)
		draw.Draw(dst, image.Rect(0, dstY+(y-origStart), pageW, dstY+(y-origStart)+chunk), src, r.Min, draw.Src)

		y += chunk
	}
}

func writePNGPages(dir string, pages []image.Image) ([]string, error) {
	var files []string
	for i, img := range pages {
		fn := filepath.Join(dir, fmt.Sprintf("page_%04d.png", i+1))
		f, err := os.Create(fn)
		if err != nil {
			return nil, err
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			return nil, err
		}
		if err := f.Close(); err != nil {
			return nil, err
		}
		files = append(files, fn)
	}
	return files, nil
}

func estimateContentBottom(lines []textLine) float64 {
	maxBottom := 0.0
	for _, ln := range lines {
		fs := ln.FontSize
		if fs <= 0 {
			fs = 10
		}
		h := fs * 1.2
		b := ln.Y + h
		if b > maxBottom {
			maxBottom = b
		}
	}
	return maxBottom
}

func condensePageContentsToSingleStream(ctx *model.Context, pageNum int) (types.IndirectRef, []byte, error) {
	pageDict, pageRef, _, err := ctx.PageDict(pageNum, false)
	if err != nil {
		return types.IndirectRef{}, nil, err
	}
	contentsObj, found := pageDict.Find("Contents")
	if !found {
		return types.IndirectRef{}, nil, fmt.Errorf("page %d missing Contents", pageNum)
	}
	streamRefs, err := contentStreamRefs(ctx, contentsObj)
	if err != nil {
		return types.IndirectRef{}, nil, err
	}
	if len(streamRefs) == 0 {
		return types.IndirectRef{}, nil, fmt.Errorf("page %d has no content streams", pageNum)
	}

	var buf bytes.Buffer
	for _, ir := range streamRefs {
		obj, err := ctx.Dereference(ir)
		if err != nil {
			return types.IndirectRef{}, nil, err
		}
		sd, ok := obj.(types.StreamDict)
		if !ok {
			return types.IndirectRef{}, nil, fmt.Errorf("page %d content is not stream dict: %T", pageNum, obj)
		}
		if err := sd.Decode(); err != nil {
			return types.IndirectRef{}, nil, err
		}
		buf.Write(sd.Content)
		buf.WriteByte('\n')
	}

	sd, err := ctx.XRefTable.NewStreamDictForBuf(buf.Bytes())
	if err != nil {
		return types.IndirectRef{}, nil, err
	}
	if err := sd.Encode(); err != nil {
		return types.IndirectRef{}, nil, err
	}
	ir, err := ctx.XRefTable.IndRefForNewObject(*sd)
	if err != nil {
		return types.IndirectRef{}, nil, err
	}

	pageDict.Update("Contents", *ir)
	if pageRef != nil {
		entry := ctx.Table[int(pageRef.ObjectNumber)]
		if entry != nil {
			entry.Object = pageDict
		}
	}

	return *ir, buf.Bytes(), nil
}

func writePageContentStream(ctx *model.Context, contentRef types.IndirectRef, content []byte) error {
	entry := ctx.Table[int(contentRef.ObjectNumber)]
	if entry == nil {
		return fmt.Errorf("missing content stream obj: %s", contentRef.PDFString())
	}
	sd, ok := entry.Object.(types.StreamDict)
	if !ok {
		return fmt.Errorf("unexpected content stream type: %T", entry.Object)
	}
	sd.Content = content
	if err := sd.Encode(); err != nil {
		return err
	}
	entry.Object = sd
	return nil
}

func buildFinalPageContent(p pageEditState) []byte {
	var buf bytes.Buffer
	if len(p.headContent) > 0 {
		buf.Write(p.headContent)
		if p.headContent[len(p.headContent)-1] != '\n' {
			buf.WriteByte('\n')
		}
	}
	if p.baseShift != 0 {
		buf.WriteString(fmt.Sprintf("q 1 0 0 1 0 -%.4f cm\n", p.baseShift))
		buf.Write(p.baseContent)
		if len(p.baseContent) > 0 && p.baseContent[len(p.baseContent)-1] != '\n' {
			buf.WriteByte('\n')
		}
		buf.WriteString("Q\n")
	} else {
		buf.Write(p.baseContent)
	}
	return buf.Bytes()
}

func flattenOverlays(overlaysByPage [][]gopdf.TextOverlayTopLeft, pages []pageEditState) []gopdf.TextOverlayTopLeft {
	var out []gopdf.TextOverlayTopLeft
	for pageNum := 1; pageNum < len(overlaysByPage); pageNum++ {
		shift := pages[pageNum].baseShift
		h := pages[pageNum].height
		for _, ov := range overlaysByPage[pageNum] {
			ov.Y += shift
			for ov.Page+1 < len(overlaysByPage) && h > 0 && ov.Y >= h {
				ov.Y -= h
				ov.Page++
				h = pages[ov.Page].height
			}
			if ov.Page > 0 && ov.Page < len(overlaysByPage) {
				out = append(out, ov)
			}
		}
	}
	return out
}

func pageEffectiveBottom(p pageEditState) float64 {
	return p.originalBottomY + p.translationShift + p.baseShift - p.movedOutBottom + p.movedInBottom
}

func balancePages(ctx *model.Context, pages *[]pageEditState, overlaysByPage *[][]gopdf.TextOverlayTopLeft) string {
	const bottomMargin = 6.0
	const slackThreshold = 18.0
	const maxPull = 220.0

	var log strings.Builder
	log.WriteString("\nPage balancing:\n")

	ps := *pages
	for pageNum := 1; pageNum < len(ps); pageNum++ {
		p := &ps[pageNum]
		if p.height <= bottomMargin {
			continue
		}
		target := p.height - bottomMargin
		bottom := pageEffectiveBottom(*p)
		overflow := bottom - target
		if overflow > 0.5 {
			p.movedOutBottom += overflow
			srcFull := buildFinalPageContent(*p)
			carry := overflow
			k := 1
			for carry > 0 {
				dstPageNum := pageNum + k
				for dstPageNum >= len(ps) {
					if err := appendBlankPage(ctx, pages, overlaysByPage, p.width, p.height); err != nil {
						log.WriteString(fmt.Sprintf("- page=%d overflow: failed to append blank page: %v\n", pageNum, err))
						*pages = ps
						return log.String()
					}
					ps = *pages
				}
				dst := &ps[dstPageNum]
				amt := carry
				maxBand := dst.height - bottomMargin
				if amt > maxBand {
					amt = maxBand
				}
				if amt <= 0 {
					break
				}
				clipY := dst.height - amt
				if clipY < 0 {
					clipY = 0
				}
				ty := p.height * float64(k)
				band := fmt.Sprintf("q 0 %.4f %.4f %.4f re W n 1 0 0 1 0 %.4f cm\n", clipY, dst.width, amt, ty)
				bandBytes := append([]byte(band), srcFull...)
				bandBytes = append(bandBytes, []byte("\nQ\n")...)
				dst.headContent = append(dst.headContent, bandBytes...)
				dst.baseShift += amt
				carry -= amt
				k++
			}
			log.WriteString(fmt.Sprintf("- page=%d overflow=%.2f -> pushed forward across %d page(s)\n", pageNum, overflow, k-1))
			continue
		}

		if pageNum+1 >= len(ps) {
			continue
		}
		next := &ps[pageNum+1]
		if next.height <= bottomMargin {
			continue
		}
		slack := target - bottom
		if slack > slackThreshold {
			amt := slack
			if amt > maxPull {
				amt = maxPull
			}
			if amt > target {
				amt = target
			}
			if amt <= 0 {
				continue
			}

			srcFull := buildFinalPageContent(*next)
			ty := -(next.height - amt)
			band := fmt.Sprintf("q 0 0 %.4f %.4f re W n 1 0 0 1 0 %.4f cm\n", p.width, amt, ty)
			bandBytes := append([]byte(band), srcFull...)
			bandBytes = append(bandBytes, []byte("\nQ\n")...)
			p.headContent = append(p.headContent, bandBytes...)
			p.movedInBottom += amt
			next.baseShift -= amt

			log.WriteString(fmt.Sprintf("- page=%d underflow=%.2f -> pulled %.2f from page=%d\n", pageNum, slack, amt, pageNum+1))
		}
	}

	*pages = ps
	return log.String()
}

func appendBlankPage(ctx *model.Context, pages *[]pageEditState, overlaysByPage *[][]gopdf.TextOverlayTopLeft, width, height float64) error {
	ps := *pages
	last := len(ps) - 1
	if last < 1 {
		return fmt.Errorf("no pages")
	}
	set := types.IntSet{last: true}
	dim := &types.Dim{Width: width, Height: height}
	if err := ctx.XRefTable.InsertBlankPages(set, dim, false); err != nil {
		return err
	}
	ctx.PageCount++
	newPageNum := last + 1
	ref, base, err := createEmptyPageContentStream(ctx, newPageNum)
	if err != nil {
		return err
	}
	ps = append(ps, pageEditState{
		contentRef:       ref,
		baseContent:      base,
		headContent:      nil,
		baseShift:        0,
		movedOutBottom:   0,
		movedInBottom:    0,
		originalBottomY:  0,
		translationShift: 0,
		width:            width,
		height:           height,
	})
	*pages = ps

	osl := *overlaysByPage
	osl = append(osl, nil)
	*overlaysByPage = osl
	return nil
}

func createEmptyPageContentStream(ctx *model.Context, pageNum int) (types.IndirectRef, []byte, error) {
	pageDict, pageRef, _, err := ctx.PageDict(pageNum, false)
	if err != nil {
		return types.IndirectRef{}, nil, err
	}
	sd, err := ctx.XRefTable.NewStreamDictForBuf([]byte{})
	if err != nil {
		return types.IndirectRef{}, nil, err
	}
	if err := sd.Encode(); err != nil {
		return types.IndirectRef{}, nil, err
	}
	ir, err := ctx.XRefTable.IndRefForNewObject(*sd)
	if err != nil {
		return types.IndirectRef{}, nil, err
	}
	pageDict.Update("Contents", *ir)
	if pageRef != nil {
		entry := ctx.Table[int(pageRef.ObjectNumber)]
		if entry != nil {
			entry.Object = pageDict
		}
	}
	return *ir, []byte{}, nil
}

type translationCandidate struct {
	LineIndex      int
	SourceY        float64
	X              float64
	TargetWidth    float64
	BaseFontSize   float64
	BaseLineHeight float64
	ZhFontSize     float64
	ZhLineHeight   float64
	ZhLines        []string
	Dy             float64
	Needles        []string
	Raw            string
}

type parsedContentStream struct {
	ref    types.IndirectRef
	sd     types.StreamDict
	ops    []gopdf.ContentOp
	delta  int
	opFrom int
}

func applyTranslationEditsToPage(ctx *model.Context, pageNum int, lines []textLine, pageWidth, pageHeight float64, chineseFont string) ([]gopdf.TextOverlayTopLeft, int, int, float64, string, error) {
	leftMargin := 6.0
	rightMargin := 6.0

	pageDict, _, _, err := ctx.PageDict(pageNum, false)
	if err != nil {
		return nil, 0, 0, 0, "", err
	}
	contentsObj, found := pageDict.Find("Contents")
	if !found {
		return nil, 0, 0, 0, "", nil
	}
	streamRefs, err := contentStreamRefs(ctx, contentsObj)
	if err != nil {
		return nil, 0, 0, 0, "", err
	}
	if len(streamRefs) == 0 {
		return nil, 0, 0, 0, "", nil
	}

	streams := make([]parsedContentStream, 0, len(streamRefs))
	for _, ir := range streamRefs {
		obj, err := ctx.Dereference(ir)
		if err != nil {
			return nil, 0, 0, 0, "", err
		}
		sd, ok := obj.(types.StreamDict)
		if !ok {
			return nil, 0, 0, 0, "", fmt.Errorf("unexpected contents object type: %T", obj)
		}
		if err := sd.Decode(); err != nil {
			return nil, 0, 0, 0, "", err
		}
		toks, err := gopdf.TokenizeContentStreamWithOffsets(sd.Content)
		if err != nil {
			return nil, 0, 0, 0, "", err
		}
		ops, err := gopdf.ParseContentOps(toks)
		if err != nil {
			return nil, 0, 0, 0, "", err
		}
		streams = append(streams, parsedContentStream{ref: ir, sd: sd, ops: ops})
	}

	var candidates []translationCandidate
	for i, ln := range lines {
		raw := strings.TrimSpace(ln.Text)
		if raw == "" {
			continue
		}
		if !containsEnglishLetters(raw) || isSpecialLine(raw) {
			continue
		}

		baseFontSize := ln.FontSize
		if baseFontSize <= 0 {
			baseFontSize = 12
		}
		if baseFontSize < 6 {
			baseFontSize = 6
		}
		baseLineHeight := baseFontSize * 1.20

		x := clamp(ln.MinX, leftMargin, pageWidth-rightMargin)
		lineWidth := ln.MaxX - ln.MinX
		if lineWidth < 80 {
			lineWidth = 200
		}
		targetWidth := clamp(lineWidth, 120, pageWidth-rightMargin-x)

		ch := englishToZhPlaceholder(raw)
		zhFontSize := baseFontSize * 0.92
		if zhFontSize > baseFontSize {
			zhFontSize = baseFontSize
		}
		if zhFontSize < 6 {
			zhFontSize = 6
		}
		zhLineHeight := zhFontSize * 1.25
		zhLines := wrapTextToWidth(ch, zhFontSize, targetWidth)
		if len(zhLines) == 0 {
			zhLines = []string{strings.TrimSpace(ch)}
		}
		dy := float64(len(zhLines))*zhLineHeight + zhLineHeight*0.35

		candidates = append(candidates, translationCandidate{
			LineIndex:      i,
			SourceY:        ln.Y,
			X:              x,
			TargetWidth:    targetWidth,
			BaseFontSize:   baseFontSize,
			BaseLineHeight: baseLineHeight,
			ZhFontSize:     zhFontSize,
			ZhLineHeight:   zhLineHeight,
			ZhLines:        zhLines,
			Dy:             dy,
			Needles:        shiftNeedlesForLine(ln),
			Raw:            raw,
		})
	}

	var log strings.Builder
	log.WriteString("\nTranslation edit decisions:\n")
	log.WriteString("- Output PDF keeps original page content and shifts subsequent content to make room.\n")
	log.WriteString("- Only lines with a resolvable content-stream anchor are edited.\n")

	applied := 0
	planned := len(candidates)
	openQ := 0
	curStream := 0
	shiftY := 0.0

	var overlays []gopdf.TextOverlayTopLeft

	for _, c := range candidates {
		anchorStream, anchorPos, ok := findAnchorByOps(streams, curStream, c.Needles)
		if !ok {
			log.WriteString(fmt.Sprintf("- page=%d line[%03d] skip (no anchor) y=%.2f raw=%q\n",
				pageNum, c.LineIndex, c.SourceY, clipForLog(c.Raw, 120)))
			continue
		}

		insert := []byte(fmt.Sprintf("\nq 1 0 0 1 0 -%.4f cm\n", c.Dy))
		streams[anchorStream].sd.Content = insertAt(streams[anchorStream].sd.Content, anchorPos, insert)
		streams[anchorStream].delta += len(insert)
		curStream = anchorStream
		openQ++
		applied++

		finalY := c.SourceY + shiftY
		startY := finalY + c.BaseLineHeight*0.95
		for li, s := range c.ZhLines {
			yy := startY + float64(li)*c.ZhLineHeight
			overlays = append(overlays, gopdf.TextOverlayTopLeft{
				Page:      pageNum,
				Text:      s,
				X:         c.X,
				Y:         yy,
				FontName:  chineseFont,
				FontSize:  c.ZhFontSize,
				FillColor: "0.70 0.00 0.00",
				Opacity:   1,
				OnTop:     true,
			})
		}

		log.WriteString(fmt.Sprintf("- page=%d line[%03d] apply stream=%d y=%.2f->%.2f dy=%.2f zhlines=%d raw=%q\n",
			pageNum, c.LineIndex, anchorStream, c.SourceY, finalY, c.Dy, len(c.ZhLines), clipForLog(c.Raw, 120)))

		shiftY += c.Dy
	}

	if openQ > 0 {
		last := len(streams) - 1
		streams[last].sd.Content = append(streams[last].sd.Content, []byte("\n"+strings.Repeat("Q\n", openQ))...)
	}

	for i := range streams {
		sd := streams[i].sd
		if err := sd.Encode(); err != nil {
			return nil, applied, planned, shiftY, log.String(), err
		}
		entry := ctx.Table[int(streams[i].ref.ObjectNumber)]
		if entry != nil {
			entry.Object = sd
		}
	}

	return overlays, applied, planned, shiftY, log.String(), nil
}

func findAnchorByOps(streams []parsedContentStream, curStream int, needles []string) (int, int, bool) {
	for si := curStream; si < len(streams); si++ {
		start := 0
		if si == curStream {
			start = streams[si].opFrom
		}
		ops := streams[si].ops
		for oi := start; oi < len(ops); oi++ {
			op := ops[oi]
			if !isTextShowOp(op.Name) {
				continue
			}
			if !opMatchesNeedles(op, needles) {
				continue
			}
			streams[si].opFrom = oi + 1
			pos := op.End + streams[si].delta
			return si, pos, true
		}
	}
	return 0, 0, false
}

func isTextShowOp(name string) bool {
	switch name {
	case "Tj", "TJ", "'", "\"":
		return true
	default:
		return false
	}
}

func opMatchesNeedles(op gopdf.ContentOp, needles []string) bool {
	operands := extractTextOperands(op.Args)
	for _, o := range operands {
		for _, n := range needles {
			if matchTextOperand(o, n) {
				return true
			}
		}
	}
	return false
}

func extractTextOperands(args []interface{}) []string {
	var out []string
	var walk func(v interface{})
	walk = func(v interface{}) {
		switch t := v.(type) {
		case string:
			out = append(out, t)
		case []interface{}:
			for _, it := range t {
				walk(it)
			}
		}
	}
	for _, a := range args {
		walk(a)
	}
	return out
}

func matchTextOperand(operand, needle string) bool {
	operand = strings.TrimSpace(operand)
	needle = strings.TrimSpace(needle)
	if operand == "" || needle == "" {
		return false
	}
	if strings.HasPrefix(needle, "(") && strings.HasSuffix(needle, ")") {
		needle = strings.TrimSpace(needle[1 : len(needle)-1])
	}
	if strings.HasPrefix(operand, "(") && strings.HasSuffix(operand, ")") {
		operand = strings.TrimSpace(operand[1 : len(operand)-1])
	}

	opIsHex := strings.HasPrefix(operand, "<") && strings.HasSuffix(operand, ">")
	ndIsHex := strings.HasPrefix(needle, "<") && strings.HasSuffix(needle, ">")

	if opIsHex || ndIsHex {
		opN := normalizeHexLike(operand)
		ndN := normalizeHexLike(needle)
		if opN == "" || ndN == "" {
			return false
		}
		if len(ndN) >= 8 && strings.Contains(opN, ndN) {
			return true
		}
		return opN == ndN
	}

	if operand == needle {
		return true
	}
	if len([]rune(needle)) >= 12 && strings.Contains(operand, needle) {
		return true
	}
	return false
}

func normalizeHexLike(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		s = s[1 : len(s)-1]
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			continue
		}
		if c >= 'a' && c <= 'f' {
			c -= 32
		}
		out = append(out, c)
	}
	return string(out)
}

func buildTranslationEdits(pageNum int, lines []textLine, pageWidth, pageHeight float64, chineseFont string) ([]gopdf.TextOverlayTopLeft, []contentShift, string) {
	leftMargin := 6.0
	rightMargin := 6.0
	bottomMargin := 6.0

	var log strings.Builder
	log.WriteString("\nTranslation edit decisions:\n")
	log.WriteString("- Output PDF keeps original page content and shifts subsequent content to make room.\n")
	log.WriteString("- Translated text is added as overlays in the freed vertical space.\n")

	var overlays []gopdf.TextOverlayTopLeft
	var shifts []contentShift

	shiftY := 0.0
	for i, ln := range lines {
		raw := strings.TrimSpace(ln.Text)
		if raw == "" {
			continue
		}
		if !containsEnglishLetters(raw) || isSpecialLine(raw) {
			continue
		}

		baseFontSize := ln.FontSize
		if baseFontSize <= 0 {
			baseFontSize = 12
		}
		if baseFontSize < 6 {
			baseFontSize = 6
		}
		baseLineHeight := baseFontSize * 1.20

		x := clamp(ln.MinX, leftMargin, pageWidth-rightMargin)
		lineWidth := ln.MaxX - ln.MinX
		if lineWidth < 80 {
			lineWidth = 200
		}
		targetWidth := clamp(lineWidth, 120, pageWidth-rightMargin-x)

		ch := englishToZhPlaceholder(raw)
		chFontSize := baseFontSize * 0.92
		if chFontSize > baseFontSize {
			chFontSize = baseFontSize
		}
		if chFontSize < 6 {
			chFontSize = 6
		}
		chLineHeight := chFontSize * 1.25
		chLines := wrapTextToWidth(ch, chFontSize, targetWidth)
		if len(chLines) == 0 {
			chLines = []string{strings.TrimSpace(ch)}
		}

		finalY := ln.Y + shiftY
		startY := finalY + baseLineHeight*0.95
		for li, s := range chLines {
			yy := startY + float64(li)*chLineHeight
			if yy > pageHeight-bottomMargin {
				break
			}
			overlays = append(overlays, gopdf.TextOverlayTopLeft{
				Page:      pageNum,
				Text:      s,
				X:         x,
				Y:         yy,
				FontName:  chineseFont,
				FontSize:  chFontSize,
				FillColor: "0.70 0.00 0.00",
				Opacity:   1,
				OnTop:     true,
			})
		}

		dy := float64(len(chLines))*chLineHeight + chLineHeight*0.35
		shifts = append(shifts, contentShift{
			Needles: shiftNeedlesForLine(ln),
			Dy:      dy,
		})
		log.WriteString(fmt.Sprintf("- page=%d line[%03d] x=%.2f y=%.2f->%.2f w=%.2f fs=%.2f zhfs=%.2f zhlines=%d dy=%.2f raw=%q\n",
			pageNum, i, x, ln.Y, finalY, targetWidth, baseFontSize, chFontSize, len(chLines), dy, clipForLog(raw, 120)))

		shiftY += dy
	}

	return overlays, shifts, log.String()
}

func shiftNeedlesForLine(ln textLine) []string {
	seen := map[string]bool{}
	var out []string

	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		if seen[s] {
			return
		}
		seen[s] = true
		out = append(out, s)
	}

	for _, e := range ln.Elements {
		for _, n := range explodeRawNeedles(e.RawText) {
			add(n)
		}
		add(e.Text)
	}
	add(ln.Text)

	for _, s := range append([]string{}, out...) {
		if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
			continue
		}
		if !strings.HasPrefix(s, "<") {
			add("(" + s + ")")
		}
	}

	pruned := make([]string, 0, len(out))
	for _, s := range out {
		if isTrivialNeedle(s) {
			continue
		}
		pruned = append(pruned, s)
	}
	sort.Slice(pruned, func(i, j int) bool {
		return needleScore(pruned[i]) > needleScore(pruned[j])
	})
	return pruned
}

func explodeRawNeedles(s string) []string {
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	if strings.Contains(s, "<") && strings.Contains(s, ">") && strings.Count(s, "<") > 1 {
		var out []string
		start := -1
		for i := 0; i < len(s); i++ {
			if s[i] == '<' {
				start = i
				continue
			}
			if s[i] == '>' && start >= 0 {
				out = append(out, s[start:i+1])
				start = -1
			}
		}
		if len(out) > 0 {
			return out
		}
	}
	return []string{s}
}

func needleScore(s string) int {
	s = strings.TrimSpace(s)
	score := len(s)
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		score += 1000
	}
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		score += 500
	}
	return score
}

func isTrivialNeedle(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return true
	}
	if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
		s = strings.TrimSpace(s[1 : len(s)-1])
	}
	if strings.HasPrefix(s, "<") && strings.HasSuffix(s, ">") {
		return len(s) < 6
	}
	hasAlphaNum := false
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			hasAlphaNum = true
			break
		}
	}
	if !hasAlphaNum {
		return true
	}
	return len([]rune(s)) < 2
}

func applyContentShifts(ctx *model.Context, pageNum int, shifts []contentShift) error {
	if len(shifts) == 0 {
		return nil
	}

	pageDict, _, _, err := ctx.PageDict(pageNum, false)
	if err != nil {
		return err
	}
	contentsObj, found := pageDict.Find("Contents")
	if !found {
		return nil
	}

	streamRefs, err := contentStreamRefs(ctx, contentsObj)
	if err != nil {
		return err
	}
	if len(streamRefs) == 0 {
		return nil
	}

	opIndex := 0
	openQ := 0

	for _, ir := range streamRefs {
		obj, err := ctx.Dereference(ir)
		if err != nil {
			return err
		}
		sd, ok := obj.(types.StreamDict)
		if !ok {
			continue
		}
		if err := sd.Decode(); err != nil {
			return err
		}

		content := sd.Content
		lastPos := 0
		for opIndex < len(shifts) {
			insPos, ok := findInsertionPos(content, shifts[opIndex].Needles, lastPos)
			if !ok {
				break
			}

			insert := []byte(fmt.Sprintf("\nq 1 0 0 1 0 -%.4f cm\n", shifts[opIndex].Dy))
			content = insertAt(content, insPos, insert)
			lastPos = insPos + len(insert)
			openQ++
			opIndex++
		}

		sd.Content = content
		if err := sd.Encode(); err != nil {
			return err
		}
		entry := ctx.Table[int(ir.ObjectNumber)]
		if entry != nil {
			entry.Object = sd
		}
	}

	if opIndex < len(shifts) {
		return fmt.Errorf("content match failed: applied=%d planned=%d", opIndex, len(shifts))
	}

	if openQ > 0 {
		lastRef := streamRefs[len(streamRefs)-1]
		entry := ctx.Table[int(lastRef.ObjectNumber)]
		if entry == nil {
			return nil
		}
		sd, ok := entry.Object.(types.StreamDict)
		if !ok {
			return nil
		}
		if err := sd.Decode(); err != nil {
			return err
		}
		sd.Content = append(sd.Content, []byte("\n"+strings.Repeat("Q\n", openQ))...)
		if err := sd.Encode(); err != nil {
			return err
		}
		entry.Object = sd
	}

	return nil
}

func contentStreamRefs(ctx *model.Context, contents types.Object) ([]types.IndirectRef, error) {
	switch obj := contents.(type) {
	case types.IndirectRef:
		derefObj, err := ctx.Dereference(obj)
		if err != nil {
			return nil, err
		}
		switch v := derefObj.(type) {
		case types.StreamDict:
			return []types.IndirectRef{obj}, nil
		case types.Array:
			var refs []types.IndirectRef
			for _, it := range v {
				if ir, ok := it.(types.IndirectRef); ok {
					refs = append(refs, ir)
				}
			}
			return refs, nil
		default:
			return nil, nil
		}
	case types.Array:
		var refs []types.IndirectRef
		for _, it := range obj {
			if ir, ok := it.(types.IndirectRef); ok {
				refs = append(refs, ir)
			}
		}
		return refs, nil
	case types.StreamDict:
		return nil, fmt.Errorf("page contents stream is not indirect")
	default:
		return nil, nil
	}
}

func findInsertionPos(content []byte, needles []string, from int) (int, bool) {
	for _, n := range needles {
		n = strings.TrimSpace(n)
		if n == "" {
			continue
		}
		pos := -1
		matchEnd := -1
		if strings.HasPrefix(n, "<") && strings.HasSuffix(n, ">") {
			p, end, ok := findHexNeedleMatch(content, n, from)
			if !ok {
				continue
			}
			pos = p
			matchEnd = end
		} else {
			p := bytes.Index(content[from:], []byte(n))
			if p < 0 {
				continue
			}
			pos = p + from
			matchEnd = pos + len(n)
		}
		searchStart := matchEnd
		searchEnd := searchStart + 512
		if searchEnd > len(content) {
			searchEnd = len(content)
		}
		tail := content[searchStart:searchEnd]
		opRel := -1
		opLen := 0
		candidates := [][]byte{[]byte("Tj"), []byte("TJ"), []byte("'"), []byte("\"")}
		for _, c := range candidates {
			p := bytes.Index(tail, c)
			if p < 0 {
				continue
			}
			if opRel < 0 || p < opRel {
				opRel = p
				opLen = len(c)
			}
		}
		if opRel < 0 {
			return searchStart, true
		}
		opEnd := searchStart + opRel + opLen
		etEnd := opEnd
		etSearchEnd := opEnd + 1024
		if etSearchEnd > len(content) {
			etSearchEnd = len(content)
		}
		etTail := content[opEnd:etSearchEnd]
		etPos := bytes.Index(etTail, []byte("ET"))
		if etPos >= 0 {
			etEnd = opEnd + etPos + 2
		}
		return etEnd, true
	}
	return 0, false
}

func findHexNeedleMatch(content []byte, needle string, from int) (int, int, bool) {
	digits := make([]byte, 0, len(needle))
	for i := 0; i < len(needle); i++ {
		c := needle[i]
		if c == '<' || c == '>' || c == ' ' || c == '\t' || c == '\r' || c == '\n' {
			continue
		}
		digits = append(digits, c)
	}
	if len(digits) == 0 {
		return 0, 0, false
	}

	for i := from; i < len(content); i++ {
		if content[i] != '<' {
			continue
		}
		j := i + 1
		k := 0
		for k < len(digits) {
			for j < len(content) && (content[j] == ' ' || content[j] == '\t' || content[j] == '\r' || content[j] == '\n') {
				j++
			}
			if j >= len(content) || content[j] == '>' {
				break
			}
			c1 := content[j]
			c2 := digits[k]
			if c1 >= 'a' && c1 <= 'f' {
				c1 -= 32
			}
			if c2 >= 'a' && c2 <= 'f' {
				c2 -= 32
			}
			if c1 != c2 {
				break
			}
			j++
			k++
		}
		if k != len(digits) {
			continue
		}
		for j < len(content) && (content[j] == ' ' || content[j] == '\t' || content[j] == '\r' || content[j] == '\n') {
			j++
		}
		if j < len(content) && content[j] == '>' {
			j++
		}
		return i, j, true
	}
	return 0, 0, false
}

func insertAt(b []byte, pos int, insert []byte) []byte {
	if pos < 0 {
		pos = 0
	}
	if pos > len(b) {
		pos = len(b)
	}
	out := make([]byte, 0, len(b)+len(insert))
	out = append(out, b[:pos]...)
	out = append(out, insert...)
	out = append(out, b[pos:]...)
	return out
}

func buildTranslationOverlaysOnSource(pageNum int, lines []textLine, pageWidth, pageHeight float64, chineseSansFont, chineseSerifFont string) ([]gopdf.TextOverlayTopLeft, string) {
	leftMargin := 6.0
	rightMargin := 6.0
	topMargin := 6.0
	bottomMargin := 6.0

	var log strings.Builder
	log.WriteString("\nTranslation overlay decisions:\n")
	log.WriteString("- Output PDF keeps original page content; only adds translated text overlays.\n")
	log.WriteString("- Placement: prefer below the original line if there is vertical room; supports wrapping.\n")
	log.WriteString("- Fallback: if below doesn't fit, try right side; otherwise overlay.\n")

	titleThr := detectTitleFontThreshold(lines)

	var overlays []gopdf.TextOverlayTopLeft
	for i, ln := range lines {
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

		raw := strings.TrimSpace(ln.Text)
		if raw == "" {
			continue
		}

		kind := classifyLineKind(ln, raw, titleThr)
		translate := shouldTranslateLineKind(kind, raw)
		if !translate {
			log.WriteString(fmt.Sprintf("- page=%d line[%03d] kind=%s translate=no fs=%.2f fonts=%q raw=%q\n",
				pageNum, i, kind, ln.FontSize, strings.Join(ln.FontNames, ","), clipForLog(raw, 140)))
			continue
		}
		if !containsEnglishLetters(raw) || isSpecialLine(raw) {
			log.WriteString(fmt.Sprintf("- page=%d line[%03d] kind=%s translate=no fs=%.2f fonts=%q raw=%q\n",
				pageNum, i, kind, ln.FontSize, strings.Join(ln.FontNames, ","), clipForLog(raw, 140)))
			continue
		}

		baseFontSize := ln.FontSize
		if baseFontSize <= 0 {
			baseFontSize = 12
		}
		if baseFontSize < 6 {
			baseFontSize = 6
		}
		baseLineHeight := baseFontSize * 1.20

		nextY := pageHeight - bottomMargin
		if i+1 < len(lines) && lines[i+1].Y > ln.Y {
			nextY = lines[i+1].Y
		}

		ch := englishToZhPlaceholder(raw)
		opacity := 1.0
		fill := "0 0 0"

		minX := ln.MinX
		maxX := ln.MaxX
		if maxX <= minX {
			maxX = minX + 120
		}

		belowY := ln.Y + baseLineHeight*0.95
		belowAvailableH := nextY - belowY - 1
		if belowAvailableH < 0 {
			belowAvailableH = 0
		}

		rightX := ln.MaxX + 6
		rightFits := rightX <= pageWidth-rightMargin-30

		var x, y, targetWidth float64
		var wrapped []string
		var fontSize float64
		placement := ""

		tryBelow := func() bool {
			x = clamp(minX, leftMargin, pageWidth-rightMargin)
			y = clamp(belowY, topMargin, pageHeight-bottomMargin)
			targetWidth = clamp(maxX-minX, 100, pageWidth-rightMargin-x)
			maxFont := baseFontSize
			if maxFont < 6 {
				maxFont = 6
			}
			for fs := maxFont; fs >= 6; fs -= 0.5 {
				lh := fs * 1.15
				lines := wrapTextToWidth(ch, fs, targetWidth)
				if len(lines) == 0 {
					continue
				}
				needH := float64(len(lines)) * lh
				if needH <= belowAvailableH && y+needH <= pageHeight-bottomMargin {
					placement = "below"
					fontSize = fs
					wrapped = lines
					return true
				}
			}
			return false
		}

		tryRight := func() bool {
			x = clamp(rightX, leftMargin, pageWidth-rightMargin)
			y = clamp(ln.Y, topMargin, pageHeight-bottomMargin)
			targetWidth = clamp(pageWidth-rightMargin-x, 120, pageWidth-rightMargin-x)
			placement = "right"
			fontSize = float64(pdffont.Size(ch, chineseFont, targetWidth))
			if fontSize <= 0 {
				fontSize = baseFontSize * 0.85
			}
			if fontSize > baseFontSize {
				fontSize = baseFontSize
			}
			if fontSize < 6 {
				fontSize = 6
			}
			wrapped = []string{strings.TrimSpace(ch)}
			return true
		}

		tryOverlay := func() {
			x = clamp(minX, leftMargin, pageWidth-rightMargin)
			y = clamp(ln.Y, topMargin, pageHeight-bottomMargin)
			targetWidth = clamp(maxX-minX, 100, pageWidth-rightMargin-x)
			placement = "overlay"
			fontSize = float64(pdffont.Size(ch, chineseFont, targetWidth))
			if fontSize <= 0 {
				fontSize = baseFontSize * 0.80
			}
			if fontSize > baseFontSize {
				fontSize = baseFontSize
			}
			if fontSize < 6 {
				fontSize = 6
			}
			wrapped = wrapTextToWidth(ch, fontSize, targetWidth)
			if len(wrapped) == 0 {
				wrapped = []string{strings.TrimSpace(ch)}
			}
		}

		if !tryBelow() {
			if !rightFits || !tryRight() {
				tryOverlay()
			}
		}

		lh := fontSize * 1.15
		for li, s := range wrapped {
			yy := y + float64(li)*lh
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
				FillColor: fill,
				Opacity:   opacity,
				OnTop:     true,
			})
		}

		log.WriteString(fmt.Sprintf("- page=%d line[%03d] kind=%s translate=yes place=%s x=%.2f y=%.2f w=%.2f fs=%.2f->%.2f wrap=%d cjkFont=%s raw=%q\n",
			pageNum, i, kind, placement, x, y, targetWidth, baseFontSize, fontSize, len(wrapped), chineseFont, clipForLog(raw, 120)))
	}

	return overlays, log.String()
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
	if looksLikeLatexOrMathLine(ln, raw) {
		return "latex"
	}
	if containsIPARunes(raw) {
		return "phonetic"
	}
	if isTitleLine(ln, titleThr) && looksLikeTitleText(raw) {
		return "title"
	}
	return "body"
}

func shouldTranslateLineKind(kind string, raw string) bool {
	switch kind {
	case "code", "latex", "phonetic", "special", "empty":
		return false
	default:
		return true
	}
}

func looksLikeTitleText(raw string) bool {
	rs := []rune(strings.TrimSpace(raw))
	if len(rs) == 0 {
		return false
	}
	if len(rs) > 120 {
		return false
	}
	if strings.HasSuffix(raw, ".") || strings.HasSuffix(raw, ":") {
		return true
	}
	punct := 0
	for _, r := range rs {
		if strings.ContainsRune("()[]{}<>;=+-/*\\|", r) {
			punct++
		}
	}
	return punct*6 < len(rs)
}

func looksLikeCodeLine(ln textLine, raw string) bool {
	if strings.HasPrefix(raw, "\t") || strings.HasPrefix(raw, "    ") {
		return true
	}
	for _, fn := range ln.FontNames {
		l := strings.ToLower(fn)
		if strings.Contains(l, "courier") || strings.Contains(l, "consolas") || strings.Contains(l, "mono") ||
			strings.Contains(l, "menlo") || strings.Contains(l, "monaco") || strings.Contains(l, "sourcecode") ||
			strings.Contains(l, "fira") {
			return true
		}
	}
	hints := []string{"{", "}", "==", "!=", ":=", "::", "->", "=>", "#include", "func ", "var ", "let ", "const ", "package ", "import ", "return "}
	for _, h := range hints {
		if strings.Contains(raw, h) {
			return true
		}
	}
	punct := 0
	letterOrDigit := 0
	for _, r := range raw {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			letterOrDigit++
		} else if strings.ContainsRune("[]{}()<>,.;:+-*/=\\|&_#%$^!~", r) {
			punct++
		}
	}
	if letterOrDigit == 0 {
		return false
	}
	return punct*3 > letterOrDigit
}

func looksLikeLatexOrMathLine(ln textLine, raw string) bool {
	for _, fn := range ln.FontNames {
		l := strings.ToLower(fn)
		if strings.Contains(l, "cmr") || strings.Contains(l, "cmmi") || strings.Contains(l, "cmsy") ||
			strings.Contains(l, "cmmib") || strings.Contains(l, "math") || strings.Contains(l, "symbol") {
			return true
		}
	}
	if strings.Contains(raw, "$") || strings.Contains(raw, "\\(") || strings.Contains(raw, "\\)") || strings.Contains(raw, "\\[") || strings.Contains(raw, "\\]") {
		return true
	}
	latexCmdHints := []string{"\\frac", "\\sum", "\\int", "\\sqrt", "\\left", "\\right", "\\begin", "\\end", "\\mathrm", "\\mathbf", "\\math", "\\alpha", "\\beta", "\\gamma"}
	for _, h := range latexCmdHints {
		if strings.Contains(raw, h) {
			return true
		}
	}
	for _, r := range raw {
		if strings.ContainsRune("∑∫≤≥≈≠×÷√∞→←↔∀∃", r) {
			return true
		}
		if (r >= 0x0370 && r <= 0x03FF) || (r >= 0x1D400 && r <= 0x1D7FF) {
			return true
		}
	}
	return false
}

func containsIPARunes(raw string) bool {
	for _, r := range raw {
		if (r >= 0x0250 && r <= 0x02AF) || (r >= 0x1D00 && r <= 0x1D7F) {
			return true
		}
		if strings.ContainsRune("ˈˌːˑ˘˙˞ˠˤ̩̯̃", r) {
			return true
		}
	}
	return false
}

func wrapTextToWidth(s string, fontSize, maxWidth float64) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	if maxWidth <= 0 {
		return []string{strings.TrimSpace(s)}
	}
	rs := []rune(s)
	var lines []string
	start := 0
	curW := 0.0
	lastBreak := -1
	lastBreakW := 0.0

	for i, r := range rs {
		w := runeWidthFactor(r) * fontSize
		if unicode.IsSpace(r) {
			lastBreak = i
			lastBreakW = curW + w
		}
		if curW+w > maxWidth && i > start {
			cut := i
			if lastBreak >= start {
				cut = lastBreak + 1
				curW = curW - lastBreakW
			} else {
				curW = 0
			}
			line := strings.TrimSpace(string(rs[start:cut]))
			if line != "" {
				lines = append(lines, line)
			}
			start = cut
			lastBreak = -1
			lastBreakW = 0
			continue
		}
		curW += w
	}
	if start < len(rs) {
		line := strings.TrimSpace(string(rs[start:]))
		if line != "" {
			lines = append(lines, line)
		}
	}
	return lines
}

func runeWidthFactor(r rune) float64 {
	if unicode.IsSpace(r) {
		return 0.33
	}
	if (r >= 0x4E00 && r <= 0x9FFF) ||
		(r >= 0x3400 && r <= 0x4DBF) ||
		(r >= 0x20000 && r <= 0x2A6DF) ||
		(r >= 0x2A700 && r <= 0x2B73F) ||
		(r >= 0x2B740 && r <= 0x2B81F) ||
		(r >= 0x2B820 && r <= 0x2CEAF) ||
		(r >= 0xF900 && r <= 0xFAFF) ||
		(r >= 0x2F800 && r <= 0x2FA1F) ||
		(r >= 0x3040 && r <= 0x309F) ||
		(r >= 0x30A0 && r <= 0x30FF) ||
		(r >= 0xAC00 && r <= 0xD7AF) {
		return 1.0
	}
	if r >= 0xFF00 && r <= 0xFFEF {
		return 1.0
	}
	return 0.55
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

func containsEnglishLetters(s string) bool {
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			return true
		}
	}
	return false
}

func englishToZhPlaceholder(s string) string {
	var b strings.Builder
	for _, r := range s {
		if (r >= 'A' && r <= 'Z') || (r >= 'a' && r <= 'z') {
			b.WriteRune('Z')
			continue
		}
		if unicode.IsSpace(r) {
			b.WriteRune(' ')
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isSpecialLine(s string) bool {
	t := strings.TrimSpace(s)
	if t == "" {
		return true
	}
	if strings.Contains(t, "\\") || strings.Contains(t, "$") || strings.Contains(t, "{") || strings.Contains(t, "}") ||
		strings.Contains(t, "^") || strings.Contains(t, "_") {
		return true
	}
	if strings.Contains(t, "```") {
		return true
	}
	codeLike := []string{"==", "!=", "->", "::", "=>", "<=", ">=", "++;", "--;", "{", "}", ";", "#include", "func ", "var ", "let ", "const ", "class "}
	for _, k := range codeLike {
		if strings.Contains(t, k) {
			return true
		}
	}
	phoneticLike := []rune{'ˈ', 'ˌ', 'ə', 'ɪ', 'ʊ', 'ʌ', 'ɑ', 'ɔ', 'ɛ', 'ɜ', 'θ', 'ð', 'ŋ'}
	for _, r := range phoneticLike {
		if strings.ContainsRune(t, r) {
			return true
		}
	}
	if strings.HasPrefix(t, "[") && strings.HasSuffix(t, "]") {
		return true
	}
	return false
}

func groupIntoLines(elems []gopdf.TextElementInfo) []textLine {
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
			tol := buckets[bi].fontSize * 0.35
			if tol < 2 {
				tol = 2
			}
			if tol > 6 {
				tol = 6
			}
			joinTol := buckets[bi].fontSize * 1.5
			if joinTol < 10 {
				joinTol = 10
			}
			overlap := e.X <= buckets[bi].maxX+joinTol && (e.X+e.Width) >= buckets[bi].minX-joinTol

			if abs(e.Y-buckets[bi].y) <= tol && overlap {
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
				buckets[bi].y = (buckets[bi].y*0.8 + e.Y*0.2)
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
		fontSet := map[string]struct{}{}
		var fontNames []string
		hasToUnicode := 0
		isIdentity := 0
		cidCount := 0
		replCount := 0
		toUnicodeHit := 0
		glyphNameHit := 0
		identityASCIIHit := 0
		for _, e := range b.elems {
			sb.WriteString(e.Text)
			if e.FontName != "" {
				if _, ok := fontSet[e.FontName]; !ok {
					fontSet[e.FontName] = struct{}{}
					fontNames = append(fontNames, e.FontName)
				}
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
			Text:             sb.String(),
			Elements:         b.elems,
			ElemCount:        len(b.elems),
			FontNames:        fontNames,
			HasToUnicode:     hasToUnicode,
			IsIdentity:       isIdentity,
			CIDCount:         cidCount,
			ReplacementCount: replCount,
			ToUnicodeHit:     toUnicodeHit,
			GlyphNameHit:     glyphNameHit,
			IdentityASCIIHit: identityASCIIHit,
		})
	}

	sort.Slice(out, func(i, j int) bool {
		if out[i].Y == out[j].Y {
			return out[i].MinX < out[j].MinX
		}
		return out[i].Y < out[j].Y
	})
	return out
}

func toFlowLines(lines []textLine) []flowLine {
	if len(lines) == 0 {
		return nil
	}
	out := make([]flowLine, 0, len(lines))
	var prev *textLine
	for i := range lines {
		ln := lines[i]
		raw := strings.TrimSpace(ln.Text)
		if raw == "" {
			continue
		}
		fs := ln.FontSize
		if fs <= 0 {
			fs = 12
		}
		special := isSpecialLine(raw)
		translate := containsEnglishLetters(raw) && !special
		pb := false
		if prev != nil {
			refFS := fs
			if prev.FontSize > refFS {
				refFS = prev.FontSize
			}
			if refFS <= 0 {
				refFS = 12
			}
			yGap := ln.Y - prev.Y
			if yGap > refFS*1.8 {
				pb = true
			}
			xGap := abs(ln.MinX - prev.MinX)
			if xGap > refFS*2.0 && yGap > refFS*0.8 {
				pb = true
			}
		}
		out = append(out, flowLine{
			Text:           raw,
			FontSize:       fs,
			Translate:      translate,
			Special:        special,
			ParagraphBreak: pb,
		})
		prev = &ln
	}
	return out
}

func buildBilingualLayoutReflow(lines []flowLine, pageWidth, pageHeight float64, chineseFont string) (bilingualLayout, string) {
	topMargin := 48.0
	bottomMargin := 48.0
	leftMargin := 36.0
	rightMargin := 36.0

	textWidth := pageWidth - leftMargin - rightMargin
	if textWidth < 120 {
		textWidth = pageWidth
	}
	maxY := pageHeight - bottomMargin
	if maxY < topMargin+24 {
		maxY = pageHeight
	}

	var log strings.Builder
	log.WriteString("\nReflow decisions:\n")
	log.WriteString("- Output PDF is regenerated from extracted text; paragraphs flow and paginate.\n")
	log.WriteString("- Wrapping uses heuristic width factors; font is approximated (Helvetica + chosen CJK font).\n")

	page := 1
	y := topMargin
	rows := make([]layoutRow, 0, len(lines)*3)

	ensureSpace := func(h float64) {
		if y+h > maxY {
			page++
			y = topMargin
		}
	}

	for i, ln := range lines {
		fs := ln.FontSize
		if fs <= 0 {
			fs = 12
		}
		if fs < 6 {
			fs = 6
		}
		lineHeight := fs * 1.35

		if ln.ParagraphBreak && y > topMargin {
			ensureSpace(lineHeight * 0.6)
			y += lineHeight * 0.6
		}

		origWrapped := wrapTextToWidth(ln.Text, fs, textWidth)
		if len(origWrapped) == 0 {
			origWrapped = []string{ln.Text}
		}
		for _, s := range origWrapped {
			ensureSpace(lineHeight)
			rows = append(rows, layoutRow{
				Page:     page,
				Text:     s,
				X:        leftMargin,
				Y:        y,
				FontName: "Helvetica",
				FontSize: fs,
			})
			y += lineHeight
		}

		if ln.Translate {
			ensureSpace(lineHeight * 0.15)
			y += lineHeight * 0.15

			ch := englishToZhPlaceholder(ln.Text)
			chFS := fs * 0.92
			if chFS < 6 {
				chFS = 6
			}
			chLH := chFS * 1.25

			chWrapped := wrapTextToWidth(ch, chFS, textWidth)
			if len(chWrapped) == 0 {
				chWrapped = []string{ch}
			}
			for _, s := range chWrapped {
				ensureSpace(chLH)
				rows = append(rows, layoutRow{
					Page:     page,
					Text:     s,
					X:        leftMargin,
					Y:        y,
					FontName: chineseFont,
					FontSize: chFS,
				})
				y += chLH
			}

			ensureSpace(chLH * 0.25)
			y += chLH * 0.25
		}

		if (i+1)%50 == 0 {
			log.WriteString(fmt.Sprintf("- processed %d/%d flow lines (page=%d)\n", i+1, len(lines), page))
		}
	}

	return bilingualLayout{
		Rows:      rows,
		RowCount:  len(rows),
		PageCount: page,
	}, log.String()
}

type fontDiag struct {
	Elements     int
	HasToUnicode int
	IsIdentity   int
	CIDCount     int
	Replaced     int
	ToUnicodeHit int
	GlyphHit     int
	IdentityHit  int
	Samples      []string
}

func extractionDiagnostics(elems []gopdf.TextElementInfo, lines []textLine) string {
	var b strings.Builder

	totalCID := 0
	totalRepl := 0
	totalToUni := 0
	totalGlyph := 0
	totalIdent := 0
	withToUnicode := 0
	withIdentity := 0
	fonts := map[string]*fontDiag{}

	for _, e := range elems {
		totalCID += e.CIDCount
		totalRepl += e.ReplacementCount
		totalToUni += e.ToUnicodeHit
		totalGlyph += e.GlyphNameHit
		totalIdent += e.IdentityASCIIHit
		if e.HasToUnicode {
			withToUnicode++
		}
		if e.IsIdentity {
			withIdentity++
		}
		fn := e.FontName
		if fn == "" {
			fn = "(empty)"
		}
		fd := fonts[fn]
		if fd == nil {
			fd = &fontDiag{}
			fonts[fn] = fd
		}
		fd.Elements++
		if e.HasToUnicode {
			fd.HasToUnicode++
		}
		if e.IsIdentity {
			fd.IsIdentity++
		}
		fd.CIDCount += e.CIDCount
		fd.Replaced += e.ReplacementCount
		fd.ToUnicodeHit += e.ToUnicodeHit
		fd.GlyphHit += e.GlyphNameHit
		fd.IdentityHit += e.IdentityASCIIHit
		if e.ReplacementCount > 0 && len(fd.Samples) < 3 {
			raw := strings.TrimSpace(e.RawText)
			if raw == "" {
				raw = strings.TrimSpace(e.Text)
			}
			fd.Samples = append(fd.Samples, clipForLog(raw, 80))
		}
	}

	fontNames := make([]string, 0, len(fonts))
	for k := range fonts {
		fontNames = append(fontNames, k)
	}
	sort.Strings(fontNames)

	b.WriteString("\nExtraction diagnostics:\n")
	b.WriteString(fmt.Sprintf("- Elements: %d\n", len(elems)))
	b.WriteString(fmt.Sprintf("- Unique fonts: %d\n", len(fontNames)))
	b.WriteString(fmt.Sprintf("- Elements with ToUnicode: %d\n", withToUnicode))
	b.WriteString(fmt.Sprintf("- Elements with Identity: %d\n", withIdentity))
	b.WriteString(fmt.Sprintf("- Total CID count: %d\n", totalCID))
	b.WriteString(fmt.Sprintf("- Total replacement (U+FFFD) inserted: %d\n", totalRepl))
	b.WriteString(fmt.Sprintf("- Decode hit breakdown: ToUnicode=%d GlyphName=%d IdentityASCII=%d\n", totalToUni, totalGlyph, totalIdent))

	b.WriteString("\nFonts breakdown:\n")
	for _, fn := range fontNames {
		fd := fonts[fn]
		b.WriteString(fmt.Sprintf("- %s: elems=%d toUnicode=%d identity=%d cids=%d repl=%d hits(TU=%d GN=%d ID=%d)",
			fn, fd.Elements, fd.HasToUnicode, fd.IsIdentity, fd.CIDCount, fd.Replaced, fd.ToUnicodeHit, fd.GlyphHit, fd.IdentityHit))
		if len(fd.Samples) > 0 {
			b.WriteString(fmt.Sprintf(" samples=%q", fd.Samples))
		}
		b.WriteString("\n")
	}

	linesByRepl := make([]textLine, 0, len(lines))
	for _, ln := range lines {
		linesByRepl = append(linesByRepl, ln)
	}
	sort.Slice(linesByRepl, func(i, j int) bool {
		if linesByRepl[i].ReplacementCount == linesByRepl[j].ReplacementCount {
			return (linesByRepl[i].MaxX - linesByRepl[i].MinX) > (linesByRepl[j].MaxX - linesByRepl[j].MinX)
		}
		return linesByRepl[i].ReplacementCount > linesByRepl[j].ReplacementCount
	})

	b.WriteString("\nWorst lines by replacement count (top 12):\n")
	limit := 12
	if len(linesByRepl) < limit {
		limit = len(linesByRepl)
	}
	for i := 0; i < limit; i++ {
		ln := linesByRepl[i]
		fonts := ln.FontNames
		sort.Strings(fonts)
		if len(fonts) > 6 {
			fonts = append(fonts[:6], "…")
		}
		b.WriteString(fmt.Sprintf("- y=%.2f x=%.2f..%.2f w=%.2f fs=%.2f elems=%d fonts=%v repl=%d cids=%d hits(TU=%d GN=%d ID=%d) text=%q\n",
			ln.Y, ln.MinX, ln.MaxX, ln.MaxX-ln.MinX, ln.FontSize, ln.ElemCount, fonts, ln.ReplacementCount, ln.CIDCount, ln.ToUnicodeHit, ln.GlyphNameHit, ln.IdentityASCIIHit, clipForLog(strings.TrimSpace(ln.Text), 140)))
		if ln.ReplacementCount > 0 {
			for ei := 0; ei < len(ln.Elements) && ei < 6; ei++ {
				e := ln.Elements[ei]
				b.WriteString(fmt.Sprintf("  elem[%d] x=%.2f w=%.2f fs=%.2f font=%q toUni=%v id=%v cids=%d repl=%d raw=%q text=%q\n",
					ei, e.X, e.Width, e.FontSize, e.FontName, e.HasToUnicode, e.IsIdentity, e.CIDCount, e.ReplacementCount, clipForLog(strings.TrimSpace(e.RawText), 80), clipForLog(strings.TrimSpace(e.Text), 80)))
			}
		}
	}

	return b.String()
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}

func pickChineseFont() string {
	_ = pdffont.LoadUserFonts()
	candidates := pdffont.UserFontNames()
	sort.Strings(candidates)
	for _, n := range candidates {
		ln := strings.ToLower(n)
		if strings.Contains(ln, "yahei") || strings.Contains(ln, "simsun") || strings.Contains(ln, "heiti") || strings.Contains(ln, "song") {
			return n
		}
	}
	for _, n := range candidates {
		ln := strings.ToLower(n)
		if strings.Contains(ln, "noto") || strings.Contains(ln, "sourcehan") {
			return n
		}
	}
	return ""
}

func pickChineseFonts() (sans string, serif string) {
	_ = pdffont.LoadUserFonts()
	candidates := pdffont.UserFontNames()
	sort.Strings(candidates)
	for _, n := range candidates {
		ln := strings.ToLower(n)
		if sans == "" && (strings.Contains(ln, "yahei") || strings.Contains(ln, "heiti") || strings.Contains(ln, "msyh")) {
			sans = n
		}
		if serif == "" && (strings.Contains(ln, "simsun") || strings.Contains(ln, "song")) {
			serif = n
		}
		if sans != "" && serif != "" {
			return sans, serif
		}
	}
	if sans == "" {
		sans = serif
	}
	if serif == "" {
		serif = sans
	}
	return sans, serif
}

type layoutRow struct {
	Page     int
	Text     string
	X        float64
	Y        float64
	FontName string
	FontSize float64
}

type bilingualLayout struct {
	Rows      []layoutRow
	RowCount  int
	PageCount int
}

func buildBilingualLayout(lines []textLine, pageWidth, pageHeight float64, chineseFont string) (bilingualLayout, string) {
	topMargin := 48.0
	bottomMargin := 48.0
	leftMargin := 36.0
	rightMargin := 36.0

	var log strings.Builder
	log.WriteString("\nLayout decisions:\n")
	log.WriteString("- Preserve extracted English text as-is; translation uses placeholders only.\n")
	log.WriteString("- Reading order: sort by Y then X from extracted lines.\n")
	titleThreshold := detectTitleFontThreshold(lines)
	log.WriteString(fmt.Sprintf("- Title font threshold: %.2f\n", titleThreshold))
	log.WriteString("\nPer-line mapping (input -> output rows):\n")

	page := 1
	y := topMargin
	rows := make([]layoutRow, 0, len(lines)*2)
	textWidth := pageWidth - leftMargin - rightMargin
	if textWidth < 100 {
		textWidth = pageWidth
	}
	maxY := pageHeight - bottomMargin
	if maxY < topMargin+24 {
		maxY = pageHeight
	}

	translated := 0
	special := 0
	replLines := 0
	replTotal := 0
	kept := 0

	for idx, ln := range lines {
		raw := ln.Text
		if strings.TrimSpace(raw) == "" {
			continue
		}
		kept++
		replTotal += ln.ReplacementCount
		if ln.ReplacementCount > 0 {
			replLines++
		}
		fontSize := ln.FontSize
		if fontSize <= 0 {
			fontSize = 12
		}
		lineHeight := fontSize * 1.35
		if y+lineHeight > maxY {
			page++
			y = topMargin
		}

		x := leftMargin
		targetWidth := textWidth

		rows = append(rows, layoutRow{
			Page:     page,
			Text:     raw,
			X:        x,
			Y:        y,
			FontName: "Helvetica",
			FontSize: fontSize,
		})

		isSpecial := isSpecialLine(raw)
		if isSpecial {
			special++
		}
		translate := containsEnglishLetters(raw) && !isSpecial
		if translate {
			ch := englishToZhPlaceholder(raw)
			chSize := float64(pdffont.Size(ch, chineseFont, targetWidth))
			if chSize <= 0 {
				chSize = fontSize
			}
			if chSize > fontSize {
				chSize = fontSize
			}
			chY := y + lineHeight
			if chY+lineHeight > maxY {
				page++
				y = topMargin
				chY = y + lineHeight
			}
			rows = append(rows, layoutRow{
				Page:     page,
				Text:     ch,
				X:        x,
				Y:        chY,
				FontName: chineseFont,
				FontSize: chSize,
			})
			translated++
			log.WriteString(fmt.Sprintf("[%03d] y=%.2f x=%.2f w=%.2f fs=%.2f elems=%d repl=%d cids=%d fonts=%v translate=yes special=no page=%d outY=%.2f/%0.2f raw=%q\n",
				idx, ln.Y, ln.MinX, ln.MaxX-ln.MinX, ln.FontSize, ln.ElemCount, ln.ReplacementCount, ln.CIDCount, clipFonts(ln.FontNames, 6), page, y, chY, clipForLog(raw, 140)))
			y = chY + lineHeight
		} else {
			log.WriteString(fmt.Sprintf("[%03d] y=%.2f x=%.2f w=%.2f fs=%.2f elems=%d repl=%d cids=%d fonts=%v translate=no special=%v page=%d outY=%.2f raw=%q\n",
				idx, ln.Y, ln.MinX, ln.MaxX-ln.MinX, ln.FontSize, ln.ElemCount, ln.ReplacementCount, ln.CIDCount, clipFonts(ln.FontNames, 6), isSpecial, page, y, clipForLog(raw, 140)))
			y += lineHeight
		}
	}

	return bilingualLayout{
			Rows:      rows,
			RowCount:  len(rows),
			PageCount: page,
		}, fmt.Sprintf("%s\nSummary:\n- Lines kept: %d\n- Lines with U+FFFD (replacement): %d\n- Total U+FFFD (replacement) count: %d\n- Lines flagged special (not translated): %d\n- Lines translated: %d\n",
			log.String(), kept, replLines, replTotal, special, translated)
}

func clipFonts(fonts []string, max int) []string {
	if len(fonts) == 0 {
		return nil
	}
	out := append([]string(nil), fonts...)
	sort.Strings(out)
	if max > 0 && len(out) > max {
		return append(out[:max], "…")
	}
	return out
}

type docTextCounts struct {
	TitleLines int
	BodyLines  int
	TitleChars int
	BodyChars  int
	Special    int
	Empty      int
}

func countNonSpaceRunes(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		n++
	}
	return n
}

func detectTitleFontThreshold(lines []textLine) float64 {
	var fs []float64
	for _, ln := range lines {
		t := strings.TrimSpace(ln.Text)
		if t == "" {
			continue
		}
		if isSpecialLine(t) {
			continue
		}
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
	if thr < median+3 {
		thr = median + 3
	}
	if thr < 14 {
		thr = 14
	}
	return thr
}

func isTitleLine(ln textLine, threshold float64) bool {
	if ln.FontSize <= 0 {
		return false
	}
	if ln.FontSize >= threshold {
		return true
	}
	return false
}

func countSourceDoc(lines []textLine) (docTextCounts, float64) {
	thr := detectTitleFontThreshold(lines)
	var c docTextCounts

	for _, ln := range lines {
		t := strings.TrimSpace(ln.Text)
		if t == "" {
			c.Empty++
			continue
		}
		if isSpecialLine(t) {
			c.Special++
			continue
		}
		chars := countNonSpaceRunes(t)
		if isTitleLine(ln, thr) {
			c.TitleLines++
			c.TitleChars += chars
		} else {
			c.BodyLines++
			c.BodyChars += chars
		}
	}
	return c, thr
}

func docCountReportForSource(lines []textLine) string {
	c, thr := countSourceDoc(lines)
	var b strings.Builder
	b.WriteString("\nText counts (source PDF extraction):\n")
	b.WriteString(fmt.Sprintf("- Title font threshold: %.2f\n", thr))
	b.WriteString(fmt.Sprintf("- Title: lines=%d chars=%d\n", c.TitleLines, c.TitleChars))
	b.WriteString(fmt.Sprintf("- Body:  lines=%d chars=%d\n", c.BodyLines, c.BodyChars))
	b.WriteString(fmt.Sprintf("- Excluded: specialLines=%d emptyLines=%d\n", c.Special, c.Empty))
	return b.String()
}

func docCountReportForOutput(srcLines []textLine, rows []layoutRow, chineseFont string) string {
	srcCounts, thr := countSourceDoc(srcLines)

	lineByText := map[string]textLine{}
	for _, ln := range srcLines {
		if strings.TrimSpace(ln.Text) == "" {
			continue
		}
		if _, ok := lineByText[ln.Text]; ok {
			continue
		}
		lineByText[ln.Text] = ln
	}

	var outOrig docTextCounts
	var outTrans docTextCounts

	for _, r := range rows {
		t := strings.TrimSpace(r.Text)
		if t == "" {
			continue
		}
		if r.FontName == chineseFont {
			ln, ok := lineByText[r.Text]
			if ok && isSpecialLine(strings.TrimSpace(ln.Text)) {
				continue
			}
			chars := countNonSpaceRunes(t)
			if ok && isTitleLine(ln, thr) {
				outTrans.TitleLines++
				outTrans.TitleChars += chars
			} else {
				outTrans.BodyLines++
				outTrans.BodyChars += chars
			}
			continue
		}

		ln, ok := lineByText[r.Text]
		if ok && isSpecialLine(strings.TrimSpace(ln.Text)) {
			continue
		}
		chars := countNonSpaceRunes(t)
		if ok && isTitleLine(ln, thr) {
			outOrig.TitleLines++
			outOrig.TitleChars += chars
		} else {
			outOrig.BodyLines++
			outOrig.BodyChars += chars
		}
	}

	var b strings.Builder
	b.WriteString("\nText counts (output PDF):\n")
	b.WriteString(fmt.Sprintf("- Title font threshold (from source): %.2f\n", thr))
	b.WriteString(fmt.Sprintf("- Output original: titleLines=%d titleChars=%d bodyLines=%d bodyChars=%d\n",
		outOrig.TitleLines, outOrig.TitleChars, outOrig.BodyLines, outOrig.BodyChars))
	b.WriteString(fmt.Sprintf("- Output translation: titleLines=%d titleChars=%d bodyLines=%d bodyChars=%d\n",
		outTrans.TitleLines, outTrans.TitleChars, outTrans.BodyLines, outTrans.BodyChars))
	b.WriteString(fmt.Sprintf("- Source reference: titleLines=%d titleChars=%d bodyLines=%d bodyChars=%d excludedSpecial=%d excludedEmpty=%d\n",
		srcCounts.TitleLines, srcCounts.TitleChars, srcCounts.BodyLines, srcCounts.BodyChars, srcCounts.Special, srcCounts.Empty))
	return b.String()
}

func createBlankPDF(outFile string, width, height float64, pageCount int) error {
	conf := model.NewDefaultConfiguration()
	conf.Unit = types.POINTS

	ctx, err := pdfcpu.CreateContextWithXRefTable(conf, &types.Dim{Width: width, Height: height})
	if err != nil {
		return err
	}

	rootDict, err := ctx.XRefTable.Catalog()
	if err != nil {
		return err
	}
	pagesObj, ok := rootDict.Find("Pages")
	if !ok {
		return fmt.Errorf("missing Pages in catalog")
	}
	pagesDict, err := ctx.XRefTable.DereferenceDict(pagesObj)
	if err != nil {
		return err
	}
	pagesIndRef, ok := pagesObj.(types.IndirectRef)
	if !ok {
		return fmt.Errorf("Pages is not an indirect ref")
	}

	if pageCount <= 0 {
		pageCount = 1
	}

	mediaBox := types.RectForDim(width, height)
	kids := types.Array{}
	for i := 0; i < pageCount; i++ {
		sd, _ := ctx.XRefTable.NewStreamDictForBuf(nil)
		if err := sd.Encode(); err != nil {
			return err
		}
		contentsRef, err := ctx.XRefTable.IndRefForNewObject(*sd)
		if err != nil {
			return err
		}

		pageDict := types.Dict(
			map[string]types.Object{
				"Type":      types.Name("Page"),
				"Parent":    pagesIndRef,
				"MediaBox":  mediaBox.Array(),
				"Resources": types.Dict{},
				"Contents":  *contentsRef,
			},
		)
		pageRef, err := ctx.XRefTable.IndRefForNewObject(pageDict)
		if err != nil {
			return err
		}
		kids = append(kids, *pageRef)
	}

	pagesDict["Kids"] = kids
	pagesDict["Count"] = types.Integer(pageCount)
	if entry := ctx.XRefTable.Table[pagesIndRef.ObjectNumber.Value()]; entry != nil {
		entry.Object = pagesDict
	}
	ctx.XRefTable.PageCount = pageCount

	return api.WriteContextFile(ctx, outFile)
}

func clipForLog(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	return string(r[:max]) + "…"
}
