package test

import (
	"fmt"
	"strings"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// TextElement 文本元素信息
type TextElement struct {
	Text     string
	X        float64
	Y        float64
	FontName string
	FontSize float64
}

// ImageElement 图片元素信息
type ImageElement struct {
	Name   string
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// RenderResult 渲染结果
type RenderResult struct {
	Error        error
	DebugInfo    string
	TextElements []TextElement
	Images       []ImageElement
	PageWidth    float64
	PageHeight   float64
}

// RenderTestVectorPDF 渲染 test_vector.pdf 并返回调试信息
func RenderTestVectorPDF(pdfPath, outputPath string) RenderResult {
	var debugInfo strings.Builder

	debugInfo.WriteString("Starting PDF rendering...\n")
	debugInfo.WriteString(fmt.Sprintf("Input: %s\n", pdfPath))
	debugInfo.WriteString(fmt.Sprintf("Output: %s\n", outputPath))

	// 创建 PDF 读取器
	reader := gopdf.NewPDFReader(pdfPath)
	debugInfo.WriteString("PDF reader created\n")

	// 获取页面信息
	pageInfo, err := reader.GetPageInfo(1)
	if err != nil {
		debugInfo.WriteString(fmt.Sprintf("Failed to get page info: %v\n", err))
	}

	// 提取文本和图片信息
	textElements, images := reader.ExtractPageElements(1)

	// 渲染第一页，DPI 150
	debugInfo.WriteString("Rendering page 1 at 150 DPI...\n")

	err = reader.RenderPageToPNG(1, outputPath, 150)

	if err != nil {
		debugInfo.WriteString(fmt.Sprintf("Rendering failed: %v\n", err))
		return RenderResult{
			Error:     err,
			DebugInfo: debugInfo.String(),
		}
	}

	debugInfo.WriteString("Rendering completed successfully\n")

	// 转换提取的元素
	var resultTexts []TextElement
	for _, te := range textElements {
		resultTexts = append(resultTexts, TextElement{
			Text:     te.Text,
			X:        te.X,
			Y:        te.Y,
			FontName: te.FontName,
			FontSize: te.FontSize,
		})
	}

	var resultImages []ImageElement
	for _, img := range images {
		resultImages = append(resultImages, ImageElement{
			Name:   img.Name,
			X:      img.X,
			Y:      img.Y,
			Width:  img.Width,
			Height: img.Height,
		})
	}

	return RenderResult{
		Error:        nil,
		DebugInfo:    debugInfo.String(),
		TextElements: resultTexts,
		Images:       resultImages,
		PageWidth:    pageInfo.Width,
		PageHeight:   pageInfo.Height,
	}
}

// ExtractPageElementsForReport 提取页面元素并返回格式化的报告（供 render_pdf 使用）
func ExtractPageElementsForReport(pdfPath string, pageNum int) string {
	reader := gopdf.NewPDFReader(pdfPath)

	// 获取页面信息
	pageInfo, err := reader.GetPageInfo(pageNum)
	if err != nil {
		return fmt.Sprintf("Failed to get page info: %v\n", err)
	}

	// 提取元素
	textElements, images := reader.ExtractPageElements(pageNum)

	var report string
	report += fmt.Sprintf("Page %d Element Extraction:\n", pageNum)
	report += "============================\n\n"

	report += fmt.Sprintf("Page Size: %.2f x %.2f points (%.2f x %.2f inches)\n\n",
		pageInfo.Width, pageInfo.Height,
		pageInfo.Width/72, pageInfo.Height/72)

	// 文本元素
	if len(textElements) > 0 {
		report += fmt.Sprintf("Text Elements: %d\n", len(textElements))
		report += "----------------\n"

		// 统计超出页面边界的元素
		outOfBoundsCount := 0
		maxX := 0.0
		maxY := 0.0
		for _, te := range textElements {
			if te.X > pageInfo.Width || te.Y > pageInfo.Height || te.X < 0 || te.Y < 0 {
				outOfBoundsCount++
			}
			if te.X > maxX {
				maxX = te.X
			}
			if te.Y > maxY {
				maxY = te.Y
			}
		}

		report += fmt.Sprintf("⚠️  Elements out of page bounds: %d (%.1f%%)\n",
			outOfBoundsCount, float64(outOfBoundsCount)/float64(len(textElements))*100)
		report += fmt.Sprintf("📏 Max X coordinate: %.2f (page width: %.2f)\n", maxX, pageInfo.Width)
		report += fmt.Sprintf("📏 Max Y coordinate: %.2f (page height: %.2f)\n\n", maxY, pageInfo.Height)

		maxDisplay := 20
		if len(textElements) < maxDisplay {
			maxDisplay = len(textElements)
		}

		report += "First 20 elements:\n"
		for i := 0; i < maxDisplay; i++ {
			te := textElements[i]
			outOfBounds := ""
			if te.X > pageInfo.Width || te.Y > pageInfo.Height || te.X < 0 || te.Y < 0 {
				outOfBounds = " ⚠️ OUT OF BOUNDS"
			}
			report += fmt.Sprintf("[%d] Position: (%.2f, %.2f)%s\n", i+1, te.X, te.Y, outOfBounds)
			report += fmt.Sprintf("    Font: %s, Size: %.2f\n", te.FontName, te.FontSize)

			displayText := te.Text
			if len(displayText) > 80 {
				displayText = displayText[:80] + "..."
			}
			report += fmt.Sprintf("    Text: %q\n\n", displayText)
		}

		if len(textElements) > maxDisplay {
			report += fmt.Sprintf("... and %d more text elements\n\n", len(textElements)-maxDisplay)

			// 显示一些超出边界的元素示例
			report += "Sample of out-of-bounds elements:\n"
			outOfBoundsSamples := 0
			for i, te := range textElements {
				if te.X > pageInfo.Width || te.Y > pageInfo.Height {
					report += fmt.Sprintf("[%d] Position: (%.2f, %.2f) ⚠️ OUT OF BOUNDS\n", i+1, te.X, te.Y)
					report += fmt.Sprintf("    Font: %s, Size: %.2f\n", te.FontName, te.FontSize)
					displayText := te.Text
					if len(displayText) > 80 {
						displayText = displayText[:80] + "..."
					}
					report += fmt.Sprintf("    Text: %q\n\n", displayText)
					outOfBoundsSamples++
					if outOfBoundsSamples >= 10 {
						break
					}
				}
			}
			if outOfBoundsSamples == 0 {
				report += "  (No out-of-bounds elements found)\n\n"
			}
		}
	} else {
		report += "Text Elements: None found\n\n"
	}

	// 图片元素
	if len(images) > 0 {
		report += fmt.Sprintf("Image Elements: %d\n", len(images))
		report += "-----------------\n"

		for i, img := range images {
			report += fmt.Sprintf("[%d] Name: %s\n", i+1, img.Name)
			report += fmt.Sprintf("    Position: (%.2f, %.2f)\n", img.X, img.Y)
			report += fmt.Sprintf("    Size: %.2f x %.2f\n\n", img.Width, img.Height)
		}
	} else {
		report += "Image Elements: None found\n\n"
	}

	return report
}

// ExtractFontInfoForReport 提取字体信息并返回格式化的报告
func ExtractFontInfoForReport(pdfPath string, pageNum int) string {
	var report strings.Builder

	report.WriteString("Font Information:\n")
	report.WriteString("=================\n\n")

	// 使用 gopdf 内部 API 提取字体信息
	reader := gopdf.NewPDFReader(pdfPath)
	fontInfo := reader.ExtractFontInfo(pageNum)

	if len(fontInfo) == 0 {
		report.WriteString("No fonts found\n\n")
		return report.String()
	}

	for i, font := range fontInfo {
		report.WriteString(fmt.Sprintf("[Font %d] %s\n", i+1, font.Name))
		report.WriteString(fmt.Sprintf("  BaseFont: %s\n", font.BaseFont))
		report.WriteString(fmt.Sprintf("  Subtype: %s\n", font.Subtype))
		report.WriteString(fmt.Sprintf("  Encoding: %s\n", font.Encoding))

		if font.IsIdentity {
			report.WriteString("  Identity Mapping: YES\n")
		} else {
			report.WriteString("  Identity Mapping: NO\n")
		}

		if font.HasToUnicode {
			report.WriteString(fmt.Sprintf("  ToUnicode Map: YES (%d mappings, %d ranges)\n",
				font.ToUnicodeMappings, font.ToUnicodeRanges))
		} else {
			report.WriteString("  ToUnicode Map: NO\n")
		}

		if font.CIDSystemInfo != "" {
			report.WriteString(fmt.Sprintf("  CID System Info: %s\n", font.CIDSystemInfo))
		}

		if font.EmbeddedFontSize > 0 {
			report.WriteString(fmt.Sprintf("  Embedded Font: YES (%d bytes)\n", font.EmbeddedFontSize))
		} else {
			report.WriteString("  Embedded Font: NO\n")
		}

		report.WriteString("\n")
	}

	return report.String()
}
