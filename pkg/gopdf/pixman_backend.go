package gopdf

import (
	"fmt"
	"image"
	"image/color"

	"github.com/novvoo/go-cairo/pkg/cairo"
)

// PixmanBackend Pixman 图像后端
// 提供底层的像素操作和图像处理功能
type PixmanBackend struct {
	width  int
	height int
	format cairo.PixmanFormat
	image  *cairo.PixmanImage
}

// NewPixmanBackend 创建新的 Pixman 后端
func NewPixmanBackend(width, height int, format cairo.PixmanFormat) *PixmanBackend {
	img := cairo.NewPixmanImage(format, width, height)
	if img == nil {
		return nil
	}

	return &PixmanBackend{
		width:  width,
		height: height,
		format: format,
		image:  img,
	}
}

// NewPixmanBackendFromRGBA 从 RGBA 图像创建 Pixman 后端
func NewPixmanBackendFromRGBA(rgba *image.RGBA) *PixmanBackend {
	img := cairo.FromRGBA(rgba)
	if img == nil {
		return nil
	}

	bounds := rgba.Bounds()
	return &PixmanBackend{
		width:  bounds.Dx(),
		height: bounds.Dy(),
		format: cairo.PixmanFormatARGB32,
		image:  img,
	}
}

// GetImage 获取 Pixman 图像
func (pb *PixmanBackend) GetImage() *cairo.PixmanImage {
	return pb.image
}

// GetWidth 获取宽度
func (pb *PixmanBackend) GetWidth() int {
	return pb.width
}

// GetHeight 获取高度
func (pb *PixmanBackend) GetHeight() int {
	return pb.height
}

// GetFormat 获取格式
func (pb *PixmanBackend) GetFormat() cairo.PixmanFormat {
	return pb.format
}

// ToRGBA 转换为 RGBA 图像
func (pb *PixmanBackend) ToRGBA() *image.RGBA {
	if pb.image == nil {
		return nil
	}

	// 使用 PixmanImage 的 ToRGBA 方法
	return pb.image.ToRGBA()
}

// Clear 清空图像（填充透明色）
func (pb *PixmanBackend) Clear() {
	if pb.image == nil {
		return
	}

	// 使用 Fill 方法填充透明色
	pb.image.Fill(0, 0, pb.width, pb.height, color.NRGBA{0, 0, 0, 0})
}

// Fill 填充纯色
func (pb *PixmanBackend) Fill(c color.Color) {
	if pb.image == nil {
		return
	}

	r, g, b, a := c.RGBA()
	nrgba := color.NRGBA{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
		A: uint8(a >> 8),
	}

	// 使用 PixmanImage 的 Fill 方法
	pb.image.Fill(0, 0, pb.width, pb.height, nrgba)
}

// BlendPixel 混合单个像素
func (pb *PixmanBackend) BlendPixel(x, y int, c color.Color, op cairo.Operator) error {
	if x < 0 || x >= pb.width || y < 0 || y >= pb.height {
		return fmt.Errorf("pixel coordinates out of bounds: (%d, %d)", x, y)
	}

	if pb.image == nil {
		return fmt.Errorf("pixman image is nil")
	}

	// 获取目标像素
	dst := pb.image.GetPixel(x, y)

	// 转换源颜色为 NRGBA
	r, g, b, a := c.RGBA()
	src := color.NRGBA{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
		A: uint8(a >> 8),
	}

	// 使用 Cairo 的 Porter-Duff 混合
	result := cairo.PorterDuffBlend(src, dst, op)

	// 写回像素
	pb.image.SetPixel(x, y, result)

	return nil
}

// BlendRect 混合矩形区域
func (pb *PixmanBackend) BlendRect(x, y, width, height int, c color.Color, op cairo.Operator) error {
	for dy := 0; dy < height; dy++ {
		for dx := 0; dx < width; dx++ {
			if err := pb.BlendPixel(x+dx, y+dy, c, op); err != nil {
				// 继续处理其他像素
				continue
			}
		}
	}
	return nil
}

// Composite 合成另一个 Pixman 图像
func (pb *PixmanBackend) Composite(src *PixmanBackend, srcX, srcY, dstX, dstY, width, height int, op cairo.Operator) error {
	if src == nil || src.image == nil || pb.image == nil {
		return fmt.Errorf("invalid pixman backend")
	}

	// 使用 PixmanImage 的 Composite 方法
	pb.image.Composite(op, src.image, nil, srcX, srcY, 0, 0, dstX, dstY, width, height)

	return nil
}

// GetImageBackend 获取 ImageBackend（用于 Cairo 渲染）
func (pb *PixmanBackend) GetImageBackend() *cairo.ImageBackend {
	return cairo.NewImageBackend(pb.width, pb.height)
}

// Destroy 销毁资源
func (pb *PixmanBackend) Destroy() {
	// PixmanImage 不需要显式销毁
	pb.image = nil
}
