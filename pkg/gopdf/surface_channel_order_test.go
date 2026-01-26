package gopdf

import (
	"image/color"
	"testing"
)

func TestImageSurface_MarkDirty_ARGB32ChannelOrder(t *testing.T) {
	surface := NewImageSurface(FormatARGB32, 1, 1)
	defer surface.Destroy()

	imgSurf, ok := surface.(ImageSurface)
	if !ok {
		t.Fatalf("expected ImageSurface")
	}

	data := imgSurf.GetData()
	data[0] = 10
	data[1] = 20
	data[2] = 30
	data[3] = 255

	imgSurf.MarkDirty()

	c := color.RGBAModel.Convert(imgSurf.GetGoImage().At(0, 0)).(color.RGBA)
	if c.R != 30 || c.G != 20 || c.B != 10 || c.A != 255 {
		t.Fatalf("unexpected RGBA: got (%d,%d,%d,%d), want (30,20,10,255)", c.R, c.G, c.B, c.A)
	}
}

