package test

import (
	"image"
	"os"
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// TestImageOrientationNew 测试图像方向是否正确（未翻转）
func TestImageOrientationNew(t *testing.T) {
	// 创建渲染器
	renderer := gopdf.NewPDFRenderer(400, 300)
	renderer.SetDPI(150)

	outputPath := "orientation_test_new.png"
	err := renderer.RenderToPNG(outputPath, func(ctx gopdf.Context) {
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
	err := renderer.RenderToPNG(outputPath, func(ctx gopdf.Context) {
		// 设置白色背景
		ctx.SetSourceRGB(1, 1, 1)
		ctx.Paint()

		// 使用坐标转换器
		converter := gopdf.NewCoordinateConverter(300, 300, gopdf.CoordSystemPDF)

		converter.TransformContext(ctx, func(ctx gopdf.Context) {
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

	// 在图像坐标系中，(0,0)在左上角
	// 所以左下角是 (x=50, y=bounds.Max.Y-50)
	// 右上角是 (x=350, y=50)

	// 检查左下角的绿色方块 - 调整坐标检测范围
	greenFound := false
	for dy := -10; dy <= 10; dy++ {
		for dx := -10; dx <= 10; dx++ {
			x := 50 + dx
			y := bounds.Max.Y - 50 + dy
			if x >= 0 && x < bounds.Max.X && y >= 0 && y < bounds.Max.Y {
				greenPixel := img.At(x, y)
				r, g, b, _ := greenPixel.RGBA()
				// 转换为0-255范围
				r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)
				if g8 > 200 && g8 > r8*2 && g8 > b8*2 {
					greenFound = true
					break
				}
			}
		}
		if greenFound {
			break
		}
	}

	// 检查右上角的蓝色方块
	blueFound := false
	for dy := -10; dy <= 10; dy++ {
		for dx := -10; dx <= 10; dx++ {
			x := 350 + dx
			y := 50 + dy
			if x >= 0 && x < bounds.Max.X && y >= 0 && y < bounds.Max.Y {
				bluePixel := img.At(x, y)
				r, g, b, _ := bluePixel.RGBA()
				// 转换为0-255范围
				r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)
				if b8 > 200 && b8 > r8*2 && b8 > g8*2 {
					blueFound = true
					break
				}
			}
		}
		if blueFound {
			break
		}
	}

	// 如果绿色不在左下角或蓝色不在右上角，则图像可能是翻转的
	if !greenFound || !blueFound {
		details := ""
		if !greenFound {
			details += "Green marker not in expected bottom-left position. "
		}
		if !blueFound {
			details += "Blue marker not in expected top-right position. "
		}
		return true, details
	}

	return false, ""
}

// detectCoordinateSystemIssue 检测坐标系统问题
func detectCoordinateSystemIssue(img image.Image) (bool, string) {
	bounds := img.Bounds()

	// 检查左下角的红色方块 - 扩大检测范围
	redFound := false
	for dy := -15; dy <= 15; dy++ {
		for dx := -15; dx <= 15; dx++ {
			x := 25 + dx
			y := bounds.Max.Y - 25 + dy
			if x >= 0 && x < bounds.Max.X && y >= 0 && y < bounds.Max.Y {
				pixel := img.At(x, y)
				r, g, b, _ := pixel.RGBA()
				// 转换为0-255范围
				r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)
				if r8 > 200 && r8 > g8*2 && r8 > b8*2 {
					redFound = true
					break
				}
			}
		}
		if redFound {
			break
		}
	}

	// 检查右上角的绿色方块
	greenFound := false
	for dy := -15; dy <= 15; dy++ {
		for dx := -15; dx <= 15; dx++ {
			x := 275 + dx
			y := 25 + dy
			if x >= 0 && x < bounds.Max.X && y >= 0 && y < bounds.Max.Y {
				pixel := img.At(x, y)
				r, g, b, _ := pixel.RGBA()
				// 转换为0-255范围
				r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)
				if g8 > 200 && g8 > r8*2 && g8 > b8*2 {
					greenFound = true
					break
				}
			}
		}
		if greenFound {
			break
		}
	}

	// 如果红色不在左下角或绿色不在右上角，则可能存在坐标系统问题
	if !redFound || !greenFound {
		details := ""
		if !redFound {
			details += "Red marker not in expected bottom-left position. "
		}
		if !greenFound {
			details += "Green marker not in expected top-right position. "
		}
		return true, details
	}

	return false, ""
}
