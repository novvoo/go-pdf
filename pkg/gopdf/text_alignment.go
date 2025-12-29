package gopdf

// TextAlignment specifies vertical text alignment
type TextAlignment int

const (
	// AlignBaseline aligns text on the baseline (default)
	AlignBaseline TextAlignment = iota
	// AlignTop aligns text to the top of the em square
	AlignTop
	// AlignBottom aligns text to the bottom of the em square
	AlignBottom
	// AlignMiddle aligns text to the vertical middle
	AlignMiddle
	// AlignCapHeight aligns text to the cap height
	AlignCapHeight
	// AlignXHeight aligns text to the x-height
	AlignXHeight
)

// GetAlignmentOffset calculates the Y offset for a given text alignment
func GetAlignmentOffset(alignment TextAlignment, fontMetrics *FontExtents) float64 {
	switch alignment {
	case AlignBaseline:
		return 0
	case AlignTop:
		return fontMetrics.Ascent
	case AlignBottom:
		return fontMetrics.Descent
	case AlignMiddle:
		return (fontMetrics.Ascent - fontMetrics.Descent) / 2
	case AlignCapHeight:
		// Approximate cap height as 70% of ascent
		return fontMetrics.Ascent * 0.7
	case AlignXHeight:
		// Approximate x-height as 50% of ascent
		return fontMetrics.Ascent * 0.5
	default:
		return 0
	}
}
