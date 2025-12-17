package gopdf

import (
	"image/color"
	"math"

	"github.com/novvoo/go-cairo/pkg/cairo"
)

// FillRule 填充规则
type FillRule int

const (
	FillRuleNonZero FillRule = iota // 非零缠绕规则
	FillRuleEvenOdd                 // 奇偶规则
)

// PathFiller 路径填充器
type PathFiller struct {
	ctx        cairo.Context
	fillRule   FillRule
	rasterizer *Rasterizer // 使用光栅化器进行高级路径填充
	useRaster  bool        // 是否使用光栅化器
}

// NewPathFiller 创建路径填充器
func NewPathFiller(ctx cairo.Context) *PathFiller {
	return &PathFiller{
		ctx:        ctx,
		fillRule:   FillRuleNonZero,
		rasterizer: nil,
		useRaster:  false,
	}
}

// EnableRasterizer 启用光栅化器（用于复杂路径）
func (pf *PathFiller) EnableRasterizer(width, height int) {
	pf.rasterizer = NewRasterizer(width, height)
	pf.useRaster = true
}

// DisableRasterizer 禁用光栅化器
func (pf *PathFiller) DisableRasterizer() {
	if pf.rasterizer != nil {
		pf.rasterizer.Destroy()
		pf.rasterizer = nil
	}
	pf.useRaster = false
}

// SetFillRule 设置填充规则
func (pf *PathFiller) SetFillRule(rule FillRule) {
	pf.fillRule = rule
	if rule == FillRuleEvenOdd {
		pf.ctx.SetFillRule(cairo.FillRuleEvenOdd)
	} else {
		pf.ctx.SetFillRule(cairo.FillRuleWinding)
	}
}

// FillPath 填充路径
func (pf *PathFiller) FillPath(path *Path, color *Color) error {
	if path.IsEmpty() {
		return nil
	}

	// 如果启用了光栅化器，使用光栅化器填充
	if pf.useRaster && pf.rasterizer != nil {
		return pf.fillPathWithRasterizer(path, color)
	}

	// 否则使用 Cairo 标准填充
	if color != nil {
		pf.ctx.SetSourceRGBA(color.R, color.G, color.B, color.A)
	}

	pf.buildCairoPath(path)
	pf.ctx.Fill()

	return nil
}

// fillPathWithRasterizer 使用光栅化器填充路径
func (pf *PathFiller) fillPathWithRasterizer(path *Path, fillColor *Color) error {
	if pf.rasterizer == nil {
		return nil
	}

	// 清空光栅化器
	pf.rasterizer.Clear()

	// 添加路径
	pf.rasterizer.AddPath(path, nil)

	// 确定填充规则
	var fillRule cairo.FillRule
	if pf.fillRule == FillRuleEvenOdd {
		fillRule = cairo.FillRuleEvenOdd
	} else {
		fillRule = cairo.FillRuleWinding
	}

	// 创建颜色
	var c color.Color = color.Transparent
	if fillColor != nil {
		c = &color.RGBA{
			R: uint8(fillColor.R * 255),
			G: uint8(fillColor.G * 255),
			B: uint8(fillColor.B * 255),
			A: uint8(fillColor.A * 255),
		}
	}

	// 填充
	return pf.rasterizer.Fill(c, fillRule, cairo.OperatorOver)
}

// FillPathPreserve 填充路径但保留路径
func (pf *PathFiller) FillPathPreserve(path *Path, color *Color) error {
	if path.IsEmpty() {
		return nil
	}

	if color != nil {
		pf.ctx.SetSourceRGBA(color.R, color.G, color.B, color.A)
	}

	pf.buildCairoPath(path)
	pf.ctx.FillPreserve()

	return nil
}

// StrokePath 描边路径
func (pf *PathFiller) StrokePath(path *Path, color *Color, lineWidth float64) error {
	if path.IsEmpty() {
		return nil
	}

	if color != nil {
		pf.ctx.SetSourceRGBA(color.R, color.G, color.B, color.A)
	}

	if lineWidth > 0 {
		pf.ctx.SetLineWidth(lineWidth)
	}

	pf.buildCairoPath(path)
	pf.ctx.Stroke()

	return nil
}

// FillAndStrokePath 填充并描边路径
func (pf *PathFiller) FillAndStrokePath(path *Path, fillColor, strokeColor *Color, lineWidth float64) error {
	if path.IsEmpty() {
		return nil
	}

	pf.buildCairoPath(path)

	// 先填充
	if fillColor != nil {
		pf.ctx.SetSourceRGBA(fillColor.R, fillColor.G, fillColor.B, fillColor.A)
		pf.ctx.FillPreserve()
	}

	// 再描边
	if strokeColor != nil {
		pf.ctx.SetSourceRGBA(strokeColor.R, strokeColor.G, strokeColor.B, strokeColor.A)
	}
	if lineWidth > 0 {
		pf.ctx.SetLineWidth(lineWidth)
	}
	pf.ctx.Stroke()

	return nil
}

// buildCairoPath 构建 Cairo 路径
func (pf *PathFiller) buildCairoPath(path *Path) {
	pf.ctx.NewPath()

	for _, subpath := range path.GetSubpaths() {
		for _, segment := range subpath.GetSegments() {
			switch seg := segment.(type) {
			case *MoveToSegment:
				pf.ctx.MoveTo(seg.X, seg.Y)

			case *LineToSegment:
				pf.ctx.LineTo(seg.X, seg.Y)

			case *CurveToSegment:
				pf.ctx.CurveTo(seg.X1, seg.Y1, seg.X2, seg.Y2, seg.X3, seg.Y3)

			case *RectangleSegment:
				pf.ctx.Rectangle(seg.X, seg.Y, seg.Width, seg.Height)
			}
		}

		if subpath.IsClosed() {
			pf.ctx.ClosePath()
		}
	}
}

// ClipPath 使用路径裁剪
func (pf *PathFiller) ClipPath(path *Path) error {
	if path.IsEmpty() {
		return nil
	}

	pf.buildCairoPath(path)
	pf.ctx.Clip()

	return nil
}

// ClipPathPreserve 使用路径裁剪但保留路径
func (pf *PathFiller) ClipPathPreserve(path *Path) error {
	if path.IsEmpty() {
		return nil
	}

	pf.buildCairoPath(path)
	pf.ctx.ClipPreserve()

	return nil
}

// ===== 复杂路径构建辅助函数 =====

// CreateStarPath 创建星形路径
func CreateStarPath(centerX, centerY, outerRadius, innerRadius float64, points int) *Path {
	path := NewPath()

	angleStep := 2 * math.Pi / float64(points*2)

	for i := 0; i < points*2; i++ {
		angle := float64(i)*angleStep - math.Pi/2
		var radius float64
		if i%2 == 0 {
			radius = outerRadius
		} else {
			radius = innerRadius
		}

		x := centerX + radius*math.Cos(angle)
		y := centerY + radius*math.Sin(angle)

		if i == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}

	path.ClosePath()
	return path
}

// CreatePolygonPath 创建正多边形路径
func CreatePolygonPath(centerX, centerY, radius float64, sides int) *Path {
	path := NewPath()

	angleStep := 2 * math.Pi / float64(sides)

	for i := 0; i < sides; i++ {
		angle := float64(i)*angleStep - math.Pi/2
		x := centerX + radius*math.Cos(angle)
		y := centerY + radius*math.Sin(angle)

		if i == 0 {
			path.MoveTo(x, y)
		} else {
			path.LineTo(x, y)
		}
	}

	path.ClosePath()
	return path
}

// CreateCirclePath 创建圆形路径
func CreateCirclePath(centerX, centerY, radius float64) *Path {
	path := NewPath()

	// 使用贝塞尔曲线近似圆
	// 魔法数字：4/3 * tan(π/8) ≈ 0.5522847498
	kappa := 0.5522847498

	path.MoveTo(centerX+radius, centerY)

	// 右下
	path.CurveTo(
		centerX+radius, centerY+radius*kappa,
		centerX+radius*kappa, centerY+radius,
		centerX, centerY+radius,
	)

	// 左下
	path.CurveTo(
		centerX-radius*kappa, centerY+radius,
		centerX-radius, centerY+radius*kappa,
		centerX-radius, centerY,
	)

	// 左上
	path.CurveTo(
		centerX-radius, centerY-radius*kappa,
		centerX-radius*kappa, centerY-radius,
		centerX, centerY-radius,
	)

	// 右上
	path.CurveTo(
		centerX+radius*kappa, centerY-radius,
		centerX+radius, centerY-radius*kappa,
		centerX+radius, centerY,
	)

	path.ClosePath()
	return path
}

// CreateRoundedRectPath 创建圆角矩形路径
func CreateRoundedRectPath(x, y, width, height, radius float64) *Path {
	path := NewPath()

	// 限制圆角半径
	maxRadius := math.Min(width, height) / 2
	if radius > maxRadius {
		radius = maxRadius
	}

	kappa := 0.5522847498

	// 从左上角开始
	path.MoveTo(x+radius, y)

	// 上边
	path.LineTo(x+width-radius, y)

	// 右上角
	path.CurveTo(
		x+width-radius+radius*kappa, y,
		x+width, y+radius-radius*kappa,
		x+width, y+radius,
	)

	// 右边
	path.LineTo(x+width, y+height-radius)

	// 右下角
	path.CurveTo(
		x+width, y+height-radius+radius*kappa,
		x+width-radius+radius*kappa, y+height,
		x+width-radius, y+height,
	)

	// 下边
	path.LineTo(x+radius, y+height)

	// 左下角
	path.CurveTo(
		x+radius-radius*kappa, y+height,
		x, y+height-radius+radius*kappa,
		x, y+height-radius,
	)

	// 左边
	path.LineTo(x, y+radius)

	// 左上角
	path.CurveTo(
		x, y+radius-radius*kappa,
		x+radius-radius*kappa, y,
		x+radius, y,
	)

	path.ClosePath()
	return path
}

// CreateEllipsePath 创建椭圆路径
func CreateEllipsePath(centerX, centerY, radiusX, radiusY float64) *Path {
	path := NewPath()

	kappa := 0.5522847498

	path.MoveTo(centerX+radiusX, centerY)

	// 右下
	path.CurveTo(
		centerX+radiusX, centerY+radiusY*kappa,
		centerX+radiusX*kappa, centerY+radiusY,
		centerX, centerY+radiusY,
	)

	// 左下
	path.CurveTo(
		centerX-radiusX*kappa, centerY+radiusY,
		centerX-radiusX, centerY+radiusY*kappa,
		centerX-radiusX, centerY,
	)

	// 左上
	path.CurveTo(
		centerX-radiusX, centerY-radiusY*kappa,
		centerX-radiusX*kappa, centerY-radiusY,
		centerX, centerY-radiusY,
	)

	// 右上
	path.CurveTo(
		centerX+radiusX*kappa, centerY-radiusY,
		centerX+radiusX, centerY-radiusY*kappa,
		centerX+radiusX, centerY,
	)

	path.ClosePath()
	return path
}

// CreateArcPath 创建弧形路径
func CreateArcPath(centerX, centerY, radius, startAngle, endAngle float64, clockwise bool) *Path {
	path := NewPath()

	// 起始点
	startX := centerX + radius*math.Cos(startAngle)
	startY := centerY + radius*math.Sin(startAngle)
	path.MoveTo(startX, startY)

	// 使用多段贝塞尔曲线近似弧
	segments := 16
	angleRange := endAngle - startAngle
	if !clockwise {
		angleRange = -(2*math.Pi - angleRange)
	}

	angleStep := angleRange / float64(segments)

	for i := 1; i <= segments; i++ {
		angle := startAngle + float64(i)*angleStep
		x := centerX + radius*math.Cos(angle)
		y := centerY + radius*math.Sin(angle)
		path.LineTo(x, y)
	}

	return path
}

// CreateBezierPath 创建贝塞尔曲线路径
func CreateBezierPath(points []Point) *Path {
	path := NewPath()

	if len(points) < 2 {
		return path
	}

	path.MoveTo(points[0].X, points[0].Y)

	if len(points) == 2 {
		path.LineTo(points[1].X, points[1].Y)
	} else if len(points) == 3 {
		// 二次贝塞尔转三次贝塞尔
		p0, p1, p2 := points[0], points[1], points[2]
		cp1x := p0.X + 2.0/3.0*(p1.X-p0.X)
		cp1y := p0.Y + 2.0/3.0*(p1.Y-p0.Y)
		cp2x := p2.X + 2.0/3.0*(p1.X-p2.X)
		cp2y := p2.Y + 2.0/3.0*(p1.Y-p2.Y)
		path.CurveTo(cp1x, cp1y, cp2x, cp2y, p2.X, p2.Y)
	} else if len(points) >= 4 {
		// 三次贝塞尔
		for i := 0; i+3 < len(points); i += 3 {
			p1, p2, p3 := points[i+1], points[i+2], points[i+3]
			path.CurveTo(p1.X, p1.Y, p2.X, p2.Y, p3.X, p3.Y)
		}
	}

	return path
}

// Point 表示二维点
type Point struct {
	X, Y float64
}
