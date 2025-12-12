package gopdf

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"strings"

	"github.com/novvoo/go-cairo/pkg/cairo"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// PDFReader 用于读取和渲染 PDF 文件
type PDFReader struct {
	pdfPath string
}

// NewPDFReader 创建新的 PDF 读取器
func NewPDFReader(pdfPath string) *PDFReader {
	return &PDFReader{
		pdfPath: pdfPath,
	}
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

	// 使用 go-cairo 创建渲染表面
	surface := cairo.NewImageSurface(cairo.FormatARGB32, width, height)
	defer surface.Destroy()

	cairoCtx := cairo.NewContext(surface)
	defer cairoCtx.Destroy()

	// 设置白色背景
	cairoCtx.SetSourceRGB(1, 1, 1)
	cairoCtx.Paint()

	// 缩放以匹配 DPI
	cairoCtx.Scale(scale, scale)

	// 渲染 PDF 内容到 Cairo context
	if err := renderPDFPageToCairo(r.pdfPath, pageNum, cairoCtx, widthPoints, heightPoints); err != nil {
		return fmt.Errorf("failed to render PDF page: %w", err)
	}

	// 直接使用 Cairo 保存 PNG
	if imgSurf, ok := surface.(cairo.ImageSurface); ok {
		status := imgSurf.WriteToPNG(outputPath)
		if status != cairo.StatusSuccess {
			return fmt.Errorf("failed to write PNG: %v", status)
		}
		return nil
	}

	return fmt.Errorf("failed to convert surface to image surface")
}

// RenderPageToImage 将 PDF 页面渲染为 image.Image
func (r *PDFReader) RenderPageToImage(pageNum int, dpi float64) (image.Image, error) {
	if dpi == 0 {
		dpi = 150
	}

	// 获取页面数量
	pageCount, err := api.PageCountFile(r.pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get page count: %w", err)
	}

	if pageNum < 1 || pageNum > pageCount {
		return nil, fmt.Errorf("invalid page number: %d (total pages: %d)", pageNum, pageCount)
	}

	// 获取页面尺寸
	pageDims, err := api.PageDimsFile(r.pdfPath)
	if err != nil {
		return nil, fmt.Errorf("failed to get page dimensions: %w", err)
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

	// 使用 go-cairo 创建渲染表面
	surface := cairo.NewImageSurface(cairo.FormatARGB32, width, height)
	defer surface.Destroy()

	cairoCtx := cairo.NewContext(surface)
	defer cairoCtx.Destroy()

	// 设置白色背景
	cairoCtx.SetSourceRGB(1, 1, 1)
	cairoCtx.Paint()

	// 缩放以匹配 DPI
	cairoCtx.Scale(scale, scale)

	// 渲染 PDF 内容到 Cairo context
	if err := renderPDFPageToCairo(r.pdfPath, pageNum, cairoCtx, widthPoints, heightPoints); err != nil {
		return nil, fmt.Errorf("failed to render PDF page: %w", err)
	}

	// 直接保存 Cairo surface 到 PNG，然后读取回来
	// 这样避免了颜色格式转换的问题
	tmpPath := fmt.Sprintf("temp_render_%d.png", pageNum)
	defer os.Remove(tmpPath)

	if imgSurf, ok := surface.(cairo.ImageSurface); ok {
		status := imgSurf.WriteToPNG(tmpPath)
		if status != cairo.StatusSuccess {
			return nil, fmt.Errorf("failed to write PNG: %v", status)
		}

		// 读取回来作为 image.Image
		file, err := os.Open(tmpPath)
		if err != nil {
			return nil, fmt.Errorf("failed to open temp PNG: %w", err)
		}
		defer file.Close()

		img, err := png.Decode(file)
		if err != nil {
			return nil, fmt.Errorf("failed to decode PNG: %w", err)
		}

		return img, nil
	}

	return nil, fmt.Errorf("failed to convert surface to image")
}

// GetPageCount 获取 PDF 的页数
func (r *PDFReader) GetPageCount() (int, error) {
	return api.PageCountFile(r.pdfPath)
}

// PageInfo 页面信息
type PageInfo struct {
	Width  float64
	Height float64
}

// TextElementInfo 文本元素信息
type TextElementInfo struct {
	Text     string
	X        float64
	Y        float64
	FontName string
	FontSize float64
}

// ImageElementInfo 图片元素信息
type ImageElementInfo struct {
	Name   string
	X      float64
	Y      float64
	Width  float64
	Height float64
}

// GetPageInfo 获取页面信息
func (r *PDFReader) GetPageInfo(pageNum int) (PageInfo, error) {
	pageDims, err := api.PageDimsFile(r.pdfPath)
	if err != nil {
		return PageInfo{}, fmt.Errorf("failed to get page dimensions: %w", err)
	}

	if pageNum < 1 || pageNum > len(pageDims) {
		return PageInfo{Width: 612, Height: 792}, nil // 默认 Letter 尺寸
	}

	dim := pageDims[pageNum-1]
	return PageInfo{
		Width:  dim.Width,
		Height: dim.Height,
	}, nil
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

	contentStreams, err := extractContentStreams(ctx, contents)
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
	baseFontSize := 0.0                   // Tf 操作符设置的基础字体大小
	currentMatrix := &Matrix{A: 1, D: 1}  // 单位矩阵
	textLineMatrix := &Matrix{A: 1, D: 1} // 文本行矩阵
	ctm := NewIdentityMatrix()            // 当前变换矩阵 (Current Transformation Matrix)

	for _, op := range operators {
		// 跳过忽略的操作符
		if op.Name() == "IGNORE" {
			continue
		}

		switch op.Name() {
		case "BT": // 开始文本对象
			// 重置文本矩阵和文本行矩阵为单位矩阵
			currentMatrix = &Matrix{A: 1, D: 1}
			textLineMatrix = &Matrix{A: 1, D: 1}
			debugPrintf("[DEBUG] BT operator: Reset text matrices\n")

		case "ET": // 结束文本对象
			debugPrintf("[DEBUG] ET operator: End text object\n")

		case "Tf": // 设置字体
			if tfOp, ok := op.(*OpSetFont); ok {
				currentFont = tfOp.FontName
				baseFontSize = tfOp.FontSize
				debugPrintf("[DEBUG] Tf operator: Font=%s, Size=%.2f\n", currentFont, baseFontSize)
			}

		case "Tm": // 设置文本矩阵
			if tmOp, ok := op.(*OpSetTextMatrix); ok {
				currentMatrix = tmOp.Matrix.Clone()
				textLineMatrix = tmOp.Matrix.Clone()
				debugPrintf("[DEBUG] Tm operator: Matrix=%s\n", currentMatrix.String())
			}

		case "cm": // 连接变换矩阵
			if cmOp, ok := op.(*OpConcatMatrix); ok {
				// 更新当前变换矩阵：CTM' = cm × CTM
				ctm = cmOp.Matrix.Multiply(ctm)
				debugPrintf("[DEBUG] cm operator: Matrix=%s, new CTM=%s\n", cmOp.Matrix.String(), ctm.String())
			}

		case "Td": // 文本位置偏移
			if tdOp, ok := op.(*OpMoveTextPosition); ok {
				translation := &Matrix{A: 1, D: 1, E: tdOp.Tx, F: tdOp.Ty}
				textLineMatrix = translation.Multiply(textLineMatrix)
				currentMatrix = textLineMatrix.Clone()
				debugPrintf("[DEBUG] Td operator: Tx=%.2f, Ty=%.2f, new E=%.2f, F=%.2f\n",
					tdOp.Tx, tdOp.Ty, currentMatrix.E, currentMatrix.F)
			}

		case "Tj", "TJ", "'", "\"": // 显示文本
			var text string
			var textArray []interface{}

			switch t := op.(type) {
			case *OpShowText:
				text = t.Text
			case *OpShowTextArray:
				textArray = t.Array
				for _, elem := range t.Array {
					if s, ok := elem.(string); ok {
						text += s
					}
				}
			case *OpShowTextNextLine:
				text = t.Text
			case *OpShowTextWithSpacing:
				text = t.Text
			}

			// 解码文本（处理CID字体和十六进制字符串）
			if text != "" {
				font := resources.GetFont(currentFont)
				if font != nil {
					text = decodeTextStringWithFontAndIdentity(text, font.ToUnicodeMap, font.IsIdentity)
				} else {
					text = decodeTextString(text)
				}
			}

			if text != "" && currentMatrix != nil {
				// 应用当前变换矩阵 (CTM) 到文本矩阵
				// 根据 PDF 规范：最终坐标 = (x, y) × Tm × CTM
				// 这里文本位置是 (0, 0)，所以最终位置就是 CTM × Tm 的平移部分
				finalMatrix := ctm.Multiply(currentMatrix)

				// PDF 坐标系：左下角为原点，Y 轴向上
				// 转换为屏幕坐标系：左上角为原点，Y 轴向下
				x := finalMatrix.E
				y := pageInfo.Height - finalMatrix.F

				// 计算有效字体大小：基础大小 * 文本矩阵的垂直缩放
				// 文本矩阵的 D 分量表示垂直缩放
				// 特殊情况：如果 Tf 设置的字体大小为 0，则直接使用文本矩阵的缩放作为字体大小
				effectiveFontSize := baseFontSize
				if currentMatrix != nil {
					scale := currentMatrix.D
					if scale < 0 {
						scale = -scale
					}
					if baseFontSize == 0 {
						// 当 Tf 设置字体大小为 0 时，字体大小完全由文本矩阵决定
						effectiveFontSize = scale
					} else {
						effectiveFontSize = baseFontSize * scale
					}
				}

				debugPrintf("[DEBUG] Text element: baseFontSize=%.2f, scale=%.2f, effectiveFontSize=%.2f\n",
					baseFontSize, currentMatrix.D, effectiveFontSize)

				textElements = append(textElements, TextElementInfo{
					Text:     text,
					X:        x,
					Y:        y,
					FontName: currentFont,
					FontSize: effectiveFontSize,
				})

				// 更新文本矩阵：显示文本后，文本位置会向右移动
				// 计算文本宽度（估算）
				var textDisplacement float64

				if textArray != nil {
					// TJ 操作符：处理文本数组和字距调整
					xOffset := 0.0
					for _, item := range textArray {
						switch v := item.(type) {
						case string:
							// 解码并计算文本宽度
							decodedText := ""
							font := resources.GetFont(currentFont)
							if font != nil {
								decodedText = decodeTextStringWithFontAndIdentity(v, font.ToUnicodeMap, font.IsIdentity)
							} else {
								decodedText = decodeTextString(v)
							}
							if decodedText != "" {
								runeCount := float64(len([]rune(decodedText)))
								xOffset += runeCount * effectiveFontSize * 0.5
							}
						case float64:
							// 字距调整：负值向右移动，正值向左移动
							xOffset -= v * effectiveFontSize / 1000.0
						case int:
							xOffset -= float64(v) * effectiveFontSize / 1000.0
						}
					}
					textDisplacement = xOffset
				} else {
					// Tj 操作符：简单文本
					runeCount := float64(len([]rune(text)))
					textDisplacement = runeCount * effectiveFontSize * 0.5
				}

				// 更新文本矩阵
				if textDisplacement != 0 {
					translation := &Matrix{A: 1, D: 1, E: textDisplacement, F: 0}
					currentMatrix = currentMatrix.Multiply(translation)
					debugPrintf("[DEBUG] Updated text matrix after rendering: E=%.2f\n", currentMatrix.E)
				}
			}

		case "Do": // 绘制 XObject（可能是图片）
			if doOp, ok := op.(*OpDoXObject); ok {
				xobj := resources.GetXObject(doOp.XObjectName)
				if xobj != nil && xobj.Subtype == "/Image" {
					// 获取当前变换矩阵来确定图片位置
					x := currentMatrix.E
					y := pageInfo.Height - currentMatrix.F

					imageElements = append(imageElements, ImageElementInfo{
						Name:   doOp.XObjectName,
						X:      x,
						Y:      y,
						Width:  float64(xobj.Width),
						Height: float64(xobj.Height),
					})
				}
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

// renderPDFPageToCairo 将 PDF 页面内容渲染到 Cairo context
func renderPDFPageToCairo(pdfPath string, pageNum int, cairoCtx cairo.Context, width, height float64) error {
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

	// 保存 Cairo 状态
	cairoCtx.Save()
	defer cairoCtx.Restore()

	// 设置裁剪区域，防止内容超出页面边界
	cairoCtx.Rectangle(0, 0, width, height)
	cairoCtx.Clip()

	// PDF 坐标系转换：PDF 使用左下角为原点，Y 轴向上
	// Cairo 使用左上角为原点，Y 轴向下
	// 需要翻转 Y 轴并平移
	cairoCtx.Translate(0, height)
	cairoCtx.Scale(1, -1)

	// 处理页面的 MediaBox, CropBox, Rotate 等属性
	if err := applyPageTransformations(pageDict, cairoCtx, width, height); err != nil {
		debugPrintf("Warning: failed to apply page transformations: %v\n", err)
	}

	// 创建渲染上下文
	renderCtx := NewRenderContext(cairoCtx, width, height)

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
	contentStreams, err := extractContentStreams(ctx, contents)
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

	return nil
}

// applyPageTransformations 应用页面级别的变换（旋转、裁剪等）
func applyPageTransformations(pageDict types.Dict, cairoCtx cairo.Context, width, height float64) error {
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
				cairoCtx.Translate(width, 0)
				cairoCtx.Rotate(1.5707963267948966) // π/2
			case 180:
				cairoCtx.Translate(width, height)
				cairoCtx.Rotate(3.141592653589793) // π
			case 270:
				cairoCtx.Translate(0, height)
				cairoCtx.Rotate(4.71238898038469) // 3π/2
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
				cairoCtx.Translate(-x1, -y1)
			}
		}
	}

	return nil
}

// extractContentStreams 提取页面的所有内容流
func extractContentStreams(ctx *model.Context, contents types.Object) ([][]byte, error) {
	var streams [][]byte

	switch obj := contents.(type) {
	case types.IndirectRef:
		// 解引用
		derefObj, err := ctx.Dereference(obj)
		if err != nil {
			return nil, fmt.Errorf("failed to dereference contents: %w", err)
		}
		debugPrintf("   Dereferenced to: %T\n", derefObj)
		return extractContentStreams(ctx, derefObj)

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
			itemStreams, err := extractContentStreams(ctx, item)
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

	// 加载字体
	if fontsObj, found := resourcesDict.Find("Font"); found {
		if fontsDict, ok := fontsObj.(types.Dict); ok {
			for fontName, fontObj := range fontsDict {
				if err := loadFont(ctx, fontName, fontObj, resources); err != nil {
					debugPrintf("Warning: failed to load font %s: %v\n", fontName, err)
				}
			}
		}
	}

	// 加载 XObjects
	if xobjectsObj, found := resourcesDict.Find("XObject"); found {
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
		if extGStateDict, ok := extGStateObj.(types.Dict); ok {
			for gsName, gsObj := range extGStateDict {
				if err := loadExtGState(ctx, gsName, gsObj, resources); err != nil {
					debugPrintf("Warning: failed to load ExtGState %s: %v\n", gsName, err)
				}
			}
		}
	}

	return nil
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
		if name, ok := encoding.(types.Name); ok {
			font.Encoding = name.String()
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

	// 加载 ToUnicode CMap（用于 CID 字体）
	if toUnicodeObj, found := fontDict.Find("ToUnicode"); found {
		if indRef, ok := toUnicodeObj.(types.IndirectRef); ok {
			// 解引用 ToUnicode 流
			derefObj, err := ctx.Dereference(indRef)
			if err == nil {
				if streamDict, ok := derefObj.(types.StreamDict); ok {
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
						if err == nil {
							font.ToUnicodeMap = cidMap
							debugPrintf("✓ Loaded ToUnicode CMap for font %s (%d mappings, %d ranges)\n",
								fontName, len(cidMap.Mappings), len(cidMap.Ranges))
						} else {
							debugPrintf("Warning: failed to parse ToUnicode CMap for font %s: %v\n", fontName, err)
						}
					}
				}
			}
		}
	}

	// 检查是否使用 Identity-H 或 Identity-V 编码
	isIdentity := false
	if font.Encoding == "/Identity-H" || font.Encoding == "Identity-H" ||
		font.Encoding == "/Identity-V" || font.Encoding == "Identity-V" {
		isIdentity = true
		font.IsIdentity = true
		debugPrintf("✓ Detected Identity encoding for font %s: %s\n", fontName, font.Encoding)
	}

	// 如果没有 ToUnicode，尝试从 poppler-data 加载
	if font.ToUnicodeMap == nil && font.Subtype == "/Type0" {
		// 尝试从字体名称推断 CID 系统信息
		// 例如: MicrosoftYaHeiUI-Bold 可能是中文字体
		registry := guessCIDRegistry(font.BaseFont)
		if registry != "" {
			debugPrintf("→ Trying to load CID map from poppler-data: %s for font %s\n", registry, fontName)
			cidMap, err := LoadCIDToUnicodeFromRegistry(registry)
			if err == nil {
				font.ToUnicodeMap = cidMap
				font.CIDSystemInfo = registry
				debugPrintf("✓ Loaded CID map from poppler-data: %s (%d mappings)\n", registry, len(cidMap.Mappings))
			} else {
				debugPrintf("Warning: failed to load CID map for %s: %v\n", registry, err)
				// 如果加载失败，尝试使用Identity映射作为后备
				if !isIdentity {
					debugPrintf("→ Falling back to Identity mapping for font %s\n", fontName)
					font.IsIdentity = true
				}
			}
		} else if !isIdentity {
			// 如果无法推断注册表，使用Identity映射作为后备
			debugPrintf("→ Cannot guess CID registry, using Identity mapping for font %s\n", fontName)
			font.IsIdentity = true
		}
	}

	resources.AddFont(fontName, font)
	return nil
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
	decoded, _, err := ctx.DereferenceStreamDict(streamDict)
	if err != nil {
		return fmt.Errorf("failed to decode XObject stream: %w", err)
	}
	if decoded != nil {
		xobj.Stream = decoded.Content
	}

	// 根据子类型加载特定属性
	switch xobj.Subtype {
	case "/Form":
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
					xobj.Matrix.A = float64(v)
				}
				if v, ok := arr[1].(types.Float); ok {
					xobj.Matrix.B = float64(v)
				}
				if v, ok := arr[2].(types.Float); ok {
					xobj.Matrix.C = float64(v)
				}
				if v, ok := arr[3].(types.Float); ok {
					xobj.Matrix.D = float64(v)
				}
				if v, ok := arr[4].(types.Float); ok {
					xobj.Matrix.E = float64(v)
				}
				if v, ok := arr[5].(types.Float); ok {
					xobj.Matrix.F = float64(v)
				}
			}
		}

	case "/Image":
		// 加载图像 XObject 属性
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

		if colorSpace, found := streamDict.Find("ColorSpace"); found {
			if name, ok := colorSpace.(types.Name); ok {
				xobj.ColorSpace = name.String()
			}
		}

		if bpc, found := streamDict.Find("BitsPerComponent"); found {
			if num, ok := bpc.(types.Integer); ok {
				xobj.BitsPerComponent = int(num)
			}
		}
	}

	resources.AddXObject(xobjName, xobj)
	return nil
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
				// 处理转义字符
				text = strings.ReplaceAll(text, "\\n", "\n")
				text = strings.ReplaceAll(text, "\\r", "")
				text = strings.ReplaceAll(text, "\\t", "\t")
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
						result.WriteString(" ")
					} else if stream[j] == '\'' || stream[j] == '"' {
						result.WriteString(text)
						result.WriteString("\n")
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
						text = strings.ReplaceAll(text, "\\n", "\n")
						text = strings.ReplaceAll(text, "\\r", "")
						text = strings.ReplaceAll(text, "\\t", "\t")
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
					result.WriteString(" ")
					i += 2
				}
			}
			continue
		}

		i++
	}

	text := result.String()
	if text == "" {
		return ""
	}

	// 清理多余的空白
	text = strings.TrimSpace(text)
	return text
}

// ConvertCairoSurfaceToImage 将 Cairo surface 转换为 Go image.Image（导出供外部使用）
func ConvertCairoSurfaceToImage(imgSurf cairo.ImageSurface) image.Image {
	data := imgSurf.GetData()
	stride := imgSurf.GetStride()
	width := imgSurf.GetWidth()
	height := imgSurf.GetHeight()

	img := image.NewRGBA(image.Rect(0, 0, width, height))

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			offset := y*stride + x*4
			// Cairo 使用 BGRA 预乘 alpha 格式
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

// ConvertPDFPageToImage 使用 Cairo 将 PDF 页面转换为图像的辅助函数
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
