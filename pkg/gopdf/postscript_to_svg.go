package gopdf

import (
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
)

type psRGB struct {
	R, G, B float64
}

type psStyle struct {
	Color      psRGB
	LineWidth  float64
	LineCap    int
	LineJoin   int
	MiterLimit float64
	Dash       []float64
	DashOffset float64
	FontSize   float64
	FontName   string
}

type psRenderState struct {
	CTM   *Matrix
	Style psStyle
}

type svgPathShape struct {
	Kind           string
	D              string
	Points         string
	RectX          float64
	RectY          float64
	RectW          float64
	RectH          float64
	Fill           string
	Stroke         string
	StrokeWidth    float64
	StrokeLineCap  string
	StrokeLineJoin string
	StrokeMiter    float64
	StrokeDash     []float64
	StrokeDashOff  float64
}

type svgText struct {
	X, Y     float64
	FontSize float64
	Fill     string
	FontName string
	Text     string
}

type svgElemRecord struct {
	ModuleID int
	Kind     svgElemKind
	Path     svgPathShape
	Text     svgText
}

func ConvertPostScriptToSVG(psPath, svgPath string) error {
	if psPath == "" || svgPath == "" {
		return fmt.Errorf("missing psPath or svgPath")
	}

	data, err := os.ReadFile(psPath)
	if err != nil {
		return err
	}

	pageW, pageH, err := parsePostScriptBoundingBox(string(data))
	if err != nil {
		return err
	}

	pages := splitPostScriptPages(string(data))
	if len(pages) == 0 {
		return fmt.Errorf("no pages found in PostScript: %s", psPath)
	}

	var pageElems [][]svgElemRecord
	for _, page := range pages {
		elems := renderPostScriptPageToSVG(page, pageH)
		pageElems = append(pageElems, elems)
	}

	var out bytes.Buffer
	enc := xml.NewEncoder(&out)
	enc.Indent("", "  ")
	out.WriteString(xml.Header)

	totalH := pageH * float64(len(pages))
	start := xml.StartElement{
		Name: xml.Name{Local: "svg"},
		Attr: []xml.Attr{
			{Name: xml.Name{Local: "xmlns"}, Value: "http://www.w3.org/2000/svg"},
			{Name: xml.Name{Local: "width"}, Value: formatSVGFloat(pageW)},
			{Name: xml.Name{Local: "height"}, Value: formatSVGFloat(totalH)},
			{Name: xml.Name{Local: "viewBox"}, Value: fmt.Sprintf("0 0 %s %s", formatSVGFloat(pageW), formatSVGFloat(totalH))},
			{Name: xml.Name{Local: "data-page-count"}, Value: strconv.Itoa(len(pages))},
			{Name: xml.Name{Local: "data-page-width"}, Value: formatSVGFloat(pageW)},
			{Name: xml.Name{Local: "data-page-height"}, Value: formatSVGFloat(pageH)},
		},
	}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}

	for i := range pages {
		yOff := pageH * float64(i)
		gStart := xml.StartElement{
			Name: xml.Name{Local: "g"},
			Attr: []xml.Attr{
				{Name: xml.Name{Local: "data-page"}, Value: strconv.Itoa(i + 1)},
				{Name: xml.Name{Local: "transform"}, Value: fmt.Sprintf("translate(0,%s)", formatSVGFloat(yOff))},
			},
		}
		if err := enc.EncodeToken(gStart); err != nil {
			return err
		}

		openModule := 0
		moduleElemIndex := 0
		closeModule := func() error {
			if openModule <= 0 {
				return nil
			}
			openModule = 0
			moduleElemIndex = 0
			return enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "g"}})
		}

		for _, e := range pageElems[i] {
			if e.ModuleID != openModule {
				if err := closeModule(); err != nil {
					return err
				}
				if e.ModuleID > 0 {
					openModule = e.ModuleID
					if err := enc.EncodeToken(xml.StartElement{
						Name: xml.Name{Local: "g"},
						Attr: []xml.Attr{
							{Name: xml.Name{Local: "data-module"}, Value: strconv.Itoa(e.ModuleID)},
							{Name: xml.Name{Local: "id"}, Value: fmt.Sprintf("p%d-m%d", i+1, e.ModuleID)},
						},
					}); err != nil {
						return err
					}
				}
			}

			switch e.Kind {
			case svgElemPath:
				p := e.Path
				moduleElemIndex++
				if err := encodeSVGShape(enc, i+1, openModule, moduleElemIndex, p); err != nil {
					return err
				}
			case svgElemText:
				t := e.Text
				moduleElemIndex++
				fontName := strings.TrimSpace(t.FontName)
				if fontName == "" {
					fontName = "Helvetica"
				}
				attrs := []xml.Attr{
					{Name: xml.Name{Local: "id"}, Value: fmt.Sprintf("p%d-m%d-e%d", i+1, openModule, moduleElemIndex)},
					{Name: xml.Name{Local: "x"}, Value: formatSVGFloat(t.X)},
					{Name: xml.Name{Local: "y"}, Value: formatSVGFloat(t.Y)},
					{Name: xml.Name{Local: "fill"}, Value: t.Fill},
					{Name: xml.Name{Local: "font-size"}, Value: formatSVGFloat(t.FontSize)},
					{Name: xml.Name{Local: "font-family"}, Value: fontName},
				}
				if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "text"}, Attr: attrs}); err != nil {
					return err
				}
				if err := enc.EncodeToken(xml.CharData([]byte(t.Text))); err != nil {
					return err
				}
				if err := enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "text"}}); err != nil {
					return err
				}
			}
		}
		if err := closeModule(); err != nil {
			return err
		}

		if err := enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "g"}}); err != nil {
			return err
		}
	}

	if err := enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "svg"}}); err != nil {
		return err
	}
	if err := enc.Flush(); err != nil {
		return err
	}

	if err := ensureDirForFile(svgPath); err != nil {
		return err
	}
	return os.WriteFile(svgPath, out.Bytes(), 0644)
}

func encodeSVGShape(enc *xml.Encoder, page, module, idx int, p svgPathShape) error {
	var attrs []xml.Attr
	attrs = append(attrs,
		xml.Attr{Name: xml.Name{Local: "id"}, Value: fmt.Sprintf("p%d-m%d-e%d", page, module, idx)},
		xml.Attr{Name: xml.Name{Local: "data-kind"}, Value: p.Kind},
		xml.Attr{Name: xml.Name{Local: "fill"}, Value: p.Fill},
		xml.Attr{Name: xml.Name{Local: "stroke"}, Value: p.Stroke},
	)

	if p.Stroke != "none" {
		attrs = append(attrs,
			xml.Attr{Name: xml.Name{Local: "stroke-width"}, Value: formatSVGFloat(p.StrokeWidth)},
			xml.Attr{Name: xml.Name{Local: "stroke-linecap"}, Value: p.StrokeLineCap},
			xml.Attr{Name: xml.Name{Local: "stroke-linejoin"}, Value: p.StrokeLineJoin},
			xml.Attr{Name: xml.Name{Local: "stroke-miterlimit"}, Value: formatSVGFloat(p.StrokeMiter)},
		)
		if len(p.StrokeDash) > 0 {
			var dash strings.Builder
			for di, dv := range p.StrokeDash {
				if di > 0 {
					dash.WriteByte(',')
				}
				dash.WriteString(formatSVGFloat(dv))
			}
			attrs = append(attrs,
				xml.Attr{Name: xml.Name{Local: "stroke-dasharray"}, Value: dash.String()},
				xml.Attr{Name: xml.Name{Local: "stroke-dashoffset"}, Value: formatSVGFloat(p.StrokeDashOff)},
			)
		}
	}

	switch p.Kind {
	case "rect":
		attrs = append(attrs,
			xml.Attr{Name: xml.Name{Local: "x"}, Value: formatSVGFloat(p.RectX)},
			xml.Attr{Name: xml.Name{Local: "y"}, Value: formatSVGFloat(p.RectY)},
			xml.Attr{Name: xml.Name{Local: "width"}, Value: formatSVGFloat(p.RectW)},
			xml.Attr{Name: xml.Name{Local: "height"}, Value: formatSVGFloat(p.RectH)},
		)
		if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "rect"}, Attr: attrs}); err != nil {
			return err
		}
		return enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "rect"}})
	case "polygon":
		attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "points"}, Value: p.Points})
		if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "polygon"}, Attr: attrs}); err != nil {
			return err
		}
		return enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "polygon"}})
	case "polyline":
		attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "points"}, Value: p.Points})
		if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "polyline"}, Attr: attrs}); err != nil {
			return err
		}
		return enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "polyline"}})
	default:
		attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "d"}, Value: p.D})
		if err := enc.EncodeToken(xml.StartElement{Name: xml.Name{Local: "path"}, Attr: attrs}); err != nil {
			return err
		}
		return enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "path"}})
	}
}

func parsePostScriptBoundingBox(ps string) (float64, float64, error) {
	sc := bufio.NewScanner(strings.NewReader(ps))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.Contains(line, "BoundingBox:") {
			continue
		}
		i := strings.Index(line, "BoundingBox:")
		if i < 0 {
			continue
		}
		fields := strings.Fields(line[i+len("BoundingBox:"):])
		if len(fields) < 4 {
			continue
		}
		w, err1 := strconv.ParseFloat(fields[2], 64)
		h, err2 := strconv.ParseFloat(fields[3], 64)
		if err1 == nil && err2 == nil && w > 0 && h > 0 {
			return w, h, nil
		}
	}
	if err := sc.Err(); err != nil {
		return 0, 0, err
	}
	return 0, 0, fmt.Errorf("missing BoundingBox in PostScript")
}

func splitPostScriptPages(ps string) []string {
	var pages []string
	var buf bytes.Buffer

	sc := bufio.NewScanner(strings.NewReader(ps))
	inDocument := false
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "%") {
			if strings.HasPrefix(line, "%%Page:") {
				inDocument = true
			}
			continue
		}
		if !inDocument {
			continue
		}
		buf.WriteString(line)
		buf.WriteByte('\n')
		if line == "showpage" {
			pages = append(pages, buf.String())
			buf.Reset()
		}
	}
	if s := strings.TrimSpace(buf.String()); s != "" {
		pages = append(pages, buf.String())
	}
	return pages
}

func renderPostScriptPageToSVG(page string, pageH float64) []svgElemRecord {
	state := psRenderState{
		CTM: NewIdentityMatrix(),
		Style: psStyle{
			Color:      psRGB{0, 0, 0},
			LineWidth:  1,
			LineCap:    1,
			LineJoin:   1,
			MiterLimit: 10,
			FontSize:   12,
			FontName:   "Helvetica",
		},
	}
	var stack []psRenderState

	var elems []svgElemRecord
	moduleCounter := 0
	var moduleStack []int

	currentModule := func() int {
		if len(moduleStack) == 0 {
			return 0
		}
		return moduleStack[len(moduleStack)-1]
	}

	var pb psPathBuilder
	curX, curY := 0.0, 0.0
	hasPoint := false

	flipY := func(y float64) float64 {
		if pageH <= 0 {
			return y
		}
		return pageH - y
	}

	flushPath := func(paint string) {
		shape, ok := pb.Flush(paint, state)
		if !ok {
			return
		}

		elems = append(elems, svgElemRecord{ModuleID: currentModule(), Kind: svgElemPath, Path: shape})
	}

	sc := bufio.NewScanner(strings.NewReader(page))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}

		if strings.HasSuffix(line, " show") {
			s, ok := parsePSShowString(line)
			if !ok || !hasPoint {
				continue
			}
			scale := avgScale(state.CTM)
			fs := state.Style.FontSize * scale
			if fs <= 0 {
				fs = 12
			}
			elems = append(elems, svgElemRecord{ModuleID: currentModule(), Kind: svgElemText, Text: svgText{
				X:        curX,
				Y:        curY,
				FontSize: fs,
				Fill:     rgbToHex(state.Style.Color),
				FontName: state.Style.FontName,
				Text:     s,
			}})
			continue
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		op := fields[len(fields)-1]
		args := fields[:len(fields)-1]
		nums := parsePSFloats(args)

		switch op {
		case "gsave":
			stack = append(stack, psRenderState{CTM: state.CTM.Clone(), Style: state.Style})
			moduleCounter++
			moduleStack = append(moduleStack, moduleCounter)
		case "grestore":
			if len(stack) > 0 {
				state = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
			if len(moduleStack) > 0 {
				moduleStack = moduleStack[:len(moduleStack)-1]
			}
		case "translate":
			if len(nums) >= 2 {
				state.CTM = state.CTM.Translate(nums[0], nums[1])
			}
		case "scale":
			if len(nums) >= 2 {
				state.CTM = state.CTM.Scale(nums[0], nums[1])
			}
		case "concat":
			if len(nums) >= 6 {
				m := &Matrix{XX: nums[0], YX: nums[1], XY: nums[2], YY: nums[3], X0: nums[4], Y0: nums[5]}
				state.CTM = state.CTM.Multiply(m)
			}
		case "setrgbcolor":
			if len(nums) >= 3 {
				state.Style.Color = psRGB{clamp01Unit(nums[0]), clamp01Unit(nums[1]), clamp01Unit(nums[2])}
			}
		case "setlinewidth":
			if len(nums) >= 1 {
				state.Style.LineWidth = nums[0]
			}
		case "setlinecap":
			if len(nums) >= 1 {
				state.Style.LineCap = int(nums[0])
			}
		case "setlinejoin":
			if len(nums) >= 1 {
				state.Style.LineJoin = int(nums[0])
			}
		case "setmiterlimit":
			if len(nums) >= 1 {
				state.Style.MiterLimit = nums[0]
			}
		case "setdash":
			if len(nums) == 0 {
				state.Style.Dash = nil
				state.Style.DashOffset = 0
				continue
			}
			if len(nums) >= 1 {
				state.Style.DashOffset = nums[len(nums)-1]
				if len(nums) > 1 {
					state.Style.Dash = append([]float64{}, nums[:len(nums)-1]...)
				} else {
					state.Style.Dash = nil
				}
			}
		case "newpath":
			pb.Reset()
			hasPoint = false
		case "moveto":
			if len(nums) >= 2 {
				x, y := state.CTM.Transform(nums[0], nums[1])
				y = flipY(y)
				curX, curY = x, y
				hasPoint = true
				pb.MoveTo(x, y)
			}
		case "lineto":
			if len(nums) >= 2 {
				x, y := state.CTM.Transform(nums[0], nums[1])
				y = flipY(y)
				curX, curY = x, y
				hasPoint = true
				pb.LineTo(x, y)
			}
		case "curveto":
			if len(nums) >= 6 {
				x1, y1 := state.CTM.Transform(nums[0], nums[1])
				x2, y2 := state.CTM.Transform(nums[2], nums[3])
				x3, y3 := state.CTM.Transform(nums[4], nums[5])
				y1 = flipY(y1)
				y2 = flipY(y2)
				y3 = flipY(y3)
				curX, curY = x3, y3
				hasPoint = true
				pb.CurveTo(x1, y1, x2, y2, x3, y3)
			}
		case "closepath":
			pb.Close()
		case "fill":
			flushPath("fill")
		case "stroke":
			flushPath("stroke")
		case "setfont":
			if fn, fs, ok := parsePSSetFont(line); ok {
				state.Style.FontName = fn
				state.Style.FontSize = fs
			}
		}
	}

	return elems
}

type psPoint struct {
	X, Y float64
}

type psPathBuilder struct {
	d        strings.Builder
	points   []psPoint
	hasCurve bool
	closed   bool
	hasPath  bool
}

func (p *psPathBuilder) Reset() {
	p.d.Reset()
	p.points = nil
	p.hasCurve = false
	p.closed = false
	p.hasPath = false
}

func (p *psPathBuilder) MoveTo(x, y float64) {
	p.ensureSpace()
	p.d.WriteString("M ")
	p.d.WriteString(formatSVGFloat(x))
	p.d.WriteByte(' ')
	p.d.WriteString(formatSVGFloat(y))
	p.points = []psPoint{{X: x, Y: y}}
	p.closed = false
	p.hasPath = true
}

func (p *psPathBuilder) LineTo(x, y float64) {
	if !p.hasPath {
		p.MoveTo(x, y)
		return
	}
	p.ensureSpace()
	p.d.WriteString("L ")
	p.d.WriteString(formatSVGFloat(x))
	p.d.WriteByte(' ')
	p.d.WriteString(formatSVGFloat(y))
	p.points = append(p.points, psPoint{X: x, Y: y})
}

func (p *psPathBuilder) CurveTo(x1, y1, x2, y2, x3, y3 float64) {
	if !p.hasPath {
		p.MoveTo(x3, y3)
		return
	}
	p.hasCurve = true
	p.ensureSpace()
	p.d.WriteString("C ")
	p.d.WriteString(formatSVGFloat(x1))
	p.d.WriteByte(' ')
	p.d.WriteString(formatSVGFloat(y1))
	p.d.WriteByte(' ')
	p.d.WriteString(formatSVGFloat(x2))
	p.d.WriteByte(' ')
	p.d.WriteString(formatSVGFloat(y2))
	p.d.WriteByte(' ')
	p.d.WriteString(formatSVGFloat(x3))
	p.d.WriteByte(' ')
	p.d.WriteString(formatSVGFloat(y3))
}

func (p *psPathBuilder) Close() {
	if !p.hasPath {
		return
	}
	p.closed = true
	p.ensureSpace()
	p.d.WriteString("Z")
}

func (p *psPathBuilder) ensureSpace() {
	if p.d.Len() > 0 {
		p.d.WriteByte(' ')
	}
}

func (p *psPathBuilder) Flush(paint string, state psRenderState) (svgPathShape, bool) {
	if !p.hasPath {
		return svgPathShape{}, false
	}

	d := strings.TrimSpace(p.d.String())
	points := append([]psPoint{}, p.points...)
	hasCurve := p.hasCurve
	closed := p.closed
	p.Reset()
	if d == "" {
		return svgPathShape{}, false
	}

	scale := avgScale(state.CTM)
	strokeW := state.Style.LineWidth * scale
	if strokeW <= 0 {
		strokeW = 0.1
	}

	shape := svgPathShape{
		Kind:           "path",
		D:              d,
		Fill:           "none",
		Stroke:         "none",
		StrokeWidth:    strokeW,
		StrokeLineCap:  lineCapToSVG(state.Style.LineCap),
		StrokeLineJoin: lineJoinToSVG(state.Style.LineJoin),
		StrokeMiter:    state.Style.MiterLimit,
		StrokeDash:     append([]float64{}, state.Style.Dash...),
		StrokeDashOff:  state.Style.DashOffset * scale,
	}

	col := rgbToHex(state.Style.Color)
	switch paint {
	case "fill":
		shape.Fill = col
	case "stroke":
		shape.Stroke = col
	}

	if !hasCurve {
		if ok, rx, ry, rw, rh := axisAlignedRect(points, closed); ok {
			shape.Kind = "rect"
			shape.D = ""
			shape.Points = ""
			shape.RectX = rx
			shape.RectY = ry
			shape.RectW = rw
			shape.RectH = rh
			return shape, true
		}
		if len(points) >= 2 && len(points) <= 200 {
			shape.D = ""
			shape.RectX, shape.RectY, shape.RectW, shape.RectH = 0, 0, 0, 0
			shape.Points = encodeSVGPoints(points)
			if closed {
				shape.Kind = "polygon"
			} else {
				shape.Kind = "polyline"
			}
			return shape, true
		}
	}

	return shape, true
}

func encodeSVGPoints(pts []psPoint) string {
	var b strings.Builder
	for i, p := range pts {
		if i > 0 {
			b.WriteByte(' ')
		}
		b.WriteString(formatSVGFloat(p.X))
		b.WriteByte(',')
		b.WriteString(formatSVGFloat(p.Y))
	}
	return b.String()
}

func axisAlignedRect(pts []psPoint, closed bool) (bool, float64, float64, float64, float64) {
	if !closed || len(pts) != 4 {
		return false, 0, 0, 0, 0
	}
	p0 := pts[0]
	p1 := pts[1]
	p2 := pts[2]
	p3 := pts[3]

	const eps = 1e-3
	eq := func(a, b float64) bool { return math.Abs(a-b) <= eps }

	if !(eq(p0.Y, p1.Y) && eq(p1.X, p2.X) && eq(p2.Y, p3.Y) && eq(p3.X, p0.X)) {
		return false, 0, 0, 0, 0
	}

	minX := math.Min(math.Min(p0.X, p1.X), math.Min(p2.X, p3.X))
	maxX := math.Max(math.Max(p0.X, p1.X), math.Max(p2.X, p3.X))
	minY := math.Min(math.Min(p0.Y, p1.Y), math.Min(p2.Y, p3.Y))
	maxY := math.Max(math.Max(p0.Y, p1.Y), math.Max(p2.Y, p3.Y))

	w := maxX - minX
	h := maxY - minY
	if w <= eps || h <= eps {
		return false, 0, 0, 0, 0
	}
	return true, minX, minY, w, h
}

func parsePSSetFont(line string) (string, float64, bool) {
	fields := strings.Fields(line)
	if len(fields) < 5 {
		return "", 0, false
	}
	if fields[len(fields)-1] != "setfont" {
		return "", 0, false
	}
	var fontName string
	var fontSize float64
	for i := 0; i < len(fields); i++ {
		if strings.HasPrefix(fields[i], "/") && fields[i] != "/" {
			fontName = strings.TrimPrefix(fields[i], "/")
		}
		if fields[i] == "scalefont" && i >= 1 {
			if v, err := strconv.ParseFloat(fields[i-1], 64); err == nil {
				fontSize = v
			}
		}
	}
	if fontName == "" || fontSize <= 0 {
		return "", 0, false
	}
	return fontName, fontSize, true
}

func parsePSShowString(line string) (string, bool) {
	i := strings.IndexByte(line, '(')
	j := strings.LastIndexByte(line, ')')
	if i < 0 || j < 0 || j <= i {
		return "", false
	}
	raw := line[i+1 : j]
	return unescapePSString(raw), true
}

func unescapePSString(s string) string {
	var b strings.Builder
	esc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !esc {
			if c == '\\' {
				esc = true
				continue
			}
			b.WriteByte(c)
			continue
		}
		esc = false
		switch c {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		default:
			b.WriteByte(c)
		}
	}
	if esc {
		b.WriteByte('\\')
	}
	return b.String()
}

func parsePSFloats(tokens []string) []float64 {
	out := make([]float64, 0, len(tokens))
	for _, t := range tokens {
		clean := strings.TrimSpace(t)
		if clean == "" {
			continue
		}
		clean = strings.Trim(clean, "[]")
		if clean == "" {
			continue
		}
		if v, err := strconv.ParseFloat(clean, 64); err == nil {
			out = append(out, v)
		}
	}
	return out
}

func lineCapToSVG(v int) string {
	switch v {
	case 0:
		return "butt"
	case 1:
		return "round"
	case 2:
		return "square"
	default:
		return "butt"
	}
}

func lineJoinToSVG(v int) string {
	switch v {
	case 0:
		return "miter"
	case 1:
		return "round"
	case 2:
		return "bevel"
	default:
		return "miter"
	}
}

func clamp01Unit(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func rgbToHex(c psRGB) string {
	r := int(math.Round(clamp01Unit(c.R) * 255))
	g := int(math.Round(clamp01Unit(c.G) * 255))
	b := int(math.Round(clamp01Unit(c.B) * 255))
	return fmt.Sprintf("#%02x%02x%02x", r, g, b)
}

func avgScale(m *Matrix) float64 {
	if m == nil {
		return 1
	}
	sx := math.Hypot(m.XX, m.YX)
	sy := math.Hypot(m.XY, m.YY)
	if sx <= 0 && sy <= 0 {
		return 1
	}
	if sx <= 0 {
		return sy
	}
	if sy <= 0 {
		return sx
	}
	return (sx + sy) * 0.5
}

func formatSVGFloat(v float64) string {
	s := strconv.FormatFloat(v, 'f', 4, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	if s == "" || s == "-0" {
		return "0"
	}
	return s
}
