package gopdf

// Resources 表示 PDF 资源字典
type Resources struct {
	// 扩展图形状态
	ExtGState map[string]map[string]interface{}
	// 颜色空间
	ColorSpace map[string]interface{}
	// 图案
	Pattern map[string]interface{}
	// 阴影
	Shading map[string]interface{}
	// XObject（表单和图像）
	XObject map[string]*XObject
	// 字体
	Font map[string]*Font
	// 属性列表
	Properties map[string]interface{}
}

// NewResources 创建新的资源字典
func NewResources() *Resources {
	return &Resources{
		ExtGState:  make(map[string]map[string]interface{}),
		ColorSpace: make(map[string]interface{}),
		Pattern:    make(map[string]interface{}),
		Shading:    make(map[string]interface{}),
		XObject:    make(map[string]*XObject),
		Font:       make(map[string]*Font),
		Properties: make(map[string]interface{}),
	}
}

// GetExtGState 获取扩展图形状态
func (r *Resources) GetExtGState(name string) map[string]interface{} {
	return r.ExtGState[name]
}

// SetExtGState 设置扩展图形状态
func (r *Resources) SetExtGState(name string, state map[string]interface{}) {
	r.ExtGState[name] = state
}

// GetXObject 获取 XObject
func (r *Resources) GetXObject(name string) *XObject {
	return r.XObject[name]
}

// SetXObject 设置 XObject
func (r *Resources) SetXObject(name string, xobj *XObject) {
	r.XObject[name] = xobj
}

// GetFont 获取字体
func (r *Resources) GetFont(name string) *Font {
	return r.Font[name]
}

// SetFont 设置字体
func (r *Resources) SetFont(name string, font *Font) {
	r.Font[name] = font
}

// AddFont 添加字体（别名）
func (r *Resources) AddFont(name string, font *Font) {
	r.SetFont(name, font)
}

// AddXObject 添加 XObject（别名）
func (r *Resources) AddXObject(name string, xobj *XObject) {
	r.SetXObject(name, xobj)
}

// AddExtGState 添加扩展图形状态（别名）
func (r *Resources) AddExtGState(name string, state map[string]interface{}) {
	r.SetExtGState(name, state)
}

// GetColorSpace 获取颜色空间
func (r *Resources) GetColorSpace(name string) interface{} {
	return r.ColorSpace[name]
}

// SetColorSpace 设置颜色空间
func (r *Resources) SetColorSpace(name string, cs interface{}) {
	r.ColorSpace[name] = cs
}

// GetPattern 获取图案
func (r *Resources) GetPattern(name string) interface{} {
	return r.Pattern[name]
}

// SetPattern 设置图案
func (r *Resources) SetPattern(name string, pattern interface{}) {
	r.Pattern[name] = pattern
}

// GetShading 获取阴影
func (r *Resources) GetShading(name string) interface{} {
	return r.Shading[name]
}

// SetShading 设置阴影
func (r *Resources) SetShading(name string, shading interface{}) {
	r.Shading[name] = shading
}

// Merge 合并另一个资源字典
func (r *Resources) Merge(other *Resources) {
	if other == nil {
		return
	}

	for k, v := range other.ExtGState {
		r.ExtGState[k] = v
	}
	for k, v := range other.ColorSpace {
		r.ColorSpace[k] = v
	}
	for k, v := range other.Pattern {
		r.Pattern[k] = v
	}
	for k, v := range other.Shading {
		r.Shading[k] = v
	}
	for k, v := range other.XObject {
		r.XObject[k] = v
	}
	for k, v := range other.Font {
		r.Font[k] = v
	}
	for k, v := range other.Properties {
		r.Properties[k] = v
	}
}

// Clone 复制资源字典
func (r *Resources) Clone() *Resources {
	newRes := NewResources()

	for k, v := range r.ExtGState {
		newState := make(map[string]interface{})
		for sk, sv := range v {
			newState[sk] = sv
		}
		newRes.ExtGState[k] = newState
	}

	for k, v := range r.ColorSpace {
		newRes.ColorSpace[k] = v
	}
	for k, v := range r.Pattern {
		newRes.Pattern[k] = v
	}
	for k, v := range r.Shading {
		newRes.Shading[k] = v
	}
	for k, v := range r.XObject {
		newRes.XObject[k] = v
	}
	for k, v := range r.Font {
		newRes.Font[k] = v
	}
	for k, v := range r.Properties {
		newRes.Properties[k] = v
	}

	return newRes
}

// CountShadings 返回资源中的渐变数量
func (r *Resources) CountShadings() int {
	return len(r.Shading)
}

// CountPatterns 返回资源中的图案数量
func (r *Resources) CountPatterns() int {
	return len(r.Pattern)
}

// CountExtGStates 返回资源中的扩展图形状态数量
func (r *Resources) CountExtGStates() int {
	return len(r.ExtGState)
}

// GetAllXObjects 返回所有 XObject
func (r *Resources) GetAllXObjects() []*XObject {
	xobjects := make([]*XObject, 0, len(r.XObject))
	for _, xobj := range r.XObject {
		xobjects = append(xobjects, xobj)
	}
	return xobjects
}
