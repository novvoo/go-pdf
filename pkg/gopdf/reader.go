package gopdf

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"

	"github.com/novvoo/go-cairo/pkg/cairo"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// PDFReader 用于读取和渲染 PDF 文件
type PDFReader struct {
	pdfPath string
}

// NewPDFReader 创建新的 PDF 读取器
func NewPDFReader(pdfPath string) *PDFReader {
	return &PDFReader{
		pdfPath: pdfPath,
	}
}

// RenderPageToPNG 将 PDF 的指定页面渲染为 PNG 图片
// pageNum: 页码（从 1 开始）
// outputPath: 输出 PNG 文件路径
// dpi: 渲染分辨率，默认 150
func (r *PDFReader) RenderPageToPNG(pageNum int, outputPath string, dpi float64) error {
	if dpi == 0 {
		dpi = 150
	}

	// 获取页面数量
	pageCount, err := api.PageCountFile(r.pdfPath)
	if err != nil {
		return fmt.Errorf("failed to get page count: %w", err)
	}

	if pageNum < 1 || pageNum > pageCount {
		return fmt.Errorf("invalid page number: %d (total pages: %d)", pageNum, pageCount)
	}

	// 获取页面尺寸
	pageDims, err := api.PageDimsFile(r.pdfPath)
	if err != nil {
		return fmt.Errorf("failed to get page dimensions: %w", err)
	}

	// 默认页面尺寸（Letter size: 8.5 x 11 inches）
	widthPoints := 612.0  // 8.5 * 72
	heightPoints := 792.0 // 11 * 72

	if pageNum <= len(pageDims) {
		dim := pageDims[pageNum-1]
		widthPoints = dim.Width
		heightPoints = dim.Height
	}

	// 根据 DPI 计算渲染尺寸
	scale := dpi / 72.0
	width := int(widthPoints * scale)
	height := int(heightPoints * scale)

	// 使用 go-cairo 创建渲染表面
	surface := cairo.NewImageSurface(cairo.FormatARGB32, width, height)
	defer surface.Destroy()

	cairoCtx := cairo.NewContext(surface)
	defer cairoCtx.Destroy()

	// 设置白色背景
	cairoCtx.SetSourceRGB(1, 1, 1)
	cairoCtx.Paint()

	// 缩放以匹配 DPI
	cairoCtx.Scale(scale, scale)

	// 渲染 PDF 内容到 Cairo context
	if err := renderPDFPageToCairo(r.pdfPath, pageNum, cairoCtx, widthPoints, heightPoints); err != nil {
		return fmt.Errorf("failed to render PDF page: %w", err)
	}

	// 直接使用 Cairo 保存 PNG
	if imgSurf, ok := surface.(cairo.ImageSurface); ok {
		status := imgSurf.WriteToPNG(outputPath)
		if status != cairo.StatusSuccess {
			return fmt.Errorf("failed to write PNG: %v", status)
		}
		return nil
	}

	return fmt.Errorf("failed to convert surface to image surface")
}

// RenderPageToImage 将 PDF 页面渲染为 image.Image
func (r *PDFReader) RenderPageToImage(pageNum int, dpi float64) (image.Image, error) {
	if dpi == 0 {
		dpi = 150
	}

	// 获取页面数量
	pageCount, err := api.PageCountFile(r.pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get page count: %w", err)
	}

	if pageNum < 1 || pageNum > pageCount {
		return nil, fmt.Errorf("invalid page number: %d (total pages: %d)", pageNum, pageCount)
	}

	// 获取页面尺寸
	pageDims, err := api.PageDimsFile(r.pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get page dimensions: %w", err)
	}

	// 默认页面尺寸（Letter size: 8.5 x 11 inches）
	widthPoints := 612.0  // 8.5 * 72
	heightPoints := 792.0 // 11 * 72

	if pageNum <= len(pageDims) {
		dim := pageDims[pageNum-1]
		widthPoints = dim.Width
		heightPoints = dim.Height
	}

	// 根据 DPI 计算渲染尺寸
	scale := dpi / 72.0
	width := int(widthPoints * scale)
	height := int(heightPoints * scale)

	// 使用 go-cairo 创建渲染表面
	surface := cairo.NewImageSurface(cairo.FormatARGB32, width, height)
	defer surface.Destroy()

	cairoCtx := cairo.NewContext(surface)
	defer cairoCtx.Destroy()

	// 设置白色背景
	cairoCtx.SetSourceRGB(1, 1, 1)
	cairoCtx.Paint()

	// 缩放以匹配 DPI
	cairoCtx.Scale(scale, scale)

	// 渲染 PDF 内容到 Cairo context
	if err := renderPDFPageToCairo(r.pdfPath, pageNum, cairoCtx, widthPoints, heightPoints); err != nil {
		return nil, fmt.Errorf("failed to render PDF page: %w", err)
	}

	// 直接保存 Cairo surface 到 PNG，然后读取回来
	// 这样避免了颜色格式转换的问题
	tmpPath := fmt.Sprintf("temp_render_%d.png", pageNum)
	defer os.Remove(tmpPath)

	if imgSurf, ok := surface.(cairo.ImageSurface); ok {
		status := imgSurf.WriteToPNG(tmpPath)
		if status != cairo.StatusSuccess {
			return nil, fmt.Errorf("failed to write PNG: %v", status)
		}

		// 读取回来作为 image.Image
		file, err := os.Open(tmpPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open temp PNG: %w", err)
		}
		defer file.Close()

		img, err := png.Decode(file)
		if err != nil {
			return nil, fmt.Errorf("failed to decode PNG: %w", err)
		}

		return img, nil
	}

	return nil, fmt.Errorf("failed to convert surface to image")
}

// GetPageCount 获取 PDF 的页数
func (r *PDFReader) GetPageCount() (int, error) {
	return api.PageCountFile(r.pdfPath)
}

// RenderAllPagesToPNG 将所有页面渲染为 PNG 文件
func (r *PDFReader) RenderAllPagesToPNG(outputDir string, dpi float64) error {
	pageCount, err := r.GetPageCount()
	if err != nil {
		return err
	}

	// 确保输出目录存在
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	for i := 1; i <= pageCount; i++ {
		outputPath := fmt.Sprintf("%s/page_%d.png", outputDir, i)
		if err := r.RenderPageToPNG(i, outputPath, dpi); err != nil {
			return fmt.Errorf("failed to render page %d: %w", i, err)
		}
	}

	return nil
}

// renderPDFPageToCairo 将 PDF 页面内容渲染到 Cairo context
func renderPDFPageToCairo(pdfPath string, pageNum int, cairoCtx cairo.Context, width, height float64) error {
	// 打开 PDF 文件并读取上下文
	ctx, err := api.ReadContextFile(pdfPath)
	if err != nil {
		return fmt.Errorf("failed to read PDF context: %w", err)
	}

	// 提取页面文本
	text, err := extractPageText(ctx, pageNum)
	if err != nil {
		// 如果提取失败，显示错误信息
		text = fmt.Sprintf("Failed to extract text from page %d: %v", pageNum, err)
		fmt.Printf("[DEBUG] Text extraction error: %v\n", err)
	} else {
		fmt.Printf("[DEBUG] Extracted text length: %d\n", len(text))
		if len(text) > 0 && len(text) < 200 {
			fmt.Printf("[DEBUG] Extracted text: %q\n", text)
		}
	}

	// 如果没有文本内容，显示提示
	if text == "" {
		text = fmt.Sprintf("Page %d (No text content found)", pageNum)
	}

	// 使用 PangoCairo 渲染文本
	cairoCtx.SetSourceRGB(0, 0, 0)

	layout := cairoCtx.PangoCairoCreateLayout().(*cairo.PangoCairoLayout)
	fontDesc := cairo.NewPangoFontDescription()
	fontDesc.SetFamily("sans-serif")
	fontDesc.SetSize(12)
	layout.SetFontDescription(fontDesc)

	// 设置文本宽度以支持自动换行
	layout.SetWidth(int((width - 40) * 1024)) // Pango 使用 1024 为单位
	layout.SetText(text)

	cairoCtx.MoveTo(20, 20)
	cairoCtx.PangoCairoShowText(layout)

	return nil
}

// extractPageText 从 PDF 页面提取文本内容
func extractPageText(ctx *model.Context, pageNum int) (string, error) {
	// 使用 pdfcpu 的 ExtractPageContent 提取文本
	// 这会返回页面的内容流

	// 获取页面字典
	pageDict, _, _, err := ctx.PageDict(pageNum, false)
	if err != nil {
		return "", fmt.Errorf("failed to get page dict: %w", err)
	}

	// 提取页面内容流
	contents, _ := pageDict.Find("Contents")
	if contents == nil {
		return "Empty page", nil
	}

	var textContent string

	// 处理内容对象
	switch obj := contents.(type) {
	case types.IndirectRef:
		// 解引用
		derefObj, err := ctx.Dereference(obj)
		if err != nil {
			return "", fmt.Errorf("failed to dereference contents: %w", err)
		}

		if streamDict, ok := derefObj.(types.StreamDict); ok {
			decoded, _, err := ctx.DereferenceStreamDict(streamDict)
			if err == nil && decoded != nil {
				content := string(decoded.Content)
				fmt.Printf("[DEBUG] Content stream length: %d bytes\n", len(content))
				if len(content) < 500 {
					fmt.Printf("[DEBUG] Content stream: %q\n", content)
				} else {
					fmt.Printf("[DEBUG] Content stream preview: %q...\n", content[:500])
				}
				textContent = extractTextFromStream(content)
			}
		}

	case types.StreamDict:
		// 直接解码流内容
		decoded, _, err := ctx.DereferenceStreamDict(obj)
		if err == nil && decoded != nil {
			textContent = extractTextFromStream(string(decoded.Content))
		}

	case types.Array:
		// 多个内容流
		for _, item := range obj {
			var streamDict types.StreamDict
			var ok bool

			if indRef, isRef := item.(types.IndirectRef); isRef {
				derefObj, err := ctx.Dereference(indRef)
				if err == nil {
					streamDict, ok = derefObj.(types.StreamDict)
				}
			} else {
				streamDict, ok = item.(types.StreamDict)
			}

			if ok {
				decoded, _, err := ctx.DereferenceStreamDict(streamDict)
				if err == nil && decoded != nil {
					textContent += extractTextFromStream(string(decoded.Content)) + "\n"
				}
			}
		}
	}

	if textContent == "" {
		return "No extractable text found", nil
	}

	return textContent, nil
}

// extractTextFromStream 从 PDF 内容流中提取文本
func extractTextFromStream(stream string) string {
	// 提取 PDF 内容流中的文本
	// 支持 Tj, TJ, ' 和 " 操作符
	var result strings.Builder

	i := 0
	for i < len(stream) {
		// 跳过空白字符
		for i < len(stream) && (stream[i] == ' ' || stream[i] == '\t' || stream[i] == '\r' || stream[i] == '\n') {
			i++
		}

		if i >= len(stream) {
			break
		}

		// 查找文本字符串 (...)
		if stream[i] == '(' {
			start := i + 1
			i++
			depth := 1

			// 找到匹配的右括号，处理转义
			for i < len(stream) && depth > 0 {
				if stream[i] == '\\' && i+1 < len(stream) {
					i += 2 // 跳过转义字符
					continue
				}
				if stream[i] == '(' {
					depth++
				} else if stream[i] == ')' {
					depth--
				}
				i++
			}

			if depth == 0 {
				text := stream[start : i-1]
				// 处理转义字符
				text = strings.ReplaceAll(text, "\\n", "\n")
				text = strings.ReplaceAll(text, "\\r", "")
				text = strings.ReplaceAll(text, "\\t", "\t")
				text = strings.ReplaceAll(text, "\\(", "(")
				text = strings.ReplaceAll(text, "\\)", ")")
				text = strings.ReplaceAll(text, "\\\\", "\\")

				// 检查后面是否有文本显示操作符
				j := i
				for j < len(stream) && (stream[j] == ' ' || stream[j] == '\t' || stream[j] == '\r' || stream[j] == '\n') {
					j++
				}

				// 检查是否是文本操作符 Tj, ', "
				if j < len(stream) {
					if j+1 < len(stream) && stream[j:j+2] == "Tj" {
						result.WriteString(text)
						result.WriteString(" ")
					} else if stream[j] == '\'' || stream[j] == '"' {
						result.WriteString(text)
						result.WriteString("\n")
					}
				}
			}
			continue
		}

		// 查找数组 [...]（用于 TJ 操作符）
		if stream[i] == '[' {
			i++
			for i < len(stream) && stream[i] != ']' {
				// 跳过空白
				for i < len(stream) && (stream[i] == ' ' || stream[i] == '\t' || stream[i] == '\r' || stream[i] == '\n') {
					i++
				}

				if i < len(stream) && stream[i] == '(' {
					start := i + 1
					i++
					depth := 1

					for i < len(stream) && depth > 0 {
						if stream[i] == '\\' && i+1 < len(stream) {
							i += 2
							continue
						}
						if stream[i] == '(' {
							depth++
						} else if stream[i] == ')' {
							depth--
						}
						i++
					}

					if depth == 0 {
						text := stream[start : i-1]
						text = strings.ReplaceAll(text, "\\n", "\n")
						text = strings.ReplaceAll(text, "\\r", "")
						text = strings.ReplaceAll(text, "\\t", "\t")
						text = strings.ReplaceAll(text, "\\(", "(")
						text = strings.ReplaceAll(text, "\\)", ")")
						text = strings.ReplaceAll(text, "\\\\", "\\")
						result.WriteString(text)
					}
				} else if i < len(stream) && stream[i] != ']' {
					i++
				}
			}

			if i < len(stream) && stream[i] == ']' {
				i++
				// 检查 TJ 操作符
				for i < len(stream) && (stream[i] == ' ' || stream[i] == '\t' || stream[i] == '\r' || stream[i] == '\n') {
					i++
				}
				if i+1 < len(stream) && stream[i:i+2] == "TJ" {
					result.WriteString(" ")
					i += 2
				}
			}
			continue
		}

		i++
	}

	text := result.String()
	if text == "" {
		return ""
	}

	// 清理多余的空白
	text = strings.TrimSpace(text)
	return text
}

// convertCairoSurfaceToImage 将 Cairo surface 转换为 Go image.Image
func convertCairoSurfaceToImage(imgSurf cairo.ImageSurface) image.Image {
	data := imgSurf.GetData()
	stride := imgSurf.GetStride()
	width := imgSurf.GetWidth()
	height := imgSurf.GetHeight()

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := y*stride + x*4
			// Cairo 使用 BGRA 预乘 alpha 格式
			b := data[offset+0]
			g := data[offset+1]
			r := data[offset+2]
			a := data[offset+3]

			// 如果使用了预乘 alpha，需要反预乘
			if a > 0 && a < 255 {
				alpha := float64(a)
				r = uint8(float64(r) * 255.0 / alpha)
				g = uint8(float64(g) * 255.0 / alpha)
				b = uint8(float64(b) * 255.0 / alpha)
			}

			img.SetRGBA(x, y, color.RGBA{R: r, G: g, B: b, A: a})
		}
	}

	return img
}

// ConvertPDFPageToImage 使用 Cairo 将 PDF 页面转换为图像的辅助函数
func ConvertPDFPageToImage(pdfPath string, pageNum int, width, height int) (image.Image, error) {
	reader := NewPDFReader(pdfPath)
	dpi := float64(width) / 8.5 // 假设 Letter size
	return reader.RenderPageToImage(pageNum, dpi)
}

// SaveImageToPNG 保存图像为 PNG 文件
func SaveImageToPNG(img image.Image, outputPath string) error {
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	// 使用标准库的 png 包保存
	return png.Encode(outFile, img)
}
