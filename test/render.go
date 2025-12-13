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

// ExtractAdvancedFeaturesForReport 提取高级 PDF 功能信息用于报告
func ExtractAdvancedFeaturesForReport(pdfPath string, pageNum int) string {
	var report strings.Builder

	// 打开 PDF
	ctx, err := gopdf.ReadContextFile(pdfPath)
	if err != nil {
		report.WriteString(fmt.Sprintf("❌ Failed to read PDF: %v\n", err))
		return report.String()
	}

	// 获取页面字典
	pageDict, _, _, err := ctx.PageDict(pageNum, false)
	if err != nil {
		report.WriteString(fmt.Sprintf("❌ Failed to get page dict: %v\n", err))
		return report.String()
	}

	// 提取注释
	report.WriteString("Annotations:\n")
	report.WriteString("------------\n")
	annotations, err := gopdf.ExtractAnnotations(ctx, pageDict)
	if err != nil {
		report.WriteString(fmt.Sprintf("❌ Failed to extract annotations: %v\n", err))
	} else if len(annotations) == 0 {
		report.WriteString("No annotations found\n")
	} else {
		report.WriteString(fmt.Sprintf("Found %d annotation(s):\n", len(annotations)))
		for i, annot := range annotations {
			report.WriteString(fmt.Sprintf("  [%d] Type: %s\n", i+1, annot.Subtype))
			report.WriteString(fmt.Sprintf("      Rect: (%.2f, %.2f, %.2f, %.2f)\n",
				annot.Rect[0], annot.Rect[1], annot.Rect[2], annot.Rect[3]))
			if annot.Contents != "" {
				report.WriteString(fmt.Sprintf("      Contents: %s\n", annot.Contents))
			}
			if len(annot.Color) >= 3 {
				report.WriteString(fmt.Sprintf("      Color: RGB(%.2f, %.2f, %.2f)\n",
					annot.Color[0], annot.Color[1], annot.Color[2]))
			}
			report.WriteString(fmt.Sprintf("      Visible: %v, Printable: %v\n",
				annot.IsVisible(), annot.IsPrintable()))
		}
	}
	report.WriteString("\n")

	// 提取表单字段
	report.WriteString("Form Fields:\n")
	report.WriteString("------------\n")
	formFields, err := gopdf.ExtractFormFields(ctx)
	if err != nil {
		report.WriteString(fmt.Sprintf("❌ Failed to extract form fields: %v\n", err))
	} else if len(formFields) == 0 {
		report.WriteString("No form fields found\n")
	} else {
		report.WriteString(fmt.Sprintf("Found %d form field(s):\n", len(formFields)))
		for i, field := range formFields {
			report.WriteString(fmt.Sprintf("  [%d] Name: %s\n", i+1, field.FieldName))
			report.WriteString(fmt.Sprintf("      Type: %s\n", field.FieldType))
			if field.Value != "" {
				report.WriteString(fmt.Sprintf("      Value: %s\n", field.Value))
			}
			if field.DefaultValue != "" {
				report.WriteString(fmt.Sprintf("      Default: %s\n", field.DefaultValue))
			}
			if len(field.Rect) >= 4 {
				report.WriteString(fmt.Sprintf("      Rect: (%.2f, %.2f, %.2f, %.2f)\n",
					field.Rect[0], field.Rect[1], field.Rect[2], field.Rect[3]))
			}
			report.WriteString(fmt.Sprintf("      ReadOnly: %v, Required: %v\n",
				field.IsReadOnly(), field.IsRequired()))
			if field.IsCheckbox() {
				report.WriteString(fmt.Sprintf("      Checkbox - Checked: %v\n", field.IsChecked()))
			} else if field.IsRadioButton() {
				report.WriteString(fmt.Sprintf("      Radio Button - Selected: %v\n", field.IsChecked()))
			}
		}
	}
	report.WriteString("\n")

	// 检查透明度组、渐变、图案等
	report.WriteString("Advanced Graphics:\n")
	report.WriteString("------------------\n")

	// 加载资源
	resources := gopdf.NewResources()
	if resourcesObj, found := pageDict.Find("Resources"); found {
		if err := gopdf.LoadResourcesPublic(ctx, resourcesObj, resources); err == nil {
			// 检查渐变
			shadingCount := resources.CountShadings()
			if shadingCount > 0 {
				report.WriteString(fmt.Sprintf("✓ Found %d shading(s) (gradients)\n", shadingCount))
			}

			// 检查图案
			patternCount := resources.CountPatterns()
			if patternCount > 0 {
				report.WriteString(fmt.Sprintf("✓ Found %d pattern(s)\n", patternCount))
			}

			// 检查扩展图形状态（混合模式、透明度）
			extGStateCount := resources.CountExtGStates()
			if extGStateCount > 0 {
				report.WriteString(fmt.Sprintf("✓ Found %d extended graphics state(s) (blend modes/transparency)\n", extGStateCount))
			}

			// 检查 XObject 中的透明度组
			xobjects := resources.GetAllXObjects()
			transparencyGroupCount := 0
			for _, xobj := range xobjects {
				if xobj.Group != nil {
					transparencyGroupCount++
				}
			}
			if transparencyGroupCount > 0 {
				report.WriteString(fmt.Sprintf("✓ Found %d transparency group(s)\n", transparencyGroupCount))
			}

			if shadingCount == 0 && patternCount == 0 && extGStateCount == 0 && transparencyGroupCount == 0 {
				report.WriteString("No advanced graphics features detected\n")
			}
		} else {
			report.WriteString(fmt.Sprintf("⚠️  Failed to load resources: %v\n", err))
		}
	} else {
		report.WriteString("No resources found on page\n")
	}

	return report.String()
}
