package gopdf

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/font"
	"github.com/go-text/typesetting/shaping"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/math/fixed"
)

// Font cache to avoid re-parsing fonts
var (
	fontCache     = make(map[string]font.Face)
	fontDataCache = make(map[string][]byte)
	fontCacheMu   sync.RWMutex
)

func RegisterFontData(key string, data []byte) error {
	if key == "" || len(data) == 0 {
		return fmt.Errorf("invalid font registration")
	}

	fontCacheMu.RLock()
	if _, ok := fontCache[key]; ok {
		fontCacheMu.RUnlock()
		return nil
	}
	fontCacheMu.RUnlock()

	face, err := parseFontData(data)
	if err != nil {
		return err
	}

	fontCacheMu.Lock()
	fontCache[key] = face
	fontDataCache[key] = data
	fontCacheMu.Unlock()

	return nil
}

// Internal font data storage
var embeddedFonts = map[string][]byte{
	"Go-Regular":       goregular.TTF,
	"Go-Bold":          gobold.TTF,
	"Go-Italic":        goitalic.TTF,
	"Go-BoldItalic":    gobolditalic.TTF,
	"sans-regular":     goregular.TTF, // Will try DejaVuSans from assets first
	"sans-bold":        gobold.TTF,
	"sans-italic":      goitalic.TTF,
	"sans-bolditalic":  gobolditalic.TTF,
	"serif-regular":    goregular.TTF,
	"serif-bold":       gobold.TTF,
	"serif-italic":     goitalic.TTF,
	"serif-bolditalic": gobolditalic.TTF,
	"mono-regular":     goregular.TTF,
	"mono-bold":        gobold.TTF,
	"mono-italic":      goitalic.TTF,
	"mono-bolditalic":  gobolditalic.TTF,
}

// Fallback fonts for better Unicode support (especially CJK characters)
// Priority order: CJK fonts first for better Unicode support, then Latin fonts
var fallbackFontPaths = []string{
	// Windows system fonts for CJK support - PRIORITIZE for Chinese text
	"C:/Windows/Fonts/msyh.ttc",   // Microsoft YaHei (Simplified Chinese)
	"C:/Windows/Fonts/msyhbd.ttc", // Microsoft YaHei Bold
	"C:/Windows/Fonts/simsun.ttc", // SimSun (Simplified Chinese)
	"C:/Windows/Fonts/simhei.ttf", // SimHei (Simplified Chinese)
	"C:/Windows/Fonts/msjh.ttc",   // Microsoft JhengHei (Traditional Chinese)
	// macOS system fonts for CJK support
	"/System/Library/Fonts/PingFang.ttc",                   // PingFang SC (Simplified Chinese)
	"/System/Library/Fonts/Hiragino Sans GB.ttc",           // Hiragino Sans GB
	"/System/Library/Fonts/STHeiti Light.ttc",              // STHeiti
	"/System/Library/Fonts/Supplemental/Songti.ttc",        // Songti SC
	"/System/Library/Fonts/Supplemental/Arial Unicode.ttf", // Arial Unicode MS
	// Linux system fonts for CJK support
	"/usr/share/fonts/truetype/droid/DroidSansFallbackFull.ttf",
	"/usr/share/fonts/truetype/noto/NotoSansCJK-Regular.ttc",
	"/usr/share/fonts/opentype/noto/NotoSansCJK-Regular.ttc",
	// Local assets - for Latin text (proportional fonts)
	// Try both relative to current directory and parent directory
	"assets/DejaVuSans.ttf",
	"../assets/DejaVuSans.ttf",
	"resource/font/luxisr.ttf",
	"../resource/font/luxisr.ttf",
}

var mathFallbackFontPaths = []string{
	"C:/Windows/Fonts/cambria.ttc",
	"C:/Windows/Fonts/seguisym.ttf",
	"C:/Windows/Fonts/symbol.ttf",
}

// LoadFontFromFile loads a font from a file path
func LoadFontFromFile(path string) (font.Face, []byte, error) {
	// Check cache first
	fontCacheMu.RLock()
	if face, ok := fontCache[path]; ok {
		data := fontDataCache[path]
		fontCacheMu.RUnlock()
		return face, data, nil
	}
	fontCacheMu.RUnlock()

	// Read file
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	// Parse font
	face, err := parseFontData(data)
	if err != nil {
		return nil, nil, err
	}

	// Cache it
	fontCacheMu.Lock()
	fontCache[path] = face
	fontDataCache[path] = data
	fontCacheMu.Unlock()

	return face, data, nil
}

func parseFontData(data []byte) (font.Face, error) {
	if len(data) >= 4 && string(data[:4]) == "ttcf" {
		faces, err := font.ParseTTC(bytes.NewReader(data))
		if err != nil {
			return nil, err
		}
		if len(faces) == 0 {
			return nil, fmt.Errorf("ttc has no faces")
		}
		if len(faces) == 1 {
			return faces[0], nil
		}

		mathSamples := []rune{'∑', '∫', '≤', '≥', 'α', 'β', 'ℝ', 'ℤ', '∞'}
		best := faces[0]
		bestScore := 0
		for _, face := range faces {
			score := scoreFaceCoverage(face, mathSamples)
			if score > bestScore {
				bestScore = score
				best = face
			}
		}
		if bestScore > 0 {
			return best, nil
		}
		return faces[0], nil
	}
	return font.ParseTTF(bytes.NewReader(data))
}

func scoreFaceCoverage(face font.Face, samples []rune) int {
	if face == nil || len(samples) == 0 {
		return 0
	}
	score := 0
	shaper := &shaping.HarfbuzzShaper{}
	for _, r := range samples {
		in := shaping.Input{
			Text:      []rune{r},
			RunStart:  0,
			RunEnd:    1,
			Direction: di.DirectionLTR,
			Face:      face,
			Size:      fixed.I(12),
		}
		out := shaper.Shape(in)
		if len(out.Glyphs) > 0 && out.Glyphs[0].GlyphID != 0 {
			score++
		}
	}
	return score
}

// LoadEmbeddedFont loads an embedded font by name
func LoadEmbeddedFont(name string) (font.Face, []byte, error) {
	fontCacheMu.RLock()
	if face, ok := fontCache[name]; ok {
		data := fontDataCache[name]
		fontCacheMu.RUnlock()
		return face, data, nil
	}
	fontCacheMu.RUnlock()

	if name == "math" || strings.HasPrefix(name, "math-") {
		for _, fallbackPath := range mathFallbackFontPaths {
			face, fontData, err := LoadFontFromFile(fallbackPath)
			if err == nil {
				fontCacheMu.Lock()
				fontCache[name] = face
				fontDataCache[name] = fontData
				fontCacheMu.Unlock()
				return face, fontData, nil
			}
		}
	}

	if name == "cjk" || (len(name) >= 3 && name[:3] == "cjk") {
		for _, fallbackPath := range fallbackFontPaths {
			face, fontData, err := LoadFontFromFile(fallbackPath)
			if err == nil {
				// Cache with the requested name
				fontCacheMu.Lock()
				fontCache[name] = face
				fontDataCache[name] = fontData
				fontCacheMu.Unlock()
				// Debug: print which font was loaded (commented out for production)
				// fmt.Printf("[字体加载] %s -> %s\n", name, fallbackPath)
				return face, fontData, nil
			}
		}
	}

	// Try loading from embedded fonts
	data, ok := embeddedFonts[name]
	if !ok {
		// Try loading from assets directory
		assetsPath := filepath.Join("assets", name+".ttf")
		if face, fontData, err := LoadFontFromFile(assetsPath); err == nil {
			return face, fontData, nil
		}
		// Fallback to Go-Regular
		data = goregular.TTF
	}

	face, err := font.ParseTTF(bytes.NewReader(data))
	if err != nil {
		return nil, nil, err
	}

	fontCacheMu.Lock()
	fontCache[name] = face
	fontDataCache[name] = data
	fontCacheMu.Unlock()

	return face, data, nil
}

// GetDefaultFont returns the default embedded font
func GetDefaultFont() (font.Face, []byte) {
	face, data, err := LoadEmbeddedFont("Go-Regular")
	if err != nil {
		// This should never happen as Go-Regular is embedded
		panic("failed to load default font")
	}
	return face, data
}

// GetDejaVuSans returns the DejaVu Sans font
func GetDejaVuSans() (font.Face, []byte) {
	face, data, err := LoadEmbeddedFont("DejaVuSans")
	if err != nil {
		return GetDefaultFont()
	}
	return face, data
}
