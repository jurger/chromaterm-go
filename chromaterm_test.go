package chromaterm

import (
	"encoding/json"
	"os"
	"strconv"
	"testing"
)

// testdata/chromaterm.json was produced by running the original ChromaTerm
// 0.10.7 (Python) over the same inputs, so these tests pin the port to its
// behaviour. Python is not needed to run them.
type golden struct {
	Rules []struct {
		Description string              `json:"description"`
		Spans       map[string][][2]int `json:"spans"`
	} `json:"rules"`
	Defaults []struct {
		RGB bool   `json:"rgb"`
		In  string `json:"in"`
		Out string `json:"out"`
	} `json:"defaults"`
	Algorithm []struct {
		Rules []struct {
			Regex     string `json:"regex"`
			Color     any    `json:"color"`
			Exclusive bool   `json:"exclusive"`
		} `json:"rules"`
		Inputs  []string `json:"inputs"`
		Outputs []string `json:"outputs"`
	} `json:"algorithm"`
	Splits []struct {
		In     string      `json:"in"`
		Chunks [][2]string `json:"chunks"`
	} `json:"splits"`
}

func loadGolden(t *testing.T) golden {
	t.Helper()
	raw, err := os.ReadFile("testdata/chromaterm.json")
	if err != nil {
		t.Fatal(err)
	}
	var g golden
	if err := json.Unmarshal(raw, &g); err != nil {
		t.Fatal(err)
	}
	return g
}

func defaultConfig(t *testing.T, rgb bool) *Config {
	t.Helper()
	c := NewConfig()
	if errs := c.Load(DefaultConfig, rgb); len(errs) > 0 {
		t.Fatalf("default config: %v", errs)
	}
	return c
}

func TestDefaultRulesMatchLikeChromaTerm(t *testing.T) {
	g := loadGolden(t)
	c := defaultConfig(t, true)
	byName := map[string]*Rule{}
	for _, rule := range c.Rules {
		byName[rule.Description] = rule
	}
	for _, want := range g.Rules {
		rule := byName[want.Description]
		if rule == nil {
			t.Fatalf("missing default rule %q", want.Description)
		}
		for text, spans := range want.Spans {
			got := rule.matches(text)
			if len(got) != len(spans) {
				t.Errorf("%s on %q: %d matches, want %d (%v)", want.Description, text, len(got), len(spans), spans)
				continue
			}
			for i, span := range spans {
				if got[i].start != span[0] || got[i].end != span[1] {
					t.Errorf("%s on %q: match %d at [%d,%d), want %v", want.Description, text, i, got[i].start, got[i].end, span)
				}
			}
		}
	}
}

func TestDefaultHighlightLikeChromaTerm(t *testing.T) {
	g := loadGolden(t)
	for _, want := range g.Defaults {
		c := defaultConfig(t, want.RGB)
		if got := c.Highlight(want.In); got != want.Out {
			t.Errorf("rgb=%v %q:\n got %q\nwant %q", want.RGB, want.In, got, want.Out)
		}
	}
}

func TestHighlightAlgorithmLikeChromaTerm(t *testing.T) {
	g := loadGolden(t)
	for i, scenario := range g.Algorithm {
		c := NewConfig()
		for _, r := range scenario.Rules {
			colors := map[any]Color{}
			add := func(key any, spec string) {
				color, err := ParseColor(spec, nil, true)
				if err != nil {
					t.Fatal(err)
				}
				colors[key] = color
			}
			switch spec := r.Color.(type) {
			case string:
				add(0, spec)
			case map[string]any:
				for key, value := range spec {
					if n, err := strconv.Atoi(key); err == nil {
						add(n, value.(string))
					} else {
						add(key, value.(string))
					}
				}
			}
			rule, err := NewRule(r.Regex, colors, "", r.Exclusive)
			if err != nil {
				t.Fatal(err)
			}
			c.Rules = append(c.Rules, rule)
		}
		sortExclusiveFirst(c.Rules)
		for j, input := range scenario.Inputs {
			if got := c.Highlight(input); got != scenario.Outputs[j] {
				t.Errorf("scenario %d input %d %q:\n got %q\nwant %q", i, j, input, got, scenario.Outputs[j])
			}
		}
	}
}

func TestSplitBufferLikeChromaTerm(t *testing.T) {
	g := loadGolden(t)
	for _, want := range g.Splits {
		got := splitBuffer([]byte(want.In))
		if len(got) != len(want.Chunks) {
			t.Errorf("%q: %d chunks, want %d", want.In, len(got), len(want.Chunks))
			continue
		}
		for i, c := range want.Chunks {
			if string(got[i].data) != c[0] || string(got[i].sep) != c[1] {
				t.Errorf("%q chunk %d = (%q,%q), want (%q,%q)", want.In, i, got[i].data, got[i].sep, c[0], c[1])
			}
		}
	}
}
