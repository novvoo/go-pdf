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
	if !strings.Contains(s, "%%Page:") {
		t.Fatalf("expected PS to contain page markers")
	}
	if !(strings.Contains(s, " show") || strings.Contains(s, " curveto") || strings.Contains(s, " lineto")) {
		t.Fatalf("expected PS to contain either text operators or vector paths")
	}
}

func TestWritePostScriptBestEffortRasterPagesNotRotated180(t *testing.T) {
	helper := NewTestHelper(t)
	pdfPath := helper.FindTestPDF("test.pdf")

	tmp, err := os.CreateTemp("", "gopdf_pdf_to_ps_rot_*.ps")
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
	if strings.Contains(s, "[1 0 0 -1 0") && strings.Contains(s, "colorimage") {
		t.Fatalf("expected best-effort PS to avoid double Y-flip for images")
	}
	if !strings.Contains(s, "[1 0 0 1 0 0]") {
		t.Fatalf("expected best-effort PS to include non-flipped image matrix")
	}
}
