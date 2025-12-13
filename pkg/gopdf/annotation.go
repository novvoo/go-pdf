package gopdf

// Annotation 表示 PDF 注释
// 注释是页面上的交互元素，如文本注释、高亮、链接等
type Annotation struct {
	Subtype    string                 // 注释子类型（Text, Highlight, Link 等）
	Rect       []float64              // 注释矩形 [x1 y1 x2 y2]
	Contents   string                 // 注释内容文本
	Color      []float64              // 注释颜色（RGB 或 CMYK）
	Appearance map[string]interface{} // 外观流字典（AP entry）
	Flags      int                    // 注释标志
	QuadPoints []float64              // 四边形点（用于高亮等）
	Name       string                 // 注释名称（用于某些类型）
}

// NewAnnotation 创建新的注释
func NewAnnotation(subtype string) *Annotation {
	return &Annotation{
		Subtype:    subtype,
		Rect:       make([]float64, 4),
		Color:      make([]float64, 3),
		Appearance: make(map[string]interface{}),
		Flags:      0,
	}
}

// GetRect 获取注释矩形
func (a *Annotation) GetRect() (x1, y1, x2, y2 float64) {
	if len(a.Rect) >= 4 {
		return a.Rect[0], a.Rect[1], a.Rect[2], a.Rect[3]
	}
	return 0, 0, 0, 0
}

// GetColor 获取注释颜色（RGB）
func (a *Annotation) GetColor() (r, g, b float64) {
	if len(a.Color) >= 3 {
		return a.Color[0], a.Color[1], a.Color[2]
	}
	return 0, 0, 0
}

// IsVisible 检查注释是否可见
func (a *Annotation) IsVisible() bool {
	// 检查 Hidden 标志（bit 1）
	return (a.Flags & 0x02) == 0
}

// IsPrintable 检查注释是否可打印
func (a *Annotation) IsPrintable() bool {
	// 检查 Print 标志（bit 2）
	return (a.Flags & 0x04) != 0
}
