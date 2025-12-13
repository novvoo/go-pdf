package gopdf

import (
	"fmt"
	"strings"

	"github.com/novvoo/go-cairo/pkg/cairo"
)

// TextState 文本状态
type TextState struct {
	// 文本矩阵
	TextMatrix *Matrix
	// 文本行矩阵
	TextLineMatrix *Matrix
	// 字符间距
	CharSpacing float64
	// 单词间距
	WordSpacing float64
	// 水平缩放
	HorizontalScaling float64
	// 行距
	Leading float64
	// 字体
	Font     *Font
	FontSize float64
	// 渲染模式
	RenderMode int // 0=填充, 1=描边, 2=填充+描边, 3=不可见, 4-7=裁剪变体
	// 上升
	Rise float64
}

// NewTextState 创建新的文本状态
func NewTextState() *TextState {
	return &TextState{
		TextMatrix:        NewIdentityMatrix(),
		TextLineMatrix:    NewIdentityMatrix(),
		CharSpacing:       0,
		WordSpacing:       0,
		HorizontalScaling: 100, // 100%
		Leading:           0,
		FontSize:          12,
		RenderMode:        0,
		Rise:              0,
	}
}

// Clone 复制文本状态
func (ts *TextState) Clone() *TextState {
	return &TextState{
		TextMatrix:        ts.TextMatrix.Clone(),
		TextLineMatrix:    ts.TextLineMatrix.Clone(),
		CharSpacing:       ts.CharSpacing,
		WordSpacing:       ts.WordSpacing,
		HorizontalScaling: ts.HorizontalScaling,
		Leading:           ts.Leading,
		Font:              ts.Font,
		FontSize:          ts.FontSize,
		RenderMode:        ts.RenderMode,
		Rise:              ts.Rise,
	}
}

// Font 字体信息
type Font struct {
	Name             string
	BaseFont         string
	Subtype          string
	Encoding         string
	ToUnicodeMap     *CIDToUnicodeMap // CID 字体的 Unicode 映射
	CIDSystemInfo    string           // CID 字体的系统信息 (Registry-Ordering)
	EmbeddedFontData []byte           // 嵌入的字体数据 (TTF/CFF)
	IsIdentity       bool             // 是否使用 Identity 映射 (CID = Unicode)
	Widths           *FontWidths      // 字形宽度信息
	DefaultWidth     float64          // 默认字形宽度（用于 CID 字体）
	MissingWidth     float64          // 缺失字形的宽度
}

// FontWidths 字形宽度信息
type FontWidths struct {
	// Type1/TrueType 字体：FirstChar 到 LastChar 的宽度数组
	FirstChar int
	LastChar  int
	Widths    []float64

	// CID 字体：CID 到宽度的映射
	CIDWidths map[uint16]float64
	// CID 字体：宽度范围
	CIDRanges []CIDWidthRange
}

// CIDWidthRange CID 字体的宽度范围
type CIDWidthRange struct {
	StartCID uint16
	EndCID   uint16
	Width    float64   // 如果是单一宽度
	Widths   []float64 // 如果是宽度数组
}

// GetWidth 获取字符的宽度（以千分之一 em 为单位）
func (f *Font) GetWidth(cid uint16) float64 {
	if f.Widths == nil {
		// 如果没有宽度信息，返回默认宽度
		if f.DefaultWidth > 0 {
			return f.DefaultWidth
		}
		// 使用通用默认值：500（半个 em）
		return 500.0
	}

	// CID 字体
	if f.Subtype == "/Type0" || len(f.Widths.CIDWidths) > 0 || len(f.Widths.CIDRanges) > 0 {
		// 首先查找直接映射
		if width, ok := f.Widths.CIDWidths[cid]; ok {
			return width
		}

		// 然后查找范围映射
		for _, r := range f.Widths.CIDRanges {
			if cid >= r.StartCID && cid <= r.EndCID {
				if r.Width > 0 {
					// 单一宽度
					return r.Width
				}
				if len(r.Widths) > 0 {
					// 宽度数组
					offset := int(cid - r.StartCID)
					if offset < len(r.Widths) {
						return r.Widths[offset]
					}
				}
			}
		}

		// 使用默认宽度
		if f.DefaultWidth > 0 {
			return f.DefaultWidth
		}
		if f.MissingWidth > 0 {
			return f.MissingWidth
		}
		return 500.0
	}

	// Type1/TrueType 字体
	if len(f.Widths.Widths) > 0 {
		charCode := int(cid)
		if charCode >= f.Widths.FirstChar && charCode <= f.Widths.LastChar {
			offset := charCode - f.Widths.FirstChar
			if offset < len(f.Widths.Widths) {
				return f.Widths.Widths[offset]
			}
		}
	}

	// 使用默认宽度
	if f.MissingWidth > 0 {
		return f.MissingWidth
	}
	return 500.0
}

// ===== 文本对象操作符 =====

// OpBeginText BT - 开始文本对象
type OpBeginText struct{}

func (op *OpBeginText) Name() string { return "BT" }

func (op *OpBeginText) Execute(ctx *RenderContext) error {
	// 重置文本矩阵和文本行矩阵为单位矩阵
	ctx.TextState.TextMatrix = NewIdentityMatrix()
	ctx.TextState.TextLineMatrix = NewIdentityMatrix()
	debugPrintf("[BT] Begin text object - Reset text matrices\n")
	return nil
}

// OpEndText ET - 结束文本对象
type OpEndText struct{}

func (op *OpEndText) Name() string { return "ET" }

func (op *OpEndText) Execute(ctx *RenderContext) error {
	// 文本对象结束，不需要特殊处理
	debugPrintf("[ET] End text object\n")
	return nil
}

// ===== 文本定位操作符 =====

// OpSetTextMatrix Tm - 设置文本矩阵
type OpSetTextMatrix struct {
	Matrix *Matrix
}

func (op *OpSetTextMatrix) Name() string { return "Tm" }

func (op *OpSetTextMatrix) Execute(ctx *RenderContext) error {
	ctx.TextState.TextMatrix = op.Matrix.Clone()
	ctx.TextState.TextLineMatrix = op.Matrix.Clone()

	// 注意：文本矩阵是独立的，不应该影响图形状态的 CTM
	// 文本渲染时会单独应用文本矩阵
	debugPrintf("[Tm] Set text matrix: [%.2f %.2f %.2f %.2f %.2f %.2f]\n",
		op.Matrix.A, op.Matrix.B, op.Matrix.C, op.Matrix.D, op.Matrix.E, op.Matrix.F)

	return nil
}

// OpMoveTextPosition Td - 移动文本位置
type OpMoveTextPosition struct {
	Tx, Ty float64
}

func (op *OpMoveTextPosition) Name() string { return "Td" }

func (op *OpMoveTextPosition) Execute(ctx *RenderContext) error {
	// 根据PDF规范：Tm = Tlm = Tlm × [1 0 0 1 tx ty]
	// 正确的矩阵乘法顺序：先应用当前矩阵，再应用平移
	translation := NewTranslationMatrix(op.Tx, op.Ty)
	ctx.TextState.TextLineMatrix = ctx.TextState.TextLineMatrix.Multiply(translation)
	ctx.TextState.TextMatrix = ctx.TextState.TextLineMatrix.Clone()

	// 注意：文本矩阵是独立的，不应该影响图形状态的 CTM
	// 文本渲染时会单独应用文本矩阵
	debugPrintf("[Td] Move text position: tx=%.2f, ty=%.2f -> New Tm: [%.2f %.2f %.2f %.2f %.2f %.2f]\n",
		op.Tx, op.Ty,
		ctx.TextState.TextMatrix.A, ctx.TextState.TextMatrix.B,
		ctx.TextState.TextMatrix.C, ctx.TextState.TextMatrix.D,
		ctx.TextState.TextMatrix.E, ctx.TextState.TextMatrix.F)

	return nil
}

// OpMoveTextPositionSetLeading TD - 移动文本位置并设置行距
type OpMoveTextPositionSetLeading struct {
	Tx, Ty float64
}

func (op *OpMoveTextPositionSetLeading) Name() string { return "TD" }

func (op *OpMoveTextPositionSetLeading) Execute(ctx *RenderContext) error {
	ctx.TextState.Leading = -op.Ty
	return (&OpMoveTextPosition{Tx: op.Tx, Ty: op.Ty}).Execute(ctx)
}

// OpMoveToNextLine T* - 移动到下一行
type OpMoveToNextLine struct{}

func (op *OpMoveToNextLine) Name() string { return "T*" }

func (op *OpMoveToNextLine) Execute(ctx *RenderContext) error {
	return (&OpMoveTextPosition{
		Tx: 0,
		Ty: -ctx.TextState.Leading,
	}).Execute(ctx)
}

// ===== 文本状态操作符 =====

// OpSetCharSpacing Tc - 设置字符间距
type OpSetCharSpacing struct {
	Spacing float64
}

func (op *OpSetCharSpacing) Name() string { return "Tc" }

func (op *OpSetCharSpacing) Execute(ctx *RenderContext) error {
	ctx.TextState.CharSpacing = op.Spacing
	return nil
}

// OpSetWordSpacing Tw - 设置单词间距
type OpSetWordSpacing struct {
	Spacing float64
}

func (op *OpSetWordSpacing) Name() string { return "Tw" }

func (op *OpSetWordSpacing) Execute(ctx *RenderContext) error {
	ctx.TextState.WordSpacing = op.Spacing
	return nil
}

// OpSetHorizontalScaling Tz - 设置水平缩放
type OpSetHorizontalScaling struct {
	Scale float64 // 百分比
}

func (op *OpSetHorizontalScaling) Name() string { return "Tz" }

func (op *OpSetHorizontalScaling) Execute(ctx *RenderContext) error {
	ctx.TextState.HorizontalScaling = op.Scale
	return nil
}

// OpSetLeading TL - 设置行距
type OpSetLeading struct {
	Leading float64
}

func (op *OpSetLeading) Name() string { return "TL" }

func (op *OpSetLeading) Execute(ctx *RenderContext) error {
	ctx.TextState.Leading = op.Leading
	return nil
}

// OpSetFont Tf - 设置字体和字号
type OpSetFont struct {
	FontName string
	FontSize float64
}

func (op *OpSetFont) Name() string { return "Tf" }

func (op *OpSetFont) Execute(ctx *RenderContext) error {
	// 设置字体大小，如果为0则使用默认值12
	if op.FontSize > 0 {
		ctx.TextState.FontSize = op.FontSize
	} else {
		// 字体大小为0可能意味着字体大小在文本矩阵中指定
		// 保持当前字体大小或使用默认值
		if ctx.TextState.FontSize == 0 {
			ctx.TextState.FontSize = 12
		}
	}

	// 从资源中获取字体
	font := ctx.Resources.GetFont(op.FontName)
	if font != nil {
		ctx.TextState.Font = font
		debugPrintf("[Tf] Set font: %s (BaseFont: %s), Size: %.2f\n",
			op.FontName, font.BaseFont, ctx.TextState.FontSize)
	} else {
		// 使用默认字体
		ctx.TextState.Font = &Font{
			Name:     op.FontName,
			BaseFont: "Helvetica",
			Subtype:  "Type1",
			Encoding: "WinAnsiEncoding",
		}
		debugPrintf("[Tf] Set font: %s (default), Size: %.2f\n",
			op.FontName, ctx.TextState.FontSize)
	}
	return nil
}

// OpSetTextRenderMode Tr - 设置文本渲染模式
type OpSetTextRenderMode struct {
	Mode int
}

func (op *OpSetTextRenderMode) Name() string { return "Tr" }

func (op *OpSetTextRenderMode) Execute(ctx *RenderContext) error {
	ctx.TextState.RenderMode = op.Mode
	return nil
}

// OpSetTextRise Ts - 设置文本上升
type OpSetTextRise struct {
	Rise float64
}

func (op *OpSetTextRise) Name() string { return "Ts" }

func (op *OpSetTextRise) Execute(ctx *RenderContext) error {
	ctx.TextState.Rise = op.Rise
	return nil
}

// ===== 文本显示操作符 =====

// OpShowText Tj - 显示文本
type OpShowText struct {
	Text string
}

func (op *OpShowText) Name() string { return "Tj" }

func (op *OpShowText) Execute(ctx *RenderContext) error {
	return renderText(ctx, op.Text, nil)
}

// OpShowTextNextLine ' - 移到下一行并显示文本
type OpShowTextNextLine struct {
	Text string
}

func (op *OpShowTextNextLine) Name() string { return "'" }

func (op *OpShowTextNextLine) Execute(ctx *RenderContext) error {
	// 等同于 T* Tj
	// 先移动到下一行
	if err := (&OpMoveToNextLine{}).Execute(ctx); err != nil {
		return err
	}
	// 然后显示文本（会自动更新TextMatrix）
	debugPrintf("['] Moving to next line and showing text\n")
	return (&OpShowText{Text: op.Text}).Execute(ctx)
}

// OpShowTextWithSpacing " - 设置间距并显示文本
type OpShowTextWithSpacing struct {
	WordSpacing float64
	CharSpacing float64
	Text        string
}

func (op *OpShowTextWithSpacing) Name() string { return "\"" }

func (op *OpShowTextWithSpacing) Execute(ctx *RenderContext) error {
	// 等同于 Tw Tc T* Tj
	// 先设置间距参数
	debugPrintf("[\"] Setting WordSpacing=%.4f CharSpacing=%.4f\n", op.WordSpacing, op.CharSpacing)
	ctx.TextState.WordSpacing = op.WordSpacing
	ctx.TextState.CharSpacing = op.CharSpacing
	// 然后移动到下一行并显示文本
	return (&OpShowTextNextLine{Text: op.Text}).Execute(ctx)
}

// OpShowTextArray TJ - 显示文本数组（带位置调整）
type OpShowTextArray struct {
	Array []interface{} // string 或 float64
}

func (op *OpShowTextArray) Name() string { return "TJ" }

func (op *OpShowTextArray) Execute(ctx *RenderContext) error {
	return renderText(ctx, "", op.Array)
}

// renderText 渲染文本到 Cairo
func renderText(ctx *RenderContext, text string, array []interface{}) error {
	state := ctx.GetCurrentState()
	textState := ctx.TextState

	// 调试输出：文本状态
	debugPrintf("\n[TEXT_STATE] CharSpacing=%.4f WordSpacing=%.4f HScale=%.2f%% FontSize=%.2f\n",
		textState.CharSpacing, textState.WordSpacing, textState.HorizontalScaling, textState.FontSize)

	// 保存 Cairo 状态
	ctx.CairoCtx.Save()
	defer ctx.CairoCtx.Restore()

	// 应用文本矩阵
	// 注意：由于 renderPDFPageToCairo 已经应用了全局 Y 轴翻转 (Scale(1, -1))
	// 而某些 PDF 的 Tm 矩阵中也包含了 Y 轴翻转 (d=-1)
	// 我们需要检测并处理这种情况，避免双重翻转
	tm := textState.TextMatrix.Clone()

	// 如果文本矩阵的 D 分量是负数，说明 PDF 已经做了 Y 轴翻转
	// 这种情况下，F 值已经是从顶部算起的坐标
	// 全局变换做了 Translate(0, height) + Scale(1, -1)
	// 所以我们只需要反转 D，保持 F 不变
	if tm.D < 0 {
		// 只反转 D，不改变 F
		tm.D = -tm.D
		// F 保持不变，因为它已经是正确的坐标
	}

	tm.ApplyToCairoContext(ctx.CairoCtx)

	// 应用文本上升
	if textState.Rise != 0 {
		ctx.CairoCtx.Translate(0, textState.Rise)
	}

	// 设置字体
	// 计算有效字体大小
	// 在某些PDF中，字体大小通过文本矩阵的缩放来指定
	fontSize := textState.FontSize

	// 如果基础字体大小为0或很小，从文本矩阵中提取字体大小
	if fontSize < 1.0 && textState.TextMatrix != nil {
		// 从文本矩阵中提取缩放因子
		// 文本矩阵格式: [a b c d e f]
		// a 和 d 是水平和垂直缩放因子
		scaleX := textState.TextMatrix.A
		scaleY := textState.TextMatrix.D

		if scaleX < 0 {
			scaleX = -scaleX
		}
		if scaleY < 0 {
			scaleY = -scaleY
		}

		// 使用较大的缩放因子作为字体大小
		if scaleX > scaleY {
			fontSize = scaleX
		} else {
			fontSize = scaleY
		}

		// 如果提取的字体大小仍然太小，使用默认值
		if fontSize < 1.0 {
			fontSize = 12.0
		}
	} else if textState.TextMatrix != nil {
		// 如果有基础字体大小，应用文本矩阵的缩放
		scale := textState.TextMatrix.D
		if scale < 0 {
			scale = -scale
		}
		if scale > 0.001 {
			fontSize = fontSize * scale
		}
	}

	fontFamily := "sans-serif"
	if textState.Font != nil && textState.Font.BaseFont != "" {
		fontFamily = mapPDFFont(textState.Font.BaseFont)
	}

	// 获取当前字体的 ToUnicode 映射
	var toUnicodeMap *CIDToUnicodeMap
	if textState.Font != nil {
		toUnicodeMap = textState.Font.ToUnicodeMap
	}

	// 使用 PangoCairo 渲染文本
	layout := ctx.CairoCtx.PangoCairoCreateLayout().(*cairo.PangoCairoLayout)
	fontDesc := cairo.NewPangoFontDescription()

	// 检查是否有嵌入的字体数据
	if textState.Font != nil && len(textState.Font.EmbeddedFontData) > 0 {
		// 使用嵌入的字体数据
		// 尝试创建自定义字体
		userFont := cairo.NewUserFontFace()
		if userFont != nil {
			// 这里我们暂时使用字体族名称，但在实际应用中，
			// 我们需要将 EmbeddedFontData 传递给字体渲染系统
			fontDesc.SetFamily(fontFamily)
			debugPrintf("✓ Using embedded font data for font %s (%d bytes)\n", fontFamily, len(textState.Font.EmbeddedFontData))

			// TODO: 实际的字体数据加载需要在底层的cairo/pango库中实现
			// 当前版本的go-cairo可能不直接支持从[]byte加载字体
		} else {
			// 回退到系统字体
			fontDesc.SetFamily(fontFamily)
			debugPrintf("⚠️  Failed to create user font, falling back to system font: %s\n", fontFamily)
		}
	} else {
		// 使用系统字体
		fontDesc.SetFamily(fontFamily)
	}

	fontDesc.SetSize(fontSize)
	layout.SetFontDescription(fontDesc)

	// 应用水平缩放
	horizontalScale := 1.0
	if textState.HorizontalScaling != 100 {
		horizontalScale = textState.HorizontalScaling / 100.0
		ctx.CairoCtx.Scale(horizontalScale, 1.0)
	}

	// 设置颜色（根据渲染模式）
	switch textState.RenderMode {
	case 0: // 填充
		if state.FillColor != nil {
			debugPrintf("[TEXT_STATE] Using FillColor: RGB(%.3f, %.3f, %.3f, %.3f)\n",
				state.FillColor.R, state.FillColor.G, state.FillColor.B, state.FillColor.A)
			ctx.CairoCtx.SetSourceRGBA(
				state.FillColor.R,
				state.FillColor.G,
				state.FillColor.B,
				state.FillColor.A,
			)
		} else {
			// 默认使用黑色
			debugPrintf("[TEXT_STATE] Using default black color\n")
			ctx.CairoCtx.SetSourceRGBA(0, 0, 0, 1)
		}
	case 1: // 描边
		if state.StrokeColor != nil {
			ctx.CairoCtx.SetSourceRGBA(
				state.StrokeColor.R,
				state.StrokeColor.G,
				state.StrokeColor.B,
				state.StrokeColor.A,
			)
		}
	case 2: // 填充+描边
		if state.FillColor != nil {
			ctx.CairoCtx.SetSourceRGBA(
				state.FillColor.R,
				state.FillColor.G,
				state.FillColor.B,
				state.FillColor.A,
			)
		}
	case 3: // 不可见
		return nil
	}

	// 计算文本位移（用于更新文本矩阵）
	var textDisplacement float64

	// 渲染文本
	if array != nil {
		// TJ 操作符：处理文本数组
		debugPrintf("[TJ_ARRAY] Processing %d items\n", len(array))
		x := 0.0
		totalTextWidth := 0.0 // 累计文本宽度用于更新文本矩阵

		for idx, item := range array {
			switch v := item.(type) {
			case string:
				// 解码文本并获取 CID 数组
				decodedText, cids := decodeTextStringWithCIDs(v, toUnicodeMap, textState.Font)
				if decodedText == "" {
					// 如果无法解码，跳过
					debugPrintf("[TJ_ARRAY][%d] Empty string after decode\n", idx)
					continue
				}

				debugPrintf("[TJ_ARRAY][%d] Text=%q (len=%d runes, %d CIDs) at x=%.2f\n",
					idx, decodedText, len([]rune(decodedText)), len(cids), x)

				layout.SetText(decodedText)
				ctx.CairoCtx.MoveTo(x, 0)
				// 使用 PangoCairo 直接渲染文本（支持基本的字距调整）
				ctx.CairoCtx.PangoCairoShowText(layout)

				// 使用实际的字形宽度计算文本宽度
				textWidth := calculateTextWidth(cids, textState, decodedText)
				debugPrintf("[TJ_ARRAY][%d] Calculated width=%.2f\n", idx, textWidth)

				x += textWidth
				totalTextWidth += textWidth

			case float64:
				// PDF规范：负值表示向右移动，正值表示向左移动
				// 调整值以千分之一em为单位
				kerningAdjustment := -v * fontSize / 1000.0
				debugPrintf("[TJ_ARRAY][%d] Kerning=%.0f adj=%.2f (x: %.2f -> %.2f)\n",
					idx, v, kerningAdjustment, x, x+kerningAdjustment)
				x += kerningAdjustment
				totalTextWidth += kerningAdjustment

			case int:
				kerningAdjustment := -float64(v) * fontSize / 1000.0
				debugPrintf("[TJ_ARRAY][%d] Kerning=%d adj=%.2f (x: %.2f -> %.2f)\n",
					idx, v, kerningAdjustment, x, x+kerningAdjustment)
				x += kerningAdjustment
				totalTextWidth += kerningAdjustment
			}
		}

		// TJ 操作符应该更新文本矩阵位置
		// 应用水平缩放到总位移
		textDisplacement = totalTextWidth * horizontalScale
		debugPrintf("[TJ_ARRAY] Total displacement=%.2f (totalWidth=%.2f × scale=%.2f)\n",
			textDisplacement, totalTextWidth, horizontalScale)
	} else {
		// Tj 操作符：简单文本
		// 解码文本并获取 CID 数组
		decodedText, cids := decodeTextStringWithCIDs(text, toUnicodeMap, textState.Font)
		if decodedText != "" {
			// 打印文本用于调试
			debugPrintf("[Tj] Text=%q (len=%d runes, %d CIDs) at Tm=[%.2f, %.2f]\n",
				decodedText, len([]rune(decodedText)), len(cids), tm.E, tm.F)
			layout.SetText(decodedText)
			debugPrintf("[Tj] About to render text at current position\n")
			ctx.CairoCtx.PangoCairoShowText(layout)
			debugPrintf("[Tj] Text rendered\n")

			// 使用实际的字形宽度计算文本宽度
			textWidth := calculateTextWidth(cids, textState, decodedText)
			debugPrintf("[Tj] Calculated width=%.2f\n", textWidth)

			// 应用水平缩放到位移
			textDisplacement = textWidth * horizontalScale
			debugPrintf("[Tj] Final displacement=%.2f (width=%.2f × scale=%.2f)\n", textDisplacement, textWidth, horizontalScale)
		}
	}

	// 在 Cairo 状态恢复后更新文本矩阵
	// 注意：文本位移应该在文本空间中进行
	// 根据 PDF 规范，文本位移是：Tm' = Tm × [1 0 0 1 tx 0]
	if textDisplacement != 0 {
		// 在文本空间中移动
		translation := NewTranslationMatrix(textDisplacement, 0)
		textState.TextMatrix = textState.TextMatrix.Multiply(translation)
	}

	return nil

	// 注意：由于go-cairo库的限制，无法完全实现高级的kerning功能
	// 当前实现已尽可能应用了TJ操作符中的数字偏移到文本位置
}

// decodeTextStringWithFont 使用字体的 ToUnicode 映射解码文本
func decodeTextStringWithFont(text string, toUnicodeMap *CIDToUnicodeMap) string {
	// 检查是否是十六进制字符串
	if len(text) >= 2 && text[0] == '<' && text[len(text)-1] == '>' {
		hexStr := text[1 : len(text)-1]
		hexStr = strings.ReplaceAll(hexStr, " ", "")

		// 转换十六进制到字节
		var result []byte
		for i := 0; i < len(hexStr); i += 2 {
			if i+1 < len(hexStr) {
				var b byte
				fmt.Sscanf(hexStr[i:i+2], "%02x", &b)
				result = append(result, b)
			}
		}

		// 如果有 ToUnicode 映射，使用它
		if toUnicodeMap != nil && len(result) >= 2 && len(result)%2 == 0 {
			var cids []uint16
			for i := 0; i < len(result); i += 2 {
				cid := uint16(result[i])<<8 | uint16(result[i+1])
				cids = append(cids, cid)
			}
			return toUnicodeMap.MapCIDsToUnicode(cids)
		}

		// 否则尝试标准解码
		return decodeTextString(text)
	}

	// 普通字符串
	return text
}

// decodeTextStringWithCIDs 解码文本并返回 Unicode 字符串和 CID 数组
func decodeTextStringWithCIDs(text string, toUnicodeMap *CIDToUnicodeMap, font *Font) (string, []uint16) {
	// 检查是否是十六进制字符串
	if len(text) >= 2 && text[0] == '<' && text[len(text)-1] == '>' {
		hexStr := text[1 : len(text)-1]
		hexStr = strings.ReplaceAll(hexStr, " ", "")

		// 转换十六进制到字节
		var result []byte
		for i := 0; i < len(hexStr); i += 2 {
			if i+1 < len(hexStr) {
				var b byte
				fmt.Sscanf(hexStr[i:i+2], "%02x", &b)
				result = append(result, b)
			}
		}

		if len(result) < 2 || len(result)%2 != 0 {
			return "", nil
		}

		// 提取CID数组
		var cids []uint16
		for i := 0; i < len(result); i += 2 {
			cid := uint16(result[i])<<8 | uint16(result[i+1])
			cids = append(cids, cid)
		}

		// 解码为 Unicode
		var decoded strings.Builder
		isIdentity := font != nil && font.IsIdentity

		// 如果有 ToUnicode 映射，优先使用它
		if toUnicodeMap != nil {
			allMapped := true
			for _, cid := range cids {
				if uni, ok := toUnicodeMap.MapCIDToUnicode(cid); ok {
					decoded.WriteRune(uni)
				} else {
					allMapped = false
					break
				}
			}

			// 如果所有CID都成功映射，返回结果
			if allMapped {
				return decoded.String(), cids
			}
			decoded.Reset()
		}

		// 如果ToUnicode映射失败或不存在，且是Identity映射，CID直接等于Unicode码点
		if isIdentity {
			for _, cid := range cids {
				decoded.WriteRune(rune(cid))
			}
			return decoded.String(), cids
		}

		// 否则尝试标准解码
		decodedStr := decodeTextString(text)
		return decodedStr, cids
	}

	// 普通字符串 - 转换为 CID 数组（字节码）
	var cids []uint16
	for i := 0; i < len(text); i++ {
		cids = append(cids, uint16(text[i]))
	}
	return text, cids
}

// calculateTextWidth 使用字形宽度计算文本宽度
func calculateTextWidth(cids []uint16, textState *TextState, decodedText string) float64 {
	if textState.Font == nil || len(cids) == 0 {
		// 回退到简单估算
		runeCount := float64(len([]rune(decodedText)))
		return runeCount * textState.FontSize * 0.5
	}

	totalWidth := 0.0

	// 使用字形宽度计算
	for _, cid := range cids {
		// 获取字形宽度（以千分之一 em 为单位）
		glyphWidth := textState.Font.GetWidth(cid)
		// 转换为用户空间单位：width = glyphWidth * fontSize / 1000
		width := glyphWidth * textState.FontSize / 1000.0
		totalWidth += width

		// 应用字符间距
		totalWidth += textState.CharSpacing
	}

	// 应用单词间距（只对空格字符）
	if textState.WordSpacing != 0 {
		spaceCount := 0
		for _, ch := range decodedText {
			if ch == ' ' {
				spaceCount++
			}
		}
		totalWidth += textState.WordSpacing * float64(spaceCount)
	}

	return totalWidth
}

// decodeTextStringWithFontAndIdentity 使用字体的 ToUnicode 映射解码文本，支持Identity映射
func decodeTextStringWithFontAndIdentity(text string, toUnicodeMap *CIDToUnicodeMap, isIdentity bool) string {
	// 检查是否是十六进制字符串
	if len(text) >= 2 && text[0] == '<' && text[len(text)-1] == '>' {
		hexStr := text[1 : len(text)-1]
		hexStr = strings.ReplaceAll(hexStr, " ", "")

		// 转换十六进制到字节
		var result []byte
		for i := 0; i < len(hexStr); i += 2 {
			if i+1 < len(hexStr) {
				var b byte
				fmt.Sscanf(hexStr[i:i+2], "%02x", &b)
				result = append(result, b)
			}
		}

		if len(result) < 2 || len(result)%2 != 0 {
			return ""
		}

		// 提取CID数组
		var cids []uint16
		for i := 0; i < len(result); i += 2 {
			cid := uint16(result[i])<<8 | uint16(result[i+1])
			cids = append(cids, cid)
		}

		// 如果有 ToUnicode 映射，优先使用它
		if toUnicodeMap != nil {
			var decoded strings.Builder
			allMapped := true

			for _, cid := range cids {
				if uni, ok := toUnicodeMap.MapCIDToUnicode(cid); ok {
					decoded.WriteRune(uni)
				} else {
					allMapped = false
					break
				}
			}

			// 如果所有CID都成功映射，返回结果
			if allMapped {
				return decoded.String()
			}
		}

		// 如果ToUnicode映射失败或不存在，且是Identity映射，CID直接等于Unicode码点
		if isIdentity {
			var runes []rune
			for _, cid := range cids {
				runes = append(runes, rune(cid))
			}
			return string(runes)
		}

		// 否则尝试标准解码
		return decodeTextString(text)
	}

	// 普通字符串
	return text
}

// decodeTextString 解码 PDF 文本字符串
// 处理普通字符串和十六进制字符串 <...>
func decodeTextString(text string) string {
	// 检查是否是十六进制字符串
	if len(text) >= 2 && text[0] == '<' && text[len(text)-1] == '>' {
		// 十六进制字符串：<48656C6C6F> -> "Hello"
		hexStr := text[1 : len(text)-1]

		// 移除空格
		hexStr = strings.ReplaceAll(hexStr, " ", "")

		// 转换十六进制到字节
		var result []byte
		for i := 0; i < len(hexStr); i += 2 {
			if i+1 < len(hexStr) {
				var b byte
				fmt.Sscanf(hexStr[i:i+2], "%02x", &b)
				result = append(result, b)
			}
		}

		// 尝试 UTF-16BE 解码（CID 字体常用）
		if len(result) >= 2 && len(result)%2 == 0 {
			// 检查是否有 BOM
			if result[0] == 0xFE && result[1] == 0xFF {
				result = result[2:] // 跳过 BOM
			}

			// UTF-16BE 解码
			var runes []rune
			for i := 0; i < len(result); i += 2 {
				if i+1 < len(result) {
					r := rune(result[i])<<8 | rune(result[i+1])
					if r != 0 {
						runes = append(runes, r)
					}
				}
			}
			if len(runes) > 0 {
				return string(runes)
			}
		}

		// 如果不是 UTF-16，尝试作为 Latin-1
		// 但首先检查是否是 CID 字体的字形 ID
		// CID 通常是 2 字节的值，如果所有字节都 > 0，可能是 CID
		if len(result) >= 2 && len(result)%2 == 0 {
			allHighBytes := true
			for i := 0; i < len(result); i += 2 {
				if result[i] == 0 {
					allHighBytes = false
					break
				}
			}
			if allHighBytes {
				// 可能是 CID 字体，返回占位符
				// 每个 CID 用一个方块表示
				return strings.Repeat("■", len(result)/2)
			}
		}

		return string(result)
	}

	// 普通字符串，直接返回
	return text
}

// mapPDFFont 将 PDF 字体名称映射到系统字体
func mapPDFFont(pdfFont string) string {
	fontMap := map[string]string{
		"Helvetica":             "sans-serif",
		"Helvetica-Bold":        "sans-serif",
		"Helvetica-Oblique":     "sans-serif",
		"Helvetica-BoldOblique": "sans-serif",
		"Times-Roman":           "serif",
		"Times-Bold":            "serif",
		"Times-Italic":          "serif",
		"Times-BoldItalic":      "serif",
		"Courier":               "monospace",
		"Courier-Bold":          "monospace",
		"Courier-Oblique":       "monospace",
		"Courier-BoldOblique":   "monospace",
		"Symbol":                "sans-serif",
		"ZapfDingbats":          "sans-serif",
	}

	if mapped, ok := fontMap[pdfFont]; ok {
		return mapped
	}
	return "sans-serif"
}
