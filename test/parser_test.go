package test

import (
	"testing"

	"github.com/novvoo/go-pdf/pkg/gopdf"
)

// TestParseTokens 测试token解析功能
func TestParseTokens(t *testing.T) {
	tests := []struct {
		name    string
		tokens  []string
		wantLen int
		wantOps []string
		wantErr bool
	}{
		{
			name:    "graphics state save and restore",
			tokens:  []string{"q", "Q"},
			wantLen: 2,
			wantOps: []string{"q", "Q"},
		},
		{
			name:    "set line width",
			tokens:  []string{"1.5", "w"},
			wantLen: 1,
			wantOps: []string{"w"},
		},
		{
			name:    "move to coordinate",
			tokens:  []string{"100", "200", "m"},
			wantLen: 1,
			wantOps: []string{"m"},
		},
		{
			name:    "draw line",
			tokens:  []string{"100", "200", "m", "300", "400", "l"},
			wantLen: 2,
			wantOps: []string{"m", "l"},
		},
		{
			name:    "rectangle",
			tokens:  []string{"10", "20", "100", "50", "re"},
			wantLen: 1,
			wantOps: []string{"re"},
		},
		{
			name:    "fill and stroke",
			tokens:  []string{"f", "S"},
			wantLen: 2,
			wantOps: []string{"f", "S"},
		},
		{
			name:    "set RGB color",
			tokens:  []string{"1", "0", "0", "rg", "0", "1", "0", "RG"},
			wantLen: 2,
			wantOps: []string{"rg", "RG"},
		},
		{
			name:    "set gray color",
			tokens:  []string{"0.5", "g", "0.8", "G"},
			wantLen: 2,
			wantOps: []string{"g", "G"},
		},
		{
			name:    "set CMYK color",
			tokens:  []string{"0", "1", "1", "0", "k"},
			wantLen: 1,
			wantOps: []string{"k"},
		},
		{
			name:    "transform matrix",
			tokens:  []string{"1", "0", "0", "1", "100", "200", "cm"},
			wantLen: 1,
			wantOps: []string{"cm"},
		},
		{
			name:    "text operations",
			tokens:  []string{"BT", "/F1", "12", "Tf", "(Hello)", "Tj", "ET"},
			wantLen: 4,
			wantOps: []string{"BT", "Tf", "Tj", "ET"},
		},
		{
			name:    "text matrix",
			tokens:  []string{"1", "0", "0", "1", "50", "100", "Tm"},
			wantLen: 1,
			wantOps: []string{"Tm"},
		},
		{
			name:    "text position",
			tokens:  []string{"10", "20", "Td"},
			wantLen: 1,
			wantOps: []string{"Td"},
		},
		{
			name:    "array parameter",
			tokens:  []string{"[", "1", "2", "3", "]", "0", "d"},
			wantLen: 1,
			wantOps: []string{"d"},
		},
		{
			name:    "dictionary parameter",
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
			name:    "curve operations",
			tokens:  []string{"10", "20", "30", "40", "50", "60", "c"},
			wantLen: 1,
			wantOps: []string{"c"},
		},
		{
			name:    "close path",
			tokens:  []string{"h"},
			wantLen: 1,
			wantOps: []string{"h"},
		},
		{
			name:    "clipping path",
			tokens:  []string{"W", "n"},
			wantLen: 2,
			wantOps: []string{"W", "n"},
		},
		{
			name:    "marked content",
			tokens:  []string{"/P", "BMC", "EMC"},
			wantLen: 2,
			wantOps: []string{"BMC", "EMC"},
		},
		{
			name:    "empty tokens",
			tokens:  []string{},
			wantLen: 0,
			wantOps: []string{},
		},
		{
			name:    "tokens with empty strings",
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

// TestParseTokensEdgeCases 测试边界情况
func TestParseTokensEdgeCases(t *testing.T) {
	tests := []struct {
		name    string
		tokens  []string
		wantErr bool
	}{
		{
			name:    "nil tokens",
			tokens:  nil,
			wantErr: false,
		},
		{
			name:    "very long token sequence",
			tokens:  make([]string, 10000),
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := gopdf.ParseTokens(tt.tokens)
			if (err != nil) != tt.wantErr {
				t.Errorf("parseTokens() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// BenchmarkParseTokens 基准测试token解析性能
func BenchmarkParseTokens(b *testing.B) {
	tokens := []string{"q", "1", "0", "0", "1", "100", "200", "cm", "BT", "/F1", "12", "Tf", "(Hello)", "Tj", "ET", "Q"}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = gopdf.ParseTokens(tokens)
	}
}
