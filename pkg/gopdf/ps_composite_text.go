package gopdf

import (
	"encoding/hex"
	"fmt"
	"strings"
	"unicode"
	"unicode/utf16"
)

func pangoPSShowTextComposite(c *context, layout *PangoPdfLayout) {
	if c == nil || layout == nil || layout.fontDesc == nil {
		return
	}

	x, y := c.GetCurrentPoint()
	if x == 0 && y == 0 && c.HasCurrentPoint() == False {
		x, y = 0, 0
	}

	fontSize := layout.fontDesc.size
	if fontSize <= 0 {
		fontSize = 12
	}

	c.psApplySourceColor()

	text := ensureValidUTF8(layout.GetText())
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return
	}

	lineHeight := fontSize * 1.2
	currentY := y
	for _, line := range lines {
		if line == "" {
			currentY += lineHeight
			continue
		}

		lineX := x
		if layout.align != PangoAlignLeft && layout.width > 0 {
			layoutWidth := float64(layout.width) / 1024.0
			textWidth := estimateTextWidthSimple(line, fontSize)
			switch layout.align {
			case PangoAlignRight:
				lineX = x + (layoutWidth - textWidth)
			case PangoAlignCenter:
				lineX = x + (layoutWidth-textWidth)/2
			}
		}

		c.psWritef("%.4f %.4f moveto\n", lineX, currentY)

		fontName := "Helvetica"
		if family := strings.TrimSpace(layout.fontDesc.family); family != "" {
			fontName = psSafeFontName(family)
		}
		if strings.EqualFold(fontName, "symbol") {
			fontName = "math"
		}

		c.psLineAggAdd(lineX, currentY, fontSize, line)
		c.psWritef("%% utf8=%s\n", psEscapePSComment(line, 240))

		if psLineNeedsOutline(line) {
			if err := psShowUTF8TextAsOutline(c, layout, lineX, currentY, fontSize, line); err != nil {
				fallbackText := psASCIIShowFallbackText(line)
				if fallbackText == "" {
					fallbackText = "?"
				}
				c.psWritef("/Helvetica findfont %.4f scalefont setfont\n(%s) show\n", fontSize, escapePSString(fallbackText))
			}
			currentY += lineHeight
			continue
		}

		if composite := psPickCompositeFontName(line); composite != "" {
			if err := psShowUTF8TextAsOutline(c, layout, lineX, currentY, fontSize, line); err != nil {
				fallbackText := psASCIIShowFallbackText(line)
				if fallbackText == "" {
					fallbackText = "?"
				}
				c.psWritef("/Helvetica findfont %.4f scalefont setfont\n(%s) show\n", fontSize, escapePSString(fallbackText))
			}
			currentY += lineHeight
			continue
		}

		if strings.EqualFold(fontName, "math") {
			encoded := psEncodeWithRuneToByteMap(line, psDefaultMathRuneToByte())
			c.psWritef("/math findfont %.4f scalefont setfont\n", fontSize)
			c.psWritef("(%s) show\n", psEscapePSBytesToLiteralStringContent(encoded))
			currentY += lineHeight
			continue
		}

		c.psWritef("/%s findfont %.4f scalefont setfont\n", fontName, fontSize)
		if psIsASCIIOnly(line) {
			c.psWritef("(%s) show\n", escapePSString(line))
		} else {
			psWriteMixedTextWithSymbolFallback(c, fontName, fontSize, line)
		}
		currentY += lineHeight
	}

	lastLine := lines[len(lines)-1]
	if lastLine != "" {
		c.currentPoint.x = x + estimateTextWidthSimple(lastLine, fontSize)
		c.currentPoint.y = currentY - lineHeight
		c.currentPoint.hasPoint = true
	}
}

func psLineNeedsOutline(s string) bool {
	for _, r := range s {
		if unicode.IsSpace(r) {
			continue
		}
		if unicode.IsMark(r) {
			return true
		}
		if r >= 0x20 && r <= 0x7E {
			continue
		}
		switch r {
		case 0xFB00, 0xFB01, 0xFB02, 0xFB03, 0xFB04, 0xFB05, 0xFB06:
			continue
		case 0x2013, 0x2014, 0x2212:
			continue
		case 0x2018, 0x2019, 0x201C, 0x201D:
			continue
		}
		if _, ok := psDefaultMathRuneToByte()[r]; ok {
			continue
		}
		return true
	}
	return false
}

func psShowUTF8TextAsOutline(c *context, layout *PangoPdfLayout, x, y, fontSize float64, text string) error {
	if c == nil || layout == nil {
		return fmt.Errorf("nil context")
	}
	text = ensureValidUTF8(text)
	if strings.TrimSpace(text) == "" {
		return nil
	}

	family := strings.TrimSpace(layout.fontDesc.family)
	if family == "" {
		family = "sans"
	}

	switch psPickCompositeFontName(text) {
	case "GoPDF-GB":
		ensureRepoFontRegistered("cjk-regular", "fonts/ofl/zhimangxing/ZhiMangXing-Regular.ttf")
		family = "sans-cjk"
	case "GoPDF-JP":
		ensureRepoFontRegistered("cjk-regular", "fonts/apache/kosugi/Kosugi-Regular.ttf")
		family = "sans-cjk"
	case "GoPDF-KR":
		ensureRepoFontRegistered("cjk-regular", "fonts/apache/kosugi/Kosugi-Regular.ttf")
		family = "sans-cjk"
	}

	fontFace := NewPangoPdfFont(family, FontSlantNormal, FontWeightNormal)
	defer fontFace.Destroy()
	fontMatrix := NewMatrix()
	fontMatrix.InitScale(fontSize, fontSize)
	ctm := NewMatrix()
	ctm.InitIdentity()
	scaledFont := NewPangoPdfScaledFont(fontFace, fontMatrix, ctm, nil)
	defer scaledFont.Destroy()
	scaledFont.flipY = shouldFlipGlyphY(c)

	glyphs, _, _, status := scaledFont.TextToGlyphs(x, y, text)
	if status != StatusSuccess {
		return newError(status, "TextToGlyphs")
	}

	c.psApplySourceColor()
	for _, g := range glyphs {
		path, err := scaledFont.GlyphPath(g.Index)
		if err != nil || path == nil || path.Status != StatusSuccess {
			continue
		}
		if err := psWriteOutlinePathAt(c, path, g.X, g.Y); err != nil {
			return err
		}
		c.psWritef("fill\n")
	}

	return nil
}

func psWriteOutlinePathAt(c *context, p *Path, tx, ty float64) error {
	if c == nil || p == nil {
		return nil
	}
	c.psWritef("newpath\n")
	for _, d := range p.Data {
		switch d.Type {
		case PathMoveTo:
			if len(d.Points) >= 1 {
				c.psWritef("%.4f %.4f moveto\n", tx+d.Points[0].X, ty+d.Points[0].Y)
			}
		case PathLineTo:
			if len(d.Points) >= 1 {
				c.psWritef("%.4f %.4f lineto\n", tx+d.Points[0].X, ty+d.Points[0].Y)
			}
		case PathCurveTo:
			if len(d.Points) >= 3 {
				c.psWritef("%.4f %.4f %.4f %.4f %.4f %.4f curveto\n",
					tx+d.Points[0].X, ty+d.Points[0].Y,
					tx+d.Points[1].X, ty+d.Points[1].Y,
					tx+d.Points[2].X, ty+d.Points[2].Y,
				)
			}
		case PathClosePath:
			c.psWritef("closepath\n")
		}
	}
	return nil
}

func psEncodeWithRuneToByteMap(s string, runeToByte map[rune]byte) []byte {
	if s == "" {
		return nil
	}
	out := make([]byte, 0, len(s)+8)
	for _, r := range s {
		if r == 0 || r == '\uFFFD' {
			out = append(out, byte('?'))
			continue
		}
		if r == '\t' || r == '\n' || r == '\r' || unicode.IsSpace(r) {
			out = append(out, byte(' '))
			continue
		}
		if r >= 0x20 && r <= 0x7E {
			out = append(out, byte(r))
			continue
		}
		if by, ok := runeToByte[r]; ok {
			out = append(out, by)
			continue
		}
		out = append(out, byte('?'))
	}
	return out
}

func psIsASCIIOnly(s string) bool {
	for _, r := range s {
		if r > 0x7F {
			return false
		}
	}
	return true
}

func psMathFontNeedsMixedFallback(s string) bool {
	letters := 0
	runes := 0
	hasSpace := false
	for _, r := range s {
		runes++
		if unicode.IsSpace(r) {
			hasSpace = true
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			letters++
		}
	}
	if letters == 0 {
		return false
	}
	if letters >= 2 {
		return true
	}
	if hasSpace || runes > 2 {
		return true
	}
	return false
}

func psPickCompositeFontName(s string) string {
	for _, r := range s {
		if r <= 0x7F {
			continue
		}
		switch {
		case unicode.In(r, unicode.Han):
			return "GoPDF-GB"
		case unicode.In(r, unicode.Hiragana, unicode.Katakana):
			return "GoPDF-JP"
		case unicode.In(r, unicode.Hangul):
			return "GoPDF-KR"
		}
	}
	return ""
}

func psIsASCIICompatibleAfterMapping(s string) bool {
	for _, r := range s {
		switch {
		case r >= 0x20 && r <= 0x7E:
			continue
		case unicode.IsSpace(r):
			continue
		case r == 0x2022, r == 0x2212, r == 0x00D7, r == 0x00B7:
			continue
		case r == 0x2013, r == 0x2014:
			continue
		case r == 0x2018, r == 0x2019, r == 0x201C, r == 0x201D:
			continue
		default:
			return false
		}
	}
	return true
}

func psEncodeSymbolEncodingBytes(s string) ([]byte, bool) {
	out := make([]byte, 0, len(s))
	ok := true
	for _, r := range s {
		if r == 0 || r == '\uFFFD' {
			out = append(out, byte('?'))
			ok = false
			continue
		}
		if r == '\t' || r == '\n' || r == '\r' || unicode.IsSpace(r) {
			out = append(out, byte(' '))
			continue
		}
		if r >= 0x20 && r <= 0x7E {
			out = append(out, byte(r))
			continue
		}
		if by, ok := psUnicodeToSymbolEncodingByte(r); ok {
			out = append(out, by)
			continue
		}
		out = append(out, byte('?'))
		ok = false
	}
	return out, ok
}

func psUnicodeToSymbolEncodingByte(r rune) (byte, bool) {
	switch r {
	case 0x2208:
		return 0xCE, true
	case 0x2209:
		return 0xCF, true
	case 0x2264:
		return 0xA3, true
	case 0x2265:
		return 0xB3, true
	case 0x2260:
		return 0xB9, true
	case 0x00B1:
		return 0xB1, true
	case 0x00D7:
		return 0xB4, true
	case 0x00F7:
		return 0xB8, true
	case 0x2212:
		return 0x2D, true
	case 0x221A:
		return 0xD6, true
	case 0x221E:
		return 0xA5, true
	case 0x220F:
		return 0xD5, true
	case 0x2211:
		return 0xE5, true
	case 0x222B:
		return 0xF2, true
	case 0x22C5:
		return 0xD7, true
	case 0x2202:
		return 0xB6, true
	case 0x2207:
		return 0xD1, true
	case 0x2022:
		return 0xB7, true
	case 0x223C:
		return 0x7E, true
	case 0x2190:
		return 0xAC, true
	case 0x2191:
		return 0xAD, true
	case 0x2192:
		return 0xAE, true
	case 0x2193:
		return 0xAF, true
	case 0x2194:
		return 0xAB, true
	case 0xF8E5:
		return 0x60, true
	case 0x03F5:
		return 0x65, true
	case 0x03B1:
		return 0x61, true
	case 0x03B2:
		return 0x62, true
	case 0x03B3:
		return 0x67, true
	case 0x03B4:
		return 0x64, true
	case 0x03B5:
		return 0x65, true
	case 0x03B6:
		return 0x7A, true
	case 0x03B7:
		return 0x68, true
	case 0x03B8:
		return 0x71, true
	case 0x03B9:
		return 0x69, true
	case 0x03BA:
		return 0x6B, true
	case 0x03BB:
		return 0x6C, true
	case 0x03BC:
		return 0x6D, true
	case 0x03BD:
		return 0x6E, true
	case 0x03BE:
		return 0x78, true
	case 0x03BF:
		return 0x6F, true
	case 0x03C0:
		return 0x70, true
	case 0x03C1:
		return 0x72, true
	case 0x03C3:
		return 0x73, true
	case 0x03C4:
		return 0x74, true
	case 0x03C5:
		return 0x75, true
	case 0x03C6:
		return 0x66, true
	case 0x03C7:
		return 0x63, true
	case 0x03C8:
		return 0x79, true
	case 0x03C9:
		return 0x77, true
	case 0x0391:
		return 0x41, true
	case 0x0392:
		return 0x42, true
	case 0x0393:
		return 0x47, true
	case 0x0394:
		return 0x44, true
	case 0x0395:
		return 0x45, true
	case 0x0396:
		return 0x5A, true
	case 0x0397:
		return 0x48, true
	case 0x0398:
		return 0x51, true
	case 0x0399:
		return 0x49, true
	case 0x039A:
		return 0x4B, true
	case 0x039B:
		return 0x4C, true
	case 0x039C:
		return 0x4D, true
	case 0x039D:
		return 0x4E, true
	case 0x039E:
		return 0x58, true
	case 0x039F:
		return 0x4F, true
	case 0x03A0:
		return 0x50, true
	case 0x03A1:
		return 0x52, true
	case 0x03A3:
		return 0x53, true
	case 0x03A4:
		return 0x54, true
	case 0x03A5:
		return 0x55, true
	case 0x03A6:
		return 0x46, true
	case 0x03A7:
		return 0x43, true
	case 0x03A8:
		return 0x59, true
	case 0x03A9:
		return 0x57, true
	default:
		return 0, false
	}
}

func psEscapePSBytesToLiteralStringContent(b []byte) string {
	var sb strings.Builder
	for _, by := range b {
		switch by {
		case '\\':
			sb.WriteString("\\\\")
		case '(':
			sb.WriteString("\\(")
		case ')':
			sb.WriteString("\\)")
		case '\n':
			sb.WriteString("\\n")
		case '\r':
			sb.WriteString("\\r")
		case '\t':
			sb.WriteString("\\t")
		default:
			if by < 0x20 || by >= 0x7F {
				sb.WriteString(fmt.Sprintf("\\%03o", by))
			} else {
				sb.WriteByte(by)
			}
		}
	}
	return sb.String()
}

func psWriteMixedTextWithSymbolFallback(c *context, baseFont string, fontSize float64, s string) {
	if c == nil {
		return
	}
	seg := make([]byte, 0, len(s)+8)
	flushSeg := func() {
		if len(seg) == 0 {
			return
		}
		c.psWritef("(%s) show\n", psEscapePSBytesToLiteralStringContent(seg))
		seg = seg[:0]
	}

	writeSymbolByte := func(by byte) {
		flushSeg()
		c.psWritef("/math findfont %.4f scalefont setfont\n", fontSize)
		c.psWritef("(%s) show\n", psEscapePSBytesToLiteralStringContent([]byte{by}))
		c.psWritef("/%s findfont %.4f scalefont setfont\n", baseFont, fontSize)
	}

	for _, r := range s {
		switch r {
		case 0xFB00:
			seg = append(seg, 'f', 'f')
			continue
		case 0xFB01:
			seg = append(seg, 'f', 'i')
			continue
		case 0xFB02:
			seg = append(seg, 'f', 'l')
			continue
		case 0xFB03:
			seg = append(seg, 'f', 'f', 'i')
			continue
		case 0xFB04:
			seg = append(seg, 'f', 'f', 'l')
			continue
		case 0xFB05, 0xFB06:
			seg = append(seg, 's', 't')
			continue
		}
		if unicode.IsSpace(r) {
			seg = append(seg, ' ')
			continue
		}
		if r >= 0x20 && r <= 0x7E {
			seg = append(seg, byte(r))
			continue
		}
		if r == 0x2022 {
			seg = append(seg, 0x95)
			continue
		}
		if by, ok := psDefaultMathRuneToByte()[r]; ok {
			writeSymbolByte(by)
			continue
		}
		switch r {
		case 0x2013, 0x2014, 0x2212:
			seg = append(seg, '-')
		case 0x2018, 0x2019:
			seg = append(seg, '\'')
		case 0x201C, 0x201D:
			seg = append(seg, '"')
		case 0x2113:
			seg = append(seg, 'l')
		case 0x1D53C:
			seg = append(seg, 'E')
		default:
			seg = append(seg, '?')
		}
	}
	flushSeg()
}

func psASCIIShowFallbackText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
		case 0xFB00:
			b.WriteString("ff")
			continue
		case 0xFB01:
			b.WriteString("fi")
			continue
		case 0xFB02:
			b.WriteString("fl")
			continue
		case 0xFB03:
			b.WriteString("ffi")
			continue
		case 0xFB04:
			b.WriteString("ffl")
			continue
		case 0xFB05, 0xFB06:
			b.WriteString("st")
			continue
		case 0x2022:
			b.WriteString("- ")
			continue
		case 0x2212:
			b.WriteByte('-')
			continue
		case 0x00D7:
			b.WriteByte('x')
			continue
		case 0x00B7:
			b.WriteByte('.')
			continue
		case 0x2013, 0x2014:
			b.WriteByte('-')
			continue
		case 0x2018, 0x2019:
			b.WriteByte('\'')
			continue
		case 0x201C, 0x201D:
			b.WriteByte('"')
			continue
		case 0x2113:
			b.WriteByte('l')
			continue
		case 0x1D53C:
			b.WriteByte('E')
			continue
		}
		if r >= 0x20 && r <= 0x7E {
			b.WriteRune(r)
			continue
		}
		if unicode.IsSpace(r) {
			b.WriteByte(' ')
			continue
		}
		b.WriteByte('?')
	}
	return strings.TrimSpace(b.String())
}

func psUTF16HexStringWithBOM(s string) string {
	runes := []rune(s)
	u16 := utf16.Encode(runes)
	b := make([]byte, 0, 2+len(u16)*2)
	b = append(b, 0xFE, 0xFF)
	for _, v := range u16 {
		b = append(b, byte(v>>8), byte(v))
	}
	return "<" + strings.ToUpper(hex.EncodeToString(b)) + ">"
}

func psEscapePSComment(s string, maxRunes int) string {
	s = ensureValidUTF8(s)
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	s = strings.TrimSpace(s)
	r := []rune(s)
	if maxRunes > 0 && len(r) > maxRunes {
		s = string(r[:maxRunes]) + "…"
	}
	return s
}
