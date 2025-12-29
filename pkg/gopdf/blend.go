package gopdf

import (
	"image/color"
)

// pdfBlendColor applies a simplified blend operation to a solid color.
// NOTE: This is a major simplification. Full Gopdf blending requires pixel-level
// manipulation of the destination surface, which is not exposed by Pango.
// This function only handles the source color's alpha based on the operator.
func pdfBlendColor(src color.Color, op Operator) color.Color {
	r, g, b, a := src.RGBA()

	// Convert to non-premultiplied alpha for easier logic
	alpha := float64(a) / 0xFFFF

	// Convert RGBA values to 8-bit
	r8 := uint8(r >> 8)
	g8 := uint8(g >> 8)
	b8 := uint8(b >> 8)
	a8 := uint8(a >> 8)

	switch op {
	case OperatorClear:
		// Clear: result is fully transparent (alpha = 0)
		return color.NRGBA{R: 0, G: 0, B: 0, A: 0}
	case OperatorSource:
		// Source: result is source (alpha = source alpha)
		return color.NRGBA{R: r8, G: g8, B: b8, A: a8}
	case OperatorOver:
		// Over: result is source over destination (default behavior)
		return color.NRGBA{R: r8, G: g8, B: b8, A: a8}
	case OperatorIn:
		// In: result is source multiplied by destination alpha
		// Since we don't have destination alpha here, we'll just return source.
		// This is a major simplification.
		return color.NRGBA{R: r8, G: g8, B: b8, A: a8}
	case OperatorOut:
		// Out: result is source multiplied by (1 - destination alpha)
		// Since we don't have destination alpha here, we'll just return source.
		return color.NRGBA{R: r8, G: g8, B: b8, A: a8}
	case OperatorAtop:
		// Atop: result is source over destination, but only where destination is opaque.
		// Simplification: return source.
		return color.NRGBA{R: r8, G: g8, B: b8, A: a8}
	case OperatorDest:
		// Dest: result is destination (fully transparent source)
		return color.NRGBA{R: 0, G: 0, B: 0, A: 0}
	case OperatorDestOver:
		// Dest Over: result is destination over source (source is transparent)
		return color.NRGBA{R: 0, G: 0, B: 0, A: 0}
	case OperatorDestIn:
		// Dest In: result is destination multiplied by source alpha
		// Simplification: return source with alpha applied.
		return color.NRGBA{R: uint8(float64(r8) * alpha), G: uint8(float64(g8) * alpha), B: uint8(float64(b8) * alpha), A: a8}
	case OperatorDestOut:
		// Dest Out: result is destination multiplied by (1 - source alpha)
		// Simplification: return source with inverted alpha.
		invAlpha := 1.0 - alpha
		return color.NRGBA{R: uint8(float64(r8) * invAlpha), G: uint8(float64(g8) * invAlpha), B: uint8(float64(b8) * invAlpha), A: uint8(float64(a8) * invAlpha)}
	case OperatorDestAtop:
		// Dest Atop: result is destination over source, but only where source is opaque.
		// Simplification: return source.
		return color.NRGBA{R: r8, G: g8, B: b8, A: a8}
	case OperatorXor:
		// Xor: result is source XOR destination
		// Simplification: return source.
		return color.NRGBA{R: r8, G: g8, B: b8, A: a8}
	case OperatorAdd:
		// Add: result is source + destination
		// Simplification: return source (would need destination to add properly).
		return color.NRGBA{R: r8, G: g8, B: b8, A: a8}
	case OperatorSaturate:
		// Saturate: result is source saturated by destination alpha
		// Simplification: return source.
		return color.NRGBA{R: r8, G: g8, B: b8, A: a8}
	default:
		return color.NRGBA{R: r8, G: g8, B: b8, A: a8}
	}
}

// TODO: Implement full pixel-level blending by replacing Pango's drawing mechanism
// with a custom one that uses image/draw.Drawer and applies the blend function.
