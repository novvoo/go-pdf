package gopdf

import (
	"fmt"
	"math"
)

// NewIdentityMatrix 创建单位矩阵
func NewIdentityMatrix() *Matrix {
	return &Matrix{
		XX: 1, YX: 0,
		XY: 0, YY: 1,
		X0: 0, Y0: 0,
	}
}

// Apply 对点进行变换（别名，更符合设计文档）
func (m *Matrix) Apply(x, y float64) (float64, float64) {
	return m.Transform(x, y)
}

// NewTranslationMatrix 创建平移矩阵
func NewTranslationMatrix(tx, ty float64) *Matrix {
	return &Matrix{
		XX: 1, YX: 0,
		XY: 0, YY: 1,
		X0: tx, Y0: ty,
	}
}

// NewScaleMatrix 创建缩放矩阵
func NewScaleMatrix(sx, sy float64) *Matrix {
	return &Matrix{
		XX: sx, YX: 0,
		XY: 0, YY: sy,
		X0: 0, Y0: 0,
	}
}

// NewRotationMatrix 创建旋转矩阵（角度以弧度为单位）
func NewRotationMatrix(angle float64) *Matrix {
	cos := math.Cos(angle)
	sin := math.Sin(angle)
	return &Matrix{
		XX: cos, YX: sin,
		XY: -sin, YY: cos,
		X0: 0, Y0: 0,
	}
}

// NewRotationMatrixDegrees 创建旋转矩阵（角度以度为单位）
func NewRotationMatrixDegrees(degrees float64) *Matrix {
	return NewRotationMatrix(degrees * math.Pi / 180.0)
}

// Multiply 矩阵乘法：this * other
// 用于组合多个变换
func (m *Matrix) Multiply(other *Matrix) *Matrix {
	return &Matrix{
		XX: m.XX*other.XX + m.YX*other.XY,
		YX: m.XX*other.YX + m.YX*other.YY,
		XY: m.XY*other.XX + m.YY*other.XY,
		YY: m.XY*other.YX + m.YY*other.YY,
		X0: m.X0*other.XX + m.Y0*other.XY + other.X0,
		Y0: m.X0*other.YX + m.Y0*other.YY + other.Y0,
	}
}

// Transform 对点进行变换
func (m *Matrix) Transform(x, y float64) (float64, float64) {
	newX := m.XX*x + m.XY*y + m.X0
	newY := m.YX*x + m.YY*y + m.Y0
	return newX, newY
}

// TransformDistance 对距离向量进行变换（不包括平移）
func (m *Matrix) TransformDistance(dx, dy float64) (float64, float64) {
	newDx := m.XX*dx + m.XY*dy
	newDy := m.YX*dx + m.YY*dy
	return newDx, newDy
}

// Invert 计算逆矩阵
func (m *Matrix) Invert() (*Matrix, error) {
	det := m.XX*m.YY - m.YX*m.XY
	if math.Abs(det) < 1e-10 {
		return nil, fmt.Errorf("matrix is not invertible (determinant is zero)")
	}

	invDet := 1.0 / det
	return &Matrix{
		XX: m.YY * invDet,
		YX: -m.YX * invDet,
		XY: -m.XY * invDet,
		YY: m.XX * invDet,
		X0: (m.XY*m.Y0 - m.YY*m.X0) * invDet,
		Y0: (m.YX*m.X0 - m.XX*m.Y0) * invDet,
	}, nil
}

// Translate 添加平移变换
func (m *Matrix) Translate(tx, ty float64) *Matrix {
	return m.Multiply(NewTranslationMatrix(tx, ty))
}

// Scale 添加缩放变换
func (m *Matrix) Scale(sx, sy float64) *Matrix {
	return m.Multiply(NewScaleMatrix(sx, sy))
}

// Rotate 添加旋转变换（弧度）
func (m *Matrix) Rotate(angle float64) *Matrix {
	return m.Multiply(NewRotationMatrix(angle))
}

// RotateDegrees 添加旋转变换（度）
func (m *Matrix) RotateDegrees(degrees float64) *Matrix {
	return m.Multiply(NewRotationMatrixDegrees(degrees))
}

// Clone 复制矩阵
func (m *Matrix) Clone() *Matrix {
	return &Matrix{
		XX: m.XX, YX: m.YX,
		XY: m.XY, YY: m.YY,
		X0: m.X0, Y0: m.Y0,
	}
}

// String 返回矩阵的字符串表示
func (m *Matrix) String() string {
	return fmt.Sprintf("[%.3f %.3f %.3f %.3f %.3f %.3f]", m.XX, m.YX, m.XY, m.YY, m.X0, m.Y0)
}

// ApplyToGopdfContext 将矩阵应用到 Gopdf context
func (m *Matrix) ApplyToGopdfContext(ctx Context) {
	ctx.Transform(m)
}

// SetGopdfContextMatrix 设置 Gopdf context 的变换矩阵
func (m *Matrix) SetGopdfContextMatrix(ctx Context) {
	ctx.SetMatrix(m)
}
