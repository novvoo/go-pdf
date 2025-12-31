package test

import (
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// TestICCBasedDeviceGray 测试 ICCBased DeviceGray 颜色空间
func TestICCBasedDeviceGray(t *testing.T) {
	tests := []struct {
		name       string
		components []float64
		wantR      float64
		wantG      float64
		wantB      float64
	}{
		{
			name:       "Black",
			components: []float64{0.0},
			wantR:      0.0,
			wantG:      0.0,
			wantB:      0.0,
		},
		{
			name:       "White",
			components: []float64{1.0},
			wantR:      1.0,
			wantG:      1.0,
			wantB:      1.0,
		},
		{
			name:       "Mid Gray",
			components: []float64{0.5},
			wantR:      0.5,
			wantG:      0.5,
			wantB:      0.5,
		},
		{
			name:       "Dark Gray",
			components: []float64{0.25},
			wantR:      0.25,
			wantG:      0.25,
			wantB:      0.25,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 创建 ICCBased DeviceGray 颜色空间
			cs := &gopdf.ICCBasedColorSpace{
				NumComponents: 1,
				Alternate:     &gopdf.DeviceGrayColorSpace{},
			}

			r, g, b, err := cs.ConvertToRGB(tt.components)
			if err != nil {
				t.Fatalf("ConvertToRGB failed: %v", err)
			}

			if !floatEqual(r, tt.wantR, 0.001) {
				t.Errorf("R = %v, want %v", r, tt.wantR)
			}
			if !floatEqual(g, tt.wantG, 0.001) {
				t.Errorf("G = %v, want %v", g, tt.wantG)
			}
			if !floatEqual(b, tt.wantB, 0.001) {
				t.Errorf("B = %v, want %v", b, tt.wantB)
			}
		})
	}
}

// TestICCBasedDeviceGrayWithRange 测试带范围限制的 ICCBased DeviceGray
func TestICCBasedDeviceGrayWithRange(t *testing.T) {
	// 创建带范围的 ICCBased DeviceGray 颜色空间
	// Range: [0.0, 1.0] - 标准范围
	cs := &gopdf.ICCBasedColorSpace{
		NumComponents: 1,
		Alternate:     &gopdf.DeviceGrayColorSpace{},
		Range:         []float64{0.0, 1.0},
	}

	tests := []struct {
		name       string
		components []float64
		wantR      float64
		wantG      float64
		wantB      float64
	}{
		{
			name:       "In range - 0.5",
			components: []float64{0.5},
			wantR:      0.5,
			wantG:      0.5,
			wantB:      0.5,
		},
		{
			name:       "Below range - clamped to 0",
			components: []float64{-0.5},
			wantR:      0.0,
			wantG:      0.0,
			wantB:      0.0,
		},
		{
			name:       "Above range - clamped to 1",
			components: []float64{1.5},
			wantR:      1.0,
			wantG:      1.0,
			wantB:      1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, g, b, err := cs.ConvertToRGB(tt.components)
			if err != nil {
				t.Fatalf("ConvertToRGB failed: %v", err)
			}

			if !floatEqual(r, tt.wantR, 0.001) {
				t.Errorf("R = %v, want %v", r, tt.wantR)
			}
			if !floatEqual(g, tt.wantG, 0.001) {
				t.Errorf("G = %v, want %v", g, tt.wantG)
			}
			if !floatEqual(b, tt.wantB, 0.001) {
				t.Errorf("B = %v, want %v", b, tt.wantB)
			}
		})
	}
}

// TestICCBasedDeviceGrayWithCustomRange 测试自定义范围的 ICCBased DeviceGray
func TestICCBasedDeviceGrayWithCustomRange(t *testing.T) {
	// 创建自定义范围的 ICCBased DeviceGray 颜色空间
	// Range: [0.0, 100.0] - 将 0-100 映射到 0-1
	cs := &gopdf.ICCBasedColorSpace{
		NumComponents: 1,
		Alternate:     &gopdf.DeviceGrayColorSpace{},
		Range:         []float64{0.0, 100.0},
	}

	tests := []struct {
		name       string
		components []float64
		wantR      float64
		wantG      float64
		wantB      float64
	}{
		{
			name:       "0 -> 0.0",
			components: []float64{0.0},
			wantR:      0.0,
			wantG:      0.0,
			wantB:      0.0,
		},
		{
			name:       "50 -> 0.5",
			components: []float64{50.0},
			wantR:      0.5,
			wantG:      0.5,
			wantB:      0.5,
		},
		{
			name:       "100 -> 1.0",
			components: []float64{100.0},
			wantR:      1.0,
			wantG:      1.0,
			wantB:      1.0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r, g, b, err := cs.ConvertToRGB(tt.components)
			if err != nil {
				t.Fatalf("ConvertToRGB failed: %v", err)
			}

			if !floatEqual(r, tt.wantR, 0.001) {
				t.Errorf("R = %v, want %v", r, tt.wantR)
			}
			if !floatEqual(g, tt.wantG, 0.001) {
				t.Errorf("G = %v, want %v", g, tt.wantG)
			}
			if !floatEqual(b, tt.wantB, 0.001) {
				t.Errorf("B = %v, want %v", b, tt.wantB)
			}
		})
	}
}

// TestICCBasedDeviceGrayWithoutAlternate 测试没有 Alternate 的 ICCBased DeviceGray
func TestICCBasedDeviceGrayWithoutAlternate(t *testing.T) {
	// 创建没有 Alternate 的 ICCBased DeviceGray 颜色空间
	// 应该使用默认的灰度转换
	cs := &gopdf.ICCBasedColorSpace{
		NumComponents: 1,
	}

	r, g, b, err := cs.ConvertToRGB([]float64{0.75})
	if err != nil {
		t.Fatalf("ConvertToRGB failed: %v", err)
	}

	want := 0.75
	if !floatEqual(r, want, 0.001) || !floatEqual(g, want, 0.001) || !floatEqual(b, want, 0.001) {
		t.Errorf("RGB = (%v, %v, %v), want (%v, %v, %v)", r, g, b, want, want, want)
	}
}

// TestICCBasedDeviceGrayRGBA 测试 ICCBased DeviceGray 的 RGBA 转换
func TestICCBasedDeviceGrayRGBA(t *testing.T) {
	cs := &gopdf.ICCBasedColorSpace{
		NumComponents: 1,
		Alternate:     &gopdf.DeviceGrayColorSpace{},
	}

	r, g, b, a, err := cs.ConvertToRGBA([]float64{0.6}, 0.8)
	if err != nil {
		t.Fatalf("ConvertToRGBA failed: %v", err)
	}

	if !floatEqual(r, 0.6, 0.001) {
		t.Errorf("R = %v, want 0.6", r)
	}
	if !floatEqual(g, 0.6, 0.001) {
		t.Errorf("G = %v, want 0.6", g)
	}
	if !floatEqual(b, 0.6, 0.001) {
		t.Errorf("B = %v, want 0.6", b)
	}
	if !floatEqual(a, 0.8, 0.001) {
		t.Errorf("A = %v, want 0.8", a)
	}
}

// floatEqual 比较两个浮点数是否在误差范围内相等
func floatEqual(a, b, epsilon float64) bool {
	diff := a - b
	if diff < 0 {
		diff = -diff
	}
	return diff < epsilon
}
