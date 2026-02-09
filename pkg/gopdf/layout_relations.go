package gopdf

import (
	"math"
	"sort"
)

type Relation struct {
	From string `json:"from"`
	To   string `json:"to"`
	Type string `json:"type"`
}

type RelationGraph struct {
	Relations []Relation          `json:"relations,omitempty"`
	Nearest   map[string]Nearest  `json:"nearest"`
	Reading   map[string]ReadLink `json:"reading"`
}

type Nearest struct {
	Above   string `json:"above,omitempty"`
	Below   string `json:"below,omitempty"`
	LeftOf  string `json:"leftOf,omitempty"`
	RightOf string `json:"rightOf,omitempty"`
}

type ReadLink struct {
	Prev string `json:"prev,omitempty"`
	Next string `json:"next,omitempty"`
}

func AssignReadingOrder(elems []LayoutElement) []LayoutElement {
	type item struct {
		idx int
		x   float64
		y   float64
	}
	var items []item
	for i := range elems {
		if elems[i].BBox == nil {
			continue
		}
		b := elems[i].BBox
		items = append(items, item{idx: i, x: b.MinX, y: b.MaxY})
	}
	sort.Slice(items, func(i, j int) bool {
		yi, yj := items[i].y, items[j].y
		if math.Abs(yi-yj) > 2 {
			return yi > yj
		}
		return items[i].x < items[j].x
	})
	for order, it := range items {
		elems[it.idx].Reading = order + 1
	}
	return elems
}

func BuildRelations(elems []LayoutElement) RelationGraph {
	const eps = 0.5

	type boxed struct {
		id string
		b  LayoutBBox
	}
	var bs []boxed
	for _, e := range elems {
		if e.BBox == nil {
			continue
		}
		bs = append(bs, boxed{id: e.ID, b: *e.BBox})
	}

	var rels []Relation
	nearest := map[string]Nearest{}
	reading := map[string]ReadLink{}

	for _, a := range bs {
		nearest[a.id] = Nearest{}
	}

	type best struct {
		id   string
		dist float64
	}
	bestAbove := map[string]best{}
	bestBelow := map[string]best{}
	bestLeft := map[string]best{}
	bestRight := map[string]best{}

	for i := 0; i < len(bs); i++ {
		for j := i + 1; j < len(bs); j++ {
			a := bs[i]
			b := bs[j]

			if a.b.Overlaps(b.b) {
				rels = append(rels,
					Relation{From: a.id, To: b.id, Type: "overlaps"},
					Relation{From: b.id, To: a.id, Type: "overlaps"},
				)
			}
			if a.b.Contains(b.b) {
				rels = append(rels, Relation{From: a.id, To: b.id, Type: "contains"})
			} else if b.b.Contains(a.b) {
				rels = append(rels, Relation{From: b.id, To: a.id, Type: "contains"})
			}

			if a.b.MaxY <= b.b.MinY+eps {
				rels = append(rels, Relation{From: a.id, To: b.id, Type: "below"})
				if overlap1D(a.b.MinX, a.b.MaxX, b.b.MinX, b.b.MaxX) {
					d := b.b.MinY - a.b.MaxY
					if cur, ok := bestAbove[a.id]; !ok || d < cur.dist {
						bestAbove[a.id] = best{id: b.id, dist: d}
					}
					if cur, ok := bestBelow[b.id]; !ok || d < cur.dist {
						bestBelow[b.id] = best{id: a.id, dist: d}
					}
				}
			} else if b.b.MaxY <= a.b.MinY+eps {
				rels = append(rels, Relation{From: a.id, To: b.id, Type: "above"})
				if overlap1D(a.b.MinX, a.b.MaxX, b.b.MinX, b.b.MaxX) {
					d := a.b.MinY - b.b.MaxY
					if cur, ok := bestBelow[a.id]; !ok || d < cur.dist {
						bestBelow[a.id] = best{id: b.id, dist: d}
					}
					if cur, ok := bestAbove[b.id]; !ok || d < cur.dist {
						bestAbove[b.id] = best{id: a.id, dist: d}
					}
				}
			}

			if a.b.MaxX <= b.b.MinX+eps {
				rels = append(rels, Relation{From: a.id, To: b.id, Type: "leftOf"})
				if overlap1D(a.b.MinY, a.b.MaxY, b.b.MinY, b.b.MaxY) {
					d := b.b.MinX - a.b.MaxX
					if cur, ok := bestRight[a.id]; !ok || d < cur.dist {
						bestRight[a.id] = best{id: b.id, dist: d}
					}
					if cur, ok := bestLeft[b.id]; !ok || d < cur.dist {
						bestLeft[b.id] = best{id: a.id, dist: d}
					}
				}
			} else if b.b.MaxX <= a.b.MinX+eps {
				rels = append(rels, Relation{From: a.id, To: b.id, Type: "rightOf"})
				if overlap1D(a.b.MinY, a.b.MaxY, b.b.MinY, b.b.MaxY) {
					d := a.b.MinX - b.b.MaxX
					if cur, ok := bestLeft[a.id]; !ok || d < cur.dist {
						bestLeft[a.id] = best{id: b.id, dist: d}
					}
					if cur, ok := bestRight[b.id]; !ok || d < cur.dist {
						bestRight[b.id] = best{id: a.id, dist: d}
					}
				}
			}
		}
	}

	for id := range nearest {
		n := nearest[id]
		if v, ok := bestAbove[id]; ok {
			n.Above = v.id
		}
		if v, ok := bestBelow[id]; ok {
			n.Below = v.id
		}
		if v, ok := bestLeft[id]; ok {
			n.LeftOf = v.id
		}
		if v, ok := bestRight[id]; ok {
			n.RightOf = v.id
		}
		nearest[id] = n
	}

	type ord struct {
		id    string
		order int
	}
	var order []ord
	for _, e := range elems {
		if e.Reading > 0 {
			order = append(order, ord{id: e.ID, order: e.Reading})
		}
	}
	sort.Slice(order, func(i, j int) bool { return order[i].order < order[j].order })
	for i := range order {
		id := order[i].id
		link := reading[id]
		if i > 0 {
			link.Prev = order[i-1].id
		}
		if i+1 < len(order) {
			link.Next = order[i+1].id
		}
		reading[id] = link
	}

	return RelationGraph{Relations: rels, Nearest: nearest, Reading: reading}
}

func overlap1D(a0, a1, b0, b1 float64) bool {
	return a0 < b1 && a1 > b0
}

