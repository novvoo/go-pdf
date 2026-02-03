package gopdf

import (
	"fmt"
	"strings"
)

type TextReplaceRequest struct {
	Page int

	Find    string
	Replace string

	FontName string
	FillColor string

	BackgroundColor string
	Opacity         float64
}

func (r *PDFReader) ReplaceTextTopLeftFile(outFile string, req TextReplaceRequest) (int, error) {
	if r == nil || r.pdfPath == "" {
		return 0, fmt.Errorf("missing pdf reader")
	}
	if outFile == "" {
		return 0, fmt.Errorf("missing outFile")
	}
	find := strings.TrimSpace(req.Find)
	if find == "" {
		return 0, fmt.Errorf("missing find text")
	}
	replace := req.Replace

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

	var overlays []TextOverlayTopLeft
	count := 0

	for _, pageNum := range pages {
		elems, _ := r.ExtractPageElements(pageNum)
		for _, te := range elems {
			t := strings.TrimSpace(te.Text)
			if t == "" {
				continue
			}
			if !strings.Contains(t, find) {
				continue
			}
			newText := strings.ReplaceAll(te.Text, req.Find, replace)
			if strings.TrimSpace(newText) == "" {
				continue
			}

			fontName := req.FontName
			if strings.TrimSpace(fontName) == "" {
				fontName = "Helvetica"
			}
			fontSize := te.FontSize
			if fontSize <= 0 {
				fontSize = 12
			}

			fill := req.FillColor
			if strings.TrimSpace(fill) == "" {
				fill = "0 0 0"
			}
			bg := strings.TrimSpace(req.BackgroundColor)

			overlays = append(overlays, TextOverlayTopLeft{
				Page:            pageNum,
				Text:            newText,
				X:               te.X,
				Y:               te.Y,
				FontName:        fontName,
				FontSize:        fontSize,
				FillColor:       fill,
				BackgroundColor: bg,
				Opacity:         req.Opacity,
				OnTop:           true,
			})
			count++
		}
	}

	if count == 0 {
		return 0, fmt.Errorf("no matches")
	}
	if err := r.AddTextOverlaysTopLeftFile(outFile, overlays); err != nil {
		return 0, err
	}

	return count, nil
}

