//go:build ignore
// +build ignore

package main

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"strings"
)

type Element struct {
	XMLName  xml.Name
	Attrs    []xml.Attr `xml:",any,attr"`
	Content  []byte     `xml:",innerxml"`
	Children []Element  `xml:",any"`
}

func main() {
	file1 := "example/test_vector.svg"
	file2 := "example/test_vector_translated.svg"
	logFile := "svg_diff.log"

	f1, err := os.Open(file1)
	if err != nil {
		panic(err)
	}
	defer f1.Close()

	f2, err := os.Open(file2)
	if err != nil {
		panic(err)
	}
	defer f2.Close()

	d1 := xml.NewDecoder(f1)
	d2 := xml.NewDecoder(f2)

	log, err := os.Create(logFile)
	if err != nil {
		panic(err)
	}
	defer log.Close()

	fmt.Println("Starting Diff Tool...")
	fmt.Fprintf(log, "Diffing %s vs %s\n", file1, file2)

	// Simple token-based diff might be better if structure is similar
	// But let's try to parse into a struct if possible, or just token stream comparison.
	// Since we expect structure to be preserved mostly, token stream is good.

	// Reset file pointers
	f1.Seek(0, 0)
	f2.Seek(0, 0)
	d1 = xml.NewDecoder(f1)
	d2 = xml.NewDecoder(f2)

	compareTokens(d1, d2, log)
}

func compareTokens(d1, d2 *xml.Decoder, log io.Writer) {
	var t1, t2 xml.Token
	var err1, err2 error

	// Skip whitespace
	next1 := func() (xml.Token, error) {
		for {
			t, err := d1.Token()
			if err != nil {
				return nil, err
			}
			if c, ok := t.(xml.CharData); ok {
				if len(strings.TrimSpace(string(c))) == 0 {
					continue
				}
			}
			return t, nil
		}
	}

	wrapperDepth := 0

	next2 := func() (xml.Token, error) {
		for {
			t, err := d2.Token()
			if err != nil {
				return nil, err
			}
			if c, ok := t.(xml.CharData); ok {
				if len(strings.TrimSpace(string(c))) == 0 {
					continue
				}
			}

			// Check for wrapper group start
			if se, ok := t.(xml.StartElement); ok {
				isWrapper := false
				if se.Name.Local == "g" {
					hasTransform := false
					hasOther := false
					transformVal := ""
					for _, a := range se.Attr {
						if a.Name.Local == "transform" && strings.HasPrefix(a.Value, "translate(0,") {
							hasTransform = true
							transformVal = a.Value
						} else {
							hasOther = true
						}
					}
					if hasTransform && !hasOther {
						isWrapper = true
						fmt.Fprintf(log, "Found Wrapper Group: %s\n", transformVal)
					}
				}
				if isWrapper {
					wrapperDepth++
					continue
				}
			}
			return t, nil
		}
	}

	path := []string{}

	for {
		t1, err1 = next1()
		t2, err2 = next2()

		// Handle wrapper end in t2
		for wrapperDepth > 0 {
			// If t2 is </g> and t1 is NOT </g>, it's likely the wrapper end.
			// Or if t1 IS </g>, but we are desynced?
			// Let's be strict: if t2 is </g> and it causes a mismatch with t1.
			// Or better: we know we skipped a start tag, so we MUST skip an end tag eventually.
			// The wrapper surrounds ONE element tree.
			// But we don't know when that tree ends just by looking at tokens.
			// Strategy: When we see a Mismatch, and t2 is </g>, and wrapperDepth > 0, skip t2.

			// We can't do it inside next2 because next2 doesn't know about mismatch.
			// So we do it here.
			if err2 == nil {
				if e2, ok := t2.(xml.EndElement); ok && e2.Name.Local == "g" {
					// Check if t1 matches
					match := false
					if e1, ok1 := t1.(xml.EndElement); ok1 && e1.Name.Local == "g" {
						match = true
					}

					if !match {
						// t2 is </g>, t1 is something else.
						// This must be the wrapper end.
						wrapperDepth--
						t2, err2 = next2()
						continue
					}
				}
			}
			break
		}

		if err1 == io.EOF && err2 == io.EOF {
			break
		}
		if err1 != nil || err2 != nil {
			if err1 != err2 {
				fmt.Fprintf(log, "Error mismatch: %v vs %v\n", err1, err2)
			}
			break
		}

		// Check if t2 is a translation layer start
		if se, ok := t2.(xml.StartElement); ok {
			// Log coordinates for all objects in target
			cx := getAttr(se, "x")
			cy := getAttr(se, "y")
			if cx != "" || cy != "" {
				fmt.Fprintf(log, "[Object] <%s> x=%s y=%s\n", se.Name.Local, cx, cy)
			}

			isTrans := false
			for _, attr := range se.Attr {
				if attr.Name.Local == "data-layer" && attr.Value == "translation-text" {
					isTrans = true
					break
				}
			}
			if isTrans {
				// Dump some attributes of the translation layer to help debug position
				attrs := []string{}
				for _, a := range se.Attr {
					attrs = append(attrs, fmt.Sprintf("%s=%q", a.Name.Local, a.Value))
				}
				fmt.Fprintf(log, "[%s] Found translation layer (valid addition): %s\n", strings.Join(path, "/"), strings.Join(attrs, " "))

				// Inspect translation layer content
				inspectTranslationLayer(d2, log)
				// Fetch next t2 to compare with current t1
				t2, err2 = next2()
				if err2 != nil {
					break
				}
			}
		}

		// Compare t1 and t2
		diffToken(t1, t2, log, path)

		if se, ok := t1.(xml.StartElement); ok {
			id := ""
			for _, a := range se.Attr {
				if a.Name.Local == "id" {
					id = a.Value
					break
				}
			}
			tag := se.Name.Local
			if id != "" {
				tag += "#" + id
			}
			path = append(path, tag)
		}
		if _, ok := t1.(xml.EndElement); ok {
			if len(path) > 0 {
				path = path[:len(path)-1]
			}
		}
	}
}

func consumeElement(d *xml.Decoder, tagName string) {
	depth := 1
	for depth > 0 {
		t, err := d.Token()
		if err != nil {
			break
		}
		switch t.(type) {
		case xml.StartElement:
			depth++
		case xml.EndElement:
			depth--
		}
	}
}

func getAttr(se xml.StartElement, name string) string {
	for _, a := range se.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func inspectTranslationLayer(d *xml.Decoder, log io.Writer) {
	depth := 1
	var lastX, lastY string

	for depth > 0 {
		t, err := d.Token()
		if err != nil {
			break
		}
		switch v := t.(type) {
		case xml.StartElement:
			depth++
			lastX = getAttr(v, "x")
			lastY = getAttr(v, "y")
		case xml.EndElement:
			depth--
		case xml.CharData:
			s := strings.TrimSpace(string(v))
			if len(s) > 0 {
				fmt.Fprintf(log, "Translation Text: %q\n", s)
				if lastX != "" {
					fmt.Fprintf(log, "  X: %s\n", lastX)
				}
				if lastY != "" {
					fmt.Fprintf(log, "  Y: %s\n", lastY)
				}
			}
		}
	}
}

func diffToken(t1, t2 xml.Token, log io.Writer, path []string) {
	pathStr := strings.Join(path, "/")

	switch v1 := t1.(type) {
	case xml.StartElement:
		v2, ok := t2.(xml.StartElement)
		if !ok {
			fmt.Fprintf(log, "[%s] Type mismatch: StartElement vs %T\n", pathStr, t2)
			return
		}
		if v1.Name.Local != v2.Name.Local {
			fmt.Fprintf(log, "[%s] Tag mismatch: <%s> vs <%s>\n", pathStr, v1.Name.Local, v2.Name.Local)
			return
		}
		// Compare attributes
		attrs1 := map[string]string{}
		for _, a := range v1.Attr {
			attrs1[a.Name.Local] = a.Value
		}
		attrs2 := map[string]string{}
		for _, a := range v2.Attr {
			attrs2[a.Name.Local] = a.Value
		}

		for k, val1 := range attrs1 {
			val2, ok := attrs2[k]
			if !ok {
				fmt.Fprintf(log, "[%s] Missing attr in target: %s=%q\n", pathStr, k, val1)
				continue
			}
			if val1 != val2 {
				// Ignore small float diffs if strictly numeric
				// But strings usually differ visually
				if k == "d" || k == "points" || k == "x" || k == "y" {
					fmt.Fprintf(log, "[%s] Attr %s changed: %q -> %q\n", pathStr, k, val1, val2)
				} else if k == "font-family" || k == "font-size" {
					fmt.Fprintf(log, "[%s] Style attr %s changed: %q -> %q\n", pathStr, k, val1, val2)
				} else {
					fmt.Fprintf(log, "[%s] Attr %s changed: %q -> %q\n", pathStr, k, val1, val2)
				}
			}
		}
	case xml.EndElement:
		v2, ok := t2.(xml.EndElement)
		if !ok {
			fmt.Fprintf(log, "[%s] Type mismatch: EndElement vs %T\n", pathStr, t2)
			return
		}
		if v1.Name.Local != v2.Name.Local {
			fmt.Fprintf(log, "[%s] EndTag mismatch: </%s> vs </%s>\n", pathStr, v1.Name.Local, v2.Name.Local)
		}
	case xml.CharData:
		v2, ok := t2.(xml.CharData)
		if !ok {
			fmt.Fprintf(log, "[%s] Type mismatch: CharData vs %T\n", pathStr, t2)
			return
		}
		s1 := strings.TrimSpace(string(v1))
		s2 := strings.TrimSpace(string(v2))
		if s1 != s2 {
			fmt.Fprintf(log, "[%s] Content changed: %q -> %q\n", pathStr, s1, s2)
		}
	}
}
