package gopdf

// Pattern 表示 PDF 图案
type Pattern struct {
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
func NewPattern() *Pattern {
	return &Pattern{
		PatternType: 1, // 默认为 Tiling
		PaintType:   1, // 默认为 Colored
		TilingType:  1, // 默认为 Constant
		Matrix:      NewIdentityMatrix(),
		Resources:   NewResources(),
	}
}

// IsTilingPattern 检查是否为平铺图案
func (p *Pattern) IsTilingPattern() bool {
	return p.PatternType == 1
}

// IsShadingPattern 检查是否为阴影图案
func (p *Pattern) IsShadingPattern() bool {
	return p.PatternType == 2
}

// IsColoredPattern 检查是否为彩色图案
func (p *Pattern) IsColoredPattern() bool {
	return p.PaintType == 1
}

// IsUncoloredPattern 检查是否为无色图案
func (p *Pattern) IsUncoloredPattern() bool {
	return p.PaintType == 2
}

// GetBBox 获取边界框
func (p *Pattern) GetBBox() (float64, float64, float64, float64) {
	if len(p.BBox) >= 4 {
		return p.BBox[0], p.BBox[1], p.BBox[2], p.BBox[3]
	}
	return 0, 0, 1, 1 // 默认单位正方形
}

// GetWidth 获取图案宽度
func (p *Pattern) GetWidth() float64 {
	if len(p.BBox) >= 4 {
		return p.BBox[2] - p.BBox[0]
	}
	return 1
}

// GetHeight 获取图案高度
func (p *Pattern) GetHeight() float64 {
	if len(p.BBox) >= 4 {
		return p.BBox[3] - p.BBox[1]
	}
	return 1
}
