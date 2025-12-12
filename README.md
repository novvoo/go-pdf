# go-pdf

Go PDF rendering library using Cairo graphics.

## Project Structure

```
go-pdf/
├── pkg/
│   └── gopdf/          # Core library
│       ├── renderer.go # PDF rendering functionality
│       └── reader.go   # PDF reading functionality
├── cmd/                # Example programs
│   ├── example.go              # Basic usage example
│   ├── circle_comparison.go    # Circle drawing comparison
│   ├── render_demo.go          # Rendering demonstration
│   ├── render_pdf.go           # PDF rendering with layer merging
│   ├── render_pdf_complete.go  # Complete PDF rendering demo
│   └── merge_layers.go         # Layer merging utility
├── go.mod
├── go.sum
├── test.pdf            # Test PDF file
└── README.md
```

## Features

- **PDF Rendering**: Render graphics to PDF using Cairo
- **PNG Export**: Export rendered content as PNG images
- **Layer Merging**: Merge multiple image layers
- **Image to PDF**: Convert images to PDF format
- **High DPI Support**: Configurable DPI for high-quality output

## Installation

```bash
go get go-pdf/pkg/gopdf
```

## Usage

### Basic Example

```go
package main

import (
    "go-pdf/pkg/gopdf"
    "github.com/novvoo/go-cairo/pkg/cairo"
)

func main() {
    // Create renderer
    renderer := gopdf.NewPDFRenderer(600, 400)
    renderer.SetDPI(150)

    // Render to PDF
    renderer.RenderToPDF("output.pdf", func(ctx cairo.Context) {
        ctx.SetSourceRGB(0.2, 0.4, 0.8)
        ctx.Rectangle(50, 50, 200, 100)
        ctx.Fill()
    })
}
```

### Running Examples

```bash
# Basic example
go run cmd/example.go

# Render demo
go run cmd/render_demo.go

# Render PDF with layer merging
go run cmd/render_pdf.go

# Merge layers
go run cmd/merge_layers.go
```

## API Reference

### PDFRenderer

#### NewPDFRenderer(width, height float64) *PDFRenderer
Creates a new PDF renderer with specified dimensions (in points).

#### SetDPI(dpi float64)
Sets the rendering DPI (default: 72).

#### RenderToPDF(outputPath string, drawFunc func(ctx cairo.Context)) error
Renders graphics to a PDF file.

#### RenderToPNG(outputPath string, drawFunc func(ctx cairo.Context)) error
Renders graphics to a PNG file.

#### CreatePDFFromImage(imagePath, outputPath string) error
Creates a PDF from an image file.

### PDFReader

#### NewPDFReader(pdfPath string) *PDFReader
Creates a new PDF reader.

#### RenderPageToPNG(pageNum int, outputPath string, dpi float64) error
Renders a PDF page to PNG .

#### RenderPageToImage(pageNum int, dpi float64) (image.Image, error)
Renders a PDF page to an image.Image.

## Dependencies

- [go-cairo](https://github.com/novvoo/go-cairo) - Cairo graphics bindings for Go


## License

MIT License
