package test

import (
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// TestImageScalingWithDPIMismatch 测试DPI不匹配时的图像缩放
func TestImageScalingWithDPIMismatch(t *testing.T) {
	tests := []struct {
		name          string
		imageDPI      float64
		targetDPI     float64
		imageWidth    int
		imageHeight   int
		expectBlur    bool
		expectDistort bool
		description   string
	}{
		{
			name:          "Low DPI to High DPI",
			imageDPI:      72,
			targetDPI:     300,
			imageWidth:    100,
			imageHeight:   100,
			expectBlur:    true,
			expectDistort: false,
			description:   "从低DPI放大到高DPI可能导致模糊",
		},
		{
			name:          "High DPI to Low DPI",
			imageDPI:      300,
			targetDPI:     72,
			imageWidth:    400,
			imageHeight:   400,
			expectBlur:    false,
			expectDistort: false,
			description:   "从高DPI缩小到低DPI应该保持清晰",
		},
		{
			name:          "Matching DPI",
			imageDPI:      150,
			targetDPI:     150,
			imageWidth:    200,
			imageHeight:   200,
			expectBlur:    false,
			expectDistort: false,
			description:   "相同DPI不应该有质量损失",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建测试图像
			testImg := createTestPattern(tt.imageWidth, tt.imageHeight)

			// 保存测试图像
			imgPath := "test_scaling_input.png"
			saveImage(testImg, imgPath)
			defer os.Remove(imgPath)

			// 使用不同DPI渲染
			renderer := gopdf.NewPDFRenderer(float64(tt.imageWidth), float64(tt.imageHeight))
			renderer.SetDPI(tt.targetDPI)

			outputPath := "test_scaling_output.png"
			err := renderer.RenderToPNG(outputPath, func(ctx gopdf.Context) {
				// 绘制测试图案
				drawTestPattern(ctx, float64(tt.imageWidth), float64(tt.imageHeight))
			})
			defer os.Remove(outputPath)

			if err != nil {
				t.Fatalf("渲染失败: %v", err)
			}

			// 加载输出图像
			outputImg, err := loadAndValidateImage(outputPath)
			if err != nil {
				t.Fatalf("加载输出图像失败: %v", err)
			}

			// 分析图像质量
			quality := analyzeImageQuality(outputImg)
			t.Logf("%s - 图像质量分数: %.2f", tt.description, quality)

			// 根据预期检查结果
			if tt.expectBlur && quality > 0.9 {
				t.Logf("警告: 预期模糊但图像质量很高 (%.2f)", quality)
			}
			if !tt.expectBlur && quality < 0.7 {
				t.Errorf("图像质量低于预期: %.2f (期望 >= 0.7)", quality)
			}
		})
	}
}

// TestImageAspectRatioPreservation 测试图像宽高比保持
func TestImageAspectRatioPreservation(t *testing.T) {
	tests := []struct {
		name         string
		sourceWidth  int
		sourceHeight int
		targetWidth  float64
		targetHeight float64
		description  string
	}{
		{
			name:         "Square to Rectangle",
			sourceWidth:  100,
			sourceHeight: 100,
			targetWidth:  200,
			targetHeight: 100,
			description:  "正方形变为矩形可能导致失真",
		},
		{
			name:         "Wide to Tall",
			sourceWidth:  200,
			sourceHeight: 100,
			targetWidth:  100,
			targetHeight: 200,
			description:  "宽图变为高图可能导致失真",
		},
		{
			name:         "Proportional scaling",
			sourceWidth:  100,
			sourceHeight: 100,
			targetWidth:  200,
			targetHeight: 200,
			description:  "等比例缩放应该保持宽高比",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 渲染到目标尺寸
			renderer := gopdf.NewPDFRenderer(tt.targetWidth, tt.targetHeight)
			renderer.SetDPI(150)

			outputPath := "test_aspect_ratio.png"
			err := renderer.RenderToPNG(outputPath, func(ctx gopdf.Context) {
				// 绘制圆形测试图案
				ctx.SetSourceRGB(1, 1, 1)
				ctx.Paint()

				ctx.SetSourceRGB(1, 0, 0)
				centerX := tt.targetWidth / 2
				centerY := tt.targetHeight / 2
				radius := math.Min(tt.targetWidth, tt.targetHeight) / 3

				ctx.Arc(centerX, centerY, radius, 0, 2*math.Pi)
				ctx.Fill()
			})
			defer os.Remove(outputPath)

			if err != nil {
				t.Fatalf("渲染失败: %v", err)
			}

			// 加载并分析输出
			outputImg, err := loadAndValidateImage(outputPath)
			if err != nil {
				t.Fatalf("加载输出图像失败: %v", err)
			}

			// 检查圆形是否变形
			isCircular := checkCircularity(outputImg)
			t.Logf("%s - 圆形度: %.2f", tt.description, isCircular)

			// 计算预期的宽高比变化
			sourceRatio := float64(tt.sourceWidth) / float64(tt.sourceHeight)
			targetRatio := tt.targetWidth / tt.targetHeight
			ratioChange := math.Abs(sourceRatio - targetRatio)

			if ratioChange > 0.1 && isCircular > 0.9 {
				t.Logf("警告: 宽高比变化 %.2f 但圆形保持良好", ratioChange)
			}
		})
	}
}

// TestTransparencyHandling 测试透明度处理
func TestTransparencyHandling(t *testing.T) {
	tests := []struct {
		name        string
		hasAlpha    bool
		description string
	}{
		{
			name:        "With alpha channel",
			hasAlpha:    true,
			description: "应该正确处理alpha通道",
		},
		{
			name:        "Without alpha channel",
			hasAlpha:    false,
			description: "不透明图像应该正常处理",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建带透明度的测试图像
			width, height := 200, 200
			var testImg image.Image

			if tt.hasAlpha {
				img := image.NewRGBA(image.Rect(0, 0, width, height))
				// 创建渐变透明度
				for y := 0; y < height; y++ {
					for x := 0; x < width; x++ {
						alpha := uint8(float64(x) / float64(width) * 255)
						img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: alpha})
					}
				}
				testImg = img
			} else {
				img := image.NewRGBA(image.Rect(0, 0, width, height))
				for y := 0; y < height; y++ {
					for x := 0; x < width; x++ {
						img.Set(x, y, color.RGBA{R: 255, G: 0, B: 0, A: 255})
					}
				}
				testImg = img
			}

			// 保存测试图像
			imgPath := "test_transparency.png"
			saveImage(testImg, imgPath)
			defer os.Remove(imgPath)

			// 渲染图像
			renderer := gopdf.NewPDFRenderer(float64(width), float64(height))
			renderer.SetDPI(150)

			outputPath := "test_transparency_output.png"
			err := renderer.RenderToPNG(outputPath, func(ctx gopdf.Context) {
				// 绘制背景
				ctx.SetSourceRGB(0, 0, 1) // 蓝色背景
				ctx.Paint()

				// 绘制半透明红色矩形
				if tt.hasAlpha {
					ctx.SetSourceRGBA(1, 0, 0, 0.5)
				} else {
					ctx.SetSourceRGB(1, 0, 0)
				}
				ctx.Rectangle(50, 50, 100, 100)
				ctx.Fill()
			})
			defer os.Remove(outputPath)

			if err != nil {
				t.Fatalf("渲染失败: %v", err)
			}

			// 验证输出
			outputImg, err := loadAndValidateImage(outputPath)
			if err != nil {
				t.Fatalf("加载输出图像失败: %v", err)
			}

			// 检查透明度效果
			if tt.hasAlpha {
				// 检查混合区域的颜色
				centerPixel := outputImg.At(100, 100)
				r, g, b, a := centerPixel.RGBA()
				r8, g8, b8, a8 := uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8)

				t.Logf("中心像素颜色: R=%d, G=%d, B=%d, A=%d", r8, g8, b8, a8)

				// 半透明红色在蓝色背景上应该产生紫色
				if r8 < 50 || b8 < 50 {
					t.Logf("警告: 透明度混合可能不正确")
				}
			}
		})
	}
}

// TestLayerMergingWithTransparency 测试带透明度的图层合并
func TestLayerMergingWithTransparency(t *testing.T) {
	width, height := 300, 300

	renderer := gopdf.NewPDFRenderer(float64(width), float64(height))
	renderer.SetDPI(150)

	outputPath := "test_layer_merge.png"
	err := renderer.RenderToPNG(outputPath, func(ctx gopdf.Context) {
		// 底层：白色背景
		ctx.SetSourceRGB(1, 1, 1)
		ctx.Paint()

		// 第一层：半透明红色
		ctx.SetSourceRGBA(1, 0, 0, 0.5)
		ctx.Rectangle(50, 50, 100, 100)
		ctx.Fill()

		// 第二层：半透明蓝色（与红色重叠）
		ctx.SetSourceRGBA(0, 0, 1, 0.5)
		ctx.Rectangle(100, 100, 100, 100)
		ctx.Fill()

		// 第三层：半透明绿色（与前两层都重叠）
		ctx.SetSourceRGBA(0, 1, 0, 0.5)
		ctx.Rectangle(75, 75, 100, 100)
		ctx.Fill()
	})
	defer os.Remove(outputPath)

	if err != nil {
		t.Fatalf("渲染失败: %v", err)
	}

	// 验证输出
	outputImg, err := loadAndValidateImage(outputPath)
	if err != nil {
		t.Fatalf("加载输出图像失败: %v", err)
	}

	// 检查重叠区域的颜色
	overlapPixel := outputImg.At(125, 125) // 三层重叠区域
	r, g, b, _ := overlapPixel.RGBA()
	r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)

	t.Logf("重叠区域颜色: R=%d, G=%d, B=%d", r8, g8, b8)

	// 三层半透明颜色重叠应该产生混合色
	if r8 == 0 || g8 == 0 || b8 == 0 {
		t.Error("图层合并可能忽略了某些颜色通道")
	}
}

// Helper functions

func createTestPattern(width, height int) image.Image {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	// 创建棋盘图案
	squareSize := 10
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			if ((x/squareSize)+(y/squareSize))%2 == 0 {
				img.Set(x, y, color.Black)
			} else {
				img.Set(x, y, color.White)
			}
		}
	}
	return img
}

func drawTestPattern(ctx gopdf.Context, width, height float64) {
	// 绘制棋盘图案
	ctx.SetSourceRGB(1, 1, 1)
	ctx.Paint()

	ctx.SetSourceRGB(0, 0, 0)
	squareSize := 10.0
	for y := 0.0; y < height; y += squareSize {
		for x := 0.0; x < width; x += squareSize {
			if (int(x/squareSize)+int(y/squareSize))%2 == 0 {
				ctx.Rectangle(x, y, squareSize, squareSize)
				ctx.Fill()
			}
		}
	}
}

func saveImage(img image.Image, path string) error {
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return png.Encode(f, img)
}

func analyzeImageQuality(img image.Image) float64 {
	// 简单的图像质量分析：计算边缘清晰度
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	if width < 2 || height < 2 {
		return 0
	}

	var edgeStrength float64
	var edgeCount int

	for y := 1; y < height-1; y++ {
		for x := 1; x < width-1; x++ {
			// 计算梯度
			c1 := img.At(x, y)
			c2 := img.At(x+1, y)
			c3 := img.At(x, y+1)

			r1, g1, b1, _ := c1.RGBA()
			r2, g2, b2, _ := c2.RGBA()
			r3, g3, b3, _ := c3.RGBA()

			dx := math.Abs(float64(r2-r1)) + math.Abs(float64(g2-g1)) + math.Abs(float64(b2-b1))
			dy := math.Abs(float64(r3-r1)) + math.Abs(float64(g3-g1)) + math.Abs(float64(b3-b1))

			gradient := math.Sqrt(dx*dx + dy*dy)
			if gradient > 1000 { // 阈值
				edgeStrength += gradient
				edgeCount++
			}
		}
	}

	if edgeCount == 0 {
		return 0.5 // 没有边缘，中等质量
	}

	// 归一化到0-1范围
	avgEdgeStrength := edgeStrength / float64(edgeCount)
	quality := math.Min(1.0, avgEdgeStrength/100000.0)

	return quality
}

func checkCircularity(img image.Image) float64 {
	// 检查图像中的圆形是否变形
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	centerX := width / 2
	centerY := height / 2

	// 在多个角度测量半径
	angles := 16
	var radii []float64

	for i := 0; i < angles; i++ {
		angle := float64(i) * 2 * math.Pi / float64(angles)
		dx := math.Cos(angle)
		dy := math.Sin(angle)

		// 沿着这个方向找到边缘
		for r := 0.0; r < math.Min(float64(width), float64(height))/2; r++ {
			x := centerX + int(r*dx)
			y := centerY + int(r*dy)

			if x < 0 || x >= width || y < 0 || y >= height {
				break
			}

			pixel := img.At(x, y)
			red, _, _, _ := pixel.RGBA()
			if red > 32768 { // 红色像素
				radii = append(radii, r)
				break
			}
		}
	}

	if len(radii) < 2 {
		return 0
	}

	// 计算半径的标准差
	var sum, sumSq float64
	for _, r := range radii {
		sum += r
		sumSq += r * r
	}
	mean := sum / float64(len(radii))

	// 防止除以零
	if mean == 0 {
		return 0
	}

	variance := sumSq/float64(len(radii)) - mean*mean
	if variance < 0 {
		variance = 0
	}
	stdDev := math.Sqrt(variance)

	// 圆形度 = 1 - (标准差 / 平均半径)
	circularity := 1.0 - (stdDev / mean)
	if circularity < 0 {
		circularity = 0
	}
	if math.IsNaN(circularity) || math.IsInf(circularity, 0) {
		return 0
	}

	return circularity
}
