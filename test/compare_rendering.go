package test

import (
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// RenderingComparison 渲染对比结果
type RenderingComparison struct {
	PDFPath         string
	GoPDFImage      string
	PopplerImage    string
	DiffImage       string
	PSNR            float64 // 峰值信噪比
	MSE             float64 // 均方误差
	PixelDiff       int     // 不同像素数
	TotalPixels     int     // 总像素数
	DifferenceRatio float64 // 差异比例
}

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: go run compare_rendering.go <pdf_file>")
		os.Exit(1)
	}

	pdfPath := os.Args[1]
	comparison, err := CompareRendering(pdfPath, 150)
	if err != nil {
		fmt.Printf("Error: %v\n", err)
		os.Exit(1)
	}

	PrintComparisonReport(comparison)
}

// CompareRendering 对比 go-pdf 和 Poppler 的渲染结果
func CompareRendering(pdfPath string, dpi float64) (*RenderingComparison, error) {
	baseName := filepath.Base(pdfPath)
	baseName = baseName[:len(baseName)-len(filepath.Ext(baseName))]

	// 1. 使用 go-pdf 渲染
	goPDFOutput := fmt.Sprintf("%s_gopdf.png", baseName)
	fmt.Printf("Rendering with go-pdf to %s...\n", goPDFOutput)
	if err := renderWithGoPDF(pdfPath, goPDFOutput, dpi); err != nil {
		return nil, fmt.Errorf("go-pdf rendering failed: %w", err)
	}

	// 2. 使用 Poppler 渲染
	popplerOutput := fmt.Sprintf("%s_poppler.png", baseName)
	fmt.Printf("Rendering with Poppler to %s...\n", popplerOutput)
	if err := renderWithPoppler(pdfPath, popplerOutput, dpi); err != nil {
		return nil, fmt.Errorf("poppler rendering failed: %w", err)
	}

	// 3. 比较图像
	fmt.Println("Comparing images...")
	diffOutput := fmt.Sprintf("%s_diff.png", baseName)
	comparison, err := compareImages(goPDFOutput, popplerOutput, diffOutput)
	if err != nil {
		return nil, fmt.Errorf("image comparison failed: %w", err)
	}

	comparison.PDFPath = pdfPath
	comparison.GoPDFImage = goPDFOutput
	comparison.PopplerImage = popplerOutput
	comparison.DiffImage = diffOutput

	return comparison, nil
}

// renderWithGoPDF 使用 go-pdf 渲染 PDF
func renderWithGoPDF(pdfPath, outputPath string, dpi float64) error {
	reader := gopdf.NewPDFReader(pdfPath)
	if reader == nil {
		return fmt.Errorf("failed to create PDF reader")
	}

	return reader.RenderPageToPNG(1, outputPath, dpi)
}

// renderWithPoppler 使用 Poppler 渲染 PDF
func renderWithPoppler(pdfPath, outputPath string, dpi float64) error {
	// 使用 pdftocairo 命令
	cmd := exec.Command("pdftocairo",
		"-png",
		"-singlefile",
		"-r", fmt.Sprintf("%.0f", dpi),
		pdfPath,
		outputPath[:len(outputPath)-4], // 移除 .png 扩展名
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("pdftocairo failed: %w\nOutput: %s", err, string(output))
	}

	return nil
}

// compareImages 比较两个图像并生成差异图
func compareImages(img1Path, img2Path, diffPath string) (*RenderingComparison, error) {
	// 读取图像
	img1, err := loadImage(img1Path)
	if err != nil {
		return nil, fmt.Errorf("failed to load %s: %w", img1Path, err)
	}

	img2, err := loadImage(img2Path)
	if err != nil {
		return nil, fmt.Errorf("failed to load %s: %w", img2Path, err)
	}

	// 确保图像尺寸相同
	bounds1 := img1.Bounds()
	bounds2 := img2.Bounds()

	if bounds1.Dx() != bounds2.Dx() || bounds1.Dy() != bounds2.Dy() {
		return nil, fmt.Errorf("image dimensions don't match: %dx%d vs %dx%d",
			bounds1.Dx(), bounds1.Dy(), bounds2.Dx(), bounds2.Dy())
	}

	width := bounds1.Dx()
	height := bounds1.Dy()
	totalPixels := width * height

	// 创建差异图像
	diffImg := image.NewRGBA(image.Rect(0, 0, width, height))

	var mse float64
	pixelDiff := 0

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r1, g1, b1, _ := img1.At(x, y).RGBA()
			r2, g2, b2, _ := img2.At(x, y).RGBA()

			// 转换为 0-255 范围
			r1 = r1 >> 8
			g1 = g1 >> 8
			b1 = b1 >> 8
			r2 = r2 >> 8
			g2 = g2 >> 8
			b2 = b2 >> 8

			// 计算差异
			dr := float64(r1) - float64(r2)
			dg := float64(g1) - float64(g2)
			db := float64(b1) - float64(b2)

			// 累加均方误差
			mse += dr*dr + dg*dg + db*db

			// 计算差异强度
			diff := math.Sqrt(dr*dr + dg*dg + db*db)

			if diff > 10 { // 阈值:差异大于 10
				pixelDiff++
				// 差异像素用红色标记,强度表示差异大小
				intensity := uint8(math.Min(diff*2, 255))
				diffImg.Set(x, y, color.RGBA{R: intensity, G: 0, B: 0, A: 255})
			} else {
				// 相同像素用灰度显示
				gray := uint8((r1 + g1 + b1) / 3)
				diffImg.Set(x, y, color.RGBA{R: gray, G: gray, B: gray, A: 255})
			}
		}
	}

	// 保存差异图像
	if err := saveImage(diffImg, diffPath); err != nil {
		return nil, fmt.Errorf("failed to save diff image: %w", err)
	}

	// 计算指标
	mse = mse / float64(totalPixels*3) // 除以总像素数和颜色通道数
	psnr := 10 * math.Log10(255*255/mse)
	diffRatio := float64(pixelDiff) / float64(totalPixels)

	return &RenderingComparison{
		PSNR:            psnr,
		MSE:             mse,
		PixelDiff:       pixelDiff,
		TotalPixels:     totalPixels,
		DifferenceRatio: diffRatio,
	}, nil
}

// loadImage 加载图像文件
func loadImage(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	img, _, err := image.Decode(file)
	return img, err
}

// saveImage 保存图像文件
func saveImage(img image.Image, path string) error {
	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	return png.Encode(file, img)
}

// PrintComparisonReport 打印对比报告
func PrintComparisonReport(comp *RenderingComparison) {
	fmt.Println("\n" + strings.Repeat("=", 60))
	fmt.Println("Rendering Comparison Report")
	fmt.Println(strings.Repeat("=", 60))
	fmt.Printf("PDF File:        %s\n", comp.PDFPath)
	fmt.Printf("go-pdf Output:   %s\n", comp.GoPDFImage)
	fmt.Printf("Poppler Output:  %s\n", comp.PopplerImage)
	fmt.Printf("Diff Image:      %s\n", comp.DiffImage)
	fmt.Println(strings.Repeat("-", 60))
	fmt.Printf("Total Pixels:    %d\n", comp.TotalPixels)
	fmt.Printf("Different Pixels: %d (%.2f%%)\n",
		comp.PixelDiff, comp.DifferenceRatio*100)
	fmt.Printf("MSE:             %.2f\n", comp.MSE)
	fmt.Printf("PSNR:            %.2f dB\n", comp.PSNR)
	fmt.Println(strings.Repeat("-", 60))

	// 评估渲染质量
	var quality string
	if comp.PSNR > 40 {
		quality = "Excellent (almost identical)"
	} else if comp.PSNR > 30 {
		quality = "Good (minor differences)"
	} else if comp.PSNR > 20 {
		quality = "Fair (noticeable differences)"
	} else {
		quality = "Poor (significant differences)"
	}
	fmt.Printf("Quality:         %s\n", quality)
	fmt.Println(strings.Repeat("=", 60))
}

// BatchCompare 批量对比多个 PDF 文件
func BatchCompare(pdfFiles []string, dpi float64) ([]*RenderingComparison, error) {
	results := make([]*RenderingComparison, 0, len(pdfFiles))

	for i, pdfPath := range pdfFiles {
		fmt.Printf("\n[%d/%d] Processing %s...\n", i+1, len(pdfFiles), pdfPath)

		comparison, err := CompareRendering(pdfPath, dpi)
		if err != nil {
			fmt.Printf("Error processing %s: %v\n", pdfPath, err)
			continue
		}

		results = append(results, comparison)
		PrintComparisonReport(comparison)
	}

	return results, nil
}

// GenerateSummaryReport 生成汇总报告
func GenerateSummaryReport(results []*RenderingComparison, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer file.Close()

	fmt.Fprintf(file, "# Rendering Comparison Summary\n\n")
	fmt.Fprintf(file, "Total PDFs: %d\n\n", len(results))

	fmt.Fprintf(file, "| PDF File | Different Pixels | Diff %% | PSNR (dB) | Quality |\n")
	fmt.Fprintf(file, "|----------|------------------|---------|-----------|----------|\n")

	var totalPSNR float64
	var totalDiffRatio float64

	for _, comp := range results {
		quality := "Poor"
		if comp.PSNR > 40 {
			quality = "Excellent"
		} else if comp.PSNR > 30 {
			quality = "Good"
		} else if comp.PSNR > 20 {
			quality = "Fair"
		}

		fmt.Fprintf(file, "| %s | %d | %.2f%% | %.2f | %s |\n",
			filepath.Base(comp.PDFPath),
			comp.PixelDiff,
			comp.DifferenceRatio*100,
			comp.PSNR,
			quality)

		totalPSNR += comp.PSNR
		totalDiffRatio += comp.DifferenceRatio
	}

	if len(results) > 0 {
		avgPSNR := totalPSNR / float64(len(results))
		avgDiffRatio := totalDiffRatio / float64(len(results))

		fmt.Fprintf(file, "\n## Average Metrics\n\n")
		fmt.Fprintf(file, "- Average PSNR: %.2f dB\n", avgPSNR)
		fmt.Fprintf(file, "- Average Difference: %.2f%%\n", avgDiffRatio*100)
	}

	return nil
}
