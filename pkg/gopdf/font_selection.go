package gopdf

import "strings"

func stripSubsetPrefix(name string) string {
	name = strings.TrimPrefix(name, "/")
	if len(name) > 7 && name[6] == '+' {
		prefix := name[:6]
		valid := true
		for i := 0; i < len(prefix); i++ {
			c := prefix[i]
			if c < 'A' || c > 'Z' {
				valid = false
				break
			}
		}
		if valid {
			return name[7:]
		}
	}
	return name
}

func isTeXMathFont(baseFont string) bool {
	n := stripSubsetPrefix(baseFont)
	n = strings.ToUpper(n)
	return strings.HasPrefix(n, "CMMI") ||
		strings.HasPrefix(n, "CMSY") ||
		strings.HasPrefix(n, "CMEX") ||
		strings.HasPrefix(n, "MSBM") ||
		strings.HasPrefix(n, "MSAM")
}

