package gopdf

import (
	"fmt"
	"math"
	"strings"
	"sync/atomic"
	"unsafe"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/opentype/api"
	apifont "github.com/go-text/typesetting/opentype/api/font"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/math/fixed"
)

// PangoPdfFontMap represents a Pango font map integrated with Gopdf
type PangoPdfFontMap struct {
	refCount int32
	status   Status
	userData map[*UserDataKey]interface{}
}

// PangoPdfFont represents a Pango font integrated with Gopdf
type PangoPdfFont struct {
	baseFontFace
	family   string
	slant    FontSlant
	weight   FontWeight
	realFace font.Face
	fontData []byte
}

// PangoPdfFontMetrics represents font metrics in PangoPdf
type PangoPdfFontMetrics struct {
	refCount       int32
	status         Status
	ascent         float64
	descent        float64
	height         float64
	lineGap        float64
	underlinePos   float64
	underlineThick float64
	// strikethroughPos and strikethroughThick are reserved for future use
	_ float64 // strikethroughPos
	_ float64 // strikethroughThick
}

// PangoPdfLayout represents a Pango layout for text arrangement
type PangoPdfLayout struct {
	refCount    int32
	status      Status
	context     *PangoPdfContext
	text        string
	fontDesc    *PangoFontDescription
	width       int
	height      int
	wrap        PangoWrapMode
	ellipsize   PangoEllipsizeMode
	align       PangoAlignment
	spacing     float64
	lineSpacing float64
	userData    map[*UserDataKey]interface{}
}

// PangoPdfContext represents a Pango context integrated with Gopdf
type PangoPdfContext struct {
	refCount int32
	status   Status
	fontMap  *PangoPdfFontMap
	baseDir  PangoDirection
	userData map[*UserDataKey]interface{}
}

// PangoFontDescription describes a font in Pango
type PangoFontDescription struct {
	family  string
	style   PangoStyle
	variant PangoVariant
	weight  PangoWeight
	stretch PangoStretch
	size    float64
}

// Enumerations for PangoPdf

type PangoDirection int
type PangoStyle int
type PangoVariant int
type PangoWeight int
type PangoStretch int
type PangoWrapMode int
type PangoEllipsizeMode int
type PangoAlignment int
type PangoAttrType int

const (
	PangoDirectionLTR PangoDirection = iota
	PangoDirectionRTL
	PangoDirectionTTB
	PangoDirectionBTT
)

const (
	PangoStyleNormal PangoStyle = iota
	PangoStyleOblique
	PangoStyleItalic
)

const (
	PangoVariantNormal PangoVariant = iota
	PangoVariantSmallCaps
)

const (
	PangoWeightThin PangoWeight = 100 + iota*100
	PangoWeightUltraLight
	PangoWeightLight
	PangoWeightSemiLight
	PangoWeightBook
	PangoWeightNormal
	PangoWeightMedium
	PangoWeightSemiBold
	PangoWeightBold
	PangoWeightUltraBold
	PangoWeightHeavy
	PangoWeightUltraHeavy
)

const (
	PangoStretchUltraCondensed PangoStretch = iota
	PangoStretchExtraCondensed
	PangoStretchCondensed
	PangoStretchSemiCondensed
	PangoStretchNormal
	PangoStretchSemiExpanded
	PangoStretchExpanded
	PangoStretchExtraExpanded
	PangoStretchUltraExpanded
)

const (
	PangoWrapWord PangoWrapMode = iota
	PangoWrapChar
	PangoWrapWordChar
)

const (
	PangoEllipsizeNone PangoEllipsizeMode = iota
	PangoEllipsizeStart
	PangoEllipsizeMiddle
	PangoEllipsizeEnd
)

const (
	PangoAlignLeft PangoAlignment = iota
	PangoAlignCenter
	PangoAlignRight
)

const (
	PangoAttrInvalid PangoAttrType = iota
	PangoAttrLanguage
	PangoAttrFamily
	PangoAttrStyle
	PangoAttrWeight
	PangoAttrVariant
	PangoAttrStretch
	PangoAttrSize
	PangoAttrFontDesc
	PangoAttrForeground
	PangoAttrBackground
	PangoAttrUnderline
	PangoAttrStrikethrough
	PangoAttrRise
	PangoAttrShape
	PangoAttrScale
	PangoAttrFallback
	PangoAttrLetterSpacing
	PangoAttrFontFeatures
	PangoAttrForegroundAlpha
	PangoAttrBackgroundAlpha
	PangoAttrAllowBreaks
	PangoAttrShow
	PangoAttrInsertHyphens
	PangoAttrOverline
)

// PangoPdfScaledFont represents a scaled font in PangoPdf
type PangoPdfScaledFont struct {
	refCount    int32
	status      Status
	fontType    FontType
	fontFace    FontFace
	fontMatrix  Matrix
	ctm         Matrix
	scaleMatrix Matrix
	options     *FontOptions
	pangoFont   *PangoPdfFont
}

// NewPangoPdfFontMap creates a new Pango font map integrated with Gopdf
func NewPangoPdfFontMap() *PangoPdfFontMap {
	return &PangoPdfFontMap{
		refCount: 1,
		status:   StatusSuccess,
		userData: make(map[*UserDataKey]interface{}),
	}
}

// Reference management for PangoPdfFontMap
func (fm *PangoPdfFontMap) Reference() *PangoPdfFontMap {
	atomic.AddInt32(&fm.refCount, 1)
	return fm
}

func (fm *PangoPdfFontMap) Destroy() {
	if atomic.AddInt32(&fm.refCount, -1) == 0 {
		// Cleanup resources if needed
	}
}

func (fm *PangoPdfFontMap) GetReferenceCount() int {
	return int(atomic.LoadInt32(&fm.refCount))
}

func (fm *PangoPdfFontMap) Status() Status {
	return fm.status
}

// UserData management for PangoPdfFontMap
func (fm *PangoPdfFontMap) SetUserData(key *UserDataKey, userData unsafe.Pointer, destroy DestroyFunc) Status {
	if fm.status != StatusSuccess {
		return fm.status
	}
	if fm.userData == nil {
		fm.userData = make(map[*UserDataKey]interface{})
	}
	fm.userData[key] = userData
	_ = destroy // destroy func is currently ignored
	return StatusSuccess
}

func (fm *PangoPdfFontMap) GetUserData(key *UserDataKey) unsafe.Pointer {
	if fm.userData == nil {
		return nil
	}
	if data, ok := fm.userData[key]; ok {
		return data.(unsafe.Pointer)
	}
	return nil
}

// NewPangoPdfFont creates a new Pango font integrated with Gopdf
func NewPangoPdfFont(family string, slant FontSlant, weight FontWeight) *PangoPdfFont {
	pf := &PangoPdfFont{
		baseFontFace: baseFontFace{
			refCount: 1,
			status:   StatusSuccess,
			fontType: FontTypeUser,
			userData: make(map[*UserDataKey]interface{}),
		},
		family: family,
		slant:  slant,
		weight: weight,
	}

	// Get font key and load font
	fontKey := getFontKey(family, slant, weight)
	face, data, err := LoadEmbeddedFont(fontKey)
	if err != nil {
		// Try loading from assets if family looks like a file
		if strings.Contains(family, "/") || strings.Contains(family, "\\") {
			face, data, err = LoadFontFromFile(family)
		}
		if err != nil {
			// Final fallback to default font
			face, data = GetDefaultFont()
		}
	}

	pf.realFace = face
	pf.fontData = data

	if pf.realFace == nil {
		pf.status = StatusFontTypeMismatch
	}
	return pf
}

// FontFace interface implementation for PangoPdfFont
func (f *PangoPdfFont) Reference() FontFace {
	atomic.AddInt32(&f.refCount, 1)
	return f
}

func (f *PangoPdfFont) Destroy() {
	if atomic.AddInt32(&f.refCount, -1) == 0 {
		// nothing to free at the moment
	}
}

func (f *PangoPdfFont) GetReferenceCount() int {
	return int(atomic.LoadInt32(&f.refCount))
}

func (f *PangoPdfFont) Status() Status {
	return f.status
}

func (f *PangoPdfFont) GetType() FontType {
	return f.fontType
}

func (f *PangoPdfFont) SetUserData(key *UserDataKey, userData unsafe.Pointer, destroy DestroyFunc) Status {
	if f.status != StatusSuccess {
		return f.status
	}
	if f.userData == nil {
		f.userData = make(map[*UserDataKey]interface{})
	}
	f.userData[key] = userData
	_ = destroy // destroy func is currently ignored
	return StatusSuccess
}

func (f *PangoPdfFont) GetUserData(key *UserDataKey) unsafe.Pointer {
	if f.userData == nil {
		return nil
	}
	if data, ok := f.userData[key]; ok {
		return data.(unsafe.Pointer)
	}
	return nil
}

// NewPangoPdfFontMetrics creates new font metrics
func NewPangoPdfFontMetrics(ascent, descent, height, lineGap float64) *PangoPdfFontMetrics {
	return &PangoPdfFontMetrics{
		refCount:       1,
		status:         StatusSuccess,
		ascent:         ascent,
		descent:        descent,
		height:         height,
		lineGap:        lineGap,
		underlinePos:   -descent * 0.5,
		underlineThick: (ascent + descent) * 0.05,
	}
}

// Reference management for PangoPdfFontMetrics
func (fm *PangoPdfFontMetrics) Reference() *PangoPdfFontMetrics {
	atomic.AddInt32(&fm.refCount, 1)
	return fm
}

func (fm *PangoPdfFontMetrics) Destroy() {
	if atomic.AddInt32(&fm.refCount, -1) == 0 {
		// Cleanup resources if needed
	}
}

func (fm *PangoPdfFontMetrics) GetReferenceCount() int {
	return int(atomic.LoadInt32(&fm.refCount))
}

func (fm *PangoPdfFontMetrics) Status() Status {
	return fm.status
}

// Metric getters
func (fm *PangoPdfFontMetrics) GetAscent() float64 {
	return fm.ascent
}

func (fm *PangoPdfFontMetrics) GetDescent() float64 {
	return fm.descent
}

func (fm *PangoPdfFontMetrics) GetHeight() float64 {
	return fm.height
}

func (fm *PangoPdfFontMetrics) GetLineGap() float64 {
	return fm.lineGap
}

func (fm *PangoPdfFontMetrics) GetUnderlinePosition() float64 {
	return fm.underlinePos
}

func (fm *PangoPdfFontMetrics) GetUnderlineThickness() float64 {
	return fm.underlineThick
}

// NewPangoPdfLayout creates a new Pango layout
func NewPangoPdfLayout(context *PangoPdfContext) *PangoPdfLayout {
	return &PangoPdfLayout{
		refCount: 1,
		status:   StatusSuccess,
		context:  context,
		width:    -1, // Unset
		height:   -1, // Unset
		wrap:     PangoWrapWord,
		align:    PangoAlignLeft,
		userData: make(map[*UserDataKey]interface{}),
	}
}

// Reference management for PangoPdfLayout
func (l *PangoPdfLayout) Reference() *PangoPdfLayout {
	atomic.AddInt32(&l.refCount, 1)
	return l
}

func (l *PangoPdfLayout) Destroy() {
	if atomic.AddInt32(&l.refCount, -1) == 0 {
		if l.context != nil {
			l.context.Destroy()
		}
		if l.fontDesc != nil {
			// Destroy font description if needed
		}
	}
}

func (l *PangoPdfLayout) GetReferenceCount() int {
	return int(atomic.LoadInt32(&l.refCount))
}

func (l *PangoPdfLayout) Status() Status {
	return l.status
}

// Layout property setters and getters
func (l *PangoPdfLayout) SetText(text string) {
	l.text = text
}

func (l *PangoPdfLayout) GetText() string {
	return l.text
}

func (l *PangoPdfLayout) SetFontDescription(desc *PangoFontDescription) {
	l.fontDesc = desc
}

func (l *PangoPdfLayout) GetFontDescription() *PangoFontDescription {
	return l.fontDesc
}

func (l *PangoPdfLayout) SetWidth(width int) {
	l.width = width
}

func (l *PangoPdfLayout) GetWidth() int {
	return l.width
}

func (l *PangoPdfLayout) SetHeight(height int) {
	l.height = height
}

func (l *PangoPdfLayout) GetHeight() int {
	return l.height
}

func (l *PangoPdfLayout) SetWrap(wrap PangoWrapMode) {
	l.wrap = wrap
}

func (l *PangoPdfLayout) GetWrap() PangoWrapMode {
	return l.wrap
}

func (l *PangoPdfLayout) SetEllipsize(ellipsize PangoEllipsizeMode) {
	l.ellipsize = ellipsize
}

func (l *PangoPdfLayout) GetEllipsize() PangoEllipsizeMode {
	return l.ellipsize
}

func (l *PangoPdfLayout) SetAlignment(align PangoAlignment) {
	l.align = align
}

func (l *PangoPdfLayout) GetAlignment() PangoAlignment {
	return l.align
}

func (l *PangoPdfLayout) SetSpacing(spacing float64) {
	l.spacing = spacing
}

func (l *PangoPdfLayout) GetSpacing() float64 {
	return l.spacing
}

func (l *PangoPdfLayout) SetLineSpacing(lineSpacing float64) {
	l.lineSpacing = lineSpacing
}

func (l *PangoPdfLayout) GetLineSpacing() float64 {
	return l.lineSpacing
}

// UserData management for PangoPdfLayout
func (l *PangoPdfLayout) SetUserData(key *UserDataKey, userData unsafe.Pointer, destroy DestroyFunc) Status {
	if l.status != StatusSuccess {
		return l.status
	}
	if l.userData == nil {
		l.userData = make(map[*UserDataKey]interface{})
	}
	l.userData[key] = userData
	_ = destroy // destroy func is currently ignored
	return StatusSuccess
}

func (l *PangoPdfLayout) GetUserData(key *UserDataKey) unsafe.Pointer {
	if l.userData == nil {
		return nil
	}
	if data, ok := l.userData[key]; ok {
		return data.(unsafe.Pointer)
	}
	return nil
}

// NewPangoPdfContext creates a new Pango context integrated with Gopdf
func NewPangoPdfContext(fontMap *PangoPdfFontMap) *PangoPdfContext {
	return &PangoPdfContext{
		refCount: 1,
		status:   StatusSuccess,
		fontMap:  fontMap,
		baseDir:  PangoDirectionLTR,
		userData: make(map[*UserDataKey]interface{}),
	}
}

// Reference management for PangoPdfContext
func (c *PangoPdfContext) Reference() *PangoPdfContext {
	atomic.AddInt32(&c.refCount, 1)
	return c
}

func (c *PangoPdfContext) Destroy() {
	if atomic.AddInt32(&c.refCount, -1) == 0 {
		if c.fontMap != nil {
			c.fontMap.Destroy()
		}
	}
}

func (c *PangoPdfContext) GetReferenceCount() int {
	return int(atomic.LoadInt32(&c.refCount))
}

func (c *PangoPdfContext) Status() Status {
	return c.status
}

// Context property setters and getters
func (c *PangoPdfContext) SetFontMap(fontMap *PangoPdfFontMap) {
	if c.fontMap != nil {
		c.fontMap.Destroy()
	}
	c.fontMap = fontMap.Reference()
}

func (c *PangoPdfContext) GetFontMap() *PangoPdfFontMap {
	return c.fontMap.Reference()
}

func (c *PangoPdfContext) SetBaseDir(direction PangoDirection) {
	c.baseDir = direction
}

func (c *PangoPdfContext) GetBaseDir() PangoDirection {
	return c.baseDir
}

// UserData management for PangoPdfContext
func (c *PangoPdfContext) SetUserData(key *UserDataKey, userData unsafe.Pointer, destroy DestroyFunc) Status {
	if c.status != StatusSuccess {
		return c.status
	}
	if c.userData == nil {
		c.userData = make(map[*UserDataKey]interface{})
	}
	c.userData[key] = userData
	_ = destroy // destroy func is currently ignored
	return StatusSuccess
}

func (c *PangoPdfContext) GetUserData(key *UserDataKey) unsafe.Pointer {
	if c.userData == nil {
		return nil
	}
	if data, ok := c.userData[key]; ok {
		return data.(unsafe.Pointer)
	}
	return nil
}

// NewPangoFontDescription creates a new font description
func NewPangoFontDescription() *PangoFontDescription {
	return &PangoFontDescription{
		family:  "sans",
		style:   PangoStyleNormal,
		variant: PangoVariantNormal,
		weight:  PangoWeightNormal,
		stretch: PangoStretchNormal,
		size:    12.0, // Default size in points
	}
}

// FontDescription property setters and getters
func (fd *PangoFontDescription) SetFamily(family string) {
	fd.family = family
}

func (fd *PangoFontDescription) GetFamily() string {
	return fd.family
}

func (fd *PangoFontDescription) SetStyle(style PangoStyle) {
	fd.style = style
}

func (fd *PangoFontDescription) GetStyle() PangoStyle {
	return fd.style
}

func (fd *PangoFontDescription) SetWeight(weight PangoWeight) {
	fd.weight = weight
}

func (fd *PangoFontDescription) GetWeight() PangoWeight {
	return fd.weight
}

func (fd *PangoFontDescription) SetStretch(stretch PangoStretch) {
	fd.stretch = stretch
}

func (fd *PangoFontDescription) GetStretch() PangoStretch {
	return fd.stretch
}

func (fd *PangoFontDescription) SetSize(size float64) {
	fd.size = size
}

func (fd *PangoFontDescription) GetSize() float64 {
	return fd.size
}

// NewPangoPdfScaledFont creates a new scaled font for PangoPdf
func NewPangoPdfScaledFont(fontFace FontFace, fontMatrix, ctm *Matrix, options *FontOptions) *PangoPdfScaledFont {
	sf := &PangoPdfScaledFont{
		refCount: 1,
		status:   StatusSuccess,
		fontType: FontTypeUser,
		fontFace: fontFace.Reference(),
		options:  options,
	}

	if fontMatrix != nil {
		sf.fontMatrix = *fontMatrix
	} else {
		sf.fontMatrix = *NewMatrix()
	}
	if ctm != nil {
		sf.ctm = *ctm
	} else {
		sf.ctm = *NewMatrix()
	}
	// For our implementation we just copy fontMatrix into scaleMatrix.
	sf.scaleMatrix = sf.fontMatrix

	// If the font face is a PangoPdfFont, keep a reference to it
	if pcFont, ok := fontFace.(*PangoPdfFont); ok {
		sf.pangoFont = pcFont
	}

	return sf
}

// ScaledFont interface implementation for PangoPdfScaledFont
func (s *PangoPdfScaledFont) Reference() ScaledFont {
	atomic.AddInt32(&s.refCount, 1)
	return s
}

func (s *PangoPdfScaledFont) Destroy() {
	if atomic.AddInt32(&s.refCount, -1) == 0 {
		if s.fontFace != nil {
			s.fontFace.Destroy()
		}
	}
}

func (s *PangoPdfScaledFont) GetReferenceCount() int {
	return int(atomic.LoadInt32(&s.refCount))
}

func (s *PangoPdfScaledFont) Status() Status {
	return s.status
}

func (s *PangoPdfScaledFont) GetType() FontType {
	return s.fontType
}

func (s *PangoPdfScaledFont) SetUserData(key *UserDataKey, userData unsafe.Pointer, destroy DestroyFunc) Status {
	// For now we store user data in the associated FontFace to keep things simple.
	if s.fontFace == nil {
		return StatusNullPointer
	}
	return s.fontFace.SetUserData(key, userData, destroy)
}

func (s *PangoPdfScaledFont) GetUserData(key *UserDataKey) unsafe.Pointer {
	if s.fontFace == nil {
		return nil
	}
	return s.fontFace.GetUserData(key)
}

func (s *PangoPdfScaledFont) GetFontFace() FontFace {
	if s.fontFace == nil {
		return nil
	}
	return s.fontFace.Reference()
}

func (s *PangoPdfScaledFont) GetFontMatrix() *Matrix {
	m := s.fontMatrix
	return &m
}

func (s *PangoPdfScaledFont) GetCTM() *Matrix {
	m := s.ctm
	return &m
}

func (s *PangoPdfScaledFont) GetScaleMatrix() *Matrix {
	m := s.scaleMatrix
	return &m
}

func (s *PangoPdfScaledFont) GetFontOptions() *FontOptions {
	if s.options == nil {
		return NewFontOptions()
	}
	return s.options.Copy()
}

// getRealFace returns the underlying font.Face and checks for errors.
func (s *PangoPdfScaledFont) getRealFace() (font.Face, Status) {
	if s.fontFace == nil {
		return nil, StatusNullPointer
	}

	// Try to get as PangoPdfFont first
	if pcFont, ok := s.fontFace.(*PangoPdfFont); ok && pcFont.realFace != nil {
		return pcFont.realFace, StatusSuccess
	}

	// Fall back to toy font
	toy, ok := s.fontFace.(*toyFontFace)
	if !ok || toy.realFace == nil {
		return nil, StatusFontTypeMismatch
	}
	return toy.realFace, StatusSuccess
}

// Extents returns font extents using the real font face.
func (s *PangoPdfScaledFont) Extents() *FontExtents {
	fe := &FontExtents{}

	realFace, status := s.getRealFace()
	if status != StatusSuccess {
		// Fallback to toy extents if real face is not available
		return s.toyExtentsFallback()
	}

	// Get font metrics from go-text/typesetting
	// Ascent, Descent, Height in FUnits
	metrics, _ := realFace.FontHExtents()
	ascentFUnits := float64(metrics.Ascender)
	descentFUnits := float64(metrics.Descender)
	lineGapFUnits := float64(metrics.LineGap)

	// Convert to user space units
	fe.Ascent = ascentFUnits / 64.0
	fe.Descent = -descentFUnits / 64.0 // Descent is negative in FUnits, gopdf expects positive
	fe.Height = fe.Ascent + fe.Descent + lineGapFUnits/64.0
	fe.LineGap = lineGapFUnits / 64.0

	// Max advance is a guess without shaping a string
	fe.MaxXAdvance = fe.Ascent + fe.Descent
	fe.MaxYAdvance = 0

	// Calculate underline metrics
	fe.UnderlinePosition = -fe.Descent * 0.5
	fe.UnderlineThickness = (fe.Ascent + fe.Descent) * 0.05

	// Approximate cap height and x-height
	fe.CapHeight = fe.Ascent * 0.7 // Typical ratio
	fe.XHeight = fe.Ascent * 0.5   // Typical ratio

	return fe
}

// toyExtentsFallback returns toy font extents based on the derived font size.
func (s *PangoPdfScaledFont) toyExtentsFallback() *FontExtents {
	// Use average of xx and yy scale as size; fall back to 12 if zero.
	sx := math.Hypot(s.fontMatrix.XX, s.fontMatrix.YX)
	sy := math.Hypot(s.fontMatrix.XY, s.fontMatrix.YY)
	size := (sx + sy) * 0.5
	if size == 0 {
		size = 12
	}
	fe := &FontExtents{}
	fe.Ascent = size * 0.8
	fe.Descent = size * 0.2
	fe.Height = fe.Ascent + fe.Descent
	fe.LineGap = size * 0.2 // Typical line gap
	fe.MaxXAdvance = size
	fe.MaxYAdvance = 0
	fe.UnderlinePosition = -fe.Descent * 0.5
	fe.UnderlineThickness = size * 0.05
	fe.CapHeight = fe.Ascent * 0.7 // Typical ratio
	fe.XHeight = fe.Ascent * 0.5   // Typical ratio
	return fe
}

// TextExtents computes text extents using the real font face and shaping.
func (s *PangoPdfScaledFont) TextExtents(utf8 string) *TextExtents {
	ext := &TextExtents{}

	realFace, status := s.getRealFace()
	if status != StatusSuccess {
		return s.toyTextExtentsFallback(utf8)
	}

	// Get font size from font matrix
	fontSize := math.Hypot(s.fontMatrix.XX, s.fontMatrix.YX)
	if fontSize == 0 {
		fontSize = 12.0
	}

	// 1. Shape the text with correct font size
	runes := []rune(utf8)
	input := shaping.Input{
		Text:      runes,
		RunStart:  0,
		RunEnd:    len(runes),
		Direction: di.DirectionLTR,
		Face:      realFace,
		Size:      fixed.I(int(fontSize)), // Use actual font size
	}
	output := (&shaping.HarfbuzzShaper{}).Shape(input)

	// Calculate total advance and bounds
	var totalAdvance fixed.Int26_6
	var curX float64 // Current X position for glyph placement
	var minX, minY, maxX, maxY float64
	firstGlyph := true

	// Get units per em for coordinate conversion
	unitsPerEm := float64(realFace.Upem())
	scaleX := fontSize
	scaleY := fontSize

	for _, g := range output.Glyphs {
		// Get glyph outline for bounds calculation
		glyphData := realFace.GlyphData(api.GID(g.GlyphID))
		if outline, ok := glyphData.(api.GlyphOutline); ok {
			// Convert outline points from font units to user space
			for _, seg := range outline.Segments {
				for _, arg := range seg.Args {
					// Coordinates are in font units, convert to user space
					xInFontUnits := float64(arg.X)
					yInFontUnits := float64(arg.Y)

					x := (xInFontUnits / unitsPerEm) * scaleX
					y := (yInFontUnits / unitsPerEm) * scaleY

					// Apply Y flip to match rendering
					y = -y

					// Add glyph position (current X + offset)
					x += curX + float64(g.XOffset)/64.0
					y -= float64(g.YOffset) / 64.0 // Subtract for Y offset

					// For the first glyph, initialize bounds
					if firstGlyph {
						minX, maxX = x, x
						minY, maxY = y, y
						firstGlyph = false
					} else {
						if x < minX {
							minX = x
						}
						if x > maxX {
							maxX = x
						}
						if y < minY {
							minY = y
						}
						if y > maxY {
							maxY = y
						}
					}
				}
			}
		}

		// Advance to next glyph position
		curX += float64(g.XAdvance) / 64.0
		totalAdvance += g.XAdvance
	}

	// Convert to user space units
	ext.XAdvance = float64(totalAdvance) / 64.0
	ext.YAdvance = 0

	// Set proper width and height based on actual bounds
	ext.Width = maxX - minX
	ext.Height = maxY - minY
	ext.XBearing = minX
	ext.YBearing = minY // Already flipped, use minY directly

	return ext
}

// toyTextExtentsFallback computes naive text extents assuming fixed advance width.
func (s *PangoPdfScaledFont) toyTextExtentsFallback(utf8 string) *TextExtents {
	size := s.toyExtentsFallback().Ascent + s.toyExtentsFallback().Descent
	advancePerRune := size * 0.6

	var runeCount int
	for range utf8 {
		runeCount++
	}

	ext := &TextExtents{}
	ext.Width = float64(runeCount) * advancePerRune
	ext.Height = s.toyExtentsFallback().Height
	ext.XAdvance = ext.Width
	ext.YAdvance = 0
	ext.XBearing = 0
	ext.YBearing = -s.toyExtentsFallback().Ascent
	return ext
}

// GlyphExtents computes extents based on glyph positions.
func (s *PangoPdfScaledFont) GlyphExtents(glyphs []Glyph) *TextExtents {
	if len(glyphs) == 0 {
		return &TextExtents{}
	}
	// Assume glyph positions are advances from origin.
	last := glyphs[len(glyphs)-1]
	ext := &TextExtents{}
	ext.XAdvance = last.X
	ext.YAdvance = last.Y
	ext.Width = last.X
	ext.Height = s.Extents().Height
	ext.XBearing = 0
	ext.YBearing = -s.Extents().Ascent
	return ext
}

// GlyphPath returns the path for a single glyph ID.
func (s *PangoPdfScaledFont) GlyphPath(glyphID uint64) (*Path, error) {
	realFace, status := s.getRealFace()
	if status != StatusSuccess {
		return nil, newError(status, "failed to get real font face")
	}

	// Load the glyph outline from the font face
	gid := api.GID(glyphID)
	glyphData := realFace.GlyphData(gid)

	// Extract outline from glyph data
	outline, ok := glyphData.(api.GlyphOutline)
	if !ok {
		return nil, newError(StatusFontTypeMismatch, "glyph has no outline")
	}

	// Convert the outline to Path
	pdfPath := &Path{
		Status: StatusSuccess,
		Data:   make([]PathData, 0),
	}

	// Check if we need to flip the Y axis based on the font matrix
	// Font glyphs are designed for Y growing upward, but our coordinate system has Y growing downward.
	// Since we now use positive Y scale in font matrix, we always need to flip.
	flipY := true

	// Get font units per em and scale factor for coordinate transformation
	unitsPerEm := float64(realFace.Upem())
	scaleX := math.Hypot(s.fontMatrix.XX, s.fontMatrix.YX)
	scaleY := math.Hypot(s.fontMatrix.XY, s.fontMatrix.YY)
	if scaleX == 0 {
		scaleX = 1.0
	}
	if scaleY == 0 {
		scaleY = 1.0
	}

	// Iterate over the path segments
	// Note: The outline coordinates from go-text/typesetting are in font units (float32)
	// We need to scale them to user space and preserve the segment types
	for _, seg := range outline.Segments {
		var pd PathData

		switch seg.Op {
		case api.SegmentOpMoveTo:
			// Convert from font units to user space
			x := (float64(seg.Args[0].X) / unitsPerEm) * scaleX
			y := (float64(seg.Args[0].Y) / unitsPerEm) * scaleY
			// Apply Y flip if needed
			if flipY {
				y = -y
			}
			pd.Type = PathMoveTo
			pd.Points = []Point{{X: x, Y: y}}

		case api.SegmentOpLineTo:
			x := (float64(seg.Args[0].X) / unitsPerEm) * scaleX
			y := (float64(seg.Args[0].Y) / unitsPerEm) * scaleY
			// Apply Y flip if needed
			if flipY {
				y = -y
			}
			pd.Type = PathLineTo
			pd.Points = []Point{{X: x, Y: y}}

		case api.SegmentOpQuadTo:
			// Convert quadratic Bezier to cubic Bezier
			// For a quadratic curve with control point Q and end point P2,
			// the cubic equivalent has control points:
			// C1 = current_point + 2/3 * (Q - current_point)
			// C2 = P2 + 2/3 * (Q - P2)
			// However, since we don't track current point here, we'll use a simpler conversion
			x1 := (float64(seg.Args[0].X) / unitsPerEm) * scaleX
			y1 := (float64(seg.Args[0].Y) / unitsPerEm) * scaleY
			x2 := (float64(seg.Args[1].X) / unitsPerEm) * scaleX
			y2 := (float64(seg.Args[1].Y) / unitsPerEm) * scaleY
			// Apply Y flip if needed
			if flipY {
				y1 = -y1
				y2 = -y2
			}
			// Simplified: use the control point twice for cubic conversion
			pd.Type = PathCurveTo
			pd.Points = []Point{
				{X: x1, Y: y1},
				{X: x1, Y: y1},
				{X: x2, Y: y2},
			}

		case api.SegmentOpCubeTo:
			x1 := (float64(seg.Args[0].X) / unitsPerEm) * scaleX
			y1 := (float64(seg.Args[0].Y) / unitsPerEm) * scaleY
			x2 := (float64(seg.Args[1].X) / unitsPerEm) * scaleX
			y2 := (float64(seg.Args[1].Y) / unitsPerEm) * scaleY
			x3 := (float64(seg.Args[2].X) / unitsPerEm) * scaleX
			y3 := (float64(seg.Args[2].Y) / unitsPerEm) * scaleY
			// Apply Y flip if needed
			if flipY {
				y1 = -y1
				y2 = -y2
				y3 = -y3
			}
			pd.Type = PathCurveTo
			pd.Points = []Point{
				{X: x1, Y: y1},
				{X: x2, Y: y2},
				{X: x3, Y: y3},
			}
		}

		pdfPath.Data = append(pdfPath.Data, pd)
	}

	return pdfPath, nil
}

// GetTextBearingMetrics returns the bearing metrics for a text string
func (s *PangoPdfScaledFont) GetTextBearingMetrics(text string) (xBearing, yBearing float64, status Status) {
	metrics := s.TextExtents(text)
	if metrics == nil {
		return 0, 0, StatusFontTypeMismatch
	}
	return metrics.XBearing, metrics.YBearing, StatusSuccess
}

// GetTextAlignmentOffset calculates the Y offset for text alignment
func (s *PangoPdfScaledFont) GetTextAlignmentOffset(alignment TextAlignment) (float64, Status) {
	fontExtents := s.Extents()
	if fontExtents == nil {
		return 0, StatusFontTypeMismatch
	}
	return GetAlignmentOffset(alignment, fontExtents), StatusSuccess
}

// GetKerning returns the kerning adjustment between two runes
func (s *PangoPdfScaledFont) GetKerning(r1, r2 rune) (float64, Status) {
	realFace, status := s.getRealFace()
	if status != StatusSuccess {
		return 0, status
	}

	// Get the glyph indices for the runes
	gid1, ok1 := realFace.NominalGlyph(r1)
	gid2, ok2 := realFace.NominalGlyph(r2)
	if !ok1 || !ok2 {
		return 0, StatusInvalidGlyph
	}

	// Check if we have kerning data
	var kernValue int16
	if len(realFace.Kern) > 0 {
		// Try Kern tables first
		for _, kernSubtable := range realFace.Kern {
			if kd, ok := kernSubtable.Data.(apifont.Kern0); ok {
				kernValue = kd.KernPair(gid1, gid2)
				break
			}
		}
	} else if len(realFace.Kerx) > 0 {
		// Try Kerx tables if no Kern tables
		for _, kerxSubtable := range realFace.Kerx {
			if kd, ok := kerxSubtable.Data.(apifont.Kern0); ok {
				kernValue = kd.KernPair(gid1, gid2)
				break
			}
		}
	}

	// Scale factor from font matrix
	sx := math.Hypot(s.fontMatrix.XX, s.fontMatrix.YX)
	unitsPerEm := float64(realFace.Upem())

	// Convert kerning value to user space units
	kerning := float64(kernValue) * sx / unitsPerEm

	return kerning, StatusSuccess
}

// applyHinting applies font hinting based on the font options
// GetGlyphBearingMetrics returns the bearing metrics for a specific glyph
func (s *PangoPdfScaledFont) GetGlyphBearingMetrics(r rune) (xBearing, yBearing float64, status Status) {
	metrics, status := s.GetGlyphMetrics(r)
	if status != StatusSuccess {
		return 0, 0, status
	}
	return metrics.XBearing, metrics.YBearing, StatusSuccess
}

// GetGlyphMetrics returns detailed metrics for a specific glyph
func (s *PangoPdfScaledFont) GetGlyphMetrics(r rune) (*GlyphMetrics, Status) {
	realFace, status := s.getRealFace()
	if status != StatusSuccess {
		return nil, status
	}

	// Get the glyph index for the rune
	gid, ok := realFace.NominalGlyph(r)
	if !ok || gid == 0 {
		return nil, StatusInvalidGlyph
	}

	// Load glyph outline
	glyphData := realFace.GlyphData(gid)
	outline, ok := glyphData.(api.GlyphOutline)
	if !ok {
		return nil, StatusFontTypeMismatch
	}

	// Get font units per em and scale factor first
	unitsPerEm := float64(realFace.Upem())
	scaleX := math.Hypot(s.fontMatrix.XX, s.fontMatrix.YX)
	scaleY := math.Hypot(s.fontMatrix.XY, s.fontMatrix.YY)
	if scaleX == 0 {
		scaleX = 1.0
	}
	if scaleY == 0 {
		scaleY = 1.0
	}

	// Calculate bounding box from outline
	// Note: Outline coordinates from go-text/typesetting are in font units (float32)
	var xmin, xmax, ymin, ymax float64
	firstPoint := true

	// We need to apply Y flip here to match the actual rendered path
	flipY := true

	pointCount := 0
	for _, seg := range outline.Segments {
		for _, arg := range seg.Args {
			// Coordinates are already in font units (float32), just convert to float64
			xInFontUnits := float64(arg.X)
			yInFontUnits := float64(arg.Y)

			// Debug: print first few points for 'M'
			if r == 'M' && pointCount < 3 {
				fmt.Printf("[DEBUG] 'M' point %d: raw X=%.2f, Y=%.2f\n", pointCount, xInFontUnits, yInFontUnits)
			}

			// Apply Y flip to match rendered coordinates
			if flipY {
				yInFontUnits = -yInFontUnits
			}

			pointCount++

			if firstPoint {
				xmin, xmax = xInFontUnits, xInFontUnits
				ymin, ymax = yInFontUnits, yInFontUnits
				firstPoint = false
			} else {
				if xInFontUnits < xmin {
					xmin = xInFontUnits
				}
				if xInFontUnits > xmax {
					xmax = xInFontUnits
				}
				if yInFontUnits < ymin {
					ymin = yInFontUnits
				}
				if yInFontUnits > ymax {
					ymax = yInFontUnits
				}
			}
		}
	}

	// Scale bounding box to user space
	xmin = (xmin / unitsPerEm) * scaleX
	xmax = (xmax / unitsPerEm) * scaleX
	ymin = (ymin / unitsPerEm) * scaleY
	ymax = (ymax / unitsPerEm) * scaleY

	// Debug output for character 'M'
	if r == 'M' {
		fmt.Printf("[DEBUG GetGlyphMetrics] 'M': xmin=%.2f, xmax=%.2f, ymin=%.2f, ymax=%.2f\n", xmin, xmax, ymin, ymax)
	}

	// Get horizontal metrics from the font's hmtx table
	// HorizontalAdvance returns the advance width in font units (not 26.6 format)
	rawAdvance := realFace.HorizontalAdvance(gid)

	// Get the glyph's horizontal metrics including LSB
	// The GlyphData contains the actual outline, but we need to check if there's an LSB offset
	// In TrueType fonts, the glyph outline coordinates are relative to the glyph origin,
	// but there may be a left side bearing that offsets the visual position

	// Try to get the glyph's bounding box from the font if available
	// For now, we'll use the outline's actual bounds which already include any LSB

	// Convert from font units to user space units
	// Formula: (font_units / units_per_em) * font_size
	advanceInFontUnits := float64(rawAdvance)
	advanceWidth := (advanceInFontUnits / unitsPerEm) * scaleX

	// Create metrics
	metrics := &GlyphMetrics{
		Width:    advanceWidth,
		Height:   0, // For horizontal text
		XAdvance: advanceWidth,
		YAdvance: 0, // For horizontal text
		XBearing: xmin,
		YBearing: -ymax, // Negative because Y axis is inverted in Gopdf
	}

	// Set bounding box - these are relative to the glyph origin
	metrics.BoundingBox.XMin = xmin
	metrics.BoundingBox.YMin = ymin
	metrics.BoundingBox.XMax = xmax
	metrics.BoundingBox.YMax = ymax

	// Calculate side bearings
	metrics.LSB = xmin
	metrics.RSB = advanceWidth - xmax

	// Update XBearing to match the actual left edge of the glyph
	metrics.XBearing = xmin

	return metrics, StatusSuccess
}

// GetGlyphs returns the glyphs for a given text string.
func (s *PangoPdfScaledFont) GetGlyphs(utf8 string) (glyphs []Glyph, status Status) {
	realFace, status := s.getRealFace()
	if status != StatusSuccess {
		return nil, status
	}

	// 1. Shape the text
	runes := []rune(utf8)
	input := shaping.Input{
		Text:      runes,
		RunStart:  0,
		RunEnd:    len(runes),
		Direction: di.DirectionLTR,
		Face:      realFace,
		Size:      fixed.I(12),
	}
	output := (&shaping.HarfbuzzShaper{}).Shape(input)

	// 2. Convert shaped output to gopdf's Glyph structures
	glyphs = make([]Glyph, len(output.Glyphs))
	for i, g := range output.Glyphs {
		glyphs[i] = Glyph{
			Index: uint64(g.GlyphID),
			X:     0, // Position is not relevant for subsetting
			Y:     0,
		}
	}

	return glyphs, StatusSuccess
}

// TextToGlyphs performs text shaping to get accurate glyphs and clusters.
func (s *PangoPdfScaledFont) TextToGlyphs(x, y float64, utf8 string) (glyphs []Glyph, clusters []TextCluster, clusterFlags TextClusterFlags, status Status) {
	return s.TextToGlyphsWithOptions(x, y, utf8, nil)
}

// TextToGlyphsWithOptions performs text shaping with advanced OpenType features
func (s *PangoPdfScaledFont) TextToGlyphsWithOptions(x, y float64, utf8 string, options *ShapingOptions) (glyphs []Glyph, clusters []TextCluster, clusterFlags TextClusterFlags, status Status) {
	realFace, status := s.getRealFace()
	if status != StatusSuccess {
		return s.toyTextToGlyphsFallback(x, y, utf8)
	}

	// Get the font size from the font matrix
	// The font size is typically the YY component of the font matrix
	fontSize := math.Hypot(s.fontMatrix.XX, s.fontMatrix.YX)
	if fontSize == 0 {
		fontSize = 12.0 // Default fallback
	}

	// Get font metrics for line height calculation
	metrics, _ := realFace.FontHExtents()
	ascentFUnits := float64(metrics.Ascender)
	descentFUnits := float64(metrics.Descender)
	lineGapFUnits := float64(metrics.LineGap)

	// Calculate line height in user space units
	lineHeight := (ascentFUnits - descentFUnits + lineGapFUnits) / 64.0
	if lineHeight <= 0 {
		lineHeight = fontSize * 1.2 // Fallback to 120% of font size
	}

	// Use default options if not provided
	if options == nil {
		options = NewShapingOptions()
	}

	// Auto-detect missing options
	if options.Direction == TextDirectionAuto {
		options.Direction = DetectTextDirection(utf8)
	}
	if options.Language == "" {
		options.Language = DetectLanguage(utf8)
	}
	if options.Script == "" {
		options.Script = DetectScript(utf8)
	}

	// Split text into lines, supporting different line ending styles
	// \r\n (Windows), \n (Unix/Linux/macOS), \r (old Mac)
	lines := splitLines(utf8)

	// Process each line separately
	glyphs = make([]Glyph, 0)
	clusters = make([]TextCluster, 0)
	var curY float64

	for lineIdx, line := range lines {
		if line == "" {
			// Empty line, just advance Y
			curY += lineHeight
			continue
		}

		// 1. Shape the text with advanced options
		// fixed.I() converts an integer to 26.6 fixed point format
		runes := []rune(line)
		input := shaping.Input{
			Text:      runes,
			RunStart:  0,
			RunEnd:    len(runes),
			Direction: convertDirection(options.Direction, line),
			Face:      realFace,
			Size:      fixed.I(int(fontSize)), // Convert to 26.6 fixed point
			Language:  convertLanguage(options.Language),
			Script:    convertScript(options.Script),
		}
		output := (&shaping.HarfbuzzShaper{}).Shape(input)

		// 2. Convert shaped output to gopdf's Glyph and TextCluster structures
		var curX float64

		// Process each glyph with proper spacing
		for _, g := range output.Glyphs {
			// Position is in user space, relative to the start point (x, y)
			glyph := Glyph{
				Index: uint64(g.GlyphID),
				X:     x + curX + float64(g.XOffset)/64.0,
				Y:     y + curY - float64(g.YOffset)/64.0, // Subtract because glyph offsets are in font coordinate system
			}
			glyphs = append(glyphs, glyph)

			// Add the advance width for the next glyph
			// The shaper returns advances in 26.6 fixed point format
			curX += float64(g.XAdvance) / 64.0
		}

		// Create clusters for this line
		for range output.Glyphs {
			cluster := TextCluster{
				NumBytes:  1, // Simplified: assume 1 byte per glyph
				NumGlyphs: 1,
			}
			clusters = append(clusters, cluster)
		}

		// Move to next line (reset X, advance Y)
		if lineIdx < len(lines)-1 {
			curY += lineHeight
		}
	}

	// Cluster flags (simplified)
	clusterFlags = 0

	return glyphs, clusters, clusterFlags, StatusSuccess
}

// toyTextToGlyphsFallback performs a trivial Unicode->glyph mapping similar to
// gopdf_scaled_font_text_to_glyphs but without complex shaping.
func (s *PangoPdfScaledFont) toyTextToGlyphsFallback(x, y float64, utf8 string) (glyphs []Glyph, clusters []TextCluster, clusterFlags TextClusterFlags, status Status) {
	// Simple left-to-right mapping: one glyph per rune.
	size := s.toyExtentsFallback().Ascent + s.toyExtentsFallback().Descent
	advancePerRune := size * 0.6

	glyphs = make([]Glyph, 0, len(utf8))
	clusters = make([]TextCluster, 0, len(utf8))

	var curX = x
	// We need byte offsets for clusters.
	for i, r := range utf8 {
		g := Glyph{
			Index: uint64(r),
			X:     curX,
			Y:     y,
		}
		glyphs = append(glyphs, g)

		// Each rune maps to one cluster: num_bytes is number of bytes for this rune.
		var nextByte int
		if i == len(utf8)-1 {
			nextByte = len(utf8)
		} else {
			// This loop body is over runes, but range on string gives byte offsets
			nextByte = i + len(string(r))
		}
		cluster := TextCluster{
			NumBytes:  nextByte - i,
			NumGlyphs: 1,
		}
		clusters = append(clusters, cluster)

		curX += advancePerRune
	}

	clusterFlags = 0
	return glyphs, clusters, clusterFlags, StatusSuccess
}

// PangoPdfShowText renders text using PangoPdf directly to the surface
func PangoPdfShowText(ctx Context, layout *PangoPdfLayout) {
	if ctx.Status() != StatusSuccess {
		return
	}

	// Get current point or use (0, 0)
	x, y := ctx.GetCurrentPoint()
	if x == 0 && y == 0 && ctx.HasCurrentPoint() == False {
		x, y = 0, 0
	}

	// Create scaled font from layout's font description
	if layout.fontDesc == nil {
		ctx.(*context).status = StatusFontTypeMismatch
		return
	}

	fontFace := NewPangoPdfFont(layout.fontDesc.family, FontSlantNormal, FontWeightNormal)
	defer fontFace.Destroy()

	fontMatrix := NewMatrix()
	// Use positive Y scale - our coordinate system has Y growing downward,
	// and we'll handle the glyph flip in the rendering code
	fontMatrix.InitScale(layout.fontDesc.size, layout.fontDesc.size)

	ctm := NewMatrix()
	ctm.InitIdentity()

	sf := NewPangoPdfScaledFont(fontFace, fontMatrix, ctm, nil)
	defer sf.Destroy()

	// Get font metrics for line spacing
	fontExtents := sf.Extents()
	lineHeight := fontExtents.Height
	if layout.lineSpacing > 0 {
		lineHeight = layout.lineSpacing
	} else if layout.spacing > 0 {
		lineHeight += layout.spacing
	}

	// If lineHeight is still 0 or too small, use font size as fallback
	if lineHeight < layout.fontDesc.size*0.5 {
		lineHeight = layout.fontDesc.size * 1.2 // 120% of font size
	}

	// Split text into lines
	text := layout.GetText()
	lines := strings.Split(text, "\n")

	// Render each line
	currentY := y
	for _, line := range lines {
		// Skip empty lines but still advance Y position
		if line == "" {
			currentY += lineHeight
			continue
		}

		// Perform text shaping to get glyphs for this line
		glyphs, _, _, status := sf.TextToGlyphs(x, currentY, line)
		if status != StatusSuccess {
			ctx.(*context).status = status
			return
		}

		// Render this line's glyphs
		renderLineGlyphs(ctx, sf, glyphs, layout, x, line)

		// Move to next line
		currentY += lineHeight
	}

	// Update current point to the position after the last line
	if len(lines) > 0 {
		lastLine := lines[len(lines)-1]
		if lastLine != "" {
			extents := sf.TextExtents(lastLine)
			c := ctx.(*context)
			c.currentPoint.x = x + extents.XAdvance
			c.currentPoint.y = currentY - lineHeight + extents.YAdvance
			c.currentPoint.hasPoint = true
		}
	}
}

// renderLineGlyphs renders glyphs for a single line of text
func renderLineGlyphs(ctx Context, sf *PangoPdfScaledFont, glyphs []Glyph, layout *PangoPdfLayout, x float64, lineText string) {

	// Apply alignment adjustments
	if layout.align != PangoAlignLeft && layout.width > 0 {
		// Calculate text width for this line
		textExtents := sf.TextExtents(lineText)
		layoutWidth := float64(layout.width) / 1024.0 // Convert from Pango units

		var offsetX float64
		switch layout.align {
		case PangoAlignRight:
			offsetX = layoutWidth - textExtents.Width
		case PangoAlignCenter:
			offsetX = (layoutWidth - textExtents.Width) / 2
		}

		// Adjust all glyph positions
		for i := range glyphs {
			glyphs[i].X += offsetX
		}
	}

	// Render glyphs directly to surface using PangoPdf
	c := ctx.(*context)
	c.mu.Lock()
	defer c.mu.Unlock()

	// Get the current source pattern for text color
	source := c.gstate.source
	if source == nil {
		return
	}

	// Apply state once before rendering all glyphs to ensure gradient is set
	c.applyStateToPango()

	// Render each glyph directly to the surface
	for _, glyph := range glyphs {
		// Save context state before rendering each glyph
		c.Save()

		// Get the glyph path
		glyphPath, err := sf.GlyphPath(glyph.Index)
		if err != nil || glyphPath == nil {
			c.Restore()
			continue
		}

		if len(glyphPath.Data) == 0 {
			c.Restore()
			continue
		}

		// Clear current path and create a new one for this glyph
		c.NewPath()

		// Translate the glyph path to the correct position and add to context
		// The glyph path is in font space, we need to translate it to the glyph position
		pathSegments := 0
		for _, pathData := range glyphPath.Data {
			switch pathData.Type {
			case PathMoveTo:
				if len(pathData.Points) > 0 {
					c.MoveTo(pathData.Points[0].X+glyph.X, pathData.Points[0].Y+glyph.Y)
					pathSegments++
				}
			case PathLineTo:
				if len(pathData.Points) > 0 {
					c.LineTo(pathData.Points[0].X+glyph.X, pathData.Points[0].Y+glyph.Y)
					pathSegments++
				}
			case PathCurveTo:
				if len(pathData.Points) >= 3 {
					c.CurveTo(
						pathData.Points[0].X+glyph.X, pathData.Points[0].Y+glyph.Y,
						pathData.Points[1].X+glyph.X, pathData.Points[1].Y+glyph.Y,
						pathData.Points[2].X+glyph.X, pathData.Points[2].Y+glyph.Y,
					)
					pathSegments++
				}
			case PathClosePath:
				c.ClosePath()
				pathSegments++
			}
		}

		// Debug: print glyph info (commented out for production)
		// fmt.Printf("[DEBUG] Glyph %d at (%.2f, %.2f): added %d path segments\n", glyph.Index, glyph.X, glyph.Y, pathSegments)

		// Fill the glyph
		c.Fill()

		// Restore context state after rendering each glyph
		c.Restore()
	}
}

// PangoPdfUpdateLayout updates a layout to match the current transformation matrix of a Gopdf context
func PangoPdfUpdateLayout(ctx Context, layout *PangoPdfLayout) {
	// Implementation would synchronize the layout with the Gopdf context transformation
	// For now, this is a placeholder
	_ = ctx
	_ = layout
}

// PangoPdfCreateLayout creates a new Pango layout for a Gopdf context
func PangoPdfCreateLayout(ctx Context) *PangoPdfLayout {
	// Create a default font map and context
	fontMap := NewPangoPdfFontMap()
	pangoCtx := NewPangoPdfContext(fontMap)
	layout := NewPangoPdfLayout(pangoCtx)
	return layout
}

// GlyphCornerCoordinates represents the four corners of a glyph's bounding box
type GlyphCornerCoordinates struct {
	TopLeftX, TopLeftY         float64
	TopRightX, TopRightY       float64
	BottomLeftX, BottomLeftY   float64
	BottomRightX, BottomRightY float64
}

// GetGlyphCornerCoordinates calculates the four corner coordinates of a glyph
func (s *PangoPdfScaledFont) GetGlyphCornerCoordinates(glyph Glyph) (*GlyphCornerCoordinates, Status) {
	// Get glyph metrics
	metrics, status := s.GetGlyphMetrics(rune(glyph.Index))
	if status != StatusSuccess {
		return nil, status
	}

	// Calculate the four corners based on glyph position and advance width
	// The bounding box represents the visual bounds of the glyph
	topRightX := glyph.X + metrics.BoundingBox.XMax

	if glyph.Index == uint64('H') {
		fmt.Printf("[DEBUG GetGlyphCornerCoordinates] 'H': glyph.X=%.2f, BBox.XMax=%.2f, TopRightX=%.2f\n",
			glyph.X, metrics.BoundingBox.XMax, topRightX)
	}

	coords := &GlyphCornerCoordinates{
		TopLeftX:     glyph.X + metrics.BoundingBox.XMin,
		TopLeftY:     glyph.Y + metrics.BoundingBox.YMin,
		TopRightX:    topRightX,
		TopRightY:    glyph.Y + metrics.BoundingBox.YMin,
		BottomLeftX:  glyph.X + metrics.BoundingBox.XMin,
		BottomLeftY:  glyph.Y + metrics.BoundingBox.YMax,
		BottomRightX: topRightX,
		BottomRightY: glyph.Y + metrics.BoundingBox.YMax,
	}

	return coords, StatusSuccess
}

// CheckGlyphCollision checks if two glyphs' bounding boxes overlap
// char1 and char2 are the actual characters (runes) corresponding to the glyphs
func (s *PangoPdfScaledFont) CheckGlyphCollision(glyph1, glyph2 Glyph, char1, char2 rune) (bool, Status) {
	// Get metrics for both characters
	metrics1, status := s.GetGlyphMetrics(char1)
	if status != StatusSuccess {
		return false, status
	}

	metrics2, status := s.GetGlyphMetrics(char2)
	if status != StatusSuccess {
		return false, status
	}

	// Calculate bounding boxes in absolute coordinates
	box1MinX := glyph1.X + metrics1.BoundingBox.XMin
	box1MaxX := glyph1.X + metrics1.BoundingBox.XMax
	box1MinY := glyph1.Y + metrics1.BoundingBox.YMin
	box1MaxY := glyph1.Y + metrics1.BoundingBox.YMax

	box2MinX := glyph2.X + metrics2.BoundingBox.XMin
	box2MaxX := glyph2.X + metrics2.BoundingBox.XMax
	box2MinY := glyph2.Y + metrics2.BoundingBox.YMin
	box2MaxY := glyph2.Y + metrics2.BoundingBox.YMax

	// Check for overlap
	// Two rectangles overlap if:
	// 1. The left edge of rect1 is to the left of the right edge of rect2
	// 2. The right edge of rect1 is to the right of the left edge of rect2
	// 3. The top edge of rect1 is above the bottom edge of rect2
	// 4. The bottom edge of rect1 is below the top edge of rect2
	overlap := box1MinX < box2MaxX &&
		box1MaxX > box2MinX &&
		box1MinY < box2MaxY &&
		box1MaxY > box2MinY

	return overlap, StatusSuccess
}

// PrintGlyphInfo prints detailed information about a glyph including its corner coordinates
func (s *PangoPdfScaledFont) PrintGlyphInfo(glyph Glyph, char rune) {
	// Get metrics using the correct character, not the glyph index
	metrics, status := s.GetGlyphMetrics(char)
	if status != StatusSuccess {
		fmt.Printf("无法获取字符 '%c' 的度量信息: %v\n", char, status)
		return
	}

	// Calculate corners manually using the correct metrics
	visualWidth := metrics.BoundingBox.XMax - metrics.BoundingBox.XMin

	coords := &GlyphCornerCoordinates{
		TopLeftX:     glyph.X + metrics.BoundingBox.XMin,
		TopLeftY:     glyph.Y + metrics.BoundingBox.YMin,
		TopRightX:    glyph.X + metrics.BoundingBox.XMax,
		TopRightY:    glyph.Y + metrics.BoundingBox.YMin,
		BottomLeftX:  glyph.X + metrics.BoundingBox.XMin,
		BottomLeftY:  glyph.Y + metrics.BoundingBox.YMax,
		BottomRightX: glyph.X + metrics.BoundingBox.XMax,
		BottomRightY: glyph.Y + metrics.BoundingBox.YMax,
	}

	fmt.Printf("字符 '%c' 位置信息:\n", char)
	fmt.Printf("  位置: (%.2f, %.2f)\n", glyph.X, glyph.Y)
	fmt.Printf("  边界框: minX=%.2f, minY=%.2f, maxX=%.2f, maxY=%.2f\n",
		metrics.BoundingBox.XMin, metrics.BoundingBox.YMin,
		metrics.BoundingBox.XMax, metrics.BoundingBox.YMax)
	fmt.Printf("  视觉宽度: %.2f, Advance: %.2f\n", visualWidth, metrics.XAdvance)
	fmt.Printf("  左上角: (%.2f, %.2f)\n", coords.TopLeftX, coords.TopLeftY)
	fmt.Printf("  右上角: (%.2f, %.2f)\n", coords.TopRightX, coords.TopRightY)
	fmt.Printf("  左下角: (%.2f, %.2f)\n", coords.BottomLeftX, coords.BottomLeftY)
	fmt.Printf("  右下角: (%.2f, %.2f)\n", coords.BottomRightX, coords.BottomRightY)
	fmt.Println()
}

// PrintTextGlyphsInfo prints information for all glyphs in a text string
func (s *PangoPdfScaledFont) PrintTextGlyphsInfo(utf8 string, glyphs []Glyph) {
	runes := []rune(utf8)

	// Print info for each glyph
	for i, glyph := range glyphs {
		var char rune
		if i < len(runes) {
			char = runes[i]
		} else {
			char = rune(glyph.Index)
		}

		s.PrintGlyphInfo(glyph, char)

		// Check for collisions with subsequent glyphs
		for j := i + 1; j < len(glyphs); j++ {
			var nextChar rune
			if j < len(runes) {
				nextChar = runes[j]
			} else {
				nextChar = rune(glyphs[j].Index)
			}
			collides, status := s.CheckGlyphCollision(glyph, glyphs[j], char, nextChar)
			if status == StatusSuccess && collides {
				fmt.Printf("警告: 字符 '%c' 和 '%c' 之间存在重叠!\n\n", char, nextChar)
			}
		}
	}
}

// PangoRectangle represents a rectangle in Pango coordinates
type PangoRectangle struct {
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// GetPixelExtents returns the pixel extents of the layout
func (l *PangoPdfLayout) GetPixelExtents() *PangoRectangle {
	if l.text == "" || l.fontDesc == nil {
		return &PangoRectangle{}
	}

	// Create a temporary scaled font to get text extents
	fontFace := NewPangoPdfFont(l.fontDesc.family, FontSlantNormal, FontWeightNormal)
	defer fontFace.Destroy()

	fontMatrix := NewMatrix()
	// Use positive Y scale - our coordinate system has Y growing downward, and we'll handle the glyph flip in the rendering code
	fontMatrix.InitScale(l.fontDesc.size, l.fontDesc.size)

	ctm := NewMatrix()
	ctm.InitIdentity()

	scaledFont := NewPangoPdfScaledFont(fontFace, fontMatrix, ctm, nil)
	defer scaledFont.Destroy()

	extents := scaledFont.TextExtents(l.text)

	return &PangoRectangle{
		X:      extents.XBearing,
		Y:      extents.YBearing,
		Width:  extents.Width,
		Height: extents.Height,
	}
}

// GetFontExtents returns the font extents for the layout
func (l *PangoPdfLayout) GetFontExtents() *FontExtents {
	if l.fontDesc == nil {
		return &FontExtents{}
	}

	// Create a temporary scaled font to get font extents
	fontFace := NewPangoPdfFont(l.fontDesc.family, FontSlantNormal, FontWeightNormal)
	defer fontFace.Destroy()

	fontMatrix := NewMatrix()
	// Use positive Y scale - our coordinate system has Y growing downward, and we'll handle the glyph flip in the rendering code
	fontMatrix.InitScale(l.fontDesc.size, l.fontDesc.size)

	ctm := NewMatrix()
	ctm.InitIdentity()

	scaledFont := NewPangoPdfScaledFont(fontFace, fontMatrix, ctm, nil)
	defer scaledFont.Destroy()

	return scaledFont.Extents()
}
