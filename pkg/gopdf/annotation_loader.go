package gopdf

import (
	"fmt"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// ExtractAnnotations 从页面字典中提取注释
func ExtractAnnotations(ctx *model.Context, pageDict types.Dict) ([]*Annotation, error) {
	var annotations []*Annotation

	// 查找 Annots 数组
	annotsObj, found := pageDict.Find("Annots")
	if !found {
		return annotations, nil // 没有注释
	}

	// 解引用
	if indRef, ok := annotsObj.(types.IndirectRef); ok {
		derefObj, err := ctx.Dereference(indRef)
		if err != nil {
			return nil, fmt.Errorf("failed to dereference Annots: %w", err)
		}
		annotsObj = derefObj
	}

	// 解析注释数组
	annotsArray, ok := annotsObj.(types.Array)
	if !ok {
		return nil, fmt.Errorf("annots is not an array")
	}

	// 遍历每个注释
	for _, annotObj := range annotsArray {
		// 解引用注释对象
		if indRef, ok := annotObj.(types.IndirectRef); ok {
			derefObj, err := ctx.Dereference(indRef)
			if err != nil {
				debugPrintf("Warning: failed to dereference annotation: %v\n", err)
				continue
			}
			annotObj = derefObj
		}

		// 解析注释字典
		annotDict, ok := annotObj.(types.Dict)
		if !ok {
			debugPrintf("Warning: annotation is not a dictionary\n")
			continue
		}

		// 解析注释
		annot, err := parseAnnotation(ctx, annotDict)
		if err != nil {
			debugPrintf("Warning: failed to parse annotation: %v\n", err)
			continue
		}

		annotations = append(annotations, annot)
	}

	return annotations, nil
}

// parseAnnotation 解析单个注释字典
func parseAnnotation(ctx *model.Context, annotDict types.Dict) (*Annotation, error) {
	annot := NewAnnotation("")

	// 获取子类型
	if subtype, found := annotDict.Find("Subtype"); found {
		if name, ok := subtype.(types.Name); ok {
			annot.Subtype = name.String()
		}
	}

	// 获取矩形
	if rect, found := annotDict.Find("Rect"); found {
		if arr, ok := rect.(types.Array); ok && len(arr) >= 4 {
			for i := 0; i < 4 && i < len(arr); i++ {
				if num, ok := arr[i].(types.Float); ok {
					annot.Rect[i] = float64(num)
				} else if num, ok := arr[i].(types.Integer); ok {
					annot.Rect[i] = float64(num)
				}
			}
		}
	}

	// 获取内容
	if contents, found := annotDict.Find("Contents"); found {
		if str, ok := contents.(types.StringLiteral); ok {
			annot.Contents = str.String()
		}
	}

	// 获取颜色
	if color, found := annotDict.Find("C"); found {
		if arr, ok := color.(types.Array); ok {
			annot.Color = make([]float64, len(arr))
			for i, v := range arr {
				if num, ok := v.(types.Float); ok {
					annot.Color[i] = float64(num)
				} else if num, ok := v.(types.Integer); ok {
					annot.Color[i] = float64(num)
				}
			}
		}
	}

	// 获取标志
	if flags, found := annotDict.Find("F"); found {
		if num, ok := flags.(types.Integer); ok {
			annot.Flags = int(num)
		}
	}

	// 获取外观流（AP）
	if ap, found := annotDict.Find("AP"); found {
		// 解引用
		if indRef, ok := ap.(types.IndirectRef); ok {
			derefObj, err := ctx.Dereference(indRef)
			if err == nil {
				ap = derefObj
			}
		}

		if apDict, ok := ap.(types.Dict); ok {
			annot.Appearance = make(map[string]interface{})
			for key, value := range apDict {
				annot.Appearance[key] = value
			}
		}
	}

	// 获取四边形点（用于高亮等）
	if quadPoints, found := annotDict.Find("QuadPoints"); found {
		if arr, ok := quadPoints.(types.Array); ok {
			annot.QuadPoints = make([]float64, len(arr))
			for i, v := range arr {
				if num, ok := v.(types.Float); ok {
					annot.QuadPoints[i] = float64(num)
				} else if num, ok := v.(types.Integer); ok {
					annot.QuadPoints[i] = float64(num)
				}
			}
		}
	}

	// 获取名称（用于某些注释类型）
	if name, found := annotDict.Find("Name"); found {
		if nameObj, ok := name.(types.Name); ok {
			annot.Name = nameObj.String()
		}
	}

	return annot, nil
}
