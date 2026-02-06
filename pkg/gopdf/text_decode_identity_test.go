package gopdf

import (
	"fmt"
	"testing"

	"golang.org/x/image/font/gofont/goregular"
	"golang.org/x/image/font/sfnt"
)

func TestDecodeTextStringWithCIDs_IdentityType0_EmbeddedCMap(t *testing.T) {
	parsed, err := sfnt.Parse(goregular.TTF)
	if err != nil {
		t.Fatalf("sfnt.Parse: %v", err)
	}
	buf := &sfnt.Buffer{}
	gi, err := parsed.GlyphIndex(buf, 'A')
	if err != nil {
		t.Fatalf("GlyphIndex: %v", err)
	}
	if gi == 0 {
		t.Fatalf("expected non-zero glyph index for 'A'")
	}

	font := &Font{
		Subtype:             "Type0",
		IsIdentity:          true,
		EmbeddedFontData:    goregular.TTF,
		CIDToGIDMapIdentity: true,
	}

	text := fmt.Sprintf("<%04X>", uint16(gi))
	decoded, _, _ := decodeTextStringWithCIDs(text, nil, font)
	if decoded != "A" {
		t.Fatalf("decoded=%q want %q", decoded, "A")
	}
}

func TestDecodeTextStringWithCIDs_IdentityType0_CIDToGIDMap(t *testing.T) {
	parsed, err := sfnt.Parse(goregular.TTF)
	if err != nil {
		t.Fatalf("sfnt.Parse: %v", err)
	}
	buf := &sfnt.Buffer{}
	gi, err := parsed.GlyphIndex(buf, 'B')
	if err != nil {
		t.Fatalf("GlyphIndex: %v", err)
	}
	if gi == 0 {
		t.Fatalf("expected non-zero glyph index for 'B'")
	}

	const cid = 7
	mapping := make([]uint16, cid+1)
	mapping[cid] = uint16(gi)

	font := &Font{
		Subtype:          "Type0",
		IsIdentity:       true,
		EmbeddedFontData: goregular.TTF,
		CIDToGIDMap:      mapping,
	}

	text := fmt.Sprintf("<%04X>", cid)
	decoded, _, _ := decodeTextStringWithCIDs(text, nil, font)
	if decoded != "B" {
		t.Fatalf("decoded=%q want %q", decoded, "B")
	}
}

func TestDecodeTextStringWithCIDs_IdentityType0_MissingGlyph(t *testing.T) {
	font := &Font{
		Subtype:             "Type0",
		IsIdentity:          true,
		EmbeddedFontData:    goregular.TTF,
		CIDToGIDMapIdentity: true,
	}

	decoded, _, _ := decodeTextStringWithCIDs("<0000>", nil, font)
	if decoded != "�" {
		t.Fatalf("decoded=%q want replacement char", decoded)
	}
}

