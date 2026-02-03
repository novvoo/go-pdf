package gopdf

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	pdfcpufont "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/font"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

type svgElemKind int

const (
	svgElemPath svgElemKind = iota
	svgElemText
)

type svgElem struct {
	Kind svgElemKind
	Path svgPathElem
	Text svgTextElem
}

type svgPathElem struct {
	D              string
	Fill           string
	Stroke         string
	StrokeWidth    float64
	StrokeLineCap  string
	StrokeLineJoin string
	StrokeMiter    float64
	StrokeDash     string
	StrokeDashOff  float64
}

type svgTextElem struct {
	X, Y     float64
	FontSize float64
	Fill     string
	Text     string
}

func ConvertSVGToPDF(svgPath, pdfPath string) error {
	if svgPath == "" || pdfPath == "" {
		return fmt.Errorf("missing svgPath or pdfPath")
	}

	data, err := os.ReadFile(svgPath)
	if err != nil {
		return err
	}

	doc, err := parseSVGForPDF(data)
	if err != nil {
		return err
	}
	if doc.PageCount <= 0 {
		return fmt.Errorf("missing page count in svg")
	}
	if doc.PageWidth <= 0 || doc.PageHeight <= 0 {
		return fmt.Errorf("invalid svg page size")
	}

	ctx, pageRefs, contentRefs, err := createBlankPDFContextForSVG(doc.PageWidth, doc.PageHeight, doc.PageCount)
	if err != nil {
		return err
	}

	fm := model.FontMap{
		"Helvetica": model.FontResource{Res: model.Resource{ID: "F1"}},
	}
	fontDict, err := pdfcpufont.FontResources(ctx.XRefTable, fm)
	if err != nil {
		return err
	}
	if err := attachFontResources(ctx, pageRefs, fontDict); err != nil {
		return err
	}

	for page := 1; page <= doc.PageCount; page++ {
		var buf bytes.Buffer
		buf.WriteString(fmt.Sprintf("q 1 0 0 -1 0 %.4f cm\n", doc.PageHeight))
		for _, e := range doc.Pages[page] {
			switch e.Kind {
			case svgElemPath:
				writePDFForSVGPath(&buf, e.Path)
			case svgElemText:
				writePDFForSVGText(&buf, e.Text)
			}
		}
		buf.WriteString("Q\n")
		if page-1 < len(contentRefs) {
			if err := writePageContentStream(ctx, contentRefs[page-1], buf.Bytes()); err != nil {
				return err
			}
		}
	}

	if err := ensureDirForFile(pdfPath); err != nil {
		return err
	}
	return api.WriteContextFile(ctx, pdfPath)
}

type svgDocForPDF struct {
	PageWidth  float64
	PageHeight float64
	PageCount  int
	Pages      map[int][]svgElem
}

func parseSVGForPDF(data []byte) (*svgDocForPDF, error) {
	dec := xml.NewDecoder(bytes.NewReader(data))
	doc := &svgDocForPDF{
		Pages: map[int][]svgElem{},
	}

	currentPage := 0
	var pageStack []int
	var currentText *svgTextElem

	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			switch t.Name.Local {
			case "svg":
				for _, a := range t.Attr {
					switch a.Name.Local {
					case "data-page-count":
						if v, err := strconv.Atoi(strings.TrimSpace(a.Value)); err == nil {
							doc.PageCount = v
						}
					case "data-page-width":
						doc.PageWidth = parseSVGFloat(a.Value)
					case "data-page-height":
						doc.PageHeight = parseSVGFloat(a.Value)
					case "width":
						if doc.PageWidth <= 0 {
							doc.PageWidth = parseSVGFloat(a.Value)
						}
					case "height":
						if doc.PageHeight <= 0 && doc.PageCount <= 1 {
							doc.PageHeight = parseSVGFloat(a.Value)
						}
					}
				}
			case "g":
				pageStack = append(pageStack, currentPage)
				for _, a := range t.Attr {
					if a.Name.Local == "data-page" {
						if v, err := strconv.Atoi(strings.TrimSpace(a.Value)); err == nil {
							currentPage = v
							if currentPage > doc.PageCount {
								doc.PageCount = currentPage
							}
						}
					}
				}
			case "path":
				if currentPage <= 0 {
					continue
				}
				p := svgPathElem{Fill: "none", Stroke: "none", StrokeWidth: 1, StrokeLineCap: "butt", StrokeLineJoin: "miter", StrokeMiter: 10}
				for _, a := range t.Attr {
					switch a.Name.Local {
					case "d":
						p.D = a.Value
					case "fill":
						p.Fill = a.Value
					case "stroke":
						p.Stroke = a.Value
					case "stroke-width":
						p.StrokeWidth = parseSVGFloat(a.Value)
					case "stroke-linecap":
						p.StrokeLineCap = a.Value
					case "stroke-linejoin":
						p.StrokeLineJoin = a.Value
					case "stroke-miterlimit":
						p.StrokeMiter = parseSVGFloat(a.Value)
					case "stroke-dasharray":
						p.StrokeDash = a.Value
					case "stroke-dashoffset":
						p.StrokeDashOff = parseSVGFloat(a.Value)
					}
				}
				doc.Pages[currentPage] = append(doc.Pages[currentPage], svgElem{Kind: svgElemPath, Path: p})
			case "rect":
				if currentPage <= 0 {
					continue
				}
				p := svgPathElem{Fill: "none", Stroke: "none", StrokeWidth: 1, StrokeLineCap: "butt", StrokeLineJoin: "miter", StrokeMiter: 10}
				var x, y, w, h float64
				for _, a := range t.Attr {
					switch a.Name.Local {
					case "x":
						x = parseSVGFloat(a.Value)
					case "y":
						y = parseSVGFloat(a.Value)
					case "width":
						w = parseSVGFloat(a.Value)
					case "height":
						h = parseSVGFloat(a.Value)
					case "fill":
						p.Fill = a.Value
					case "stroke":
						p.Stroke = a.Value
					case "stroke-width":
						p.StrokeWidth = parseSVGFloat(a.Value)
					case "stroke-linecap":
						p.StrokeLineCap = a.Value
					case "stroke-linejoin":
						p.StrokeLineJoin = a.Value
					case "stroke-miterlimit":
						p.StrokeMiter = parseSVGFloat(a.Value)
					case "stroke-dasharray":
						p.StrokeDash = a.Value
					case "stroke-dashoffset":
						p.StrokeDashOff = parseSVGFloat(a.Value)
					}
				}
				p.D = buildPathDataForRect(x, y, w, h)
				doc.Pages[currentPage] = append(doc.Pages[currentPage], svgElem{Kind: svgElemPath, Path: p})
			case "polyline", "polygon":
				if currentPage <= 0 {
					continue
				}
				p := svgPathElem{Fill: "none", Stroke: "none", StrokeWidth: 1, StrokeLineCap: "butt", StrokeLineJoin: "miter", StrokeMiter: 10}
				var pts string
				for _, a := range t.Attr {
					switch a.Name.Local {
					case "points":
						pts = a.Value
					case "fill":
						p.Fill = a.Value
					case "stroke":
						p.Stroke = a.Value
					case "stroke-width":
						p.StrokeWidth = parseSVGFloat(a.Value)
					case "stroke-linecap":
						p.StrokeLineCap = a.Value
					case "stroke-linejoin":
						p.StrokeLineJoin = a.Value
					case "stroke-miterlimit":
						p.StrokeMiter = parseSVGFloat(a.Value)
					case "stroke-dasharray":
						p.StrokeDash = a.Value
					case "stroke-dashoffset":
						p.StrokeDashOff = parseSVGFloat(a.Value)
					}
				}
				coords := parseSVGPointList(pts)
				p.D = buildPathDataFromPoints(coords, t.Name.Local == "polygon")
				doc.Pages[currentPage] = append(doc.Pages[currentPage], svgElem{Kind: svgElemPath, Path: p})
			case "text":
				if currentPage <= 0 {
					continue
				}
				te := &svgTextElem{Fill: "#000000", FontSize: 12}
				for _, a := range t.Attr {
					switch a.Name.Local {
					case "x":
						te.X = parseSVGFloat(a.Value)
					case "y":
						te.Y = parseSVGFloat(a.Value)
					case "fill":
						te.Fill = a.Value
					case "font-size":
						te.FontSize = parseSVGFloat(a.Value)
					}
				}
				currentText = te
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "g":
				if len(pageStack) > 0 {
					currentPage = pageStack[len(pageStack)-1]
					pageStack = pageStack[:len(pageStack)-1]
				} else {
					currentPage = 0
				}
			case "text":
				if currentText != nil && currentPage > 0 {
					doc.Pages[currentPage] = append(doc.Pages[currentPage], svgElem{Kind: svgElemText, Text: *currentText})
				}
				currentText = nil
			}
		case xml.CharData:
			if currentText != nil {
				currentText.Text += string([]byte(t))
			}
		}
	}

	if doc.PageWidth <= 0 || doc.PageHeight <= 0 {
		return nil, fmt.Errorf("missing svg size metadata")
	}
	return doc, nil
}

func parseSVGFloat(s string) float64 {
	s = strings.TrimSpace(s)
	s = strings.TrimSuffix(s, "px")
	s = strings.TrimSuffix(s, "pt")
	v, _ := strconv.ParseFloat(s, 64)
	return v
}

func parseSVGPointList(s string) []float64 {
	s = strings.ReplaceAll(s, ",", " ")
	fields := strings.Fields(s)
	out := make([]float64, 0, len(fields))
	for _, f := range fields {
		if v, err := strconv.ParseFloat(strings.TrimSpace(f), 64); err == nil {
			out = append(out, v)
		}
	}
	return out
}

func buildPathDataFromPoints(coords []float64, closed bool) string {
	if len(coords) < 4 {
		return ""
	}
	var b strings.Builder
	b.WriteString("M ")
	b.WriteString(formatSVGFloat(coords[0]))
	b.WriteByte(' ')
	b.WriteString(formatSVGFloat(coords[1]))
	for i := 2; i+1 < len(coords); i += 2 {
		b.WriteString(" L ")
		b.WriteString(formatSVGFloat(coords[i]))
		b.WriteByte(' ')
		b.WriteString(formatSVGFloat(coords[i+1]))
	}
	if closed {
		b.WriteString(" Z")
	}
	return b.String()
}

func buildPathDataForRect(x, y, w, h float64) string {
	if w <= 0 || h <= 0 {
		return ""
	}
	x2 := x + w
	y2 := y + h
	return fmt.Sprintf("M %s %s L %s %s L %s %s L %s %s Z",
		formatSVGFloat(x), formatSVGFloat(y),
		formatSVGFloat(x2), formatSVGFloat(y),
		formatSVGFloat(x2), formatSVGFloat(y2),
		formatSVGFloat(x), formatSVGFloat(y2),
	)
}

func createBlankPDFContextForSVG(width, height float64, pageCount int) (*model.Context, []types.IndirectRef, []types.IndirectRef, error) {
	conf := model.NewDefaultConfiguration()
	conf.Unit = types.POINTS

	ctx, err := pdfcpu.CreateContextWithXRefTable(conf, &types.Dim{Width: width, Height: height})
	if err != nil {
		return nil, nil, nil, err
	}

	rootDict, err := ctx.XRefTable.Catalog()
	if err != nil {
		return nil, nil, nil, err
	}
	pagesObj, ok := rootDict.Find("Pages")
	if !ok {
		return nil, nil, nil, fmt.Errorf("missing Pages in catalog")
	}
	pagesDict, err := ctx.XRefTable.DereferenceDict(pagesObj)
	if err != nil {
		return nil, nil, nil, err
	}
	pagesIndRef, ok := pagesObj.(types.IndirectRef)
	if !ok {
		return nil, nil, nil, fmt.Errorf("Pages is not an indirect ref")
	}

	if pageCount <= 0 {
		pageCount = 1
	}

	mediaBox := types.RectForDim(width, height)
	kids := types.Array{}
	pageRefs := make([]types.IndirectRef, 0, pageCount)
	contentRefs := make([]types.IndirectRef, 0, pageCount)

	for i := 0; i < pageCount; i++ {
		sd, _ := ctx.XRefTable.NewStreamDictForBuf(nil)
		if err := sd.Encode(); err != nil {
			return nil, nil, nil, err
		}
		contentsRef, err := ctx.XRefTable.IndRefForNewObject(*sd)
		if err != nil {
			return nil, nil, nil, err
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
			return nil, nil, nil, err
		}
		kids = append(kids, *pageRef)
		pageRefs = append(pageRefs, *pageRef)
		contentRefs = append(contentRefs, *contentsRef)
	}

	pagesDict["Kids"] = kids
	pagesDict["Count"] = types.Integer(pageCount)
	if entry := ctx.XRefTable.Table[pagesIndRef.ObjectNumber.Value()]; entry != nil {
		entry.Object = pagesDict
	}
	ctx.XRefTable.PageCount = pageCount

	return ctx, pageRefs, contentRefs, nil
}

func attachFontResources(ctx *model.Context, pageRefs []types.IndirectRef, fontDict types.Dict) error {
	for _, pr := range pageRefs {
		entry := ctx.Table[int(pr.ObjectNumber)]
		if entry == nil {
			continue
		}
		pd, ok := entry.Object.(types.Dict)
		if !ok {
			continue
		}
		res, ok := pd["Resources"].(types.Dict)
		if !ok {
			res = types.Dict{}
		}
		res["Font"] = fontDict
		pd["Resources"] = res
		entry.Object = pd
	}
	return nil
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

func writePDFForSVGPath(buf *bytes.Buffer, p svgPathElem) {
	if strings.TrimSpace(p.D) == "" {
		return
	}

	if p.Stroke != "none" {
		r, g, b := parseSVGColor(p.Stroke)
		buf.WriteString(fmt.Sprintf("%.6f %.6f %.6f RG\n", r, g, b))
		if p.StrokeWidth > 0 {
			buf.WriteString(fmt.Sprintf("%.4f w\n", p.StrokeWidth))
		}
		buf.WriteString(fmt.Sprintf("%d J\n", svgLineCapToPDF(p.StrokeLineCap)))
		buf.WriteString(fmt.Sprintf("%d j\n", svgLineJoinToPDF(p.StrokeLineJoin)))
		if p.StrokeMiter > 0 {
			buf.WriteString(fmt.Sprintf("%.4f M\n", p.StrokeMiter))
		}
		if strings.TrimSpace(p.StrokeDash) != "" {
			dash := parseSVGDashArray(p.StrokeDash)
			buf.WriteString("[")
			for i, v := range dash {
				if i > 0 {
					buf.WriteByte(' ')
				}
				buf.WriteString(formatPDFNumber(v))
			}
			buf.WriteString("] ")
			buf.WriteString(formatPDFNumber(p.StrokeDashOff))
			buf.WriteString(" d\n")
		} else {
			buf.WriteString("[] 0 d\n")
		}
	}
	if p.Fill != "none" {
		r, g, b := parseSVGColor(p.Fill)
		buf.WriteString(fmt.Sprintf("%.6f %.6f %.6f rg\n", r, g, b))
	}

	writePDFPathFromSVGPathData(buf, p.D)

	if p.Fill != "none" && p.Stroke == "none" {
		buf.WriteString("f\n")
		return
	}
	if p.Stroke != "none" && p.Fill == "none" {
		buf.WriteString("S\n")
		return
	}
	if p.Stroke != "none" && p.Fill != "none" {
		buf.WriteString("B\n")
	}
}

func writePDFForSVGText(buf *bytes.Buffer, t svgTextElem) {
	s := strings.TrimSpace(t.Text)
	if s == "" {
		return
	}
	r, g, b := parseSVGColor(t.Fill)
	buf.WriteString(fmt.Sprintf("%.6f %.6f %.6f rg\n", r, g, b))
	fs := t.FontSize
	if fs <= 0 {
		fs = 12
	}
	buf.WriteString("BT\n")
	buf.WriteString(fmt.Sprintf("/F1 %.4f Tf\n", fs))
	buf.WriteString(fmt.Sprintf("1 0 0 1 %.4f %.4f Tm\n", t.X, t.Y))
	buf.WriteString("(")
	buf.WriteString(escapePDFString(s))
	buf.WriteString(") Tj\n")
	buf.WriteString("ET\n")
}

func writePDFPathFromSVGPathData(buf *bytes.Buffer, d string) {
	toks := tokenizeSVGPath(d)
	i := 0
	for i < len(toks) {
		cmd := toks[i]
		i++
		switch cmd {
		case "M":
			if i+1 >= len(toks) {
				return
			}
			x := parseSVGFloat(toks[i])
			y := parseSVGFloat(toks[i+1])
			i += 2
			buf.WriteString(fmt.Sprintf("%.4f %.4f m\n", x, y))
		case "L":
			if i+1 >= len(toks) {
				return
			}
			x := parseSVGFloat(toks[i])
			y := parseSVGFloat(toks[i+1])
			i += 2
			buf.WriteString(fmt.Sprintf("%.4f %.4f l\n", x, y))
		case "C":
			if i+5 >= len(toks) {
				return
			}
			x1 := parseSVGFloat(toks[i])
			y1 := parseSVGFloat(toks[i+1])
			x2 := parseSVGFloat(toks[i+2])
			y2 := parseSVGFloat(toks[i+3])
			x3 := parseSVGFloat(toks[i+4])
			y3 := parseSVGFloat(toks[i+5])
			i += 6
			buf.WriteString(fmt.Sprintf("%.4f %.4f %.4f %.4f %.4f %.4f c\n", x1, y1, x2, y2, x3, y3))
		case "Z":
			buf.WriteString("h\n")
		default:
		}
	}
}

func tokenizeSVGPath(d string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			out = append(out, b.String())
			b.Reset()
		}
	}
	for _, r := range d {
		if r == 'M' || r == 'L' || r == 'C' || r == 'Z' {
			flush()
			out = append(out, string(r))
			continue
		}
		if r == ',' || r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			flush()
			continue
		}
		b.WriteRune(r)
	}
	flush()
	return out
}

func parseSVGColor(s string) (float64, float64, float64) {
	s = strings.TrimSpace(s)
	if s == "" || s == "none" {
		return 0, 0, 0
	}
	if strings.HasPrefix(s, "#") && len(s) == 7 {
		r, _ := strconv.ParseInt(s[1:3], 16, 64)
		g, _ := strconv.ParseInt(s[3:5], 16, 64)
		b, _ := strconv.ParseInt(s[5:7], 16, 64)
		return float64(r) / 255, float64(g) / 255, float64(b) / 255
	}
	return 0, 0, 0
}

func svgLineCapToPDF(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "butt":
		return 0
	case "round":
		return 1
	case "square":
		return 2
	default:
		return 0
	}
}

func svgLineJoinToPDF(s string) int {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "miter":
		return 0
	case "round":
		return 1
	case "bevel":
		return 2
	default:
		return 0
	}
}

func parseSVGDashArray(s string) []float64 {
	s = strings.ReplaceAll(s, ",", " ")
	fields := strings.Fields(s)
	out := make([]float64, 0, len(fields))
	for _, f := range fields {
		v := parseSVGFloat(f)
		if v > 0 {
			out = append(out, v)
		}
	}
	return out
}

func formatPDFNumber(v float64) string {
	s := strconv.FormatFloat(v, 'f', 4, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-0" {
		return "0"
	}
	return s
}

func escapePDFString(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case '\\':
			b.WriteString("\\\\")
		case '(':
			b.WriteString("\\(")
		case ')':
			b.WriteString("\\)")
		case '\r':
			b.WriteString("\\r")
		case '\n':
			b.WriteString("\\n")
		case '\t':
			b.WriteString("\\t")
		default:
			if r < 32 {
				continue
			}
			b.WriteRune(r)
		}
	}
	return b.String()
}
