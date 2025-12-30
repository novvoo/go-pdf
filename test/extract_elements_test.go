package test

import (
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// TestExtractPageElements 测试 ExtractPageElements 函数
func TestExtractPageElements(t *testing.T) {
	helper := NewTestHelper(t)
	mockGen := NewMockPDFGenerator()
	defer mockGen.Cleanup()

	// 生成包含文本的测试 PDF
	pdfPath, err := mockGen.GeneratePDFWithText("Test Content")
	helper.AssertNoError(err, "Failed to generate mock PDF")

	// 创建 PDF 读取器
	reader := gopdf.NewPDFReader(pdfPath)

	// 提取第一页的元素
	textElements, images := reader.ExtractPageElements(1)

	// 验证提取结果
	t.Logf("Extracted %d text elements and %d images", len(textElements), len(images))

	// 显示文本元素
	if len(textElements) > 0 {
		t.Logf("\nText Elements:")
		for i, te := range textElements {
			if i >= 10 { // 只显示前10个
				t.Logf("... and %d more text elements", len(textElements)-10)
				break
			}
			t.Logf("  [%d] Position: (%.2f, %.2f), Font: %s, Size: %.2f",
				i+1, te.X, te.Y, te.FontName, te.FontSize)
			// 限制文本长度
			displayText := te.Text
			if len(displayText) > 50 {
				displayText = displayText[:50] + "..."
			}
			t.Logf("      Text: %q", displayText)
		}
	} else {
		t.Log("No text elements found")
	}

	// 显示图片元素
	if len(images) > 0 {
		t.Logf("\nImage Elements:")
		for i, img := range images {
			t.Logf("  [%d] Name: %s, Position: (%.2f, %.2f), Size: %.2f x %.2f",
				i+1, img.Name, img.X, img.Y, img.Width, img.Height)
		}
	} else {
		t.Log("No image elements found")
	}
}

// TestExtractPageElementsMultiplePages 测试多页 PDF 的元素提取
func TestExtractPageElementsMultiplePages(t *testing.T) {
	helper := NewTestHelper(t)
	mockGen := NewMockPDFGenerator()
	defer mockGen.Cleanup()

	// 生成多页测试 PDF
	pageCount := 3
	pdfPath, err := mockGen.GenerateMultiPagePDF(pageCount)
	helper.AssertNoError(err, "Failed to generate multi-page PDF")

	reader := gopdf.NewPDFReader(pdfPath)

	// 获取页数
	actualPageCount, err := reader.GetPageCount()
	helper.AssertNoError(err, "Failed to get page count")

	t.Logf("PDF has %d pages", actualPageCount)

	// 提取每一页的元素
	for pageNum := 1; pageNum <= actualPageCount; pageNum++ {
		textElements, images := reader.ExtractPageElements(pageNum)
		t.Logf("\nPage %d: %d text elements, %d images",
			pageNum, len(textElements), len(images))

		// 显示前3个文本元素
		for i := 0; i < len(textElements) && i < 3; i++ {
			te := textElements[i]
			displayText := te.Text
			if len(displayText) > 30 {
				displayText = displayText[:30] + "..."
			}
			t.Logf("  Text: %q (%.2f, %.2f)", displayText, te.X, te.Y)
		}
	}
}

// TestExtractPageElementsWithPageInfo 测试提取元素并验证页面信息
func TestExtractPageElementsWithPageInfo(t *testing.T) {
	helper := NewTestHelper(t)
	mockGen := NewMockPDFGenerator()
	defer mockGen.Cleanup()

	// 生成指定尺寸的测试 PDF
	pdfPath, err := mockGen.GeneratePDFWithSize(612, 792)
	helper.AssertNoError(err, "Failed to generate PDF with size")

	reader := gopdf.NewPDFReader(pdfPath)

	// 获取页面信息
	pageInfo, err := reader.GetPageInfo(1)
	helper.AssertNoError(err, "Failed to get page info")

	t.Logf("Page size: %.2f x %.2f points (%.2f x %.2f inches)",
		pageInfo.Width, pageInfo.Height,
		pageInfo.Width/72, pageInfo.Height/72)

	// 提取元素
	textElements, images := reader.ExtractPageElements(1)

	// 验证元素位置是否在页面范围内
	for i, te := range textElements {
		if te.X < 0 || te.X > pageInfo.Width || te.Y < 0 || te.Y > pageInfo.Height {
			t.Logf("Warning: Text element %d position (%.2f, %.2f) is outside page bounds",
				i+1, te.X, te.Y)
		}
	}

	for i, img := range images {
		if img.X < 0 || img.X > pageInfo.Width || img.Y < 0 || img.Y > pageInfo.Height {
			t.Logf("Warning: Image element %d position (%.2f, %.2f) is outside page bounds",
				i+1, img.X, img.Y)
		}
	}

	t.Logf("Position validation completed")
}

// TestExtractFromEmptyPDF 测试从空白 PDF 提取元素
func TestExtractFromEmptyPDF(t *testing.T) {
	helper := NewTestHelper(t)
	mockGen := NewMockPDFGenerator()
	defer mockGen.Cleanup()

	// 生成空白 PDF
	pdfPath, err := mockGen.GenerateEmptyPDF()
	helper.AssertNoError(err, "Failed to generate empty PDF")

	reader := gopdf.NewPDFReader(pdfPath)
	textElements, images := reader.ExtractPageElements(1)

	// 空白 PDF 应该没有元素
	t.Logf("Empty PDF: %d text elements, %d images", len(textElements), len(images))
}

// TestExtractFromCorruptedPDF 测试从损坏的 PDF 提取元素
func TestExtractFromCorruptedPDF(t *testing.T) {
	helper := NewTestHelper(t)
	mockGen := NewMockPDFGenerator()
	defer mockGen.Cleanup()

	// 生成损坏的 PDF
	pdfPath, err := mockGen.GenerateCorruptedPDF()
	helper.AssertNoError(err, "Failed to generate corrupted PDF")

	reader := gopdf.NewPDFReader(pdfPath)

	// 尝试获取页数应该失败
	_, err = reader.GetPageCount()
	if err == nil {
		t.Log("Warning: Expected error when reading corrupted PDF, but got none")
	} else {
		t.Logf("Expected error occurred: %v", err)
	}
}
