package gopdf

import (
	"fmt"
	"image"
	"image/color"

	"github.com/novvoo/go-cairo/pkg/cairo"
)

// ===== XObject 操作符 =====

// OpDoXObject Do - 绘制 XObject（表单或图像）
type OpDoXObject struct {
	XObjectName string
}

func (op *OpDoXObject) Name() string { return "Do" }

func (op *OpDoXObject) Execute(ctx *RenderContext) error {
	// 从资源中获取 XObject
	xobj := ctx.Resources.GetXObject(op.XObjectName)
	if xobj == nil {
		return fmt.Errorf("XObject %s not found", op.XObjectName)
	}

	switch xobj.Subtype {
	case "Form":
		return renderFormXObject(ctx, xobj)
	case "Image":
		return renderImageXObject(ctx, xobj)
	default:
		return fmt.Errorf("unsupported XObject subtype: %s", xobj.Subtype)
	}
}

// XObject 表示 PDF XObject
type XObject struct {
	Subtype          string             // "Form" 或 "Image"
	BBox             []float64          // 边界框 [x1 y1 x2 y2]
	Matrix           *Matrix            // 变换矩阵
	Resources        *Resources         // 资源字典（仅用于 Form）
	Stream           []byte             // 内容流
	Width            int                // 图像宽度
	Height           int                // 图像高度
	ColorSpace       string             // 颜色空间
	BitsPerComponent int                // 每个颜色分量的位数
	ImageData        image.Image        // 解码后的图像数据
	Group            *TransparencyGroup // 透明度组（仅用于 Form）
}

// renderFormXObject 渲染表单 XObject
func renderFormXObject(ctx *RenderContext, xobj *XObject) error {
	// 检查是否有透明度组
	if xobj.Group != nil {
		return renderTransparencyGroup(ctx, xobj)
	}

	// 普通表单 XObject 渲染
	// 保存图形状态
	ctx.CairoCtx.Save()
	ctx.GraphicsStack.Push()
	defer func() {
		ctx.CairoCtx.Restore()
		ctx.GraphicsStack.Pop()
	}()

	// 应用 XObject 的变换矩阵
	if xobj.Matrix != nil {
		xobj.Matrix.ApplyToCairoContext(ctx.CairoCtx)
	}

	// 应用边界框裁剪
	if len(xobj.BBox) == 4 {
		x1, y1, x2, y2 := xobj.BBox[0], xobj.BBox[1], xobj.BBox[2], xobj.BBox[3]
		ctx.CairoCtx.Rectangle(x1, y1, x2-x1, y2-y1)
		ctx.CairoCtx.Clip()
	}

	// 保存当前资源
	oldResources := ctx.Resources
	if xobj.Resources != nil {
		// 合并资源
		ctx.Resources = xobj.Resources
	}

	// 解析并执行内容流
	if len(xobj.Stream) > 0 {
		operators, err := ParseContentStream(xobj.Stream)
		if err != nil {
			return fmt.Errorf("failed to parse form XObject content: %w", err)
		}

		for _, op := range operators {
			if err := op.Execute(ctx); err != nil {
				// 继续执行其他操作符，不中断
				debugPrintf("Warning: operator %s failed: %v\n", op.Name(), err)
			}
		}
	}

	// 恢复资源
	ctx.Resources = oldResources

	return nil
}

// renderTransparencyGroup 渲染透明度组
func renderTransparencyGroup(ctx *RenderContext, xobj *XObject) error {
	group := xobj.Group

	debugPrintf("[TransparencyGroup] Rendering group: Isolated=%v, Knockout=%v\n",
		group.Isolated, group.Knockout)

	// 保存图形状态
	ctx.CairoCtx.Save()
	ctx.GraphicsStack.Push()
	defer func() {
		ctx.CairoCtx.Restore()
		ctx.GraphicsStack.Pop()
	}()

	// 应用 XObject 的变换矩阵
	if xobj.Matrix != nil {
		xobj.Matrix.ApplyToCairoContext(ctx.CairoCtx)
	}

	// 使用 Cairo push_group 创建隔离的合成表面
	// 这会创建一个临时的 surface 用于渲染组内容
	ctx.CairoCtx.PushGroup()

	// 应用边界框裁剪
	if len(xobj.BBox) == 4 {
		x1, y1, x2, y2 := xobj.BBox[0], xobj.BBox[1], xobj.BBox[2], xobj.BBox[3]
		ctx.CairoCtx.Rectangle(x1, y1, x2-x1, y2-y1)
		ctx.CairoCtx.Clip()
	}

	// 保存当前资源
	oldResources := ctx.Resources
	if xobj.Resources != nil {
		ctx.Resources = xobj.Resources
	}

	// 如果是 knockout 组，需要特殊处理
	// knockout 意味着组内对象不相互混合
	if group.Knockout {
		debugPrintf("[TransparencyGroup] Knockout mode enabled\n")
		// 在 knockout 模式下，每个对象都直接绘制到组 surface
		// 而不与之前的对象混合
		// 这需要为每个操作符创建单独的 group
		// 当前简化实现：仍然正常渲染，但记录 knockout 状态
	}

	// 解析并执行内容流
	if len(xobj.Stream) > 0 {
		operators, err := ParseContentStream(xobj.Stream)
		if err != nil {
			ctx.CairoCtx.PopGroupToSource() // 清理 group
			ctx.Resources = oldResources
			return fmt.Errorf("failed to parse transparency group content: %w", err)
		}

		for _, op := range operators {
			if err := op.Execute(ctx); err != nil {
				debugPrintf("Warning: operator %s failed in transparency group: %v\n", op.Name(), err)
			}
		}
	}

	// 恢复资源
	ctx.Resources = oldResources

	// 使用 Cairo pop_group_to_source 将组内容作为源
	ctx.CairoCtx.PopGroupToSource()

	// 应用当前图形状态的混合模式和透明度
	state := ctx.GetCurrentState()
	if state != nil {
		// 应用混合模式
		state.ApplyBlendMode(ctx.CairoCtx)

		// 应用填充透明度
		if state.FillAlpha < 1.0 {
			ctx.CairoCtx.PaintWithAlpha(state.FillAlpha)
		} else {
			ctx.CairoCtx.Paint()
		}
	} else {
		ctx.CairoCtx.Paint()
	}

	debugPrintf("[TransparencyGroup] Group rendered and composited\n")

	return nil
}

// renderImageXObject 渲染图像 XObject
func renderImageXObject(ctx *RenderContext, xobj *XObject) error {
	if xobj.ImageData == nil {
		// 尝试解码图像数据
		if err := decodeImageXObject(xobj); err != nil {
			return fmt.Errorf("failed to decode image: %w", err)
		}
	}

	if xobj.ImageData == nil {
		return fmt.Errorf("no image data available")
	}

	// 保存图形状态
	ctx.CairoCtx.Save()
	defer ctx.CairoCtx.Restore()

	// 创建 Cairo image surface
	bounds := xobj.ImageData.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	imgSurface := cairo.NewImageSurface(cairo.FormatARGB32, width, height)
	defer imgSurface.Destroy()

	// 将 Go image 转换为 Cairo surface
	if cairoImg, ok := imgSurface.(cairo.ImageSurface); ok {
		data := cairoImg.GetData()
		stride := cairoImg.GetStride()

		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				r, g, b, a := xobj.ImageData.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
				offset := y*stride + x*4

				// Cairo 使用预乘 alpha 的 BGRA 格式
				alpha := uint8(a >> 8)
				if alpha > 0 {
					data[offset+0] = uint8((b >> 8) * uint32(alpha) / 255) // B
					data[offset+1] = uint8((g >> 8) * uint32(alpha) / 255) // G
					data[offset+2] = uint8((r >> 8) * uint32(alpha) / 255) // R
					data[offset+3] = alpha                                 // A
				} else {
					data[offset+0] = 0
					data[offset+1] = 0
					data[offset+2] = 0
					data[offset+3] = 0
				}
			}
		}
		cairoImg.MarkDirty()
	}

	// 缩放图像以匹配 PDF 单位空间（1x1 单位）
	ctx.CairoCtx.Scale(1.0/float64(width), 1.0/float64(height))

	// 绘制图像
	ctx.CairoCtx.SetSourceSurface(imgSurface, 0, 0)
	ctx.CairoCtx.Paint()

	return nil
}

// decodeImageXObject 解码图像 XObject
func decodeImageXObject(xobj *XObject) error {
	if len(xobj.Stream) == 0 {
		return fmt.Errorf("no image stream data")
	}

	// 根据颜色空间和位深度解码图像
	width := xobj.Width
	height := xobj.Height
	bpc := xobj.BitsPerComponent

	if bpc == 0 {
		bpc = 8 // 默认 8 位
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	switch xobj.ColorSpace {
	case "DeviceRGB", "/DeviceRGB":
		// RGB 颜色空间
		bytesPerPixel := 3
		if bpc == 8 {
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					offset := (y*width + x) * bytesPerPixel
					if offset+2 < len(xobj.Stream) {
						r := xobj.Stream[offset]
						g := xobj.Stream[offset+1]
						b := xobj.Stream[offset+2]
						img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
					}
				}
			}
		}

	case "DeviceGray", "/DeviceGray":
		// 灰度颜色空间
		if bpc == 8 {
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					offset := y*width + x
					if offset < len(xobj.Stream) {
						gray := xobj.Stream[offset]
						img.Set(x, y, color.RGBA{R: gray, G: gray, B: gray, A: 255})
					}
				}
			}
		}

	case "DeviceCMYK", "/DeviceCMYK":
		// CMYK 颜色空间
		bytesPerPixel := 4
		if bpc == 8 {
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					offset := (y*width + x) * bytesPerPixel
					if offset+3 < len(xobj.Stream) {
						c := float64(xobj.Stream[offset]) / 255.0
						m := float64(xobj.Stream[offset+1]) / 255.0
						yc := float64(xobj.Stream[offset+2]) / 255.0
						k := float64(xobj.Stream[offset+3]) / 255.0

						r, g, b := cmykToRGB(c, m, yc, k)
						img.Set(x, y, color.RGBA{
							R: uint8(r * 255),
							G: uint8(g * 255),
							B: uint8(b * 255),
							A: 255,
						})
					}
				}
			}
		}

	case "Indexed", "/Indexed":
		// Indexed 颜色空间（调色板颜色）
		// 注意：当前实现假设调色板数据已存储在xobj.ColorSpace的附加信息中
		// 在实际应用中，需要从PDF资源中提取调色板数据
		debugPrintf("⚠️  Indexed color space detected but not fully implemented\n")

		// 创建一个简单的调色板（仅为演示）
		palette := make([]color.RGBA, 256)
		for i := 0; i < 256; i++ {
			// 简单的灰度调色板
			palette[i] = color.RGBA{R: uint8(i), G: uint8(i), B: uint8(i), A: 255}
		}

		// 使用调色板解码图像
		if bpc == 8 {
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					offset := y*width + x
					if offset < len(xobj.Stream) {
						paletteIndex := int(xobj.Stream[offset])
						if paletteIndex < len(palette) {
							img.Set(x, y, palette[paletteIndex])
						} else {
							img.Set(x, y, color.RGBA{R: 0, G: 0, B: 0, A: 255})
						}
					}
				}
			}
		}
		debugPrintf("✓ Processed Indexed color space image (%dx%d)\n", width, height)

	case "ICCBased", "/ICCBased":
		// ICCBased 颜色空间
		// 注意：当前实现只是简单地将其视为RGB处理
		// 在实际应用中，需要解析ICC配置文件并进行颜色转换
		debugPrintf("⚠️  ICCBased color space detected but using RGB approximation\n")

		// 假设是RGB颜色空间进行处理
		bytesPerPixel := 3
		if bpc == 8 {
			for y := 0; y < height; y++ {
				for x := 0; x < width; x++ {
					offset := (y*width + x) * bytesPerPixel
					if offset+2 < len(xobj.Stream) {
						r := xobj.Stream[offset]
						g := xobj.Stream[offset+1]
						b := xobj.Stream[offset+2]
						img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
					}
				}
			}
		}
		debugPrintf("✓ Processed ICCBased color space image (%dx%d)\n", width, height)

	default:
		// 不支持的颜色空间，创建占位图像
		debugPrintf("⚠️  Unsupported color space: %s, using placeholder image\n", xobj.ColorSpace)
		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				// 棋盘格图案
				if (x/10+y/10)%2 == 0 {
					img.Set(x, y, color.RGBA{R: 200, G: 200, B: 200, A: 255})
				} else {
					img.Set(x, y, color.RGBA{R: 150, G: 150, B: 150, A: 255})
				}
			}
		}
	}

	xobj.ImageData = img
	return nil
}

// ===== 内联图像操作符 =====

// OpBeginInlineImage BI - 开始内联图像
type OpBeginInlineImage struct {
	ImageDict map[string]interface{}
}

func (op *OpBeginInlineImage) Name() string { return "BI" }

func (op *OpBeginInlineImage) Execute(ctx *RenderContext) error {
	// 内联图像字典已解析，等待图像数据
	return nil
}

// OpInlineImageData ID - 内联图像数据
type OpInlineImageData struct {
	ImageData        []byte
	Width            int
	Height           int
	ColorSpace       string
	BitsPerComponent int
}

func (op *OpInlineImageData) Name() string { return "ID" }

func (op *OpInlineImageData) Execute(ctx *RenderContext) error {
	// 创建临时 XObject 并渲染
	xobj := &XObject{
		Subtype:          "Image",
		Width:            op.Width,
		Height:           op.Height,
		ColorSpace:       op.ColorSpace,
		BitsPerComponent: op.BitsPerComponent,
		Stream:           op.ImageData,
	}

	return renderImageXObject(ctx, xobj)
}

// OpEndInlineImage EI - 结束内联图像
type OpEndInlineImage struct{}

func (op *OpEndInlineImage) Name() string { return "EI" }

func (op *OpEndInlineImage) Execute(ctx *RenderContext) error {
	// 内联图像结束标记
	return nil
}
