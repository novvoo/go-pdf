package test

import (
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

func TestInspectTestImage(t *testing.T) {
	pdfPath := filepath.Join("..", "example", "test_image.pdf")
	reader := gopdf.NewPDFReader(pdfPath)

	pageInfo, err := reader.GetPageInfo(1)
	if err != nil {
		t.Fatalf("GetPageInfo: %v", err)
	}

	textElements, images := reader.ExtractPageElements(1)
	t.Logf("page: %.2fx%.2f points", pageInfo.Width, pageInfo.Height)
	t.Logf("text elements: %d", len(textElements))
	t.Logf("image elements: %d", len(images))
	for i, te := range textElements {
		t.Logf("text[%d]: pos=(%.2f,%.2f) font=%s size=%.2f text=%q", i, te.X, te.Y, te.FontName, te.FontSize, te.Text)
	}
	for i, img := range images {
		t.Logf("image[%d]: name=%s pos=(%.2f,%.2f) size=%.2fx%.2f", i, img.Name, img.X, img.Y, img.Width, img.Height)
	}
}

func TestDumpTestImageFontDict(t *testing.T) {
	pdfPath := filepath.Join("..", "example", "test_image.pdf")

	ctx, err := api.ReadContextFile(pdfPath)
	if err != nil {
		t.Fatalf("ReadContextFile: %v", err)
	}

	pageDict, _, _, err := ctx.PageDict(1, false)
	if err != nil {
		t.Fatalf("PageDict: %v", err)
	}

	resourcesObj, found := pageDict.Find("Resources")
	if !found {
		t.Fatalf("Resources not found")
	}
	if indRef, ok := resourcesObj.(types.IndirectRef); ok {
		derefObj, err := ctx.Dereference(indRef)
		if err != nil {
			t.Fatalf("Dereference resources: %v", err)
		}
		resourcesObj = derefObj
	}
	resourcesDict, ok := resourcesObj.(types.Dict)
	if !ok {
		t.Fatalf("Resources is not dict")
	}

	fontsObj, found := resourcesDict.Find("Font")
	if !found {
		t.Fatalf("Font not found")
	}
	if indRef, ok := fontsObj.(types.IndirectRef); ok {
		derefObj, err := ctx.Dereference(indRef)
		if err != nil {
			t.Fatalf("Dereference Font: %v", err)
		}
		fontsObj = derefObj
	}
	fontsDict, ok := fontsObj.(types.Dict)
	if !ok {
		t.Fatalf("Font is not dict")
	}

	fontObj, found := fontsDict.Find("C1")
	if !found {
		t.Fatalf("C1 not found")
	}
	if indRef, ok := fontObj.(types.IndirectRef); ok {
		derefObj, err := ctx.Dereference(indRef)
		if err != nil {
			t.Fatalf("Dereference C1: %v", err)
		}
		fontObj = derefObj
	}
	fontDict, ok := fontObj.(types.Dict)
	if !ok {
		t.Fatalf("C1 is not dict")
	}

	t.Logf("C1 keys: %v", keysOfDict(fontDict))
	if dfObj, found := fontDict.Find("DescendantFonts"); found {
		if indRef, ok := dfObj.(types.IndirectRef); ok {
			derefObj, err := ctx.Dereference(indRef)
			if err == nil {
				dfObj = derefObj
			}
		}
		if arr, ok := dfObj.(types.Array); ok && len(arr) > 0 {
			df0 := arr[0]
			if indRef, ok := df0.(types.IndirectRef); ok {
				derefObj, err := ctx.Dereference(indRef)
				if err == nil {
					df0 = derefObj
				}
			}
			if dfDict, ok := df0.(types.Dict); ok {
				t.Logf("Descendant[0] keys: %v", keysOfDict(dfDict))
				if cidToGid, found := dfDict.Find("CIDToGIDMap"); found {
					t.Logf("CIDToGIDMap type: %T", cidToGid)
				}
				if fdObj, found := dfDict.Find("FontDescriptor"); found {
					t.Logf("FontDescriptor type: %T", fdObj)
				}
			}
		}
	}
}

func TestDumpTestImageDecodedContent(t *testing.T) {
	pdfPath := filepath.Join("..", "example", "test_image.pdf")

	ctx, err := api.ReadContextFile(pdfPath)
	if err != nil {
		t.Fatalf("ReadContextFile: %v", err)
	}

	pageDict, _, _, err := ctx.PageDict(1, false)
	if err != nil {
		t.Fatalf("PageDict: %v", err)
	}

	contents, found := pageDict.Find("Contents")
	if !found {
		t.Fatalf("Contents not found")
	}

	contentStreams, err := gopdf.ExtractContentStreams(ctx, contents)
	if err != nil {
		t.Fatalf("ExtractContentStreams: %v", err)
	}

	for i, s := range contentStreams {
		if len(s) > 4000 {
			s = s[:4000]
		}
		t.Logf("content[%d] (%d bytes):\n%s", i, len(s), string(s))
	}
}

func TestParseColorSpaceOperators(t *testing.T) {
	ops, err := gopdf.ParseContentStream([]byte("/Cs1 cs 0.9372549 0.5490196 0 sc /Cs1 CS 0 0 0 SC"))
	if err != nil {
		t.Fatalf("ParseContentStream: %v", err)
	}
	foundCS := false
	foundcs := false
	foundSC := false
	foundsc := false
	for _, op := range ops {
		switch op.(type) {
		case *gopdf.OpSetStrokeColorSpace:
			foundCS = true
		case *gopdf.OpSetFillColorSpace:
			foundcs = true
		case *gopdf.OpSetStrokeColorN:
			foundSC = true
		case *gopdf.OpSetFillColorN:
			foundsc = true
		}
	}
	if !foundCS || !foundcs || !foundSC || !foundsc {
		t.Fatalf("expected CS/cs/SC/sc operators parsed; got CS=%v cs=%v SC=%v sc=%v", foundCS, foundcs, foundSC, foundsc)
	}
}

func keysOfDict(d types.Dict) []string {
	keys := make([]string, 0, len(d))
	for k := range d {
		keys = append(keys, k)
	}
	return keys
}

func TestType0FontEmbeddedDataLoaded(t *testing.T) {
	pdfPath := filepath.Join("..", "example", "test_image.pdf")
	reader := gopdf.NewPDFReader(pdfPath)
	fonts := reader.ExtractFontInfo(1)

	found := false
	for _, f := range fonts {
		if f.Name == "C1" {
			found = true
			if f.EmbeddedFontSize <= 0 {
				t.Fatalf("expected embedded font for %s, got %d", f.Name, f.EmbeddedFontSize)
			}
		}
	}
	if !found {
		t.Fatalf("C1 not found")
	}
}

func TestRenderTestImagePNG(t *testing.T) {
	pdfPath := filepath.Join("..", "example", "test_image.pdf")
	reader := gopdf.NewPDFReader(pdfPath)
	out := filepath.Join(t.TempDir(), "test_image.png")
	if err := reader.RenderPageToPNG(1, out, 150); err != nil {
		t.Fatalf("RenderPageToPNG: %v", err)
	}
	fi, err := os.Stat(out)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if fi.Size() <= 0 {
		t.Fatalf("empty output")
	}
}

func TestRenderTestImageNotFlippedVertically(t *testing.T) {
	pdfPath := filepath.Join("..", "example", "test_image.pdf")
	refPath := filepath.Join("..", "example", "test_image.png")

	refFile, err := os.Open(refPath)
	if err != nil {
		t.Fatalf("open ref: %v", err)
	}
	refImg, err := png.Decode(refFile)
	_ = refFile.Close()
	if err != nil {
		t.Fatalf("decode ref: %v", err)
	}

	reader := gopdf.NewPDFReader(pdfPath)
	out := filepath.Join(t.TempDir(), "test_image.png")
	if err := reader.RenderPageToPNG(1, out, 150); err != nil {
		t.Fatalf("RenderPageToPNG: %v", err)
	}

	outFile, err := os.Open(out)
	if err != nil {
		t.Fatalf("open out: %v", err)
	}
	outImg, err := png.Decode(outFile)
	_ = outFile.Close()
	if err != nil {
		t.Fatalf("decode out: %v", err)
	}

	if refImg.Bounds().Dx() != outImg.Bounds().Dx() || refImg.Bounds().Dy() != outImg.Bounds().Dy() {
		t.Fatalf("dimension mismatch ref=%v out=%v", refImg.Bounds(), outImg.Bounds())
	}

	w := refImg.Bounds().Dx()
	h := refImg.Bounds().Dy()
	crop := image.Rect(w/10, h/10, w*9/10, h*9/10)
	maeSame := meanAbsDiffRGB(refImg, outImg, crop, 7)
	maeFlip := meanAbsDiffRGBFlipY(refImg, outImg, crop, 7)

	if maeFlip+1e-6 < maeSame {
		t.Fatalf("render likely vertically flipped (maeSame=%.2f maeFlip=%.2f)", maeSame, maeFlip)
	}
}

func meanAbsDiffRGB(a, b image.Image, crop image.Rectangle, step int) float64 {
	if step <= 0 {
		step = 1
	}
	bounds := a.Bounds().Intersect(b.Bounds()).Intersect(crop)
	if bounds.Empty() {
		return 0
	}

	var sum float64
	var n float64
	for y := bounds.Min.Y; y < bounds.Max.Y; y += step {
		for x := bounds.Min.X; x < bounds.Max.X; x += step {
			ar, ag, ab, _ := a.At(x, y).RGBA()
			br, bg, bb, _ := b.At(x, y).RGBA()
			sum += math.Abs(float64(int(ar>>8)-int(br>>8))) +
				math.Abs(float64(int(ag>>8)-int(bg>>8))) +
				math.Abs(float64(int(ab>>8)-int(bb>>8)))
			n += 3
		}
	}
	if n == 0 {
		return 0
	}
	return sum / n
}

func meanAbsDiffRGBFlipY(ref, out image.Image, crop image.Rectangle, step int) float64 {
	if step <= 0 {
		step = 1
	}
	bounds := ref.Bounds().Intersect(out.Bounds()).Intersect(crop)
	if bounds.Empty() {
		return 0
	}

	h := ref.Bounds().Dy()
	var sum float64
	var n float64
	for y := bounds.Min.Y; y < bounds.Max.Y; y += step {
		ry := h - 1 - y
		for x := bounds.Min.X; x < bounds.Max.X; x += step {
			ar, ag, ab, _ := ref.At(x, ry).RGBA()
			br, bg, bb, _ := out.At(x, y).RGBA()
			sum += math.Abs(float64(int(ar>>8)-int(br>>8))) +
				math.Abs(float64(int(ag>>8)-int(bg>>8))) +
				math.Abs(float64(int(ab>>8)-int(bb>>8)))
			n += 3
		}
	}
	if n == 0 {
		return 0
	}
	return sum / n
}

func TestDumpTestImageContentOps(t *testing.T) {
	pdfPath := filepath.Join("..", "example", "test_image.pdf")

	ctx, err := api.ReadContextFile(pdfPath)
	if err != nil {
		t.Fatalf("ReadContextFile: %v", err)
	}

	pageDict, _, _, err := ctx.PageDict(1, false)
	if err != nil {
		t.Fatalf("PageDict: %v", err)
	}

	contents, found := pageDict.Find("Contents")
	if !found {
		t.Fatalf("Contents not found")
	}

	contentStreams, err := gopdf.ExtractContentStreams(ctx, contents)
	if err != nil {
		t.Fatalf("ExtractContentStreams: %v", err)
	}

	var allContent []byte
	for _, s := range contentStreams {
		allContent = append(allContent, s...)
		allContent = append(allContent, '\n')
	}

	ops, err := gopdf.ParseContentStream(allContent)
	if err != nil {
		t.Fatalf("ParseContentStream: %v", err)
	}

	maxOps := 200
	if len(ops) < maxOps {
		maxOps = len(ops)
	}

	for i := 0; i < maxOps; i++ {
		op := ops[i]
		switch v := op.(type) {
		case *gopdf.OpConcatMatrix:
			t.Logf("[%03d] cm %s", i, v.Matrix.String())
		case *gopdf.OpDoXObject:
			t.Logf("[%03d] Do %s", i, v.XObjectName)
		case *gopdf.OpSetFont:
			t.Logf("[%03d] Tf font=%s size=%.4f", i, v.FontName, v.FontSize)
		case *gopdf.OpSetTextMatrix:
			t.Logf("[%03d] Tm %s", i, v.Matrix.String())
		case *gopdf.OpShowText:
			t.Logf("[%03d] Tj text=%q", i, v.Text)
		default:
			name := op.Name()
			if (i >= 70 && i <= 120) || name == "Tj" || name == "TJ" || name == "BT" || name == "ET" || name == "Td" || name == "TD" || name == "T*" {
				t.Logf("[%03d] %s", i, name)
			}
		}
	}
}
