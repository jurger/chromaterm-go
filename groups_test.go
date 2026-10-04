package chromaterm

import (
	"slices"
	"testing"
)

func TestPythonGroupMap(t *testing.T) {
	cases := []struct {
		pattern string
		want    []int
	}{
		{`(a)(b)`, []int{1, 2}},
		{`(?P<x>a)(b)`, []int{2, 1}},
		{`(a)(?P<x>b)(c)(?<y>d)`, []int{1, 3, 2, 4}},
		{`(?:a)(?=b)(?<!c)(d)\(e\)[(f)](?#(g))`, []int{1}},
		{"(?x)(a) # (comment)\n(?P<n>b)", []int{1, 2}},
		{"(?x)(?P<n>a) # (comment)\n(b)", []int{2, 1}},
		{`[\]()](x)`, []int{1}},
	}
	for _, c := range cases {
		got, ok := pythonGroupMap(c.pattern)
		if !ok || !slices.Equal(got, c.want) {
			t.Errorf("%q: got %v (%v), want %v", c.pattern, got, ok, c.want)
		}
	}
}
