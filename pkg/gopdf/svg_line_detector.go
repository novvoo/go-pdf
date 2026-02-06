package gopdf

import (
	"encoding/xml"
	"math"
	"sort"
)

// VisualToken represents a visible element in the SVG (or a group of them)
type VisualToken struct {
	Token    xml.Token   // The StartElement or standalone token
	Tokens   []xml.Token // Full subtree tokens
	Y        float64
	MaxY     float64
	X        float64
	Height   float64
	IsVisual bool
	Index    int // Original index in the token stream
}

// VisualLine represents a grouped line of visual elements
type VisualLine struct {
	Indices []int // Indices into the original visual tokens slice
	MinY    float64
	MaxY    float64
	AvgY    float64
}

// DetectVisualLines groups visual tokens into lines based on vertical proximity
func DetectVisualLines(visuals []VisualToken) []VisualLine {
	// We create a copy of pointers to sort them for grouping logic,
	// but we store original indices in VisualLine so the caller can map back.
	type vPtr struct {
		*VisualToken
		origIdx int
	}

	var sorted []vPtr
	for i := range visuals {
		if visuals[i].IsVisual {
			sorted = append(sorted, vPtr{&visuals[i], i})
		}
	}

	// Sort by Y (top) then X (left)
	sort.Slice(sorted, func(i, j int) bool {
		if math.Abs(sorted[i].Y-sorted[j].Y) < 1.0 {
			return sorted[i].X < sorted[j].X
		}
		return sorted[i].Y < sorted[j].Y
	})

	var lines []VisualLine

	for _, v := range sorted {
		// Try to find an existing line this fits into
		placed := false
		vMid := v.Y + v.Height/2

		for i := range lines {
			line := &lines[i]
			lineMid := (line.MinY + line.MaxY) / 2
			lineHeight := line.MaxY - line.MinY
			if lineHeight < 5.0 {
				lineHeight = 10.0 // Default min height for tolerance
			}

			// Tolerance logic:
			// 1. If overlaps significantly vertically
			// 2. Or if mid-points are very close (relative to height)
			
			// Overlap calculation
			overlapStart := math.Max(v.Y, line.MinY)
			overlapEnd := math.Min(v.MaxY, line.MaxY)
			overlap := overlapEnd - overlapStart
			
			// Midpoint distance
			dist := math.Abs(vMid - lineMid)

			// Criteria:
			// - Overlap > 50% of the smaller height
			// - OR dist < 20% of line height (strict alignment)
			
			minH := math.Min(v.Height, lineHeight)
			if minH <= 0 { minH = 1.0 }

			isSameLine := false
			if overlap > 0 && overlap > minH*0.5 {
				isSameLine = true
			} else if dist < 5.0 { // Hard tolerance for small fonts
				isSameLine = true
			}

			if isSameLine {
				line.Indices = append(line.Indices, v.origIdx)
				if v.Y < line.MinY {
					line.MinY = v.Y
				}
				if v.MaxY > line.MaxY {
					line.MaxY = v.MaxY
				}
				placed = true
				break
			}
		}

		if !placed {
			lines = append(lines, VisualLine{
				Indices: []int{v.origIdx},
				MinY:    v.Y,
				MaxY:    v.MaxY,
				AvgY:    v.Y, // Initial avg
			})
		}
	}

	return lines
}

func ParseVisualTokens(tokens []xml.Token) []VisualToken {
	var visuals []VisualToken

	i := 0
	for i < len(tokens) {
		t := tokens[i]
		start := i
		i++

		switch se := t.(type) {
		case xml.StartElement:
			// Consume subtree
			depth := 1
			for depth > 0 && i < len(tokens) {
				switch tokens[i].(type) {
				case xml.StartElement:
					depth++
				case xml.EndElement:
					depth--
				}
				i++
			}
			subtree := tokens[start:i]

			// Extract bounds using helper
			y, maxY, x, ok := ExtractBoundsFromTokens(subtree)
			if ok {
				visuals = append(visuals, VisualToken{
					Token:    se,
					Tokens:   subtree,
					Y:        y,
					MaxY:     maxY,
					X:        x,
					Height:   maxY - y,
					IsVisual: true,
					Index:    len(visuals),
				})
			} else {
				visuals = append(visuals, VisualToken{
					Token:    se,
					Tokens:   subtree,
					IsVisual: false,
					Index:    len(visuals),
				})
			}
		case xml.CharData, xml.Comment, xml.ProcInst, xml.Directive:
			visuals = append(visuals, VisualToken{
				Token:    se,
				Tokens:   []xml.Token{se},
				IsVisual: false,
				Index:    len(visuals),
			})
		}
	}
	return visuals
}

func ExtractBoundsFromTokens(tokens []xml.Token) (float64, float64, float64, bool) {
	if len(tokens) == 0 {
		return 0, 0, 0, false
	}

	for _, t := range tokens {
		if s, ok := t.(xml.StartElement); ok {
			var y, h, x float64
			var hasY, hasH bool
			var minY, maxY, minX, maxX float64 = 1e9, -1e9, 1e9, -1e9
			var foundBounds bool
			var transform *Matrix

			for _, a := range s.Attr {
				val := a.Value
				switch a.Name.Local {
				case "y":
					y, _ = ParseSVGDimension(val, 0)
					hasY = true
				case "x":
					x, _ = ParseSVGDimension(val, 0)
				case "height":
					h, _ = ParseSVGDimension(val, 0)
					hasH = true
				case "font-size":
					if !hasH {
						fs, _ := ParseSVGDimension(val, 0)
						h = fs
						hasH = true
					}
				case "d":
					path := ParseSVGPath(val)
					if path != nil && path.Status == StatusSuccess {
						for _, d := range path.Data {
							for _, p := range d.Points {
								if p.X < minX {
									minX = p.X
								}
								if p.X > maxX {
									maxX = p.X
								}
								if p.Y < minY {
									minY = p.Y
								}
								if p.Y > maxY {
									maxY = p.Y
								}
								foundBounds = true
							}
						}
					}
				case "points":
					pts, _ := ParseSVGPoints(val)
					for k := 0; k+1 < len(pts); k += 2 {
						px, py := pts[k], pts[k+1]
						if px < minX {
							minX = px
						}
						if px > maxX {
							maxX = px
						}
						if py < minY {
							minY = py
						}
						if py > maxY {
							maxY = py
						}
						foundBounds = true
					}
				case "transform":
					transform, _ = ParseSVGTransform(val)
				}
			}

			if foundBounds {
				if transform != nil {
					corners := []struct{ X, Y float64 }{
						{minX, minY}, {maxX, minY}, {maxX, maxY}, {minX, maxY},
					}
					minX, maxX, minY, maxY = 1e9, -1e9, 1e9, -1e9
					for _, c := range corners {
						tx, ty := transform.Transform(c.X, c.Y)
						if tx < minX {
							minX = tx
						}
						if tx > maxX {
							maxX = tx
						}
						if ty < minY {
							minY = ty
						}
						if ty > maxY {
							maxY = ty
						}
					}
				}
				return minY, maxY, minX, true
			}

			if hasY {
				if !hasH {
					h = 12.0
				}
				y1 := y
				y2 := y + h
				x1 := x

				if transform != nil {
					// Apply transform to text anchor/box
					// We project (x, y) and (x, y+h) to approximate vertical range
					tx1, ty1 := transform.Transform(x, y)
					_, ty2 := transform.Transform(x, y+h)

					if ty1 > ty2 {
						y1, y2 = ty2, ty1
					} else {
						y1, y2 = ty1, ty2
					}
					x1 = tx1
				}
				return y1, y2, x1, true
			}
		}
	}
	return 0, 0, 0, false
}
