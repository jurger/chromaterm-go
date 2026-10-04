package chromaterm

import (
	_ "embed"
	"fmt"
	"sort"

	"gopkg.in/yaml.v3"
)

// DefaultConfig is ChromaTerm's default palette and rules.
//
//go:embed defaults.yml
var DefaultConfig string

// Config highlights data with its rules. It remembers the last colour of each
// type it emitted, so a highlight that ends in a later chunk restores the
// colour the program had set instead of the terminal default.
type Config struct {
	Rules []*Rule

	resets map[string]string
}

// NewConfig returns a config with no rules.
func NewConfig() *Config {
	c := &Config{resets: map[string]string{}}
	for typ, code := range resetCodes {
		c.resets[typ] = code
	}
	return c
}

// Load parses a ChromaTerm YAML config (palette and rules). Rules that fail
// to parse are skipped and reported in the returned errors; a config that
// cannot be read at all leaves c unchanged.
func (c *Config) Load(data string, rgb bool) []error {
	var doc struct {
		Palette map[string]string `yaml:"palette"`
		Rules   []map[string]any  `yaml:"rules"`
	}
	if err := yaml.Unmarshal([]byte(data), &doc); err != nil {
		return []error{fmt.Errorf("parse error: %w", err)}
	}
	if doc.Rules == nil {
		return []error{fmt.Errorf(`parse error: "rules" list not found in configuration`)}
	}
	var palette Palette
	if doc.Palette != nil {
		palette = Palette{}
		names := make([]string, 0, len(doc.Palette))
		for name := range doc.Palette {
			names = append(names, name)
		}
		sort.Strings(names)
		for _, name := range names {
			if err := palette.Add(name, doc.Palette[name]); err != nil {
				return []error{fmt.Errorf("palette color %q: %w", name, err)}
			}
		}
	}
	var errs []error
	var rules []*Rule
	for _, raw := range doc.Rules {
		rule, err := parseRule(raw, palette, rgb)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		rules = append(rules, rule)
	}
	sortExclusiveFirst(rules)
	c.Rules = rules
	return errs
}

// sortExclusiveFirst puts exclusive rules first, so others cannot overlap them.
func sortExclusiveFirst(rules []*Rule) {
	sort.SliceStable(rules, func(i, j int) bool { return rules[i].Exclusive && !rules[j].Exclusive })
}

func parseRule(raw map[string]any, palette Palette, rgb bool) (*Rule, error) {
	description, _ := raw["description"].(string)
	regex, ok := raw["regex"].(string)
	name := fmt.Sprintf("rule %q", truncateRegex(fmt.Sprint(raw["regex"])))
	if description != "" {
		name = fmt.Sprintf("rule %q", description)
	}
	if !ok {
		return nil, fmt.Errorf("%s: regex must be a string", name)
	}
	exclusive := false
	if value, present := raw["exclusive"]; present {
		b, ok := value.(bool)
		if !ok {
			return nil, fmt.Errorf("%s: exclusive must be a boolean", name)
		}
		exclusive = b
	}
	specs := map[any]string{}
	switch color := raw["color"].(type) {
	case string:
		specs[0] = color
	case map[string]any:
		for key, value := range color {
			s, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("%s: color must be a string", name)
			}
			specs[key] = s
		}
	case map[any]any:
		for key, value := range color {
			s, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("%s: color must be a string", name)
			}
			specs[key] = s
		}
	default:
		return nil, fmt.Errorf("%s: color must be a string", name)
	}
	colors := map[any]Color{}
	for key, spec := range specs {
		color, err := ParseColor(spec, palette, rgb)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		colors[key] = color
	}
	rule, err := NewRule(regex, colors, description, exclusive)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return rule, nil
}

func truncateRegex(s string) string {
	if r := []rune(s); len(r) > 30 {
		return string(r[:30])
	}
	return s
}

// matches gathers every rule's coloured spans. An exclusive rule blanks its
// spans with new lines so later rules cannot match there, keeping offsets.
func (c *Config) matches(data string) []match {
	var out []match
	for _, rule := range c.Rules {
		found := rule.matches(data)
		if rule.Exclusive && len(found) > 0 {
			b := []byte(data)
			for _, m := range found {
				for i := m.start; i < m.end; i++ {
					b[i] = '\n'
				}
			}
			data = string(b)
		}
		out = append(out, found...)
	}
	return out
}

// insertIndex locates where a match's start and end inserts go so the
// inserts stay in descending position order.
func insertIndex(start, end int, inserts []insert) (int, int) {
	startIndex, endIndex := -1, -1
	for i, ins := range inserts {
		if startIndex < 0 && start >= ins.pos {
			startIndex = i
		}
		// With overlapping matches other colours sit between start and end,
		// so the end index is located independently.
		if endIndex < 0 && end > ins.pos {
			endIndex = i
		}
		if startIndex >= 0 && endIndex >= 0 {
			return startIndex, endIndex
		}
	}
	if startIndex < 0 {
		startIndex = len(inserts)
	}
	if endIndex < 0 {
		endIndex = len(inserts)
	}
	return startIndex, endIndex
}

// addInserts adds the colour codes for every match to inserts, which hold
// the data's existing colours. Ported from ChromaTerm's Config.get_inserts.
func (c *Config) addInserts(data string, inserts []insert) []insert {
	for _, m := range c.matches(data) {
		startIndex, endIndex := insertIndex(m.start, m.end, inserts)
		for _, part := range m.color.parts {
			// End the highlight with the last colour of this type before the
			// match's end, if any; otherwise with the remembered colour.
			reset := c.resets[part.typ]
			for _, ins := range inserts[endIndex:] {
				if ins.typ == part.typ {
					reset = ins.code
					break
				}
				if ins.reset && ins.typ == "" {
					reset = resetCodes[part.typ]
					break
				}
			}
			startIns := insert{pos: m.start, code: part.code, typ: part.typ}
			endIns := insert{pos: m.end, code: reset, reset: true, typ: part.typ}

			// Resets of this type inside the match would cut the highlight
			// short; turn them into our colour. A full reset moves to our end.
			for i := endIndex; i < startIndex; i++ {
				ins := &inserts[i]
				if ins.reset && (ins.typ == part.typ || ins.typ == "") {
					if ins.typ == "" {
						endIns.code, endIns.reset, endIns.typ = ins.code, ins.reset, ins.typ
					}
					ins.code, ins.reset, ins.typ = part.code, false, part.typ
				}
			}
			inserts = insertAt(inserts, startIndex, startIns)
			inserts = insertAt(inserts, endIndex, endIns)
			startIndex++
			endIndex++
		}
	}
	return inserts
}

func insertAt(inserts []insert, i int, ins insert) []insert {
	inserts = append(inserts, insert{})
	copy(inserts[i+1:], inserts[i:])
	inserts[i] = ins
	return inserts
}

// Highlight returns data with the rules' colours added. Existing colours are
// stripped before matching so they don't interfere, then put back.
func (c *Config) Highlight(data string) string {
	stripped, inserts := stripColors(data)
	inserts = c.addInserts(stripped, inserts)
	if len(inserts) == 0 {
		return data
	}
	pending := map[string]bool{}
	for _, typ := range colorTypes {
		pending[typ] = true
	}
	out := stripped
	for _, ins := range inserts {
		out = out[:ins.pos] + ins.code + out[ins.pos:]
		if len(pending) == 0 {
			continue
		}
		// Inserts run from the end of the data backwards, so the first one of
		// a type is the colour still active afterwards.
		if ins.typ == "" && ins.reset {
			for typ := range pending {
				c.resets[typ] = resetCodes[typ]
			}
			pending = map[string]bool{}
		} else if pending[ins.typ] {
			c.resets[ins.typ] = ins.code
			delete(pending, ins.typ)
		}
	}
	return out
}
