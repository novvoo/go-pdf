//go:build ignore
// +build ignore

package main

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/novvoo/go-pdf/pkg/gopdf"
	"github.com/novvoo/go-pdf/test"
)

func main() {
	// 固定使用 test_vector.pdf
	pdfPath := "test/test_vector.pdf"
	outputPath := "test/test_vector.png"
	reportPath := "test/render_vector.txt"

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

	// 添加文本元素信息
	if len(result.TextElements) > 0 {
		report += "Text Elements:\n"
		report += "--------------\n"
		report += fmt.Sprintf("Total text elements: %d\n\n", len(result.TextElements))

		// 显示前 50 个文本元素
		maxDisplay := 50
		if len(result.TextElements) < maxDisplay {
			maxDisplay = len(result.TextElements)
		}

		for i := 0; i < maxDisplay; i++ {
			te := result.TextElements[i]
			report += fmt.Sprintf("[%d] Position: (%.2f, %.2f)\n", i+1, te.X, te.Y)
			report += fmt.Sprintf("    Font: %s, Size: %.2f\n", te.FontName, te.FontSize)
			// 限制文本长度
			displayText := te.Text
			if len(displayText) > 100 {
				displayText = displayText[:100] + "..."
			}
			report += fmt.Sprintf("    Text: %q\n\n", displayText)
		}

		if len(result.TextElements) > maxDisplay {
			report += fmt.Sprintf("... and %d more text elements\n\n", len(result.TextElements)-maxDisplay)
		}
	} else {
		report += "Text Elements: None found\n\n"
	}

	// 添加图片元素信息
	if len(result.Images) > 0 {
		report += "Image Elements:\n"
		report += "---------------\n"
		report += fmt.Sprintf("Total images: %d\n\n", len(result.Images))

		for i, img := range result.Images {
			report += fmt.Sprintf("[%d] Name: %s\n", i+1, img.Name)
			report += fmt.Sprintf("    Position: (%.2f, %.2f)\n", img.X, img.Y)
			report += fmt.Sprintf("    Size: %.2f x %.2f\n\n", img.Width, img.Height)
		}
	} else {
		report += "Image Elements: None found\n\n"
	}

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
		report += "Cairo/Pango Debug Output:\n"
		report += "-------------------------\n"
		report += capturedOutp

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
