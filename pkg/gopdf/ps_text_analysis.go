package gopdf

import (
	"sort"
	"strings"
)

type PSShowFragment struct {
	LineNo int
	Text   string
	Raw    string
}

type PSFragmentationReport struct {
	TotalShows             int
	LiteralShows           int
	HexShows               int
	ShortShowsLen1         int
	ShortShowsLen2         int
	ShortShowsLen3         int
	ConsecutiveLetters     int
	BodyConsecutiveLetters int
	MathConsecutiveLetters int
	Examples               []PSShowFragment
	BodyExamples           []PSShowFragment
}

func AnalyzePSShowFragmentation(psContent string, maxExamples int) PSFragmentationReport {
	var rep PSFragmentationReport
	if maxExamples <= 0 {
		maxExamples = 30
	}

	lines := strings.Split(psContent, "\n")
	examples := make([]PSShowFragment, 0, maxExamples)
	bodyExamples := make([]PSShowFragment, 0, maxExamples)

	prevIsLetters := false
	prevText := ""
	prevLineNo := 0
	prevIsMath := false
	currentFont := ""
	for i, line := range lines {
		s := strings.TrimSpace(line)
		if s == "" {
			continue
		}
		if strings.HasPrefix(s, "%%Page:") || s == "showpage" || strings.HasPrefix(s, "%%Trailer") {
			prevIsLetters = false
			prevText = ""
			prevLineNo = 0
			prevIsMath = false
			currentFont = ""
			continue
		}

		if fn, ok := extractCurrentFontFromSetFontLine(s); ok {
			currentFont = fn
		}

		shows := extractShowTextsFromLine(s)
		if len(shows) == 0 {
			continue
		}

		for _, sh := range shows {
			text, rawIsHex := sh.text, sh.rawIsHex

			rep.TotalShows++
			if rawIsHex {
				rep.HexShows++
			} else {
				rep.LiteralShows++
			}

			runes := []rune(text)
			switch len(runes) {
			case 1:
				rep.ShortShowsLen1++
			case 2:
				rep.ShortShowsLen2++
			case 3:
				rep.ShortShowsLen3++
			}

			isLetters := isAllLettersOrApostrophe(text)
			isMath := psIsMathText(text, currentFont, rawIsHex)
			if prevIsLetters && isLetters {
				prevLen := len([]rune(prevText))
				curLen := len(runes)
				if prevLen <= 12 && curLen <= 12 && (prevLen <= 2 || curLen <= 2) {
					if prevLineNo > 0 && (i+1-prevLineNo) <= 60 {
						rep.ConsecutiveLetters++
						if prevIsMath || isMath {
							rep.MathConsecutiveLetters++
						} else {
							bodyBad := (prevLen >= 3 && curLen <= 2) || (curLen >= 3 && prevLen <= 2)
							if bodyBad {
								shortPart := ""
								if prevLen <= 2 && curLen >= 3 {
									shortPart = prevText
								} else if curLen <= 2 && prevLen >= 3 {
									shortPart = text
								}
								if shortPart != "" && !isCommonShortWord(shortPart) {
									rep.BodyConsecutiveLetters++
								}
								if len(bodyExamples) < maxExamples {
									bodyExamples = append(bodyExamples, PSShowFragment{
										LineNo: i + 1,
										Text:   prevText + " + " + text,
										Raw:    strings.TrimSpace(line),
									})
								}
							}
						}
						if len(examples) < maxExamples {
							examples = append(examples, PSShowFragment{
								LineNo: i + 1,
								Text:   prevText + " + " + text,
								Raw:    strings.TrimSpace(line),
							})
						}
					}
				}
			}
			prevIsLetters = isLetters
			prevText = text
			prevLineNo = i + 1
			prevIsMath = isMath
		}
	}

	rep.Examples = examples
	rep.BodyExamples = bodyExamples
	sort.Slice(rep.Examples, func(i, j int) bool { return rep.Examples[i].LineNo < rep.Examples[j].LineNo })
	sort.Slice(rep.BodyExamples, func(i, j int) bool { return rep.BodyExamples[i].LineNo < rep.BodyExamples[j].LineNo })
	return rep
}

type psExtractedShow struct {
	text     string
	rawIsHex bool
}

func extractShowTextsFromLine(line string) []psExtractedShow {
	line = strings.TrimSpace(line)
	if line == "" || !strings.Contains(line, "show") {
		return nil
	}

	out := make([]psExtractedShow, 0, 2)
	for i := 0; i < len(line); i++ {
		switch line[i] {
		case '(':
			j := i + 1
			esc := false
			for j < len(line) {
				c := line[j]
				if esc {
					esc = false
					j++
					continue
				}
				if c == '\\' {
					esc = true
					j++
					continue
				}
				if c == ')' {
					break
				}
				j++
			}
			if j >= len(line) || line[j] != ')' {
				continue
			}
			k := j + 1
			for k < len(line) && (line[k] == ' ' || line[k] == '\t') {
				k++
			}
			if k+4 <= len(line) && line[k:k+4] == "show" {
				if t, ok := parsePSShowString(line[i : j+1]); ok {
					out = append(out, psExtractedShow{text: t, rawIsHex: false})
				}
				i = j
			}
		case '<':
			j := strings.IndexByte(line[i+1:], '>')
			if j < 0 {
				continue
			}
			j = i + 1 + j
			k := j + 1
			for k < len(line) && (line[k] == ' ' || line[k] == '\t') {
				k++
			}
			if k+4 <= len(line) && line[k:k+4] == "show" {
				if t, ok := parsePSShowString(line[i : j+1]); ok {
					out = append(out, psExtractedShow{text: t, rawIsHex: true})
				}
				i = j
			}
		}
	}
	return out
}

func isAllLettersOrApostrophe(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r == '\'' {
			continue
		}
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') {
			continue
		}
		return false
	}
	return true
}

func extractCurrentFontFromSetFontLine(line string) (string, bool) {
	line = strings.TrimSpace(line)
	if !strings.Contains(line, " findfont") || !strings.Contains(line, " setfont") {
		return "", false
	}
	if strings.HasPrefix(line, "/") {
		idx := strings.Index(line, " findfont")
		if idx <= 1 {
			return "", false
		}
		return line[1:idx], true
	}
	if strings.HasPrefix(line, "{/") {
		idx := strings.Index(line, " findfont")
		if idx <= 2 {
			return "", false
		}
		return line[2:idx], true
	}
	return "", false
}

func psIsMathText(text, font string, rawIsHex bool) bool {
	f := strings.ToLower(strings.TrimSpace(font))
	if f == "math" || f == "symbol" || strings.Contains(f, "math") {
		return true
	}
	if !rawIsHex {
		return false
	}
	for _, r := range text {
		switch {
		case r >= 0x0370 && r <= 0x03FF:
			return true
		case r >= 0x2070 && r <= 0x209F:
			return true
		case r >= 0x2200 && r <= 0x22FF:
			return true
		}
	}
	return false
}

func isCommonShortWord(s string) bool {
	w := strings.ToLower(strings.TrimSpace(s))
	switch w {
	case "a", "an", "as", "at", "by", "do", "if", "in", "is", "it", "of", "on", "or", "so", "to", "we":
		return true
	default:
		return false
	}
}
