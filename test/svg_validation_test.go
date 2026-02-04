package test

import (
	"bufio"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"strconv"
	"strings"
	"testing"
	"unicode/utf8"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

type svgTextNode struct {
	InModule    bool
	Text        string
	Fill        string
	FontSize    string
	FontFamily  string
	HasControls bool
	HasInvalid  bool
}

type svgInspect struct {
	RootName string
	GCount   int
	Path     int
	Rect     int
	Poly     int
	Text     int
	Original int
	Trans    int
	BadText  []svgTextNode
}

func TestSVGOutputIsValidXML(t *testing.T) {
	helper := NewTestHelper(t)
	psPath := helper.FindTestPDF("test_vector.ps")

	tmpSVG, err := os.CreateTemp("", "gopdf_svg_validate_*.svg")
	if err != nil {
		t.Fatalf("create temp svg: %v", err)
	}
	svgPath := tmpSVG.Name()
	tmpSVG.Close()
	defer os.Remove(svgPath)

	if err := gopdf.ConvertPostScriptToSVG(psPath, svgPath); err != nil {
		t.Fatalf("ConvertPostScriptToSVG: %v", err)
	}
	b, err := os.ReadFile(svgPath)
	if err != nil {
		t.Fatalf("read svg: %v", err)
	}

	inspect := mustInspectSVG(t, b)
	if inspect.RootName != "svg" {
		t.Fatalf("expected <svg> root, got %q", inspect.RootName)
	}
	if inspect.GCount == 0 {
		t.Fatalf("expected some <g> elements")
	}
	if inspect.Text == 0 && inspect.Path == 0 && inspect.Rect == 0 && inspect.Poly == 0 {
		t.Fatalf("expected some drawable elements")
	}
	if len(inspect.BadText) > 0 {
		ex := inspect.BadText[0]
		t.Fatalf("found invalid text nodes: controls=%v invalidXML=%v text=%q font-family=%q",
			ex.HasControls, ex.HasInvalid, previewText(ex.Text), ex.FontFamily)
	}
}

func TestSVGWithOverlayTextIsValidXML(t *testing.T) {
	helper := NewTestHelper(t)
	psPath := helper.FindTestPDF("test_vector.ps")

	tmpBase, err := os.CreateTemp("", "gopdf_svg_validate_base_*.svg")
	if err != nil {
		t.Fatalf("create temp base svg: %v", err)
	}
	baseSVG := tmpBase.Name()
	tmpBase.Close()
	defer os.Remove(baseSVG)

	if err := gopdf.ConvertPostScriptToSVG(psPath, baseSVG); err != nil {
		t.Fatalf("ConvertPostScriptToSVG: %v", err)
	}

	tmpOut, err := os.CreateTemp("", "gopdf_svg_validate_overlay_*.svg")
	if err != nil {
		t.Fatalf("create temp out svg: %v", err)
	}
	outSVG := tmpOut.Name()
	tmpOut.Close()
	defer os.Remove(outSVG)

	overlays := []gopdf.TextOverlayTopLeft{
		{Page: 1, Text: "中文 (test) & <tag> \n第二行", X: 24, Y: 24, FontSize: 12, FillColor: "0 0 0", FontName: "Helvetica"},
	}
	if err := gopdf.InsertTextOverlaysIntoSVG(baseSVG, outSVG, overlays); err != nil {
		t.Fatalf("InsertTextOverlaysIntoSVG: %v", err)
	}
	b, err := os.ReadFile(outSVG)
	if err != nil {
		t.Fatalf("read svg: %v", err)
	}

	inspect := mustInspectSVG(t, b)
	if inspect.Trans == 0 {
		t.Fatalf("expected translated <text> nodes to exist")
	}
	if len(inspect.BadText) > 0 {
		ex := inspect.BadText[0]
		t.Fatalf("found invalid text nodes: controls=%v invalidXML=%v text=%q font-family=%q",
			ex.HasControls, ex.HasInvalid, previewText(ex.Text), ex.FontFamily)
	}
}

func TestSVGOverlayChineseUsesCJKFontFallback(t *testing.T) {
	helper := NewTestHelper(t)
	psPath := helper.FindTestPDF("test_vector.ps")

	tmpSVG, err := os.CreateTemp("", "gopdf_svg_cn_*.svg")
	if err != nil {
		t.Fatalf("create temp svg: %v", err)
	}
	svgPath := tmpSVG.Name()
	tmpSVG.Close()
	defer os.Remove(svgPath)

	tmpOut, err := os.CreateTemp("", "gopdf_svg_cn_out_*.svg")
	if err != nil {
		t.Fatalf("create temp out svg: %v", err)
	}
	outPath := tmpOut.Name()
	tmpOut.Close()
	defer os.Remove(outPath)

	if err := gopdf.ConvertPostScriptToSVG(psPath, svgPath); err != nil {
		t.Fatalf("ConvertPostScriptToSVG: %v", err)
	}

	ovs := []gopdf.TextOverlayTopLeft{
		{Page: 1, Text: "中文测试", X: 50, Y: 50, FontName: "Helvetica", FontSize: 12, FillColor: "0 0 0", Opacity: 1, OnTop: true},
	}
	if err := gopdf.InsertTextOverlaysIntoSVG(svgPath, outPath, ovs); err != nil {
		t.Fatalf("InsertTextOverlaysIntoSVG: %v", err)
	}
	b, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read out svg: %v", err)
	}
	if !strings.Contains(string(b), "中文测试") {
		t.Fatalf("missing inserted chinese text")
	}
	if !strings.Contains(string(b), "Microsoft YaHei") {
		t.Fatalf("expected CJK font fallback in font-family")
	}
}

func TestPostScriptToSVGYAxisNotFlipped(t *testing.T) {
	helper := NewTestHelper(t)
	psPath := helper.FindTestPDF("test_vector.ps")

	pageH := mustParsePSPageHeight(t, psPath)
	txt, xPS, yPS := mustFindFirstShowInPS(t, psPath)

	tmpSVG, err := os.CreateTemp("", "gopdf_svg_yflip_*.svg")
	if err != nil {
		t.Fatalf("create temp svg: %v", err)
	}
	svgPath := tmpSVG.Name()
	tmpSVG.Close()
	defer os.Remove(svgPath)

	if err := gopdf.ConvertPostScriptToSVG(psPath, svgPath); err != nil {
		t.Fatalf("ConvertPostScriptToSVG: %v", err)
	}
	b, err := os.ReadFile(svgPath)
	if err != nil {
		t.Fatalf("read svg: %v", err)
	}

	expY := pageH - yPS
	found := findTextNodeByContentAndXY(t, b, txt, xPS, expY, 1.0)
	if !found {
		t.Fatalf("expected to find text %q near x=%.2f y=%.2f (pageH=%.2f, psY=%.2f)",
			previewText(txt), xPS, expY, pageH, yPS)
	}
}

func mustParsePSPageHeight(t *testing.T, psPath string) float64 {
	t.Helper()
	b, err := os.ReadFile(psPath)
	if err != nil {
		t.Fatalf("read ps: %v", err)
	}
	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if !strings.Contains(line, "BoundingBox:") {
			continue
		}
		i := strings.Index(line, "BoundingBox:")
		if i < 0 {
			continue
		}
		fields := strings.Fields(line[i+len("BoundingBox:"):])
		if len(fields) < 4 {
			continue
		}
		h, err := strconv.ParseFloat(fields[3], 64)
		if err == nil && h > 0 {
			return h
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan ps: %v", err)
	}
	t.Fatalf("missing BoundingBox in ps")
	return 0
}

func mustFindFirstShowInPS(t *testing.T, psPath string) (text string, x float64, y float64) {
	t.Helper()
	b, err := os.ReadFile(psPath)
	if err != nil {
		t.Fatalf("read ps: %v", err)
	}

	inPage := false
	ctm := gopdf.NewIdentityMatrix()
	var stack []*gopdf.Matrix
	lastX, lastY := 0.0, 0.0
	hasPoint := false

	sc := bufio.NewScanner(strings.NewReader(string(b)))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" {
			continue
		}
		if strings.HasPrefix(line, "%") {
			if strings.HasPrefix(line, "%%Page: 1") {
				inPage = true
			}
			continue
		}
		if !inPage {
			continue
		}
		if line == "showpage" {
			break
		}
		if line == "gsave" {
			stack = append(stack, ctm.Clone())
			continue
		}
		if line == "grestore" {
			if len(stack) > 0 {
				ctm = stack[len(stack)-1]
				stack = stack[:len(stack)-1]
			}
			continue
		}
		if strings.HasSuffix(line, " show") {
			s, ok := parsePSShowStringForTest(line)
			if ok && hasPoint {
				return s, lastX, lastY
			}
		}

		fields := strings.Fields(line)
		if len(fields) == 0 {
			continue
		}
		op := fields[len(fields)-1]
		args := fields[:len(fields)-1]
		nums := parsePSFloatsForTest(args)

		switch op {
		case "translate":
			if len(nums) >= 2 {
				ctm = ctm.Translate(nums[0], nums[1])
			}
		case "scale":
			if len(nums) >= 2 {
				ctm = ctm.Scale(nums[0], nums[1])
			}
		case "concat":
			if len(nums) >= 6 {
				m := &gopdf.Matrix{XX: nums[0], YX: nums[1], XY: nums[2], YY: nums[3], X0: nums[4], Y0: nums[5]}
				ctm = ctm.Multiply(m)
			}
		case "moveto":
			if len(nums) >= 2 {
				lastX, lastY = ctm.Transform(nums[0], nums[1])
				hasPoint = true
			}
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("scan ps: %v", err)
	}
	t.Fatalf("no show string found in first page of ps")
	return "", 0, 0
}

func parsePSFloatsForTest(tokens []string) []float64 {
	out := make([]float64, 0, len(tokens))
	for _, t := range tokens {
		clean := strings.TrimSpace(t)
		if clean == "" {
			continue
		}
		clean = strings.Trim(clean, "[]")
		if clean == "" {
			continue
		}
		if v, err := strconv.ParseFloat(clean, 64); err == nil {
			out = append(out, v)
		}
	}
	return out
}

func parsePSShowStringForTest(line string) (string, bool) {
	i := strings.IndexByte(line, '(')
	j := strings.LastIndexByte(line, ')')
	if i < 0 || j < 0 || j <= i {
		return "", false
	}
	raw := line[i+1 : j]
	return unescapePSStringForTest(raw), true
}

func unescapePSStringForTest(s string) string {
	var b strings.Builder
	esc := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if !esc {
			if c == '\\' {
				esc = true
				continue
			}
			b.WriteByte(c)
			continue
		}
		esc = false
		switch c {
		case 'n':
			b.WriteByte('\n')
		case 'r':
			b.WriteByte('\r')
		case 't':
			b.WriteByte('\t')
		default:
			b.WriteByte(c)
		}
	}
	if esc {
		b.WriteByte('\\')
	}
	return b.String()
}

func findTextNodeByContentAndXY(t *testing.T, b []byte, wantText string, wantX, wantY, tol float64) bool {
	t.Helper()
	dec := xml.NewDecoder(strings.NewReader(string(b)))
	var cur *svgTextNode

	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("parse svg: %v", err)
		}
		switch tt := tok.(type) {
		case xml.StartElement:
			if tt.Name.Local == "text" {
				n := &svgTextNode{}
				var xStr, yStr string
				for _, a := range tt.Attr {
					switch a.Name.Local {
					case "x":
						xStr = a.Value
					case "y":
						yStr = a.Value
					}
				}
				if xStr != "" && yStr != "" {
					if xv, err := strconv.ParseFloat(xStr, 64); err == nil {
						n.Text = ""
						n.FontFamily = ""
						n.Fill = ""
						n.FontSize = ""
						n.InModule = false
						if yv, err := strconv.ParseFloat(yStr, 64); err == nil {
							n.HasControls = false
							n.HasInvalid = false
							cur = n
							cur.Fill = fmt.Sprintf("%.4f,%.4f", xv, yv)
						}
					}
				} else {
					cur = nil
				}
			}
		case xml.EndElement:
			if tt.Name.Local == "text" && cur != nil {
				parts := strings.Split(cur.Fill, ",")
				if len(parts) == 2 {
					xv, _ := strconv.ParseFloat(parts[0], 64)
					yv, _ := strconv.ParseFloat(parts[1], 64)
					if strings.TrimSpace(cur.Text) == strings.TrimSpace(wantText) &&
						math.Abs(xv-wantX) <= tol &&
						math.Abs(yv-wantY) <= tol {
						return true
					}
				}
				cur = nil
			}
		case xml.CharData:
			if cur != nil {
				cur.Text += string([]byte(tt))
			}
		}
	}
	return false
}

func mustInspectSVG(t *testing.T, b []byte) svgInspect {
	t.Helper()
	if !utf8.Valid(b) {
		t.Fatalf("svg is not valid utf-8")
	}

	dec := xml.NewDecoder(strings.NewReader(string(b)))
	page := 0
	var pageStack []int
	moduleDepth := 0
	var cur *svgTextNode
	out := svgInspect{}

	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			t.Fatalf("parse svg: %v", err)
		}

		switch tt := tok.(type) {
		case xml.StartElement:
			if out.RootName == "" {
				out.RootName = tt.Name.Local
			}
			switch tt.Name.Local {
			case "g":
				out.GCount++
				pageStack = append(pageStack, page)
				for _, a := range tt.Attr {
					if a.Name.Local == "data-page" {
						var p int
						fmt.Sscanf(strings.TrimSpace(a.Value), "%d", &p)
						if p > 0 {
							page = p
						}
					}
					if a.Name.Local == "data-module" {
						moduleDepth++
					}
				}
			case "path":
				out.Path++
			case "rect":
				out.Rect++
			case "polyline", "polygon":
				out.Poly++
			case "text":
				out.Text++
				n := &svgTextNode{InModule: moduleDepth > 0}
				for _, a := range tt.Attr {
					switch a.Name.Local {
					case "fill":
						n.Fill = a.Value
					case "font-size":
						n.FontSize = a.Value
					case "font-family":
						n.FontFamily = a.Value
					}
				}
				cur = n
			}
		case xml.EndElement:
			switch tt.Name.Local {
			case "g":
				if len(pageStack) > 0 {
					page = pageStack[len(pageStack)-1]
					pageStack = pageStack[:len(pageStack)-1]
				}
				if moduleDepth > 0 {
					moduleDepth--
				}
			case "text":
				if cur != nil {
					analyzeTextNode(cur)
					if cur.InModule {
						out.Original++
					} else {
						out.Trans++
					}
					if cur.HasControls || cur.HasInvalid {
						out.BadText = append(out.BadText, *cur)
					}
				}
				cur = nil
			}
		case xml.CharData:
			if cur != nil {
				cur.Text += string([]byte(tt))
			}
		}
	}

	return out
}

func analyzeTextNode(n *svgTextNode) {
	for _, r := range n.Text {
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			n.HasControls = true
		}
		if !isValidXML10Rune(r) {
			n.HasInvalid = true
		}
	}
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

func previewText(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.TrimSpace(s)
	if len(s) > 140 {
		return s[:140] + "..."
	}
	return s
}
