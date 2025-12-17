package gopdf

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// FontLoader 字体加载器
type FontLoader struct {
	fontDirs      []string          // 字体搜索目录
	fontCache     map[string]string // 字体名称 -> 文件路径缓存
	embeddedFonts map[string][]byte // 嵌入字体缓存
}

// NewFontLoader 创建字体加载器
func NewFontLoader() *FontLoader {
	loader := &FontLoader{
		fontDirs:      getSystemFontDirs(),
		fontCache:     make(map[string]string),
		embeddedFonts: make(map[string][]byte),
	}
	return loader
}

// getSystemFontDirs 获取系统字体目录
func getSystemFontDirs() []string {
	var dirs []string

	switch runtime.GOOS {
	case "windows":
		// Windows 字体目录
		winDir := os.Getenv("WINDIR")
		if winDir == "" {
			winDir = "C:\\Windows"
		}
		dirs = append(dirs, filepath.Join(winDir, "Fonts"))

		// 用户字体目录
		localAppData := os.Getenv("LOCALAPPDATA")
		if localAppData != "" {
			dirs = append(dirs, filepath.Join(localAppData, "Microsoft", "Windows", "Fonts"))
		}

	case "darwin":
		// macOS 字体目录
		dirs = append(dirs,
			"/System/Library/Fonts",
			"/Library/Fonts",
			filepath.Join(os.Getenv("HOME"), "Library", "Fonts"),
		)

	case "linux":
		// Linux 字体目录
		dirs = append(dirs,
			"/usr/share/fonts",
			"/usr/local/share/fonts",
			filepath.Join(os.Getenv("HOME"), ".fonts"),
			filepath.Join(os.Getenv("HOME"), ".local", "share", "fonts"),
		)
	}

	return dirs
}

// FindFont 查找字体文件
func (fl *FontLoader) FindFont(fontName string) (string, error) {
	// 检查缓存
	if path, ok := fl.fontCache[fontName]; ok {
		return path, nil
	}

	// 标准化字体名称
	normalizedName := normalizeFontName(fontName)

	// 在字体目录中搜索
	for _, dir := range fl.fontDirs {
		path, err := fl.searchFontInDir(dir, normalizedName)
		if err == nil {
			fl.fontCache[fontName] = path
			return path, nil
		}
	}

	// 尝试字体替换
	substituteName := getFontSubstitute(normalizedName)
	if substituteName != normalizedName {
		return fl.FindFont(substituteName)
	}

	return "", fmt.Errorf("font not found: %s", fontName)
}

// searchFontInDir 在目录中搜索字体
func (fl *FontLoader) searchFontInDir(dir string, fontName string) (string, error) {
	var foundPath string

	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误,继续搜索
		}

		if info.IsDir() {
			return nil
		}

		// 检查文件扩展名
		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".ttf" && ext != ".otf" && ext != ".ttc" {
			return nil
		}

		// 检查文件名是否匹配
		baseName := strings.ToLower(strings.TrimSuffix(filepath.Base(path), ext))
		if strings.Contains(baseName, fontName) {
			foundPath = path
			return filepath.SkipDir // 找到后停止搜索
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	if foundPath != "" {
		return foundPath, nil
	}

	return "", fmt.Errorf("font not found in directory: %s", dir)
}

// LoadEmbeddedFont 加载嵌入字体
func (fl *FontLoader) LoadEmbeddedFont(fontName string, fontData []byte) error {
	fl.embeddedFonts[fontName] = fontData
	return nil
}

// GetEmbeddedFont 获取嵌入字体数据
func (fl *FontLoader) GetEmbeddedFont(fontName string) ([]byte, bool) {
	data, ok := fl.embeddedFonts[fontName]
	return data, ok
}

// normalizeFontName 标准化字体名称
func normalizeFontName(name string) string {
	// 移除前缀斜杠
	name = strings.TrimPrefix(name, "/")

	// 转换为小写
	name = strings.ToLower(name)

	// 移除特殊字符
	name = strings.ReplaceAll(name, "-", "")
	name = strings.ReplaceAll(name, "_", "")
	name = strings.ReplaceAll(name, " ", "")

	return name
}

// getFontSubstitute 获取字体替换
func getFontSubstitute(fontName string) string {
	// 字体替换映射
	substitutes := map[string]string{
		// 衬线字体
		"timesnewroman":   "times",
		"times":           "liberation serif",
		"liberationserif": "times",
		"georgia":         "times",

		// 无衬线字体
		"arial":          "helvetica",
		"helvetica":      "liberation sans",
		"liberationsans": "arial",
		"verdana":        "arial",
		"tahoma":         "arial",

		// 等宽字体
		"couriernew":     "courier",
		"courier":        "liberation mono",
		"liberationmono": "courier",
		"consolas":       "courier",

		// CJK 字体
		"simsun":         "noto sans cjk sc",
		"simhei":         "noto sans cjk sc",
		"microsoftyahei": "noto sans cjk sc",
		"msgothic":       "noto sans cjk jp",
		"mspgothic":      "noto sans cjk jp",
		"malgun":         "noto sans cjk kr",
		"malgunhgothic":  "noto sans cjk kr",
	}

	if substitute, ok := substitutes[fontName]; ok {
		return substitute
	}

	return fontName
}

// GetFontStyle 从字体名称提取样式信息
func GetFontStyle(fontName string) (family string, bold bool, italic bool) {
	nameLower := strings.ToLower(fontName)

	// 检测粗体
	bold = strings.Contains(nameLower, "bold") ||
		strings.Contains(nameLower, "heavy") ||
		strings.Contains(nameLower, "black")

	// 检测斜体
	italic = strings.Contains(nameLower, "italic") ||
		strings.Contains(nameLower, "oblique") ||
		strings.Contains(nameLower, "slant")

	// 提取字体族名称
	family = fontName
	family = strings.ReplaceAll(family, "-Bold", "")
	family = strings.ReplaceAll(family, "-Italic", "")
	family = strings.ReplaceAll(family, "-BoldItalic", "")
	family = strings.ReplaceAll(family, "-Oblique", "")
	family = strings.ReplaceAll(family, "-BoldOblique", "")
	family = strings.TrimSpace(family)

	return
}

// IsCJKFont 判断是否是 CJK 字体
func IsCJKFont(fontName string) bool {
	nameLower := strings.ToLower(fontName)

	cjkKeywords := []string{
		"cjk", "chinese", "japanese", "korean",
		"simsun", "simhei", "yahei", "songti", "heiti",
		"mincho", "gothic", "meiryo",
		"malgun", "batang", "dotum",
		"noto", "source han",
	}

	for _, keyword := range cjkKeywords {
		if strings.Contains(nameLower, keyword) {
			return true
		}
	}

	return false
}

// GetDefaultCJKFont 获取默认 CJK 字体
func GetDefaultCJKFont() string {
	switch runtime.GOOS {
	case "windows":
		return "Microsoft YaHei"
	case "darwin":
		return "PingFang SC"
	case "linux":
		return "Noto Sans CJK SC"
	default:
		return "sans-serif"
	}
}

// FontFallbackChain 字体回退链
type FontFallbackChain struct {
	fonts []string
}

// NewFontFallbackChain 创建字体回退链
func NewFontFallbackChain(primaryFont string) *FontFallbackChain {
	chain := &FontFallbackChain{
		fonts: []string{primaryFont},
	}

	// 添加通用回退字体
	if IsCJKFont(primaryFont) {
		chain.fonts = append(chain.fonts, GetDefaultCJKFont())
	}

	// 添加系统默认字体
	chain.fonts = append(chain.fonts, "sans-serif", "serif", "monospace")

	return chain
}

// GetFonts 获取回退字体列表
func (fc *FontFallbackChain) GetFonts() []string {
	return fc.fonts
}

// AddFallback 添加回退字体
func (fc *FontFallbackChain) AddFallback(fontName string) {
	fc.fonts = append(fc.fonts, fontName)
}

// FontMetricsCache 字体度量缓存
type FontMetricsCache struct {
	cache map[string]*FontMetrics
}

// NewFontMetricsCache 创建字体度量缓存
func NewFontMetricsCache() *FontMetricsCache {
	return &FontMetricsCache{
		cache: make(map[string]*FontMetrics),
	}
}

// Get 获取字体度量
func (fmc *FontMetricsCache) Get(fontName string) (*FontMetrics, bool) {
	metrics, ok := fmc.cache[fontName]
	return metrics, ok
}

// Set 设置字体度量
func (fmc *FontMetricsCache) Set(fontName string, metrics *FontMetrics) {
	fmc.cache[fontName] = metrics
}

// Clear 清空缓存
func (fmc *FontMetricsCache) Clear() {
	fmc.cache = make(map[string]*FontMetrics)
}
