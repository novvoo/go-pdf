package gopdf

import (
	"bytes"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-text/typesetting/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/gofont/gobolditalic"
	"golang.org/x/image/font/gofont/goitalic"
	"golang.org/x/image/font/gofont/goregular"
)

// Font cache to avoid re-parsing fonts
var (
	fontCache     = make(map[string]font.Face)
	fontDataCache = make(map[string][]byte)
	fontCacheMu   sync.RWMutex
)

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
	face, err := font.ParseTTF(bytes.NewReader(data))
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

// LoadEmbeddedFont loads an embedded font by name
func LoadEmbeddedFont(name string) (font.Face, []byte, error) {
	fontCacheMu.RLock()
	if face, ok := fontCache[name]; ok {
		data := fontDataCache[name]
		fontCacheMu.RUnlock()
		return face, data, nil
	}
	fontCacheMu.RUnlock()

	// For sans fonts, try fallback fonts first (better Unicode support)
	// Check if name starts with "sans" (covers sans-regular, sans-bold, sans-italic, etc.)
	if name == "sans" || (len(name) >= 4 && name[:4] == "sans") {
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
