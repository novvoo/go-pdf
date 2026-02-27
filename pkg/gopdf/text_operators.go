package gopdf

import (
	"encoding/hex"
	"fmt"
	"math"
	"strings"
	"unicode"

	"github.com/go-text/typesetting/di"
	otapi "github.com/go-text/typesetting/opentype/api"
	cfffont "github.com/go-text/typesetting/opentype/api/font/cff"
	"github.com/go-text/typesetting/opentype/tables"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/font"
	"golang.org/x/image/font/sfnt"
	"golang.org/x/image/math/fixed"
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
	CodeToGID           map[byte]uint16
	CFF                 *cfffont.CFF
	FontMatrix          [6]float64
	HasFontMatrix       bool
	ToUnicodeMap        *CIDToUnicodeMap // CID 字体的 Unicode 映射
	CIDSystemInfo       string           // CID 字体的系统信息 (Registry-Ordering)
	EmbeddedFontData    []byte           // 嵌入的字体数据 (TTF/CFF)
	IsIdentity          bool             // 是否使用 Identity 映射 (CID = Unicode)
	Widths              *FontWidths      // 字形宽度信息
	DefaultWidth        float64          // 默认字形宽度（用于 CID 字体）
	MissingWidth        float64          // 缺失字形的宽度
	CIDToGIDMap         []uint16
	CIDToGIDMapIdentity bool
	gidToUnicode        map[uint16]rune
	gidToUnicodeBuilt   bool
	embeddedSFNT        *sfnt.Font
	embeddedSFNTBuilt   bool
	cffRefCenterUnits   float64
	cffRefReady         bool
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

func (f *Font) gidToUnicodeFromEmbeddedFont(gid uint16) (rune, bool) {
	if f == nil {
		return 0, false
	}
	if !f.gidToUnicodeBuilt {
		f.gidToUnicodeBuilt = true
		if len(f.EmbeddedFontData) == 0 {
			return 0, false
		}
		parsed, err := sfnt.Parse(f.EmbeddedFontData)
		if err != nil {
			return 0, false
		}
		buf := &sfnt.Buffer{}
		m := make(map[uint16]rune, 2048)
		for r := rune(0); r <= 0xFFFF; r++ {
			gi, err := parsed.GlyphIndex(buf, r)
			if err != nil || gi == 0 {
				continue
			}
			g := uint16(gi)
			if _, exists := m[g]; !exists {
				m[g] = r
			}
		}
		if len(m) > 0 {
			f.gidToUnicode = m
		}
	}
	if f.gidToUnicode == nil {
		return 0, false
	}
	r, ok := f.gidToUnicode[gid]
	return r, ok
}

func (f *Font) ensureEmbeddedSFNT() {
	if f == nil || f.embeddedSFNTBuilt {
		return
	}
	f.embeddedSFNTBuilt = true
	if len(f.EmbeddedFontData) == 0 {
		return
	}
	parsed, err := sfnt.Parse(f.EmbeddedFontData)
	if err != nil {
		return
	}
	f.embeddedSFNT = parsed
}

func (f *Font) embeddedWidthAvailable() bool {
	if f == nil || len(f.EmbeddedFontData) == 0 {
		return false
	}
	f.ensureEmbeddedSFNT()
	return f.embeddedSFNT != nil
}

func (f *Font) widthFromEmbeddedSFNT(cid uint16) (float64, bool) {
	if f == nil || len(f.EmbeddedFontData) == 0 {
		return 0, false
	}
	f.ensureEmbeddedSFNT()
	if f.embeddedSFNT == nil {
		return 0, false
	}

	unitsPerEm := f.embeddedSFNT.UnitsPerEm()
	if unitsPerEm == 0 {
		return 0, false
	}

	gid := f.CIDToGID(cid)
	if gid == 0 {
		return 0, false
	}

	buf := &sfnt.Buffer{}
	ppem := fixed.I(int(unitsPerEm))
	adv, err := f.embeddedSFNT.GlyphAdvance(buf, sfnt.GlyphIndex(gid), ppem, font.HintingNone)
	if err != nil {
		return 0, false
	}

	advUnits := float64(adv) / 64.0
	return advUnits * 1000.0 / float64(unitsPerEm), true
}

func (f *Font) cffGlyphCenterYUnits(gid uint16) (centerY float64, height float64, ok bool) {
	if f == nil || f.CFF == nil {
		return 0, 0, false
	}
	segments, _, err := f.CFF.LoadGlyph(tables.GlyphID(gid))
	if err != nil || len(segments) == 0 {
		return 0, 0, false
	}

	minY := math.Inf(1)
	maxY := math.Inf(-1)
	for _, seg := range segments {
		for _, p := range seg.Args {
			y := float64(p.Y)
			if y < minY {
				minY = y
			}
			if y > maxY {
				maxY = y
			}
		}
	}
	if !math.IsInf(minY, 0) && !math.IsInf(maxY, 0) && maxY > minY {
		return (minY + maxY) * 0.5, maxY - minY, true
	}
	return 0, 0, false
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
		if w, ok := f.widthFromEmbeddedSFNT(cid); ok && w > 0 {
			return w
		}
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

		if w, ok := f.widthFromEmbeddedSFNT(cid); ok && w > 0 {
			return w
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
	if w, ok := f.widthFromEmbeddedSFNT(cid); ok && w > 0 {
		return w
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

	fontSize := textState.FontSize

	// 如果字体大小为0，使用默认值
	if fontSize < 1.0 {
		fontSize = 12.0
	}

	// 获取当前字体的 ToUnicode 映射
	var toUnicodeMap *CIDToUnicodeMap
	if textState.Font != nil {
		toUnicodeMap = textState.Font.ToUnicodeMap
	}

	psPreferText := false
	var psCtx *context
	if c, ok := ctx.GopdfCtx.(*context); ok && c.psSurfaceTarget() != nil {
		psPreferText = true
		psCtx = c
	}

	renderMatrix := textState.TextMatrix.Clone()
	if textState.Rise != 0 {
		renderMatrix = renderMatrix.Multiply(NewTranslationMatrix(0, textState.Rise))
	}

	candidateText := text
	if candidateText == "" && len(array) > 0 {
		strCount := 0
		for _, it := range array {
			if s, ok := it.(string); ok {
				strCount++
				if strCount == 1 {
					candidateText = s
				} else {
					candidateText = ""
					break
				}
			}
		}
	}

	candidateTokenForLabel := candidateText
	if candidateTokenForLabel == "" && len(array) > 0 {
		for _, it := range array {
			if s, ok := it.(string); ok {
				candidateTokenForLabel = s
				break
			}
		}
	}

	candidateDecoded := ""
	if psPreferText && candidateTokenForLabel != "" {
		if d, _, _ := decodeTextStringWithCIDs(candidateTokenForLabel, toUnicodeMap, textState.Font); d != "" {
			candidateDecoded = strings.TrimSpace(d)
		}
	}

	enablePSSymbolGapClamp := false
	if enablePSSymbolGapClamp && psPreferText && psCtx != nil && candidateText != "" {
		decoded, cids, _ := decodeTextStringWithCIDs(candidateText, toUnicodeMap, textState.Font)
		trimmed := strings.TrimSpace(decoded)
		runes := []rune(trimmed)
		if len(runes) > 0 && len(runes) <= 2 {
			symbolOnly := true
			for _, r := range runes {
				if unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsSpace(r) {
					symbolOnly = false
					break
				}
			}
			if symbolOnly && psCtx.psLastText.active {
				tolY := math.Max(2.0, math.Max(psCtx.psLastText.fontSize, fontSize)*0.5)
				if math.Abs(renderMatrix.Y0-psCtx.psLastText.yUser) <= tolY {
					symAdv := 0.0
					for i, cid := range cids {
						isSpace := i < len(runes) && runes[i] == ' '
						symAdv += textState.GlyphAdvance(cid, isSpace)
					}
					if symAdv > 0 && symAdv <= fontSize*1.6 {
						gap := renderMatrix.X0 - psCtx.psLastText.xEndUser
						spaceAdv := textState.GlyphAdvance(32, true)
						if spaceAdv <= 0 {
							spaceAdv = fontSize * 0.30
						}
						desired := spaceAdv * 0.5
						if desired < fontSize*0.12 {
							desired = fontSize * 0.12
						} else if desired > fontSize*0.35 {
							desired = fontSize * 0.35
						}
						if gap > desired*1.5 {
							renderMatrix.X0 = psCtx.psLastText.xEndUser + desired
						}
					}
				}
			}
		}
	}

	enablePSLabelGapClamp := true
	if enablePSLabelGapClamp && psPreferText && psCtx != nil && candidateDecoded != "" && psCtx.psLastText.active {
		prev := strings.TrimSpace(psCtx.psLastText.text)
		hasLetter := false
		hasPunct := false
		for _, r := range prev {
			if unicode.IsLetter(r) {
				hasLetter = true
				break
			}
			if unicode.IsPunct(r) || unicode.IsSymbol(r) {
				hasPunct = true
			}
		}

		isShortLabel := !hasLetter && hasPunct && len([]rune(prev)) <= 6
		if isShortLabel {
			tolY := math.Max(2.0, math.Max(psCtx.psLastText.fontSize, fontSize)*0.5)
			if math.Abs(renderMatrix.Y0-psCtx.psLastText.yUser) <= tolY {
				runes := []rune(candidateDecoded)
				startsWithWord := false
				if len(runes) > 0 {
					r0 := runes[0]
					startsWithWord = unicode.IsLetter(r0) || unicode.IsNumber(r0)
				}
				if startsWithWord {
					gap := renderMatrix.X0 - psCtx.psLastText.xEndUser
					spaceAdv := textState.GlyphAdvance(32, true)
					if spaceAdv <= 0 {
						spaceAdv = fontSize * 0.30
					}
					desired := spaceAdv
					minDesired := fontSize * 0.12
					maxDesired := fontSize * 0.60
					if desired < minDesired {
						desired = minDesired
					} else if desired > maxDesired {
						desired = maxDesired
					}
					if gap > desired*2.0 {
						renderMatrix.X0 = psCtx.psLastText.xEndUser + desired
					}
				}
			}
		}
	}

	ctx.GopdfCtx.Transform(renderMatrix)

	// 设置字体
	// 🔥 关键：字体大小直接使用 FontSize，不从文本矩阵提取
	// 因为文本矩阵的缩放已经在计算绝对坐标时应用了
	fontFamily := "sans-serif"
	registeredPDFFont := false
	embeddedPDFFontKey := ""
	if textState.Font != nil {
		if len(textState.Font.EmbeddedFontData) > 0 {
			key := "pdf:" + textState.Font.Name
			embeddedPDFFontKey = key
			if err := RegisterFontData(key, textState.Font.EmbeddedFontData); err == nil {
				registeredPDFFont = true
				if !psPreferText {
					fontFamily = key
				}
			}
		}

		if textState.Font.BaseFont != "" {
			if psPreferText || !registeredPDFFont {
				if isTeXMathFont(textState.Font.BaseFont) {
					ensureRepoFontRegistered("math-regular", "fonts/ofl/stixtwomath/STIXTwoMath-Regular.ttf")
					fontFamily = "math"
				} else {
					fontFamily = mapPDFFont(textState.Font.BaseFont)
				}
			}
		} else if psPreferText && textState.Font.Name != "" {
			fontFamily = mapPDFFont(textState.Font.Name)
		}
	}

	useGlyphIndices := false
	if textState.Font != nil &&
		(textState.Font.Subtype == "/Type0" || textState.Font.Subtype == "Type0") &&
		textState.Font.IsIdentity &&
		registeredPDFFont &&
		!psPreferText {
		useGlyphIndices = true
	}

	sampleText := text
	if len(array) > 0 {
		for _, it := range array {
			if s, ok := it.(string); ok {
				sampleText = s
				break
			}
		}
	}
	if !registeredPDFFont && (fontFamily == "sans-serif" || fontFamily == "sans") {
		decodedSample, _, _ := decodeTextStringWithCIDs(sampleText, toUnicodeMap, textState.Font)
		if shouldUseCJKFallback(textState.Font, decodedSample) {
			ensureRepoFontRegistered("cjk-regular", "fonts/ofl/notosanssc/NotoSansSC[wght].ttf")
			fontFamily = "sans-cjk"
		}
	}

	psWantsOutline := false
	containsCJK := false
	containsMathOrSymbols := false
	if psPreferText {
		scanTextForFallback := func(s string) {
			for _, r := range s {
				if r <= 0x7F {
					continue
				}
				if unicode.In(r, unicode.Han, unicode.Hangul, unicode.Hiragana, unicode.Katakana) {
					containsCJK = true
					continue
				}
				if unicode.IsSymbol(r) || unicode.IsPunct(r) {
					containsMathOrSymbols = true
					continue
				}
				if (r >= 0x2100 && r <= 0x214F) || (r >= 0x2190 && r <= 0x21FF) || (r >= 0x2200 && r <= 0x22FF) || (r >= 0x27F0 && r <= 0x27FF) {
					containsMathOrSymbols = true
					continue
				}
			}
		}

		if sampleText != "" {
			decodedSample, _, _ := decodeTextStringWithCIDs(sampleText, toUnicodeMap, textState.Font)
			psWantsOutline = shouldOutlineTextInPS(textState.Font, decodedSample, nil, TextDecodeStats{})
			scanTextForFallback(decodedSample)
		}
		if !psWantsOutline && len(array) > 0 {
			checked := 0
			for _, it := range array {
				s, ok := it.(string)
				if !ok {
					continue
				}
				decoded, _, _ := decodeTextStringWithCIDs(s, toUnicodeMap, textState.Font)
				if shouldOutlineTextInPS(textState.Font, decoded, nil, TextDecodeStats{}) {
					psWantsOutline = true
					scanTextForFallback(decoded)
					break
				}
				scanTextForFallback(decoded)
				checked++
				if checked >= 16 {
					break
				}
			}
		}
	}

	if psPreferText && registeredPDFFont && psWantsOutline {
		if embeddedPDFFontKey != "" {
			fontFamily = embeddedPDFFontKey
		}
	}

	if psPreferText && !registeredPDFFont && psWantsOutline {
		if containsCJK {
			ensureRepoFontRegistered("cjk-regular", "fonts/ofl/notosanssc/NotoSansSC[wght].ttf")
			fontFamily = "sans-cjk"
		} else if containsMathOrSymbols {
			ensureRepoFontRegistered("symbols-regular", "fonts/ofl/notosanssymbols2/NotoSansSymbols2-Regular.ttf")
			fontFamily = "symbols"
		}
	}

	psForceOutline := psPreferText && strings.HasPrefix(fontFamily, "pdf:")

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
				state.FillColor.A*state.FillAlpha,
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
				state.StrokeColor.A*state.StrokeAlpha,
			)
		}
	case 2: // 填充+描边
		if state.FillColor != nil {
			ctx.GopdfCtx.SetSourceRGBA(
				state.FillColor.R,
				state.FillColor.G,
				state.FillColor.B,
				state.FillColor.A*state.FillAlpha,
			)
		}
	case 3: // 不可见
		return nil
	}

	if state != nil {
		ctx.GopdfCtx.SetLineWidth(state.LineWidth)
		ctx.GopdfCtx.SetLineCap(state.LineCap)
		ctx.GopdfCtx.SetLineJoin(state.LineJoin)
		ctx.GopdfCtx.SetMiterLimit(state.MiterLimit)
		if len(state.DashPattern) > 0 {
			ctx.GopdfCtx.SetDash(state.DashPattern, state.DashOffset)
		}
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

	computeRunScaleAndAdvance := func(runText string, cids []uint16) (scaleX float64, adv float64) {
		runes := []rune(runText)
		drawAdv := scaledFont.TextExtents(runText).XAdvance
		if drawAdv <= 0 {
			return 1.0, 0
		}
		if textState.CharSpacing != 0 || textState.WordSpacing != 0 {
			for _, r := range runes {
				if isCJKCharacterRune(r) {
					if textState.CharSpacing != 0 {
						drawAdv += textState.CharSpacing * 0.5
					}
				} else {
					drawAdv += textState.CharSpacing
				}
				if r == ' ' {
					drawAdv += textState.WordSpacing
				}
			}
		}

		hScale := textState.HorizontalScaling / 100.0
		if hScale <= 0 {
			hScale = 1
		}

		pdfAdvOK := false
		if textState.Font != nil {
			hasWidths := textState.Font.Widths != nil &&
				(len(textState.Font.Widths.Widths) > 0 || len(textState.Font.Widths.CIDWidths) > 0 || len(textState.Font.Widths.CIDRanges) > 0)
			if hasWidths {
				pdfAdvOK = true
			} else if textState.Font.DefaultWidth > 0 && math.Abs(textState.Font.DefaultWidth-1000.0) > 1e-3 {
				pdfAdvOK = true
			} else if textState.Font.embeddedWidthAvailable() {
				pdfAdvOK = true
			}
		}

		if pdfAdvOK && len(cids) > 0 {
			pdfAdv := 0.0
			for i, cid := range cids {
				isSpace := i < len(runes) && runes[i] == ' '
				pdfAdv += textState.GlyphAdvance(cid, isSpace)
			}
			if pdfAdv > 0 {
				unclamped := pdfAdv / drawAdv
				scaleX = unclamped
				minRatio, maxRatio := 0.6, 1.6
				asciiOnly := true
				for _, r := range runes {
					if r > 0x7F {
						asciiOnly = false
						break
					}
				}
				if asciiOnly {
					minRatio, maxRatio = 0.25, 2.5
				} else {
					minRatio, maxRatio = 0.5, 2.0
				}
				if scaleX < minRatio {
					scaleX = minRatio
				} else if scaleX > maxRatio {
					scaleX = maxRatio
				}
				if scaleX != unclamped {
					return scaleX, drawAdv * scaleX
				}
				return scaleX, pdfAdv
			}
		}

		scaleX = hScale
		adv = drawAdv * hScale
		return scaleX, adv
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

	autoSymbolDy := func(runText string) float64 {
		runes := []rune(runText)
		if len(runes) != 1 {
			return 0
		}
		r := runes[0]
		if unicode.IsSpace(r) || unicode.IsLetter(r) || unicode.IsNumber(r) {
			return 0
		}
		if !(unicode.IsSymbol(r) || unicode.IsPunct(r)) {
			return 0
		}

		extSym := scaledFont.TextExtents(runText)
		extRef := scaledFont.TextExtents("x")
		if extRef.Height <= 0 {
			extRef = scaledFont.TextExtents("0")
		}
		if extSym.Height <= 0 || extRef.Height <= 0 {
			return 0
		}

		symMinY := extSym.YBearing
		symMaxY := extSym.YBearing + extSym.Height
		refMinY := extRef.YBearing
		refMaxY := extRef.YBearing + extRef.Height

		symCenter := (symMinY + symMaxY) * 0.5
		refCenter := (refMinY + refMaxY) * 0.5
		dy := refCenter - symCenter

		limit := fontSize * 0.6
		if dy > limit {
			dy = limit
		} else if dy < -limit {
			dy = -limit
		}
		return dy
	}

	renderRun := func(runText string, cids []uint16, scaleX float64) {
		if runText == "" {
			return
		}

		isMathFamily := strings.EqualFold(fontFamily, "math")

		renderRunAtX := func(x float64, t string, c []uint16, sx float64) {
			dy := 0.0
			sy := 1.0
			if !isMathFamily {
				dy = autoSymbolDy(t)
				if textState.Font != nil && len(c) == 1 && len(textState.Font.CodeToGlyphName) > 0 {
					if name, ok := textState.Font.CodeToGlyphName[byte(c[0]&0xFF)]; ok {
						if dyFactor, s, ok := delimiterAdjust(name); ok {
							dy = dyFactor * fontSize
							sy = s
						}
					}
				}
			}

			if sx != 1.0 || dy != 0.0 || sy != 1.0 {
				ctx.GopdfCtx.Save()
				ctx.GopdfCtx.Translate(x, dy)
				ctx.GopdfCtx.Scale(sx, sy)
				ctx.GopdfCtx.MoveTo(0, 0)
				layout.SetText(t)
				ctx.GopdfCtx.PangoPdfShowText(layout)
				ctx.GopdfCtx.Restore()
				return
			}

			ctx.GopdfCtx.MoveTo(x, 0)
			layout.SetText(t)
			ctx.GopdfCtx.PangoPdfShowText(layout)
		}

		if psPreferText && !isMathFamily {
			runes := []rune(runText)
			if len(runes) == len(cids) && len(runes) <= 32 {
				localX := currentX
				for i, r := range runes {
					cid := cids[i]
					isSpace := r == ' '
					pdfAdv := textState.GlyphAdvance(cid, isSpace)

					drawAdv := scaledFont.TextExtents(string(r)).XAdvance
					if drawAdv <= 0 {
						drawAdv = fontSize * 0.50
					}
					if textState.CharSpacing != 0 {
						if isCJKCharacterRune(r) {
							drawAdv += textState.CharSpacing * 0.5
						} else {
							drawAdv += textState.CharSpacing
						}
					}
					if isSpace && textState.WordSpacing != 0 {
						drawAdv += textState.WordSpacing
					}

					sx := 1.0
					if pdfAdv > 0 && drawAdv > 0 {
						sx = pdfAdv / drawAdv
						minRatio, maxRatio := 0.25, 2.5
						if sx < minRatio {
							sx = minRatio
						} else if sx > maxRatio {
							sx = maxRatio
						}
					}
					renderRunAtX(localX, string(r), []uint16{cid}, sx)
					localX += pdfAdv
				}
				return
			}
		}

		renderRunAtX(currentX, runText, cids, scaleX)
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

	renderShapedGlyphPaths := func(runText string, scaleX float64) bool {
		if runText == "" {
			return false
		}
		realFace, status := scaledFont.getRealFace()
		if status != StatusSuccess || realFace == nil {
			return false
		}

		runes := []rune(runText)
		if len(runes) == 0 {
			return false
		}

		size := fixed.Int26_6(textState.FontSize * 64)
		if size <= 0 {
			size = fixed.I(12)
		}

		input := shaping.Input{
			Text:      runes,
			RunStart:  0,
			RunEnd:    len(runes),
			Direction: di.DirectionLTR,
			Face:      realFace,
			Size:      size,
		}
		output := (&shaping.HarfbuzzShaper{}).Shape(input)
		if len(output.Glyphs) == 0 {
			return false
		}

		glyphs := make([]Glyph, 0, len(output.Glyphs))
		x := 0.0
		for _, g := range output.Glyphs {
			glyphs = append(glyphs, Glyph{
				Index: uint64(g.GlyphID),
				X:     x + float64(g.XOffset)/64.0,
				Y:     -float64(g.YOffset) / 64.0,
			})
			x += float64(g.XAdvance) / 64.0
		}

		dy := autoSymbolDy(runText)
		ctx.GopdfCtx.Save()
		ctx.GopdfCtx.Translate(currentX, dy)
		if scaleX != 1.0 {
			ctx.GopdfCtx.Scale(scaleX, 1.0)
		}
		renderGlyphRun(ctx.GopdfCtx, scaledFont, glyphs, textState.RenderMode)
		ctx.GopdfCtx.Restore()
		return true
	}

	renderType0CFFGlyphs := func(decodedText string, cids []uint16) (adv float64, ok bool) {
		if textState.Font == nil || textState.Font.CFF == nil {
			return 0, false
		}
		if textState.Font.Subtype != "/Type0" && textState.Font.Subtype != "Type0" {
			return 0, false
		}
		if len(cids) == 0 {
			return 0, false
		}

		runes := []rune(decodedText)
		x := currentX
		for i, cid := range cids {
			gid := textState.Font.CIDToGID(cid)
			segments, _, err := textState.Font.CFF.LoadGlyph(tables.GlyphID(gid))
			if err != nil || len(segments) == 0 {
				return 0, false
			}

			ctx.GopdfCtx.Save()

			fontMatrix := [6]float64{0.001, 0, 0, 0.001, 0, 0}
			if textState.Font.HasFontMatrix {
				fontMatrix = textState.Font.FontMatrix
			}
			hScale := textState.HorizontalScaling / 100.0
			m := &Matrix{
				XX: fontMatrix[0] * textState.FontSize * hScale,
				YX: fontMatrix[1] * textState.FontSize,
				XY: fontMatrix[2] * textState.FontSize * hScale,
				YY: fontMatrix[3] * textState.FontSize,
				X0: fontMatrix[4] * textState.FontSize * hScale,
				Y0: fontMatrix[5] * textState.FontSize,
			}

			r := rune(0)
			if i < len(runes) {
				r = runes[i]
			}

			minY := math.Inf(1)
			maxY := math.Inf(-1)
			for _, seg := range segments {
				for _, p := range seg.Args {
					y := float64(p.Y)
					if y < minY {
						minY = y
					}
					if y > maxY {
						maxY = y
					}
				}
			}

			centerYUnits := 0.0
			heightUnits := 0.0
			if !math.IsInf(minY, 0) && !math.IsInf(maxY, 0) && maxY > minY {
				centerYUnits = (minY + maxY) * 0.5
				heightUnits = maxY - minY
			}

			if !textState.Font.cffRefReady && heightUnits > 0 && (unicode.IsLetter(r) || unicode.IsNumber(r)) {
				textState.Font.cffRefCenterUnits = centerYUnits
				textState.Font.cffRefReady = true
			}

			dy := 0.0
			if textState.Font.cffRefReady && heightUnits > 0 && !unicode.IsSpace(r) && (unicode.IsSymbol(r) || unicode.IsPunct(r)) {
				dyUnits := textState.Font.cffRefCenterUnits - centerYUnits
				dy = dyUnits * m.YY
				limit := textState.FontSize * 0.6
				if dy > limit {
					dy = limit
				} else if dy < -limit {
					dy = -limit
				}
			}

			ctx.GopdfCtx.Translate(x, dy)
			ctx.GopdfCtx.Transform(m)

			ctx.GopdfCtx.NewPath()
			for _, seg := range segments {
				switch seg.Op {
				case otapi.SegmentOpMoveTo:
					p := seg.Args[0]
					ctx.GopdfCtx.MoveTo(float64(p.X), float64(p.Y))
				case otapi.SegmentOpLineTo:
					p := seg.Args[0]
					ctx.GopdfCtx.LineTo(float64(p.X), float64(p.Y))
				case otapi.SegmentOpQuadTo:
					p1 := seg.Args[0]
					p2 := seg.Args[1]
					ctx.GopdfCtx.CurveTo(
						float64(p1.X), float64(p1.Y),
						float64(p1.X), float64(p1.Y),
						float64(p2.X), float64(p2.Y),
					)
				case otapi.SegmentOpCubeTo:
					p1 := seg.Args[0]
					p2 := seg.Args[1]
					p3 := seg.Args[2]
					ctx.GopdfCtx.CurveTo(
						float64(p1.X), float64(p1.Y),
						float64(p2.X), float64(p2.Y),
						float64(p3.X), float64(p3.Y),
					)
				}
			}

			switch textState.RenderMode {
			case 1:
				if state != nil && state.StrokeColor != nil {
					ctx.GopdfCtx.SetSourceRGBA(
						state.StrokeColor.R,
						state.StrokeColor.G,
						state.StrokeColor.B,
						state.StrokeColor.A*state.StrokeAlpha,
					)
				}
				ctx.GopdfCtx.Stroke()
			case 2:
				if state != nil && state.FillColor != nil {
					ctx.GopdfCtx.SetSourceRGBA(
						state.FillColor.R,
						state.FillColor.G,
						state.FillColor.B,
						state.FillColor.A*state.FillAlpha,
					)
				}
				ctx.GopdfCtx.FillPreserve()
				if state != nil && state.StrokeColor != nil {
					ctx.GopdfCtx.SetSourceRGBA(
						state.StrokeColor.R,
						state.StrokeColor.G,
						state.StrokeColor.B,
						state.StrokeColor.A*state.StrokeAlpha,
					)
				}
				ctx.GopdfCtx.Stroke()
			default:
				if state != nil && state.FillColor != nil {
					ctx.GopdfCtx.SetSourceRGBA(
						state.FillColor.R,
						state.FillColor.G,
						state.FillColor.B,
						state.FillColor.A*state.FillAlpha,
					)
				}
				ctx.GopdfCtx.Fill()
			}

			ctx.GopdfCtx.Restore()

			isSpace := cid == 32
			if i < len(runes) && runes[i] == ' ' {
				isSpace = true
			}
			advStep := textState.GlyphAdvance(cid, isSpace)
			x += advStep
			adv += advStep
		}
		return adv, true
	}

	isTeXMathBaseFont := func(baseFont string) bool {
		base := strings.ToUpper(stripSubsetPrefix(baseFont))
		return strings.HasPrefix(base, "CMMI") ||
			strings.HasPrefix(base, "CMSY") ||
			strings.HasPrefix(base, "CMEX") ||
			strings.HasPrefix(base, "MSBM")
	}

	renderTeXMathFallbackGlyphs := func(runText string, cids []uint16) (adv float64, ok bool) {
		runes := []rune(runText)
		if len(runes) == 0 || len(runes) != len(cids) {
			return 0, false
		}
		if textState.Font == nil {
			return 0, false
		}

		x := currentX
		for i, r := range runes {
			cid := cids[i]

			ctx.GopdfCtx.Save()
			useCFF := textState.Font.CFF != nil
			if useCFF {
				ctx.GopdfCtx.Translate(x, 0)
				gid := uint16(cid & 0xFF)
				if mapped, ok := textState.Font.CodeToGID[byte(cid&0xFF)]; ok {
					gid = mapped
				}

				segments, _, err := textState.Font.CFF.LoadGlyph(tables.GlyphID(gid))
				if err != nil || len(segments) == 0 {
					ctx.GopdfCtx.Restore()
					return 0, false
				}

				fontMatrix := [6]float64{0.001, 0, 0, 0.001, 0, 0}
				if textState.Font.HasFontMatrix {
					fontMatrix = textState.Font.FontMatrix
				}
				hScale := textState.HorizontalScaling / 100.0
				m := &Matrix{
					XX: fontMatrix[0] * textState.FontSize * hScale,
					YX: fontMatrix[1] * textState.FontSize,
					XY: fontMatrix[2] * textState.FontSize * hScale,
					YY: fontMatrix[3] * textState.FontSize,
					X0: fontMatrix[4] * textState.FontSize * hScale,
					Y0: fontMatrix[5] * textState.FontSize,
				}
				ctx.GopdfCtx.Transform(m)

				ctx.GopdfCtx.NewPath()
				for _, seg := range segments {
					switch seg.Op {
					case otapi.SegmentOpMoveTo:
						p := seg.Args[0]
						ctx.GopdfCtx.MoveTo(float64(p.X), float64(p.Y))
					case otapi.SegmentOpLineTo:
						p := seg.Args[0]
						ctx.GopdfCtx.LineTo(float64(p.X), float64(p.Y))
					case otapi.SegmentOpQuadTo:
						p1 := seg.Args[0]
						p2 := seg.Args[1]
						ctx.GopdfCtx.CurveTo(
							float64(p1.X), float64(p1.Y),
							float64(p1.X), float64(p1.Y),
							float64(p2.X), float64(p2.Y),
						)
					case otapi.SegmentOpCubeTo:
						p1 := seg.Args[0]
						p2 := seg.Args[1]
						p3 := seg.Args[2]
						ctx.GopdfCtx.CurveTo(
							float64(p1.X), float64(p1.Y),
							float64(p2.X), float64(p2.Y),
							float64(p3.X), float64(p3.Y),
						)
					}
				}
			} else {
				dy := 0.0
				scaleY := 1.0
				if name, ok := textState.Font.CodeToGlyphName[byte(cid&0xFF)]; ok {
					if dyFactor, sy, ok := delimiterAdjust(name); ok {
						dy = dyFactor * fontSize
						scaleY = sy
					}
				}

				ctx.GopdfCtx.Translate(x, dy)

				if scaledFont == nil {
					ctx.GopdfCtx.Restore()
					return 0, false
				}

				glyphs, _, _, status := scaledFont.TextToGlyphs(0, 0, string(r))
				if status != StatusSuccess || len(glyphs) == 0 {
					ctx.GopdfCtx.Restore()
					return 0, false
				}

				gid := glyphs[0].Index
				glyphPath, err := scaledFont.GlyphPath(gid)
				if err != nil || glyphPath == nil || len(glyphPath.Data) == 0 {
					ctx.GopdfCtx.Restore()
					return 0, false
				}

				if scaleY != 1.0 {
					ctx.GopdfCtx.Scale(1.0, scaleY)
				}

				ctx.GopdfCtx.NewPath()
				for _, pathData := range glyphPath.Data {
					switch pathData.Type {
					case PathMoveTo:
						if len(pathData.Points) > 0 {
							ctx.GopdfCtx.MoveTo(pathData.Points[0].X, pathData.Points[0].Y)
						}
					case PathLineTo:
						if len(pathData.Points) > 0 {
							ctx.GopdfCtx.LineTo(pathData.Points[0].X, pathData.Points[0].Y)
						}
					case PathCurveTo:
						if len(pathData.Points) >= 3 {
							ctx.GopdfCtx.CurveTo(
								pathData.Points[0].X, pathData.Points[0].Y,
								pathData.Points[1].X, pathData.Points[1].Y,
								pathData.Points[2].X, pathData.Points[2].Y,
							)
						}
					case PathClosePath:
						ctx.GopdfCtx.ClosePath()
					}
				}
			}

			switch textState.RenderMode {
			case 1:
				if state != nil && state.StrokeColor != nil {
					ctx.GopdfCtx.SetSourceRGBA(
						state.StrokeColor.R,
						state.StrokeColor.G,
						state.StrokeColor.B,
						state.StrokeColor.A*state.StrokeAlpha,
					)
				}
				ctx.GopdfCtx.Stroke()
			case 2:
				if state != nil && state.FillColor != nil {
					ctx.GopdfCtx.SetSourceRGBA(
						state.FillColor.R,
						state.FillColor.G,
						state.FillColor.B,
						state.FillColor.A*state.FillAlpha,
					)
				}
				ctx.GopdfCtx.FillPreserve()
				if state != nil && state.StrokeColor != nil {
					ctx.GopdfCtx.SetSourceRGBA(
						state.StrokeColor.R,
						state.StrokeColor.G,
						state.StrokeColor.B,
						state.StrokeColor.A*state.StrokeAlpha,
					)
				}
				ctx.GopdfCtx.Stroke()
			default:
				if state != nil && state.FillColor != nil {
					ctx.GopdfCtx.SetSourceRGBA(
						state.FillColor.R,
						state.FillColor.G,
						state.FillColor.B,
						state.FillColor.A*state.FillAlpha,
					)
				}
				ctx.GopdfCtx.Fill()
			}

			ctx.GopdfCtx.Restore()

			isSpace := r == ' '
			advStep := textState.GlyphAdvance(cid, isSpace)
			x += advStep
			adv += advStep
		}

		currentX += adv
		return adv, true
	}

	// 渲染文本
	if len(array) > 0 {
		// TJ 操作符：处理文本数组
		debugPrintf("[TJ_ARRAY] Processing %d items\n", len(array))

		psBodyText := false
		enablePSTextSpaceSynthesis := false

		mergeTol := fontSize * 0.06
		pendingKerning := 0.0
		pendingSpaces := 0
		var bufText string
		var bufCIDs []uint16
		var bufStats TextDecodeStats
		bufHas := false

		flushBuf := func() {
			if !bufHas {
				return
			}
			decodedText := bufText
			cids := bufCIDs
			stats := bufStats

			useGlyphRun := useGlyphIndices
			outline := psPreferText && (psForceOutline || shouldOutlineTextInPS(textState.Font, decodedText, cids, stats))
			if psPreferText && decodedText != "" && !outline {
				useGlyphRun = false
			}

			if useGlyphRun {
				if psPreferText && outline && decodedText != "" {
					scaleX, adv := computeRunScaleAndAdvance(decodedText, cids)
					if renderShapedGlyphPaths(decodedText, scaleX) {
						currentX += adv
						debugPrintf("[TJ_ARRAY][BUF] Rendered shaped glyph paths, adv=%.2f\n", adv)
						bufHas = false
						bufText = ""
						bufCIDs = nil
						bufStats = TextDecodeStats{}
						return
					}
				}

				renderGlyphs(cids)
				debugPrintf("[TJ_ARRAY][BUF] Rendered glyph run\n")
			} else {
				if !psPreferText || outline {
					if adv, ok := renderType0CFFGlyphs(decodedText, cids); ok {
						currentX += adv
						debugPrintf("[TJ_ARRAY][BUF] Rendered Type0 CFF glyphs, adv=%.2f\n", adv)
						bufHas = false
						bufText = ""
						bufCIDs = nil
						bufStats = TextDecodeStats{}
						return
					}
					if isTeXMathBaseFont(textState.Font.BaseFont) {
						if adv, ok := renderTeXMathFallbackGlyphs(decodedText, cids); ok {
							debugPrintf("[TJ_ARRAY][BUF] Rendered TeX glyphs, adv=%.2f\n", adv)
							bufHas = false
							bufText = ""
							bufCIDs = nil
							bufStats = TextDecodeStats{}
							return
						}
					}
				}

				scaleX, adv := computeRunScaleAndAdvance(decodedText, cids)
				if psPreferText && outline {
					if renderShapedGlyphPaths(decodedText, scaleX) {
						currentX += adv
						debugPrintf("[TJ_ARRAY][BUF] Rendered shaped glyph paths, adv=%.2f\n", adv)
						bufHas = false
						bufText = ""
						bufCIDs = nil
						bufStats = TextDecodeStats{}
						return
					}
					if psForceOutline && len(cids) > 0 {
						renderGlyphs(cids)
						debugPrintf("[TJ_ARRAY][BUF] Rendered glyph run (forced outline)\n")
						bufHas = false
						bufText = ""
						bufCIDs = nil
						bufStats = TextDecodeStats{}
						return
					}
				}
				renderRun(decodedText, cids, scaleX)
				currentX += adv
				debugPrintf("[TJ_ARRAY][BUF] Rendered run, adv=%.2f\n", adv)
			}

			bufHas = false
			bufText = ""
			bufCIDs = nil
			bufStats = TextDecodeStats{}
		}

		for idx, item := range array {
			switch v := item.(type) {
			case string:
				if psPreferText {
					if enablePSTextSpaceSynthesis && psBodyText && pendingSpaces > 0 && bufHas {
						bufText += strings.Repeat(" ", pendingSpaces)
						pendingSpaces = 0
					}
					if bufHas {
						if math.Abs(pendingKerning) > mergeTol {
							flushBuf()
							currentX += pendingKerning
						}
					} else if pendingKerning != 0 {
						currentX += pendingKerning
					}
					pendingKerning = 0
				}

				// 解码文本并获取 CID 数组
				decodedText, cids, stats := decodeTextStringWithCIDs(v, toUnicodeMap, textState.Font)
				if decodedText == "" && !useGlyphIndices {
					debugPrintf("[TJ_ARRAY][%d] Empty string after decode\n", idx)
					continue
				}

				debugPrintf("[TJ_ARRAY][%d] Text=%q (len=%d runes, %d CIDs) at x=%.2f\n",
					idx, decodedText, len([]rune(decodedText)), len(cids), currentX)

				if psPreferText {
					if !bufHas {
						bufHas = true
						bufText = decodedText
						if !psBodyText {
							bufCIDs = append(bufCIDs[:0], cids...)
						} else {
							bufCIDs = nil
						}
						bufStats = stats
					} else {
						bufText += decodedText
						if !psBodyText {
							bufCIDs = append(bufCIDs, cids...)
						}
						bufStats.CIDCount += stats.CIDCount
						bufStats.ToUnicodeHit += stats.ToUnicodeHit
						bufStats.GlyphNameHit += stats.GlyphNameHit
						bufStats.IdentityASCIIHit += stats.IdentityASCIIHit
						bufStats.Replaced += stats.Replaced
					}
					continue
				}

				useGlyphRun := useGlyphIndices
				outline := psPreferText && (psForceOutline || shouldOutlineTextInPS(textState.Font, decodedText, cids, stats))
				if psPreferText && decodedText != "" && !outline {
					useGlyphRun = false
				}

				if useGlyphRun {
					if psPreferText && outline && decodedText != "" {
						scaleX, adv := computeRunScaleAndAdvance(decodedText, cids)
						if renderShapedGlyphPaths(decodedText, scaleX) {
							currentX += adv
							debugPrintf("[TJ_ARRAY][%d] Rendered shaped glyph paths, adv=%.2f\n", idx, adv)
							continue
						}
					}

					renderGlyphs(cids)
					debugPrintf("[TJ_ARRAY][%d] Rendered glyph run\n", idx)
				} else {
					if !psPreferText || outline {
						if adv, ok := renderType0CFFGlyphs(decodedText, cids); ok {
							currentX += adv
							debugPrintf("[TJ_ARRAY][%d] Rendered Type0 CFF glyphs, adv=%.2f\n", idx, adv)
							continue
						}
						if isTeXMathBaseFont(textState.Font.BaseFont) {
							if adv, ok := renderTeXMathFallbackGlyphs(decodedText, cids); ok {
								debugPrintf("[TJ_ARRAY][%d] Rendered TeX glyphs, adv=%.2f\n", idx, adv)
								continue
							}
						}
					}

					scaleX, adv := computeRunScaleAndAdvance(decodedText, cids)
					if psPreferText && outline {
						if renderShapedGlyphPaths(decodedText, scaleX) {
							currentX += adv
							debugPrintf("[TJ_ARRAY][%d] Rendered shaped glyph paths, adv=%.2f\n", idx, adv)
							continue
						}
					}
					renderRun(decodedText, cids, scaleX)
					currentX += adv
					debugPrintf("[TJ_ARRAY][%d] Rendered run, adv=%.2f\n", idx, adv)
				}

			case float64:
				if psPreferText {
					kerningAdjustment := -v * fontSize / 1000.0 * textState.HorizontalScaling / 100.0
					if kerningAdjustment < 0 && !registeredPDFFont {
						maxBack := fontSize * 0.5
						if kerningAdjustment < -maxBack {
							kerningAdjustment = -maxBack
						}
					}
					if enablePSTextSpaceSynthesis && psBodyText {
						spaceTol := fontSize * 0.18
						if kerningAdjustment > spaceTol {
							pendingSpaces++
							if pendingSpaces > 3 {
								pendingSpaces = 3
							}
						}
						continue
					}
					pendingKerning += kerningAdjustment
					continue
				}

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
				if psPreferText {
					kerningAdjustment := -float64(v) * fontSize / 1000.0 * textState.HorizontalScaling / 100.0
					if kerningAdjustment < 0 && !registeredPDFFont {
						maxBack := fontSize * 0.5
						if kerningAdjustment < -maxBack {
							kerningAdjustment = -maxBack
						}
					}
					if enablePSTextSpaceSynthesis && psBodyText {
						spaceTol := fontSize * 0.18
						if kerningAdjustment > spaceTol {
							pendingSpaces++
							if pendingSpaces > 3 {
								pendingSpaces = 3
							}
						}
						continue
					}
					pendingKerning += kerningAdjustment
					continue
				}

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

		if psPreferText {
			if enablePSTextSpaceSynthesis && psBodyText && pendingSpaces > 0 && bufHas {
				bufText += strings.Repeat(" ", pendingSpaces)
				pendingSpaces = 0
			}
			flushBuf()
			if !psBodyText && pendingKerning != 0 {
				currentX += pendingKerning
			}
		}
	} else {
		// Tj 操作符：简单文本
		decodedText, cids, stats := decodeTextStringWithCIDs(text, toUnicodeMap, textState.Font)
		if (decodedText != "" && !useGlyphIndices) || (useGlyphIndices && len(cids) > 0) {
			debugPrintf("[Tj] Text=%q (len=%d runes, %d CIDs) at Tm=[%.2f, %.2f]\n",
				decodedText, len([]rune(decodedText)), len(cids), textState.TextMatrix.X0, textState.TextMatrix.Y0)

			useGlyphRun := useGlyphIndices
			outline := psPreferText && (psForceOutline || shouldOutlineTextInPS(textState.Font, decodedText, cids, stats))
			if psPreferText && decodedText != "" && !outline {
				useGlyphRun = false
			}

			if useGlyphRun {
				if psPreferText && outline && decodedText != "" {
					scaleX, adv := computeRunScaleAndAdvance(decodedText, cids)
					if renderShapedGlyphPaths(decodedText, scaleX) {
						currentX += adv
						debugPrintf("[Tj] Rendered shaped glyph paths, adv=%.2f\n", adv)
						goto updateMatrix
					}
				}

				renderGlyphs(cids)
				debugPrintf("[Tj] Rendered glyph run\n")
			} else {
				if !psPreferText || outline {
					if adv, ok := renderType0CFFGlyphs(decodedText, cids); ok {
						currentX += adv
						debugPrintf("[Tj] Rendered Type0 CFF glyphs, adv=%.2f\n", adv)
						goto updateMatrix
					}
					if isTeXMathBaseFont(textState.Font.BaseFont) {
						if adv, ok := renderTeXMathFallbackGlyphs(decodedText, cids); ok {
							debugPrintf("[Tj] Rendered TeX glyphs, adv=%.2f\n", adv)
							goto updateMatrix
						}
					}
				}

				scaleX, adv := computeRunScaleAndAdvance(decodedText, cids)
				if psPreferText && outline {
					if renderShapedGlyphPaths(decodedText, scaleX) {
						currentX += adv
						debugPrintf("[Tj] Rendered shaped glyph paths, adv=%.2f\n", adv)
						goto updateMatrix
					}
					if psForceOutline && len(cids) > 0 {
						renderGlyphs(cids)
						debugPrintf("[Tj] Rendered glyph run (forced outline)\n")
						goto updateMatrix
					}
				}
				renderRun(decodedText, cids, scaleX)
				currentX += adv
				debugPrintf("[Tj] Rendered run, adv=%.2f\n", adv)
			}
		}
	}

	// 更新文本矩阵：使用PDF的字形宽度
	// 这对于在同一个BT...ET块中的多个Tj操作是必要的
updateMatrix:
	if currentX != 0 {
		translation := NewTranslationMatrix(currentX, 0)
		textState.TextMatrix = textState.TextMatrix.Multiply(translation)
		debugPrintf("[TEXT_MATRIX] Updated after text: PDF_width=%.2f, new X0=%.2f\n",
			currentX, textState.TextMatrix.X0)
	}

	if psPreferText && psCtx != nil {
		psCtx.psLastText.active = true
		psCtx.psLastText.yUser = renderMatrix.Y0
		psCtx.psLastText.fontSize = fontSize
		psCtx.psLastText.xEndUser = renderMatrix.X0 + currentX*renderMatrix.XX
		psCtx.psLastText.text = candidateDecoded
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

type TextDecodeStats struct {
	CIDCount         int
	ToUnicodeHit     int
	GlyphNameHit     int
	IdentityASCIIHit int
	Replaced         int
}

func shouldOutlineTextInPS(font *Font, decodedText string, cids []uint16, stats TextDecodeStats) bool {
	if font == nil {
		return false
	}
	base := strings.ToLower(stripSubsetPrefix(font.BaseFont))
	if base == "" {
		base = strings.ToLower(stripSubsetPrefix(font.Name))
	}

	if isTeXMathBaseFontName(font.BaseFont) {
		return strings.HasPrefix(base, "cmex")
	}

	if len(cids) == 1 && len(font.CodeToGlyphName) > 0 {
		if name, ok := font.CodeToGlyphName[byte(cids[0]&0xFF)]; ok && name != "" {
			n := strings.ToLower(strings.TrimPrefix(name, "/"))
			if strings.Contains(n, "big") || strings.Contains(n, "bigg") || strings.Contains(n, "biggg") {
				return false
			}
			if strings.Contains(n, "integral") || strings.Contains(n, "summation") || strings.Contains(n, "radical") {
				return false
			}
			if strings.Contains(n, "brace") || strings.Contains(n, "bracket") || strings.Contains(n, "paren") {
				if strings.Contains(n, "big") {
					return false
				}
			}
		}
	}

	for _, k := range []string{
		"math", "stix", "symbol", "dingbat", "zapfdingbats",
		"cmr", "cmsy", "cmex", "latinmodern", "tex",
		"bravura", "maestro", "opus", "sonata", "finale", "music",
		"courier", "consolas", "menlo", "monaco", "fira", "sourcecode", "dejavusansmono", "mono",
	} {
		if strings.Contains(base, k) {
			return false
		}
	}

	if looksLikeLaTeXText(decodedText) {
		return false
	}

	for _, r := range decodedText {
		if r == 0x2022 {
			return false
		}
		if r >= 0xE000 && r <= 0xF8FF {
			return true
		}
		if r >= 0x1D100 && r <= 0x1D1FF {
			// Musical symbols
			return false
		}
	}

	for _, r := range decodedText {
		if r <= 0x7F {
			continue
		}
		if unicode.In(r, unicode.Han, unicode.Hangul, unicode.Hiragana, unicode.Katakana) {
			return true
		}
		if unicode.IsSymbol(r) {
			return true
		}
		if unicode.IsLetter(r) || unicode.IsMark(r) {
			return true
		}
	}

	return false
}

func looksLikeLaTeXText(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if strings.Contains(s, "\\") {
		for _, k := range []string{
			"\\frac", "\\sum", "\\int", "\\sqrt", "\\alpha", "\\beta", "\\gamma",
			"\\left", "\\right", "\\begin", "\\end", "\\mathrm", "\\mathbf",
		} {
			if strings.Contains(s, k) {
				return true
			}
		}
	}
	symbols := 0
	for _, r := range s {
		switch r {
		case '$', '{', '}', '_', '^':
			symbols++
		}
	}
	return symbols >= 2
}

func isTeXMathBaseFontName(baseFont string) bool {
	base := strings.ToUpper(stripSubsetPrefix(baseFont))
	return strings.HasPrefix(base, "CMMI") ||
		strings.HasPrefix(base, "CMSY") ||
		strings.HasPrefix(base, "CMEX") ||
		strings.HasPrefix(base, "MSBM")
}

// decodeTextStringWithCIDs 解码文本并返回 Unicode 字符串、CID 数组和解码统计
func decodeTextStringWithCIDs(text string, toUnicodeMap *CIDToUnicodeMap, font *Font) (string, []uint16, TextDecodeStats) {
	// 检查是否是十六进制字符串
	if len(text) >= 2 && text[0] == '<' && text[len(text)-1] == '>' {
		result := decodePDFHexStringBytes(text)

		if font != nil && (font.Subtype == "/Type0" || font.Subtype == "Type0") && toUnicodeMap != nil && toUnicodeMap.parsed != nil {
			codes := toUnicodeMap.parsed.splitCodes(result)
			decodedStr := toUnicodeMap.parsed.decodeFontString(result)
			stats := TextDecodeStats{CIDCount: len(codes)}

			cids := make([]uint16, 0, len(codes))
			for _, c := range codes {
				if _, ok := toUnicodeMap.parsed.mapping[string(c)]; ok {
					stats.ToUnicodeHit++
				} else {
					stats.Replaced++
				}
				switch len(c) {
				case 1:
					cids = append(cids, uint16(c[0]))
				case 2:
					cids = append(cids, uint16(c[0])<<8|uint16(c[1]))
				default:
					if len(c) >= 2 {
						cids = append(cids, uint16(c[len(c)-2])<<8|uint16(c[len(c)-1]))
					}
				}
			}

			return ensureValidUTF8(decodedStr), cids, stats
		}

		var cids []uint16
		if font != nil && (font.Subtype == "/Type0" || font.Subtype == "Type0") {
			if len(result) < 2 {
				for _, b := range result {
					cids = append(cids, uint16(b))
				}
			} else if len(result)%2 != 0 {
				for _, b := range result {
					cids = append(cids, uint16(b))
				}
			} else {
				for i := 0; i < len(result); i += 2 {
					cid := uint16(result[i])<<8 | uint16(result[i+1])
					cids = append(cids, cid)
				}
			}
		} else {
			for _, b := range result {
				cids = append(cids, uint16(b))
			}
		}

		// 解码为 Unicode
		var decoded strings.Builder
		isIdentity := font != nil && font.IsIdentity
		stats := TextDecodeStats{CIDCount: len(cids)}

		for _, cid := range cids {
			if toUnicodeMap != nil {
				if uni, ok := toUnicodeMap.MapCIDToUnicode(cid); ok {
					if isValidUnicodeRune(uni) {
						decoded.WriteRune(uni)
						stats.ToUnicodeHit++
						continue
					}
				}
			}

			if font != nil && len(font.CodeToGlyphName) > 0 {
				if name, ok := font.CodeToGlyphName[byte(cid&0xFF)]; ok {
					if r, ok := glyphNameToRuneForFont(name, font); ok {
						decoded.WriteRune(r)
						stats.GlyphNameHit++
						continue
					}
				}
			}

			if font != nil {
				if r, ok := texMathRuneFromCID(font.BaseFont, cid); ok {
					decoded.WriteRune(r)
					stats.GlyphNameHit++
					continue
				}
			}

			if isIdentity && font != nil && (font.Subtype == "/Type0" || font.Subtype == "Type0") {
				gid := font.CIDToGID(cid)
				if r, ok := font.gidToUnicodeFromEmbeddedFont(gid); ok && isValidUnicodeRune(r) {
					decoded.WriteRune(r)
					continue
				}
				if font.CFF != nil {
					if name := font.CFF.GlyphName(otapi.GID(gid)); name != "" {
						if r, ok := glyphNameToRuneForFont(name, font); ok && isValidUnicodeRune(r) {
							decoded.WriteRune(r)
							stats.GlyphNameHit++
							continue
						}
					}
				}
				if cid >= 0x20 && cid <= 0x7E {
					decoded.WriteByte(byte(cid))
					stats.IdentityASCIIHit++
					continue
				}
				if lo := cid & 0x00FF; lo >= 0x20 && lo <= 0x7E {
					decoded.WriteByte(byte(lo))
					stats.IdentityASCIIHit++
					continue
				}
			}

			decoded.WriteRune('�')
			stats.Replaced++
		}
		decodedText := decoded.String()
		if font != nil {
			decodedText = texMathNormalizeDecodedText(font.BaseFont, decodedText)
		}
		return ensureValidUTF8(decodedText), cids, stats

		// 否则尝试标准解码
		decodedStr := decodeTextString(text)
		return ensureValidUTF8(decodedStr), cids, stats
	}

	// 普通字符串 - 处理反斜杠转义并转换为 CID 数组（字节码）
	rawBytes := decodePDFLiteralStringBytes(text)
	var cids []uint16
	if font != nil && (font.Subtype == "/Type0" || font.Subtype == "Type0") && toUnicodeMap != nil && toUnicodeMap.parsed != nil {
		codes := toUnicodeMap.parsed.splitCodes(rawBytes)
		decodedStr := toUnicodeMap.parsed.decodeFontString(rawBytes)
		stats := TextDecodeStats{CIDCount: len(codes)}

		for _, c := range codes {
			if _, ok := toUnicodeMap.parsed.mapping[string(c)]; ok {
				stats.ToUnicodeHit++
			} else {
				stats.Replaced++
			}
			switch len(c) {
			case 1:
				cids = append(cids, uint16(c[0]))
			case 2:
				cids = append(cids, uint16(c[0])<<8|uint16(c[1]))
			default:
				if len(c) >= 2 {
					cids = append(cids, uint16(c[len(c)-2])<<8|uint16(c[len(c)-1]))
				}
			}
		}

		return ensureValidUTF8(decodedStr), cids, stats
	}

	if font != nil && (font.Subtype == "/Type0" || font.Subtype == "Type0") && font.IsIdentity && len(rawBytes) >= 2 && len(rawBytes)%2 == 0 {
		for i := 0; i < len(rawBytes); i += 2 {
			cid := uint16(rawBytes[i])<<8 | uint16(rawBytes[i+1])
			cids = append(cids, cid)
		}
	} else {
		for _, b := range rawBytes {
			cids = append(cids, uint16(b))
		}
	}
	stats := TextDecodeStats{CIDCount: len(cids)}

	if toUnicodeMap != nil || (font != nil && len(font.CodeToGlyphName) > 0) || (font != nil && font.IsIdentity) {
		var decoded strings.Builder
		for _, cid := range cids {
			if toUnicodeMap != nil {
				if uni, ok := toUnicodeMap.MapCIDToUnicode(cid); ok {
					if isValidUnicodeRune(uni) {
						decoded.WriteRune(uni)
						stats.ToUnicodeHit++
						continue
					}
				}
			}

			if font != nil && len(font.CodeToGlyphName) > 0 {
				if name, ok := font.CodeToGlyphName[byte(cid&0xFF)]; ok {
					if r, ok := glyphNameToRuneForFont(name, font); ok {
						decoded.WriteRune(r)
						stats.GlyphNameHit++
						continue
					}
				}
			}

			if font != nil {
				if r, ok := texMathRuneFromCID(font.BaseFont, cid); ok {
					decoded.WriteRune(r)
					stats.GlyphNameHit++
					continue
				}
			}

			if font != nil && font.IsIdentity && (font.Subtype == "/Type0" || font.Subtype == "Type0") {
				gid := font.CIDToGID(cid)
				if r, ok := font.gidToUnicodeFromEmbeddedFont(gid); ok && isValidUnicodeRune(r) {
					decoded.WriteRune(r)
					continue
				}
				if font.CFF != nil {
					if name := font.CFF.GlyphName(otapi.GID(gid)); name != "" {
						if r, ok := glyphNameToRuneForFont(name, font); ok && isValidUnicodeRune(r) {
							decoded.WriteRune(r)
							stats.GlyphNameHit++
							continue
						}
					}
				}
				if cid >= 0x20 && cid <= 0x7E {
					decoded.WriteByte(byte(cid))
					stats.IdentityASCIIHit++
					continue
				}
				if lo := cid & 0x00FF; lo >= 0x20 && lo <= 0x7E {
					decoded.WriteByte(byte(lo))
					stats.IdentityASCIIHit++
					continue
				}
				decoded.WriteRune('�')
				stats.Replaced++
				continue
			}

			if font != nil && font.IsIdentity {
				uni := rune(cid)
				if isValidUnicodeRune(uni) {
					decoded.WriteRune(uni)
					stats.IdentityASCIIHit++
					continue
				}
				decoded.WriteRune('�')
				stats.Replaced++
				continue
			}

			decoded.WriteRune(rune(cid))
		}
		decodedText := decoded.String()
		if font != nil {
			decodedText = texMathNormalizeDecodedText(font.BaseFont, decodedText)
		}
		return ensureValidUTF8(decodedText), cids, stats
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
			return ensureValidUTF8(decoded.String()), cids, stats
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
				stats.Replaced++
			}
		}
		return ensureValidUTF8(decoded.String()), cids, stats
	}

	return latin1StringFromBytes(rawBytes), cids, stats
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
// decodeTextStringWithFontAndIdentity is currently unused
func _(text string, toUnicodeMap *CIDToUnicodeMap, isIdentity bool) string {
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

func decodePDFHexStringBytes(s string) []byte {
	if len(s) < 2 || s[0] != '<' {
		return nil
	}
	var out []byte
	i := 0
	for i < len(s) {
		if s[i] != '<' {
			i++
			continue
		}
		j := i + 1
		for j < len(s) && s[j] != '>' {
			j++
		}
		if j >= len(s) {
			break
		}
		seg := strings.ReplaceAll(s[i+1:j], " ", "")
		if seg != "" {
			if len(seg)%2 == 1 {
				seg += "0"
			}
			if b, err := hex.DecodeString(seg); err == nil {
				out = append(out, b...)
			}
		}
		i = j + 1
	}
	return out
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

		if len(hexStr)%2 == 1 {
			hexStr += "0"
		}

		result, err := hex.DecodeString(hexStr)
		if err != nil {
			return ""
		}

		if len(result) >= 2 && len(result)%2 == 0 && result[0] == 0xFE && result[1] == 0xFF {
			utf16Bytes := result[2:]
			var runes []rune
			for i := 0; i+1 < len(utf16Bytes); i += 2 {
				r := rune(utf16Bytes[i])<<8 | rune(utf16Bytes[i+1])
				if r != 0 {
					runes = append(runes, r)
				}
			}
			return string(runes)
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
	name := stripSubsetPrefix(pdfFont)
	name = strings.TrimSpace(strings.TrimPrefix(name, "/"))
	if name == "" {
		return "sans-serif"
	}

	lower := strings.ToLower(name)

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
	if mapped, ok := fontMap[name]; ok {
		return mapped
	}

	for _, k := range []string{
		"courier", "consola", "menlo", "monaco", "dejavusansmono",
		"sourcecode", "source code", "firacode", "fira code",
		"jetbrainsmono", "jetbrains mono", "cascadiacode", "cascadia code", "cascadiamono", "cascadia mono",
		"inconsolata", "ubuntumono", "ubuntu mono", "liberationmono", "liberation mono",
		"noto sans mono", "notosansmono", "sfmono", "sf mono",
	} {
		if strings.Contains(lower, strings.ReplaceAll(k, " ", "")) || strings.Contains(strings.ReplaceAll(lower, " ", ""), strings.ReplaceAll(k, " ", "")) {
			return "monospace"
		}
		if strings.Contains(lower, k) {
			return "monospace"
		}
	}

	for _, k := range []string{"times", "georgia", "serif", "garamond", "minion", "palatino"} {
		if strings.Contains(lower, k) {
			return "serif"
		}
	}

	for _, k := range []string{
		"lmroman", "latinmodernroman", "latin modern roman", "latin modern",
		"cmr", "computer modern", "computermodern",
	} {
		if strings.Contains(lower, strings.ReplaceAll(k, " ", "")) || strings.Contains(strings.ReplaceAll(lower, " ", ""), strings.ReplaceAll(k, " ", "")) {
			return "serif"
		}
		if strings.Contains(lower, k) {
			return "serif"
		}
	}

	for _, k := range []string{
		"lmtypewriter", "lm typewriter", "typewriter", "latinmodernmono", "latin modern mono",
		"cmtt", "computer modern typewriter", "computermoderntypewriter",
	} {
		if strings.Contains(lower, strings.ReplaceAll(k, " ", "")) || strings.Contains(strings.ReplaceAll(lower, " ", ""), strings.ReplaceAll(k, " ", "")) {
			return "monospace"
		}
		if strings.Contains(lower, k) {
			return "monospace"
		}
	}

	for _, k := range []string{"helvetica", "arial", "calibri", "segoe", "roboto", "notosans", "noto sans", "opensans", "open sans", "dejavusans", "dejavu sans", "sans"} {
		if strings.Contains(lower, k) {
			return "sans-serif"
		}
	}

	return "sans-serif"
}
