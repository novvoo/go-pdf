package gopdf

import "testing"

func TestAnalyzePSShowFragmentationConsecutiveLetters(t *testing.T) {
	ps := `
gsave
(consisten) show
grestore
gsave
0 0 moveto
1 0 0 setrgbcolor
(t) show
grestore
`
	rep := AnalyzePSShowFragmentation(ps, 10)
	if rep.TotalShows != 2 {
		t.Fatalf("TotalShows=%d want=2", rep.TotalShows)
	}
	if rep.ShortShowsLen1 != 1 {
		t.Fatalf("ShortShowsLen1=%d want=1", rep.ShortShowsLen1)
	}
	if rep.ConsecutiveLetters != 1 {
		t.Fatalf("ConsecutiveLetters=%d want=1", rep.ConsecutiveLetters)
	}
	if len(rep.Examples) == 0 {
		t.Fatalf("expected examples")
	}
}

func TestAnalyzePSShowFragmentationHexUTF16(t *testing.T) {
	ps := `<FEFF03B8> show
`
	rep := AnalyzePSShowFragmentation(ps, 10)
	if rep.TotalShows != 1 || rep.HexShows != 1 {
		t.Fatalf("TotalShows=%d HexShows=%d want 1,1", rep.TotalShows, rep.HexShows)
	}
	if rep.MathConsecutiveLetters != 0 {
		t.Fatalf("MathConsecutiveLetters=%d want=0", rep.MathConsecutiveLetters)
	}
}

func TestAnalyzePSShowFragmentationBodyVsMath(t *testing.T) {
	ps := `
/sans-serif findfont 10 scalefont setfont
(the) show
/math findfont 10 scalefont setfont
(k) show
`
	rep := AnalyzePSShowFragmentation(ps, 10)
	if rep.ConsecutiveLetters != 1 {
		t.Fatalf("ConsecutiveLetters=%d want=1", rep.ConsecutiveLetters)
	}
	if rep.BodyConsecutiveLetters != 0 {
		t.Fatalf("BodyConsecutiveLetters=%d want=0", rep.BodyConsecutiveLetters)
	}
	if rep.MathConsecutiveLetters != 1 {
		t.Fatalf("MathConsecutiveLetters=%d want=1", rep.MathConsecutiveLetters)
	}
}

func TestAnalyzePSShowFragmentationBodyBadSplit(t *testing.T) {
	ps := `
/sans-serif findfont 10 scalefont setfont
(consisten) show
(t) show
`
	rep := AnalyzePSShowFragmentation(ps, 10)
	if rep.ConsecutiveLetters != 1 {
		t.Fatalf("ConsecutiveLetters=%d want=1", rep.ConsecutiveLetters)
	}
	if rep.BodyConsecutiveLetters != 1 {
		t.Fatalf("BodyConsecutiveLetters=%d want=1", rep.BodyConsecutiveLetters)
	}
	if rep.MathConsecutiveLetters != 0 {
		t.Fatalf("MathConsecutiveLetters=%d want=0", rep.MathConsecutiveLetters)
	}
}
