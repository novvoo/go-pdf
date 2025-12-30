package test

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
)

// MockPDFGenerator 用于生成测试用的 PDF 文件
type MockPDFGenerator struct {
	tempDir string
}

// NewMockPDFGenerator 创建 mock PDF 生成器
func NewMockPDFGenerator() *MockPDFGenerator {
	tempDir := filepath.Join(os.TempDir(), "gopdf_test")
	os.MkdirAll(tempDir, 0755)
	return &MockPDFGenerator{
		tempDir: tempDir,
	}
}

// Cleanup 清理临时文件
func (m *MockPDFGenerator) Cleanup() {
	os.RemoveAll(m.tempDir)
}

// GenerateSimplePDF 生成一个简单的单页 PDF（包含文本和图片）
func (m *MockPDFGenerator) GenerateSimplePDF() (string, error) {
	pdfPath := filepath.Join(m.tempDir, "simple.pdf")

	// 创建一个蓝色方块图片数据（8x8 像素）
	imageWidth := 8
	imageHeight := 8
	imageData := make([]byte, imageWidth*imageHeight*3)
	for i := 0; i < len(imageData); i += 3 {
		imageData[i] = 0     // R
		imageData[i+1] = 0   // G
		imageData[i+2] = 255 // B - 蓝色
	}

	content := fmt.Sprintf(`%%PDF-1.4
1 0 obj
<<
/Type /Catalog
/Pages 2 0 R
>>
endobj
2 0 obj
<<
/Type /Pages
/Kids [3 0 R]
/Count 1
>>
endobj
3 0 obj
<<
/Type /Page
/Parent 2 0 R
/MediaBox [0 0 612 792]
/Contents 4 0 R
/Resources <<
/Font <<
/F1 <<
/Type /Font
/Subtype /Type1
/BaseFont /Helvetica
>>
>>
/XObject <<
/Im1 5 0 R
>>
>>
>>
endobj
4 0 obj
<<
/Length 120
>>
stream
BT
/F1 12 Tf
100 700 Td
(Hello World) Tj
ET
q
50 0 0 50 100 600 cm
/Im1 Do
Q
endstream
endobj
5 0 obj
<<
/Type /XObject
/Subtype /Image
/Width %d
/Height %d
/ColorSpace /DeviceRGB
/BitsPerComponent 8
/Length %d
>>
stream
%s
endstream
endobj
xref
0 6
0000000000 65535 f 
0000000009 00000 n 
0000000058 00000 n 
0000000115 00000 n 
0000000360 00000 n 
0000000530 00000 n 
trailer
<<
/Size 6
/Root 1 0 R
>>
startxref
700
%%%%EOF
`, imageWidth, imageHeight, len(imageData), string(imageData))

	return pdfPath, os.WriteFile(pdfPath, []byte(content), 0644)
}

// GenerateMultiPagePDF 生成多页 PDF（每页包含文本和图片）
func (m *MockPDFGenerator) GenerateMultiPagePDF(pageCount int) (string, error) {
	pdfPath := filepath.Join(m.tempDir, fmt.Sprintf("multipage_%d.pdf", pageCount))

	// 创建不同颜色的图片数据
	imageWidth := 8
	imageHeight := 8
	imageSize := imageWidth * imageHeight * 3

	// 构建页面对象引用列表
	var pageRefs bytes.Buffer
	for i := 0; i < pageCount; i++ {
		if i > 0 {
			pageRefs.WriteString(" ")
		}
		pageRefs.WriteString(fmt.Sprintf("%d 0 R", 3+i*3))
	}

	// 构建页面对象、内容流和图片对象
	var pagesContent bytes.Buffer
	objNum := 3
	for i := 0; i < pageCount; i++ {
		// 为每页创建不同颜色的图片
		imageData := make([]byte, imageSize)
		r := byte((i * 50) % 256)
		g := byte((i * 100) % 256)
		b := byte((i * 150) % 256)
		for j := 0; j < len(imageData); j += 3 {
			imageData[j] = r
			imageData[j+1] = g
			imageData[j+2] = b
		}

		// 页面对象
		pagesContent.WriteString(fmt.Sprintf(`%d 0 obj
<<
/Type /Page
/Parent 2 0 R
/MediaBox [0 0 612 792]
/Contents %d 0 R
/Resources <<
/Font <<
/F1 <<
/Type /Font
/Subtype /Type1
/BaseFont /Helvetica
>>
>>
/XObject <<
/Im%d %d 0 R
>>
>>
>>
endobj
`, objNum, objNum+1, i+1, objNum+2))

		// 内容流对象（包含文本和图片）
		text := fmt.Sprintf("Page %d of %d", i+1, pageCount)
		stream := fmt.Sprintf(`BT
/F1 18 Tf
50 750 Td
(%s) Tj
ET
BT
/F1 12 Tf
50 720 Td
(This page contains text and an image) Tj
ET
q
80 0 0 80 50 600 cm
/Im%d Do
Q
BT
/F1 10 Tf
50 580 Td
(Image color varies by page number) Tj
ET
`, text, i+1)

		pagesContent.WriteString(fmt.Sprintf(`%d 0 obj
<<
/Length %d
>>
stream
%sendstream
endobj
`, objNum+1, len(stream), stream))

		// 图片对象
		pagesContent.WriteString(fmt.Sprintf(`%d 0 obj
<<
/Type /XObject
/Subtype /Image
/Width %d
/Height %d
/ColorSpace /DeviceRGB
/BitsPerComponent 8
/Length %d
>>
stream
%s
endstream
endobj
`, objNum+2, imageWidth, imageHeight, len(imageData), string(imageData)))

		objNum += 3
	}

	content := fmt.Sprintf(`%%PDF-1.4
1 0 obj
<<
/Type /Catalog
/Pages 2 0 R
>>
endobj
2 0 obj
<<
/Type /Pages
/Kids [%s]
/Count %d
>>
endobj
%s`, pageRefs.String(), pageCount, pagesContent.String())

	return pdfPath, os.WriteFile(pdfPath, []byte(content), 0644)
}

// GeneratePDFWithText 生成包含指定文本的 PDF
func (m *MockPDFGenerator) GeneratePDFWithText(text string) (string, error) {
	pdfPath := filepath.Join(m.tempDir, "text.pdf")

	content := fmt.Sprintf(`%%PDF-1.4
1 0 obj
<<
/Type /Catalog
/Pages 2 0 R
>>
endobj
2 0 obj
<<
/Type /Pages
/Kids [3 0 R]
/Count 1
>>
endobj
3 0 obj
<<
/Type /Page
/Parent 2 0 R
/MediaBox [0 0 612 792]
/Contents 4 0 R
/Resources <<
/Font <<
/F1 <<
/Type /Font
/Subtype /Type1
/BaseFont /Helvetica
>>
>>
>>
>>
endobj
4 0 obj
<<
/Length %d
>>
stream
BT
/F1 12 Tf
100 700 Td
(%s) Tj
ET
endstream
endobj
xref
0 5
0000000000 65535 f 
0000000009 00000 n 
0000000058 00000 n 
0000000115 00000 n 
0000000317 00000 n 
trailer
<<
/Size 5
/Root 1 0 R
>>
startxref
410
%%%%EOF
`, len(text)+30, text)

	return pdfPath, os.WriteFile(pdfPath, []byte(content), 0644)
}

// GenerateEmptyPDF 生成空白 PDF（无内容）
func (m *MockPDFGenerator) GenerateEmptyPDF() (string, error) {
	pdfPath := filepath.Join(m.tempDir, "empty.pdf")

	content := `%PDF-1.4
1 0 obj
<<
/Type /Catalog
/Pages 2 0 R
>>
endobj
2 0 obj
<<
/Type /Pages
/Kids [3 0 R]
/Count 1
>>
endobj
3 0 obj
<<
/Type /Page
/Parent 2 0 R
/MediaBox [0 0 612 792]
/Contents 4 0 R
>>
endobj
4 0 obj
<<
/Length 0
>>
stream
endstream
endobj
xref
0 5
0000000000 65535 f 
0000000009 00000 n 
0000000058 00000 n 
0000000115 00000 n 
0000000229 00000 n 
trailer
<<
/Size 5
/Root 1 0 R
>>
startxref
278
%%EOF
`

	return pdfPath, os.WriteFile(pdfPath, []byte(content), 0644)
}

// GenerateCorruptedPDF 生成损坏的 PDF（用于错误处理测试）
func (m *MockPDFGenerator) GenerateCorruptedPDF() (string, error) {
	pdfPath := filepath.Join(m.tempDir, "corrupted.pdf")
	content := []byte("This is not a valid PDF file")
	return pdfPath, os.WriteFile(pdfPath, content, 0644)
}

// GeneratePDFWithSize 生成指定尺寸的 PDF
func (m *MockPDFGenerator) GeneratePDFWithSize(width, height float64) (string, error) {
	pdfPath := filepath.Join(m.tempDir, fmt.Sprintf("size_%.0fx%.0f.pdf", width, height))

	content := fmt.Sprintf(`%%PDF-1.4
1 0 obj
<<
/Type /Catalog
/Pages 2 0 R
>>
endobj
2 0 obj
<<
/Type /Pages
/Kids [3 0 R]
/Count 1
>>
endobj
3 0 obj
<<
/Type /Page
/Parent 2 0 R
/MediaBox [0 0 %.2f %.2f]
/Contents 4 0 R
>>
endobj
4 0 obj
<<
/Length 0
>>
stream
endstream
endobj
xref
0 5
0000000000 65535 f 
0000000009 00000 n 
0000000058 00000 n 
0000000115 00000 n 
0000000229 00000 n 
trailer
<<
/Size 5
/Root 1 0 R
>>
startxref
278
%%%%EOF
`, width, height)

	return pdfPath, os.WriteFile(pdfPath, []byte(content), 0644)
}

// GenerateMixedContentPDF 生成图文混排的 PDF（包含文本和图片）
func (m *MockPDFGenerator) GenerateMixedContentPDF() (string, error) {
	pdfPath := filepath.Join(m.tempDir, "mixed_content.pdf")

	// 创建一个简单的红色方块图片数据（10x10 像素，RGB 格式）
	// 每个像素 3 字节 (R, G, B)
	imageWidth := 10
	imageHeight := 10
	imageData := make([]byte, imageWidth*imageHeight*3)
	for i := 0; i < len(imageData); i += 3 {
		imageData[i] = 255 // R - 红色
		imageData[i+1] = 0 // G
		imageData[i+2] = 0 // B
	}

	content := fmt.Sprintf(`%%PDF-1.4
1 0 obj
<<
/Type /Catalog
/Pages 2 0 R
>>
endobj
2 0 obj
<<
/Type /Pages
/Kids [3 0 R]
/Count 1
>>
endobj
3 0 obj
<<
/Type /Page
/Parent 2 0 R
/MediaBox [0 0 612 792]
/Contents 4 0 R
/Resources <<
/Font <<
/F1 <<
/Type /Font
/Subtype /Type1
/BaseFont /Helvetica
>>
/F2 <<
/Type /Font
/Subtype /Type1
/BaseFont /Helvetica-Bold
>>
>>
/XObject <<
/Im1 5 0 R
>>
>>
>>
endobj
4 0 obj
<<
/Length 450
>>
stream
BT
/F2 24 Tf
50 750 Td
(Mixed Content PDF) Tj
ET
BT
/F1 14 Tf
50 720 Td
(This PDF contains both text and images) Tj
ET
BT
/F1 12 Tf
50 690 Td
(Below is a red square image:) Tj
ET
q
100 0 0 100 50 550 cm
/Im1 Do
Q
BT
/F1 12 Tf
50 520 Td
(Text after image:) Tj
0 -20 Td
(Line 1: Lorem ipsum dolor sit amet) Tj
0 -20 Td
(Line 2: consectetur adipiscing elit) Tj
0 -20 Td
(Line 3: sed do eiusmod tempor incididunt) Tj
0 -20 Td
(Line 4: ut labore et dolore magna aliqua) Tj
ET
endstream
endobj
5 0 obj
<<
/Type /XObject
/Subtype /Image
/Width %d
/Height %d
/ColorSpace /DeviceRGB
/BitsPerComponent 8
/Length %d
>>
stream
%s
endstream
endobj
xref
0 6
0000000000 65535 f 
0000000009 00000 n 
0000000058 00000 n 
0000000115 00000 n 
0000000428 00000 n 
0000000928 00000 n 
trailer
<<
/Size 6
/Root 1 0 R
>>
startxref
1100
%%%%EOF
`, imageWidth, imageHeight, len(imageData), string(imageData))

	return pdfPath, os.WriteFile(pdfPath, []byte(content), 0644)
}

// MockPDFReader 用于测试的 mock PDF 读取器
type MockPDFReader struct {
	pageCount int
	pageInfos []MockPageInfo
	shouldErr bool
	errMsg    string
}

// MockPageInfo mock 页面信息
type MockPageInfo struct {
	Width  float64
	Height float64
}

// NewMockPDFReader 创建 mock PDF 读取器
func NewMockPDFReader(pageCount int) *MockPDFReader {
	pageInfos := make([]MockPageInfo, pageCount)
	for i := range pageInfos {
		pageInfos[i] = MockPageInfo{Width: 612, Height: 792} // Letter size
	}
	return &MockPDFReader{
		pageCount: pageCount,
		pageInfos: pageInfos,
	}
}

// SetError 设置错误状态（用于测试错误处理）
func (m *MockPDFReader) SetError(errMsg string) {
	m.shouldErr = true
	m.errMsg = errMsg
}

// GetPageCount 返回页数
func (m *MockPDFReader) GetPageCount() (int, error) {
	if m.shouldErr {
		return 0, fmt.Errorf("%s", m.errMsg)
	}
	return m.pageCount, nil
}

// GetPageInfo 返回页面信息
func (m *MockPDFReader) GetPageInfo(pageNum int) (MockPageInfo, error) {
	if m.shouldErr {
		return MockPageInfo{}, fmt.Errorf("%s", m.errMsg)
	}
	if pageNum < 1 || pageNum > m.pageCount {
		return MockPageInfo{}, fmt.Errorf("invalid page number: %d", pageNum)
	}
	return m.pageInfos[pageNum-1], nil
}
