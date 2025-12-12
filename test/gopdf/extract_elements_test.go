package gopdf_test

import (
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// TestExtractPageElements 测试 ExtractPageElements 函数
func TestExtractPageElements(t *testing.T) {
	pdfPath := "../test_vector.pdf"

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
	pdfPath := "../test.pdf"

	reader := gopdf.NewPDFReader(pdfPath)

	// 获取页数
	pageCount, err := reader.GetPageCount()
	if err != nil {
		t.Fatalf("Failed to get page count: %v", err)
	}

	t.Logf("PDF has %d pages", pageCount)

	// 提取每一页的元素
	for pageNum := 1; pageNum <= pageCount; pageNum++ {
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
	pdfPath := "../test_vector.pdf"

	reader := gopdf.NewPDFReader(pdfPath)

	// 获取页面信息
	pageInfo, err := reader.GetPageInfo(1)
	if err != nil {
		t.Fatalf("Failed to get page info: %v", err)
	}

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
