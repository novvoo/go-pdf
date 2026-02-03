package gopdf

import (
	"bytes"
	"fmt"
	"os"
	"strconv"
	"strings"
)

func InsertTextOverlaysIntoSVG(inSVGPath, outSVGPath string, overlays []TextOverlayTopLeft) error {
	if inSVGPath == "" || outSVGPath == "" {
		return fmt.Errorf("missing inSVGPath or outSVGPath")
	}
	if len(overlays) == 0 {
		return fmt.Errorf("missing overlays")
	}

	data, err := os.ReadFile(inSVGPath)
	if err != nil {
		return err
	}

	byPage := map[int][]TextOverlayTopLeft{}
	for _, o := range overlays {
		if o.Page <= 0 {
			continue
		}
		byPage[o.Page] = append(byPage[o.Page], o)
	}
	if len(byPage) == 0 {
		return fmt.Errorf("no valid overlays")
	}

	out, err := insertTextOverlaysIntoSVGBytes(data, byPage)
	if err != nil {
		return err
	}

	if err := ensureDirForFile(outSVGPath); err != nil {
		return err
	}
	return os.WriteFile(outSVGPath, out, 0644)
}

type pageGroupMarker struct {
	Page  int
	Depth int
}

func rgbTripletToHex(s string) string {
	parts := strings.Fields(s)
	if len(parts) < 3 {
		return "#000000"
	}
	r, err1 := strconv.ParseFloat(parts[0], 64)
	g, err2 := strconv.ParseFloat(parts[1], 64)
	b, err3 := strconv.ParseFloat(parts[2], 64)
	if err1 != nil || err2 != nil || err3 != nil {
		return "#000000"
	}
	r = clamp01Unit(r)
	g = clamp01Unit(g)
	b = clamp01Unit(b)
	ri := int(r*255 + 0.5)
	gi := int(g*255 + 0.5)
	bi := int(b*255 + 0.5)
	return fmt.Sprintf("#%02x%02x%02x", ri, gi, bi)
}

func insertTextOverlaysIntoSVGBytes(input []byte, byPage map[int][]TextOverlayTopLeft) ([]byte, error) {
	var out bytes.Buffer
	i := 0
	depth := 0
	var pageStack []pageGroupMarker

	for i < len(input) {
		lt := bytes.IndexByte(input[i:], '<')
		if lt < 0 {
			out.Write(input[i:])
			break
		}
		lt += i
		out.Write(input[i:lt])

		gt := bytes.IndexByte(input[lt:], '>')
		if gt < 0 {
			out.Write(input[lt:])
			break
		}
		gt += lt
		tag := string(input[lt : gt+1])
		inner := strings.TrimSpace(tag[1 : len(tag)-1])

		isEndG := strings.HasPrefix(inner, "/g")
		isStartG := !isEndG && strings.HasPrefix(inner, "g") && (len(inner) == 1 || inner[1] == ' ' || inner[1] == '\t' || inner[1] == '\n' || inner[1] == '\r')
		isSelfClose := strings.HasSuffix(inner, "/")

		if isStartG && !isSelfClose {
			depth++
			if p, ok := parseDataPageFromGTag(inner); ok && p > 0 {
				pageStack = append(pageStack, pageGroupMarker{Page: p, Depth: depth})
			}
		}

		if isEndG {
			if len(pageStack) > 0 && pageStack[len(pageStack)-1].Depth == depth {
				m := pageStack[len(pageStack)-1]
				pageStack = pageStack[:len(pageStack)-1]
				if ovs, ok := byPage[m.Page]; ok && len(ovs) > 0 {
					for _, o := range ovs {
						out.WriteString(buildSVGTextElement(o))
					}
				}
			}
			if depth > 0 {
				depth--
			}
		}

		out.WriteString(tag)
		i = gt + 1
	}

	return out.Bytes(), nil
}

func parseDataPageFromGTag(inner string) (int, bool) {
	s := inner
	for {
		i := strings.Index(s, "data-page=")
		if i < 0 {
			return 0, false
		}
		s = s[i+len("data-page="):]
		if s == "" {
			return 0, false
		}
		quote := s[0]
		if quote != '"' && quote != '\'' {
			continue
		}
		s = s[1:]
		j := strings.IndexByte(s, quote)
		if j < 0 {
			return 0, false
		}
		val := strings.TrimSpace(s[:j])
		if v, err := strconv.Atoi(val); err == nil {
			return v, true
		}
		s = s[j+1:]
	}
}

func buildSVGTextElement(o TextOverlayTopLeft) string {
	fontSize := o.FontSize
	if fontSize <= 0 {
		fontSize = 12
	}

	fill := strings.TrimSpace(o.FillColor)
	if fill == "" {
		fill = "0 0 0"
	}
	fillHex := rgbTripletToHex(fill)

	fontFamily := strings.TrimSpace(o.FontName)
	if fontFamily == "" {
		fontFamily = "Helvetica"
	}

	var b strings.Builder
	b.WriteString("\n  <text x=\"")
	b.WriteString(escapeXMLAttr(formatSVGFloat(o.X)))
	b.WriteString("\" y=\"")
	b.WriteString(escapeXMLAttr(formatSVGFloat(o.Y)))
	b.WriteString("\" fill=\"")
	b.WriteString(escapeXMLAttr(fillHex))
	b.WriteString("\" font-size=\"")
	b.WriteString(escapeXMLAttr(formatSVGFloat(fontSize)))
	b.WriteString("\" font-family=\"")
	b.WriteString(escapeXMLAttr(fontFamily))
	b.WriteString("\"")
	if o.Opacity > 0 && o.Opacity < 1 {
		b.WriteString(" opacity=\"")
		b.WriteString(escapeXMLAttr(formatSVGFloat(o.Opacity)))
		b.WriteString("\"")
	}
	b.WriteString(">")
	b.WriteString(escapeXMLText(sanitizeXMLText(o.Text)))
	b.WriteString("</text>\n")
	return b.String()
}

func sanitizeXMLText(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			continue
		}
		if !isValidXML10Rune(r) {
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

func isValidXML10Rune(r rune) bool {
	switch {
	case r == 0x9 || r == 0xA || r == 0xD:
		return true
	case r >= 0x20 && r <= 0xD7FF:
		return true
	case r >= 0xE000 && r <= 0xFFFD:
		return true
	case r >= 0x10000 && r <= 0x10FFFF:
		return true
	default:
		return false
	}
}

func escapeXMLText(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	return s
}

func escapeXMLAttr(s string) string {
	s = escapeXMLText(s)
	s = strings.ReplaceAll(s, "\"", "&quot;")
	s = strings.ReplaceAll(s, "'", "&apos;")
	return s
}
