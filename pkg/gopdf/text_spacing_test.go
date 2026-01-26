package gopdf

import (
	"image"
	"image/color"
	"testing"
)

func TestRenderText_LatinWordDoesNotSplit(t *testing.T) {
	surface := NewImageSurface(FormatARGB32, 600, 200)
	defer surface.Destroy()

	imgSurf := surface.(ImageSurface)
	ctx := NewContext(surface)
	defer ctx.Destroy()

	ctx.SetSourceRGB(1, 1, 1)
	ctx.Paint()

	rc := NewRenderContext(ctx, 600, 200)
	rc.TextState.FontSize = 80
	rc.TextState.HorizontalScaling = 100
	rc.TextState.CharSpacing = 0
	rc.TextState.WordSpacing = 0
	rc.TextState.TextMatrix = NewIdentityMatrix()
	rc.TextState.TextMatrix.X0 = 20
	rc.TextState.TextMatrix.Y0 = 140
	rc.TextState.TextLineMatrix = rc.TextState.TextMatrix.Clone()
	rc.TextState.Font = &Font{
		BaseFont:     "Helvetica",
		Subtype:      "/Type0",
		DefaultWidth: 1000,
	}

	if err := renderText(rc, "demo", nil); err != nil {
		t.Fatalf("renderText failed: %v", err)
	}

	img, ok := imgSurf.GetGoImage().(*image.RGBA)
	if !ok || img == nil {
		t.Fatalf("expected RGBA go image")
	}

	minX, maxX, ok := darkPixelBounds(img, img.Bounds())
	if !ok {
		t.Fatalf("expected some rendered pixels")
	}
	width := maxX - minX + 1
	if width > 260 {
		t.Fatalf("word appears too widely spaced (bounds width=%d)", width)
	}
}

func TestRenderText_HorizontalScalingAffectsGlyphRendering(t *testing.T) {
	renderWithScaling := func(hScale float64) int {
		surface := NewImageSurface(FormatARGB32, 800, 240)
		defer surface.Destroy()

		imgSurf := surface.(ImageSurface)
		ctx := NewContext(surface)
		defer ctx.Destroy()

		ctx.SetSourceRGB(1, 1, 1)
		ctx.Paint()

		rc := NewRenderContext(ctx, 800, 240)
		rc.TextState.FontSize = 90
		rc.TextState.HorizontalScaling = hScale
		rc.TextState.CharSpacing = 0
		rc.TextState.WordSpacing = 0
		rc.TextState.TextMatrix = NewIdentityMatrix()
		rc.TextState.TextMatrix.X0 = 20
		rc.TextState.TextMatrix.Y0 = 170
		rc.TextState.TextLineMatrix = rc.TextState.TextMatrix.Clone()
		rc.TextState.Font = &Font{
			BaseFont:     "Helvetica",
			Subtype:      "/Type0",
			DefaultWidth: 1000,
		}

		if err := renderText(rc, "TEST", nil); err != nil {
			t.Fatalf("renderText failed: %v", err)
		}

		img, ok := imgSurf.GetGoImage().(*image.RGBA)
		if !ok || img == nil {
			t.Fatalf("expected RGBA go image")
		}

		minX, maxX, ok := darkPixelBounds(img, img.Bounds())
		if !ok {
			t.Fatalf("expected some rendered pixels")
		}
		return maxX - minX + 1
	}

	w100 := renderWithScaling(100)
	w50 := renderWithScaling(50)

	if w50 >= int(float64(w100)*0.8) {
		t.Fatalf("expected horizontal scaling to reduce width: w100=%d w50=%d", w100, w50)
	}
}

func TestRenderText_UsesPdfMetricsWhenReasonable(t *testing.T) {
	surface := NewImageSurface(FormatARGB32, 800, 240)
	defer surface.Destroy()

	ctx := NewContext(surface)
	defer ctx.Destroy()

	rc := NewRenderContext(ctx, 800, 240)
	rc.TextState.FontSize = 40
	rc.TextState.HorizontalScaling = 100
	rc.TextState.CharSpacing = 0
	rc.TextState.WordSpacing = 0
	rc.TextState.TextMatrix = NewIdentityMatrix()
	rc.TextState.TextLineMatrix = rc.TextState.TextMatrix.Clone()

	fontFamily := mapPDFFont("Helvetica")
	fontFace := NewPangoPdfFont(fontFamily, FontSlantNormal, FontWeightNormal)
	defer fontFace.Destroy()
	fontMatrix := NewMatrix()
	fontMatrix.InitScale(rc.TextState.FontSize, rc.TextState.FontSize)
	ctm := NewMatrix()
	ctm.InitIdentity()
	scaledFont := NewPangoPdfScaledFont(fontFace, fontMatrix, ctm, nil)
	defer scaledFont.Destroy()

	runText := "ABCD"
	pangoAdv := scaledFont.TextExtents(runText).XAdvance
	if pangoAdv <= 0 {
		t.Fatalf("expected positive pango advance")
	}

	targetAdv := pangoAdv * 0.8
	defaultWidth := targetAdv / (float64(len([]rune(runText))) * rc.TextState.FontSize) * 1000.0
	if defaultWidth <= 0 {
		t.Fatalf("expected positive default width")
	}

	rc.TextState.Font = &Font{
		BaseFont:     "Helvetica",
		Subtype:      "/Type0",
		DefaultWidth: defaultWidth,
	}

	if err := renderText(rc, runText, nil); err != nil {
		t.Fatalf("renderText failed: %v", err)
	}

	got := rc.TextState.TextMatrix.X0
	if diff := got - targetAdv; diff < -0.5 || diff > 0.5 {
		t.Fatalf("expected text matrix advance ~= %.2f, got %.2f", targetAdv, got)
	}
}

func darkPixelBounds(img image.Image, r image.Rectangle) (minX, maxX int, ok bool) {
	r = r.Intersect(img.Bounds())
	if r.Empty() {
		return 0, 0, false
	}

	minX = r.Max.X
	maxX = r.Min.X
	found := false
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
			if c.A > 0 && c.R < 200 && c.G < 200 && c.B < 200 {
				found = true
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
			}
		}
	}
	if !found {
		return 0, 0, false
	}
	return minX, maxX, true
}
