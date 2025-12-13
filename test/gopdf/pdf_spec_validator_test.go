package gopdf

import (
	"fmt"
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
	"github.com/pdfcpu/pdfcpu/pkg/api"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// TestPDFSpecValidation 测试PDF规范验证
func TestPDFSpecValidation(t *testing.T) {
	pdfPath := "../test_vector.pdf"

	// 读取PDF文件
	ctx, err := api.ReadContextFile(pdfPath)
	if err != nil {
		t.Fatalf("无法读取PDF文件: %v", err)
	}

	// 验证第一页
	operators, err := extractPageOperators(ctx, 1)
	if err != nil {
		t.Fatalf("无法提取页面操作符: %v", err)
	}

	t.Logf("提取到 %d 个操作符", len(operators))

	// 创建验证器并验证
	validator := NewPDFSpecValidator()
	validator.ValidateOperators(operators)

	// 打印报告
	validator.PrintReport()

	// 如果有错误，测试失败
	if validator.HasErrors() {
		t.Errorf("发现 %d 个PDF规范错误", len(validator.GetErrors()))
		for _, err := range validator.GetErrors() {
			t.Logf("  错误: %s", err)
		}
	}

	// 警告不会导致测试失败，但会记录
	if validator.HasWarnings() {
		t.Logf("发现 %d 个PDF规范警告", len(validator.GetWarnings()))
		for _, warn := range validator.GetWarnings() {
			t.Logf("  警告: %s", warn)
		}
	}

	// 打印操作符统计
	printOperatorStats(t, operators)
}

// TestGraphicsStateValidation 测试图形状态操作符验证
func TestGraphicsStateValidation(t *testing.T) {
	tests := []struct {
		name      string
		operators []gopdf.PDFOperator
		wantError bool
	}{
		{
			name: "正常的q/Q配对",
			operators: []gopdf.PDFOperator{
				&gopdf.OpSaveState{},
				&gopdf.OpRestoreState{},
			},
			wantError: false,
		},
		{
			name: "Q多于q",
			operators: []gopdf.PDFOperator{
				&gopdf.OpRestoreState{},
			},
			wantError: true,
		},
		{
			name: "q未配对",
			operators: []gopdf.PDFOperator{
				&gopdf.OpSaveState{},
			},
			wantError: true,
		},
		{
			name: "嵌套的q/Q",
			operators: []gopdf.PDFOperator{
				&gopdf.OpSaveState{},
				&gopdf.OpSaveState{},
				&gopdf.OpRestoreState{},
				&gopdf.OpRestoreState{},
			},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewPDFSpecValidator()
			validator.ValidateOperators(tt.operators)

			hasError := validator.HasErrors()
			if hasError != tt.wantError {
				t.Errorf("期望错误=%v, 实际错误=%v", tt.wantError, hasError)
				if hasError {
					for _, err := range validator.GetErrors() {
						t.Logf("  %s", err)
					}
				}
			}
		})
	}
}

// TestTextObjectValidation 测试文本对象验证
func TestTextObjectValidation(t *testing.T) {
	tests := []struct {
		name      string
		operators []gopdf.PDFOperator
		wantError bool
	}{
		{
			name: "正常的BT/ET配对",
			operators: []gopdf.PDFOperator{
				&gopdf.OpBeginText{},
				&gopdf.OpSetFont{FontName: "F1", FontSize: 12},
				&gopdf.OpShowText{Text: "Hello"},
				&gopdf.OpEndText{},
			},
			wantError: false,
		},
		{
			name: "嵌套的BT",
			operators: []gopdf.PDFOperator{
				&gopdf.OpBeginText{},
				&gopdf.OpBeginText{},
				&gopdf.OpEndText{},
			},
			wantError: true,
		},
		{
			name: "文本对象外的Tj",
			operators: []gopdf.PDFOperator{
				&gopdf.OpShowText{Text: "Hello"},
			},
			wantError: true,
		},
		{
			name: "未关闭的文本对象",
			operators: []gopdf.PDFOperator{
				&gopdf.OpBeginText{},
				&gopdf.OpShowText{Text: "Hello"},
			},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewPDFSpecValidator()
			validator.ValidateOperators(tt.operators)

			hasError := validator.HasErrors()
			if hasError != tt.wantError {
				t.Errorf("期望错误=%v, 实际错误=%v", tt.wantError, hasError)
				if hasError {
					for _, err := range validator.GetErrors() {
						t.Logf("  %s", err)
					}
				}
			}
		})
	}
}

// TestColorValidation 测试颜色操作符验证
func TestColorValidation(t *testing.T) {
	tests := []struct {
		name      string
		operator  gopdf.PDFOperator
		wantError bool
	}{
		{
			name:      "有效的RGB颜色",
			operator:  &gopdf.OpSetFillColorRGB{R: 0.5, G: 0.5, B: 0.5},
			wantError: false,
		},
		{
			name:      "RGB颜色超出范围",
			operator:  &gopdf.OpSetFillColorRGB{R: 1.5, G: 0.5, B: 0.5},
			wantError: true,
		},
		{
			name:      "RGB颜色为负数",
			operator:  &gopdf.OpSetStrokeColorRGB{R: -0.1, G: 0.5, B: 0.5},
			wantError: true,
		},
		{
			name:      "有效的灰度颜色",
			operator:  &gopdf.OpSetFillColorGray{Gray: 0.5},
			wantError: false,
		},
		{
			name:      "灰度颜色超出范围",
			operator:  &gopdf.OpSetFillColorGray{Gray: 1.5},
			wantError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewPDFSpecValidator()
			validator.ValidateOperators([]gopdf.PDFOperator{tt.operator})

			hasError := validator.HasErrors()
			if hasError != tt.wantError {
				t.Errorf("期望错误=%v, 实际错误=%v", tt.wantError, hasError)
				if hasError {
					for _, err := range validator.GetErrors() {
						t.Logf("  %s", err)
					}
				}
			}
		})
	}
}

// TestMatrixValidation 测试矩阵验证
func TestMatrixValidation(t *testing.T) {
	tests := []struct {
		name      string
		matrix    *gopdf.Matrix
		wantError bool
	}{
		{
			name:      "单位矩阵",
			matrix:    &gopdf.Matrix{A: 1, B: 0, C: 0, D: 1, E: 0, F: 0},
			wantError: false,
		},
		{
			name:      "缩放矩阵",
			matrix:    &gopdf.Matrix{A: 2, B: 0, C: 0, D: 2, E: 0, F: 0},
			wantError: false,
		},
		{
			name:      "不可逆矩阵（行列式为0）",
			matrix:    &gopdf.Matrix{A: 1, B: 2, C: 2, D: 4, E: 0, F: 0},
			wantError: true,
		},
		{
			name:      "极小缩放",
			matrix:    &gopdf.Matrix{A: 0.000001, B: 0, C: 0, D: 0.000001, E: 0, F: 0},
			wantError: false, // 警告，不是错误
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			validator := NewPDFSpecValidator()
			validator.ValidateOperators([]gopdf.PDFOperator{
				&gopdf.OpConcatMatrix{Matrix: tt.matrix},
			})

			hasError := validator.HasErrors()
			if hasError != tt.wantError {
				t.Errorf("期望错误=%v, 实际错误=%v", tt.wantError, hasError)
				if hasError {
					for _, err := range validator.GetErrors() {
						t.Logf("  %s", err)
					}
				}
			}
		})
	}
}

// extractPageOperators 提取页面的所有操作符
func extractPageOperators(ctx *model.Context, pageNum int) ([]gopdf.PDFOperator, error) {
	pageDict, _, _, err := ctx.PageDict(pageNum, false)
	if err != nil {
		return nil, fmt.Errorf("无法获取页面字典: %w", err)
	}

	contents, found := pageDict.Find("Contents")
	if !found {
		return nil, fmt.Errorf("页面没有内容流")
	}

	contentStreams, err := extractContentStreams(ctx, contents)
	if err != nil {
		return nil, fmt.Errorf("无法提取内容流: %w", err)
	}

	// 合并所有内容流
	var allContent []byte
	for _, stream := range contentStreams {
		allContent = append(allContent, stream...)
		allContent = append(allContent, '\n')
	}

	// 解析操作符
	operators, err := gopdf.ParseContentStream(allContent)
	if err != nil {
		return nil, fmt.Errorf("无法解析内容流: %w", err)
	}

	return operators, nil
}

// extractContentStreams 提取页面的所有内容流
func extractContentStreams(ctx *model.Context, contents types.Object) ([][]byte, error) {
	var streams [][]byte

	switch obj := contents.(type) {
	case types.IndirectRef:
		derefObj, err := ctx.Dereference(obj)
		if err != nil {
			return nil, fmt.Errorf("无法解引用内容: %w", err)
		}
		return extractContentStreams(ctx, derefObj)

	case types.StreamDict:
		if len(obj.Content) == 0 && len(obj.Raw) > 0 {
			err := obj.Decode()
			if err != nil {
				return nil, fmt.Errorf("无法解码流: %w", err)
			}
		}
		if len(obj.Content) > 0 {
			streams = append(streams, obj.Content)
		}

	case types.Array:
		for _, item := range obj {
			itemStreams, err := extractContentStreams(ctx, item)
			if err == nil {
				streams = append(streams, itemStreams...)
			}
		}
	}

	return streams, nil
}

// printOperatorStats 打印操作符统计信息
func printOperatorStats(t *testing.T, operators []gopdf.PDFOperator) {
	stats := make(map[string]int)

	for _, op := range operators {
		opName := op.Name()
		if opName != "IGNORE" {
			stats[opName]++
		}
	}

	t.Log("\n操作符统计:")
	for opName, count := range stats {
		t.Logf("  %s: %d", opName, count)
	}
}
