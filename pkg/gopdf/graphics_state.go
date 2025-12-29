package gopdf

// GraphicsState 表示 PDF 图形状态
// 包含当前变换矩阵 (CTM)、颜色、线宽等
type GraphicsState struct {
	CTM               *Matrix              // 当前变换矩阵
	StrokeColor       *Color               // 描边颜色
	FillColor         *Color               // 填充颜色
	LineWidth         float64              // 线宽
	LineCap           LineCap              // 线端点样式
	LineJoin          LineJoin             // 线连接样式
	MiterLimit        float64              // 斜接限制
	DashPattern       []float64            // 虚线模式
	DashOffset        float64              // 虚线偏移
	CoordConverter    *CoordinateConverter // 坐标转换器
	BlendMode         string               // 混合模式
	FillAlpha         float64              // 填充透明度 (ca)
	StrokeAlpha       float64              // 描边透明度 (CA)
	TransparencyGroup *TransparencyGroup   // 当前透明度组（如果有）
	SoftMask          *SoftMask            // 软遮罩
	StrokeColorSpace  ColorSpace           // 描边颜色空间
	FillColorSpace    ColorSpace           // 填充颜色空间
	AlphaIsShape      bool                 // Alpha 是否为形状（AIS）
	TextKnockout      bool                 // 文本敲除（TK）
	OverprintMode     int                  // 叠印模式（OPM）
}

// NewGraphicsState 创建新的图形状态
func NewGraphicsState(width, height float64) *GraphicsState {
	return &GraphicsState{
		CTM:               NewIdentityMatrix(),
		StrokeColor:       &Color{R: 0, G: 0, B: 0, A: 1}, // 黑色
		FillColor:         &Color{R: 0, G: 0, B: 0, A: 1}, // 黑色
		LineWidth:         1.0,
		LineCap:           LineCapButt,
		LineJoin:          LineJoinMiter,
		MiterLimit:        10.0,
		DashPattern:       nil,
		DashOffset:        0,
		CoordConverter:    NewCoordinateConverter(width, height, CoordSystemPDF),
		BlendMode:         "Normal",
		FillAlpha:         1.0,
		StrokeAlpha:       1.0,
		TransparencyGroup: nil,
		SoftMask:          nil,
		StrokeColorSpace:  &DeviceRGBColorSpace{},
		FillColorSpace:    &DeviceRGBColorSpace{},
		AlphaIsShape:      false,
		TextKnockout:      true,
		OverprintMode:     0,
	}
}

// Clone 复制图形状态
func (gs *GraphicsState) Clone() *GraphicsState {
	newState := &GraphicsState{
		CTM:               gs.CTM.Clone(),
		StrokeColor:       &Color{R: gs.StrokeColor.R, G: gs.StrokeColor.G, B: gs.StrokeColor.B, A: gs.StrokeColor.A},
		FillColor:         &Color{R: gs.FillColor.R, G: gs.FillColor.G, B: gs.FillColor.B, A: gs.FillColor.A},
		LineWidth:         gs.LineWidth,
		LineCap:           gs.LineCap,
		LineJoin:          gs.LineJoin,
		MiterLimit:        gs.MiterLimit,
		DashOffset:        gs.DashOffset,
		CoordConverter:    gs.CoordConverter,
		BlendMode:         gs.BlendMode,
		FillAlpha:         gs.FillAlpha,
		StrokeAlpha:       gs.StrokeAlpha,
		TransparencyGroup: gs.TransparencyGroup, // 共享透明度组引用
		SoftMask:          gs.SoftMask,          // 共享软遮罩引用
		StrokeColorSpace:  gs.StrokeColorSpace,
		FillColorSpace:    gs.FillColorSpace,
		AlphaIsShape:      gs.AlphaIsShape,
		TextKnockout:      gs.TextKnockout,
		OverprintMode:     gs.OverprintMode,
	}

	if gs.DashPattern != nil {
		newState.DashPattern = make([]float64, len(gs.DashPattern))
		copy(newState.DashPattern, gs.DashPattern)
	}

	return newState
}

// ApplyToGopdfContext 将图形状态应用到 Gopdf context
func (gs *GraphicsState) ApplyToGopdfContext(ctx Context) {
	// 应用变换矩阵
	gs.CTM.SetGopdfContextMatrix(ctx)

	// 应用描边颜色
	if gs.StrokeColor != nil {
		ctx.SetSourceRGBA(gs.StrokeColor.R, gs.StrokeColor.G, gs.StrokeColor.B, gs.StrokeColor.A)
	}

	// 应用线宽
	ctx.SetLineWidth(gs.LineWidth)

	// 应用线端点样式
	ctx.SetLineCap(gs.LineCap)

	// 应用线连接样式
	ctx.SetLineJoin(gs.LineJoin)

	// 应用斜接限制
	ctx.SetMiterLimit(gs.MiterLimit)

	// 应用虚线模式
	if len(gs.DashPattern) > 0 {
		ctx.SetDash(gs.DashPattern, gs.DashOffset)
	}
}

// Translate 平移 CTM
func (gs *GraphicsState) Translate(tx, ty float64) {
	gs.CTM = gs.CTM.Translate(tx, ty)
}

// Scale 缩放 CTM
func (gs *GraphicsState) Scale(sx, sy float64) {
	gs.CTM = gs.CTM.Scale(sx, sy)
}

// Rotate 旋转 CTM（弧度）
func (gs *GraphicsState) Rotate(angle float64) {
	gs.CTM = gs.CTM.Rotate(angle)
}

// RotateDegrees 旋转 CTM（度）
func (gs *GraphicsState) RotateDegrees(degrees float64) {
	gs.CTM = gs.CTM.RotateDegrees(degrees)
}

// SetStrokeColor 设置描边颜色
func (gs *GraphicsState) SetStrokeColor(r, g, b, a float64) {
	gs.StrokeColor = &Color{R: r, G: g, B: b, A: a}
}

// SetFillColor 设置填充颜色
func (gs *GraphicsState) SetFillColor(r, g, b, a float64) {
	gs.FillColor = &Color{R: r, G: g, B: b, A: a}
}

// SetLineWidth 设置线宽
func (gs *GraphicsState) SetLineWidth(width float64) {
	gs.LineWidth = width
}

// SetDash 设置虚线模式
func (gs *GraphicsState) SetDash(pattern []float64, offset float64) {
	gs.DashPattern = pattern
	gs.DashOffset = offset
}

// GraphicsStateStack 图形状态栈
// 用于实现 PDF 的 q/Q 操作符（保存/恢复图形状态）
type GraphicsStateStack struct {
	stack []*GraphicsState
}

// NewGraphicsStateStack 创建新的图形状态栈
func NewGraphicsStateStack(width, height float64) *GraphicsStateStack {
	return &GraphicsStateStack{
		stack: []*GraphicsState{NewGraphicsState(width, height)},
	}
}

// Current 获取当前图形状态
func (s *GraphicsStateStack) Current() *GraphicsState {
	if len(s.stack) == 0 {
		return nil
	}
	return s.stack[len(s.stack)-1]
}

// Push 保存当前图形状态（q 操作符）
func (s *GraphicsStateStack) Push() {
	current := s.Current()
	if current != nil {
		s.stack = append(s.stack, current.Clone())
	}
}

// Pop 恢复之前的图形状态（Q 操作符）
func (s *GraphicsStateStack) Pop() *GraphicsState {
	if len(s.stack) <= 1 {
		return s.Current() // 保持至少一个状态
	}

	popped := s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]
	return popped
}

// Depth 返回栈深度
func (s *GraphicsStateStack) Depth() int {
	return len(s.stack)
}
