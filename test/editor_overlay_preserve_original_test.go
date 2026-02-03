package test

import (
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

func TestAddTextOverlayPreservesOriginalContent(t *testing.T) {
	helper := NewTestHelper(t)
	mockGen := NewMockPDFGenerator()
	defer mockGen.Cleanup()

	pdfPath, err := mockGen.GenerateMultiPagePDF(3)
	helper.AssertNoError(err, "Failed to generate mock PDF")

	reader := gopdf.NewPDFReader(pdfPath)
	defer reader.Close()

	origText, _ := reader.ExtractPageElements(2)
	helper.AssertTrue(len(origText) > 0, "Expected original text elements on page 2")

	var wantText string
	var wantSize float64
	for _, te := range origText {
		if te.Text == "Page 2 of 3" && te.FontSize > 0 {
			wantText = te.Text
			wantSize = te.FontSize
			break
		}
	}
	helper.AssertTrue(wantText != "", "Expected to find title text on page 2 in original PDF")

	outPath := filepath.Join(filepath.Dir(pdfPath), "overlay_out.pdf")
	helper.CleanupFile(outPath)

	overlays := []gopdf.TextOverlayTopLeft{
		{
			Page:      2,
			Text:      "TRANSLATED",
			X:         50,
			Y:         120,
			FontName:  "Helvetica",
			FontSize:  12,
			FillColor: "1 0 0",
			Opacity:   1,
			OnTop:     true,
		},
	}

	helper.AssertNoError(reader.AddTextOverlaysTopLeftFile(outPath, overlays), "Failed to add overlay")
	helper.AssertFileExists(outPath)

	origStat, err := os.Stat(pdfPath)
	helper.AssertNoError(err, "Failed to stat original PDF")
	outStat, err := os.Stat(outPath)
	helper.AssertNoError(err, "Failed to stat output PDF")
	helper.AssertTrue(outStat.Size() > origStat.Size(), "Expected output PDF size to increase after overlay")

	outReader := gopdf.NewPDFReader(outPath)
	defer outReader.Close()

	outPageCount, err := outReader.GetPageCount()
	helper.AssertNoError(err, "Failed to get output page count")
	helper.AssertEqual(outPageCount, 3, "Expected output page count to remain unchanged")

	outText, _ := outReader.ExtractPageElements(2)
	helper.AssertTrue(len(outText) > 0, "Expected text elements in output PDF")

	foundOriginal := false
	for _, te := range outText {
		if te.Text == wantText && math.Abs(te.FontSize-wantSize) < 0.5 {
			foundOriginal = true
		}
	}

	helper.AssertTrue(foundOriginal, "Expected original text (and its font size) to remain in output PDF")
}
