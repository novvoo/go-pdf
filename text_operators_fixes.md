# PDF文本操作符修复说明

## 问题总结

根据你的描述，以下是PDF文本操作符中的潜在问题：

### 1. **Td操作符 - 矩阵乘法顺序错误**
**问题**: `translation.Multiply(textLineMatrix)` 顺序不符合PDF规范
**影响**: 当文本矩阵包含缩放时，平移会被错误缩放
**修复**: 应改为 `textLineMatrix.Multiply(translation)`

### 2. **TJ操作符 - 未正确应用调整值**
**问题**: 
- 数字调整值（kerning）的符号处理错误
- 未累计总位移来更新文本矩阵
- 未应用字符间距和单词间距到TJ数组

**影响**: 复杂文本中出现间距错误和位置偏移

### 3. **' 操作符 - 行移动后未更新矩阵**
**问题**: 显示文本后可能未正确更新TextMatrix
**影响**: 后续文本位置可能不正确

### 4. **" 操作符 - 与TJ结合时调整被忽略**
**问题**: 设置的间距参数在TJ操作中未被正确应用
**影响**: 带间距的文本显示不正确

## 修复方案

### 修复1: Td操作符矩阵乘法顺序

**位置**: `pkg/gopdf/text_operators.go` 第115-130行

**原代码**:
```go
func (op *OpMoveTextPosition) Execute(ctx *RenderContext) error {
	// Tm = Tlm = [1 0 0 1 tx ty] × Tlm
	translation := NewTranslationMatrix(op.Tx, op.Ty)
	ctx.TextState.TextLineMatrix = translation.Multiply(ctx.TextState.TextLineMatrix)
	ctx.TextState.TextMatrix = ctx.TextState.TextLineMatrix.Clone()
	...
}
```

**修复后**:
```go
func (op *OpMoveTextPosition) Execute(ctx *RenderContext) error {
	// 根据PDF规范：Tm = Tlm = Tlm × [1 0 0 1 tx ty]
	// 正确的矩阵乘法顺序：先应用当前矩阵，再应用平移
	translation := NewTranslationMatrix(op.Tx, op.Ty)
	ctx.TextState.TextLineMatrix = ctx.TextState.TextLineMatrix.Multiply(translation)
	ctx.TextState.TextMatrix = ctx.TextState.TextLineMatrix.Clone()
	...
}
```

### 修复2: TJ操作符正确应用调整

**位置**: `pkg/gopdf/text_operators.go` renderText函数中的TJ处理部分

**关键修复点**:

1. **kerning调整符号修正**:
```go
// 原代码（错误）:
kerningAdjustment := v * fontSize / 1000.0
x -= kerningAdjustment

// 修复后（正确）:
// PDF规范：负值向右移动，正值向左移动
kerningAdjustment := -v * fontSize / 1000.0
x += kerningAdjustment
```

2. **累计总位移**:
```go
// 添加变量跟踪总位移
totalTextWidth := 0.0

// 在每个文本片段和调整后累加
totalTextWidth += textWidth  // 文本宽度
totalTextWidth += kerningAdjustment  // 调整值

// 最后更新文本矩阵
textDisplacement = totalTextWidth * horizontalScale
```

3. **应用字符间距和单词间距**:
```go
// 在TJ数组的每个文本片段中应用间距
if textState.CharSpacing != 0 {
	charAdj := textState.CharSpacing * runeCount
	textWidth += charAdj
}

// 应用单词间距
spaceCount := 0
for _, ch := range decodedText {
	if ch == ' ' {
		spaceCount++
	}
}
if spaceCount > 0 && textState.WordSpacing != 0 {
	wordAdj := textState.WordSpacing * float64(spaceCount)
	textWidth += wordAdj
}
```

### 修复3: ' 操作符确保更新

**位置**: `pkg/gopdf/text_operators.go` 第240-250行

**原代码**:
```go
func (op *OpShowTextNextLine) Execute(ctx *RenderContext) error {
	// 等同于 T* Tj
	if err := (&OpMoveToNextLine{}).Execute(ctx); err != nil {
		return err
	}
	return (&OpShowText{Text: op.Text}).Execute(ctx)
}
```

**修复后**:
```go
func (op *OpShowTextNextLine) Execute(ctx *RenderContext) error {
	// 等同于 T* Tj
	// 先移动到下一行
	if err := (&OpMoveToNextLine{}).Execute(ctx); err != nil {
		return err
	}
	// 然后显示文本（会自动更新TextMatrix）
	debugPrintf("['] Moving to next line and showing text\n")
	return (&OpShowText{Text: op.Text}).Execute(ctx)
}
```

### 修复4: " 操作符间距应用

**位置**: `pkg/gopdf/text_operators.go` 第252-262行

**原代码**:
```go
func (op *OpShowTextWithSpacing) Execute(ctx *RenderContext) error {
	// 等同于 Tw Tc '
	ctx.TextState.WordSpacing = op.WordSpacing
	ctx.TextState.CharSpacing = op.CharSpacing
	return (&OpShowTextNextLine{Text: op.Text}).Execute(ctx)
}
```

**修复后**:
```go
func (op *OpShowTextWithSpacing) Execute(ctx *RenderContext) error {
	// 等同于 Tw Tc T* Tj
	// 先设置间距参数
	debugPrintf("[\"] Setting WordSpacing=%.4f CharSpacing=%.4f\n", op.WordSpacing, op.CharSpacing)
	ctx.TextState.WordSpacing = op.WordSpacing
	ctx.TextState.CharSpacing = op.CharSpacing
	// 然后移动到下一行并显示文本
	return (&OpShowTextNextLine{Text: op.Text}).Execute(ctx)
}
```

## 测试建议

修复后应测试以下场景：

1. **缩放文本的平移**: 创建包含缩放矩阵的文本，然后使用Td移动
2. **TJ数组的kerning**: 测试包含正负调整值的TJ数组
3. **字符和单词间距**: 测试Tc、Tw与TJ的组合
4. **多行文本**: 测试'和"操作符的行为
5. **复杂文本布局**: 测试混合使用各种操作符的PDF文档

## 实施步骤

1. 备份当前的 `text_operators.go` 文件
2. 按照上述修复方案逐一修改代码
3. 运行现有测试确保没有回归
4. 使用包含复杂文本布局的PDF进行测试
5. 验证文本位置和间距是否正确
