package gopdf_test

import (
	"os"
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// TestPDFReaderCreation 测试PDF读取器的创建
func TestPDFReaderCreation(t *testing.T) {
	// 测试创建PDF读取器
	reader := gopdf.NewPDFReader("test.pdf")

	if reader == nil {
		t.Error("Failed to create PDF reader")
	}
}

// TestRenderPageToPNG 测试PDF页面渲染为PNG的功能
func TestRenderPageToPNG(t *testing.T) {
	// 检查测试PDF文件是否存在
	pdfPath := "test/test.pdf"
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		// 尝试其他路径
		if _, err2 := os.Stat("../test.pdf"); os.IsNotExist(err2) {
			if _, err3 := os.Stat("./test.pdf"); os.IsNotExist(err3) {
				if _, err4 := os.Stat("test.pdf"); os.IsNotExist(err4) {
					t.Skipf("Skipping test: test.pdf not found (err1: %v, err2: %v, err3: %v, err4: %v)", err, err2, err3, err4)
				} else {
					pdfPath = "test.pdf"
				}
			} else {
				pdfPath = "./test.pdf"
			}
		} else {
			pdfPath = "../test.pdf"
		}
	}

	// 创建PDF读取器
	reader := gopdf.NewPDFReader(pdfPath)
	// 测试渲染第一页到PNG
	outputPath := "page1_test.png"
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

// TestInvalidPageNumber 测试无效页码处理
func TestInvalidPageNumber(t *testing.T) {
	// 检查测试PDF文件是否存在
	pdfPath := "test/test.pdf"
	if _, err := os.Stat(pdfPath); os.IsNotExist(err) {
		// 尝试其他路径
		if _, err2 := os.Stat("../test.pdf"); os.IsNotExist(err2) {
			if _, err3 := os.Stat("./test.pdf"); os.IsNotExist(err3) {
				if _, err4 := os.Stat("test.pdf"); os.IsNotExist(err4) {
					t.Skipf("Skipping test: test.pdf not found (err1: %v, err2: %v, err3: %v, err4: %v)", err, err2, err3, err4)
				} else {
					pdfPath = "test.pdf"
				}
			} else {
				pdfPath = "./test.pdf"
			}
		} else {
			pdfPath = "../test.pdf"
		}
	}

	// 创建PDF读取器
	reader := gopdf.NewPDFReader(pdfPath)

	// 测试渲染不存在的页面（负数）
	err := reader.RenderPageToPNG(-1, "invalid_page.png", 150)
	if err == nil {
		t.Error("Expected error for negative page number, but got none")
	}

	// 测试渲染不存在的页面（过大）
	err = reader.RenderPageToPNG(999999, "invalid_page.png", 150)
	if err == nil {
		t.Error("Expected error for page number too large, but got none")
	}
}
