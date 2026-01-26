package test

import (
	"image"
	"image/color"
	"os"
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

func TestPangoTextUnderYAxisFlip(t *testing.T) {
	renderer := gopdf.NewPDFRenderer(200, 200)
	renderer.SetDPI(150)

	outputPath := "pango_yaxis_flip_test.png"
	err := renderer.RenderToPNG(outputPath, func(ctx gopdf.Context) {
		ctx.SetSourceRGB(1, 1, 1)
		ctx.Paint()

		ctx.SetSourceRGB(0, 0, 0)
		layout := ctx.PangoPdfCreateLayout().(*gopdf.PangoPdfLayout)
		fontDesc := gopdf.NewPangoFontDescription()
		fontDesc.SetFamily("sans-serif")
		fontDesc.SetSize(96)
		layout.SetFontDescription(fontDesc)
		layout.SetText("p")

		ctx.Translate(0, 200)
		ctx.Scale(1, -1)

		ctx.MoveTo(50, 100)
		ctx.PangoPdfShowText(layout)
	})
	if err != nil {
		t.Fatalf("Failed to render PNG: %v", err)
	}
	defer os.Remove(outputPath)

	img, err := loadAndValidateImage(outputPath)
	if err != nil {
		t.Fatalf("Failed to load output image: %v", err)
	}

	above := countDarkPixels(img, image.Rect(40, 60, 160, 99))
	below := countDarkPixels(img, image.Rect(40, 101, 160, 150))

	if below < 50 {
		t.Fatalf("Expected dark pixels below baseline, got %d (above=%d)", below, above)
	}
	if below <= above {
		t.Fatalf("Expected more dark pixels below baseline than above (below=%d, above=%d)", below, above)
	}
}

func countDarkPixels(img image.Image, r image.Rectangle) int {
	r = r.Intersect(img.Bounds())
	if r.Empty() {
		return 0
	}

	n := 0
	for y := r.Min.Y; y < r.Max.Y; y++ {
		for x := r.Min.X; x < r.Max.X; x++ {
			c := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
			if c.R < 160 && c.G < 160 && c.B < 160 && c.A > 0 {
				n++
			}
		}
	}
	return n
}

