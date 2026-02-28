package gopdf

import (
	"fmt"
	"image"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

func (r *PDFReader) WritePostScriptBestEffort(outPath string, rasterDPI float64) error {
	ctx, err := api.ReadContextFile(r.pdfPath)
	if err != nil {
		return fmt.Errorf("failed to read PDF context: %w", err)
	}

	pageCount, err := r.GetPageCount()
	if err != nil {
		return err
	}
	if pageCount == 0 {
		return fmt.Errorf("no pages")
	}

	first, err := r.GetPageInfo(1)
	if err != nil {
		return err
	}

	surface := NewPSSurface(outPath, first.Width, first.Height)
	if surface == nil || surface.Status() != StatusSuccess {
		if surface == nil {
			return fmt.Errorf("failed to create ps surface")
		}
		return fmt.Errorf("failed to create ps surface: %v", surface.Status())
	}
	defer surface.Destroy()

	for pageNum := 1; pageNum <= pageCount; pageNum++ {
		pageDict, _, _, err := ctx.PageDict(pageNum, false)
		if err != nil {
			return fmt.Errorf("failed to get page dict: %w", err)
		}

		info, err := r.GetPageInfo(pageNum)
		if err != nil {
			return err
		}
		if ps, ok := surface.(*psSurface); ok {
			ps.SetSize(info.Width, info.Height)
		}

		needsRaster, err := pdfPageUsesAdvancedTransparencyFeatures(ctx, pageDict, pageNum)
		if err != nil {
			return err
		}

		if needsRaster {
			img, err := r.RenderPageToImage(pageNum, rasterDPI)
			if err != nil {
				return err
			}
			if err := psWriteRasterPage(surface, img, info.Width, info.Height); err != nil {
				return err
			}
			surface.ShowPage()
			continue
		}

		pageCtx := NewContext(surface)
		pageCtx.SetSourceRGB(1, 1, 1)
		pageCtx.Rectangle(0, 0, info.Width, info.Height)
		pageCtx.Fill()

		if err := renderPDFPageToGopdf(r.pdfPath, pageNum, pageCtx, info.Width, info.Height); err != nil {
			pageCtx.Destroy()
			return err
		}
		pageCtx.Destroy()
		surface.ShowPage()
	}

	return nil
}

func pdfPageUsesAdvancedTransparencyFeatures(pdfcpuCtx *model.Context, pageDict types.Dict, pageNum int) (bool, error) {
	res := NewResources()
	if resourcesObj, found := pageDict.Find("Resources"); found {
		if err := loadResources(pdfcpuCtx, resourcesObj, res); err != nil {
			return false, fmt.Errorf("failed to load resources: %w", err)
		}
	}

	pageContent, err := pdfcpuCtx.PageContent(pageDict, pageNum)
	if err != nil {
		return false, fmt.Errorf("failed to read page content: %w", err)
	}

	seen := map[string]bool{}
	if uses, err := scanContentForExtGStateUsage(res, pageContent, seen); err != nil {
		return false, err
	} else if uses {
		return true, nil
	}

	return false, nil
}

func scanContentForExtGStateUsage(res *Resources, content []byte, visitedForms map[string]bool) (bool, error) {
	if res == nil || len(content) == 0 {
		return false, nil
	}
	ops, err := ParseContentStream(content)
	if err != nil {
		return false, err
	}

	for _, op := range ops {
		switch t := op.(type) {
		case *OpSetGraphicsState:
			ext := res.GetExtGState(t.DictName)
			if ext == nil {
				continue
			}
			if bm, ok := ext["BM"].(string); ok {
				if bm != "" && bm != "Normal" && bm != "/Normal" {
					return true, nil
				}
			}
			if ca, ok := ext["ca"].(float64); ok {
				if ca != 1 {
					return true, nil
				}
			}
			if CA, ok := ext["CA"].(float64); ok {
				if CA != 1 {
					return true, nil
				}
			}
			if smask, ok := ext["SMask"]; ok && smask != nil {
				if s, ok := smask.(string); ok && (s == "None" || s == "/None") {
					continue
				}
				return true, nil
			}
		case *OpDoXObject:
			xobj := res.GetXObject(t.XObjectName)
			if xobj == nil {
				continue
			}
			if xobj.Subtype != "Form" && xobj.Subtype != "/Form" {
				continue
			}
			key := xobj.Subtype + ":" + t.XObjectName
			if visitedForms[key] {
				continue
			}
			visitedForms[key] = true
			childRes := res
			if xobj.Resources != nil {
				childRes = xobj.Resources
			}
			uses, err := scanContentForExtGStateUsage(childRes, xobj.Stream, visitedForms)
			if err != nil {
				return false, err
			}
			if uses {
				return true, nil
			}
		}
	}

	return false, nil
}

func psWriteRasterPage(surface Surface, img image.Image, pageWidthPoints, pageHeightPoints float64) error {
	if img == nil {
		return nil
	}
	ps, ok := surface.(*psSurface)
	if !ok || ps.writer == nil {
		return fmt.Errorf("ps surface not available")
	}
	ps.ensureInPage()

	b := img.Bounds()
	wpx := b.Dx()
	hpx := b.Dy()
	if wpx <= 0 || hpx <= 0 {
		return nil
	}

	scaleX := pageWidthPoints / float64(wpx)
	scaleY := pageHeightPoints / float64(hpx)

	if _, err := fmt.Fprintf(ps.writer, "gsave\n%.8f %.8f scale\n", scaleX, scaleY); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(ps.writer, "/infile currentfile /ASCIIHexDecode filter def\n/picstr %d string def\n", wpx*3); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(ps.writer, "%d %d 8 [1 0 0 1 0 0] { infile picstr readstring pop } false 3 colorimage\n", wpx, hpx); err != nil {
		return err
	}
	if err := writeImageHexRGB(ps.writer, img); err != nil {
		return err
	}
	if _, err := fmt.Fprintf(ps.writer, "\n>\ninfile closefile\n\ngrestore\n"); err != nil {
		return err
	}
	return ps.writer.Flush()
}
