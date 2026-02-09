package gopdf

import (
	"fmt"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

type xobjScanState struct {
	ctm       *Matrix
	resources *Resources
}

func (r *PDFReader) extractPageXObjectLayoutElements(pageNum int, imagesByName map[string]string, recurseForms bool) ([]LayoutElement, error) {
	ctx, err := r.getContext()
	if err != nil {
		return nil, err
	}
	if ctx == nil {
		return nil, fmt.Errorf("nil context")
	}

	pageDict, _, _, err := ctx.PageDict(pageNum, false)
	if err != nil {
		return nil, err
	}
	pageInfo, _ := r.GetPageInfo(pageNum)

	pageTransform := NewIdentityMatrix()
	pageTransform = pageTransform.Translate(0, pageInfo.Height)
	pageTransform = pageTransform.Scale(1, -1)

	if rotateObj, found := pageDict.Find("Rotate"); found {
		var rotation int
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

	resources := NewResources()
	if resourcesObj, found := pageDict.Find("Resources"); found {
		_ = loadResources(ctx, resourcesObj, resources)
	}

	contents, found := pageDict.Find("Contents")
	if !found {
		return nil, nil
	}
	streams, err := ExtractContentStreams(ctx, contents)
	if err != nil {
		return nil, err
	}
	var allContent []byte
	for _, s := range streams {
		allContent = append(allContent, s...)
		allContent = append(allContent, '\n')
	}
	if len(allContent) == 0 {
		return nil, nil
	}

	toks, err := TokenizeContentStreamWithOffsets(allContent)
	if err != nil {
		return nil, err
	}
	ops, err := ParseContentOps(toks)
	if err != nil {
		return nil, err
	}

	state := xobjScanState{
		ctm:       NewIdentityMatrix(),
		resources: resources,
	}

	var out []LayoutElement
	seq := 1

	var scanOps func(ops []ContentOp, state xobjScanState, root *Resources, depth int) error
	scanOps = func(ops []ContentOp, state xobjScanState, root *Resources, depth int) error {
		localStack := []xobjScanState{}
		ctm := state.ctm.Clone()
		res := state.resources
		for _, op := range ops {
			switch op.Name {
			case "q":
				localStack = append(localStack, xobjScanState{ctm: ctm.Clone(), resources: res})
			case "Q":
				if n := len(localStack); n > 0 {
					prev := localStack[n-1]
					localStack = localStack[:n-1]
					ctm = prev.ctm.Clone()
					res = prev.resources
				}
			case "cm":
				if len(op.Args) == 6 {
					m, ok := argsToMatrix(op.Args)
					if ok {
						ctm = ctm.Multiply(m)
					}
				}
			case "Do":
				if len(op.Args) != 1 {
					continue
				}
				name, ok := op.Args[0].(string)
				if !ok || name == "" {
					continue
				}
				if name[0] == '/' {
					name = name[1:]
				}
				xobj := (*XObject)(nil)
				if res != nil {
					xobj = res.GetXObject(name)
				}
				if xobj == nil && root != nil {
					xobj = root.GetXObject(name)
				}
				if xobj == nil {
					continue
				}

				deviceCTM := pageTransform.Multiply(ctm)
				kind := "xobject"
				if xobj.Subtype == "/Image" || xobj.Subtype == "Image" {
					kind = "image"
				}

				bbox, approx := xobjectBBoxInDevice(xobj, deviceCTM, ctm)
				el := LayoutElement{
					ID:     fmt.Sprintf("p%04d-x%04d", pageNum, seq),
					Page:   pageNum,
					Kind:   kind,
					Name:   name,
					BBox:   &bbox,
					Matrix: layoutMatrixFromGopdf(deviceCTM),
					Approx: approx,
				}
				if kind == "image" && imagesByName != nil {
					if f, ok := imagesByName[name]; ok {
						el.File = f
					}
				}
				seq++
				out = append(out, el)

				if recurseForms && depth < 8 && (xobj.Subtype == "/Form" || xobj.Subtype == "Form") && len(xobj.Stream) > 0 {
					formRes := xobj.Resources
					if formRes == nil {
						formRes = res
					}
					formCTM := ctm
					if xobj.Matrix != nil {
						formCTM = formCTM.Multiply(xobj.Matrix)
					}

					formToks, err := TokenizeContentStreamWithOffsets(xobj.Stream)
					if err != nil {
						continue
					}
					formOps, err := ParseContentOps(formToks)
					if err != nil {
						continue
					}
					_ = scanOps(formOps, xobjScanState{ctm: formCTM, resources: formRes}, root, depth+1)
				}
			}
		}
		return nil
	}

	if err := scanOps(ops, state, resources, 0); err != nil {
		return nil, err
	}
	return out, nil
}

func argsToMatrix(args []interface{}) (*Matrix, bool) {
	if len(args) != 6 {
		return nil, false
	}
	var f [6]float64
	for i := 0; i < 6; i++ {
		switch v := args[i].(type) {
		case float64:
			f[i] = v
		case int:
			f[i] = float64(v)
		default:
			return nil, false
		}
	}
	return &Matrix{XX: f[0], YX: f[1], XY: f[2], YY: f[3], X0: f[4], Y0: f[5]}, true
}

func xobjectBBoxInDevice(xobj *XObject, deviceCTM *Matrix, userCTM *Matrix) (LayoutBBox, bool) {
	if xobj == nil {
		return LayoutBBox{}, true
	}
	if xobj.Subtype == "/Form" || xobj.Subtype == "Form" {
		m := deviceCTM
		if xobj.Matrix != nil {
			m = deviceCTM.Multiply(xobj.Matrix)
		}
		if len(xobj.BBox) == 4 {
			return bboxFromRect(m, xobj.BBox[0], xobj.BBox[1], xobj.BBox[2], xobj.BBox[3]), false
		}
		_ = userCTM
		return bboxFromUnitSquare(m), true
	}
	return bboxFromUnitSquare(deviceCTM), false
}

func bboxFromUnitSquare(m *Matrix) LayoutBBox {
	x0, y0 := m.Transform(0, 0)
	x1, y1 := m.Transform(1, 0)
	x2, y2 := m.Transform(0, 1)
	x3, y3 := m.Transform(1, 1)
	minX := min4F(x0, x1, x2, x3)
	minY := min4F(y0, y1, y2, y3)
	maxX := max4F(x0, x1, x2, x3)
	maxY := max4F(y0, y1, y2, y3)
	return LayoutBBox{MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY}
}

func bboxFromRect(m *Matrix, x1, y1, x2, y2 float64) LayoutBBox {
	x0, y0 := m.Transform(x1, y1)
	x1p, y1p := m.Transform(x2, y1)
	x2p, y2p := m.Transform(x1, y2)
	x3, y3 := m.Transform(x2, y2)
	minX := min4F(x0, x1p, x2p, x3)
	minY := min4F(y0, y1p, y2p, y3)
	maxX := max4F(x0, x1p, x2p, x3)
	maxY := max4F(y0, y1p, y2p, y3)
	return LayoutBBox{MinX: minX, MinY: minY, MaxX: maxX, MaxY: maxY}
}

func min4F(a, b, c, d float64) float64 {
	m := a
	if b < m {
		m = b
	}
	if c < m {
		m = c
	}
	if d < m {
		m = d
	}
	return m
}

func max4F(a, b, c, d float64) float64 {
	m := a
	if b > m {
		m = b
	}
	if c > m {
		m = c
	}
	if d > m {
		m = d
	}
	return m
}
