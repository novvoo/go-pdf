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

	// 注意：不使用 Save/Restore，因为会撤销绘制操作
	// Do 操作符外层已经有 q/Q 来保存/恢复状态

	// 创建 Cairo image surface
	bounds := xobj.ImageData.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	debugPrintf("[renderImageXObject] Creating surface: %dx%d pixels\n", width, height)

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

	// 手动创建 Cairo surface，使用 RGB24 格式（不带 alpha），避免预乘问题
	imgSurface := cairo.NewImageSurface(cairo.FormatRGB24, width, height)
	defer imgSurface.Destroy()

	// 手动填充数据
	if cairoImg, ok := imgSurface.(cairo.ImageSurface); ok {
		data := cairoImg.GetData()
		stride := cairoImg.GetStride()

		for y := 0; y < height; y++ {
			for x := 0; x < width; x++ {
				r, g, b, _ := xobj.ImageData.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
				offset := y*stride + x*4

				// Cairo RGB24 格式：BGRX 字节序（X 是未使用的字节）
				r8 := uint8(r >> 8)
				g8 := uint8(g >> 8)
				b8 := uint8(b >> 8)

				data[offset+0] = b8 // B
				data[offset+1] = g8 // G
				data[offset+2] = r8 // R
				data[offset+3] = 0  // 未使用
			}
		}
		cairoImg.MarkDirty()

		// 验证数据
		debugPrintf("[renderImageXObject] Cairo RGB24 surface pixel (0,0): B=%d G=%d R=%d\n",
			data[0], data[1], data[2])
		if width > 100 && height > 100 {
			offset := 100*stride + 100*4
			debugPrintf("[renderImageXObject] Cairo RGB24 surface pixel (100,100): B=%d G=%d R=%d\n",
				data[offset], data[offset+1], data[offset+2])
		}
	}

	// PDF 规范：图像 XObject 的用户空间是 1x1 单位
	// Do 操作符之前的 cm 矩阵已经将 1x1 单位空间映射到实际尺寸

	// 在 PDF 中，图像的坐标系是：
	// - 原点在左下角
	// - (0,0) 到 (1,1) 映射到整个图像
	// - X 轴向右，Y 轴向上

	// 我们需要：
	// 1. 将图像缩放到 1x1 单位空间
	// 2. 翻转 Y 轴（因为图像数据是从上到下的）
	// 3. 让 cm 矩阵将其放大到正确尺寸

	debugPrintf("[renderImageXObject] Applying transformations\n")

	// PDF 图像 XObject 的坐标系：
	// - 图像占据 (0,0) 到 (1,1) 的单位正方形
	// - 原点在左下角，Y 轴向上
	//
	// Cairo 图像的坐标系：
	// - 图像数据从 (0,0) 开始，Y 轴向下
	//
	// 变换步骤：
	// 1. 翻转 Y 轴：Scale(1, -1)
	// 2. 平移到正确位置：Translate(0, -1)
	// 3. 缩放到图像像素尺寸：Scale(width, height)

	// PDF 规范：图像占据单位正方形 (0,0) 到 (1,1)
	// 使用 pattern 矩阵来处理图像的缩放和翻转

	// 设置图像为源
	ctx.CairoCtx.SetSourceSurface(imgSurface, 0, 0)

	// 获取 pattern
	pattern := ctx.CairoCtx.GetSource()
	pattern.SetFilter(cairo.FilterBest)

	// 创建 pattern 矩阵
	// Pattern 矩阵是从用户空间到 pattern 空间的变换
	// 我们需要将单位正方形 (0,0)-(1,1) 映射到图像像素坐标 (0,0)-(width,height)
	// 同时处理 Y 轴翻转
	//
	// 用户空间点 (u, v) 映射到 pattern 空间点 (x, y):
	// x = u * width
	// y = (1 - v) * height  (翻转 Y 轴)
	//
	// 矩阵形式: [width, 0, 0, -height, 0, height]

	matrix := cairo.NewMatrix()
	matrix.XX = float64(width)
	matrix.YX = 0
	matrix.XY = 0
	matrix.YY = -float64(height)
	matrix.X0 = 0
	matrix.Y0 = float64(height)

	pattern.SetMatrix(matrix)

	debugPrintf("[renderImageXObject] Pattern matrix set: [%.2f, %.2f, %.2f, %.2f, %.2f, %.2f]\n",
		matrix.XX, matrix.YX, matrix.XY, matrix.YY, matrix.X0, matrix.Y0)

	// 绘制单位正方形
	ctx.CairoCtx.Rectangle(0, 0, 1, 1)
	ctx.CairoCtx.Fill()

	debugPrintf("[renderImageXObject] Image painted\n")

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
