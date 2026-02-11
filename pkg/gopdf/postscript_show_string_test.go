package gopdf

import "testing"

func TestParsePSShowStringLiteral(t *testing.T) {
	got, ok := parsePSShowString("(Hello\\nWorld) show")
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got != "Hello\nWorld" {
		t.Fatalf("got=%q want=%q", got, "Hello\nWorld")
	}
}

func TestParsePSShowStringHexUTF16BE(t *testing.T) {
	got, ok := parsePSShowString("<FEFF00480065006C006C006F> show")
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got != "Hello" {
		t.Fatalf("got=%q want=%q", got, "Hello")
	}
}

func TestParsePSShowStringHexUTF16LE(t *testing.T) {
	got, ok := parsePSShowString("<FFFE480065006C006C006F00> show")
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got != "Hello" {
		t.Fatalf("got=%q want=%q", got, "Hello")
	}
}

func TestParsePSShowStringHexNoBOMLatin1Fallback(t *testing.T) {
	got, ok := parsePSShowString("<48656C6C6F> show")
	if !ok {
		t.Fatalf("expected ok=true")
	}
	if got != "Hello" {
		t.Fatalf("got=%q want=%q", got, "Hello")
	}
}

