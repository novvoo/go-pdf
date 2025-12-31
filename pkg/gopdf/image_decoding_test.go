package gopdf

import (
	"image"
	"testing"
)

func TestDecodeImageXObject_SMask(t *testing.T) {
	// Create SMask XObject (DeviceGray, 8bpc)
	// 2x2 mask:
	// 0   (transparent) | 128 (semi)
	// 255 (opaque)      | 0   (transparent)
	maskData := []byte{0, 128, 255, 0}
	smaskXObj := &XObject{
		Subtype:          "Image",
		Width:            2,
		Height:           2,
		ColorSpace:       "DeviceGray",
		BitsPerComponent: 8,
		Stream:           maskData,
	}

	// Create Main XObject (DeviceRGB, 8bpc)
	// 2x2 red image
	imgData := []byte{
		255, 0, 0, 255, 0, 0,
		255, 0, 0, 255, 0, 0,
	}
	xobj := &XObject{
		Subtype:          "Image",
		Width:            2,
		Height:           2,
		ColorSpace:       "DeviceRGB",
		BitsPerComponent: 8,
		Stream:           imgData,
		SMask:            smaskXObj,
	}

	img, err := decodeImageXObject(xobj)
	if err != nil {
		t.Fatalf("Failed to decode image with SMask: %v", err)
	}

	// Check pixels
	// (0,0): Mask=0 -> Alpha=0
	_, _, _, a := img.At(0, 0).RGBA()
	if a != 0 {
		t.Errorf("At(0,0): Expected Alpha 0, got %d", a)
	}

	// Check pixels using direct Pix access to verify strict values set by applySMask

	// Direct Pix check (non-premultiplied)
	// (0,0) -> Mask 0 -> Alpha 0
	idx := 0
	if img.Pix[idx+3] != 0 {
		t.Errorf("Pix(0,0) Alpha: expected 0, got %d", img.Pix[idx+3])
	}

	// (1,0) -> Mask 128 -> Alpha 128
	idx = (0*2 + 1) * 4
	if img.Pix[idx+3] != 128 {
		t.Errorf("Pix(1,0) Alpha: expected 128, got %d", img.Pix[idx+3])
	}
	// Color should remain (it's non-premultiplied in memory for image.RGBA ?)
	// Actually standard image.RGBA uses pre-multiplied alpha?
	// "The Pix field holds the image's pixels, in R, G, B, A order. The pixel at (x, y) starts at Pix[(y-Rect.Min.Y)*Stride + (x-Rect.Min.X)*4]."
	// Go's image.RGBA documentation: "RGBA is an in-memory image whose At method returns color.RGBA values."
	// color.RGBA struct comments: "If you are using this to store a color in an image.RGBA, note that the Alpha field is ignored." - WAIT NO.
	// Let's check implementation of decodeDeviceRGB in reader.go.
	// It sets img.Pix[dstIdx+3] = 255. It sets raw values.
	// And applySMask modifies img.Pix[offset+3] = newAlpha.
	// So for image.RGBA, the raw bytes in Pix are R,G,B,A.
	// Standard Go image.RGBA assumes premultiplied alpha for drawing, but here we are just storing data.
	// However, if we just check Pix, we verify what we wrote.

	if img.Pix[idx+0] != 255 { // R
		t.Errorf("Pix(1,0) Red: expected 255, got %d", img.Pix[idx+0])
	}

	// (0,1) -> Mask 255 -> Alpha 255
	idx = (1*2 + 0) * 4
	if img.Pix[idx+3] != 255 {
		t.Errorf("Pix(0,1) Alpha: expected 255, got %d", img.Pix[idx+3])
	}
}

func TestDecodeImageXObject_Indexed(t *testing.T) {
	// 2x2 image, Indexed 2 colors
	// Palette: color 0 = Black (0,0,0), color 1 = Green (0,255,0)
	palette := []byte{0, 0, 0, 0, 255, 0}

	// Data: 0, 1, 1, 0 (indices)
	// 8 bits per component
	stream := []byte{0, 1, 1, 0}

	xobj := &XObject{
		Subtype:          "Image",
		Width:            2,
		Height:           2,
		ColorSpace:       "/Indexed",
		BitsPerComponent: 8,
		Stream:           stream,
		Palette:          palette,
	}

	img, err := decodeImageXObject(xobj)
	if err != nil {
		t.Fatalf("Failed to decode Indexed image: %v", err)
	}

	// (0,0) -> 0 -> Black
	checkPixel(t, img, 0, 0, 0, 0, 0, 255)

	// (1,0) -> 1 -> Green
	checkPixel(t, img, 1, 0, 0, 255, 0, 255)
}

func TestDecodeImageXObject_ICCBased_CMYK(t *testing.T) {
	// Simulate ICCBased with 4 components (CMYK)
	// 1 pixel: Cyan (1.0, 0, 0, 0) -> should be R=0, G=255, B=255 (roughly)
	// Data is 8bpc: 255, 0, 0, 0
	stream := []byte{255, 0, 0, 0}

	xobj := &XObject{
		Subtype:          "Image",
		Width:            1,
		Height:           1,
		ColorSpace:       "/ICCBased",
		ColorComponents:  4, // N=4
		BitsPerComponent: 8,
		Stream:           stream,
	}

	img, err := decodeImageXObject(xobj)
	if err != nil {
		t.Fatalf("Failed to decode ICCBased CMYK: %v", err)
	}

	r, g, b, _ := img.At(0, 0).RGBA()
	// Cyan means no Red.
	if (r >> 8) > 10 { // Allow small error margin
		t.Errorf("Expected Cyan (low Red), got R=%d", r>>8)
	}
	if (g >> 8) < 200 {
		t.Errorf("Expected Cyan (high Green), got G=%d", g>>8)
	}
	if (b >> 8) < 200 {
		t.Errorf("Expected Cyan (high Blue), got B=%d", b>>8)
	}
}

func TestDecodeImageXObject_DeviceCMYK_Conversion(t *testing.T) {
	// Test conversion formula
	// C=255, M=0, Y=255, K=0 (Green) -> R=0, G=255, B=0
	stream := []byte{255, 0, 255, 0}
	xobj := &XObject{
		Subtype:          "Image",
		Width:            1,
		Height:           1,
		ColorSpace:       "DeviceCMYK",
		BitsPerComponent: 8,
		Stream:           stream,
	}

	img, err := decodeImageXObject(xobj)
	if err != nil {
		t.Fatalf("Failed to decode DeviceCMYK: %v", err)
	}

	checkPixel(t, img, 0, 0, 0, 255, 0, 255)
}

func checkPixel(t *testing.T, img *image.RGBA, x, y int, r, g, b, a uint8) {
	t.Helper()
	idx := img.PixOffset(x, y)
	if img.Pix[idx+0] != r || img.Pix[idx+1] != g || img.Pix[idx+2] != b || img.Pix[idx+3] != a {
		t.Errorf("Pixel(%d,%d): expected (%d,%d,%d,%d), got (%d,%d,%d,%d)",
			x, y, r, g, b, a,
			img.Pix[idx+0], img.Pix[idx+1], img.Pix[idx+2], img.Pix[idx+3])
	}
}
