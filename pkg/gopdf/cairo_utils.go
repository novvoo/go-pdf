package gopdf

import (
	"fmt"
	"image"
	"image/color"

	"github.com/novvoo/go-cairo/pkg/cairo"
)

// CairoImageConverter 提供 Cairo 图像格式转换工具
type CairoImageConverter struct{}

// NewCairoImageConverter 创建新的转换器
func NewCairoImageConverter() *CairoImageConverter {
	return &CairoImageConverter{}
}

// ImageToCairoSurface 将 Go image.Image 转换为 Cairo ImageSurface
// 正确处理 Stride 和预乘 Alpha
func (c *CairoImageConverter) ImageToCairoSurface(img image.Image, format cairo.Format) (cairo.ImageSurface, error) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	// 创建 Cairo surface
	surface := cairo.NewImageSurface(format, width, height)
	imgSurf, ok := surface.(cairo.ImageSurface)
	if !ok {
		surface.Destroy()
		return nil, fmt.Errorf("failed to create image surface")
	}

	// 获取 surface 数据和 stride
	data := imgSurf.GetData()
	stride := imgSurf.GetStride()

	// 根据格式转换
	switch format {
	case cairo.FormatARGB32:
		c.convertToARGB32Premultiplied(img, data, stride, bounds)
	case cairo.FormatRGB24:
		c.convertToRGB24(img, data, stride, bounds)
	case cairo.FormatA8:
		c.convertToA8(img, data, stride, bounds)
	case cairo.FormatA1:
		c.convertToA1(img, data, stride, bounds)
	default:
		imgSurf.Destroy()
		return nil, fmt.Errorf("unsupported format: %v", format)
	}

	// 标记 surface 已修改
	imgSurf.MarkDirty()

	return imgSurf, nil
}

// convertToARGB32Premultiplied 转换为 ARGB32 格式（预乘 Alpha）
// Cairo 使用 BGRA 字节序，且需要预乘 Alpha
func (c *CairoImageConverter) convertToARGB32Premultiplied(img image.Image, data []byte, stride int, bounds image.Rectangle) {
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, a := img.At(x, y).RGBA()

			// 转换为 8 位
			r8 := uint8(r >> 8)
			g8 := uint8(g >> 8)
			b8 := uint8(b >> 8)
			a8 := uint8(a >> 8)

			// 计算在 data 中的偏移量（使用 stride）
			rowOffset := (y - bounds.Min.Y) * stride
			colOffset := (x - bounds.Min.X) * 4
			offset := rowOffset + colOffset

			// 预乘 Alpha
			if a8 > 0 && a8 < 255 {
				// 预乘公式: color_premul = color * alpha / 255
				r8 = uint8(uint32(r8) * uint32(a8) / 255)
				g8 = uint8(uint32(g8) * uint32(a8) / 255)
				b8 = uint8(uint32(b8) * uint32(a8) / 255)
			} else if a8 == 0 {
				r8, g8, b8 = 0, 0, 0
			}

			// Cairo 使用 BGRA 字节序
			data[offset+0] = b8 // B
			data[offset+1] = g8 // G
			data[offset+2] = r8 // R
			data[offset+3] = a8 // A
		}
	}
}

// convertToRGB24 转换为 RGB24 格式（无 Alpha）
func (c *CairoImageConverter) convertToRGB24(img image.Image, data []byte, stride int, bounds image.Rectangle) {
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()

			rowOffset := (y - bounds.Min.Y) * stride
			colOffset := (x - bounds.Min.X) * 4
			offset := rowOffset + colOffset

			// RGB24 格式，第 4 字节未使用
			data[offset+0] = uint8(b >> 8) // B
			data[offset+1] = uint8(g >> 8) // G
			data[offset+2] = uint8(r >> 8) // R
			data[offset+3] = 0             // 未使用
		}
	}
}

// convertToA8 转换为 A8 格式（仅 Alpha 通道）
func (c *CairoImageConverter) convertToA8(img image.Image, data []byte, stride int, bounds image.Rectangle) {
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()

			rowOffset := (y - bounds.Min.Y) * stride
			colOffset := (x - bounds.Min.X)
			offset := rowOffset + colOffset

			data[offset] = uint8(a >> 8)
		}
	}
}

// convertToA1 转换为 A1 格式（1 位 Alpha）
func (c *CairoImageConverter) convertToA1(img image.Image, data []byte, stride int, bounds image.Rectangle) {
	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			_, _, _, a := img.At(x, y).RGBA()

			rowOffset := (y - bounds.Min.Y) * stride
			byteOffset := rowOffset + (x-bounds.Min.X)/8
			bitOffset := uint(7 - ((x - bounds.Min.X) % 8))

			// Alpha > 50% 视为不透明
			if a > 32768 {
				data[byteOffset] |= (1 << bitOffset)
			} else {
				data[byteOffset] &^= (1 << bitOffset)
			}
		}
	}
}

// CairoSurfaceToImage 将 Cairo ImageSurface 转换为 Go image.Image
// 正确处理 Stride 和反预乘 Alpha
func (c *CairoImageConverter) CairoSurfaceToImage(imgSurf cairo.ImageSurface) image.Image {
	data := imgSurf.GetData()
	stride := imgSurf.GetStride()
	width := imgSurf.GetWidth()
	height := imgSurf.GetHeight()
	format := imgSurf.GetFormat()

	// 创建 RGBA 图像
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	switch format {
	case cairo.FormatARGB32:
		c.convertFromARGB32Premultiplied(data, stride, img)
	case cairo.FormatRGB24:
		c.convertFromRGB24(data, stride, img)
	case cairo.FormatA8:
		c.convertFromA8(data, stride, img)
	case cairo.FormatA1:
		c.convertFromA1(data, stride, img)
	default:
		debugPrintf("Warning: Unsupported Cairo format %v, treating as ARGB32\n", format)
		c.convertFromARGB32Premultiplied(data, stride, img)
	}

	return img
}

// convertFromARGB32Premultiplied 从 ARGB32 预乘格式转换
func (c *CairoImageConverter) convertFromARGB32Premultiplied(data []byte, stride int, img *image.RGBA) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// 使用 stride 计算偏移量
			cairoOffset := y*stride + x*4

			// Cairo 使用 BGRA 预乘 alpha 格式
			b := data[cairoOffset+0]
			g := data[cairoOffset+1]
			r := data[cairoOffset+2]
			a := data[cairoOffset+3]

			// 反预乘 Alpha
			if a > 0 && a < 255 {
				alpha := uint32(a)
				r = uint8(uint32(r) * 255 / alpha)
				g = uint8(uint32(g) * 255 / alpha)
				b = uint8(uint32(b) * 255 / alpha)
			}

			// 写入 Go image（RGBA 格式）
			imgOffset := y*img.Stride + x*4
			img.Pix[imgOffset+0] = r
			img.Pix[imgOffset+1] = g
			img.Pix[imgOffset+2] = b
			img.Pix[imgOffset+3] = a
		}
	}
}

// convertFromRGB24 从 RGB24 格式转换
func (c *CairoImageConverter) convertFromRGB24(data []byte, stride int, img *image.RGBA) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			cairoOffset := y*stride + x*4

			b := data[cairoOffset+0]
			g := data[cairoOffset+1]
			r := data[cairoOffset+2]

			imgOffset := y*img.Stride + x*4
			img.Pix[imgOffset+0] = r
			img.Pix[imgOffset+1] = g
			img.Pix[imgOffset+2] = b
			img.Pix[imgOffset+3] = 255 // 完全不透明
		}
	}
}

// convertFromA8 从 A8 格式转换
func (c *CairoImageConverter) convertFromA8(data []byte, stride int, img *image.RGBA) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			cairoOffset := y*stride + x
			a := data[cairoOffset]

			imgOffset := y*img.Stride + x*4
			img.Pix[imgOffset+0] = 255
			img.Pix[imgOffset+1] = 255
			img.Pix[imgOffset+2] = 255
			img.Pix[imgOffset+3] = a
		}
	}
}

// convertFromA1 从 A1 格式转换
func (c *CairoImageConverter) convertFromA1(data []byte, stride int, img *image.RGBA) {
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			byteOffset := y*stride + x/8
			bitOffset := uint(7 - (x % 8))
			bit := (data[byteOffset] >> bitOffset) & 1

			var a uint8
			if bit == 1 {
				a = 255
			} else {
				a = 0
			}

			imgOffset := y*img.Stride + x*4
			img.Pix[imgOffset+0] = 255
			img.Pix[imgOffset+1] = 255
			img.Pix[imgOffset+2] = 255
			img.Pix[imgOffset+3] = a
		}
	}
}

// GetStrideForWidth 计算给定宽度和格式的 stride
// Cairo 要求 stride 必须是 4 字节对齐
func GetStrideForWidth(width int, format cairo.Format) int {
	var bytesPerPixel int

	switch format {
	case cairo.FormatARGB32, cairo.FormatRGB24:
		bytesPerPixel = 4
	case cairo.FormatA8:
		bytesPerPixel = 1
	case cairo.FormatA1:
		// A1 格式每个像素 1 位
		return ((width + 31) / 32) * 4 // 32 位对齐
	default:
		bytesPerPixel = 4
	}

	stride := width * bytesPerPixel
	// 确保 4 字节对齐
	if stride%4 != 0 {
		stride = ((stride / 4) + 1) * 4
	}

	return stride
}

// PremultiplyAlpha 预乘 Alpha 通道
func PremultiplyAlpha(c color.Color) color.RGBA {
	r, g, b, a := c.RGBA()

	if a == 0 {
		return color.RGBA{0, 0, 0, 0}
	}

	if a == 0xffff {
		return color.RGBA{
			R: uint8(r >> 8),
			G: uint8(g >> 8),
			B: uint8(b >> 8),
			A: 255,
		}
	}

	// 预乘公式
	alpha := uint32(a >> 8)
	return color.RGBA{
		R: uint8(uint32(r>>8) * alpha / 255),
		G: uint8(uint32(g>>8) * alpha / 255),
		B: uint8(uint32(b>>8) * alpha / 255),
		A: uint8(alpha),
	}
}

// UnpremultiplyAlpha 反预乘 Alpha 通道
func UnpremultiplyAlpha(c color.RGBA) color.RGBA {
	if c.A == 0 {
		return color.RGBA{0, 0, 0, 0}
	}

	if c.A == 255 {
		return c
	}

	// 反预乘公式
	alpha := uint32(c.A)
	return color.RGBA{
		R: uint8(uint32(c.R) * 255 / alpha),
		G: uint8(uint32(c.G) * 255 / alpha),
		B: uint8(uint32(c.B) * 255 / alpha),
		A: c.A,
	}
}
