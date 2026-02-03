package test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

func TestReplaceTextTopLeftFile(t *testing.T) {
	helper := NewTestHelper(t)
	mockGen := NewMockPDFGenerator()
	defer mockGen.Cleanup()

	inPath, err := mockGen.GeneratePDFWithText("Hello World")
	helper.AssertNoError(err, "Failed to generate mock PDF")

	outPath := filepath.Join(filepath.Dir(inPath), "text_replaced.pdf")
	helper.CleanupFile(outPath)

	r := gopdf.NewPDFReader(inPath)
	defer r.Close()

	n, err := r.ReplaceTextTopLeftFile(outPath, gopdf.TextReplaceRequest{
		Page:            1,
		Find:            "Hello World",
		Replace:         "Hi World",
		FontName:        "Helvetica",
		FillColor:       "0 0 0",
		BackgroundColor: "1 1 1",
		Opacity:         1,
	})
	helper.AssertNoError(err, "Failed to replace text")
	helper.AssertTrue(n > 0, "Expected at least one text match")
	helper.AssertFileExists(outPath)

	inStat, err := os.Stat(inPath)
	helper.AssertNoError(err, "Failed to stat input pdf")
	outStat, err := os.Stat(outPath)
	helper.AssertNoError(err, "Failed to stat output pdf")
	helper.AssertTrue(outStat.Size() > inStat.Size(), "Expected output PDF to grow after overlay")
}

