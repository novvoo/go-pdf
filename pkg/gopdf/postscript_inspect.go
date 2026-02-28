package gopdf

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type PSShowOccurrence struct {
	PageNo   int
	LineNo   int
	FontName string
	FontSize float64
	Text     string
	RawLine  string
	RawIsHex bool
}

func ExtractPostScriptShows(psContent string) []PSShowOccurrence {
	lines := strings.Split(psContent, "\n")
	out := make([]PSShowOccurrence, 0, 256)
	pageNo := 0
	fontName := ""
	fontSize := 0.0
	for i, line := range lines {
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		if strings.HasPrefix(s, "%%Page:") {
			fields := strings.Fields(s)
			if len(fields) >= 2 {
				if p, err := strconv.Atoi(fields[1]); err == nil {
					pageNo = p
				}
			}
			fontName = ""
			fontSize = 0
			continue
		}
		if s == "showpage" || strings.HasPrefix(s, "%%Trailer") {
			fontName = ""
			fontSize = 0
			continue
		}
		if fn, fs, ok := parsePSSetFont(s); ok {
			fontName = fn
			fontSize = fs
		}
		shows := extractShowTextsFromLine(s)
		if len(shows) == 0 {
			continue
		}
		for _, sh := range shows {
			out = append(out, PSShowOccurrence{
				PageNo:   pageNo,
				LineNo:   i + 1,
				FontName: fontName,
				FontSize: fontSize,
				Text:     sh.text,
				RawLine:  strings.TrimRight(line, "\r"),
				RawIsHex: sh.rawIsHex,
			})
		}
	}
	return out
}

type PSColorImage struct {
	PageNo  int
	LineNo  int
	Width   int
	Height  int
	RGB     []byte
	RawHead string
}

func ExtractPostScriptColorImages(psContent string, maxImages int) ([]PSColorImage, error) {
	lines := strings.Split(psContent, "\n")
	out := make([]PSColorImage, 0, 8)
	pageNo := 0
	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(strings.TrimRight(lines[i], "\r"))
		if strings.HasPrefix(line, "%%Page:") {
			fields := strings.Fields(line)
			if len(fields) >= 2 {
				if p, err := strconv.Atoi(fields[1]); err == nil {
					pageNo = p
				}
			}
			continue
		}
		w, h, ok := parsePSColorImageHeader(line)
		if !ok {
			continue
		}
		need := w * h * 3
		needHex := need * 2
		hexDigits := make([]byte, 0, needHex)
		j := i + 1
		for ; j < len(lines) && len(hexDigits) < needHex; j++ {
			ln := strings.TrimRight(lines[j], "\r")
			for k := 0; k < len(ln) && len(hexDigits) < needHex; k++ {
				c := ln[k]
				switch {
				case c >= '0' && c <= '9':
					hexDigits = append(hexDigits, c)
				case c >= 'a' && c <= 'f':
					hexDigits = append(hexDigits, c)
				case c >= 'A' && c <= 'F':
					hexDigits = append(hexDigits, c)
				}
			}
		}
		if len(hexDigits) < needHex {
			return out, fmt.Errorf("colorimage data too short at line %d: got_hex=%d need_hex=%d", i+1, len(hexDigits), needHex)
		}
		b := make([]byte, need)
		if _, err := hex.Decode(b, hexDigits[:needHex]); err != nil {
			return out, fmt.Errorf("decode colorimage hex at line %d: %w", i+1, err)
		}
		out = append(out, PSColorImage{
			PageNo:  pageNo,
			LineNo:  i + 1,
			Width:   w,
			Height:  h,
			RGB:     b,
			RawHead: line,
		})
		i = j
		if maxImages > 0 && len(out) >= maxImages {
			break
		}
	}
	return out, nil
}

func WritePSColorImagesToDir(images []PSColorImage, dir string, prefix string) ([]string, error) {
	if prefix == "" {
		prefix = "psimg"
	}
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(images))
	for idx, im := range images {
		if im.Width <= 0 || im.Height <= 0 {
			continue
		}
		rgba := image.NewRGBA(image.Rect(0, 0, im.Width, im.Height))
		p := 0
		for y := 0; y < im.Height; y++ {
			for x := 0; x < im.Width; x++ {
				rgba.Set(x, y, color.RGBA{R: im.RGB[p], G: im.RGB[p+1], B: im.RGB[p+2], A: 255})
				p += 3
			}
		}
		name := fmt.Sprintf("%s_p%03d_%03d_%dx%d.png", prefix, maxInt(1, im.PageNo), idx+1, im.Width, im.Height)
		outPath := filepath.Join(dir, name)
		f, err := os.Create(outPath)
		if err != nil {
			return paths, err
		}
		encErr := png.Encode(f, rgba)
		closeErr := f.Close()
		if encErr != nil {
			return paths, encErr
		}
		if closeErr != nil {
			return paths, closeErr
		}
		paths = append(paths, outPath)
	}
	return paths, nil
}

func parsePSColorImageHeader(line string) (int, int, bool) {
	var w, h int
	var w2, h2, h3 int
	if _, err := fmt.Sscanf(line, "%d %d 8 [%d 0 0 -%d 0 %d]", &w, &h, &w2, &h2, &h3); err == nil {
		if w <= 0 || h <= 0 {
			return 0, 0, false
		}
		return w, h, true
	}
	if _, err := fmt.Sscanf(line, "%d %d 8 [1 0 0 -1 0 %d]", &w, &h, &h3); err == nil {
		if w <= 0 || h <= 0 {
			return 0, 0, false
		}
		return w, h, true
	}
	if _, err := fmt.Sscanf(line, "%d %d 8 [1 0 0 1 0 0]", &w, &h); err == nil {
		if w <= 0 || h <= 0 {
			return 0, 0, false
		}
		return w, h, true
	}
	return 0, 0, false
}

func ExtractNonASCIIShows(shows []PSShowOccurrence) []PSShowOccurrence {
	out := make([]PSShowOccurrence, 0, len(shows))
	for _, sh := range shows {
		if containsNonASCII(sh.Text) {
			out = append(out, sh)
		}
	}
	return out
}

func containsNonASCII(s string) bool {
	for _, r := range s {
		if r > 0x7F {
			return true
		}
	}
	return false
}

func FormatPSShowReport(shows []PSShowOccurrence, maxLines int) string {
	if maxLines <= 0 {
		maxLines = 2000
	}
	var b bytes.Buffer
	for i := 0; i < len(shows) && i < maxLines; i++ {
		sh := shows[i]
		fmt.Fprintf(&b, "page=%d line=%d hex=%t font=%s size=%.2f text=%q raw=%q\n", sh.PageNo, sh.LineNo, sh.RawIsHex, sh.FontName, sh.FontSize, sh.Text, strings.TrimSpace(sh.RawLine))
	}
	return b.String()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
