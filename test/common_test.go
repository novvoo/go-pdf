package test

import (
	"image"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

// TestHelper 提供通用的测试辅助功能
type TestHelper struct {
	t *testing.T
}

// NewTestHelper 创建测试辅助工具
func NewTestHelper(t *testing.T) *TestHelper {
	return &TestHelper{t: t}
}

// FindTestPDF 查找测试PDF文件
func (h *TestHelper) FindTestPDF(filename string) string {
	paths := []string{
		filename,
		filepath.Join("test", filename),
		filepath.Join("..", filename),
		filepath.Join("example", filename),
	}

	for _, path := range paths {
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	h.t.Skipf("Test PDF file not found: %s", filename)
	return ""
}

// LoadAndValidateImage 加载并验证图片
func (h *TestHelper) LoadAndValidateImage(filename string) image.Image {
	file, err := os.Open(filename)
	if err != nil {
		h.t.Fatalf("Failed to open image: %v", err)
	}
	defer file.Close()

	img, err := png.Decode(file)
	if err != nil {
		h.t.Fatalf("Failed to decode PNG: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Empty() {
		h.t.Fatal("Image has empty bounds")
	}

	return img
}

// CleanupFile 清理测试文件
func (h *TestHelper) CleanupFile(filename string) {
	if err := os.Remove(filename); err != nil && !os.IsNotExist(err) {
		h.t.Logf("Warning: Failed to cleanup file %s: %v", filename, err)
	}
}

// AssertFileExists 断言文件存在
func (h *TestHelper) AssertFileExists(filename string) {
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		h.t.Errorf("Expected file does not exist: %s", filename)
	}
}

// AssertNoError 断言没有错误
func (h *TestHelper) AssertNoError(err error, msg string) {
	if err != nil {
		h.t.Fatalf("%s: %v", msg, err)
	}
}

// AssertError 断言有错误
func (h *TestHelper) AssertError(err error, msg string) {
	if err == nil {
		h.t.Errorf("%s: expected error but got none", msg)
	}
}

// AssertEqual 断言相等
func (h *TestHelper) AssertEqual(got, want any, msg string) {
	if got != want {
		h.t.Errorf("%s: got %v, want %v", msg, got, want)
	}
}

// AssertTrue 断言为真
func (h *TestHelper) AssertTrue(condition bool, msg string) {
	if !condition {
		h.t.Errorf("%s: expected true but got false", msg)
	}
}

// AssertFalse 断言为假
func (h *TestHelper) AssertFalse(condition bool, msg string) {
	if condition {
		h.t.Errorf("%s: expected false but got true", msg)
	}
}
