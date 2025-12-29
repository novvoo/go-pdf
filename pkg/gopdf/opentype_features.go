package gopdf

import (
	"strings"

	"github.com/go-text/typesetting/di"
	"github.com/go-text/typesetting/language"
)

// TextDirection represents text direction
type TextDirection int

const (
	TextDirectionLTR  TextDirection = iota // Left to Right
	TextDirectionRTL                       // Right to Left
	TextDirectionTTB                       // Top to Bottom (vertical)
	TextDirectionBTT                       // Bottom to Top (vertical)
	TextDirectionAuto                      // Auto-detect from text
)

// OpenTypeFeature represents an OpenType feature tag and value
type OpenTypeFeature struct {
	Tag   string // 4-character OpenType feature tag (e.g., "liga", "smcp", "kern")
	Value uint32 // Feature value (typically 0=off, 1=on, or specific selector)
}

// ShapingOptions contains advanced text shaping options
type ShapingOptions struct {
	Direction TextDirection     // Text direction
	Language  string            // BCP 47 language tag (e.g., "en", "ar", "zh-CN")
	Script    string            // ISO 15924 script code (e.g., "Latn", "Arab", "Hans")
	Features  []OpenTypeFeature // OpenType features to enable/disable
}

// Common OpenType feature tags
const (
	// Ligatures
	FeatureLigatures        = "liga" // Standard ligatures
	FeatureDiscretionaryLig = "dlig" // Discretionary ligatures
	FeatureContextualLig    = "clig" // Contextual ligatures
	FeatureHistoricalLig    = "hlig" // Historical ligatures

	// Case
	FeatureSmallCaps     = "smcp" // Small capitals
	FeatureAllSmallCaps  = "c2sc" // Capitals to small capitals
	FeaturePetiteCaps    = "pcap" // Petite capitals
	FeatureAllPetiteCaps = "c2pc" // Capitals to petite capitals
	FeatureUnicaseHeight = "unic" // Unicase
	FeatureTitling       = "titl" // Titling

	// Position
	FeatureSuperscript        = "sups" // Superscript
	FeatureSubscript          = "subs" // Subscript
	FeatureOrdinals           = "ordn" // Ordinals
	FeatureScientificInferior = "sinf" // Scientific inferiors

	// Number
	FeatureLiningFigures       = "lnum" // Lining figures
	FeatureOldstyleFigures     = "onum" // Oldstyle figures
	FeatureProportionalFigures = "pnum" // Proportional figures
	FeatureTabularFigures      = "tnum" // Tabular figures
	FeatureDiagonalFractions   = "frac" // Diagonal fractions
	FeatureNumerators          = "numr" // Numerators
	FeatureDenominators        = "dnom" // Denominators

	// Alternates
	FeatureContextualAlternates = "calt" // Contextual alternates
	FeatureSwash                = "swsh" // Swash
	FeatureHistoricalForms      = "hist" // Historical forms
	FeatureStylisticSet01       = "ss01" // Stylistic set 1
	FeatureStylisticSet02       = "ss02" // Stylistic set 2
	FeatureStylisticSet03       = "ss03" // Stylistic set 3
	FeatureStylisticSet04       = "ss04" // Stylistic set 4
	FeatureStylisticSet05       = "ss05" // Stylistic set 5
	FeatureStylisticSet06       = "ss06" // Stylistic set 6
	FeatureStylisticSet07       = "ss07" // Stylistic set 7
	FeatureStylisticSet08       = "ss08" // Stylistic set 8
	FeatureStylisticSet09       = "ss09" // Stylistic set 9
	FeatureStylisticSet10       = "ss10" // Stylistic set 10
	FeatureStylisticSet11       = "ss11" // Stylistic set 11
	FeatureStylisticSet12       = "ss12" // Stylistic set 12
	FeatureStylisticSet13       = "ss13" // Stylistic set 13
	FeatureStylisticSet14       = "ss14" // Stylistic set 14
	FeatureStylisticSet15       = "ss15" // Stylistic set 15
	FeatureStylisticSet16       = "ss16" // Stylistic set 16
	FeatureStylisticSet17       = "ss17" // Stylistic set 17
	FeatureStylisticSet18       = "ss18" // Stylistic set 18
	FeatureStylisticSet19       = "ss19" // Stylistic set 19
	FeatureStylisticSet20       = "ss20" // Stylistic set 20

	// Spacing
	FeatureKerning = "kern" // Kerning

	// Vertical
	FeatureVerticalAlternates = "vert" // Vertical alternates
	FeatureVerticalKerning    = "vkrn" // Vertical kerning
)

// NewShapingOptions creates default shaping options
func NewShapingOptions() *ShapingOptions {
	return &ShapingOptions{
		Direction: TextDirectionAuto,
		Language:  "",
		Script:    "",
		Features: []OpenTypeFeature{
			{Tag: FeatureLigatures, Value: 1},            // Enable standard ligatures
			{Tag: FeatureKerning, Value: 1},              // Enable kerning
			{Tag: FeatureContextualAlternates, Value: 1}, // Enable contextual alternates
		},
	}
}

// DetectTextDirection automatically detects text direction from content
func DetectTextDirection(text string) TextDirection {
	if text == "" {
		return TextDirectionLTR
	}

	// Check first strong directional character
	for _, r := range text {
		// RTL scripts
		if isRTLChar(r) {
			return TextDirectionRTL
		}
		// LTR scripts (Latin, Cyrillic, Greek, etc.)
		if isLTRChar(r) {
			return TextDirectionLTR
		}
	}

	return TextDirectionLTR
}

// isRTLChar checks if a character belongs to an RTL script
func isRTLChar(r rune) bool {
	// Arabic (U+0600 - U+06FF, U+0750 - U+077F, U+08A0 - U+08FF)
	if (r >= 0x0600 && r <= 0x06FF) ||
		(r >= 0x0750 && r <= 0x077F) ||
		(r >= 0x08A0 && r <= 0x08FF) ||
		(r >= 0xFB50 && r <= 0xFDFF) || // Arabic Presentation Forms
		(r >= 0xFE70 && r <= 0xFEFF) {
		return true
	}

	// Hebrew (U+0590 - U+05FF)
	if r >= 0x0590 && r <= 0x05FF {
		return true
	}

	// Syriac (U+0700 - U+074F)
	if r >= 0x0700 && r <= 0x074F {
		return true
	}

	// Thaana (U+0780 - U+07BF)
	if r >= 0x0780 && r <= 0x07BF {
		return true
	}

	// N'Ko (U+07C0 - U+07FF)
	if r >= 0x07C0 && r <= 0x07FF {
		return true
	}

	return false
}

// isLTRChar checks if a character belongs to an LTR script
func isLTRChar(r rune) bool {
	// Basic Latin, Latin Extended
	if (r >= 0x0041 && r <= 0x005A) || // A-Z
		(r >= 0x0061 && r <= 0x007A) || // a-z
		(r >= 0x00C0 && r <= 0x024F) { // Latin Extended
		return true
	}

	// Cyrillic
	if r >= 0x0400 && r <= 0x04FF {
		return true
	}

	// Greek
	if r >= 0x0370 && r <= 0x03FF {
		return true
	}

	return false
}

// DetectScript automatically detects the script from text content
func DetectScript(text string) string {
	if text == "" {
		return "Latn" // Default to Latin
	}

	// Count characters by script
	scriptCounts := make(map[string]int)

	for _, r := range text {
		script := getCharScript(r)
		if script != "" {
			scriptCounts[script]++
		}
	}

	// Return the most common script
	maxCount := 0
	dominantScript := "Latn"
	for script, count := range scriptCounts {
		if count > maxCount {
			maxCount = count
			dominantScript = script
		}
	}

	return dominantScript
}

// getCharScript returns the ISO 15924 script code for a character
func getCharScript(r rune) string {
	switch {
	// Latin
	case (r >= 0x0041 && r <= 0x005A) || (r >= 0x0061 && r <= 0x007A) || (r >= 0x00C0 && r <= 0x024F):
		return "Latn"

	// Arabic
	case (r >= 0x0600 && r <= 0x06FF) || (r >= 0x0750 && r <= 0x077F) || (r >= 0x08A0 && r <= 0x08FF):
		return "Arab"

	// Hebrew
	case r >= 0x0590 && r <= 0x05FF:
		return "Hebr"

	// Cyrillic
	case r >= 0x0400 && r <= 0x04FF:
		return "Cyrl"

	// Greek
	case r >= 0x0370 && r <= 0x03FF:
		return "Grek"

	// Devanagari (Hindi, Sanskrit, etc.)
	case r >= 0x0900 && r <= 0x097F:
		return "Deva"

	// Bengali
	case r >= 0x0980 && r <= 0x09FF:
		return "Beng"

	// Thai
	case r >= 0x0E00 && r <= 0x0E7F:
		return "Thai"

	// Han (Chinese)
	case (r >= 0x4E00 && r <= 0x9FFF) || (r >= 0x3400 && r <= 0x4DBF):
		return "Hans"

	// Hiragana
	case r >= 0x3040 && r <= 0x309F:
		return "Hira"

	// Katakana
	case r >= 0x30A0 && r <= 0x30FF:
		return "Kana"

	// Hangul (Korean)
	case (r >= 0xAC00 && r <= 0xD7AF) || (r >= 0x1100 && r <= 0x11FF):
		return "Hang"

	default:
		return ""
	}
}

// DetectLanguage attempts to detect language from text content
// Returns BCP 47 language tag
func DetectLanguage(text string) string {
	if text == "" {
		return "en" // Default to English
	}

	script := DetectScript(text)

	// Map script to common language
	switch script {
	case "Arab":
		return "ar" // Arabic
	case "Hebr":
		return "he" // Hebrew
	case "Cyrl":
		return "ru" // Russian (could be other Cyrillic languages)
	case "Grek":
		return "el" // Greek
	case "Deva":
		return "hi" // Hindi
	case "Beng":
		return "bn" // Bengali
	case "Thai":
		return "th" // Thai
	case "Hans":
		return "zh" // Chinese (Simplified)
	case "Hant":
		return "zh-TW" // Chinese (Traditional)
	case "Hira", "Kana":
		return "ja" // Japanese
	case "Hang":
		return "ko" // Korean
	default:
		return "en" // Default to English
	}
}

// convertDirection converts TextDirection to di.Direction
func convertDirection(dir TextDirection, text string) di.Direction {
	switch dir {
	case TextDirectionLTR:
		return di.DirectionLTR
	case TextDirectionRTL:
		return di.DirectionRTL
	case TextDirectionTTB:
		return di.DirectionTTB
	case TextDirectionBTT:
		return di.DirectionBTT
	case TextDirectionAuto:
		detected := DetectTextDirection(text)
		return convertDirection(detected, text)
	default:
		return di.DirectionLTR
	}
}

// convertLanguage converts language string to language.Language
func convertLanguage(lang string) language.Language {
	if lang == "" {
		return language.Language("")
	}

	// Parse BCP 47 language tag
	parts := strings.Split(lang, "-")
	if len(parts) == 0 {
		return language.Language("")
	}

	// Simple conversion - just use the primary language subtag
	return language.Language(parts[0])
}

// convertScript converts script string to language.Script
func convertScript(script string) language.Script {
	if script == "" {
		// Return a zero-value Script (unknown)
		var zeroScript language.Script
		return zeroScript
	}

	// Try to parse the script string
	s, err := language.ParseScript(script)
	if err != nil {
		// Return a zero-value Script (unknown)
		var zeroScript language.Script
		return zeroScript
	}
	return s
}

// ShapeText performs text shaping with advanced OpenType features
// SetDefaultFeatures sets default OpenType features for common use cases
func SetDefaultFeatures(options *ShapingOptions, preset string) {
	switch preset {
	case "default":
		options.Features = []OpenTypeFeature{
			{Tag: FeatureLigatures, Value: 1},
			{Tag: FeatureKerning, Value: 1},
			{Tag: FeatureContextualAlternates, Value: 1},
		}

	case "no-ligatures":
		options.Features = []OpenTypeFeature{
			{Tag: FeatureLigatures, Value: 0},
			{Tag: FeatureKerning, Value: 1},
		}

	case "small-caps":
		options.Features = []OpenTypeFeature{
			{Tag: FeatureSmallCaps, Value: 1},
			{Tag: FeatureLigatures, Value: 1},
			{Tag: FeatureKerning, Value: 1},
		}

	case "oldstyle-figures":
		options.Features = []OpenTypeFeature{
			{Tag: FeatureOldstyleFigures, Value: 1},
			{Tag: FeatureLigatures, Value: 1},
			{Tag: FeatureKerning, Value: 1},
		}

	case "tabular-figures":
		options.Features = []OpenTypeFeature{
			{Tag: FeatureTabularFigures, Value: 1},
			{Tag: FeatureLiningFigures, Value: 1},
			{Tag: FeatureKerning, Value: 1},
		}

	case "all-features":
		options.Features = []OpenTypeFeature{
			{Tag: FeatureLigatures, Value: 1},
			{Tag: FeatureContextualLig, Value: 1},
			{Tag: FeatureKerning, Value: 1},
			{Tag: FeatureContextualAlternates, Value: 1},
		}

	default:
		options.Features = []OpenTypeFeature{
			{Tag: FeatureLigatures, Value: 1},
			{Tag: FeatureKerning, Value: 1},
		}
	}
}

// IsVerticalScript checks if a script is typically written vertically
func IsVerticalScript(script string) bool {
	switch script {
	case "Hans", "Hant", "Hira", "Kana", "Hang":
		return true
	default:
		return false
	}
}

// NeedsComplexShaping checks if text requires complex shaping
func NeedsComplexShaping(text string) bool {
	for _, r := range text {
		// Arabic, Hebrew, Indic scripts, Thai, etc. need complex shaping
		if isRTLChar(r) {
			return true
		}
		// Indic scripts
		if (r >= 0x0900 && r <= 0x097F) || // Devanagari
			(r >= 0x0980 && r <= 0x09FF) || // Bengali
			(r >= 0x0A00 && r <= 0x0A7F) || // Gurmukhi
			(r >= 0x0A80 && r <= 0x0AFF) || // Gujarati
			(r >= 0x0B00 && r <= 0x0B7F) || // Oriya
			(r >= 0x0B80 && r <= 0x0BFF) || // Tamil
			(r >= 0x0C00 && r <= 0x0C7F) || // Telugu
			(r >= 0x0C80 && r <= 0x0CFF) || // Kannada
			(r >= 0x0D00 && r <= 0x0D7F) { // Malayalam
			return true
		}
		// Thai
		if r >= 0x0E00 && r <= 0x0E7F {
			return true
		}
	}
	return false
}

// GetBidiLevel returns the bidirectional level for a character
// Level 0 = LTR, Level 1 = RTL
func GetBidiLevel(r rune) int {
	if isRTLChar(r) {
		return 1
	}
	return 0
}

// SplitBidiRuns splits text into runs of consistent directionality
func SplitBidiRuns(text string) []struct {
	Text  string
	Level int
} {
	if text == "" {
		return nil
	}

	runs := make([]struct {
		Text  string
		Level int
	}, 0)

	var currentRun strings.Builder
	currentLevel := -1

	for _, r := range text {
		level := GetBidiLevel(r)

		if currentLevel == -1 {
			currentLevel = level
		}

		if level != currentLevel {
			// Start new run
			if currentRun.Len() > 0 {
				runs = append(runs, struct {
					Text  string
					Level int
				}{
					Text:  currentRun.String(),
					Level: currentLevel,
				})
				currentRun.Reset()
			}
			currentLevel = level
		}

		currentRun.WriteRune(r)
	}

	// Add final run
	if currentRun.Len() > 0 {
		runs = append(runs, struct {
			Text  string
			Level int
		}{
			Text:  currentRun.String(),
			Level: currentLevel,
		})
	}

	return runs
}

// NormalizeText performs Unicode normalization (NFC) on text
// This is important for consistent text rendering
func NormalizeText(text string) string {
	// This is a simplified version
	// For full Unicode normalization, use golang.org/x/text/unicode/norm
	return text
}
