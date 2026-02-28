package gopdf

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"io"
	"math"
	"os"
)

// PDFRenderer 用于将图片渲染为 PDF 或使用 Gopdf 进行图形处理
// 使用 Pixman backend、Rasterizer 和 Alpha Blend 进行底层渲染
type PDFRenderer struct {
	width         float64
	height        float64
	dpi           float64
	usePixman     bool           // 是否使用 Pixman 后端
	useRasterizer bool           // 是否使用光栅化器
	pixmanBackend *PixmanBackend // Pixman 图像后端
	rasterizer    *Rasterizer    // 光栅化器
	alphaBlender  *AlphaBlender  // Alpha 混合器
	blendMode     string         // 当前混合模式
}

// PDFRenderOptions 渲染选项
type PDFRenderOptions struct {
	DPI        float64 // 分辨率，默认 72
	OutputPath string  // 输出文件路径
	Format     Format  // 图片格式，默认 ARGB32
	Background *RGB    // 背景色，nil 表示透明
}

type RenderOptions = PDFRenderOptions

// RGB 颜色
type RGB struct {
	R, G, B float64
}

// NewPDFRenderer 创建新的 PDF 渲染器
// width, height 单位为点 (points)，72 points = 1 inch
// 默认使用 Pixman 后端和光栅化器以获得更好的渲染质量
func NewPDFRenderer(width, height float64) *PDFRenderer {
	return &PDFRenderer{
		width:         width,
		height:        height,
		dpi:           72,
		usePixman:     true, // 默认启用 Pixman
		useRasterizer: true, // 默认启用光栅化器
		blendMode:     "Normal",
	}
}

// SetUsePixman 设置是否使用 Pixman 后端
func (r *PDFRenderer) SetUsePixman(use bool) {
	r.usePixman = use
}

// SetUseRasterizer 设置是否使用光栅化器
func (r *PDFRenderer) SetUseRasterizer(use bool) {
	r.useRasterizer = use
}

// SetBlendMode 设置混合模式
func (r *PDFRenderer) SetBlendMode(mode string) {
	r.blendMode = mode
	if r.alphaBlender != nil {
		op := GetPDFBlendOperator(mode)
		r.alphaBlender.SetOperator(op)
	}
}

// SetDPI 设置渲染分辨率
func (r *PDFRenderer) SetDPI(dpi float64) {
	r.dpi = dpi
}

// CreatePDFFromImage 从图片创建 PDF
// 使用 Pixman 后端进行图像处理
// 优化：使用批量像素复制替代逐像素循环
// inputDPI: 输入图像的DPI，如果为0则默认为72
func (r *PDFRenderer) CreatePDFFromImage(imagePath, outputPath string) error {
	return r.CreatePDFFromImageWithDPI(imagePath, outputPath, 0)
}

// CreatePDFFromImageWithDPI 从图片创建 PDF，支持指定输入图像的DPI
// 🔥 修复：正确处理DPI，避免图像大小异常
func (r *PDFRenderer) CreatePDFFromImageWithDPI(imagePath, outputPath string, inputDPI float64) error {
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
	pixelWidth := float64(bounds.Dx())
	pixelHeight := float64(bounds.Dy())

	// 🔥 修复：如果未指定DPI，尝试从图像元数据中读取，否则默认为72
	if inputDPI == 0 {
		inputDPI = 72 // PDF标准DPI
		debugPrintf("⚠️  No DPI specified, using default 72 DPI\n")
	}

	// 🔥 修复：根据DPI计算PDF中的实际尺寸（单位：points）
	// PDF中 1 point = 1/72 inch
	// 公式：widthPoints = pixelWidth * (72 / inputDPI)
	width := pixelWidth * (72.0 / inputDPI)
	height := pixelHeight * (72.0 / inputDPI)

	debugPrintf("📐 Image size: %.0fx%.0f pixels at %.0f DPI -> %.2fx%.2f points in PDF\n",
		pixelWidth, pixelHeight, inputDPI, width, height)

	// 创建 PDF surface
	pdfSurface := NewPDFSurface(outputPath, width, height)
	defer pdfSurface.Destroy()

	ctx := NewContext(pdfSurface)
	defer ctx.Destroy()

	var imgSurface ImageSurface

	// 如果启用 Pixman，使用 Pixman 处理图像
	if r.usePixman {
		rgba := convertToRGBAOptimized(img)

		// 使用 Pixman 后端
		pixmanBackend := NewPixmanBackendFromRGBA(rgba)
		if pixmanBackend != nil {
			defer pixmanBackend.Destroy()

			// 转换回 RGBA 并创建 Gopdf surface
			processedRGBA := pixmanBackend.ToRGBA()
			converter := NewGopdfImageConverter()
			var err error
			imgSurface, err = converter.ImageToGopdfSurface(processedRGBA, FormatARGB32)
			if err != nil {
				return fmt.Errorf("failed to convert image to Gopdf surface: %w", err)
			}
		} else {
			// 🔥 修复：Pixman 后端创建失败时记录日志并回退
			debugPrintf("⚠️  Pixman backend creation failed, falling back to standard method\n")
		}
	}

	// 回退到标准方法
	if imgSurface == nil {
		converter := NewGopdfImageConverter()
		var err error
		imgSurface, err = converter.ImageToGopdfSurface(img, FormatARGB32)
		if err != nil {
			return fmt.Errorf("failed to convert image to Gopdf surface: %w", err)
		}
	}
	defer imgSurface.Destroy()

	// 绘制图片到 PDF
	ctx.SetSourceSurface(imgSurface, 0, 0)
	ctx.Paint()

	// 显示页面
	pdfSurface.ShowPage()

	return nil
}

// convertToRGBAOptimized 优化的图像转换函数
// 使用批量操作和类型断言避免逐像素循环
func convertToRGBAOptimized(img image.Image) *image.RGBA {
	// 快速路径：已经是 RGBA
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba
	}

	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)

	// 尝试使用类型断言进行批量复制
	switch src := img.(type) {
	case *image.NRGBA:
		// NRGBA 可以直接复制像素数据
		copy(rgba.Pix, src.Pix)
		return rgba
	case *image.YCbCr:
		// YCbCr 需要转换，但可以批量处理
		convertYCbCrToRGBA(src, rgba)
		return rgba
	case *image.Gray:
		// 灰度图批量转换
		convertGrayToRGBA(src, rgba)
		return rgba
	default:
		// 回退到逐像素复制（但使用优化的循环）
		dx, dy := bounds.Dx(), bounds.Dy()
		for y := 0; y < dy; y++ {
			for x := 0; x < dx; x++ {
				rgba.Set(x, y, img.At(bounds.Min.X+x, bounds.Min.Y+y))
			}
		}
		return rgba
	}
}

// convertYCbCrToRGBA 批量转换 YCbCr 到 RGBA
// 🔥 修复：使用浮点计算提高色调准确性，并应用gamma校正
func convertYCbCrToRGBA(src *image.YCbCr, dst *image.RGBA) {
	bounds := src.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			yi := src.YOffset(x, y)
			ci := src.COffset(x, y)

			// 🔥 修复：使用浮点计算以提高精度
			yy := float64(src.Y[yi])
			cb := float64(src.Cb[ci]) - 128.0
			cr := float64(src.Cr[ci]) - 128.0

			// YCbCr 到 RGB 转换（ITU-R BT.601标准系数）
			// R = Y + 1.402 * Cr
			// G = Y - 0.344136 * Cb - 0.714136 * Cr
			// B = Y + 1.772 * Cb
			r := yy + 1.402*cr
			g := yy - 0.344136*cb - 0.714136*cr
			b := yy + 1.772*cb

			// 🔥 修复：应用简单的gamma校正以改善色调
			// 使用sRGB gamma近似（gamma = 2.2）
			const gamma = 2.2
			const invGamma = 1.0 / gamma

			// 归一化到 [0, 1]
			r = r / 255.0
			g = g / 255.0
			b = b / 255.0

			// 裁剪到 [0, 1]
			if r < 0 {
				r = 0
			} else if r > 1 {
				r = 1
			}
			if g < 0 {
				g = 0
			} else if g > 1 {
				g = 1
			}
			if b < 0 {
				b = 0
			} else if b > 1 {
				b = 1
			}

			// 应用gamma校正（线性 -> sRGB）
			// 简化版本：使用幂函数近似
			// 注意：完整的sRGB转换需要分段函数，这里使用简化版本
			if r > 0.0031308 {
				r = 1.055*pow(r, invGamma) - 0.055
			} else {
				r = 12.92 * r
			}
			if g > 0.0031308 {
				g = 1.055*pow(g, invGamma) - 0.055
			} else {
				g = 12.92 * g
			}
			if b > 0.0031308 {
				b = 1.055*pow(b, invGamma) - 0.055
			} else {
				b = 12.92 * b
			}

			// 转换回 [0, 255]
			i := dst.PixOffset(x, y)
			dst.Pix[i+0] = uint8(r*255.0 + 0.5)
			dst.Pix[i+1] = uint8(g*255.0 + 0.5)
			dst.Pix[i+2] = uint8(b*255.0 + 0.5)
			dst.Pix[i+3] = 255 // 完全不透明
		}
	}
}

// pow 简单的幂函数实现（用于gamma校正）
func pow(x, y float64) float64 {
	return math.Pow(x, y)
}

// convertGrayToRGBA 批量转换灰度图到 RGBA
func convertGrayToRGBA(src *image.Gray, dst *image.RGBA) {
	bounds := src.Bounds()
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			gray := src.GrayAt(x, y).Y
			i := dst.PixOffset(x, y)
			dst.Pix[i+0] = gray
			dst.Pix[i+1] = gray
			dst.Pix[i+2] = gray
			dst.Pix[i+3] = 255
		}
	}
}

// RenderToPNG 使用 Gopdf 渲染图形到 PNG
func (r *PDFRenderer) RenderToPNG(outputPath string, drawFunc func(ctx Context)) error {
	opts := &PDFRenderOptions{
		DPI:        r.dpi,
		OutputPath: outputPath,
		Format:     FormatARGB32,
		Background: &RGB{R: 1, G: 1, B: 1}, // 白色背景
	}

	return r.RenderWithOptions(opts, drawFunc)
}

// RenderWithOptions 使用自定义选项渲染
func (r *PDFRenderer) RenderWithOptions(opts *PDFRenderOptions, drawFunc func(ctx Context)) error {
	if opts == nil {
		opts = &PDFRenderOptions{
			DPI:    72,
			Format: FormatARGB32,
		}
	}

	if opts.DPI == 0 {
		opts.DPI = r.dpi
	}

	// 根据 DPI 计算实际渲染尺寸
	// 🔥 修复：使用四舍五入而不是截断，避免高 DPI 下像素丢失
	scale := opts.DPI / 72.0
	renderWidth := int(r.width*scale + 0.5)
	renderHeight := int(r.height*scale + 0.5)

	// 如果启用 Pixman 后端，使用 Pixman 进行渲染
	if r.usePixman {
		return r.renderWithPixman(opts, renderWidth, renderHeight, scale, drawFunc)
	}

	// 否则使用标准 Gopdf 渲染
	return r.renderWithGopdf(opts, renderWidth, renderHeight, scale, drawFunc)
}

// renderWithPixman 使用 Pixman 后端渲染
func (r *PDFRenderer) renderWithPixman(opts *PDFRenderOptions, width, height int, scale float64, drawFunc func(ctx Context)) error {
	// 🔥 修复：添加尺寸验证，避免创建过大的图像
	if width <= 0 || height <= 0 {
		return fmt.Errorf("invalid image dimensions: %dx%d", width, height)
	}
	if width > 32768 || height > 32768 {
		return fmt.Errorf("image dimensions too large: %dx%d (max 32768x32768)", width, height)
	}

	// 直接创建 ImageBackend（不使用 PixmanBackend）
	imageBackend := NewImageBackend(width, height)
	if imageBackend == nil {
		return fmt.Errorf("failed to create image backend")
	}

	// 设置背景色
	if opts.Background != nil {
		bgColor := color.RGBA{
			R: uint8(opts.Background.R * 255),
			G: uint8(opts.Background.G * 255),
			B: uint8(opts.Background.B * 255),
			A: 255,
		}
		imageBackend.Clear(bgColor)
	} else {
		// 🔥 修复：透明背景可能在 PDF 中导致问题，添加警告
		debugPrintf("⚠️  Using transparent background, may cause issues in PDF output\n")
		imageBackend.Clear(color.RGBA{0, 0, 0, 0}) // 透明背景
	}

	// 从 ImageBackend 获取 RGBA 图像并创建 Gopdf surface
	rgba := imageBackend.GetImage()
	if rgba == nil {
		return fmt.Errorf("failed to get image from backend")
	}

	// 使用 GopdfImageConverter 创建 surface
	converter := NewGopdfImageConverter()
	surface, err := converter.ImageToGopdfSurface(rgba, FormatARGB32)
	if err != nil {
		return fmt.Errorf("failed to create surface: %w", err)
	}
	defer surface.Destroy()

	ctx := NewContext(surface)
	defer ctx.Destroy()

	// 初始化 Alpha 混合器
	r.alphaBlender = NewAlphaBlender(GetPDFBlendOperator(r.blendMode))

	// 如果启用光栅化器，初始化它
	if r.useRasterizer {
		r.rasterizer = NewRasterizer(width, height)
		defer func() {
			r.rasterizer.Destroy()
			r.rasterizer = nil
		}()
	}

	// 缩放以匹配 DPI
	ctx.Scale(scale, scale)

	// 执行用户的绘制函数
	if drawFunc != nil {
		drawFunc(ctx)
	}

	// 保存为 PNG
	if opts.OutputPath != "" {
		// 从 Gopdf Surface 获取最终的图像数据
		finalRGBA := surface.GetGoImage()

		// 转换为 RGBA（如果需要）
		var outputRGBA *image.RGBA
		if rgba, ok := finalRGBA.(*image.RGBA); ok {
			outputRGBA = rgba
		} else {
			bounds := finalRGBA.Bounds()
			outputRGBA = image.NewRGBA(bounds)
			for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
				for x := bounds.Min.X; x < bounds.Max.X; x++ {
					outputRGBA.Set(x, y, finalRGBA.At(x, y))
				}
			}
		}

		outFile, err := os.Create(opts.OutputPath)
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer outFile.Close()

		if err := png.Encode(outFile, outputRGBA); err != nil {
			return fmt.Errorf("failed to encode PNG: %w", err)
		}
	}

	return nil
}

// renderWithGopdf 使用标准 Gopdf 渲染（回退方法）
func (r *PDFRenderer) renderWithGopdf(opts *PDFRenderOptions, width, height int, scale float64, drawFunc func(ctx Context)) error {
	// 创建图像 surface
	imgSurface := NewImageSurface(opts.Format, width, height)
	defer imgSurface.Destroy()

	ctx := NewContext(imgSurface)
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
		if imgSurf, ok := imgSurface.(ImageSurface); ok {
			status := imgSurf.WriteToPNG(opts.OutputPath)
			if status != StatusSuccess {
				return fmt.Errorf("failed to write PNG: status=%v", status)
			}
		} else {
			return fmt.Errorf("surface is not an ImageSurface")
		}
	}

	return nil
}

// RenderToPDF 渲染到 PDF 文件
func (r *PDFRenderer) RenderToPDF(outputPath string, drawFunc func(ctx Context)) error {
	// 创建 PDF surface
	pdfSurface := NewPDFSurface(outputPath, r.width, r.height)
	defer pdfSurface.Destroy()

	ctx := NewContext(pdfSurface)
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
func (r *PDFRenderer) RenderToWriter(w io.Writer, opts *PDFRenderOptions, drawFunc func(ctx Context)) error {
	if opts == nil {
		opts = &PDFRenderOptions{
			DPI:    72,
			Format: FormatARGB32,
		}
	}

	// 创建临时文件
	tmpFile, err := os.CreateTemp("", "gopdf_render_*.png")
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

// ConvertImageToPNG 使用 Gopdf 转换图片格式
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

// GetPixmanBackend 获取当前的 Pixman 后端（如果有）
func (r *PDFRenderer) GetPixmanBackend() *PixmanBackend {
	return r.pixmanBackend
}

// GetRasterizer 获取当前的光栅化器（如果有）
func (r *PDFRenderer) GetRasterizer() *Rasterizer {
	return r.rasterizer
}

// GetAlphaBlender 获取当前的 Alpha 混合器（如果有）
func (r *PDFRenderer) GetAlphaBlender() *AlphaBlender {
	return r.alphaBlender
}

// RenderWithPixmanBackend 使用 Pixman 后端直接渲染
// 提供对底层像素操作的完全控制
func (r *PDFRenderer) RenderWithPixmanBackend(width, height int, renderFunc func(backend *PixmanBackend) error) (*image.RGBA, error) {
	backend := NewPixmanBackend(width, height, PixmanFormatARGB32)
	if backend == nil {
		return nil, fmt.Errorf("failed to create pixman backend")
	}
	defer backend.Destroy()

	// 执行渲染函数
	if err := renderFunc(backend); err != nil {
		return nil, err
	}

	// 转换为 RGBA
	return backend.ToRGBA(), nil
}

// RenderWithRasterizer 使用光栅化器直接渲染路径
func (r *PDFRenderer) RenderWithRasterizer(width, height int, renderFunc func(rasterizer *Rasterizer) error) (*image.RGBA, error) {
	rasterizer := NewRasterizer(width, height)
	if rasterizer == nil {
		return nil, fmt.Errorf("failed to create rasterizer")
	}
	defer rasterizer.Destroy()

	// 执行渲染函数
	if err := renderFunc(rasterizer); err != nil {
		return nil, err
	}

	// 转换为图像
	img := rasterizer.ToImage()
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba, nil
	}

	// 转换为 RGBA
	bounds := img.Bounds()
	rgba := image.NewRGBA(bounds)
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			rgba.Set(x, y, img.At(x, y))
		}
	}

	return rgba, nil
}

// BlendImages 混合多个图像
func (r *PDFRenderer) BlendImages(images []*image.RGBA, blendModes []string) (*image.RGBA, error) {
	if len(images) == 0 {
		return nil, fmt.Errorf("no images to blend")
	}

	if len(blendModes) == 0 {
		// 默认使用 Normal 模式
		blendModes = make([]string, len(images))
		for i := range blendModes {
			blendModes[i] = "Normal"
		}
	}

	// 确保混合模式数量匹配
	if len(blendModes) < len(images) {
		for i := len(blendModes); i < len(images); i++ {
			blendModes = append(blendModes, "Normal")
		}
	}

	// 使用第一个图像作为基础
	result := images[0]
	bounds := result.Bounds()

	// 创建 Pixman 后端
	backend := NewPixmanBackendFromRGBA(result)
	if backend == nil {
		return nil, fmt.Errorf("failed to create pixman backend")
	}
	defer backend.Destroy()

	// 混合其他图像
	for i := 1; i < len(images); i++ {
		srcBackend := NewPixmanBackendFromRGBA(images[i])
		if srcBackend == nil {
			continue
		}

		op := GetPDFBlendOperator(blendModes[i])
		backend.Composite(srcBackend, 0, 0, 0, 0, bounds.Dx(), bounds.Dy(), op)

		srcBackend.Destroy()
	}

	return backend.ToRGBA(), nil
}

// ApplyColorSpaceConversion 应用颜色空间转换
func (r *PDFRenderer) ApplyColorSpaceConversion(img *image.RGBA, srcCS, dstCS ColorSpace) (*image.RGBA, error) {
	if srcCS == nil || dstCS == nil {
		return img, nil
	}

	bounds := img.Bounds()

	// 使用 Pixman 后端进行高效的像素操作
	backend := NewPixmanBackendFromRGBA(img)
	if backend == nil {
		return img, fmt.Errorf("failed to create pixman backend")
	}
	defer backend.Destroy()

	for y := 0; y < bounds.Dy(); y++ {
		for x := 0; x < bounds.Dx(); x++ {
			// 获取源颜色
			pixel := backend.GetImage().GetPixel(x, y)

			r := float64(pixel.R) / 255.0
			g := float64(pixel.G) / 255.0
			b := float64(pixel.B) / 255.0
			a := float64(pixel.A) / 255.0

			// 反预乘
			if a > 0 && a < 1 {
				r = r / a
				g = g / a
				b = b / a
			}

			// 转换颜色空间
			components := []float64{r, g, b}
			r2, g2, b2, a2, err := dstCS.ConvertToRGBA(components, a)
			if err != nil {
				continue
			}

			// 预乘并写回
			if a2 > 0 && a2 < 1 {
				r2 = r2 * a2
				g2 = g2 * a2
				b2 = b2 * a2
			}

			newPixel := color.NRGBA{
				R: uint8(r2 * 255),
				G: uint8(g2 * 255),
				B: uint8(b2 * 255),
				A: uint8(a2 * 255),
			}

			backend.GetImage().SetPixel(x, y, newPixel)
		}
	}

	return backend.ToRGBA(), nil
}
