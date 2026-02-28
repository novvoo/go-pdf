package gopdf

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/jpeg"
	"image/png"
	"os"
	"strings"
	"unicode"

	otcff "github.com/go-text/typesetting/opentype/api/font/cff"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// PDFReader 用于读取和渲染 PDF 文件
type PDFReader struct {
	pdfPath        string
	resourceCache  map[int]*Resources // 页面资源缓存
	contextCache   *model.Context     // PDF 上下文缓存
	pageCountCache int                // 页数缓存
	pageDimsCache  []PageInfo         // 页面尺寸缓存
}

// NewPDFReader 创建新的 PDF 读取器
func NewPDFReader(pdfPath string) *PDFReader {
	return &PDFReader{
		pdfPath:        pdfPath,
		resourceCache:  make(map[int]*Resources),
		pageCountCache: -1, // -1 表示未缓存
	}
}

// Close 关闭 PDF 读取器并清理缓存
func (r *PDFReader) Close() error {
	r.resourceCache = nil
	r.contextCache = nil
	r.pageDimsCache = nil
	return nil
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

	// 使用 go-pdf 创建渲染表面
	surface := NewImageSurface(FormatARGB32, width, height)
	defer surface.Destroy()

	gopdfCtx := NewContext(surface)
	defer gopdfCtx.Destroy()

	// 设置白色背景
	gopdfCtx.SetSourceRGB(1, 1, 1)
	gopdfCtx.Paint()

	// 缩放以匹配 DPI
	gopdfCtx.Scale(scale, scale)

	// 渲染 PDF 内容到 Gopdf context
	if err := renderPDFPageToGopdf(r.pdfPath, pageNum, gopdfCtx, widthPoints, heightPoints); err != nil {
		return fmt.Errorf("failed to render PDF page: %w", err)
	}

	// 直接使用 Gopdf 保存 PNG
	if imgSurf, ok := surface.(ImageSurface); ok {
		status := imgSurf.WriteToPNG(outputPath)
		if status != StatusSuccess {
			return fmt.Errorf("failed to write PNG: %v", status)
		}
		return nil
	}

	return fmt.Errorf("failed to convert surface to image surface")
}

// RenderPageToImage 将 PDF 页面渲染为 image.Image
// 优化：避免临时文件，直接从 surface 转换
func (r *PDFReader) RenderPageToImage(pageNum int, dpi float64) (image.Image, error) {
	if dpi == 0 {
		dpi = 150
	}

	// 使用缓存的页面数量
	pageCount, err := r.GetPageCount()
	if err != nil {
		return nil, fmt.Errorf("failed to get page count: %w", err)
	}

	if pageNum < 1 || pageNum > pageCount {
		return nil, fmt.Errorf("invalid page number: %d (total pages: %d)", pageNum, pageCount)
	}

	// 使用缓存的页面信息
	pageInfo, err := r.GetPageInfo(pageNum)
	if err != nil {
		return nil, fmt.Errorf("failed to get page info: %w", err)
	}

	widthPoints := pageInfo.Width
	heightPoints := pageInfo.Height

	// 根据 DPI 计算渲染尺寸
	scale := dpi / 72.0
	width := int(widthPoints * scale)
	height := int(heightPoints * scale)

	// 使用 go-pdf 创建渲染表面
	surface := NewImageSurface(FormatARGB32, width, height)
	if surface == nil {
		return nil, fmt.Errorf("failed to create image surface")
	}
	defer surface.Destroy()

	gopdfCtx := NewContext(surface)
	defer gopdfCtx.Destroy()

	// 设置白色背景
	gopdfCtx.SetSourceRGB(1, 1, 1)
	gopdfCtx.Paint()

	// 缩放以匹配 DPI
	gopdfCtx.Scale(scale, scale)

	// 渲染 PDF 内容到 Gopdf context
	if err := renderPDFPageToGopdf(r.pdfPath, pageNum, gopdfCtx, widthPoints, heightPoints); err != nil {
		return nil, fmt.Errorf("failed to render PDF page: %w", err)
	}

	// 优化：直接从 surface 转换，避免临时文件
	if imgSurf, ok := surface.(ImageSurface); ok {
		if goImg := imgSurf.GetGoImage(); goImg != nil {
			if rgba, ok := goImg.(*image.RGBA); ok {
				cloned := image.NewRGBA(rgba.Rect)
				copy(cloned.Pix, rgba.Pix)
				return cloned, nil
			}
			cloned := image.NewRGBA(goImg.Bounds())
			draw.Draw(cloned, cloned.Bounds(), goImg, goImg.Bounds().Min, draw.Src)
			return cloned, nil
		}
		return ConvertGopdfSurfaceToImage(imgSurf), nil
	}

	return nil, fmt.Errorf("failed to convert surface to image")
}

// GetPageCount 获取 PDF 的页数
// 优化：使用缓存避免重复读取
func (r *PDFReader) GetPageCount() (int, error) {
	if r.pageCountCache > 0 {
		return r.pageCountCache, nil
	}

	count, err := api.PageCountFile(r.pdfPath)
	if err != nil {
		return 0, err
	}

	r.pageCountCache = count
	return count, nil
}

// PageInfo 页面信息
type PageInfo struct {
	Width  float64
	Height float64
}

// TextElementInfo 文本元素信息
type TextElementInfo struct {
	Text             string
	RawText          string
	X                float64
	Y                float64
	Width            float64
	Height           float64
	FontName         string
	FontBaseName     string
	FontSize         float64
	HasToUnicode     bool
	IsIdentity       bool
	CIDCount         int
	ReplacementCount int
	ToUnicodeHit     int
	GlyphNameHit     int
	IdentityASCIIHit int
}

// ImageElementInfo 图片元素信息
type ImageElementInfo struct {
	Name   string
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// ExtractImageData 从 PDF 中提取图像数据
// 🔥 新增：完整的图像提取功能，支持解码和导出
func (r *PDFReader) ExtractImageData(pageNum int, imageName string) (*image.RGBA, error) {
	// 打开 PDF 文件并读取上下文
	ctx, err := api.ReadContextFile(r.pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read PDF context: %w", err)
	}

	// 获取页面字典
	pageDict, _, _, err := ctx.PageDict(pageNum, false)
	if err != nil {
		return nil, fmt.Errorf("failed to get page dict: %w", err)
	}

	// 提取资源
	resources := NewResources()
	if resourcesObj, found := pageDict.Find("Resources"); found {
		if err := loadResources(ctx, resourcesObj, resources); err != nil {
			return nil, fmt.Errorf("failed to load resources: %w", err)
		}
	}

	// 获取图像 XObject
	xobj := resources.GetXObject(imageName)
	if xobj == nil {
		return nil, fmt.Errorf("image %s not found", imageName)
	}

	if xobj.Subtype != "/Image" && xobj.Subtype != "Image" {
		return nil, fmt.Errorf("%s is not an image (subtype: %s)", imageName, xobj.Subtype)
	}

	// 解码图像数据
	return decodeImageXObject(xobj)
}

// decodeImageXObject 解码图像 XObject 为 RGBA 图像
// 🔥 修复：改进 ICCBased 和 Indexed 颜色空间的处理
func decodeImageXObject(xobj *XObject) (*image.RGBA, error) {
	if len(xobj.Stream) == 0 {
		return nil, fmt.Errorf("image stream is empty")
	}

	width := xobj.Width
	height := xobj.Height
	bpc := xobj.BitsPerComponent
	colorSpace := xobj.ColorSpace

	Debug("[image] Decoding image: %dx%d, BPC=%d, ColorSpace=%s, Stream=%d bytes\n",
		width, height, bpc, colorSpace, len(xobj.Stream))
	Debug("[image] ColorComponents=%d\n", xobj.ColorComponents)

	if filtersContain(xobj.Filters, "/DCTDecode", "DCTDecode") {
		img, err := decodeJPEGToRGBA(xobj.Stream)
		if err != nil {
			return nil, err
		}
		return applySMask(img, xobj)
	}

	// 根据颜色空间解码
	switch colorSpace {
	case "DeviceRGB", "/DeviceRGB":
		img, err := decodeDeviceRGB(xobj.Stream, width, height, bpc)
		if err != nil {
			return nil, err
		}
		return applySMask(img, xobj)
	case "DeviceGray", "/DeviceGray":
		img, err := decodeDeviceGray(xobj.Stream, width, height, bpc)
		if err != nil {
			return nil, err
		}
		return applySMask(img, xobj)
	case "DeviceCMYK", "/DeviceCMYK":
		img, err := decodeDeviceCMYK(xobj.Stream, width, height, bpc)
		if err != nil {
			return nil, err
		}
		return applySMask(img, xobj)
	case "ICCBased", "/ICCBased": // 🔥 修复：同时支持带斜杠和不带斜杠的格式
		// 🔥 修复：ICC 颜色空间，根据组件数判断实际颜色空间
		// 优先使用从 ICC profile 中解析出的 N 值
		numComponents := 0
		if xobj.ColorComponents > 0 {
			numComponents = xobj.ColorComponents
			Debug("[icc] ICCBased using pre-resolved N=%d\n", numComponents)
		} else {
			// 回退：通过数据大小推断
			// ⚠️ 警告：这种推断方法不可靠！
			// 问题：如果数据大小恰好是 width*height*4，可能是：
			// 1. CMYK 图像（4个颜色分量）
			// 2. RGB + Alpha 图像（3个颜色分量 + 1个alpha通道）
			// 3. RGB 图像 + padding
			if width > 0 && height > 0 && bpc == 8 {
				estimatedComponents := len(xobj.Stream) / (width * height)

				// 🔥 新增：更智能的推断逻辑
				// 如果推断出4个分量，但没有明确的CMYK标识，优先假设是RGB
				// 因为现代图像（特别是Mac截图）更常用RGB而非CMYK
				if estimatedComponents == 4 {
					Debug("[icc] Estimated 4 components, ambiguous; defaulting to RGB\n")
					// 默认使用RGB，除非有其他证据表明是CMYK
					numComponents = 3
				} else {
					numComponents = estimatedComponents
				}
			}
			Debug("[icc] ICCBased estimated components: %d\n", numComponents)
		}

		Debug("[icc] ICCBased final numComponents=%d\n", numComponents)

		if numComponents == 4 {
			debugPrintf("[decodeImageXObject] ICCBased with 4 components, treating as CMYK\n")
			img, err := decodeDeviceCMYK(xobj.Stream, width, height, bpc)
			if err != nil {
				return nil, err
			}
			return applySMask(img, xobj)
		} else if numComponents == 3 {
			debugPrintf("[decodeImageXObject] ICCBased with 3 components, treating as RGB\n")
			img, err := decodeDeviceRGB(xobj.Stream, width, height, bpc)
			if err != nil {
				return nil, err
			}
			return applySMask(img, xobj)
		} else if numComponents == 1 {
			debugPrintf("[decodeImageXObject] ICCBased with 1 component, treating as Gray\n")
			img, err := decodeDeviceGray(xobj.Stream, width, height, bpc)
			if err != nil {
				return nil, err
			}
			return applySMask(img, xobj)
		} else {
			// 默认尝试 RGB
			debugPrintf("[decodeImageXObject] ICCBased with unknown components (%d), trying RGB\n", numComponents)
			img, err := decodeDeviceRGB(xobj.Stream, width, height, bpc)
			if err != nil {
				return nil, err
			}
			return applySMask(img, xobj)
		}
	case "Indexed", "/Indexed": // 🔥 修复：同时支持带斜杠和不带斜杠的格式
		// 🔥 修复：索引颜色空间，使用提取的调色板
		debugPrintf("[decodeImageXObject] Indexed color space detected\n")

		if len(xobj.Palette) > 0 {
			debugPrintf("[decodeImageXObject] Using pre-loaded palette (%d bytes)\n", len(xobj.Palette))
			img, err := decodeIndexedColorSpace(xobj.Stream, width, height, bpc, xobj.Palette)
			if err == nil {
				return applySMask(img, xobj)
			}
			debugPrintf("[decodeImageXObject] Failed to decode Indexed with palette: %v, falling back\n", err)
		}

		// 尝试旧的通过数组提取（如果 Palette 字段未填充）
		if xobj.ColorSpaceArray != nil {
			if arr, ok := xobj.ColorSpaceArray.([]interface{}); ok && len(arr) >= 4 {
				// 尝试解析 lookup
				// 这里简单处理字符串 lookup，Stream lookup 应该已经被 loadXObject 处理到 Palette 中了
				lookup := arr[3]
				var palette []byte
				if str, ok := lookup.(types.StringLiteral); ok {
					palette = []byte(str)
				} else if str, ok := lookup.(types.HexLiteral); ok {
					palette = []byte(str)
				}

				if len(palette) > 0 {
					img, err := decodeIndexedColorSpace(xobj.Stream, width, height, bpc, palette)
					if err == nil {
						return applySMask(img, xobj)
					}
				}
			}
		}

		// 回退：将索引值作为灰度处理
		debugPrintf("[decodeImageXObject] Indexed color space: no palette available, using grayscale fallback\n")
		img, err := decodeDeviceGray(xobj.Stream, width, height, bpc)
		if err != nil {
			return nil, err
		}
		return applySMask(img, xobj)
	default:
		debugPrintf("[decodeImageXObject] Unknown color space %s, trying RGB\n", colorSpace)
		img, err := decodeDeviceRGB(xobj.Stream, width, height, bpc)
		if err != nil {
			return nil, err
		}
		return applySMask(img, xobj)
	}
}

// applySMask 应用软遮罩（如果存在）
func applySMask(img *image.RGBA, xobj *XObject) (*image.RGBA, error) {
	if xobj.SMask == nil {
		return img, nil
	}

	debugPrintf("[applySMask] Applying SMask to image (%dx%d)\n", xobj.Width, xobj.Height)

	// 解码 SMask 图像
	// SMask 通常是 DeviceGray
	maskData, err := decodeImageXObject(xobj.SMask)
	if err != nil {
		debugPrintf("[applySMask] Warning: Failed to decode SMask: %v\n", err)
		return img, nil // 失败时忽略 mask
	}

	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	maskBounds := maskData.Bounds()
	maskWidth := maskBounds.Dx()
	maskHeight := maskBounds.Dy()

	// 应用 mask 到 alpha 通道
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// 计算 mask 坐标 (简单缩放)
			mx := x * maskWidth / width
			my := y * maskHeight / height

			if mx >= maskWidth {
				mx = maskWidth - 1
			}
			if my >= maskHeight {
				my = maskHeight - 1
			}

			// 获取 mask 像素 (使用其红色通道作为 alpha 值，因为 mask 应该是灰度的)
			r, _, _, _ := maskData.At(mx, my).RGBA()
			maskVal := uint8(r >> 8)

			// 获取原图像素
			offset := img.PixOffset(x, y)

			// 更新 alpha
			// 注意：如果是预乘 alpha 格式，需要相应调整 RGB 值
			// image.RGBA 是非预乘的，但 Go 的 image 包处理可能会混淆
			// 手动设置 Pix 是最安全的

			// 现有 alpha
			currentAlpha := img.Pix[offset+3]

			// 混合 alpha: result = current * mask
			// Normalize to 0-1 range then multiply
			newAlpha := uint8(float64(currentAlpha) * float64(maskVal) / 255.0)

			img.Pix[offset+3] = newAlpha
		}
	}

	debugPrintf("[applySMask] SMask applied successfully\n")
	return img, nil
}

func filtersContain(filters []string, want ...string) bool {
	if len(filters) == 0 || len(want) == 0 {
		return false
	}
	for _, f := range filters {
		ff := strings.TrimSpace(f)
		for _, w := range want {
			if ff == w {
				return true
			}
		}
	}
	return false
}

func decodeJPEGToRGBA(jpegBytes []byte) (*image.RGBA, error) {
	if len(jpegBytes) == 0 {
		return nil, fmt.Errorf("empty jpeg")
	}
	im, err := jpeg.Decode(bytes.NewReader(jpegBytes))
	if err != nil {
		return nil, err
	}
	b := im.Bounds()
	rgba := image.NewRGBA(image.Rect(0, 0, b.Dx(), b.Dy()))
	draw.Draw(rgba, rgba.Bounds(), im, b.Min, draw.Src)
	return rgba, nil
}

// decodeDeviceRGB 解码 DeviceRGB 图像
func decodeDeviceRGB(data []byte, width, height, bpc int) (*image.RGBA, error) {
	return DecodeDeviceRGBPublic(data, width, height, bpc)
}

// DecodeDeviceRGBPublic 公开的RGB解码函数，供测试使用
func DecodeDeviceRGBPublic(data []byte, width, height, bpc int) (*image.RGBA, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	if bpc == 8 {
		// 🔥 修复：智能检测字节/像素
		// 支持3字节/像素（RGB）和4字节/像素（RGB+Alpha或RGB+padding）
		bytesPerPixel := len(data) / (width * height)

		if bytesPerPixel < 3 {
			return nil, fmt.Errorf("insufficient data: expected at least %d bytes (3 bpp), got %d", width*height*3, len(data))
		}

		Debug("[rgb] Bytes per pixel: %d (total: %d bytes for %dx%d)\n", bytesPerPixel, len(data), width, height)

		if bytesPerPixel == 3 {
			// 标准RGB：3字节/像素
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					srcIdx := (y*width + x) * 3
					dstIdx := img.PixOffset(x, y)
					img.Pix[dstIdx+0] = data[srcIdx+0] // R
					img.Pix[dstIdx+1] = data[srcIdx+1] // G
					img.Pix[dstIdx+2] = data[srcIdx+2] // B
					img.Pix[dstIdx+3] = 255            // A
				}
			}
		} else if bytesPerPixel == 4 {
			// 🔥 关键修复：4字节/像素，可能是RGB+Alpha或RGB+padding
			// 我们只取前3个字节作为RGB，忽略第4个字节
			Debug("[rgb] 4 bytes/pixel detected; treating as RGB (ignoring 4th byte)\n")

			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					srcIdx := (y*width + x) * 4
					dstIdx := img.PixOffset(x, y)
					img.Pix[dstIdx+0] = data[srcIdx+0] // R
					img.Pix[dstIdx+1] = data[srcIdx+1] // G
					img.Pix[dstIdx+2] = data[srcIdx+2] // B
					// 第4个字节可能是Alpha或padding，我们可以尝试使用它
					// 但为了安全起见，先假设是不透明的
					img.Pix[dstIdx+3] = 255 // A (假设不透明)

					// 🔥 可选：如果第4个字节看起来像alpha值，使用它
					// 这需要启发式判断，暂时保守处理
				}
			}
		} else {
			// 其他情况：尝试按3字节/像素处理
			Debug("[rgb] Unusual bytes per pixel: %d; attempting 3-byte stride\n", bytesPerPixel)
			expectedSize := width * height * 3
			if len(data) < expectedSize {
				return nil, fmt.Errorf("insufficient data: expected %d bytes, got %d", expectedSize, len(data))
			}

			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					srcIdx := (y*width + x) * 3
					dstIdx := img.PixOffset(x, y)
					img.Pix[dstIdx+0] = data[srcIdx+0] // R
					img.Pix[dstIdx+1] = data[srcIdx+1] // G
					img.Pix[dstIdx+2] = data[srcIdx+2] // B
					img.Pix[dstIdx+3] = 255            // A
				}
			}
		}
	} else {
		return nil, fmt.Errorf("unsupported bits per component: %d", bpc)
	}

	return img, nil
}

// decodeDeviceGray 解码 DeviceGray 图像
func decodeDeviceGray(data []byte, width, height, bpc int) (*image.RGBA, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	if bpc == 8 {
		// 8 位灰度
		expectedSize := width * height
		if len(data) < expectedSize {
			return nil, fmt.Errorf("insufficient data: expected %d bytes, got %d", expectedSize, len(data))
		}

		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				srcIdx := y*width + x
				dstIdx := img.PixOffset(x, y)
				// PDF DeviceGray: 0=黑色，255=白色（标准定义）
				gray := data[srcIdx]
				img.Pix[dstIdx+0] = gray
				img.Pix[dstIdx+1] = gray
				img.Pix[dstIdx+2] = gray
				img.Pix[dstIdx+3] = 255
			}
		}
	} else if bpc == 1 {
		// 1 位黑白
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				byteIdx := (y*width + x) / 8
				bitIdx := 7 - ((y*width + x) % 8)
				if byteIdx >= len(data) {
					break
				}
				bit := (data[byteIdx] >> bitIdx) & 1
				gray := uint8(0)
				if bit == 1 {
					gray = 255
				}
				dstIdx := img.PixOffset(x, y)
				img.Pix[dstIdx+0] = gray
				img.Pix[dstIdx+1] = gray
				img.Pix[dstIdx+2] = gray
				img.Pix[dstIdx+3] = 255
			}
		}
	} else {
		return nil, fmt.Errorf("unsupported bits per component: %d", bpc)
	}

	return img, nil
}

// decodeDeviceCMYK 解码 DeviceCMYK 图像
func decodeDeviceCMYK(data []byte, width, height, bpc int) (*image.RGBA, error) {
	return DecodeDeviceCMYKPublic(data, width, height, bpc)
}

// DecodeDeviceCMYKPublic 公开的CMYK解码函数，供测试使用
func DecodeDeviceCMYKPublic(data []byte, width, height, bpc int) (*image.RGBA, error) {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	if bpc == 8 {
		// 8 位每通道
		expectedSize := width * height * 4
		if len(data) < expectedSize {
			return nil, fmt.Errorf("insufficient data: expected %d bytes, got %d", expectedSize, len(data))
		}

		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				srcIdx := (y*width + x) * 4
				// PDF CMYK 标准定义：0 = 无墨水（白色），255 = 满墨水（全色）
				// 归一化到 [0, 1] 范围
				c := float64(data[srcIdx+0]) / 255.0
				m := float64(data[srcIdx+1]) / 255.0
				yy := float64(data[srcIdx+2]) / 255.0
				k := float64(data[srcIdx+3]) / 255.0

				// 标准 CMYK 到 RGB 转换公式
				// R = 255 × (1 - C) × (1 - K)
				// G = 255 × (1 - M) × (1 - K)
				// B = 255 × (1 - Y) × (1 - K)
				r := (1.0 - c) * (1.0 - k) * 255.0
				g := (1.0 - m) * (1.0 - k) * 255.0
				b := (1.0 - yy) * (1.0 - k) * 255.0

				// 确保值在 [0, 255] 范围内
				if r < 0 {
					r = 0
				} else if r > 255 {
					r = 255
				}
				if g < 0 {
					g = 0
				} else if g > 255 {
					g = 255
				}
				if b < 0 {
					b = 0
				} else if b > 255 {
					b = 255
				}

				dstIdx := img.PixOffset(x, y)
				img.Pix[dstIdx+0] = uint8(r)
				img.Pix[dstIdx+1] = uint8(g)
				img.Pix[dstIdx+2] = uint8(b)
				img.Pix[dstIdx+3] = 255
			}
		}
	} else {
		return nil, fmt.Errorf("unsupported bits per component: %d", bpc)
	}

	return img, nil
}

// decodeIndexedColorSpace 解码索引颜色空间图像
// 🔥 新增：支持 Indexed 颜色空间的调色板解码
func decodeIndexedColorSpace(data []byte, width, height, bpc int, palette []byte) (*image.RGBA, error) {
	debugPrintf("[decodeIndexedColorSpace] Decoding indexed image: %dx%d, BPC=%d, Palette size=%d\n", width, height, bpc, len(palette))

	// 调色板应该是 RGB (3字节/条目)
	// 虽然 PDF 支持 Base 颜色空间为其他 (如 CMYK)，但 RGB 最常见
	// 这里假设 Base 是 DeviceRGB (3字节)
	// 如果 Palette 大小不是 3 的倍数，需要注意
	bytesPerEntry := 3 // 默认 RGB

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	if bpc == 8 {
		expectedSize := width * height
		if len(data) < expectedSize {
			return nil, fmt.Errorf("insufficient data: expected %d bytes, got %d", expectedSize, len(data))
		}

		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				idxVal := data[y*width+x]

				// 查找调色板
				pIdx := int(idxVal) * bytesPerEntry

				r, g, b := uint8(0), uint8(0), uint8(0)

				if pIdx+2 < len(palette) {
					r = palette[pIdx]
					g = palette[pIdx+1]
					b = palette[pIdx+2]
				}

				dstIdx := img.PixOffset(x, y)
				img.Pix[dstIdx+0] = r
				img.Pix[dstIdx+1] = g
				img.Pix[dstIdx+2] = b
				img.Pix[dstIdx+3] = 255
			}
		}
	} else if bpc == 4 {
		// 4 bpc: 2 pixels per byte
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				byteIdx := (y*width + x) / 2
				isHigh := ((y*width + x) % 2) == 0

				if byteIdx >= len(data) {
					break
				}

				b := data[byteIdx]
				var idxVal uint8
				if isHigh {
					idxVal = (b >> 4) & 0x0F
				} else {
					idxVal = b & 0x0F
				}

				pIdx := int(idxVal) * bytesPerEntry
				r, g, b := uint8(0), uint8(0), uint8(0)

				if pIdx+2 < len(palette) {
					r = palette[pIdx]
					g = palette[pIdx+1]
					b = palette[pIdx+2]
				}

				dstIdx := img.PixOffset(x, y)
				img.Pix[dstIdx+0] = r
				img.Pix[dstIdx+1] = g
				img.Pix[dstIdx+2] = b
				img.Pix[dstIdx+3] = 255
			}
		}
	} else if bpc == 1 || bpc == 2 {
		// 支持 1 和 2 bpc 索引
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				// 获取 bit stream 中的值
				bitOffset := (y*width + x) * bpc
				byteIdx := bitOffset / 8
				bitShift := 8 - bpc - (bitOffset % 8)

				if byteIdx >= len(data) {
					break
				}

				mask := byte((1 << bpc) - 1)
				idxVal := (data[byteIdx] >> bitShift) & mask

				pIdx := int(idxVal) * bytesPerEntry
				r, g, bl := uint8(0), uint8(0), uint8(0)

				if pIdx+2 < len(palette) {
					r = palette[pIdx]
					g = palette[pIdx+1]
					bl = palette[pIdx+2]
				}

				dstIdx := img.PixOffset(x, y)
				img.Pix[dstIdx+0] = r
				img.Pix[dstIdx+1] = g
				img.Pix[dstIdx+2] = bl
				img.Pix[dstIdx+3] = 255
			}
		}
	} else {
		return nil, fmt.Errorf("unsupported bits per component for Indexed: %d", bpc)
	}

	return img, nil
}

// GetPageInfo 获取页面信息
// 优化：使用缓存避免重复读取
func (r *PDFReader) GetPageInfo(pageNum int) (PageInfo, error) {
	// 检查缓存
	if r.pageDimsCache != nil && pageNum > 0 && pageNum <= len(r.pageDimsCache) {
		return r.pageDimsCache[pageNum-1], nil
	}

	// 加载所有页面尺寸到缓存
	if r.pageDimsCache == nil {
		pageDims, err := api.PageDimsFile(r.pdfPath)
		if err != nil {
			return PageInfo{Width: 612, Height: 792}, fmt.Errorf("failed to get page dimensions: %w", err)
		}

		r.pageDimsCache = make([]PageInfo, len(pageDims))
		for i, dim := range pageDims {
			r.pageDimsCache[i] = PageInfo{
				Width:  dim.Width,
				Height: dim.Height,
			}
		}
	}

	if pageNum < 1 || pageNum > len(r.pageDimsCache) {
		return PageInfo{Width: 612, Height: 792}, nil // 默认 Letter 尺寸
	}

	return r.pageDimsCache[pageNum-1], nil
}

// ExtractPageElements 提取页面中的文本和图片元素
func (r *PDFReader) ExtractPageElements(pageNum int) ([]TextElementInfo, []ImageElementInfo) {
	var textElements []TextElementInfo
	var imageElements []ImageElementInfo

	// 打开 PDF 文件并读取上下文
	ctx, err := api.ReadContextFile(r.pdfPath)
	if err != nil {
		debugPrintf("Failed to read PDF context: %v\n", err)
		return textElements, imageElements
	}

	// 获取页面字典
	pageDict, _, _, err := ctx.PageDict(pageNum, false)
	if err != nil {
		debugPrintf("Failed to get page dict: %v\n", err)
		return textElements, imageElements
	}

	// 获取页面尺寸
	pageInfo, _ := r.GetPageInfo(pageNum)

	pageTransform := NewIdentityMatrix()
	pageTransform = pageTransform.Translate(0, pageInfo.Height)
	pageTransform = pageTransform.Scale(1, -1)

	if rotateObj, found := pageDict.Find("Rotate"); found {
		var rotation int
		switch v := rotateObj.(type) {
		case types.Integer:
			rotation = int(v)
		case types.Float:
			rotation = int(v)
		}

		if rotation != 0 {
			rotation = rotation % 360
			switch rotation {
			case 90:
				pageTransform = pageTransform.Translate(pageInfo.Width, 0)
				pageTransform = pageTransform.Rotate(1.5707963267948966)
			case 180:
				pageTransform = pageTransform.Translate(pageInfo.Width, pageInfo.Height)
				pageTransform = pageTransform.Rotate(3.141592653589793)
			case 270:
				pageTransform = pageTransform.Translate(0, pageInfo.Height)
				pageTransform = pageTransform.Rotate(4.71238898038469)
			}
		}
	}

	if cropBoxObj, found := pageDict.Find("CropBox"); found {
		if arr, ok := cropBoxObj.(types.Array); ok && len(arr) == 4 {
			var x1, y1 float64
			if v, ok := arr[0].(types.Float); ok {
				x1 = float64(v)
			} else if v, ok := arr[0].(types.Integer); ok {
				x1 = float64(v)
			}
			if v, ok := arr[1].(types.Float); ok {
				y1 = float64(v)
			} else if v, ok := arr[1].(types.Integer); ok {
				y1 = float64(v)
			}

			if x1 != 0 || y1 != 0 {
				pageTransform = pageTransform.Translate(-x1, -y1)
			}
		}
	}

	// 提取资源
	resources := NewResources()
	if resourcesObj, found := pageDict.Find("Resources"); found {
		if err := loadResources(ctx, resourcesObj, resources); err != nil {
			debugPrintf("Failed to load resources: %v\n", err)
		}
	}

	// 提取内容流
	contents, found := pageDict.Find("Contents")
	if !found {
		return textElements, imageElements
	}

	contentStreams, err := ExtractContentStreams(ctx, contents)
	if err != nil {
		debugPrintf("Failed to extract content streams: %v\n", err)
		return textElements, imageElements
	}

	// 合并所有内容流
	var allContent []byte
	for _, stream := range contentStreams {
		allContent = append(allContent, stream...)
		allContent = append(allContent, '\n')
	}

	// 解析操作符
	operators, err := ParseContentStream(allContent)
	if err != nil {
		debugPrintf("Failed to parse content stream: %v\n", err)
		return textElements, imageElements
	}

	// 分析操作符以提取文本和图片信息
	currentFont := ""
	baseFontSize := 0.0                     // Tf 操作符设置的基础字体大小
	currentMatrix := &Matrix{XX: 1, YY: 1}  // 单位矩阵
	textLineMatrix := &Matrix{XX: 1, YY: 1} // 文本行矩阵
	ctm := NewIdentityMatrix()              // 当前变换矩阵 (Current Transformation Matrix)

	// 图形状态栈，用于保存和恢复完整的图形状态
	type GraphicsState struct {
		ctm            *Matrix
		currentFont    string
		baseFontSize   float64
		currentMatrix  *Matrix
		textLineMatrix *Matrix
		fillColor      [3]float64
		strokeColor    [3]float64
		lineWidth      float64
		lineCap        int
		lineJoin       int
		miterLimit     float64
		dashPattern    []float64
		dashPhase      float64
	}
	var graphicsStateStack []*GraphicsState

	// 初始化图形状态
	fillColor := [3]float64{0, 0, 0}
	strokeColor := [3]float64{0, 0, 0}
	lineWidth := 1.0
	lineCap := 0
	lineJoin := 0
	miterLimit := 10.0
	var dashPattern []float64
	dashPhase := 0.0

	for _, op := range operators {
		// 跳过忽略的操作符
		if op.Name() == "IGNORE" {
			continue
		}

		switch op.Name() {
		case "q": // 保存图形状态
			// 保存完整的图形状态到栈
			graphicsStateStack = append(graphicsStateStack, &GraphicsState{
				ctm:            ctm.Clone(),
				currentFont:    currentFont,
				baseFontSize:   baseFontSize,
				currentMatrix:  currentMatrix.Clone(),
				textLineMatrix: textLineMatrix.Clone(),
				fillColor:      fillColor,
				strokeColor:    strokeColor,
				lineWidth:      lineWidth,
				lineCap:        lineCap,
				lineJoin:       lineJoin,
				miterLimit:     miterLimit,
				dashPattern:    append([]float64(nil), dashPattern...),
				dashPhase:      dashPhase,
			})
			debugPrintf("[DEBUG] q operator: Saved graphics state, stack depth=%d\n", len(graphicsStateStack))

		case "Q": // 恢复图形状态
			// 从栈中弹出并恢复完整的图形状态
			if len(graphicsStateStack) > 0 {
				state := graphicsStateStack[len(graphicsStateStack)-1]
				graphicsStateStack = graphicsStateStack[:len(graphicsStateStack)-1]
				ctm = state.ctm
				currentFont = state.currentFont
				baseFontSize = state.baseFontSize
				currentMatrix = state.currentMatrix
				textLineMatrix = state.textLineMatrix
				fillColor = state.fillColor
				strokeColor = state.strokeColor
				lineWidth = state.lineWidth
				lineCap = state.lineCap
				lineJoin = state.lineJoin
				miterLimit = state.miterLimit
				dashPattern = state.dashPattern
				dashPhase = state.dashPhase
				debugPrintf("[DEBUG] Q operator: Restored graphics state, stack depth=%d, CTM=%s\n",
					len(graphicsStateStack), ctm.String())
			} else {
				// 🔥 修复：栈为空时报错，符合 PDF 规范
				debugPrintf("[DEBUG] Q operator: ERROR - graphics state stack is empty (unmatched Q without q)\n")
				// 保持当前状态，但记录错误
				// 在生产环境中，可以考虑返回错误或设置错误标志
			}

		case "BT": // 开始文本对象
			// 🔥 修复：重置文本矩阵和文本行矩阵为单位矩阵
			// 同时重置文本状态（字符间距、单词间距等）
			currentMatrix = NewIdentityMatrix()
			textLineMatrix = NewIdentityMatrix()
			debugPrintf("[DEBUG] BT operator: Reset text matrices and text state\n")

		case "ET": // 结束文本对象
			debugPrintf("[DEBUG] ET operator: End text object\n")

		case "Tf": // 设置字体
			if tfOp, ok := op.(*OpSetFont); ok {
				currentFont = tfOp.FontName
				baseFontSize = tfOp.FontSize
				// 🔥 修复：验证字体是否存在
				font := resources.GetFont(currentFont)
				if font == nil {
					debugPrintf("[DEBUG] Tf operator: WARNING - Font %s not found in resources\n", currentFont)
				} else {
					debugPrintf("[DEBUG] Tf operator: Font=%s (BaseFont=%s), Size=%.2f\n",
						currentFont, font.BaseFont, baseFontSize)
				}
			}

		case "Tm": // 设置文本矩阵
			if tmOp, ok := op.(*OpSetTextMatrix); ok {
				currentMatrix = tmOp.Matrix.Clone()
				textLineMatrix = tmOp.Matrix.Clone()
				debugPrintf("[DEBUG] Tm operator: Matrix=%s\n", currentMatrix.String())
			}

		case "cm": // 连接变换矩阵
			if cmOp, ok := op.(*OpConcatMatrix); ok {
				// 更新当前变换矩阵：CTM' = CTM × cm
				ctm = ctm.Multiply(cmOp.Matrix)
				debugPrintf("[DEBUG] cm operator: Matrix=%s, new CTM=%s\n", cmOp.Matrix.String(), ctm.String())
			}

		case "Td": // 文本位置偏移
			if tdOp, ok := op.(*OpMoveTextPosition); ok {
				translation := &Matrix{XX: 1, YY: 1, X0: tdOp.Tx, Y0: tdOp.Ty}
				textLineMatrix = translation.Multiply(textLineMatrix)
				currentMatrix = textLineMatrix.Clone()
				debugPrintf("[DEBUG] Td operator: Tx=%.2f, Ty=%.2f, new X0=%.2f, Y0=%.2f\n",
					tdOp.Tx, tdOp.Ty, currentMatrix.X0, currentMatrix.Y0)
			}

		case "Tj", "TJ", "'", "\"": // 显示文本
			var text string
			var textDisplacement float64 // 文本位移（用于更新文本矩阵）
			var rawText string
			var originalCIDs []uint16
			var hasToUnicode bool
			var isIdentity bool
			var decodeStats TextDecodeStats
			decodedInTJ := false

			switch t := op.(type) {
			case *OpShowText:
				text = t.Text
			case *OpShowTextArray:
				font := resources.GetFont(currentFont)
				spaceAdvanceThreshold := baseFontSize * 0.25
				if spaceAdvanceThreshold < 2 {
					spaceAdvanceThreshold = 2
				}
				pendingSpace := false

				var out strings.Builder
				var lastRune rune
				hasLastRune := false

				appendDecoded := func(seg string) {
					if seg == "" {
						return
					}
					var first rune
					for _, r := range seg {
						first = r
						break
					}
					if pendingSpace && out.Len() > 0 && hasLastRune && !unicode.IsSpace(lastRune) && first != 0 && !unicode.IsSpace(first) &&
						(((unicode.IsLetter(lastRune) || unicode.IsDigit(lastRune)) && (unicode.IsLetter(first) || unicode.IsDigit(first))) ||
							((lastRune == ':' || lastRune == ';' || lastRune == ',' || lastRune == ')' || lastRune == ']' || lastRune == '}') && (unicode.IsLetter(first) || unicode.IsDigit(first)))) {
						out.WriteByte(' ')
						lastRune = ' '
						hasLastRune = true
					}
					pendingSpace = false
					out.WriteString(seg)
					for _, r := range seg {
						lastRune = r
						hasLastRune = true
					}
				}

				for _, elem := range t.Array {
					if s, ok := elem.(string); ok {
						rawText += s
						if font != nil {
							hasToUnicode = font.ToUnicodeMap != nil
							isIdentity = font.IsIdentity
							seg, cids, ds := decodeTextStringWithCIDs(s, font.ToUnicodeMap, font)
							if seg == "" && s != "" {
								seg = s
							}
							appendDecoded(seg)
							originalCIDs = append(originalCIDs, cids...)
							decodeStats.CIDCount += ds.CIDCount
							decodeStats.ToUnicodeHit += ds.ToUnicodeHit
							decodeStats.GlyphNameHit += ds.GlyphNameHit
							decodeStats.IdentityASCIIHit += ds.IdentityASCIIHit
							decodeStats.Replaced += ds.Replaced
							decodedInTJ = true
						} else {
							appendDecoded(s)
						}
					} else if num, ok := elem.(float64); ok {
						adjustment := -num / 1000.0 * baseFontSize
						textDisplacement += adjustment
						if adjustment >= spaceAdvanceThreshold {
							pendingSpace = true
						}
						debugPrintf("[DEBUG] TJ kerning: num=%.0f, adjustment=%.4f, cumulative=%.4f\n",
							num, adjustment, textDisplacement)
					} else if num, ok := elem.(int); ok {
						adjustment := -float64(num) / 1000.0 * baseFontSize
						textDisplacement += adjustment
						if adjustment >= spaceAdvanceThreshold {
							pendingSpace = true
						}
						debugPrintf("[DEBUG] TJ kerning: num=%d, adjustment=%.4f, cumulative=%.4f\n",
							num, adjustment, textDisplacement)
					}
				}

				if decodedInTJ {
					text = out.String()
				} else {
					text = rawText
				}
			case *OpShowTextNextLine:
				text = t.Text
			case *OpShowTextWithSpacing:
				text = t.Text
			}

			// 解码文本（处理CID字体和十六进制字符串）
			// 同时保存原始 CID 数组用于宽度计算
			if rawText == "" && text != "" {
				rawText = text
			}
			if text != "" && !decodedInTJ {
				font := resources.GetFont(currentFont)
				if font != nil {
					hasToUnicode = font.ToUnicodeMap != nil
					isIdentity = font.IsIdentity
					decoded, cids, ds := decodeTextStringWithCIDs(text, font.ToUnicodeMap, font)
					if decoded == "" && rawText != "" {
						decoded = rawText
					}
					text = decoded
					originalCIDs = cids
					decodeStats = ds
				} else {
					text = decodeTextString(text)
				}
			}

			if text != "" && currentMatrix != nil {
				// 应用当前变换矩阵 (CTM) 到文本矩阵
				// 这里文本位置是 (0, 0)，所以最终位置就是 (CTM × Tm) 的平移部分
				finalMatrix := ctm.Multiply(currentMatrix)

				x, y := pageTransform.Transform(finalMatrix.X0, finalMatrix.Y0)

				// 计算有效字体大小：基础大小 * 文本矩阵的垂直缩放
				// 文本矩阵的 YY 分量表示垂直缩放
				// 特殊情况：如果 Tf 设置的字体大小为 0，则直接使用文本矩阵的缩放作为字体大小
				effectiveFontSize := baseFontSize
				scale := currentMatrix.YY
				if scale < 0 {
					scale = -scale
				}
				if baseFontSize == 0 {
					// 当 Tf 设置字体大小为 0 时，字体大小完全由文本矩阵决定
					effectiveFontSize = scale
				} else {
					effectiveFontSize = baseFontSize * scale
				}

				debugPrintf("[DEBUG] Text element: baseFontSize=%.2f, scale=%.2f, effectiveFontSize=%.2f\n",
					baseFontSize, currentMatrix.YY, effectiveFontSize)

				baseFontName := ""
				if f := resources.GetFont(currentFont); f != nil {
					baseFontName = strings.TrimSpace(f.BaseFont)
				}

				textElements = append(textElements, TextElementInfo{
					Text:             text,
					RawText:          rawText,
					X:                x,
					Y:                y,
					FontName:         currentFont,
					FontBaseName:     baseFontName,
					FontSize:         effectiveFontSize,
					HasToUnicode:     hasToUnicode,
					IsIdentity:       isIdentity,
					CIDCount:         decodeStats.CIDCount,
					ReplacementCount: decodeStats.Replaced,
					ToUnicodeHit:     decodeStats.ToUnicodeHit,
					GlyphNameHit:     decodeStats.GlyphNameHit,
					IdentityASCIIHit: decodeStats.IdentityASCIIHit,
				})

				// 🔥 修复：改进文本宽度计算，考虑字体默认宽度和缺失宽度
				var textWidth float64
				font := resources.GetFont(currentFont)
				if font != nil && len(originalCIDs) > 0 {
					// 使用 CID 数组进行精确的字体宽度计算
					for _, cid := range originalCIDs {
						width := font.GetWidth(cid)
						// 🔥 修复：确保宽度不为 0
						if width == 0 {
							if font.DefaultWidth > 0 {
								width = font.DefaultWidth
							} else if font.MissingWidth > 0 {
								width = font.MissingWidth
							} else {
								width = 1000.0 // 使用 1 em 作为默认值
							}
						}
						textWidth += (width / 1000.0) * effectiveFontSize
					}
					debugPrintf("[DEBUG] Calculated text width from CIDs: %.2f (%d CIDs)\n", textWidth, len(originalCIDs))
				} else if font != nil {
					// 回退到基于字符数的估算
					// 🔥 修复：改进 CJK 字符宽度估算
					runeCount := 0
					totalWidthFactor := 0.0
					for _, r := range text {
						runeCount++
						// 更精确的 CJK 字符范围检测
						if (r >= 0x4E00 && r <= 0x9FFF) || // CJK统一表意文字
							(r >= 0x3400 && r <= 0x4DBF) || // CJK扩展A
							(r >= 0x20000 && r <= 0x2A6DF) || // CJK扩展B
							(r >= 0x2A700 && r <= 0x2B73F) || // CJK扩展C
							(r >= 0x2B740 && r <= 0x2B81F) || // CJK扩展D
							(r >= 0x2B820 && r <= 0x2CEAF) || // CJK扩展E
							(r >= 0xF900 && r <= 0xFAFF) || // CJK兼容表意文字
							(r >= 0x2F800 && r <= 0x2FA1F) || // CJK兼容表意文字补充
							(r >= 0x3040 && r <= 0x309F) || // 平假名
							(r >= 0x30A0 && r <= 0x30FF) || // 片假名
							(r >= 0xAC00 && r <= 0xD7AF) { // 韩文音节
							totalWidthFactor += 1.0 // CJK字符通常是全角
						} else if r >= 0xFF00 && r <= 0xFFEF {
							// 全角ASCII和半角片假名
							totalWidthFactor += 1.0
						} else {
							totalWidthFactor += 0.5 // 拉丁字符通常是半角
						}
					}
					if runeCount > 0 {
						textWidth = totalWidthFactor * effectiveFontSize
					} else {
						textWidth = 0
					}
					debugPrintf("[DEBUG] Estimated text width: %.2f (totalFactor=%.2f, runeCount=%d)\n",
						textWidth, totalWidthFactor, runeCount)
				} else {
					// 最后的回退：简单估算
					runeCount := float64(len([]rune(text)))
					textWidth = runeCount * effectiveFontSize * 0.5
					debugPrintf("[DEBUG] Fallback text width: %.2f (no font info)\n", textWidth)
				}

				if len(textElements) > 0 {
					last := &textElements[len(textElements)-1]
					last.Width = textWidth
					last.Height = effectiveFontSize
				}

				// 先应用字距调整，再应用文本宽度
				totalDisplacement := textWidth + textDisplacement
				if totalDisplacement != 0 {
					translation := &Matrix{XX: 1, YY: 1, X0: totalDisplacement, Y0: 0}
					currentMatrix = currentMatrix.Multiply(translation)
					debugPrintf("[DEBUG] Total displacement: %.2f (width=%.2f, kerning=%.2f), new X0=%.2f\n",
						totalDisplacement, textWidth, textDisplacement, currentMatrix.X0)
				}
			}

		case "Do": // 绘制 XObject（可能是图片）
			if doOp, ok := op.(*OpDoXObject); ok {
				xobj := resources.GetXObject(doOp.XObjectName)
				if xobj != nil && (xobj.Subtype == "/Image" || xobj.Subtype == "Image") {
					// 🔥 修复：使用完整的矩阵变换来计算图片位置和尺寸
					// PDF图像XObject占据单位正方形(0,0)到(1,1)
					// 需要通过CTM变换这四个角点来获取实际位置

					// 计算图片的四个角点在用户空间中的位置
					// 左下角 (0, 0)
					x0, y0 := ctm.Transform(0, 0)
					// 右下角 (1, 0)
					x1, y1 := ctm.Transform(1, 0)
					// 左上角 (0, 1)
					x2, y2 := ctm.Transform(0, 1)
					// 右上角 (1, 1)
					x3, y3 := ctm.Transform(1, 1)

					x0s, y0s := pageTransform.Transform(x0, y0)
					x1s, y1s := pageTransform.Transform(x1, y1)
					x2s, y2s := pageTransform.Transform(x2, y2)
					x3s, y3s := pageTransform.Transform(x3, y3)

					minXs := min(min(x0s, x1s), min(x2s, x3s))
					maxXs := max(max(x0s, x1s), max(x2s, x3s))
					minYs := min(min(y0s, y1s), min(y2s, y3s))
					maxYs := max(max(y0s, y1s), max(y2s, y3s))

					x := minXs
					y := minYs
					actualWidth := maxXs - minXs
					actualHeight := maxYs - minYs

					// 🔥 修复：添加图像流数据和完整的元数据
					imageElements = append(imageElements, ImageElementInfo{
						Name:   doOp.XObjectName,
						X:      x,
						Y:      y,
						Width:  actualWidth,
						Height: actualHeight,
					})

					debugPrintf("[DEBUG] Do operator: Image %s at (%.2f, %.2f), size: %.2fx%.2f (original: %dx%d)\n",
						doOp.XObjectName, x, y, actualWidth, actualHeight, xobj.Width, xobj.Height)
					debugPrintf("[DEBUG]   Corners: (%.2f,%.2f) (%.2f,%.2f) (%.2f,%.2f) (%.2f,%.2f)\n",
						x0, y0, x1, y1, x2, y2, x3, y3)
					debugPrintf("[DEBUG]   ColorSpace: %s, BitsPerComponent: %d, Stream size: %d bytes\n",
						xobj.ColorSpace, xobj.BitsPerComponent, len(xobj.Stream))
				}
			}

		// 图形状态操作符 - 用于提取元素时记录状态
		case "w": // 设置线宽
			if wOp, ok := op.(*OpSetLineWidth); ok {
				lineWidth = wOp.Width
				debugPrintf("[DEBUG] w operator: LineWidth=%.2f\n", lineWidth)
			}

		case "J": // 设置线端点样式
			if jOp, ok := op.(*OpSetLineCap); ok {
				lineCap = jOp.Cap
				debugPrintf("[DEBUG] J operator: LineCap=%d\n", lineCap)
			}

		case "j": // 设置线连接样式
			if jOp, ok := op.(*OpSetLineJoin); ok {
				lineJoin = jOp.Join
				debugPrintf("[DEBUG] j operator: LineJoin=%d\n", lineJoin)
			}

		case "M": // 设置斜接限制
			if mOp, ok := op.(*OpSetMiterLimit); ok {
				miterLimit = mOp.Limit
				debugPrintf("[DEBUG] M operator: MiterLimit=%.2f\n", miterLimit)
			}

		case "d": // 设置虚线模式
			if dOp, ok := op.(*OpSetDash); ok {
				dashPattern = dOp.Pattern
				dashPhase = dOp.Offset
				debugPrintf("[DEBUG] d operator: DashPattern=%v, Phase=%.2f\n", dashPattern, dashPhase)
			}

		case "rg": // 设置填充颜色 (RGB)
			if rgOp, ok := op.(*OpSetFillColorRGB); ok {
				fillColor[0] = rgOp.R
				fillColor[1] = rgOp.G
				fillColor[2] = rgOp.B
				debugPrintf("[DEBUG] rg operator: FillColor=(%.2f, %.2f, %.2f)\n", fillColor[0], fillColor[1], fillColor[2])
			}

		case "RG": // 设置描边颜色 (RGB)
			if rgOp, ok := op.(*OpSetStrokeColorRGB); ok {
				strokeColor[0] = rgOp.R
				strokeColor[1] = rgOp.G
				strokeColor[2] = rgOp.B
				debugPrintf("[DEBUG] RG operator: StrokeColor=(%.2f, %.2f, %.2f)\n", strokeColor[0], strokeColor[1], strokeColor[2])
			}
		}
	}

	return textElements, imageElements
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

// renderPDFPageToGopdf 将 PDF 页面内容渲染到 Gopdf context
func renderPDFPageToGopdf(pdfPath string, pageNum int, gopdfCtx Context, width, height float64) error {
	// 打开 PDF 文件并读取上下文
	ctx, err := api.ReadContextFile(pdfPath)
	if err != nil {
		return fmt.Errorf("failed to read PDF context: %w", err)
	}

	// 获取页面字典
	pageDict, _, _, err := ctx.PageDict(pageNum, false)
	if err != nil {
		return fmt.Errorf("failed to get page dict: %w", err)
	}

	// 保存 Gopdf 状态
	gopdfCtx.Save()
	defer gopdfCtx.Restore()

	// 设置裁剪区域，防止内容超出页面边界
	// 注意：裁剪应该在所有变换之后应用，否则会裁剪掉变换后的内容
	// 暂时禁用裁剪以调试渲染问题
	// gopdfCtx.Rectangle(0, 0, width, height)
	// gopdfCtx.Clip()

	// PDF 坐标系转换：PDF 使用左下角为原点，Y 轴向上
	// Gopdf 使用左上角为原点，Y 轴向下
	// 需要翻转 Y 轴并平移
	gopdfCtx.Translate(0, height)
	gopdfCtx.Scale(1, -1)

	// 处理页面的 MediaBox, CropBox, Rotate 等属性
	if err := applyPageTransformations(pageDict, gopdfCtx, width, height); err != nil {
		debugPrintf("Warning: failed to apply page transformations: %v\n", err)
	}

	gopdfCtx.SetLineWidth(1.0)
	gopdfCtx.SetLineCap(LineCapButt)
	gopdfCtx.SetLineJoin(LineJoinMiter)
	gopdfCtx.SetMiterLimit(10.0)

	// 创建渲染上下文
	renderCtx := NewRenderContext(gopdfCtx, width, height)

	// 提取页面资源
	if resourcesObj, found := pageDict.Find("Resources"); found {
		if err := loadResources(ctx, resourcesObj, renderCtx.Resources); err != nil {
			debugPrintf("Warning: failed to load resources: %v\n", err)
		}
	}

	// 提取页面内容流
	contents, found := pageDict.Find("Contents")
	if !found {
		debugPrintln("⚠️  Page has no Contents entry")
		return nil
	}

	// 解析并渲染内容流
	contentStreams, err := ExtractContentStreams(ctx, contents)
	if err != nil {
		return fmt.Errorf("failed to extract content streams: %w", err)
	}

	// 合并所有内容流
	var allContent []byte
	for _, stream := range contentStreams {
		allContent = append(allContent, stream...)
		allContent = append(allContent, '\n')
	}

	// 如果内容流为空或太小，PDF 可能没有矢量内容
	if len(allContent) < 10 {
		debugPrintln("⚠️  Content stream is empty or too small, PDF may have no vector content")
		return nil
	}

	// 解析操作符
	operators, err := ParseContentStream(allContent)
	if err != nil {
		return fmt.Errorf("failed to parse content stream: %w", err)
	}

	// 执行所有操作符
	debugPrintf("📊 Executing %d PDF operators...\n", len(operators))

	opCount := make(map[string]int)
	for _, op := range operators {
		// 跳过忽略的操作符
		if op.Name() == "IGNORE" {
			continue
		}

		opCount[op.Name()]++
		if err := op.Execute(renderCtx); err != nil {
			// 继续执行，不中断渲染
			debugPrintf("⚠️  Operator %s failed: %v\n", op.Name(), err)
		}
	}

	// 显示操作符统计
	debugPrintln("\n📈 Operator Statistics:")
	for opName, count := range opCount {
		if count > 0 {
			debugPrintf("   %s: %d\n", opName, count)
		}
	}

	// 渲染注释（在页面内容之后）
	annotations, err := ExtractAnnotations(ctx, pageDict)
	if err != nil {
		debugPrintf("⚠️  Failed to extract annotations: %v\n", err)
	} else if len(annotations) > 0 {
		debugPrintf("\n📌 Rendering %d annotations...\n", len(annotations))
		annotRenderer := NewAnnotationRenderer(gopdfCtx)
		for i, annot := range annotations {
			if err := annotRenderer.RenderAnnotation(annot); err != nil {
				debugPrintf("⚠️  Failed to render annotation %d: %v\n", i, err)
			}
		}
	}

	// 渲染表单字段（在注释之后）
	formFields, err := ExtractFormFields(ctx)
	if err != nil {
		debugPrintf("⚠️  Failed to extract form fields: %v\n", err)
	} else if len(formFields) > 0 {
		debugPrintf("\n📝 Rendering %d form fields...\n", len(formFields))
		formRenderer := NewFormRenderer(gopdfCtx)
		for i, field := range formFields {
			if err := formRenderer.RenderFormField(field); err != nil {
				debugPrintf("⚠️  Failed to render form field %d: %v\n", i, err)
			}
		}
	}

	return nil
}

// applyPageTransformations 应用页面级别的变换（旋转、裁剪等）
func applyPageTransformations(pageDict types.Dict, gopdfCtx Context, width, height float64) error {
	// 处理页面旋转
	if rotateObj, found := pageDict.Find("Rotate"); found {
		var rotation int
		switch v := rotateObj.(type) {
		case types.Integer:
			rotation = int(v)
		case types.Float:
			rotation = int(v)
		}

		// 应用旋转（90, 180, 270 度）
		if rotation != 0 {
			rotation = rotation % 360
			switch rotation {
			case 90:
				gopdfCtx.Translate(width, 0)
				gopdfCtx.Rotate(1.5707963267948966) // π/2
			case 180:
				gopdfCtx.Translate(width, height)
				gopdfCtx.Rotate(3.141592653589793) // π
			case 270:
				gopdfCtx.Translate(0, height)
				gopdfCtx.Rotate(4.71238898038469) // 3π/2
			}
		}
	}

	// 处理 CropBox（如果存在）
	if cropBoxObj, found := pageDict.Find("CropBox"); found {
		if arr, ok := cropBoxObj.(types.Array); ok && len(arr) == 4 {
			var x1, y1 float64
			if v, ok := arr[0].(types.Float); ok {
				x1 = float64(v)
			} else if v, ok := arr[0].(types.Integer); ok {
				x1 = float64(v)
			}
			if v, ok := arr[1].(types.Float); ok {
				y1 = float64(v)
			} else if v, ok := arr[1].(types.Integer); ok {
				y1 = float64(v)
			}

			// 应用裁剪框的平移
			if x1 != 0 || y1 != 0 {
				gopdfCtx.Translate(-x1, -y1)
			}
		}
	}

	return nil
}

// ExtractContentStreams 提取页面的所有内容流（公开函数）
func ExtractContentStreams(ctx *model.Context, contents types.Object) ([][]byte, error) {
	var streams [][]byte

	switch obj := contents.(type) {
	case types.IndirectRef:
		// 解引用
		derefObj, err := ctx.Dereference(obj)
		if err != nil {
			return nil, fmt.Errorf("failed to dereference contents: %w", err)
		}
		debugPrintf("   Dereferenced to: %T\n", derefObj)
		return ExtractContentStreams(ctx, derefObj)

	case types.StreamDict:
		// 单个流
		debugPrintf("   Decoding StreamDict...\n")
		debugPrintf("   Raw: %d bytes, Content: %d bytes\n", len(obj.Raw), len(obj.Content))

		// 如果 Content 为空但 Raw 不为空，需要解码
		if len(obj.Content) == 0 && len(obj.Raw) > 0 {
			debugPrintf("   Calling Decode()...\n")
			err := obj.Decode()
			if err != nil {
				debugPrintf("   ⚠️  Decode error: %v\n", err)
				return nil, fmt.Errorf("failed to decode stream: %w", err)
			}
			debugPrintf("   ✓ After decode: %d bytes\n", len(obj.Content))
		}

		if len(obj.Content) > 0 {
			streams = append(streams, obj.Content)
		} else {
			debugPrintf("   ⚠️  No content available\n")
		}

	case types.Array:
		// 多个流
		debugPrintf("   Processing array with %d items\n", len(obj))
		for i, item := range obj {
			debugPrintf("   Array item %d: %T\n", i, item)
			itemStreams, err := ExtractContentStreams(ctx, item)
			if err == nil {
				streams = append(streams, itemStreams...)
			} else {
				debugPrintf("   ⚠️  Error extracting item %d: %v\n", i, err)
			}
		}

	default:
		debugPrintf("   ⚠️  Unknown contents type: %T\n", obj)
	}

	return streams, nil
}

// loadResources 加载页面资源
func loadResources(ctx *model.Context, resourcesObj types.Object, resources *Resources) error {
	return loadResourcesWithDepth(ctx, resourcesObj, resources, 0)
}

// loadResourcesWithDepth 加载页面资源（带深度限制以防止循环引用）
func loadResourcesWithDepth(ctx *model.Context, resourcesObj types.Object, resources *Resources, depth int) error {
	// 防止无限递归（最大深度限制）
	const maxDepth = 20
	if depth > maxDepth {
		return fmt.Errorf("resource loading depth exceeded (possible circular reference)")
	}

	// 解引用资源对象
	if indRef, ok := resourcesObj.(types.IndirectRef); ok {
		derefObj, err := ctx.Dereference(indRef)
		if err != nil {
			return err
		}
		resourcesObj = derefObj
	}

	resourcesDict, ok := resourcesObj.(types.Dict)
	if !ok {
		return fmt.Errorf("resources is not a dictionary")
	}

	deref := func(obj types.Object) types.Object {
		if indRef, ok := obj.(types.IndirectRef); ok {
			derefObj, err := ctx.Dereference(indRef)
			if err == nil {
				return derefObj
			}
		}
		return obj
	}

	// 加载字体
	if fontsObj, found := resourcesDict.Find("Font"); found {
		fontsObj = deref(fontsObj)
		if fontsDict, ok := fontsObj.(types.Dict); ok {
			for fontName, fontObj := range fontsDict {
				if err := loadFont(ctx, fontName, fontObj, resources); err != nil {
					debugPrintf("Warning: failed to load font %s: %v\n", fontName, err)
				}
			}
		}
	}

	if csObj, found := resourcesDict.Find("ColorSpace"); found {
		if indRef, ok := csObj.(types.IndirectRef); ok {
			derefObj, err := ctx.Dereference(indRef)
			if err == nil {
				csObj = derefObj
			}
		}
		if csDict, ok := csObj.(types.Dict); ok {
			for csName, csVal := range csDict {
				cs, err := parseColorSpaceObject(ctx, csVal)
				if err != nil {
					continue
				}
				if cs != nil {
					resources.SetColorSpace(csName, cs)
				}
			}
		}
	}

	// 加载 XObjects
	if xobjectsObj, found := resourcesDict.Find("XObject"); found {
		xobjectsObj = deref(xobjectsObj)
		if xobjectsDict, ok := xobjectsObj.(types.Dict); ok {
			for xobjName, xobjObj := range xobjectsDict {
				if err := loadXObject(ctx, xobjName, xobjObj, resources); err != nil {
					debugPrintf("Warning: failed to load XObject %s: %v\n", xobjName, err)
				}
			}
		}
	}

	// 加载扩展图形状态
	if extGStateObj, found := resourcesDict.Find("ExtGState"); found {
		extGStateObj = deref(extGStateObj)
		if extGStateDict, ok := extGStateObj.(types.Dict); ok {
			for gsName, gsObj := range extGStateDict {
				if err := loadExtGState(ctx, gsName, gsObj, resources); err != nil {
					debugPrintf("Warning: failed to load ExtGState %s: %v\n", gsName, err)
				}
			}
		}
	}

	// 加载 Shading（渐变）
	if shadingObj, found := resourcesDict.Find("Shading"); found {
		shadingObj = deref(shadingObj)
		if shadingDict, ok := shadingObj.(types.Dict); ok {
			for shadingName, shadingObjItem := range shadingDict {
				if err := loadShading(ctx, shadingName, shadingObjItem, resources); err != nil {
					debugPrintf("Warning: failed to load Shading %s: %v\n", shadingName, err)
				}
			}
		}
	}

	return nil
}

func parseColorSpaceObject(ctx *model.Context, obj types.Object) (ColorSpace, error) {
	if indRef, ok := obj.(types.IndirectRef); ok {
		derefObj, err := ctx.Dereference(indRef)
		if err != nil {
			return nil, err
		}
		obj = derefObj
	}

	switch v := obj.(type) {
	case types.Name:
		return GetColorSpace(v.String()), nil
	case types.Array:
		if len(v) == 0 {
			return nil, fmt.Errorf("empty colorspace array")
		}
		head, ok := v[0].(types.Name)
		if !ok {
			return nil, fmt.Errorf("invalid colorspace head")
		}
		switch head.String() {
		case "/ICCBased", "ICCBased":
			if len(v) < 2 {
				return nil, fmt.Errorf("invalid ICCBased colorspace")
			}
			ref := v[1]
			if indRef, ok := ref.(types.IndirectRef); ok {
				derefObj, err := ctx.Dereference(indRef)
				if err != nil {
					return nil, err
				}
				ref = derefObj
			}
			streamDict, ok := ref.(types.StreamDict)
			if !ok {
				return nil, fmt.Errorf("ICCBased not stream")
			}
			if len(streamDict.Content) == 0 && len(streamDict.Raw) > 0 {
				_ = streamDict.Decode()
			}

			n := 3
			if nObj, found := streamDict.Find("N"); found {
				if iv, ok := nObj.(types.Integer); ok {
					n = int(iv)
				}
			}
			var alternate ColorSpace
			if altObj, found := streamDict.Find("Alternate"); found {
				if name, ok := altObj.(types.Name); ok {
					alternate = GetColorSpace(name.String())
				}
			}
			rangeVals := []float64(nil)
			if rObj, found := streamDict.Find("Range"); found {
				if arr, ok := rObj.(types.Array); ok {
					rangeVals = make([]float64, 0, len(arr))
					for _, it := range arr {
						if fv, ok := it.(types.Float); ok {
							rangeVals = append(rangeVals, float64(fv))
						} else if iv, ok := it.(types.Integer); ok {
							rangeVals = append(rangeVals, float64(iv))
						}
					}
				}
			}
			return &ICCBasedColorSpace{
				NumComponents: n,
				Alternate:     alternate,
				Range:         rangeVals,
				Metadata:      nil,
			}, nil

		case "/Indexed", "Indexed":
			if len(v) < 4 {
				return nil, fmt.Errorf("invalid Indexed colorspace")
			}
			base, err := parseColorSpaceObject(ctx, v[1])
			if err != nil {
				return nil, err
			}
			hi := 0
			if iv, ok := v[2].(types.Integer); ok {
				hi = int(iv)
			} else if fv, ok := v[2].(types.Float); ok {
				hi = int(fv)
			}
			lookupBytes := []byte(nil)
			lookupObj := v[3]
			if indRef, ok := lookupObj.(types.IndirectRef); ok {
				derefObj, err := ctx.Dereference(indRef)
				if err == nil {
					lookupObj = derefObj
				}
			}
			if sd, ok := lookupObj.(types.StreamDict); ok {
				if len(sd.Content) == 0 && len(sd.Raw) > 0 {
					_ = sd.Decode()
				}
				lookupBytes = sd.Content
			} else if hl, ok := lookupObj.(types.HexLiteral); ok {
				lookupBytes = []byte(hl)
			} else if sl, ok := lookupObj.(types.StringLiteral); ok {
				lookupBytes = []byte(sl)
			}
			return &IndexedColorSpace{
				Base:   base,
				HiVal:  hi,
				Lookup: lookupBytes,
			}, nil

		default:
			return GetColorSpace(head.String()), nil
		}
	default:
		return nil, fmt.Errorf("unsupported colorspace object %T", obj)
	}
}

func parseEncodingDict(encDict types.Dict, font *Font) {
	if font == nil {
		return
	}

	if baseEnc, found := encDict.Find("BaseEncoding"); found {
		if name, ok := baseEnc.(types.Name); ok {
			font.BaseEncoding = name.String()
			if font.Encoding == "" {
				font.Encoding = name.String()
			}
		}
	}

	diffsObj, found := encDict.Find("Differences")
	if !found {
		return
	}
	diffs, ok := diffsObj.(types.Array)
	if !ok {
		return
	}

	if font.CodeToGlyphName == nil {
		font.CodeToGlyphName = make(map[byte]string, 256)
	}

	code := -1
	for _, o := range diffs {
		switch v := o.(type) {
		case types.Integer:
			code = int(v.Value())
		case types.Name:
			if code >= 0 && code <= 255 {
				font.CodeToGlyphName[byte(code)] = v.String()
				code++
			}
		}
	}
}

// loadFont 加载字体资源
func loadFont(ctx *model.Context, fontName string, fontObj types.Object, resources *Resources) error {
	// 解引用
	if indRef, ok := fontObj.(types.IndirectRef); ok {
		derefObj, err := ctx.Dereference(indRef)
		if err != nil {
			return err
		}
		fontObj = derefObj
	}

	fontDict, ok := fontObj.(types.Dict)
	if !ok {
		return fmt.Errorf("font is not a dictionary")
	}

	font := &Font{
		Name: fontName,
	}

	if baseFont, found := fontDict.Find("BaseFont"); found {
		if name, ok := baseFont.(types.Name); ok {
			font.BaseFont = name.String()
		}
	}

	if subtype, found := fontDict.Find("Subtype"); found {
		if name, ok := subtype.(types.Name); ok {
			font.Subtype = name.String()
		}
	}

	if encoding, found := fontDict.Find("Encoding"); found {
		switch v := encoding.(type) {
		case types.Name:
			font.Encoding = v.String()
		case types.IndirectRef:
			if derefObj, err := ctx.Dereference(v); err == nil {
				if encDict, ok := derefObj.(types.Dict); ok {
					parseEncodingDict(encDict, font)
				} else if name, ok := derefObj.(types.Name); ok {
					font.Encoding = name.String()
				}
			}
		case types.Dict:
			parseEncodingDict(v, font)
		}
	}

	if font.Subtype == "/Type0" || font.Subtype == "Type0" {
		if descendantFontDict, err := getType0DescendantFontDict(ctx, fontDict); err == nil && descendantFontDict != nil {
			if csiObj, found := descendantFontDict.Find("CIDSystemInfo"); found {
				if indRef, ok := csiObj.(types.IndirectRef); ok {
					derefObj, err := ctx.Dereference(indRef)
					if err == nil {
						csiObj = derefObj
					}
				}
				if csiDict, ok := csiObj.(types.Dict); ok {
					registry := ""
					ordering := ""
					if rObj, found := csiDict.Find("Registry"); found {
						switch v := rObj.(type) {
						case types.StringLiteral:
							registry = string(v)
						case types.HexLiteral:
							registry = string(v)
						}
					}
					if oObj, found := csiDict.Find("Ordering"); found {
						switch v := oObj.(type) {
						case types.StringLiteral:
							ordering = string(v)
						case types.HexLiteral:
							ordering = string(v)
						}
					}
					if registry != "" && ordering != "" {
						font.CIDSystemInfo = registry + "-" + ordering
					}
				}
			}

			if fontDescriptorObj, found := descendantFontDict.Find("FontDescriptor"); found {
				if indRef, ok := fontDescriptorObj.(types.IndirectRef); ok {
					derefObj, err := ctx.Dereference(indRef)
					if err == nil {
						if fontDescriptorDict, ok := derefObj.(types.Dict); ok {
							if fontFileObj, found := fontDescriptorDict.Find("FontFile2"); found {
								if fontFileRef, ok := fontFileObj.(types.IndirectRef); ok {
									fontFileData, err := loadFontFileData(ctx, fontFileRef)
									if err == nil {
										font.EmbeddedFontData = fontFileData
										debugPrintf("✓ Loaded embedded TTF font data for font %s (%d bytes)\n", fontName, len(fontFileData))
									}
								}
							} else if fontFileObj, found := fontDescriptorDict.Find("FontFile3"); found {
								if fontFileRef, ok := fontFileObj.(types.IndirectRef); ok {
									fontFileData, err := loadFontFileData(ctx, fontFileRef)
									if err == nil {
										font.EmbeddedFontData = fontFileData
										debugPrintf("✓ Loaded embedded CFF font data for font %s (%d bytes)\n", fontName, len(fontFileData))
										if cff, err := otcff.Parse(font.EmbeddedFontData); err == nil {
											font.CFF = cff
										}
										if len(font.CIDToGIDMap) == 0 && !font.CIDToGIDMapIdentity {
											if mapping, fontMatrix, hasFontMatrix, err := parseCFFCIDToGIDMap(font.EmbeddedFontData); err == nil && len(mapping) > 0 {
												font.CIDToGIDMap = mapping
												font.FontMatrix = fontMatrix
												font.HasFontMatrix = hasFontMatrix
											}
										} else {
											if _, fontMatrix, hasFontMatrix, err := parseCFFCIDToGIDMap(font.EmbeddedFontData); err == nil {
												font.FontMatrix = fontMatrix
												font.HasFontMatrix = hasFontMatrix
											}
										}
									}
								}
							}

							if missingWidthObj, found := fontDescriptorDict.Find("MissingWidth"); found {
								if num, ok := missingWidthObj.(types.Integer); ok {
									font.MissingWidth = float64(num)
								} else if num, ok := missingWidthObj.(types.Float); ok {
									font.MissingWidth = float64(num)
								}
							}
						}
					}
				}
			}

			if cidToGidObj, found := descendantFontDict.Find("CIDToGIDMap"); found {
				mapping, identity, err := loadCIDToGIDMap(ctx, cidToGidObj)
				if err == nil {
					font.CIDToGIDMap = mapping
					font.CIDToGIDMapIdentity = identity
				}
			}
		}
	}

	// 加载字体文件数据（用于嵌入字体）
	if fontDescriptorObj, found := fontDict.Find("FontDescriptor"); found {
		if indRef, ok := fontDescriptorObj.(types.IndirectRef); ok {
			derefObj, err := ctx.Dereference(indRef)
			if err == nil {
				if fontDescriptorDict, ok := derefObj.(types.Dict); ok {
					// 尝试加载 FontFile2 (TTF) 或 FontFile3 (CFF)
					if fontFileObj, found := fontDescriptorDict.Find("FontFile2"); found {
						if fontFileRef, ok := fontFileObj.(types.IndirectRef); ok {
							fontFileData, err := loadFontFileData(ctx, fontFileRef)
							if err == nil {
								font.EmbeddedFontData = fontFileData
								debugPrintf("✓ Loaded embedded TTF font data for font %s (%d bytes)\n", fontName, len(fontFileData))
							} else {
								debugPrintf("Warning: failed to load FontFile2 data for font %s: %v\n", fontName, err)
							}
						}
					} else if fontFileObj, found := fontDescriptorDict.Find("FontFile3"); found {
						if fontFileRef, ok := fontFileObj.(types.IndirectRef); ok {
							fontFileData, err := loadFontFileData(ctx, fontFileRef)
							if err == nil {
								font.EmbeddedFontData = fontFileData
								debugPrintf("✓ Loaded embedded CFF font data for font %s (%d bytes)\n", fontName, len(fontFileData))
							} else {
								debugPrintf("Warning: failed to load FontFile3 data for font %s: %v\n", fontName, err)
							}
						}
					}
				}
			}
		}
	}

	hasUsefulToUnicode := font.ToUnicodeMap != nil && (len(font.ToUnicodeMap.Mappings) > 0 || len(font.ToUnicodeMap.Ranges) > 0)
	if (font.Subtype == "/Type1" || font.Subtype == "Type1" || font.Subtype == "/MMType1" || font.Subtype == "MMType1") &&
		!hasUsefulToUnicode &&
		len(font.CodeToGlyphName) == 0 &&
		len(font.EmbeddedFontData) > 0 {
		if enc, codeToGID, fontMatrix, hasFontMatrix, err := parseCFFEncoding(font.EmbeddedFontData); err == nil && len(enc) > 0 {
			font.CodeToGlyphName = enc
			font.CodeToGID = codeToGID
			font.FontMatrix = fontMatrix
			font.HasFontMatrix = hasFontMatrix
			if cff, err := otcff.Parse(font.EmbeddedFontData); err == nil {
				font.CFF = cff
			}
		}
	}

	// 加载 ToUnicode CMap（用于 CID 字体）
	if toUnicodeObj, found := fontDict.Find("ToUnicode"); found {
		var streamDict types.StreamDict
		var hasStream bool
		switch v := toUnicodeObj.(type) {
		case types.StreamDict:
			streamDict = v
			hasStream = true
		case types.IndirectRef:
			derefObj, err := ctx.Dereference(v)
			if err == nil {
				if sd, ok := derefObj.(types.StreamDict); ok {
					streamDict = sd
					hasStream = true
				}
			}
		}
		if hasStream {
			// 先解码流
			if len(streamDict.Content) == 0 && len(streamDict.Raw) > 0 {
				err := streamDict.Decode()
				if err != nil {
					debugPrintf("Warning: failed to decode ToUnicode stream for font %s: %v\n", fontName, err)
				}
			}

			// 解析 ToUnicode CMap
			if len(streamDict.Content) > 0 {
				cidMap, err := ParseToUnicodeCMap(streamDict.Content)
				if err == nil && cidMap != nil {
					font.ToUnicodeMap = cidMap
					debugPrintf("✓ Loaded ToUnicode CMap for font %s (%d mappings, %d ranges)\n",
						fontName, len(cidMap.Mappings), len(cidMap.Ranges))
				} else if err != nil {
					debugPrintf("Warning: failed to parse ToUnicode CMap for font %s: %v\n", fontName, err)
				}
			}
		}
	}

	// 先设置 Subtype，因为 loadFontWidths 需要它
	// （Subtype 已经在上面设置了）

	// 加载字形宽度信息
	if err := loadFontWidths(ctx, fontDict, font); err != nil {
		debugPrintf("Warning: failed to load font widths for %s: %v\n", fontName, err)
	} else {
		if font.Widths != nil {
			if font.Subtype == "/Type0" {
				debugPrintf("✓ Loaded font widths for %s: %d CID mappings, %d ranges, default=%.0f\n",
					fontName, len(font.Widths.CIDWidths), len(font.Widths.CIDRanges), font.DefaultWidth)
			} else {
				debugPrintf("✓ Loaded font widths for %s: %d widths (FirstChar=%d, LastChar=%d)\n",
					fontName, len(font.Widths.Widths), font.Widths.FirstChar, font.Widths.LastChar)
			}
		}
	}

	// 检查是否使用 Identity-H 或 Identity-V 编码
	if font.Encoding == "/Identity-H" || font.Encoding == "Identity-H" ||
		font.Encoding == "/Identity-V" || font.Encoding == "Identity-V" {
		font.IsIdentity = true
		debugPrintf("✓ Detected Identity encoding for font %s: %s\n", fontName, font.Encoding)
	}

	// 如果没有 ToUnicode，尝试从 poppler-data 加载
	if (font.Subtype == "/Type0" || font.Subtype == "Type0") && !isUsefulCIDToUnicodeMap(font.ToUnicodeMap) {
		primary := strings.TrimSpace(font.CIDSystemInfo)
		if strings.Contains(strings.ToLower(primary), "identity") {
			primary = ""
		}
		secondary := guessCIDRegistry(font.BaseFont)

		tryLoad := func(registry string) bool {
			registry = strings.TrimSpace(registry)
			if registry == "" {
				return false
			}
			debugPrintf("→ Trying to load CID map from poppler-data: %s for font %s\n", registry, fontName)
			cidMap, err := LoadCIDToUnicodeFromRegistry(registry)
			if err != nil || cidMap == nil || !isUsefulCIDToUnicodeMap(cidMap) {
				if err != nil {
					debugPrintf("Warning: failed to load CID map for %s: %v\n", registry, err)
				}
				return false
			}
			font.ToUnicodeMap = cidMap
			if font.CIDSystemInfo == "" {
				font.CIDSystemInfo = registry
			}
			debugPrintf("✓ Loaded CID map from poppler-data: %s (%d mappings)\n", registry, len(cidMap.Mappings))
			return true
		}

		if !tryLoad(primary) {
			_ = tryLoad(secondary)
		}
	}

	resources.AddFont(fontName, font)
	return nil
}

func isUsefulCIDToUnicodeMap(m *CIDToUnicodeMap) bool {
	return m != nil && (len(m.Mappings) > 0 || len(m.Ranges) > 0)
}

func getType0DescendantFontDict(ctx *model.Context, fontDict types.Dict) (types.Dict, error) {
	descendantFontsObj, found := fontDict.Find("DescendantFonts")
	if !found {
		return nil, fmt.Errorf("no DescendantFonts")
	}

	if indRef, ok := descendantFontsObj.(types.IndirectRef); ok {
		derefObj, err := ctx.Dereference(indRef)
		if err != nil {
			return nil, err
		}
		descendantFontsObj = derefObj
	}

	arr, ok := descendantFontsObj.(types.Array)
	if !ok || len(arr) == 0 {
		return nil, fmt.Errorf("DescendantFonts is empty")
	}

	df0 := arr[0]
	if indRef, ok := df0.(types.IndirectRef); ok {
		derefObj, err := ctx.Dereference(indRef)
		if err != nil {
			return nil, err
		}
		df0 = derefObj
	}

	dfDict, ok := df0.(types.Dict)
	if !ok {
		return nil, fmt.Errorf("descendant font is not dict")
	}

	return dfDict, nil
}

func loadCIDToGIDMap(ctx *model.Context, obj types.Object) ([]uint16, bool, error) {
	if indRef, ok := obj.(types.IndirectRef); ok {
		derefObj, err := ctx.Dereference(indRef)
		if err != nil {
			return nil, false, err
		}
		obj = derefObj
	}

	if name, ok := obj.(types.Name); ok {
		s := name.String()
		if strings.Contains(s, "Identity") {
			return nil, true, nil
		}
	}

	streamDict, ok := obj.(types.StreamDict)
	if !ok {
		return nil, false, fmt.Errorf("CIDToGIDMap is not stream")
	}
	if len(streamDict.Content) == 0 && len(streamDict.Raw) > 0 {
		if err := streamDict.Decode(); err != nil {
			return nil, false, err
		}
	}
	b := streamDict.Content
	if len(b) < 2 {
		return nil, false, nil
	}

	m := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		m = append(m, binary.BigEndian.Uint16(b[i:i+2]))
	}
	return m, false, nil
}

// guessCIDRegistry 从字体名称推断 CID 注册表
func guessCIDRegistry(fontName string) string {
	fontName = strings.ToLower(fontName)

	// 中文字体
	if strings.Contains(fontName, "simhei") || strings.Contains(fontName, "simsun") ||
		strings.Contains(fontName, "yahei") || strings.Contains(fontName, "nsimsun") ||
		strings.Contains(fontName, "fangsong") || strings.Contains(fontName, "kaiti") {
		return "Adobe-GB1"
	}

	// 繁体中文字体
	if strings.Contains(fontName, "mingliu") || strings.Contains(fontName, "pmingliu") ||
		strings.Contains(fontName, "dfkai") {
		return "Adobe-CNS1"
	}

	// 日文字体
	if strings.Contains(fontName, "gothic") || strings.Contains(fontName, "mincho") ||
		strings.Contains(fontName, "meiryo") || strings.Contains(fontName, "msmincho") ||
		strings.Contains(fontName, "msgothic") {
		return "Adobe-Japan1"
	}

	// 韩文字体
	if strings.Contains(fontName, "batang") || strings.Contains(fontName, "dotum") ||
		strings.Contains(fontName, "gulim") || strings.Contains(fontName, "malgun") {
		return "Adobe-Korea1"
	}

	return ""
}

// loadXObject 加载 XObject 资源
func loadXObject(ctx *model.Context, xobjName string, xobjObj types.Object, resources *Resources) error {
	// 解引用
	if indRef, ok := xobjObj.(types.IndirectRef); ok {
		derefObj, err := ctx.Dereference(indRef)
		if err != nil {
			return err
		}
		xobjObj = derefObj
	}

	streamDict, ok := xobjObj.(types.StreamDict)
	if !ok {
		return fmt.Errorf("XObject is not a stream")
	}

	xobj := &XObject{}

	// 获取子类型
	if subtype, found := streamDict.Find("Subtype"); found {
		if name, ok := subtype.(types.Name); ok {
			xobj.Subtype = name.String()
		}
	}

	// 解码流内容
	debugPrintf("[loadXObject] Decoding stream for %s...\n", xobjName)
	debugPrintf("[loadXObject] Raw stream length: %d bytes\n", len(streamDict.Raw))

	// 先尝试使用 DereferenceStreamDict
	decoded, _, err := ctx.DereferenceStreamDict(streamDict)
	if err != nil {
		debugPrintf("[loadXObject] ERROR: Failed to decode stream: %v\n", err)
		return fmt.Errorf("failed to decode XObject stream: %w", err)
	}

	if decoded != nil && len(decoded.Content) > 0 {
		xobj.Stream = decoded.Content
		debugPrintf("[loadXObject] Stream decoded via DereferenceStreamDict: %d bytes\n", len(xobj.Stream))
	} else {
		// 如果 DereferenceStreamDict 返回空内容，尝试直接解码
		debugPrintf("[loadXObject] DereferenceStreamDict returned empty, trying direct decode...\n")
		if len(streamDict.Content) == 0 && len(streamDict.Raw) > 0 {
			err := streamDict.Decode()
			if err != nil {
				debugPrintf("[loadXObject] ERROR: Direct decode failed: %v\n", err)
				return fmt.Errorf("failed to decode XObject stream: %w", err)
			}
		}
		xobj.Stream = streamDict.Content
		debugPrintf("[loadXObject] Stream decoded via direct Decode(): %d bytes\n", len(xobj.Stream))
	}

	// 🔥 新增:应用额外的图像滤镜(如果需要)
	// pdfcpu 的 Decode() 已经处理了 Filter 字段中的标准滤镜
	// 但对于某些特殊情况,我们可能需要额外处理
	filters := []string{}
	if filterObj, found := streamDict.Find("Filter"); found {
		// Filter 可以是单个名称或数组
		if name, ok := filterObj.(types.Name); ok {
			filters = append(filters, name.String())
		} else if arr, ok := filterObj.(types.Array); ok {
			for _, f := range arr {
				if name, ok := f.(types.Name); ok {
					filters = append(filters, name.String())
				}
			}
		}

		debugPrintf("[loadXObject] Filters detected: %v\n", filters)

		// 检查是否需要应用 Predictor
		if decodeParmsObj, found := streamDict.Find("DecodeParms"); found {
			var predictor int
			var columns int
			var colors int = 1
			var bitsPerComponent int = 8

			// DecodeParms 可以是字典或数组
			var decodeParms types.Dict
			if dict, ok := decodeParmsObj.(types.Dict); ok {
				decodeParms = dict
			} else if arr, ok := decodeParmsObj.(types.Array); ok {
				if len(arr) > 0 {
					if dict, ok := arr[0].(types.Dict); ok {
						decodeParms = dict
					}
				}
			}

			if decodeParms != nil {
				if p, found := decodeParms.Find("Predictor"); found {
					if num, ok := p.(types.Integer); ok {
						predictor = int(num)
					}
				}
				if c, found := decodeParms.Find("Columns"); found {
					if num, ok := c.(types.Integer); ok {
						columns = int(num)
					}
				}
				if c, found := decodeParms.Find("Colors"); found {
					if num, ok := c.(types.Integer); ok {
						colors = int(num)
					}
				}
				if b, found := decodeParms.Find("BitsPerComponent"); found {
					if num, ok := b.(types.Integer); ok {
						bitsPerComponent = int(num)
					}
				}

				// 应用 Predictor
				if predictor > 1 && columns > 0 {
					debugPrintf("[loadXObject] Applying predictor: %d (columns=%d, colors=%d, bpc=%d)\n",
						predictor, columns, colors, bitsPerComponent)
					predicted, err := ApplyPredictor(xobj.Stream, predictor, columns, colors, bitsPerComponent)
					if err == nil {
						xobj.Stream = predicted
						debugPrintf("[loadXObject] Predictor applied successfully: %d bytes\n", len(xobj.Stream))
					} else {
						debugPrintf("[loadXObject] Warning: Failed to apply predictor: %v\n", err)
					}
				}
			}
		}
	}
	xobj.Filters = filters

	// 根据子类型加载特定属性
	switch xobj.Subtype {
	case "/Form", "Form":
		// 加载表单 XObject 属性
		if bbox, found := streamDict.Find("BBox"); found {
			if arr, ok := bbox.(types.Array); ok {
				xobj.BBox = make([]float64, len(arr))
				for i, v := range arr {
					if num, ok := v.(types.Float); ok {
						xobj.BBox[i] = float64(num)
					} else if num, ok := v.(types.Integer); ok {
						xobj.BBox[i] = float64(num)
					}
				}
			}
		}

		if matrix, found := streamDict.Find("Matrix"); found {
			if arr, ok := matrix.(types.Array); ok && len(arr) == 6 {
				xobj.Matrix = &Matrix{}
				if v, ok := arr[0].(types.Float); ok {
					xobj.Matrix.XX = float64(v)
				}
				if v, ok := arr[1].(types.Float); ok {
					xobj.Matrix.YX = float64(v)
				}
				if v, ok := arr[2].(types.Float); ok {
					xobj.Matrix.XY = float64(v)
				}
				if v, ok := arr[3].(types.Float); ok {
					xobj.Matrix.YY = float64(v)
				}
				if v, ok := arr[4].(types.Float); ok {
					xobj.Matrix.X0 = float64(v)
				}
				if v, ok := arr[5].(types.Float); ok {
					xobj.Matrix.Y0 = float64(v)
				}
			}
		}

		if resObj, found := streamDict.Find("Resources"); found {
			xobj.Resources = NewResources()
			if err := loadResourcesWithDepth(ctx, resObj, xobj.Resources, 1); err != nil {
				debugPrintf("Warning: failed to load Form XObject resources %s: %v\n", xobjName, err)
			}
		}

		// 检查是否有 Group 属性（透明度组）
		if group, found := streamDict.Find("Group"); found {
			// 解引用 Group 字典
			if indRef, ok := group.(types.IndirectRef); ok {
				derefGroup, err := ctx.Dereference(indRef)
				if err == nil {
					group = derefGroup
				}
			}

			if groupDict, ok := group.(types.Dict); ok {
				// 检查是否为透明度组
				if subtype, found := groupDict.Find("S"); found {
					if name, ok := subtype.(types.Name); ok && name.String() == "/Transparency" {
						isolated := false
						knockout := false
						colorSpace := "DeviceRGB"

						// 读取 I (Isolated) 标志
						if i, found := groupDict.Find("I"); found {
							if b, ok := i.(types.Boolean); ok {
								isolated = bool(b)
							}
						}

						// 读取 K (Knockout) 标志
						if k, found := groupDict.Find("K"); found {
							if b, ok := k.(types.Boolean); ok {
								knockout = bool(b)
							}
						}

						// 读取 CS (ColorSpace)
						if cs, found := groupDict.Find("CS"); found {
							if name, ok := cs.(types.Name); ok {
								colorSpace = name.String()
							}
						}

						xobj.Group = NewTransparencyGroup(isolated, knockout, colorSpace)
						debugPrintf("[Group] Transparency group detected: Isolated=%v, Knockout=%v, CS=%s\n",
							isolated, knockout, colorSpace)
					}
				}
			}
		}

	case "/Image", "Image":
		// 加载图像 XObject 属性
		if width, found := streamDict.Find("Width"); found {
			if num, ok := width.(types.Integer); ok {
				xobj.Width = int(num)
			} else if num, ok := width.(types.Float); ok {
				xobj.Width = int(num)
			}
		}

		if height, found := streamDict.Find("Height"); found {
			if num, ok := height.(types.Integer); ok {
				xobj.Height = int(num)
			} else if num, ok := height.(types.Float); ok {
				xobj.Height = int(num)
			}
		}

		// 解析颜色空间
		colorSpaceFound := false
		if colorSpace, found := streamDict.Find("ColorSpace"); found {
			colorSpaceFound = true
			if name, ok := colorSpace.(types.Name); ok {
				xobj.ColorSpace = name.String()
				debugPrintf("[loadXObject] ColorSpace (Name): %s\n", xobj.ColorSpace)
			} else if arr, ok := colorSpace.(types.Array); ok {
				// ColorSpace 是数组，例如 [/ICCBased ...] 或 [/Indexed ...]
				xobj.ColorSpaceArray = arr
				if len(arr) > 0 {
					if name, ok := arr[0].(types.Name); ok {
						xobj.ColorSpace = name.String()
						debugPrintf("[loadXObject] ColorSpace (Array): %s, array length: %d\n", xobj.ColorSpace, len(arr))
					}
				}
			} else if indRef, ok := colorSpace.(types.IndirectRef); ok {
				// ColorSpace 可能是间接引用
				derefCS, err := ctx.Dereference(indRef)
				if err == nil {
					if name, ok := derefCS.(types.Name); ok {
						xobj.ColorSpace = name.String()
						debugPrintf("[loadXObject] ColorSpace (IndirectRef->Name): %s\n", xobj.ColorSpace)
					} else if arr, ok := derefCS.(types.Array); ok {
						xobj.ColorSpaceArray = arr
						if len(arr) > 0 {
							if name, ok := arr[0].(types.Name); ok {
								xobj.ColorSpace = name.String()
								debugPrintf("[loadXObject] ColorSpace (IndirectRef->Array): %s, array length: %d\n", xobj.ColorSpace, len(arr))
							}
						}
					}
				}
			}
		}

		// 🔥 修复：进一步解析 ColorSpace 数组以获取关键信息
		if xobj.ColorSpace == "/ICCBased" || xobj.ColorSpace == "ICCBased" {
			// 解析 ICCBased 数组以获取 N (颜色分量数) 和 Alternate (备用颜色空间)
			if arr, ok := xobj.ColorSpaceArray.(types.Array); ok && len(arr) > 1 {
				if indRef, ok := arr[1].(types.IndirectRef); ok {
					// 解引用 ICC profile stream
					obj, err := ctx.Dereference(indRef)
					if err == nil {
						if streamDict, ok := obj.(types.StreamDict); ok {
							// 获取 N (颜色分量数)
							if nObj, found := streamDict.Find("N"); found {
								if n, ok := nObj.(types.Integer); ok {
									xobj.ColorComponents = int(n)
									debugPrintf("[loadXObject] ICCBased profile has N=%d components\n", xobj.ColorComponents)
									Debug("[load] ICCBased profile N=%d\n", xobj.ColorComponents)
								}
							}

							// 🔥 新增：获取 Alternate (备用颜色空间)
							// Alternate 用于当 ICC profile 无法使用时的回退
							if altObj, found := streamDict.Find("Alternate"); found {
								if altName, ok := altObj.(types.Name); ok {
									altColorSpace := altName.String()
									debugPrintf("[loadXObject] ICCBased has Alternate colorspace: %s\n", altColorSpace)
									Debug("[load] ICCBased Alternate: %s\n", altColorSpace)

									// 🔥 关键修复：如果Alternate是DeviceRGB，强制使用RGB解码
									// 这可以避免将RGB图像误判为CMYK
									if altColorSpace == "/DeviceRGB" || altColorSpace == "DeviceRGB" {
										if xobj.ColorComponents == 0 {
											xobj.ColorComponents = 3
											debugPrintf("[loadXObject] Set ColorComponents=3 based on Alternate DeviceRGB\n")
											Debug("[load] Forced N=3 from Alternate DeviceRGB\n")
										}
									} else if altColorSpace == "/DeviceGray" || altColorSpace == "DeviceGray" {
										if xobj.ColorComponents == 0 {
											xobj.ColorComponents = 1
											debugPrintf("[loadXObject] Set ColorComponents=1 based on Alternate DeviceGray\n")
											Debug("[load] Forced N=1 from Alternate DeviceGray\n")
										}
									} else if altColorSpace == "/DeviceCMYK" || altColorSpace == "DeviceCMYK" {
										if xobj.ColorComponents == 0 {
											xobj.ColorComponents = 4
											debugPrintf("[loadXObject] Set ColorComponents=4 based on Alternate DeviceCMYK\n")
											Debug("[load] Forced N=4 from Alternate DeviceCMYK\n")
										}
									}
								}
							}
						}
					}
				}
			}
		} else if xobj.ColorSpace == "/Indexed" || xobj.ColorSpace == "Indexed" {
			// 解析 Indexed 数组以获取调色板
			if arr, ok := xobj.ColorSpaceArray.(types.Array); ok && len(arr) >= 4 {
				// [/Indexed base hival lookup]
				lookup := arr[3]

				// lookup 可以是 Stream (间接引用) 或 String
				if indRef, ok := lookup.(types.IndirectRef); ok {
					obj, err := ctx.Dereference(indRef)
					if err == nil {
						if streamDict, ok := obj.(types.StreamDict); ok {
							// 解码流
							if err := streamDict.Decode(); err == nil {
								xobj.Palette = streamDict.Content
								debugPrintf("[loadXObject] Loaded Indexed palette from stream: %d bytes\n", len(xobj.Palette))
							}
						} else if str, ok := obj.(types.StringLiteral); ok {
							xobj.Palette = []byte(str)
							debugPrintf("[loadXObject] Loaded Indexed palette from dereferenced string: %d bytes\n", len(xobj.Palette))
						} else if str, ok := obj.(types.HexLiteral); ok {
							xobj.Palette = []byte(str) // HexLiteral在pdfcpu中是binary
							debugPrintf("[loadXObject] Loaded Indexed palette from dereferenced hex string: %d bytes\n", len(xobj.Palette))
						}
					}
				} else if str, ok := lookup.(types.StringLiteral); ok {
					xobj.Palette = []byte(str)
					debugPrintf("[loadXObject] Loaded Indexed palette from string: %d bytes\n", len(xobj.Palette))
				} else if str, ok := lookup.(types.HexLiteral); ok {
					xobj.Palette = []byte(str)
					debugPrintf("[loadXObject] Loaded Indexed palette from hex string: %d bytes\n", len(xobj.Palette))
				}
			}
		}

		// 如果没有找到 ColorSpace，根据图像属性推断
		if !colorSpaceFound || xobj.ColorSpace == "" {
			// 根据 BitsPerComponent 推断颜色空间
			if xobj.BitsPerComponent == 1 {
				xobj.ColorSpace = "DeviceGray"
				debugPrintf("[loadXObject] ColorSpace not found, inferred DeviceGray (1-bit image)\n")
			} else if xobj.BitsPerComponent == 8 {
				xobj.ColorSpace = "DeviceRGB"
				debugPrintf("[loadXObject] ColorSpace not found, using default: DeviceRGB (8-bit image)\n")
			} else {
				xobj.ColorSpace = "DeviceRGB"
				debugPrintf("[loadXObject] ColorSpace not found, using default: DeviceRGB (%d-bit image)\n", xobj.BitsPerComponent)
			}
		}

		if bpc, found := streamDict.Find("BitsPerComponent"); found {
			if num, ok := bpc.(types.Integer); ok {
				xobj.BitsPerComponent = int(num)
			} else if num, ok := bpc.(types.Float); ok {
				xobj.BitsPerComponent = int(num)
			}
		}

		// 🔍 处理软遮罩 (SMask)
		if smaskObj, found := streamDict.Find("SMask"); found {
			debugPrintf("[loadXObject] Found SMask for image %s\n", xobjName)
			// SMask 可以是 /None 或者是一个 XObject 引用
			if name, ok := smaskObj.(types.Name); ok && name.String() == "/None" {
				debugPrintf("[loadXObject] SMask is /None, ignoring\n")
			} else if indRef, ok := smaskObj.(types.IndirectRef); ok {
				debugPrintf("[loadXObject] SMask is an indirect reference: %v, attempting load\n", indRef)
				// 🔥 修复：加载 SMask XObject
				// 我们需要手动加载这里的引用的 XObject，而不是通过 loadXObject (因为它会添加到 resources)
				// 这里实现一个简化的加载逻辑
				smaskXObj, err := loadSMaskXObject(ctx, indRef)
				if err == nil {
					xobj.SMask = smaskXObj
					debugPrintf("[loadXObject] Successfully loaded SMask: %dx%d\n", smaskXObj.Width, smaskXObj.Height)
				} else {
					debugPrintf("[loadXObject] Failed to load SMask: %v\n", err)
				}
			}
		}
	}

	resources.AddXObject(xobjName, xobj)
	return nil
}

// loadSMaskXObject 加载软遮罩 XObject (简化版 loadXObject)
func loadSMaskXObject(ctx *model.Context, indRef types.IndirectRef) (*XObject, error) {
	// 解引用
	obj, err := ctx.Dereference(indRef)
	if err != nil {
		return nil, err
	}

	streamDict, ok := obj.(types.StreamDict)
	if !ok {
		return nil, fmt.Errorf("SMask XObject is not a stream")
	}

	xobj := &XObject{
		Subtype: "Image", // SMask 总是 Image 或 Form (通常 Image)
	}

	// 读取基础属性
	if width, found := streamDict.Find("Width"); found {
		if num, ok := width.(types.Integer); ok {
			xobj.Width = int(num)
		}
	}
	if height, found := streamDict.Find("Height"); found {
		if num, ok := height.(types.Integer); ok {
			xobj.Height = int(num)
		}
	}
	if bpc, found := streamDict.Find("BitsPerComponent"); found {
		if num, ok := bpc.(types.Integer); ok {
			xobj.BitsPerComponent = int(num)
		}
	}

	// 读取颜色空间 (通常是 DeviceGray)
	if colorSpace, found := streamDict.Find("ColorSpace"); found {
		if name, ok := colorSpace.(types.Name); ok {
			xobj.ColorSpace = name.String()
		} else if indRef, ok := colorSpace.(types.IndirectRef); ok {
			derefCS, err := ctx.Dereference(indRef)
			if err == nil {
				if name, ok := derefCS.(types.Name); ok {
					xobj.ColorSpace = name.String()
				}
			}
		}
	}
	if xobj.ColorSpace == "" {
		xobj.ColorSpace = "DeviceGray" // 默认
	}

	// 解码流
	if err := streamDict.Decode(); err != nil {
		return nil, fmt.Errorf("failed to decode SMask stream: %w", err)
	}
	xobj.Stream = streamDict.Content

	// 可能需要应用 Filters (简略处理，假设 Decode 已处理)
	// 如果 pdfcpu 没处理 Filter，这里可能会有问题，但通常 Decode() 会处理

	return xobj, nil
}

// loadFontWidths 加载字体宽度信息
func loadFontWidths(ctx *model.Context, fontDict types.Dict, font *Font) error {
	// 对于 Type0 (CID) 字体，需要从 DescendantFonts 中读取宽度信息
	if font.Subtype == "/Type0" || font.Subtype == "Type0" {
		return loadCIDFontWidths(ctx, fontDict, font)
	}

	// 对于 Type1/TrueType 字体，直接从字体字典读取
	widths := &FontWidths{
		CIDWidths: make(map[uint16]float64),
	}

	// 读取 FirstChar 和 LastChar
	if firstCharObj, found := fontDict.Find("FirstChar"); found {
		if num, ok := firstCharObj.(types.Integer); ok {
			widths.FirstChar = int(num)
		}
	}

	if lastCharObj, found := fontDict.Find("LastChar"); found {
		if num, ok := lastCharObj.(types.Integer); ok {
			widths.LastChar = int(num)
		}
	}

	// 读取 Widths 数组
	if widthsObj, found := fontDict.Find("Widths"); found {
		// 解引用
		if indRef, ok := widthsObj.(types.IndirectRef); ok {
			derefObj, err := ctx.Dereference(indRef)
			if err == nil {
				widthsObj = derefObj
			}
		}

		if widthsArray, ok := widthsObj.(types.Array); ok {
			widths.Widths = make([]float64, len(widthsArray))
			for i, w := range widthsArray {
				if num, ok := w.(types.Integer); ok {
					widths.Widths[i] = float64(num)
				} else if num, ok := w.(types.Float); ok {
					widths.Widths[i] = float64(num)
				}
			}
			debugPrintf("✓ Loaded %d width values for font %s (FirstChar=%d, LastChar=%d)\n",
				len(widths.Widths), font.Name, widths.FirstChar, widths.LastChar)
		}
	}

	// 读取 MissingWidth（从 FontDescriptor）
	if fontDescriptorObj, found := fontDict.Find("FontDescriptor"); found {
		if indRef, ok := fontDescriptorObj.(types.IndirectRef); ok {
			derefObj, err := ctx.Dereference(indRef)
			if err == nil {
				if fontDescriptorDict, ok := derefObj.(types.Dict); ok {
					if missingWidthObj, found := fontDescriptorDict.Find("MissingWidth"); found {
						if num, ok := missingWidthObj.(types.Integer); ok {
							font.MissingWidth = float64(num)
						} else if num, ok := missingWidthObj.(types.Float); ok {
							font.MissingWidth = float64(num)
						}
					}
				}
			}
		}
	}

	font.Widths = widths
	return nil
}

// loadCIDFontWidths 加载 CID 字体的宽度信息
func loadCIDFontWidths(ctx *model.Context, fontDict types.Dict, font *Font) error {
	// Type0 字体的宽度信息在 DescendantFonts 中
	descendantFontsObj, found := fontDict.Find("DescendantFonts")
	if !found {
		return fmt.Errorf("no DescendantFonts in Type0 font")
	}

	// 解引用
	if indRef, ok := descendantFontsObj.(types.IndirectRef); ok {
		derefObj, err := ctx.Dereference(indRef)
		if err != nil {
			return err
		}
		descendantFontsObj = derefObj
	}

	// DescendantFonts 是一个数组，通常只有一个元素
	descendantFontsArray, ok := descendantFontsObj.(types.Array)
	if !ok || len(descendantFontsArray) == 0 {
		return fmt.Errorf("DescendantFonts is not an array or is empty")
	}

	// 获取第一个 descendant font
	descendantFontObj := descendantFontsArray[0]
	if indRef, ok := descendantFontObj.(types.IndirectRef); ok {
		derefObj, err := ctx.Dereference(indRef)
		if err != nil {
			return err
		}
		descendantFontObj = derefObj
	}

	descendantFontDict, ok := descendantFontObj.(types.Dict)
	if !ok {
		return fmt.Errorf("descendant font is not a dictionary")
	}

	widths := &FontWidths{
		CIDWidths: make(map[uint16]float64),
		CIDRanges: make([]CIDWidthRange, 0),
	}

	// 读取 DW (Default Width)
	if dwObj, found := descendantFontDict.Find("DW"); found {
		if num, ok := dwObj.(types.Integer); ok {
			font.DefaultWidth = float64(num)
		} else if num, ok := dwObj.(types.Float); ok {
			font.DefaultWidth = float64(num)
		}
		debugPrintf("✓ Default width for CID font %s: %.0f\n", font.Name, font.DefaultWidth)
	}

	// 🔥 修复：如果默认宽度为0或未设置，使用合理的默认值
	// PDF规范建议CID字体的默认宽度通常是1000（1 em）
	if font.DefaultWidth == 0 {
		font.DefaultWidth = 1000.0
		debugPrintf("✓ Using fallback default width for CID font %s: 1000\n", font.Name)
	}

	// 读取 W (Widths) 数组
	// 格式: [c1 c2 w] 或 [c [w1 w2 ... wn]]
	if wObj, found := descendantFontDict.Find("W"); found {
		// 解引用
		if indRef, ok := wObj.(types.IndirectRef); ok {
			derefObj, err := ctx.Dereference(indRef)
			if err == nil {
				wObj = derefObj
			}
		}

		if wArray, ok := wObj.(types.Array); ok {
			if err := parseCIDWidthsArray(wArray, widths); err != nil {
				debugPrintf("Warning: failed to parse CID widths array: %v\n", err)
			} else {
				debugPrintf("✓ Loaded CID widths for font %s: %d direct mappings, %d ranges\n",
					font.Name, len(widths.CIDWidths), len(widths.CIDRanges))
			}
		}
	}

	font.Widths = widths
	return nil
}

// parseCIDWidthsArray 解析 CID 字体的 W 数组
// 格式: [c1 c2 w] 表示 CID c1 到 c2 的宽度都是 w
// 格式: [c [w1 w2 ... wn]] 表示从 CID c 开始的连续 CID 的宽度
func parseCIDWidthsArray(wArray types.Array, widths *FontWidths) error {
	i := 0
	for i < len(wArray) {
		// 读取起始 CID
		startCIDObj := wArray[i]
		startCID, ok := getInteger(startCIDObj)
		if !ok {
			i++
			continue
		}

		if i+1 >= len(wArray) {
			break
		}

		// 检查下一个元素是数组还是整数
		nextObj := wArray[i+1]

		if nextArray, ok := nextObj.(types.Array); ok {
			// 格式: [c [w1 w2 ... wn]]
			for j, widthObj := range nextArray {
				if width, ok := getNumber(widthObj); ok {
					cid := uint16(startCID + int64(j))
					widths.CIDWidths[cid] = width
				}
			}
			i += 2
		} else {
			// 格式: [c1 c2 w]
			if i+2 >= len(wArray) {
				break
			}

			endCID, ok := getInteger(wArray[i+1])
			if !ok {
				i++
				continue
			}

			width, ok := getNumber(wArray[i+2])
			if !ok {
				i++
				continue
			}

			// 添加范围
			widths.CIDRanges = append(widths.CIDRanges, CIDWidthRange{
				StartCID: uint16(startCID),
				EndCID:   uint16(endCID),
				Width:    width,
			})

			i += 3
		}
	}

	return nil
}

// getInteger 从 PDF 对象获取整数值
func getInteger(obj types.Object) (int64, bool) {
	if num, ok := obj.(types.Integer); ok {
		return int64(num), true
	}
	return 0, false
}

// getNumber 从 PDF 对象获取数值（整数或浮点数）
func getNumber(obj types.Object) (float64, bool) {
	if num, ok := obj.(types.Integer); ok {
		return float64(num), true
	}
	if num, ok := obj.(types.Float); ok {
		return float64(num), true
	}
	return 0, false
}

// loadFontFileData 从间接引用加载字体文件数据
func loadFontFileData(ctx *model.Context, fontFileRef types.IndirectRef) ([]byte, error) {
	// 解引用字体文件对象
	fontFileObj, err := ctx.Dereference(fontFileRef)
	if err != nil {
		return nil, fmt.Errorf("failed to dereference font file: %w", err)
	}

	// 检查是否为流字典
	if streamDict, ok := fontFileObj.(types.StreamDict); ok {
		// 如果内容为空但原始数据存在，需要解码
		if len(streamDict.Content) == 0 && len(streamDict.Raw) > 0 {
			if err := streamDict.Decode(); err != nil {
				return nil, fmt.Errorf("failed to decode font file stream: %w", err)
			}
		}

		// 返回解码后的内容
		if len(streamDict.Content) > 0 {
			return streamDict.Content, nil
		}
		return nil, fmt.Errorf("font file stream is empty")
	}

	return nil, fmt.Errorf("font file is not a stream dictionary")
}

// loadExtGState 加载扩展图形状态
func loadExtGState(ctx *model.Context, gsName string, gsObj types.Object, resources *Resources) error {
	// 解引用
	if indRef, ok := gsObj.(types.IndirectRef); ok {
		derefObj, err := ctx.Dereference(indRef)
		if err != nil {
			return err
		}
		gsObj = derefObj
	}

	gsDict, ok := gsObj.(types.Dict)
	if !ok {
		return fmt.Errorf("ExtGState is not a dictionary")
	}

	extGState := make(map[string]interface{})

	// 提取常见的图形状态参数
	for key, value := range gsDict {
		switch v := value.(type) {
		case types.Float:
			extGState[key] = float64(v)
		case types.Integer:
			extGState[key] = int(v)
		case types.Name:
			extGState[key] = v.String()
		case types.Boolean:
			extGState[key] = bool(v)
		}
	}

	resources.AddExtGState(gsName, extGState)
	return nil
}

// ExtractPageText 从 PDF 页面提取文本内容（导出供外部使用）
func ExtractPageText(ctx *model.Context, pageNum int) (string, error) {
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
				textContent = ExtractTextFromStream(string(decoded.Content))
			}
		}

	case types.StreamDict:
		// 直接解码流内容
		decoded, _, err := ctx.DereferenceStreamDict(obj)
		if err == nil && decoded != nil {
			textContent = ExtractTextFromStream(string(decoded.Content))
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
					textContent += ExtractTextFromStream(string(decoded.Content)) + "\n"
				}
			}
		}
	}

	if textContent == "" {
		return "No extractable text found", nil
	}

	return textContent, nil
}

// ExtractTextFromStream 从 PDF 内容流中提取文本（导出供外部使用）
func ExtractTextFromStream(stream string) string {
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
				// 处理转义字符 - 保留所有特殊字符
				text = strings.ReplaceAll(text, "\\n", "\n")
				text = strings.ReplaceAll(text, "\\r", "\r")
				text = strings.ReplaceAll(text, "\\t", "\t")
				text = strings.ReplaceAll(text, "\\b", "\b")
				text = strings.ReplaceAll(text, "\\f", "\f")
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
					} else if stream[j] == '\'' {
						// ' 操作符：移到下一行并显示文本
						result.WriteString(text)
					} else if stream[j] == '"' {
						// " 操作符：设置间距并显示文本
						result.WriteString(text)
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
						// 处理转义字符 - 保留所有特殊字符
						text = strings.ReplaceAll(text, "\\n", "\n")
						text = strings.ReplaceAll(text, "\\r", "\r")
						text = strings.ReplaceAll(text, "\\t", "\t")
						text = strings.ReplaceAll(text, "\\b", "\b")
						text = strings.ReplaceAll(text, "\\f", "\f")
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
					// 不自动添加空格，保留原始文本
					i += 2
				}
			}
			continue
		}

		i++
	}

	text := result.String()
	// 不清理空白，保留原始格式
	return text
}

// ConvertGopdfSurfaceToImage 将 Gopdf surface 转换为 Go image.Image（导出供外部使用）
func ConvertGopdfSurfaceToImage(imgSurf ImageSurface) image.Image {
	if imgSurf == nil {
		return nil
	}
	if goImg := imgSurf.GetGoImage(); goImg != nil {
		if rgba, ok := goImg.(*image.RGBA); ok {
			cloned := image.NewRGBA(rgba.Rect)
			copy(cloned.Pix, rgba.Pix)
			return cloned
		}
		cloned := image.NewRGBA(goImg.Bounds())
		draw.Draw(cloned, cloned.Bounds(), goImg, goImg.Bounds().Min, draw.Src)
		return cloned
	}
	data := imgSurf.GetData()
	stride := imgSurf.GetStride()
	width := imgSurf.GetWidth()
	height := imgSurf.GetHeight()

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := y*stride + x*4
			// Gopdf 使用 BGRA 预乘 alpha 格式
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

// ConvertPDFPageToImage 使用 Gopdf 将 PDF 页面转换为图像的辅助函数
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

// FontInfo 字体信息
type FontInfo struct {
	Name              string
	BaseFont          string
	Subtype           string
	Encoding          string
	IsIdentity        bool
	HasToUnicode      bool
	ToUnicodeMappings int
	ToUnicodeRanges   int
	CIDSystemInfo     string
	EmbeddedFontSize  int
}

// ExtractFontInfo 提取页面中使用的字体信息
func (r *PDFReader) ExtractFontInfo(pageNum int) []FontInfo {
	var fontInfos []FontInfo

	// 打开 PDF 文件并读取上下文
	ctx, err := api.ReadContextFile(r.pdfPath)
	if err != nil {
		debugPrintf("Failed to read PDF context: %v\n", err)
		return fontInfos
	}

	// 获取页面字典
	pageDict, _, _, err := ctx.PageDict(pageNum, false)
	if err != nil {
		debugPrintf("Failed to get page dict: %v\n", err)
		return fontInfos
	}

	// 提取资源
	resources := NewResources()
	if resourcesObj, found := pageDict.Find("Resources"); found {
		if err := loadResources(ctx, resourcesObj, resources); err != nil {
			debugPrintf("Failed to load resources: %v\n", err)
			return fontInfos
		}
	}

	// 遍历所有字体
	for name, font := range resources.Font {
		info := FontInfo{
			Name:             name,
			BaseFont:         font.BaseFont,
			Subtype:          font.Subtype,
			Encoding:         font.Encoding,
			IsIdentity:       font.IsIdentity,
			CIDSystemInfo:    font.CIDSystemInfo,
			EmbeddedFontSize: len(font.EmbeddedFontData),
		}

		if font.ToUnicodeMap != nil {
			info.HasToUnicode = true
			info.ToUnicodeMappings = len(font.ToUnicodeMap.Mappings)
			info.ToUnicodeRanges = len(font.ToUnicodeMap.Ranges)
		}

		fontInfos = append(fontInfos, info)
	}

	return fontInfos
}

// LoadResourcesPublic 公开的资源加载函数，供测试使用
func LoadResourcesPublic(ctx *model.Context, resourcesObj types.Object, resources *Resources) error {
	return loadResources(ctx, resourcesObj, resources)
}

// ReadContextFile 公开的上下文读取函数，供测试使用
func ReadContextFile(pdfPath string) (*model.Context, error) {
	return api.ReadContextFile(pdfPath)
}

// extractCIDsFromText 从文本字符串中提取 CID 数组
// extractCIDsFromText is currently unused
func _(text string) []uint16 {
	// 检查是否是十六进制字符串
	if len(text) >= 2 && text[0] == '<' && text[len(text)-1] == '>' {
		hexStr := text[1 : len(text)-1]
		hexStr = strings.ReplaceAll(hexStr, " ", "")

		// 转换十六进制到字节
		var result []byte
		for i := 0; i < len(hexStr); i += 2 {
			if i+1 < len(hexStr) {
				var b byte
				fmt.Sscanf(hexStr[i:i+2], "%02x", &b)
				result = append(result, b)
			}
		}

		if len(result) < 2 || len(result)%2 != 0 {
			return nil
		}

		// 提取 CID 数组（2 字节一个 CID）
		var cids []uint16
		for i := 0; i < len(result); i += 2 {
			cid := uint16(result[i])<<8 | uint16(result[i+1])
			cids = append(cids, cid)
		}
		return cids
	}

	// 普通字符串 - 转换为 CID 数组（字节码）
	var cids []uint16
	for i := 0; i < len(text); i++ {
		cids = append(cids, uint16(text[i]))
	}
	return cids
}

// min returns the minimum of two float64 values
func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// max returns the maximum of two float64 values
func max(a, b float64) float64 {
	if a > b {
		return a
	}
	return b
}
