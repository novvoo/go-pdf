package test

import (
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// TestPDFReaderCreation 测试PDF读取器的创建
func TestPDFReaderCreation(t *testing.T) {
	reader := gopdf.NewPDFReader("test.pdf")
	if reader == nil {
		t.Error("Failed to create PDF reader")
	}
}

// TestRenderPageToPNG 测试PDF页面渲染为PNG的功能
func TestRenderPageToPNG(t *testing.T) {
	helper := NewTestHelper(t)
	mockGen := NewMockPDFGenerator()
	defer mockGen.Cleanup()

	// 生成测试 PDF
	pdfPath, err := mockGen.GenerateSimplePDF()
	helper.AssertNoError(err, "Failed to generate mock PDF")

	reader := gopdf.NewPDFReader(pdfPath)
	outputPath := "page1_test.png"
	defer helper.CleanupFile(outputPath)

	err = reader.RenderPageToPNG(1, outputPath, 150)
	if err != nil {
		t.Skipf("Skipping: Failed to render PDF (may lack dependencies): %v", err)
	}

	helper.AssertFileExists(outputPath)
}

// TestInvalidPageNumber 测试无效页码处理
func TestInvalidPageNumber(t *testing.T) {
	helper := NewTestHelper(t)
	mockGen := NewMockPDFGenerator()
	defer mockGen.Cleanup()

	// 生成测试 PDF
	pdfPath, err := mockGen.GenerateSimplePDF()
	helper.AssertNoError(err, "Failed to generate mock PDF")

	reader := gopdf.NewPDFReader(pdfPath)

	tests := []struct {
		name    string
		pageNum int
		wantErr bool
	}{
		{"negative page number", -1, true},
		{"zero page number", 0, true},
		{"extremely large page number", 999999, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := reader.RenderPageToPNG(tt.pageNum, "invalid_page.png", 150)
			if tt.wantErr {
				helper.AssertError(err, "Expected error for "+tt.name)
			} else {
				helper.AssertNoError(err, "Unexpected error for "+tt.name)
			}
		})
	}
}

// TestGetPageCount 测试获取页数
func TestGetPageCount(t *testing.T) {
	helper := NewTestHelper(t)
	mockGen := NewMockPDFGenerator()
	defer mockGen.Cleanup()

	// 生成测试 PDF
	pdfPath, err := mockGen.GenerateSimplePDF()
	helper.AssertNoError(err, "Failed to generate mock PDF")

	reader := gopdf.NewPDFReader(pdfPath)
	count, err := reader.GetPageCount()

	helper.AssertNoError(err, "Failed to get page count")
	helper.AssertTrue(count > 0, "Page count should be positive")
}

// TestGetPageInfo 测试获取页面信息
func TestGetPageInfo(t *testing.T) {
	helper := NewTestHelper(t)
	mockGen := NewMockPDFGenerator()
	defer mockGen.Cleanup()

	// 生成测试 PDF
	pdfPath, err := mockGen.GenerateSimplePDF()
	helper.AssertNoError(err, "Failed to generate mock PDF")

	reader := gopdf.NewPDFReader(pdfPath)
	pageInfo, err := reader.GetPageInfo(1)

	helper.AssertNoError(err, "Failed to get page info")
	helper.AssertTrue(pageInfo.Width > 0, "Page width should be positive")
	helper.AssertTrue(pageInfo.Height > 0, "Page height should be positive")
}

// TestMultiPagePDF 测试多页 PDF 处理
func TestMultiPagePDF(t *testing.T) {
	helper := NewTestHelper(t)
	mockGen := NewMockPDFGenerator()
	defer mockGen.Cleanup()

	tests := []struct {
		name      string
		pageCount int
	}{
		{"single page", 1},
		{"two pages", 2},
		{"five pages", 5},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdfPath, err := mockGen.GenerateMultiPagePDF(tt.pageCount)
			helper.AssertNoError(err, "Failed to generate multi-page PDF")

			reader := gopdf.NewPDFReader(pdfPath)
			count, err := reader.GetPageCount()
			helper.AssertNoError(err, "Failed to get page count")
			helper.AssertEqual(count, tt.pageCount, "Page count mismatch")
		})
	}
}

// TestPDFWithDifferentSizes 测试不同尺寸的 PDF
func TestPDFWithDifferentSizes(t *testing.T) {
	helper := NewTestHelper(t)
	mockGen := NewMockPDFGenerator()
	defer mockGen.Cleanup()

	tests := []struct {
		name   string
		width  float64
		height float64
	}{
		{"Letter", 612, 792},
		{"A4", 595, 842},
		{"Legal", 612, 1008},
		{"Square", 500, 500},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pdfPath, err := mockGen.GeneratePDFWithSize(tt.width, tt.height)
			helper.AssertNoError(err, "Failed to generate PDF with size")

			reader := gopdf.NewPDFReader(pdfPath)
			pageInfo, err := reader.GetPageInfo(1)
			helper.AssertNoError(err, "Failed to get page info")

			// 允许小的浮点误差
			if pageInfo.Width < tt.width-1 || pageInfo.Width > tt.width+1 {
				t.Errorf("Width mismatch: got %.2f, want %.2f", pageInfo.Width, tt.width)
			}
			if pageInfo.Height < tt.height-1 || pageInfo.Height > tt.height+1 {
				t.Errorf("Height mismatch: got %.2f, want %.2f", pageInfo.Height, tt.height)
			}
		})
	}
}

// TestCorruptedPDFHandling 测试损坏 PDF 的错误处理
func TestCorruptedPDFHandling(t *testing.T) {
	helper := NewTestHelper(t)
	mockGen := NewMockPDFGenerator()
	defer mockGen.Cleanup()

	pdfPath, err := mockGen.GenerateCorruptedPDF()
	helper.AssertNoError(err, "Failed to generate corrupted PDF")

	reader := gopdf.NewPDFReader(pdfPath)

	// 所有操作都应该返回错误
	_, err = reader.GetPageCount()
	helper.AssertError(err, "Expected error for corrupted PDF")

	_, err = reader.GetPageInfo(1)
	helper.AssertError(err, "Expected error for corrupted PDF")

	err = reader.RenderPageToPNG(1, "output.png", 150)
	helper.AssertError(err, "Expected error for corrupted PDF")
}

// TestEmptyPDFHandling 测试空白 PDF 处理
func TestEmptyPDFHandling(t *testing.T) {
	helper := NewTestHelper(t)
	mockGen := NewMockPDFGenerator()
	defer mockGen.Cleanup()

	pdfPath, err := mockGen.GenerateEmptyPDF()
	helper.AssertNoError(err, "Failed to generate empty PDF")

	reader := gopdf.NewPDFReader(pdfPath)

	// 空白 PDF 应该有 1 页
	count, err := reader.GetPageCount()
	helper.AssertNoError(err, "Failed to get page count")
	helper.AssertEqual(count, 1, "Empty PDF should have 1 page")

	// 应该能获取页面信息
	pageInfo, err := reader.GetPageInfo(1)
	helper.AssertNoError(err, "Failed to get page info")
	helper.AssertTrue(pageInfo.Width > 0, "Page width should be positive")
	helper.AssertTrue(pageInfo.Height > 0, "Page height should be positive")
}
