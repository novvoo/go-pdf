package gopdf

import (
	"fmt"

	"github.com/novvoo/go-cairo/pkg/cairo"
)

// AnnotationRenderer 注释渲染器
type AnnotationRenderer struct {
	cairoCtx cairo.Context
}

// NewAnnotationRenderer 创建新的注释渲染器
func NewAnnotationRenderer(cairoCtx cairo.Context) *AnnotationRenderer {
	return &AnnotationRenderer{
		cairoCtx: cairoCtx,
	}
}

// RenderAnnotation 渲染注释（根据子类型分发）
func (r *AnnotationRenderer) RenderAnnotation(annot *Annotation) error {
	// 检查注释是否可见
	if !annot.IsVisible() {
		debugPrintf("[Annotation] Skipping hidden annotation: %s\n", annot.Subtype)
		return nil
	}

	debugPrintf("[Annotation] Rendering annotation: %s\n", annot.Subtype)

	// 根据子类型分发
	switch annot.Subtype {
	case "/Text":
		return r.RenderTextAnnotation(annot)
	case "/Highlight":
		return r.RenderHighlightAnnotation(annot)
	case "/Link":
		// 链接注释通常不需要视觉渲染
		debugPrintf("[Annotation] Link annotation (no visual rendering)\n")
		return nil
	case "/Popup":
		// 弹出注释通常不需要独立渲染
		debugPrintf("[Annotation] Popup annotation (no visual rendering)\n")
		return nil
	default:
		debugPrintf("[Annotation] Unsupported annotation type: %s\n", annot.Subtype)
		return nil
	}
}

// RenderTextAnnotation 渲染文本注释（评论图标）
func (r *AnnotationRenderer) RenderTextAnnotation(annot *Annotation) error {
	x1, y1, x2, y2 := annot.GetRect()

	// 保存状态
	r.cairoCtx.Save()
	defer r.cairoCtx.Restore()

	// 如果有外观流，优先使用外观流
	if len(annot.Appearance) > 0 {
		debugPrintf("[Annotation] Text annotation has appearance stream\n")
		// TODO: 渲染外观流
		// 当前简化实现：绘制简单的图标
	}

	// 获取颜色
	red, green, blue := annot.GetColor()
	if red == 0 && green == 0 && blue == 0 {
		// 默认黄色（常见的注释颜色）
		red, green, blue = 1.0, 1.0, 0.0
	}

	// 绘制简单的评论图标（圆形）
	centerX := (x1 + x2) / 2
	centerY := (y1 + y2) / 2
	radius := (x2 - x1) / 2
	if radius > (y2-y1)/2 {
		radius = (y2 - y1) / 2
	}

	// 填充圆形
	r.cairoCtx.Arc(centerX, centerY, radius, 0, 6.28318530718) // 2*π
	r.cairoCtx.SetSourceRGB(red, green, blue)
	r.cairoCtx.Fill()

	// 绘制边框
	r.cairoCtx.Arc(centerX, centerY, radius, 0, 6.28318530718)
	r.cairoCtx.SetSourceRGB(red*0.7, green*0.7, blue*0.7)
	r.cairoCtx.SetLineWidth(1.0)
	r.cairoCtx.Stroke()

	debugPrintf("[Annotation] Rendered text annotation at (%.2f, %.2f)\n", centerX, centerY)
	return nil
}

// RenderHighlightAnnotation 渲染高亮注释
func (r *AnnotationRenderer) RenderHighlightAnnotation(annot *Annotation) error {
	// 保存状态
	r.cairoCtx.Save()
	defer r.cairoCtx.Restore()

	// 如果有外观流，优先使用外观流
	if len(annot.Appearance) > 0 {
		debugPrintf("[Annotation] Highlight annotation has appearance stream\n")
		// TODO: 渲染外观流
	}

	// 获取颜色
	red, green, blue := annot.GetColor()
	if red == 0 && green == 0 && blue == 0 {
		// 默认黄色高亮
		red, green, blue = 1.0, 1.0, 0.0
	}

	// 如果有四边形点，使用它们
	if len(annot.QuadPoints) >= 8 {
		// QuadPoints 定义了高亮区域的四边形
		// 格式：[x1 y1 x2 y2 x3 y3 x4 y4] 按逆时针顺序
		r.cairoCtx.MoveTo(annot.QuadPoints[0], annot.QuadPoints[1])
		for i := 2; i < len(annot.QuadPoints); i += 2 {
			r.cairoCtx.LineTo(annot.QuadPoints[i], annot.QuadPoints[i+1])
		}
		r.cairoCtx.ClosePath()
	} else {
		// 使用矩形
		x1, y1, x2, y2 := annot.GetRect()
		r.cairoCtx.Rectangle(x1, y1, x2-x1, y2-y1)
	}

	// 使用半透明填充
	r.cairoCtx.SetSourceRGBA(red, green, blue, 0.3)
	r.cairoCtx.Fill()

	debugPrintf("[Annotation] Rendered highlight annotation\n")
	return nil
}

// RenderAnnotationAppearance 渲染注释的外观流
func (r *AnnotationRenderer) RenderAnnotationAppearance(annot *Annotation, appearanceKey string) error {
	// 获取外观流
	apObj, found := annot.Appearance[appearanceKey]
	if !found {
		return fmt.Errorf("appearance %s not found", appearanceKey)
	}

	// TODO: 解析并渲染外观流
	// 外观流是一个 XObject Form，需要像渲染 Form XObject 一样处理
	debugPrintf("[Annotation] Appearance stream rendering not yet implemented\n")

	_ = apObj // 避免未使用变量警告
	return nil
}
