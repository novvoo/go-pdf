package gopdf

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"golang.org/x/image/font/sfnt"
)

type psType42FontSpec struct {
	PSName      string
	AliasNames  []string
	TTFPath     string
	EncodingTag string
	ByteToRune  map[byte]rune
}

type psByteRunePair struct {
	B byte
	R rune
}

func psMathEncodingPairs() []psByteRunePair {
	return []psByteRunePair{
		{0x80, 0x03B1},
		{0x81, 0x03B2},
		{0x82, 0x03B3},
		{0x83, 0x03B4},
		{0x84, 0x03B5},
		{0x85, 0x03B6},
		{0x86, 0x03B7},
		{0x87, 0x03B8},
		{0x88, 0x03B9},
		{0x89, 0x03BA},
		{0x8A, 0x03BB},
		{0x8B, 0x03BC},
		{0x8C, 0x03BD},
		{0x8D, 0x03BE},
		{0x8E, 0x03BF},
		{0x8F, 0x03C0},
		{0x90, 0x03C1},
		{0x91, 0x03C3},
		{0x92, 0x03C2},
		{0x93, 0x03C4},
		{0x94, 0x03C5},
		{0x95, 0x03C6},
		{0x96, 0x03C7},
		{0x97, 0x03C8},
		{0x98, 0x03C9},
		{0x99, 0x03F5},
		{0x9A, 0x03D1},
		{0x9B, 0x03F1},
		{0x9C, 0x03D5},
		{0x9D, 0x0393},
		{0x9E, 0x0394},
		{0x9F, 0x0398},
		{0xA0, 0x039B},
		{0xA1, 0x03A0},
		{0xA2, 0x03A3},
		{0xA3, 0x03A6},
		{0xA4, 0x03A8},
		{0xA5, 0x03A9},
		{0xA6, 0x1D53C},
		{0xA7, 0x2113},
		{0xA8, 0x2200},
		{0xA9, 0x2203},
		{0xAA, 0x2204},
		{0xAB, 0x2205},
		{0xAC, 0x2202},
		{0xAD, 0x2207},
		{0xAE, 0x2211},
		{0xAF, 0x220F},
		{0xB0, 0x222B},
		{0xB1, 0x222C},
		{0xB2, 0x222E},
		{0xB3, 0x221A},
		{0xB4, 0x221E},
		{0xB5, 0x221D},
		{0xB6, 0x22C5},
		{0xB7, 0x00B1},
		{0xB8, 0x2212},
		{0xB9, 0x00D7},
		{0xBA, 0x00F7},
		{0xBB, 0x2260},
		{0xBC, 0x2264},
		{0xBD, 0x2265},
		{0xBE, 0x2261},
		{0xBF, 0x2248},
		{0xC0, 0x2243},
		{0xC1, 0x2245},
		{0xC2, 0x2208},
		{0xC3, 0x2209},
		{0xC4, 0x2227},
		{0xC5, 0x2228},
		{0xC6, 0x2229},
		{0xC7, 0x222A},
		{0xC8, 0x2282},
		{0xC9, 0x2283},
		{0xCA, 0x2286},
		{0xCB, 0x2287},
		{0xCC, 0x2295},
		{0xCD, 0x2296},
		{0xCE, 0x2297},
		{0xCF, 0x2299},
		{0xD0, 0x22A5},
		{0xD1, 0x2220},
		{0xD2, 0x2190},
		{0xD3, 0x2192},
		{0xD4, 0x2191},
		{0xD5, 0x2193},
		{0xD6, 0x2194},
		{0xD7, 0x21A6},
		{0xD8, 0x21D0},
		{0xD9, 0x21D2},
		{0xDA, 0x21D4},
		{0xDB, 0x223C},
		{0xDC, 0x2234},
		{0xDD, 0x2235},
		{0xDE, 0x2022},
		{0xDF, 0x2032},
		{0xE0, 0x2033},
		{0xE1, 0x2034},
		{0xE2, 0x2057},
	}
}

func psDefaultMathByteToRune() map[byte]rune {
	m := map[byte]rune{}
	for by := byte(0x20); by <= 0x7E; by++ {
		m[by] = rune(by)
	}
	for _, p := range psMathEncodingPairs() {
		m[p.B] = p.R
	}
	return m
}

func psDefaultMathRuneToByte() map[rune]byte {
	m := map[rune]byte{}
	for by := byte(0x20); by <= 0x7E; by++ {
		m[rune(by)] = by
	}
	for _, p := range psMathEncodingPairs() {
		m[p.R] = p.B
	}
	return m
}

func psDefaultLatinByteToRune() map[byte]rune {
	m := map[byte]rune{}
	for by := byte(0x20); by <= 0x7E; by++ {
		m[by] = rune(by)
	}
	m[0x95] = 0x2022
	return m
}

func psTryWriteType42FontAliases(w *bufio.Writer, specs []psType42FontSpec) bool {
	any := false
	for _, spec := range specs {
		if spec.PSName == "" || len(spec.AliasNames) == 0 || spec.TTFPath == "" {
			continue
		}
		data, err := os.ReadFile(spec.TTFPath)
		if err != nil {
			continue
		}
		if !psLooksLikeTrueType(data) {
			continue
		}
		if err := psWriteType42Font(w, spec.PSName, spec.EncodingTag, data, spec.ByteToRune); err != nil {
			continue
		}
		okAliases := true
		for _, alias := range spec.AliasNames {
			alias = strings.TrimSpace(alias)
			if alias == "" {
				continue
			}
			if _, err := fmt.Fprintf(w, "/%s findfont /%s exch definefont pop\n", spec.PSName, alias); err != nil {
				okAliases = false
				break
			}
		}
		if !okAliases {
			continue
		}
		any = true
	}
	return any
}

func psLooksLikeTrueType(data []byte) bool {
	if len(data) < 4 {
		return false
	}
	if data[0] == 0x00 && data[1] == 0x01 && data[2] == 0x00 && data[3] == 0x00 {
		return true
	}
	if string(data[:4]) == "true" {
		return true
	}
	return false
}

func psWriteType42Font(w *bufio.Writer, psName string, encodingTag string, ttf []byte, byteToRune map[byte]rune) error {
	parsed, err := sfnt.Parse(ttf)
	if err != nil {
		return err
	}
	buf := &sfnt.Buffer{}

	encName := psName + "_Enc"
	if encodingTag != "" {
		encName = psName + "_" + encodingTag + "_Enc"
	}

	if _, err := fmt.Fprintf(w, "/%s 256 array def\n", encName); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(w, "0 1 255 { %s exch /.notdef put } for\n", encName); err != nil {
		return err
	}

	charStrings := make(map[string]uint16, 256)
	for i := 0; i < 256; i++ {
		by := byte(i)
		r, ok := byteToRune[by]
		if !ok {
			continue
		}
		gi, err := parsed.GlyphIndex(buf, r)
		if err != nil {
			continue
		}
		if gi == 0 {
			continue
		}
		name := fmt.Sprintf("g%d", i)
		charStrings[name] = uint16(gi)
		if _, err := fmt.Fprintf(w, "%s %d /%s put\n", encName, i, name); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(w, "/%s 128 dict dup begin\n", psName); err != nil {
		return err
	}
	units := parsed.UnitsPerEm()
	if units == 0 {
		units = 2048
	}
	scale := 1.0 / float64(units)
	if _, err := fmt.Fprintf(w, "/FontType 42 def\n/FontName /%s def\n/FontMatrix [%.10f 0 0 %.10f 0 0] def\n/FontBBox [-2048 -2048 4096 4096] def\n/Encoding %s def\n", psName, scale, scale, encName); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "/CharStrings %d dict dup begin\n/.notdef 0 def\n", len(charStrings)+1); err != nil {
		return err
	}
	keys := make([]string, 0, len(charStrings))
	for k := range charStrings {
		keys = append(keys, k)
	}
	sortStrings(keys)
	for _, k := range keys {
		if _, err := fmt.Fprintf(w, "/%s %d def\n", k, charStrings[k]); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "end readonly def\n"); err != nil {
		return err
	}

	if _, err := fmt.Fprintf(w, "/sfnts [\n"); err != nil {
		return err
	}
	const chunkSize = 16384
	for off := 0; off < len(ttf); off += chunkSize {
		end := off + chunkSize
		if end > len(ttf) {
			end = len(ttf)
		}
		seg := ttf[off:end]
		hexStr := strings.ToUpper(hex.EncodeToString(seg))
		if _, err := fmt.Fprintf(w, "<%s>\n", hexStr); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(w, "] def\nend\n/%s exch definefont pop\n", psName); err != nil {
		return err
	}
	return nil
}

func sortStrings(a []string) {
	if len(a) < 2 {
		return
	}
	for i := 0; i < len(a)-1; i++ {
		for j := i + 1; j < len(a); j++ {
			if strings.Compare(a[i], a[j]) > 0 {
				a[i], a[j] = a[j], a[i]
			}
		}
	}
}
