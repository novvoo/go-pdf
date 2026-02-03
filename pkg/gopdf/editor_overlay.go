package gopdf

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

type TextOverlay struct {
	Page int
	Text string

	X float64
	Y float64

	FontName string
	FontSize float64

	FillColor string
	BackgroundColor string
	Opacity   float64

	OnTop bool
}

func (t TextOverlay) normalize() TextOverlay {
	out := t
	if out.FontName == "" {
		out.FontName = "Helvetica"
	}
	if out.FontSize <= 0 {
		out.FontSize = 12
	}
	if out.FillColor == "" {
		out.FillColor = "0 0 0"
	}
	out.BackgroundColor = strings.TrimSpace(out.BackgroundColor)
	if out.Opacity <= 0 {
		out.Opacity = 1
	}
	if out.Opacity > 1 {
		out.Opacity = 1
	}
	return out
}

func watermarkForTextOverlay(o TextOverlay) (*model.Watermark, error) {
	o = o.normalize()
	fontSize := int(math.Round(o.FontSize))
	parts := []string{
		"position:bl",
		fmt.Sprintf("offset:%.4f %.4f", o.X, o.Y),
		"scalefactor:1 abs",
		"rotation:0",
		fmt.Sprintf("points:%d", fontSize),
		fmt.Sprintf("fontname:%s", o.FontName),
		fmt.Sprintf("fillcolor:%s", o.FillColor),
		fmt.Sprintf("opacity:%.4f", o.Opacity),
		"rendermode:0",
		"aligntext:left",
	}
	if o.BackgroundColor != "" {
		parts = append(parts, fmt.Sprintf("backgroundcolor:%s", o.BackgroundColor))
	}
	desc := strings.Join(parts, ", ")

	wm, err := api.TextWatermark(o.Text, desc, o.OnTop, false, types.POINTS)
	if err != nil {
		return nil, err
	}
	wm.Update = false
	return wm, nil
}

func AddTextOverlaysFile(inFile, outFile string, overlays []TextOverlay) error {
	if inFile == "" || outFile == "" {
		return fmt.Errorf("missing inFile or outFile")
	}
	if len(overlays) == 0 {
		return fmt.Errorf("missing overlays")
	}

	m := map[int][]*model.Watermark{}
	for _, o := range overlays {
		if o.Page <= 0 {
			return fmt.Errorf("invalid page: %d", o.Page)
		}
		wm, err := watermarkForTextOverlay(o)
		if err != nil {
			return err
		}
		m[o.Page] = append(m[o.Page], wm)
	}

	conf := model.NewDefaultConfiguration()
	conf.Unit = types.POINTS
	return api.AddWatermarksSliceMapFile(inFile, outFile, m, conf)
}

type TextOverlayTopLeft struct {
	Page int
	Text string

	X float64
	Y float64

	FontName string
	FontSize float64

	FillColor string
	BackgroundColor string
	Opacity   float64

	OnTop bool
}

func (r *PDFReader) AddTextOverlaysTopLeftFile(outFile string, overlays []TextOverlayTopLeft) error {
	if r == nil || r.pdfPath == "" {
		return fmt.Errorf("missing pdf reader")
	}
	if outFile == "" {
		return fmt.Errorf("missing outFile")
	}
	if len(overlays) == 0 {
		return fmt.Errorf("missing overlays")
	}

	ctx, err := api.ReadContextFile(r.pdfPath)
	if err != nil {
		return err
	}

	var out []TextOverlay
	for _, o := range overlays {
		if o.Page <= 0 {
			return fmt.Errorf("invalid page: %d", o.Page)
		}
		pageInfo, err := r.GetPageInfo(o.Page)
		if err != nil {
			return err
		}

		pageDict, _, _, err := ctx.PageDict(o.Page, false)
		if err != nil {
			return err
		}

		pageTransform := NewIdentityMatrix()
		pageTransform = pageTransform.Translate(0, pageInfo.Height)
		pageTransform = pageTransform.Scale(1, -1)

		if rotateObj, found := pageDict.Find("Rotate"); found {
			rotation := 0
			switch v := rotateObj.(type) {
			case types.Integer:
				rotation = int(v)
			case types.Float:
				rotation = int(v)
			}

			if rotation != 0 {
				rotation = rotation % 360
				switch rotation {
				case 90:
					pageTransform = pageTransform.Translate(pageInfo.Width, 0)
					pageTransform = pageTransform.Rotate(1.5707963267948966)
				case 180:
					pageTransform = pageTransform.Translate(pageInfo.Width, pageInfo.Height)
					pageTransform = pageTransform.Rotate(3.141592653589793)
				case 270:
					pageTransform = pageTransform.Translate(0, pageInfo.Height)
					pageTransform = pageTransform.Rotate(4.71238898038469)
				}
			}
		}

		inv, err := pageTransform.Invert()
		if err != nil {
			return err
		}
		pdfX, pdfY := inv.Transform(o.X, o.Y)

		out = append(out, TextOverlay{
			Page:      o.Page,
			Text:      o.Text,
			X:         pdfX,
			Y:         pdfY,
			FontName:  o.FontName,
			FontSize:  o.FontSize,
			FillColor: o.FillColor,
			BackgroundColor: o.BackgroundColor,
			Opacity:   o.Opacity,
			OnTop:     o.OnTop,
		})
	}

	return AddTextOverlaysFile(r.pdfPath, outFile, out)
}

func ParsePageSelection(page int) []string {
	if page <= 0 {
		return nil
	}
	return []string{strconv.Itoa(page)}
}
