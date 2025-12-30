# go-pdf

Go PDF rendering library using Gopdf graphics.

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

- **PDF Rendering**: Render graphics to PDF using Gopdf
- **PNG Export**: Export rendered content as PNG images
- **Layer Merging**: Merge multiple image layers
- **Image to PDF**: Convert images to PDF format
- **High DPI Support**: Configurable DPI for high-quality output
- **Image Filters**: Support for FlateDecode, DCTDecode, ASCIIHexDecode, RunLengthDecode
- **Font Loader**: Cross-platform font search and management with CJK support
- **Rendering Comparison**: Tools to compare rendering quality with Poppler

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
    
)

func main() {
    // Create renderer
    renderer := gopdf.NewPDFRenderer(600, 400)
    renderer.SetDPI(150)

    // Render to PDF
    renderer.RenderToPDF("output.pdf", func(ctx gopdf.Context) {
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

#### RenderToPDF(outputPath string, drawFunc func(ctx gopdf.Context)) error
Renders graphics to a PDF file.

#### RenderToPNG(outputPath string, drawFunc func(ctx gopdf.Context)) error
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

- [go-pdf](https://github.com/novvoo/go-pdf) - Gopdf graphics bindings for Go
- [pdfcpu](https://github.com/pdfcpu/pdfcpu) - PDF processing library

## Recent Improvements

### Image Filters
- ✅ FlateDecode (zlib decompression)
- ✅ DCTDecode (JPEG decoding)
- ✅ ASCIIHexDecode
- ✅ RunLengthDecode
- ✅ PNG Predictor support

### Font Handling
- ✅ Cross-platform font search (Windows/macOS/Linux)
- ✅ Font substitution mechanism
- ✅ CJK font support
- ✅ Font fallback chains
- ✅ Font metrics caching

### Testing Tools
- ✅ Rendering comparison with Poppler
- ✅ PSNR/MSE quality metrics
- ✅ Difference image generation
- ✅ Batch comparison support


## Testing

```bash
# Run unit tests
cd test
go test -v

# Run rendering comparison (requires Poppler tools)
go run compare_rendering.go ../test.pdf

# Run benchmarks
go test -bench=. -benchmem
```

## License

MIT License
