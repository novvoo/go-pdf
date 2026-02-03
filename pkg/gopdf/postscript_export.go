package gopdf

import (
	"bufio"
	"fmt"
	"image"
	"os"
)

func (r *PDFReader) WritePostScript(outPath string, dpi float64) error {
	pageCount, err := r.GetPageCount()
	if err != nil {
		return err
	}
	if pageCount == 0 {
		return fmt.Errorf("no pages")
	}
	pageInfo, err := r.GetPageInfo(1)
	if err != nil {
		return err
	}

	images := make([]image.Image, 0, pageCount)
	for i := 1; i <= pageCount; i++ {
		img, err := r.RenderPageToImage(i, dpi)
		if err != nil {
			return err
		}
		images = append(images, img)
	}

	return WritePostScriptFromImages(outPath, images, pageInfo.Width, pageInfo.Height)
}

func WritePostScriptFromImages(outPath string, pages []image.Image, pageWidthPoints, pageHeightPoints float64) error {
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	defer w.Flush()

	if _, err := fmt.Fprintf(w, "%%!PS-Adobe-3.0\n%%%%Creator: go-pdf\n%%%%Pages: %d\n%%%%BoundingBox: 0 0 %.0f %.0f\n%%%%EndComments\n", len(pages), pageWidthPoints, pageHeightPoints); err != nil {
		return err
	}

	for i, img := range pages {
		if img == nil {
			continue
		}
		b := img.Bounds()
		wpx := b.Dx()
		hpx := b.Dy()
		if wpx <= 0 || hpx <= 0 {
			continue
		}

		if _, err := fmt.Fprintf(w, "%%%%Page: %d %d\n", i+1, i+1); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "gsave\n0 %.4f translate\n%.8f %.8f scale\n", pageHeightPoints, pageWidthPoints/float64(wpx), pageHeightPoints/float64(hpx)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "%d %d 8 [%d 0 0 -%d 0 %d] {<\n", wpx, hpx, wpx, hpx, hpx); err != nil {
			return err
		}
		if err := writeImageHexRGB(w, img); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "\n>} false 3 colorimage\ngrestore\nshowpage\n"); err != nil {
			return err
		}
	}

	_, err = fmt.Fprintf(w, "%%%%Trailer\n%%%%EOF\n")
	return err
}

func writeImageHexRGB(w *bufio.Writer, img image.Image) error {
	b := img.Bounds()
	lineLen := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bb, _ := img.At(x, y).RGBA()
			if err := writeHexByte(w, byte(r>>8), &lineLen); err != nil {
				return err
			}
			if err := writeHexByte(w, byte(g>>8), &lineLen); err != nil {
				return err
			}
			if err := writeHexByte(w, byte(bb>>8), &lineLen); err != nil {
				return err
			}
		}
	}
	return nil
}

func writeHexByte(w *bufio.Writer, b byte, lineLen *int) error {
	const hex = "0123456789ABCDEF"
	if err := w.WriteByte(hex[b>>4]); err != nil {
		return err
	}
	if err := w.WriteByte(hex[b&0x0F]); err != nil {
		return err
	}
	*lineLen += 2
	if *lineLen >= 96 {
		if err := w.WriteByte('\n'); err != nil {
			return err
		}
		*lineLen = 0
	}
	return nil
}

