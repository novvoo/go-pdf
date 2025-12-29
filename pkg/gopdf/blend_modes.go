package gopdf

// gopdfBlendModes PDF 混合模式到 Gopdf 操作符的映射
var gopdfBlendModes = map[string]Operator{
	"Normal":     OperatorOver,
	"Multiply":   OperatorMultiply,
	"Screen":     OperatorScreen,
	"Overlay":    OperatorOverlay,
	"Darken":     OperatorDarken,
	"Lighten":    OperatorLighten,
	"ColorDodge": OperatorColorDodge,
	"ColorBurn":  OperatorColorBurn,
	"HardLight":  OperatorHardLight,
	"SoftLight":  OperatorSoftLight,
	"Difference": OperatorDifference,
	"Exclusion":  OperatorExclusion,
}

// GetGopdfBlendMode 获取 Gopdf 混合模式操作符
// 如果不支持，返回 Normal 模式并记录警告
func GetGopdfBlendMode(pdfBlendMode string) Operator {
	if op, ok := gopdfBlendModes[pdfBlendMode]; ok {
		return op
	}

	// 不支持的混合模式，回退到 Normal
	debugPrintf("Warning: Unsupported blend mode '%s', falling back to Normal\n", pdfBlendMode)
	return OperatorOver
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

// ApplyBlendMode 将混合模式应用到 Gopdf context
func (gs *GraphicsState) ApplyBlendMode(ctx Context) {
	operator := GetGopdfBlendMode(gs.BlendMode)
	ctx.SetOperator(operator)
}
