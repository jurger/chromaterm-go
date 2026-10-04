package chromaterm

import "strings"

// pythonGroupMap maps Python group numbers to regexp2's. Python numbers
// capturing groups in order of appearance; regexp2 (like .NET) numbers the
// unnamed ones first and the named ones after them. Element p-1 holds the
// regexp2 number of Python group p. ok is false if the pattern could not be
// scanned, in which case numbers are used as they are.
func pythonGroupMap(pattern string) (groups []int, ok bool) {
	type group struct{ named bool }
	var found []group
	verbose := false
	inClass := false
	for i := 0; i < len(pattern); i++ {
		c := pattern[i]
		switch {
		case c == '\\':
			i++ // skip the escaped character
		case inClass:
			if c == ']' {
				inClass = false
			}
		case c == '[':
			inClass = true
			// A leading ] (or ^]) is a literal member of the class.
			if i+1 < len(pattern) && pattern[i+1] == '^' {
				i++
			}
			if i+1 < len(pattern) && pattern[i+1] == ']' {
				i++
			}
		case verbose && c == '#':
			for i < len(pattern) && pattern[i] != '\n' {
				i++
			}
		case c == '(':
			if !strings.HasPrefix(pattern[i:], "(?") {
				found = append(found, group{})
				continue
			}
			rest := pattern[i+2:]
			switch {
			case strings.HasPrefix(rest, "P<"), strings.HasPrefix(rest, "'"),
				strings.HasPrefix(rest, "<") && !strings.HasPrefix(rest, "<=") && !strings.HasPrefix(rest, "<!"):
				found = append(found, group{named: true})
			case strings.HasPrefix(rest, "#"):
				end := strings.IndexByte(rest, ')')
				if end < 0 {
					return nil, false
				}
				i += 2 + end
			default:
				// Inline flags such as (?ix) or (?i:...) may turn on verbose mode.
				end := strings.IndexAny(rest, ":)")
				if end >= 0 && strings.Trim(rest[:end], "aiLmsux-") == "" {
					if flags, _, _ := strings.Cut(rest[:end], "-"); strings.Contains(flags, "x") {
						verbose = true
					}
				}
			}
		}
	}
	unnamed := 0
	for _, g := range found {
		if !g.named {
			unnamed++
		}
	}
	nextUnnamed, nextNamed := 1, unnamed+1
	for _, g := range found {
		if g.named {
			groups = append(groups, nextNamed)
			nextNamed++
		} else {
			groups = append(groups, nextUnnamed)
			nextUnnamed++
		}
	}
	return groups, true
}
