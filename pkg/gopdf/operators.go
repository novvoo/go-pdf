package gopdf

import (
	"fmt"

	"github.com/novvoo/go-cairo/pkg/cairo"
)

// PDFOperator 表示 PDF 操作符接口
type PDFOperator interface {
	Execute(ctx *RenderContext) error
	Name() string
}

// RenderContext PDF 渲染上下文
type RenderContext struct {
	CairoCtx           cairo.Context
	GraphicsStack      *GraphicsStateStack
	MarkedContentStack *MarkedContentStack
	CurrentPath        *Path
	TextState          *TextState
	Resources          *Resources
	XObjectCache       map[string]cairo.Surface
}

// NewRenderContext 创建新的渲染上下文
func NewRenderContext(cairoCtx cairo.Context, width, height float64) *RenderContext {
	return &RenderContext{
		CairoCtx:           cairoCtx,
		GraphicsStack:      NewGraphicsStateStack(width, height),
		MarkedContentStack: NewMarkedContentStack(),
		CurrentPath:        NewPath(),
		TextState:          NewTextState(),
		Resources:          NewResources(),
		XObjectCache:       make(map[string]cairo.Surface),
	}
}

// GetCurrentState 获取当前图形状态
func (rc *RenderContext) GetCurrentState() *GraphicsState {
	return rc.GraphicsStack.Current()
}

// ===== 图形状态操作符 =====

// OpSaveState q - 保存图形状态
type OpSaveState struct{}

func (op *OpSaveState) Name() string { return "q" }

func (op *OpSaveState) Execute(ctx *RenderContext) error {
	ctx.GraphicsStack.Push()
	ctx.CairoCtx.Save()
	debugPrintf("[q] Save graphics state - Stack depth: %d\n", ctx.GraphicsStack.Depth())
	return nil
}

// OpRestoreState Q - 恢复图形状态
type OpRestoreState struct{}

func (op *OpRestoreState) Name() string { return "Q" }

func (op *OpRestoreState) Execute(ctx *RenderContext) error {
	ctx.GraphicsStack.Pop()
	ctx.CairoCtx.Restore()
	debugPrintf("[Q] Restore graphics state - Stack depth: %d\n", ctx.GraphicsStack.Depth())
	return nil
}

// OpConcatMatrix cm - 连接变换矩阵
type OpConcatMatrix struct {
	Matrix *Matrix
}

func (op *OpConcatMatrix) Name() string { return "cm" }

func (op *OpConcatMatrix) Execute(ctx *RenderContext) error {
	state := ctx.GetCurrentState()
	oldCTM := state.CTM.Clone()

	// PDF 规范: CTM_new = cm × CTM_old (右乘)
	// 这意味着新的变换矩阵应该是 op.Matrix.Multiply(state.CTM)
	state.CTM = op.Matrix.Multiply(state.CTM)

	// 对于 Cairo，我们只需要应用增量变换（cm 矩阵本身）
	// Cairo 的 Transform 会自动与当前矩阵组合
	op.Matrix.ApplyToCairoContext(ctx.CairoCtx)

	debugPrintf("[cm] Concat matrix: [%.2f %.2f %.2f %.2f %.2f %.2f]\n",
		op.Matrix.A, op.Matrix.B, op.Matrix.C, op.Matrix.D, op.Matrix.E, op.Matrix.F)
	debugPrintf("     Old CTM: [%.2f %.2f %.2f %.2f %.2f %.2f]\n",
		oldCTM.A, oldCTM.B, oldCTM.C, oldCTM.D, oldCTM.E, oldCTM.F)
	debugPrintf("     New CTM: [%.2f %.2f %.2f %.2f %.2f %.2f]\n",
		state.CTM.A, state.CTM.B, state.CTM.C, state.CTM.D, state.CTM.E, state.CTM.F)
	return nil
}

// OpSetLineWidth w - 设置线宽
type OpSetLineWidth struct {
	Width float64
}

func (op *OpSetLineWidth) Name() string { return "w" }

func (op *OpSetLineWidth) Execute(ctx *RenderContext) error {
	state := ctx.GetCurrentState()
	state.LineWidth = op.Width
	ctx.CairoCtx.SetLineWidth(op.Width)
	return nil
}

// OpSetLineCap J - 设置线端点样式
type OpSetLineCap struct {
	Cap int // 0=butt, 1=round, 2=square
}

func (op *OpSetLineCap) Name() string { return "J" }

func (op *OpSetLineCap) Execute(ctx *RenderContext) error {
	state := ctx.GetCurrentState()
	var cap cairo.LineCap
	switch op.Cap {
	case 0:
		cap = cairo.LineCapButt
	case 1:
		cap = cairo.LineCapRound
	case 2:
		cap = cairo.LineCapSquare
	default:
		cap = cairo.LineCapButt
	}
	state.LineCap = cap
	ctx.CairoCtx.SetLineCap(cap)
	return nil
}

// OpSetLineJoin j - 设置线连接样式
type OpSetLineJoin struct {
	Join int // 0=miter, 1=round, 2=bevel
}

func (op *OpSetLineJoin) Name() string { return "j" }

func (op *OpSetLineJoin) Execute(ctx *RenderContext) error {
	state := ctx.GetCurrentState()
	var join cairo.LineJoin
	switch op.Join {
	case 0:
		join = cairo.LineJoinMiter
	case 1:
		join = cairo.LineJoinRound
	case 2:
		join = cairo.LineJoinBevel
	default:
		join = cairo.LineJoinMiter
	}
	state.LineJoin = join
	ctx.CairoCtx.SetLineJoin(join)
	return nil
}

// OpSetMiterLimit M - 设置斜接限制
type OpSetMiterLimit struct {
	Limit float64
}

func (op *OpSetMiterLimit) Name() string { return "M" }

func (op *OpSetMiterLimit) Execute(ctx *RenderContext) error {
	state := ctx.GetCurrentState()
	state.MiterLimit = op.Limit
	ctx.CairoCtx.SetMiterLimit(op.Limit)
	return nil
}

// OpSetDash d - 设置虚线模式
type OpSetDash struct {
	Pattern []float64
	Offset  float64
}

func (op *OpSetDash) Name() string { return "d" }

func (op *OpSetDash) Execute(ctx *RenderContext) error {
	state := ctx.GetCurrentState()
	state.SetDash(op.Pattern, op.Offset)
	ctx.CairoCtx.SetDash(op.Pattern, op.Offset)
	return nil
}

// OpSetGraphicsState gs - 设置图形状态参数
type OpSetGraphicsState struct {
	DictName string
}

func (op *OpSetGraphicsState) Name() string { return "gs" }

func (op *OpSetGraphicsState) Execute(ctx *RenderContext) error {
	// 从资源字典中获取扩展图形状态
	extGState := ctx.Resources.GetExtGState(op.DictName)
	if extGState == nil {
		return fmt.Errorf("graphics state %s not found", op.DictName)
	}

	state := ctx.GetCurrentState()

	// 应用扩展图形状态参数
	if lw, ok := extGState["LW"].(float64); ok {
		state.LineWidth = lw
		ctx.CairoCtx.SetLineWidth(lw)
	}

	if lc, ok := extGState["LC"].(int); ok {
		(&OpSetLineCap{Cap: lc}).Execute(ctx)
	}

	if lj, ok := extGState["LJ"].(int); ok {
		(&OpSetLineJoin{Join: lj}).Execute(ctx)
	}

	if ml, ok := extGState["ML"].(float64); ok {
		state.MiterLimit = ml
		ctx.CairoCtx.SetMiterLimit(ml)
	}

	// 混合模式
	if bm, ok := extGState["BM"].(string); ok {
		state.SetBlendMode(bm)
		state.ApplyBlendMode(ctx.CairoCtx)
		debugPrintf("[gs] Set blend mode: %s\n", bm)
	}

	// 填充透明度
	if ca, ok := extGState["ca"].(float64); ok {
		state.SetFillAlpha(ca)
		debugPrintf("[gs] Set fill alpha: %.2f\n", ca)
	}

	// 描边透明度
	if CA, ok := extGState["CA"].(float64); ok {
		state.SetStrokeAlpha(CA)
		debugPrintf("[gs] Set stroke alpha: %.2f\n", CA)
	}

	return nil
}

// ===== 路径构造操作符 =====

// OpMoveTo m - 移动到
type OpMoveTo struct {
	X, Y float64
}

func (op *OpMoveTo) Name() string { return "m" }

func (op *OpMoveTo) Execute(ctx *RenderContext) error {
	ctx.CurrentPath.MoveTo(op.X, op.Y)
	ctx.CairoCtx.MoveTo(op.X, op.Y)
	return nil
}

// OpLineTo l - 直线到
type OpLineTo struct {
	X, Y float64
}

func (op *OpLineTo) Name() string { return "l" }

func (op *OpLineTo) Execute(ctx *RenderContext) error {
	ctx.CurrentPath.LineTo(op.X, op.Y)
	ctx.CairoCtx.LineTo(op.X, op.Y)
	return nil
}

// OpCurveTo c - 三次贝塞尔曲线
type OpCurveTo struct {
	X1, Y1, X2, Y2, X3, Y3 float64
}

func (op *OpCurveTo) Name() string { return "c" }

func (op *OpCurveTo) Execute(ctx *RenderContext) error {
	ctx.CurrentPath.CurveTo(op.X1, op.Y1, op.X2, op.Y2, op.X3, op.Y3)
	ctx.CairoCtx.CurveTo(op.X1, op.Y1, op.X2, op.Y2, op.X3, op.Y3)
	return nil
}

// OpCurveToV v - 三次贝塞尔曲线（初始点重复）
type OpCurveToV struct {
	X2, Y2, X3, Y3 float64
}

func (op *OpCurveToV) Name() string { return "v" }

func (op *OpCurveToV) Execute(ctx *RenderContext) error {
	// 当前点作为第一个控制点
	x, y := ctx.CairoCtx.GetCurrentPoint()
	ctx.CurrentPath.CurveTo(x, y, op.X2, op.Y2, op.X3, op.Y3)
	ctx.CairoCtx.CurveTo(x, y, op.X2, op.Y2, op.X3, op.Y3)
	return nil
}

// OpCurveToY y - 三次贝塞尔曲线（终点重复）
type OpCurveToY struct {
	X1, Y1, X3, Y3 float64
}

func (op *OpCurveToY) Name() string { return "y" }

func (op *OpCurveToY) Execute(ctx *RenderContext) error {
	// 终点作为第二个控制点
	ctx.CurrentPath.CurveTo(op.X1, op.Y1, op.X3, op.Y3, op.X3, op.Y3)
	ctx.CairoCtx.CurveTo(op.X1, op.Y1, op.X3, op.Y3, op.X3, op.Y3)
	return nil
}

// OpRectangle re - 矩形
type OpRectangle struct {
	X, Y, Width, Height float64
}

func (op *OpRectangle) Name() string { return "re" }

func (op *OpRectangle) Execute(ctx *RenderContext) error {
	ctx.CurrentPath.Rectangle(op.X, op.Y, op.Width, op.Height)
	ctx.CairoCtx.Rectangle(op.X, op.Y, op.Width, op.Height)
	return nil
}

// OpClosePath h - 闭合路径
type OpClosePath struct{}

func (op *OpClosePath) Name() string { return "h" }

func (op *OpClosePath) Execute(ctx *RenderContext) error {
	ctx.CurrentPath.ClosePath()
	ctx.CairoCtx.ClosePath()
	return nil
}

// ===== 路径绘制操作符 =====

// OpStroke S - 描边
type OpStroke struct{}

func (op *OpStroke) Name() string { return "S" }

func (op *OpStroke) Execute(ctx *RenderContext) error {
	state := ctx.GetCurrentState()
	filler := NewPathFiller(ctx.CairoCtx)

	if err := filler.StrokePath(ctx.CurrentPath, state.StrokeColor, state.LineWidth); err != nil {
		return err
	}

	ctx.CurrentPath.Clear()
	return nil
}

// OpCloseAndStroke s - 闭合并描边
type OpCloseAndStroke struct{}

func (op *OpCloseAndStroke) Name() string { return "s" }

func (op *OpCloseAndStroke) Execute(ctx *RenderContext) error {
	ctx.CairoCtx.ClosePath()
	return (&OpStroke{}).Execute(ctx)
}

// OpFill f/F - 填充（非零缠绕规则）
type OpFill struct{}

func (op *OpFill) Name() string { return "f" }

func (op *OpFill) Execute(ctx *RenderContext) error {
	state := ctx.GetCurrentState()
	filler := NewPathFiller(ctx.CairoCtx)
	filler.SetFillRule(FillRuleNonZero)

	if err := filler.FillPath(ctx.CurrentPath, state.FillColor); err != nil {
		return err
	}

	ctx.CurrentPath.Clear()
	return nil
}

// OpFillEvenOdd f* - 填充（奇偶规则）
type OpFillEvenOdd struct{}

func (op *OpFillEvenOdd) Name() string { return "f*" }

func (op *OpFillEvenOdd) Execute(ctx *RenderContext) error {
	state := ctx.GetCurrentState()
	filler := NewPathFiller(ctx.CairoCtx)
	filler.SetFillRule(FillRuleEvenOdd)

	if err := filler.FillPath(ctx.CurrentPath, state.FillColor); err != nil {
		return err
	}

	ctx.CurrentPath.Clear()
	return nil
}

// OpFillAndStroke B - 填充并描边
type OpFillAndStroke struct{}

func (op *OpFillAndStroke) Name() string { return "B" }

func (op *OpFillAndStroke) Execute(ctx *RenderContext) error {
	state := ctx.GetCurrentState()
	filler := NewPathFiller(ctx.CairoCtx)
	filler.SetFillRule(FillRuleNonZero)

	if err := filler.FillAndStrokePath(ctx.CurrentPath, state.FillColor, state.StrokeColor, state.LineWidth); err != nil {
		return err
	}

	ctx.CurrentPath.Clear()
	return nil
}

// OpCloseAndFillAndStroke b - 闭合、填充并描边
type OpCloseAndFillAndStroke struct{}

func (op *OpCloseAndFillAndStroke) Name() string { return "b" }

func (op *OpCloseAndFillAndStroke) Execute(ctx *RenderContext) error {
	ctx.CairoCtx.ClosePath()
	return (&OpFillAndStroke{}).Execute(ctx)
}

// OpEndPath n - 结束路径（不绘制）
type OpEndPath struct{}

func (op *OpEndPath) Name() string { return "n" }

func (op *OpEndPath) Execute(ctx *RenderContext) error {
	ctx.CairoCtx.NewPath()
	ctx.CurrentPath.Clear()
	return nil
}

// OpClip W - 裁剪（非零缠绕规则）
type OpClip struct{}

func (op *OpClip) Name() string { return "W" }

func (op *OpClip) Execute(ctx *RenderContext) error {
	filler := NewPathFiller(ctx.CairoCtx)
	filler.SetFillRule(FillRuleNonZero)
	return filler.ClipPath(ctx.CurrentPath)
}

// OpClipEvenOdd W* - 裁剪（奇偶规则）
type OpClipEvenOdd struct{}

func (op *OpClipEvenOdd) Name() string { return "W*" }

func (op *OpClipEvenOdd) Execute(ctx *RenderContext) error {
	filler := NewPathFiller(ctx.CairoCtx)
	filler.SetFillRule(FillRuleEvenOdd)
	return filler.ClipPath(ctx.CurrentPath)
}

// ===== 颜色操作符 =====

// OpSetStrokeColorRGB RG - 设置描边颜色（RGB）
type OpSetStrokeColorRGB struct {
	R, G, B float64
}

func (op *OpSetStrokeColorRGB) Name() string { return "RG" }

func (op *OpSetStrokeColorRGB) Execute(ctx *RenderContext) error {
	state := ctx.GetCurrentState()
	state.SetStrokeColor(op.R, op.G, op.B, 1.0)
	return nil
}

// OpSetFillColorRGB rg - 设置填充颜色（RGB）
type OpSetFillColorRGB struct {
	R, G, B float64
}

func (op *OpSetFillColorRGB) Name() string { return "rg" }

func (op *OpSetFillColorRGB) Execute(ctx *RenderContext) error {
	state := ctx.GetCurrentState()
	state.SetFillColor(op.R, op.G, op.B, 1.0)
	return nil
}

// OpSetStrokeColorGray G - 设置描边颜色（灰度）
type OpSetStrokeColorGray struct {
	Gray float64
}

func (op *OpSetStrokeColorGray) Name() string { return "G" }

func (op *OpSetStrokeColorGray) Execute(ctx *RenderContext) error {
	state := ctx.GetCurrentState()
	state.SetStrokeColor(op.Gray, op.Gray, op.Gray, 1.0)
	return nil
}

// OpSetFillColorGray g - 设置填充颜色（灰度）
type OpSetFillColorGray struct {
	Gray float64
}

func (op *OpSetFillColorGray) Name() string { return "g" }

func (op *OpSetFillColorGray) Execute(ctx *RenderContext) error {
	state := ctx.GetCurrentState()
	state.SetFillColor(op.Gray, op.Gray, op.Gray, 1.0)
	return nil
}

// OpSetStrokeColorCMYK K - 设置描边颜色（CMYK）
type OpSetStrokeColorCMYK struct {
	C, M, Y, K float64
}

func (op *OpSetStrokeColorCMYK) Name() string { return "K" }

func (op *OpSetStrokeColorCMYK) Execute(ctx *RenderContext) error {
	r, g, b := cmykToRGB(op.C, op.M, op.Y, op.K)
	state := ctx.GetCurrentState()
	state.SetStrokeColor(r, g, b, 1.0)
	return nil
}

// OpSetFillColorCMYK k - 设置填充颜色（CMYK）
type OpSetFillColorCMYK struct {
	C, M, Y, K float64
}

func (op *OpSetFillColorCMYK) Name() string { return "k" }

func (op *OpSetFillColorCMYK) Execute(ctx *RenderContext) error {
	r, g, b := cmykToRGB(op.C, op.M, op.Y, op.K)
	state := ctx.GetCurrentState()
	state.SetFillColor(r, g, b, 1.0)
	return nil
}

// cmykToRGB 将 CMYK 转换为 RGB
func cmykToRGB(c, m, y, k float64) (float64, float64, float64) {
	r := (1 - c) * (1 - k)
	g := (1 - m) * (1 - k)
	b := (1 - y) * (1 - k)
	return r, g, b
}

// OpIgnore - 忽略的操作符（用于标记内容等）
type OpIgnore struct{}

func (op *OpIgnore) Name() string { return "IGNORE" }

func (op *OpIgnore) Execute(ctx *RenderContext) error {
	// 什么都不做
	return nil
}

// ===== Shading 操作符 =====

// OpPaintShading sh - 使用 shading 填充区域
type OpPaintShading struct {
	ShadingName string
}

func (op *OpPaintShading) Name() string { return "sh" }

func (op *OpPaintShading) Execute(ctx *RenderContext) error {
	// 从资源中获取 shading
	shadingObj := ctx.Resources.GetShading(op.ShadingName)
	if shadingObj == nil {
		debugPrintf("Warning: Shading %s not found\n", op.ShadingName)
		return nil
	}

	shading, ok := shadingObj.(*Shading)
	if !ok {
		debugPrintf("Warning: Shading %s is not a valid Shading object\n", op.ShadingName)
		return nil
	}

	// 创建渐变渲染器
	renderer := NewGradientRenderer(ctx.CairoCtx)

	var pattern cairo.Pattern
	var err error

	// 根据 shading 类型渲染
	if shading.IsLinearGradient() {
		pattern, err = renderer.RenderLinearGradient(shading)
	} else if shading.IsRadialGradient() {
		pattern, err = renderer.RenderRadialGradient(shading)
	} else {
		debugPrintf("Warning: Unsupported shading type %d\n", shading.ShadingType)
		return nil
	}

	if err != nil {
		debugPrintf("Warning: Failed to render shading: %v\n", err)
		return nil
	}

	if pattern != nil {
		// 应用渐变填充整个裁剪区域
		ctx.CairoCtx.SetSource(pattern)
		ctx.CairoCtx.Paint()
		pattern.Destroy()
	}

	return nil
}

// ===== Pattern 操作符 =====
// 注意：Pattern 操作符（scn/SCN）的完整实现需要扩展现有的颜色操作符
// 当前版本中，pattern 支持已经通过 Resources 加载，但操作符集成待完善
// 这是一个复杂的功能，需要修改颜色空间处理逻辑

// OpSetFillPattern scn - 设置填充图案（占位符）
type OpSetFillPattern struct {
	PatternName string
	ColorValues []float64
}

func (op *OpSetFillPattern) Name() string { return "scn" }

func (op *OpSetFillPattern) Execute(ctx *RenderContext) error {
	// 从资源中获取 pattern
	patternObj := ctx.Resources.GetPattern(op.PatternName)
	if patternObj == nil {
		debugPrintf("Warning: Pattern %s not found\n", op.PatternName)
		return nil
	}

	pattern, ok := patternObj.(*Pattern)
	if !ok {
		debugPrintf("Warning: Pattern %s is not a valid Pattern object\n", op.PatternName)
		return nil
	}

	// 创建图案渲染器
	renderer := NewPatternRenderer(ctx.CairoCtx)

	// 应用图案填充
	if err := renderer.ApplyPatternFill(pattern); err != nil {
		debugPrintf("Warning: Failed to apply pattern fill: %v\n", err)
		return nil
	}

	return nil
}

// OpSetStrokePattern SCN - 设置描边图案（占位符）
type OpSetStrokePattern struct {
	PatternName string
	ColorValues []float64
}

func (op *OpSetStrokePattern) Name() string { return "SCN" }

func (op *OpSetStrokePattern) Execute(ctx *RenderContext) error {
	// 从资源中获取 pattern
	patternObj := ctx.Resources.GetPattern(op.PatternName)
	if patternObj == nil {
		debugPrintf("Warning: Pattern %s not found\n", op.PatternName)
		return nil
	}

	pattern, ok := patternObj.(*Pattern)
	if !ok {
		debugPrintf("Warning: Pattern %s is not a valid Pattern object\n", op.PatternName)
		return nil
	}

	// 创建图案渲染器
	renderer := NewPatternRenderer(ctx.CairoCtx)

	// 应用图案描边
	if err := renderer.ApplyPatternStroke(pattern); err != nil {
		debugPrintf("Warning: Failed to apply pattern stroke: %v\n", err)
		return nil
	}

	return nil
}
