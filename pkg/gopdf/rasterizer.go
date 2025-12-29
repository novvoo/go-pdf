package gopdf

import (
	"image"
	"image/color"
)

// Rasterizer 光栅化器
// 使用 Gopdf 的 AdvancedRasterizer 将矢量路径转换为像素
type Rasterizer struct {
	width      int
	height     int
	rasterizer *AdvancedRasterizer
	backend    *PixmanBackend
}

// NewRasterizer 创建新的光栅化器
func NewRasterizer(width, height int) *Rasterizer {
	return &Rasterizer{
		width:      width,
		height:     height,
		rasterizer: &AdvancedRasterizer{}, // Placeholder - not yet implemented
		backend:    NewPixmanBackend(width, height, PixmanFormatARGB32),
	}
}

// GetWidth 获取宽度
func (r *Rasterizer) GetWidth() int {
	return r.width
}

// GetHeight 获取高度
func (r *Rasterizer) GetHeight() int {
	return r.height
}

// GetBackend 获取 Pixman 后端
func (r *Rasterizer) GetBackend() *PixmanBackend {
	return r.backend
}

// Clear 清空光栅化器
func (r *Rasterizer) Clear() {
	if r.rasterizer != nil {
		r.rasterizer.Reset()
	}
	if r.backend != nil {
		r.backend.Clear()
	}
}

// AddPath 添加路径到光栅化器
func (r *Rasterizer) AddPath(path *PathImpl, transform *Matrix) {
	if r.rasterizer == nil || path == nil {
		return
	}

	for _, subpath := range path.GetSubpaths() {
		var firstX, firstY float64
		var lastX, lastY float64
		isFirst := true

		for _, segment := range subpath.GetSegments() {
			switch seg := segment.(type) {
			case *MoveToSegment:
				x, y := seg.X, seg.Y
				if transform != nil {
					x, y = transform.Transform(x, y)
				}
				lastX, lastY = x, y
				if isFirst {
					firstX, firstY = x, y
					isFirst = false
				}

			case *LineToSegment:
				x, y := seg.X, seg.Y
				if transform != nil {
					x, y = transform.Transform(x, y)
				}
				r.rasterizer.AddLine(lastX, lastY, x, y)
				lastX, lastY = x, y

			case *CurveToSegment:
				x1, y1 := seg.X1, seg.Y1
				x2, y2 := seg.X2, seg.Y2
				x3, y3 := seg.X3, seg.Y3
				if transform != nil {
					x1, y1 = transform.Transform(x1, y1)
					x2, y2 = transform.Transform(x2, y2)
					x3, y3 = transform.Transform(x3, y3)
				}
				r.rasterizer.AddCubicBezier(lastX, lastY, x1, y1, x2, y2, x3, y3)
				lastX, lastY = x3, y3

			case *RectangleSegment:
				x, y := seg.X, seg.Y
				w, h := seg.Width, seg.Height
				if transform != nil {
					x, y = transform.Transform(x, y)
					x2, y2 := transform.Transform(x+w, y+h)
					w = x2 - x
					h = y2 - y
				}
				// 矩形转换为四条边
				r.rasterizer.AddLine(x, y, x+w, y)
				r.rasterizer.AddLine(x+w, y, x+w, y+h)
				r.rasterizer.AddLine(x+w, y+h, x, y+h)
				r.rasterizer.AddLine(x, y+h, x, y)
				lastX, lastY = x, y
			}
		}

		// 如果子路径是闭合的，添加闭合线段
		if subpath.IsClosed() && !isFirst {
			r.rasterizer.AddLine(lastX, lastY, firstX, firstY)
		}
	}
}

// Fill 填充路径
func (r *Rasterizer) Fill(fillColor color.Color, fillRule FillRule, op Operator) error {
	if r.rasterizer == nil || r.backend == nil {
		return nil
	}

	// 获取 RGBA 图像
	rgba := r.backend.ToRGBA()
	if rgba == nil {
		rgba = image.NewRGBA(image.Rect(0, 0, r.width, r.height))
	}

	// 光栅化路径
	r.rasterizer.Rasterize(rgba, fillColor, fillRule)

	// 将结果写回 Pixman 后端
	tempBackend := NewPixmanBackendFromRGBA(rgba)
	if tempBackend != nil {
		// 使用指定的混合模式合成
		r.backend.Composite(tempBackend, 0, 0, 0, 0, r.width, r.height, op)
		tempBackend.Destroy()
	}

	return nil
}

// Stroke 描边路径（简化实现）
func (r *Rasterizer) Stroke(strokeColor color.Color, lineWidth float64, op Operator) error {
	// AdvancedRasterizer 不直接支持描边
	// 这里简化处理，使用填充代替
	return r.Fill(strokeColor, FillRuleWinding, op)
}

// ToImage 转换为图像
func (r *Rasterizer) ToImage() image.Image {
	if r.backend == nil {
		return nil
	}
	return r.backend.ToRGBA()
}

// Destroy 销毁资源
func (r *Rasterizer) Destroy() {
	// AdvancedRasterizer 不需要显式销毁
	r.rasterizer = nil
	if r.backend != nil {
		r.backend.Destroy()
		r.backend = nil
	}
}

// RasterizerPool 光栅化器池
// 用于复用光栅化器，减少内存分配
type RasterizerPool struct {
	pool          chan *Rasterizer
	width, height int
}

// NewRasterizerPool 创建光栅化器池
func NewRasterizerPool(width, height, poolSize int) *RasterizerPool {
	pool := &RasterizerPool{
		pool:   make(chan *Rasterizer, poolSize),
		width:  width,
		height: height,
	}

	// 预分配光栅化器
	for i := 0; i < poolSize; i++ {
		pool.pool <- NewRasterizer(width, height)
	}

	return pool
}

// Get 获取光栅化器
func (p *RasterizerPool) Get() *Rasterizer {
	select {
	case r := <-p.pool:
		r.Clear()
		return r
	default:
		// 池为空，创建新的
		return NewRasterizer(p.width, p.height)
	}
}

// Put 归还光栅化器
func (p *RasterizerPool) Put(r *Rasterizer) {
	if r == nil {
		return
	}

	select {
	case p.pool <- r:
		// 成功归还
	default:
		// 池已满，销毁
		r.Destroy()
	}
}

// Destroy 销毁池
func (p *RasterizerPool) Destroy() {
	close(p.pool)
	for r := range p.pool {
		r.Destroy()
	}
}
