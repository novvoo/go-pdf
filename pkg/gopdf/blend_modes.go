package gopdf

import "github.com/novvoo/go-cairo/pkg/cairo"

// cairoBlendModes PDF 混合模式到 Cairo 操作符的映射
var cairoBlendModes = map[string]cairo.Operator{
	"Normal":     cairo.OperatorOver,
	"Multiply":   cairo.OperatorMultiply,
	"Screen":     cairo.OperatorScreen,
	"Overlay":    cairo.OperatorOverlay,
	"Darken":     cairo.OperatorDarken,
	"Lighten":    cairo.OperatorLighten,
	"ColorDodge": cairo.OperatorColorDodge,
	"ColorBurn":  cairo.OperatorColorBurn,
	"HardLight":  cairo.OperatorHardLight,
	"SoftLight":  cairo.OperatorSoftLight,
	"Difference": cairo.OperatorDifference,
	"Exclusion":  cairo.OperatorExclusion,
}

// GetCairoBlendMode 获取 Cairo 混合模式操作符
// 如果不支持，返回 Normal 模式并记录警告
func GetCairoBlendMode(pdfBlendMode string) cairo.Operator {
	if op, ok := cairoBlendModes[pdfBlendMode]; ok {
		return op
	}

	// 不支持的混合模式，回退到 Normal
	debugPrintf("Warning: Unsupported blend mode '%s', falling back to Normal\n", pdfBlendMode)
	return cairo.OperatorOver
}

// SetBlendMode 设置图形状态的混合模式
func (gs *GraphicsState) SetBlendMode(blendMode string) {
	gs.BlendMode = blendMode
}

// SetFillAlpha 设置填充透明度
func (gs *GraphicsState) SetFillAlpha(alpha float64) {
	gs.FillAlpha = alpha
	if gs.FillColor != nil {
		gs.FillColor.A = alpha
	}
}

// SetStrokeAlpha 设置描边透明度
func (gs *GraphicsState) SetStrokeAlpha(alpha float64) {
	gs.StrokeAlpha = alpha
	if gs.StrokeColor != nil {
		gs.StrokeColor.A = alpha
	}
}

// ApplyBlendMode 将混合模式应用到 Cairo context
func (gs *GraphicsState) ApplyBlendMode(ctx cairo.Context) {
	operator := GetCairoBlendMode(gs.BlendMode)
	ctx.SetOperator(operator)
}
