package gopdf

import "unicode/utf8"

func decodePDFLiteralStringBytes(s string) []byte {
	if s == "" {
		return nil
	}
	out := make([]byte, 0, len(s))
	for i := 0; i < len(s); i++ {
		b := s[i]
		if b != '\\' {
			out = append(out, b)
			continue
		}
		if i+1 >= len(s) {
			break
		}
		i++
		esc := s[i]
		switch esc {
		case 'n':
			out = append(out, '\n')
		case 'r':
			out = append(out, '\r')
		case 't':
			out = append(out, '\t')
		case 'b':
			out = append(out, '\b')
		case 'f':
			out = append(out, '\f')
		case '\\', '(', ')':
			out = append(out, esc)
		case '\n':
		case '\r':
			if i+1 < len(s) && s[i+1] == '\n' {
				i++
			}
		case '0', '1', '2', '3', '4', '5', '6', '7':
			val := int(esc - '0')
			digits := 1
			for digits < 3 && i+1 < len(s) {
				next := s[i+1]
				if next < '0' || next > '7' {
					break
				}
				i++
				val = val*8 + int(next-'0')
				digits++
			}
			out = append(out, byte(val&0xFF))
		default:
			out = append(out, esc)
		}
	}
	return out
}

func latin1StringFromBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	runes := make([]rune, 0, len(b))
	for _, bb := range b {
		runes = append(runes, rune(bb))
	}
	return string(runes)
}

func ensureValidUTF8(s string) string {
	if utf8.ValidString(s) {
		return s
	}
	b := []byte(s)
	runes := make([]rune, 0, len(b))
	for len(b) > 0 {
		r, size := utf8.DecodeRune(b)
		if r == utf8.RuneError && size == 1 {
			runes = append(runes, rune(b[0]))
			b = b[1:]
			continue
		}
		runes = append(runes, r)
		b = b[size:]
	}
	return string(runes)
}

