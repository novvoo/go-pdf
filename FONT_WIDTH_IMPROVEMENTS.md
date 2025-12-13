# 字体宽度计算改进

## 概述

改进了 PDF 文本渲染中的字体占位（宽度）计算，从简单的估算改为使用实际的字形宽度信息。

## 改进内容

### 1. 扩展 Font 结构

添加了字形宽度相关的字段：

```go
type Font struct {
    // ... 原有字段 ...
    Widths           *FontWidths  // 字形宽度信息
    DefaultWidth     float64      // 默认字形宽度（CID 字体）
    MissingWidth     float64      // 缺失字形的宽度
}

type FontWidths struct {
    // Type1/TrueType 字体：FirstChar 到 LastChar 的宽度数组
    FirstChar int
    LastChar  int
    Widths    []float64

    // CID 字体：CID 到宽度的映射
    CIDWidths map[uint16]float64
    CIDRanges []CIDWidthRange
}
```

### 2. 从 PDF 加载字形宽度

实现了以下函数来从 PDF 字体字典中读取宽度信息：

- `loadFontWidths()` - 主加载函数，根据字体类型分发
- `loadCIDFontWidths()` - 加载 CID 字体（Type0）的宽度信息