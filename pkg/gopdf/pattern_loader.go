package gopdf

import (
	"fmt"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// loadPattern 加载 Pattern 资源
func loadPattern(ctx *model.Context, patternName string, patternObj types.Object, resources *Resources) error {
	// 解引用
	if indRef, ok := patternObj.(types.IndirectRef); ok {
		derefObj, err := ctx.Dereference(indRef)
		if err != nil {
			return err
		}
		patternObj = derefObj
	}

	// Pattern 可以是字典或流字典
	var patternDict types.Dict
	var streamDict *types.StreamDict

	switch obj := patternObj.(type) {
	case types.Dict:
		patternDict = obj
	case types.StreamDict:
		patternDict = obj.Dict
		streamDict = &obj
	default:
		return fmt.Errorf("pattern is not a dictionary or stream")
	}

	pattern := NewPattern()

	// 获取 PatternType
	if patternType, found := patternDict.Find("PatternType"); found {
		if pt, ok := patternType.(types.Integer); ok {
			pattern.PatternType = int(pt)
		}
	}

	// 如果是 Tiling Pattern (Type 1)
	if pattern.IsTilingPattern() {
		// 获取 PaintType
		if paintType, found := patternDict.Find("PaintType"); found {
			if pt, ok := paintType.(types.Integer); ok {
				pattern.PaintType = int(pt)
			}
		}

		// 获取 TilingType
		if tilingType, found := patternDict.Find("TilingType"); found {
			if tt, ok := tilingType.(types.Integer); ok {
				pattern.TilingType = int(tt)
			}
		}

		// 获取 BBox
		if bbox, found := patternDict.Find("BBox"); found {
			if arr, ok := bbox.(types.Array); ok {
				pattern.BBox = make([]float64, len(arr))
				for i, v := range arr {
					if num, ok := v.(types.Float); ok {
						pattern.BBox[i] = float64(num)
					} else if num, ok := v.(types.Integer); ok {
						pattern.BBox[i] = float64(num)
					}
				}
			}
		}

		// 获取 XStep
		if xstep, found := patternDict.Find("XStep"); found {
			if num, ok := xstep.(types.Float); ok {
				pattern.XStep = float64(num)
			} else if num, ok := xstep.(types.Integer); ok {
				pattern.XStep = float64(num)
			}
		}

		// 获取 YStep
		if ystep, found := patternDict.Find("YStep"); found {
			if num, ok := ystep.(types.Float); ok {
				pattern.YStep = float64(num)
			} else if num, ok := ystep.(types.Integer); ok {
				pattern.YStep = float64(num)
			}
		}

		// 获取 Matrix
		if matrix, found := patternDict.Find("Matrix"); found {
			if arr, ok := matrix.(types.Array); ok && len(arr) == 6 {
				pattern.Matrix = &Matrix{}
				if v, ok := arr[0].(types.Float); ok {
					pattern.Matrix.A = float64(v)
				} else if v, ok := arr[0].(types.Integer); ok {
					pattern.Matrix.A = float64(v)
				}
				if v, ok := arr[1].(types.Float); ok {
					pattern.Matrix.B = float64(v)
				} else if v, ok := arr[1].(types.Integer); ok {
					pattern.Matrix.B = float64(v)
				}
				if v, ok := arr[2].(types.Float); ok {
					pattern.Matrix.C = float64(v)
				} else if v, ok := arr[2].(types.Integer); ok {
					pattern.Matrix.C = float64(v)
				}
				if v, ok := arr[3].(types.Float); ok {
					pattern.Matrix.D = float64(v)
				} else if v, ok := arr[3].(types.Integer); ok {
					pattern.Matrix.D = float64(v)
				}
				if v, ok := arr[4].(types.Float); ok {
					pattern.Matrix.E = float64(v)
				} else if v, ok := arr[4].(types.Integer); ok {
					pattern.Matrix.E = float64(v)
				}
				if v, ok := arr[5].(types.Float); ok {
					pattern.Matrix.F = float64(v)
				} else if v, ok := arr[5].(types.Integer); ok {
					pattern.Matrix.F = float64(v)
				}
			}
		}

		// 获取 Resources
		if resourcesObj, found := patternDict.Find("Resources"); found {
			patternResources := NewResources()
			if err := loadResources(ctx, resourcesObj, patternResources); err != nil {
				debugPrintf("Warning: failed to load pattern resources: %v\n", err)
			} else {
				pattern.Resources = patternResources
			}
		}

		// 获取内容流
		if streamDict != nil {
			// 解码流
			decoded, _, err := ctx.DereferenceStreamDict(*streamDict)
			if err == nil && decoded != nil {
				pattern.Stream = decoded.Content
			}
		}
	}

	// 存储到资源
	resources.SetPattern(patternName, pattern)
	debugPrintf("✓ Loaded pattern %s (type %d)\n", patternName, pattern.PatternType)

	return nil
}
