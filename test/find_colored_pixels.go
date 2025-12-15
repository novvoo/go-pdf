//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"image/png"
	"os"
)

func main() {
	// 打开PNG文件
	file, err := os.Open("test/test.png")
	if err != nil {
		fmt.Printf("Failed to open PNG: %v\n", err)
		return
	}
	defer file.Close()

	// 解码PNG
	img, err := png.Decode(file)
	if err != nil {
		fmt.Printf("Failed to decode PNG: %v\n", err)
		return
	}

	bounds := img.Bounds()
	fmt.Printf("Image size: %dx%d\n\n", bounds.Dx(), bounds.Dy())

	// 查找绿色像素的位置（专门分析绿色区域）
	fmt.Println("Looking for green pixels (G > R and G > B):")

	var greenPixels []struct {
		x, y    int
		r, g, b uint8
	}
	minX, minY := bounds.Max.X, bounds.Max.Y
	maxX, maxY := bounds.Min.X, bounds.Min.Y

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			r8, g8, b8 := uint8(r>>8), uint8(g>>8), uint8(b>>8)

			// 检查是否是绿色（G > R 且 G > B）
			if g8 > r8 && g8 > b8 && g8 > 100 {
				greenPixels = append(greenPixels, struct {
					x, y    int
					r, g, b uint8
				}{x, y, r8, g8, b8})
				if x < minX {
					minX = x
				}
				if x > maxX {
					maxX = x
				}
				if y < minY {
					minY = y
				}
				if y > maxY {
					maxY = y
				}
			}
		}
	}

	fmt.Printf("Found %d green pixels\n\n", len(greenPixels))

	if len(greenPixels) > 0 {
		fmt.Printf("Green rectangle bounds:\n")
		fmt.Printf("  X: %d to %d (width: %d)\n", minX, maxX, maxX-minX+1)
		fmt.Printf("  Y: %d to %d (height: %d)\n\n", minY, maxY, maxY-minY+1)

		fmt.Println("Sample green pixels (first 20):")
		for i, p := range greenPixels {
			if i >= 20 {
				break
			}
			fmt.Printf("  (%d, %d): RGB(%d,%d,%d)\n", p.x, p.y, p.r, p.g, p.b)
		}
	}
}
