package gopdf

import (
	"fmt"

	"github.com/novvoo/go-cairo/pkg/cairo"
)

// GradientRenderer 渐变渲染器
type GradientRenderer struct {
	ctx cairo.Context
}

// NewGradientRenderer 创建新的渐变渲染器
func NewGradientRenderer(ctx cairo.Context) *GradientRenderer {
	return &GradientRenderer{
		ctx: ctx,
	}
}

// RenderLinearGradient 渲染线性渐变
func (gr *GradientRenderer) RenderLinearGradient(shading *Shading) (cairo.Pattern, error) {
	if !shading.IsLinearGradient() {
		return nil, fmt.Errorf("not a linear gradient (ShadingType=%d)", shading.ShadingType)
	}

	// 获取线性渐变坐标
	x0, y0, x1, y1 := shading.GetLinearCoords()

	// 创建 Cairo 线性渐变
	pattern := cairo.NewPatternLinear(x0, y0, x1, y1)

	// 设置扩展模式
	if len(shading.Extend) >= 2 {
		if shading.Extend[0] && shading.Extend[1] {
			pattern.SetExtend(cairo.ExtendPad)
		} else if shading.Extend[0] || shading.Extend[1] {
			pattern.SetExtend(cairo.ExtendPad)
		} else {
			pattern.SetExtend(cairo.ExtendNone)
		}
	}

	// 添加颜色停止点
	if err := gr.addColorStops(pattern, shading); err != nil {
		pattern.Destroy()
		return nil, err
	}

	debugPrintf("✓ Created linear gradient: (%.2f,%.2f) -> (%.2f,%.2f)\n", x0, y0, x1, y1)
	return pattern, nil
}

// RenderRadialGradient 渲染径向渐变
func (gr *GradientRenderer) RenderRadialGradient(shading *Shading) (cairo.Pattern, error) {
	if !shading.IsRadialGradient() {
		return nil, fmt.Errorf("not a radial gradient (ShadingType=%d)", shading.ShadingType)
	}

	// 获取径向渐变坐标
	x0, y0, r0, x1, y1, r1 := shading.GetRadialCoords()

	// 创建 Cairo 径向渐变
	pattern := cairo.NewPatternRadial(x0, y0, r0, x1, y1, r1)

	// 设置扩展模式
	if len(shading.Extend) >= 2 {
		if shading.Extend[0] && shading.Extend[1] {
			pattern.SetExtend(cairo.ExtendPad)
		} else if shading.Extend[0] || shading.Extend[1] {
			pattern.SetExtend(cairo.ExtendPad)
		} else {
			pattern.SetExtend(cairo.ExtendNone)
		}
	}

	// 添加颜色停止点
	if err := gr.addColorStops(pattern, shading); err != nil {
		pattern.Destroy()
		return nil, err
	}

	debugPrintf("✓ Created radial gradient: (%.2f,%.2f,%.2f) -> (%.2f,%.2f,%.2f)\n",
		x0, y0, r0, x1, y1, r1)
	return pattern, nil
}

// addColorStops 添加颜色停止点到渐变
func (gr *GradientRenderer) addColorStops(pattern cairo.Pattern, shading *Shading) error {
	if shading.Function == nil {
		// 没有函数，使用默认黑到白渐变
		if gradPattern, ok := pattern.(cairo.GradientPattern); ok {
			gradPattern.AddColorStopRGBA(0, 0, 0, 0, 1)
			gradPattern.AddColorStopRGBA(1, 1, 1, 1, 1)
		}
		return nil
	}

	// 使用函数生成颜色停止点
	// 在 [0, 1] 范围内采样多个点
	numStops := 10 // 可以根据需要调整
	for i := 0; i <= numStops; i++ {
		t := float64(i) / float64(numStops)
		colors := shading.Function.EvaluateFunction(t)

		// 转换颜色到 RGB
		r, g, b, a := gr.convertColorToRGBA(colors, shading.ColorSpace)
		if gradPattern, ok := pattern.(cairo.GradientPattern); ok {
			gradPattern.AddColorStopRGBA(t, r, g, b, a)
		}
	}

	return nil
}

// convertColorToRGBA 将颜色转换为 RGBA
func (gr *GradientRenderer) convertColorToRGBA(colors []float64, colorSpace string) (float64, float64, float64, float64) {
	// 默认 alpha 为 1.0
	alpha := 1.0

	switch colorSpace {
	case "/DeviceRGB", "DeviceRGB":
		if len(colors) >= 3 {
			return colors[0], colors[1], colors[2], alpha
		}
		return 0, 0, 0, alpha

	case "/DeviceGray", "DeviceGray":
		if len(colors) >= 1 {
			gray := colors[0]
			return gray, gray, gray, alpha
		}
		return 0, 0, 0, alpha

	case "/DeviceCMYK", "DeviceCMYK":
		if len(colors) >= 4 {
			c, m, y, k := colors[0], colors[1], colors[2], colors[3]
			r, g, b := cmykToRGB(c, m, y, k)
			return r, g, b, alpha
		}
		return 0, 0, 0, alpha

	default:
		// 未知颜色空间，假设为 RGB
		debugPrintf("Warning: Unknown color space '%s', assuming RGB\n", colorSpace)
		if len(colors) >= 3 {
			return colors[0], colors[1], colors[2], alpha
		}
		return 0, 0, 0, alpha
	}
}

// EvaluateFunction 计算函数在 t 处的颜色值（导出供外部使用）
func (gr *GradientRenderer) EvaluateFunction(fn *ShadingFunction, t float64) []float64 {
	if fn == nil {
		return []float64{0, 0, 0}
	}
	return fn.EvaluateFunction(t)
}
