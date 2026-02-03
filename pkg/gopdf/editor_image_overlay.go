package gopdf

import (
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"math"
	"os"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

type ImageOverlay struct {
	Page int

	ImagePath string

	X float64
	Y float64

	Width  float64
	Height float64

	Opacity  float64
	Rotation float64

	OnTop bool
}

func (o ImageOverlay) normalize() ImageOverlay {
	out := o
	out.ImagePath = strings.TrimSpace(out.ImagePath)
	if out.Opacity <= 0 {
		out.Opacity = 1
	}
	if out.Opacity > 1 {
		out.Opacity = 1
	}
	if math.IsNaN(out.Rotation) || math.IsInf(out.Rotation, 0) {
		out.Rotation = 0
	}
	return out
}

func watermarkForImageOverlay(o ImageOverlay) (*model.Watermark, error) {
	o = o.normalize()
	if o.ImagePath == "" {
		return nil, fmt.Errorf("missing image path")
	}
	if _, err := os.Stat(o.ImagePath); err != nil {
		return nil, err
	}

	f, err := os.Open(o.ImagePath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return nil, err
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return nil, fmt.Errorf("invalid image dimensions")
	}

	scale := 1.0
	if o.Width > 0 {
		scale = o.Width / float64(cfg.Width)
	} else if o.Height > 0 {
		scale = o.Height / float64(cfg.Height)
	}
	if scale <= 0 {
		scale = 1
	}

	desc := strings.Join([]string{
		"position:bl",
		fmt.Sprintf("offset:%.4f %.4f", o.X, o.Y),
		fmt.Sprintf("scalefactor:%.6f abs", scale),
		fmt.Sprintf("rotation:%.4f", o.Rotation),
		fmt.Sprintf("opacity:%.4f", o.Opacity),
	}, ", ")

	wm, err := api.ImageWatermark(o.ImagePath, desc, o.OnTop, false, types.POINTS)
	if err != nil {
		return nil, err
	}
	wm.Update = false
	return wm, nil
}

func AddImageOverlaysFile(inFile, outFile string, overlays []ImageOverlay) error {
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
		wm, err := watermarkForImageOverlay(o)
		if err != nil {
			return err
		}
		m[o.Page] = append(m[o.Page], wm)
	}

	conf := model.NewDefaultConfiguration()
	conf.Unit = types.POINTS
	return api.AddWatermarksSliceMapFile(inFile, outFile, m, conf)
}

type ImageOverlayTopLeft struct {
	Page int

	ImagePath string

	X float64
	Y float64

	Width  float64
	Height float64

	Opacity  float64
	Rotation float64

	OnTop bool
}

func (r *PDFReader) AddImageOverlaysTopLeftFile(outFile string, overlays []ImageOverlayTopLeft) error {
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

	var out []ImageOverlay
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

		pdfX, pdfY := inv.Transform(o.X, o.Y+o.Height)
		out = append(out, ImageOverlay{
			Page:      o.Page,
			ImagePath: o.ImagePath,
			X:         pdfX,
			Y:         pdfY,
			Width:     o.Width,
			Height:    o.Height,
			Opacity:   o.Opacity,
			Rotation:  o.Rotation,
			OnTop:     o.OnTop,
		})
	}

	return AddImageOverlaysFile(r.pdfPath, outFile, out)
}

