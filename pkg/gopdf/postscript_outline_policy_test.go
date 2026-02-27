package gopdf

import "testing"

func TestShouldOutlineTextInPS(t *testing.T) {
	cases := []struct {
		name    string
		font    *Font
		text    string
		cids    []uint16
		stats   TextDecodeStats
		outline bool
	}{
		{name: "body-helvetica", font: &Font{BaseFont: "Helvetica"}, text: "time.", outline: false},
		{name: "bullet", font: &Font{BaseFont: "Helvetica"}, text: "•", outline: false},
		{name: "greek-theta", font: &Font{BaseFont: "Helvetica"}, text: "θ", outline: false},
		{name: "tex-math-font", font: &Font{BaseFont: "CMEX10"}, text: "(", outline: true},
		{name: "code-mono-font", font: &Font{BaseFont: "DejaVuSansMono"}, text: "for(i=0;i<10;i++)", outline: false},
		{name: "music-font", font: &Font{BaseFont: "Bravura"}, text: "\uE050", outline: false},
		{name: "latex-string", font: &Font{BaseFont: "Helvetica"}, text: "\\frac{1}{2}", outline: false},
		{name: "pua-rune", font: &Font{BaseFont: "Helvetica"}, text: "\uE000", outline: true},
		{
			name: "big-delimiter-glyphname",
			font: &Font{
				BaseFont:        "SomeMathFont",
				CodeToGlyphName: map[byte]string{0x01: "parenrightBigg"},
			},
			text:    ")",
			cids:    []uint16{0x01},
			outline: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := shouldOutlineTextInPS(tc.font, tc.text, tc.cids, tc.stats)
			if got != tc.outline {
				t.Fatalf("got=%v want=%v", got, tc.outline)
			}
		})
	}
}
