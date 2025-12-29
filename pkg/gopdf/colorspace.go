package gopdf

import (
	"fmt"
	"math"
)

// ColorSpace 颜色空间接口
type ColorSpace interface {
	GetName() string
	GetNumComponents() int
	ConvertToRGB(components []float64) (r, g, b float64, err error)
	ConvertToRGBA(components []float64, alpha float64) (r, g, b, a float64, err error)
}

// DeviceRGBColorSpace RGB 设备颜色空间
type DeviceRGBColorSpace struct{}

func (cs *DeviceRGBColorSpace) GetName() string       { return "DeviceRGB" }
func (cs *DeviceRGBColorSpace) GetNumComponents() int { return 3 }

func (cs *DeviceRGBColorSpace) ConvertToRGB(components []float64) (r, g, b float64, err error) {
	if len(components) < 3 {
		return 0, 0, 0, fmt.Errorf("rgb requires 3 components, got %d", len(components))
	}
	return clamp01(components[0]), clamp01(components[1]), clamp01(components[2]), nil
}

func (cs *DeviceRGBColorSpace) ConvertToRGBA(components []float64, alpha float64) (r, g, b, a float64, err error) {
	r, g, b, err = cs.ConvertToRGB(components)
	return r, g, b, clamp01(alpha), err
}

// DeviceGrayColorSpace 灰度设备颜色空间
type DeviceGrayColorSpace struct{}

func (cs *DeviceGrayColorSpace) GetName() string       { return "DeviceGray" }
func (cs *DeviceGrayColorSpace) GetNumComponents() int { return 1 }

func (cs *DeviceGrayColorSpace) ConvertToRGB(components []float64) (r, g, b float64, err error) {
	if len(components) < 1 {
		return 0, 0, 0, fmt.Errorf("gray requires 1 component, got %d", len(components))
	}
	gray := clamp01(components[0])
	return gray, gray, gray, nil
}

func (cs *DeviceGrayColorSpace) ConvertToRGBA(components []float64, alpha float64) (r, g, b, a float64, err error) {
	r, g, b, err = cs.ConvertToRGB(components)
	return r, g, b, clamp01(alpha), err
}

// DeviceCMYKColorSpace CMYK 设备颜色空间
type DeviceCMYKColorSpace struct{}

func (cs *DeviceCMYKColorSpace) GetName() string       { return "DeviceCMYK" }
func (cs *DeviceCMYKColorSpace) GetNumComponents() int { return 4 }

func (cs *DeviceCMYKColorSpace) ConvertToRGB(components []float64) (r, g, b float64, err error) {
	if len(components) < 4 {
		return 0, 0, 0, fmt.Errorf("cmyk requires 4 components, got %d", len(components))
	}
	c := clamp01(components[0])
	m := clamp01(components[1])
	y := clamp01(components[2])
	k := clamp01(components[3])

	// CMYK 到 RGB 转换
	r = (1 - c) * (1 - k)
	g = (1 - m) * (1 - k)
	b = (1 - y) * (1 - k)

	return r, g, b, nil
}

func (cs *DeviceCMYKColorSpace) ConvertToRGBA(components []float64, alpha float64) (r, g, b, a float64, err error) {
	r, g, b, err = cs.ConvertToRGB(components)
	return r, g, b, clamp01(alpha), err
}

// CalRGBColorSpace 校准 RGB 颜色空间
type CalRGBColorSpace struct {
	WhitePoint []float64 // XYZ 白点
	BlackPoint []float64 // XYZ 黑点
	Gamma      []float64 // RGB gamma 值
	Matrix     []float64 // 3x3 变换矩阵
}

func (cs *CalRGBColorSpace) GetName() string       { return "CalRGB" }
func (cs *CalRGBColorSpace) GetNumComponents() int { return 3 }

func (cs *CalRGBColorSpace) ConvertToRGB(components []float64) (r, g, b float64, err error) {
	if len(components) < 3 {
		return 0, 0, 0, fmt.Errorf("calRGB requires 3 components, got %d", len(components))
	}

	// 应用 gamma 校正
	a := math.Pow(clamp01(components[0]), cs.getGamma(0))
	b_val := math.Pow(clamp01(components[1]), cs.getGamma(1))
	c := math.Pow(clamp01(components[2]), cs.getGamma(2))

	// 应用矩阵变换（如果有）
	if len(cs.Matrix) == 9 {
		x := cs.Matrix[0]*a + cs.Matrix[1]*b_val + cs.Matrix[2]*c
		y := cs.Matrix[3]*a + cs.Matrix[4]*b_val + cs.Matrix[5]*c
		z := cs.Matrix[6]*a + cs.Matrix[7]*b_val + cs.Matrix[8]*c

		// XYZ 到 RGB 转换（简化版）
		r = 3.2406*x - 1.5372*y - 0.4986*z
		g = -0.9689*x + 1.8758*y + 0.0415*z
		b = 0.0557*x - 0.2040*y + 1.0570*z

		return clamp01(r), clamp01(g), clamp01(b), nil
	}

	// 没有矩阵，直接使用 gamma 校正后的值
	return clamp01(a), clamp01(b_val), clamp01(c), nil
}

func (cs *CalRGBColorSpace) ConvertToRGBA(components []float64, alpha float64) (r, g, b, a float64, err error) {
	r, g, b, err = cs.ConvertToRGB(components)
	return r, g, b, clamp01(alpha), err
}

func (cs *CalRGBColorSpace) getGamma(index int) float64 {
	if len(cs.Gamma) > index {
		return cs.Gamma[index]
	}
	return 1.0 // 默认 gamma
}

// CalGrayColorSpace 校准灰度颜色空间
type CalGrayColorSpace struct {
	WhitePoint []float64 // XYZ 白点
	BlackPoint []float64 // XYZ 黑点
	Gamma      float64   // Gamma 值
}

func (cs *CalGrayColorSpace) GetName() string       { return "CalGray" }
func (cs *CalGrayColorSpace) GetNumComponents() int { return 1 }

func (cs *CalGrayColorSpace) ConvertToRGB(components []float64) (r, g, b float64, err error) {
	if len(components) < 1 {
		return 0, 0, 0, fmt.Errorf("calGray requires 1 component, got %d", len(components))
	}

	// 应用 gamma 校正
	gray := math.Pow(clamp01(components[0]), cs.Gamma)
	return gray, gray, gray, nil
}

func (cs *CalGrayColorSpace) ConvertToRGBA(components []float64, alpha float64) (r, g, b, a float64, err error) {
	r, g, b, err = cs.ConvertToRGB(components)
	return r, g, b, clamp01(alpha), err
}

// LabColorSpace Lab 颜色空间
type LabColorSpace struct {
	WhitePoint []float64 // XYZ 白点
	BlackPoint []float64 // XYZ 黑点
	Range      []float64 // a* 和 b* 的范围 [amin amax bmin bmax]
}

func (cs *LabColorSpace) GetName() string       { return "Lab" }
func (cs *LabColorSpace) GetNumComponents() int { return 3 }

func (cs *LabColorSpace) ConvertToRGB(components []float64) (r, g, b float64, err error) {
	if len(components) < 3 {
		return 0, 0, 0, fmt.Errorf("lab requires 3 components, got %d", len(components))
	}

	// Lab 到 XYZ 转换
	L := components[0] * 100 // L* 范围 0-100
	a := components[1]       // a* 范围通常 -128 到 127
	b_val := components[2]   // b* 范围通常 -128 到 127

	// 应用范围限制
	if len(cs.Range) >= 4 {
		a = clampRange(a, cs.Range[0], cs.Range[1])
		b_val = clampRange(b_val, cs.Range[2], cs.Range[3])
	}

	fy := (L + 16) / 116
	fx := a/500 + fy
	fz := fy - b_val/200

	// 获取白点（默认 D65）
	xn, yn, zn := 0.95047, 1.0, 1.08883
	if len(cs.WhitePoint) >= 3 {
		xn, yn, zn = cs.WhitePoint[0], cs.WhitePoint[1], cs.WhitePoint[2]
	}

	// 反函数
	x := xn * labInvF(fx)
	y := yn * labInvF(fy)
	z := zn * labInvF(fz)

	// XYZ 到 RGB 转换（sRGB）
	r = 3.2406*x - 1.5372*y - 0.4986*z
	g = -0.9689*x + 1.8758*y + 0.0415*z
	b = 0.0557*x - 0.2040*y + 1.0570*z

	// Gamma 校正
	r = srgbGamma(r)
	g = srgbGamma(g)
	b = srgbGamma(b)

	return clamp01(r), clamp01(g), clamp01(b), nil
}

func (cs *LabColorSpace) ConvertToRGBA(components []float64, alpha float64) (r, g, b, a float64, err error) {
	r, g, b, err = cs.ConvertToRGB(components)
	return r, g, b, clamp01(alpha), err
}

// ICCBasedColorSpace 基于 ICC 配置文件的颜色空间
type ICCBasedColorSpace struct {
	NumComponents int
	Alternate     ColorSpace // 备用颜色空间
	Range         []float64  // 组件范围
	Metadata      []byte     // ICC 配置文件数据
}

func (cs *ICCBasedColorSpace) GetName() string       { return "ICCBased" }
func (cs *ICCBasedColorSpace) GetNumComponents() int { return cs.NumComponents }

func (cs *ICCBasedColorSpace) ConvertToRGB(components []float64) (r, g, b float64, err error) {
	// 如果有备用颜色空间，使用它
	if cs.Alternate != nil {
		return cs.Alternate.ConvertToRGB(components)
	}

	// 否则根据组件数量猜测
	switch cs.NumComponents {
	case 1:
		// 灰度
		gray := clamp01(components[0])
		return gray, gray, gray, nil
	case 3:
		// RGB
		return clamp01(components[0]), clamp01(components[1]), clamp01(components[2]), nil
	case 4:
		// CMYK
		return (&DeviceCMYKColorSpace{}).ConvertToRGB(components)
	default:
		return 0, 0, 0, fmt.Errorf("unsupported ICC component count: %d", cs.NumComponents)
	}
}

func (cs *ICCBasedColorSpace) ConvertToRGBA(components []float64, alpha float64) (r, g, b, a float64, err error) {
	r, g, b, err = cs.ConvertToRGB(components)
	return r, g, b, clamp01(alpha), err
}

// IndexedColorSpace 索引颜色空间
type IndexedColorSpace struct {
	Base   ColorSpace // 基础颜色空间
	HiVal  int        // 最大索引值
	Lookup []byte     // 查找表
}

func (cs *IndexedColorSpace) GetName() string       { return "Indexed" }
func (cs *IndexedColorSpace) GetNumComponents() int { return 1 }

func (cs *IndexedColorSpace) ConvertToRGB(components []float64) (r, g, b float64, err error) {
	if len(components) < 1 {
		return 0, 0, 0, fmt.Errorf("indexed requires 1 component")
	}

	index := int(components[0])
	if index < 0 || index > cs.HiVal {
		return 0, 0, 0, fmt.Errorf("index %d out of range [0, %d]", index, cs.HiVal)
	}

	// 从查找表获取颜色分量
	numComponents := cs.Base.GetNumComponents()
	offset := index * numComponents

	if offset+numComponents > len(cs.Lookup) {
		return 0, 0, 0, fmt.Errorf("lookup table too small")
	}

	// 提取分量并转换为 0-1 范围
	baseComponents := make([]float64, numComponents)
	for i := 0; i < numComponents; i++ {
		baseComponents[i] = float64(cs.Lookup[offset+i]) / 255.0
	}

	// 使用基础颜色空间转换
	return cs.Base.ConvertToRGB(baseComponents)
}

func (cs *IndexedColorSpace) ConvertToRGBA(components []float64, alpha float64) (r, g, b, a float64, err error) {
	r, g, b, err = cs.ConvertToRGB(components)
	return r, g, b, clamp01(alpha), err
}

// 辅助函数

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func clampRange(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

func labInvF(t float64) float64 {
	delta := 6.0 / 29.0
	if t > delta {
		return t * t * t
	}
	return 3 * delta * delta * (t - 4.0/29.0)
}

func srgbGamma(v float64) float64 {
	if v <= 0.0031308 {
		return 12.92 * v
	}
	return 1.055*math.Pow(v, 1/2.4) - 0.055
}

// GetColorSpace 根据名称获取颜色空间
func GetColorSpace(name string) ColorSpace {
	switch name {
	case "DeviceRGB", "/DeviceRGB":
		return &DeviceRGBColorSpace{}
	case "DeviceGray", "/DeviceGray":
		return &DeviceGrayColorSpace{}
	case "DeviceCMYK", "/DeviceCMYK":
		return &DeviceCMYKColorSpace{}
	default:
		// 默认返回 RGB
		return &DeviceRGBColorSpace{}
	}
}

func RgbToHSL(r, g, b float64) (h, s, l float64) {
	return rgbToHSL(r, g, b)
}

// rgb到HSL转换（内部）
func rgbToHSL(r, g, b float64) (h, s, l float64) {
	max := math.Max(math.Max(r, g), b)
	min := math.Min(math.Min(r, g), b)
	l = (max + min) / 2

	if max == min {
		h, s = 0, 0 // 灰色
	} else {
		d := max - min
		if l > 0.5 {
			s = d / (2 - max - min)
		} else {
			s = d / (max + min)
		}

		switch max {
		case r:
			h = (g - b) / d
			if g < b {
				h += 6
			}
		case g:
			h = (b-r)/d + 2
		case b:
			h = (r-g)/d + 4
		}
		h /= 6
	}
	return
}

// HslToRGB HSL 到 RGB 转换
func HslToRGB(h, s, l float64) (r, g, b float64) {
	return hslToRGB(h, s, l)
}

// hsl到RGB转换（内部）
func hslToRGB(h, s, l float64) (r, g, b float64) {
	if s == 0 {
		r, g, b = l, l, l // 灰色
	} else {
		var q float64
		if l < 0.5 {
			q = l * (1 + s)
		} else {
			q = l + s - l*s
		}
		p := 2*l - q
		r = hueToRGB(p, q, h+1.0/3.0)
		g = hueToRGB(p, q, h)
		b = hueToRGB(p, q, h-1.0/3.0)
	}
	return
}

func hueToRGB(p, q, t float64) float64 {
	if t < 0 {
		t += 1
	}
	if t > 1 {
		t -= 1
	}
	if t < 1.0/6.0 {
		return p + (q-p)*6*t
	}
	if t < 1.0/2.0 {
		return q
	}
	if t < 2.0/3.0 {
		return p + (q-p)*(2.0/3.0-t)*6
	}
	return p
}

// ColorDeltaE2000 calculates color difference using Delta E 2000 algorithm
func ColorDeltaE2000(l1, a1, b1, l2, a2, b2 float64) float64 {
	// 简化的 Delta E 2000 实现
	const kL, kC, kH = 1.0, 1.0, 1.0

	c1 := math.Sqrt(a1*a1 + b1*b1)
	c2 := math.Sqrt(a2*a2 + b2*b2)
	cBar := (c1 + c2) / 2

	g := 0.5 * (1 - math.Sqrt(math.Pow(cBar, 7)/(math.Pow(cBar, 7)+math.Pow(25, 7))))

	a1p := (1 + g) * a1
	a2p := (1 + g) * a2

	c1p := math.Sqrt(a1p*a1p + b1*b1)
	c2p := math.Sqrt(a2p*a2p + b2*b2)

	h1p := math.Atan2(b1, a1p)
	h2p := math.Atan2(b2, a2p)

	if h1p < 0 {
		h1p += 2 * math.Pi
	}
	if h2p < 0 {
		h2p += 2 * math.Pi
	}

	dL := l2 - l1
	dC := c2p - c1p
	dH := h2p - h1p

	if dH > math.Pi {
		dH -= 2 * math.Pi
	} else if dH < -math.Pi {
		dH += 2 * math.Pi
	}

	dH = 2 * math.Sqrt(c1p*c2p) * math.Sin(dH/2)

	lBar := (l1 + l2) / 2
	cBar = (c1p + c2p) / 2

	sL := 1 + (0.015*(lBar-50)*(lBar-50))/math.Sqrt(20+(lBar-50)*(lBar-50))
	sC := 1 + 0.045*cBar
	sH := 1 + 0.015*cBar

	deltaE := math.Sqrt(
		math.Pow(dL/(kL*sL), 2) +
			math.Pow(dC/(kC*sC), 2) +
			math.Pow(dH/(kH*sH), 2))

	return deltaE
}
