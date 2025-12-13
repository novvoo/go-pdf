package gopdf

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseContentStream 解析 PDF 内容流
func ParseContentStream(stream []byte) ([]PDFOperator, error) {
	content := string(stream)
	tokens := tokenize(content)
	return parseTokens(tokens)
}

// tokenize 将内容流分词
func tokenize(content string) []string {
	var tokens []string
	var current strings.Builder
	inString := false
	inHexString := false
	escape := false

	for i := 0; i < len(content); i++ {
		ch := content[i]

		if escape {
			current.WriteByte(ch)
			escape = false
			continue
		}

		if inString {
			current.WriteByte(ch)
			if ch == '\\' {
				escape = true
			} else if ch == ')' {
				inString = false
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}

		if inHexString {
			current.WriteByte(ch)
			if ch == '>' {
				inHexString = false
				tokens = append(tokens, current.String())
				current.Reset()
			}
			continue
		}

		switch ch {
		case '(':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			inString = true
			current.WriteByte(ch)

		case '<':
			if i+1 < len(content) && content[i+1] == '<' {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
				tokens = append(tokens, "<<")
				i++
			} else {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
				inHexString = true
				current.WriteByte(ch)
			}

		case '>':
			if i+1 < len(content) && content[i+1] == '>' {
				if current.Len() > 0 {
					tokens = append(tokens, current.String())
					current.Reset()
				}
				tokens = append(tokens, ">>")
				i++
			}

		case '[', ']':
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}
			tokens = append(tokens, string(ch))

		case ' ', '\t', '\r', '\n':
			// 只在非字符串上下文中作为分隔符
			if current.Len() > 0 {
				tokens = append(tokens, current.String())
				current.Reset()
			}

		default:
			current.WriteByte(ch)
		}
	}

	if current.Len() > 0 {
		tokens = append(tokens, current.String())
	}

	return tokens
}

// ParseTokens 解析 token 为操作符（导出供测试使用）
func ParseTokens(tokens []string) ([]PDFOperator, error) {
	return parseTokens(tokens)
}

// parseTokens 解析 token 为操作符
func parseTokens(tokens []string) ([]PDFOperator, error) {
	var operators []PDFOperator
	var stack []interface{}

	for i := 0; i < len(tokens); i++ {
		token := tokens[i]

		if token == "" {
			continue
		}

		if token == "[" {
			array := []interface{}{}
			i++
			for i < len(tokens) && tokens[i] != "]" {
				if val := parseValue(tokens[i]); val != nil {
					array = append(array, val)
				}
				i++
			}
			stack = append(stack, array)
			continue
		}

		if token == "<<" {
			dict := make(map[string]interface{})
			i++
			for i < len(tokens) && tokens[i] != ">>" {
				if i+1 < len(tokens) {
					key := tokens[i]
					i++
					value := parseValue(tokens[i])
					dict[key] = value
				}
				i++
			}
			stack = append(stack, dict)
			continue
		}

		if op := createOperator(token, stack); op != nil {
			operators = append(operators, op)
			stack = nil
		} else {
			if val := parseValue(token); val != nil {
				stack = append(stack, val)
			}
		}
	}

	return operators, nil
}

// parseValue 解析值
func parseValue(token string) interface{} {
	if strings.HasPrefix(token, "(") && strings.HasSuffix(token, ")") {
		return token[1 : len(token)-1]
	}

	if strings.HasPrefix(token, "<") && strings.HasSuffix(token, ">") {
		return token
	}

	if strings.HasPrefix(token, "/") {
		return token
	}

	if f, err := strconv.ParseFloat(token, 64); err == nil {
		return f
	}

	if token == "true" {
		return true
	}
	if token == "false" {
		return false
	}

	if token == "null" {
		return nil
	}

	return token
}

// createOperator 根据操作符名称和参数创建操作符对象
func createOperator(name string, args []interface{}) PDFOperator {
	switch name {
	case "q":
		return &OpSaveState{}
	case "Q":
		return &OpRestoreState{}
	case "cm":
		if len(args) >= 6 {
			return &OpConcatMatrix{
				Matrix: &Matrix{
					A: toFloat(args[0]), B: toFloat(args[1]),
					C: toFloat(args[2]), D: toFloat(args[3]),
					E: toFloat(args[4]), F: toFloat(args[5]),
				},
			}
		}
	case "w":
		if len(args) >= 1 {
			return &OpSetLineWidth{Width: toFloat(args[0])}
		}
	case "J":
		if len(args) >= 1 {
			return &OpSetLineCap{Cap: int(toFloat(args[0]))}
		}
	case "j":
		if len(args) >= 1 {
			return &OpSetLineJoin{Join: int(toFloat(args[0]))}
		}
	case "M":
		if len(args) >= 1 {
			return &OpSetMiterLimit{Limit: toFloat(args[0])}
		}
	case "d":
		if len(args) >= 2 {
			pattern := toFloatArray(args[0])
			offset := toFloat(args[1])
			return &OpSetDash{Pattern: pattern, Offset: offset}
		}
	case "gs":
		if len(args) >= 1 {
			return &OpSetGraphicsState{DictName: toString(args[0])}
		}
	case "m":
		if len(args) >= 2 {
			return &OpMoveTo{X: toFloat(args[0]), Y: toFloat(args[1])}
		}
	case "l":
		if len(args) >= 2 {
			return &OpLineTo{X: toFloat(args[0]), Y: toFloat(args[1])}
		}
	case "c":
		if len(args) >= 6 {
			return &OpCurveTo{
				X1: toFloat(args[0]), Y1: toFloat(args[1]),
				X2: toFloat(args[2]), Y2: toFloat(args[3]),
				X3: toFloat(args[4]), Y3: toFloat(args[5]),
			}
		}
	case "v":
		if len(args) >= 4 {
			return &OpCurveToV{
				X2: toFloat(args[0]), Y2: toFloat(args[1]),
				X3: toFloat(args[2]), Y3: toFloat(args[3]),
			}
		}
	case "y":
		if len(args) >= 4 {
			return &OpCurveToY{
				X1: toFloat(args[0]), Y1: toFloat(args[1]),
				X3: toFloat(args[2]), Y3: toFloat(args[3]),
			}
		}
	case "re":
		if len(args) >= 4 {
			return &OpRectangle{
				X: toFloat(args[0]), Y: toFloat(args[1]),
				Width: toFloat(args[2]), Height: toFloat(args[3]),
			}
		}
	case "h":
		return &OpClosePath{}
	case "S":
		return &OpStroke{}
	case "s":
		return &OpCloseAndStroke{}
	case "f", "F":
		return &OpFill{}
	case "f*":
		return &OpFillEvenOdd{}
	case "B":
		return &OpFillAndStroke{}
	case "b":
		return &OpCloseAndFillAndStroke{}
	case "n":
		return &OpEndPath{}
	case "W":
		return &OpClip{}
	case "W*":
		return &OpClipEvenOdd{}
	case "RG":
		if len(args) >= 3 {
			return &OpSetStrokeColorRGB{
				R: toFloat(args[0]),
				G: toFloat(args[1]),
				B: toFloat(args[2]),
			}
		}
	case "rg":
		if len(args) >= 3 {
			return &OpSetFillColorRGB{
				R: toFloat(args[0]),
				G: toFloat(args[1]),
				B: toFloat(args[2]),
			}
		}
	case "G":
		if len(args) >= 1 {
			return &OpSetStrokeColorGray{Gray: toFloat(args[0])}
		}
	case "g":
		if len(args) >= 1 {
			return &OpSetFillColorGray{Gray: toFloat(args[0])}
		}
	case "K":
		if len(args) >= 4 {
			return &OpSetStrokeColorCMYK{
				C: toFloat(args[0]), M: toFloat(args[1]),
				Y: toFloat(args[2]), K: toFloat(args[3]),
			}
		}
	case "k":
		if len(args) >= 4 {
			return &OpSetFillColorCMYK{
				C: toFloat(args[0]), M: toFloat(args[1]),
				Y: toFloat(args[2]), K: toFloat(args[3]),
			}
		}
	case "BT":
		return &OpBeginText{}
	case "ET":
		return &OpEndText{}
	case "EMC":
		// 结束标记内容
		return &OpEndMarkedContent{}
	case "BDC":
		// 开始标记内容（带属性）
		// BDC 有2个参数：标签名和属性字典
		if len(args) >= 2 {
			tag := toString(args[0])
			var properties map[string]interface{}
			if dict, ok := args[1].(map[string]interface{}); ok {
				properties = dict
			}
			return &OpBeginMarkedContentWithProperties{
				Tag:        tag,
				Properties: properties,
			}
		}
		return &OpBeginMarkedContentWithProperties{Tag: "Unknown"}
	case "BMC":
		// 开始标记内容（简单）
		// BMC 有1个参数：标签名
		if len(args) >= 1 {
			return &OpBeginMarkedContent{Tag: toString(args[0])}
		}
		return &OpBeginMarkedContent{Tag: "Unknown"}
	case "Tm":
		if len(args) >= 6 {
			return &OpSetTextMatrix{
				Matrix: &Matrix{
					A: toFloat(args[0]), B: toFloat(args[1]),
					C: toFloat(args[2]), D: toFloat(args[3]),
					E: toFloat(args[4]), F: toFloat(args[5]),
				},
			}
		}
	case "Td":
		if len(args) >= 2 {
			return &OpMoveTextPosition{Tx: toFloat(args[0]), Ty: toFloat(args[1])}
		}
	case "TD":
		if len(args) >= 2 {
			return &OpMoveTextPositionSetLeading{Tx: toFloat(args[0]), Ty: toFloat(args[1])}
		}
	case "T*":
		return &OpMoveToNextLine{}
	case "Tc":
		if len(args) >= 1 {
			return &OpSetCharSpacing{Spacing: toFloat(args[0])}
		}
	case "Tw":
		if len(args) >= 1 {
			return &OpSetWordSpacing{Spacing: toFloat(args[0])}
		}
	case "Tz":
		if len(args) >= 1 {
			return &OpSetHorizontalScaling{Scale: toFloat(args[0])}
		}
	case "TL":
		if len(args) >= 1 {
			return &OpSetLeading{Leading: toFloat(args[0])}
		}
	case "Tf":
		if len(args) >= 2 {
			return &OpSetFont{FontName: toString(args[0]), FontSize: toFloat(args[1])}
		}
	case "Tr":
		if len(args) >= 1 {
			return &OpSetTextRenderMode{Mode: int(toFloat(args[0]))}
		}
	case "Ts":
		if len(args) >= 1 {
			return &OpSetTextRise{Rise: toFloat(args[0])}
		}
	case "Tj":
		if len(args) >= 1 {
			return &OpShowText{Text: toString(args[0])}
		}
	case "'":
		if len(args) >= 1 {
			return &OpShowTextNextLine{Text: toString(args[0])}
		}
	case "\"":
		if len(args) >= 3 {
			return &OpShowTextWithSpacing{
				WordSpacing: toFloat(args[0]),
				CharSpacing: toFloat(args[1]),
				Text:        toString(args[2]),
			}
		}
	case "TJ":
		if len(args) >= 1 {
			return &OpShowTextArray{Array: toArray(args[0])}
		}
	case "Do":
		if len(args) >= 1 {
			return &OpDoXObject{XObjectName: toString(args[0])}
		}
	case "BI":
		if len(args) >= 1 {
			if dict, ok := args[0].(map[string]interface{}); ok {
				return &OpBeginInlineImage{ImageDict: dict}
			}
		}
		return &OpBeginInlineImage{ImageDict: make(map[string]interface{})}
	case "ID":
		return &OpInlineImageData{}
	case "EI":
		return &OpEndInlineImage{}
	case "sh":
		// sh 操作符 - 使用 shading 填充
		if len(args) >= 1 {
			return &OpPaintShading{ShadingName: toString(args[0])}
		}
	}

	return nil
}

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case int:
		return float64(val)
	case string:
		if f, err := strconv.ParseFloat(val, 64); err == nil {
			return f
		}
	}
	return 0
}

func toString(v interface{}) string {
	if s, ok := v.(string); ok {
		// 移除名称前缀 /
		s = strings.TrimPrefix(s, "/")

		// 如果是字符串字面量 (...)，移除括号并处理转义序列
		if strings.HasPrefix(s, "(") && strings.HasSuffix(s, ")") {
			s = s[1 : len(s)-1]
			// 处理 PDF 字符串中的转义序列
			s = unescapePDFString(s)
		}

		return s
	}
	return fmt.Sprintf("%v", v)
}

// unescapePDFString 处理 PDF 字符串中的转义序列
func unescapePDFString(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\\' && i+1 < len(s) {
			switch s[i+1] {
			case 'n':
				result.WriteByte('\n')
				i += 2
			case 'r':
				result.WriteByte('\r')
				i += 2
			case 't':
				result.WriteByte('\t')
				i += 2
			case 'b':
				result.WriteByte('\b')
				i += 2
			case 'f':
				result.WriteByte('\f')
				i += 2
			case '(':
				result.WriteByte('(')
				i += 2
			case ')':
				result.WriteByte(')')
				i += 2
			case '\\':
				result.WriteByte('\\')
				i += 2
			case '\r':
				// \<回车> 被忽略
				i += 2
				if i < len(s) && s[i] == '\n' {
					i++ // 跳过 \r\n
				}
			case '\n':
				// \<换行> 被忽略
				i += 2
			default:
				// 八进制转义 \ddd
				if s[i+1] >= '0' && s[i+1] <= '7' {
					octal := 0
					digits := 0
					j := i + 1
					for j < len(s) && j < i+4 && s[j] >= '0' && s[j] <= '7' {
						octal = octal*8 + int(s[j]-'0')
						digits++
						j++
					}
					if digits > 0 {
						result.WriteByte(byte(octal))
						i = j
					} else {
						// 无效的转义，保留反斜杠
						result.WriteByte('\\')
						i++
					}
				} else {
					// 无效的转义，保留反斜杠
					result.WriteByte('\\')
					i++
				}
			}
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}

func toFloatArray(v interface{}) []float64 {
	if arr, ok := v.([]interface{}); ok {
		result := make([]float64, len(arr))
		for i, item := range arr {
			result[i] = toFloat(item)
		}
		return result
	}
	return nil
}

func toArray(v interface{}) []interface{} {
	if arr, ok := v.([]interface{}); ok {
		return arr
	}
	return nil
}
