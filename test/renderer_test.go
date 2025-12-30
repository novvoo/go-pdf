package test

import (
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// TestPDFRenderer 测试PDF渲染器的基本功能
func TestPDFRenderer(t *testing.T) {
	helper := NewTestHelper(t)

	renderer := gopdf.NewPDFRenderer(600, 800)
	renderer.SetDPI(150)

	outputPath := "test_output.png"
	defer helper.CleanupFile(outputPath)

	err := renderer.RenderToPNG(outputPath, func(ctx gopdf.Context) {
		ctx.SetSourceRGB(0.5, 0.5, 0.5)
		ctx.Rectangle(50, 50, 100, 100)
		ctx.Fill()
	})

	helper.AssertNoError(err, "Failed to render to PNG")
	helper.AssertFileExists(outputPath)
}

// TestPDFRendererDimensions 测试不同尺寸的渲染器
func TestPDFRendererDimensions(t *testing.T) {
	tests := []struct {
		name   string
		width  float64
		height float64
	}{
		{"small", 100, 100},
		{"medium", 600, 800},
		{"large", 2000, 3000},
		{"wide", 1000, 500},
		{"tall", 500, 1000},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			renderer := gopdf.NewPDFRenderer(tt.width, tt.height)
			if renderer == nil {
				t.Errorf("Failed to create renderer with dimensions %.0fx%.0f", tt.width, tt.height)
			}
		})
	}
}

// TestConvertImageToPNG 测试图片格式转换功能
func TestConvertImageToPNG(t *testing.T) {
	helper := NewTestHelper(t)
	imagePath := helper.FindTestPDF("test.png")

	outputPath := "converted_test.png"
	defer helper.CleanupFile(outputPath)

	err := gopdf.ConvertImageToPNG(imagePath, outputPath)
	if err != nil {
		t.Skipf("Skipping: Failed to convert image (may lack dependencies): %v", err)
	}

	helper.AssertFileExists(outputPath)
}

// TestCoordinateConverter 测试坐标转换功能
func TestCoordinateConverter(t *testing.T) {
	tests := []struct {
		name      string
		width     float64
		height    float64
		system    gopdf.CoordinateSystem
		x         float64
		y         float64
		expectedX float64
		expectedY float64
	}{
		{
			name:  "PDF to Gopdf",
			width: 600, height: 800,
			system: gopdf.CoordSystemPDF,
			x:      100, y: 100,
			expectedX: 100, expectedY: 700,
		},
		{
			name:  "origin point",
			width: 600, height: 800,
			system: gopdf.CoordSystemPDF,
			x:      0, y: 0,
			expectedX: 0, expectedY: 800,
		},
		{
			name:  "top-right corner",
			width: 600, height: 800,
			system: gopdf.CoordSystemPDF,
			x:      600, y: 800,
			expectedX: 600, expectedY: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			converter := gopdf.NewCoordinateConverter(tt.width, tt.height, tt.system)
			if converter == nil {
				t.Fatal("Failed to create coordinate converter")
			}

			x, y := converter.PDFToGopdf(tt.x, tt.y)
			if x != tt.expectedX || y != tt.expectedY {
				t.Errorf("PDFToGopdf(%f, %f) = (%f, %f), want (%f, %f)",
					tt.x, tt.y, x, y, tt.expectedX, tt.expectedY)
			}

			// 测试反向转换
			x2, y2 := converter.GopdfToPDF(x, y)
			if x2 != tt.x || y2 != tt.y {
				t.Errorf("GopdfToPDF(%f, %f) = (%f, %f), want (%f, %f)",
					x, y, x2, y2, tt.x, tt.y)
			}
		})
	}
}

// BenchmarkPDFRenderer 基准测试渲染性能
func BenchmarkPDFRenderer(b *testing.B) {
	renderer := gopdf.NewPDFRenderer(600, 800)
	renderer.SetDPI(150)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = renderer.RenderToPNG("bench_output.png", func(ctx gopdf.Context) {
			ctx.SetSourceRGB(0.5, 0.5, 0.5)
			ctx.Rectangle(50, 50, 100, 100)
			ctx.Fill()
		})
	}
}
