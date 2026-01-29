package gopdf

import "strings"

func glyphNameToRuneForFont(name string, font *Font) (rune, bool) {
	if font == nil {
		return glyphNameToRune(name)
	}
	base := strings.ToUpper(stripSubsetPrefix(font.BaseFont))
	if strings.HasPrefix(base, "MSBM") {
		if r, ok := msbmGlyphNameToRune(name); ok {
			return r, true
		}
	}
	return glyphNameToRune(name)
}

func msbmGlyphNameToRune(name string) (rune, bool) {
	name = strings.TrimPrefix(name, "/")
	if len(name) == 1 && name[0] >= 'A' && name[0] <= 'Z' {
		return rune(0x1D538 + int(name[0]-'A')), true
	}
	if len(name) == 1 && name[0] >= 'a' && name[0] <= 'z' {
		return rune(0x1D552 + int(name[0]-'a')), true
	}
	return 0, false
}

func msbmRuneFromCID(cid uint16) (rune, bool) {
	if cid >= 'A' && cid <= 'Z' {
		return rune(0x1D538 + int(cid-'A')), true
	}
	if cid >= 'a' && cid <= 'z' {
		return rune(0x1D552 + int(cid-'a')), true
	}
	return 0, false
}
