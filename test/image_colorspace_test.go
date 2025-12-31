package test

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// TestImageColorSpaceDebug 测试图像颜色空间的调试
// 用于诊断银灰色变黄色的问题
func TestImageColorSpaceDebug(t *testing.T) {
	// 创建一个测试图像：银灰色 (RGB: 192, 192, 192)
	width, height := 100, 100
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// 填充银灰色
	silverGray := color.RGBA{R: 192, G: 192, B: 192, A: 255}
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			img.Set(x, y, silverGray)
		}
	}

	// 保存测试图像
	f, err := os.Create("test_silver_gray.png")
	if err != nil {
		t.Fatalf("Failed to create test image: %v", err)
	}
	defer f.Close()
	defer os.Remove("test_silver_gray.png")

	if err := png.Encode(f, img); err != nil {
		t.Fatalf("Failed to encode test image: %v", err)
	}
	f.Close()

	t.Logf("✓ Created test image with silver gray (192, 192, 192)")

	// 测试不同颜色空间的转换
	testColorSpaceConversions(t, silverGray)
}

func testColorSpaceConversions(t *testing.T, original color.RGBA) {
	t.Logf("\n=== Testing Color Space Conversions ===")
	t.Logf("Original color: R=%d, G=%d, B=%d", original.R, original.G, original.B)

	// 1. DeviceRGB (应该保持不变)
	t.Logf("\n1. DeviceRGB:")
	rgbCS := &gopdf.DeviceRGBColorSpace{}
	r, g, b, _ := rgbCS.ConvertToRGB([]float64{
		float64(original.R) / 255.0,
		float64(original.G) / 255.0,
		float64(original.B) / 255.0,
	})
	t.Logf("   Result: R=%d, G=%d, B=%d", uint8(r*255), uint8(g*255), uint8(b*255))

	// 2. DeviceGray (应该变成灰度)
	t.Logf("\n2. DeviceGray:")
	grayCS := &gopdf.DeviceGrayColorSpace{}
	gray := (float64(original.R) + float64(original.G) + float64(original.B)) / 3.0 / 255.0
	r, g, b, _ = grayCS.ConvertToRGB([]float64{gray})
	t.Logf("   Result: R=%d, G=%d, B=%d", uint8(r*255), uint8(g*255), uint8(b*255))

	// 3. DeviceCMYK (错误的转换会导致颜色偏移)
	t.Logf("\n3. DeviceCMYK (if misidentified):")
	cmykCS := &gopdf.DeviceCMYKColorSpace{}

	// 如果RGB值被误认为CMYK值，会发生什么？
	// 银灰色 RGB(192, 192, 192) 如果被当作 CMYK(192, 192, 192, 0)
	// 归一化后: C=0.75, M=0.75, Y=0.75, K=0
	// 转换: R=(1-0.75)*(1-0)=0.25, G=0.25, B=0.25
	// 结果: RGB(64, 64, 64) - 深灰色

	// 但如果是 CMYK(0, 0, 192, 0) 被误读
	// C=0, M=0, Y=0.75, K=0
	// 转换: R=1, G=1, B=0.25
	// 结果: RGB(255, 255, 64) - 黄色！

	// 测试场景1: RGB值直接当作CMYK
	r, g, b, _ = cmykCS.ConvertToRGB([]float64{
		float64(original.R) / 255.0,
		float64(original.G) / 255.0,
		float64(original.B) / 255.0,
		0,
	})
	t.Logf("   Scenario 1 (RGB as CMYK): R=%d, G=%d, B=%d", uint8(r*255), uint8(g*255), uint8(b*255))

	// 测试场景2: 只有Y通道有值（这会产生黄色）
	r, g, b, _ = cmykCS.ConvertToRGB([]float64{0, 0, float64(original.R) / 255.0, 0})
	t.Logf("   Scenario 2 (Only Y channel): R=%d, G=%d, B=%d", uint8(r*255), uint8(g*255), uint8(b*255))

	// 4. ICCBased with 3 components (应该像RGB)
	t.Logf("\n4. ICCBased (3 components, RGB):")
	iccCS := &gopdf.ICCBasedColorSpace{
		NumComponents: 3,
		Alternate:     &gopdf.DeviceRGBColorSpace{},
	}
	r, g, b, _ = iccCS.ConvertToRGB([]float64{
		float64(original.R) / 255.0,
		float64(original.G) / 255.0,
		float64(original.B) / 255.0,
	})
	t.Logf("   Result: R=%d, G=%d, B=%d", uint8(r*255), uint8(g*255), uint8(b*255))

	// 5. ICCBased with 4 components (会被当作CMYK)
	t.Logf("\n5. ICCBased (4 components, CMYK):")
	iccCS4 := &gopdf.ICCBasedColorSpace{
		NumComponents: 4,
		Alternate:     &gopdf.DeviceCMYKColorSpace{},
	}
	// 如果RGB数据被误认为4通道
	r, g, b, _ = iccCS4.ConvertToRGB([]float64{
		float64(original.R) / 255.0,
		float64(original.G) / 255.0,
		float64(original.B) / 255.0,
		0,
	})
	t.Logf("   Result: R=%d, G=%d, B=%d", uint8(r*255), uint8(g*255), uint8(b*255))
}

// TestCMYKToRGBConversion 测试CMYK到RGB的转换
func TestCMYKToRGBConversion(t *testing.T) {
	tests := []struct {
		name                            string
		c, m, y, k                      float64
		expectedR, expectedG, expectedB uint8
		description                     string
	}{
		{
			name: "纯白色",
			c:    0, m: 0, y: 0, k: 0,
			expectedR: 255, expectedG: 255, expectedB: 255,
			description: "CMYK(0,0,0,0) should be white",
		},
		{
			name: "纯黑色",
			c:    0, m: 0, y: 0, k: 1,
			expectedR: 0, expectedG: 0, expectedB: 0,
			description: "CMYK(0,0,0,1) should be black",
		},
		{
			name: "纯黄色",
			c:    0, m: 0, y: 1, k: 0,
			expectedR: 255, expectedG: 255, expectedB: 0,
			description: "CMYK(0,0,1,0) should be yellow",
		},
		{
			name: "银灰色 (正确的CMYK值)",
			c:    0, m: 0, y: 0, k: 0.25,
			expectedR: 192, expectedG: 192, expectedB: 192,
			description: "CMYK(0,0,0,0.25) should be silver gray",
		},
		{
			name: "错误场景: RGB(192,192,192)被当作CMYK",
			c:    0.75, m: 0.75, y: 0.75, k: 0,
			expectedR: 64, expectedG: 64, expectedB: 64,
			description: "If RGB(192,192,192) is treated as CMYK, it becomes dark gray",
		},
	}

	cmykCS := &gopdf.DeviceCMYKColorSpace{}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, g, b, _ := cmykCS.ConvertToRGB([]float64{tt.c, tt.m, tt.y, tt.k})
			actualR := uint8(r * 255)
			actualG := uint8(g * 255)
			actualB := uint8(b * 255)

			t.Logf("%s", tt.description)
			t.Logf("  CMYK: (%.2f, %.2f, %.2f, %.2f)", tt.c, tt.m, tt.y, tt.k)
			t.Logf("  Expected RGB: (%d, %d, %d)", tt.expectedR, tt.expectedG, tt.expectedB)
			t.Logf("  Actual RGB:   (%d, %d, %d)", actualR, actualG, actualB)

			// 允许一定的误差
			tolerance := uint8(5)
			if absDiff(actualR, tt.expectedR) > tolerance ||
				absDiff(actualG, tt.expectedG) > tolerance ||
				absDiff(actualB, tt.expectedB) > tolerance {
				t.Errorf("Color mismatch (tolerance=%d)", tolerance)
			}
		})
	}
}

// TestICCBasedColorSpaceDetection 测试ICCBased颜色空间的检测
func TestICCBasedColorSpaceDetection(t *testing.T) {
	t.Log("\n=== Testing ICCBased Color Space Detection ===")

	// 模拟不同的ICCBased场景
	scenarios := []struct {
		name       string
		components int
		dataSize   int
		width      int
		height     int
		expected   string
	}{
		{
			name:       "RGB图像 (3 components)",
			components: 3,
			dataSize:   300, // 10x10 pixels * 3 bytes
			width:      10,
			height:     10,
			expected:   "RGB",
		},
		{
			name:       "CMYK图像 (4 components)",
			components: 4,
			dataSize:   400, // 10x10 pixels * 4 bytes
			width:      10,
			height:     10,
			expected:   "CMYK",
		},
		{
			name:       "灰度图像 (1 component)",
			components: 1,
			dataSize:   100, // 10x10 pixels * 1 byte
			width:      10,
			height:     10,
			expected:   "Gray",
		},
		{
			name:       "错误检测: RGB数据但推断为CMYK",
			components: 0,   // 未设置，需要推断
			dataSize:   400, // 10x10 pixels * 4 bytes (但实际是RGB+alpha)
			width:      10,
			height:     10,
			expected:   "CMYK (错误!)",
		},
	}

	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			// 模拟推断逻辑
			numComponents := sc.components
			if numComponents == 0 && sc.width > 0 && sc.height > 0 {
				numComponents = sc.dataSize / (sc.width * sc.height)
			}

			var detected string
			switch numComponents {
			case 1:
				detected = "Gray"
			case 3:
				detected = "RGB"
			case 4:
				detected = "CMYK"
			default:
				detected = fmt.Sprintf("Unknown (%d components)", numComponents)
			}

			t.Logf("  Data size: %d bytes, Image: %dx%d", sc.dataSize, sc.width, sc.height)
			t.Logf("  Components: %d (explicit) or %d (inferred)", sc.components, numComponents)
			t.Logf("  Detected: %s, Expected: %s", detected, sc.expected)

			if detected == "CMYK" && sc.name == "错误检测: RGB数据但推断为CMYK" {
				t.Logf("  ⚠️  WARNING: This is the bug! RGB+Alpha data is being detected as CMYK!")
				t.Logf("  ⚠️  This will cause silver gray to turn yellow!")
			}
		})
	}
}
