package gopdf

import (
	"bufio"
	"compress/zlib"
	"encoding/ascii85"
	"fmt"
	"image"
	"os"
)

func (r *PDFReader) WritePostScriptA85Flate(outPath string, dpi float64) error {
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

	return WritePostScriptFromImagesA85Flate(outPath, images, pageInfo.Width, pageInfo.Height)
}

func WritePostScriptFromImagesA85Flate(outPath string, pages []image.Image, pageWidthPoints, pageHeightPoints float64) error {
	f, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := bufio.NewWriter(f)
	defer w.Flush()

	if _, err := fmt.Fprintf(w, "%%!PS-Adobe-3.0\n%%%%Creator: go-pdf\n%%%%LanguageLevel: 2\n%%%%DocumentData: Clean7Bit\n%%%%Pages: %d\n%%%%BoundingBox: 0 0 %.0f %.0f\n%%%%EndComments\n", len(pages), pageWidthPoints, pageHeightPoints); err != nil {
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
		if _, err := fmt.Fprintf(w, "%%%%PageBoundingBox: 0 0 %.0f %.0f\n", pageWidthPoints, pageHeightPoints); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "<< /PageSize [%.0f %.0f] >> setpagedevice\n", pageWidthPoints, pageHeightPoints); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "gsave\n%.8f %.8f scale\n", pageWidthPoints/float64(wpx), pageHeightPoints/float64(hpx)); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "/infile currentfile /ASCII85Decode filter /FlateDecode filter def\n/picstr %d string def\n%d %d 8 [1 0 0 -1 0 %d] { infile picstr readstring pop } false 3 colorimage\n", wpx*3, wpx, hpx, hpx); err != nil {
			return err
		}
		if err := writeImageA85FlateRGB(w, img); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "\n~>\ninfile closefile\n\ngrestore\nshowpage\n"); err != nil {
			return err
		}
	}

	_, err = fmt.Fprintf(w, "%%%%Trailer\n%%%%EOF\n")
	return err
}

func writeImageA85FlateRGB(w *bufio.Writer, img image.Image) error {
	b := img.Bounds()
	wpx := b.Dx()
	if wpx <= 0 {
		return nil
	}

	a85 := ascii85.NewEncoder(w)
	z := zlib.NewWriter(a85)
	row := make([]byte, wpx*3)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		idx := 0
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bb, _ := img.At(x, y).RGBA()
			row[idx+0] = byte(r >> 8)
			row[idx+1] = byte(g >> 8)
			row[idx+2] = byte(bb >> 8)
			idx += 3
		}
		if _, err := z.Write(row); err != nil {
			z.Close()
			a85.Close()
			return err
		}
	}
	if err := z.Close(); err != nil {
		a85.Close()
		return err
	}
	return a85.Close()
}
