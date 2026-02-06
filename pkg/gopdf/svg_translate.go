package gopdf

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
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

func cleanAttrs(attrs []xml.Attr) []xml.Attr {
	if len(attrs) == 0 {
		return attrs
	}
	out := make([]xml.Attr, 0, len(attrs))
	for _, a := range attrs {
		if a.Name.Local == "xmlns" {
			continue
		}
		if a.Name.Space == "xmlns" {
			continue
		}
		out = append(out, a)
	}
	return out
}

func insertTextOverlaysIntoSVGBytes(input []byte, byPage map[int][]TextOverlayTopLeft) ([]byte, error) {
	dec := xml.NewDecoder(bytes.NewReader(input))
	var out bytes.Buffer
	enc := xml.NewEncoder(&out)
	enc.Indent("", "  ")

	// We process the stream. When we hit a page group, we buffer its children, process them, and write them back.
	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			t.Attr = cleanAttrs(t.Attr)
			if t.Name.Local == "g" {
				// Check for data-page
				page := 0
				for _, a := range t.Attr {
					if a.Name.Local == "data-page" {
						if v, err := strconv.Atoi(strings.TrimSpace(a.Value)); err == nil {
							page = v
						}
					}
				}

				if page > 0 {
					// Found a page group. Encode the start tag.
					if err := enc.EncodeToken(t); err != nil {
						return nil, err
					}

					// Now consume all children until matching end tag
					// We need to buffer tokens to perform analysis
					var children []xml.Token
					depth := 1
					for depth > 0 {
						childTok, err := dec.Token()
						if err != nil {
							return nil, err
						}
						childTok = xml.CopyToken(childTok)

						switch ct := childTok.(type) {
						case xml.StartElement:
							ct.Attr = cleanAttrs(ct.Attr)
							childTok = ct
							depth++
						case xml.EndElement:
							depth--
						}

						if depth > 0 {
							children = append(children, childTok)
						}
					}

					// Process children
					if ovs, ok := byPage[page]; ok {
						if err := processAndEncodePageContent(enc, children, ovs); err != nil {
							return nil, err
						}
					} else {
						// Just write back as is (with cleanup)
						if err := encodeTokensWithCleanup(enc, children); err != nil {
							return nil, err
						}
					}

					// Write the closing </g> for the page
					if err := enc.EncodeToken(xml.EndElement{Name: t.Name}); err != nil {
						return nil, err
					}
					continue
				}
			}
			if err := enc.EncodeToken(t); err != nil {
				return nil, err
			}

		case xml.EndElement:
			if err := enc.EncodeToken(t); err != nil {
				return nil, err
			}

		default:
			if err := enc.EncodeToken(t); err != nil {
				return nil, err
			}
		}
	}

	if err := enc.Flush(); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func processAndEncodePageContent(enc *xml.Encoder, tokens []xml.Token, overlays []TextOverlayTopLeft) error {
	// Parse tokens into visual elements
	visuals := ParseVisualTokens(tokens)

	// Group elements into visual lines
	lines := DetectVisualLines(visuals)

	// Sort overlays
	sort.Slice(overlays, func(i, j int) bool {
		if overlays[i].Y == overlays[j].Y {
			return overlays[i].X < overlays[j].X
		}
		return overlays[i].Y < overlays[j].Y
	})

	groups := groupOverlays(overlays)

	insertions := map[int]InsertionInfo{}
	var reflowSteps []ReflowStep
	var unmatched []TextOverlayTopLeft

	for _, group := range groups {
		if len(group) == 0 {
			continue
		}
		ov := group[0]

		bestLineIdx := -1
		minDist := 1000.0

		for idx, line := range lines {
			lineY := line.MinY
			dy := ov.Y - lineY
			if dy < -50 || dy > 150 {
				continue
			}

			dist := math.Abs(dy)
			if bestLineIdx == -1 || dist < minDist {
				bestLineIdx = idx
				minDist = dist
			} else if math.Abs(dist-minDist) < 5.0 {
				if dy >= 0 {
					bestLineIdx = idx
					minDist = dist
				}
			}
		}

		if bestLineIdx >= 0 {
			line := lines[bestLineIdx]
			anchorIdx := -1
			for _, eIdx := range line.Indices {
				if eIdx > anchorIdx {
					anchorIdx = eIdx
				}
			}
			insertions[anchorIdx] = InsertionInfo{
				Group:       group,
				RefLineMaxY: line.MaxY,
			}
		} else {
			unmatched = append(unmatched, group...)
		}
	}

	// Write unmatched at the end
	if len(unmatched) > 0 {
		if err := encodeTranslationGroup(enc, unmatched, 0); err != nil {
			return err
		}
	}

	// Iterate visuals
	for idx, v := range visuals {
		shift := calculateTotalShift(v.Y, reflowSteps)

		// Write the element (subtree)
		if shift > 0.1 {
			// Wrap in transform
			startG := xml.StartElement{
				Name: xml.Name{Local: "g"},
				Attr: []xml.Attr{{Name: xml.Name{Local: "transform"}, Value: fmt.Sprintf("translate(0, %s)", formatSVGFloat(shift))}},
			}
			if err := enc.EncodeToken(startG); err != nil {
				return err
			}
			if err := encodeTokensWithCleanup(enc, v.Tokens); err != nil {
				return err
			}
			if err := enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "g"}}); err != nil {
				return err
			}
		} else {
			if err := encodeTokensWithCleanup(enc, v.Tokens); err != nil {
				return err
			}
		}

		if info, ok := insertions[idx]; ok {
			group := info.Group
			refY := v.Y
			totalShift := calculateTotalShift(refY, reflowSteps)

			topPadding := group[0].FontSize * 0.2
			if topPadding < 2.0 {
				topPadding = 2.0
			}

			localOffset := (info.RefLineMaxY - group[0].Y) + topPadding

			groupMinY, groupMaxY := group[0].Y, group[0].Y
			for _, o := range group {
				if o.Y < groupMinY {
					groupMinY = o.Y
				}
				bottom := o.Y + o.FontSize
				if bottom > groupMaxY {
					groupMaxY = bottom
				}
			}
			transHeight := groupMaxY - groupMinY
			if transHeight < group[0].FontSize {
				transHeight = group[0].FontSize
			}

			bottomPadding := group[0].FontSize * 0.4
			shiftAmount := topPadding + transHeight + bottomPadding

			if err := encodeTranslationGroup(enc, group, totalShift+localOffset); err != nil {
				return err
			}

			reflowSteps = append(reflowSteps, ReflowStep{
				ThresholdY: info.RefLineMaxY + 1.0,
				Shift:      shiftAmount,
			})
		}
	}

	return nil
}

func encodeTokensWithCleanup(enc *xml.Encoder, tokens []xml.Token) error {
	suppressStack := []bool{}

	for _, t := range tokens {
		switch se := t.(type) {
		case xml.StartElement:
			se.Attr = cleanAttrs(se.Attr)
			if se.Name.Local == "g" {
				// Check suppression
				shouldSuppress := false
				hasDataPage := false
				modID := 0
				hasVisuals := false

				for _, a := range se.Attr {
					if a.Name.Local == "data-page" {
						hasDataPage = true
					}
					if a.Name.Local == "data-module" {
						modID, _ = strconv.Atoi(a.Value)
					}
					n := a.Name.Local
					if n == "transform" || n == "style" || n == "clip-path" || n == "mask" || n == "filter" || n == "opacity" {
						hasVisuals = true
					}
				}

				if !hasDataPage && modID > 0 && !hasVisuals {
					shouldSuppress = true
				}

				suppressStack = append(suppressStack, shouldSuppress)
				if shouldSuppress {
					continue
				}
			} else if se.Name.Local == "path" || se.Name.Local == "rect" || se.Name.Local == "polyline" || se.Name.Local == "polygon" {
				// Clean attributes
				newAttrs := make([]xml.Attr, 0, len(se.Attr))
				for _, a := range se.Attr {
					if a.Name.Local == "id" && strings.HasPrefix(a.Value, "p") && strings.Contains(a.Value, "-m") {
						continue
					}
					if a.Name.Local == "data-kind" {
						continue
					}
					newAttrs = append(newAttrs, a)
				}
				se.Attr = newAttrs
				if err := enc.EncodeToken(se); err != nil {
					return err
				}
				continue
			}

			if err := enc.EncodeToken(se); err != nil {
				return err
			}

		case xml.EndElement:
			if se.Name.Local == "g" {
				if len(suppressStack) > 0 {
					suppressed := suppressStack[len(suppressStack)-1]
					suppressStack = suppressStack[:len(suppressStack)-1]
					if suppressed {
						continue
					}
				}
			}
			if err := enc.EncodeToken(se); err != nil {
				return err
			}

		case xml.CharData:
			if len(strings.TrimSpace(string(se))) == 0 {
				continue
			}
			if err := enc.EncodeToken(se); err != nil {
				return err
			}

		default:
			if err := enc.EncodeToken(t); err != nil {
				return err
			}
		}
	}
	return nil
}

func encodeTranslationGroup(enc *xml.Encoder, group []TextOverlayTopLeft, shift float64) error {
	if len(group) == 0 {
		return nil
	}

	start := xml.StartElement{
		Name: xml.Name{Local: "g"},
		Attr: []xml.Attr{{Name: xml.Name{Local: "data-layer"}, Value: "translation-text"}},
	}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}

	for _, ov := range group {
		shiftedOv := ov
		shiftedOv.Y += shift

		// Halo
		if err := encodeTextElement(enc, shiftedOv, true); err != nil {
			return err
		}
		// Main
		if err := encodeTextElement(enc, shiftedOv, false); err != nil {
			return err
		}
	}

	return enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "g"}})
}

func encodeTextElement(enc *xml.Encoder, o TextOverlayTopLeft, isHalo bool) error {
	fontSize := o.FontSize
	if fontSize <= 0 {
		fontSize = 12
	}

	attrs := []xml.Attr{
		{Name: xml.Name{Local: "x"}, Value: formatSVGFloat(o.X)},
		{Name: xml.Name{Local: "y"}, Value: formatSVGFloat(o.Y)},
		{Name: xml.Name{Local: "font-size"}, Value: formatSVGFloat(fontSize)},
		{Name: xml.Name{Local: "font-family"}, Value: chooseSVGFontFamily(o.Text, o.FontName)},
		{Name: xml.Name{Local: "xml:space"}, Value: "preserve"},
	}

	if isHalo {
		attrs = append(attrs,
			xml.Attr{Name: xml.Name{Local: "fill"}, Value: "white"},
			xml.Attr{Name: xml.Name{Local: "stroke"}, Value: "white"},
			xml.Attr{Name: xml.Name{Local: "stroke-width"}, Value: "3"},
			xml.Attr{Name: xml.Name{Local: "stroke-linejoin"}, Value: "round"},
		)
		if o.Opacity > 0 && o.Opacity < 1 {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "opacity"}, Value: formatSVGFloat(o.Opacity * 0.8)})
		}
	} else {
		fill := strings.TrimSpace(o.FillColor)
		if fill == "" {
			fill = "0 0 0"
		}
		attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "fill"}, Value: rgbTripletToHex(fill)})

		if o.Opacity > 0 && o.Opacity < 1 {
			attrs = append(attrs, xml.Attr{Name: xml.Name{Local: "opacity"}, Value: formatSVGFloat(o.Opacity)})
		}
	}

	if o.TextLength > 0 {
		attrs = append(attrs,
			xml.Attr{Name: xml.Name{Local: "textLength"}, Value: formatSVGFloat(o.TextLength)},
			xml.Attr{Name: xml.Name{Local: "lengthAdjust"}, Value: "spacingAndGlyphs"},
		)
	}

	start := xml.StartElement{Name: xml.Name{Local: "text"}, Attr: attrs}
	if err := enc.EncodeToken(start); err != nil {
		return err
	}
	if err := enc.EncodeToken(xml.CharData([]byte(ensureValidUTF8(sanitizeXMLText(o.Text))))); err != nil {
		return err
	}
	return enc.EncodeToken(xml.EndElement{Name: xml.Name{Local: "text"}})
}

type InsertionInfo struct {
	Group       []TextOverlayTopLeft
	RefLineMaxY float64
}

type ReflowStep struct {
	ThresholdY float64
	Shift      float64
}

// visualLine struct removed in favor of svg_line_detector.go implementation

func calculateTotalShift(y float64, steps []ReflowStep) float64 {
	shift := 0.0
	for _, s := range steps {
		if s.ThresholdY < y {
			shift += s.Shift
		}
	}
	return shift
}

func groupOverlays(overlays []TextOverlayTopLeft) [][]TextOverlayTopLeft {
	if len(overlays) == 0 {
		return nil
	}
	var groups [][]TextOverlayTopLeft
	var currentGroup []TextOverlayTopLeft

	for _, ov := range overlays {
		if len(currentGroup) == 0 {
			currentGroup = append(currentGroup, ov)
			continue
		}
		last := currentGroup[len(currentGroup)-1]
		dx := math.Abs(ov.X - last.X)
		dy := ov.Y - last.Y
		if dx < 5.0 && dy > 0 && dy < last.FontSize*2.5 {
			currentGroup = append(currentGroup, ov)
		} else {
			groups = append(groups, currentGroup)
			currentGroup = []TextOverlayTopLeft{ov}
		}
	}
	if len(currentGroup) > 0 {
		groups = append(groups, currentGroup)
	}
	return groups
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
	if strings.Contains(lower, "yahei") || strings.Contains(lower, "msyh") || strings.Contains(lower, "simsun") || strings.Contains(lower, "simhei") || strings.Contains(lower, "pingfang") || strings.Contains(lower, "hiragino") || strings.Contains(lower, "noto") || strings.Contains(lower, "cjk") {
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
