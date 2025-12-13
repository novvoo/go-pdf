package gopdf

import (
	"fmt"

	"github.com/novvoo/go-cairo/pkg/cairo"
)

// FormRenderer 表单字段渲染器
type FormRenderer struct {
	cairoCtx cairo.Context
}

// NewFormRenderer 创建新的表单字段渲染器
func NewFormRenderer(cairoCtx cairo.Context) *FormRenderer {
	return &FormRenderer{
		cairoCtx: cairoCtx,
	}
}

// RenderFormField 渲染表单字段（根据字段类型分发）
func (r *FormRenderer) RenderFormField(field *FormField) error {
	debugPrintf("[FormField] Rendering field: %s (type: %s)\n", field.FieldName, field.FieldType)

	// 根据字段类型分发
	if field.IsTextField() {
		return r.RenderTextField(field)
	} else if field.IsCheckbox() {
		return r.RenderCheckbox(field)
	} else if field.IsRadioButton() {
		return r.RenderRadioButton(field)
	} else if field.IsChoiceField() {
		return r.RenderChoiceField(field)
	} else {
		debugPrintf("[FormField] Unsupported field type: %s\n", field.FieldType)
		return nil
	}
}

// RenderTextField 渲染文本输入字段
func (r *FormRenderer) RenderTextField(field *FormField) error {
	x1, y1, x2, y2 := field.GetRect()

	// 保存状态
	r.cairoCtx.Save()
	defer r.cairoCtx.Restore()

	// 如果有外观流，优先使用外观流
	if len(field.Appearance) > 0 {
		debugPrintf("[FormField] Text field has appearance stream\n")
		// TODO: 渲染外观流
	}

	// 绘制文本框边框
	r.cairoCtx.Rectangle(x1, y1, x2-x1, y2-y1)
	r.cairoCtx.SetSourceRGB(0.8, 0.8, 0.8) // 浅灰色背景
	r.cairoCtx.FillPreserve()
	r.cairoCtx.SetSourceRGB(0.0, 0.0, 0.0) // 黑色边框
	r.cairoCtx.SetLineWidth(1.0)
	r.cairoCtx.Stroke()

	// 显示字段值或默认值
	displayValue := field.Value
	if displayValue == "" {
		displayValue = field.DefaultValue
	}

	if displayValue != "" {
		// 简化实现：显示文本（实际应该使用字体渲染）
		debugPrintf("[FormField] Text field value: %s\n", displayValue)
		// TODO: 使用字体渲染文本
	}

	debugPrintf("[FormField] Rendered text field at (%.2f, %.2f)\n", x1, y1)
	return nil
}

// RenderCheckbox 渲染复选框字段
func (r *FormRenderer) RenderCheckbox(field *FormField) error {
	x1, y1, x2, y2 := field.GetRect()

	// 保存状态
	r.cairoCtx.Save()
	defer r.cairoCtx.Restore()

	// 如果有外观流，优先使用外观流
	if len(field.Appearance) > 0 {
		debugPrintf("[FormField] Checkbox has appearance stream\n")
		// TODO: 渲染外观流
	}

	// 绘制复选框边框
	r.cairoCtx.Rectangle(x1, y1, x2-x1, y2-y1)
	r.cairoCtx.SetSourceRGB(1.0, 1.0, 1.0) // 白色背景
	r.cairoCtx.FillPreserve()
	r.cairoCtx.SetSourceRGB(0.0, 0.0, 0.0) // 黑色边框
	r.cairoCtx.SetLineWidth(1.0)
	r.cairoCtx.Stroke()

	// 如果选中，绘制勾选标记
	if field.IsChecked() {
		// 绘制简单的 X 标记
		padding := (x2 - x1) * 0.2
		r.cairoCtx.MoveTo(x1+padding, y1+padding)
		r.cairoCtx.LineTo(x2-padding, y2-padding)
		r.cairoCtx.MoveTo(x2-padding, y1+padding)
		r.cairoCtx.LineTo(x1+padding, y2-padding)
		r.cairoCtx.SetSourceRGB(0.0, 0.0, 0.0)
		r.cairoCtx.SetLineWidth(2.0)
		r.cairoCtx.Stroke()
		debugPrintf("[FormField] Checkbox is checked\n")
	} else {
		debugPrintf("[FormField] Checkbox is unchecked\n")
	}

	debugPrintf("[FormField] Rendered checkbox at (%.2f, %.2f)\n", x1, y1)
	return nil
}

// RenderRadioButton 渲染单选按钮字段
func (r *FormRenderer) RenderRadioButton(field *FormField) error {
	x1, y1, x2, y2 := field.GetRect()

	// 保存状态
	r.cairoCtx.Save()
	defer r.cairoCtx.Restore()

	// 计算圆心和半径
	centerX := (x1 + x2) / 2
	centerY := (y1 + y2) / 2
	radius := (x2 - x1) / 2
	if radius > (y2-y1)/2 {
		radius = (y2 - y1) / 2
	}

	// 绘制圆形边框
	r.cairoCtx.Arc(centerX, centerY, radius, 0, 6.28318530718) // 2*π
	r.cairoCtx.SetSourceRGB(1.0, 1.0, 1.0) // 白色背景
	r.cairoCtx.FillPreserve()
	r.cairoCtx.SetSourceRGB(0.0, 0.0, 0.0) // 黑色边框
	r.cairoCtx.SetLineWidth(1.0)
	r.cairoCtx.Stroke()

	// 如果选中，绘制内部圆点
	if field.IsChecked() {
		r.cairoCtx.Arc(centerX, centerY, radius*0.5, 0, 6.28318530718)
		r.cairoCtx.SetSourceRGB(0.0, 0.0, 0.0)
		r.cairoCtx.Fill()
		debugPrintf("[FormField] Radio button is selected\n")
	} else {
		debugPrintf("[FormField] Radio button is not selected\n")
	}

	debugPrintf("[FormField] Rendered radio button at (%.2f, %.2f)\n", centerX, centerY)
	return nil
}

// RenderChoiceField 渲染选择字段（下拉列表或列表框）
func (r *FormRenderer) RenderChoiceField(field *FormField) error {
	x1, y1, x2, y2 := field.GetRect()

	// 保存状态
	r.cairoCtx.Save()
	defer r.cairoCtx.Restore()

	// 绘制选择框边框
	r.cairoCtx.Rectangle(x1, y1, x2-x1, y2-y1)
	r.cairoCtx.SetSourceRGB(1.0, 1.0, 1.0) // 白色背景
	r.cairoCtx.FillPreserve()
	r.cairoCtx.SetSourceRGB(0.0, 0.0, 0.0) // 黑色边框
	r.cairoCtx.SetLineWidth(1.0)
	r.cairoCtx.Stroke()

	// 显示当前选中的值
	if field.Value != "" {
		debugPrintf("[FormField] Choice field value: %s\n", field.Value)
		// TODO: 使用字体渲染文本
	}

	debugPrintf("[FormField] Rendered choice field at (%.2f, %.2f)\n", x1, y1)
	return nil
}

// RenderFieldAppearance 渲染字段的外观流
func (r *FormRenderer) RenderFieldAppearance(field *FormField, appearanceKey string) error {
	// 获取外观流
	apObj, found := field.Appearance[appearanceKey]
	if !found {
		return fmt.Errorf("appearance %s not found", appearanceKey)
	}

	// TODO: 解析并渲染外观流
	// 外观流是一个 XObject Form，需要像渲染 Form XObject 一样处理
	debugPrintf("[FormField] Appearance stream rendering not yet implemented\n")

	_ = apObj // 避免未使用变量警告
	return nil
}
