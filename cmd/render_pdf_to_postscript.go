//go:build gopdfcmd
// +build gopdfcmd

package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

func main() {
	inPath := flag.String("in", "example/test.pdf", "input pdf path")
	outPS := flag.String("out", "", "output ps path")
	mode := flag.String("mode", "best", "best|vector")
	dpi := flag.Float64("dpi", 144.0, "dpi for best-effort raster fallback")
	inspectDir := flag.String("inspect", "", "inspection output dir")
	maxShowLines := flag.Int("max-show", 2000, "max show lines to write")
	flag.Parse()

	if *outPS == "" {
		base := strings.TrimSuffix(*inPath, filepath.Ext(*inPath))
		*outPS = base + ".ps"
	}
	if *inspectDir == "" {
		base := strings.TrimSuffix(*outPS, filepath.Ext(*outPS))
		*inspectDir = base + "_inspect"
	}
	if err := os.MkdirAll(*inspectDir, 0755); err != nil {
		panic(err)
	}

	reader := gopdf.NewPDFReader(*inPath)
	defer reader.Close()

	switch strings.ToLower(strings.TrimSpace(*mode)) {
	case "vector":
		if err := reader.WritePostScriptVector(*outPS); err != nil {
			panic(err)
		}
	default:
		if err := reader.WritePostScriptBestEffort(*outPS, *dpi); err != nil {
			panic(err)
		}
	}

	psBytes, err := os.ReadFile(*outPS)
	if err != nil {
		panic(err)
	}
	psContent := string(psBytes)

	frag := gopdf.AnalyzePSShowFragmentation(psContent, 40)
	shows := gopdf.ExtractPostScriptShows(psContent)
	nonASCII := gopdf.ExtractNonASCIIShows(shows)
	images, imgErr := gopdf.ExtractPostScriptColorImages(psContent, 0)

	showAllPath := filepath.Join(*inspectDir, "ps_shows_all.txt")
	showNonASCIIPath := filepath.Join(*inspectDir, "ps_shows_nonascii.txt")
	if err := os.WriteFile(showAllPath, []byte(gopdf.FormatPSShowReport(shows, *maxShowLines)), 0644); err != nil {
		panic(err)
	}
	if err := os.WriteFile(showNonASCIIPath, []byte(gopdf.FormatPSShowReport(nonASCII, *maxShowLines)), 0644); err != nil {
		panic(err)
	}

	var imagePaths []string
	if imgErr == nil && len(images) > 0 {
		imgDir := filepath.Join(*inspectDir, "colorimage")
		paths, err := gopdf.WritePSColorImagesToDir(images, imgDir, "colorimage")
		if err != nil {
			panic(err)
		}
		imagePaths = paths
	}

	sumPath := filepath.Join(*inspectDir, "summary.txt")
	var b strings.Builder
	b.WriteString("PS Inspect Summary\n")
	b.WriteString("==================\n")
	b.WriteString(fmt.Sprintf("Time: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("PDF: %s\n", *inPath))
	b.WriteString(fmt.Sprintf("PS: %s\n\n", *outPS))
	b.WriteString("Text (show)\n")
	b.WriteString(fmt.Sprintf("- shows=%d literal=%d hex=%d\n", frag.TotalShows, frag.LiteralShows, frag.HexShows))
	b.WriteString(fmt.Sprintf("- short(len=1)=%d len=2=%d len=3=%d\n", frag.ShortShowsLen1, frag.ShortShowsLen2, frag.ShortShowsLen3))
	b.WriteString(fmt.Sprintf("- consecutive-letter-fragments=%d (body=%d math=%d)\n", frag.ConsecutiveLetters, frag.BodyConsecutiveLetters, frag.MathConsecutiveLetters))
	b.WriteString(fmt.Sprintf("- shows extracted=%d non-ascii=%d\n", len(shows), len(nonASCII)))
	b.WriteString(fmt.Sprintf("- report(all)=%s\n", showAllPath))
	b.WriteString(fmt.Sprintf("- report(non-ascii)=%s\n\n", showNonASCIIPath))
	b.WriteString("Images (colorimage)\n")
	if imgErr != nil {
		b.WriteString(fmt.Sprintf("- extract error: %v\n", imgErr))
	} else {
		b.WriteString(fmt.Sprintf("- blocks=%d\n", len(images)))
		if len(imagePaths) > 0 {
			b.WriteString(fmt.Sprintf("- exported=%d\n", len(imagePaths)))
			maxList := len(imagePaths)
			if maxList > 20 {
				maxList = 20
			}
			for i := 0; i < maxList; i++ {
				b.WriteString(fmt.Sprintf("  - %s\n", imagePaths[i]))
			}
			if len(imagePaths) > maxList {
				b.WriteString(fmt.Sprintf("  ... omitted %d paths\n", len(imagePaths)-maxList))
			}
		}
	}

	if err := os.WriteFile(sumPath, []byte(b.String()), 0644); err != nil {
		panic(err)
	}

	fmt.Printf("OK\nPS: %s\nInspect: %s\n", *outPS, *inspectDir)
}
