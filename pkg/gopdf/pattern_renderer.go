package gopdf

import (
	"fmt"

	"github.com/novvoo/go-cairo/pkg/cairo"
)

// PatternRenderer 图案渲染器
type PatternRenderer struct {
	ctx cairo.Context
}

// NewPatternRenderer 创建新的图案渲染器
func NewPatternRenderer(ctx cairo.Context) *PatternRenderer {
	return &PatternRenderer{
		ctx: ctx,
	}
}

// RenderTilingPattern 渲染平铺图案
func (pr *PatternRenderer) RenderTilingPattern(pattern *Pattern) (cairo.Pattern, error) {
	if !pattern.IsTilingPattern() {
		return nil, fmt.Errorf("not a tiling pattern (PatternType=%d)", pattern.PatternType)
	}

	// 创建图案单元表面
	surface, err := pr.CreatePatternSurface(pattern)
	if err != nil {
		return nil, err
	}
	defer surface.Destroy()

	// 从表面创建 Cairo 图案
	cairoPattern := cairo.NewPatternForSurface(surface)

	// 设置平铺模式
	cairoPattern.SetExtend(cairo.ExtendRepeat)

	// 应用变换矩阵
	// 注意：Cairo 图案矩阵的应用方式与 PDF 不同
	// 这里我们暂时跳过矩阵变换，后续可以通过 context 变换实现

	debugPrintf("✓ Created tiling pattern: %.2fx%.2f, step=(%.2f,%.2f)\n",
		pattern.GetWidth(), pattern.GetHeight(), pattern.XStep, pattern.YStep)

	return cairoPattern, nil
}

// CreatePatternSurface 创建图案单元表面
func (pr *PatternRenderer) CreatePatternSurface(pattern *Pattern) (cairo.Surface, error) {
	// 获取图案边界框
	x1, y1, x2, y2 := pattern.GetBBox()
	width := x2 - x1
	height := y2 - y1

	if width <= 0 || height <= 0 {
		return nil, fmt.Errorf("invalid pattern bbox: %.2f,%.2f,%.2f,%.2f", x1, y1, x2, y2)
	}

	// 创建图像表面用于渲染图案单元
	surface := cairo.NewImageSurface(cairo.FormatARGB32, int(width), int(height))

	// 创建上下文
	patternCtx := cairo.NewContext(surface)
	defer patternCtx.Destroy()

	// 设置透明背景
	patternCtx.SetSourceRGBA(0, 0, 0, 0)
	patternCtx.Paint()

	// 平移以匹配边界框
	patternCtx.Translate(-x1, -y1)

	// 如果有内容流，解析并渲染
	if len(pattern.Stream) > 0 {
		// 解析图案内容流
		operators, err := ParseContentStream(pattern.Stream)
		if err != nil {
			debugPrintf("Warning: failed to parse pattern stream: %v\n", err)
			return surface, nil
		}

		// 创建渲染上下文
		renderCtx := NewRenderContext(patternCtx, width, height)
		renderCtx.Resources = pattern.Resources

		// 执行操作符
		for _, op := range operators {
			if err := op.Execute(renderCtx); err != nil {
				debugPrintf("Warning: pattern operator %s failed: %v\n", op.Name(), err)
			}
		}
	}

	return surface, nil
}

// ApplyPatternFill 应用图案填充
func (pr *PatternRenderer) ApplyPatternFill(pattern *Pattern) error {
	cairoPattern, err := pr.RenderTilingPattern(pattern)
	if err != nil {
		return err
	}
	defer cairoPattern.Destroy()

	pr.ctx.SetSource(cairoPattern)
	return nil
}

// ApplyPatternStroke 应用图案描边
func (pr *PatternRenderer) ApplyPatternStroke(pattern *Pattern) error {
	cairoPattern, err := pr.RenderTilingPattern(pattern)
	if err != nil {
		return err
	}
	defer cairoPattern.Destroy()

	pr.ctx.SetSource(cairoPattern)
	return nil
}
