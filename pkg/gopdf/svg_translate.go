package gopdf

import (
	"bytes"
	"fmt"
	"math"
	"os"
	"regexp"
	"sort"
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

	var pageBuffer bytes.Buffer
	inPage := false
	pageDepth := -1
	currentPage := 0

	for i < len(input) {
		lt := bytes.IndexByte(input[i:], '<')
		if lt < 0 {
			if inPage {
				pageBuffer.Write(input[i:])
			} else {
				out.Write(input[i:])
			}
			break
		}
		lt += i

		if inPage {
			pageBuffer.Write(input[i:lt])
		} else {
			out.Write(input[i:lt])
		}

		gt := bytes.IndexByte(input[lt:], '>')
		if gt < 0 {
			if inPage {
				pageBuffer.Write(input[lt:])
			} else {
				out.Write(input[lt:])
			}
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
			if !inPage {
				if p, ok := parseDataPageFromGTag(inner); ok && p > 0 {
					inPage = true
					pageDepth = depth
					currentPage = p
				}
			}
		}

		if inPage {
			pageBuffer.WriteString(tag)
		} else {
			out.WriteString(tag)
		}

		if isEndG {
			if inPage && depth == pageDepth {
				// End of page group
				pageContent := pageBuffer.Bytes()
				if ovs, ok := byPage[currentPage]; ok && len(ovs) > 0 {
					processed := processPageContent(pageContent, ovs)
					out.Write(processed)
				} else {
					out.Write(pageContent)
				}

				pageBuffer.Reset()
				inPage = false
				currentPage = 0
				pageDepth = -1
			}
			if depth > 0 {
				depth--
			}
		}

		i = gt + 1
	}

	return out.Bytes(), nil
}

type svgVisualElement struct {
	EndOffset int
	Y         float64
	X         float64
}

func processPageContent(content []byte, overlays []TextOverlayTopLeft) []byte {
	elements := parseVisualElements(content)
	insertions := map[int][]string{}
	var unmatched []TextOverlayTopLeft

	// Sort overlays by Y to help with matching
	sort.Slice(overlays, func(i, j int) bool {
		if overlays[i].Y == overlays[j].Y {
			return overlays[i].X < overlays[j].X
		}
		return overlays[i].Y < overlays[j].Y
	})

	for _, ov := range overlays {
		bestIdx := -1
		minDist := 1000000.0

		for idx, el := range elements {
			// We want the text element to be above the overlay (or same line)
			// el.Y is the anchor Y (baseline). ov.Y is overlay Y.
			// If ov.Y is significantly above el.Y, then el is below the overlay, so we shouldn't append after it?
			// Usually we insert AFTER the line above.
			// So we look for el where el.Y <= ov.Y (el is above or at same level)

			// Allow some tolerance if the overlay is slightly above (e.g. font size diff)
			if el.Y > ov.Y+10 {
				continue
			}

			// Vertical distance (positive if overlay is below)
			dy := ov.Y - el.Y

			// Heuristic: The translation is usually within close proximity (e.g. 50-100 units)
			if dy > 150 {
				continue
			}
			if dy < -50 { // Don't match if overlay is way above element
				continue
			}

			// Horizontal alignment preference
			dx := math.Abs(ov.X - el.X)

			// Combined distance metric
			// Weight vertical distance more
			dist := math.Abs(dy) + dx*0.2

			if dist < minDist {
				minDist = dist
				bestIdx = idx
			}
		}

		if bestIdx >= 0 {
			insertions[elements[bestIdx].EndOffset] = append(insertions[elements[bestIdx].EndOffset], buildSVGTranslationGroup(ov))
		} else {
			unmatched = append(unmatched, ov)
		}
	}

	var out bytes.Buffer

	// Handle unmatched: put them after the opening tag of the page group
	// The content passed starts with <g ...> and ends with </g>
	// We want to insert inside, at the top.
	firstTagEnd := bytes.IndexByte(content, '>')
	if firstTagEnd >= 0 {
		out.Write(content[:firstTagEnd+1])
		for _, ov := range unmatched {
			out.WriteString("\n")
			out.WriteString(buildSVGTranslationGroup(ov))
		}

		lastPos := firstTagEnd + 1

		// Get all insertion points and sort them
		var offsets []int
		for off := range insertions {
			offsets = append(offsets, off)
		}
		sort.Ints(offsets)

		for _, off := range offsets {
			if off > lastPos && off <= len(content) {
				out.Write(content[lastPos:off])
				for _, s := range insertions[off] {
					out.WriteString("\n")
					out.WriteString(s)
				}
				lastPos = off
			}
		}

		if lastPos < len(content) {
			out.Write(content[lastPos:])
		}
	} else {
		out.Write(content)
	}

	return out.Bytes()
}

// Regex to find coordinates in SVG elements
var (
	rePathD      = regexp.MustCompile(`(?i)[d]="\s*M\s*([-0-9.]+)[,\s]+([-0-9.]+)`)
	rePolyPoints = regexp.MustCompile(`(?i)points="\s*([-0-9.]+)[,\s]+([-0-9.]+)`)
	reTextY      = regexp.MustCompile(`(?i)y="([-0-9.]+)"`)
	reTextX      = regexp.MustCompile(`(?i)x="([-0-9.]+)"`)
)

func parseVisualElements(content []byte) []svgVisualElement {
	var elems []svgVisualElement
	i := 0

	// Skip the opening tag of the page itself
	firstGt := bytes.IndexByte(content, '>')
	if firstGt >= 0 {
		i = firstGt + 1
	}

	for i < len(content) {
		lt := bytes.IndexByte(content[i:], '<')
		if lt < 0 {
			break
		}
		lt += i

		gt := bytes.IndexByte(content[lt:], '>')
		if gt < 0 {
			break
		}
		gt += lt

		tagContent := string(content[lt : gt+1])
		inner := strings.TrimSpace(tagContent[1 : len(tagContent)-1])
		tagName := getTagName(inner)

		// We only care about g, path, polyline, text
		if tagName != "g" && tagName != "path" && tagName != "polyline" && tagName != "text" {
			i = gt + 1
			continue
		}

		endOffset := -1

		if strings.HasSuffix(inner, "/") {
			endOffset = gt + 1
		} else {
			// Find closing tag
			endOffset = findClosingTagOffset(content, lt, tagName)
		}

		if endOffset == -1 {
			i = gt + 1
			continue
		}

		// Extract coordinates from the full element content
		elementContent := content[lt:endOffset]
		y, x, ok := extractCoords(elementContent)
		if ok {
			elems = append(elems, svgVisualElement{
				EndOffset: endOffset,
				Y:         y,
				X:         x,
			})
		}

		i = endOffset
	}
	return elems
}

func getTagName(inner string) string {
	parts := strings.Fields(inner)
	if len(parts) > 0 {
		return parts[0]
	}
	return ""
}

func findClosingTagOffset(content []byte, startOffset int, tagName string) int {
	depth := 0
	curr := startOffset

	// Simple scanner to match tags
	// Note: This is not a full XML parser, but sufficient for standard SVG structure
	for curr < len(content) {
		lt := bytes.IndexByte(content[curr:], '<')
		if lt < 0 {
			break
		}
		lt += curr

		gt := bytes.IndexByte(content[lt:], '>')
		if gt < 0 {
			break
		}
		gt += lt

		tag := string(content[lt : gt+1])
		inner := strings.TrimSpace(tag[1 : len(tag)-1])

		isClose := strings.HasPrefix(inner, "/")
		name := getTagName(strings.TrimPrefix(inner, "/"))

		if name == tagName {
			if isClose {
				if depth == 0 {
					return gt + 1
				}
				depth--
			} else if !strings.HasSuffix(inner, "/") {
				// Start tag
				if curr != startOffset { // Don't count the initial tag
					depth++
				}
			}
		}

		curr = gt + 1
	}
	return -1
}

func extractCoords(chunk []byte) (float64, float64, bool) {
	// Try Text
	if mY := reTextY.FindSubmatch(chunk); mY != nil {
		y, _ := strconv.ParseFloat(string(mY[1]), 64)
		x := 0.0
		if mX := reTextX.FindSubmatch(chunk); mX != nil {
			x, _ = strconv.ParseFloat(string(mX[1]), 64)
		}
		return y, x, true
	}

	// Try Path
	if m := rePathD.FindSubmatch(chunk); m != nil {
		x, _ := strconv.ParseFloat(string(m[1]), 64)
		y, _ := strconv.ParseFloat(string(m[2]), 64)
		return y, x, true
	}

	// Try Polyline
	if m := rePolyPoints.FindSubmatch(chunk); m != nil {
		x, _ := strconv.ParseFloat(string(m[1]), 64)
		y, _ := strconv.ParseFloat(string(m[2]), 64)
		return y, x, true
	}

	return 0, 0, false
}

func parseSVGAttribute(tagContent, attr string) (string, bool) {
	needle := attr + "="
	idx := strings.Index(tagContent, needle)
	if idx < 0 {
		return "", false
	}

	rest := tagContent[idx+len(needle):]
	if len(rest) == 0 {
		return "", false
	}

	quote := rest[0]
	if quote != '"' && quote != '\'' {
		return "", false
	}

	rest = rest[1:]
	end := strings.IndexByte(rest, quote)
	if end < 0 {
		return "", false
	}

	return rest[:end], true
}

func parseDataPageFromGTag(inner string) (int, bool) {
	val, ok := parseSVGAttribute(inner, "data-page")
	if !ok {
		return 0, false
	}
	if v, err := strconv.Atoi(val); err == nil {
		return v, true
	}
	return 0, false
}

func buildSVGTranslationGroup(o TextOverlayTopLeft) string {
	var b strings.Builder
	b.WriteString("  <g data-layer=\"translation-text\">\n")
	b.WriteString(buildSVGTextElement(o, "    "))
	b.WriteString("  </g>")
	return b.String()
}

func buildSVGTextElement(o TextOverlayTopLeft, indent string) string {
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
	fontFamily = chooseSVGFontFamily(o.Text, fontFamily)

	var b strings.Builder
	if indent == "" {
		indent = "  "
	}
	b.WriteString(indent)
	b.WriteString("<text x=\"")
	// Use explicit formatting instead of helper to avoid dependency issues
	b.WriteString(escapeXMLAttr(strconv.FormatFloat(o.X, 'f', -1, 64)))
	b.WriteString("\" y=\"")
	b.WriteString(escapeXMLAttr(strconv.FormatFloat(o.Y, 'f', -1, 64)))
	b.WriteString("\" fill=\"")
	b.WriteString(escapeXMLAttr(fillHex))
	b.WriteString("\" font-size=\"")
	b.WriteString(escapeXMLAttr(strconv.FormatFloat(fontSize, 'f', -1, 64)))
	b.WriteString("\" font-family=\"")
	b.WriteString(escapeXMLAttr(fontFamily))
	b.WriteString("\"")
	b.WriteString(" xml:space=\"preserve\"")
	if o.Opacity > 0 && o.Opacity < 1 {
		b.WriteString(" opacity=\"")
		b.WriteString(escapeXMLAttr(strconv.FormatFloat(o.Opacity, 'f', -1, 64)))
		b.WriteString("\"")
	}
	b.WriteString(">")
	b.WriteString(escapeXMLText(sanitizeXMLText(o.Text)))
	b.WriteString("</text>\n")
	return b.String()
}

func chooseSVGFontFamily(text, requested string) string {
	req := strings.TrimSpace(requested)
	if req == "" {
		req = "Helvetica"
	}
	if !containsCJKText(text) {
		return req
	}

	lower := strings.ToLower(req)
	if strings.Contains(lower, "yahei") ||
		strings.Contains(lower, "msyh") ||
		strings.Contains(lower, "simsun") ||
		strings.Contains(lower, "simhei") ||
		strings.Contains(lower, "pingfang") ||
		strings.Contains(lower, "hiragino") ||
		strings.Contains(lower, "noto") ||
		strings.Contains(lower, "cjk") {
		return req
	}

	return "Microsoft YaHei, SimSun, PingFang SC, Noto Sans CJK SC, " + req + ", sans-serif"
}

func containsCJKText(s string) bool {
	for _, r := range s {
		if isCJKCharacterRune(r) {
			return true
		}
	}
	return false
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
