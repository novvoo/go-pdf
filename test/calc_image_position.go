//go:build ignore
// +build ignore

package main

import (
	"fmt"
)

func main() {
	// 页面初始变换矩阵
	// [0.24 0.00 0.00 -0.24 0.00 841.92]
	pageMatrix := []float64{0.24, 0.00, 0.00, -0.24, 0.00, 841.92}
	
	// 图像变换矩阵（在 Do 之前）
	// [2177.0835 0 0 -725.00006 154.166687 1547.91553]
	imgMatrix := []float64{2177.0835, 0, 0, -725.00006, 154.166687, 1547.91553}
	
	fmt.Println("Calculating image position...")
	fmt.Println("=============================\n")
	
	// 图像在 PDF 用户空间中的四个角点 (0,0), (1,0), (1,1), (0,1)
	corners := []struct{
		x, y float64
		name string
	}{
		{0, 0, "Bottom-left"},
		{1, 0, "Bottom-right"},
		{1, 1, "Top-right"},
		{0, 1, "Top-left"},
	}
	
	fmt.Println("Image corners in PDF user space (after image cm):")
	for _, c := range corners {
		// 应用图像变换矩阵
		x := imgMatrix[0]*c.x + imgMatrix[2]*c.y + imgMatrix[4]
		y := imgMatrix[1]*c.x + imgMatrix[3]*c.y + imgMatrix[5]
		fmt.Printf("  %s (%.1f, %.1f) -> PDF (%.2f, %.2f)\n", c.name, c.x, c.y, x, y)
	}
	
	fmt.Println("\nImage corners in Cairo device space (after page cm):")
	for _, c := range corners {
		// 应用图像变换矩阵
		x1 := imgMatrix[0]*c.x + imgMatrix[2]*c.y + imgMatrix[4]
		y1 := imgMatrix[1]*c.x + imgMatrix[3]*c.y + imgMatrix[5]
		
		// 应用页面变换矩阵
		x2 := pageMatrix[0]*x1 + pageMatrix[2]*y1 + pageMatrix[4]
		y2 := pageMatrix[1]*x1 + pageMatrix[3]*y1 + pageMatrix[5]
		
		fmt.Printf("  %s -> Cairo (%.2f, %.2f)\n", c.name, x2, y2)
	}
	
	// 计算图像区域的边界
	fmt.Println("\nImage bounding box in Cairo coordinates:")
	
	// Bottom-left (0,0)
	x1 := imgMatrix[4]
	y1 := imgMatrix[5]
	cx1 := pageMatrix[0]*x1 + pageMatrix[2]*y1 + pageMatrix[4]
	cy1 := pageMatrix[1]*x1 + pageMatrix[3]*y1 + pageMatrix[5]
	
	// Top-right (1,1)
	x2 := imgMatrix[0] + imgMatrix[2] + imgMatrix[4]
	y2 := imgMatrix[1] + imgMatrix[3] + imgMatrix[5]
	cx2 := pageMatrix[0]*x2 + pageMatrix[2]*y2 + pageMatrix[4]
	cy2 := pageMatrix[1]*x2 + pageMatrix[3]*y2 + pageMatrix[5]
	
	fmt.Printf("  X: %.2f to %.2f (width: %.2f)\n", cx1, cx2, cx2-cx1)
	fmt.Printf("  Y: %.2f to %.2f (height: %.2f)\n", cy2, cy1, cy1-cy2)
}
