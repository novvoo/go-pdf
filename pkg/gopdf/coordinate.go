package gopdf

import (
	"github.com/novvoo/go-cairo/pkg/cairo"
)

// CoordinateSystem 表示坐标系统类型
type CoordinateSystem int

const (
	// CoordSystemPDF PDF 坐标系统：原点在左下角，Y 轴向上
	CoordSystemPDF CoordinateSystem = iota
	// CoordSystemCairo Cairo/屏幕坐标系统：原点在左上角，Y 轴向下
	CoordSystemCairo
)

// CoordinateConverter 坐标系统转换器
type CoordinateConverter struct {
	pageWidth  float64
	pageHeight float64
	system     CoordinateSystem
}

// NewCoordinateConverter 创建坐标转换器
func NewCoordinateConverter(width, height float64, system CoordinateSystem) *CoordinateConverter {
	return &CoordinateConverter{
		pageWidth:  width,
		pageHeight: height,
		system:     system,
	}
}

// PDFToCairo 将 PDF 坐标转换为 Cairo 坐标
func (c *CoordinateConverter) PDFToCairo(x, y float64) (float64, float64) {
	// PDF: 原点在左下角，Y 轴向上
	// Cairo: 原点在左上角，Y 轴向下
	return x, c.pageHeight - y
}

// CairoToPDF 将 Cairo 坐标转换为 PDF 坐标
func (c *CoordinateConverter) CairoToPDF(x, y float64) (float64, float64) {
	return x, c.pageHeight - y
}

// ConvertPoint 根据当前坐标系统转换点
func (c *CoordinateConverter) ConvertPoint(x, y float64, from, to CoordinateSystem) (float64, float64) {
	if from == to {
		return x, y
	}

	if from == CoordSystemPDF && to == CoordSystemCairo {
		return c.PDFToCairo(x, y)
	}

	return c.CairoToPDF(x, y)
}

// GetTransformMatrix 获取从 PDF 坐标系到 Cairo 坐标系的变换矩阵
func (c *CoordinateConverter) GetTransformMatrix() *Matrix {
	// PDF 到 Cairo 的变换：
	// 1. Y 轴翻转（乘以 -1）
	// 2. 平移到正确位置（向下移动 pageHeight）
	return &Matrix{
		A: 1, B: 0,
		C: 0, D: -1,
		E: 0, F: c.pageHeight,
	}
}

// ApplyPDFCoordinateSystem 将 Cairo context 设置为 PDF 坐标系统
func (c *CoordinateConverter) ApplyPDFCoordinateSystem(ctx cairo.Context) {
	// 保存当前状态
	ctx.Save()

	// 应用变换矩阵
	matrix := c.GetTransformMatrix()
	matrix.ApplyToCairoContext(ctx)
}

// RestoreCoordinateSystem 恢复之前的坐标系统
func (c *CoordinateConverter) RestoreCoordinateSystem(ctx cairo.Context) {
	ctx.Restore()
}

// TransformContext 临时应用 PDF 坐标系统执行绘制函数
func (c *CoordinateConverter) TransformContext(ctx cairo.Context, drawFunc func(cairo.Context)) {
	c.ApplyPDFCoordinateSystem(ctx)
	defer c.RestoreCoordinateSystem(ctx)
	drawFunc(ctx)
}
