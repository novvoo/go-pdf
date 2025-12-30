//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/novvoo/go-pdf/pkg/gopdf"
	"github.com/novvoo/go-pdf/test"
)

func main() {
	// 固定使用 test_vector.pdf
	pdfPath := "example/test_vector.pdf"
	outputPath := "example/test_vector.png"
	reportPath := "example/render_vector.txt"

	// 立即重定向所有输出到缓冲区，确保终端完全静默
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	r, w, _ := os.Pipe()
	os.Stdout = w
	os.Stderr = w

	// 创建缓冲区捕获 gopdf 的调试输出
	var debugBuf bytes.Buffer
	gopdf.SetDebugOutput(&debugBuf)

	// 在后台读取输出
	outputChan := make(chan string)
	go func() {
		var buf bytes.Buffer
		io.Copy(&buf, r)
		outputChan <- buf.String()
	}()

	var report string
	report += "PDF Rendering Report\n"
	report += "====================\n"
	report += fmt.Sprintf("Time: %s\n\n", time.Now().Format("2006-01-02 15:04:05"))

	// 测试 parseTokens 函数
	report += "ParseTokens Test:\n"
	report += "-----------------\n"
	testTokens := []string{"q", "1", "0", "0", "1", "100", "200", "cm", "Q"}
	if ops, err := gopdf.ParseTokens(testTokens); err == nil {
		report += fmt.Sprintf("✅ ParseTokens test passed: %d operators parsed\n", len(ops))
		for i, op := range ops {
			report += fmt.Sprintf("  [%d] %s\n", i+1, op.Name())
		}
	} else {
		report += fmt.Sprintf("❌ ParseTokens test failed: %v\n", err)
	}
	report += "\n"

	// 检查 PDF 文件是否存在
	if !fileExists(pdfPath) {
		report += fmt.Sprintf("❌ Error: PDF file not found: %s\n", pdfPath)
		w.Close()
		os.Stdout = oldStdout
		os.Stderr = oldStderr
		<-outputChan
		writeReport(reportPath, report)
		return
	}

	report += fmt.Sprintf("📄 Input PDF: %s\n", pdfPath)
	report += fmt.Sprintf("📁 Output PNG: %s\n\n", outputPath)

	// 使用测试模块进行渲染调试
	report += "Rendering Process:\n"
	report += "------------------\n"

	// 执行渲染
	result := test.RenderTestVectorPDF(pdfPath, outputPath)

	// 恢复标准输出和标准错误
	w.Close()
	os.Stdout = oldStdout
	os.Stderr = oldStderr

	// 获取捕获的输出
	capturedOutput := <-outputChan

	if result.Error != nil {
		report += fmt.Sprintf("❌ Rendering failed: %v\n", result.Error)
		report += fmt.Sprintf("\nDebug Info:\n%s\n", result.DebugInfo)
		if capturedOutput != "" {
			report += fmt.Sprintf("\nCaptured Output:\n%s\n", capturedOutput)
		}
		writeReport(reportPath, report)
		return
	}

	report += "✅ PDF rendered successfully\n"
	report += fmt.Sprintf("✅ Output saved to: %s\n\n", outputPath)

	// 在图片上添加标准区域标记和图片元素标记
	if err := addStandardRegions(outputPath, result.PageWidth, result.PageHeight, result.Images, 150); err != nil {
		report += fmt.Sprintf("⚠️  Failed to add standard regions: %v\n", err)
	} else {
		report += "✅ Standard regions added to image\n"
		if len(result.Images) > 0 {
			report += fmt.Sprintf("✅ Marked %d image element(s) in the output\n", len(result.Images))
		}
	}

	// 获取输出文件信息
	if fileInfo, err := os.Stat(outputPath); err == nil {
		report += "Output File Info:\n"
		report += "-----------------\n"
		report += fmt.Sprintf("Size: %d bytes\n", fileInfo.Size())
		report += fmt.Sprintf("Created: %s\n\n", fileInfo.ModTime().Format("2006-01-02 15:04:05"))
	}

	// 添加页面信息
	report += "Page Information:\n"
	report += "-----------------\n"
	report += fmt.Sprintf("Page Size: %.2f x %.2f points\n", result.PageWidth, result.PageHeight)
	report += fmt.Sprintf("Page Size: %.2f x %.2f inches\n\n", result.PageWidth/72, result.PageHeight/72)

	// 添加字体信息
	report += "\n"
	fontReport := test.ExtractFontInfoForReport(pdfPath, 1)
	report += fontReport

	// 添加 ExtractPageElements 测试结果
	report += "\n"
	report += "ExtractPageElements Test:\n"
	report += "=========================\n"
	extractReport := test.ExtractPageElementsForReport(pdfPath, 1)
	report += extractReport

	// 添加高级 PDF 功能信息
	report += "\n"
	report += "Advanced PDF Features:\n"
	report += "======================\n"
	advancedReport := test.ExtractAdvancedFeaturesForReport(pdfPath, 1)
	report += advancedReport

	// 添加字体宽度计算信息
	report += "\n"
	report += "Font Width Calculation:\n"
	report += "=======================\n"
	fontWidthReport := test.ExtractFontWidthInfoForReport(pdfPath, 1)
	report += fontWidthReport

	// 添加颜色空间信息
	report += "\n"
	report += "Color Space Analysis:\n"
	report += "=====================\n"
	colorSpaceReport := test.ExtractColorSpaceInfoForReport(pdfPath, 1)
	report += colorSpaceReport

	// 添加详细的文本位置信息
	report += "\n"
	report += "Detailed Text Positioning:\n"
	report += "==========================\n"
	textPosReport := test.ExtractDetailedTextPositionsForReport(pdfPath, 1)
	report += textPosReport

	// 添加调试信息
	if result.DebugInfo != "" {
		report += "Debug Information:\n"
		report += "------------------\n"
		report += result.DebugInfo + "\n\n"
	}

	// 添加 gopdf 的调试输出（操作符执行信息）
	debugOutput := debugBuf.String()
	if debugOutput != "" {
		report += "Operator Execution Log:\n"
		report += "------------------------\n"
		report += debugOutput + "\n"
	}

	// 添加捕获的输出（包括 C 库的 DEBUG 信息）
	if capturedOutput != "" {
		report += "Gopdf/Pango Debug Output:\n"
		report += "-------------------------\n"
		report += capturedOutput + "\n\n"
	}

	report += "Status: SUCCESS\n"

	// 写入报告
	writeReport(reportPath, report)
}

// fileExists 检查文件是否存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// writeReport 写入报告文件（静默模式，不输出任何信息）
func writeReport(path string, content string) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		// 静默失败，不输出任何信息
		return
	}

	// 静默写入，不输出任何信息
	os.WriteFile(path, []byte(content), 0644)
}

// addStandardRegions 在渲染的图片上添加标准区域标记和图片元素标记
func addStandardRegions(imagePath string, pageWidth, pageHeight float64, imageElements []test.ImageElement, dpi int) error {
	// 读取图片
	file, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		return fmt.Errorf("failed to decode PNG: %w", err)
	}
	file.Close()

	// 创建可绘制的图片
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	draw.Draw(rgba, bounds, img, bounds.Min, draw.Src)

	// 计算缩放比例（PDF points 到像素）
	scale := float64(dpi) / 72.0

	// 定义标准区域（以 PDF points 为单位，PDF 坐标系原点在左下角）
	// 需要转换为图片坐标系（原点在左上角）
	regions := []struct {
		name   string
		x, y   float64 // PDF 坐标（左下角为原点）
		width  float64
		height float64
		color  color.RGBA
	}{
		// 页边距区域（假设 0.5 英寸 = 36 points）
		{"Top Margin", 0, pageHeight - 36, pageWidth, 36, color.RGBA{255, 0, 0, 255}},         // 红色
		{"Bottom Margin", 0, 0, pageWidth, 36, color.RGBA{255, 0, 0, 255}},                    // 红色
		{"Left Margin", 0, 36, 36, pageHeight - 72, color.RGBA{0, 255, 0, 255}},               // 绿色
		{"Right Margin", pageWidth - 36, 36, 36, pageHeight - 72, color.RGBA{0, 255, 0, 255}}, // 绿色

		// 内容区域
		{"Content Area", 36, 36, pageWidth - 72, pageHeight - 72, color.RGBA{0, 0, 255, 255}}, // 蓝色
	}

	// 绘制区域边框
	for _, region := range regions {
		// PDF 坐标转换为图片坐标
		// PDF: (x, y) 其中 y 是从底部开始
		// 图片: (x, imgHeight - y - height) 其中 y 是从顶部开始
		x1 := int(region.x * scale)
		y1 := int((pageHeight - region.y - region.height) * scale)
		x2 := int((region.x + region.width) * scale)
		y2 := int((pageHeight - region.y) * scale)

		// 绘制矩形边框（3像素宽）
		drawRect(rgba, x1, y1, x2, y2, region.color, 3)
	}

	// 绘制 PDF 中的图片元素边框
	if len(imageElements) > 0 {
		imageColor := color.RGBA{255, 0, 255, 255} // 洋红色（Magenta）
		for i, imgElem := range imageElements {
			// PDF 坐标转换为图片坐标
			x1 := int(imgElem.X * scale)
			y1 := int((pageHeight - imgElem.Y - imgElem.Height) * scale)
			x2 := int((imgElem.X + imgElem.Width) * scale)
			y2 := int((pageHeight - imgElem.Y) * scale)

			// 绘制图片边框（4像素宽，更醒目）
			drawRect(rgba, x1, y1, x2, y2, imageColor, 4)

			// 在图片左上角绘制编号标记
			drawImageLabel(rgba, x1, y1, i+1, imageColor)
		}
	}

	// 在四个角绘制十字标记
	crossSize := 20
	crossColor := color.RGBA{255, 255, 0, 255} // 黄色
	corners := []struct{ x, y int }{
		{0, 0},                               // 左上
		{bounds.Max.X - 1, 0},                // 右上
		{0, bounds.Max.Y - 1},                // 左下
		{bounds.Max.X - 1, bounds.Max.Y - 1}, // 右下
	}

	for _, corner := range corners {
		drawCross(rgba, corner.x, corner.y, crossSize, crossColor, 2)
	}

	// 保存修改后的图片
	outFile, err := os.Create(imagePath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	if err := png.Encode(outFile, rgba); err != nil {
		return fmt.Errorf("failed to encode PNG: %w", err)
	}

	return nil
}

// drawRect 绘制矩形边框
func drawRect(img *image.RGBA, x1, y1, x2, y2 int, col color.RGBA, thickness int) {
	// 确保坐标在图片范围内
	bounds := img.Bounds()
	if x1 < bounds.Min.X {
		x1 = bounds.Min.X
	}
	if y1 < bounds.Min.Y {
		y1 = bounds.Min.Y
	}
	if x2 > bounds.Max.X {
		x2 = bounds.Max.X
	}
	if y2 > bounds.Max.Y {
		y2 = bounds.Max.Y
	}

	// 绘制四条边
	for t := 0; t < thickness; t++ {
		// 上边
		for x := x1; x < x2; x++ {
			if y1+t < bounds.Max.Y {
				img.Set(x, y1+t, col)
			}
		}
		// 下边
		for x := x1; x < x2; x++ {
			if y2-t-1 >= bounds.Min.Y {
				img.Set(x, y2-t-1, col)
			}
		}
		// 左边
		for y := y1; y < y2; y++ {
			if x1+t < bounds.Max.X {
				img.Set(x1+t, y, col)
			}
		}
		// 右边
		for y := y1; y < y2; y++ {
			if x2-t-1 >= bounds.Min.X {
				img.Set(x2-t-1, y, col)
			}
		}
	}
}

// drawCross 绘制十字标记
func drawCross(img *image.RGBA, cx, cy, size int, col color.RGBA, thickness int) {
	bounds := img.Bounds()

	// 绘制水平线
	for t := 0; t < thickness; t++ {
		for x := cx - size; x <= cx+size; x++ {
			if x >= bounds.Min.X && x < bounds.Max.X && cy+t >= bounds.Min.Y && cy+t < bounds.Max.Y {
				img.Set(x, cy+t, col)
			}
		}
	}

	// 绘制垂直线
	for t := 0; t < thickness; t++ {
		for y := cy - size; y <= cy+size; y++ {
			if y >= bounds.Min.Y && y < bounds.Max.Y && cx+t >= bounds.Min.X && cx+t < bounds.Max.X {
				img.Set(cx+t, y, col)
			}
		}
	}
}

// drawImageLabel 在图片元素位置绘制编号标记
func drawImageLabel(img *image.RGBA, x, y, number int, col color.RGBA) {
	bounds := img.Bounds()

	// 绘制一个小方块作为标签背景
	labelSize := 30
	bgColor := color.RGBA{0, 0, 0, 200} // 半透明黑色背景

	// 绘制背景方块
	for dy := 0; dy < labelSize; dy++ {
		for dx := 0; dx < labelSize; dx++ {
			px := x + dx
			py := y + dy
			if px >= bounds.Min.X && px < bounds.Max.X && py >= bounds.Min.Y && py < bounds.Max.Y {
				img.Set(px, py, bgColor)
			}
		}
	}

	// 绘制边框
	for i := 0; i < 2; i++ {
		// 上边
		for dx := 0; dx < labelSize; dx++ {
			if x+dx >= bounds.Min.X && x+dx < bounds.Max.X && y+i >= bounds.Min.Y && y+i < bounds.Max.Y {
				img.Set(x+dx, y+i, col)
			}
		}
		// 下边
		for dx := 0; dx < labelSize; dx++ {
			if x+dx >= bounds.Min.X && x+dx < bounds.Max.X && y+labelSize-1-i >= bounds.Min.Y && y+labelSize-1-i < bounds.Max.Y {
				img.Set(x+dx, y+labelSize-1-i, col)
			}
		}
		// 左边
		for dy := 0; dy < labelSize; dy++ {
			if x+i >= bounds.Min.X && x+i < bounds.Max.X && y+dy >= bounds.Min.Y && y+dy < bounds.Max.Y {
				img.Set(x+i, y+dy, col)
			}
		}
		// 右边
		for dy := 0; dy < labelSize; dy++ {
			if x+labelSize-1-i >= bounds.Min.X && x+labelSize-1-i < bounds.Max.X && y+dy >= bounds.Min.Y && y+dy < bounds.Max.Y {
				img.Set(x+labelSize-1-i, y+dy, col)
			}
		}
	}

	// 绘制简单的数字（使用像素点阵）
	drawSimpleNumber(img, x+8, y+8, number, col)
}

// drawSimpleNumber 绘制简单的数字（1-9）
func drawSimpleNumber(img *image.RGBA, x, y, number int, digitColor color.RGBA) {
	bounds := img.Bounds()

	// 简单的 3x5 点阵数字
	digits := map[int][][]bool{
		1: {
			{false, true, false},
			{true, true, false},
			{false, true, false},
			{false, true, false},
			{true, true, true},
		},
		2: {
			{true, true, true},
			{false, false, true},
			{true, true, true},
			{true, false, false},
			{true, true, true},
		},
		3: {
			{true, true, true},
			{false, false, true},
			{true, true, true},
			{false, false, true},
			{true, true, true},
		},
		4: {
			{true, false, true},
			{true, false, true},
			{true, true, true},
			{false, false, true},
			{false, false, true},
		},
		5: {
			{true, true, true},
			{true, false, false},
			{true, true, true},
			{false, false, true},
			{true, true, true},
		},
		6: {
			{true, true, true},
			{true, false, false},
			{true, true, true},
			{true, false, true},
			{true, true, true},
		},
		7: {
			{true, true, true},
			{false, false, true},
			{false, true, false},
			{false, true, false},
			{false, true, false},
		},
		8: {
			{true, true, true},
			{true, false, true},
			{true, true, true},
			{true, false, true},
			{true, true, true},
		},
		9: {
			{true, true, true},
			{true, false, true},
			{true, true, true},
			{false, false, true},
			{true, true, true},
		},
	}

	pattern, ok := digits[number]
	if !ok || number < 1 || number > 9 {
		return
	}

	// 绘制数字（每个点放大2x2像素）
	scale := 2
	for row := 0; row < len(pattern); row++ {
		for col := 0; col < len(pattern[row]); col++ {
			if pattern[row][col] {
				for dy := 0; dy < scale; dy++ {
					for dx := 0; dx < scale; dx++ {
						px := x + col*scale + dx
						py := y + row*scale + dy
						if px >= bounds.Min.X && px < bounds.Max.X && py >= bounds.Min.Y && py < bounds.Max.Y {
							img.Set(px, py, digitColor)
						}
					}
				}
			}
		}
	}
}
