package gopdf

// Shading 表示 PDF 阴影（渐变）
type Shading struct {
	ShadingType int              // 1-7, 重点支持 2 (线性) 和 3 (径向)
	ColorSpace  string           // 颜色空间
	Coords      []float64        // 坐标数组
	Function    *ShadingFunction // 颜色函数
	Extend      []bool           // 扩展标志 [开始, 结束]
	Background  []float64        // 背景颜色（可选）
	BBox        []float64        // 边界框（可选）
	AntiAlias   bool             // 抗锯齿（可选）
}

// ShadingFunction 表示阴影函数
type ShadingFunction struct {
	FunctionType int       // 函数类型：2 = 指数插值, 3 = 缝合, 4 = PostScript
	Domain       []float64 // 定义域 [min, max]
	Range        []float64 // 值域（可选）
	C0           []float64 // 起始颜色
	C1           []float64 // 结束颜色
	N            float64   // 指数（用于类型 2）

	// 用于缝合函数（类型 3）
	Functions []interface{} // 子函数数组
	Bounds    []float64     // 边界数组
	Encode    []float64     // 编码数组
}

// ShadingPattern 表示阴影图案
type ShadingPattern struct {
	Shading *Shading
	Matrix  *Matrix // 变换矩阵
}

// NewShading 创建新的阴影
func NewShading() *Shading {
	return &Shading{
		Extend:    []bool{false, false},
		AntiAlias: false,
	}
}

// NewShadingFunction 创建新的阴影函数
func NewShadingFunction() *ShadingFunction {
	return &ShadingFunction{
		FunctionType: 2, // 默认为指数插值
		Domain:       []float64{0, 1},
		N:            1.0, // 线性插值
	}
}

// IsLinearGradient 检查是否为线性渐变
func (s *Shading) IsLinearGradient() bool {
	return s.ShadingType == 2
}

// IsRadialGradient 检查是否为径向渐变
func (s *Shading) IsRadialGradient() bool {
	return s.ShadingType == 3
}

// GetLinearCoords 获取线性渐变坐标
// 返回: x0, y0, x1, y1
func (s *Shading) GetLinearCoords() (float64, float64, float64, float64) {
	if len(s.Coords) >= 4 {
		return s.Coords[0], s.Coords[1], s.Coords[2], s.Coords[3]
	}
	return 0, 0, 1, 0 // 默认水平渐变
}

// GetRadialCoords 获取径向渐变坐标
// 返回: x0, y0, r0, x1, y1, r1
func (s *Shading) GetRadialCoords() (float64, float64, float64, float64, float64, float64) {
	if len(s.Coords) >= 6 {
		return s.Coords[0], s.Coords[1], s.Coords[2],
			s.Coords[3], s.Coords[4], s.Coords[5]
	}
	return 0, 0, 0, 1, 0, 1 // 默认从中心到边缘
}

// EvaluateFunction 计算函数在 t 处的颜色值
// t 应该在 [0, 1] 范围内
func (sf *ShadingFunction) EvaluateFunction(t float64) []float64 {
	// 确保 t 在定义域内
	if len(sf.Domain) >= 2 {
		if t < sf.Domain[0] {
			t = sf.Domain[0]
		}
		if t > sf.Domain[1] {
			t = sf.Domain[1]
		}
	}

	switch sf.FunctionType {
	case 2: // 指数插值
		return sf.evaluateExponential(t)
	case 3: // 缝合函数
		return sf.evaluateStitching(t)
	default:
		// 不支持的函数类型，返回起始颜色
		debugPrintf("Warning: Unsupported shading function type %d\n", sf.FunctionType)
		if len(sf.C0) > 0 {
			return sf.C0
		}
		return []float64{0, 0, 0} // 黑色
	}
}

// evaluateExponential 计算指数插值
func (sf *ShadingFunction) evaluateExponential(t float64) []float64 {
	if len(sf.C0) == 0 || len(sf.C1) == 0 {
		return []float64{0, 0, 0}
	}

	// 确保 C0 和 C1 长度相同
	numComponents := len(sf.C0)
	if len(sf.C1) < numComponents {
		numComponents = len(sf.C1)
	}

	result := make([]float64, numComponents)

	// 指数插值: C(t) = C0 + t^N * (C1 - C0)
	tPowN := 1.0
	if sf.N != 1.0 {
		tPowN = 1.0
		for i := 0; i < int(sf.N); i++ {
			tPowN *= t
		}
	} else {
		tPowN = t
	}

	for i := 0; i < numComponents; i++ {
		result[i] = sf.C0[i] + tPowN*(sf.C1[i]-sf.C0[i])

		// 确保在 [0, 1] 范围内
		if result[i] < 0 {
			result[i] = 0
		}
		if result[i] > 1 {
			result[i] = 1
		}
	}

	return result
}

// evaluateStitching 计算缝合函数
func (sf *ShadingFunction) evaluateStitching(t float64) []float64 {
	// 缝合函数暂时不实现，返回起始颜色
	debugPrintf("Warning: Stitching functions not yet implemented\n")
	if len(sf.C0) > 0 {
		return sf.C0
	}
	return []float64{0, 0, 0}
}
