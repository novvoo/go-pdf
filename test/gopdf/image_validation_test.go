package gopdf_test

import (
	"os"
	"testing"

	"github.com/novvoo/go-cairo/pkg/cairo"
	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// TestImageOrientation 测试图像方向是否正确（未翻转）
func TestImageOrientation(t *testing.T) {
	// 创建渲染器
	renderer := gopdf.NewPDFRenderer(400, 300)
	renderer.SetDPI(150)

	outputPath := "orientation_test.png"
	err := renderer.RenderToPNG(outputPath, func(ctx cairo.Context) {
		// 设置白色背景
		ctx.SetSourceRGB(1, 1, 1)
		ctx.Paint()

		// 绘制一个有方向性的图案来测试翻转
		// 绘制一个箭头指向右上角
		ctx.SetSourceRGB(1, 0, 0) // 红色
		ctx.SetLineWidth(3)

		// 箭头主线 (从左下到右上)
		ctx.MoveTo(50, 250)
		ctx.LineTo(350, 50)
		ctx.Stroke()

		// 箭头头部
		ctx.MoveTo(340, 60)
		ctx.LineTo(350, 50)
		ctx.LineTo(340, 40)
		ctx.Stroke()

		// 在左下角标记起点
		ctx.SetSourceRGB(0, 1, 0) // 绿色
		ctx.Rectangle(45, 245, 10, 10)
		ctx.Fill()

		// 在右上角标记终点
		ctx.SetSourceRGB(0, 0, 1) // 蓝色
		ctx.Rectangle(345, 45, 10, 10)
		ctx.Fill()
	})

	if err != nil {
		t.Fatalf("Failed to render orientation test: %v", err)
	}

	// 验证图像
	img, err := loadAndValidateImage(outputPath)
	if err != nil {
		t.Fatalf("Failed to load image: %v", err)
	}

	// 检查关键像素点的颜色来验证方向
	bounds := img.Bounds()

	// 检查左下角的绿色方块
	greenPixel := img.At(50, bounds.Max.Y-55) // 接近左下角
	r, g, b, _ := greenPixel.RGBA()
	if !(g > r && g > b) {
		t.Error("Image may be flipped: green marker not found at expected position")
	}

	// 检查右上角的蓝色方块
	bluePixel := img.At(350, 50) // 右上角
	r, g, b, _ = bluePixel.RGBA()
	if !(b > r && b > g) {
		t.Error("Image may be flipped: blue marker not found at expected position")
	}

	// 清理测试文件
	os.Remove(outputPath)
}

// TestImageFlippingDetection 测试图像翻转检测
func TestImageFlippingDetection(t *testing.T) {
	// 创建渲染器
	renderer := gopdf.NewPDFRenderer(200, 200)
	renderer.SetDPI(150)

	outputPath := "flip_detection_test.png"
	err := renderer.RenderToPNG(outputPath, func(ctx cairo.Context) {
		// 设置白色背景
		ctx.SetSourceRGB(1, 1, 1)
		ctx.Paint()

		// 绘制一个不对称的图案来检测翻转
		// 上半部分绘制红色
		ctx.SetSourceRGB(1, 0, 0)
		ctx.Rectangle(0, 0, 200, 100)
		ctx.Fill()

		// 下半部分绘制蓝色
		ctx.SetSourceRGB(0, 0, 1)
		ctx.Rectangle(0, 100, 200, 100)
		ctx.Fill()

		// 在顶部中心绘制一个小绿点
		ctx.SetSourceRGB(0, 1, 0)
		ctx.Rectangle(95, 5, 10, 10)
		ctx.Fill()
	})

	if err != nil {
		t.Fatalf("Failed to render flip detection test: %v", err)
	}

	// 验证图像
	img, err := loadAndValidateImage(outputPath)
	if err != nil {
		t.Fatalf("Failed to load image: %v", err)
	}

	// 检查图像上半部分应该是红色，下半部分应该是蓝色
	bounds := img.Bounds()
	_ = bounds // 避免未使用变量警告

	// 检查上半部分中心点颜色
	upperPixel := img.At(100, 50) // 上半部分中心
	r, g, b, _ := upperPixel.RGBA()
	if r <= g || r <= b {
		t.Error("Image may be flipped: red color not found in upper half")
	}

	// 检查下半部分中心点颜色
	lowerPixel := img.At(100, 150) // 下半部分中心
	r, g, b, _ = lowerPixel.RGBA()
	if b <= r || b <= g {
		t.Error("Image may be flipped: blue color not found in lower half")
	}

	// 检查顶部的绿点
	topPixel := img.At(100, 10) // 顶部附近
	r, g, b, _ = topPixel.RGBA()
	if g <= r || g <= b {
		t.Error("Image may be flipped: green dot not found at top")
	}

	// 清理测试文件
	os.Remove(outputPath)
}

// TestCoordinateSystemConsistency 测试坐标系统一致性
func TestCoordinateSystemConsistency(t *testing.T) {
	// 创建渲染器
	renderer := gopdf.NewPDFRenderer(300, 300)
	renderer.SetDPI(150)

	outputPath := "coordinate_test.png"
	err := renderer.RenderToPNG(outputPath, func(ctx cairo.Context) {
		// 设置白色背景
		ctx.SetSourceRGB(1, 1, 1)
		ctx.Paint()

		// 使用坐标转换器
		converter := gopdf.NewCoordinateConverter(300, 300, gopdf.CoordSystemPDF)

		converter.TransformContext(ctx, func(ctx cairo.Context) {
			// 在PDF坐标系统中绘制：(0,0)应在左下角
			ctx.SetSourceRGB(1, 0, 0)
			ctx.Rectangle(10, 10, 30, 30) // 应该在左下角附近
			ctx.Fill()

			// 在右上角绘制一个标记
			ctx.SetSourceRGB(0, 1, 0)
			ctx.Rectangle(260, 260, 30, 30) // 应该在右上角附近
			ctx.Fill()
		})
	})

	if err != nil {
		t.Fatalf("Failed to render coordinate test: %v", err)
	}

	// 验证图像
	img, err := loadAndValidateImage(outputPath)
	if err != nil {
		t.Fatalf("Failed to load image: %v", err)
	}

	// 检查坐标系统的正确性
	bounds := img.Bounds()

	// 检查左下角的红色方块 (考虑到图像可能的翻转)
	leftBottomPixel := img.At(25, bounds.Max.Y-25) // 左下角附近
	r, g, b, _ := leftBottomPixel.RGBA()
	if r > g && r > b {
		// 红色点在预期位置
		t.Log("Red marker found at expected bottom-left position")
	} else {
		t.Error("Coordinate system may be inconsistent: red marker not found at bottom-left")
	}

	// 检查右上角的绿色方块
	rightTopPixel := img.At(275, 25) // 右上角附近
	r, g, b, _ = rightTopPixel.RGBA()
	if g > r && g > b {
		// 绿色点在预期位置
		t.Log("Green marker found at expected top-right position")
	} else {
		t.Error("Coordinate system may be inconsistent: green marker not found at top-right")
	}

	// 清理测试文件
	os.Remove(outputPath)
}
