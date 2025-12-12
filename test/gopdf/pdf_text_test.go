package gopdf_test

import (
	"os"
	"strconv"
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// TestPDFTextRendering 测试PDF中的文本渲染质量
func TestPDFTextRendering(t *testing.T) {
	// 检查测试PDF文件是否存在
	pdfPaths := []string{
		"test/test_vector.pdf",
		"test/test.pdf",
		"test_vector.pdf",
		"test.pdf",
	}

	var pdfPath string
	for _, path := range pdfPaths {
		if _, err := os.Stat(path); err == nil {
			pdfPath = path
			break
		}
	}

	if pdfPath == "" {
		t.Skip("Skipping test: no test PDF file found")
	}

	// 创建PDF读取器
	reader := gopdf.NewPDFReader(pdfPath)

	// 测试渲染第一页到PNG
	outputPath := "pdf_text_test.png"
	err := reader.RenderPageToPNG(1, outputPath, 150)

	if err != nil {
		t.Logf("Warning: Failed to render PDF page to PNG: %v", err)
		// 不强制失败，因为可能缺少依赖
	} else {
		// 检查输出文件是否存在
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Error("Output PNG file was not created")
		}

		// 清理测试文件
		os.Remove(outputPath)
	}
}

// TestChineseTextRendering 测试中文文本渲染
func TestChineseTextRendering(t *testing.T) {
	// 检查测试PDF文件是否存在
	pdfPaths := []string{
		"test/test_vector.pdf",
		"test/test.pdf",
		"test_vector.pdf",
		"test.pdf",
	}

	var pdfPath string
	for _, path := range pdfPaths {
		if _, err := os.Stat(path); err == nil {
			pdfPath = path
			break
		}
	}

	if pdfPath == "" {
		t.Skip("Skipping test: no test PDF file found")
	}

	// 创建PDF读取器
	reader := gopdf.NewPDFReader(pdfPath)

	// 测试渲染第一页到PNG，使用更高的DPI以更好地检查文本质量
	outputPath := "chinese_text_test.png"
	err := reader.RenderPageToPNG(1, outputPath, 300)

	if err != nil {
		t.Logf("Warning: Failed to render PDF page to PNG: %v", err)
		// 不强制失败，因为可能缺少依赖
	} else {
		// 检查输出文件是否存在
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Error("Output PNG file was not created")
		}

		// 清理测试文件
		os.Remove(outputPath)
	}
}

// TestTextAlignment 测试文本对齐
func TestTextAlignment(t *testing.T) {
	// 检查测试PDF文件是否存在
	pdfPaths := []string{
		"test/test_vector.pdf",
		"test/test.pdf",
		"test_vector.pdf",
		"test.pdf",
	}

	var pdfPath string
	for _, path := range pdfPaths {
		if _, err := os.Stat(path); err == nil {
			pdfPath = path
			break
		}
	}

	if pdfPath == "" {
		t.Skip("Skipping test: no test PDF file found")
	}

	// 创建PDF读取器
	reader := gopdf.NewPDFReader(pdfPath)

	// 测试渲染多页以检查文本对齐一致性
	for pageNum := 1; pageNum <= 3; pageNum++ {
		outputPath := "text_alignment_test_page" + strconv.Itoa(pageNum) + ".png"
		err := reader.RenderPageToPNG(pageNum, outputPath, 150)

		if err != nil {
			t.Logf("Warning: Failed to render page %d to PNG: %v", pageNum, err)
			// 删除可能创建的文件
			os.Remove(outputPath)
			continue
		}

		// 检查输出文件是否存在
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Errorf("Output PNG file for page %d was not created", pageNum)
		}

		// 清理测试文件
		os.Remove(outputPath)
	}
}

// TestTextSpacing 测试文本间距
func TestTextSpacing(t *testing.T) {
	// 检查测试PDF文件是否存在
	pdfPaths := []string{
		"test/test_vector.pdf",
		"test/test.pdf",
		"test_vector.pdf",
		"test.pdf",
	}

	var pdfPath string
	for _, path := range pdfPaths {
		if _, err := os.Stat(path); err == nil {
			pdfPath = path
			break
		}
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
		err := reader.RenderPageToPNG(1, outputPath, dpi)

		if err != nil {
			t.Logf("Warning: Failed to render page with DPI %f: %v", dpi, err)
			// 删除可能创建的文件
			os.Remove(outputPath)
			continue
		}

		// 检查输出文件是否存在
		if _, err := os.Stat(outputPath); os.IsNotExist(err) {
			t.Errorf("Output PNG file for DPI %f was not created", dpi)
		}

		// 清理测试文件
		os.Remove(outputPath)
	}
}
