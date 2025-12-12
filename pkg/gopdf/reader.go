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

	// PDF 坐标系转换：PDF 使用左下角为原点，Y 轴向上
	// Cairo 使用左上角为原点，Y 轴向下
	// 需要翻转 Y 轴并平移
	cairoCtx.Translate(0, height)
	cairoCtx.Scale(1, -1)

	// 处理页面的 MediaBox, CropBox, Rotate 等属性
	if err := applyPageTransformations(pageDict, cairoCtx, width, height); err != nil {
		fmt.Printf("Warning: failed to apply page transformations: %v\n", err)
	}

	// 创建渲染上下文
	renderCtx := NewRenderContext(cairoCtx, width, height)

	// 提取页面资源
	if resourcesObj, found := pageDict.Find("Resources"); found {
		if err := loadResources(ctx, resourcesObj, renderCtx.Resources); err != nil {
			fmt.Printf("Warning: failed to load resources: %v\n", err)
		}
	}

	// 提取页面内容流
	contents, found := pageDict.Find("Contents")
	if !found {
		fmt.Println("⚠️  Page has no Contents entry")
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
		fmt.Println("⚠️  Content stream is empty or too small, PDF may have no vector content")
		return nil
	}

	// 解析操作符
	operators, err := ParseContentStream(allContent)
	if err != nil {
		return fmt.Errorf("failed to parse content stream: %w", err)
	}

	// 执行所有操作符
	fmt.Printf("📊 Executing %d PDF operators...\n", len(operators))

	opCount := make(map[string]int)
	for _, op := range operators {
		opCount[op.Name()]++
		if err := op.Execute(renderCtx); err != nil {
			// 继续执行，不中断渲染
			fmt.Printf("⚠️  Operator %s failed: %v\n", op.Name(), err)
		}
	}

	// 显示操作符统计
	fmt.Println("\n📈 Operator Statistics:")
	for opName, count := range opCount {
		if count > 0 {
			fmt.Printf("   %s: %d\n", opName, count)
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
		fmt.Printf("   Dereferenced to: %T\n", derefObj)
		return extractContentStreams(ctx, derefObj)

	case types.StreamDict:
		// 单个流
		fmt.Printf("   Decoding StreamDict...\n")
		fmt.Printf("   Raw: %d bytes, Content: %d bytes\n", len(obj.Raw), len(obj.Content))

		// 如果 Content 为空但 Raw 不为空，需要解码
		if len(obj.Content) == 0 && len(obj.Raw) > 0 {
			fmt.Printf("   Calling Decode()...\n")
			err := obj.Decode()
			if err != nil {
				fmt.Printf("   ⚠️  Decode error: %v\n", err)
				return nil, fmt.Errorf("failed to decode stream: %w", err)
			}
			fmt.Printf("   ✓ After decode: %d bytes\n", len(obj.Content))
		}

		if len(obj.Content) > 0 {
			streams = append(streams, obj.Content)
		} else {
			fmt.Printf("   ⚠️  No content available\n")
		}

	case types.Array:
		// 多个流
		fmt.Printf("   Processing array with %d items\n", len(obj))
		for i, item := range obj {
			fmt.Printf("   Array item %d: %T\n", i, item)
			itemStreams, err := extractContentStreams(ctx, item)
			if err == nil {
				streams = append(streams, itemStreams...)
			} else {
				fmt.Printf("   ⚠️  Error extracting item %d: %v\n", i, err)
			}
		}

	default:
		fmt.Printf("   ⚠️  Unknown contents type: %T\n", obj)
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
					fmt.Printf("Warning: failed to load font %s: %v\n", fontName, err)
				}
			}
		}
	}

	// 加载 XObjects
	if xobjectsObj, found := resourcesDict.Find("XObject"); found {
		if xobjectsDict, ok := xobjectsObj.(types.Dict); ok {
			for xobjName, xobjObj := range xobjectsDict {
				if err := loadXObject(ctx, xobjName, xobjObj, resources); err != nil {
					fmt.Printf("Warning: failed to load XObject %s: %v\n", xobjName, err)
				}
			}
		}
	}

	// 加载扩展图形状态
	if extGStateObj, found := resourcesDict.Find("ExtGState"); found {
		if extGStateDict, ok := extGStateObj.(types.Dict); ok {
			for gsName, gsObj := range extGStateDict {
				if err := loadExtGState(ctx, gsName, gsObj, resources); err != nil {
					fmt.Printf("Warning: failed to load ExtGState %s: %v\n", gsName, err)
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
							fmt.Printf("Warning: failed to decode ToUnicode stream for font %s: %v\n", fontName, err)
						}
					}

					// 解析 ToUnicode CMap
					if len(streamDict.Content) > 0 {
						cidMap, err := ParseToUnicodeCMap(streamDict.Content)
						if err == nil {
							font.ToUnicodeMap = cidMap
							fmt.Printf("✓ Loaded ToUnicode CMap for font %s (%d mappings, %d ranges)\n",
								fontName, len(cidMap.Mappings), len(cidMap.Ranges))
						} else {
							fmt.Printf("Warning: failed to parse ToUnicode CMap for font %s: %v\n", fontName, err)
						}
					}
				}
			}
		}
	}

	// 如果没有 ToUnicode，尝试从 poppler-data 加载
	if font.ToUnicodeMap == nil && font.Subtype == "/Type0" {
		// 尝试从字体名称推断 CID 系统信息
		// 例如: MicrosoftYaHeiUI-Bold 可能是中文字体
		registry := guessCIDRegistry(font.BaseFont)
		if registry != "" {
			fmt.Printf("→ Trying to load CID map from poppler-data: %s for font %s\n", registry, fontName)
			cidMap, err := LoadCIDToUnicodeFromRegistry(registry)
			if err == nil {
				font.ToUnicodeMap = cidMap
				font.CIDSystemInfo = registry
				fmt.Printf("✓ Loaded CID map from poppler-data: %s (%d mappings)\n", registry, len(cidMap.Mappings))
			} else {
				fmt.Printf("Warning: failed to load CID map for %s: %v\n", registry, err)
			}
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

