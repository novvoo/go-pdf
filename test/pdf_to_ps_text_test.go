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

	if err := reader.WritePostScriptBestEffortWithOptions(psPath, gopdf.PostScriptBestEffortOptions{RasterDPI: 144, ForceVector: false}); err != nil {
		t.Fatalf("WritePostScriptBestEffortWithOptions: %v", err)
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

func TestWritePostScriptBestEffortProducesText(t *testing.T) {
	helper := NewTestHelper(t)
	pdfPath := helper.FindTestPDF("test.pdf")

	tmp, err := os.CreateTemp("", "gopdf_pdf_to_ps_force_vector_*.ps")
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
	shows := gopdf.ExtractPostScriptShows(string(b))
	if len(shows) == 0 {
		t.Fatalf("expected PS to contain text show operators")
	}
}

func TestWritePostScriptBestEffortSplitTextAndImages(t *testing.T) {
	helper := NewTestHelper(t)
	pdfPath := helper.FindTestPDF("test_vector.pdf")

	textTmp, err := os.CreateTemp("", "gopdf_ps_text_*.ps")
	if err != nil {
		t.Fatalf("create temp text ps: %v", err)
	}
	textPSPath := textTmp.Name()
	textTmp.Close()
	defer os.Remove(textPSPath)

	imageTmp, err := os.CreateTemp("", "gopdf_ps_images_*.ps")
	if err != nil {
		t.Fatalf("create temp image ps: %v", err)
	}
	imagePSPath := imageTmp.Name()
	imageTmp.Close()
	defer os.Remove(imagePSPath)

	reader := gopdf.NewPDFReader(pdfPath)
	defer reader.Close()

	if err := reader.WritePostScriptBestEffortSplit(textPSPath, imagePSPath, 144); err != nil {
		t.Fatalf("WritePostScriptBestEffortSplit: %v", err)
	}

	textBytes, err := os.ReadFile(textPSPath)
	if err != nil {
		t.Fatalf("read text ps: %v", err)
	}
	imageBytes, err := os.ReadFile(imagePSPath)
	if err != nil {
		t.Fatalf("read image ps: %v", err)
	}

	textShows := gopdf.ExtractPostScriptShows(string(textBytes))
	imageShows := gopdf.ExtractPostScriptShows(string(imageBytes))
	if len(textShows) == 0 {
		t.Fatalf("expected text-only PS to contain shows")
	}
	if len(imageShows) != 0 {
		t.Fatalf("expected image-only PS to contain no shows, got=%d", len(imageShows))
	}
}
