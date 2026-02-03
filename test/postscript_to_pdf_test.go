package test

import (
	"bytes"
	"os"
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func TestConvertPostScriptToSVGAndPDF(t *testing.T) {
	helper := NewTestHelper(t)
	psPath := helper.FindTestPDF("test_vector.ps")

	tmpSVG, err := os.CreateTemp("", "gopdf_ps_to_svg_*.svg")
	if err != nil {
		t.Fatalf("create temp svg: %v", err)
	}
	svgPath := tmpSVG.Name()
	tmpSVG.Close()
	defer os.Remove(svgPath)

	tmp, err := os.CreateTemp("", "gopdf_ps_to_pdf_*.pdf")
	if err != nil {
		t.Fatalf("create temp pdf: %v", err)
	}
	outPath := tmp.Name()
	tmp.Close()
	defer os.Remove(outPath)

	if err := gopdf.ConvertPostScriptToPDFWithSVG(psPath, svgPath, outPath); err != nil {
		t.Fatalf("ConvertPostScriptToPDFWithSVG: %v", err)
	}
	if fi, err := os.Stat(svgPath); err != nil || fi.Size() == 0 {
		t.Fatalf("expected svg to be created: err=%v size=%v", err, func() int64 {
			if fi == nil {
				return 0
			}
			return fi.Size()
		}())
	}

	pageCount, err := api.PageCountFile(outPath)
	if err != nil {
		t.Fatalf("PageCountFile: %v", err)
	}
	if pageCount != 3 {
		t.Fatalf("unexpected page count: got=%d want=3", pageCount)
	}
}

func TestInsertTextOverlaysIntoSVG(t *testing.T) {
	helper := NewTestHelper(t)
	psPath := helper.FindTestPDF("test_vector.ps")

	tmpSVG, err := os.CreateTemp("", "gopdf_ps_to_svg_base_*.svg")
	if err != nil {
		t.Fatalf("create temp svg: %v", err)
	}
	baseSVG := tmpSVG.Name()
	tmpSVG.Close()
	defer os.Remove(baseSVG)

	if err := gopdf.ConvertPostScriptToSVG(psPath, baseSVG); err != nil {
		t.Fatalf("ConvertPostScriptToSVG: %v", err)
	}
	if b, err := os.ReadFile(baseSVG); err != nil || !bytes.Contains(b, []byte("data-module=")) {
		t.Fatalf("expected module groups in base svg: err=%v", err)
	}

	tmpSVG2, err := os.CreateTemp("", "gopdf_ps_to_svg_translated_*.svg")
	if err != nil {
		t.Fatalf("create temp svg: %v", err)
	}
	outSVG := tmpSVG2.Name()
	tmpSVG2.Close()
	defer os.Remove(outSVG)

	overlays := []gopdf.TextOverlayTopLeft{
		{Page: 1, Text: "TEST OVERLAY", X: 24, Y: 24, FontSize: 12, FillColor: "0 0 0"},
	}
	if err := gopdf.InsertTextOverlaysIntoSVG(baseSVG, outSVG, overlays); err != nil {
		t.Fatalf("InsertTextOverlaysIntoSVG: %v", err)
	}
	if fi, err := os.Stat(outSVG); err != nil || fi.Size() == 0 {
		t.Fatalf("expected output svg to exist: err=%v size=%v", err, func() int64 {
			if fi == nil {
				return 0
			}
			return fi.Size()
		}())
	}
	if b, err := os.ReadFile(outSVG); err != nil || !bytes.Contains(b, []byte("TEST OVERLAY")) || !bytes.Contains(b, []byte("<text")) {
		t.Fatalf("expected overlay text to be injected: err=%v", err)
	}
}
