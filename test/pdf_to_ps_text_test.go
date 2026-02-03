package test

import (
	"os"
	"strings"
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

func TestWritePostScriptBestEffortPreservesText(t *testing.T) {
	helper := NewTestHelper(t)
	pdfPath := helper.FindTestPDF("test_vector.pdf")

	tmp, err := os.CreateTemp("", "gopdf_pdf_to_ps_*.ps")
	if err != nil {
		t.Fatalf("create temp ps: %v", err)
	}
	psPath := tmp.Name()
	tmp.Close()
	defer os.Remove(psPath)

	reader := gopdf.NewPDFReader(pdfPath)
	defer reader.Close()

	if err := reader.WritePostScriptBestEffort(psPath, 144); err != nil {
		t.Fatalf("WritePostScriptBestEffort: %v", err)
	}
	b, err := os.ReadFile(psPath)
	if err != nil {
		t.Fatalf("read ps: %v", err)
	}
	s := string(b)
	if !strings.Contains(s, " setfont") {
		t.Fatalf("expected PS to contain setfont")
	}
	if !strings.Contains(s, " show") {
		t.Fatalf("expected PS to contain show")
	}
}
