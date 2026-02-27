package gopdf

import "testing"

func TestDecodeTextStringWithCIDs_CMMI_BacktickIsScriptL(t *testing.T) {
	font := &Font{
		BaseFont: "CMMI10",
		CodeToGlyphName: map[byte]string{
			'`': "grave",
		},
	}

	decoded, _, _ := decodeTextStringWithCIDs("`", nil, font)
	if decoded != "ℓ" {
		t.Fatalf("decoded=%q want %q", decoded, "ℓ")
	}
}
