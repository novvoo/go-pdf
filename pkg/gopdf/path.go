package gopdf

// Path 表示 PDF 路径
type Path struct {
	subpaths []*Subpath
	current  *Subpath
}

// Subpath 表示子路径
type Subpath struct {
	segments []PathSegment
	closed   bool
}

// PathSegment 表示路径段
type PathSegment interface {
	Type() string
}

// MoveToSegment 移动到
type MoveToSegment struct {
	X, Y float64
}

func (s *MoveToSegment) Type() string { return "MoveTo" }

// LineToSegment 直线到
type LineToSegment struct {
	X, Y float64
}

func (s *LineToSegment) Type() string { return "LineTo" }

// CurveToSegment 三次贝塞尔曲线
type CurveToSegment struct {
	X1, Y1, X2, Y2, X3, Y3 float64
}

func (s *CurveToSegment) Type() string { return "CurveTo" }

// RectangleSegment 矩形
type RectangleSegment struct {
	X, Y, Width, Height float64
}

func (s *RectangleSegment) Type() string { return "Rectangle" }

// NewPath 创建新路径
func NewPath() *Path {
	return &Path{
		subpaths: make([]*Subpath, 0),
	}
}

// MoveTo 移动到新位置（开始新的子路径）
func (p *Path) MoveTo(x, y float64) {
	p.current = &Subpath{
		segments: []PathSegment{&MoveToSegment{X: x, Y: y}},
		closed:   false,
	}
	p.subpaths = append(p.subpaths, p.current)
}

// LineTo 添加直线段
func (p *Path) LineTo(x, y float64) {
	if p.current == nil {
		p.MoveTo(x, y)
		return
	}
	p.current.segments = append(p.current.segments, &LineToSegment{X: x, Y: y})
}

// CurveTo 添加三次贝塞尔曲线段
func (p *Path) CurveTo(x1, y1, x2, y2, x3, y3 float64) {
	if p.current == nil {
		p.MoveTo(x1, y1)
	}
	p.current.segments = append(p.current.segments, &CurveToSegment{
		X1: x1, Y1: y1,
		X2: x2, Y2: y2,
		X3: x3, Y3: y3,
	})
}

// Rectangle 添加矩形
func (p *Path) Rectangle(x, y, width, height float64) {
	p.MoveTo(x, y)
	p.current.segments = append(p.current.segments, &RectangleSegment{
		X: x, Y: y, Width: width, Height: height,
	})
	p.current.closed = true
}

// ClosePath 闭合当前子路径
func (p *Path) ClosePath() {
	if p.current != nil {
		p.current.closed = true
	}
}

// Clear 清空路径
func (p *Path) Clear() {
	p.subpaths = make([]*Subpath, 0)
	p.current = nil
}

// IsEmpty 检查路径是否为空
func (p *Path) IsEmpty() bool {
	return len(p.subpaths) == 0
}

// GetSubpaths 获取所有子路径
func (p *Path) GetSubpaths() []*Subpath {
	return p.subpaths
}

// IsClosed 检查子路径是否闭合
func (sp *Subpath) IsClosed() bool {
	return sp.closed
}

// GetSegments 获取子路径的所有段
func (sp *Subpath) GetSegments() []PathSegment {
	return sp.segments
}
