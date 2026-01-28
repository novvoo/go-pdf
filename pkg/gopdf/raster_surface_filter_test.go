package gopdf

import (
	"image"
	"image/color"
	"testing"
)

func TestSurfacePatternBilinearSampling(t *testing.T) {
	surface := NewImageSurface(FormatARGB32, 2, 2)
	defer surface.Destroy()

	imgSurf := surface.(ImageSurface)
	img, ok := imgSurf.GetGoImage().(*image.RGBA)
	if !ok || img == nil {
		t.Fatalf("expected RGBA go image")
	}

	img.SetRGBA(0, 0, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	img.SetRGBA(1, 0, color.RGBA{R: 0, G: 255, B: 0, A: 255})
	img.SetRGBA(0, 1, color.RGBA{R: 0, G: 0, B: 255, A: 255})
	img.SetRGBA(1, 1, color.RGBA{R: 255, G: 255, B: 255, A: 255})

	pattern := NewPatternForSurface(surface)
	pattern.SetExtend(ExtendPad)
	pattern.SetFilter(FilterBilinear)
	pattern.SetMatrix(NewIdentityMatrix())

	rc := &rasterContext{
		img:            img,
		color:          color.NRGBA{R: 0, G: 0, B: 0, A: 0},
		matrix:         *NewIdentityMatrix(),
		surfacePattern: pattern,
	}

	c := rc.getSurfacePatternColor(1.0, 1.0)
	r16, g16, b16, a16 := c.RGBA()
	if a16 == 0 {
		t.Fatalf("expected non-transparent sample")
	}
	r8 := uint8(r16 >> 8)
	g8 := uint8(g16 >> 8)
	b8 := uint8(b16 >> 8)

	if r8 < 110 || r8 > 145 || g8 < 110 || g8 > 145 || b8 < 110 || b8 > 145 {
		t.Fatalf("unexpected bilinear color: got rgb=(%d,%d,%d), want around 128", r8, g8, b8)
	}
}
