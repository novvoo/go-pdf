package test

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

func TestReplaceImageByNameTopLeftFile(t *testing.T) {
	helper := NewTestHelper(t)
	mockGen := NewMockPDFGenerator()
	defer mockGen.Cleanup()

	inPath, err := mockGen.GenerateSimplePDF()
	helper.AssertNoError(err, "Failed to generate mock PDF")

	imgPath := filepath.Join(filepath.Dir(inPath), "replacement.png")
	helper.CleanupFile(imgPath)

	img := image.NewRGBA(image.Rect(0, 0, 16, 16))
	for y := 0; y < 16; y++ {
		for x := 0; x < 16; x++ {
			img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
		}
	}
	f, err := os.Create(imgPath)
	helper.AssertNoError(err, "Failed to create png")
	helper.AssertNoError(png.Encode(f, img), "Failed to encode png")
	helper.AssertNoError(f.Close(), "Failed to close png")

	outPath := filepath.Join(filepath.Dir(inPath), "img_replaced.pdf")
	helper.CleanupFile(outPath)

	r := gopdf.NewPDFReader(inPath)
	defer r.Close()

	n, err := r.ReplaceImageByNameTopLeftFile(outPath, gopdf.ImageReplaceRequest{
		Page:      1,
		ImageName: "Im1",
		ImagePath: imgPath,
		Opacity:   1,
	})
	helper.AssertNoError(err, "Failed to replace image")
	helper.AssertTrue(n > 0, "Expected at least one image replaced")
	helper.AssertFileExists(outPath)

	inStat, err := os.Stat(inPath)
	helper.AssertNoError(err, "Failed to stat input pdf")
	outStat, err := os.Stat(outPath)
	helper.AssertNoError(err, "Failed to stat output pdf")
	helper.AssertTrue(outStat.Size() > inStat.Size(), "Expected output PDF to grow after overlay")
}

