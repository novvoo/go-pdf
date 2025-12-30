package test

import (
	"fmt"
	"image"
	"image/png"
	"os"
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// TestTextRendering 测试文本渲染的基本功能
func TestTextRendering(t *testing.T) {
	// 创建渲染器
	renderer := gopdf.NewPDFRenderer(800, 600)
	renderer.SetDPI(150)

	// 测试渲染包含中英文的文本
	outputPath := "text_rendering_test.png"
	err := renderer.RenderToPNG(outputPath, func(ctx gopdf.Context) {
		// 设置白色背景
		ctx.SetSourceRGB(1, 1, 1)
		ctx.Paint()

		// 绘制英文文本
		ctx.SetSourceRGB(0, 0, 0)
		layout := ctx.PangoPdfCreateLayout().(*gopdf.PangoPdfLayout)

		// 测试英文文本渲染
		fontDesc := gopdf.NewPangoFontDescription()
		fontDesc.SetFamily("sans-serif")
		fontDesc.SetSize(24)
		layout.SetFontDescription(fontDesc)
		layout.SetText("Hello World! This is English text.")

		ctx.MoveTo(50, 50)
		ctx.PangoPdfShowText(layout)

		// 测试中文文本渲染
		fontDesc.SetSize(20)
		layout.SetFontDescription(fontDesc)
		layout.SetText("你好世界！这是中文文本。")

		ctx.MoveTo(50, 100)
		ctx.PangoPdfShowText(layout)

		// 测试混合文本渲染
		layout.SetText("Mixed text: 你好World!")

		ctx.MoveTo(50, 150)
		ctx.PangoPdfShowText(layout)
	})

	if err != nil {
		t.Fatalf("Failed to render text to PNG: %v", err)
	}

	// 检查输出文件是否存在
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("Output PNG file was not created")
	}

	// 验证生成的图片
	img, err := loadAndValidateImage(outputPath)
	if err != nil {
		t.Errorf("Failed to validate image: %v", err)
	} else {
		// 检查图片尺寸
		bounds := img.Bounds()
		if bounds.Dx() <= 0 || bounds.Dy() <= 0 {
			t.Error("Generated image has invalid dimensions")
		}
	}

	// 清理测试文件
	os.Remove(outputPath)
}

// TestTextPositioning 测试文本定位准确性
func TestTextPositioning(t *testing.T) {
	renderer := gopdf.NewPDFRenderer(600, 400)
	renderer.SetDPI(150)

	outputPath := "text_positioning_test.png"
	err := renderer.RenderToPNG(outputPath, func(ctx gopdf.Context) {
		// 设置白色背景
		ctx.SetSourceRGB(1, 1, 1)
		ctx.Paint()

		// 绘制网格参考线
		ctx.SetSourceRGB(0.8, 0.8, 0.8)
		ctx.SetLineWidth(1)

		// 垂直线
		for x := 0; x <= 600; x += 50 {
			ctx.MoveTo(float64(x), 0)
			ctx.LineTo(float64(x), 400)
			ctx.Stroke()
		}

		// 水平线
		for y := 0; y <= 400; y += 50 {
			ctx.MoveTo(0, float64(y))
			ctx.LineTo(600, float64(y))
			ctx.Stroke()
		}

		// 在特定位置绘制文本
		ctx.SetSourceRGB(0, 0, 0)
		layout := ctx.PangoPdfCreateLayout().(*gopdf.PangoPdfLayout)
		fontDesc := gopdf.NewPangoFontDescription()
		fontDesc.SetFamily("sans-serif")
		fontDesc.SetSize(16)
		layout.SetFontDescription(fontDesc)

		// 在不同位置绘制文本
		testPositions := []struct {
			x, y float64
			text string
		}{
			{50, 50, "Position (50,50)"},
			{200, 100, "Position (200,100)"},
			{400, 200, "Position (400,200)"},
			{100, 300, "Position (100,300)"},
		}

		for _, pos := range testPositions {
			layout.SetText(pos.text)
			ctx.MoveTo(pos.x, pos.y)
			ctx.PangoPdfShowText(layout)
		}
	})

	if err != nil {
		t.Fatalf("Failed to render text positioning test: %v", err)
	}

	// 检查输出文件
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("Output PNG file was not created")
	}

	// 清理测试文件
	os.Remove(outputPath)
}

// TestTextScaling 测试文本缩放功能
func TestTextScaling(t *testing.T) {
	renderer := gopdf.NewPDFRenderer(800, 600)
	renderer.SetDPI(150)

	outputPath := "text_scaling_test.png"
	err := renderer.RenderToPNG(outputPath, func(ctx gopdf.Context) {
		// 设置白色背景
		ctx.SetSourceRGB(1, 1, 1)
		ctx.Paint()

		ctx.SetSourceRGB(0, 0, 0)
		layout := ctx.PangoPdfCreateLayout().(*gopdf.PangoPdfLayout)

		// 测试不同字体大小
		sizes := []int{12, 16, 20, 24, 32, 48}
		yPos := 50.0

		for _, size := range sizes {
			fontDesc := gopdf.NewPangoFontDescription()
			fontDesc.SetFamily("sans-serif")
			fontDesc.SetSize(float64(size))
			layout.SetFontDescription(fontDesc)

			text := fmt.Sprintf("Font Size %d: Hello World!", size)
			layout.SetText(text)

			ctx.MoveTo(50, yPos)
			ctx.PangoPdfShowText(layout)

			yPos += float64(size) + 10
		}
	})

	if err != nil {
		t.Fatalf("Failed to render text scaling test: %v", err)
	}

	// 检查输出文件
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("Output PNG file was not created")
	}

	// 清理测试文件
	os.Remove(outputPath)
}

// TestTextOverlap 测试文本重叠问题
func TestTextOverlap(t *testing.T) {
	renderer := gopdf.NewPDFRenderer(600, 400)
	renderer.SetDPI(150)

	outputPath := "text_overlap_test.png"
	err := renderer.RenderToPNG(outputPath, func(ctx gopdf.Context) {
		// 设置白色背景
		ctx.SetSourceRGB(1, 1, 1)
		ctx.Paint()

		ctx.SetSourceRGB(0, 0, 0)
		layout := ctx.PangoPdfCreateLayout().(*gopdf.PangoPdfLayout)

		// 测试英文单词间距
		fontDesc := gopdf.NewPangoFontDescription()
		fontDesc.SetFamily("sans-serif")
		fontDesc.SetSize(24)
		layout.SetFontDescription(fontDesc)

		// 正常英文文本
		layout.SetText("English words should not overlap")
		ctx.MoveTo(50, 50)
		ctx.PangoPdfShowText(layout)

		// 测试中文字符间距
		layout.SetText("中文字符也不应该重叠")
		ctx.MoveTo(50, 100)
		ctx.PangoPdfShowText(layout)

		// 测试长单词换行
		layout.SetText("Supercalifragilisticexpialidocious")
		ctx.MoveTo(50, 150)
		ctx.PangoPdfShowText(layout)
	})

	if err != nil {
		t.Fatalf("Failed to render text overlap test: %v", err)
	}

	// 检查输出文件
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("Output PNG file was not created")
	}

	// 清理测试文件
	os.Remove(outputPath)
}

// TestTextVisibility 测试文本在画布可视范围内的可见性
func TestTextVisibility(t *testing.T) {
	renderer := gopdf.NewPDFRenderer(400, 300)
	renderer.SetDPI(150)

	outputPath := "text_visibility_test.png"
	err := renderer.RenderToPNG(outputPath, func(ctx gopdf.Context) {
		// 设置白色背景
		ctx.SetSourceRGB(1, 1, 1)
		ctx.Paint()

		ctx.SetSourceRGB(0, 0, 0)
		layout := ctx.PangoPdfCreateLayout().(*gopdf.PangoPdfLayout)
		fontDesc := gopdf.NewPangoFontDescription()
		fontDesc.SetFamily("sans-serif")
		fontDesc.SetSize(16)
		layout.SetFontDescription(fontDesc)

		// 测试在边界处的文本
		testCases := []struct {
			x, y float64
			text string
		}{
			{0, 20, "Left edge text"},
			{350, 20, "Right edge text"},
			{20, 0, "Top edge text"},
			{20, 280, "Bottom edge text"},
			{0, 0, "Corner text"},
			{380, 280, "Opposite corner text"},
		}

		for _, tc := range testCases {
			layout.SetText(tc.text)
			ctx.MoveTo(tc.x, tc.y)
			ctx.PangoPdfShowText(layout)
		}
	})

	if err != nil {
		t.Fatalf("Failed to render text visibility test: %v", err)
	}

	// 检查输出文件
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("Output PNG file was not created")
	}

	// 清理测试文件
	os.Remove(outputPath)
}

// loadAndValidateImage 加载并验证图片
func loadAndValidateImage(filename string) (image.Image, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		return nil, err
	}

	// 基本验证
	bounds := img.Bounds()
	if bounds.Empty() {
		return nil, os.ErrInvalid
	}

	return img, nil
}
