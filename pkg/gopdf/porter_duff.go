package gopdf

import (
	"image/color"
	"math"
)

// Porter-Duff 混合操作的完整实现
// 支持所有 Gopdf 定义的 30 种混合模式

// PorterDuffBlend 执行 Porter-Duff 混合
func PorterDuffBlend(src, dst color.NRGBA, op Operator) color.NRGBA {
	// 转换为预乘 alpha
	srcR := float64(src.R) * float64(src.A) / 255.0
	srcG := float64(src.G) * float64(src.A) / 255.0
	srcB := float64(src.B) * float64(src.A) / 255.0
	srcA := float64(src.A) / 255.0

	dstR := float64(dst.R) * float64(dst.A) / 255.0
	dstG := float64(dst.G) * float64(dst.A) / 255.0
	dstB := float64(dst.B) * float64(dst.A) / 255.0
	dstA := float64(dst.A) / 255.0

	var outR, outG, outB, outA float64

	switch op {
	case OperatorClear:
		// 清除: 结果完全透明
		outR, outG, outB, outA = 0, 0, 0, 0

	case OperatorSource:
		// 源: 结果是源
		outR, outG, outB, outA = srcR, srcG, srcB, srcA

	case OperatorOver:
		// 源覆盖目标
		outA = srcA + dstA*(1-srcA)
		outR = srcR + dstR*(1-srcA)
		outG = srcG + dstG*(1-srcA)
		outB = srcB + dstB*(1-srcA)

	case OperatorIn:
		// 源在目标内
		outA = srcA * dstA
		outR = srcR * dstA
		outG = srcG * dstA
		outB = srcB * dstA

	case OperatorOut:
		// 源在目标外
		outA = srcA * (1 - dstA)
		outR = srcR * (1 - dstA)
		outG = srcG * (1 - dstA)
		outB = srcB * (1 - dstA)

	case OperatorAtop:
		// 源在目标上方
		outA = srcA*dstA + dstA*(1-srcA)
		outR = srcR*dstA + dstR*(1-srcA)
		outG = srcG*dstA + dstG*(1-srcA)
		outB = srcB*dstA + dstB*(1-srcA)

	case OperatorDest:
		// 目标
		outR, outG, outB, outA = dstR, dstG, dstB, dstA

	case OperatorDestOver:
		// 目标覆盖源
		outA = dstA + srcA*(1-dstA)
		outR = dstR + srcR*(1-dstA)
		outG = dstG + srcG*(1-dstA)
		outB = dstB + srcB*(1-dstA)

	case OperatorDestIn:
		// 目标在源内
		outA = dstA * srcA
		outR = dstR * srcA
		outG = dstG * srcA
		outB = dstB * srcA

	case OperatorDestOut:
		// 目标在源外
		outA = dstA * (1 - srcA)
		outR = dstR * (1 - srcA)
		outG = dstG * (1 - srcA)
		outB = dstB * (1 - srcA)

	case OperatorDestAtop:
		// 目标在源上方
		outA = dstA*srcA + srcA*(1-dstA)
		outR = dstR*srcA + srcR*(1-dstA)
		outG = dstG*srcA + srcG*(1-dstA)
		outB = dstB*srcA + srcB*(1-dstA)

	case OperatorXor:
		// 异或
		outA = srcA*(1-dstA) + dstA*(1-srcA)
		outR = srcR*(1-dstA) + dstR*(1-srcA)
		outG = srcG*(1-dstA) + dstG*(1-srcA)
		outB = srcB*(1-dstA) + dstB*(1-srcA)

	case OperatorAdd:
		// 相加
		outA = math.Min(srcA+dstA, 1.0)
		outR = math.Min(srcR+dstR, outA)
		outG = math.Min(srcG+dstG, outA)
		outB = math.Min(srcB+dstB, outA)

	case OperatorSaturate:
		// 饱和
		outA = math.Min(srcA+dstA, 1.0)
		outR = math.Min(srcR+dstR, outA)
		outG = math.Min(srcG+dstG, outA)
		outB = math.Min(srcB+dstB, outA)

	case OperatorMultiply:
		// 正片叠底
		outA = srcA + dstA*(1-srcA)
		if outA > 0 {
			outR = (srcR*dstR + srcR*(1-dstA) + dstR*(1-srcA))
			outG = (srcG*dstG + srcG*(1-dstA) + dstG*(1-srcA))
			outB = (srcB*dstB + srcB*(1-dstA) + dstB*(1-srcA))
		}

	case OperatorScreen:
		// 滤色
		outA = srcA + dstA*(1-srcA)
		if outA > 0 {
			outR = srcR + dstR - srcR*dstR
			outG = srcG + dstG - srcG*dstG
			outB = srcB + dstB - srcB*dstB
		}

	case OperatorOverlay:
		// 叠加
		outA = srcA + dstA*(1-srcA)
		if outA > 0 {
			outR = blendOverlay(srcR, dstR, srcA, dstA)
			outG = blendOverlay(srcG, dstG, srcA, dstA)
			outB = blendOverlay(srcB, dstB, srcA, dstA)
		}

	case OperatorDarken:
		// 变暗
		outA = srcA + dstA*(1-srcA)
		if outA > 0 {
			outR = math.Min(srcR+dstR*(1-srcA), dstR+srcR*(1-dstA))
			outG = math.Min(srcG+dstG*(1-srcA), dstG+srcG*(1-dstA))
			outB = math.Min(srcB+dstB*(1-srcA), dstB+srcB*(1-dstA))
		}

	case OperatorLighten:
		// 变亮
		outA = srcA + dstA*(1-srcA)
		if outA > 0 {
			outR = math.Max(srcR+dstR*(1-srcA), dstR+srcR*(1-dstA))
			outG = math.Max(srcG+dstG*(1-srcA), dstG+srcG*(1-dstA))
			outB = math.Max(srcB+dstB*(1-srcA), dstB+srcB*(1-dstA))
		}

	case OperatorColorDodge:
		// 颜色减淡
		outA = srcA + dstA*(1-srcA)
		if outA > 0 {
			outR = blendColorDodge(srcR, dstR, srcA, dstA)
			outG = blendColorDodge(srcG, dstG, srcA, dstA)
			outB = blendColorDodge(srcB, dstB, srcA, dstA)
		}

	case OperatorColorBurn:
		// 颜色加深
		outA = srcA + dstA*(1-srcA)
		if outA > 0 {
			outR = blendColorBurn(srcR, dstR, srcA, dstA)
			outG = blendColorBurn(srcG, dstG, srcA, dstA)
			outB = blendColorBurn(srcB, dstB, srcA, dstA)
		}

	case OperatorHardLight:
		// 强光
		outA = srcA + dstA*(1-srcA)
		if outA > 0 {
			outR = blendHardLight(srcR, dstR, srcA, dstA)
			outG = blendHardLight(srcG, dstG, srcA, dstA)
			outB = blendHardLight(srcB, dstB, srcA, dstA)
		}

	case OperatorSoftLight:
		// 柔光
		outA = srcA + dstA*(1-srcA)
		if outA > 0 {
			outR = blendSoftLight(srcR, dstR, srcA, dstA)
			outG = blendSoftLight(srcG, dstG, srcA, dstA)
			outB = blendSoftLight(srcB, dstB, srcA, dstA)
		}

	case OperatorDifference:
		// 差值
		outA = srcA + dstA*(1-srcA)
		if outA > 0 {
			outR = math.Abs(srcR-dstR) + srcR*(1-dstA) + dstR*(1-srcA)
			outG = math.Abs(srcG-dstG) + srcG*(1-dstA) + dstG*(1-srcA)
			outB = math.Abs(srcB-dstB) + srcB*(1-dstA) + dstB*(1-srcA)
		}

	case OperatorExclusion:
		// 排除
		outA = srcA + dstA*(1-srcA)
		if outA > 0 {
			outR = srcR + dstR - 2*srcR*dstR
			outG = srcG + dstG - 2*srcG*dstG
			outB = srcB + dstB - 2*srcB*dstB
		}

	case OperatorHslHue, OperatorHslSaturation, OperatorHslColor, OperatorHslLuminosity:
		// HSL 混合模式
		outA = srcA + dstA*(1-srcA)
		if outA > 0 {
			outR, outG, outB = blendHSL(srcR/srcA, srcG/srcA, srcB/srcA,
				dstR/dstA, dstG/dstA, dstB/dstA, op)
			outR *= outA
			outG *= outA
			outB *= outA
		}

	default:
		// 默认使用 Over 模式
		outA = srcA + dstA*(1-srcA)
		outR = srcR + dstR*(1-srcA)
		outG = srcG + dstG*(1-srcA)
		outB = srcB + dstB*(1-srcA)
	}

	// 转换回非预乘 alpha
	var r, g, b, a uint8
	a = uint8(math.Min(math.Max(outA*255, 0), 255))
	if outA > 0.001 {
		r = uint8(math.Min(math.Max(outR/outA*255, 0), 255))
		g = uint8(math.Min(math.Max(outG/outA*255, 0), 255))
		b = uint8(math.Min(math.Max(outB/outA*255, 0), 255))
	}

	return color.NRGBA{R: r, G: g, B: b, A: a}
}

// 辅助混合函数

func blendOverlay(src, dst, srcA, dstA float64) float64 {
	if dstA == 0 {
		return src
	}
	dstNorm := dst / dstA
	srcNorm := src / srcA

	var result float64
	if dstNorm < 0.5 {
		result = 2 * srcNorm * dstNorm
	} else {
		result = 1 - 2*(1-srcNorm)*(1-dstNorm)
	}
	return result*srcA*dstA + src*(1-dstA) + dst*(1-srcA)
}

func blendColorDodge(src, dst, srcA, dstA float64) float64 {
	if srcA == 0 || dstA == 0 {
		return src + dst
	}
	srcNorm := src / srcA
	dstNorm := dst / dstA

	var result float64
	if srcNorm >= 1.0 {
		result = 1.0
	} else {
		result = math.Min(1.0, dstNorm/(1.0-srcNorm))
	}
	return result*srcA*dstA + src*(1-dstA) + dst*(1-srcA)
}

func blendColorBurn(src, dst, srcA, dstA float64) float64 {
	if srcA == 0 || dstA == 0 {
		return src + dst
	}
	srcNorm := src / srcA
	dstNorm := dst / dstA

	var result float64
	if srcNorm <= 0.0 {
		result = 0.0
	} else {
		result = math.Max(0.0, 1.0-(1.0-dstNorm)/srcNorm)
	}
	return result*srcA*dstA + src*(1-dstA) + dst*(1-srcA)
}

func blendHardLight(src, dst, srcA, dstA float64) float64 {
	if srcA == 0 || dstA == 0 {
		return src + dst
	}
	srcNorm := src / srcA
	dstNorm := dst / dstA

	var result float64
	if srcNorm < 0.5 {
		result = 2 * srcNorm * dstNorm
	} else {
		result = 1 - 2*(1-srcNorm)*(1-dstNorm)
	}
	return result*srcA*dstA + src*(1-dstA) + dst*(1-srcA)
}

func blendSoftLight(src, dst, srcA, dstA float64) float64 {
	if srcA == 0 || dstA == 0 {
		return src + dst
	}
	srcNorm := src / srcA
	dstNorm := dst / dstA

	var result float64
	if srcNorm < 0.5 {
		result = dstNorm - (1-2*srcNorm)*dstNorm*(1-dstNorm)
	} else {
		var d float64
		if dstNorm < 0.25 {
			d = ((16*dstNorm-12)*dstNorm + 4) * dstNorm
		} else {
			d = math.Sqrt(dstNorm)
		}
		result = dstNorm + (2*srcNorm-1)*(d-dstNorm)
	}
	return result*srcA*dstA + src*(1-dstA) + dst*(1-srcA)
}

func blendHSL(srcR, srcG, srcB, dstR, dstG, dstB float64, op Operator) (float64, float64, float64) {
	srcH, srcS, srcL := rgbToHSL(srcR, srcG, srcB)
	dstH, dstS, dstL := rgbToHSL(dstR, dstG, dstB)

	var h, s, l float64
	switch op {
	case OperatorHslHue:
		h, s, l = srcH, dstS, dstL
	case OperatorHslSaturation:
		h, s, l = dstH, srcS, dstL
	case OperatorHslColor:
		h, s, l = srcH, srcS, dstL
	case OperatorHslLuminosity:
		h, s, l = dstH, dstS, srcL
	default:
		h, s, l = srcH, srcS, srcL
	}

	return hslToRGB(h, s, l)
}
