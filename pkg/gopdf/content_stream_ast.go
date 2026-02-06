package gopdf

import (
	"fmt"
	"strconv"
	"unicode"
)

type ContentToken struct {
	Raw        string
	Start, End int
}

type ContentOp struct {
	Name string
	Args []interface{}

	TokenStart int
	TokenEnd   int

	Start int
	End   int
}

func TokenizeContentStreamWithOffsets(b []byte) ([]ContentToken, error) {
	var out []ContentToken

	i := 0
	emit := func(start, end int) {
		if end <= start {
			return
		}
		out = append(out, ContentToken{Raw: string(b[start:end]), Start: start, End: end})
	}

	for i < len(b) {
		c := b[i]
		if c == ' ' || c == '\t' || c == '\r' || c == '\n' || c == 0 {
			i++
			continue
		}

		if c == '%' {
			i++
			for i < len(b) && b[i] != '\n' && b[i] != '\r' {
				i++
			}
			continue
		}

		if c == '(' {
			start := i
			i++
			depth := 1
			escape := false
			for i < len(b) && depth > 0 {
				ch := b[i]
				if escape {
					escape = false
					i++
					continue
				}
				if ch == '\\' {
					escape = true
					i++
					continue
				}
				if ch == '(' {
					depth++
				} else if ch == ')' {
					depth--
				}
				i++
			}
			if depth != 0 {
				return nil, fmt.Errorf("unterminated literal string")
			}
			emit(start, i)
			continue
		}

		if c == '<' {
			if i+1 < len(b) && b[i+1] == '<' {
				emit(i, i+2)
				i += 2
				continue
			}
			start := i
			i++
			for i < len(b) && b[i] != '>' {
				i++
			}
			if i >= len(b) {
				return nil, fmt.Errorf("unterminated hex string")
			}
			i++
			emit(start, i)
			continue
		}

		if c == '>' {
			if i+1 < len(b) && b[i+1] == '>' {
				emit(i, i+2)
				i += 2
				continue
			}
		}

		if c == '[' || c == ']' {
			emit(i, i+1)
			i++
			continue
		}

		if c == '\'' || c == '"' {
			emit(i, i+1)
			i++
			continue
		}

		start := i
		for i < len(b) {
			ch := b[i]
			if ch == ' ' || ch == '\t' || ch == '\r' || ch == '\n' || ch == 0 {
				break
			}
			if ch == '(' || ch == ')' || ch == '[' || ch == ']' || ch == '<' || ch == '>' || ch == '\'' || ch == '"' || ch == '%' {
				break
			}
			i++
		}
		emit(start, i)
	}

	return out, nil
}

func ParseContentOps(tokens []ContentToken) ([]ContentOp, error) {
	var ops []ContentOp
	var stack []interface{}
	stackStart := -1

	for i := 0; i < len(tokens); i++ {
		tok := tokens[i].Raw
		if tok == "" {
			continue
		}

		if tok == "[" {
			if stackStart < 0 {
				stackStart = i
			}
			array := []interface{}{}
			i++
			for i < len(tokens) && tokens[i].Raw != "]" {
				if val := parseValue(tokens[i].Raw); val != nil {
					array = append(array, val)
				}
				i++
			}
			stack = append(stack, array)
			continue
		}

		if tok == "<<" {
			if stackStart < 0 {
				stackStart = i
			}
			dict := make(map[string]interface{})
			i++
			for i < len(tokens) && tokens[i].Raw != ">>" {
				if i+1 < len(tokens) {
					key := tokens[i].Raw
					i++
					value := parseValue(tokens[i].Raw)
					dict[key] = value
				}
				i++
			}
			stack = append(stack, dict)
			continue
		}

		if isOperatorToken(tok) {
			startTok := i
			if len(stack) > 0 && stackStart >= 0 {
				startTok = stackStart
			}
			op := ContentOp{
				Name:       tok,
				Args:       append([]interface{}(nil), stack...),
				TokenStart: startTok,
				TokenEnd:   i,
				Start:      tokens[startTok].Start,
				End:        tokens[i].End,
			}
			ops = append(ops, op)
			stack = nil
			stackStart = -1
			continue
		}

		if val := parseValue(tok); val != nil {
			if stackStart < 0 {
				stackStart = i
			}
			stack = append(stack, val)
		}
	}

	return ops, nil
}

// isKnownOperator is currently unused but kept for potential future AST validation
func _(name string) bool {
	switch name {
	case "q", "Q", "cm",
		"BT", "ET",
		"Tf", "Tm", "Td", "TD", "T*", "Tj", "TJ", "'", "\"",
		"Do":
		return true
	default:
		return false
	}
}

// tokenIsOperator is currently unused
func _(tok ContentToken, op string) bool {
	// return isKnownOperator(tok) // commented out as isKnownOperator is unused
	return false
}

func isOperatorToken(tok string) bool {
	if tok == "" {
		return false
	}
	if tok == "true" || tok == "false" || tok == "null" {
		return false
	}
	if tok == "[" || tok == "]" || tok == "<<" || tok == ">>" {
		return false
	}
	if tok[0] == '/' || tok[0] == '(' || tok[0] == '<' || tok[0] == '%' {
		return false
	}
	if tok == "'" || tok == `"` {
		return true
	}
	if _, err := strconv.ParseFloat(tok, 64); err == nil {
		return false
	}
	for _, r := range tok {
		if r == '*' {
			continue
		}
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}
