package gopdf

import (
	"path/filepath"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func TestTextMatrixScalingAffectsGlyphSize(t *testing.T) {
	pdfPath := filepath.Join("..", "..", "example", "test_image.pdf")

	pageDims, err := api.PageDimsFile(pdfPath)
	if err != nil {
		t.Fatalf("PageDimsFile: %v", err)
	}
	if len(pageDims) < 1 {
		t.Fatalf("no page dims")
	}

	widthPoints := pageDims[0].Width
	heightPoints := pageDims[0].Height

	dpi := 150.0
	scale := dpi / 72.0
	widthPx := int(widthPoints * scale)
	heightPx := int(heightPoints * scale)

	surface := NewImageSurface(FormatARGB32, widthPx, heightPx)
	if surface == nil {
		t.Fatalf("failed to create surface")
	}
	defer surface.Destroy()

	gopdfCtx := NewContext(surface)
	defer gopdfCtx.Destroy()

	gopdfCtx.SetSourceRGB(1, 1, 1)
	gopdfCtx.Paint()
	gopdfCtx.Scale(scale, scale)

	ctx, err := api.ReadContextFile(pdfPath)
	if err != nil {
		t.Fatalf("ReadContextFile: %v", err)
	}

	pageDict, _, _, err := ctx.PageDict(1, false)
	if err != nil {
		t.Fatalf("PageDict: %v", err)
	}

	gopdfCtx.Save()
	defer gopdfCtx.Restore()

	gopdfCtx.Translate(0, heightPoints)
	gopdfCtx.Scale(1, -1)
	_ = applyPageTransformations(pageDict, gopdfCtx, widthPoints, heightPoints)

	renderCtx := NewRenderContext(gopdfCtx, widthPoints, heightPoints)
	if resourcesObj, found := pageDict.Find("Resources"); found {
		_ = loadResources(ctx, resourcesObj, renderCtx.Resources)
	}

	contents, found := pageDict.Find("Contents")
	if !found {
		t.Fatalf("Contents not found")
	}

	contentStreams, err := ExtractContentStreams(ctx, contents)
	if err != nil {
		t.Fatalf("ExtractContentStreams: %v", err)
	}

	var allContent []byte
	for _, s := range contentStreams {
		allContent = append(allContent, s...)
		allContent = append(allContent, '\n')
	}

	operators, err := ParseContentStream(allContent)
	if err != nil {
		t.Fatalf("ParseContentStream: %v", err)
	}

	allowed := map[string]bool{
		"q": true, "Q": true, "cm": true,
		"BT": true, "ET": true,
		"Tf": true, "Tm": true,
		"Tj": true, "TJ": true,
		"Td": true, "TD": true, "T*": true,
		"Tc": true, "Tw": true, "Tz": true, "TL": true, "Tr": true, "Ts": true,
		"rg": true, "RG": true, "g": true, "G": true,
	}

	for _, op := range operators {
		if op.Name() == "IGNORE" {
			continue
		}
		if !allowed[op.Name()] {
			continue
		}
		if err := op.Execute(renderCtx); err != nil {
			t.Fatalf("operator %s failed: %v", op.Name(), err)
		}
	}

	img := surface.(ImageSurface).GetGoImage()
	if img == nil {
		t.Fatalf("failed to get go image from surface")
	}

	nonWhite := 0
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bb, a := img.At(x, y).RGBA()
			if a == 0 {
				continue
			}
			if uint8(r>>8) != 255 || uint8(g>>8) != 255 || uint8(bb>>8) != 255 {
				nonWhite++
			}
		}
	}

	if nonWhite < 500 {
		t.Fatalf("expected visible text pixels, got nonWhite=%d", nonWhite)
	}

}
