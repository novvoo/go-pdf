package gopdf_test

import (
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
)

// TestMarkedContentStack 测试标记内容栈
func TestMarkedContentStack(t *testing.T) {
	stack := gopdf.NewMarkedContentStack()

	// 测试空栈
	if stack.Depth() != 0 {
		t.Errorf("Expected empty stack depth 0, got %d", stack.Depth())
	}

	// 测试 Push
	stack.Push("P", nil)
	if stack.Depth() != 1 {
		t.Errorf("Expected stack depth 1, got %d", stack.Depth())
	}

	// 测试嵌套
	stack.Push("Span", map[string]interface{}{"Lang": "en"})
	if stack.Depth() != 2 {
		t.Errorf("Expected stack depth 2, got %d", stack.Depth())
	}

	// 测试 Current
	current := stack.Current()
	if current == nil {
		t.Error("Expected current section, got nil")
	} else if current.Tag != "Span" {
		t.Errorf("Expected current tag 'Span', got '%s'", current.Tag)
	}

	// 测试 Pop
	popped := stack.Pop()
	if popped == nil {
		t.Error("Expected popped section, got nil")
	} else if popped.Tag != "Span" {
		t.Errorf("Expected popped tag 'Span', got '%s'", popped.Tag)
	}

	if stack.Depth() != 1 {
		t.Errorf("Expected stack depth 1 after pop, got %d", stack.Depth())
	}

	// 测试 Pop 到空
	stack.Pop()
	if stack.Depth() != 0 {
		t.Errorf("Expected stack depth 0 after final pop, got %d", stack.Depth())
	}
}

// TestBlendModes 测试混合模式
func TestBlendModes(t *testing.T) {
	testCases := []struct {
		pdfMode   string
		shouldMap bool
	}{
		{"Normal", true},
		{"Multiply", true},
		{"Screen", true},
		{"Overlay", true},
		{"Darken", true},
		{"Lighten", true},
		{"ColorDodge", true},
		{"ColorBurn", true},
		{"HardLight", true},
		{"SoftLight", true},
		{"Difference", true},
		{"Exclusion", true},
		{"InvalidMode", true}, // 应该回退到 Normal
	}

	for _, tc := range testCases {
		t.Run(tc.pdfMode, func(t *testing.T) {
			cairoMode := gopdf.GetCairoBlendMode(tc.pdfMode)
			if cairoMode < 0 {
				t.Errorf("GetCairoBlendMode(%s) returned invalid mode: %d", tc.pdfMode, cairoMode)
			}
		})
	}
}

// TestTransparencyGroup 测试透明度组
func TestTransparencyGroup(t *testing.T) {
	// 测试创建透明度组
	group := gopdf.NewTransparencyGroup(true, false, "DeviceRGB")

	if !group.Isolated {
		t.Error("Expected Isolated to be true")
	}

	if group.Knockout {
		t.Error("Expected Knockout to be false")
	}

	if group.ColorSpace != "DeviceRGB" {
		t.Errorf("Expected ColorSpace 'DeviceRGB', got '%s'", group.ColorSpace)
	}
}

// TestAnnotationStructure 测试注释结构
func TestAnnotationStructure(t *testing.T) {
	// 测试创建注释
	annot := gopdf.NewAnnotation("/Text")

	if annot.Subtype != "/Text" {
		t.Errorf("Expected Subtype '/Text', got '%s'", annot.Subtype)
	}

	// 测试设置矩形
	annot.Rect = []float64{100, 200, 150, 250}
	x1, y1, x2, y2 := annot.GetRect()
	if x1 != 100 || y1 != 200 || x2 != 150 || y2 != 250 {
		t.Errorf("GetRect() returned incorrect values: (%.2f, %.2f, %.2f, %.2f)", x1, y1, x2, y2)
	}

	// 测试设置颜色
	annot.Color = []float64{1.0, 0.5, 0.0}
	r, g, b := annot.GetColor()
	if r != 1.0 || g != 0.5 || b != 0.0 {
		t.Errorf("GetColor() returned incorrect values: (%.2f, %.2f, %.2f)", r, g, b)
	}

	// 测试可见性标志
	annot.Flags = 0x00 // 可见
	if !annot.IsVisible() {
		t.Error("Expected annotation to be visible")
	}

	annot.Flags = 0x02 // Hidden 标志
	if annot.IsVisible() {
		t.Error("Expected annotation to be hidden")
	}

	// 测试打印标志
	annot.Flags = 0x04 // Print 标志
	if !annot.IsPrintable() {
		t.Error("Expected annotation to be printable")
	}
}

// TestFormFieldStructure 测试表单字段结构
func TestFormFieldStructure(t *testing.T) {
	// 测试创建文本字段
	field := gopdf.NewFormField("/Tx")

	if field.FieldType != "/Tx" {
		t.Errorf("Expected FieldType '/Tx', got '%s'", field.FieldType)
	}

	if !field.IsTextField() {
		t.Error("Expected field to be a text field")
	}

	// 测试复选框
	checkbox := gopdf.NewFormField("/Btn")
	checkbox.Flags = 0x00 // 非 Radio
	if !checkbox.IsCheckbox() {
		t.Error("Expected field to be a checkbox")
	}

	// 测试单选按钮
	radio := gopdf.NewFormField("/Btn")
	radio.Flags = 0x8000 // Radio 标志
	if !radio.IsRadioButton() {
		t.Error("Expected field to be a radio button")
	}

	// 测试选中状态
	checkbox.Value = "Yes"
	if !checkbox.IsChecked() {
		t.Error("Expected checkbox to be checked")
	}

	checkbox.Value = "Off"
	if checkbox.IsChecked() {
		t.Error("Expected checkbox to be unchecked")
	}

	// 测试只读标志
	field.Flags = 0x01 // ReadOnly
	if !field.IsReadOnly() {
		t.Error("Expected field to be read-only")
	}

	// 测试必填标志
	field.Flags = 0x02 // Required
	if !field.IsRequired() {
		t.Error("Expected field to be required")
	}
}

// TestShadingStructure 测试渐变结构
func TestShadingStructure(t *testing.T) {
	// 测试线性渐变
	linearShading := &gopdf.Shading{
		ShadingType: 2,
		ColorSpace:  "DeviceRGB",
		Coords:      []float64{0, 0, 100, 100},
	}

	if !linearShading.IsLinearGradient() {
		t.Error("Expected shading to be linear gradient")
	}

	if linearShading.IsRadialGradient() {
		t.Error("Expected shading not to be radial gradient")
	}

	// 测试径向渐变
	radialShading := &gopdf.Shading{
		ShadingType: 3,
		ColorSpace:  "DeviceRGB",
		Coords:      []float64{50, 50, 0, 50, 50, 100},
	}

	if !radialShading.IsRadialGradient() {
		t.Error("Expected shading to be radial gradient")
	}

	if radialShading.IsLinearGradient() {
		t.Error("Expected shading not to be linear gradient")
	}
}

// TestPatternStructure 测试图案结构
func TestPatternStructure(t *testing.T) {
	// 测试创建图案
	pattern := &gopdf.Pattern{
		PatternType: 1,
		PaintType:   1,
		TilingType:  1,
		BBox:        []float64{0, 0, 10, 10},
		XStep:       10,
		YStep:       10,
	}

	if pattern.PatternType != 1 {
		t.Errorf("Expected PatternType 1, got %d", pattern.PatternType)
	}

	if pattern.XStep != 10 || pattern.YStep != 10 {
		t.Errorf("Expected XStep=10, YStep=10, got XStep=%.2f, YStep=%.2f", pattern.XStep, pattern.YStep)
	}
}

// TestAnnotationExtraction 测试从真实 PDF 提取注释
func TestAnnotationExtraction(t *testing.T) {
	pdfPath := "../test.pdf"

	// 检查文件是否存在
	if _, err := api.PageCountFile(pdfPath); err != nil {
		t.Skipf("Skipping test: PDF file not found: %s", pdfPath)
		return
	}

	// 打开 PDF
	ctx, err := api.ReadContextFile(pdfPath)
	if err != nil {
		t.Fatalf("Failed to read PDF: %v", err)
	}

	// 获取第一页
	pageDict, _, _, err := ctx.PageDict(1, false)
	if err != nil {
		t.Fatalf("Failed to get page dict: %v", err)
	}

	// 提取注释
	annotations, err := gopdf.ExtractAnnotations(ctx, pageDict)
	if err != nil {
		t.Fatalf("Failed to extract annotations: %v", err)
	}

	t.Logf("Found %d annotations on page 1", len(annotations))

	// 验证注释结构
	for i, annot := range annotations {
		if annot.Subtype == "" {
			t.Errorf("Annotation %d has empty Subtype", i)
		}

		if len(annot.Rect) != 4 {
			t.Errorf("Annotation %d has invalid Rect length: %d", i, len(annot.Rect))
		}

		t.Logf("Annotation %d: Type=%s, Rect=(%.2f, %.2f, %.2f, %.2f)",
			i, annot.Subtype, annot.Rect[0], annot.Rect[1], annot.Rect[2], annot.Rect[3])
	}
}

// TestFormFieldExtraction 测试从真实 PDF 提取表单字段
func TestFormFieldExtraction(t *testing.T) {
	pdfPath := "../test.pdf"

	// 检查文件是否存在
	if _, err := api.PageCountFile(pdfPath); err != nil {
		t.Skipf("Skipping test: PDF file not found: %s", pdfPath)
		return
	}

	// 打开 PDF
	ctx, err := api.ReadContextFile(pdfPath)
	if err != nil {
		t.Fatalf("Failed to read PDF: %v", err)
	}

	// 提取表单字段
	formFields, err := gopdf.ExtractFormFields(ctx)
	if err != nil {
		t.Fatalf("Failed to extract form fields: %v", err)
	}

	t.Logf("Found %d form fields in document", len(formFields))

	// 验证表单字段结构
	for i, field := range formFields {
		if field.FieldType == "" {
			t.Errorf("Form field %d has empty FieldType", i)
		}

		t.Logf("Form field %d: Name=%s, Type=%s, Value=%s",
			i, field.FieldName, field.FieldType, field.Value)
	}
}

// TestAdvancedFeaturesIntegration 测试高级功能集成
func TestAdvancedFeaturesIntegration(t *testing.T) {
	pdfPath := "../test.pdf"
	outputPath := "../test_advanced.png"

	// 检查文件是否存在
	if _, err := api.PageCountFile(pdfPath); err != nil {
		t.Skipf("Skipping test: PDF file not found: %s", pdfPath)
		return
	}

	// 创建 PDF 读取器
	reader := gopdf.NewPDFReader(pdfPath)

	// 渲染页面（包含所有高级功能）
	err := reader.RenderPageToPNG(1, outputPath, 150)
	if err != nil {
		t.Fatalf("Failed to render PDF with advanced features: %v", err)
	}

	t.Logf("Successfully rendered PDF with advanced features to %s", outputPath)

	// 清理
	// os.Remove(outputPath)
}
