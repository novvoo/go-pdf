package gopdf

import (
	"fmt"
	"image"
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
	Width            int                // 图像宽度（PDF 字典中声明的逻辑宽度）
	Height           int                // 图像高度（PDF 字典中声明的逻辑高度）
	ColorSpace       string             // 颜色空间
	ColorSpaceArray  interface{}        // 🔥 新增：颜色空间数组（用于 Indexed 等复杂颜色空间）
	BitsPerComponent int                // 每个颜色分量的位数
	ImageData        image.Image        // 解码后的图像数据
	Group            *TransparencyGroup // 透明度组（仅用于 Form）
	// 🔥 新增：图像 DPI 相关信息
	// 注意：PDF 规范中没有直接的 DPI 字段，但可以通过以下方式推断：
	// 1. 如果 Width/Height 与解码后的像素尺寸不同，说明有缩放
	// 2. 外层 CTM 矩阵决定了图像在页面上的实际尺寸
	ActualPixelWidth  int      // 解码后的实际像素宽度
	ActualPixelHeight int      // 解码后的实际像素高度
	SMask             *XObject // 🔥 新增：软遮罩（透明度掩码）
	ColorComponents   int      // 🔥 新增：颜色分量数（来自 ICCBased N 或其他）
	Palette           []byte   // 🔥 新增：调色板数据（用于 Indexed 颜色空间）
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
		imgData, err := decodeImageXObject(xobj)
		if err != nil {
			return fmt.Errorf("failed to decode image: %w", err)
		}
		xobj.ImageData = imgData
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

	// 🔥 修复：检查 XObject 字典中的 Width 和 Height 是否与解码后的图像尺寸不同
	// 如果不同，说明图像可能有 DPI 信息或需要缩放
	if xobj.Width > 0 && xobj.Height > 0 && (xobj.Width != width || xobj.Height != height) {
		debugPrintf("[renderImageXObject] ⚠️  XObject dimensions (%dx%d) differ from decoded image (%dx%d)\n",
			xobj.Width, xobj.Height, width, height)
		debugPrintf("[renderImageXObject] This may indicate DPI mismatch or image scaling\n")
		// 注意：我们仍然使用解码后的实际像素尺寸，因为 XObject 的 Width/Height
		// 只是 PDF 字典中的声明值，实际渲染应该基于解码后的数据
	}

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
	// 关键理解：
	// 1. PDF 字典中的 Width/Height 是图像的"逻辑"尺寸（采样数）
	// 2. 解码后的实际像素可能与 Width/Height 相同或不同
	// 3. 外层 CTM 矩阵决定了图像在页面上的物理尺寸（单位：points）
	// 4. 我们需要将解码后的像素映射到单位空间 [0,1]x[0,1]
	//
	// 正确的做法：
	// - 使用 PDF 字典中的 Width/Height 作为逻辑尺寸
	// - 如果解码后的像素尺寸不同，说明图像被重采样了
	// - 但渲染时应该使用解码后的实际像素，以保证质量

	// 使用 PDF 字典中声明的尺寸（如果可用）
	logicalWidth := xobj.Width
	logicalHeight := xobj.Height

	// 如果 PDF 字典中没有声明尺寸，或者尺寸为 0，使用实际像素尺寸
	if logicalWidth == 0 {
		logicalWidth = width
	}
	if logicalHeight == 0 {
		logicalHeight = height
	}

	// 保存实际像素尺寸到 XObject（用于调试和分析）
	xobj.ActualPixelWidth = width
	xobj.ActualPixelHeight = height

	debugPrintf("[renderImageXObject] Logical dimensions (from PDF): %dx%d\n", logicalWidth, logicalHeight)
	debugPrintf("[renderImageXObject] Actual pixel dimensions (decoded): %dx%d\n", width, height)

	// 计算 DPI 比率（如果逻辑尺寸与实际像素不同）
	if logicalWidth != width || logicalHeight != height {
		dpiRatioX := float64(width) / float64(logicalWidth)
		dpiRatioY := float64(height) / float64(logicalHeight)
		debugPrintf("[renderImageXObject] DPI ratio: X=%.2f, Y=%.2f (higher = higher resolution)\n",
			dpiRatioX, dpiRatioY)
	}

	// 🔥 关键修复：使用解码后的实际像素尺寸进行渲染
	// 这样可以保证高分辨率图像的质量
	// 外层 CTM 已经设置了正确的物理尺寸，我们只需要将像素映射到单位空间
	debugPrintf("[renderImageXObject] Using actual pixel dimensions for rendering: %dx%d\n", width, height)

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

		// 🔥 新增：计算图像的实际 DPI
		// CTM 的 XX 和 YY 分量表示图像在页面上的物理尺寸（单位：points）
		// 1 point = 1/72 inch
		// DPI = (pixels / points) * 72
		physicalWidthPoints := state.CTM.XX
		physicalHeightPoints := state.CTM.YY
		if physicalHeightPoints < 0 {
			physicalHeightPoints = -physicalHeightPoints
		}

		if physicalWidthPoints > 0 && physicalHeightPoints > 0 {
			dpiX := (float64(width) / physicalWidthPoints) * 72.0
			dpiY := (float64(height) / physicalHeightPoints) * 72.0

			debugPrintf("[renderImageXObject] 📊 Image DPI Analysis:\n")
			debugPrintf("[renderImageXObject]    Physical size: %.2f x %.2f points (%.2f x %.2f inches)\n",
				physicalWidthPoints, physicalHeightPoints,
				physicalWidthPoints/72.0, physicalHeightPoints/72.0)
			debugPrintf("[renderImageXObject]    Pixel size: %d x %d pixels\n", width, height)
			debugPrintf("[renderImageXObject]    Effective DPI: %.1f x %.1f\n", dpiX, dpiY)

			// 警告：如果 DPI 显著高于 72，说明图像被缩小了
			if dpiX > 100 || dpiY > 100 {
				debugPrintf("[renderImageXObject]    ⚠️  High DPI detected! Image is being downscaled in PDF.\n")
				debugPrintf("[renderImageXObject]    This is normal for high-resolution images embedded in PDFs.\n")
			}

			// 警告：如果 DPI 显著低于 72，说明图像被放大了（可能模糊）
			if dpiX < 50 || dpiY < 50 {
				debugPrintf("[renderImageXObject]    ⚠️  Low DPI detected! Image is being upscaled in PDF.\n")
				debugPrintf("[renderImageXObject]    This may result in blurry or pixelated output.\n")
			}
		}
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
	imgData, err := decodeImageXObject(xobj)
	if err != nil {
		return nil
	}
	return imgData
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
