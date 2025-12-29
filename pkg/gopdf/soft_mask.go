package gopdf

import (
	"fmt"
	"image"
)

// SoftMask 软遮罩（用于高级透明度效果）
type SoftMask struct {
	Type    string      // "Alpha" 或 "Luminosity"
	G       *XObject    // 遮罩的图形对象（Form XObject）
	BC      []float64   // 背景颜色
	TR      interface{} // 传递函数
	Surface Surface     // 渲染的遮罩 surface
	SubType string      // 子类型
	Matte   []float64   // 遮罩的背景色（用于预乘）
}

// NewSoftMask 创建新的软遮罩
func NewSoftMask(maskType string, g *XObject) *SoftMask {
	return &SoftMask{
		Type:    maskType,
		G:       g,
		BC:      []float64{0, 0, 0}, // 默认黑色背景
		Surface: nil,
	}
}

// RenderSoftMask 渲染软遮罩到 Gopdf surface
func (sm *SoftMask) RenderSoftMask(ctx *RenderContext, width, height int) error {
	if sm.G == nil {
		return fmt.Errorf("soft mask has no graphics object")
	}

	// 创建遮罩 surface
	// 对于 Alpha 类型，使用 A8 格式（只有 alpha 通道）
	// 对于 Luminosity 类型，使用 ARGB32 格式
	var format Format
	if sm.Type == "Alpha" {
		format = FormatA8
	} else {
		format = FormatARGB32
	}

	maskSurface := NewImageSurface(format, width, height)
	if maskSurface == nil {
		return fmt.Errorf("failed to create mask surface")
	}

	maskCtx := NewContext(maskSurface)
	if maskCtx.Status() != StatusSuccess {
		maskSurface.Destroy()
		return fmt.Errorf("failed to create mask context")
	}
	defer maskCtx.Destroy()

	// 设置背景色
	if len(sm.BC) >= 3 {
		maskCtx.SetSourceRGB(sm.BC[0], sm.BC[1], sm.BC[2])
		maskCtx.Paint()
	}

	// 创建临时渲染上下文
	tempCtx := &RenderContext{
		GopdfCtx:           maskCtx,
		GraphicsStack:      NewGraphicsStateStack(float64(width), float64(height)),
		MarkedContentStack: NewMarkedContentStack(),
		CurrentPath:        NewPath(),
		TextState:          NewTextState(),
		Resources:          ctx.Resources, // 共享资源
		XObjectCache:       make(map[string]Surface),
	}

	// 渲染遮罩内容
	if err := renderFormXObject(tempCtx, sm.G); err != nil {
		maskSurface.Destroy()
		return fmt.Errorf("failed to render soft mask: %w", err)
	}

	// 如果是 Luminosity 类型，需要转换为 alpha
	if sm.Type == "Luminosity" {
		sm.convertLuminosityToAlpha(maskSurface)
	}

	sm.Surface = maskSurface
	return nil
}

// convertLuminosityToAlpha 将亮度转换为 alpha 值
// 使用 Pixman 后端进行高效的像素操作
func (sm *SoftMask) convertLuminosityToAlpha(surface Surface) {
	imgSurface, ok := surface.(ImageSurface)
	if !ok {
		return
	}

	width := imgSurface.GetWidth()
	height := imgSurface.GetHeight()

	// 创建 Pixman 后端进行像素操作
	// 从 Gopdf surface 获取 RGBA 数据
	converter := NewGopdfImageConverter()
	img := converter.GopdfSurfaceToImage(imgSurface)
	rgba, ok := img.(*image.RGBA)
	if !ok {
		// 回退到原始方法
		sm.convertLuminosityToAlphaFallback(surface)
		return
	}

	backend := NewPixmanBackendFromRGBA(rgba)
	if backend == nil {
		sm.convertLuminosityToAlphaFallback(surface)
		return
	}
	defer backend.Destroy()

	// 使用 Pixman 处理像素
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			// 获取像素
			pixel := backend.GetImage().GetPixel(x, y)

			// 计算亮度（使用标准公式）
			r := float64(pixel.R)
			g := float64(pixel.G)
			b := float64(pixel.B)
			luminosity := 0.299*r + 0.587*g + 0.114*b

			// 设置 alpha 值
			pixel.A = uint8(luminosity)
			backend.GetImage().SetPixel(x, y, pixel)
		}
	}

	// 将处理后的数据写回 Gopdf surface
	resultRGBA := backend.ToRGBA()
	resultSurface, err := converter.ImageToGopdfSurface(resultRGBA, FormatARGB32)
	if err == nil {
		// 复制数据回原 surface
		srcData := resultSurface.GetData()
		dstData := imgSurface.GetData()
		copy(dstData, srcData)
		imgSurface.MarkDirty()
		resultSurface.Destroy()
	}
}

// convertLuminosityToAlphaFallback 回退方法
func (sm *SoftMask) convertLuminosityToAlphaFallback(surface Surface) {
	imgSurface, ok := surface.(ImageSurface)
	if !ok {
		return
	}

	data := imgSurface.GetData()
	stride := imgSurface.GetStride()
	width := imgSurface.GetWidth()
	height := imgSurface.GetHeight()

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := y*stride + x*4
			b := float64(data[offset+0])
			g := float64(data[offset+1])
			r := float64(data[offset+2])
			luminosity := 0.299*r + 0.587*g + 0.114*b
			data[offset+3] = uint8(luminosity)
		}
	}

	imgSurface.MarkDirty()
}

// ApplySoftMask 应用软遮罩到 Gopdf context
func (sm *SoftMask) ApplySoftMask(ctx Context) error {
	if sm.Surface == nil {
		return fmt.Errorf("soft mask not rendered")
	}

	// 创建 surface pattern 并应用遮罩
	pattern := NewPatternForSurface(sm.Surface)
	if pattern == nil {
		return fmt.Errorf("failed to create pattern from mask surface")
	}
	defer pattern.Destroy()

	// 使用 Gopdf 的 mask 功能应用遮罩
	ctx.Mask(pattern)

	return nil
}

// Destroy 销毁软遮罩资源
func (sm *SoftMask) Destroy() {
	if sm.Surface != nil {
		sm.Surface.Destroy()
		sm.Surface = nil
	}
}

// SoftMaskStack 软遮罩栈
type SoftMaskStack struct {
	stack []*SoftMask
}

// NewSoftMaskStack 创建新的软遮罩栈
func NewSoftMaskStack() *SoftMaskStack {
	return &SoftMaskStack{
		stack: make([]*SoftMask, 0),
	}
}

// Push 压入软遮罩
func (s *SoftMaskStack) Push(mask *SoftMask) {
	s.stack = append(s.stack, mask)
}

// Pop 弹出软遮罩
func (s *SoftMaskStack) Pop() *SoftMask {
	if len(s.stack) == 0 {
		return nil
	}
	mask := s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]
	return mask
}

// Current 获取当前软遮罩
func (s *SoftMaskStack) Current() *SoftMask {
	if len(s.stack) == 0 {
		return nil
	}
	return s.stack[len(s.stack)-1]
}

// IsEmpty 检查栈是否为空
func (s *SoftMaskStack) IsEmpty() bool {
	return len(s.stack) == 0
}

// Clear 清空栈
func (s *SoftMaskStack) Clear() {
	for _, mask := range s.stack {
		mask.Destroy()
	}
	s.stack = s.stack[:0]
}

// ApplyCurrentMask 应用当前软遮罩
func (s *SoftMaskStack) ApplyCurrentMask(ctx Context) error {
	mask := s.Current()
	if mask == nil {
		return nil
	}
	return mask.ApplySoftMask(ctx)
}
