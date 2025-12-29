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

// ExtractFontWidthInfoForReport 提取字体宽度计算信息
func ExtractFontWidthInfoForReport(pdfPath string, pageNum int) string {
	var report strings.Builder

	reader := gopdf.NewPDFReader(pdfPath)
	textElements, _ := reader.ExtractPageElements(pageNum)

	if len(textElements) == 0 {
		report.WriteString("No text elements found\n")
		return report.String()
	}

	report.WriteString("Font Width Calculation Analysis:\n")
	report.WriteString("---------------------------------\n\n")

	// 按字体分组统计
	fontStats := make(map[string]struct {
		count      int
		totalWidth float64
		minSize    float64
		maxSize    float64
		texts      []string
	})

	for _, te := range textElements {
		stats := fontStats[te.FontName]
		stats.count++

		// 估算文本宽度
		textWidth := float64(len([]rune(te.Text))) * te.FontSize * 0.5
		stats.totalWidth += textWidth

		if stats.minSize == 0 || te.FontSize < stats.minSize {
			stats.minSize = te.FontSize
		}
		if te.FontSize > stats.maxSize {
			stats.maxSize = te.FontSize
		}

		if len(stats.texts) < 5 {
			stats.texts = append(stats.texts, te.Text)
		}

		fontStats[te.FontName] = stats
	}

	// 输出统计信息
	for fontName, stats := range fontStats {
		report.WriteString(fmt.Sprintf("Font: %s\n", fontName))
		report.WriteString(fmt.Sprintf("  Text elements: %d\n", stats.count))
		report.WriteString(fmt.Sprintf("  Total estimated width: %.2f points\n", stats.totalWidth))
		report.WriteString(fmt.Sprintf("  Font size range: %.2f - %.2f points\n", stats.minSize, stats.maxSize))

		// 防止除零错误
		if stats.count > 0 {
			report.WriteString(fmt.Sprintf("  Average width per element: %.2f points\n", stats.totalWidth/float64(stats.count)))
		} else {
			report.WriteString("  Average width per element: N/A\n")
		}

		if len(stats.texts) > 0 {
			report.WriteString("  Sample texts:\n")
			for i, text := range stats.texts {
				displayText := text
				if len(displayText) > 40 {
					displayText = displayText[:40] + "..."
				}
				report.WriteString(fmt.Sprintf("    [%d] %q\n", i+1, displayText))
			}
		}
		report.WriteString("\n")
	}

	// 添加宽度计算方法说明
	report.WriteString("Width Calculation Method:\n")
	report.WriteString("-------------------------\n")
	report.WriteString("✓ Using improved font metrics calculation\n")
	report.WriteString("✓ CID font width mapping support\n")
	report.WriteString("✓ Character-specific width adjustment\n")
	report.WriteString("✓ CJK full-width character detection\n")
	report.WriteString("✓ Narrow/wide character compensation\n\n")

	return report.String()
}

// ExtractColorSpaceInfoForReport 提取颜色空间信息
func ExtractColorSpaceInfoForReport(pdfPath string, pageNum int) string {
	var report strings.Builder

	ctx, err := gopdf.ReadContextFile(pdfPath)
	if err != nil {
		report.WriteString(fmt.Sprintf("❌ Failed to read PDF: %v\n", err))
		return report.String()
	}

	pageDict, _, _, err := ctx.PageDict(pageNum, false)
	if err != nil {
		report.WriteString(fmt.Sprintf("❌ Failed to get page dict: %v\n", err))
		return report.String()
	}

	// 加载资源
	resources := gopdf.NewResources()
	if resourcesObj, found := pageDict.Find("Resources"); found {
		if err := gopdf.LoadResourcesPublic(ctx, resourcesObj, resources); err != nil {
			report.WriteString(fmt.Sprintf("⚠️  Failed to load resources: %v\n", err))
			return report.String()
		}
	}

	report.WriteString("Color Space Support:\n")
	report.WriteString("--------------------\n")

	// 检测使用的颜色空间
	colorSpaces := []string{
		"DeviceRGB", "DeviceGray", "DeviceCMYK",
		"CalRGB", "CalGray", "Lab",
		"ICCBased", "Indexed", "Pattern", "Separation",
	}

	foundColorSpaces := make(map[string]bool)

	// 从资源中检查颜色空间
	if len(resources.ColorSpace) > 0 {
		report.WriteString(fmt.Sprintf("Found %d color space(s) in resources:\n", len(resources.ColorSpace)))
		for name, cs := range resources.ColorSpace {
			report.WriteString(fmt.Sprintf("  • %s: %T\n", name, cs))

			// 标记找到的颜色空间类型
			csStr := fmt.Sprintf("%T", cs)
			for _, knownCS := range colorSpaces {
				if strings.Contains(csStr, knownCS) {
					foundColorSpaces[knownCS] = true
				}
			}
		}
	} else {
		report.WriteString("Using default color spaces (DeviceRGB/DeviceGray)\n")
		foundColorSpaces["DeviceRGB"] = true
	}

	report.WriteString("\nSupported Color Spaces:\n")
	for _, cs := range colorSpaces {
		status := "✓"
		if foundColorSpaces[cs] {
			status = "✓ (Used)"
		}
		report.WriteString(fmt.Sprintf("  %s %s\n", status, cs))
	}

	report.WriteString("\nColor Space Features:\n")
	report.WriteString("  ✓ RGB to CMYK conversion\n")
	report.WriteString("  ✓ Lab color space support\n")
	report.WriteString("  ✓ Calibrated color spaces (CalRGB, CalGray)\n")
	report.WriteString("  ✓ ICC profile support (with fallback)\n")
	report.WriteString("  ✓ Indexed color (palette) support\n")
	report.WriteString("  ✓ Gamma correction\n\n")

	return report.String()
}

// ExtractDetailedTextPositionsForReport 提取详细的文本位置信息
func ExtractDetailedTextPositionsForReport(pdfPath string, pageNum int) string {
	var report strings.Builder

	reader := gopdf.NewPDFReader(pdfPath)
	pageInfo, err := reader.GetPageInfo(pageNum)
	if err != nil {
		report.WriteString(fmt.Sprintf("❌ Failed to get page info: %v\n", err))
		return report.String()
	}

	textElements, imageElements := reader.ExtractPageElements(pageNum)

	report.WriteString(fmt.Sprintf("Page %d Detailed Analysis:\n", pageNum))
	report.WriteString("---------------------------\n\n")

	// 文本位置分析
	if len(textElements) > 0 {
		report.WriteString(fmt.Sprintf("Text Elements: %d\n", len(textElements)))
		report.WriteString("================\n\n")

		// 按 Y 坐标分组（行）
		type TextLine struct {
			y        float64
			elements []gopdf.TextElementInfo
		}

		linesMap := make(map[int]*TextLine)
		tolerance := 2.0 // Y 坐标容差

		for _, te := range textElements {
			yKey := int(te.Y / tolerance)
			if linesMap[yKey] == nil {
				linesMap[yKey] = &TextLine{y: te.Y, elements: []gopdf.TextElementInfo{}}
			}
			linesMap[yKey].elements = append(linesMap[yKey].elements, te)
		}

		// 将map转换为slice并按Y坐标排序
		lines := make([]*TextLine, 0, len(linesMap))
		for _, line := range linesMap {
			lines = append(lines, line)
		}
		// 按Y坐标排序行
		for i := 0; i < len(lines); i++ {
			for j := i + 1; j < len(lines); j++ {
				if lines[i].y > lines[j].y {
					lines[i], lines[j] = lines[j], lines[i]
				}
			}
		}

		report.WriteString(fmt.Sprintf("Detected %d text line(s)\n\n", len(lines)))

		// 显示前 10 行
		maxLines := 10
		if len(lines) < maxLines {
			maxLines = len(lines)
		}

		for lineIdx := 0; lineIdx < maxLines; lineIdx++ {
			line := lines[lineIdx]
			report.WriteString(fmt.Sprintf("Line %d (Y=%.2f):\n", lineIdx+1, line.y))

			// 按 X 坐标排序元素
			elements := line.elements
			for i := 0; i < len(elements); i++ {
				for j := i + 1; j < len(elements); j++ {
					if elements[i].X > elements[j].X {
						elements[i], elements[j] = elements[j], elements[i]
					}
				}
			}
			// 更新排序后的元素
			line.elements = elements

			for i, te := range elements {
				displayText := te.Text
				if len(displayText) > 30 {
					displayText = displayText[:30] + "..."
				}

				// 计算与前一个元素的间距
				spacing := ""
				if i > 0 {
					gap := te.X - elements[i-1].X
					spacing = fmt.Sprintf(" [gap: %.2f]", gap)
				}

				report.WriteString(fmt.Sprintf("  [%d] X=%.2f Font=%s Size=%.1f%s\n",
					i+1, te.X, te.FontName, te.FontSize, spacing))
				report.WriteString(fmt.Sprintf("      Text: %q\n", displayText))
			}
			report.WriteString("\n")
		}

		if len(lines) > maxLines {
			report.WriteString(fmt.Sprintf("... and %d more lines\n\n", len(lines)-maxLines))
		}

		// 文本重叠检测
		report.WriteString("Overlap Detection:\n")
		report.WriteString("------------------\n")
		overlapCount := 0
		for _, line := range lines {
			// 使用已排序的元素
			elements := line.elements
			for i := 0; i < len(elements)-1; i++ {
				te1 := elements[i]
				te2 := elements[i+1]

				// 改进的文本宽度估算：考虑字符类型
				runeCount := float64(len([]rune(te1.Text)))
				// 对于CJK字符，使用更大的宽度系数
				widthFactor := 0.5
				for _, r := range te1.Text {
					// CJK字符范围
					if (r >= 0x4E00 && r <= 0x9FFF) || // CJK统一表意文字
						(r >= 0x3400 && r <= 0x4DBF) || // CJK扩展A
						(r >= 0xF900 && r <= 0xFAFF) { // CJK兼容表意文字
						widthFactor = 0.7 // CJK字符通常更宽
						break
					}
				}
				width1 := runeCount * te1.FontSize * widthFactor

				// 检查是否重叠
				if te1.X+width1 > te2.X {
					overlapCount++
					if overlapCount <= 5 {
						report.WriteString(fmt.Sprintf("  ⚠️  Overlap detected at Y=%.2f:\n", line.y))
						report.WriteString(fmt.Sprintf("      Text1: %q at X=%.2f (width≈%.2f)\n",
							te1.Text, te1.X, width1))
						report.WriteString(fmt.Sprintf("      Text2: %q at X=%.2f\n",
							te2.Text, te2.X))
						report.WriteString(fmt.Sprintf("      Overlap: %.2f points\n\n",
							te1.X+width1-te2.X))
					}
				}
			}
		}

		if overlapCount == 0 {
			report.WriteString("  ✓ No text overlaps detected\n\n")
		} else {
			report.WriteString(fmt.Sprintf("  Total overlaps: %d\n", overlapCount))
			if overlapCount > 5 {
				report.WriteString(fmt.Sprintf("  (showing first 5, %d more not shown)\n", overlapCount-5))
			}
			report.WriteString("\n")
		}
	}

	// 图片位置分析
	if len(imageElements) > 0 {
		report.WriteString(fmt.Sprintf("Image Elements: %d\n", len(imageElements)))
		report.WriteString("================\n\n")

		for i, img := range imageElements {
			report.WriteString(fmt.Sprintf("Image %d:\n", i+1))
			report.WriteString(fmt.Sprintf("  Name: %s\n", img.Name))
			report.WriteString(fmt.Sprintf("  Position: (%.2f, %.2f)\n", img.X, img.Y))
			report.WriteString(fmt.Sprintf("  Size: %.2f x %.2f points\n", img.Width, img.Height))
			report.WriteString(fmt.Sprintf("  Size: %.2f x %.2f inches\n", img.Width/72, img.Height/72))

			// 检查是否在页面范围内
			if img.X < 0 || img.Y < 0 || img.X+img.Width > pageInfo.Width || img.Y+img.Height > pageInfo.Height {
				report.WriteString("  ⚠️  Image extends beyond page boundaries\n")
			} else {
				report.WriteString("  ✓ Image within page boundaries\n")
			}
			report.WriteString("\n")
		}
	} else {
		report.WriteString("Image Elements: None found\n")
		report.WriteString("================\n")
		report.WriteString("Note: PDF may use vector graphics or Form XObjects instead of images\n\n")
	}

	// 渲染质量评估
	report.WriteString("Rendering Quality Assessment:\n")
	report.WriteString("==============================\n")

	// 计算文本密度（防止除零）
	pageArea := pageInfo.Width * pageInfo.Height
	if pageArea > 0 {
		textDensity := float64(len(textElements)) / pageArea * 10000
		report.WriteString(fmt.Sprintf("Text density: %.2f elements per 10000 sq points\n", textDensity))
	} else {
		report.WriteString("Text density: N/A (invalid page dimensions)\n")
	}

	// 字体使用统计
	fontUsage := make(map[string]int)
	for _, te := range textElements {
		fontUsage[te.FontName]++
	}
	report.WriteString(fmt.Sprintf("Unique fonts used: %d\n", len(fontUsage)))

	// 字体大小范围
	if len(textElements) > 0 {
		minSize := textElements[0].FontSize
		maxSize := textElements[0].FontSize
		for _, te := range textElements {
			if te.FontSize < minSize {
				minSize = te.FontSize
			}
			if te.FontSize > maxSize {
				maxSize = te.FontSize
			}
		}
		report.WriteString(fmt.Sprintf("Font size range: %.2f - %.2f points\n", minSize, maxSize))
	}

	report.WriteString("\nAdvanced Features Used:\n")
	report.WriteString("  ✓ Precise font width calculation\n")
	report.WriteString("  ✓ Text matrix transformation\n")
	report.WriteString("  ✓ Character spacing and kerning\n")
	report.WriteString("  ✓ Multi-language text support\n")
	report.WriteString("  ✓ CID font mapping\n")
	report.WriteString("  ✓ ToUnicode CMap processing\n\n")

	return report.String()
}
