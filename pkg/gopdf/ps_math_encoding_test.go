package gopdf

import "testing"

func TestPSMathEncoding_ContainsScriptL(t *testing.T) {
	s := "αβγθλπσϵℓ∑∏∫⊕⊗⊆⇒⇔≈∇∅∀∃∧∨∩∪→↦⊥∠"
	encoded := psEncodeWithRuneToByteMap(s, psDefaultMathRuneToByte())
	if len(encoded) != len([]rune(s)) {
		t.Fatalf("encoded len=%d want %d", len(encoded), len([]rune(s)))
	}
	for i, by := range encoded {
		if by == byte('?') {
			t.Fatalf("unmapped rune at index=%d", i)
		}
	}
}
