package gopdf

import (
	"image"
	"image/color"
	"math"
	"unsafe"
)

// Gopdf Version
const (
	VersionMajor = 1
	VersionMinor = 18
	VersionMicro = 2
)

// Bool represents gopdf_bool_t
type Bool int

const (
	False Bool = 0
	True  Bool = 1
)

// Status represents gopdf_status_t - error status codes
type Error struct {
	Status Status
	Msg    string
}

func (e Error) Error() string {
	if e.Msg != "" {
		return e.Msg
	}
	return e.Status.String()
}

// Is implements the errors.Is interface, allowing comparison with other gopdf.Error types.
func (e Error) Is(target error) bool {
	if targetErr, ok := target.(Error); ok {
		return e.Status == targetErr.Status
	}
	return false
}

func newError(status Status, msg string) error {
	if status == StatusSuccess {
		return nil
	}
	return Error{Status: status, Msg: msg}
}

type Status int

const (
	StatusSuccess Status = iota
	StatusNoMemory
	StatusInvalidRestore
	StatusInvalidPopGroup
	StatusNoCurrentPoint
	StatusInvalidMatrix
	StatusInvalidStatus
	StatusNullPointer
	StatusInvalidString
	StatusInvalidPathData
	StatusReadError
	StatusWriteError
	StatusSurfaceFinished
	StatusSurfaceTypeMismatch
	StatusPatternTypeMismatch
	StatusInvalidContent
	StatusInvalidFormat
	StatusInvalidVisual
	StatusFileNotFound
	StatusInvalidDash
	StatusInvalidDscComment
	StatusInvalidIndex
	StatusClipNotRepresentable
	StatusTempFileError
	StatusInvalidStride
	StatusFontTypeMismatch
	StatusUserFontImmutable
	StatusUserFontError
	StatusNegativeCount
	StatusInvalidClusters
	StatusInvalidSlant
	StatusInvalidWeight
	StatusInvalidSize
	StatusUserFontNotImplemented
	StatusDeviceTypeMismatch
	StatusDeviceError
	StatusInvalidMeshConstruction
	StatusDeviceFinished
	StatusJbig2GlobalMissing
	StatusPngError
	StatusFreetypeError
	StatusWin32GdiError
	StatusTagError
	StatusDwriteError
	StatusSvgFontError
	StatusInvalidGlyph
	StatusLastStatus
)

// Content represents gopdf_content_t - surface content types
type Content int

const (
	ContentColor      Content = 0x1000
	ContentAlpha      Content = 0x2000
	ContentColorAlpha Content = 0x3000
)

// Format represents gopdf_format_t - pixel formats for image surfaces
type Format int

const (
	FormatInvalid  Format = -1
	FormatARGB32   Format = 0
	FormatRGB24    Format = 1
	FormatA8       Format = 2
	FormatA1       Format = 3
	FormatRGB16565 Format = 4
	FormatRGB30    Format = 5
	FormatRGB96F   Format = 6
	FormatRGBA128F Format = 7
)

// SurfaceType represents gopdf_surface_type_t - surface types
type SurfaceType int

const (
	SurfaceTypeImage SurfaceType = iota
	SurfaceTypePDF
	SurfaceTypePS
	SurfaceTypeSVG
	SurfaceTypeRecording
	SurfaceTypeWin32
	SurfaceTypeQuartz
	SurfaceTypeXCB
	SurfaceTypeXLib
	SurfaceTypeGlitz
	SurfaceTypeQuartzImage
	SurfaceTypeScript
	SurfaceTypeWin32Printing
	SurfaceTypeOS2
	SurfaceTypeVGL
	SurfaceTypeExtension
	SurfaceTypeDLS
	SurfaceTypeDRM
	SurfaceTypeTee
	SurfaceTypeXML
	SurfaceTypeSkia
	SurfaceTypeSubsurface
	SurfaceTypeCogl
	SurfaceTypeWin32GDI
	SurfaceTypeRecordingSurface
	SurfaceTypeObserver
	SurfaceTypeInvalid
)

// Dither represents gopdf_dither_t - dithering modes
type Dither int

const (
	DitherNone Dither = iota
	DitherDefault
	DitherFast
	DitherGood
	DitherBest
)

// Extend represents gopdf_extend_t - pattern extend modes
type Extend int

const (
	ExtendNone Extend = iota
	ExtendRepeat
	ExtendReflect
	ExtendPad
)

// Filter represents gopdf_filter_t - pattern filter modes
type Filter int

const (
	FilterFast Filter = iota
	FilterGood
	FilterBest
	FilterNearest
	FilterBilinear
	FilterGaussian
)

// PatternType represents gopdf_pattern_type_t - pattern types
type PatternType int

const (
	PatternTypeSolid PatternType = iota
	PatternTypeSurface
	PatternTypeLinear
	PatternTypeRadial
	PatternTypeMesh
	PatternTypeRasterSource
)

// Operator represents gopdf_operator_t - compositing operators
type Operator int

// BlendFunc defines a function that blends a source and destination color.
type BlendFunc func(src, dst color.Color) color.Color

const (
	OperatorClear Operator = iota
	OperatorSource
	OperatorOver
	OperatorIn
	OperatorOut
	OperatorAtop
	OperatorDest
	OperatorDestOver
	OperatorDestIn
	OperatorDestOut
	OperatorDestAtop
	OperatorXor
	OperatorAdd
	OperatorSaturate
	OperatorMultiply
	OperatorScreen
	OperatorOverlay
	OperatorDarken
	OperatorLighten
	OperatorColorDodge
	OperatorColorBurn
	OperatorHardLight
	OperatorSoftLight
	OperatorDifference
	OperatorExclusion
	OperatorHslHue
	OperatorHslSaturation
	OperatorHslColor
	OperatorHslLuminosity
)

// Antialias represents gopdf_antialias_t - antialiasing modes
type Antialias int

const (
	AntialiasDefault Antialias = iota
	AntialiasNone
	AntialiasGray
	AntialiasSubpixel
	AntialiasFast
	AntialiasGood
	AntialiasBest
)

// Color represents an RGBA color with float components in [0,1].
type Color struct {
	R, G, B, A float64
}

// FillRule represents gopdf_fill_rule_t - fill rule for paths
type FillRule int

const (
	FillRuleWinding FillRule = iota
	FillRuleEvenOdd
)

// LineCap represents gopdf_line_cap_t - line cap styles
type LineCap int

const (
	LineCapButt LineCap = iota
	LineCapRound
	LineCapSquare
)

// LineJoin represents gopdf_line_join_t - line join styles
type LineJoin int

const (
	LineJoinMiter LineJoin = iota
	LineJoinRound
	LineJoinBevel
)

// Matrix represents gopdf_matrix_t - 2D affine transformation matrix
type Matrix struct {
	XX, YX float64
	XY, YY float64
	X0, Y0 float64
}

// NewMatrix creates an identity matrix
func NewMatrix() *Matrix {
	return &Matrix{
		XX: 1.0, YX: 0.0,
		XY: 0.0, YY: 1.0,
		X0: 0.0, Y0: 0.0,
	}
}

// InitIdentity initializes matrix to identity
func (m *Matrix) InitIdentity() {
	m.XX = 1.0
	m.YX = 0.0
	m.XY = 0.0
	m.YY = 1.0
	m.X0 = 0.0
	m.Y0 = 0.0
}

// InitTranslate initializes matrix to translation
func (m *Matrix) InitTranslate(tx, ty float64) {
	m.InitIdentity()
	m.X0 = tx
	m.Y0 = ty
}

// InitScale initializes matrix to scaling
func (m *Matrix) InitScale(sx, sy float64) {
	m.InitIdentity()
	m.XX = sx
	m.YY = sy
}

// InitRotate initializes matrix to rotation
func (m *Matrix) InitRotate(radians float64) {
	s := math.Sin(radians)
	c := math.Cos(radians)
	m.XX = c
	m.YX = s
	m.XY = -s
	m.YY = c
	m.X0 = 0.0
	m.Y0 = 0.0
}

// InitSkew initializes matrix to skew (shear)
func (m *Matrix) InitSkew(shearX, shearY float64) {
	m.InitIdentity()
	m.XY = shearX // Skew along X-axis
	m.YX = shearY // Skew along Y-axis
}

// MatrixDecompose decomposes the matrix into translation, rotation, scale, and shear components.
// The decomposition is not unique, but this follows a common convention.
func MatrixDecompose(m *Matrix) (tx, ty, rotation, scaleX, scaleY, shear float64, status Status) {
	tx = m.X0
	ty = m.Y0

	// Calculate scale factors
	scaleX = math.Hypot(m.XX, m.YX)
	scaleY = math.Hypot(m.XY, m.YY)

	// Check for degenerate matrix
	if scaleX == 0 || scaleY == 0 {
		return tx, ty, 0, 0, 0, 0, StatusInvalidMatrix
	}

	// Normalize components
	nXX := m.XX / scaleX
	nYX := m.YX / scaleX
	nXY := m.XY / scaleY
	nYY := m.YY / scaleY

	// Calculate rotation (from XX and YX)
	rotation = math.Atan2(nYX, nXX)

	// Calculate shear (from dot product of normalized X and Y vectors)
	shear = nXX*nXY + nYX*nYY

	// Adjust scaleY and nXY/nYY for shear
	if shear != 0 {
		// Remove shear from Y vector
		nXY -= shear * nXX
		nYY -= shear * nYX

		// Recalculate scaleY and normalize Y vector
		scaleY = math.Hypot(nXY, nYY)
		if scaleY == 0 {
			return tx, ty, rotation, scaleX, 0, shear, StatusInvalidMatrix
		}
		nXY /= scaleY
		nYY /= scaleY

		// Recalculate shear (should be close to zero now)
		shear = nXX*nXY + nYX*nYY
	}

	// Check for reflection (determinant sign)
	det := nXX*nYY - nYX*nXY
	if det < 0 {
		// Reflection detected, usually handled by making one scale factor negative
		scaleX = -scaleX
		// 防止除零
		if scaleX != 0 {
			rotation = math.Atan2(m.YX/scaleX, m.XX/scaleX)
		}
	}

	return tx, ty, rotation, scaleX, scaleY, shear, StatusSuccess
}

// Rectangle represents gopdf_rectangle_t - floating point rectangle
type Rectangle struct {
	X, Y          float64
	Width, Height float64
}

// RectangleInt represents gopdf_rectangle_int_t - integer rectangle
type RectangleInt struct {
	X, Y          int
	Width, Height int
}

// GlyphTransform represents transformation matrix for individual glyphs
// Following Gopdf's text rendering model
type GlyphTransform struct {
	// Matrix is the transformation matrix [xx, xy, yx, yy, x0, y0]
	Matrix Matrix

	// OffsetX is additional X offset
	OffsetX float64

	// OffsetY is additional Y offset
	OffsetY float64
}

// UserDataKey represents gopdf_user_data_key_t - key for user data
type UserDataKey struct {
	Unused int
}

// DestroyFunc represents gopdf_destroy_func_t - cleanup callback
type DestroyFunc func(data unsafe.Pointer)

// WriteFunc represents gopdf_write_func_t - write callback for surfaces
type WriteFunc func(closure interface{}, data []byte) error

// ReadFunc represents gopdf_read_func_t - read callback for surfaces
type ReadFunc func(closure interface{}, data []byte) error

// TextExtents represents gopdf_text_extents_t - text measurement
type TextExtents struct {
	XBearing, YBearing float64
	Width, Height      float64
	XAdvance, YAdvance float64
}

// FontExtents represents gopdf_font_extents_t - font metrics
type FontExtents struct {
	// Ascent is the distance from the baseline to the highest point
	// typically used by glyphs in the font
	Ascent float64

	// Descent is the distance from the baseline to the lowest point
	// typically used by glyphs (positive value)
	Descent float64

	// Height is the recommended line spacing (ascent + descent + linegap)
	Height float64

	// MaxAdvance is the maximum advance width for all glyphs
	MaxXAdvance float64

	// MaxYAdvance is the maximum advance height for all glyphs
	MaxYAdvance float64

	// UnderlinePosition is the position of the underline
	UnderlinePosition float64

	// UnderlineThickness is the thickness of the underline
	UnderlineThickness float64

	// CapHeight is the height of capital letters
	CapHeight float64

	// XHeight is the height of lowercase 'x'
	XHeight float64

	// LineGap is the recommended additional spacing between lines
	LineGap float64
}

// Glyph represents gopdf_glyph_t - positioned glyph
type Glyph struct {
	Index uint64
	X, Y  float64
}

// GlyphMetrics contains detailed metrics for a single glyph
// Following Gopdf's text rendering model
type GlyphMetrics struct {
	// Width is the horizontal distance from the origin to the point where
	// the next glyph should be drawn (advance width)
	Width float64

	// Height is the vertical distance from the origin to the point where
	// the next glyph should be drawn for vertical text (advance height)
	Height float64

	// XBearing is the horizontal distance from the origin to the leftmost
	// part of the glyph
	XBearing float64

	// YBearing is the vertical distance from the origin (baseline) to the
	// topmost part of the glyph
	YBearing float64

	// XAdvance is the horizontal distance to advance for the next glyph
	XAdvance float64

	// YAdvance is the vertical distance to advance for the next glyph
	YAdvance float64

	// BoundingBox contains the tight bounding box of the glyph outline
	BoundingBox struct {
		XMin, YMin float64
		XMax, YMax float64
	}

	// LSB is the left side bearing
	LSB float64

	// RSB is the right side bearing
	RSB float64
}

// TextMetrics contains metrics for a text string
type TextMetrics struct {
	// XBearing is the horizontal distance from the origin to the leftmost
	// part of the text
	XBearing float64

	// YBearing is the vertical distance from the origin to the topmost
	// part of the text
	YBearing float64

	// Width is the width of the text bounding box
	Width float64

	// Height is the height of the text bounding box
	Height float64

	// XAdvance is the horizontal distance to advance after drawing the text
	XAdvance float64

	// YAdvance is the vertical distance to advance after drawing the text
	YAdvance float64
}

// TextCluster represents gopdf_text_cluster_t - text cluster mapping
type TextCluster struct {
	NumBytes  int
	NumGlyphs int
}

// Point represents a point in the path
type Point struct {
	X, Y float64
}

// TextClusterFlags represents gopdf_text_cluster_flags_t - cluster flags
type TextClusterFlags int

const (
	TextClusterFlagBackward TextClusterFlags = 0x00000001
)

// FontSlant represents gopdf_font_slant_t - font slant styles
type FontSlant int

const (
	FontSlantNormal FontSlant = iota
	FontSlantItalic
	FontSlantOblique
)

// FontWeight represents gopdf_font_weight_t - font weight styles
type FontWeight int

const (
	FontWeightNormal FontWeight = iota
	FontWeightBold
)

// Status.String() provides a human-readable error message.
func (s Status) String() string {
	switch s {
	case StatusSuccess:
		return "success"
	case StatusNoMemory:
		return "no memory"
	case StatusInvalidRestore:
		return "invalid restore"
	case StatusInvalidPopGroup:
		return "invalid pop group"
	case StatusNoCurrentPoint:
		return "no current point"
	case StatusInvalidMatrix:
		return "invalid matrix"
	case StatusInvalidStatus:
		return "invalid status"
	case StatusNullPointer:
		return "null pointer"
	case StatusInvalidString:
		return "invalid string"
	case StatusInvalidPathData:
		return "invalid path data"
	case StatusReadError:
		return "read error"
	case StatusWriteError:
		return "write error"
	case StatusSurfaceFinished:
		return "surface finished"
	case StatusSurfaceTypeMismatch:
		return "surface type mismatch"
	case StatusPatternTypeMismatch:
		return "pattern type mismatch"
	case StatusInvalidContent:
		return "invalid content"
	case StatusInvalidFormat:
		return "invalid format"
	case StatusInvalidVisual:
		return "invalid visual"
	case StatusFileNotFound:
		return "file not found"
	case StatusInvalidDash:
		return "invalid dash"
	case StatusInvalidDscComment:
		return "invalid dsc comment"
	case StatusInvalidIndex:
		return "invalid index"
	case StatusClipNotRepresentable:
		return "clip not representable"
	case StatusTempFileError:
		return "temp file error"
	case StatusInvalidStride:
		return "invalid stride"
	case StatusFontTypeMismatch:
		return "font type mismatch"
	case StatusUserFontImmutable:
		return "user font immutable"
	case StatusUserFontError:
		return "user font error"
	case StatusNegativeCount:
		return "negative count"
	case StatusInvalidClusters:
		return "invalid clusters"
	case StatusInvalidSlant:
		return "invalid slant"
	case StatusInvalidWeight:
		return "invalid weight"
	case StatusInvalidSize:
		return "invalid size"
	case StatusUserFontNotImplemented:
		return "user font not implemented"
	case StatusDeviceTypeMismatch:
		return "device type mismatch"
	case StatusDeviceError:
		return "device error"
	case StatusInvalidMeshConstruction:
		return "invalid mesh construction"
	case StatusDeviceFinished:
		return "device finished"
	case StatusJbig2GlobalMissing:
		return "jbig2 global missing"
	case StatusPngError:
		return "png error"
	case StatusFreetypeError:
		return "freetype error"
	case StatusWin32GdiError:
		return "win32 gdi error"
	case StatusTagError:
		return "tag error"
	case StatusDwriteError:
		return "dwrite error"
	case StatusSvgFontError:
		return "svg font error"
	case StatusLastStatus:
		return "last status"
	default:
		return "unknown error"
	}
}

// Helper math functions
func Sin(x float64) float64 {
	return math.Sin(x)
}

func Cos(x float64) float64 {
	return math.Cos(x)
}

// SubpixelOrder represents gopdf_subpixel_order_t - subpixel order for LCD displays
type SubpixelOrder int

const (
	SubpixelOrderDefault SubpixelOrder = iota
	SubpixelOrderRGB
	SubpixelOrderBGR
	SubpixelOrderVRGB
	SubpixelOrderVBGR
)

// HintStyle represents gopdf_hint_style_t - hinting style
type HintStyle int

const (
	HintStyleDefault HintStyle = iota
	HintStyleNone
	HintStyleSlight
	HintStyleMedium
	HintStyleFull
)

// HintMetrics represents gopdf_hint_metrics_t - hinting metrics
type HintMetrics int

const (
	HintMetricsDefault HintMetrics = iota
	HintMetricsOff
	HintMetricsOn
)

// ColorMode represents gopdf_color_mode_t - color mode for fonts
type ColorMode int

const (
	ColorModeDefault ColorMode = iota
	ColorModeNoColor
	ColorModeColor
)

// AdvancedRasterizer is a placeholder for advanced rasterization functionality
// TODO: Implement advanced rasterizer
type AdvancedRasterizer struct {
	// Implementation pending
}

// Reset resets the rasterizer (placeholder)
func (ar *AdvancedRasterizer) Reset() {
	// TODO: Implement
}

// AddLine adds a line to the rasterizer (placeholder)
func (ar *AdvancedRasterizer) AddLine(x0, y0, x1, y1 float64) {
	// TODO: Implement
}

// AddCubicBezier adds a cubic bezier curve to the rasterizer (placeholder)
func (ar *AdvancedRasterizer) AddCubicBezier(x0, y0, x1, y1, x2, y2, x3, y3 float64) {
	// TODO: Implement
}

// Rasterize rasterizes the path (placeholder)
func (ar *AdvancedRasterizer) Rasterize(dst *image.RGBA, fillColor color.Color, fillRule FillRule) error {
	// TODO: Implement
	return nil
}

// NewGlyphTransform creates a new identity glyph transform
func NewGlyphTransform() *GlyphTransform {
	return &GlyphTransform{
		Matrix: *NewMatrix(),
	}
}

// Scale applies scaling to the glyph transform
func (gt *GlyphTransform) Scale(sx, sy float64) {
	scalingMatrix := &Matrix{}
	scalingMatrix.InitScale(sx, sy)
	resultMatrix := &Matrix{}
	MatrixMultiply(resultMatrix, &gt.Matrix, scalingMatrix)
	gt.Matrix = *resultMatrix
}

// Rotate applies rotation to the glyph transform (angle in radians)
func (gt *GlyphTransform) Rotate(angle float64) {
	rotationMatrix := &Matrix{}
	rotationMatrix.InitRotate(angle)
	resultMatrix := &Matrix{}
	MatrixMultiply(resultMatrix, &gt.Matrix, rotationMatrix)
	gt.Matrix = *resultMatrix
}

// Translate applies translation to the glyph transform
func (gt *GlyphTransform) Translate(tx, ty float64) {
	gt.OffsetX += tx
	gt.OffsetY += ty
}
