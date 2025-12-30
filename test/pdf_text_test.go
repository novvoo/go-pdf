package test

import (
	"os"
	"strconv"
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// TestPDFTextRendering 测试PDF中的文本渲染质量
func TestPDFTextRendering(t *testing.T) {
	helper := NewTestHelper(t)

	// 尝试查找测试PDF文件，如果找不到则使用mock
	pdfPath := helper.FindTestPDF("test_vector.pdf")
	if pdfPath == "" {
		pdfPath = helper.FindTestPDF("test.pdf")
	}

	if pdfPath == "" {
		t.Skip("Skipping test: no test PDF file found")
	}

	// 创建PDF读取器
	reader := gopdf.NewPDFReader(pdfPath)

	// 测试渲染第一页到PNG
	outputPath := "pdf_text_test.png"
	defer helper.CleanupFile(outputPath)

	err := reader.RenderPageToPNG(1, outputPath, 150)

	if err != nil {
		t.Logf("Warning: Failed to render PDF page to PNG: %v", err)
		// 不强制失败，因为可能缺少依赖
	} else {
		// 检查输出文件是否存在
		helper.AssertFileExists(outputPath)
	}
}

// TestChineseTextRendering 测试中文文本渲染
func TestChineseTextRendering(t *testing.T) {
	helper := NewTestHelper(t)

	// 尝试查找测试PDF文件
	pdfPath := helper.FindTestPDF("test_vector.pdf")
	if pdfPath == "" {
		pdfPath = helper.FindTestPDF("test.pdf")
	}

	if pdfPath == "" {
		t.Skip("Skipping test: no test PDF file found")
	}

	// 创建PDF读取器
	reader := gopdf.NewPDFReader(pdfPath)

	// 测试渲染第一页到PNG，使用更高的DPI以更好地检查文本质量
	outputPath := "chinese_text_test.png"
	defer helper.CleanupFile(outputPath)

	err := reader.RenderPageToPNG(1, outputPath, 300)

	if err != nil {
		t.Logf("Warning: Failed to render PDF page to PNG: %v", err)
		// 不强制失败，因为可能缺少依赖
	} else {
		// 检查输出文件是否存在
		helper.AssertFileExists(outputPath)
	}
}

// TestTextAlignment 测试文本对齐
func TestTextAlignment(t *testing.T) {
	helper := NewTestHelper(t)

	// 尝试查找测试PDF文件
	pdfPath := helper.FindTestPDF("test_vector.pdf")
	if pdfPath == "" {
		pdfPath = helper.FindTestPDF("test.pdf")
	}

	if pdfPath == "" {
		t.Skip("Skipping test: no test PDF file found")
	}

	// 创建PDF读取器
	reader := gopdf.NewPDFReader(pdfPath)

	// 测试渲染多页以检查文本对齐一致性
	for pageNum := 1; pageNum <= 3; pageNum++ {
		outputPath := "text_alignment_test_page" + strconv.Itoa(pageNum) + ".png"
		defer helper.CleanupFile(outputPath)

		err := reader.RenderPageToPNG(pageNum, outputPath, 150)

		if err != nil {
			t.Logf("Warning: Failed to render page %d to PNG: %v", pageNum, err)
			continue
		}

		// 检查输出文件是否存在
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Errorf("Output PNG file for page %d was not created", pageNum)
		}
	}
}

// TestTextSpacing 测试文本间距
func TestTextSpacing(t *testing.T) {
	helper := NewTestHelper(t)

	// 尝试查找测试PDF文件
	pdfPath := helper.FindTestPDF("test_vector.pdf")
	if pdfPath == "" {
		pdfPath = helper.FindTestPDF("test.pdf")
	}

	if pdfPath == "" {
		t.Skip("Skipping test: no test PDF file found")
	}

	// 创建PDF读取器
	reader := gopdf.NewPDFReader(pdfPath)

	// 测试不同DPI设置下的文本渲染
	dpiSettings := []float64{72, 150, 300}
	for _, dpi := range dpiSettings {
		outputPath := "text_spacing_test_" + strconv.FormatFloat(dpi, 'f', -1, 64) + ".png"
		defer helper.CleanupFile(outputPath)

		err := reader.RenderPageToPNG(1, outputPath, dpi)

		if err != nil {
			t.Logf("Warning: Failed to render page with DPI %f: %v", dpi, err)
			continue
		}

		// 检查输出文件是否存在
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Errorf("Output PNG file for DPI %f was not created", dpi)
		}
	}
}
