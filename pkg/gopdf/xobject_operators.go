package gopdf

import (
	"fmt"
	"image"
	"image/color"
)

// ===== XObject 操作符 =====

// OpDoXObject Do - 绘制 XObject（表单或图像）
type OpDoXObject struct {
	XObjectName string
}

func (op *OpDoXObject) Name() string { return "Do" }

func (op *OpDoXObject) Execute(ctx *RenderContext) error {
	debugPrintf("[Do] Drawing XObject: %s\n", op.XObjectName)

	// 从资源中获取 XObject
	xobj := ctx.Resources.GetXObject(op.XObjectName)
	if xobj == nil {
		debugPrintf("[Do] ⚠️  XObject %s not found in resources\n", op.XObjectName)
		return fmt.Errorf("XObject %s not found", op.XObjectName)
	}

	debugPrintf("[Do] XObject type: %s\n", xobj.Subtype)

	switch xobj.Subtype {
	case "Form", "/Form":
		debugPrintf("[Do] Rendering Form XObject\n")
		return renderFormXObject(ctx, xobj)
	case "Image", "/Image":
		debugPrintf("[Do] Rendering Image XObject (size: %dx%d)\n", xobj.Width, xobj.Height)
		return renderImageXObject(ctx, xobj)
	default:
		debugPrintf("[Do] ⚠️  Unsupported XObject subtype: %s\n", xobj.Subtype)
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
	ctx.GopdfCtx.Save()
	ctx.GraphicsStack.Push()
	defer func() {
		ctx.GopdfCtx.Restore()
		ctx.GraphicsStack.Pop()
	}()

	// 应用 XObject 的变换矩阵
	if xobj.Matrix != nil {
		xobj.Matrix.ApplyToGopdfContext(ctx.GopdfCtx)
	}

	// 应用边界框裁剪
	if len(xobj.BBox) == 4 {
		x1, y1, x2, y2 := xobj.BBox[0], xobj.BBox[1], xobj.BBox[2], xobj.BBox[3]
		ctx.GopdfCtx.Rectangle(x1, y1, x2-x1, y2-y1)
		ctx.GopdfCtx.Clip()
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
	ctx.GopdfCtx.Save()
	ctx.GraphicsStack.Push()
	defer func() {
		ctx.GopdfCtx.Restore()
		ctx.GraphicsStack.Pop()
	}()

	// 应用 XObject 的变换矩阵
	if xobj.Matrix != nil {
		xobj.Matrix.ApplyToGopdfContext(ctx.GopdfCtx)
	}

	// 使用 Gopdf push_group 创建隔离的合成表面
	// 这会创建一个临时的 surface 用于渲染组内容
	ctx.GopdfCtx.PushGroup()

	// 应用边界框裁剪
	if len(xobj.BBox) == 4 {
		x1, y1, x2, y2 := xobj.BBox[0], xobj.BBox[1], xobj.BBox[2], xobj.BBox[3]
		ctx.GopdfCtx.Rectangle(x1, y1, x2-x1, y2-y1)
		ctx.GopdfCtx.Clip()
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
			ctx.GopdfCtx.PopGroupToSource() // 清理 group
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

	// 使用 Gopdf pop_group_to_source 将组内容作为源
	ctx.GopdfCtx.PopGroupToSource()

	// 应用当前图形状态的混合模式和透明度
	state := ctx.GetCurrentState()
	if state != nil {
		// 应用混合模式
		state.ApplyBlendMode(ctx.GopdfCtx)

		// 应用填充透明度
		if state.FillAlpha < 1.0 {
			ctx.GopdfCtx.PaintWithAlpha(state.FillAlpha)
		} else {
			ctx.GopdfCtx.Paint()
		}
	} else {
		ctx.GopdfCtx.Paint()
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

	// 创建 Gopdf image surface
	bounds := xobj.ImageData.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	debugPrintf("[renderImageXObject] Creating surface: %dx%d pixels\n", width, height)
	debugPrintf("[renderImageXObject] XObject dimensions: %dx%d\n", xobj.Width, xobj.Height)

	// 采样图片数据来验证颜色
	if width > 0 && height > 0 {
		r, g, b, a := xobj.ImageData.At(0, 0).RGBA()
		debugPrintf("[renderImageXObject] Sample pixel (0,0): R=%d G=%d B=%d A=%d\n",
			uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8))
		if width > 100 && height > 100 {
			r, g, b, a = xobj.ImageData.At(100, 100).RGBA()
			debugPrintf("[renderImageXObject] Sample pixel (100,100): R=%d G=%d B=%d A=%d\n",
				uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8))
		}
	}

	// 🔥 修复：PDF 图像 XObject 占据单位正方形 (0,0) 到 (1,1)
	// 我们需要将图像的像素尺寸缩放到单位空间
	// 使用图像的实际像素尺寸进行缩放
	debugPrintf("[renderImageXObject] Using pixel dimensions: %dx%d for scaling to unit square\n", width, height)

	// 使用 ARGB32 格式以支持透明度
	imgSurface := NewImageSurface(FormatARGB32, width, height)
	defer imgSurface.Destroy()

	// 手动填充数据
	if gopdfImg, ok := imgSurface.(ImageSurface); ok {
		data := gopdfImg.GetData()
		stride := gopdfImg.GetStride()

		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				r, g, b, a := xobj.ImageData.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
				offset := y*stride + x*4

				// Gopdf ARGB32 格式：预乘 BGRA 字节序（小端系统）
				// 需要将颜色值预乘 alpha
				a8 := uint8(a >> 8)
				r8 := uint8(r >> 8)
				g8 := uint8(g >> 8)
				b8 := uint8(b >> 8)

				// 预乘 alpha
				if a8 < 255 {
					alpha := float64(a8) / 255.0
					r8 = uint8(float64(r8) * alpha)
					g8 = uint8(float64(g8) * alpha)
					b8 = uint8(float64(b8) * alpha)
				}

				if offset+3 < len(data) {
					data[offset+0] = b8 // B
					data[offset+1] = g8 // G
					data[offset+2] = r8 // R
					data[offset+3] = a8 // A
				}
			}
		}

		gopdfImg.MarkDirty()
	}

	debugPrintf("[renderImageXObject] Applying transformations\n")

	// 获取当前图形状态
	state := ctx.GetCurrentState()
	if state != nil && state.CTM != nil {
		debugPrintf("[renderImageXObject] CTM: [%.3f %.3f %.3f %.3f %.3f %.3f]\n",
			state.CTM.XX, state.CTM.YX, state.CTM.XY, state.CTM.YY, state.CTM.X0, state.CTM.Y0)
	}

	// PDF 图像 XObject 占据单位正方形 (0,0) 到 (1,1)
	// 外层的 cm 矩阵已经设置了实际尺寸和位置
	//
	// 关键理解：
	// - PDF 中图像 XObject 定义在单位空间 [0,1]x[0,1]
	// - 外层 cm 矩阵将这个单位空间映射到页面坐标
	// - 我们需要将图像像素映射到这个单位空间
	//
	// 变换策略：
	// 1. 翻转 Y 轴（PDF Y 向上，Gopdf Y 向下）
	// 2. 缩放图像使其填充单位正方形

	// 保存当前变换
	ctx.GopdfCtx.Save()

	// 🔍 重置操作符和混合模式，确保图像正常绘制
	ctx.GopdfCtx.SetOperator(OperatorOver)
	debugPrintf("[renderImageXObject] Set operator to Over\n")

	// PDF 图像 XObject 的坐标系统：
	// - 图像占据单位正方形 (0,0) 到 (1,1)
	// - 图像的 (0,0) 在左下角，(1,1) 在右上角
	// - Gopdf 的 (0,0) 在左上角
	// - 外层 CTM 已经设置了位置和大小
	//
	// 变换步骤：
	// 1. 缩放图像到单位空间：width 像素 -> 1 单位
	// 2. 翻转 Y 轴：PDF Y 向上 -> Gopdf Y 向下

	// 检查当前 CTM 的 Y 轴方向
	// 如果 CTM.YY > 0，Y 轴是 PDF 方向（向上），需要翻转
	// 如果 CTM.YY < 0，Y 轴是 Gopdf 方向（向下），不需要翻转
	needFlipY := false
	if state != nil && state.CTM != nil {
		if state.CTM.YY > 0 {
			needFlipY = true
			debugPrintf("[renderImageXObject] CTM.YY=%.3f > 0, Y axis is PDF direction (up), need flip\n", state.CTM.YY)
		} else {
			debugPrintf("[renderImageXObject] CTM.YY=%.3f < 0, Y axis is Gopdf direction (down), no flip needed\n", state.CTM.YY)
		}
	}

	// 🔥 修复：缩放图像到单位空间
	// PDF 图像 XObject 占据单位正方形 (0,0) 到 (1,1)
	// 我们需要将图像像素映射到这个单位空间
	// scaleX = 1.0 / width 表示将 width 个像素缩放到 1 个单位
	if width == 0 || height == 0 {
		debugPrintf("[renderImageXObject] ⚠️  Invalid image dimensions: %dx%d, skipping render\n", width, height)
		return fmt.Errorf("invalid image dimensions: %dx%d", width, height)
	}

	scaleX := 1.0 / float64(width)
	scaleY := 1.0 / float64(height)

	debugPrintf("[renderImageXObject] Scale factors: X=%.6f (1/%d), Y=%.6f (1/%d)\n",
		scaleX, width, scaleY, height)

	// 应用变换
	if needFlipY {
		// Y 轴是 PDF 方向，需要翻转
		ctx.GopdfCtx.Scale(scaleX, -scaleY)
		ctx.GopdfCtx.Translate(0, -float64(height))
		debugPrintf("[renderImageXObject] Applied: Scale(%.6f, %.6f) + Translate(0, %.0f)\n",
			scaleX, -scaleY, -float64(height))
	} else {
		// Y 轴已经是 Gopdf 方向，只需缩放
		ctx.GopdfCtx.Scale(scaleX, scaleY)
		debugPrintf("[renderImageXObject] Applied: Scale(%.6f, %.6f)\n", scaleX, scaleY)
	}

	debugPrintf("[renderImageXObject] Transformation applied\n")

	// 设置图像为源
	ctx.GopdfCtx.SetSourceSurface(imgSurface, 0, 0)
	debugPrintf("[renderImageXObject] Set source surface\n")

	// 设置过滤器
	pattern := ctx.GopdfCtx.GetSource()
	pattern.SetFilter(FilterBest)

	// 🔍 调试：检查 pattern 的矩阵
	debugPrintf("[renderImageXObject] Pattern filter set to Best\n")

	debugPrintf("[renderImageXObject] Painting image\n")

	// 绘制图像 - 使用 PaintWithAlpha 以确保透明度正确处理
	// 如果图形状态有 alpha，使用它；否则使用 1.0（完全不透明）
	alpha := 1.0
	if state != nil {
		alpha = state.FillAlpha
	}
	if alpha < 1.0 {
		ctx.GopdfCtx.PaintWithAlpha(alpha)
		debugPrintf("[renderImageXObject] Painted with alpha=%.2f\n", alpha)
	} else {
		ctx.GopdfCtx.Paint()
		debugPrintf("[renderImageXObject] Painted with alpha=1.0\n")
	}

	// 恢复变换
	ctx.GopdfCtx.Restore()

	debugPrintf("[renderImageXObject] Image painted successfully\n")

	return nil
}

// DecodeImageXObjectPublic 公开的图像解码函数，供测试使用
func DecodeImageXObjectPublic(xobj *XObject) image.Image {
	if err := decodeImageXObject(xobj); err != nil {
		return nil
	}
	return xobj.ImageData
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

	debugPrintf("[decodeImageXObject] Decoding image: %dx%d, BPC=%d, ColorSpace=%s, Stream=%d bytes\n",
		width, height, bpc, xobj.ColorSpace, len(xobj.Stream))

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	switch xobj.ColorSpace {
	case "DeviceRGB", "/DeviceRGB":
		// RGB 颜色空间
		bytesPerPixel := 3
		expectedBytes := width * height * bytesPerPixel
		debugPrintf("[decodeImageXObject] DeviceRGB: expected %d bytes, got %d bytes\n", expectedBytes, len(xobj.Stream))

		if bpc == 8 {
			// 采样前几个像素来检查数据
			if len(xobj.Stream) >= 30 {
				debugPrintf("[decodeImageXObject] First 10 pixels (RGB):\n")
				for i := 0; i < 10 && i*3+2 < len(xobj.Stream); i++ {
					r := xobj.Stream[i*3]
					g := xobj.Stream[i*3+1]
					b := xobj.Stream[i*3+2]
					debugPrintf("  Pixel %d: R=%d G=%d B=%d\n", i, r, g, b)
				}
			}

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
			debugPrintf("[decodeImageXObject] DeviceRGB decoding completed\n")
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

		// 计算实际的字节数来推断颜色分量数
		expectedBytes := width * height
		bytesPerPixel := 3 // 默认 RGB

		// 根据实际数据大小推断颜色分量数
		if len(xobj.Stream) >= expectedBytes*4 {
			bytesPerPixel = 4 // CMYK
			debugPrintf("[ICCBased] Detected 4 components (CMYK), stream size: %d bytes\n", len(xobj.Stream))
		} else if len(xobj.Stream) >= expectedBytes*3 {
			bytesPerPixel = 3 // RGB
			debugPrintf("[ICCBased] Detected 3 components (RGB), stream size: %d bytes\n", len(xobj.Stream))
		} else if len(xobj.Stream) >= expectedBytes {
			bytesPerPixel = 1 // Gray
			debugPrintf("[ICCBased] Detected 1 component (Gray), stream size: %d bytes\n", len(xobj.Stream))
		}

		// 采样前几个像素来检查数据
		needInvert := false
		if len(xobj.Stream) >= 30 && bytesPerPixel >= 3 {
			debugPrintf("[ICCBased] First 5 pixels:\n")
			blackCount := 0
			for i := 0; i < 5 && i*bytesPerPixel+2 < len(xobj.Stream); i++ {
				if bytesPerPixel == 3 {
					r := xobj.Stream[i*bytesPerPixel]
					g := xobj.Stream[i*bytesPerPixel+1]
					b := xobj.Stream[i*bytesPerPixel+2]
					debugPrintf("  Pixel %d: R=%d G=%d B=%d\n", i, r, g, b)
					if r < 10 && g < 10 && b < 10 {
						blackCount++
					}
				} else if bytesPerPixel == 4 {
					c := xobj.Stream[i*bytesPerPixel]
					m := xobj.Stream[i*bytesPerPixel+1]
					y := xobj.Stream[i*bytesPerPixel+2]
					k := xobj.Stream[i*bytesPerPixel+3]
					debugPrintf("  Pixel %d: C=%d M=%d Y=%d K=%d\n", i, c, m, y, k)
				}
			}

			// 采样中间部分的像素
			midOffset := len(xobj.Stream) / 2
			midOffset = (midOffset / bytesPerPixel) * bytesPerPixel // 对齐到像素边界
			debugPrintf("[ICCBased] Middle 5 pixels (offset %d):\n", midOffset/bytesPerPixel)
			for i := 0; i < 5 && midOffset+i*bytesPerPixel+2 < len(xobj.Stream); i++ {
				if bytesPerPixel == 3 {
					r := xobj.Stream[midOffset+i*bytesPerPixel]
					g := xobj.Stream[midOffset+i*bytesPerPixel+1]
					b := xobj.Stream[midOffset+i*bytesPerPixel+2]
					debugPrintf("  Pixel %d: R=%d G=%d B=%d\n", i, r, g, b)
					if r < 10 && g < 10 && b < 10 {
						blackCount++
					}
				} else if bytesPerPixel == 4 {
					c := xobj.Stream[midOffset+i*bytesPerPixel]
					m := xobj.Stream[midOffset+i*bytesPerPixel+1]
					y := xobj.Stream[midOffset+i*bytesPerPixel+2]
					k := xobj.Stream[midOffset+i*bytesPerPixel+3]
					debugPrintf("  Pixel %d: C=%d M=%d Y=%d K=%d\n", i, c, m, y, k)
				}
			}

			// 如果大部分采样像素都是黑色，可能需要反转颜色
			// 这通常发生在某些 ICC Profile 中，特别是从 CMYK 转换来的
			// 暂时禁用自动反转，让用户确认原始图像颜色
			if blackCount >= 8 {
				needInvert = false // 暂时禁用
				debugPrintf("[ICCBased] ⚠️  Detected mostly black pixels (%d/10), but auto-invert is disabled\n", blackCount)
			}
		}

		if bpc == 8 {
			if bytesPerPixel == 3 {
				// RGB
				for y := 0; y < height; y++ {
					for x := 0; x < width; x++ {
						offset := (y*width + x) * bytesPerPixel
						if offset+2 < len(xobj.Stream) {
							r := xobj.Stream[offset]
							g := xobj.Stream[offset+1]
							b := xobj.Stream[offset+2]

							// 如果需要反转颜色
							if needInvert {
								r = 255 - r
								g = 255 - g
								b = 255 - b
							}

							img.Set(x, y, color.RGBA{R: r, G: g, B: b, A: 255})
						}
					}
				}
			} else if bytesPerPixel == 4 {
				// CMYK
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
			} else if bytesPerPixel == 1 {
				// Gray
				for y := 0; y < height; y++ {
					for x := 0; x < width; x++ {
						offset := y*width + x
						if offset < len(xobj.Stream) {
							gray := xobj.Stream[offset]

							// 如果需要反转颜色
							if needInvert {
								gray = 255 - gray
							}

							img.Set(x, y, color.RGBA{R: gray, G: gray, B: gray, A: 255})
						}
					}
				}
			}
		}
		if needInvert {
			debugPrintf("✓ Processed ICCBased color space image (%dx%d, %d components, inverted)\n", width, height, bytesPerPixel)
		} else {
			debugPrintf("✓ Processed ICCBased color space image (%dx%d, %d components)\n", width, height, bytesPerPixel)
		}

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
