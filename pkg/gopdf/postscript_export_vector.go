package gopdf

import "fmt"

func (r *PDFReader) WritePostScriptVector(outPath string) error {
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
		info, err := r.GetPageInfo(pageNum)
		if err != nil {
			return err
		}
		if ps, ok := surface.(*psSurface); ok {
			ps.SetSize(info.Width, info.Height)
		}

		ctx := NewContext(surface)
		ctx.SetSourceRGB(1, 1, 1)
		ctx.Rectangle(0, 0, info.Width, info.Height)
		ctx.Fill()

		if err := renderPDFPageToGopdf(r.pdfPath, pageNum, ctx, info.Width, info.Height); err != nil {
			ctx.Destroy()
			return err
		}
		ctx.Destroy()
		surface.ShowPage()
	}

	return nil
}

