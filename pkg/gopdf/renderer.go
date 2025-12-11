package gopdf

import (
	"fmt"
	"image"
	"image/png"
	"io"
	"os"

	"github.com/novvoo/go-cairo/pkg/cairo"
)

// PDFRenderer 用于将图片渲染为 PDF 或使用 Cairo 进行图形处理
type PDFRenderer struct {
	width  float64
	height float64
	dpi    float64
}

// RenderOptions 渲染选项
type RenderOptions struct {
	DPI        float64      // 分辨率，默认 72
	OutputPath string       // 输出文件路径
	Format     cairo.Format // 图片格式，默认 ARGB32
	Background *RGB         // 背景色，nil 表示透明
}

// RGB 颜色
type RGB struct {
	R, G, B float64
}

// NewPDFRenderer 创建新的 PDF 渲染器
// width, height 单位为点 (points)，72 points = 1 inch
func NewPDFRenderer(width, height float64) *PDFRenderer {
	return &PDFRenderer{
		width:  width,
		height: height,
		dpi:    72,
	}
}

// SetDPI 设置渲染分辨率
func (r *PDFRenderer) SetDPI(dpi float64) {
	r.dpi = dpi
}

// CreatePDFFromImage 从图片创建 PDF
func (r *PDFRenderer) CreatePDFFromImage(imagePath, outputPath string) error {
	// 读取图片
	imgFile, err := os.Open(imagePath)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}
	defer imgFile.Close()

	img, _, err := image.Decode(imgFile)
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	bounds := img.Bounds()
	width := float64(bounds.Dx())
	height := float64(bounds.Dy())

	// 创建 PDF surface
	pdfSurface := cairo.NewPDFSurface(outputPath, width, height)
	defer pdfSurface.Destroy()

	ctx := cairo.NewContext(pdfSurface)
	defer ctx.Destroy()

	// 创建临时 image surface 来加载图片
	imgSurface := cairo.NewImageSurface(cairo.FormatARGB32, bounds.Dx(), bounds.Dy())
	defer imgSurface.Destroy()

	// 将 Go image 转换为 Cairo surface
	if imgSurf, ok := imgSurface.(cairo.ImageSurface); ok {
		data := imgSurf.GetData()
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				r, g, b, a := img.At(x, y).RGBA()
				offset := (y-bounds.Min.Y)*imgSurf.GetStride() + (x-bounds.Min.X)*4
				// Cairo 使用预乘 alpha 的 BGRA 格式
				data[offset+0] = uint8(b >> 8)
				data[offset+1] = uint8(g >> 8)
				data[offset+2] = uint8(r >> 8)
				data[offset+3] = uint8(a >> 8)
			}
		}
		imgSurf.MarkDirty()
	}

	// 绘制图片到 PDF
	ctx.SetSourceSurface(imgSurface, 0, 0)
	ctx.Paint()

	// 显示页面
	pdfSurface.ShowPage()

	return nil
}

// RenderToPNG 使用 Cairo 渲染图形到 PNG
func (r *PDFRenderer) RenderToPNG(outputPath string, drawFunc func(ctx cairo.Context)) error {
	opts := &RenderOptions{
		DPI:        r.dpi,
		OutputPath: outputPath,
		Format:     cairo.FormatARGB32,
		Background: &RGB{R: 1, G: 1, B: 1}, // 白色背景
	}

	return r.RenderWithOptions(opts, drawFunc)
}

// RenderWithOptions 使用自定义选项渲染
func (r *PDFRenderer) RenderWithOptions(opts *RenderOptions, drawFunc func(ctx cairo.Context)) error {
	if opts == nil {
		opts = &RenderOptions{
			DPI:    72,
			Format: cairo.FormatARGB32,
		}
	}

	if opts.DPI == 0 {
		opts.DPI = r.dpi
	}

	// 根据 DPI 计算实际渲染尺寸
	scale := opts.DPI / 72.0
	renderWidth := int(r.width * scale)
	renderHeight := int(r.height * scale)

	// 创建图像 surface
	imgSurface := cairo.NewImageSurface(opts.Format, renderWidth, renderHeight)
	defer imgSurface.Destroy()

	ctx := cairo.NewContext(imgSurface)
	defer ctx.Destroy()

	// 设置背景色
	if opts.Background != nil {
		ctx.SetSourceRGB(opts.Background.R, opts.Background.G, opts.Background.B)
		ctx.Paint()
	}

	// 缩放以匹配 DPI
	ctx.Scale(scale, scale)

	// 执行用户的绘制函数
	if drawFunc != nil {
		drawFunc(ctx)
	}

	// 保存为 PNG
	if opts.OutputPath != "" {
		if imgSurf, ok := imgSurface.(cairo.ImageSurface); ok {
			status := imgSurf.WriteToPNG(opts.OutputPath)
			if status != cairo.StatusSuccess {
				return fmt.Errorf("failed to write PNG: %v", status)
			}
		}
	}

	return nil
}

// RenderToPDF 渲染到 PDF 文件
func (r *PDFRenderer) RenderToPDF(outputPath string, drawFunc func(ctx cairo.Context)) error {
	// 创建 PDF surface
	pdfSurface := cairo.NewPDFSurface(outputPath, r.width, r.height)
	defer pdfSurface.Destroy()

	ctx := cairo.NewContext(pdfSurface)
	defer ctx.Destroy()

	// 执行用户的绘制函数
	if drawFunc != nil {
		drawFunc(ctx)
	}

	// 显示页面
	pdfSurface.ShowPage()

	return nil
}

// RenderToWriter 渲染到 io.Writer (PNG 格式)
func (r *PDFRenderer) RenderToWriter(w io.Writer, opts *RenderOptions, drawFunc func(ctx cairo.Context)) error {
	if opts == nil {
		opts = &RenderOptions{
			DPI:    72,
			Format: cairo.FormatARGB32,
		}
	}

	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "cairo_render_*.png")
	if err != nil {
		return fmt.Errorf("failed to create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()
	defer os.Remove(tmpPath)

	// 渲染到临时文件
	opts.OutputPath = tmpPath
	if err := r.RenderWithOptions(opts, drawFunc); err != nil {
		return err
	}

	// 读取并写入到 writer
	data, err := os.ReadFile(tmpPath)
	if err != nil {
		return fmt.Errorf("failed to read rendered image: %w", err)
	}

	if _, err := w.Write(data); err != nil {
		return fmt.Errorf("failed to write to writer: %w", err)
	}

	return nil
}

// ConvertImageToPNG 使用 Cairo 转换图片格式
func ConvertImageToPNG(inputPath, outputPath string) error {
	// 读取图片
	imgFile, err := os.Open(inputPath)
	if err != nil {
		return fmt.Errorf("failed to open image: %w", err)
	}
	defer imgFile.Close()

	img, _, err := image.Decode(imgFile)
	if err != nil {
		return fmt.Errorf("failed to decode image: %w", err)
	}

	// 保存为 PNG
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	return png.Encode(outFile, img)
}
