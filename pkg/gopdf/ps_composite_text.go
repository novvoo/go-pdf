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

		if psIsASCIIOnly(line) {
			fontName := "Helvetica"
			if family := strings.TrimSpace(layout.fontDesc.family); family != "" {
				fontName = psSafeFontName(family)
			}
			if strings.EqualFold(fontName, "math") || strings.EqualFold(fontName, "symbol") {
				fontName = "sans-serif"
			}
			escaped := escapePSString(line)
			c.psLineAggAddFromShowLine(lineX, currentY, fontSize, "("+escaped+") show")
			c.psWritef("%% utf8=%s\n", psEscapePSComment(line, 240))
			c.psWritef("/%s findfont %.4f scalefont setfont\n", fontName, fontSize)
			c.psWritef("(%s) show\n", escaped)
			currentY += lineHeight
			continue
		}

		compositeFont := psPickCompositeFontName(line)
		if compositeFont == "" {
			fontName := "Helvetica"
			if family := strings.TrimSpace(layout.fontDesc.family); family != "" {
				fontName = psSafeFontName(family)
			}
			c.psLineAggAdd(lineX, currentY, fontSize, line)
			c.psWritef("%% utf8=%s\n", psEscapePSComment(line, 240))
			if strings.EqualFold(fontName, "math") || strings.EqualFold(fontName, "symbol") {
				fontName = "sans-serif"
			}
			c.psWritef("/%s findfont %.4f scalefont setfont\n", fontName, fontSize)
			psWriteMixedTextWithSymbolFallback(c, fontName, fontSize, line)
			currentY += lineHeight
			continue
		}

		fontName := compositeFont
		if fontName == "" {
			fontName = "GoPDF-GB"
		}
		hexShow := psUTF16HexStringWithBOM(line)
		c.psLineAggAddFromShowLine(lineX, currentY, fontSize, hexShow+" show")
		fallbackText := psASCIIShowFallbackText(line)
		if fallbackText == "" {
			fallbackText = "?"
		}
		c.psWritef("%% utf8=%s\n", psEscapePSComment(line, 240))
		c.psWritef("/_GoPDF_HasCompositeFont true def\n")
		c.psWritef("{/%s findfont %.4f scalefont setfont} stopped { /Helvetica findfont %.4f scalefont setfont /_GoPDF_HasCompositeFont false def } if\n", fontName, fontSize, fontSize)
		c.psWritef("_GoPDF_HasCompositeFont { %s show } { (%s) show } ifelse\n", hexShow, escapePSString(fallbackText))
		currentY += lineHeight
	}

	lastLine := lines[len(lines)-1]
	if lastLine != "" {
		c.currentPoint.x = x + estimateTextWidthSimple(lastLine, fontSize)
		c.currentPoint.y = currentY - lineHeight
		c.currentPoint.hasPoint = true
	}
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
	var seg strings.Builder
	flushSeg := func() {
		if seg.Len() == 0 {
			return
		}
		c.psWritef("(%s) show\n", escapePSString(seg.String()))
		seg.Reset()
	}

	writeSymbolByte := func(by byte) {
		flushSeg()
		c.psWritef("/math findfont %.4f scalefont setfont\n", fontSize)
		c.psWritef("(%s) show\n", psEscapePSBytesToLiteralStringContent([]byte{by}))
		c.psWritef("/%s findfont %.4f scalefont setfont\n", baseFont, fontSize)
	}

	for _, r := range s {
		if unicode.IsSpace(r) {
			seg.WriteByte(' ')
			continue
		}
		if r >= 0x20 && r <= 0x7E {
			seg.WriteRune(r)
			continue
		}
		if by, ok := psUnicodeToSymbolEncodingByte(r); ok {
			writeSymbolByte(by)
			continue
		}
		switch r {
		case 0x2013, 0x2014, 0x2212:
			seg.WriteByte('-')
		case 0x2018, 0x2019:
			seg.WriteByte('\'')
		case 0x201C, 0x201D:
			seg.WriteByte('"')
		case 0x2113:
			seg.WriteByte('l')
		case 0x1D53C:
			seg.WriteByte('E')
		default:
			seg.WriteByte('?')
		}
	}
	flushSeg()
}

func psASCIIShowFallbackText(s string) string {
	var b strings.Builder
	for _, r := range s {
		switch r {
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
