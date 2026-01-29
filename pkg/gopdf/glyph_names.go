package gopdf

import (
	"strconv"
	"strings"
)

func glyphNameToRune(name string) (rune, bool) {
	if name == "" {
		return 0, false
	}
	name = strings.TrimPrefix(name, "/")
	if name == ".notdef" {
		return 0, false
	}

	if len(name) == 1 {
		return rune(name[0]), true
	}

	switch name {
	case "space":
		return ' ', true
	case "comma":
		return ',', true
	case "period", "dot":
		return '.', true
	case "colon":
		return ':', true
	case "semicolon":
		return ';', true
	case "parenleft":
		return '(', true
	case "parenright":
		return ')', true
	case "bracketleft":
		return '[', true
	case "bracketright":
		return ']', true
	case "braceleft":
		return '{', true
	case "braceright":
		return '}', true
	case "slash":
		return '/', true
	case "backslash":
		return '\\', true
	case "plus":
		return '+', true
	case "equal":
		return '=', true
	case "less":
		return '<', true
	case "greater":
		return '>', true
	case "bar":
		return '|', true
	case "underscore":
		return '_', true
	case "asterisk":
		return '*', true
	case "quotedbl":
		return '"', true
	case "quotesingle":
		return '\'', true
	}

	switch name {
	case "zero":
		return '0', true
	case "one":
		return '1', true
	case "two":
		return '2', true
	case "three":
		return '3', true
	case "four":
		return '4', true
	case "five":
		return '5', true
	case "six":
		return '6', true
	case "seven":
		return '7', true
	case "eight":
		return '8', true
	case "nine":
		return '9', true
	}

	if r, ok := greekGlyphNameToRune(name); ok {
		return r, true
	}
	if r, ok := commonMathGlyphNameToRune(name); ok {
		return r, true
	}

	if strings.HasPrefix(name, "uni") && len(name) == 7 {
		if v, err := strconv.ParseUint(name[3:], 16, 32); err == nil {
			return rune(v), true
		}
	}
	if strings.HasPrefix(name, "u") && (len(name) == 5 || len(name) == 7) {
		if v, err := strconv.ParseUint(name[1:], 16, 32); err == nil {
			return rune(v), true
		}
	}

	return 0, false
}

func greekGlyphNameToRune(name string) (rune, bool) {
	switch name {
	case "Alpha":
		return 'Α', true
	case "Beta":
		return 'Β', true
	case "Gamma":
		return 'Γ', true
	case "Delta":
		return 'Δ', true
	case "Epsilon":
		return 'Ε', true
	case "Zeta":
		return 'Ζ', true
	case "Eta":
		return 'Η', true
	case "Theta":
		return 'Θ', true
	case "Iota":
		return 'Ι', true
	case "Kappa":
		return 'Κ', true
	case "Lambda":
		return 'Λ', true
	case "Mu":
		return 'Μ', true
	case "Nu":
		return 'Ν', true
	case "Xi":
		return 'Ξ', true
	case "Omicron":
		return 'Ο', true
	case "Pi":
		return 'Π', true
	case "Rho":
		return 'Ρ', true
	case "Sigma":
		return 'Σ', true
	case "Tau":
		return 'Τ', true
	case "Upsilon":
		return 'Υ', true
	case "Phi":
		return 'Φ', true
	case "Chi":
		return 'Χ', true
	case "Psi":
		return 'Ψ', true
	case "Omega":
		return 'Ω', true
	case "alpha":
		return 'α', true
	case "beta":
		return 'β', true
	case "gamma":
		return 'γ', true
	case "delta":
		return 'δ', true
	case "epsilon":
		return 'ε', true
	case "zeta":
		return 'ζ', true
	case "eta":
		return 'η', true
	case "theta":
		return 'θ', true
	case "iota":
		return 'ι', true
	case "kappa":
		return 'κ', true
	case "lambda":
		return 'λ', true
	case "mu":
		return 'μ', true
	case "nu":
		return 'ν', true
	case "xi":
		return 'ξ', true
	case "omicron":
		return 'ο', true
	case "pi":
		return 'π', true
	case "rho":
		return 'ρ', true
	case "sigma":
		return 'σ', true
	case "tau":
		return 'τ', true
	case "upsilon":
		return 'υ', true
	case "phi":
		return 'φ', true
	case "chi":
		return 'χ', true
	case "psi":
		return 'ψ', true
	case "omega":
		return 'ω', true
	case "sigma1", "varsigma":
		return 'ς', true
	case "phi1", "varphi":
		return 'ϕ', true
	case "theta1", "vartheta":
		return 'ϑ', true
	case "pi1", "varpi":
		return 'ϖ', true
	case "omega1":
		return 'ω', true
	}
	return 0, false
}

func commonMathGlyphNameToRune(name string) (rune, bool) {
	switch name {
	case "minus", "hyphen":
		return '−', true
	case "plus":
		return '+', true
	case "times", "multiply":
		return '×', true
	case "divide":
		return '÷', true
	case "pm":
		return '±', true
	case "mp":
		return '∓', true
	case "leq", "lessequal":
		return '≤', true
	case "geq", "greaterequal":
		return '≥', true
	case "notequal", "neq":
		return '≠', true
	case "approx", "approxequal":
		return '≈', true
	case "equivalence":
		return '≡', true
	case "infinity", "infty":
		return '∞', true
	case "partialdiff", "partial":
		return '∂', true
	case "integral":
		return '∫', true
	case "summation":
		return '∑', true
	case "product":
		return '∏', true
	case "radical", "sqrt":
		return '√', true
	case "element", "in":
		return '∈', true
	case "notelement", "notin":
		return '∉', true
	case "union":
		return '∪', true
	case "intersection":
		return '∩', true
	case "subset":
		return '⊂', true
	case "superset":
		return '⊃', true
	case "subseteq":
		return '⊆', true
	case "superseteq":
		return '⊇', true
	case "logicaland":
		return '∧', true
	case "logicalor":
		return '∨', true
	case "arrowleft":
		return '←', true
	case "arrowright":
		return '→', true
	case "arrowup":
		return '↑', true
	case "arrowdown":
		return '↓', true
	case "degree":
		return '°', true
	case "bullet":
		return '•', true
	case "ellipsis", "ldots":
		return '…', true
	case "real":
		return 'ℜ', true
	case "Reals":
		return 'ℝ', true
	case "Integers":
		return 'ℤ', true
	case "Naturals":
		return 'ℕ', true
	case "Rationals":
		return 'ℚ', true
	}
	return 0, false
}
