package gopdf

import (
	"sync/atomic"
	"unsafe"
)

// PatternImpl 表示 PDF 图案的具体实现
type PatternImpl struct {
	PatternType int        // 1 = Tiling, 2 = Shading
	PaintType   int        // 1 = Colored, 2 = Uncolored
	TilingType  int        // 1 = Constant, 2 = No distortion, 3 = Faster
	BBox        []float64  // 边界框 [x1 y1 x2 y2]
	XStep       float64    // X 方向步长
	YStep       float64    // Y 方向步长
	Matrix      *Matrix    // 变换矩阵
	Resources   *Resources // 图案资源
	Stream      []byte     // 图案内容流
}

// NewPattern 创建新的图案
func NewPattern() *PatternImpl {
	return &PatternImpl{
		PatternType: 1, // 默认为 Tiling
		PaintType:   1, // 默认为 Colored
		TilingType:  1, // 默认为 Constant
		Matrix:      NewIdentityMatrix(),
		Resources:   NewResources(),
	}
}

// IsTilingPattern 检查是否为平铺图案
func (p *PatternImpl) IsTilingPattern() bool {
	return p.PatternType == 1
}

// IsShadingPattern 检查是否为阴影图案
func (p *PatternImpl) IsShadingPattern() bool {
	return p.PatternType == 2
}

// IsColoredPattern 检查是否为彩色图案
func (p *PatternImpl) IsColoredPattern() bool {
	return p.PaintType == 1
}

// IsUncoloredPattern 检查是否为无色图案
func (p *PatternImpl) IsUncoloredPattern() bool {
	return p.PaintType == 2
}

// GetBBox 获取边界框
func (p *PatternImpl) GetBBox() (float64, float64, float64, float64) {
	if len(p.BBox) >= 4 {
		return p.BBox[0], p.BBox[1], p.BBox[2], p.BBox[3]
	}
	return 0, 0, 1, 1 // 默认单位正方形
}

// GetWidth 获取图案宽度
func (p *PatternImpl) GetWidth() float64 {
	if len(p.BBox) >= 4 {
		return p.BBox[2] - p.BBox[0]
	}
	return 1
}

// GetHeight 获取图案高度
func (p *PatternImpl) GetHeight() float64 {
	if len(p.BBox) >= 4 {
		return p.BBox[3] - p.BBox[1]
	}
	return 1
}

// solidPattern implements solid color patterns
type solidPattern struct {
	basePattern
	red, green, blue, alpha float64
}

// surfacePattern implements surface patterns
type surfacePattern struct {
	basePattern
	surface Surface
}

// gradientPattern is the base for gradient patterns
type gradientPattern struct {
	basePattern
	stops []gradientStop
}

type gradientStop struct {
	offset                  float64
	red, green, blue, alpha float64
}

// linearGradient implements linear gradient patterns
type linearGradient struct {
	gradientPattern
	x0, y0, x1, y1 float64
}

// radialGradient implements radial gradient patterns
type radialGradient struct {
	gradientPattern
	cx0, cy0, radius0 float64
	cx1, cy1, radius1 float64
}

// meshPattern implements mesh gradient patterns
type meshPattern struct {
	basePattern
	patches      []*MeshPatch
	currentPatch *MeshPatch
}

// MeshPatch represents a single patch in the mesh pattern.
type MeshPatch struct {
	// 4 control points for a Coons patch
	controlPoints [4]Point

	// 4 corner colors
	cornerColors [4]Color
}

// RasterSourceAcquireFunc is the callback function to acquire the surface for a raster source pattern.
type RasterSourceAcquireFunc func(pattern Pattern, target Surface, extents *Rectangle) Surface

// RasterSourceReleaseFunc is the callback function to release the surface for a raster source pattern.
type RasterSourceReleaseFunc func(pattern Pattern, surface Surface)

// rasterSourcePattern implements raster source patterns
type rasterSourcePattern struct {
	basePattern
	acquireFunc RasterSourceAcquireFunc
	releaseFunc RasterSourceReleaseFunc
}

// basePattern provides common pattern functionality
type basePattern struct {
	refCount    int32
	status      Status
	patternType PatternType
	matrix      Matrix
	extend      Extend
	filter      Filter
	userData    map[*UserDataKey]interface{}
}

// NewPatternRGB creates a solid color pattern with RGB values
func NewPatternRGB(red, green, blue float64) Pattern {
	return NewPatternRGBA(red, green, blue, 1.0)
}

// NewPatternRGBA creates a solid color pattern with RGBA values
func NewPatternRGBA(red, green, blue, alpha float64) Pattern {
	pattern := &solidPattern{
		basePattern: basePattern{
			refCount:    1,
			status:      StatusSuccess,
			patternType: PatternTypeSolid,
			extend:      ExtendNone,
			filter:      FilterFast,
			userData:    make(map[*UserDataKey]interface{}),
		},
		red:   red,
		green: green,
		blue:  blue,
		alpha: alpha,
	}
	pattern.matrix.InitIdentity()
	return pattern
}

// NewPatternForSurface creates a pattern from a surface
func NewPatternForSurface(surface Surface) Pattern {
	if surface == nil {
		return newPatternInError(StatusNullPointer)
	}

	pattern := &surfacePattern{
		basePattern: basePattern{
			refCount:    1,
			status:      StatusSuccess,
			patternType: PatternTypeSurface,
			extend:      ExtendNone,
			filter:      FilterFast,
			userData:    make(map[*UserDataKey]interface{}),
		},
		surface: surface.Reference(),
	}
	pattern.matrix.InitIdentity()
	return pattern
}

// NewPatternLinear creates a linear gradient pattern
func NewPatternLinear(x0, y0, x1, y1 float64) Pattern {
	pattern := &linearGradient{
		gradientPattern: gradientPattern{
			basePattern: basePattern{
				refCount:    1,
				status:      StatusSuccess,
				patternType: PatternTypeLinear,
				extend:      ExtendNone,
				filter:      FilterFast,
				userData:    make(map[*UserDataKey]interface{}),
			},
			stops: make([]gradientStop, 0),
		},
		x0: x0, y0: y0,
		x1: x1, y1: y1,
	}
	pattern.matrix.InitIdentity()
	return pattern
}

// NewPatternMesh creates a new mesh pattern.
func NewPatternMesh() Pattern {
	pattern := &meshPattern{
		basePattern: basePattern{
			refCount:    1,
			status:      StatusSuccess,
			patternType: PatternTypeMesh,
			extend:      ExtendNone,
			filter:      FilterFast,
			userData:    make(map[*UserDataKey]interface{}),
		},
		patches: make([]*MeshPatch, 0),
	}
	pattern.matrix.InitIdentity()
	return pattern
}

// MeshPatternBeginPatch starts a new patch.
func (p *meshPattern) MeshPatternBeginPatch() error {
	if p.currentPatch != nil {
		return newError(StatusInvalidMeshConstruction, "patch already in progress")
	}
	p.currentPatch = &MeshPatch{}
	return nil
}

// MeshPatternEndPatch ends the current patch and adds it to the pattern.
func (p *meshPattern) MeshPatternEndPatch() error {
	if p.currentPatch == nil {
		return newError(StatusInvalidMeshConstruction, "no patch in progress")
	}
	p.patches = append(p.patches, p.currentPatch)
	p.currentPatch = nil
	return nil
}

// MeshPatternSetControlPoint sets a control point for the current patch.
func (p *meshPattern) MeshPatternSetControlPoint(pointNum int, x, y float64) error {
	if p.currentPatch == nil {
		return newError(StatusInvalidMeshConstruction, "no patch in progress")
	}
	if pointNum < 0 || pointNum > 3 {
		return newError(StatusInvalidIndex, "control point index out of range (0-3)")
	}
	p.currentPatch.controlPoints[pointNum] = Point{X: x, Y: y}
	return nil
}

// MeshPatternSetCornerColor sets a corner color for the current patch.
func (p *meshPattern) MeshPatternSetCornerColor(cornerNum int, red, green, blue, alpha float64) error {
	if p.currentPatch == nil {
		return newError(StatusInvalidMeshConstruction, "no patch in progress")
	}
	if cornerNum < 0 || cornerNum > 3 {
		return newError(StatusInvalidIndex, "corner color index out of range (0-3)")
	}
	p.currentPatch.cornerColors[cornerNum] = Color{R: red, G: green, B: blue, A: alpha}
	return nil
}

// NewPatternRasterSource creates a new raster source pattern.
func NewPatternRasterSource(acquireFunc RasterSourceAcquireFunc, releaseFunc RasterSourceReleaseFunc) Pattern {
	pattern := &rasterSourcePattern{
		basePattern: basePattern{
			refCount:    1,
			status:      StatusSuccess,
			patternType: PatternTypeRasterSource,
			extend:      ExtendNone,
			filter:      FilterFast,
			userData:    make(map[*UserDataKey]interface{}),
		},
		acquireFunc: acquireFunc,
		releaseFunc: releaseFunc,
	}
	pattern.matrix.InitIdentity()
	return pattern
}

// radialGradient implements radial gradient patterns
func NewPatternRadial(cx0, cy0, radius0, cx1, cy1, radius1 float64) Pattern {
	pattern := &radialGradient{
		gradientPattern: gradientPattern{
			basePattern: basePattern{
				refCount:    1,
				status:      StatusSuccess,
				patternType: PatternTypeRadial,
				extend:      ExtendNone,
				filter:      FilterFast,
				userData:    make(map[*UserDataKey]interface{}),
			},
			stops: make([]gradientStop, 0),
		},
		cx0: cx0, cy0: cy0, radius0: radius0,
		cx1: cx1, cy1: cy1, radius1: radius1,
	}
	pattern.matrix.InitIdentity()
	return pattern
}

func newPatternInError(status Status) Pattern {
	pattern := &solidPattern{
		basePattern: basePattern{
			refCount:    1,
			status:      status,
			patternType: PatternTypeSolid,
			userData:    make(map[*UserDataKey]interface{}),
		},
	}
	return pattern
}

// Base pattern interface implementation

func (p *basePattern) Reference() Pattern {
	atomic.AddInt32(&p.refCount, 1)
	// Return the actual pattern type, not basePattern
	return p.getPattern()
}

func (p *basePattern) getPattern() Pattern {
	// This is a bit of a hack - in a real implementation we'd need
	// to store a reference to the concrete type
	return nil // This will be overridden in concrete types
}

func (p *basePattern) Destroy() {
	if atomic.AddInt32(&p.refCount, -1) == 0 {
		// Clean up resources specific to pattern type
		p.cleanup()
	}
}

func (p *basePattern) cleanup() {
	// Base cleanup - overridden in concrete types
}

func (p *basePattern) GetReferenceCount() int {
	return int(atomic.LoadInt32(&p.refCount))
}

func (p *basePattern) Status() Status {
	return p.status
}

func (p *basePattern) GetType() PatternType {
	return p.patternType
}

func (p *basePattern) SetUserData(key *UserDataKey, userData unsafe.Pointer, destroy DestroyFunc) Status {
	if p.status != StatusSuccess {
		return p.status
	}

	p.userData[key] = userData
	// TODO: Store destroy function and call it when appropriate
	return StatusSuccess
}

func (p *basePattern) GetUserData(key *UserDataKey) unsafe.Pointer {
	if data, exists := p.userData[key]; exists {
		return data.(unsafe.Pointer)
	}
	return nil
}

func (p *basePattern) SetMatrix(matrix *Matrix) {
	if p.status != StatusSuccess {
		return
	}
	p.matrix = *matrix
}

func (p *basePattern) GetMatrix() *Matrix {
	matrix := &Matrix{}
	*matrix = p.matrix
	return matrix
}

func (p *basePattern) SetExtend(extend Extend) {
	if p.status != StatusSuccess {
		return
	}
	p.extend = extend
}

func (p *basePattern) GetExtend() Extend {
	return p.extend
}

func (p *basePattern) SetFilter(filter Filter) {
	if p.status != StatusSuccess {
		return
	}
	p.filter = filter
}

func (p *basePattern) GetFilter() Filter {
	return p.filter
}

// Solid pattern implementation

// (deleted unused getPattern)

func (p *solidPattern) Reference() Pattern {
	atomic.AddInt32(&p.refCount, 1)
	return p
}

func (p *solidPattern) GetRGBA() (red, green, blue, alpha float64) {
	return p.red, p.green, p.blue, p.alpha
}

// Surface pattern implementation

// ... existing code ...

// (deleted unused getPattern)

// (deleted unused cleanup)

func (p *surfacePattern) GetSurface() Surface {
	return p.surface.Reference()
}

func (p *surfacePattern) Reference() Pattern {
	atomic.AddInt32(&p.refCount, 1)
	return p
}

// (deleted unused getPattern)

// (deleted unused cleanup)

// Mesh pattern implementation

// ... existing code ...

// ... existing code ...

// Raster Source pattern implementation

// ... existing code ...

// linearGradient implementation
func (p *gradientPattern) AddColorStopRGB(offset, red, green, blue float64) Status {
	if p.status != StatusSuccess {
		return p.status
	}

	if offset < 0.0 || offset > 1.0 {
		p.status = StatusInvalidIndex
		return p.status
	}

	// TODO: Add support for HSV interpolation as suggested in the document.
	// This would require a separate function or flag to determine the interpolation mode.

	stop := gradientStop{
		offset: offset,
		red:    red,
		green:  green,
		blue:   blue,
		alpha:  1.0,
	}

	// Insert in sorted order by offset
	inserted := false
	for i, existingStop := range p.stops {
		if offset <= existingStop.offset {
			// Insert at position i
			p.stops = append(p.stops[:i], append([]gradientStop{stop}, p.stops[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted {
		p.stops = append(p.stops, stop)
	}
	return StatusSuccess
}

func (p *gradientPattern) AddColorStopRGBA(offset, red, green, blue, alpha float64) Status {
	if p.status != StatusSuccess {
		return p.status
	}

	if offset < 0.0 || offset > 1.0 {
		p.status = StatusInvalidIndex
		return p.status
	}

	// TODO: Add support for HSV interpolation as suggested in the document.
	// This would require a separate function or flag to determine the interpolation mode.

	stop := gradientStop{
		offset: offset,
		red:    red,
		green:  green,
		blue:   blue,
		alpha:  alpha,
	}

	// Insert in sorted order by offset
	inserted := false
	for i, existingStop := range p.stops {
		if offset <= existingStop.offset {
			// Insert at position i
			p.stops = append(p.stops[:i], append([]gradientStop{stop}, p.stops[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted {
		p.stops = append(p.stops, stop)
	}
	return StatusSuccess
}

func (p *gradientPattern) GetColorStopCount() int {
	return len(p.stops)
}

func (p *gradientPattern) GetColorStop(index int) (offset, red, green, blue, alpha float64, status Status) {
	if index < 0 || index >= len(p.stops) {
		return 0, 0, 0, 0, 0, StatusInvalidIndex
	}

	stop := p.stops[index]
	return stop.offset, stop.red, stop.green, stop.blue, stop.alpha, StatusSuccess
}

// Linear gradient implementation

func (p *linearGradient) Reference() Pattern {
	atomic.AddInt32(&p.refCount, 1)
	return p
}

func (p *linearGradient) GetLinearPoints() (x0, y0, x1, y1 float64) {
	return p.x0, p.y0, p.x1, p.y1
}

// Radial gradient implementation

func (p *radialGradient) Reference() Pattern {
	atomic.AddInt32(&p.refCount, 1)
	return p
}

func (p *radialGradient) GetRadialCircles() (cx0, cy0, radius0, cx1, cy1, radius1 float64) {
	return p.cx0, p.cy0, p.radius0, p.cx1, p.cy1, p.radius1
}

// Pattern-specific interfaces for type assertions

type SolidPattern interface {
	Pattern
	GetRGBA() (red, green, blue, alpha float64)
}

type SurfacePattern interface {
	Pattern
	GetSurface() Surface
}

type GradientPattern interface {
	Pattern
	AddColorStopRGB(offset, red, green, blue float64) Status
	AddColorStopRGBA(offset, red, green, blue, alpha float64) Status
	GetColorStopCount() int
	GetColorStop(index int) (offset, red, green, blue, alpha float64, status Status)
}

type LinearGradientPattern interface {
	GradientPattern
	GetLinearPoints() (x0, y0, x1, y1 float64)
}

type RadialGradientPattern interface {
	GradientPattern
	GetRadialCircles() (cx0, cy0, radius0, cx1, cy1, radius1 float64)
}
