package gopdf

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"sort"
	"strconv"
	"unicode/utf16"
)

type toUnicodeCMap struct {
	mapping        map[string]string
	codeSpaceLens  []int
	codeSpaceMax   int
	codeSpaceReady bool
}

func parseToUnicodeCMapBytes(b []byte) (*toUnicodeCMap, error) {
	lx := &cmapLexer{b: b}
	out := &toUnicodeCMap{mapping: map[string]string{}}

	var lastInt int
	for {
		tok, ok, err := lx.next()
		if err != nil {
			return nil, err
		}
		if !ok {
			break
		}

		switch tok.kind {
		case cmapTokInt:
			lastInt, _ = strconv.Atoi(tok.s)
		case cmapTokKeyword:
			switch tok.s {
			case "begincodespacerange":
				if err := out.readCodeSpaceRanges(lx, lastInt); err != nil {
					return nil, err
				}
			case "beginbfchar":
				if err := out.readBFChar(lx, lastInt); err != nil {
					return nil, err
				}
			case "beginbfrange":
				if err := out.readBFRange(lx, lastInt); err != nil {
					return nil, err
				}
			}
		}
	}

	out.finalizeCodeSpace()
	return out, nil
}

func (m *toUnicodeCMap) finalizeCodeSpace() {
	if m.codeSpaceReady {
		return
	}
	if len(m.codeSpaceLens) == 0 {
		m.codeSpaceLens = []int{2, 1}
		m.codeSpaceMax = 2
		m.codeSpaceReady = true
		return
	}
	sort.Ints(m.codeSpaceLens)
	dedup := make([]int, 0, len(m.codeSpaceLens))
	last := -1
	for _, v := range m.codeSpaceLens {
		if v <= 0 {
			continue
		}
		if v != last {
			dedup = append(dedup, v)
			last = v
		}
	}
	m.codeSpaceLens = dedup
	m.codeSpaceMax = m.codeSpaceLens[len(m.codeSpaceLens)-1]
	m.codeSpaceReady = true
}

func (m *toUnicodeCMap) readCodeSpaceRanges(lx *cmapLexer, n int) error {
	for i := 0; i < n; i++ {
		a, ok, err := lx.next()
		if err != nil {
			return err
		}
		if !ok || a.kind != cmapTokHex {
			return fmt.Errorf("cmap: expected hex start for codespace range")
		}
		b, ok, err := lx.next()
		if err != nil {
			return err
		}
		if !ok || b.kind != cmapTokHex {
			return fmt.Errorf("cmap: expected hex end for codespace range")
		}
		if len(a.b) > 0 {
			m.codeSpaceLens = append(m.codeSpaceLens, len(a.b))
		}
	}
	for {
		t, ok, err := lx.peek()
		if err != nil {
			return err
		}
		if !ok || t.kind != cmapTokKeyword || t.s != "endcodespacerange" {
			break
		}
		_, _, _ = lx.next()
		break
	}
	return nil
}

func (m *toUnicodeCMap) readBFChar(lx *cmapLexer, n int) error {
	for i := 0; i < n; i++ {
		src, ok, err := lx.next()
		if err != nil {
			return err
		}
		if !ok || src.kind != cmapTokHex {
			return fmt.Errorf("cmap: expected hex src in bfchar")
		}
		dst, ok, err := lx.next()
		if err != nil {
			return err
		}
		if !ok || dst.kind != cmapTokHex {
			return fmt.Errorf("cmap: expected hex dst in bfchar")
		}
		m.mapping[string(src.b)] = decodeUnicodeBytes(dst.b)
	}
	for {
		t, ok, err := lx.peek()
		if err != nil {
			return err
		}
		if !ok || t.kind != cmapTokKeyword || t.s != "endbfchar" {
			break
		}
		_, _, _ = lx.next()
		break
	}
	return nil
}

func (m *toUnicodeCMap) readBFRange(lx *cmapLexer, n int) error {
	for i := 0; i < n; i++ {
		start, ok, err := lx.next()
		if err != nil {
			return err
		}
		if !ok || start.kind != cmapTokHex {
			return fmt.Errorf("cmap: expected hex start in bfrange")
		}
		end, ok, err := lx.next()
		if err != nil {
			return err
		}
		if !ok || end.kind != cmapTokHex {
			return fmt.Errorf("cmap: expected hex end in bfrange")
		}

		dst, ok, err := lx.next()
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("cmap: expected dst in bfrange")
		}

		switch dst.kind {
		case cmapTokHex:
			if err := m.applyRangeSingle(start.b, end.b, dst.b); err != nil {
				return err
			}
		case cmapTokLBracket:
			if err := m.applyRangeArray(lx, start.b, end.b); err != nil {
				return err
			}
		default:
			return fmt.Errorf("cmap: unsupported bfrange dst kind: %v", dst.kind)
		}
	}
	for {
		t, ok, err := lx.peek()
		if err != nil {
			return err
		}
		if !ok || t.kind != cmapTokKeyword || t.s != "endbfrange" {
			break
		}
		_, _, _ = lx.next()
		break
	}
	return nil
}

func (m *toUnicodeCMap) applyRangeSingle(start, end, dst []byte) error {
	if len(start) != len(end) {
		return nil
	}
	startN := bytesToUint(start)
	endN := bytesToUint(end)
	if endN < startN {
		return nil
	}

	dstDecoded := decodeUnicodeBytes(dst)
	if r := []rune(dstDecoded); len(r) == 1 {
		base := r[0]
		for i := uint32(0); i <= endN-startN; i++ {
			k := make([]byte, len(start))
			uintToBytes(startN+i, k)
			m.mapping[string(k)] = string(base + rune(i))
		}
		return nil
	}

	for i := uint32(0); i <= endN-startN; i++ {
		k := make([]byte, len(start))
		uintToBytes(startN+i, k)
		m.mapping[string(k)] = dstDecoded
	}
	return nil
}

func (m *toUnicodeCMap) applyRangeArray(lx *cmapLexer, start, end []byte) error {
	if len(start) != len(end) {
		return nil
	}
	startN := bytesToUint(start)
	endN := bytesToUint(end)
	if endN < startN {
		return nil
	}
	expect := int(endN-startN) + 1
	for i := 0; i < expect; i++ {
		t, ok, err := lx.next()
		if err != nil {
			return err
		}
		if !ok {
			return fmt.Errorf("cmap: unexpected EOF in bfrange array")
		}
		if t.kind != cmapTokHex {
			return fmt.Errorf("cmap: expected hex in bfrange array")
		}
		k := make([]byte, len(start))
		uintToBytes(startN+uint32(i), k)
		m.mapping[string(k)] = decodeUnicodeBytes(t.b)
	}

	t, ok, err := lx.next()
	if err != nil {
		return err
	}
	if !ok || t.kind != cmapTokRBracket {
		return fmt.Errorf("cmap: expected ] to close bfrange array")
	}
	return nil
}

func (m *toUnicodeCMap) decodeFontString(raw []byte) string {
	m.finalizeCodeSpace()
	if len(raw) == 0 {
		return ""
	}

	lens := append([]int(nil), m.codeSpaceLens...)
	sort.Sort(sort.Reverse(sort.IntSlice(lens)))

	var sb bytes.Buffer
	i := 0
	for i < len(raw) {
		matched := false
		for _, l := range lens {
			if l <= 0 || i+l > len(raw) {
				continue
			}
			if s, ok := m.mapping[string(raw[i:i+l])]; ok {
				sb.WriteString(s)
				i += l
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		i++
	}
	return sb.String()
}

func (m *toUnicodeCMap) splitCodes(raw []byte) (codes [][]byte) {
	m.finalizeCodeSpace()
	if len(raw) == 0 {
		return nil
	}
	lens := append([]int(nil), m.codeSpaceLens...)
	sort.Sort(sort.Reverse(sort.IntSlice(lens)))
	i := 0
	for i < len(raw) {
		matched := false
		for _, l := range lens {
			if l <= 0 || i+l > len(raw) {
				continue
			}
			if _, ok := m.mapping[string(raw[i:i+l])]; ok {
				c := append([]byte(nil), raw[i:i+l]...)
				codes = append(codes, c)
				i += l
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		codes = append(codes, raw[i:i+1])
		i++
	}
	return codes
}

type cmapTokKind int

const (
	cmapTokInt cmapTokKind = iota + 1
	cmapTokHex
	cmapTokKeyword
	cmapTokLBracket
	cmapTokRBracket
)

type cmapTok struct {
	kind cmapTokKind
	s    string
	b    []byte
}

type cmapLexer struct {
	b      []byte
	i      int
	peeked *cmapTok
}

func (l *cmapLexer) peek() (cmapTok, bool, error) {
	if l.peeked != nil {
		return *l.peeked, true, nil
	}
	t, ok, err := l.next()
	if err != nil {
		return cmapTok{}, false, err
	}
	if !ok {
		return cmapTok{}, false, nil
	}
	l.peeked = &t
	return t, true, nil
}

func (l *cmapLexer) next() (cmapTok, bool, error) {
	if l.peeked != nil {
		t := *l.peeked
		l.peeked = nil
		return t, true, nil
	}

	l.skipWSAndComments()
	if l.i >= len(l.b) {
		return cmapTok{}, false, nil
	}

	ch := l.b[l.i]
	switch {
	case ch == '[':
		l.i++
		return cmapTok{kind: cmapTokLBracket, s: "["}, true, nil
	case ch == ']':
		l.i++
		return cmapTok{kind: cmapTokRBracket, s: "]"}, true, nil
	case ch == '<':
		if l.i+1 < len(l.b) && l.b[l.i+1] == '<' {
			l.i += 2
			return cmapTok{kind: cmapTokKeyword, s: "<<"}, true, nil
		}
		return l.readHex()
	case ch == '>':
		if l.i+1 < len(l.b) && l.b[l.i+1] == '>' {
			l.i += 2
			return cmapTok{kind: cmapTokKeyword, s: ">>"}, true, nil
		}
		return l.readKeyword()
	case ch == '+' || ch == '-' || (ch >= '0' && ch <= '9'):
		return l.readIntOrKeyword()
	default:
		return l.readKeyword()
	}
}

func (l *cmapLexer) skipWSAndComments() {
	for l.i < len(l.b) {
		ch := l.b[l.i]
		if ch == '%' {
			for l.i < len(l.b) && l.b[l.i] != '\n' && l.b[l.i] != '\r' {
				l.i++
			}
			continue
		}
		if ch == 0x00 || ch == 0x09 || ch == 0x0A || ch == 0x0D || ch == 0x20 {
			l.i++
			continue
		}
		break
	}
}

func (l *cmapLexer) readHex() (cmapTok, bool, error) {
	if l.b[l.i] != '<' {
		return cmapTok{}, false, fmt.Errorf("cmap: expected <")
	}
	l.i++
	start := l.i
	for l.i < len(l.b) && l.b[l.i] != '>' {
		l.i++
	}
	if l.i >= len(l.b) {
		return cmapTok{}, false, fmt.Errorf("cmap: unterminated hex string")
	}
	raw := bytes.ReplaceAll(l.b[start:l.i], []byte(" "), nil)
	l.i++

	if len(raw)%2 == 1 {
		raw = append(raw, '0')
	}
	decoded := make([]byte, hex.DecodedLen(len(raw)))
	n, err := hex.Decode(decoded, raw)
	if err != nil {
		return cmapTok{}, false, err
	}
	decoded = decoded[:n]
	return cmapTok{kind: cmapTokHex, b: decoded}, true, nil
}

func (l *cmapLexer) readIntOrKeyword() (cmapTok, bool, error) {
	start := l.i
	if l.b[l.i] == '+' || l.b[l.i] == '-' {
		l.i++
	}
	for l.i < len(l.b) && l.b[l.i] >= '0' && l.b[l.i] <= '9' {
		l.i++
	}
	s := string(l.b[start:l.i])
	if s == "+" || s == "-" {
		return l.readKeywordFrom(start)
	}
	return cmapTok{kind: cmapTokInt, s: s}, true, nil
}

func (l *cmapLexer) readKeyword() (cmapTok, bool, error) {
	start := l.i
	for l.i < len(l.b) {
		ch := l.b[l.i]
		if ch == 0x00 || ch == 0x09 || ch == 0x0A || ch == 0x0D || ch == 0x20 || ch == '<' || ch == '[' || ch == ']' || ch == '>' {
			break
		}
		l.i++
	}
	return cmapTok{kind: cmapTokKeyword, s: string(l.b[start:l.i])}, true, nil
}

func (l *cmapLexer) readKeywordFrom(start int) (cmapTok, bool, error) {
	l.i = start
	return l.readKeyword()
}

func decodeUnicodeBytes(b []byte) string {
	if len(b) == 0 {
		return ""
	}
	if len(b)%2 != 0 {
		out := make([]rune, 0, len(b))
		for _, by := range b {
			out = append(out, rune(by))
		}
		return string(out)
	}
	if len(b) >= 2 && b[0] == 0xFE && b[1] == 0xFF {
		b = b[2:]
	}
	if len(b) == 0 {
		return ""
	}
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		u = append(u, uint16(b[i])<<8|uint16(b[i+1]))
	}
	return string(utf16.Decode(u))
}

func bytesToUint(b []byte) uint32 {
	var v uint32
	for _, by := range b {
		v = (v << 8) | uint32(by)
	}
	return v
}

func uintToBytes(v uint32, out []byte) {
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = byte(v & 0xFF)
		v >>= 8
	}
}
