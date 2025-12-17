package gopdf

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"image"
	"image/jpeg"
	"io"
)

// ImageFilter 图像滤镜接口
type ImageFilter interface {
	Decode(data []byte) ([]byte, error)
	Name() string
}

// DecodeImageWithFilters 使用滤镜链解码图像数据
func DecodeImageWithFilters(data []byte, filters []string) ([]byte, error) {
	result := data

	for _, filterName := range filters {
		filter := GetImageFilter(filterName)
		if filter == nil {
			return nil, fmt.Errorf("unsupported filter: %s", filterName)
		}

		decoded, err := filter.Decode(result)
		if err != nil {
			return nil, fmt.Errorf("filter %s failed: %w", filterName, err)
		}
		result = decoded
	}

	return result, nil
}

// GetImageFilter 根据名称获取滤镜
func GetImageFilter(name string) ImageFilter {
	switch name {
	case "FlateDecode", "/FlateDecode":
		return &FlateDecodeFilter{}
	case "DCTDecode", "/DCTDecode":
		return &DCTDecodeFilter{}
	case "LZWDecode", "/LZWDecode":
		return &LZWDecodeFilter{}
	case "ASCII85Decode", "/ASCII85Decode":
		return &ASCII85DecodeFilter{}
	case "ASCIIHexDecode", "/ASCIIHexDecode":
		return &ASCIIHexDecodeFilter{}
	case "RunLengthDecode", "/RunLengthDecode":
		return &RunLengthDecodeFilter{}
	default:
		return nil
	}
}

// FlateDecodeFilter zlib/deflate 解压滤镜
type FlateDecodeFilter struct{}

func (f *FlateDecodeFilter) Name() string { return "FlateDecode" }

func (f *FlateDecodeFilter) Decode(data []byte) ([]byte, error) {
	reader, err := zlib.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to create zlib reader: %w", err)
	}
	defer reader.Close()

	var buf bytes.Buffer
	if _, err := io.Copy(&buf, reader); err != nil {
		return nil, fmt.Errorf("failed to decompress: %w", err)
	}

	return buf.Bytes(), nil
}

// DCTDecodeFilter JPEG 解码滤镜
type DCTDecodeFilter struct{}

func (f *DCTDecodeFilter) Name() string { return "DCTDecode" }

func (f *DCTDecodeFilter) Decode(data []byte) ([]byte, error) {
	// JPEG 数据通常不需要进一步解码
	// 可以直接传递给 image.Decode
	img, err := jpeg.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("failed to decode JPEG: %w", err)
	}

	// 转换为原始 RGB 数据
	bounds := img.Bounds()
	width := bounds.Dx()
	height := bounds.Dy()

	result := make([]byte, width*height*3)
	offset := 0

	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			r, g, b, _ := img.At(x, y).RGBA()
			result[offset] = uint8(r >> 8)
			result[offset+1] = uint8(g >> 8)
			result[offset+2] = uint8(b >> 8)
			offset += 3
		}
	}

	return result, nil
}

// LZWDecodeFilter LZW 解压滤镜
type LZWDecodeFilter struct{}

func (f *LZWDecodeFilter) Name() string { return "LZWDecode" }

func (f *LZWDecodeFilter) Decode(data []byte) ([]byte, error) {
	// TODO: 实现 LZW 解压
	// 可以使用 compress/lzw 包
	return nil, fmt.Errorf("LZW decode not implemented yet")
}

// ASCII85DecodeFilter ASCII85 解码滤镜
type ASCII85DecodeFilter struct{}

func (f *ASCII85DecodeFilter) Name() string { return "ASCII85Decode" }

func (f *ASCII85DecodeFilter) Decode(data []byte) ([]byte, error) {
	// TODO: 实现 ASCII85 解码
	return nil, fmt.Errorf("ASCII85 decode not implemented yet")
}

// ASCIIHexDecodeFilter ASCII Hex 解码滤镜
type ASCIIHexDecodeFilter struct{}

func (f *ASCIIHexDecodeFilter) Name() string { return "ASCIIHexDecode" }

func (f *ASCIIHexDecodeFilter) Decode(data []byte) ([]byte, error) {
	var result []byte
	var currentByte byte
	highNibble := true

	for _, b := range data {
		// 跳过空白字符
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			continue
		}

		// 结束标记
		if b == '>' {
			break
		}

		// 转换十六进制字符
		var nibble byte
		if b >= '0' && b <= '9' {
			nibble = b - '0'
		} else if b >= 'A' && b <= 'F' {
			nibble = b - 'A' + 10
		} else if b >= 'a' && b <= 'f' {
			nibble = b - 'a' + 10
		} else {
			return nil, fmt.Errorf("invalid hex character: %c", b)
		}

		if highNibble {
			currentByte = nibble << 4
			highNibble = false
		} else {
			currentByte |= nibble
			result = append(result, currentByte)
			highNibble = true
		}
	}

	// 如果有未完成的字节，补0
	if !highNibble {
		result = append(result, currentByte)
	}

	return result, nil
}

// RunLengthDecodeFilter 行程编码解码滤镜
type RunLengthDecodeFilter struct{}

func (f *RunLengthDecodeFilter) Name() string { return "RunLengthDecode" }

func (f *RunLengthDecodeFilter) Decode(data []byte) ([]byte, error) {
	var result []byte
	i := 0

	for i < len(data) {
		length := int(data[i])
		i++

		if length == 128 {
			// EOD 标记
			break
		}

		if length < 128 {
			// 复制接下来的 length+1 个字节
			count := length + 1
			if i+count > len(data) {
				return nil, fmt.Errorf("unexpected end of data")
			}
			result = append(result, data[i:i+count]...)
			i += count
		} else {
			// 重复接下来的字节 257-length 次
			count := 257 - length
			if i >= len(data) {
				return nil, fmt.Errorf("unexpected end of data")
			}
			b := data[i]
			i++
			for j := 0; j < count; j++ {
				result = append(result, b)
			}
		}
	}

	return result, nil
}

// DecodeJPEGImage 解码 JPEG 图像为 image.Image
func DecodeJPEGImage(data []byte) (image.Image, error) {
	return jpeg.Decode(bytes.NewReader(data))
}

// ApplyPredictor 应用预测器(用于某些滤镜的后处理)
func ApplyPredictor(data []byte, predictor int, columns int, colors int, bitsPerComponent int) ([]byte, error) {
	if predictor == 1 {
		// 无预测器
		return data, nil
	}

	if predictor >= 10 && predictor <= 15 {
		// PNG 预测器
		return applyPNGPredictor(data, predictor, columns, colors, bitsPerComponent)
	}

	return nil, fmt.Errorf("unsupported predictor: %d", predictor)
}

// applyPNGPredictor 应用 PNG 预测器
func applyPNGPredictor(data []byte, predictor int, columns int, colors int, bitsPerComponent int) ([]byte, error) {
	bytesPerPixel := (colors * bitsPerComponent + 7) / 8
	rowBytes := (columns*colors*bitsPerComponent + 7) / 8
	stride := rowBytes + 1 // +1 for predictor byte

	if len(data)%stride != 0 {
		return nil, fmt.Errorf("invalid data length for PNG predictor")
	}

	rows := len(data) / stride
	result := make([]byte, rows*rowBytes)

	for row := 0; row < rows; row++ {
		srcOffset := row * stride
		dstOffset := row * rowBytes
		predictor := data[srcOffset]

		// 复制当前行数据
		copy(result[dstOffset:dstOffset+rowBytes], data[srcOffset+1:srcOffset+1+rowBytes])

		// 应用预测器
		switch predictor {
		case 0: // None
			// 已经复制,无需处理
		case 1: // Sub
			for i := bytesPerPixel; i < rowBytes; i++ {
				result[dstOffset+i] += result[dstOffset+i-bytesPerPixel]
			}
		case 2: // Up
			if row > 0 {
				prevOffset := (row - 1) * rowBytes
				for i := 0; i < rowBytes; i++ {
					result[dstOffset+i] += result[prevOffset+i]
				}
			}
		case 3: // Average
			for i := 0; i < rowBytes; i++ {
				var left, up byte
				if i >= bytesPerPixel {
					left = result[dstOffset+i-bytesPerPixel]
				}
				if row > 0 {
					up = result[(row-1)*rowBytes+i]
				}
				result[dstOffset+i] += (left + up) / 2
			}
		case 4: // Paeth
			for i := 0; i < rowBytes; i++ {
				var left, up, upLeft byte
				if i >= bytesPerPixel {
					left = result[dstOffset+i-bytesPerPixel]
				}
				if row > 0 {
					up = result[(row-1)*rowBytes+i]
					if i >= bytesPerPixel {
						upLeft = result[(row-1)*rowBytes+i-bytesPerPixel]
					}
				}
				result[dstOffset+i] += paethPredictor(left, up, upLeft)
			}
		}
	}

	return result, nil
}

// paethPredictor Paeth 预测器算法
func paethPredictor(a, b, c byte) byte {
	p := int(a) + int(b) - int(c)
	pa := abs(p - int(a))
	pb := abs(p - int(b))
	pc := abs(p - int(c))

	if pa <= pb && pa <= pc {
		return a
	} else if pb <= pc {
		return b
	}
	return c
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}
