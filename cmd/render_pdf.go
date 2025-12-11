//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"log"
	"os"

	"github.com/novvoo/go-pdf/pkg/gopdf"

	"github.com/novvoo/go-cairo/pkg/cairo"
)

func main() {
	fmt.Println("🎨 Rendering test.pdf to PNG")
	fmt.Println("=" + string(make([]byte, 50)))

	pdfPath := "test.pdf"
	outputPath := "test.png"
	rendered := false

	// 方法 1: 尝试使用 pdfcpu + go-cairo 渲染真实的 PDF
	fmt.Println("\n📖 Attempting to render PDF using pdfcpu + go-cairo...")

	if fileExists(pdfPath) {
		reader := gopdf.NewPDFReader(pdfPath)
		err := reader.RenderPageToPNG(1, outputPath, 150)
		if err != nil {
			fmt.Printf("⚠️  pdfcpu rendering failed: %v\n", err)
		} else {
			fmt.Printf("✅ PDF rendered successfully with pdfcpu: %s\n", outputPath)
			rendered = true
		}
	} else {
		fmt.Printf("⚠️  PDF file not found: %s\n", pdfPath)
	}

	// 方法 2: 如果系统工具不可用，使用 go-cairo 创建演示内容
	if !rendered {
		fmt.Println("\n📄 System tools not available, creating demo content with go-cairo...")

		renderer := gopdf.NewPDFRenderer(600, 800)
		renderer.SetDPI(150)

		err := renderer.RenderToPNG(outputPath, drawTestContent)
		if err != nil {
			log.Fatalf("❌ Failed to create PNG: %v", err)
		}
		fmt.Printf("✅ Demo PNG created: %s\n", outputPath)
		rendered = true
	}

	fmt.Println("\n" + string(make([]byte, 50)))
	if rendered {
		fmt.Println("🎉 Rendering completed!")
		fmt.Printf("📁 Output: %s\n", outputPath)
	} else {
		fmt.Println("❌ Failed to render image")
	}

	if !fileExists(pdfPath) {
		fmt.Println("\n💡 Note: test.pdf not found.")
		fmt.Println("   This demo uses pdfcpu + go-cairo for PDF rendering (pure Go solution)")
		fmt.Println("   Alternative options:")
		fmt.Println("   1. ImageMagick: magick convert -density 150 your.pdf[0] test.png")
		fmt.Println("   2. poppler-utils: pdftoppm -png -singlefile -r 150 your.pdf test")
	}
}

// fileExists 检查文件是否存在
func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

func drawTestContent(ctx cairo.Context) {
	// 这个函数被 RenderToPDF 和 RenderToPNG 共用
	// 绘制标题
	ctx.SetSourceRGB(0.1, 0.1, 0.1)
	layout := ctx.PangoCairoCreateLayout().(*cairo.PangoCairoLayout)
	fontDesc := cairo.NewPangoFontDescription()
	fontDesc.SetFamily("sans-serif")
	fontDesc.SetSize(36)
	fontDesc.SetWeight(cairo.PangoWeightBold)
	layout.SetFontDescription(fontDesc)
	layout.SetText("Test PDF Rendering")

	ctx.MoveTo(50, 50)
	ctx.PangoCairoShowText(layout)

	// 绘制副标题
	fontDesc.SetSize(18)
	fontDesc.SetWeight(cairo.PangoWeightNormal)
	layout.SetFontDescription(fontDesc)
	layout.SetText("This is a demonstration of PDF rendering capabilities")
	ctx.SetSourceRGB(0.4, 0.4, 0.4)
	ctx.MoveTo(50, 110)
	ctx.PangoCairoShowText(layout)

	// 绘制分隔线
	ctx.SetSourceRGB(0.2, 0.4, 0.8)
	ctx.SetLineWidth(2)
	ctx.MoveTo(50, 150)
	ctx.LineTo(550, 150)
	ctx.Stroke()

	// 绘制一些图形示例
	// 矩形
	ctx.SetSourceRGB(0.9, 0.3, 0.3)
	ctx.Rectangle(50, 180, 120, 80)
	ctx.Fill()

	ctx.SetSourceRGB(0, 0, 0)
	fontDesc.SetSize(14)
	layout.SetFontDescription(fontDesc)
	layout.SetText("Rectangle")
	ctx.MoveTo(60, 280)
	ctx.PangoCairoShowText(layout)

	// 圆形
	ctx.SetSourceRGB(0.3, 0.9, 0.3)
	ctx.Arc(280, 220, 40, 0, 6.28318530718)
	ctx.Fill()

	layout.SetText("Circle")
	ctx.MoveTo(250, 280)
	ctx.PangoCairoShowText(layout)

	// 线条
	ctx.SetSourceRGB(0.3, 0.3, 0.9)
	ctx.SetLineWidth(5)
	ctx.MoveTo(380, 180)
	ctx.LineTo(500, 260)
	ctx.Stroke()

	layout.SetText("Line")
	ctx.MoveTo(420, 280)
	ctx.PangoCairoShowText(layout)

	// 绘制文本框
	ctx.SetSourceRGB(0.95, 0.95, 0.95)
	ctx.Rectangle(50, 320, 500, 150)
	ctx.Fill()

	ctx.SetSourceRGB(0, 0, 0)
	ctx.SetLineWidth(1)
	ctx.Rectangle(50, 320, 500, 150)
	ctx.Stroke()

	fontDesc.SetSize(16)
	layout.SetFontDescription(fontDesc)
	layout.SetText("PDF Rendering Features:")
	ctx.MoveTo(70, 340)
	ctx.PangoCairoShowText(layout)

	fontDesc.SetSize(14)
	layout.SetFontDescription(fontDesc)

	features := []string{
		"✓ Vector graphics rendering",
		"✓ Text with custom fonts",
		"✓ Multiple shapes and colors",
		"✓ High-quality output",
	}

	y := 370.0
	for _, feature := range features {
		layout.SetText(feature)
		ctx.MoveTo(90, y)
		ctx.PangoCairoShowText(layout)
		y += 30
	}

	// 底部信息
	ctx.SetSourceRGB(0.5, 0.5, 0.5)
	fontDesc.SetSize(12)
	layout.SetFontDescription(fontDesc)
	layout.SetText("Generated with go-cairo library • https://github.com/novvoo/go-cairo")
	ctx.MoveTo(50, 750)
	ctx.PangoCairoShowText(layout)
}
