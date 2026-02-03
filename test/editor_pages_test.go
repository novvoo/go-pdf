package test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

func TestReorderPagesFile(t *testing.T) {
	helper := NewTestHelper(t)
	mockGen := NewMockPDFGenerator()
	defer mockGen.Cleanup()

	inPath, err := mockGen.GenerateMultiPagePDF(3)
	helper.AssertNoError(err, "Failed to generate mock PDF")

	outPath := filepath.Join(filepath.Dir(inPath), "reordered.pdf")
	helper.CleanupFile(outPath)

	helper.AssertNoError(gopdf.ReorderPagesFile(inPath, outPath, []int{3, 1, 2}), "Failed to reorder pages")
	helper.AssertFileExists(outPath)

	r := gopdf.NewPDFReader(outPath)
	defer r.Close()

	pageCount, err := r.GetPageCount()
	helper.AssertNoError(err, "Failed to get page count")
	helper.AssertEqual(pageCount, 3, "Page count mismatch after reorder")

	elems, _ := r.ExtractPageElements(1)
	found := false
	for _, te := range elems {
		if strings.Contains(te.Text, "Page 3 of 3") {
			found = true
			break
		}
	}
	helper.AssertTrue(found, "Expected page 1 to contain content from original page 3")
}

func TestRemovePagesFile(t *testing.T) {
	helper := NewTestHelper(t)
	mockGen := NewMockPDFGenerator()
	defer mockGen.Cleanup()

	inPath, err := mockGen.GenerateMultiPagePDF(3)
	helper.AssertNoError(err, "Failed to generate mock PDF")

	outPath := filepath.Join(filepath.Dir(inPath), "removed.pdf")
	helper.CleanupFile(outPath)

	helper.AssertNoError(gopdf.RemovePagesFile(inPath, outPath, []int{2}), "Failed to remove pages")
	helper.AssertFileExists(outPath)

	r := gopdf.NewPDFReader(outPath)
	defer r.Close()

	pageCount, err := r.GetPageCount()
	helper.AssertNoError(err, "Failed to get page count")
	helper.AssertEqual(pageCount, 2, "Page count mismatch after removal")
}

