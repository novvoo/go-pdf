package gopdf

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
)

type bbox struct {
	MinX float64
	MinY float64
	MaxX float64
	MaxY float64
	Set  bool
}

func (b *bbox) AddPoint(x, y float64) {
	if !b.Set {
		b.MinX, b.MaxX = x, x
		b.MinY, b.MaxY = y, y
		b.Set = true
		return
	}
	if x < b.MinX {
		b.MinX = x
	}
	if x > b.MaxX {
		b.MaxX = x
	}
	if y < b.MinY {
		b.MinY = y
	}
	if y > b.MaxY {
		b.MaxY = y
	}
}

func (b bbox) Inside(o bbox, pad float64) bool {
	if !b.Set || !o.Set {
		return false
	}
	return b.MinX >= o.MinX-pad && b.MaxX <= o.MaxX+pad && b.MinY >= o.MinY-pad && b.MaxY <= o.MaxY+pad
}

func (b bbox) Area() float64 {
	if !b.Set {
		return 0
	}
	w := b.MaxX - b.MinX
	h := b.MaxY - b.MinY
	if w <= 0 || h <= 0 {
		return 0
	}
	return w * h
}

func (b bbox) IntersectArea(o bbox) float64 {
	if !b.Set || !o.Set {
		return 0
	}
	minX := math.Max(b.MinX, o.MinX)
	minY := math.Max(b.MinY, o.MinY)
	maxX := math.Min(b.MaxX, o.MaxX)
	maxY := math.Min(b.MaxY, o.MaxY)
	w := maxX - minX
	h := maxY - minY
	if w <= 0 || h <= 0 {
		return 0
	}
	return w * h
}

func RewriteSVGTextFirst(inSVGPath, outSVGPath string, sourceByPage map[int][]TextElementInfo, translations []TextOverlayTopLeft) error {
	if inSVGPath == "" || outSVGPath == "" {
		return fmt.Errorf("missing inSVGPath or outSVGPath")
	}
	input, err := os.ReadFile(inSVGPath)
	if err != nil {
		return err
	}

	if sourceByPage == nil {
		sourceByPage = map[int][]TextElementInfo{}
	}
	trByPage := map[int][]TextOverlayTopLeft{}
	for _, o := range translations {
		if o.Page <= 0 || strings.TrimSpace(o.Text) == "" {
			continue
		}
		trByPage[o.Page] = append(trByPage[o.Page], o)
	}

	hide, err := detectTextOutlineModules(input, sourceByPage)
	if err != nil {
		return err
	}

	out, err := rewriteSVGWithTextLayers(input, hide, sourceByPage, trByPage)
	if err != nil {
		return err
	}
	if err := ensureDirForFile(outSVGPath); err != nil {
		return err
	}
	return os.WriteFile(outSVGPath, out, 0644)
}

func detectTextOutlineModules(svg []byte, sourceByPage map[int][]TextElementInfo) (map[int]map[int]bool, error) {
	dec := xml.NewDecoder(bytes.NewReader(svg))

	moduleBBoxes := map[int]map[int]*bbox{}
	currentPage := 0
	var pageStack []int
	var moduleStack []int

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
			if t.Name.Local == "g" {
				pageStack = append(pageStack, currentPage)
				for _, a := range t.Attr {
					if a.Name.Local == "data-page" {
						if v, err := strconv.Atoi(strings.TrimSpace(a.Value)); err == nil {
							currentPage = v
						}
					}
				}
				modID := 0
				for _, a := range t.Attr {
					if a.Name.Local == "data-module" {
						if v, err := strconv.Atoi(strings.TrimSpace(a.Value)); err == nil {
							modID = v
						}
					}
				}
				moduleStack = append(moduleStack, modID)
			}

			if currentPage <= 0 {
				continue
			}
			curMod := 0
			if len(moduleStack) > 0 {
				curMod = moduleStack[len(moduleStack)-1]
			}
			if curMod <= 0 {
				continue
			}

			switch t.Name.Local {
			case "path":
				var d string
				for _, a := range t.Attr {
					if a.Name.Local == "d" {
						d = a.Value
						break
					}
				}
				if d == "" {
					continue
				}
				b := getOrCreateBBox(moduleBBoxes, currentPage, curMod)
				addPathBBox(b, d)
			case "rect":
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
					}
				}
				if w <= 0 || h <= 0 {
					continue
				}
				b := getOrCreateBBox(moduleBBoxes, currentPage, curMod)
				b.AddPoint(x, y)
				b.AddPoint(x+w, y+h)
			case "polyline", "polygon":
				var pts string
				for _, a := range t.Attr {
					if a.Name.Local == "points" {
						pts = a.Value
						break
					}
				}
				if pts == "" {
					continue
				}
				b := getOrCreateBBox(moduleBBoxes, currentPage, curMod)
				addPointsBBox(b, pts)
			}

		case xml.EndElement:
			if t.Name.Local == "g" {
				if len(moduleStack) > 0 {
					moduleStack = moduleStack[:len(moduleStack)-1]
				}
				if len(pageStack) > 0 {
					currentPage = pageStack[len(pageStack)-1]
					pageStack = pageStack[:len(pageStack)-1]
				} else {
					currentPage = 0
				}
			}
		}
	}

	hide := map[int]map[int]bool{}
	for page, mods := range moduleBBoxes {
		texts := sourceByPage[page]
		if len(texts) == 0 {
			continue
		}
		for modID, mb := range mods {
			if mb == nil || !mb.Set {
				continue
			}
			for _, t := range texts {
				if !isReliableTextElement(t) {
					continue
				}
				tb := bboxFromTextElement(t)
				inter := mb.IntersectArea(tb)
				ma := mb.Area()
				if ma <= 0 {
					continue
				}
				overlap := inter / ma

				maxH := math.Max(2, t.Height*2.0)
				maxW := math.Max(2, t.Width+t.Height*2.0)
				if t.Width <= 0 {
					maxW = math.Max(2, t.FontSize*0.6*float64(len([]rune(t.Text)))+t.Height*2.0)
				}

				if overlap >= 0.85 && (mb.MaxY-mb.MinY) <= maxH && (mb.MaxX-mb.MinX) <= maxW {
					if hide[page] == nil {
						hide[page] = map[int]bool{}
					}
					hide[page][modID] = true
					break
				}
			}
		}
	}

	return hide, nil
}

func isReliableTextElement(t TextElementInfo) bool {
	if strings.TrimSpace(t.Text) == "" {
		return false
	}
	if t.CIDCount <= 0 {
		return t.ReplacementCount == 0
	}
	return float64(t.ReplacementCount)/float64(t.CIDCount) <= 0.1
}

func bboxFromTextElement(t TextElementInfo) bbox {
	x0 := t.X
	x1 := t.X + t.Width
	if t.Width <= 0 {
		x1 = t.X + t.FontSize*0.6*float64(len([]rune(t.Text)))
	}
	h := t.Height
	if h <= 0 {
		h = t.FontSize
	}
	y0 := t.Y - h*1.2
	y1 := t.Y + h*0.4
	b := bbox{}
	b.AddPoint(x0, y0)
	b.AddPoint(x1, y1)
	return b
}

func rewriteSVGWithTextLayers(input []byte, hide map[int]map[int]bool, sourceByPage map[int][]TextElementInfo, trByPage map[int][]TextOverlayTopLeft) ([]byte, error) {
	dec := xml.NewDecoder(bytes.NewReader(input))
	var out bytes.Buffer
	out.WriteString(xml.Header)
	enc := xml.NewEncoder(&out)
	enc.Indent("", "  ")

	currentPage := 0
	var pageStack []int
	var gHadDataPageStack []bool

	skipDepth := 0

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
			if skipDepth > 0 {
				skipDepth++
				continue
			}

			if t.Name.Local == "g" {
				pageStack = append(pageStack, currentPage)
				hasDataPage := false
				for _, a := range t.Attr {
					if a.Name.Local == "data-page" {
						if v, err := strconv.Atoi(strings.TrimSpace(a.Value)); err == nil {
							currentPage = v
							hasDataPage = true
						}
					}
				}
				gHadDataPageStack = append(gHadDataPageStack, hasDataPage)

				if currentPage > 0 {
					modID := 0
					for _, a := range t.Attr {
						if a.Name.Local == "data-module" {
							if v, err := strconv.Atoi(strings.TrimSpace(a.Value)); err == nil {
								modID = v
							}
						}
					}
					if modID > 0 && hide[currentPage] != nil && hide[currentPage][modID] {
						skipDepth = 1
						continue
					}
				}
			}

			if err := enc.EncodeToken(t); err != nil {
				return nil, err
			}

		case xml.EndElement:
			if skipDepth > 0 {
				skipDepth--
				if skipDepth == 0 {
					if t.Name.Local == "g" {
						if len(gHadDataPageStack) > 0 {
							gHadDataPageStack = gHadDataPageStack[:len(gHadDataPageStack)-1]
						}
						if len(pageStack) > 0 {
							currentPage = pageStack[len(pageStack)-1]
							pageStack = pageStack[:len(pageStack)-1]
						} else {
							currentPage = 0
						}
					}
				}
				continue
			}

			if t.Name.Local == "g" {
				if len(gHadDataPageStack) > 0 && gHadDataPageStack[len(gHadDataPageStack)-1] && currentPage > 0 {
					if err := encodeSourceTextLayer(enc, sourceByPage[currentPage]); err != nil {
						return nil, err
					}
					if err := encodeTranslationTextLayer(enc, trByPage[currentPage]); err != nil {
						return nil, err
					}
				}
			}

			if err := enc.EncodeToken(t); err != nil {
				return nil, err
			}

			if t.Name.Local == "g" {
				if len(gHadDataPageStack) > 0 {
					gHadDataPageStack = gHadDataPageStack[:len(gHadDataPageStack)-1]
				}
				if len(pageStack) > 0 {
					currentPage = pageStack[len(pageStack)-1]
					pageStack = pageStack[:len(pageStack)-1]
				} else {
					currentPage = 0
				}
			}

		default:
			if skipDepth > 0 {
				continue
			}
			if err := enc.EncodeToken(tok); err != nil {
				return nil, err
			}
		}
	}

	if err := enc.Flush(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func encodeSourceTextLayer(enc *xml.Encoder, texts []TextElementInfo) error {
	if len(texts) == 0 {
		return nil
	}
	start := xml.StartElement{
		Name: xml.Name{Local: "g"},
		Attr: []xml.Attr{{Name: xml.Name{Local: "data-layer"}, Value: "source-text"}},
	}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	for _, t := range texts {
		if !isReliableTextElement(t) {
			continue
		}
		fontFamily := strings.TrimSpace(t.FontBaseName)
		if fontFamily == "" {
			fontFamily = "Helvetica"
		}
		te := xml.StartElement{
			Name: xml.Name{Local: "text"},
			Attr: []xml.Attr{
				{Name: xml.Name{Local: "x"}, Value: formatSVGFloat(t.X)},
				{Name: xml.Name{Local: "y"}, Value: formatSVGFloat(t.Y)},
				{Name: xml.Name{Local: "fill"}, Value: "#000000"},
				{Name: xml.Name{Local: "font-size"}, Value: formatSVGFloat(t.FontSize)},
				{Name: xml.Name{Local: "font-family"}, Value: fontFamily},
			},
		}
		if err := enc.EncodeToken(te); err != nil {
			return err
		}
		if err := enc.EncodeToken(xml.CharData([]byte(ensureValidUTF8(t.Text)))); err != nil {
			return err
		}
		if err := enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "text"}}); err != nil {
			return err
		}
	}
	return enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "g"}})
}

func encodeTranslationTextLayer(enc *xml.Encoder, ovs []TextOverlayTopLeft) error {
	if len(ovs) == 0 {
		return nil
	}
	start := xml.StartElement{
		Name: xml.Name{Local: "g"},
		Attr: []xml.Attr{{Name: xml.Name{Local: "data-layer"}, Value: "translation-text"}},
	}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	for _, o := range ovs {
		txt := strings.TrimSpace(o.Text)
		if txt == "" {
			continue
		}
		fill := strings.TrimSpace(o.FillColor)
		if fill == "" {
			fill = "0 0 0"
		}
		fontFamily := strings.TrimSpace(o.FontName)
		if fontFamily == "" {
			fontFamily = "Helvetica"
		}
		fontSize := o.FontSize
		if fontSize <= 0 {
			fontSize = 12
		}
		attrs := []xml.Attr{
			{Name: xml.Name{Local: "x"}, Value: formatSVGFloat(o.X)},
			{Name: xml.Name{Local: "y"}, Value: formatSVGFloat(o.Y)},
			{Name: xml.Name{Local: "fill"}, Value: rgbTripletToHex(fill)},
			{Name: xml.Name{Local: "font-size"}, Value: formatSVGFloat(fontSize)},
			{Name: xml.Name{Local: "font-family"}, Value: fontFamily},
		}
		if o.Opacity > 0 && o.Opacity < 1 {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "opacity"}, Value: formatSVGFloat(o.Opacity)})
		}
		te := xml.StartElement{Name: xml.Name{Local: "text"}, Attr: attrs}
		if err := enc.EncodeToken(te); err != nil {
			return err
		}
		if err := enc.EncodeToken(xml.CharData([]byte(ensureValidUTF8(txt)))); err != nil {
			return err
		}
		if err := enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "text"}}); err != nil {
			return err
		}
	}
	return enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "g"}})
}

func getOrCreateBBox(m map[int]map[int]*bbox, page, mod int) *bbox {
	pm := m[page]
	if pm == nil {
		pm = map[int]*bbox{}
		m[page] = pm
	}
	b := pm[mod]
	if b == nil {
		b = &bbox{MinX: math.Inf(1), MinY: math.Inf(1), MaxX: math.Inf(-1), MaxY: math.Inf(-1)}
		pm[mod] = b
	}
	return b
}

func addPathBBox(b *bbox, d string) {
	nums := extractFloatTokens(d)
	for i := 0; i+1 < len(nums); i += 2 {
		b.AddPoint(nums[i], nums[i+1])
	}
}

func addPointsBBox(b *bbox, pts string) {
	nums := extractFloatTokens(pts)
	for i := 0; i+1 < len(nums); i += 2 {
		b.AddPoint(nums[i], nums[i+1])
	}
}

func extractFloatTokens(s string) []float64 {
	out := make([]float64, 0, 64)
	var cur strings.Builder
	flush := func() {
		if cur.Len() == 0 {
			return
		}
		v, err := strconv.ParseFloat(cur.String(), 64)
		if err == nil && !math.IsNaN(v) && !math.IsInf(v, 0) {
			out = append(out, v)
		}
		cur.Reset()
	}

	for _, r := range s {
		if (r >= '0' && r <= '9') || r == '-' || r == '+' || r == '.' || r == 'e' || r == 'E' {
			cur.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}
