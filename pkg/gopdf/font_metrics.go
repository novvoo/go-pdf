package gopdf

import (
	"fmt"
	"math"
)

// FontMetrics 字体度量信息
type FontMetrics struct {
	Ascent       float64 // 上升高度
	Descent      float64 // 下降深度
	CapHeight    float64 // 大写字母高度
	XHeight      float64 // 小写字母 x 的高度
	ItalicAngle  float64 // 斜体角度
	StemV        float64 // 垂直笔画宽度
	StemH        float64 // 水平笔画宽度
	AvgWidth     float64 // 平均字符宽度
	MaxWidth     float64 // 最大字符宽度
	MissingWidth float64 // 缺失字形的宽度
	Leading      float64 // 行间距
	Flags        int     // 字体标志
}

// CalculateTextWidth 计算文本宽度
// text: 文本内容
// font: 字体信息
// fontSize: 字体大小
func CalculateTextWidth(text string, font *Font, fontSize float64) float64 {
	if font == nil || fontSize == 0 {
		return 0
	}

	var totalWidth float64
	runes := []rune(text)

	// 如果有字形宽度信息，使用精确计算
	if font.Widths != nil {
		for _, r := range runes {
			// 将 rune 转换为字符编码
			charCode := uint16(r)

			// 获取字形宽度（单位：千分之一 em）
			width := font.GetWidth(charCode)

			// 转换为用户空间单位：width / 1000 * fontSize
			totalWidth += (width / 1000.0) * fontSize
		}
	} else {
		// 没有字形宽度信息，使用改进的估算
		totalWidth = estimateTextWidth(text, font, fontSize)
	}

	return totalWidth
}

// estimateTextWidth 估算文本宽度（当没有精确字形宽度信息时）
func estimateTextWidth(text string, font *Font, fontSize float64) float64 {
	runes := []rune(text)
	runeCount := float64(len(runes))

	if runeCount == 0 {
		return 0
	}

	// 根据字体类型和特征使用不同的估算系数
	var widthFactor float64 = 0.50 // 默认值

	// 根据字体名称推测类型
	baseFontLower := ""
	if font != nil && font.BaseFont != "" {
		baseFontLower = font.BaseFont
	}

	// 简单的字体类型检测
	if contains(baseFontLower, "Mono") || contains(baseFontLower, "Courier") {
		widthFactor = 0.6 // 等宽字体
	} else if contains(baseFontLower, "Times") || contains(baseFontLower, "Serif") {
		widthFactor = 0.52 // 衬线字体
	} else if contains(baseFontLower, "Sans") || contains(baseFontLower, "Arial") || contains(baseFontLower, "Helvetica") {
		widthFactor = 0.50 // 无衬线字体
	} else if contains(baseFontLower, "Script") || contains(baseFontLower, "Cursive") {
		widthFactor = 0.45 // 手写体
	}

	// 对于 CJK 字符，使用不同的系数
	cjkCount := 0
	for _, r := range runes {
		if isCJKCharacter(r) {
			cjkCount++
		}
	}

	if cjkCount > 0 {
		// CJK 字符通常是全角的
		cjkWidth := float64(cjkCount) * fontSize * 1.0
		latinCount := runeCount - float64(cjkCount)
		latinWidth := latinCount * fontSize * widthFactor
		return cjkWidth + latinWidth
	}

	// 考虑字符宽度的变化
	// 窄字符（i, l, t）和宽字符（W, M）的比例
	narrowCount := 0
	wideCount := 0
	for _, r := range runes {
		switch r {
		case 'i', 'l', 'I', 'j', 't', 'f', 'r', '!', '|', '.', ',', ':', ';', '\'':
			narrowCount++
		case 'W', 'M', 'm', 'w', '@', '%', '#':
			wideCount++
		}
	}

	// 调整宽度因子
	if narrowCount > 0 {
		widthFactor -= float64(narrowCount) / runeCount * 0.15
	}
	if wideCount > 0 {
		widthFactor += float64(wideCount) / runeCount * 0.15
	}

	return runeCount * fontSize * widthFactor
}

// isCJKCharacter 判断是否是 CJK 字符
func isCJKCharacter(r rune) bool {
	// CJK 统一表意文字
	if r >= 0x4E00 && r <= 0x9FFF {
		return true
	}
	// CJK 扩展 A
	if r >= 0x3400 && r <= 0x4DBF {
		return true
	}
	// CJK 扩展 B-F
	if r >= 0x20000 && r <= 0x2EBEF {
		return true
	}
	// CJK 兼容表意文字
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

// CalculateTextWidthWithKerning 计算带字距调整的文本宽度
// text: 文本内容
// font: 字体信息
// fontSize: 字体大小
// kerningAdjustments: 字距调整数组（来自 TJ 操作符）
func CalculateTextWidthWithKerning(text string, font *Font, fontSize float64, kerningAdjustments []float64) float64 {
	baseWidth := CalculateTextWidth(text, font, fontSize)

	// 应用字距调整
	// PDF 中的字距调整单位是千分之一 em
	var kerningTotal float64
	for _, adj := range kerningAdjustments {
		kerningTotal += adj
	}

	// 转换为用户空间单位
	kerningWidth := (kerningTotal / 1000.0) * fontSize

	return baseWidth + kerningWidth
}

// GetCharacterWidth 获取单个字符的宽度
func GetCharacterWidth(char rune, font *Font, fontSize float64) float64 {
	if font == nil || fontSize == 0 {
		return 0
	}

	charCode := uint16(char)

	if font.Widths != nil {
		width := font.GetWidth(charCode)
		return (width / 1000.0) * fontSize
	}

	// 使用估算
	return estimateTextWidth(string(char), font, fontSize)
}

// contains 检查字符串是否包含子串（不区分大小写）
func contains(s, substr string) bool {
	// 简单的包含检查
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			c1 := s[i+j]
			c2 := substr[j]
			// 转换为小写比较
			if c1 >= 'A' && c1 <= 'Z' {
				c1 += 32
			}
			if c2 >= 'A' && c2 <= 'Z' {
				c2 += 32
			}
			if c1 != c2 {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// CalculateStringBounds 计算字符串的边界框
func CalculateStringBounds(text string, font *Font, fontSize float64, x, y float64) (minX, minY, maxX, maxY float64) {
	width := CalculateTextWidth(text, font, fontSize)

	// 简化的边界框计算
	// 实际应该考虑字体的 ascent/descent
	ascent := fontSize * 0.8
	descent := fontSize * 0.2

	minX = x
	minY = y - descent
	maxX = x + width
	maxY = y + ascent

	return
}

// NormalizeWidth 标准化宽度值
// 将字形宽度从字体单位转换为用户空间单位
func NormalizeWidth(width float64, fontSize float64) float64 {
	return (width / 1000.0) * fontSize
}

// GetSpaceWidth 获取空格字符的宽度
func GetSpaceWidth(font *Font, fontSize float64) float64 {
	if font == nil || fontSize == 0 {
		return fontSize * 0.25 // 默认空格宽度
	}

	spaceCode := uint16(' ')

	if font.Widths != nil {
		width := font.GetWidth(spaceCode)
		return (width / 1000.0) * fontSize
	}

	return fontSize * 0.25
}

// CalculateAverageCharWidth 计算平均字符宽度
func CalculateAverageCharWidth(font *Font, fontSize float64) float64 {
	if font == nil || fontSize == 0 {
		return 0
	}

	if font.Widths != nil && len(font.Widths.CIDWidths) > 0 {
		// 计算所有字形宽度的平均值
		var total float64
		count := 0
		for _, width := range font.Widths.CIDWidths {
			total += width
			count++
		}
		if count > 0 {
			avgWidth := total / float64(count)
			return (avgWidth / 1000.0) * fontSize
		}
	}

	// 使用默认值
	return fontSize * 0.5
}

// RoundWidth 四舍五入宽度值到指定精度
func RoundWidth(width float64, precision int) float64 {
	multiplier := math.Pow(10, float64(precision))
	return math.Round(width*multiplier) / multiplier
}

// InterpolateWidth 在两个宽度值之间插值
func InterpolateWidth(width1, width2, t float64) float64 {
	return width1 + (width2-width1)*t
}

// ValidateWidth 验证宽度值是否合理
func ValidateWidth(width, fontSize float64) error {
	if width < 0 {
		return fmt.Errorf("negative width: %.2f", width)
	}
	if width > fontSize*2 {
		return fmt.Errorf("width %.2f exceeds reasonable limit for font size %.2f", width, fontSize)
	}
	return nil
}
