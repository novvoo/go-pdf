package gopdf

import (
	"fmt"
	"math"
	"strings"
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
	Name                string
	BaseFont            string
	Subtype             string
	Encoding            string
	BaseEncoding        string
	CodeToGlyphName     map[byte]string
	ToUnicodeMap        *CIDToUnicodeMap // CID 字体的 Unicode 映射
	CIDSystemInfo       string           // CID 字体的系统信息 (Registry-Ordering)
	EmbeddedFontData    []byte           // 嵌入的字体数据 (TTF/CFF)
	IsIdentity          bool             // 是否使用 Identity 映射 (CID = Unicode)
	Widths              *FontWidths      // 字形宽度信息
	DefaultWidth        float64          // 默认字形宽度（用于 CID 字体）
	MissingWidth        float64          // 缺失字形的宽度
	CIDToGIDMap         []uint16
	CIDToGIDMapIdentity bool
}

func (f *Font) CIDToGID(cid uint16) uint16 {
	if f == nil {
		return cid
	}
	if f.CIDToGIDMapIdentity {
		return cid
	}
	if len(f.CIDToGIDMap) > 0 && int(cid) < len(f.CIDToGIDMap) {
		gid := f.CIDToGIDMap[cid]
		if gid != 0 {
			return gid
		}
	}
	return cid
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
		// 🔥 修复：如果没有宽度信息，优先使用字体的默认宽度
		if f.DefaultWidth > 0 {
			return f.DefaultWidth
		}
		if f.MissingWidth > 0 {
			return f.MissingWidth
		}
		// 使用 1 em (1000 单位) 作为最后的回退
		return 1000.0
	}

	// CID 字体
	// 注意：Subtype可能是"Type0"或"/Type0"
	if f.Subtype == "/Type0" || f.Subtype == "Type0" || len(f.Widths.CIDWidths) > 0 || len(f.Widths.CIDRanges) > 0 {
		// 首先查找直接映射
		if width, ok := f.Widths.CIDWidths[cid]; ok {
			// 🔥 修复：如果宽度为0，使用默认宽度
			if width == 0 {
				if f.DefaultWidth > 0 {
					return f.DefaultWidth
				}
				if f.MissingWidth > 0 {
					return f.MissingWidth
				}
				return 1000.0
			}
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
						width := r.Widths[offset]
						// 🔥 修复：如果宽度为0，使用默认宽度
						if width == 0 {
							if f.DefaultWidth > 0 {
								return f.DefaultWidth
							}
							if f.MissingWidth > 0 {
								return f.MissingWidth
							}
							return 1000.0
						}
						return width
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
		return 1000.0
	}

	// Type1/TrueType 字体
	if len(f.Widths.Widths) > 0 {
		charCode := int(cid)
		if charCode >= f.Widths.FirstChar && charCode <= f.Widths.LastChar {
			offset := charCode - f.Widths.FirstChar
			if offset < len(f.Widths.Widths) {
				width := f.Widths.Widths[offset]
				// 🔥 修复：如果宽度为0，使用默认宽度
				if width == 0 {
					if f.MissingWidth > 0 {
						return f.MissingWidth
					}
					return 1000.0
				}
				return width
			}
		}
	}

	// 使用默认宽度
	if f.MissingWidth > 0 {
		return f.MissingWidth
	}
	return 1000.0
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
		op.Matrix.XX, op.Matrix.YX, op.Matrix.XY, op.Matrix.YY, op.Matrix.X0, op.Matrix.Y0)

	return nil
}

// OpMoveTextPosition Td - 移动文本位置
type OpMoveTextPosition struct {
	Tx, Ty float64
}

func (op *OpMoveTextPosition) Name() string { return "Td" }

func (op *OpMoveTextPosition) Execute(ctx *RenderContext) error {
	// 根据PDF规范：Tlm = Tlm × [1 0 0 1 tx ty]，然后 Tm = Tlm
	translation := NewTranslationMatrix(op.Tx, op.Ty)
	ctx.TextState.TextLineMatrix = ctx.TextState.TextLineMatrix.Multiply(translation)
	ctx.TextState.TextMatrix = ctx.TextState.TextLineMatrix.Clone()

	debugPrintf("[Td] Move text position: tx=%.2f, ty=%.2f -> New Tm: [%.2f %.2f %.2f %.2f %.2f %.2f]\n",
		op.Tx, op.Ty,
		ctx.TextState.TextMatrix.XX, ctx.TextState.TextMatrix.YX,
		ctx.TextState.TextMatrix.XY, ctx.TextState.TextMatrix.YY,
		ctx.TextState.TextMatrix.X0, ctx.TextState.TextMatrix.Y0)

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
	// 🔥 关键修复：T* 必须重置 X 坐标到行首
	// 根据 PDF 规范：Tlm = Tlm × [1 0 0 1 0 -Tl]，然后 Tm = Tlm
	// 这意味着只移动 Y，X 保持为 TextLineMatrix 的初始 X

	// 保存当前行的起始 X 坐标
	lineStartX := ctx.TextState.TextLineMatrix.X0

	// 移动 Y 坐标
	ctx.TextState.TextLineMatrix = ctx.TextState.TextLineMatrix.Translate(0, -ctx.TextState.Leading)

	// 🔥 修复：确保 X 坐标重置到行首
	// 如果 TextLineMatrix 的 X 被之前的文本操作修改了，需要重置
	ctx.TextState.TextLineMatrix.X0 = lineStartX

	// 重置 TextMatrix 为 TextLineMatrix
	ctx.TextState.TextMatrix = ctx.TextState.TextLineMatrix.Clone()

	debugPrintf("[T*] Next line: Leading=%.2f -> New Tm: [%.2f %.2f %.2f %.2f %.2f %.2f]\n",
		ctx.TextState.Leading,
		ctx.TextState.TextMatrix.XX, ctx.TextState.TextMatrix.YX,
		ctx.TextState.TextMatrix.XY, ctx.TextState.TextMatrix.YY,
		ctx.TextState.TextMatrix.X0, ctx.TextState.TextMatrix.Y0)

	return nil
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
	Array []any // string 或 float64
}

func (op *OpShowTextArray) Name() string { return "TJ" }

func (op *OpShowTextArray) Execute(ctx *RenderContext) error {
	return renderText(ctx, "", op.Array)
}

// GlyphWithPosition 带位置的字形
type GlyphWithPosition struct {
	CID        uint16
	Rune       rune
	X, Y       float64
	FontFamily string  // 字体族名
	FontSize   float64 // 字体大小
}

// renderText 渲染文本到 Gopdf
func renderText(ctx *RenderContext, text string, array []any) error {
	state := ctx.GetCurrentState()
	textState := ctx.TextState

	// 调试输出：文本状态
	debugPrintf("\n[TEXT_STATE] CharSpacing=%.4f WordSpacing=%.4f HScale=%.2f%% FontSize=%.2f\n",
		textState.CharSpacing, textState.WordSpacing, textState.HorizontalScaling, textState.FontSize)

	// 保存 Gopdf 状态
	ctx.GopdfCtx.Save()
	defer ctx.GopdfCtx.Restore()

	textMatrix := textState.TextMatrix.Clone()
	if textState.Rise != 0 {
		textMatrix = textMatrix.Multiply(NewTranslationMatrix(0, textState.Rise))
	}
	ctx.GopdfCtx.Transform(textMatrix)

	// 设置字体
	// 🔥 关键：字体大小直接使用 FontSize，不从文本矩阵提取
	// 因为文本矩阵的缩放已经在计算绝对坐标时应用了
	fontSize := textState.FontSize

	// 如果字体大小为0，使用默认值
	if fontSize < 1.0 {
		fontSize = 12.0
	}

	fontFamily := "sans-serif"
	registeredPDFFont := false
	if textState.Font != nil {
		if len(textState.Font.EmbeddedFontData) > 0 {
			key := "pdf:" + textState.Font.Name
			if err := RegisterFontData(key, textState.Font.EmbeddedFontData); err == nil {
				fontFamily = key
				registeredPDFFont = true
			}
		}
		if !registeredPDFFont && textState.Font.BaseFont != "" {
			if isTeXMathFont(textState.Font.BaseFont) {
				fontFamily = "math"
			} else {
				fontFamily = mapPDFFont(textState.Font.BaseFont)
			}
		}
	}

	// 获取当前字体的 ToUnicode 映射
	var toUnicodeMap *CIDToUnicodeMap
	if textState.Font != nil {
		toUnicodeMap = textState.Font.ToUnicodeMap
	}

	useGlyphIndices := false
	if textState.Font != nil &&
		(textState.Font.Subtype == "/Type0" || textState.Font.Subtype == "Type0") &&
		textState.Font.IsIdentity &&
		registeredPDFFont {
		useGlyphIndices = true
	}

	sampleText := text
	if array != nil {
		for _, it := range array {
			if s, ok := it.(string); ok {
				sampleText = s
				break
			}
		}
	}
	if !registeredPDFFont && (fontFamily == "sans-serif" || fontFamily == "sans") {
		decodedSample, _ := decodeTextStringWithCIDs(sampleText, toUnicodeMap, textState.Font)
		if shouldUseCJKFallback(textState.Font, decodedSample) {
			fontFamily = "sans-cjk"
		}
	}

	// 🔥 使用 PangoPdf 进行文本渲染
	// PangoPdf 会处理字体选择和文本布局
	debugPrintf("[TEXT_RENDER] Using PangoPdf text rendering: font=%s, size=%.2f\n", fontFamily, fontSize)

	// 🔥 关键：不应用水平缩放到Gopdf上下文
	// 水平缩放已经在 GlyphAdvance 计算中处理了
	// 这样避免双重缩放

	// 设置颜色（根据渲染模式）
	switch textState.RenderMode {
	case 0: // 填充
		if state.FillColor != nil {
			debugPrintf("[TEXT_STATE] Using FillColor: RGB(%.3f, %.3f, %.3f, %.3f)\n",
				state.FillColor.R, state.FillColor.G, state.FillColor.B, state.FillColor.A)
			ctx.GopdfCtx.SetSourceRGBA(
				state.FillColor.R,
				state.FillColor.G,
				state.FillColor.B,
				state.FillColor.A,
			)
		} else {
			// 默认使用黑色
			debugPrintf("[TEXT_STATE] Using default black color\n")
			ctx.GopdfCtx.SetSourceRGBA(0, 0, 0, 1)
		}
	case 1: // 描边
		if state.StrokeColor != nil {
			ctx.GopdfCtx.SetSourceRGBA(
				state.StrokeColor.R,
				state.StrokeColor.G,
				state.StrokeColor.B,
				state.StrokeColor.A,
			)
		}
	case 2: // 填充+描边
		if state.FillColor != nil {
			ctx.GopdfCtx.SetSourceRGBA(
				state.FillColor.R,
				state.FillColor.G,
				state.FillColor.B,
				state.FillColor.A,
			)
		}
	case 3: // 不可见
		return nil
	}

	// 🔥 新策略：使用 Pango 自动布局
	// 只记录文本的起始位置，让 Pango 处理字符间距和宽度
	currentX := 0.0 // 文本空间中的相对 X 位置

	layout := ctx.GopdfCtx.PangoPdfCreateLayout().(*PangoPdfLayout)
	fontDesc := NewPangoFontDescription()
	fontDesc.SetFamily(fontFamily)
	fontDesc.SetSize(fontSize)
	layout.SetFontDescription(fontDesc)

	fontFace := NewPangoPdfFont(fontFamily, FontSlantNormal, FontWeightNormal)
	defer fontFace.Destroy()
	fontMatrix := NewMatrix()
	fontMatrix.InitScale(fontSize, fontSize)
	ctm := NewMatrix()
	ctm.InitIdentity()
	scaledFont := NewPangoPdfScaledFont(fontFace, fontMatrix, ctm, nil)
	defer scaledFont.Destroy()
	scaledFont.flipY = shouldFlipGlyphY(ctx.GopdfCtx)

	pangoAdvanceForLayout := func(runText string) float64 {
		if runText == "" {
			return 0
		}

		ext := scaledFont.TextExtents(runText)
		adv := ext.XAdvance

		if textState.CharSpacing != 0 || textState.WordSpacing != 0 {
			for _, r := range []rune(runText) {
				if isCJKCharacterRune(r) {
					if textState.CharSpacing != 0 {
						adv += textState.CharSpacing * 0.5
					}
				} else {
					adv += textState.CharSpacing
				}
				if r == ' ' {
					adv += textState.WordSpacing
				}
			}
		}

		adv *= textState.HorizontalScaling / 100.0
		return adv
	}

	pdfAdvanceForLayout := func(runText string, cids []uint16) (float64, bool) {
		if textState.Font == nil || len(cids) == 0 || runText == "" {
			return 0, false
		}
		hasWidths := textState.Font.Widths != nil &&
			(len(textState.Font.Widths.Widths) > 0 || len(textState.Font.Widths.CIDWidths) > 0 || len(textState.Font.Widths.CIDRanges) > 0)
		if !hasWidths {
			if textState.Font.DefaultWidth <= 0 {
				return 0, false
			}
			if math.Abs(textState.Font.DefaultWidth-1000.0) < 1e-3 {
				return 0, false
			}
		}

		adv := 0.0
		runes := []rune(runText)
		for i, cid := range cids {
			isSpace := i < len(runes) && runes[i] == ' '
			adv += textState.GlyphAdvance(cid, isSpace)
		}
		return adv, true
	}

	computeRunScaleAndAdvance := func(runText string, cids []uint16) (scaleX float64, adv float64) {
		hScale := textState.HorizontalScaling / 100.0
		if hScale <= 0 {
			hScale = 1
		}

		pangoAdvUnscaled := scaledFont.TextExtents(runText).XAdvance
		if pangoAdvUnscaled <= 0 {
			return hScale, 0
		}

		asciiOnly := true
		for _, r := range runText {
			if r > 0x7F {
				asciiOnly = false
				break
			}
		}

		pangoAdvScaled := pangoAdvUnscaled * hScale
		if pdfAdv, ok := pdfAdvanceForLayout(runText, cids); ok && pdfAdv > 0 {
			ratio := pdfAdv / pangoAdvScaled
			minRatio, maxRatio := 0.6, 1.6
			if !asciiOnly {
				minRatio, maxRatio = 0.5, 2.0
			}
			if ratio >= minRatio && ratio <= maxRatio {
				return hScale * ratio, pdfAdv
			}
		}

		return hScale, pangoAdvanceForLayout(runText)
	}

	delimiterAdjust := func(glyphName string) (dyFactor float64, scaleY float64, ok bool) {
		glyphName = strings.TrimPrefix(glyphName, "/")
		switch glyphName {
		case "parenleftbig", "parenrightbig", "bracketleftbig", "bracketrightbig":
			return -0.95, 1.70, true
		case "parenleftBig", "parenrightBig", "bracketleftBig", "bracketrightBig":
			return -1.10, 2.00, true
		case "parenleftBigg", "parenrightBigg", "bracketleftBigg", "bracketrightBigg":
			return -1.25, 2.35, true
		case "parenleftBiggg", "parenrightBiggg", "bracketleftBiggg", "bracketrightBiggg":
			return -1.40, 2.70, true
		default:
			return 0, 1.0, false
		}
	}

	renderRun := func(runText string, cids []uint16, scaleX float64) {
		if runText == "" {
			return
		}

		dy := 0.0
		scaleY := 1.0
		if textState.Font != nil && len(cids) == 1 && len(textState.Font.CodeToGlyphName) > 0 {
			if name, ok := textState.Font.CodeToGlyphName[byte(cids[0]&0xFF)]; ok {
				if dyFactor, sy, ok := delimiterAdjust(name); ok {
					dy = dyFactor * fontSize
					scaleY = sy
				}
			}
		}

		if scaleX != 1.0 || dy != 0.0 || scaleY != 1.0 {
			ctx.GopdfCtx.Save()
			ctx.GopdfCtx.Translate(currentX, dy)
			ctx.GopdfCtx.Scale(scaleX, scaleY)
			ctx.GopdfCtx.MoveTo(0, 0)
			layout.SetText(runText)
			ctx.GopdfCtx.PangoPdfShowText(layout)
			ctx.GopdfCtx.Restore()
			return
		}

		ctx.GopdfCtx.MoveTo(currentX, 0)
		layout.SetText(runText)
		ctx.GopdfCtx.PangoPdfShowText(layout)
	}

	renderGlyphs := func(cids []uint16) {
		glyphs := make([]Glyph, 0, len(cids))
		baseX := currentX
		x := 0.0
		for _, cid := range cids {
			gid := textState.Font.CIDToGID(cid)
			glyphs = append(glyphs, Glyph{Index: uint64(gid), X: baseX + x, Y: 0})
			x += textState.GlyphAdvance(cid, cid == 32)
		}
		renderGlyphRun(ctx.GopdfCtx, scaledFont, glyphs, textState.RenderMode)
		currentX = baseX + x
	}

	// 渲染文本
	if array != nil {
		// TJ 操作符：处理文本数组
		debugPrintf("[TJ_ARRAY] Processing %d items\n", len(array))

		for idx, item := range array {
			switch v := item.(type) {
			case string:
				// 解码文本并获取 CID 数组
				decodedText, cids := decodeTextStringWithCIDs(v, toUnicodeMap, textState.Font)
				if decodedText == "" && !useGlyphIndices {
					debugPrintf("[TJ_ARRAY][%d] Empty string after decode\n", idx)
					continue
				}

				debugPrintf("[TJ_ARRAY][%d] Text=%q (len=%d runes, %d CIDs) at x=%.2f\n",
					idx, decodedText, len([]rune(decodedText)), len(cids), currentX)

				if useGlyphIndices {
					renderGlyphs(cids)
					debugPrintf("[TJ_ARRAY][%d] Rendered glyph run\n", idx)
				} else {
					scaleX, adv := computeRunScaleAndAdvance(decodedText, cids)
					renderRun(decodedText, cids, scaleX)

					currentX += adv
					debugPrintf("[TJ_ARRAY][%d] Rendered run, adv=%.2f\n", idx, adv)
				}

			case float64:
				// PDF规范：负值表示向右移动，正值表示向左移动
				// 调整值以千分之一em为单位
				kerningAdjustment := -v * fontSize / 1000.0 * textState.HorizontalScaling / 100.0
				if kerningAdjustment < 0 && !registeredPDFFont {
					maxBack := fontSize * 0.5
					if kerningAdjustment < -maxBack {
						kerningAdjustment = -maxBack
					}
				}
				debugPrintf("[TJ_ARRAY][%d] Kerning=%.0f adj=%.2f (x: %.2f -> %.2f)\n",
					idx, v, kerningAdjustment, currentX, currentX+kerningAdjustment)
				currentX += kerningAdjustment

			case int:
				kerningAdjustment := -float64(v) * fontSize / 1000.0 * textState.HorizontalScaling / 100.0
				if kerningAdjustment < 0 && !registeredPDFFont {
					maxBack := fontSize * 0.5
					if kerningAdjustment < -maxBack {
						kerningAdjustment = -maxBack
					}
				}
				debugPrintf("[TJ_ARRAY][%d] Kerning=%d adj=%.2f (x: %.2f -> %.2f)\n",
					idx, v, kerningAdjustment, currentX, currentX+kerningAdjustment)
				currentX += kerningAdjustment
			}
		}
	} else {
		// Tj 操作符：简单文本
		decodedText, cids := decodeTextStringWithCIDs(text, toUnicodeMap, textState.Font)
		if (decodedText != "" && !useGlyphIndices) || (useGlyphIndices && len(cids) > 0) {
			debugPrintf("[Tj] Text=%q (len=%d runes, %d CIDs) at Tm=[%.2f, %.2f]\n",
				decodedText, len([]rune(decodedText)), len(cids), textState.TextMatrix.X0, textState.TextMatrix.Y0)

			if useGlyphIndices {
				renderGlyphs(cids)
				debugPrintf("[Tj] Rendered glyph run\n")
			} else {
				scaleX, adv := computeRunScaleAndAdvance(decodedText, cids)
				renderRun(decodedText, cids, scaleX)

				currentX += adv
				debugPrintf("[Tj] Rendered run, adv=%.2f\n", adv)
			}
		}
	}

	// 更新文本矩阵：使用PDF的字形宽度
	// 这对于在同一个BT...ET块中的多个Tj操作是必要的
	if currentX != 0 {
		translation := NewTranslationMatrix(currentX, 0)
		textState.TextMatrix = textState.TextMatrix.Multiply(translation)
		debugPrintf("[TEXT_MATRIX] Updated after text: PDF_width=%.2f, new X0=%.2f\n",
			currentX, textState.TextMatrix.X0)
	}

	return nil

	// 注意：由于go-pdf库的限制，无法完全实现高级的kerning功能
	// 当前实现已尽可能应用了TJ操作符中的数字偏移到文本位置
}

func renderGlyphRun(ctx Context, sf *PangoPdfScaledFont, glyphs []Glyph, renderMode int) {
	for _, glyph := range glyphs {
		ctx.Save()

		glyphPath, err := sf.GlyphPath(glyph.Index)
		if err != nil || glyphPath == nil || len(glyphPath.Data) == 0 {
			ctx.Restore()
			continue
		}

		ctx.NewPath()
		for _, pathData := range glyphPath.Data {
			switch pathData.Type {
			case PathMoveTo:
				if len(pathData.Points) > 0 {
					ctx.MoveTo(pathData.Points[0].X+glyph.X, pathData.Points[0].Y+glyph.Y)
				}
			case PathLineTo:
				if len(pathData.Points) > 0 {
					ctx.LineTo(pathData.Points[0].X+glyph.X, pathData.Points[0].Y+glyph.Y)
				}
			case PathCurveTo:
				if len(pathData.Points) >= 3 {
					ctx.CurveTo(
						pathData.Points[0].X+glyph.X, pathData.Points[0].Y+glyph.Y,
						pathData.Points[1].X+glyph.X, pathData.Points[1].Y+glyph.Y,
						pathData.Points[2].X+glyph.X, pathData.Points[2].Y+glyph.Y,
					)
				}
			case PathClosePath:
				ctx.ClosePath()
			}
		}

		switch renderMode {
		case 1:
			ctx.Stroke()
		case 2:
			ctx.FillPreserve()
			ctx.Stroke()
		default:
			ctx.Fill()
		}

		ctx.Restore()
	}
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
					if isValidUnicodeRune(uni) {
						decoded.WriteRune(uni)
					} else {
						decoded.WriteRune('�')
					}
				} else {
					allMapped = false
					break
				}
			}

			if allMapped {
				return ensureValidUTF8(decoded.String()), cids
			}
			decoded.Reset()
		}

		if font != nil && len(font.CodeToGlyphName) > 0 {
			for _, cid := range cids {
				name, ok := font.CodeToGlyphName[byte(cid&0xFF)]
				if !ok {
					decoded.WriteRune('�')
					continue
				}
				if r, ok := glyphNameToRuneForFont(name, font); ok {
					decoded.WriteRune(r)
				} else {
					decoded.WriteRune('�')
				}
			}
			return ensureValidUTF8(decoded.String()), cids
		}

		// 如果ToUnicode映射失败或不存在，且是Identity映射，CID直接等于Unicode码点
		if isIdentity {
			for _, cid := range cids {
				r := rune(cid)
				// 验证Unicode码点有效性
				if isValidUnicodeRune(r) {
					decoded.WriteRune(r)
				} else {
					debugPrintf("⚠️ Invalid Unicode codepoint: U+%04X\n", cid)
					decoded.WriteRune('�') // 使用替换字符
				}
			}
			return decoded.String(), cids
		}

		// 否则尝试标准解码
		decodedStr := decodeTextString(text)
		return ensureValidUTF8(decodedStr), cids
	}

	// 普通字符串 - 处理反斜杠转义并转换为 CID 数组（字节码）
	rawBytes := decodePDFLiteralStringBytes(text)
	var cids []uint16
	for _, b := range rawBytes {
		cids = append(cids, uint16(b))
	}

	if toUnicodeMap != nil {
		var decoded strings.Builder
		allMapped := true
		for _, cid := range cids {
			if uni, ok := toUnicodeMap.MapCIDToUnicode(cid); ok {
				if isValidUnicodeRune(uni) {
					decoded.WriteRune(uni)
				} else {
					decoded.WriteRune('�')
				}
			} else {
				allMapped = false
				break
			}
		}
		if allMapped {
			return ensureValidUTF8(decoded.String()), cids
		}
	}

	if font != nil && len(font.CodeToGlyphName) > 0 {
		var decoded strings.Builder
		for _, cid := range cids {
			name, ok := font.CodeToGlyphName[byte(cid&0xFF)]
			if !ok {
				decoded.WriteRune(rune(cid))
				continue
			}
			if r, ok := glyphNameToRuneForFont(name, font); ok {
				decoded.WriteRune(r)
			} else {
				decoded.WriteRune(rune(cid))
			}
		}
		return ensureValidUTF8(decoded.String()), cids
	}

	if font != nil {
		base := strings.ToUpper(stripSubsetPrefix(font.BaseFont))
		if strings.HasPrefix(base, "MSBM") {
			var decoded strings.Builder
			for _, cid := range cids {
				if r, ok := msbmRuneFromCID(cid); ok {
					decoded.WriteRune(r)
				} else {
					decoded.WriteRune(rune(cid))
				}
			}
			return ensureValidUTF8(decoded.String()), cids
		}
	}

	if font != nil && font.IsIdentity {
		var decoded strings.Builder
		for _, cid := range cids {
			r := rune(cid)
			if isValidUnicodeRune(r) {
				decoded.WriteRune(r)
			} else {
				decoded.WriteRune('�')
			}
		}
		return ensureValidUTF8(decoded.String()), cids
	}

	return latin1StringFromBytes(rawBytes), cids
}

// isValidUnicodeRune 验证Unicode码点是否有效
func isValidUnicodeRune(r rune) bool {
	// 检查是否是有效的UTF-8 rune
	if r < 0 || r > 0x10FFFF {
		return false
	}
	// 排除代理对范围(U+D800到U+DFFF)
	if r >= 0xD800 && r <= 0xDFFF {
		return false
	}
	return true
}

// GlyphAdvance 计算单个字形的推进距离（核心方法）
func (ts *TextState) GlyphAdvance(cid uint16, isSpace bool) float64 {
	if ts.Font == nil {
		return 0.0
	}

	// 1. 获取字形宽度（千分之一 em）
	glyphWidth := ts.Font.GetWidth(cid)

	// 2. 检测是否是CJK字符
	isCJK := isCJKCharacterFromCID(cid)

	// 3. 处理零宽度情况
	if glyphWidth == 0 {
		if isCJK {
			// CJK字符通常是全角(1 em)
			glyphWidth = 1000.0
			debugPrintf("[GlyphAdvance] CID %d is CJK with zero width, using 1em\n", cid)
		} else if ts.Font.DefaultWidth > 0 {
			glyphWidth = ts.Font.DefaultWidth
			debugPrintf("[GlyphAdvance] CID %d has zero width, using DefaultWidth: %.0f\n", cid, glyphWidth)
		} else if ts.Font.MissingWidth > 0 {
			glyphWidth = ts.Font.MissingWidth
			debugPrintf("[GlyphAdvance] CID %d has zero width, using MissingWidth: %.0f\n", cid, glyphWidth)
		} else {
			// 非CJK字符使用较小的默认值
			glyphWidth = 500.0 // 0.5 em
			debugPrintf("[GlyphAdvance] CID %d has zero width, using 0.5em\n", cid)
		}
	}

	// 4. 转换为用户空间单位
	adv := glyphWidth * ts.FontSize / 1000.0

	// 5. 添加字符间距(CJK字符可能需要不同的间距)
	if isCJK {
		// CJK字符通常不需要额外的字符间距
		// 但如果明确设置了,仍然应用(减半)
		if ts.CharSpacing != 0 {
			adv += ts.CharSpacing * 0.5
		}
	} else {
		adv += ts.CharSpacing
	}

	// 6. 如果是空格，添加单词间距
	if isSpace {
		adv += ts.WordSpacing
	}

	// 7. 应用水平缩放
	adv *= ts.HorizontalScaling / 100.0

	return adv
}

// isCJKCharacterFromCID 从CID判断是否是CJK字符
func isCJKCharacterFromCID(cid uint16) bool {
	r := rune(cid)
	return isCJKCharacterRune(r)
}

// isCJKCharacterRune 判断rune是否是CJK字符
func isCJKCharacterRune(r rune) bool {
	// CJK统一表意文字
	if r >= 0x4E00 && r <= 0x9FFF {
		return true
	}
	// CJK扩展A
	if r >= 0x3400 && r <= 0x4DBF {
		return true
	}
	// CJK扩展B-F
	if r >= 0x20000 && r <= 0x2EBEF {
		return true
	}
	// CJK兼容表意文字
	if r >= 0xF900 && r <= 0xFAFF {
		return true
	}
	// 日文假名
	if r >= 0x3040 && r <= 0x30FF {
		return true
	}
	// 韩文音节
	if r >= 0xAC00 && r <= 0xD7AF {
		return true
	}
	return false
}

func shouldUseCJKFallback(font *Font, decodedText string) bool {
	for _, r := range decodedText {
		if isCJKCharacterRune(r) {
			return true
		}
	}
	if font == nil {
		return false
	}
	if strings.Contains(font.CIDSystemInfo, "GB1") ||
		strings.Contains(font.CIDSystemInfo, "CNS1") ||
		strings.Contains(font.CIDSystemInfo, "Japan1") ||
		strings.Contains(font.CIDSystemInfo, "Korea1") {
		return true
	}
	bf := strings.ToLower(font.BaseFont)
	if strings.Contains(bf, "yahei") ||
		strings.Contains(bf, "msyh") ||
		strings.Contains(bf, "simsun") ||
		strings.Contains(bf, "song") ||
		strings.Contains(bf, "hei") ||
		strings.Contains(bf, "pingfang") {
		return true
	}
	return false
}

// CalculateTextWidthFromCIDs 使用字形宽度计算文本宽度（从 CID 数组）
func CalculateTextWidthFromCIDs(cids []uint16, textState *TextState, decodedText string) float64 {
	if textState.Font == nil || len(cids) == 0 {
		// 关键修复：当没有字体信息时，返回0而不是过估
		// 这样可以避免推动后续文本向右偏移
		// Pango 会自动处理文本布局和宽度
		debugPrintf("[WIDTH] No font info, returning 0 (Pango will handle layout)\n")
		return 0.0
	}

	totalWidth := 0.0
	runes := []rune(decodedText)

	// 使用字形宽度计算
	for i, cid := range cids {
		// 检查是否是空格
		isSpace := i < len(runes) && runes[i] == ' '

		// 使用统一的 advance 计算
		adv := textState.GlyphAdvance(cid, isSpace)
		totalWidth += adv
	}

	debugPrintf("[WIDTH] Calculated width=%.2f for %d CIDs\n", totalWidth, len(cids))
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
