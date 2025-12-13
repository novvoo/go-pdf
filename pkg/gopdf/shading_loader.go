package gopdf

import (
	"fmt"

	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/model"
	"github.com/pdfcpu/pdfcpu/pkg/pdfcpu/types"
)

// loadShading 加载 Shading 资源
func loadShading(ctx *model.Context, shadingName string, shadingObj types.Object, resources *Resources) error {
	// 解引用
	if indRef, ok := shadingObj.(types.IndirectRef); ok {
		derefObj, err := ctx.Dereference(indRef)
		if err != nil {
			return err
		}
		shadingObj = derefObj
	}

	shadingDict, ok := shadingObj.(types.Dict)
	if !ok {
		return fmt.Errorf("shading is not a dictionary")
	}

	shading := NewShading()

	// 获取 ShadingType
	if shadingType, found := shadingDict.Find("ShadingType"); found {
		if st, ok := shadingType.(types.Integer); ok {
			shading.ShadingType = int(st)
		}
	}

	// 获取 ColorSpace
	if colorSpace, found := shadingDict.Find("ColorSpace"); found {
		if cs, ok := colorSpace.(types.Name); ok {
			shading.ColorSpace = cs.String()
		}
	}

	// 获取 Coords
	if coords, found := shadingDict.Find("Coords"); found {
		if arr, ok := coords.(types.Array); ok {
			shading.Coords = make([]float64, len(arr))
			for i, v := range arr {
				if num, ok := v.(types.Float); ok {
					shading.Coords[i] = float64(num)
				} else if num, ok := v.(types.Integer); ok {
					shading.Coords[i] = float64(num)
				}
			}
		}
	}

	// 获取 Extend
	if extend, found := shadingDict.Find("Extend"); found {
		if arr, ok := extend.(types.Array); ok && len(arr) >= 2 {
			shading.Extend = make([]bool, 2)
			if b, ok := arr[0].(types.Boolean); ok {
				shading.Extend[0] = bool(b)
			}
			if b, ok := arr[1].(types.Boolean); ok {
				shading.Extend[1] = bool(b)
			}
		}
	}

	// 获取 Function
	if function, found := shadingDict.Find("Function"); found {
		shadingFunc, err := parseShadingFunction(ctx, function)
		if err == nil {
			shading.Function = shadingFunc
		} else {
			debugPrintf("Warning: failed to parse shading function: %v\n", err)
		}
	}

	// 存储到资源
	resources.SetShading(shadingName, shading)
	debugPrintf("✓ Loaded shading %s (type %d)\n", shadingName, shading.ShadingType)

	return nil
}

// parseShadingFunction 解析 Shading Function
func parseShadingFunction(ctx *model.Context, functionObj types.Object) (*ShadingFunction, error) {
	// 解引用
	if indRef, ok := functionObj.(types.IndirectRef); ok {
		derefObj, err := ctx.Dereference(indRef)
		if err != nil {
			return nil, err
		}
		functionObj = derefObj
	}

	funcDict, ok := functionObj.(types.Dict)
	if !ok {
		return nil, fmt.Errorf("function is not a dictionary")
	}

	shadingFunc := NewShadingFunction()

	// 获取 FunctionType
	if funcType, found := funcDict.Find("FunctionType"); found {
		if ft, ok := funcType.(types.Integer); ok {
			shadingFunc.FunctionType = int(ft)
		}
	}

	// 获取 Domain
	if domain, found := funcDict.Find("Domain"); found {
		if arr, ok := domain.(types.Array); ok {
			shadingFunc.Domain = make([]float64, len(arr))
			for i, v := range arr {
				if num, ok := v.(types.Float); ok {
					shadingFunc.Domain[i] = float64(num)
				} else if num, ok := v.(types.Integer); ok {
					shadingFunc.Domain[i] = float64(num)
				}
			}
		}
	}

	// 获取 C0 (起始颜色)
	if c0, found := funcDict.Find("C0"); found {
		if arr, ok := c0.(types.Array); ok {
			shadingFunc.C0 = make([]float64, len(arr))
			for i, v := range arr {
				if num, ok := v.(types.Float); ok {
					shadingFunc.C0[i] = float64(num)
				} else if num, ok := v.(types.Integer); ok {
					shadingFunc.C0[i] = float64(num)
				}
			}
		}
	}

	// 获取 C1 (结束颜色)
	if c1, found := funcDict.Find("C1"); found {
		if arr, ok := c1.(types.Array); ok {
			shadingFunc.C1 = make([]float64, len(arr))
			for i, v := range arr {
				if num, ok := v.(types.Float); ok {
					shadingFunc.C1[i] = float64(num)
				} else if num, ok := v.(types.Integer); ok {
					shadingFunc.C1[i] = float64(num)
				}
			}
		}
	}

	// 获取 N (指数)
	if n, found := funcDict.Find("N"); found {
		if num, ok := n.(types.Float); ok {
			shadingFunc.N = float64(num)
		} else if num, ok := n.(types.Integer); ok {
			shadingFunc.N = float64(num)
		}
	}

	return shadingFunc, nil
}
