package gopdf

import (
	"fmt"
	"math"
	"sort"
	"strconv"
	"strings"
)

type LayoutExtractOptions struct {
	ImagesByName   map[string]string
	RecurseForms   bool
	IncludeText    bool
	IncludeXObject bool
}

func (r *PDFReader) ExtractPageLayoutElements(pageNum int, opt LayoutExtractOptions) ([]LayoutElement, error) {
	if !opt.IncludeText && !opt.IncludeXObject {
		opt.IncludeText = true
		opt.IncludeXObject = true
	}
	var out []LayoutElement

	if opt.IncludeText {
		text, err := r.extractPageTextLayoutElements(pageNum)
		if err != nil {
			return nil, err
		}
		out = append(out, text...)
	}
	if opt.IncludeXObject {
		xobjs, err := r.extractPageXObjectLayoutElements(pageNum, opt.ImagesByName, opt.RecurseForms)
		if err != nil {
			return nil, err
		}
		out = append(out, xobjs...)
	}
	return out, nil
}

func (r *PDFReader) extractPageTextLayoutElements(pageNum int) ([]LayoutElement, error) {
	elems, _ := r.ExtractPageElements(pageNum)
	if len(elems) == 0 {
		return nil, nil
	}
	items := make([]TextElementInfo, 0, len(elems))
	for _, e := range elems {
		if e.Width == 0 && e.Height == 0 {
			continue
		}
		a := stripControlNul(e.Text)
		b := stripControlNul(e.RawText)
		if a == "" && b == "" {
			continue
		}
		items = append(items, e)
	}
	if len(items) == 0 {
		return nil, nil
	}

	sort.Slice(items, func(i, j int) bool {
		yi, yj := items[i].Y+items[i].Height, items[j].Y+items[j].Height
		if math.Abs(yi-yj) > 1 {
			return yi > yj
		}
		return items[i].X < items[j].X
	})

	seq := 1
	var out []LayoutElement
	type run struct {
		font     string
		fontBase string
		fontSize float64
		enc      string
		hasTU    bool
		isID     bool
		cidN     int
		repN     int
		tuHit    int
		glyphHit int
		idASCII  int
		y        float64
		b        LayoutBBox
		sb       []rune
	}
	var cur *run

	flush := func() {
		if cur == nil || len(cur.sb) == 0 {
			cur = nil
			return
		}
		raw := map[string]string{
			"textEncoding": cur.enc,
		}
		if cur.font != "" {
			raw["font"] = cur.font
		}
		if cur.fontBase != "" {
			raw["fontBase"] = cur.fontBase
		}
		if cur.fontSize > 0 {
			raw["fontSize"] = strconv.FormatFloat(cur.fontSize, 'f', -1, 64)
		}
		raw["hasToUnicode"] = strconv.FormatBool(cur.hasTU)
		raw["isIdentity"] = strconv.FormatBool(cur.isID)
		raw["cidCount"] = strconv.Itoa(cur.cidN)
		raw["replacementCount"] = strconv.Itoa(cur.repN)
		raw["toUnicodeHit"] = strconv.Itoa(cur.tuHit)
		raw["glyphNameHit"] = strconv.Itoa(cur.glyphHit)
		raw["identityASCIIHit"] = strconv.Itoa(cur.idASCII)
		el := LayoutElement{
			ID:     fmt.Sprintf("p%04d-t%04d", pageNum, seq),
			Page:   pageNum,
			Kind:   "text",
			Text:   string(cur.sb),
			BBox:   &LayoutBBox{MinX: cur.b.MinX, MinY: cur.b.MinY, MaxX: cur.b.MaxX, MaxY: cur.b.MaxY},
			Raw:    raw,
			Approx: false,
		}
		seq++
		out = append(out, el)
		cur = nil
	}

	for _, e := range items {
		s, enc := bestTextForElement(e)
		if s == "" {
			continue
		}
		b := LayoutBBox{MinX: e.X, MinY: e.Y, MaxX: e.X + e.Width, MaxY: e.Y + e.Height}
		y0 := b.MaxY

		if cur == nil {
			cur = &run{
				font:     e.FontName,
				fontBase: e.FontBaseName,
				fontSize: e.FontSize,
				enc:      enc,
				hasTU:    e.HasToUnicode,
				isID:     e.IsIdentity,
				cidN:     e.CIDCount,
				repN:     e.ReplacementCount,
				tuHit:    e.ToUnicodeHit,
				glyphHit: e.GlyphNameHit,
				idASCII:  e.IdentityASCIIHit,
				y:        y0,
				b:        b,
				sb:       []rune(s),
			}
			continue
		}

		sameLine := math.Abs(cur.y-y0) <= math.Max(1, e.FontSize*0.2)
		sameStyle := cur.font == e.FontName && cur.fontBase == e.FontBaseName && math.Abs(cur.fontSize-e.FontSize) <= 0.5
		gap := b.MinX - cur.b.MaxX
		joinable := sameLine && sameStyle && gap >= -0.5 && gap <= math.Max(2, e.FontSize*0.8)

		if !joinable {
			flush()
			cur = &run{
				font:     e.FontName,
				fontBase: e.FontBaseName,
				fontSize: e.FontSize,
				enc:      enc,
				hasTU:    e.HasToUnicode,
				isID:     e.IsIdentity,
				cidN:     e.CIDCount,
				repN:     e.ReplacementCount,
				tuHit:    e.ToUnicodeHit,
				glyphHit: e.GlyphNameHit,
				idASCII:  e.IdentityASCIIHit,
				y:        y0,
				b:        b,
				sb:       []rune(s),
			}
			continue
		}

		if gap > math.Max(1.5, e.FontSize*0.25) {
			cur.sb = append(cur.sb, ' ')
		}
		cur.sb = append(cur.sb, []rune(s)...)
		cur.b = unifyBBox(cur.b, b)
		cur.y = y0
	}
	flush()
	return out, nil
}

func unifyBBox(a, b LayoutBBox) LayoutBBox {
	if b.MinX < a.MinX {
		a.MinX = b.MinX
	}
	if b.MinY < a.MinY {
		a.MinY = b.MinY
	}
	if b.MaxX > a.MaxX {
		a.MaxX = b.MaxX
	}
	if b.MaxY > a.MaxY {
		a.MaxY = b.MaxY
	}
	return a
}

func bestTextForElement(e TextElementInfo) (text string, enc string) {
	a := stripControlNul(e.Text)
	b := stripControlNul(e.RawText)
	if a != "" && !looksLikeCIDHexDump(a) {
		return a, "go-pdf"
	}
	if b != "" && !looksLikeCIDHexDump(b) {
		return b, "go-pdf-raw"
	}
	if a != "" {
		return a, "go-pdf"
	}
	return b, "go-pdf-raw"
}

func stripControlNul(s string) string {
	if s == "" {
		return s
	}
	s = strings.ReplaceAll(s, "\x00", "")
	s = strings.Map(func(r rune) rune {
		if r == '\n' || r == '\t' {
			return r
		}
		if r < 0x20 {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	return s
}

func looksLikeCIDHexDump(s string) bool {
	if len(s) < 6 {
		return false
	}
	if !strings.Contains(s, "><") {
		return false
	}
	open := strings.Count(s, "<")
	close := strings.Count(s, ">")
	return open > 2 && open == close
}
