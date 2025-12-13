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

// NewFormField 创建新的表单