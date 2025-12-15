package gopdf_test

import (
	"fmt"
	"math"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// PDFSpecValidator PDF规范验证器
type PDFSpecValidator struct {
	errors   []string
	warnings []string
}

// NewPDFSpecValidator 创建新的验证器
func NewPDFSpecValidator() *PDFSpecValidator {
	return &PDFSpecValidator{
		errors:   make([]string, 0),
		warnings: make([]string, 0),
	}
}

// ValidateOperators 验证操作符序列是否符合PDF规范
func (v *PDFSpecValidator) ValidateOperators(operators []gopdf.PDFOperator) {
	v.errors = make([]string, 0)
	v.warnings = make([]string, 0)

	// 状态跟踪
	graphicsStateDepth := 0
	inTextObject := false
	inPathConstruction := false
	hasPathOperations := false

	for i, op := range operators {
		opName := op.Name()

		// 跳过忽略的操作符
		if opName == "IGNORE" {
			continue
		}

		// 验证图形状态操作符
		switch opName {
		case "q":
			graphicsStateDepth++
			if graphicsStateDepth > 28 {
				v.addWarning(i, opName, "图形状态栈深度超过推荐值28")
			}

		case "Q":
			graphicsStateDepth--
			if graphicsStateDepth < 0 {
				v.addError(i, opName, "图形状态栈下溢：Q操作符多于q操作符")
			}

		case "cm":
			v.validateConcatMatrix(i, op)

		case "w":
			v.validateLineWidth(i, op)

		case "J":
			v.validateLineCap(i, op)

		case "j":
			v.validateLineJoin(i, op)

		case "M":
			v.validateMiterLimit(i, op)

		case "d":
			v.validateDashPattern(i, op)
		}

		// 验证颜色操作符
		switch opName {
		case "RG", "rg":
			v.validateRGBColor(i, op)
		case "G", "g":
			v.validateGrayColor(i, op)
		case "K", "k":
			v.validateCMYKColor(i, op)
		}

		// 验证路径操作符
		switch opName {
		case "m", "l", "c", "v", "y", "re", "h":
			inPathConstruction = true
			hasPathOperations = true
		case "S", "s", "f", "F", "f*", "B", "B*", "b", "b*", "n":
			if !hasPathOperations {
				v.addWarning(i, opName, "路径绘制操作符前没有路径构造操作符")
			}
			inPathConstruction = false
			hasPathOperations = false
		case "W", "W*":
			if !inPathConstruction {
				v.addError(i, opName, "裁剪操作符必须在路径构造后、路径绘制前使用")
			}
		}

		// 验证文本操作符
		switch opName {
		case "BT":
			if inTextObject {
				v.addError(i, opName, "嵌套的文本对象：BT操作符在另一个文本对象内")
			}
			inTextObject = true

		case "ET":
			if !inTextObject {
				v.addError(i, opName, "ET操作符在文本对象外")
			}
			inTextObject = false

		case "Tm", "Td", "TD", "T*", "Tc", "Tw", "Tz", "TL", "Tf", "Tr", "Ts", "Tj", "'", "\"", "TJ":
			if !inTextObject {
				v.addError(i, opName, "文本操作符必须在BT和ET之间使用")
			}
			v.validateTextOperator(i, op)
		}

		// 验证XObject操作符
		if opName == "Do" {
			if inTextObject {
				v.addWarning(i, opName, "在文本对象内使用Do操作符（某些PDF查看器可能不支持）")
			}
		}
	}

	// 最终状态检查
	if graphicsStateDepth != 0 {
		v.addError(-1, "EOF", fmt.Sprintf("图形状态栈不平衡：%d个未配对的q操作符", graphicsStateDepth))
	}

	if inTextObject {
		v.addError(-1, "EOF", "文本对象未关闭：缺少ET操作符")
	}
}

// 验证矩阵变换
func (v *PDFSpecValidator) validateConcatMatrix(index int, op gopdf.PDFOperator) {
	if cmOp, ok := op.(*gopdf.OpConcatMatrix); ok {
		m := cmOp.Matrix

		// 检查矩阵是否可逆（行列式不为0）
		det := m.A*m.D - m.B*m.C

		// 检查是否有极端的缩放
		scaleX := math.Sqrt(m.A*m.A + m.B*m.B)
		scaleY := math.Sqrt(m.C*m.C + m.D*m.D)

		// 如果行列式非常小，检查是否是由于极小缩放导致的
		if math.Abs(det) < 1e-10 {
			// 如果缩放因子很小（<= 1e-5），这是警告而不是错误
			if scaleX <= 1e-5 || scaleY <= 1e-5 {
				v.addWarning(index, "cm", fmt.Sprintf("极小的缩放因子导致矩阵接近奇异：X=%.2e, Y=%.2e, det=%.2e", scaleX, scaleY, det))
			} else {
				// 否则这是一个真正的不可逆矩阵
				v.addError(index, "cm", fmt.Sprintf("变换矩阵不可逆（行列式=%.2e）", det))
			}
		} else {
			// 行列式正常，但缩放因子可能仍然很极端
			if scaleX <= 1e-5 || scaleY <= 1e-5 {
				v.addWarning(index, "cm", fmt.Sprintf("极小的缩放因子：X=%.2e, Y=%.2e", scaleX, scaleY))
			}
		}

		if scaleX > 1e6 || scaleY > 1e6 {
			v.addWarning(index, "cm", fmt.Sprintf("极大的缩放因子：X=%.2e, Y=%.2e", scaleX, scaleY))
		}
	}
}

// 验证线宽
func (v *PDFSpecValidator) validateLineWidth(index int, op gopdf.PDFOperator) {
	if wOp, ok := op.(*gopdf.OpSetLineWidth); ok {
		if wOp.Width < 0 {
			v.addError(index, "w", fmt.Sprintf("线宽不能为负数：%.2f", wOp.Width))
		}
		if wOp.Width > 1000 {
			v.addWarning(index, "w", fmt.Sprintf("线宽过大：%.2f", wOp.Width))
		}
	}
}

// 验证线端点样式
func (v *PDFSpecValidator) validateLineCap(index int, op gopdf.PDFOperator) {
	if jOp, ok := op.(*gopdf.OpSetLineCap); ok {
		if jOp.Cap < 0 || jOp.Cap > 2 {
			v.addError(index, "J", fmt.Sprintf("无效的线端点样式：%d（必须是0、1或2）", jOp.Cap))
		}
	}
}

// 验证线连接样式
func (v *PDFSpecValidator) validateLineJoin(index int, op gopdf.PDFOperator) {
	if jOp, ok := op.(*gopdf.OpSetLineJoin); ok {
		if jOp.Join < 0 || jOp.Join > 2 {
			v.addError(index, "j", fmt.Sprintf("无效的线连接样式：%d（必须是0、1或2）", jOp.Join))
		}
	}
}

// 验证斜接限制
func (v *PDFSpecValidator) validateMiterLimit(index int, op gopdf.PDFOperator) {
	if mOp, ok := op.(*gopdf.OpSetMiterLimit); ok {
		if mOp.Limit < 1 {
			v.addError(index, "M", fmt.Sprintf("斜接限制必须>=1：%.2f", mOp.Limit))
		}
	}
}

// 验证虚线模式
func (v *PDFSpecValidator) validateDashPattern(index int, op gopdf.PDFOperator) {
	if dOp, ok := op.(*gopdf.OpSetDash); ok {
		// 检查虚线数组
		for i, val := range dOp.Pattern {
			if val < 0 {
				v.addError(index, "d", fmt.Sprintf("虚线模式值不能为负数：pattern[%d]=%.2f", i, val))
			}
		}

		// 检查偏移量
		if dOp.Offset < 0 {
			v.addWarning(index, "d", fmt.Sprintf("虚线偏移量为负数：%.2f", dOp.Offset))
		}

		// 检查是否所有值都为0
		allZero := true
		for _, val := range dOp.Pattern {
			if val != 0 {
				allZero = false
				break
			}
		}
		if allZero && len(dOp.Pattern) > 0 {
			v.addWarning(index, "d", "虚线模式所有值都为0")
		}
	}
}

// 验证RGB颜色
func (v *PDFSpecValidator) validateRGBColor(index int, op gopdf.PDFOperator) {
	var r, g, b float64

	switch colorOp := op.(type) {
	case *gopdf.OpSetStrokeColorRGB:
		r, g, b = colorOp.R, colorOp.G, colorOp.B
	case *gopdf.OpSetFillColorRGB:
		r, g, b = colorOp.R, colorOp.G, colorOp.B
	default:
		return
	}

	v.validateColorComponent(index, op.Name(), "R", r)
	v.validateColorComponent(index, op.Name(), "G", g)
	v.validateColorComponent(index, op.Name(), "B", b)
}

// 验证灰度颜色
func (v *PDFSpecValidator) validateGrayColor(index int, op gopdf.PDFOperator) {
	var gray float64

	switch colorOp := op.(type) {
	case *gopdf.OpSetStrokeColorGray:
		gray = colorOp.Gray
	case *gopdf.OpSetFillColorGray:
		gray = colorOp.Gray
	default:
		return
	}

	v.validateColorComponent(index, op.Name(), "Gray", gray)
}

// 验证CMYK颜色
func (v *PDFSpecValidator) validateCMYKColor(index int, op gopdf.PDFOperator) {
	var c, m, y, k float64

	switch colorOp := op.(type) {
	case *gopdf.OpSetStrokeColorCMYK:
		c, m, y, k = colorOp.C, colorOp.M, colorOp.Y, colorOp.K
	case *gopdf.OpSetFillColorCMYK:
		c, m, y, k = colorOp.C, colorOp.M, colorOp.Y, colorOp.K
	default:
		return
	}

	v.validateColorComponent(index, op.Name(), "C", c)
	v.validateColorComponent(index, op.Name(), "M", m)
	v.validateColorComponent(index, op.Name(), "Y", y)
	v.validateColorComponent(index, op.Name(), "K", k)
}

// 验证颜色分量
func (v *PDFSpecValidator) validateColorComponent(index int, opName, component string, value float64) {
	if value < 0 || value > 1 {
		v.addError(index, opName, fmt.Sprintf("颜色分量%s超出范围[0,1]：%.3f", component, value))
	}
}

// 验证文本操作符
func (v *PDFSpecValidator) validateTextOperator(index int, op gopdf.PDFOperator) {
	switch textOp := op.(type) {
	case *gopdf.OpSetFont:
		if textOp.FontSize < 0 {
			v.addError(index, "Tf", fmt.Sprintf("字体大小不能为负数：%.2f", textOp.FontSize))
		}
		if textOp.FontSize > 1000 {
			v.addWarning(index, "Tf", fmt.Sprintf("字体大小过大：%.2f", textOp.FontSize))
		}

	case *gopdf.OpSetTextMatrix:
		m := textOp.Matrix
		det := m.A*m.D - m.B*m.C
		if math.Abs(det) < 1e-10 {
			v.addError(index, "Tm", fmt.Sprintf("文本矩阵不可逆（行列式=%.2e）", det))
		}

	case *gopdf.OpSetCharSpacing:
		if math.Abs(textOp.Spacing) > 100 {
			v.addWarning(index, "Tc", fmt.Sprintf("字符间距过大：%.2f", textOp.Spacing))
		}

	case *gopdf.OpSetWordSpacing:
		if math.Abs(textOp.Spacing) > 100 {
			v.addWarning(index, "Tw", fmt.Sprintf("单词间距过大：%.2f", textOp.Spacing))
		}

	case *gopdf.OpSetHorizontalScaling:
		if textOp.Scale <= 0 {
			v.addError(index, "Tz", fmt.Sprintf("水平缩放必须>0：%.2f", textOp.Scale))
		}
		if textOp.Scale < 10 || textOp.Scale > 1000 {
			v.addWarning(index, "Tz", fmt.Sprintf("水平缩放超出常规范围[10,1000]：%.2f", textOp.Scale))
		}

	case *gopdf.OpSetTextRenderMode:
		if textOp.Mode < 0 || textOp.Mode > 7 {
			v.addError(index, "Tr", fmt.Sprintf("无效的文本渲染模式：%d（必须是0-7）", textOp.Mode))
		}
	}
}

// 添加错误
func (v *PDFSpecValidator) addError(index int, opName, message string) {
	if index >= 0 {
		v.errors = append(v.errors, fmt.Sprintf("[操作符 #%d: %s] 错误: %s", index+1, opName, message))
	} else {
		v.errors = append(v.errors, fmt.Sprintf("[%s] 错误: %s", opName, message))
	}
}

// 添加警告
func (v *PDFSpecValidator) addWarning(index int, opName, message string) {
	if index >= 0 {
		v.warnings = append(v.warnings, fmt.Sprintf("[操作符 #%d: %s] 警告: %s", index+1, opName, message))
	} else {
		v.warnings = append(v.warnings, fmt.Sprintf("[%s] 警告: %s", opName, message))
	}
}

// GetErrors 获取所有错误
func (v *PDFSpecValidator) GetErrors() []string {
	return v.errors
}

// GetWarnings 获取所有警告
func (v *PDFSpecValidator) GetWarnings() []string {
	return v.warnings
}

// HasErrors 是否有错误
func (v *PDFSpecValidator) HasErrors() bool {
	return len(v.errors) > 0
}

// HasWarnings 是否有警告
func (v *PDFSpecValidator) HasWarnings() bool {
	return len(v.warnings) > 0
}

// PrintReport 打印验证报告
func (v *PDFSpecValidator) PrintReport() {
	fmt.Println("\n" + repeatStr("=", 80))
	fmt.Println("PDF 规范验证报告")
	fmt.Println(repeatStr("=", 80))

	if !v.HasErrors() && !v.HasWarnings() {
		fmt.Println("\n✓ 所有操作符都符合PDF规范")
		return
	}

	if v.HasErrors() {
		fmt.Printf("\n错误 (%d):\n", len(v.errors))
		fmt.Println(repeatStr("-", 80))
		for _, err := range v.errors {
			fmt.Printf("  ✗ %s\n", err)
		}
	}

	if v.HasWarnings() {
		fmt.Printf("\n警告 (%d):\n", len(v.warnings))
		fmt.Println(repeatStr("-", 80))
		for _, warn := range v.warnings {
			fmt.Printf("  ⚠ %s\n", warn)
		}
	}

	fmt.Println("\n" + repeatStr("=", 80))
}

// 辅助函数：重复字符串
func repeatStr(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}
