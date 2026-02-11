package gopdf

import "testing"

func TestParsePSShowString_HexUTF16(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{
			name: "utf16be_bom",
			in:   "<FEFF0043006F> show",
			want: "Co",
		},
		{
			name: "utf16be_no_bom",
			in:   "<0043006F> show",
			want: "Co",
		},
		{
			name: "utf16le_no_bom",
			in:   "<43006F00> show",
			want: "Co",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := parsePSShowString(tt.in)
			if !ok {
				t.Fatalf("parsePSShowString ok=false")
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

