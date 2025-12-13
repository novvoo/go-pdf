package gopdf

// FormField 表示 PDF 表单字段
// 表单字段是交互式表单元素，如文本框、复选框、单选按钮等
type FormField struct {
	FieldType    string                 // 字段类型（Tx, Btn, Ch, Sig）
	FieldName    string                 // 字段名称
	Value        string                 // 字段当前值
	DefaultValue string                 // 字段默认值
	Rect         []float64              // 字段矩形 [x1 y1 x2 y2]
	Appearance   map[string]interface{} // 外观流字典（AP entry）
	Flags        int                    // 字段标志
	Options      []string               // 选项列表（用于选择字段）
}

// NewFormField 创建新的表单字段
func NewFormField(fieldType string) *FormField {
	return &FormField{
		FieldType:  fieldType,
		Rect:       make([]float64, 4),
		Appearance: make(map[string]interface{}),
		Flags:      0,
		Options:    make([]string, 0),
	}
}

// GetRect 获取字段矩形
func (f *FormField) GetRect() (x1, y1, x2, y2 float64) {
	if len(f.Rect) >= 4 {
		return f.Rect[0], f.Rect[1], f.Rect[2], f.Rect[3]
	}
	return 0, 0, 0, 0
}

// IsReadOnly 检查字段是否只读
func (f *FormField) IsReadOnly() bool {
	// 检查 ReadOnly 标志（bit 0）
	return (f.Flags & 0x01) != 0
}

// IsRequired 检查字段是否必填
func (f *FormField) IsRequired() bool {
	// 检查 Required 标志（bit 1）
	return (f.Flags & 0x02) != 0
}

// IsCheckbox 检查是否为复选框
func (f *FormField) IsCheckbox() bool {
	return f.FieldType == "/Btn" && (f.Flags&0x8000) == 0 // 非 Radio 按钮
}

// IsRadioButton 检查是否为单选按钮
func (f *FormField) IsRadioButton() bool {
	return f.FieldType == "/Btn" && (f.Flags&0x8000) != 0 // Radio 标志
}

// IsTextField 检查是否为文本字段
func (f *FormField) IsTextField() bool {
	return f.FieldType == "/Tx"
}

// IsChoiceField 检查是否为选择字段（下拉列表或列表框）
func (f *FormField) IsChoiceField() bool {
	return f.FieldType == "/Ch"
}

// IsChecked 检查复选框/单选按钮是否被选中
func (f *FormField) IsChecked() bool {
	// 值为 "Yes" 或 "On" 表示选中
	return f.Value == "Yes" || f.Value == "On" || f.Value == "/Yes" || f.Value == "/On"
}
