package gopdf

import "testing"

func TestTokenizeContentStreamWithOffsets(t *testing.T) {
	b := []byte("q 1 0 0 1 10 20 cm BT (Hello\\)World) Tj ET Q")
	toks, err := TokenizeContentStreamWithOffsets(b)
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	if len(toks) == 0 {
		t.Fatalf("expected tokens")
	}
	if toks[0].Raw != "q" {
		t.Fatalf("expected first token q, got %q", toks[0].Raw)
	}
	foundString := false
	for _, tk := range toks {
		if tk.Raw == "(Hello\\)World)" {
			foundString = true
			if string(b[tk.Start:tk.End]) != tk.Raw {
				t.Fatalf("offset mismatch for string token")
			}
		}
	}
	if !foundString {
		t.Fatalf("expected literal string token")
	}
}

func TestParseContentOps_BasicText(t *testing.T) {
	b := []byte("BT (Hello) Tj ET")
	toks, err := TokenizeContentStreamWithOffsets(b)
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	ops, err := ParseContentOps(toks)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if len(ops) < 3 {
		t.Fatalf("expected at least 3 ops, got %d", len(ops))
	}
	if ops[0].Name != "BT" {
		t.Fatalf("expected first op BT, got %q", ops[0].Name)
	}
	foundTj := false
	for _, op := range ops {
		if op.Name == "Tj" {
			foundTj = true
			if len(op.Args) != 1 {
				t.Fatalf("expected Tj 1 arg, got %d", len(op.Args))
			}
			if s, ok := op.Args[0].(string); !ok || s != "Hello" {
				t.Fatalf("expected Tj arg Hello, got %#v", op.Args[0])
			}
			if op.End <= op.Start {
				t.Fatalf("expected valid op range")
			}
		}
	}
	if !foundTj {
		t.Fatalf("expected Tj op")
	}
}

func TestParseContentOps_TJHex(t *testing.T) {
	b := []byte("BT [<00 0A> -120 <00FF>] TJ ET")
	toks, err := TokenizeContentStreamWithOffsets(b)
	if err != nil {
		t.Fatalf("tokenize: %v", err)
	}
	ops, err := ParseContentOps(toks)
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	foundTJ := false
	for _, op := range ops {
		if op.Name != "TJ" {
			continue
		}
		foundTJ = true
		if len(op.Args) != 1 {
			t.Fatalf("expected TJ 1 arg, got %d", len(op.Args))
		}
		arr, ok := op.Args[0].([]interface{})
		if !ok || len(arr) != 3 {
			t.Fatalf("expected TJ array len=3, got %#v", op.Args[0])
		}
		if s, ok := arr[0].(string); !ok || s != "<00 0A>" {
			t.Fatalf("expected first TJ elem hex string, got %#v", arr[0])
		}
	}
	if !foundTJ {
		t.Fatalf("expected TJ op")
	}
}

