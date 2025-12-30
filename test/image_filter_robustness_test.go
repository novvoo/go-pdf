package test

import (
	"bytes"
	"compress/zlib"
	"image"
	"image/jpeg"
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// TestCorruptedJPEGHandling 测试损坏的JPEG数据处理
func TestCorruptedJPEGHandling(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectError bool
		description string
	}{
		{
			name:        "Invalid JPEG header",
			data:        []byte{0xFF, 0xD8, 0xFF, 0x00}, // 无效的JPEG头部
			expectError: true,
			description: "应该检测到无效的JPEG头部",
		},
		{
			name:        "Truncated JPEG",
			data:        []byte{0xFF, 0xD8, 0xFF, 0xE0}, // 截断的JPEG
			expectError: true,
			description: "应该检测到截断的JPEG数据",
		},
		{
			name:        "Empty data",
			data:        []byte{},
			expectError: true,
			description: "应该拒绝空数据",
		},
		{
			name:        "Random bytes",
			data:        []byte{0x00, 0x01, 0x02, 0x03, 0x04},
			expectError: true,
			description: "应该拒绝随机字节",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &gopdf.DCTDecodeFilter{}
			_, err := filter.Decode(tt.data)

			if tt.expectError && err == nil {
				t.Errorf("%s: 期望错误但没有返回错误", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: 不期望错误但返回了错误: %v", tt.description, err)
			}
			if err != nil {
				t.Logf("正确捕获错误: %v", err)
			}
		})
	}
}

// TestCorruptedFlateDecodeHandling 测试损坏的FlateDecode数据处理
func TestCorruptedFlateDecodeHandling(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectError bool
		description string
	}{
		{
			name:        "Invalid zlib header",
			data:        []byte{0x00, 0x01, 0x02, 0x03},
			expectError: true,
			description: "应该检测到无效的zlib头部",
		},
		{
			name:        "Truncated zlib stream",
			data:        []byte{0x78, 0x9C}, // 有效头部但数据截断
			expectError: true,
			description: "应该检测到截断的zlib流",
		},
		{
			name:        "Empty data",
			data:        []byte{},
			expectError: true,
			description: "应该拒绝空数据",
		},
		{
			name:        "Corrupted checksum",
			data:        createCorruptedZlibData(),
			expectError: true,
			description: "应该检测到校验和错误",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &gopdf.FlateDecodeFilter{}
			_, err := filter.Decode(tt.data)

			if tt.expectError && err == nil {
				t.Errorf("%s: 期望错误但没有返回错误", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: 不期望错误但返回了错误: %v", tt.description, err)
			}
			if err != nil {
				t.Logf("正确捕获错误: %v", err)
			}
		})
	}
}

// TestRunLengthDecodeEdgeCases 测试RunLengthDecode的边界情况
func TestRunLengthDecodeEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectError bool
		description string
	}{
		{
			name:        "Unexpected end of data - copy mode",
			data:        []byte{0x05}, // 需要复制6个字节但数据不足
			expectError: true,
			description: "应该检测到数据不足",
		},
		{
			name:        "Unexpected end of data - repeat mode",
			data:        []byte{0x81}, // 需要重复字节但没有数据
			expectError: true,
			description: "应该检测到重复模式下数据不足",
		},
		{
			name:        "Valid EOD marker",
			data:        []byte{0x80}, // EOD标记 (128)
			expectError: false,
			description: "应该正确处理EOD标记",
		},
		{
			name:        "Empty data",
			data:        []byte{},
			expectError: false,
			description: "空数据应该返回空结果",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &gopdf.RunLengthDecodeFilter{}
			_, err := filter.Decode(tt.data)

			if tt.expectError && err == nil {
				t.Errorf("%s: 期望错误但没有返回错误", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: 不期望错误但返回了错误: %v", tt.description, err)
			}
			if err != nil {
				t.Logf("正确捕获错误: %v", err)
			}
		})
	}
}

// TestASCII85DecodeEdgeCases 测试ASCII85Decode的边界情况
func TestASCII85DecodeEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectError bool
		description string
	}{
		{
			name:        "Invalid character",
			data:        []byte("ABC\x00DEF"),
			expectError: true,
			description: "应该拒绝无效字符",
		},
		{
			name:        "Character out of range",
			data:        []byte("ABCvwxyz"), // 'v'之后的字符超出范围
			expectError: true,
			description: "应该拒绝超出范围的字符",
		},
		{
			name:        "Invalid 'z' placement",
			data:        []byte("ABz"), // 'z'不能出现在组中间
			expectError: true,
			description: "应该检测到'z'的错误位置",
		},
		{
			name:        "Valid terminator",
			data:        []byte("~>"),
			expectError: false,
			description: "应该正确处理终止符",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &gopdf.ASCII85DecodeFilter{}
			_, err := filter.Decode(tt.data)

			if tt.expectError && err == nil {
				t.Errorf("%s: 期望错误但没有返回错误", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: 不期望错误但返回了错误: %v", tt.description, err)
			}
			if err != nil {
				t.Logf("正确捕获错误: %v", err)
			}
		})
	}
}

// TestASCIIHexDecodeEdgeCases 测试ASCIIHexDecode的边界情况
func TestASCIIHexDecodeEdgeCases(t *testing.T) {
	tests := []struct {
		name        string
		data        []byte
		expectError bool
		description string
	}{
		{
			name:        "Invalid hex character",
			data:        []byte("48656C6CG"),
			expectError: true,
			description: "应该拒绝无效的十六进制字符",
		},
		{
			name:        "Valid with whitespace",
			data:        []byte("48 65 6C 6C 6F>"),
			expectError: false,
			description: "应该正确处理带空格的十六进制",
		},
		{
			name:        "Odd number of digits",
			data:        []byte("48656C6C6>"),
			expectError: false,
			description: "应该正确处理奇数个十六进制数字（补0）",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			filter := &gopdf.ASCIIHexDecodeFilter{}
			_, err := filter.Decode(tt.data)

			if tt.expectError && err == nil {
				t.Errorf("%s: 期望错误但没有返回错误", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: 不期望错误但返回了错误: %v", tt.description, err)
			}
			if err != nil {
				t.Logf("正确捕获错误: %v", err)
			}
		})
	}
}

// TestFilterChainWithCorruptedData 测试过滤器链处理损坏数据
func TestFilterChainWithCorruptedData(t *testing.T) {
	// 测试多个过滤器链中的错误传播
	tests := []struct {
		name        string
		filters     []string
		data        []byte
		expectError bool
		description string
	}{
		{
			name:        "Corrupted data in first filter",
			filters:     []string{"FlateDecode", "ASCIIHexDecode"},
			data:        []byte{0x00, 0x01, 0x02},
			expectError: true,
			description: "第一个过滤器失败应该停止处理",
		},
		{
			name:        "Unsupported filter",
			filters:     []string{"UnsupportedFilter"},
			data:        []byte{0x00},
			expectError: true,
			description: "应该拒绝不支持的过滤器",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := gopdf.DecodeImageWithFilters(tt.data, tt.filters)

			if tt.expectError && err == nil {
				t.Errorf("%s: 期望错误但没有返回错误", tt.description)
			}
			if !tt.expectError && err != nil {
				t.Errorf("%s: 不期望错误但返回了错误: %v", tt.description, err)
			}
			if err != nil {
				t.Logf("正确捕获错误: %v", err)
			}
		})
	}
}

// TestValidJPEGDecoding 测试有效的JPEG解码
func TestValidJPEGDecoding(t *testing.T) {
	// 创建一个简单的有效JPEG图像
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	for y := 0; y < 10; y++ {
		for x := 0; x < 10; x++ {
			img.Set(x, y, image.White)
		}
	}

	var buf bytes.Buffer
	err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90})
	if err != nil {
		t.Fatalf("无法创建测试JPEG: %v", err)
	}

	filter := &gopdf.DCTDecodeFilter{}
	decoded, err := filter.Decode(buf.Bytes())
	if err != nil {
		t.Errorf("有效JPEG解码失败: %v", err)
	}
	if len(decoded) == 0 {
		t.Error("解码后的数据为空")
	}
	t.Logf("成功解码JPEG，输出大小: %d 字节", len(decoded))
}

// TestValidFlateDecoding 测试有效的FlateDecode
func TestValidFlateDecoding(t *testing.T) {
	// 创建有效的zlib压缩数据
	original := []byte("Hello, World! This is a test string for FlateDecode.")
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	_, err := w.Write(original)
	if err != nil {
		t.Fatalf("无法创建测试数据: %v", err)
	}
	w.Close()

	filter := &gopdf.FlateDecodeFilter{}
	decoded, err := filter.Decode(buf.Bytes())
	if err != nil {
		t.Errorf("有效FlateDecode解码失败: %v", err)
	}
	if string(decoded) != string(original) {
		t.Errorf("解码结果不匹配，期望: %s, 得到: %s", original, decoded)
	}
	t.Logf("成功解码FlateDecode，输出: %s", decoded)
}

// Helper function: 创建损坏的zlib数据
func createCorruptedZlibData() []byte {
	// 创建有效的zlib数据然后损坏它
	var buf bytes.Buffer
	w := zlib.NewWriter(&buf)
	w.Write([]byte("test"))
	w.Close()

	data := buf.Bytes()
	// 损坏校验和
	if len(data) > 4 {
		data[len(data)-1] ^= 0xFF
	}
	return data
}
