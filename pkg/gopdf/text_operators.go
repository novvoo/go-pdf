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
	Name          string
	BaseFont      string
	Subtype       string
	Encoding      string
	ToUnicodeMap  *CIDToUnicodeMap // CID 字体的 Unicode 映射
	CIDSystemInfo string           // CID 字体的系统信息 (Registry-Ordering)
}

// ===== 文本对象操作符 =====

// OpBeginText BT - 开始文本对象
type OpBeginText struct{}

func (op *OpBeginText) Name() string { return "BT" }

func (op *OpBeginText) Execute(ctx *RenderContext) error {
	// 重置文本矩阵和文本行矩阵为单位矩阵
	ctx.TextState.TextMatrix = NewIdentityMatrix()
	ctx.TextState.TextLineMatrix = NewIdentityMatrix()
	return nil
}

// OpEndText ET - 结束文本对象
type OpEndText struct{}

func (op *OpEndText) Name() string { return "ET" }

func (op *OpEndText) Execute(ctx *RenderContext) error {
	// 文本对象结束，不需要特殊处理
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
	return nil
}

// OpMoveTextPosition Td - 移动文本位置
type OpMoveTextPosition struct {
	Tx, Ty float64
}

func (op *OpMoveTextPosition) Name() string { return "Td" }

func (op *OpMoveTextPosition) Execute(ctx *RenderContext) error {
	// Tm = Tlm = [1 0 0 1 tx ty] × Tlm
	translation := NewTranslationMatrix(op.Tx, op.Ty)
	ctx.TextState.TextLineMatrix = translation.Multiply(ctx.TextState.TextLineMatrix)
	ctx.TextState.TextMatrix = ctx.TextState.TextLineMatrix.Clone()
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
	ctx.TextState.FontSize = op.FontSize
	// 从资源中获取字体
	font := ctx.Resources.GetFont(op.FontName)
	if font != nil {
		ctx.TextState.Font = font
	} else {
		// 使用默认字体
		ctx.TextState.Font = &Font{
			Name:     op.FontName,
			BaseFont: "Helvetica",
			Subtype:  "Type1",
			Encoding: "WinAnsiEncoding",
		}
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
	if err := (&OpMoveToNextLine{}).Execute(ctx); err != nil {
		return err
	}
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
	// 等同于 Tw Tc '
	ctx.TextState.WordSpacing = op.WordSpacing
	ctx.TextState.CharSpacing = op.CharSpacing
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
	fontSize := textState.FontSize
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
	fontDesc.SetFamily(fontFamily)
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
			ctx.CairoCtx.SetSourceRGBA(
				state.FillColor.R,
				state.FillColor.G,
				state.FillColor.B,
				state.FillColor.A,
			)
		} else {
			// 默认使用黑色
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
		x := 0.0
		for _, item := range array {
			switch v := item.(type) {
			case string:
				// 解码文本（处理十六进制字符串和 CID 字体）
				decodedText := decodeTextStringWithFont(v, toUnicodeMap)
				if decodedText == "" {
					// 如果无法解码，跳过
					continue
				}

				layout.SetText(decodedText)
				ctx.CairoCtx.MoveTo(x, 0)
				ctx.CairoCtx.PangoCairoShowText(layout)

				// 计算文本宽度（估算，使用 rune 数量而不是字节数）
				textWidth := float64(len([]rune(decodedText))) * fontSize * 0.5
				x += textWidth

				// 应用字符间距
				x += textState.CharSpacing * float64(len([]rune(decodedText)))

			case float64:
				// 负值表示向右移动，正值表示向左移动
				x -= v * fontSize / 1000.0

			case int:
				x -= float64(v) * fontSize / 1000.0
			}
		}
		// TJ 操作符不更新文本矩阵
		textDisplacement = 0
	} else {
		// Tj 操作符：简单文本
		// 解码文本（处理十六进制字符串和 CID 字体）
		decodedText := decodeTextStringWithFont(text, toUnicodeMap)
		if decodedText != "" {
			// 打印前几个文本用于调试
			if len(decodedText) > 0 && len(decodedText) < 50 {
				fmt.Printf("[TEXT] Rendering at Tm=[%.0f, %.0f]: %q\n", tm.E, tm.F, decodedText)
			}
			layout.SetText(decodedText)
			ctx.CairoCtx.PangoCairoShowText(layout)

			// 计算文本宽度（用于更新文本矩阵，使用 rune 数量）
			runeCount := float64(len([]rune(decodedText)))
			textWidth := runeCount * fontSize * 0.5
			textWidth += textState.CharSpacing * runeCount

			// 计算单词间距
			spaceCount := 0
			for _, ch := range decodedText {
				if ch == ' ' {
					spaceCount++
				}
			}
			textWidth += textState.WordSpacing * float64(spaceCount)

			// 应用水平缩放到位移
			textDisplacement = textWidth * horizontalScale
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
