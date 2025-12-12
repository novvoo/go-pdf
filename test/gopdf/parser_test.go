package gopdf_test

import (
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

func TestParseTokens(t *testing.T) {
	tests := []struct {
		name    string
		tokens  []string
		wantLen int
		wantOps []string
		wantErr bool
	}{
		{
			name:    "保存和恢复图形状态",
			tokens:  []string{"q", "Q"},
			wantLen: 2,
			wantOps: []string{"q", "Q"},
		},
		{
			name:    "设置线宽",
			tokens:  []string{"1.5", "w"},
			wantLen: 1,
			wantOps: []string{"w"},
		},
		{
			name:    "移动到坐标",
			tokens:  []string{"100", "200", "m"},
			wantLen: 1,
			wantOps: []string{"m"},
		},
		{
			name:    "画线",
			tokens:  []string{"100", "200", "m", "300", "400", "l"},
			wantLen: 2,
			wantOps: []string{"m", "l"},
		},
		{
			name:    "矩形",
			tokens:  []string{"10", "20", "100", "50", "re"},
			wantLen: 1,
			wantOps: []string{"re"},
		},
		{
			name:    "填充和描边",
			tokens:  []string{"f", "S"},
			wantLen: 2,
			wantOps: []string{"f", "S"},
		},
		{
			name:    "设置RGB颜色",
			tokens:  []string{"1", "0", "0", "rg", "0", "1", "0", "RG"},
			wantLen: 2,
			wantOps: []string{"rg", "RG"},
		},
		{
			name:    "设置灰度颜色",
			tokens:  []string{"0.5", "g", "0.8", "G"},
			wantLen: 2,
			wantOps: []string{"g", "G"},
		},
		{
			name:    "设置CMYK颜色",
			tokens:  []string{"0", "1", "1", "0", "k"},
			wantLen: 1,
			wantOps: []string{"k"},
		},
		{
			name:    "变换矩阵",
			tokens:  []string{"1", "0", "0", "1", "100", "200", "cm"},
			wantLen: 1,
			wantOps: []string{"cm"},
		},
		{
			name:    "文本操作",
			tokens:  []string{"BT", "/F1", "12", "Tf", "(Hello)", "Tj", "ET"},
			wantLen: 4,
			wantOps: []string{"BT", "Tf", "Tj", "ET"},
		},
		{
			name:    "文本矩阵",
			tokens:  []string{"1", "0", "0", "1", "50", "100", "Tm"},
			wantLen: 1,
			wantOps: []string{"Tm"},
		},
		{
			name:    "文本位置",
			tokens:  []string{"10", "20", "Td"},
			wantLen: 1,
			wantOps: []string{"Td"},
		},
		{
			name:    "数组参数",
			tokens:  []string{"[", "1", "2", "3", "]", "0", "d"},
			wantLen: 1,
			wantOps: []string{"d"},
		},
		{
			name:    "字典参数",
			tokens:  []string{"<<", "/Type", "/ExtGState", ">>", "gs"},
			wantLen: 1,
			wantOps: []string{"gs"},
		},
		{
			name:    "XObject",
			tokens:  []string{"/Image1", "Do"},
			wantLen: 1,
			wantOps: []string{"Do"},
		},
		{
			name:    "曲线操作",
			tokens:  []string{"10", "20", "30", "40", "50", "60", "c"},
			wantLen: 1,
			wantOps: []string{"c"},
		},
		{
			name:    "关闭路径",
			tokens:  []string{"h"},
			wantLen: 1,
			wantOps: []string{"h"},
		},
		{
			name:    "裁剪路径",
			tokens:  []string{"W", "n"},
			wantLen: 2,
			wantOps: []string{"W", "n"},
		},
		{
			name:    "标记内容",
			tokens:  []string{"/P", "BMC", "EMC"},
			wantLen: 2,
			wantOps: []string{"IGNORE", "IGNORE"},
		},
		{
			name:    "空tokens",
			tokens:  []string{},
			wantLen: 0,
			wantOps: []string{},
		},
		{
			name:    "包含空字符串的tokens",
			tokens:  []string{"", "q", "", "Q", ""},
			wantLen: 2,
			wantOps: []string{"q", "Q"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ops, err := gopdf.ParseTokens(tt.tokens)

			if (err != nil) != tt.wantErr {
				t.Errorf("parseTokens() error = %v, wantErr %v", err, tt.wantErr)
				return
			}

			if len(ops) != tt.wantLen {
				t.Errorf("parseTokens() got %d operators, want %d", len(ops), tt.wantLen)
				return
			}

			for i, op := range ops {
				if i < len(tt.wantOps) && op.Name() != tt.wantOps[i] {
					t.Errorf("operator[%d] name = %v, want %v", i, op.Name(), tt.wantOps[i])
				}
			}
		})
	}
}
