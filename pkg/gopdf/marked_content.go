package gopdf

// MarkedContentSection 表示一个标记内容区域
type MarkedContentSection struct {
	Tag        string
	Properties map[string]interface{}
}

// MarkedContentStack 标记内容栈，用于跟踪嵌套的标记内容区域
type MarkedContentStack struct {
	stack []*MarkedContentSection
}

// NewMarkedContentStack 创建新的标记内容栈
func NewMarkedContentStack() *MarkedContentStack {
	return &MarkedContentStack{
		stack: make([]*MarkedContentSection, 0),
	}
}

// Push 将新的标记内容区域压入栈
func (s *MarkedContentStack) Push(tag string, properties map[string]interface{}) {
	section := &MarkedContentSection{
		Tag:        tag,
		Properties: properties,
	}
	s.stack = append(s.stack, section)
	debugPrintf("[BMC/BDC] Push marked content: tag=%s, depth=%d\n", tag, len(s.stack))
}

// Pop 从栈中弹出当前标记内容区域
func (s *MarkedContentStack) Pop() *MarkedContentSection {
	if len(s.stack) == 0 {
		debugPrintf("[EMC] Warning: Attempting to pop from empty marked content stack\n")
		return nil
	}

	popped := s.stack[len(s.stack)-1]
	s.stack = s.stack[:len(s.stack)-1]
	debugPrintf("[EMC] Pop marked content: tag=%s, depth=%d\n", popped.Tag, len(s.stack))
	return popped
}

// Current 获取当前标记内容区域（栈顶）
func (s *MarkedContentStack) Current() *MarkedContentSection {
	if len(s.stack) == 0 {
		return nil
	}
	return s.stack[len(s.stack)-1]
}

// Depth 返回栈深度
func (s *MarkedContentStack) Depth() int {
	return len(s.stack)
}

// IsEmpty 检查栈是否为空
func (s *MarkedContentStack) IsEmpty() bool {
	return len(s.stack) == 0
}

// OpBeginMarkedContent BMC - 开始标记内容（简单）
type OpBeginMarkedContent struct {
	Tag string
}

func (op *OpBeginMarkedContent) Name() string { return "BMC" }

func (op *OpBeginMarkedContent) Execute(ctx *RenderContext) error {
	ctx.MarkedContentStack.Push(op.Tag, nil)
	// 标记内容不影响渲染，只是记录结构信息
	return nil
}

// OpBeginMarkedContentWithProperties BDC - 开始标记内容（带属性）
type OpBeginMarkedContentWithProperties struct {
	Tag        string
	Properties map[string]interface{}
}

func (op *OpBeginMarkedContentWithProperties) Name() string { return "BDC" }

func (op *OpBeginMarkedContentWithProperties) Execute(ctx *RenderContext) error {
	ctx.MarkedContentStack.Push(op.Tag, op.Properties)
	// 标记内容不影响渲染，只是记录结构信息
	return nil
}

// OpEndMarkedContent EMC - 结束标记内容
type OpEndMarkedContent struct{}

func (op *OpEndMarkedContent) Name() string { return "EMC" }

func (op *OpEndMarkedContent) Execute(ctx *RenderContext) error {
	ctx.MarkedContentStack.Pop()
	// 标记内容不影响渲染，只是记录结构信息
	return nil
}
