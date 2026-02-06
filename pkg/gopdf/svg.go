package gopdf

import (
	"bytes"
	"fmt"
	"image/color"
	"io"
	"math"
	"strconv"
	"strings"

	"github.com/tdewolff/parse/v2"
	"github.com/tdewolff/parse/v2/css"
	"github.com/tdewolff/parse/v2/xml"
	"golang.org/x/net/html"
)

type svgDef func(string, Context)

type parserSvgElem struct {
	tag   string
	id    string
	attrs map[string]string
}

type svgState struct {
	strokeMiterLimit float64
	// textX, textY are currently unused but reserved for future text positioning logic
	_              float64
	textAnchor     string
	fontFamily     string
	fontSize       float64
	currentColor   color.RGBA
	fillPattern    Pattern
	strokePattern  Pattern
	strokeWidth    float64
	strokeLineCap  LineCap
	strokeLineJoin LineJoin
	dashArray      []float64
	dashOffset     float64
}

var svgDefaultState = svgState{
	strokeMiterLimit: 4.0,
	textAnchor:       "start",
	fontFamily:       "serif",
	fontSize:         16.0, // in px
	currentColor:     color.RGBA{0, 0, 0, 255},
	fillPattern:      NewPatternRGB(0, 0, 0), // Default fill is black
	strokePattern:    nil,                    // Default stroke is none
	strokeWidth:      1.0,
	strokeLineCap:    LineCapButt,
	strokeLineJoin:   LineJoinMiter,
}

type svgParser struct {
	z   *parse.Input
	err error
	ctx Context

	width, height, diagonal float64

	elemStack  []parserSvgElem
	stateStack []svgState
	state      svgState

	cssRules []cssRule // from <style>
	defs     map[string]svgDef

	// active definitions for attributes
	activeDefs map[string]svgDef
}

func (svg *svgParser) parseViewBox(attrWidth, attrHeight, attrViewBox string) (float64, float64, [4]float64) {
	var err error
	var viewbox [4]float64
	var width, height float64
	if attrViewBox != "" {
		vals := strings.Split(attrViewBox, " ")
		if len(vals) != 4 {
			svg.err = parse.NewErrorLexer(svg.z, "bad viewBox")
		} else {
			for i := 0; i < 4; i++ {
				viewbox[i], err = strconv.ParseFloat(vals[i], 64)
				if err != nil && svg.err == nil {
					svg.err = parse.NewErrorLexer(svg.z, "bad viewBox: %w", err)
				}
			}
		}
	}
	if attrWidth != "" && !strings.HasSuffix(attrWidth, "%") {
		width, _ = ParseSVGDimension(attrWidth, 1.0)
	} else {
		width = viewbox[2] * 25.4 / 96.0
	}
	if attrHeight != "" && !strings.HasSuffix(attrHeight, "%") {
		height, _ = ParseSVGDimension(attrHeight, 1.0)
	} else {
		height = viewbox[3] * 25.4 / 96.0
	}
	return width, height, viewbox
}

func (svg *svgParser) init(width, height float64, viewbox [4]float64) {
	svg.width, svg.height = width*96.0/25.4, height*96.0/25.4
	svg.diagonal = math.Sqrt((svg.width*svg.width + svg.height*svg.height) / 2.0)

	if 0.0 < viewbox[2] && 0.0 < viewbox[3] {
		svg.ctx.Scale(width/viewbox[2], height/viewbox[3])
		svg.ctx.Translate(-viewbox[0], -viewbox[1])
	}

	svg.state = svgDefaultState
	// Initialize context state
	svg.ctx.SetLineJoin(svg.state.strokeLineJoin)
	svg.ctx.SetMiterLimit(svg.state.strokeMiterLimit)
	svg.ctx.SetLineWidth(svg.state.strokeWidth)
	svg.ctx.SetLineCap(svg.state.strokeLineCap)
}

func (svg *svgParser) push(tag string, attrs map[string]string) {
	svg.ctx.Save()
	// Deep copy state for stack
	newState := svg.state
	// Patterns need reference? Pattern interface has Reference().
	if svg.state.fillPattern != nil {
		newState.fillPattern = svg.state.fillPattern.Reference()
	}
	if svg.state.strokePattern != nil {
		newState.strokePattern = svg.state.strokePattern.Reference()
	}

	svg.stateStack = append(svg.stateStack, newState)
	svg.elemStack = append(svg.elemStack, parserSvgElem{tag, attrs["id"], attrs})

	if val, ok := attrs["data-page"]; ok {
		if n, err := strconv.Atoi(val); err == nil {
			if pc, ok := svg.ctx.(interface{ SetPage(int) }); ok {
				pc.SetPage(n)
			}
		}
	}
}

func (svg *svgParser) pop() {
	if len(svg.stateStack) == 0 {
		svg.err = parse.NewErrorLexer(svg.z, "invalid SVG")
		return
	}
	svg.elemStack = svg.elemStack[:len(svg.elemStack)-1]

	// Restore state
	prevState := svg.stateStack[len(svg.stateStack)-1]
	// Clean up current state patterns
	if svg.state.fillPattern != nil {
		svg.state.fillPattern.Destroy()
	}
	if svg.state.strokePattern != nil {
		svg.state.strokePattern.Destroy()
	}
	svg.state = prevState
	svg.stateStack = svg.stateStack[:len(svg.stateStack)-1]

	svg.ctx.Restore()
}

// Public Parsing Helpers

func ParseSVGNumber(v string) (float64, error) {
	if len(v) == 0 {
		return 0.0, nil
	}
	percentage := v[len(v)-1] == '%'
	if percentage {
		v = v[:len(v)-1]
	}
	num, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0.0, err
	}
	if percentage {
		num /= 100.0
	}
	return num, nil
}

func ParseSVGDimension(v string, parent float64) (float64, error) {
	if len(v) == 0 {
		return 0.0, nil
	}

	nn, _ := parse.Dimension([]byte(v))
	num, err := strconv.ParseFloat(v[:nn], 64)
	if err != nil {
		return 0.0, err
	}

	dim := v[nn:]
	switch strings.ToLower(dim) {
	case "cm":
		return num * 10.0 * 96.0 / 25.4, nil
	case "mm":
		return num * 96.0 / 25.4, nil
	case "q":
		return num * 0.25 * 96.0 / 25.4, nil
	case "in":
		return num * 96.0, nil
	case "pc":
		return num * 96.0 / 6.0, nil
	case "pt":
		return num * 96.0 / 72.0, nil
	case "", "px":
		return num, nil
	case "deg":
		return num, nil
	case "grad":
		return num / 400.0 * 360.0, nil
	case "rad":
		return num / math.Pi * 180.0, nil
	case "turn":
		return num * 360.0, nil
	case "%":
		return num * parent / 100.0, nil
	}
	return 0.0, fmt.Errorf("unknown dimension: %s", dim)
}

func ParseSVGColorComponent(v string) (uint8, error) {
	v = strings.TrimSpace(v)
	if len(v) == 0 {
		return 0, nil
	} else if v[len(v)-1] == '%' {
		num, err := strconv.ParseFloat(v[:len(v)-1], 64)
		if err != nil {
			return 0, err
		}
		return uint8(num*255.0 + 0.5), nil
	}
	num, err := strconv.ParseUint(v, 10, 8)
	if err != nil {
		return 0, err
	}
	return uint8(num), nil
}

func ParseSVGColor(v string) (color.RGBA, error) {
	if len(v) == 0 {
		return color.RGBA{0, 0, 0, 255}, nil
	} else if v[0] == '#' {
		return Hex(v), nil
	}
	v = strings.ToLower(v)
	if col, ok := cssColors[v]; ok {
		return col, nil
	}
	var col color.RGBA
	if strings.HasPrefix(v, "rgb(") && strings.HasSuffix(v, ")") {
		comps := strings.Split(v[4:len(v)-1], ",")
		if len(comps) != 3 {
			return color.RGBA{0, 0, 0, 255}, fmt.Errorf("bad rgb function: %s", v)
		}
		r, _ := ParseSVGColorComponent(comps[0])
		g, _ := ParseSVGColorComponent(comps[1])
		b, _ := ParseSVGColorComponent(comps[2])
		col = color.RGBA{r, g, b, 255}
	} else if strings.HasPrefix(v, "rgba(") && strings.HasSuffix(v, ")") {
		comps := strings.Split(v[5:len(v)-1], ",")
		if len(comps) != 4 {
			return color.RGBA{0, 0, 0, 255}, fmt.Errorf("bad rgba function: %s", v)
		}
		alphaStr := strings.TrimSpace(comps[3])
		alpha, err := strconv.ParseFloat(alphaStr, 64)
		if err != nil {
			return color.RGBA{0, 0, 0, 255}, fmt.Errorf("bad alpha component: %w", err)
		}
		col.A = uint8(alpha * 255.0)
		r, _ := ParseSVGColorComponent(comps[0])
		g, _ := ParseSVGColorComponent(comps[1])
		b, _ := ParseSVGColorComponent(comps[2])
		col.R = uint8(float64(r)*float64(col.A)/255.0 + 0.5)
		col.G = uint8(float64(g)*float64(col.A)/255.0 + 0.5)
		col.B = uint8(float64(b)*float64(col.A)/255.0 + 0.5)
	} else {
		col = color.RGBA{0, 0, 0, 255}
	}
	return col, nil
}

func ParseSVGPoints(v string) ([]float64, error) {
	v = strings.ReplaceAll(v, "\n", ",")
	v = strings.ReplaceAll(v, "\t", ",")
	v = strings.ReplaceAll(v, " ", ",")

	vals := []float64{}
	for _, item := range strings.Split(v, ",") {
		if 0 < len(item) {
			val, err := strconv.ParseFloat(item, 64)
			if err != nil {
				return nil, err
			}
			vals = append(vals, val)
		}
	}
	return vals, nil
}

func ParseSVGTransform(v string) (*Matrix, error) {
	i, j := 0, 0
	m := NewMatrix()
	var fun string
	for i < len(v) {
		if v[i] == '(' {
			fun = strings.ToLower(strings.TrimSpace(v[j:i]))
			j = i + 1
		} else if v[i] == ')' {
			d, err := ParseSVGPoints(v[j:i])
			if err != nil {
				return nil, err
			}
			switch fun {
			case "matrix":
				if len(d) != 6 {
					return nil, fmt.Errorf("bad transform matrix")
				} else {
					newM := &Matrix{XX: d[0], YX: d[1], XY: d[2], YY: d[3], X0: d[4], Y0: d[5]}
					m = m.Multiply(newM)
				}
			case "translate":
				if len(d) != 1 && len(d) != 2 {
					return nil, fmt.Errorf("bad transform translate")
				} else if len(d) == 1 {
					m = m.Translate(d[0], 0.0)
				} else {
					m = m.Translate(d[0], d[1])
				}
			case "scale":
				if len(d) != 1 && len(d) != 2 {
					return nil, fmt.Errorf("bad transform scale")
				} else if len(d) == 1 {
					m = m.Scale(d[0], d[0])
				} else {
					m = m.Scale(d[0], d[1])
				}
			case "rotate":
				if len(d) != 1 && len(d) != 3 {
					return nil, fmt.Errorf("bad transform rotate")
				} else if len(d) == 1 {
					m = m.RotateDegrees(d[0])
				} else {
					m = m.Translate(d[1], d[2]).RotateDegrees(d[0]).Translate(-d[1], -d[2])
				}
			case "skewx":
				if len(d) != 1 {
					return nil, fmt.Errorf("bad transform skewX")
				} else {
					skewM := NewMatrix()
					skewM.InitSkew(math.Tan(d[0]*math.Pi/180), 0)
					m = m.Multiply(skewM)
				}
			case "skewy":
				if len(d) != 1 {
					return nil, fmt.Errorf("bad transform skewY")
				} else {
					skewM := NewMatrix()
					skewM.InitSkew(0, math.Tan(d[0]*math.Pi/180))
					m = m.Multiply(skewM)
				}
			}
			j = i + 1
		}
		i++
	}
	return m, nil
}

// Internal wrappers using the public helpers

func (svg *svgParser) parseNumber(v string) float64 {
	val, err := ParseSVGNumber(v)
	if err != nil && svg.err == nil {
		svg.err = parse.NewErrorLexer(svg.z, "bad number: %w: %s", err, v)
	}
	return val
}

func (svg *svgParser) parseDimension(v string, parent float64) float64 {
	val, err := ParseSVGDimension(v, parent)
	if err != nil && svg.err == nil {
		svg.err = parse.NewErrorLexer(svg.z, "bad dimension: %w: %s", err, v)
	}
	return val
}

// parseColorComponent is currently unused
func (svg *svgParser) _(v string) uint8 {
	val, err := ParseSVGColorComponent(v)
	if err != nil && svg.err == nil {
		svg.err = parse.NewErrorLexer(svg.z, "bad color component: %w: %s", err, v)
	}
	return val
}

func (svg *svgParser) parsePaint(v string) Pattern {
	if v == "none" {
		return nil // None
	}
	c := svg.parseColor(v)
	return NewPatternRGBA(float64(c.R)/255.0, float64(c.G)/255.0, float64(c.B)/255.0, float64(c.A)/255.0)
}

func (svg *svgParser) parseColor(v string) color.RGBA {
	col, err := ParseSVGColor(v)
	if err != nil && svg.err == nil {
		svg.err = parse.NewErrorLexer(svg.z, "%w", err)
	}
	return col
}

func Hex(hex string) color.RGBA {
	var r, g, b, a uint8 = 0, 0, 0, 255
	if len(hex) == 4 {
		fmt.Sscanf(hex, "#%1x%1x%1x", &r, &g, &b)
		r *= 17
		g *= 17
		b *= 17
	} else if len(hex) == 5 {
		fmt.Sscanf(hex, "#%1x%1x%1x%1x", &r, &g, &b, &a)
		r *= 17
		g *= 17
		b *= 17
		a *= 17
	} else if len(hex) == 7 {
		fmt.Sscanf(hex, "#%02x%02x%02x", &r, &g, &b)
	} else if len(hex) == 9 {
		fmt.Sscanf(hex, "#%02x%02x%02x%02x", &r, &g, &b, &a)
	}
	return color.RGBA{r, g, b, a}
}

func (svg *svgParser) parsePoints(v string) []float64 {
	vals, err := ParseSVGPoints(v)
	if err != nil && svg.err == nil {
		svg.err = parse.NewErrorLexer(svg.z, "bad number array: %w: %s", err, v)
	}
	return vals
}

func (svg *svgParser) parseTransform(v string) *Matrix {
	m, err := ParseSVGTransform(v)
	if err != nil && svg.err == nil {
		svg.err = parse.NewErrorLexer(svg.z, "%w", err)
	}
	if m == nil {
		return NewMatrix()
	}
	return m
}

func (svg *svgParser) parseAttributes(l *xml.Lexer) (xml.TokenType, []string, map[string]string) {
	var tt xml.TokenType
	attrs := map[string]string{}
	attrNames := []string{}
	for {
		tt, _ = l.Next()
		if tt != xml.AttributeToken {
			break
		}
		val := l.AttrVal()
		if len(val) < 2 {
			break
		}
		val = val[1 : len(val)-1]
		attrNames = append(attrNames, string(l.Text()))
		attrs[string(l.Text())] = string(val)
	}
	return tt, attrNames, attrs
}

type svgTag struct {
	parent    *svgTag
	name      string
	attrNames []string
	attrs     map[string]string
	content   []*svgTag
}

func (svg *svgParser) parseTag(l *xml.Lexer) *svgTag {
	var root, parent *svgTag
	for {
		tt, data := l.Next()
		if tt == xml.ErrorToken {
			if l.Err() != io.EOF {
				svg.err = l.Err()
			} else {
				svg.err = parse.NewErrorLexer(svg.z, "unexpected end-of-file")
			}
			break
		} else if tt == xml.StartTagToken {
			var attrNames []string
			var attrs map[string]string
			name := string(data[1:])
			tt, attrNames, attrs = svg.parseAttributes(l)
			tag := &svgTag{
				parent:    parent,
				name:      name,
				attrNames: attrNames,
				attrs:     attrs,
			}

			if parent == nil {
				root = tag
			} else {
				parent.content = append(parent.content, tag)
			}

			if tt == xml.StartTagCloseVoidToken {
				if parent == nil {
					break
				}
			} else {
				parent = tag
			}

			if name == "style" {
				tt, data = l.Next()
				_ = data // avoid unused error
				_ = tt   // avoid unused error
				if tt == xml.TextToken {
					svg.parseStyle(data)
					_, _ = l.Next()
				} else {
					svg.err = parse.NewErrorLexer(svg.z, "Bad style tag")
				}
				break
			}
		} else if tt == xml.EndTagToken {
			if parent == nil {
				break
			}
			parent = parent.parent
			if parent == nil {
				break
			}
		}
	}
	return root
}

func (svg *svgParser) parseDefs(l *xml.Lexer) {
	for {
		tag := svg.parseTag(l)
		if tag == nil {
			break
		}
		id := tag.attrs["id"]
		if id == "" {
			continue
		}
		switch tag.name {
		case "linearGradient":
			if _, ok := tag.attrs["x2"]; !ok {
				tag.attrs["x2"] = "100%"
			}
			x1 := svg.parseDimension(tag.attrs["x1"], 1.0)
			x2 := svg.parseDimension(tag.attrs["x2"], 1.0)
			y1 := svg.parseDimension(tag.attrs["y1"], 1.0)
			y2 := svg.parseDimension(tag.attrs["y2"], 1.0)

			stops := []gradientStop{}
			for _, tag := range tag.content {
				if tag.name != "stop" {
					continue
				}

				offset := svg.parseNumber(tag.attrs["offset"])
				stopColor := svg.parseColor(tag.attrs["stop-color"])
				stopOpacity := 1.0
				if v, ok := tag.attrs["stop-opacity"]; ok {
					stopOpacity = svg.parseNumber(v)
				}
				stops = append(stops, gradientStop{
					offset: offset,
					red:    float64(stopColor.R) / 255.0,
					green:  float64(stopColor.G) / 255.0,
					blue:   float64(stopColor.B) / 255.0,
					alpha:  float64(stopColor.A) / 255.0 * stopOpacity,
				})
			}

			svg.defs[id] = func(attr string, c Context) {
				pat := NewPatternLinear(x1, y1, x2, y2)
				if gp, ok := pat.(GradientPattern); ok {
					for _, s := range stops {
						gp.AddColorStopRGBA(s.offset, s.red, s.green, s.blue, s.alpha)
					}
				}
				if transform := tag.attrs["gradientTransform"]; transform != "" {
					m := svg.parseTransform(transform)
					pat.SetMatrix(m)
				}

				if attr == "fill" {
					svg.state.fillPattern = pat
				} else if attr == "stroke" {
					svg.state.strokePattern = pat
				}
			}
		case "radialGradient":
			if _, ok := tag.attrs["cx"]; !ok {
				tag.attrs["cx"] = "50%"
			}
			if _, ok := tag.attrs["cy"]; !ok {
				tag.attrs["cy"] = "50%"
			}
			if _, ok := tag.attrs["r"]; !ok {
				tag.attrs["r"] = "50%"
			}

			cx := svg.parseDimension(tag.attrs["cx"], 1.0)
			cy := svg.parseDimension(tag.attrs["cy"], 1.0)
			r := svg.parseDimension(tag.attrs["r"], 1.0)

			stops := []gradientStop{}
			for _, tag := range tag.content {
				if tag.name != "stop" {
					continue
				}
				offset := svg.parseNumber(tag.attrs["offset"])
				stopColor := svg.parseColor(tag.attrs["stop-color"])
				stopOpacity := 1.0
				if v, ok := tag.attrs["stop-opacity"]; ok {
					stopOpacity = svg.parseNumber(v)
				}
				stops = append(stops, gradientStop{
					offset: offset,
					red:    float64(stopColor.R) / 255.0,
					green:  float64(stopColor.G) / 255.0,
					blue:   float64(stopColor.B) / 255.0,
					alpha:  float64(stopColor.A) / 255.0 * stopOpacity,
				})
			}

			svg.defs[id] = func(attr string, c Context) {
				pat := NewPatternRadial(cx, cy, 0, cx, cy, r)
				if gp, ok := pat.(GradientPattern); ok {
					for _, s := range stops {
						gp.AddColorStopRGBA(s.offset, s.red, s.green, s.blue, s.alpha)
					}
				}
				if transform := tag.attrs["gradientTransform"]; transform != "" {
					m := svg.parseTransform(transform)
					pat.SetMatrix(m)
				}
				if attr == "fill" {
					svg.state.fillPattern = pat
				} else if attr == "stroke" {
					svg.state.strokePattern = pat
				}
			}
		}
	}
}

func (svg *svgParser) parseStyle(b []byte) {
	p := css.NewParser(parse.NewInputBytes(b), false)
	selectors := []cssSelector{}
	for {
		gt, _, _ := p.Next()
		if gt == css.ErrorGrammar {
			break
		} else if gt == css.BeginRulesetGrammar || gt == css.QualifiedRuleGrammar {
			selector := cssSelector{}
			node := cssSelectorNode{op: ' '}
			vals := p.Values()
			for i := 0; i < len(vals); i++ {
				t := vals[i]
				if t.TokenType == css.WhitespaceToken || t.TokenType == css.DelimToken && t.Data[0] == '>' {
					selector = append(selector, node)
					node = cssSelectorNode{op: ' '}
					if t.TokenType == css.DelimToken {
						node.op = '>'
					}
				} else if t.TokenType == css.IdentToken || t.TokenType == css.DelimToken && t.Data[0] == '*' {
					node.typ = string(t.Data)
				} else if t.TokenType == css.DelimToken && (t.Data[0] == '.' || t.Data[0] == '#') && i+1 < len(vals) && vals[i+1].TokenType == css.IdentToken {
					if t.Data[0] == '#' {
						node.attrs = append(node.attrs, cssAttrSelector{op: '=', attr: "id", val: string(vals[i+1].Data)})
					} else {
						node.attrs = append(node.attrs, cssAttrSelector{op: '~', attr: "class", val: string(vals[i+1].Data)})
					}
					i++
				} else if t.TokenType == css.DelimToken && t.Data[0] == '[' && i+2 < len(vals) && vals[i+1].TokenType == css.IdentToken && vals[i+2].TokenType == css.DelimToken {
					if vals[i+2].Data[0] == ']' {
						node.attrs = append(node.attrs, cssAttrSelector{op: 0, attr: string(vals[i+1].Data)})
						i += 2
					} else if i+4 < len(vals) && vals[i+3].TokenType == css.IdentToken && vals[i+4].TokenType == css.DelimToken && vals[i+4].Data[0] == ']' {
						node.attrs = append(node.attrs, cssAttrSelector{op: vals[i+2].Data[0], attr: string(vals[i+1].Data), val: string(vals[i+3].Data)})
						i += 4
					}
				}
			}
			selector = append(selector, node)
			selectors = append(selectors, selector)
		}

		if gt == css.BeginRulesetGrammar {
			props := []cssProperty{}
			for {
				gt, _, data := p.Next()
				if gt != css.DeclarationGrammar {
					break
				}

				val := strings.Builder{}
				for _, t := range p.Values() {
					val.Write(t.Data)
				}
				props = append(props, cssProperty{string(data), val.String()})
			}
			svg.cssRules = append(svg.cssRules, cssRule{
				selectors: selectors,
				props:     props,
			})
			selectors = selectors[:0:0]
		}
	}
}

func (svg *svgParser) parseStyleAttribute(style string) []cssProperty {
	props := []cssProperty{}
	p := css.NewParser(parse.NewInput(bytes.NewBufferString(style)), true)
	for {
		gt, _, data := p.Next()
		if gt == css.ErrorGrammar {
			break
		} else if gt == css.DeclarationGrammar {
			val := strings.Builder{}
			for _, t := range p.Values() {
				val.Write(t.Data)
			}
			props = append(props, cssProperty{string(data), val.String()})
		}
	}
	return props
}

func (svg *svgParser) setStyling(props []cssProperty) {
	for _, rule := range svg.cssRules {
		if rule.AppliesTo(svg.elemStack) {
			for _, prop := range rule.props {
				svg.setAttribute(prop.key, prop.val)
			}
		}
	}

	for _, prop := range props {
		if prop.key == "style" {
			for _, styleProp := range svg.parseStyleAttribute(prop.val) {
				svg.setAttribute(styleProp.key, styleProp.val)
			}
		} else {
			svg.setAttribute(prop.key, prop.val)
		}
	}
}

func (svg *svgParser) parseUrlID(val string) string {
	if strings.HasPrefix(val, "url(") && strings.HasSuffix(val, ")") {
		if 6 < len(val) && (val[4] == '#' || val[5] == '#') {
			if val[4] == '#' {
				return val[5 : len(val)-1]
			} else {
				return val[6 : len(val)-2]
			}
		}
	}
	return ""
}

func (svg *svgParser) setAttribute(key, val string) {
	switch key {
	case "fill":
		if id := svg.parseUrlID(val); id != "" {
			svg.activeDefs["fill"] = svg.defs[id]
		} else {
			if svg.state.fillPattern != nil {
				svg.state.fillPattern.Destroy()
			}
			svg.state.fillPattern = svg.parsePaint(val)
		}
	case "stroke":
		if id := svg.parseUrlID(val); id != "" {
			svg.activeDefs["stroke"] = svg.defs[id]
		} else {
			if svg.state.strokePattern != nil {
				svg.state.strokePattern.Destroy()
			}
			svg.state.strokePattern = svg.parsePaint(val)
		}
	case "stroke-width":
		svg.state.strokeWidth = svg.parseDimension(val, svg.diagonal)
		svg.ctx.SetLineWidth(svg.state.strokeWidth)
	case "stroke-dashoffset":
		svg.state.dashOffset = svg.parseDimension(val, svg.diagonal)
		svg.ctx.SetDash(svg.state.dashArray, svg.state.dashOffset)
	case "stroke-dasharray":
		if val == "none" {
			svg.state.dashArray = []float64{}
		} else {
			svg.state.dashArray = svg.parsePoints(val)
		}
		svg.ctx.SetDash(svg.state.dashArray, svg.state.dashOffset)
	case "stroke-linecap":
		if val == "butt" {
			svg.state.strokeLineCap = LineCapButt
		} else if val == "round" {
			svg.state.strokeLineCap = LineCapRound
		} else if val == "square" {
			svg.state.strokeLineCap = LineCapSquare
		}
		svg.ctx.SetLineCap(svg.state.strokeLineCap)
	case "stroke-linejoin":
		if val == "bevel" {
			svg.state.strokeLineJoin = LineJoinBevel
		} else if val == "miter" {
			svg.state.strokeLineJoin = LineJoinMiter
		} else if val == "round" {
			svg.state.strokeLineJoin = LineJoinRound
		}
		svg.ctx.SetLineJoin(svg.state.strokeLineJoin)
	case "stroke-miterlimit":
		svg.state.strokeMiterLimit = svg.parseDimension(val, svg.diagonal)
		svg.ctx.SetMiterLimit(svg.state.strokeMiterLimit)
	case "transform":
		m := svg.parseTransform(val)
		svg.ctx.Transform(m)
	case "text-anchor":
		svg.state.textAnchor = val
	case "font-family":
		svg.state.fontFamily = val
	case "font-size":
		svg.state.fontSize = svg.parseDimension(val, svg.height)
	case "color":
		svg.state.currentColor = svg.parseColor(val)
	}
}

// DrawSVG reads SVG from r and draws it to ctx
func DrawSVG(r io.Reader, ctx Context) error {
	z := parse.NewInput(r)
	defer z.Restore()

	l := xml.NewLexer(z)
	svg := svgParser{
		z:          z,
		ctx:        ctx,
		defs:       map[string]svgDef{},
		activeDefs: map[string]svgDef{},
	}
	for {
		tt, data := l.Next()
		switch tt {
		case xml.ErrorToken:
			if l.Err() != io.EOF {
				return l.Err()
			} else if svg.err != nil {
				return svg.err
			}
			return nil
		case xml.StartTagToken:
			tag := string(data[1:])
			tt, attrNames, attrs := svg.parseAttributes(l)

			if tag == "svg" {
				width, height, viewbox := svg.parseViewBox(attrs["width"], attrs["height"], attrs["viewBox"])
				svg.init(width, height, viewbox)
			}

			if tag == "style" {
				tt, data = l.Next()
				_ = data // avoid unused error
				_ = tt   // avoid unused error
				if tt == xml.TextToken {
					svg.parseStyle(data)
					_, _ = l.Next()
				} else {
					return fmt.Errorf("bad style tag")
				}
				break
			} else if tag == "defs" {
				if tt != xml.StartTagCloseVoidToken {
					svg.parseDefs(l)
				}
				break
			}

			svg.push(tag, attrs)

			props := []cssProperty{}
			for _, key := range attrNames {
				props = append(props, cssProperty{key, attrs[key]})
			}
			svg.setStyling(props)

			for attr, applyDef := range svg.activeDefs {
				if applyDef != nil {
					applyDef(attr, svg.ctx)
				}
				svg.activeDefs[attr] = nil
			}

			svg.drawShape(tag, attrs)

			if tt == xml.StartTagCloseVoidToken {
				svg.pop()
			}
		case xml.TextToken:
			if 0 < len(svg.elemStack) {
				tag := svg.elemStack[len(svg.elemStack)-1].tag
				if tag == "text" {
					_ = html.UnescapeString(string(data))
				}
			}
		case xml.EndTagToken:
			svg.pop()
		}
	}
}

func (svg *svgParser) drawShape(tag string, attrs map[string]string) {
	// Apply paint
	fill := svg.state.fillPattern
	stroke := svg.state.strokePattern

	switch tag {
	case "circle":
		cx := svg.parseDimension(attrs["cx"], svg.width)
		cy := svg.parseDimension(attrs["cy"], svg.height)
		r := svg.parseDimension(attrs["r"], svg.diagonal)
		svg.ctx.DrawCircle(cx, cy, r)
	case "path":
		p := ParseSVGPath(attrs["d"])
		if p != nil {
			svg.ctx.AppendPath(p)
		}
	case "polygon", "polyline":
		points := svg.parsePoints(attrs["points"])
		if len(points) >= 2 {
			svg.ctx.NewPath()
			svg.ctx.MoveTo(points[0], points[1])
			for i := 2; i+1 < len(points); i += 2 {
				svg.ctx.LineTo(points[i], points[i+1])
			}
			if tag == "polygon" {
				svg.ctx.ClosePath()
			}
		}
	case "line":
		x1 := svg.parseDimension(attrs["x1"], svg.width)
		y1 := svg.parseDimension(attrs["y1"], svg.height)
		x2 := svg.parseDimension(attrs["x2"], svg.width)
		y2 := svg.parseDimension(attrs["y2"], svg.height)
		svg.ctx.NewPath()
		svg.ctx.MoveTo(x1, y1)
		svg.ctx.LineTo(x2, y2)
	case "rect":
		x := svg.parseDimension(attrs["x"], svg.width)
		y := svg.parseDimension(attrs["y"], svg.height)
		width := svg.parseDimension(attrs["width"], svg.width)
		height := svg.parseDimension(attrs["height"], svg.height)
		svg.ctx.Rectangle(x, y, width, height)
	}

	// Perform fill and stroke
	if fill != nil {
		svg.ctx.SetSource(fill)
		if stroke != nil {
			svg.ctx.FillPreserve()
		} else {
			svg.ctx.Fill()
		}
	}
	if stroke != nil {
		svg.ctx.SetSource(stroke)
		svg.ctx.Stroke()
	}

	// Clear path if not done (e.g. if no fill/stroke)
	svg.ctx.NewPath()
}

// ParseSVGPath parses an SVG path string and returns a Path object
func ParseSVGPath(d string) *Path {
	path := &PathImpl{
		subpaths: make([]*Subpath, 0),
	}

	// Simple tokenizer adapted from existing code or built from scratch
	toks := tokenizeSVGPathData(d)
	i := 0

	// Track current point for relative commands
	curX, curY := 0.0, 0.0
	// Track subpath start for close
	startX, startY := 0.0, 0.0

	ensureCurrent := func(x, y float64) {
		if path.current == nil {
			path.MoveTo(x, y)
			startX, startY = x, y
		}
	}

	for i < len(toks) {
		cmd := toks[i]
		i++
		switch cmd {
		case "M":
			if i+1 < len(toks) {
				x, _ := strconv.ParseFloat(toks[i], 64)
				y, _ := strconv.ParseFloat(toks[i+1], 64)
				i += 2
				path.MoveTo(x, y)
				curX, curY = x, y
				startX, startY = x, y
				// Subsequent pairs are L
				for i+1 < len(toks) && !isCommand(toks[i]) {
					x, _ = strconv.ParseFloat(toks[i], 64)
					y, _ = strconv.ParseFloat(toks[i+1], 64)
					i += 2
					path.LineTo(x, y)
					curX, curY = x, y
				}
			}
		case "m":
			if i+1 < len(toks) {
				dx, _ := strconv.ParseFloat(toks[i], 64)
				dy, _ := strconv.ParseFloat(toks[i+1], 64)
				i += 2
				x, y := curX+dx, curY+dy
				path.MoveTo(x, y)
				curX, curY = x, y
				startX, startY = x, y
				for i+1 < len(toks) && !isCommand(toks[i]) {
					dx, _ = strconv.ParseFloat(toks[i], 64)
					dy, _ = strconv.ParseFloat(toks[i+1], 64)
					i += 2
					x, y = curX+dx, curY+dy
					path.LineTo(x, y)
					curX, curY = x, y
				}
			}
		case "L":
			if i+1 < len(toks) {
				x, _ := strconv.ParseFloat(toks[i], 64)
				y, _ := strconv.ParseFloat(toks[i+1], 64)
				i += 2
				ensureCurrent(curX, curY)
				path.LineTo(x, y)
				curX, curY = x, y
				for i+1 < len(toks) && !isCommand(toks[i]) {
					x, _ = strconv.ParseFloat(toks[i], 64)
					y, _ = strconv.ParseFloat(toks[i+1], 64)
					i += 2
					path.LineTo(x, y)
					curX, curY = x, y
				}
			}
		case "l":
			if i+1 < len(toks) {
				dx, _ := strconv.ParseFloat(toks[i], 64)
				dy, _ := strconv.ParseFloat(toks[i+1], 64)
				i += 2
				ensureCurrent(curX, curY)
				x, y := curX+dx, curY+dy
				path.LineTo(x, y)
				curX, curY = x, y
				for i+1 < len(toks) && !isCommand(toks[i]) {
					dx, _ = strconv.ParseFloat(toks[i], 64)
					dy, _ = strconv.ParseFloat(toks[i+1], 64)
					i += 2
					x, y = curX+dx, curY+dy
					path.LineTo(x, y)
					curX, curY = x, y
				}
			}
		case "C":
			if i+5 < len(toks) {
				x1, _ := strconv.ParseFloat(toks[i], 64)
				y1, _ := strconv.ParseFloat(toks[i+1], 64)
				x2, _ := strconv.ParseFloat(toks[i+2], 64)
				y2, _ := strconv.ParseFloat(toks[i+3], 64)
				x, _ := strconv.ParseFloat(toks[i+4], 64)
				y, _ := strconv.ParseFloat(toks[i+5], 64)
				i += 6
				ensureCurrent(curX, curY)
				path.CurveTo(x1, y1, x2, y2, x, y)
				curX, curY = x, y
				for i+5 < len(toks) && !isCommand(toks[i]) {
					x1, _ = strconv.ParseFloat(toks[i], 64)
					y1, _ = strconv.ParseFloat(toks[i+1], 64)
					x2, _ = strconv.ParseFloat(toks[i+2], 64)
					y2, _ = strconv.ParseFloat(toks[i+3], 64)
					x, _ = strconv.ParseFloat(toks[i+4], 64)
					y, _ = strconv.ParseFloat(toks[i+5], 64)
					i += 6
					path.CurveTo(x1, y1, x2, y2, x, y)
					curX, curY = x, y
				}
			}
		case "c":
			if i+5 < len(toks) {
				dx1, _ := strconv.ParseFloat(toks[i], 64)
				dy1, _ := strconv.ParseFloat(toks[i+1], 64)
				dx2, _ := strconv.ParseFloat(toks[i+2], 64)
				dy2, _ := strconv.ParseFloat(toks[i+3], 64)
				dx, _ := strconv.ParseFloat(toks[i+4], 64)
				dy, _ := strconv.ParseFloat(toks[i+5], 64)
				i += 6
				ensureCurrent(curX, curY)
				x1, y1 := curX+dx1, curY+dy1
				x2, y2 := curX+dx2, curY+dy2
				x, y := curX+dx, curY+dy
				path.CurveTo(x1, y1, x2, y2, x, y)
				curX, curY = x, y
				for i+5 < len(toks) && !isCommand(toks[i]) {
					dx1, _ = strconv.ParseFloat(toks[i], 64)
					dy1, _ = strconv.ParseFloat(toks[i+1], 64)
					dx2, _ = strconv.ParseFloat(toks[i+2], 64)
					dy2, _ = strconv.ParseFloat(toks[i+3], 64)
					dx, _ = strconv.ParseFloat(toks[i+4], 64)
					dy, _ = strconv.ParseFloat(toks[i+5], 64)
					i += 6
					x1, y1 = curX+dx1, curY+dy1
					x2, y2 = curX+dx2, curY+dy2
					x, y = curX+dx, curY+dy
					path.CurveTo(x1, y1, x2, y2, x, y)
					curX, curY = x, y
				}
			}
		case "Z", "z":
			path.ClosePath()
			curX, curY = startX, startY
		}
	}

	// Convert PathImpl to *Path
	// PathImpl has subpaths which contain segments
	// *Path has Data []PathData

	outData := []PathData{}
	for _, sub := range path.subpaths {
		for _, seg := range sub.segments {
			switch s := seg.(type) {
			case *MoveToSegment:
				outData = append(outData, PathData{Type: PathMoveTo, Points: []Point{{X: s.X, Y: s.Y}}})
			case *LineToSegment:
				outData = append(outData, PathData{Type: PathLineTo, Points: []Point{{X: s.X, Y: s.Y}}})
			case *CurveToSegment:
				outData = append(outData, PathData{Type: PathCurveTo, Points: []Point{{X: s.X1, Y: s.Y1}, {X: s.X2, Y: s.Y2}, {X: s.X3, Y: s.Y3}}})
			case *RectangleSegment:
				// Rectangle is not a primitive in PathData, decompose
				x, y, w, h := s.X, s.Y, s.Width, s.Height
				outData = append(outData, PathData{Type: PathMoveTo, Points: []Point{{X: x, Y: y}}})
				outData = append(outData, PathData{Type: PathLineTo, Points: []Point{{X: x + w, Y: y}}})
				outData = append(outData, PathData{Type: PathLineTo, Points: []Point{{X: x + w, Y: y + h}}})
				outData = append(outData, PathData{Type: PathLineTo, Points: []Point{{X: x, Y: y + h}}})
				outData = append(outData, PathData{Type: PathClosePath, Points: []Point{}})
			}
		}
		if sub.closed && len(sub.segments) > 0 {
			// Check if last was already close (Rectangle adds it)
			lastType := outData[len(outData)-1].Type
			if lastType != PathClosePath {
				outData = append(outData, PathData{Type: PathClosePath, Points: []Point{}})
			}
		}
	}

	return &Path{Data: outData, Status: StatusSuccess}
}

func isCommand(s string) bool {
	return strings.ContainsAny(s, "MmLlCczZ")
}

func tokenizeSVGPathData(d string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() > 0 {
			out = append(out, b.String())
			b.Reset()
		}
	}
	for _, r := range d {
		if strings.ContainsRune("MmLlCcZz", r) {
			flush()
			out = append(out, string(r))
			continue
		}
		if r == ',' || r == ' ' || r == '\n' || r == '\t' || r == '\r' {
			flush()
			continue
		}
		b.WriteRune(r)
	}
	flush()
	return out
}

// CSS Colors map
var cssColors = map[string]color.RGBA{
	"aliceblue":            {240, 248, 255, 255},
	"antiquewhite":         {250, 235, 215, 255},
	"aqua":                 {0, 255, 255, 255},
	"aquamarine":           {127, 255, 212, 255},
	"azure":                {240, 255, 255, 255},
	"beige":                {245, 245, 220, 255},
	"bisque":               {255, 228, 196, 255},
	"black":                {0, 0, 0, 255},
	"blanchedalmond":       {255, 235, 205, 255},
	"blue":                 {0, 0, 255, 255},
	"blueviolet":           {138, 43, 226, 255},
	"brown":                {165, 42, 42, 255},
	"burlywood":            {222, 184, 135, 255},
	"cadetblue":            {95, 158, 160, 255},
	"chartreuse":           {127, 255, 0, 255},
	"chocolate":            {210, 105, 30, 255},
	"coral":                {255, 127, 80, 255},
	"cornflowerblue":       {100, 149, 237, 255},
	"cornsilk":             {255, 248, 220, 255},
	"crimson":              {220, 20, 60, 255},
	"cyan":                 {0, 255, 255, 255},
	"darkblue":             {0, 0, 139, 255},
	"darkcyan":             {0, 139, 139, 255},
	"darkgoldenrod":        {184, 134, 11, 255},
	"darkgray":             {169, 169, 169, 255},
	"darkgreen":            {0, 100, 0, 255},
	"darkgrey":             {169, 169, 169, 255},
	"darkkhaki":            {189, 183, 107, 255},
	"darkmagenta":          {139, 0, 139, 255},
	"darkolivegreen":       {85, 107, 47, 255},
	"darkorange":           {255, 140, 0, 255},
	"darkorchid":           {153, 50, 204, 255},
	"darkred":              {139, 0, 0, 255},
	"darksalmon":           {233, 150, 122, 255},
	"darkseagreen":         {143, 188, 143, 255},
	"darkslateblue":        {72, 61, 139, 255},
	"darkslategray":        {47, 79, 79, 255},
	"darkslategrey":        {47, 79, 79, 255},
	"darkturquoise":        {0, 206, 209, 255},
	"darkviolet":           {148, 0, 211, 255},
	"deeppink":             {255, 20, 147, 255},
	"deepskyblue":          {0, 191, 255, 255},
	"dimgray":              {105, 105, 105, 255},
	"dimgrey":              {105, 105, 105, 255},
	"dodgerblue":           {30, 144, 255, 255},
	"firebrick":            {178, 34, 34, 255},
	"floralwhite":          {255, 250, 240, 255},
	"forestgreen":          {34, 139, 34, 255},
	"fuchsia":              {255, 0, 255, 255},
	"gainsboro":            {220, 220, 220, 255},
	"ghostwhite":           {248, 248, 255, 255},
	"gold":                 {255, 215, 0, 255},
	"goldenrod":            {218, 165, 32, 255},
	"gray":                 {128, 128, 128, 255},
	"green":                {0, 128, 0, 255},
	"greenyellow":          {173, 255, 47, 255},
	"grey":                 {128, 128, 128, 255},
	"honeydew":             {240, 255, 240, 255},
	"hotpink":              {255, 105, 180, 255},
	"indianred":            {205, 92, 92, 255},
	"indigo":               {75, 0, 130, 255},
	"ivory":                {255, 255, 240, 255},
	"khaki":                {240, 230, 140, 255},
	"lavender":             {230, 230, 250, 255},
	"lavenderblush":        {255, 240, 245, 255},
	"lawngreen":            {124, 252, 0, 255},
	"lemonchiffon":         {255, 250, 205, 255},
	"lightblue":            {173, 216, 230, 255},
	"lightcoral":           {240, 128, 128, 255},
	"lightcyan":            {224, 255, 255, 255},
	"lightgoldenrodyellow": {250, 250, 210, 255},
	"lightgray":            {211, 211, 211, 255},
	"lightgreen":           {144, 238, 144, 255},
	"lightgrey":            {211, 211, 211, 255},
	"lightpink":            {255, 182, 193, 255},
	"lightsalmon":          {255, 160, 122, 255},
	"lightseagreen":        {32, 178, 170, 255},
	"lightskyblue":         {135, 206, 250, 255},
	"lightslategray":       {119, 136, 153, 255},
	"lightslategrey":       {119, 136, 153, 255},
	"lightsteelblue":       {176, 196, 222, 255},
	"lightyellow":          {255, 255, 224, 255},
	"lime":                 {0, 255, 0, 255},
	"limegreen":            {50, 205, 50, 255},
	"linen":                {250, 240, 230, 255},
	"magenta":              {255, 0, 255, 255},
	"maroon":               {128, 0, 0, 255},
	"mediumaquamarine":     {102, 205, 170, 255},
	"mediumblue":           {0, 0, 205, 255},
	"mediumorchid":         {186, 85, 211, 255},
	"mediumpurple":         {147, 112, 219, 255},
	"mediumseagreen":       {60, 179, 113, 255},
	"mediumslateblue":      {123, 104, 238, 255},
	"mediumspringgreen":    {0, 250, 154, 255},
	"mediumturquoise":      {72, 209, 204, 255},
	"mediumvioletred":      {199, 21, 133, 255},
	"midnightblue":         {25, 25, 112, 255},
	"mintcream":            {245, 255, 250, 255},
	"mistyrose":            {255, 228, 225, 255},
	"moccasin":             {255, 228, 181, 255},
	"navajowhite":          {255, 222, 173, 255},
	"navy":                 {0, 0, 128, 255},
	"oldlace":              {253, 245, 230, 255},
	"olive":                {128, 128, 0, 255},
	"olivedrab":            {107, 142, 35, 255},
	"orange":               {255, 165, 0, 255},
	"orangered":            {255, 69, 0, 255},
	"orchid":               {218, 112, 214, 255},
	"palegoldenrod":        {238, 232, 170, 255},
	"palegreen":            {152, 251, 152, 255},
	"paleturquoise":        {175, 238, 238, 255},
	"palevioletred":        {219, 112, 147, 255},
	"papayawhip":           {255, 239, 213, 255},
	"peachpuff":            {255, 218, 185, 255},
	"peru":                 {205, 133, 63, 255},
	"pink":                 {255, 192, 203, 255},
	"plum":                 {221, 160, 221, 255},
	"powderblue":           {176, 224, 230, 255},
	"purple":               {128, 0, 128, 255},
	"red":                  {255, 0, 0, 255},
	"rosybrown":            {188, 143, 143, 255},
	"royalblue":            {65, 105, 225, 255},
	"saddlebrown":          {139, 69, 19, 255},
	"salmon":               {250, 128, 114, 255},
	"sandybrown":           {244, 164, 96, 255},
	"seagreen":             {46, 139, 87, 255},
	"seashell":             {255, 245, 238, 255},
	"sienna":               {160, 82, 45, 255},
	"silver":               {192, 192, 192, 255},
	"skyblue":              {135, 206, 235, 255},
	"slateblue":            {106, 90, 205, 255},
	"slategray":            {112, 128, 144, 255},
	"slategrey":            {112, 128, 144, 255},
	"snow":                 {255, 250, 250, 255},
	"springgreen":          {0, 255, 127, 255},
	"steelblue":            {70, 130, 180, 255},
	"tan":                  {210, 180, 140, 255},
	"teal":                 {0, 128, 128, 255},
	"thistle":              {216, 191, 216, 255},
	"tomato":               {255, 99, 71, 255},
	"turquoise":            {64, 224, 208, 255},
	"violet":               {238, 130, 238, 255},
	"wheat":                {245, 222, 179, 255},
	"white":                {255, 255, 255, 255},
	"whitesmoke":           {245, 245, 245, 255},
	"yellow":               {255, 255, 0, 255},
	"yellowgreen":          {154, 205, 50, 255},
}

type cssSelectorNode struct {
	op    byte   // space or >, first is always space
	typ   string // is * for universal
	attrs []cssAttrSelector
}

func (sel cssSelectorNode) AppliesTo(elem parserSvgElem) bool {
	if sel.typ != "*" && sel.typ != "" && sel.typ != elem.tag {
		return false
	}
	for _, attr := range sel.attrs {
		if !attr.AppliesTo(elem) {
			return false
		}
	}
	return true
}

type cssAttrSelector struct {
	op   byte // empty, =, ~, |
	attr string
	val  string
}

func (sel cssAttrSelector) AppliesTo(elem parserSvgElem) bool {
	switch sel.op {
	case 0:
		_, ok := elem.attrs[sel.attr]
		return ok
	case '=':
		return elem.attrs[sel.attr] == sel.val
	case '~':
		vals := strings.Split(elem.attrs[sel.attr], " ")
		for _, val := range vals {
			if val != "" && val == sel.val {
				return true
			}
		}
		return false
	case '|':
		return elem.attrs[sel.attr] == sel.val || strings.HasPrefix(elem.attrs[sel.attr], sel.val+"-")
	}
	return false
}

type cssSelector []cssSelectorNode

func (sels cssSelector) AppliesTo(elems []parserSvgElem) bool {
	ielem := 0
Retry:
	isel := 0
	ielemNext := len(elems)
	for isel < len(sels) && ielem < len(elems) {
		switch sels[isel].op {
		case ' ':
			for {
				if ielem == len(elems) {
					ielem = ielemNext
					goto Retry
				} else if sels[isel].AppliesTo(elems[ielem]) {
					ielem++
					break
				}
				ielem++
			}
			if ielemNext == len(elems) {
				ielemNext = ielem
			}
		case '>':
			if !sels[isel].AppliesTo(elems[ielem]) {
				ielem = ielemNext
				goto Retry
			}
			ielem++
		default:
			return false
		}
		isel++
	}
	return len(sels) != 0 && isel == len(sels)
}

type cssProperty struct {
	key, val string
}

type cssRule struct {
	selectors []cssSelector
	props     []cssProperty
}

func (rule cssRule) AppliesTo(elems []parserSvgElem) bool {
	for _, sels := range rule.selectors {
		if sels.AppliesTo(elems) {
			return true
		}
	}
	return false
}
