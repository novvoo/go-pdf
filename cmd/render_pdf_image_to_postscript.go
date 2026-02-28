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
	inPath := flag.String("in", "example/test_image.pdf", "input pdf path")
	outPS := flag.String("out", "", "output ps path")
	mode := flag.String("mode", "xobject", "xobject|raster")
	encoding := flag.String("encoding", "hex", "raster encoding: hex|a85")
	outPDF := flag.String("pdf", "", "output pdf path (raster mode only)")
	dpi := flag.Float64("dpi", 144.0, "render dpi")
	inspectDir := flag.String("inspect", "", "inspection output dir")
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
	case "raster":
		enc := strings.ToLower(strings.TrimSpace(*encoding))
		switch enc {
		case "a85", "a85flate", "a85+flate", "flate":
			if err := reader.WritePostScriptA85Flate(*outPS, *dpi); err != nil {
				panic(err)
			}
		default:
			if err := reader.WritePostScript(*outPS, *dpi); err != nil {
				panic(err)
			}
		}

		if enc == "hex" || enc == "" {
			if *outPDF == "" {
				base := strings.TrimSuffix(*outPS, filepath.Ext(*outPS))
				*outPDF = base + ".ps.pdf"
			}
			if filepath.Clean(*outPDF) == filepath.Clean(*inPath) {
				base := strings.TrimSuffix(*outPS, filepath.Ext(*outPS))
				*outPDF = base + ".ps.pdf"
			}
			if err := gopdf.ConvertPostScriptRasterToPDF(*outPS, *outPDF); err != nil {
				panic(err)
			}
		}
	default:
		if err := reader.WritePostScriptVector(*outPS); err != nil {
			panic(err)
		}
	}

	psBytes, err := os.ReadFile(*outPS)
	if err != nil {
		panic(err)
	}
	psContent := string(psBytes)

	images, imgErr := gopdf.ExtractPostScriptColorImages(psContent, 0)
	var imagePaths []string
	if imgErr == nil && len(images) > 0 {
		imgDir := filepath.Join(*inspectDir, "pages")
		paths, err := gopdf.WritePSColorImagesToDir(images, imgDir, "page")
		if err != nil {
			panic(err)
		}
		imagePaths = paths
	}

	sumPath := filepath.Join(*inspectDir, "summary.txt")
	var b strings.Builder
	b.WriteString("PS Raster Inspect Summary\n")
	b.WriteString("=========================\n")
	b.WriteString(fmt.Sprintf("Time: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	b.WriteString(fmt.Sprintf("PDF: %s\n", *inPath))
	b.WriteString(fmt.Sprintf("PS: %s\n", *outPS))
	b.WriteString(fmt.Sprintf("DPI: %.2f\n\n", *dpi))
	b.WriteString("Images (pages as colorimage)\n")
	if imgErr != nil {
		b.WriteString(fmt.Sprintf("- extract error: %v\n", imgErr))
	} else {
		b.WriteString(fmt.Sprintf("- blocks=%d\n", len(images)))
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

	if err := os.WriteFile(sumPath, []byte(b.String()), 0644); err != nil {
		panic(err)
	}

	if strings.ToLower(strings.TrimSpace(*mode)) == "raster" {
		enc := strings.ToLower(strings.TrimSpace(*encoding))
		if enc == "hex" || enc == "" {
			fmt.Printf("OK\nPS: %s\nInspect: %s\nPDF: %s\n", *outPS, *inspectDir, *outPDF)
			return
		}
		fmt.Printf("OK\nPS: %s\nInspect: %s\n", *outPS, *inspectDir)
		return
	}
	fmt.Printf("OK\nPS: %s\nInspect: %s\n", *outPS, *inspectDir)
}
