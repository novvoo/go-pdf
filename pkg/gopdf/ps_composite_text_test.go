package gopdf

import (
	"os"
	"strings"
	"testing"
)

func TestPSUTF16HexStringWithBOM(t *testing.T) {
	got := psUTF16HexStringWithBOM("Hello")
	want := "<FEFF00480065006C006C006F>"
	if got != want {
		t.Fatalf("got=%q want=%q", got, want)
	}
}

func TestPSSurfaceHeaderContainsCompositeFontDefs(t *testing.T) {
	tmp, err := os.CreateTemp("", "gopdf_ps_header_*.ps")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)

	s := NewPSSurface(path, 100, 100)
	s.Destroy()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ps: %v", err)
	}
	content := string(b)
	if !strings.Contains(content, "/GoPDF-GB") {
		t.Fatalf("expected header to contain GoPDF composite font prolog")
	}
}

func TestPangoPdfShowTextOnPSSurfaceUsesOutlineForCJK(t *testing.T) {
	tmp, err := os.CreateTemp("", "gopdf_ps_text_*.ps")
	if err != nil {
		t.Fatalf("create temp: %v", err)
	}
	path := tmp.Name()
	tmp.Close()
	defer os.Remove(path)

	s := NewPSSurface(path, 200, 200)
	ctx := NewContext(s).(*context)
	layout := ctx.PangoPdfCreateLayout().(*PangoPdfLayout)
	fd := NewPangoFontDescription()
	fd.SetFamily("sans")
	fd.SetSize(12)
	layout.SetFontDescription(fd)
	layout.SetText("中文")

	ctx.MoveTo(10, 10)
	ctx.PangoPdfShowText(layout)
	s.Destroy()

	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read ps: %v", err)
	}
	content := string(b)
	if strings.Contains(content, "/GoPDF-GB findfont") || strings.Contains(content, "<FEFF") {
		t.Fatalf("expected PS to use outlines for CJK (no Type0/UTF-16 show), got: %q", content)
	}
	if !strings.Contains(content, "newpath") || !strings.Contains(content, "curveto") || !strings.Contains(content, "fill") {
		t.Fatalf("expected PS to contain outline drawing ops, got: %q", content)
	}
}
