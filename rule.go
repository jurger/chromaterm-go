package chromaterm

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/dlclark/regexp2"
)

// matchTimeout bounds a single rule's regex on one chunk, so a pathological
// pattern cannot stall the session.
const matchTimeout = 100 * time.Millisecond

// Python named-group syntax, translated for regexp2.
var (
	pyNamedGroupRE = regexp.MustCompile(`\(\?P<`)
	pyBackrefRE    = regexp.MustCompile(`\(\?P=([A-Za-z_][A-Za-z0-9_]*)\)`)
)

// Rule highlights the matches of a regular expression. Colours are keyed by
// regex group; group 0 colours the whole match.
type Rule struct {
	Description string
	Regex       string
	Exclusive   bool

	re     *regexp2.Regexp
	colors []groupColor
}

type groupColor struct {
	group int
	color Color
}

type match struct {
	start, end int
	color      Color
}

// NewRule compiles regex, which uses ChromaTerm's (Python) syntax: lookaround,
// backreferences and (?P<name>...) groups are supported. colors maps a group
// number or name to its colour.
func NewRule(regex string, colors map[any]Color, description string, exclusive bool) (*Rule, error) {
	pattern := pyNamedGroupRE.ReplaceAllString(regex, "(?<")
	pattern = pyBackrefRE.ReplaceAllString(pattern, `\k<$1>`)
	re, err := regexp2.Compile(pattern, regexp2.None)
	if err != nil {
		return nil, err
	}
	re.MatchTimeout = matchTimeout
	rule := &Rule{Description: description, Regex: regex, Exclusive: exclusive, re: re}
	groups := len(re.GetGroupNumbers()) - 1
	groupMap, mapped := pythonGroupMap(regex)
	if mapped && len(groupMap) != groups {
		mapped = false
	}
	for key, color := range colors {
		group := 0
		switch k := key.(type) {
		case int:
			if k < 0 || k > groups {
				return nil, fmt.Errorf("regex has %d group(s); %d is invalid", groups, k)
			}
			group = k
			if mapped && k > 0 {
				group = groupMap[k-1]
			}
		case string:
			n := re.GroupNumberFromName(k)
			if n < 0 {
				return nil, fmt.Errorf("named group %q not in regex", k)
			}
			group = n
		default:
			return nil, fmt.Errorf("group must be an integer or a string")
		}
		if group < 0 || group > groups {
			return nil, fmt.Errorf("regex has %d group(s); %d is invalid", groups, group)
		}
		rule.colors = append(rule.colors, groupColor{group: group, color: color})
	}
	sort.Slice(rule.colors, func(i, j int) bool { return rule.colors[i].group < rule.colors[j].group })
	return rule, nil
}

// matches returns the coloured spans of the rule's matches in data, as byte
// offsets. Empty spans and spans across a new line are skipped.
func (r *Rule) matches(data string) []match {
	if len(r.colors) == 0 {
		return nil
	}
	// regexp2 indexes runes; map them back to byte offsets. Invalid UTF-8
	// bytes decode to one rune each, as in regexp2's own conversion.
	offsets := make([]int, 0, len(data)+1)
	for i := 0; i < len(data); {
		offsets = append(offsets, i)
		_, size := utf8.DecodeRuneInString(data[i:])
		i += size
	}
	offsets = append(offsets, len(data))

	var out []match
	m, err := r.re.FindStringMatch(data)
	for err == nil && m != nil {
		for _, gc := range r.colors {
			g := m.GroupByNumber(gc.group)
			if g == nil || len(g.Captures) == 0 {
				continue
			}
			start, end := offsets[g.Index], offsets[g.Index+g.Length]
			if start != end && !strings.Contains(data[start:end], "\n") {
				out = append(out, match{start: start, end: end, color: gc.color})
			}
		}
		m, err = r.re.FindNextMatch(m)
	}
	return out
}
