//go:build ignore
// +build ignore

package main

import (
	"encoding/xml"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf8"
)

type svgTextNode struct {
	Page                 int
	InModule             bool
	Text                 string
	Fill                 string
	FontSize             string
	FontFamily           string
	HasInvalidXMLRunes   bool
	InvalidRunePositions []int
	HasNonASCII          bool
	HasReplacement       bool
	HasControls          bool
}

func main() {
	var inPath string
	flag.StringVar(&inPath, "in", "example/test_vector_translated.svg", "input svg path")
	flag.Parse()

	b, err := os.ReadFile(inPath)
	if err != nil {
		fmt.Printf("read: %v\n", err)
		os.Exit(1)
	}

	if !utf8.Valid(b) {
		fmt.Printf("utf8: invalid\n")
		os.Exit(2)
	}

	report, err := inspectSVG(b)
	if err != nil {
		fmt.Printf("parse: %v\n", err)
		os.Exit(3)
	}
	fmt.Print(report)
}

func inspectSVG(b []byte) (string, error) {
	dec := xml.NewDecoder(strings.NewReader(string(b)))

	var rootSeen bool
	var rootName string

	page := 0
	var pageStack []int
	moduleDepth := 0
	moduleStack := 0

	var currentText *svgTextNode
	var texts []svgTextNode
	var textCount, pathCount, rectCount, polyCount, gCount int

	for {
		tok, err := dec.Token()
		if err != nil {
			if err == io.EOF {
				break
			}
			return "", err
		}

		switch t := tok.(type) {
		case xml.StartElement:
			if !rootSeen {
				rootSeen = true
				rootName = t.Name.Local
			}

			switch t.Name.Local {
			case "g":
				gCount++
				pageStack = append(pageStack, page)
				for _, a := range t.Attr {
					if a.Name.Local == "data-page" {
						var p int
						fmt.Sscanf(strings.TrimSpace(a.Value), "%d", &p)
						if p > 0 {
							page = p
						}
					}
					if a.Name.Local == "data-module" {
						moduleDepth++
						moduleStack++
					}
				}
			case "path":
				pathCount++
			case "rect":
				rectCount++
			case "polyline", "polygon":
				polyCount++
			case "text":
				textCount++
				n := &svgTextNode{Page: page, InModule: moduleDepth > 0}
				for _, a := range t.Attr {
					switch a.Name.Local {
					case "fill":
						n.Fill = a.Value
					case "font-size":
						n.FontSize = a.Value
					case "font-family":
						n.FontFamily = a.Value
					}
				}
				currentText = n
			}
		case xml.EndElement:
			switch t.Name.Local {
			case "g":
				if len(pageStack) > 0 {
					page = pageStack[len(pageStack)-1]
					pageStack = pageStack[:len(pageStack)-1]
				}
				if moduleDepth > 0 {
					moduleDepth--
				}
			case "text":
				if currentText != nil {
					analyzeTextNode(currentText)
					texts = append(texts, *currentText)
				}
				currentText = nil
			}
		case xml.CharData:
			if currentText != nil {
				currentText.Text += string([]byte(t))
			}
		}
	}

	var bld strings.Builder
	bld.WriteString("SVG Inspect Report\n")
	bld.WriteString("==================\n")
	bld.WriteString(fmt.Sprintf("Root element: %s\n", rootName))
	bld.WriteString(fmt.Sprintf("Element counts: g=%d path=%d rect=%d poly=%d text=%d\n", gCount, pathCount, rectCount, polyCount, textCount))

	if rootName != "svg" {
		bld.WriteString("❌ Not an <svg> root\n")
	}
	if !rootSeen {
		bld.WriteString("❌ No root element\n")
	}
	bld.WriteString("\n")

	var originalTexts, translatedTexts []svgTextNode
	for _, t := range texts {
		if t.InModule {
			originalTexts = append(originalTexts, t)
		} else {
			translatedTexts = append(translatedTexts, t)
		}
	}

	bld.WriteString("Text classification\n")
	bld.WriteString("-------------------\n")
	bld.WriteString(fmt.Sprintf("Original (inside data-module): %d\n", len(originalTexts)))
	bld.WriteString(fmt.Sprintf("Translated (direct under data-page): %d\n", len(translatedTexts)))
	bld.WriteString("\n")

	bld.WriteString("Original text as <text> vs outlines\n")
	bld.WriteString("-----------------------------------\n")
	if len(originalTexts) == 0 {
		bld.WriteString("✅ No <text> nodes inside modules; original glyphs are likely drawn as <path>/<rect>/<poly*>\n")
	} else {
		bld.WriteString("⚠️  Found <text> nodes inside modules; original may include real <text> in SVG\n")
		bld.WriteString(fmt.Sprintf("Example original text: %q\n", previewText(originalTexts[0].Text)))
	}
	bld.WriteString("\n")

	bld.WriteString("Translated text sanity\n")
	bld.WriteString("----------------------\n")
	if len(translatedTexts) == 0 {
		bld.WriteString("❌ No translated <text> nodes detected\n")
	} else {
		var invalid, controls, repl, nonASCII int
		var badExample *svgTextNode
		for i := range translatedTexts {
			t := &translatedTexts[i]
			if t.HasInvalidXMLRunes {
				invalid++
				if badExample == nil {
					badExample = t
				}
			}
			if t.HasControls {
				controls++
				if badExample == nil {
					badExample = t
				}
			}
			if t.HasReplacement {
				repl++
			}
			if t.HasNonASCII {
				nonASCII++
			}
		}
		bld.WriteString(fmt.Sprintf("Non-ASCII: %d/%d\n", nonASCII, len(translatedTexts)))
		bld.WriteString(fmt.Sprintf("U+FFFD replacement: %d/%d\n", repl, len(translatedTexts)))
		bld.WriteString(fmt.Sprintf("Control chars (<0x20 except TAB/LF/CR): %d/%d\n", controls, len(translatedTexts)))
		bld.WriteString(fmt.Sprintf("Invalid XML 1.0 runes: %d/%d\n", invalid, len(translatedTexts)))

		style := translatedTexts[0]
		bld.WriteString(fmt.Sprintf("Style sample: fill=%q font-size=%q font-family=%q\n", style.Fill, style.FontSize, style.FontFamily))

		if badExample != nil {
			bld.WriteString(fmt.Sprintf("Example problematic text: %q\n", previewText(badExample.Text)))
		} else {
			bld.WriteString("✅ No invalid XML runes detected in translated text nodes\n")
		}
	}

	_ = moduleStack
	return bld.String(), nil
}

func analyzeTextNode(n *svgTextNode) {
	for i, r := range n.Text {
		if r == utf8.RuneError {
			n.HasReplacement = true
		}
		if r > 127 {
			n.HasNonASCII = true
		}
		if r < 0x20 && r != '\t' && r != '\n' && r != '\r' {
			n.HasControls = true
		}
		if !isValidXML10Rune(r) {
			n.HasInvalidXMLRunes = true
			n.InvalidRunePositions = append(n.InvalidRunePositions, i)
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
