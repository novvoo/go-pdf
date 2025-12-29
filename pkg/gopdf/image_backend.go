package gopdf

import (
	"image"
	"image/color"
	"math"
)

// ImageBackend 图像后端
// 提供高性能的像素级操作
type ImageBackend struct {
	img    *image.RGBA
	width  int
	height int
}

// NewImageBackend 创建新的图像后端
func NewImageBackend(width, height int) *ImageBackend {
	return &ImageBackend{
		img:    image.NewRGBA(image.Rect(0, 0, width, height)),
		width:  width,
		height: height,
	}
}

// GetImage 获取图像
func (b *ImageBackend) GetImage() *image.RGBA {
	return b.img
}

// Clear 清空图像
func (b *ImageBackend) Clear(c color.Color) {
	r, g, bl, a := c.RGBA()
	fillColor := color.NRGBA{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(bl >> 8),
		A: uint8(a >> 8),
	}

	for y := 0; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			b.img.Set(x, y, fillColor)
		}
	}
}

// FillRect 填充矩形
func (b *ImageBackend) FillRect(x, y, width, height int, c color.Color) {
	for dy := 0; dy < height; dy++ {
		for dx := 0; dx < width; dx++ {
			px := x + dx
			py := y + dy
			if px >= 0 && py >= 0 && px < b.width && py < b.height {
				b.img.Set(px, py, c)
			}
		}
	}
}

// BlendPixel 混合单个像素
func (b *ImageBackend) BlendPixel(x, y int, c color.Color, op Operator) {
	if x < 0 || y < 0 || x >= b.width || y >= b.height {
		return
	}

	src := colorToNRGBA(c)
	dst := colorToNRGBA(b.img.At(x, y))
	result := PorterDuffBlend(src, dst, op)
	b.img.Set(x, y, result)
}

// DrawLine 绘制直线
func (b *ImageBackend) DrawLine(x0, y0, x1, y1 int, c color.Color, width float64) {
	// Bresenham 算法
	dx := abs(x1 - x0)
	dy := abs(y1 - y0)
	sx := -1
	if x0 < x1 {
		sx = 1
	}
	sy := -1
	if y0 < y1 {
		sy = 1
	}
	err := dx - dy

	for {
		b.drawThickPixel(x0, y0, c, width)

		if x0 == x1 && y0 == y1 {
			break
		}

		e2 := 2 * err
		if e2 > -dy {
			err -= dy
			x0 += sx
		}
		if e2 < dx {
			err += dx
			y0 += sy
		}
	}
}

// drawThickPixel 绘制粗像素
func (b *ImageBackend) drawThickPixel(x, y int, c color.Color, width float64) {
	halfWidth := int(math.Ceil(width / 2))
	for dy := -halfWidth; dy <= halfWidth; dy++ {
		for dx := -halfWidth; dx <= halfWidth; dx++ {
			px := x + dx
			py := y + dy
			if px >= 0 && py >= 0 && px < b.width && py < b.height {
				dist := math.Sqrt(float64(dx*dx + dy*dy))
				if dist <= width/2 {
					b.img.Set(px, py, c)
				}
			}
		}
	}
}

// colorToNRGBA 转换颜色为 NRGBA
func colorToNRGBA(c color.Color) color.NRGBA {
	if nrgba, ok := c.(color.NRGBA); ok {
		return nrgba
	}
	r, g, b, a := c.RGBA()
	return color.NRGBA{
		R: uint8(r >> 8),
		G: uint8(g >> 8),
		B: uint8(b >> 8),
		A: uint8(a >> 8),
	}
}

// SmoothBilinear 双线性插值平滑
func (b *ImageBackend) SmoothBilinear() {
	if b.width < 2 || b.height < 2 {
		return
	}

	// 创建临时缓冲区
	temp := image.NewRGBA(image.Rect(0, 0, b.width, b.height))

	// 对每个像素应用双线性插值
	for y := 1; y < b.height-1; y++ {
		for x := 1; x < b.width-1; x++ {
			// 获取周围9个像素
			var rSum, gSum, bSum, aSum uint32
			count := uint32(0)

			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					px := x + dx
					py := y + dy
					if px >= 0 && py >= 0 && px < b.width && py < b.height {
						c := b.img.At(px, py)
						r, g, bl, a := c.RGBA()
						rSum += r >> 8
						gSum += g >> 8
						bSum += bl >> 8
						aSum += a >> 8
						count++
					}
				}
			}

			// 计算平均值
			temp.Set(x, y, color.NRGBA{
				R: uint8(rSum / count),
				G: uint8(gSum / count),
				B: uint8(bSum / count),
				A: uint8(aSum / count),
			})
		}
	}

	// 复制边缘像素
	for x := 0; x < b.width; x++ {
		temp.Set(x, 0, b.img.At(x, 0))
		temp.Set(x, b.height-1, b.img.At(x, b.height-1))
	}
	for y := 0; y < b.height; y++ {
		temp.Set(0, y, b.img.At(0, y))
		temp.Set(b.width-1, y, b.img.At(b.width-1, y))
	}

	// 将结果复制回原图像
	b.img = temp
}

// SmoothGaussian 高斯模糊平滑
func (b *ImageBackend) SmoothGaussian(radius int) {
	if radius < 1 {
		radius = 1
	}
	if b.width < 2 || b.height < 2 {
		return
	}

	// 生成高斯核
	kernel := generateGaussianKernel(radius)
	kernelSize := len(kernel)
	halfSize := kernelSize / 2

	// 创建临时缓冲区
	temp := image.NewRGBA(image.Rect(0, 0, b.width, b.height))

	// 应用高斯模糊
	for y := 0; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			var rSum, gSum, bSum, aSum float64
			var weightSum float64

			for ky := 0; ky < kernelSize; ky++ {
				for kx := 0; kx < kernelSize; kx++ {
					px := x + kx - halfSize
					py := y + ky - halfSize

					if px >= 0 && py >= 0 && px < b.width && py < b.height {
						c := b.img.At(px, py)
						r, g, bl, a := c.RGBA()
						weight := kernel[ky][kx]

						rSum += float64(r>>8) * weight
						gSum += float64(g>>8) * weight
						bSum += float64(bl>>8) * weight
						aSum += float64(a>>8) * weight
						weightSum += weight
					}
				}
			}

			if weightSum > 0 {
				temp.Set(x, y, color.NRGBA{
					R: uint8(rSum / weightSum),
					G: uint8(gSum / weightSum),
					B: uint8(bSum / weightSum),
					A: uint8(aSum / weightSum),
				})
			} else {
				temp.Set(x, y, b.img.At(x, y))
			}
		}
	}

	b.img = temp
}

// SmoothMedian 中值滤波平滑（去噪效果好）
func (b *ImageBackend) SmoothMedian(windowSize int) {
	if windowSize < 3 {
		windowSize = 3
	}
	if windowSize%2 == 0 {
		windowSize++
	}
	if b.width < windowSize || b.height < windowSize {
		return
	}

	halfSize := windowSize / 2
	temp := image.NewRGBA(image.Rect(0, 0, b.width, b.height))

	for y := 0; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			var rValues, gValues, bValues, aValues []uint8

			for dy := -halfSize; dy <= halfSize; dy++ {
				for dx := -halfSize; dx <= halfSize; dx++ {
					px := x + dx
					py := y + dy
					if px >= 0 && py >= 0 && px < b.width && py < b.height {
						c := b.img.At(px, py)
						r, g, bl, a := c.RGBA()
						rValues = append(rValues, uint8(r>>8))
						gValues = append(gValues, uint8(g>>8))
						bValues = append(bValues, uint8(bl>>8))
						aValues = append(aValues, uint8(a>>8))
					}
				}
			}

			temp.Set(x, y, color.NRGBA{
				R: median(rValues),
				G: median(gValues),
				B: median(bValues),
				A: median(aValues),
			})
		}
	}

	b.img = temp
}

// generateGaussianKernel 生成高斯核
func generateGaussianKernel(radius int) [][]float64 {
	size := radius*2 + 1
	kernel := make([][]float64, size)
	sigma := float64(radius) / 2.0
	sum := 0.0

	for i := 0; i < size; i++ {
		kernel[i] = make([]float64, size)
		for j := 0; j < size; j++ {
			x := float64(i - radius)
			y := float64(j - radius)
			kernel[i][j] = math.Exp(-(x*x + y*y) / (2 * sigma * sigma))
			sum += kernel[i][j]
		}
	}

	// 归一化
	for i := 0; i < size; i++ {
		for j := 0; j < size; j++ {
			kernel[i][j] /= sum
		}
	}

	return kernel
}

// median 计算中值
func median(values []uint8) uint8 {
	if len(values) == 0 {
		return 0
	}

	// 简单排序
	sorted := make([]uint8, len(values))
	copy(sorted, values)
	for i := 0; i < len(sorted); i++ {
		for j := i + 1; j < len(sorted); j++ {
			if sorted[i] > sorted[j] {
				sorted[i], sorted[j] = sorted[j], sorted[i]
			}
		}
	}

	return sorted[len(sorted)/2]
}

// SmoothBilateral 双边滤波平滑（保持边缘）
// 双边滤波同时考虑空间距离和颜色差异，能够在平滑的同时保持边缘
// 参数：
//   - spatialSigma: 空间域标准差，控制空间权重（通常 3-5）
//   - colorSigma: 颜色域标准差，控制颜色相似度权重（通常 20-50）
func (b *ImageBackend) SmoothBilateral(spatialSigma, colorSigma float64) {
	if b.width < 2 || b.height < 2 {
		return
	}

	// 计算窗口大小（通常为 3*sigma）
	windowSize := int(math.Ceil(spatialSigma * 3))
	if windowSize < 1 {
		windowSize = 1
	}

	// 创建临时缓冲区
	temp := image.NewRGBA(image.Rect(0, 0, b.width, b.height))

	// 预计算空间权重
	spatialWeight := make([][]float64, windowSize*2+1)
	for i := range spatialWeight {
		spatialWeight[i] = make([]float64, windowSize*2+1)
		for j := range spatialWeight[i] {
			dx := float64(i - windowSize)
			dy := float64(j - windowSize)
			dist := math.Sqrt(dx*dx + dy*dy)
			spatialWeight[i][j] = math.Exp(-(dist * dist) / (2 * spatialSigma * spatialSigma))
		}
	}

	// 对每个像素应用双边滤波
	for y := 0; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			// 获取中心像素颜色
			centerColor := b.img.At(x, y)
			cr, cg, cb, _ := centerColor.RGBA()
			centerR := float64(cr >> 8)
			centerG := float64(cg >> 8)
			centerB := float64(cb >> 8)
			// 注意：我们只用 RGB 来计算颜色差异，不包括 alpha
			// 但 alpha 通道仍然会被平滑处理（通过 neighborA）

			var rSum, gSum, bSum, aSum float64
			var weightSum float64

			// 遍历窗口内的像素
			for dy := -windowSize; dy <= windowSize; dy++ {
				for dx := -windowSize; dx <= windowSize; dx++ {
					px := x + dx
					py := y + dy

					if px >= 0 && py >= 0 && px < b.width && py < b.height {
						// 获取邻域像素颜色
						neighborColor := b.img.At(px, py)
						nr, ng, nb, na := neighborColor.RGBA()
						neighborR := float64(nr >> 8)
						neighborG := float64(ng >> 8)
						neighborB := float64(nb >> 8)
						neighborA := float64(na >> 8)

						// 计算颜色差异（包括 RGB，不包括 alpha）
						colorDiff := math.Sqrt(
							(centerR-neighborR)*(centerR-neighborR) +
								(centerG-neighborG)*(centerG-neighborG) +
								(centerB-neighborB)*(centerB-neighborB))

						// 计算颜色权重（颜色相似度）
						colorWeight := math.Exp(-(colorDiff * colorDiff) / (2 * colorSigma * colorSigma))

						// 组合空间权重和颜色权重
						weight := spatialWeight[dy+windowSize][dx+windowSize] * colorWeight

						rSum += neighborR * weight
						gSum += neighborG * weight
						bSum += neighborB * weight
						aSum += neighborA * weight
						weightSum += weight
					}
				}
			}

			// 归一化并设置像素
			if weightSum > 0 {
				temp.Set(x, y, color.NRGBA{
					R: uint8(rSum / weightSum),
					G: uint8(gSum / weightSum),
					B: uint8(bSum / weightSum),
					A: uint8(aSum / weightSum),
				})
			} else {
				temp.Set(x, y, centerColor)
			}
		}
	}

	b.img = temp
}

// SmoothWithEdgeDetection 基于边缘检测的选择性平滑（使用高斯模糊）
// 流程：
// 1. 使用 Sobel 算子检测边缘
// 2. 创建边缘掩码（边缘=0，非边缘=1）
// 3. 对掩码进行高斯模糊实现羽化
// 4. 根据掩码选择性地应用高斯平滑
func (b *ImageBackend) SmoothWithEdgeDetection(smoothRadius int, edgeThreshold float64) {
	b.smoothWithEdgeDetectionInternal(smoothRadius, edgeThreshold, "gaussian")
}

// SmoothBilateralWithEdgeDetection 基于边缘检测的选择性双边滤波
// 结合边缘检测和双边滤波，提供最强的边缘保护
func (b *ImageBackend) SmoothBilateralWithEdgeDetection(spatialSigma, colorSigma, edgeThreshold float64) {
	if b.width < 3 || b.height < 3 {
		return
	}

	// 步骤1: 使用 Sobel 算子检测边缘
	edgeMask := b.detectEdgesSobel(edgeThreshold)

	// 步骤2: 对边缘掩码进行高斯模糊（羽化）
	featheredMask := b.featherMask(edgeMask, 2)

	// 步骤3: 对原图应用双边滤波
	tempBackend := &ImageBackend{
		img:    image.NewRGBA(b.img.Bounds()),
		width:  b.width,
		height: b.height,
	}
	copy(tempBackend.img.Pix, b.img.Pix)
	tempBackend.SmoothBilateral(spatialSigma, colorSigma)

	// 步骤4: 根据羽化后的掩码混合原图和平滑图
	smoothed := image.NewRGBA(image.Rect(0, 0, b.width, b.height))
	for y := 0; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			maskValue := featheredMask[y][x]
			origColor := b.img.At(x, y)
			smoothColor := tempBackend.img.At(x, y)

			or, og, ob, oa := origColor.RGBA()
			sr, sg, sb, sa := smoothColor.RGBA()

			r := uint8((float64(or>>8)*(1-maskValue) + float64(sr>>8)*maskValue))
			g := uint8((float64(og>>8)*(1-maskValue) + float64(sg>>8)*maskValue))
			bl := uint8((float64(ob>>8)*(1-maskValue) + float64(sb>>8)*maskValue))
			a := uint8((float64(oa>>8)*(1-maskValue) + float64(sa>>8)*maskValue))

			smoothed.Set(x, y, color.NRGBA{R: r, G: g, B: bl, A: a})
		}
	}

	b.img = smoothed
}

// smoothWithEdgeDetectionInternal 内部方法，支持不同的平滑算法
func (b *ImageBackend) smoothWithEdgeDetectionInternal(smoothRadius int, edgeThreshold float64, method string) {
	if b.width < 3 || b.height < 3 {
		return
	}

	// 步骤1: 使用 Sobel 算子检测边缘
	edgeMask := b.detectEdgesSobel(edgeThreshold)

	// 步骤2: 对边缘掩码进行高斯模糊（羽化）
	featheredMask := b.featherMask(edgeMask, 2)

	// 步骤3: 对原图应用平滑
	smoothed := image.NewRGBA(image.Rect(0, 0, b.width, b.height))
	tempBackend := &ImageBackend{
		img:    image.NewRGBA(b.img.Bounds()),
		width:  b.width,
		height: b.height,
	}
	copy(tempBackend.img.Pix, b.img.Pix)

	switch method {
	case "gaussian":
		tempBackend.SmoothGaussian(smoothRadius)
	case "median":
		tempBackend.SmoothMedian(smoothRadius)
	case "bilinear":
		tempBackend.SmoothBilinear()
	}

	// 步骤4: 根据羽化后的掩码混合原图和平滑图
	for y := 0; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			// 获取掩码值（0=边缘，1=非边缘）
			maskValue := featheredMask[y][x]

			// 获取原图和平滑图的颜色
			origColor := b.img.At(x, y)
			smoothColor := tempBackend.img.At(x, y)

			or, og, ob, oa := origColor.RGBA()
			sr, sg, sb, sa := smoothColor.RGBA()

			// 根据掩码混合：边缘保持原样，非边缘使用平滑
			r := uint8((float64(or>>8)*(1-maskValue) + float64(sr>>8)*maskValue))
			g := uint8((float64(og>>8)*(1-maskValue) + float64(sg>>8)*maskValue))
			bl := uint8((float64(ob>>8)*(1-maskValue) + float64(sb>>8)*maskValue))
			a := uint8((float64(oa>>8)*(1-maskValue) + float64(sa>>8)*maskValue))

			smoothed.Set(x, y, color.NRGBA{R: r, G: g, B: bl, A: a})
		}
	}

	b.img = smoothed
}

// detectEdgesSobel 使用 Sobel 算子检测边缘
// 返回边缘掩码：0=边缘，1=非边缘
func (b *ImageBackend) detectEdgesSobel(threshold float64) [][]float64 {
	// Sobel 算子
	sobelX := [][]int{
		{-1, 0, 1},
		{-2, 0, 2},
		{-1, 0, 1},
	}
	sobelY := [][]int{
		{-1, -2, -1},
		{0, 0, 0},
		{1, 2, 1},
	}

	// 计算梯度强度
	gradient := make([][]float64, b.height)
	maxGradient := 0.0

	for y := 0; y < b.height; y++ {
		gradient[y] = make([]float64, b.width)
		for x := 0; x < b.width; x++ {
			if x < 1 || x >= b.width-1 || y < 1 || y >= b.height-1 {
				gradient[y][x] = 0
				continue
			}

			var gx, gy float64

			// 应用 Sobel 算子
			for dy := -1; dy <= 1; dy++ {
				for dx := -1; dx <= 1; dx++ {
					c := b.img.At(x+dx, y+dy)
					r, g, bl, _ := c.RGBA()
					// 转换为灰度
					gray := float64(r>>8)*0.299 + float64(g>>8)*0.587 + float64(bl>>8)*0.114

					gx += gray * float64(sobelX[dy+1][dx+1])
					gy += gray * float64(sobelY[dy+1][dx+1])
				}
			}

			// 计算梯度强度
			magnitude := math.Sqrt(gx*gx + gy*gy)
			gradient[y][x] = magnitude

			if magnitude > maxGradient {
				maxGradient = magnitude
			}
		}
	}

	// 归一化并应用阈值
	edgeMask := make([][]float64, b.height)
	for y := 0; y < b.height; y++ {
		edgeMask[y] = make([]float64, b.width)
		for x := 0; x < b.width; x++ {
			normalizedGradient := gradient[y][x] / maxGradient
			if normalizedGradient > threshold {
				edgeMask[y][x] = 0.0 // 边缘，不平滑
			} else {
				edgeMask[y][x] = 1.0 // 非边缘，平滑
			}
		}
	}

	return edgeMask
}

// featherMask 对掩码进行高斯模糊实现羽化效果
func (b *ImageBackend) featherMask(mask [][]float64, radius int) [][]float64 {
	if radius < 1 {
		return mask
	}

	// 生成高斯核
	kernel := generateGaussianKernel(radius)
	kernelSize := len(kernel)
	halfSize := kernelSize / 2

	// 创建羽化后的掩码
	feathered := make([][]float64, b.height)
	for y := 0; y < b.height; y++ {
		feathered[y] = make([]float64, b.width)
		for x := 0; x < b.width; x++ {
			var sum, weightSum float64

			for ky := 0; ky < kernelSize; ky++ {
				for kx := 0; kx < kernelSize; kx++ {
					px := x + kx - halfSize
					py := y + ky - halfSize

					if px >= 0 && py >= 0 && px < b.width && py < b.height {
						weight := kernel[ky][kx]
						sum += mask[py][px] * weight
						weightSum += weight
					}
				}
			}

			if weightSum > 0 {
				feathered[y][x] = sum / weightSum
			} else {
				feathered[y][x] = mask[y][x]
			}
		}
	}

	return feathered
}

// SmoothAnisotropicDiffusion 各向异性扩散（Perona-Malik 算法）
// 这是一种经典的边缘保持平滑算法，通过控制扩散方向来保护边缘
// 参数：
//   - iterations: 迭代次数（通常 5-20）
//   - kappa: 扩散系数阈值（控制边缘敏感度，通常 10-50）
//   - lambda: 扩散速率（通常 0.1-0.25，值越大扩散越快）
func (b *ImageBackend) SmoothAnisotropicDiffusion(iterations int, kappa, lambda float64) {
	if b.width < 2 || b.height < 2 {
		return
	}

	// 转换为灰度进行梯度计算（但保留彩色输出）
	for iter := 0; iter < iterations; iter++ {
		temp := image.NewRGBA(image.Rect(0, 0, b.width, b.height))

		for y := 1; y < b.height-1; y++ {
			for x := 1; x < b.width-1; x++ {
				// 获取当前像素
				center := b.img.At(x, y)
				cr, cg, cb, ca := center.RGBA()

				// 计算四个方向的梯度和扩散系数
				// 北、南、东、西
				directions := []struct{ dx, dy int }{
					{0, -1}, // 北
					{0, 1},  // 南
					{1, 0},  // 东
					{-1, 0}, // 西
				}

				var rSum, gSum, bSum, aSum float64
				var coeffSum float64

				for _, dir := range directions {
					nx, ny := x+dir.dx, y+dir.dy
					if nx >= 0 && ny >= 0 && nx < b.width && ny < b.height {
						neighbor := b.img.At(nx, ny)
						nr, ng, nb, na := neighbor.RGBA()

						// 计算颜色梯度
						gradR := float64(nr>>8) - float64(cr>>8)
						gradG := float64(ng>>8) - float64(cg>>8)
						gradB := float64(nb>>8) - float64(cb>>8)
						gradMag := math.Sqrt(gradR*gradR + gradG*gradG + gradB*gradB)

						// 计算扩散系数（边缘处扩散系数小，平坦区域扩散系数大）
						// 使用 Perona-Malik 函数：c(x) = exp(-(grad/kappa)^2)
						coeff := math.Exp(-(gradMag * gradMag) / (kappa * kappa))

						rSum += coeff * gradR
						gSum += coeff * gradG
						bSum += coeff * gradB
						aSum += coeff * float64(na>>8-ca>>8)
						coeffSum += coeff
					}
				}

				// 更新像素值
				newR := float64(cr>>8) + lambda*rSum
				newG := float64(cg>>8) + lambda*gSum
				newB := float64(cb>>8) + lambda*bSum
				newA := float64(ca>>8) + lambda*aSum

				// 限制在 [0, 255] 范围内
				newR = math.Max(0, math.Min(255, newR))
				newG = math.Max(0, math.Min(255, newG))
				newB = math.Max(0, math.Min(255, newB))
				newA = math.Max(0, math.Min(255, newA))

				temp.Set(x, y, color.NRGBA{
					R: uint8(newR),
					G: uint8(newG),
					B: uint8(newB),
					A: uint8(newA),
				})
			}
		}

		// 复制边缘像素
		for x := 0; x < b.width; x++ {
			temp.Set(x, 0, b.img.At(x, 0))
			temp.Set(x, b.height-1, b.img.At(x, b.height-1))
		}
		for y := 0; y < b.height; y++ {
			temp.Set(0, y, b.img.At(0, y))
			temp.Set(b.width-1, y, b.img.At(b.width-1, y))
		}

		b.img = temp
	}
}

// SmoothGuidedFilter 引导滤波（快速边缘保持滤波）
// 这是一种快速的边缘保持滤波算法，计算效率高
// 参数：
//   - radius: 窗口半径
//   - epsilon: 正则化参数（控制边缘保持程度，值越小边缘保持越好）
func (b *ImageBackend) SmoothGuidedFilter(radius int, epsilon float64) {
	if b.width < 2 || b.height < 2 || radius < 1 {
		return
	}

	// 简化版引导滤波：使用图像自身作为引导图
	// 1. 计算均值
	meanI := b.boxFilter(b.img, radius)

	// 2. 计算方差
	temp := image.NewRGBA(image.Rect(0, 0, b.width, b.height))
	for y := 0; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			c := b.img.At(x, y)
			r, g, bl, a := c.RGBA()
			// I * I
			temp.Set(x, y, color.NRGBA{
				R: uint8((r >> 8) * (r >> 8) / 255),
				G: uint8((g >> 8) * (g >> 8) / 255),
				B: uint8((bl >> 8) * (bl >> 8) / 255),
				A: uint8(a >> 8),
			})
		}
	}
	meanII := b.boxFilter(temp, radius)

	// 3. 计算线性系数 a 和 b
	result := image.NewRGBA(image.Rect(0, 0, b.width, b.height))
	for y := 0; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			mI := meanI.At(x, y)
			mII := meanII.At(x, y)

			mir, mig, mib, _ := mI.RGBA()
			miir, miig, miib, _ := mII.RGBA()

			// var = E[I^2] - E[I]^2
			varR := float64(miir>>8) - float64(mir>>8)*float64(mir>>8)/255.0
			varG := float64(miig>>8) - float64(mig>>8)*float64(mig>>8)/255.0
			varB := float64(miib>>8) - float64(mib>>8)*float64(mib>>8)/255.0

			// a = var / (var + epsilon)
			aR := varR / (varR + epsilon)
			aG := varG / (varG + epsilon)
			aB := varB / (varB + epsilon)

			// b = (1 - a) * meanI
			bR := (1 - aR) * float64(mir>>8)
			bG := (1 - aG) * float64(mig>>8)
			bB := (1 - aB) * float64(mib>>8)

			// q = a * I + b
			origC := b.img.At(x, y)
			or, og, ob, oa := origC.RGBA()

			qR := aR*float64(or>>8) + bR
			qG := aG*float64(og>>8) + bG
			qB := aB*float64(ob>>8) + bB

			result.Set(x, y, color.NRGBA{
				R: uint8(math.Max(0, math.Min(255, qR))),
				G: uint8(math.Max(0, math.Min(255, qG))),
				B: uint8(math.Max(0, math.Min(255, qB))),
				A: uint8(oa >> 8),
			})
		}
	}

	b.img = result
}

// boxFilter 盒式滤波（快速均值滤波）
func (b *ImageBackend) boxFilter(img *image.RGBA, radius int) *image.RGBA {
	result := image.NewRGBA(image.Rect(0, 0, b.width, b.height))

	for y := 0; y < b.height; y++ {
		for x := 0; x < b.width; x++ {
			var rSum, gSum, bSum, aSum uint32
			var count uint32

			for dy := -radius; dy <= radius; dy++ {
				for dx := -radius; dx <= radius; dx++ {
					nx, ny := x+dx, y+dy
					if nx >= 0 && ny >= 0 && nx < b.width && ny < b.height {
						c := img.At(nx, ny)
						r, g, bl, a := c.RGBA()
						rSum += r >> 8
						gSum += g >> 8
						bSum += bl >> 8
						aSum += a >> 8
						count++
					}
				}
			}

			if count > 0 {
				result.Set(x, y, color.NRGBA{
					R: uint8(rSum / count),
					G: uint8(gSum / count),
					B: uint8(bSum / count),
					A: uint8(aSum / count),
				})
			}
		}
	}

	return result
}
