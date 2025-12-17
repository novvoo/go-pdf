package gopdf

import (
	"image/color"

	"github.com/novvoo/go-cairo/pkg/cairo"
)

// AlphaBlender Alpha 混合器
// 提供各种 Porter-Duff 混合模式和 PDF 混合模式
type AlphaBlender struct {
	operator cairo.Operator
}

// NewAlphaBlender 创建新的 Alpha 混合器
func NewAlphaBlender(op cairo.Operator) *AlphaBlender {
	return &AlphaBlender{
		operator: op,
	}
}

// SetOperator 设置混合操作符
func (ab *AlphaBlender) SetOperator(op cairo.Operator) {
	ab.operator = op
}

// GetOperator 获取混合操作符
func (ab *AlphaBlender) GetOperator() cairo.Operator {
	return ab.operator
}

// Blend 混合两个颜色
func (ab *AlphaBlender) Blend(src, dst color.Color) color.Color {
	// 转换为 NRGBA
	sr, sg, sb, sa := src.RGBA()
	dr, dg, db, da := dst.RGBA()

	srcNRGBA := color.NRGBA{
		R: uint8(sr >> 8),
		G: uint8(sg >> 8),
		B: uint8(sb >> 8),
		A: uint8(sa >> 8),
	}

	dstNRGBA := color.NRGBA{
		R: uint8(dr >> 8),
		G: uint8(dg >> 8),
		B: uint8(db >> 8),
		A: uint8(da >> 8),
	}

	// 使用 Cairo 的 Porter-Duff 混合
	return cairo.PorterDuffBlend(srcNRGBA, dstNRGBA, ab.operator)
}

// BlendWithAlpha 混合两个颜色，并指定额外的 alpha
func (ab *AlphaBlender) BlendWithAlpha(src, dst color.Color, alpha float64) color.Color {
	// 调整源颜色的 alpha
	sr, sg, sb, sa := src.RGBA()
	
	finalAlpha := uint8(float64(sa>>8) * alpha)
	
	srcNRGBA := color.NRGBA{
		R: uint8(sr >> 8),
		G: uint8(sg >> 8),
		B: uint8(sb >> 8),
		A: finalAlpha,
	}

	dr, dg, db, da := dst.RGBA()
	dstNRGBA := color.NRGBA{
		R: uint8(dr >> 8),
		G: uint8(dg >> 8),
		B: uint8(db >> 8),
		A: uint8(da >> 8),
	}

	return cairo.PorterDuffBlend(srcNRGBA, dstNRGBA, ab.operator)
}

// BlendLayers 混合多个图层
func (ab *AlphaBlender) BlendLayers(layers []color.Color) color.Color {
	if len(layers) == 0 {
		return color.Transparent
	}

	result := layers[0]
	for i := 1; i < len(layers); i++ {
		result = ab.Blend(layers[i], result)
	}

	return result
}

// ===== PDF 混合模式辅助函数 =====

// GetPDFBlendOperator 根据 PDF 混合模式名称获取 Cairo 操作符
func GetPDFBlendOperator(blendMode string) cairo.Operator {
	return GetCairoBlendMode(blendMode)
}

// BlendWithPDFMode 使用 PDF 混合模式混合颜色
func BlendWithPDFMode(src, dst color.Color, blendMode string) color.Color {
	op := GetPDFBlendOperator(blendMode)
	blender := NewAlphaBlender(op)
	return blender.Blend(src, dst)
}

// ===== 预乘 Alpha 处理 =====

// PremultiplyColor 预乘颜色的 alpha
func PremultiplyColor(c color.Color) color.NRGBA {
	r, g, b, a := c.RGBA()
	
	if a == 0 {
		return color.NRGBA{0, 0, 0, 0}
	}
	
	if a == 0xffff {
		return color.NRGBA{
			R: uint8(r >> 8),
			G: uint8(g >> 8),
			B: uint8(b >> 8),
			A: 255,
		}
	}
	
	// 预乘公式: color_premul = color * alpha / 255
	alpha := uint32(a >> 8)
	return color.NRGBA{
		R: uint8((uint32(r>>8) * alpha) / 255),
		G: uint8((uint32(g>>8) * alpha) / 255),
		B: uint8((uint32(b>>8) * alpha) / 255),
		A: uint8(alpha),
	}
}

// UnpremultiplyColor 反预乘颜色的 alpha
func UnpremultiplyColor(c color.NRGBA) color.NRGBA {
	if c.A == 0 {
		return color.NRGBA{0, 0, 0, 0}
	}
	
	if c.A == 255 {
		return c
	}
	
	// 反预乘公式: color = color_premul * 255 / alpha
	alpha := uint32(c.A)
	return color.NRGBA{
		R: uint8((uint32(c.R) * 255) / alpha),
		G: uint8((uint32(c.G) * 255) / alpha),
		B: uint8((uint32(c.B) * 255) / alpha),
		A: c.A,
	}
}

// ===== 颜色空间混合 =====

// BlendInColorSpace 在指定颜色空间中混合颜色
func BlendInColorSpace(src, dst color.Color, cs ColorSpace, op cairo.Operator) (color.Color, error) {
	// 将颜色转换到指定颜色空间
	// 这里简化处理，直接在 RGB 空间混合
	// 实际应该先转换到目标颜色空间，混合后再转回 RGB
	
	blender := NewAlphaBlender(op)
	return blender.Blend(src, dst), nil
}

// ===== 透明度组混合 =====

// BlendTransparencyGroup 混合透明度组
type TransparencyGroupBlender struct {
	isolated bool
	knockout bool
	blender  *AlphaBlender
}

// NewTransparencyGroupBlender 创建透明度组混合器
func NewTransparencyGroupBlender(isolated, knockout bool, op cairo.Operator) *TransparencyGroupBlender {
	return &TransparencyGroupBlender{
		isolated: isolated,
		knockout: knockout,
		blender:  NewAlphaBlender(op),
	}
}

// BlendGroup 混合透明度组到背景
func (tgb *TransparencyGroupBlender) BlendGroup(group, background color.Color) color.Color {
	if tgb.isolated {
		// 隔离组：不使用背景色
		return tgb.blender.Blend(group, color.Transparent)
	}
	
	// 非隔离组：与背景混合
	return tgb.blender.Blend(group, background)
}

// BlendWithKnockout 使用敲除模式混合
func (tgb *TransparencyGroupBlender) BlendWithKnockout(layers []color.Color) color.Color {
	if len(layers) == 0 {
		return color.Transparent
	}
	
	if !tgb.knockout {
		// 非敲除模式：正常混合所有图层
		return tgb.blender.BlendLayers(layers)
	}
	
	// 敲除模式：每个图层独立混合到背景，不相互混合
	// 这里简化处理，返回最上层
	return layers[len(layers)-1]
}

// ===== 软遮罩混合 =====

// ApplySoftMaskToColor 应用软遮罩到颜色
func ApplySoftMaskToColor(c color.Color, maskAlpha uint8) color.Color {
	r, g, b, a := c.RGBA()
	
	// 将遮罩 alpha 应用到颜色
	finalAlpha := uint8((uint32(a>>8) * uint32(maskAlpha)) / 255)
	
	return color.NRGBA{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
		A: finalAlpha,
	}
}

// ===== 混合模式工具函数 =====

// IsBlendModeSupported 检查混合模式是否支持
func IsBlendModeSupported(blendMode string) bool {
	_, ok := cairoBlendModes[blendMode]
	return ok
}

// GetSupportedBlendModes 获取所有支持的混合模式
func GetSupportedBlendModes() []string {
	modes := make([]string, 0, len(cairoBlendModes))
	for mode := range cairoBlendModes {
		modes = append(modes, mode)
	}
	return modes
}

// BlendModeToString 将 Cairo 操作符转换为 PDF 混合模式名称
func BlendModeToString(op cairo.Operator) string {
	for mode, operator := range cairoBlendModes {
		if operator == op {
			return mode
		}
	}
	return "Normal"
}
