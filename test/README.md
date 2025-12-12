# 测试模块说明

## 目录结构

```
test/
├── gopdf/                       # gopdf包的测试文件
│   ├── renderer_test.go          # PDF渲染器测试
│   ├── reader_test.go           # PDF读取器测试
│   ├── text_rendering_test.go   # 文本渲染质量测试
│   ├── pdf_text_test.go         # PDF文本渲染专项测试
│   ├── image_validation_test.go # 图像验证测试（检测翻转等问题）
│   └── image_orientation_test.go # 图像方向测试
├── main_test.go                 # 主测试文件
├── run_tests.bat                # Windows测试运行脚本
├── debug_render.bat             # render_pdf.go调试脚本
└── README.md                    # 本说明文件
```

## 运行测试

### 运行所有测试

```bash
# 在项目根目录运行
go test ./test/gopdf
```

### 运行特定包的测试

```bash
# 运行gopdf包的所有测试
go test ./test/gopdf

# 运行特定测试函数
go test -run TestPDFRenderer ./test/gopdf

# 运行文本渲染相关测试
go test -run TestText ./test/gopdf

# 运行图像验证测试
go test -run TestImage ./test/gopdf

# 运行图像方向测试
go test -run TestImageOrientationNew ./test/gopdf
```

### 运行测试并显示详细信息

```bash
# 运行测试并显示详细输出
go test -v ./test/gopdf

# 运行测试并启用竞态检测
go test -race ./test/gopdf

# 运行特定测试并显示详细信息
go test -v -run TestTextRendering ./test/gopdf

# 运行图像验证测试并显示详细信息
go test -v -run TestImageOrientation ./test/gopdf

# 运行图像方向测试并显示详细信息
go test -v -run TestImageOrientationNew ./test/gopdf
```

### 在Windows上运行测试

可以直接双击运行 `run_tests.bat` 脚本，或在命令行中执行：

```cmd
cd test
run_tests.bat
```

### 调试render_pdf.go

可以直接双击运行 `debug_render.bat` 脚本，或在命令行中执行：

```cmd
debug_render.bat
```

或者直接运行：

```bash
go run cmd/render_pdf.go
```

## 测试内容

### renderer_test.go

包含以下测试：
- `TestPDFRenderer`: 测试PDF渲染器的基本功能
- `TestConvertImageToPNG`: 测试图片格式转换功能
- `TestCoordinateConverter`: 测试坐标转换功能

### reader_test.go

包含以下测试：
- `TestPDFReaderCreation`: 测试PDF读取器的创建
- `TestRenderPageToPNG`: 测试PDF页面渲染为PNG的功能
- `TestInvalidPageNumber`: 测试无效页码处理

### text_rendering_test.go

包含以下文本渲染质量测试：
- `TestTextRendering`: 测试基本文本渲染功能，包括中英文混合
- `TestTextPositioning`: 测试文本定位准确性
- `TestTextScaling`: 测试不同字体大小的文本渲染
- `TestTextOverlap`: 测试文本重叠问题（字母间距）
- `TestTextVisibility`: 测试文本在画布边界处的可见性

### pdf_text_test.go

包含以下PDF文本渲染专项测试：
- `TestPDFTextRendering`: 测试PDF中的文本渲染质量
- `TestChineseTextRendering`: 专门测试中文文本渲染
- `TestTextAlignment`: 测试多页文本对齐一致性
- `TestTextSpacing`: 测试不同DPI设置下的文本间距

### image_validation_test.go

包含以下图像验证测试：
- `TestImageOrientation`: 测试图像方向是否正确（检测翻转问题）
- `TestImageFlippingDetection`: 测试图像翻转检测
- `TestCoordinateSystemConsistency`: 测试坐标系统一致性

### image_orientation_test.go

包含以下图像方向测试：
- `TestImageOrientationNew`: 测试图像方向是否正确（增强版检测）
- `TestCoordinateSystemNew`: 测试坐标系统一致性（增强版检测）

## 注意事项

1. 某些测试需要`test.pdf`文件存在才能完整运行
2. 测试过程中生成的文件会在测试结束后自动清理
3. 部分测试可能会因为缺少系统依赖而跳过，这是正常的
4. 测试会自动查找不同路径下的测试文件，包括`test/test.pdf`和`test/test.png`
5. 文本渲染测试特别关注中英文显示、字符间距、文本位置等问题
6. `render_pdf.go`现在能够自动查找`test`目录下的PDF文件
7. 图像验证测试可以检测输出图像的方向问题（如翻转）
8. 图像方向测试专门用于检测和报告图像翻转问题