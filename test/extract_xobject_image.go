//go:build ignore
// +build ignore

package main

import (
	"fmt"
	"image/png"
	"os"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

func main() {
	pdfPath := "test/test.pdf"
	outputPath := "test/xobject_image.png"

	fmt.Println("Extracting XObject image from PDF...")

	ctx, err := gopdf.ReadContextFile(pdfPath)
	if err != nil {
		fmt.Printf("Error reading PDF: %v\n", err)
		return
	}

	// 获取第一页
	pageDict, _, _, err := ctx.PageDict(1, false)
	if err != nil {
		fmt.Printf("Error getting page: %v\n", err)
		return
	}

	// 加载资源
	resources := gopdf.NewResources()
	if resourcesObj, found := pageDict.Find("Resources"); found {
		if err := gopdf.LoadResourcesPublic(ctx, resourcesObj, resources); err != nil {
			fmt.Printf("Error loading resources: %v\n", err)
			return
		}
	}

	// 获取 XObject
	xobjects := resources.GetAllXObjects()
	if len(xobjects) == 0 {
		fmt.Println("No XObjects found")
		return
	}

	fmt.Printf("Found %d XObject(s)\n", len(xobjects))

	// 找到第一个图片 XObject
	var imgXObj *gopdf.XObject
	for _, xobj := range xobjects {
		if xobj.Subtype == "Image" || xobj.Subtype == "/Image" {
			imgXObj = xobj
			break
		}
	}

	if imgXObj == nil {
		fmt.Println("No image XObject found")
		return
	}

	fmt.Printf("Image XObject: %dx%d, ColorSpace=%s, BPC=%d, Stream=%d bytes\n",
		imgXObj.Width, imgXObj.Height, imgXObj.ColorSpace, imgXObj.BitsPerComponent, len(imgXObj.Stream))

	// 手动解码图片
	width := imgXObj.Width
	height := imgXObj.Height
	
	if len(imgXObj.Stream) == 0 {
		fmt.Println("Error: No image stream data")
		return
	}

	// 创建图片
	img := gopdf.DecodeImageXObjectPublic(imgXObj)
	if img == nil {
		fmt.Println("Error: Failed to decode image")
		return
	}

	// 保存图片
	outFile, err := os.Create(outputPath)
	if err != nil {
		fmt.Printf("Error creating output file: %v\n", err)
		return
	}
	defer outFile.Close()

	err = png.Encode(outFile, img)
	if err != nil {
		fmt.Printf("Error encoding PNG: %v\n", err)
		return
	}

	fmt.Printf("✓ Image saved to: %s\n", outputPath)
	
	// 采样几个像素来验证颜色
	fmt.Println("\nSample pixels:")
	for i := 0; i < 5; i++ {
		x := i * 100
		y := i * 50
		if x < width && y < height {
			r, g, b, a := img.At(x, y).RGBA()
			fmt.Printf("  Pixel (%d,%d): R=%d G=%d B=%d A=%d\n", 
				x, y, uint8(r>>8), uint8(g>>8), uint8(b>>8), uint8(a>>8))
		}
	}
}
