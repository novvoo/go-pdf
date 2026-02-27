package gopdf

import (
	"image"
	"image/color"
	"os"
	"path/filepath"
	"testing"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func TestConvertPostScriptRasterToPDF(t *testing.T) {
	tmpDir := t.TempDir()
	psPath := filepath.Join(tmpDir, "in.ps")
	pdfPath := filepath.Join(tmpDir, "out.pdf")

	img := image.NewRGBA(image.Rect(0, 0, 4, 4))
	for y := 0; y < 4; y++ {
		for x := 0; x < 4; x++ {
			img.SetRGBA(x, y, color.RGBA{R: 255, G: 255, B: 255, A: 255})
		}
	}
	img.SetRGBA(1, 1, color.RGBA{R: 255, G: 0, B: 0, A: 255})

	if err := WritePostScriptFromImages(psPath, []image.Image{img}, 200, 100); err != nil {
		t.Fatalf("WritePostScriptFromImages: %v", err)
	}
	if fi, err := os.Stat(psPath); err != nil || fi.Size() == 0 {
		t.Fatalf("expected ps to be created: err=%v size=%v", err, func() int64 {
			if fi == nil {
				return 0
			}
			return fi.Size()
		}())
	}

	if err := ConvertPostScriptRasterToPDF(psPath, pdfPath); err != nil {
		t.Fatalf("ConvertPostScriptRasterToPDF: %v", err)
	}
	if fi, err := os.Stat(pdfPath); err != nil || fi.Size() == 0 {
		t.Fatalf("expected pdf to be created: err=%v size=%v", err, func() int64 {
			if fi == nil {
				return 0
			}
			return fi.Size()
		}())
	}
	pageCount, err := api.PageCountFile(pdfPath)
	if err != nil {
		t.Fatalf("PageCountFile: %v", err)
	}
	if pageCount != 1 {
		t.Fatalf("unexpected page count: got=%d want=1", pageCount)
	}
}

