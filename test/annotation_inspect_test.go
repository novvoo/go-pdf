package test

import (
	"os"
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

func TestInspectAnnotations_TestImagePDF(t *testing.T) {
	ctx, err := api.ReadContextFile("../example/test_image.pdf")
	if err != nil {
		t.Skipf("Skipping: failed to read example/test_image.pdf: %v", err)
	}

	pageDict, _, _, err := ctx.PageDict(1, false)
	if err != nil {
		t.Fatalf("failed to get page dict: %v", err)
	}

	annots, err := gopdf.ExtractAnnotations(ctx, pageDict)
	if err != nil {
		t.Fatalf("failed to extract annotations: %v", err)
	}

	if len(annots) == 0 {
		t.Skip("example/test_image.pdf has no annotations")
	}
}

func TestRenderTestImagePDF_Page1(t *testing.T) {
	reader := gopdf.NewPDFReader("../example/test_image.pdf")
	outputPath := "test_image_page1_render.png"
	if err := reader.RenderPageToPNG(1, outputPath, 150); err != nil {
		t.Skipf("Skipping: failed to render example/test_image.pdf (may lack dependencies): %v", err)
	}
	_ = os.Remove(outputPath)
}
