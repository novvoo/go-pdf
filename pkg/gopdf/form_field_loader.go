package gopdf

import (
	"fmt"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// ExtractFormFields 从文档目录中提取表单字段
func ExtractFormFields(ctx *model.Context) ([]*FormField, error) {
	var formFields []*FormField

	// 获取文档目录
	catalog := ctx.RootDict
	if catalog == nil {
		return formFields, nil // 没有目录
	}

	// 查找 AcroForm 条目
	acroFormObj, found := catalog.Find("AcroForm")
	if !found {
		return formFields, nil // 没有表单
	}

	// 解引用
	if indRef, ok := acroFormObj.(types.IndirectRef); ok {
		derefObj, err := ctx.Dereference(indRef)
		if err != nil {
			return nil, fmt.Errorf("failed to dereference AcroForm: %w", err)
		}
		acroFormObj = derefObj
	}

	// 解析 AcroForm 字典
	acroFormDict, ok := acroFormObj.(types.Dict)
	if !ok {
		return nil, fmt.Errorf("AcroForm is not a dictionary")
	}

	// 查找 Fields 数组
	fieldsObj, found := acroFormDict.Find("Fields")
	if !found {
		return formFields, nil // 没有字段
	}

	// 解引用
	if indRef, ok := fieldsObj.(types.IndirectRef); ok {
		derefObj, err := ctx.Dereference(indRef)
		if err != nil {
			return nil, fmt.Errorf("failed to dereference Fields: %w", err)
		}
		fieldsObj = derefObj
	}

	// 解析字段数组
	fieldsArray, ok := fieldsObj.(types.Array)
	if !ok {
		return nil, fmt.Errorf("Fields is not an array")
	}

	// 遍历每个字段
	for _, fieldObj := range fieldsArray {
		// 解引用字段对象
		if indRef, ok := fieldObj.(types.IndirectRef); ok {
			derefObj, err := ctx.Dereference(indRef)
			if err != nil {
				debugPrintf("Warning: failed to dereference form field: %v\n", err)
				continue
			}
			fieldObj = derefObj
		}

		// 解析字段字典
		fieldDict, ok := fieldObj.(types.Dict)
		if !ok {
			debugPrintf("Warning: form field is not a dictionary\n")
			continue
		}

		// 解析字段
		field, err := parseFormField(ctx, fieldDict)
		if err != nil {
			debugPrintf("Warning: failed to parse form field: %v\n", err)
			continue
		}

		formFields = append(formFields, field)
	}

	return formFields, nil
}

// parseFormField 解析单个表单字段字典
func parseFormField(ctx *model.Context, fieldDict types.Dict) (*FormField, error) {
	field := NewFormField("")

	// 获取字段类型
	if ft, found := fieldDict.Find("FT"); found {
		if name, ok := ft.(types.Name); ok {
			field.FieldType = name.String()
		}
	}

	// 获取字段名称
	if t, found := fieldDict.Find("T"); found {
		if str, ok := t.(types.StringLiteral); ok {
			field.FieldName = str.String()
		}
	}

	// 获取字段值
	if v, found := fieldDict.Find("V"); found {
		switch val := v.(type) {
		case types.StringLiteral:
			field.Value = val.String()
		case types.Name:
			field.Value = val.String()
		}
	}

	// 获取默认值
	if dv, found := fieldDict.Find("DV"); found {
		switch val := dv.(type) {
		case types.StringLiteral:
			field.DefaultValue = val.String()
		case types.Name:
			field.DefaultValue = val.String()
		}
	}

	// 获取矩形（从 Widget 注释）
	// 表单字段通常有关联的 Widget 注释
	if rect, found := fieldDict.Find("Rect"); found {
		if arr, ok := rect.(types.Array); ok && len(arr) >= 4 {
			for i := 0; i < 4 && i < len(arr); i++ {
				if num, ok := arr[i].(types.Float); ok {
					field.Rect[i] = float64(num)
				} else if num, ok := arr[i].(types.Integer); ok {
					field.Rect[i] = float64(num)
				}
			}
		}
	}

	// 获取标志
	if ff, found := fieldDict.Find("Ff"); found {
		if num, ok := ff.(types.Integer); ok {
			field.Flags = int(num)
		}
	}

	// 获取外观流（AP）
	if ap, found := fieldDict.Find("AP"); found {
		// 解引用
		if indRef, ok := ap.(types.IndirectRef); ok {
			derefObj, err := ctx.Dereference(indRef)
			if err == nil {
				ap = derefObj
			}
		}

		if apDict, ok := ap.(types.Dict); ok {
			field.Appearance = make(map[string]interface{})
			for key, value := range apDict {
				field.Appearance[key] = value
			}
		}
	}

	// 获取选项（用于选择字段）
	if opt, found := fieldDict.Find("Opt"); found {
		if arr, ok := opt.(types.Array); ok {
			field.Options = make([]string, 0, len(arr))
			for _, item := range arr {
				if str, ok := item.(types.StringLiteral); ok {
					field.Options = append(field.Options, str.String())
				}
			}
		}
	}

	return field, nil
}
