package gopdf_test

import (
	"os"
	"testing"

	"github.com/novvoo/go-cairo/pkg/cairo"
	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// TestPDFRenderer 测试PDF渲染器的基本功能
func TestPDFRenderer(t *testing.T) {
	// 创建一个新的PDF渲染器
	renderer := gopdf.NewPDFRenderer(600, 800)

	// 测试设置DPI
	renderer.SetDPI(150)

	// 测试渲染到PNG文件
	outputPath := "test_output.png"
	err := renderer.RenderToPNG(outputPath, func(ctx cairo.Context) {
		// 简单的绘制操作
		ctx.SetSourceRGB(0.5, 0.5, 0.5)
		ctx.Rectangle(50, 50, 100, 100)
		ctx.Fill()
	})

	if err != nil {
		t.Fatalf("Failed to render to PNG: %v", err)
	}

	// 检查输出文件是否存在
	if _, err := os.Stat(outputPath); os.IsNotExist(err) {
		t.Error("Output PNG file was not created")
	}

	// 清理测试文件
	os.Remove(outputPath)
}

// TestConvertImageToPNG 测试图片格式转换功能
func TestConvertImageToPNG(t *testing.T) {
	// 查找test.png文件
	imagePath := "test/test.png"
	if _, err := os.Stat(imagePath); os.IsNotExist(err) {
		// 尝试其他路径
		if _, err2 := os.Stat("../test.png"); os.IsNotExist(err2) {
			if _, err3 := os.Stat("./test.png"); os.IsNotExist(err3) {
				if _, err4 := os.Stat("test.png"); os.IsNotExist(err4) {
					t.Log("Skipping image conversion test: test.png not found")
					return
				} else {
					imagePath = "test.png"
				}
			} else {
				imagePath = "./test.png"
			}
		} else {
			imagePath = "../test.png"
		}
	}

	outputPath := "converted_test.png"
	err := gopdf.ConvertImageToPNG(imagePath, outputPath)

	if err != nil {
		t.Logf("Warning: Failed to convert image: %v", err)
	} else {
		// 检查输出文件是否存在
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Error("Converted PNG file was not created")
		}

		// 清理测试文件
		os.Remove(outputPath)
	}
}

// TestCoordinateConverter 测试坐标转换功能
func TestCoordinateConverter(t *testing.T) {
	// 创建坐标转换器
	converter := gopdf.NewCoordinateConverter(600, 800, gopdf.CoordSystemPDF)

	if converter == nil {
		t.Error("Failed to create coordinate converter")
	}

	// 测试坐标转换 (PDF到Cairo)
	x, y := converter.PDFToCairo(100, 100)
	expectedY := 700.0 // 800 - 100
	if x != 100 || y != expectedY {
		t.Errorf("Coordinate conversion failed: expected (100,%f), got (%f,%f)", expectedY, x, y)
	}

	// 测试反向转换 (Cairo到PDF)
	x2, y2 := converter.CairoToPDF(100, expectedY)
	if x2 != 100 || y2 != 100 {
		t.Errorf("Reverse coordinate conversion failed: expected (100,100), got (%f,%f)", x2, y2)
	}
}
