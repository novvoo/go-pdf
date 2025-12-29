package gopdf

import (
	"image"
	"image/color"
)

// Pixman - 像素操作和图像后端
// 这是 Gopdf 的核心图像处理引擎的 Go 实现

// PixmanImage 表示一个像素图像，支持多种格式
type PixmanImage struct {
	data   []byte
	width  int
	height int
	stride int
	format PixmanFormat
}

// PixmanFormat 定义像素格式
type PixmanFormat int

const (
	PixmanFormatARGB32 PixmanFormat = iota
	PixmanFormatRGB24
	PixmanFormatA8
	PixmanFormatA1
	PixmanFormatRGB16_565
)

// NewPixmanImage 创建新的 Pixman 图像
func NewPixmanImage(format PixmanFormat, width, height int) *PixmanImage {
	stride := calculateStride(format, width)
	data := make([]byte, stride*height)
	return &PixmanImage{
		data:   data,
		width:  width,
		height: height,
		stride: stride,
		format: format,
	}
}

// calculateStride 计算每行的字节数
func calculateStride(format PixmanFormat, width int) int {
	switch format {
	case PixmanFormatARGB32, PixmanFormatRGB24:
		return width * 4
	case PixmanFormatA8:
		return width
	case PixmanFormatA1:
		return (width + 31) / 32 * 4
	case PixmanFormatRGB16_565:
		return width * 2
	default:
		return width * 4
	}
}

// GetPixel 获取指定位置的像素
func (img *PixmanImage) GetPixel(x, y int) color.NRGBA {
	if x < 0 || y < 0 || x >= img.width || y >= img.height {
		return color.NRGBA{}
	}

	offset := y*img.stride + x*4
	if offset+3 >= len(img.data) {
		return color.NRGBA{}
	}

	switch img.format {
	case PixmanFormatARGB32:
		// ARGB32 格式: [A, R, G, B]
		return color.NRGBA{
			R: img.data[offset+1],
			G: img.data[offset+2],
			B: img.data[offset+3],
			A: img.data[offset+0],
		}
	case PixmanFormatRGB24:
		// RGB24 格式: [X, R, G, B]
		return color.NRGBA{
			R: img.data[offset+1],
			G: img.data[offset+2],
			B: img.data[offset+3],
			A: 255,
		}
	default:
		return color.NRGBA{}
	}
}

// SetPixel 设置指定位置的像素
func (img *PixmanImage) SetPixel(x, y int, c color.NRGBA) {
	if x < 0 || y < 0 || x >= img.width || y >= img.height {
		return
	}

	offset := y*img.stride + x*4
	if offset+3 >= len(img.data) {
		return
	}

	switch img.format {
	case PixmanFormatARGB32:
		img.data[offset+0] = c.A
		img.data[offset+1] = c.R
		img.data[offset+2] = c.G
		img.data[offset+3] = c.B
	case PixmanFormatRGB24:
		img.data[offset+0] = 0
		img.data[offset+1] = c.R
		img.data[offset+2] = c.G
		img.data[offset+3] = c.B
	}
}

// Composite 执行图像合成操作
func (img *PixmanImage) Composite(op Operator, src *PixmanImage, mask *PixmanImage,
	srcX, srcY, maskX, maskY, destX, destY, width, height int) {

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			dx := destX + x
			dy := destY + y
			sx := srcX + x
			sy := srcY + y

			if dx < 0 || dy < 0 || dx >= img.width || dy >= img.height {
				continue
			}

			srcColor := src.GetPixel(sx, sy)
			dstColor := img.GetPixel(dx, dy)

			// 应用遮罩
			if mask != nil {
				mx := maskX + x
				my := maskY + y
				maskColor := mask.GetPixel(mx, my)
				srcColor.A = uint8((uint32(srcColor.A) * uint32(maskColor.A)) / 255)
			}

			// 执行混合
			result := PorterDuffBlend(srcColor, dstColor, op)
			img.SetPixel(dx, dy, result)
		}
	}
}

// Fill 填充矩形区域
func (img *PixmanImage) Fill(x, y, width, height int, c color.NRGBA) {
	for dy := 0; dy < height; dy++ {
		for dx := 0; dx < width; dx++ {
			img.SetPixel(x+dx, y+dy, c)
		}
	}
}

// ToRGBA 转换为 Go 标准 RGBA 图像
func (img *PixmanImage) ToRGBA() *image.RGBA {
	rgba := image.NewRGBA(image.Rect(0, 0, img.width, img.height))
	for y := 0; y < img.height; y++ {
		for x := 0; x < img.width; x++ {
			c := img.GetPixel(x, y)
			rgba.Set(x, y, c)
		}
	}
	return rgba
}

// FromRGBA 从 Go 标准 RGBA 图像创建
func FromRGBA(rgba *image.RGBA) *PixmanImage {
	bounds := rgba.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	img := NewPixmanImage(PixmanFormatARGB32, width, height)
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			c := rgba.At(x, y)
			if nrgba, ok := c.(color.NRGBA); ok {
				img.SetPixel(x, y, nrgba)
			} else {
				r, g, b, a := c.RGBA()
				img.SetPixel(x, y, color.NRGBA{
					R: uint8(r >> 8),
					G: uint8(g >> 8),
					B: uint8(b >> 8),
					A: uint8(a >> 8),
				})
			}
		}
	}
	return img
}
