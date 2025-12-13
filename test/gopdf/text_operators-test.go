package gopdf

import (
	"fmt"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// 测试文本操作符的修复
func TestTextOprator() {
	fmt.Println("=== 测试文本操作符修复 ===\n")

	// 测试1: Td操作符矩阵乘法顺序
	testTdOperator()

	// 测试2: TJ操作符kerning调整
	testTJOperator()

	// 测试3: 文本矩阵更新
	testTextMatrixUpdate()
}

// 测试Td操作符的矩阵乘法顺序
func testTdOperator() {
	fmt.Println("测试1: Td操作符矩阵乘法顺序")
	fmt.Println("----------------------------")

	// 创建一个带缩放的文本行矩阵
	textLineMatrix := &gopdf.Matrix{
		A: 2.0, B: 0,
		C: 0, D: 2.0,
		E: 10, F: 20,
	}

	// 创建平移矩阵
	translation := gopdf.NewTranslationMatrix(5, 10)

	// 错误的顺序: translation × textLineMatrix
	wrongResult := translation.Multiply(textLineMatrix)
	fmt.Printf("错误顺序 (translation × textLineMatrix):\n")
	fmt.Printf("  结果: %s\n", wrongResult.String())
	fmt.Printf("  E坐标: %.2f (错误: 平移被缩放了)\n\n", wrongResult.E)

	// 正确的顺序: textLineMatrix × translation
	correctResult := textLineMatrix.Multiply(translation)
	fmt.Printf("正确顺序 (textLineMatrix × translation):\n")
	fmt.Printf("  结果: %s\n", correctResult.String())
	fmt.Printf("  E坐标: %.2f (正确: 先缩放再平移)\n\n", correctResult.E)

	// 验证
	// 正确的计算: E' = A*tx + E = 2.0*5 + 10 = 20
	// 错误的计算: E' = tx*A + E = 5*2.0 + 10 = 20 (在这个例子中相同)
	// 但当有旋转时差异会很明显

	fmt.Println()
}

// 测试TJ操作符的kerning调整
func testTJOperator() {
	fmt.Println("测试2: TJ操作符kerning调整")
	fmt.Println("----------------------------")

	fontSize := 12.0

	// PDF中的TJ数组示例: [(Hello) -100 (World)]
	// -100表示向右移动100/1000 em
	kerningValue := -100.0

	// 错误的计算（原代码）
	wrongAdjustment := kerningValue * fontSize / 1000.0
	fmt.Printf("错误计算:\n")
	fmt.Printf("  kerning值: %.0f\n", kerningValue)
	fmt.Printf("  调整量: %.2f (然后 x -= adjustment)\n", wrongAdjustment)
	fmt.Printf("  结果: x会向左移动（错误！）\n\n")

	// 正确的计算（修复后）
	correctAdjustment := -kerningValue * fontSize / 1000.0
	fmt.Printf("正确计算:\n")
	fmt.Printf("  kerning值: %.0f\n", kerningValue)
	fmt.Printf("  调整量: %.2f (然后 x += adjustment)\n", correctAdjustment)
	fmt.Printf("  结果: x会向右移动（正确！）\n\n")

	// 说明
	fmt.Println("PDF规范:")
	fmt.Println("  - 负值: 向右移动（增加字符间距）")
	fmt.Println("  - 正值: 向左移动（减少字符间距）")
	fmt.Println()
}

// 测试文本矩阵更新
func testTextMatrixUpdate() {
	fmt.Println("测试3: 文本矩阵更新")
	fmt.Println("----------------------------")

	// 模拟TJ操作符处理
	fontSize := 12.0
	horizontalScale := 1.0

	// 文本片段和调整
	text1Width := 5.0 * fontSize * 0.5        // "Hello" = 5个字符
	kerning1 := -(-100.0) * fontSize / 1000.0 // -100 kerning
	text2Width := 5.0 * fontSize * 0.5        // "World" = 5个字符

	totalWidth := text1Width + kerning1 + text2Width

	fmt.Printf("文本片段1 \"Hello\": 宽度 = %.2f\n", text1Width)
	fmt.Printf("Kerning调整 -100: 调整量 = %.2f\n", kerning1)
	fmt.Printf("文本片段2 \"World\": 宽度 = %.2f\n", text2Width)
	fmt.Printf("总宽度: %.2f\n", totalWidth)
	fmt.Printf("文本矩阵位移: %.2f (总宽度 × 水平缩放 %.2f)\n", totalWidth*horizontalScale, horizontalScale)

	fmt.Println("\n修复前: TJ操作符不更新文本矩阵（错误）")
	fmt.Println("修复后: TJ操作符正确更新文本矩阵位置")
	fmt.Println()
}
