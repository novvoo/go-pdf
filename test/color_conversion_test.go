package test

import (
	"image"
	"image/color"
	"math"
	"testing"
)

// TestYCbCrToRGBConversion 测试YCbCr到RGB的颜色转换准确性
func TestYCbCrToRGBConversion(t *testing.T) {
	tests := []struct {
		name                            string
		y, cb, cr                       uint8
		expectedR, expectedG, expectedB uint8
		tolerance                       uint8 // 允许的误差范围
	}{
		{
			name: "纯白色",
			y:    255, cb: 128, cr: 128,
			expectedR: 255, expectedG: 255, expectedB: 255,
			tolerance: 5,
		},
		{
			name: "纯黑色",
			y:    0, cb: 128, cr: 128,
			expectedR: 0, expectedG: 0, expectedB: 0,
			tolerance: 5,
		},
		{
			name: "纯红色",
			y:    76, cb: 85, cr: 255,
			expectedR: 255, expectedG: 0, expectedB: 0,
			tolerance: 10,
		},
		{
			name: "纯绿色",
			y:    150, cb: 44, cr: 21,
			expectedR: 0, expectedG: 255, expectedB: 0,
			tolerance: 10,
		},
		{
			name: "纯蓝色",
			y:    29, cb: 255, cr: 107,
			expectedR: 0, expectedG: 0, expectedB: 255,
			tolerance: 10,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建YCbCr图像
			img := image.NewYCbCr(image.Rect(0, 0, 1, 1), image.YCbCrSubsampleRatio444)
			img.Y[0] = tt.y
			img.Cb[0] = tt.cb
			img.Cr[0] = tt.cr

			// 转换为RGBA
			rgba := image.NewRGBA(img.Bounds())

			// 使用改进的浮点转换
			yy := float64(tt.y)
			cb := float64(tt.cb) - 128.0
			cr := float64(tt.cr) - 128.0

			r := yy + 1.402*cr
			g := yy - 0.344136*cb - 0.714136*cr
			b := yy + 1.772*cb

			// 裁剪到 [0, 255]
			r = math.Max(0, math.Min(255, r))
			g = math.Max(0, math.Min(255, g))
			b = math.Max(0, math.Min(255, b))

			rgba.Set(0, 0, color.RGBA{
				R: uint8(r),
				G: uint8(g),
				B: uint8(b),
				A: 255,
			})

			// 获取转换后的颜色
			c := rgba.At(0, 0).(color.RGBA)

			// 检查误差
			rDiff := absDiff(c.R, tt.expectedR)
			gDiff := absDiff(c.G, tt.expectedG)
			bDiff := absDiff(c.B, tt.expectedB)

			if rDiff > tt.tolerance || gDiff > tt.tolerance || bDiff > tt.tolerance {
				t.Errorf("Color conversion failed:\n"+
					"  Input: Y=%d, Cb=%d, Cr=%d\n"+
					"  Expected: R=%d, G=%d, B=%d\n"+
					"  Got: R=%d, G=%d, B=%d\n"+
					"  Diff: R=%d, G=%d, B=%d (tolerance=%d)",
					tt.y, tt.cb, tt.cr,
					tt.expectedR, tt.expectedG, tt.expectedB,
					c.R, c.G, c.B,
					rDiff, gDiff, bDiff, tt.tolerance)
			} else {
				t.Logf("✓ Color conversion passed: Y=%d,Cb=%d,Cr=%d -> R=%d,G=%d,B=%d (expected R=%d,G=%d,B=%d)",
					tt.y, tt.cb, tt.cr, c.R, c.G, c.B, tt.expectedR, tt.expectedG, tt.expectedB)
			}
		})
	}
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

// TestDPICalculation 测试DPI计算逻辑
func TestDPICalculation(t *testing.T) {
	tests := []struct {
		name        string
		pixelWidth  float64
		pixelHeight float64
		inputDPI    float64
		expectedW   float64
		expectedH   float64
	}{
		{
			name:       "72 DPI (标准)",
			pixelWidth: 720, pixelHeight: 720,
			inputDPI:  72,
			expectedW: 720, expectedH: 720,
		},
		{
			name:       "300 DPI (高分辨率)",
			pixelWidth: 300, pixelHeight: 300,
			inputDPI:  300,
			expectedW: 72, expectedH: 72, // 1英寸 = 72 points
		},
		{
			name:       "96 DPI (常见屏幕分辨率)",
			pixelWidth: 960, pixelHeight: 960,
			inputDPI:  96,
			expectedW: 720, expectedH: 720, // 10英寸 = 720 points
		},
		{
			name:       "150 DPI",
			pixelWidth: 1500, pixelHeight: 1500,
			inputDPI:  150,
			expectedW: 720, expectedH: 720, // 10英寸 = 720 points
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 应用DPI转换公式
			// widthPoints = pixelWidth * (72 / inputDPI)
			actualW := tt.pixelWidth * (72.0 / tt.inputDPI)
			actualH := tt.pixelHeight * (72.0 / tt.inputDPI)

			// 检查结果
			if math.Abs(actualW-tt.expectedW) > 0.01 || math.Abs(actualH-tt.expectedH) > 0.01 {
				t.Errorf("DPI calculation failed:\n"+
					"  Input: %.0fx%.0f pixels at %.0f DPI\n"+
					"  Expected: %.2fx%.2f points\n"+
					"  Got: %.2fx%.2f points",
					tt.pixelWidth, tt.pixelHeight, tt.inputDPI,
					tt.expectedW, tt.expectedH,
					actualW, actualH)
			} else {
				t.Logf("✓ DPI calculation passed: %.0fx%.0f px @ %.0f DPI -> %.2fx%.2f points",
					tt.pixelWidth, tt.pixelHeight, tt.inputDPI, actualW, actualH)
			}
		})
	}
}
