package gopdf

import (
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
	Name     string
	BaseFont string
	Subtype  string
	Encoding string
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
	textState.TextMatrix.ApplyToCairoContext(ctx.CairoCtx)

	// 由于 PDF 坐标系已经在页面级别翻转，文本需要再次翻转回来
	// 以保持文本正向显示
	ctx.CairoCtx.Scale(1, -1)

	// 应用文本上升
	if textState.Rise != 0 {
		ctx.CairoCtx.Translate(0, -textState.Rise) // 注意 Y 轴已翻转
	}

	// 设置字体
	fontSize := textState.FontSize
	fontFamily := "sans-serif"
	if textState.Font != nil && textState.Font.BaseFont != "" {
		fontFamily = mapPDFFont(textState.Font.BaseFont)
	}

	// 使用 PangoCairo 渲染文本
	layout := ctx.CairoCtx.PangoCairoCreateLayout().(*cairo.PangoCairoLayout)
	fontDesc := cairo.NewPangoFontDescription()
	fontDesc.SetFamily(fontFamily)
	fontDesc.SetSize(fontSize)
	layout.SetFontDescription(fontDesc)

	// 应用水平缩放
	if textState.HorizontalScaling != 100 {
		scale := textState.HorizontalScaling / 100.0
		ctx.CairoCtx.Scale(scale, 1.0)
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

	// 渲染文本
	if array != nil {
		// TJ 操作符：处理文本数组
		x := 0.0
		for _, item := range array {
			switch v := item.(type) {
			case string:
				layout.SetText(v)
				ctx.CairoCtx.MoveTo(x, 0)
				ctx.CairoCtx.PangoCairoShowText(layout)

				// 计算文本宽度（估算）
				textWidth := float64(len(v)) * fontSize * 0.5
				x += textWidth

				// 应用字符间距
				x += textState.CharSpacing * float64(len(v))

			case float64:
				// 负值表示向右移动，正值表示向左移动
				x -= v * fontSize / 1000.0

			case int:
				x -= float64(v) * fontSize / 1000.0
			}
		}
	} else {
		// Tj 操作符：简单文本
		layout.SetText(text)
		ctx.CairoCtx.PangoCairoShowText(layout)

		// 更新文本矩阵位置（估算文本宽度）
		textWidth := float64(len(text)) * fontSize * 0.5
		textWidth += textState.CharSpacing * float64(len(text))

		// 计算单词间距
		spaceCount := 0
		for _, ch := range text {
			if ch == ' ' {
				spaceCount++
			}
		}
		textWidth += textState.WordSpacing * float64(spaceCount)

		// 更新文本矩阵
		textState.TextMatrix = textState.TextMatrix.Translate(textWidth, 0)
	}

	return nil
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
