package gopdf

import "testing"

func TestConvertGopdfSurfaceToImage_UsesGoImageBuffer(t *testing.T) {
	surf := NewImageSurface(FormatARGB32, 10, 10)
	imgSurf, ok := surf.(ImageSurface)
	if !ok {
		t.Fatalf("expected ImageSurface")
	}
	defer surf.Destroy()

	ctx := NewContext(surf)
	defer ctx.Destroy()

	ctx.SetSourceRGB(1, 1, 1)
	if err := ctx.Paint(); err != nil {
		t.Fatalf("paint: %v", err)
	}

	img := ConvertGopdfSurfaceToImage(imgSurf)
	if img == nil {
		t.Fatalf("expected non-nil image")
	}

	r, g, b, a := img.At(0, 0).RGBA()
	if a == 0 {
		t.Fatalf("expected non-zero alpha")
	}
	if r>>8 < 200 || g>>8 < 200 || b>>8 < 200 {
		t.Fatalf("expected white-ish pixel, got r=%d g=%d b=%d a=%d", r>>8, g>>8, b>>8, a>>8)
	}
}

