package gopdf



// TransparencyGroup 表示 PDF 透明度组
// 透明度组是一种特殊的 XObject，用于实现高级透明度效果
type TransparencyGroup struct {
	Isolated   bool          // 是否隔离（不使用背景）
	Knockout   bool          // 是否敲除（内部对象不混合）
	ColorSpace string        // 颜色空间
	Surface    Surface // 用于渲染组内容的 Gopdf surface
}

// NewTransparencyGroup 创建新的透明度组
func NewTransparencyGroup(isolated, knockout bool, colorSpace string) *TransparencyGroup {
	return &TransparencyGroup{
		Isolated:   isolated,
		Knockout:   knockout,
		ColorSpace: colorSpace,
		Surface:    nil, // 将在渲染时创建
	}
}
