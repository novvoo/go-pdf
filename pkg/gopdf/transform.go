package gopdf

import (
	"fmt"
	"math"

	"github.com/novvoo/go-cairo/pkg/cairo"
)

// Matrix 表示 2D 变换矩阵 (CTM - Current Transformation Matrix)
// PDF 使用 3x3 矩阵的简化形式：
// [ a  b  0 ]
// [ c  d  0 ]
// [ e  f  1 ]
// 其中 (e, f) 是平移，(a, d) 是缩放，(b, c) 是旋转/倾斜
type Matrix struct {
	A, B, C, D, E, F float64
}

// NewIdentityMatrix 创建单位矩阵
func NewIdentityMatrix() *Matrix {
	return &Matrix{
		A: 1, B: 0,
		C: 0, D: 1,
		E: 0, F: 0,
	}
}

// Apply 对点进行变换（别名，更符合设计文档）
func (m *Matrix) Apply(x, y float64) (float64, float64) {
	return m.Transform(x, y)
}

// NewTranslationMatrix 创建平移矩阵
func NewTranslationMatrix(tx, ty float64) *Matrix {
	return &Matrix{
		A: 1, B: 0,
		C: 0, D: 1,
		E: tx, F: ty,
	}
}

// NewScaleMatrix 创建缩放矩阵
func NewScaleMatrix(sx, sy float64) *Matrix {
	return &Matrix{
		A: sx, B: 0,
		C: 0, D: sy,
		E: 0, F: 0,
	}
}

// NewRotationMatrix 创建旋转矩阵（角度以弧度为单位）
func NewRotationMatrix(angle float64) *Matrix {
	cos := math.Cos(angle)
	sin := math.Sin(angle)
	return &Matrix{
		A: cos, B: sin,
		C: -sin, D: cos,
		E: 0, F: 0,
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
		A: m.A*other.A + m.B*other.C,
		B: m.A*other.B + m.B*other.D,
		C: m.C*other.A + m.D*other.C,
		D: m.C*other.B + m.D*other.D,
		E: m.E*other.A + m.F*other.C + other.E,
		F: m.E*other.B + m.F*other.D + other.F,
	}
}

// Transform 对点进行变换
func (m *Matrix) Transform(x, y float64) (float64, float64) {
	newX := m.A*x + m.C*y + m.E
	newY := m.B*x + m.D*y + m.F
	return newX, newY
}

// TransformDistance 对距离向量进行变换（不包括平移）
func (m *Matrix) TransformDistance(dx, dy float64) (float64, float64) {
	newDx := m.A*dx + m.C*dy
	newDy := m.B*dx + m.D*dy
	return newDx, newDy
}

// Invert 计算逆矩阵
func (m *Matrix) Invert() (*Matrix, error) {
	det := m.A*m.D - m.B*m.C
	if math.Abs(det) < 1e-10 {
		return nil, fmt.Errorf("matrix is not invertible (determinant is zero)")
	}

	invDet := 1.0 / det
	return &Matrix{
		A: m.D * invDet,
		B: -m.B * invDet,
		C: -m.C * invDet,
		D: m.A * invDet,
		E: (m.C*m.F - m.D*m.E) * invDet,
		F: (m.B*m.E - m.A*m.F) * invDet,
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
		A: m.A, B: m.B,
		C: m.C, D: m.D,
		E: m.E, F: m.F,
	}
}

// String 返回矩阵的字符串表示
func (m *Matrix) String() string {
	return fmt.Sprintf("[%.3f %.3f %.3f %.3f %.3f %.3f]", m.A, m.B, m.C, m.D, m.E, m.F)
}

// ToCairoMatrix 转换为 Cairo 矩阵格式
func (m *Matrix) ToCairoMatrix() cairo.Matrix {
	return cairo.Matrix{
		XX: m.A, YX: m.B,
		XY: m.C, YY: m.D,
		X0: m.E, Y0: m.F,
	}
}

// FromCairoMatrix 从 Cairo 矩阵创建
func FromCairoMatrix(cm cairo.Matrix) *Matrix {
	return &Matrix{
		A: cm.XX, B: cm.YX,
		C: cm.XY, D: cm.YY,
		E: cm.X0, F: cm.Y0,
	}
}

// ApplyToCairoContext 将矩阵应用到 Cairo context
func (m *Matrix) ApplyToCairoContext(ctx cairo.Context) {
	cairoMatrix := m.ToCairoMatrix()
	ctx.Transform(&cairoMatrix)
}

// SetCairoContextMatrix 设置 Cairo context 的变换矩阵
func (m *Matrix) SetCairoContextMatrix(ctx cairo.Context) {
	cairoMatrix := m.ToCairoMatrix()
	ctx.SetMatrix(&cairoMatrix)
}
