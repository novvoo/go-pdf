//go:build gopdfcmd
// +build gopdfcmd

package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/pdfcpu/pdfcpu/pkg/api"
)

type pageReport struct {
	pageNo       int
	bbox         string
	pageSize     string
	colorImages  []imageReport
	lineStart    int
	lineEnd      int
	setPageLine  int
	pageBBoxLine int
}

type imageReport struct {
	lineNo            int
	width             int
	height            int
	decodeProc        string
	matrix            string
	concatTrail       []string
	hexDigits         int64
	expectedBytes     int64
	computedBytes     int64
	firstHexSample    string
	nonHexSeen        int
	endedAtLine       int
	dataLines         int
	allFFPrefixDigits int
	all00PrefixDigits int
}

var (
	rePage       = regexp.MustCompile(`^%%Page:\s*(\d+)\s+(\d+)\s*$`)
	reBBox       = regexp.MustCompile(`^%%PageBoundingBox:\s*(.*)$`)
	rePageSize   = regexp.MustCompile(`^<<\s*/PageSize\s*\[\s*([0-9.]+)\s+([0-9.]+)\s*\]\s*>>\s*setpagedevice\s*$`)
	reConcat     = regexp.MustCompile(`^\[(.+)\]\s+concat\s*$`)
	reColorImage = regexp.MustCompile(`^(\d+)\s+(\d+)\s+8\s+\[(.+)\]\s+\{\s*(.+)\s*\}\s+false\s+3\s+colorimage\s*$`)
)

func main() {
	pdfPath := flag.String("pdf", "", "optional: pdf file path to compare page count/dims")
	psPath := flag.String("ps", "", "ps file path")
	maxConcatTrail := flag.Int("trail", 12, "how many concat lines to keep")
	maxSampleHex := flag.Int("sample", 256, "max hex digits sample")
	flag.Parse()

	if strings.TrimSpace(*psPath) == "" {
		fmt.Fprintln(os.Stderr, "-ps is required")
		os.Exit(2)
	}

	if strings.TrimSpace(*pdfPath) != "" {
		count, err := api.PageCountFile(*pdfPath)
		if err != nil {
			fmt.Printf("PDF: %s\nPageCount: error: %v\n\n", *pdfPath, err)
		} else {
			fmt.Printf("PDF: %s\nPageCount: %d\n", *pdfPath, count)
			if dims, err := api.PageDimsFile(*pdfPath); err == nil {
				for i, d := range dims {
					fmt.Printf("  Page %d: %.0f x %.0f\n", i+1, d.Width, d.Height)
				}
			}
			fmt.Printf("\n")
		}
	}

	f, err := os.Open(*psPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer f.Close()

	r := bufio.NewReaderSize(f, 1<<20)
	lineNo := 0

	var pages []pageReport
	curPage := pageReport{pageNo: 0}
	curPageOpen := false
	concatTrail := make([]string, 0, *maxConcatTrail)

	inImage := false
	var curImg imageReport
	curImgPageIdx := -1

	flushPage := func() {
		if !curPageOpen {
			return
		}
		pages = append(pages, curPage)
		curPage = pageReport{pageNo: 0}
		curPageOpen = false
	}

	flushImage := func(finalLine int) {
		if !inImage {
			return
		}
		curImg.computedBytes = curImg.hexDigits / 2
		curImg.endedAtLine = finalLine
		if curImgPageIdx >= 0 && curImgPageIdx < len(pages) {
			pages[curImgPageIdx].colorImages = append(pages[curImgPageIdx].colorImages, curImg)
		} else if curImgPageIdx == len(pages) && curPageOpen {
			curPage.colorImages = append(curPage.colorImages, curImg)
		}
		inImage = false
		curImg = imageReport{}
		curImgPageIdx = -1
	}

	for {
		line, err := r.ReadString('\n')
		if len(line) == 0 && err != nil {
			break
		}
		lineNo++
		trim := strings.TrimRight(line, "\r\n")

		if inImage {
			if strings.TrimSpace(trim) == "grestore" {
				flushImage(lineNo)
				continue
			}
			curImg.dataLines++
			for i := 0; i < len(trim); i++ {
				b := trim[i]
				switch {
				case b >= '0' && b <= '9':
					curImg.hexDigits++
					if b == '0' {
						if curImg.firstHexSample != "" && len(curImg.firstHexSample) < *maxSampleHex {
							curImg.firstHexSample += string(b)
						}
						curImg.all00PrefixDigits++
					} else {
						curImg.all00PrefixDigits = 0
					}
				case b >= 'a' && b <= 'f':
					curImg.hexDigits++
					if curImg.firstHexSample != "" && len(curImg.firstHexSample) < *maxSampleHex {
						curImg.firstHexSample += string(b)
					}
					curImg.all00PrefixDigits = 0
					curImg.allFFPrefixDigits = 0
				case b >= 'A' && b <= 'F':
					curImg.hexDigits++
					if len(curImg.firstHexSample) < *maxSampleHex {
						curImg.firstHexSample += string(b)
					}
					if b == 'F' {
						curImg.allFFPrefixDigits++
					} else {
						curImg.allFFPrefixDigits = 0
					}
					curImg.all00PrefixDigits = 0
				case b == ' ' || b == '\t':
				default:
					curImg.nonHexSeen++
					curImg.allFFPrefixDigits = 0
					curImg.all00PrefixDigits = 0
				}
			}
			if curImg.firstHexSample == "" {
				curImg.firstHexSample = ""
			}
			continue
		}

		if m := rePage.FindStringSubmatch(trim); m != nil {
			flushPage()
			p, _ := strconv.Atoi(m[1])
			curPage = pageReport{pageNo: p, lineStart: lineNo}
			curPageOpen = true
			concatTrail = concatTrail[:0]
			continue
		}

		if !curPageOpen {
			continue
		}
		curPage.lineEnd = lineNo

		if m := reBBox.FindStringSubmatch(trim); m != nil {
			curPage.bbox = strings.TrimSpace(m[1])
			curPage.pageBBoxLine = lineNo
			continue
		}

		if m := rePageSize.FindStringSubmatch(trim); m != nil {
			curPage.pageSize = m[1] + " " + m[2]
			curPage.setPageLine = lineNo
			continue
		}

		if reConcat.MatchString(trim) {
			concatTrail = append(concatTrail, trim)
			if len(concatTrail) > *maxConcatTrail {
				copy(concatTrail, concatTrail[len(concatTrail)-*maxConcatTrail:])
				concatTrail = concatTrail[:*maxConcatTrail]
			}
			continue
		}

		if m := reColorImage.FindStringSubmatch(trim); m != nil {
			w, _ := strconv.Atoi(m[1])
			h, _ := strconv.Atoi(m[2])
			curImg = imageReport{
				lineNo:        lineNo,
				width:         w,
				height:        h,
				matrix:        strings.TrimSpace(m[3]),
				decodeProc:    strings.TrimSpace(m[4]),
				expectedBytes: int64(w) * int64(h) * 3,
				concatTrail:   append([]string(nil), concatTrail...),
			}
			inImage = true
			curImgPageIdx = len(pages)
			continue
		}
	}

	flushImage(lineNo)
	flushPage()

	report(pages)
}

func report(pages []pageReport) {
	fmt.Printf("PS Analysis\n")
	fmt.Printf("Pages: %d\n\n", len(pages))
	for _, p := range pages {
		fmt.Printf("Page %d (lines %d-%d)\n", p.pageNo, p.lineStart, p.lineEnd)
		if p.bbox != "" {
			fmt.Printf("  PageBoundingBox(line %d): %s\n", p.pageBBoxLine, p.bbox)
		} else {
			fmt.Printf("  PageBoundingBox: <missing>\n")
		}
		if p.pageSize != "" {
			fmt.Printf("  PageSize(line %d): %s\n", p.setPageLine, p.pageSize)
		} else {
			fmt.Printf("  PageSize: <missing>\n")
		}
		fmt.Printf("  colorimage blocks: %d\n", len(p.colorImages))
		for i, img := range p.colorImages {
			pct := 0.0
			if img.expectedBytes > 0 {
				pct = float64(img.computedBytes) / float64(img.expectedBytes) * 100
			}
			status := "OK"
			if img.computedBytes < img.expectedBytes {
				status = "SHORT"
			} else if img.computedBytes > img.expectedBytes {
				status = "LONG"
			}
			fmt.Printf("    #%d line %d: %dx%d matrix=[%s]\n", i+1, img.lineNo, img.width, img.height, img.matrix)
			fmt.Printf("       data lines=%d endLine=%d expected=%d bytes got=%d bytes (%.2f%%) %s\n", img.dataLines, img.endedAtLine, img.expectedBytes, img.computedBytes, pct, status)
			if img.nonHexSeen > 0 {
				fmt.Printf("       warning: non-hex chars seen inside data: %d\n", img.nonHexSeen)
			}
			if img.firstHexSample != "" {
				prefix := img.firstHexSample
				if len(prefix) > 64 {
					prefix = prefix[:64]
				}
				fmt.Printf("       hex sample: %s\n", prefix)
			}
			if len(img.concatTrail) > 0 {
				fmt.Printf("       concat trail (last %d):\n", len(img.concatTrail))
				for _, ln := range img.concatTrail {
					fmt.Printf("         %s\n", ln)
				}
			}
		}
		fmt.Printf("\n")
	}
}
