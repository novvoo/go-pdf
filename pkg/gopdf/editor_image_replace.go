package gopdf

import (
	"fmt"
	"strings"
)

type ImageReplaceRequest struct {
	Page int

	ImageName string
	ImagePath string

	Opacity float64
}

func (r *PDFReader) ReplaceImageByNameTopLeftFile(outFile string, req ImageReplaceRequest) (int, error) {
	if r == nil || r.pdfPath == "" {
		return 0, fmt.Errorf("missing pdf reader")
	}
	if outFile == "" {
		return 0, fmt.Errorf("missing outFile")
	}
	name := strings.TrimSpace(req.ImageName)
	if name == "" {
		return 0, fmt.Errorf("missing image name")
	}
	imgPath := strings.TrimSpace(req.ImagePath)
	if imgPath == "" {
		return 0, fmt.Errorf("missing image path")
	}

	pages := []int{}
	if req.Page > 0 {
		pages = append(pages, req.Page)
	} else {
		n, err := r.GetPageCount()
		if err != nil {
			return 0, err
		}
		for i := 1; i <= n; i++ {
			pages = append(pages, i)
		}
	}

	var overlays []ImageOverlayTopLeft
	count := 0
	for _, pageNum := range pages {
		_, imgs := r.ExtractPageElements(pageNum)
		for _, im := range imgs {
			if strings.TrimSpace(im.Name) != name {
				continue
			}
			overlays = append(overlays, ImageOverlayTopLeft{
				Page:      pageNum,
				ImagePath: imgPath,
				X:         im.X,
				Y:         im.Y,
				Width:     im.Width,
				Height:    im.Height,
				Opacity:   req.Opacity,
				Rotation:  0,
				OnTop:     true,
			})
			count++
		}
	}

	if count == 0 {
		return 0, fmt.Errorf("no images matched")
	}
	if err := r.AddImageOverlaysTopLeftFile(outFile, overlays); err != nil {
		return 0, err
	}
	return count, nil
}

