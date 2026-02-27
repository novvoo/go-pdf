package gopdf

import (
	"bytes"
	"fmt"
	"os"
	"strings"
	"unsafe"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
	pdfcpufont "github.com/pdfcpu/pdfcpu/pkg/pdfcpu/font"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

func ConvertSVGToPDF(svgPath, pdfPath string) error {
	if svgPath == "" || pdfPath == "" {
		return fmt.Errorf("missing svgPath or pdfPath")
	}

	f, err := os.Open(svgPath)
	if err != nil {
		return err
	}
	defer f.Close()

	pdfCtx := NewPDFGeneratorContext()
	if err := DrawSVG(f, pdfCtx); err != nil {
		return err
	}

	if pdfCtx.pageCount == 0 {
		return fmt.Errorf("no pages generated")
	}

	// Create PDF using pdfcpu
	width, height := pdfCtx.width, pdfCtx.height
	if width <= 0 {
		width = 595.28 // A4 width
	}
	if height <= 0 {
		height = 841.89 // A4 height
	}

	ctx, pageRefs, contentRefs, err := createBlankPDFContextForSVG(width, height, pdfCtx.pageCount)
	if err != nil {
		return err
	}

	fm := model.FontMap{
		"Helvetica":  model.FontResource{Res: model.Resource{ID: "F1"}},
		"serif":      model.FontResource{Res: model.Resource{ID: "F1"}},
		"sans":       model.FontResource{Res: model.Resource{ID: "F1"}},
		"sans-serif": model.FontResource{Res: model.Resource{ID: "F1"}},
		"sans-cjk":   model.FontResource{Res: model.Resource{ID: "F1"}},
		"monospace":  model.FontResource{Res: model.Resource{ID: "F1"}},
		"math":       model.FontResource{Res: model.Resource{ID: "F1"}},
	}
	fontDict, err := pdfcpufont.FontResources(ctx.XRefTable, fm)
	if err != nil {
		return err
	}
	if err := attachFontResources(ctx, pageRefs, fontDict); err != nil {
		return err
	}

	for page := 1; page <= pdfCtx.pageCount; page++ {
		buf := pdfCtx.buffers[page]
		if buf == nil {
			buf = &bytes.Buffer{}
		}

		// Prepend transformation to flip Y axis (PDF is bottom-up, SVG is top-down)
		// DrawSVG output is in SVG coordinates.
		// We need to wrap it: q 1 0 0 -1 0 height cm ... Q
		var wrapper bytes.Buffer
		wrapper.WriteString(fmt.Sprintf("q 1 0 0 -1 0 %.4f cm\n", height))
		wrapper.Write(buf.Bytes())
		wrapper.WriteString("Q\n")

		if page-1 < len(contentRefs) {
			if err := writePageContentStream(ctx, contentRefs[page-1], wrapper.Bytes()); err != nil {
				return err
			}
		}
	}

	if err := ensureDirForFile(pdfPath); err != nil {
		return err
	}
	return api.WriteContextFile(ctx, pdfPath)
}

// PDFGeneratorContext implements Context and PageAwareContext
type PDFGeneratorContext struct {
	buffers    map[int]*bytes.Buffer
	currPage   int
	currBuf    *bytes.Buffer
	width      float64
	height     float64
	pageCount  int
	currentPat Pattern
}

func NewPDFGeneratorContext() *PDFGeneratorContext {
	ctx := &PDFGeneratorContext{
		buffers:  make(map[int]*bytes.Buffer),
		currPage: 1,
	}
	ctx.currBuf = &bytes.Buffer{}
	ctx.buffers[1] = ctx.currBuf
	ctx.pageCount = 1
	return ctx
}

func (c *PDFGeneratorContext) SetPage(n int) {
	if n < 1 {
		n = 1
	}
	c.currPage = n
	if n > c.pageCount {
		c.pageCount = n
	}
	if _, ok := c.buffers[n]; !ok {
		c.buffers[n] = &bytes.Buffer{}
	}
	c.currBuf = c.buffers[n]
}

func (c *PDFGeneratorContext) w(s string) {
	c.currBuf.WriteString(s)
}

func (c *PDFGeneratorContext) wf(format string, a ...interface{}) {
	c.currBuf.WriteString(fmt.Sprintf(format, a...))
}

// Context interface implementation

func (c *PDFGeneratorContext) Reference() Context      { return c }
func (c *PDFGeneratorContext) Destroy()                {}
func (c *PDFGeneratorContext) GetReferenceCount() int  { return 1 }
func (c *PDFGeneratorContext) Status() Status          { return StatusSuccess }
func (c *PDFGeneratorContext) GetTarget() Surface      { return nil }
func (c *PDFGeneratorContext) GetGroupTarget() Surface { return nil }
func (c *PDFGeneratorContext) SetUserData(key *UserDataKey, userData unsafe.Pointer, destroy DestroyFunc) Status {
	return StatusSuccess
}
func (c *PDFGeneratorContext) GetUserData(key *UserDataKey) unsafe.Pointer { return nil }

func (c *PDFGeneratorContext) Save() error {
	c.w("q\n")
	return nil
}

func (c *PDFGeneratorContext) Restore() error {
	c.w("Q\n")
	return nil
}

func (c *PDFGeneratorContext) PushGroup()                           {}
func (c *PDFGeneratorContext) PushGroupWithContent(content Content) {}
func (c *PDFGeneratorContext) PopGroup() Pattern                    { return nil }
func (c *PDFGeneratorContext) PopGroupToSource()                    {}

func (c *PDFGeneratorContext) Paint() error                                            { return nil }
func (c *PDFGeneratorContext) PaintWithAlpha(alpha float64) error                      { return nil }
func (c *PDFGeneratorContext) Mask(pattern Pattern)                                    {}
func (c *PDFGeneratorContext) MaskSurface(surface Surface, surfaceX, surfaceY float64) {}

func (c *PDFGeneratorContext) Stroke() error {
	c.applyStrokeColor()
	c.w("S\n")
	return nil
}
func (c *PDFGeneratorContext) StrokePreserve() error {
	c.applyStrokeColor()
	c.w("S\n") // PDF S operator ends the path, so preserve is tricky. 's' is close and stroke. 'S' is stroke. 'n' is no-op.
	// PDF doesn't support "stroke preserve" natively in one op easily without path re-construction?
	// Actually 'S' clears the path.
	// To preserve, we'd need to not clear it.
	// But standard PDF operators consume the path.
	// We might need to duplicate the path construction if we really want to preserve.
	// For now, treat as Stroke.
	return nil
}
func (c *PDFGeneratorContext) Fill() error {
	c.applyFillColor()
	c.w("f\n")
	return nil
}
func (c *PDFGeneratorContext) FillPreserve() error {
	c.applyFillColor()
	c.w("f\n") // Same issue as StrokePreserve
	return nil
}

func (c *PDFGeneratorContext) SetSource(source Pattern) {
	c.currentPat = source
}
func (c *PDFGeneratorContext) SetSourceRGB(r, g, b float64) {
	c.currentPat = NewPatternRGB(r, g, b)
}
func (c *PDFGeneratorContext) SetSourceRGBA(r, g, b, a float64) {
	c.currentPat = NewPatternRGBA(r, g, b, a)
}
func (c *PDFGeneratorContext) SetSourceSurface(surface Surface, x, y float64) {}
func (c *PDFGeneratorContext) GetSource() Pattern                             { return c.currentPat }

func (c *PDFGeneratorContext) applyFillColor() {
	if c.currentPat == nil {
		return
	}
	if sp, ok := c.currentPat.(SolidPattern); ok {
		r, g, b, _ := sp.GetRGBA()
		c.wf("%.6f %.6f %.6f rg\n", r, g, b)
	}
}

func (c *PDFGeneratorContext) applyStrokeColor() {
	if c.currentPat == nil {
		return
	}
	if sp, ok := c.currentPat.(SolidPattern); ok {
		r, g, b, _ := sp.GetRGBA()
		c.wf("%.6f %.6f %.6f RG\n", r, g, b)
	}
}

func (c *PDFGeneratorContext) SetOperator(op Operator)          {}
func (c *PDFGeneratorContext) GetOperator() Operator            { return OperatorOver }
func (c *PDFGeneratorContext) SetTolerance(tolerance float64)   {}
func (c *PDFGeneratorContext) GetTolerance() float64            { return 0.1 }
func (c *PDFGeneratorContext) SetAntialias(antialias Antialias) {}
func (c *PDFGeneratorContext) GetAntialias() Antialias          { return AntialiasDefault }
func (c *PDFGeneratorContext) SetFillRule(fillRule FillRule)    {}
func (c *PDFGeneratorContext) GetFillRule() FillRule            { return FillRuleWinding }

func (c *PDFGeneratorContext) SetLineWidth(width float64) {
	c.wf("%.4f w\n", width)
}
func (c *PDFGeneratorContext) GetLineWidth() float64 { return 1 }

func (c *PDFGeneratorContext) SetLineCap(lineCap LineCap) {
	lc := 0
	switch lineCap {
	case LineCapButt:
		lc = 0
	case LineCapRound:
		lc = 1
	case LineCapSquare:
		lc = 2
	}
	c.wf("%d J\n", lc)
}
func (c *PDFGeneratorContext) GetLineCap() LineCap { return LineCapButt }

func (c *PDFGeneratorContext) SetLineJoin(lineJoin LineJoin) {
	lj := 0
	switch lineJoin {
	case LineJoinMiter:
		lj = 0
	case LineJoinRound:
		lj = 1
	case LineJoinBevel:
		lj = 2
	}
	c.wf("%d j\n", lj)
}
func (c *PDFGeneratorContext) GetLineJoin() LineJoin { return LineJoinMiter }

func (c *PDFGeneratorContext) SetDash(dashes []float64, offset float64) {
	c.w("[")
	for i, v := range dashes {
		if i > 0 {
			c.w(" ")
		}
		c.wf("%.4f", v)
	}
	c.wf("] %.4f d\n", offset)
}
func (c *PDFGeneratorContext) GetDashCount() int             { return 0 }
func (c *PDFGeneratorContext) GetDash() ([]float64, float64) { return nil, 0 }

func (c *PDFGeneratorContext) SetMiterLimit(limit float64) {
	c.wf("%.4f M\n", limit)
}
func (c *PDFGeneratorContext) GetMiterLimit() float64 { return 10 }

func (c *PDFGeneratorContext) Translate(tx, ty float64) {
	c.wf("1 0 0 1 %.4f %.4f cm\n", tx, ty)
}
func (c *PDFGeneratorContext) Scale(sx, sy float64) {
	c.wf("%.4f 0 0 %.4f 0 0 cm\n", sx, sy)
}
func (c *PDFGeneratorContext) Rotate(angle float64) {
	// Not implemented in simple DrawSVG use case usually (handled by Transform)
}
func (c *PDFGeneratorContext) Transform(matrix *Matrix) {
	c.wf("%.4f %.4f %.4f %.4f %.4f %.4f cm\n", matrix.XX, matrix.YX, matrix.XY, matrix.YY, matrix.X0, matrix.Y0)
}
func (c *PDFGeneratorContext) SetMatrix(matrix *Matrix) {}
func (c *PDFGeneratorContext) GetMatrix() *Matrix       { return &Matrix{XX: 1, YY: 1} }
func (c *PDFGeneratorContext) IdentityMatrix()          {}

func (c *PDFGeneratorContext) UserToDevice(x, y float64) (float64, float64)           { return x, y }
func (c *PDFGeneratorContext) UserToDeviceDistance(dx, dy float64) (float64, float64) { return dx, dy }
func (c *PDFGeneratorContext) DeviceToUser(x, y float64) (float64, float64)           { return x, y }
func (c *PDFGeneratorContext) DeviceToUserDistance(dx, dy float64) (float64, float64) { return dx, dy }

func (c *PDFGeneratorContext) NewPath() {
	// Implicitly started by drawing ops
}
func (c *PDFGeneratorContext) MoveTo(x, y float64) {
	c.wf("%.4f %.4f m\n", x, y)
}
func (c *PDFGeneratorContext) NewSubPath() {}
func (c *PDFGeneratorContext) LineTo(x, y float64) {
	c.wf("%.4f %.4f l\n", x, y)
}
func (c *PDFGeneratorContext) CurveTo(x1, y1, x2, y2, x3, y3 float64) {
	c.wf("%.4f %.4f %.4f %.4f %.4f %.4f c\n", x1, y1, x2, y2, x3, y3)
}
func (c *PDFGeneratorContext) Arc(xc, yc, radius, angle1, angle2 float64)         {}
func (c *PDFGeneratorContext) ArcNegative(xc, yc, radius, angle1, angle2 float64) {}
func (c *PDFGeneratorContext) RelMoveTo(dx, dy float64)                           {}
func (c *PDFGeneratorContext) RelLineTo(dx, dy float64)                           {}
func (c *PDFGeneratorContext) RelCurveTo(dx1, dy1, dx2, dy2, dx3, dy3 float64)    {}
func (c *PDFGeneratorContext) Rectangle(x, y, width, height float64) {
	c.wf("%.4f %.4f %.4f %.4f re\n", x, y, width, height)
}
func (c *PDFGeneratorContext) DrawCircle(xc, yc, radius float64) {
	// Approximate circle with 4 Bezier curves
	magic := 0.551915024494
	offset := radius * magic
	c.MoveTo(xc+radius, yc)
	c.CurveTo(xc+radius, yc+offset, xc+offset, yc+radius, xc, yc+radius)
	c.CurveTo(xc-offset, yc+radius, xc-radius, yc+offset, xc-radius, yc)
	c.CurveTo(xc-radius, yc-offset, xc-offset, yc-radius, xc, yc-radius)
	c.CurveTo(xc+offset, yc-radius, xc+radius, yc-offset, xc+radius, yc)
	c.ClosePath()
}
func (c *PDFGeneratorContext) ClosePath() {
	c.w("h\n")
}
func (c *PDFGeneratorContext) PathExtents() (x1, y1, x2, y2 float64) { return 0, 0, 0, 0 }

func (c *PDFGeneratorContext) Clip()                                   {}
func (c *PDFGeneratorContext) ClipPreserve()                           {}
func (c *PDFGeneratorContext) ClipExtents() (x1, y1, x2, y2 float64)   { return 0, 0, 0, 0 }
func (c *PDFGeneratorContext) InClip(x, y float64) Bool                { return 1 }
func (c *PDFGeneratorContext) ResetClip()                              {}
func (c *PDFGeneratorContext) CopyClipRectangleList() *RectangleList   { return nil }
func (c *PDFGeneratorContext) InStroke(x, y float64) Bool              { return 0 }
func (c *PDFGeneratorContext) InFill(x, y float64) Bool                { return 0 }
func (c *PDFGeneratorContext) StrokeExtents() (x1, y1, x2, y2 float64) { return 0, 0, 0, 0 }
func (c *PDFGeneratorContext) FillExtents() (x1, y1, x2, y2 float64)   { return 0, 0, 0, 0 }
func (c *PDFGeneratorContext) HasCurrentPoint() Bool                   { return 0 }
func (c *PDFGeneratorContext) GetCurrentPoint() (x, y float64)         { return 0, 0 }

func (c *PDFGeneratorContext) CopyPath() *Path     { return nil }
func (c *PDFGeneratorContext) CopyPathFlat() *Path { return nil }
func (c *PDFGeneratorContext) AppendPath(path *Path) {
	for _, pd := range path.Data {
		switch pd.Type {
		case PathMoveTo:
			c.MoveTo(pd.Points[0].X, pd.Points[0].Y)
		case PathLineTo:
			c.LineTo(pd.Points[0].X, pd.Points[0].Y)
		case PathCurveTo:
			c.CurveTo(pd.Points[0].X, pd.Points[0].Y, pd.Points[1].X, pd.Points[1].Y, pd.Points[2].X, pd.Points[2].Y)
		case PathClosePath:
			c.ClosePath()
		}
	}
}

func (c *PDFGeneratorContext) ShowGlyphs(glyphs []Glyph) {}
func (c *PDFGeneratorContext) ShowTextGlyphs(utf8 string, glyphs []Glyph, clusters []TextCluster, clusterFlags TextClusterFlags) {
}
func (c *PDFGeneratorContext) GlyphPath(glyphs []Glyph)                 {}
func (c *PDFGeneratorContext) TextExtents(utf8 string) *TextExtents     { return nil }
func (c *PDFGeneratorContext) GlyphExtents(glyphs []Glyph) *TextExtents { return nil }

func (c *PDFGeneratorContext) SetFontMatrix(matrix *Matrix)        {}
func (c *PDFGeneratorContext) GetFontMatrix() *Matrix              { return nil }
func (c *PDFGeneratorContext) SetFontOptions(options *FontOptions) {}
func (c *PDFGeneratorContext) GetFontOptions() *FontOptions        { return nil }
func (c *PDFGeneratorContext) SetFontFace(fontFace FontFace)       {}
func (c *PDFGeneratorContext) GetFontFace() FontFace               { return nil }
func (c *PDFGeneratorContext) SetScaledFont(scaledFont ScaledFont) {}
func (c *PDFGeneratorContext) GetScaledFont() ScaledFont           { return nil }
func (c *PDFGeneratorContext) FontExtents() *FontExtents           { return nil }

func (c *PDFGeneratorContext) PangoPdfCreateLayout() interface{}       { return nil }
func (c *PDFGeneratorContext) PangoPdfUpdateLayout(layout interface{}) {}
func (c *PDFGeneratorContext) PangoPdfShowText(layout interface{})     {}

// --- Helper functions from original file ---

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
		return nil, nil, nil, fmt.Errorf("pages is not an indirect ref")
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
	for _, pageRef := range pageRefs {
		pageDict, err := ctx.XRefTable.DereferenceDict(pageRef)
		if err != nil {
			return err
		}
		resObj, found := pageDict.Find("Resources")
		var resDict types.Dict
		if found {
			resDict, err = ctx.XRefTable.DereferenceDict(resObj)
			if err != nil {
				return err
			}
		} else {
			resDict = types.Dict{}
		}
		resDict["Font"] = fontDict
		pageDict["Resources"] = resDict
	}
	return nil
}

// ensureDirForFile is defined in postscript_to_pdf.go

// escapePDFString is currently unused
func _(s string) string {
	s = strings.ReplaceAll(s, "\\", "\\\\")
	s = strings.ReplaceAll(s, "(", "\\(")
	s = strings.ReplaceAll(s, ")", "\\)")
	return s
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
