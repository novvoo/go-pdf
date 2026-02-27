package gopdf

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"sort"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

func ConvertPostScriptRasterToPDF(psPath, outPDF string) error {
	if psPath == "" || outPDF == "" {
		return fmt.Errorf("missing psPath or outPDF")
	}
	data, err := os.ReadFile(psPath)
	if err != nil {
		return err
	}
	pageW, pageH, err := parsePostScriptBoundingBox(string(data))
	if err != nil {
		return err
	}
	imgs, err := ExtractPostScriptColorImages(string(data), 0)
	if err != nil {
		return err
	}
	if len(imgs) == 0 {
		return fmt.Errorf("no colorimage blocks found")
	}

	sort.Slice(imgs, func(i, j int) bool {
		if imgs[i].PageNo == imgs[j].PageNo {
			return imgs[i].LineNo < imgs[j].LineNo
		}
		return imgs[i].PageNo < imgs[j].PageNo
	})

	tmpDir, err := os.MkdirTemp("", "gopdf_ps_raster_")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	pngPaths := make([]string, 0, len(imgs))
	for i, im := range imgs {
		if im.Width <= 0 || im.Height <= 0 {
			continue
		}
		rgba := image.NewRGBA(image.Rect(0, 0, im.Width, im.Height))
		p := 0
		for y := 0; y < im.Height; y++ {
			for x := 0; x < im.Width; x++ {
				rgba.Set(x, y, color.RGBA{R: im.RGB[p], G: im.RGB[p+1], B: im.RGB[p+2], A: 255})
				p += 3
			}
		}
		name := fmt.Sprintf("p%03d_%03d.png", maxInt(1, im.PageNo), i+1)
		outPath := filepath.Join(tmpDir, name)
		f, err := os.Create(outPath)
		if err != nil {
			return err
		}
		encErr := png.Encode(f, rgba)
		closeErr := f.Close()
		if encErr != nil {
			return encErr
		}
		if closeErr != nil {
			return closeErr
		}
		pngPaths = append(pngPaths, outPath)
	}
	if len(pngPaths) == 0 {
		return fmt.Errorf("no images decoded")
	}

	imp, err := api.Import("pos:full", types.POINTS)
	if err != nil {
		return err
	}
	imp.PageDim = &types.Dim{Width: pageW, Height: pageH}
	imp.InpUnit = types.POINTS

	if err := ensureDirForFile(outPDF); err != nil {
		return err
	}
	return api.ImportImagesFile(pngPaths, outPDF, imp, model.NewDefaultConfiguration())
}
