package gopdf

// CoordinateSystem 表示坐标系统类型
type CoordinateSystem int

const (
	// CoordSystemPDF PDF 坐标系统：原点在左下角，Y 轴向上
	CoordSystemPDF CoordinateSystem = iota
	// CoordSystemGopdf Gopdf/屏幕坐标系统：原点在左上角，Y 轴向下
	CoordSystemGopdf
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

// PDFToGopdf 将 PDF 坐标转换为 Gopdf 坐标
func (c *CoordinateConverter) PDFToGopdf(x, y float64) (float64, float64) {
	// PDF: 原点在左下角，Y 轴向上
	// Gopdf: 原点在左上角，Y 轴向下
	return x, c.pageHeight - y
}

// GopdfToPDF 将 Gopdf 坐标转换为 PDF 坐标
func (c *CoordinateConverter) GopdfToPDF(x, y float64) (float64, float64) {
	return x, c.pageHeight - y
}

// ConvertPoint 根据当前坐标系统转换点
func (c *CoordinateConverter) ConvertPoint(x, y float64, from, to CoordinateSystem) (float64, float64) {
	if from == to {
		return x, y
	}

	if from == CoordSystemPDF && to == CoordSystemGopdf {
		return c.PDFToGopdf(x, y)
	}

	return c.GopdfToPDF(x, y)
}

// GetTransformMatrix 获取从 PDF 坐标系到 Gopdf 坐标系的变换矩阵
func (c *CoordinateConverter) GetTransformMatrix() *Matrix {
	// PDF 到 Gopdf 的变换：
	// 1. Y 轴翻转（乘以 -1）
	// 2. 平移到正确位置（向下移动 pageHeight）
	return &Matrix{
		XX: 1, YX: 0,
		XY: 0, YY: -1,
		X0: 0, Y0: c.pageHeight,
	}
}

// ApplyPDFCoordinateSystem 将 Gopdf context 设置为 PDF 坐标系统
func (c *CoordinateConverter) ApplyPDFCoordinateSystem(ctx Context) {
	// 保存当前状态
	ctx.Save()

	// 应用变换矩阵
	matrix := c.GetTransformMatrix()
	matrix.ApplyToGopdfContext(ctx)
}

// RestoreCoordinateSystem 恢复之前的坐标系统
func (c *CoordinateConverter) RestoreCoordinateSystem(ctx Context) {
	ctx.Restore()
}

// TransformContext 临时应用 PDF 坐标系统执行绘制函数
func (c *CoordinateConverter) TransformContext(ctx Context, drawFunc func(Context)) {
	c.ApplyPDFCoordinateSystem(ctx)
	defer c.RestoreCoordinateSystem(ctx)
	drawFunc(ctx)
}
