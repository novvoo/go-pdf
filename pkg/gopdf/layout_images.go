package gopdf

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu"
)

type ExportPageImagesOptions struct {
	BaseName     string
	MaxPageDigits int
}

func (r *PDFReader) ExportPageImages(pageNum int, imagesDir string, opt ExportPageImagesOptions) (map[string]string, error) {
	ctx, err := r.getContext()
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, fmt.Errorf("nil context")
	}

	out := map[string]string{}
	mm, err := pdfcpu.ExtractPageImages(ctx, pageNum, false)
	if err != nil {
		return nil, err
	}
	for _, img := range mm {
		if img.Reader == nil {
			continue
		}
		qual := img.Name
		if img.Thumb {
			qual = "thumb"
		}
		ext := img.FileType
		if ext == "" {
			ext = "bin"
		}
		f := fmt.Sprintf("%s_%0*d_%s.%s", opt.BaseName, opt.MaxPageDigits, pageNum, qual, ext)
		dst := filepath.Join(imagesDir, f)
		w, err := os.Create(dst)
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(w, img); err != nil {
			_ = w.Close()
			return nil, err
		}
		if err := w.Close(); err != nil {
			return nil, err
		}
		out[img.Name] = filepath.ToSlash(filepath.Join("images", f))
	}
	for k, v := range out {
		out[k] = strings.ReplaceAll(v, "\\", "/")
	}
	return out, nil
}

