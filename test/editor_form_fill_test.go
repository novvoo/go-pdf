package test

import (
	"path/filepath"
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

func TestFillFormFieldsMapFileNoForm(t *testing.T) {
	helper := NewTestHelper(t)
	mockGen := NewMockPDFGenerator()
	defer mockGen.Cleanup()

	inPath, err := mockGen.GeneratePDFWithText("No Form")
	helper.AssertNoError(err, "Failed to generate mock PDF")

	outPath := filepath.Join(filepath.Dir(inPath), "form_filled.pdf")
	helper.CleanupFile(outPath)

	err = gopdf.FillFormFieldsMapFile(inPath, outPath, map[string]string{"field": "value"})
	helper.AssertError(err, "Expected fill form to fail for PDF without forms")
}

