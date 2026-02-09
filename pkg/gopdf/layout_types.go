package gopdf

type LayoutElement struct {
	ID      string            `json:"id"`
	Page    int               `json:"page"`
	Kind    string            `json:"kind"`
	Text    string            `json:"text,omitempty"`
	Name    string            `json:"name,omitempty"`
	File    string            `json:"file,omitempty"`
	BBox    *LayoutBBox       `json:"bbox,omitempty"`
	Matrix  *LayoutMatrix     `json:"matrix,omitempty"`
	Raw     map[string]string `json:"raw,omitempty"`
	Approx  bool              `json:"approx"`
	Reading int               `json:"readingOrder,omitempty"`
}

type LayoutBBox struct {
	MinX float64 `json:"minX"`
	MinY float64 `json:"minY"`
	MaxX float64 `json:"maxX"`
	MaxY float64 `json:"maxY"`
}

func (b LayoutBBox) Width() float64  { return b.MaxX - b.MinX }
func (b LayoutBBox) Height() float64 { return b.MaxY - b.MinY }

func (b LayoutBBox) Overlaps(o LayoutBBox) bool {
	return b.MinX < o.MaxX && b.MaxX > o.MinX && b.MinY < o.MaxY && b.MaxY > o.MinY
}

func (b LayoutBBox) Contains(o LayoutBBox) bool {
	return b.MinX <= o.MinX && b.MinY <= o.MinY && b.MaxX >= o.MaxX && b.MaxY >= o.MaxY
}

type LayoutMatrix struct {
	A float64 `json:"a"`
	B float64 `json:"b"`
	C float64 `json:"c"`
	D float64 `json:"d"`
	E float64 `json:"e"`
	F float64 `json:"f"`
}

func layoutMatrixFromGopdf(m *Matrix) *LayoutMatrix {
	if m == nil {
		return nil
	}
	return &LayoutMatrix{
		A: m.XX,
		B: m.YX,
		C: m.XY,
		D: m.YY,
		E: m.X0,
		F: m.Y0,
	}
}

