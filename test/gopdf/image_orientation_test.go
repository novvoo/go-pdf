package gopdf_test

import (
	"image"
	"os"
	"testing"

	"github.com/novvoo/go-cairo/pkg/cairo"
	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// TestImageOrientationNew 测试图像方向是否正确（未翻转）
func TestImageOrientationNew(t *testing.T) {
	// 创建渲染器
	renderer := gopdf.NewPDFRenderer(400, 300)
	renderer.SetDPI(150)

	outputPath := "orientation_test_new.png"
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

	// 检测图像是否翻转
	isFlipped, details := detectImageFlipping(img)
	if isFlipped {
		t.Errorf("Image is flipped! Details: %s", details)
	} else {
		t.Log("Image orientation is correct")
	}

	// 清理测试文件
	os.Remove(outputPath)
}

// TestCoordinateSystemNew 测试坐标系统一致性
func TestCoordinateSystemNew(t *testing.T) {
	// 创建渲染器
	renderer := gopdf.NewPDFRenderer(300, 300)
	renderer.SetDPI(150)

	outputPath := "coordinate_system_test_new.png"
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
		t.Fatalf("Failed to render coordinate system test: %v", err)
	}

	// 验证图像
	img, err := loadAndValidateImage(outputPath)
	if err != nil {
		t.Fatalf("Failed to load image: %v", err)
	}

	// 检测坐标系统问题
	hasCoordIssue, details := detectCoordinateSystemIssue(img)
	if hasCoordIssue {
		t.Errorf("Coordinate system issue detected! Details: %s", details)
	} else {
		t.Log("Coordinate system is consistent")
	}

	// 清理测试文件
	os.Remove(outputPath)
}

// detectImageFlipping 检测图像是否翻转
func detectImageFlipping(img image.Image) (bool, string) {
	bounds := img.Bounds()

	// 检查左下角的绿色方块
	greenPixel := img.At(50, bounds.Max.Y-55) // 接近左下角
	r, g, b, _ := greenPixel.RGBA()
	isGreenInBottomLeft := g > r && g > b

	// 检查右上角的蓝色方块
	bluePixel := img.At(350, 50) // 右上角
	r, g, b, _ = bluePixel.RGBA()
	isBlueInTopRight := b > r && b > g

	// 如果绿色不在左下角或蓝色不在右上角，则图像可能是翻转的
	if !isGreenInBottomLeft || !isBlueInTopRight {
		details := ""
		if !isGreenInBottomLeft {
			details += "Green marker not in expected bottom-left position. "
		}
		if !isBlueInTopRight {
			details += "Blue marker not in expected top-right position. "
		}
		return true, details
	}

	return false, ""
}

// detectCoordinateSystemIssue 检测坐标系统问题
func detectCoordinateSystemIssue(img image.Image) (bool, string) {
	bounds := img.Bounds()

	// 检查左下角的红色方块 (考虑到图像可能的翻转)
	leftBottomPixel := img.At(25, bounds.Max.Y-25) // 左下角附近
	r, g, b, _ := leftBottomPixel.RGBA()
	isRedInBottomLeft := r > g && r > b

	// 检查右上角的绿色方块
	rightTopPixel := img.At(275, 25) // 右上角附近
	r, g, b, _ = rightTopPixel.RGBA()
	isGreenInTopRight := g > r && g > b

	// 如果红色不在左下角或绿色不在右上角，则可能存在坐标系统问题
	if !isRedInBottomLeft || !isGreenInTopRight {
		details := ""
		if !isRedInBottomLeft {
			details += "Red marker not in expected bottom-left position. "
		}
		if !isGreenInTopRight {
			details += "Green marker not in expected top-right position. "
		}
		return true, details
	}

	return false, ""
}
