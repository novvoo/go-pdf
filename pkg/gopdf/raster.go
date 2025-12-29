package gopdf

import (
	"image"
	"image/color"
	"image/draw"
	"math"
)

// rasterContext is a simple rasterizer that replaces Pango.GraphicContext
type rasterContext struct {
	img    *image.RGBA
	color  color.Color
	stroke color.Color
	width  float64

	// Current path
	path []pathPoint

	// Transform matrix
	matrix Matrix

	// Line properties
	lineCap    LineCap
	lineJoin   LineJoin
	lineDash   []float64
	dashOffset float64

	// Gradient pattern (if set)
	gradientPattern Pattern

	// Surface pattern (if set)
	surfacePattern Pattern
}

type pathPoint struct {
	x, y       float64
	cp1x, cp1y float64 // First control point for curves
	cp2x, cp2y float64 // Second control point for curves
	op         pathPointOp
}

type pathPointOp int

const (
	opMoveTo pathPointOp = iota
	opLineTo
	opCurveTo
	opClose
)

type transformedPoint struct {
	x, y       float64
	cp1x, cp1y float64
	cp2x, cp2y float64
	op         pathPointOp
}

// newRasterContext creates a new raster context for the given image
func newRasterContext(img *image.RGBA) *rasterContext {
	return &rasterContext{
		img:    img,
		color:  color.Black,
		stroke: color.Black,
		width:  1.0,
		path:   make([]pathPoint, 0),
	}
}

// BeginPath starts a new path
func (r *rasterContext) BeginPath() {
	r.path = r.path[:0]
}

// MoveTo moves to a point
func (r *rasterContext) MoveTo(x, y float64) {
	r.path = append(r.path, pathPoint{x: x, y: y, op: opMoveTo})
}

// LineTo draws a line to a point
func (r *rasterContext) LineTo(x, y float64) {
	r.path = append(r.path, pathPoint{x: x, y: y, op: opLineTo})
}

// CubicCurveTo draws a cubic Bezier curve
func (r *rasterContext) CubicCurveTo(x1, y1, x2, y2, x3, y3 float64) {
	// Store the curve with its control points
	r.path = append(r.path, pathPoint{
		x: x3, y: y3,
		cp1x: x1, cp1y: y1,
		cp2x: x2, cp2y: y2,
		op: opCurveTo,
	})
	// Debug: Print when curve is added
	// fmt.Printf("[DEBUG] CubicCurveTo called: cp1=(%.2f,%.2f) cp2=(%.2f,%.2f) end=(%.2f,%.2f)\n", x1, y1, x2, y2, x3, y3)
}

// Close closes the current path
func (r *rasterContext) Close() {
	r.path = append(r.path, pathPoint{x: 0, y: 0, op: opClose})
}

// SetLineWidth sets the line width
func (r *rasterContext) SetLineWidth(width float64) {
	r.width = width
}

// SetLineCap sets the line cap style
func (r *rasterContext) SetLineCap(cap LineCap) {
	r.lineCap = cap
}

// SetLineJoin sets the line join style
func (r *rasterContext) SetLineJoin(join LineJoin) {
	r.lineJoin = join
}

// SetLineDash sets the line dash pattern
func (r *rasterContext) SetLineDash(dash []float64, offset float64) {
	r.lineDash = dash
	r.dashOffset = offset
}

// SetFillColor sets the fill color
func (r *rasterContext) SetFillColor(c color.Color) {
	r.color = c
}

// SetStrokeColor sets the stroke color
func (r *rasterContext) SetStrokeColor(c color.Color) {
	r.stroke = c
}

// SetMatrixTransform sets the transformation matrix
func (r *rasterContext) SetMatrixTransform(m [6]float64) {
	r.matrix = Matrix{
		XX: m[0], YX: m[1],
		XY: m[2], YY: m[3],
		X0: m[4], Y0: m[5],
	}
}

// SetFontSize sets the font size (placeholder)
func (r *rasterContext) SetFontSize(size float64) {
	// Placeholder - font rendering is handled separately
}

// SetGradientPattern sets a gradient pattern for filling
func (r *rasterContext) SetGradientPattern(pattern Pattern) {
	r.gradientPattern = pattern
}

// SetSurfacePattern sets a surface pattern for filling
func (r *rasterContext) SetSurfacePattern(pattern Pattern) {
	r.surfacePattern = pattern
	// Clear gradient pattern when surface pattern is set
	if pattern != nil {
		r.gradientPattern = nil
	}
}

// Stroke strokes the current path
func (r *rasterContext) Stroke() {
	if len(r.path) == 0 {
		return
	}

	var lastX, lastY float64
	var startX, startY float64
	hasStart := false

	for _, pt := range r.path {
		switch pt.op {
		case opMoveTo:
			lastX, lastY = pt.x, pt.y
			startX, startY = pt.x, pt.y
			hasStart = true
		case opLineTo:
			if hasStart {
				r.drawLine(lastX, lastY, pt.x, pt.y, r.stroke)
			}
			lastX, lastY = pt.x, pt.y
		case opCurveTo:
			if hasStart {
				// Draw curve by flattening it with high quality
				r.drawCurve(lastX, lastY, pt.cp1x, pt.cp1y, pt.cp2x, pt.cp2y, pt.x, pt.y, r.stroke)
			}
			lastX, lastY = pt.x, pt.y
		case opClose:
			if hasStart {
				r.drawLine(lastX, lastY, startX, startY, r.stroke)
			}
		}
	}
}

// drawCurve draws a cubic Bezier curve by flattening it adaptively
func (r *rasterContext) drawCurve(x0, y0, x1, y1, x2, y2, x3, y3 float64, c color.Color) {
	// Adaptive subdivision with high quality tolerance (smaller = smoother)
	r.drawCurveRecursive(x0, y0, x1, y1, x2, y2, x3, y3, c, 0.05, 0)
}

// drawCurveRecursive recursively subdivides and draws a cubic Bezier curve
func (r *rasterContext) drawCurveRecursive(x0, y0, x1, y1, x2, y2, x3, y3 float64, c color.Color, tolerance float64, depth int) {
	// Limit recursion depth to prevent stack overflow
	if depth > 12 {
		r.drawLine(x0, y0, x3, y3, c)
		return
	}

	// Check if curve is flat enough using distance from control points to line
	dx := x3 - x0
	dy := y3 - y0
	d2 := math.Abs((x1-x3)*dy - (y1-y3)*dx)
	d3 := math.Abs((x2-x3)*dy - (y2-y3)*dx)

	if (d2+d3)*(d2+d3) < tolerance*(dx*dx+dy*dy) {
		r.drawLine(x0, y0, x3, y3, c)
		return
	}

	// Subdivide curve using De Casteljau's algorithm
	x01 := (x0 + x1) / 2
	y01 := (y0 + y1) / 2
	x12 := (x1 + x2) / 2
	y12 := (y1 + y2) / 2
	x23 := (x2 + x3) / 2
	y23 := (y2 + y3) / 2
	x012 := (x01 + x12) / 2
	y012 := (y01 + y12) / 2
	x123 := (x12 + x23) / 2
	y123 := (y12 + y23) / 2
	x0123 := (x012 + x123) / 2
	y0123 := (y012 + y123) / 2

	// Recursively draw both halves
	r.drawCurveRecursive(x0, y0, x01, y01, x012, y012, x0123, y0123, c, tolerance, depth+1)
	r.drawCurveRecursive(x0123, y0123, x123, y123, x23, y23, x3, y3, c, tolerance, depth+1)
}

// Fill fills the current path with antialiasing
func (r *rasterContext) Fill() {
	if len(r.path) == 0 {
		return
	}

	bounds := r.img.Bounds()

	// Transform path points to device space and find bounding box
	transformedPath := make([]transformedPoint, len(r.path))
	minX, minY := math.MaxFloat64, math.MaxFloat64
	maxX, maxY := -math.MaxFloat64, -math.MaxFloat64

	for i, pt := range r.path {
		// Transform endpoint
		tx, ty := MatrixTransformPoint(&r.matrix, pt.x, pt.y)
		transformedPath[i].x = tx
		transformedPath[i].y = ty
		transformedPath[i].op = pt.op

		if pt.op == opMoveTo || pt.op == opLineTo || pt.op == opCurveTo {
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

			// For curves, also transform and check control points
			if pt.op == opCurveTo {
				cp1x, cp1y := MatrixTransformPoint(&r.matrix, pt.cp1x, pt.cp1y)
				cp2x, cp2y := MatrixTransformPoint(&r.matrix, pt.cp2x, pt.cp2y)
				transformedPath[i].cp1x = cp1x
				transformedPath[i].cp1y = cp1y
				transformedPath[i].cp2x = cp2x
				transformedPath[i].cp2y = cp2y

				if cp1x < minX {
					minX = cp1x
				}
				if cp1x > maxX {
					maxX = cp1x
				}
				if cp1y < minY {
					minY = cp1y
				}
				if cp1y > maxY {
					maxY = cp1y
				}
				if cp2x < minX {
					minX = cp2x
				}
				if cp2x > maxX {
					maxX = cp2x
				}
				if cp2y < minY {
					minY = cp2y
				}
				if cp2y > maxY {
					maxY = cp2y
				}
			}
		}
	}

	// Clip to image bounds
	x1 := int(math.Max(minX-1, float64(bounds.Min.X)))
	y1 := int(math.Max(minY-1, float64(bounds.Min.Y)))
	x2 := int(math.Min(maxX+1, float64(bounds.Max.X)))
	y2 := int(math.Min(maxY+1, float64(bounds.Max.Y)))

	// Fill using supersampling antialiasing (4x4 grid per pixel)
	const samples = 4
	const invSamples = 1.0 / (samples * samples)

	pixelCount := 0
	for y := y1; y < y2; y++ {
		for x := x1; x < x2; x++ {
			pixelCount++
			// Count how many subpixel samples are inside the path
			coverage := 0
			for sy := 0; sy < samples; sy++ {
				for sx := 0; sx < samples; sx++ {
					// Sample at subpixel position
					sampleX := float64(x) + (float64(sx)+0.5)/float64(samples)
					sampleY := float64(y) + (float64(sy)+0.5)/float64(samples)
					if r.pointInTransformedPath(sampleX, sampleY, transformedPath) {
						coverage++
					}
				}
			}

			// Apply antialiasing based on coverage
			if coverage > 0 {
				alpha := float64(coverage) * invSamples
				// Use surface pattern, gradient, or solid color
				pixelColor := r.color
				if r.surfacePattern != nil {
					pixelColor = r.getSurfacePatternColor(float64(x), float64(y))
				} else if r.gradientPattern != nil {
					pixelColor = r.getGradientColor(float64(x), float64(y))
				}
				r.blendPixel(x, y, pixelColor, alpha)
			}
		}
	}
}

// blendPixel blends a color with the existing pixel using premultiplied alpha blending
// This matches Gopdf's blending behavior which uses premultiplied alpha
func (r *rasterContext) blendPixel(x, y int, c color.Color, alpha float64) {
	if x < 0 || y < 0 || x >= r.img.Bounds().Dx() || y >= r.img.Bounds().Dy() {
		return
	}

	// Get source color components (non-premultiplied)
	sr, sg, sb, sa := c.RGBA()
	srcR := float64(sr>>8) / 255.0
	srcG := float64(sg>>8) / 255.0
	srcB := float64(sb>>8) / 255.0
	srcA := float64(sa>>8) / 255.0 * alpha

	// Get destination color (RGBA format is non-premultiplied)
	dst := r.img.At(x, y)
	dr, dg, db, da := dst.RGBA()
	dstR := float64(dr>>8) / 255.0
	dstG := float64(dg>>8) / 255.0
	dstB := float64(db>>8) / 255.0
	dstA := float64(da>>8) / 255.0

	// Premultiply source color
	srcRp := srcR * srcA
	srcGp := srcG * srcA
	srcBp := srcB * srcA

	// Premultiply destination color
	dstRp := dstR * dstA
	dstGp := dstG * dstA
	dstBp := dstB * dstA

	// Porter-Duff "over" operator with premultiplied alpha:
	// result = src + dst * (1 - srcA)
	outA := srcA + dstA*(1-srcA)
	outRp := srcRp + dstRp*(1-srcA)
	outGp := srcGp + dstGp*(1-srcA)
	outBp := srcBp + dstBp*(1-srcA)

	// Unpremultiply for RGBA output
	var outR, outG, outB float64
	if outA > 0.0001 {
		outR = outRp / outA
		outG = outGp / outA
		outB = outBp / outA
	}

	// Clamp and convert to uint8
	result := color.NRGBA{
		R: uint8(math.Min(math.Max(outR*255, 0), 255)),
		G: uint8(math.Min(math.Max(outG*255, 0), 255)),
		B: uint8(math.Min(math.Max(outB*255, 0), 255)),
		A: uint8(math.Min(math.Max(outA*255, 0), 255)),
	}

	r.img.Set(x, y, result)
}

// pointInTransformedPath checks if a point is inside a transformed path
func (r *rasterContext) pointInTransformedPath(x, y float64, path []transformedPoint) bool {
	winding := 0
	var lastX, lastY float64
	var startX, startY float64
	hasStart := false

	for _, pt := range path {
		switch pt.op {
		case opMoveTo:
			lastX, lastY = pt.x, pt.y
			startX, startY = pt.x, pt.y
			hasStart = true
		case opLineTo:
			if hasStart {
				if crossesRay(lastX, lastY, pt.x, pt.y, x, y) {
					if lastY <= y {
						winding++
					} else {
						winding--
					}
				}
			}
			lastX, lastY = pt.x, pt.y
		case opCurveTo:
			if hasStart {
				// For curves, check crossings along the curve
				winding += curveCrossings(lastX, lastY, pt.cp1x, pt.cp1y, pt.cp2x, pt.cp2y, pt.x, pt.y, x, y)
			}
			lastX, lastY = pt.x, pt.y
		case opClose:
			if hasStart {
				if crossesRay(lastX, lastY, startX, startY, x, y) {
					if lastY <= y {
						winding++
					} else {
						winding--
					}
				}
			}
		}
	}

	return winding != 0
}

// drawLine draws an antialiased line with specified width
func (r *rasterContext) drawLine(x0, y0, x1, y1 float64, c color.Color) {
	// Transform points
	x0t, y0t := MatrixTransformPoint(&r.matrix, x0, y0)
	x1t, y1t := MatrixTransformPoint(&r.matrix, x1, y1)

	// Calculate line direction and length
	dx := x1t - x0t
	dy := y1t - y0t
	length := math.Sqrt(dx*dx + dy*dy)

	if length < 0.01 {
		// Line is too short, just draw a point
		r.drawAntialiasedCircle(x0t, y0t, r.width/2, c)
		return
	}

	// Normalize direction
	dx /= length
	dy /= length

	// Calculate bounding box
	halfWidth := r.width / 2
	minX := math.Min(x0t, x1t) - halfWidth - 1
	maxX := math.Max(x0t, x1t) + halfWidth + 1
	minY := math.Min(y0t, y1t) - halfWidth - 1
	maxY := math.Max(y0t, y1t) + halfWidth + 1

	bounds := r.img.Bounds()
	x1i := int(math.Max(minX, float64(bounds.Min.X)))
	y1i := int(math.Max(minY, float64(bounds.Min.Y)))
	x2i := int(math.Min(maxX, float64(bounds.Max.X)))
	y2i := int(math.Min(maxY, float64(bounds.Max.Y)))

	// Draw antialiased line using distance field
	for y := y1i; y < y2i; y++ {
		for x := x1i; x < x2i; x++ {
			// Calculate distance from pixel center to line segment
			px_center := float64(x) + 0.5
			py_center := float64(y) + 0.5

			dist := r.pointToLineSegmentDistance(px_center, py_center, x0t, y0t, x1t, y1t)

			// Calculate coverage based on distance
			coverage := 1.0 - math.Max(0, math.Min(1, dist-halfWidth+0.5))

			if coverage > 0 {
				r.blendPixel(x, y, c, coverage)
			}
		}
	}
}

// drawAntialiasedCircle draws an antialiased circle (used for line caps)
func (r *rasterContext) drawAntialiasedCircle(cx, cy, radius float64, c color.Color) {
	bounds := r.img.Bounds()
	x1 := int(math.Max(cx-radius-1, float64(bounds.Min.X)))
	y1 := int(math.Max(cy-radius-1, float64(bounds.Min.Y)))
	x2 := int(math.Min(cx+radius+1, float64(bounds.Max.X)))
	y2 := int(math.Min(cy+radius+1, float64(bounds.Max.Y)))

	for y := y1; y < y2; y++ {
		for x := x1; x < x2; x++ {
			px := float64(x) + 0.5
			py := float64(y) + 0.5
			dx := px - cx
			dy := py - cy
			dist := math.Sqrt(dx*dx + dy*dy)

			// Antialiased edge
			coverage := 1.0 - math.Max(0, math.Min(1, dist-radius+0.5))

			if coverage > 0 {
				r.blendPixel(x, y, c, coverage)
			}
		}
	}
}

// pointToLineSegmentDistance calculates the distance from a point to a line segment
func (r *rasterContext) pointToLineSegmentDistance(px, py, x0, y0, x1, y1 float64) float64 {
	dx := x1 - x0
	dy := y1 - y0
	lengthSq := dx*dx + dy*dy

	if lengthSq < 0.0001 {
		// Line segment is a point
		dpx := px - x0
		dpy := py - y0
		return math.Sqrt(dpx*dpx + dpy*dpy)
	}

	// Calculate projection parameter
	t := ((px-x0)*dx + (py-y0)*dy) / lengthSq
	t = math.Max(0, math.Min(1, t))

	// Calculate closest point on segment
	closestX := x0 + t*dx
	closestY := y0 + t*dy

	// Calculate distance
	dpx := px - closestX
	dpy := py - closestY
	return math.Sqrt(dpx*dpx + dpy*dpy)
}

// curveCrossings counts how many times a cubic Bezier curve crosses a horizontal ray
func curveCrossings(x0, y0, x1, y1, x2, y2, x3, y3, px, py float64) int {
	// Subdivide curve and count crossings
	return curveCrossingsRecursive(x0, y0, x1, y1, x2, y2, x3, y3, px, py, 0)
}

// curveCrossingsRecursive recursively subdivides curve to count ray crossings
func curveCrossingsRecursive(x0, y0, x1, y1, x2, y2, x3, y3, px, py float64, depth int) int {
	// Limit recursion depth
	if depth > 12 {
		if crossesRay(x0, y0, x3, y3, px, py) {
			if y0 <= py {
				return 1
			}
			return -1
		}
		return 0
	}

	// Check if curve is flat enough
	dx := x3 - x0
	dy := y3 - y0
	d2 := math.Abs((x1-x3)*dy - (y1-y3)*dx)
	d3 := math.Abs((x2-x3)*dy - (y2-y3)*dx)

	if (d2+d3)*(d2+d3) < 0.05*(dx*dx+dy*dy) {
		if crossesRay(x0, y0, x3, y3, px, py) {
			if y0 <= py {
				return 1
			}
			return -1
		}
		return 0
	}

	// Subdivide curve
	x01 := (x0 + x1) / 2
	y01 := (y0 + y1) / 2
	x12 := (x1 + x2) / 2
	y12 := (y1 + y2) / 2
	x23 := (x2 + x3) / 2
	y23 := (y2 + y3) / 2
	x012 := (x01 + x12) / 2
	y012 := (y01 + y12) / 2
	x123 := (x12 + x23) / 2
	y123 := (y12 + y23) / 2
	x0123 := (x012 + x123) / 2
	y0123 := (y012 + y123) / 2

	// Count crossings in both halves
	count := curveCrossingsRecursive(x0, y0, x01, y01, x012, y012, x0123, y0123, px, py, depth+1)
	count += curveCrossingsRecursive(x0123, y0123, x123, y123, x23, y23, x3, y3, px, py, depth+1)
	return count
}

// crossesRay checks if a line segment crosses a horizontal ray from the point
func crossesRay(x1, y1, x2, y2, px, py float64) bool {
	if (y1 <= py && y2 > py) || (y1 > py && y2 <= py) {
		// 防止除零
		if y2 == y1 {
			return false
		}
		t := (py - y1) / (y2 - y1)
		x := x1 + t*(x2-x1)
		return x > px
	}
	return false
}

// Clear fills the image with a color
func (r *rasterContext) Clear(c color.Color) {
	draw.Draw(r.img, r.img.Bounds(), &image.Uniform{c}, image.Point{}, draw.Src)
}

// getGradientColor calculates the color at a given point for the current gradient pattern
func (r *rasterContext) getGradientColor(x, y float64) color.Color {
	if r.gradientPattern == nil {
		return r.color
	}

	// First, transform from device space to user space using inverse of current matrix
	invMatrix := r.matrix
	if MatrixInvert(&invMatrix) != StatusSuccess {
		return r.color
	}
	ux, uy := MatrixTransformPoint(&invMatrix, x, y)

	// Then apply pattern matrix directly (from user space to pattern space)
	// According to Gopdf spec, pattern matrix is user-to-pattern transformation
	patternMatrix := r.gradientPattern.GetMatrix()
	px, py := MatrixTransformPoint(patternMatrix, ux, uy)

	switch pattern := r.gradientPattern.(type) {
	case LinearGradientPattern:
		return r.getLinearGradientColor(pattern, px, py)
	case RadialGradientPattern:
		return r.getRadialGradientColor(pattern, px, py)
	default:
		return r.color
	}
}

// getLinearGradientColor calculates color for linear gradient
func (r *rasterContext) getLinearGradientColor(pattern LinearGradientPattern, x, y float64) color.Color {
	x0, y0, x1, y1 := pattern.GetLinearPoints()

	// Calculate projection of point onto gradient line
	dx := x1 - x0
	dy := y1 - y0
	length := math.Sqrt(dx*dx + dy*dy)

	if length < 0.0001 {
		// Degenerate gradient
		if pattern.GetColorStopCount() > 0 {
			_, r, g, b, a, _ := pattern.GetColorStop(0)
			return color.NRGBA{
				R: uint8(r * 255),
				G: uint8(g * 255),
				B: uint8(b * 255),
				A: uint8(a * 255),
			}
		}
		return color.Black
	}

	// Normalized direction vector
	ndx := dx / length
	ndy := dy / length

	// Project point onto gradient line
	t := ((x-x0)*ndx + (y-y0)*ndy) / length

	// Handle extend modes
	extend := pattern.GetExtend()
	t = r.applyExtendMode(t, extend)

	// Interpolate color from stops
	return r.interpolateColorStops(pattern, t)
}

// getRadialGradientColor calculates color for radial gradient
func (r *rasterContext) getRadialGradientColor(pattern RadialGradientPattern, x, y float64) color.Color {
	cx0, cy0, _, cx1, cy1, radius1 := pattern.GetRadialCircles()

	// Calculate distance from point to gradient centers
	dx0 := x - cx0
	dy0 := y - cy0
	dist0 := math.Sqrt(dx0*dx0 + dy0*dy0)

	dx1 := x - cx1
	dy1 := y - cy1
	dist1 := math.Sqrt(dx1*dx1 + dy1*dy1)

	// Simple radial gradient: interpolate based on distance from outer circle
	dcx := cx1 - cx0
	dcy := cy1 - cy0
	centerDist := math.Sqrt(dcx*dcx + dcy*dcy)

	var t float64
	if centerDist < 0.0001 {
		// Concentric circles
		if radius1 > 0.0001 {
			t = dist1 / radius1
		}
	} else {
		// Offset circles - use distance from inner circle center
		if radius1 > 0.0001 {
			t = dist0 / radius1
		}
	}

	// Handle extend modes
	extend := pattern.GetExtend()
	t = r.applyExtendMode(t, extend)

	// Interpolate color from stops
	return r.interpolateColorStops(pattern, t)
}

// applyExtendMode applies the extend mode to a gradient parameter t
func (r *rasterContext) applyExtendMode(t float64, extend Extend) float64 {
	switch extend {
	case ExtendNone, ExtendPad:
		// Clamp to [0, 1]
		if t < 0 {
			return 0
		}
		if t > 1 {
			return 1
		}
		return t
	case ExtendRepeat:
		// Repeat: t mod 1
		t = math.Mod(t, 1.0)
		if t < 0 {
			t += 1.0
		}
		return t
	case ExtendReflect:
		// Reflect: bounce back and forth
		t = math.Mod(t, 2.0)
		if t < 0 {
			t += 2.0
		}
		if t > 1.0 {
			t = 2.0 - t
		}
		return t
	default:
		return math.Max(0, math.Min(1, t))
	}
}

// interpolateColorStops interpolates color from gradient stops
func (r *rasterContext) interpolateColorStops(pattern GradientPattern, t float64) color.Color {
	stopCount := pattern.GetColorStopCount()
	if stopCount == 0 {
		return color.Black
	}

	if stopCount == 1 {
		_, r, g, b, a, _ := pattern.GetColorStop(0)
		return color.NRGBA{
			R: uint8(r * 255),
			G: uint8(g * 255),
			B: uint8(b * 255),
			A: uint8(a * 255),
		}
	}

	// Find the two stops to interpolate between
	var stop1Offset, stop1R, stop1G, stop1B, stop1A float64
	var stop2Offset, stop2R, stop2G, stop2B, stop2A float64

	// Get first stop
	stop1Offset, stop1R, stop1G, stop1B, stop1A, _ = pattern.GetColorStop(0)

	// If t is before first stop, use first stop color
	if t <= stop1Offset {
		return color.NRGBA{
			R: uint8(stop1R * 255),
			G: uint8(stop1G * 255),
			B: uint8(stop1B * 255),
			A: uint8(stop1A * 255),
		}
	}

	// Find the stops to interpolate between
	for i := 1; i < stopCount; i++ {
		stop2Offset, stop2R, stop2G, stop2B, stop2A, _ = pattern.GetColorStop(i)

		if t <= stop2Offset {
			// Interpolate between stop1 and stop2
			if stop2Offset-stop1Offset < 0.0001 {
				// Stops are at same position, use second stop
				return color.NRGBA{
					R: uint8(stop2R * 255),
					G: uint8(stop2G * 255),
					B: uint8(stop2B * 255),
					A: uint8(stop2A * 255),
				}
			}

			// Linear interpolation
			factor := (t - stop1Offset) / (stop2Offset - stop1Offset)
			r := stop1R + (stop2R-stop1R)*factor
			g := stop1G + (stop2G-stop1G)*factor
			b := stop1B + (stop2B-stop1B)*factor
			a := stop1A + (stop2A-stop1A)*factor

			return color.NRGBA{
				R: uint8(math.Min(math.Max(r*255, 0), 255)),
				G: uint8(math.Min(math.Max(g*255, 0), 255)),
				B: uint8(math.Min(math.Max(b*255, 0), 255)),
				A: uint8(math.Min(math.Max(a*255, 0), 255)),
			}
		}

		// Move to next stop
		stop1Offset, stop1R, stop1G, stop1B, stop1A = stop2Offset, stop2R, stop2G, stop2B, stop2A
	}

	// If t is after last stop, use last stop color
	return color.NRGBA{
		R: uint8(stop2R * 255),
		G: uint8(stop2G * 255),
		B: uint8(stop2B * 255),
		A: uint8(stop2A * 255),
	}
}

// getSurfacePatternColor gets the color from a surface pattern at the given point
func (r *rasterContext) getSurfacePatternColor(x, y float64) color.Color {
	if r.surfacePattern == nil {
		return r.color
	}

	// Transform from device space to user space
	invMatrix := r.matrix
	if MatrixInvert(&invMatrix) != StatusSuccess {
		return r.color
	}
	ux, uy := MatrixTransformPoint(&invMatrix, x, y)

	// Type assert to SurfacePattern interface
	surfPattern, ok := r.surfacePattern.(SurfacePattern)
	if !ok {
		return r.color
	}

	// Apply pattern matrix (user space to pattern space)
	patternMatrix := surfPattern.GetMatrix()
	px, py := MatrixTransformPoint(patternMatrix, ux, uy)

	// Get the surface from the pattern
	surface := surfPattern.GetSurface()
	if surface == nil {
		return r.color
	}

	// Get the image from the surface
	imgSurface, ok := surface.(ImageSurface)
	if !ok {
		return r.color
	}

	goImg := imgSurface.GetGoImage()
	if goImg == nil {
		return r.color
	}

	bounds := goImg.Bounds()

	// Convert to integer coordinates
	ix := int(math.Floor(px))
	iy := int(math.Floor(py))

	// Handle extend modes
	extend := surfPattern.GetExtend()
	switch extend {
	case ExtendRepeat:
		// Wrap coordinates
		if bounds.Dx() > 0 {
			ix = ((ix % bounds.Dx()) + bounds.Dx()) % bounds.Dx()
		}
		if bounds.Dy() > 0 {
			iy = ((iy % bounds.Dy()) + bounds.Dy()) % bounds.Dy()
		}
	case ExtendReflect:
		// Mirror coordinates
		if bounds.Dx() > 0 {
			period := bounds.Dx() * 2
			ix = ix % period
			if ix < 0 {
				ix += period
			}
			if ix >= bounds.Dx() {
				ix = period - ix - 1
			}
		}
		if bounds.Dy() > 0 {
			period := bounds.Dy() * 2
			iy = iy % period
			if iy < 0 {
				iy += period
			}
			if iy >= bounds.Dy() {
				iy = period - iy - 1
			}
		}
	case ExtendPad:
		// Clamp to edges
		if ix < bounds.Min.X {
			ix = bounds.Min.X
		}
		if ix >= bounds.Max.X {
			ix = bounds.Max.X - 1
		}
		if iy < bounds.Min.Y {
			iy = bounds.Min.Y
		}
		if iy >= bounds.Max.Y {
			iy = bounds.Max.Y - 1
		}
	default: // ExtendNone
		// Return transparent for out-of-bounds
		if ix < bounds.Min.X || ix >= bounds.Max.X || iy < bounds.Min.Y || iy >= bounds.Max.Y {
			return color.NRGBA{R: 0, G: 0, B: 0, A: 0}
		}
	}

	// Get the color at the calculated position
	return goImg.At(ix, iy)
}
