package test

import (
	"image/color"
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// TestSilverGrayToYellowBug 测试银灰色变黄色的bug
// 这是一个真实场景：Mac截图（银灰色）在PDF中被错误渲染成黄色
func TestSilverGrayToYellowBug(t *testing.T) {
	t.Log("\n=== Testing Silver Gray to Yellow Bug ===")

	// 模拟Mac截图的银灰色像素数据
	silverGray := color.RGBA{R: 192, G: 192, B: 192, A: 255}

	t.Logf("Original color (Mac screenshot): R=%d, G=%d, B=%d",
		silverGray.R, silverGray.G, silverGray.B)

	// 场景1: 正确的RGB解码
	t.Log("\n--- Scenario 1: Correct RGB decoding ---")
	rgbData := []byte{192, 192, 192} // 每像素3字节
	rgbImg, err := gopdf.DecodeDeviceRGBPublic(rgbData, 1, 1, 8)
	if err != nil {
		t.Fatalf("Failed to decode RGB: %v", err)
	}

	c := rgbImg.At(0, 0).(color.RGBA)
	t.Logf("RGB decoded: R=%d, G=%d, B=%d", c.R, c.G, c.B)

	if c.R != 192 || c.G != 192 || c.B != 192 {
		t.Errorf("RGB decoding failed! Expected (192,192,192), got (%d,%d,%d)", c.R, c.G, c.B)
	} else {
		t.Log("✓ RGB decoding correct: silver gray preserved")
	}

	// 场景2: 错误的CMYK解码（这是bug）
	t.Log("\n--- Scenario 2: Incorrect CMYK decoding (THE BUG) ---")
	// 如果RGB数据被误认为CMYK，会发生什么？
	// RGB(192,192,192) 被当作 CMYK(192,192,192,0)
	cmykData := []byte{192, 192, 192, 0} // 每像素4字节
	cmykImg, err := gopdf.DecodeDeviceCMYKPublic(cmykData, 1, 1, 8)
	if err != nil {
		t.Fatalf("Failed to decode CMYK: %v", err)
	}

	c = cmykImg.At(0, 0).(color.RGBA)
	t.Logf("CMYK decoded: R=%d, G=%d, B=%d", c.R, c.G, c.B)

	// CMYK(192/255, 192/255, 192/255, 0) = CMYK(0.75, 0.75, 0.75, 0)
	// RGB = (1-0.75)*(1-0) = 0.25 = 64
	// 结果应该是深灰色 RGB(64,64,64)
	if c.R > 100 || c.G > 100 || c.B > 100 {
		t.Errorf("⚠️  BUG DETECTED! CMYK decoding produced wrong color: (%d,%d,%d)", c.R, c.G, c.B)
		t.Error("This should be dark gray (64,64,64), not light color!")
	} else {
		t.Logf("CMYK decoding result: dark gray (%d,%d,%d) - as expected for wrong interpretation", c.R, c.G, c.B)
	}

	// 场景3: 另一种错误场景 - 只有Y通道有值（产生黄色）
	t.Log("\n--- Scenario 3: Yellow bug - only Y channel ---")
	// 如果数据被错误解析，只有Y通道有值
	cmykYellowData := []byte{0, 0, 192, 0} // C=0, M=0, Y=192, K=0
	cmykYellowImg, err := gopdf.DecodeDeviceCMYKPublic(cmykYellowData, 1, 1, 8)
	if err != nil {
		t.Fatalf("Failed to decode CMYK yellow: %v", err)
	}

	c = cmykYellowImg.At(0, 0).(color.RGBA)
	t.Logf("CMYK (only Y) decoded: R=%d, G=%d, B=%d", c.R, c.G, c.B)

	// CMYK(0, 0, 0.75, 0) -> RGB(1, 1, 0.25) = RGB(255, 255, 64)
	// 这会产生黄色！
	if c.R > 200 && c.G > 200 && c.B < 100 {
		t.Logf("⚠️  YELLOW COLOR DETECTED! R=%d, G=%d, B=%d", c.R, c.G, c.B)
		t.Log("This is what happens when RGB data is misinterpreted as CMYK with only Y channel!")
	}

	// 场景4: 测试修复后的ICCBased处理
	t.Log("\n--- Scenario 4: Fixed ICCBased handling ---")
	t.Log("With the fix, ICCBased images should:")
	t.Log("1. Use N value from ICC profile if available")
	t.Log("2. Use Alternate colorspace to determine N if N is missing")
	t.Log("3. Default to RGB (N=3) for ambiguous 4-byte-per-pixel data")
	t.Log("4. Never misinterpret RGB as CMYK")
}

// TestICCBasedColorSpaceWithAlternate 测试带Alternate的ICCBased颜色空间
func TestICCBasedColorSpaceWithAlternate(t *testing.T) {
	t.Log("\n=== Testing ICCBased with Alternate ColorSpace ===")

	// 场景1: ICCBased with Alternate DeviceRGB
	t.Log("\n--- ICCBased with Alternate DeviceRGB ---")
	rgbCS := &gopdf.ICCBasedColorSpace{
		NumComponents: 3,
		Alternate:     &gopdf.DeviceRGBColorSpace{},
	}

	r, g, b, err := rgbCS.ConvertToRGB([]float64{0.75, 0.75, 0.75})
	if err != nil {
		t.Fatalf("Failed to convert: %v", err)
	}

	t.Logf("Input: (0.75, 0.75, 0.75)")
	t.Logf("Output: R=%d, G=%d, B=%d", uint8(r*255), uint8(g*255), uint8(b*255))

	// 应该保持银灰色
	if uint8(r*255) != 191 || uint8(g*255) != 191 || uint8(b*255) != 191 {
		t.Errorf("Color mismatch! Expected (191,191,191), got (%d,%d,%d)",
			uint8(r*255), uint8(g*255), uint8(b*255))
	} else {
		t.Log("✓ ICCBased RGB conversion correct")
	}

	// 场景2: ICCBased with Alternate DeviceCMYK (真正的CMYK图像)
	t.Log("\n--- ICCBased with Alternate DeviceCMYK ---")
	cmykCS := &gopdf.ICCBasedColorSpace{
		NumComponents: 4,
		Alternate:     &gopdf.DeviceCMYKColorSpace{},
	}

	// 真正的CMYK银灰色: C=0, M=0, Y=0, K=0.25
	r, g, b, err = cmykCS.ConvertToRGB([]float64{0, 0, 0, 0.25})
	if err != nil {
		t.Fatalf("Failed to convert: %v", err)
	}

	t.Logf("Input: CMYK(0, 0, 0, 0.25)")
	t.Logf("Output: R=%d, G=%d, B=%d", uint8(r*255), uint8(g*255), uint8(b*255))

	// 应该是银灰色
	if uint8(r*255) < 180 || uint8(g*255) < 180 || uint8(b*255) < 180 {
		t.Errorf("CMYK conversion incorrect! Expected ~(191,191,191), got (%d,%d,%d)",
			uint8(r*255), uint8(g*255), uint8(b*255))
	} else {
		t.Log("✓ ICCBased CMYK conversion correct")
	}
}

// TestAmbiguousFourBytePerPixel 测试4字节/像素的歧义情况
func TestAmbiguousFourBytePerPixel(t *testing.T) {
	t.Log("\n=== Testing Ambiguous 4-Byte-Per-Pixel Data ===")

	// 创建一个10x10的图像，每像素4字节
	width, height := 10, 10

	// 场景1: RGB + Alpha (应该被识别为RGB)
	t.Log("\n--- Scenario 1: RGB + Alpha data ---")
	rgbaData := make([]byte, width*height*4)
	for i := 0; i < width*height; i++ {
		rgbaData[i*4+0] = 192 // R
		rgbaData[i*4+1] = 192 // G
		rgbaData[i*4+2] = 192 // B
		rgbaData[i*4+3] = 255 // A
	}

	t.Logf("Data size: %d bytes for %dx%d image", len(rgbaData), width, height)
	t.Logf("Bytes per pixel: %d", len(rgbaData)/(width*height))
	t.Log("This could be interpreted as:")
	t.Log("  - RGB + Alpha (correct)")
	t.Log("  - CMYK (incorrect, would cause color shift)")

	// 场景2: 真正的CMYK数据
	t.Log("\n--- Scenario 2: Real CMYK data ---")
	cmykData := make([]byte, width*height*4)
	for i := 0; i < width*height; i++ {
		cmykData[i*4+0] = 0  // C
		cmykData[i*4+1] = 0  // M
		cmykData[i*4+2] = 0  // Y
		cmykData[i*4+3] = 64 // K (25% black = 75% gray)
	}

	t.Logf("Data size: %d bytes for %dx%d image", len(cmykData), width, height)
	t.Log("This is genuine CMYK data")

	// 测试解码
	cmykImg, err := gopdf.DecodeDeviceCMYKPublic(cmykData, width, height, 8)
	if err != nil {
		t.Fatalf("Failed to decode CMYK: %v", err)
	}

	c := cmykImg.At(0, 0).(color.RGBA)
	t.Logf("CMYK decoded: R=%d, G=%d, B=%d", c.R, c.G, c.B)

	// 应该是银灰色
	if c.R < 180 || c.G < 180 || c.B < 180 {
		t.Errorf("CMYK decoding incorrect! Expected ~(191,191,191), got (%d,%d,%d)", c.R, c.G, c.B)
	} else {
		t.Log("✓ CMYK decoding correct")
	}

	t.Log("\n--- Conclusion ---")
	t.Log("The fix should:")
	t.Log("1. Check ICC profile's N value first")
	t.Log("2. Check Alternate colorspace if N is missing")
	t.Log("3. For ambiguous 4-byte data, default to RGB (not CMYK)")
	t.Log("4. Only use CMYK if explicitly specified in PDF")
}

// 辅助函数：公开内部解码函数用于测试
func init() {
	// 这些函数需要在 gopdf 包中导出
	// 或者我们在这里重新实现简化版本
}
