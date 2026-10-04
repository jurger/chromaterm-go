package chromaterm

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
)

// Colour types in ChromaTerm's order. fg and bg take a hex colour; the rest
// are styles with a fixed code.
var colorTypes = []string{"fg", "bg", "blink", "bold", "invert", "italic", "strike", "underline"}

var styleNames = []string{"blink", "bold", "invert", "italic", "strike", "underline"}

var styleCodes = map[string]string{
	"blink":     "\x1b[5m",
	"bold":      "\x1b[1m",
	"invert":    "\x1b[7m",
	"italic":    "\x1b[3m",
	"strike":    "\x1b[9m",
	"underline": "\x1b[4m",
}

// resetCodes are the codes that end each colour type.
var resetCodes = map[string]string{
	"fg":        "\x1b[39m",
	"bg":        "\x1b[49m",
	"blink":     "\x1b[25m",
	"bold":      "\x1b[22m",
	"invert":    "\x1b[27m",
	"italic":    "\x1b[23m",
	"strike":    "\x1b[29m",
	"underline": "\x1b[24m",
}

var (
	paletteColorRE = regexp.MustCompile(`\b([bf])\.([a-z0-9_-]+)\b`)
	hexColorRE     = regexp.MustCompile(`([bf])#([0-9a-f]{6})`)
	colorSpecRE    = regexp.MustCompile(`^(([bf]#[0-9a-f]{6}|` + strings.Join(styleNames, "|") + `)(\s+|$))+$`)
	styleRE        = regexp.MustCompile(strings.Join(styleNames, "|"))
	paletteNameRE  = regexp.MustCompile(`^[a-z0-9_-]+$`)
	paletteValueRE = regexp.MustCompile(`^#[0-9a-f]{6}$`)

	// sgrRE is any Select Graphic Rendition sequence; sgrColorRE only the
	// ones made of numeric parameters, which can be decoded into colours.
	sgrRE      = regexp.MustCompile(`\x1b\[[\x30-\x3f]*[\x20-\x2f]*m`)
	sgrColorRE = regexp.MustCompile(`^\x1b\[[0-9;]*m$`)
)

// colorPart is one colour type of a Color and the code that sets it.
type colorPart struct {
	typ  string
	code string
}

// Color is a parsed ChromaTerm colour such as "f#ff0000 bold" or
// "b.status-1".
type Color struct {
	Spec  string
	parts []colorPart
}

// Palette maps colour names to hex colours, for "f.name" and "b.name".
type Palette map[string]string

// Add registers a palette colour.
func (p Palette) Add(name, value string) error {
	name = strings.ToLower(strings.TrimSpace(name))
	value = strings.ToLower(strings.TrimSpace(value))
	if _, reserved := resetCodes[name]; reserved {
		return fmt.Errorf("color name %q is reserved", name)
	}
	if _, exists := p[name]; exists {
		return fmt.Errorf("a color named %q already exists", name)
	}
	if !paletteNameRE.MatchString(name) {
		return fmt.Errorf("color name %q accepts alphanumerics, dashes, and underscores only", name)
	}
	if !paletteValueRE.MatchString(value) {
		return fmt.Errorf("palette color %q must be in `#123abc` format", value)
	}
	p[name] = value
	return nil
}

func (p Palette) resolve(spec string) (string, error) {
	var err error
	resolved := paletteColorRE.ReplaceAllStringFunc(spec, func(match string) string {
		parts := paletteColorRE.FindStringSubmatch(match)
		value, ok := p[parts[2]]
		if !ok {
			err = fmt.Errorf("color %q not in palette", parts[2])
			return match
		}
		return parts[1] + value
	})
	return resolved, err
}

// ParseColor parses a colour spec. rgb selects 24-bit colour codes; otherwise
// hex colours are downscaled to the xterm-256 palette.
func ParseColor(spec string, palette Palette, rgb bool) (Color, error) {
	value := strings.ToLower(strings.TrimSpace(spec))
	normalized := strings.Join(dedupe(strings.Fields(value)), " ")
	if paletteColorRE.MatchString(value) {
		if palette == nil {
			return Color{}, fmt.Errorf("palette color name present, but no palette specified")
		}
		resolved, err := palette.resolve(value)
		if err != nil {
			return Color{}, err
		}
		value = resolved
	}
	if !colorSpecRE.MatchString(value) {
		return Color{}, fmt.Errorf("invalid color format %q", value)
	}
	color := Color{Spec: normalized}
	for _, match := range hexColorRE.FindAllStringSubmatch(value, -1) {
		target, typ := "\x1b[38;", "fg"
		if match[1] == "b" {
			target, typ = "\x1b[48;", "bg"
		}
		for _, part := range color.parts {
			if part.typ == typ {
				return Color{}, fmt.Errorf("color accepts one foreground and one background colors")
			}
		}
		r, _ := strconv.ParseUint(match[2][0:2], 16, 8)
		g, _ := strconv.ParseUint(match[2][2:4], 16, 8)
		b, _ := strconv.ParseUint(match[2][4:6], 16, 8)
		if rgb {
			target += fmt.Sprintf("2;%d;%d;%d", r, g, b)
		} else {
			target += fmt.Sprintf("5;%d", rgbToXterm256(int(r), int(g), int(b)))
		}
		color.parts = append(color.parts, colorPart{typ: typ, code: target + "m"})
	}
	for _, style := range dedupe(styleRE.FindAllString(value, -1)) {
		color.parts = append(color.parts, colorPart{typ: style, code: styleCodes[style]})
	}
	return color, nil
}

func dedupe(values []string) []string {
	seen := map[string]bool{}
	out := values[:0:0]
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			out = append(out, value)
		}
	}
	return out
}

// rgbToXterm256 picks the closest xterm-256 colour, from either the 6x6x6
// colour cube or the grey ramp.
func rgbToXterm256(r, g, b int) int {
	closest := func(value int, steps []int) int {
		best := 0
		for i, step := range steps {
			if abs(step-value) < abs(steps[best]-value) {
				best = i
			}
		}
		return best
	}
	distance := func(nr, ng, nb int) int {
		return (nr-r)*(nr-r) + (ng-g)*(ng-g) + (nb-b)*(nb-b)
	}
	rgbSteps := []int{0, 95, 135, 175, 215, 255}
	ri, gi, bi := closest(r, rgbSteps), closest(g, rgbSteps), closest(b, rgbSteps)
	rgbDistance := distance(rgbSteps[ri], rgbSteps[gi], rgbSteps[bi])

	graySteps := make([]int, 0, 24)
	for step := 8; step < 239; step += 10 {
		graySteps = append(graySteps, step)
	}
	grayIndex := closest((r+g+b)/3, graySteps)
	gray := graySteps[grayIndex]
	if distance(gray, gray, gray) < rgbDistance {
		return 232 + grayIndex
	}
	return 16 + 36*ri + 6*gi + bi
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}

// insert is a colour code to put into the data at pos. reset marks codes
// that end a colour; typ is "" for a full reset or an unrecognised code.
type insert struct {
	pos   int
	code  string
	reset bool
	typ   string
}

// decodeSGR splits an SGR sequence into its individual colours. Codes that
// are not plain numeric colours are returned as a single untyped insert.
func decodeSGR(source string, isReset bool) []insert {
	if !sgrColorRE.MatchString(source) {
		return []insert{{code: source}}
	}
	sgr := func(code string) string { return "\x1b[" + code + "m" }
	codes := strings.Split(strings.TrimRight(strings.TrimLeft(source, "\x1b["), "m"), ";")
	var out []insert
	for i := 0; i < len(codes); i++ {
		code := codes[i]
		n, _ := strconv.Atoi(code)
		switch {
		case code == "" || n == 0:
			out = append(out, insert{code: sgr("0"), reset: true})
		case code == "38" || code == "48":
			typ := "fg"
			if code == "48" {
				typ = "bg"
			}
			switch {
			case len(codes) > i+2 && codes[i+1] == "5":
				code = strings.Join(codes[i:i+3], ";")
				i += 2
			case len(codes) > i+4 && codes[i+1] == "2":
				code = strings.Join(codes[i:i+5], ";")
				i += 4
			default:
				return []insert{{code: source}}
			}
			out = append(out, insert{code: sgr(code), reset: isReset, typ: typ})
		default:
			item := insert{code: sgr(strconv.Itoa(n))}
			if typ := singleCodeType(n); typ != "" {
				item.typ = typ
				item.reset = isReset || item.code == resetCodes[typ]
			}
			out = append(out, item)
		}
	}
	return out
}

// singleCodeType is the colour type a one-number SGR code belongs to.
func singleCodeType(n int) string {
	switch {
	case n >= 30 && n <= 37, n == 39, n >= 90 && n <= 97:
		return "fg"
	case n >= 40 && n <= 47, n == 49, n >= 100 && n <= 107:
		return "bg"
	case n == 5 || n == 25:
		return "blink"
	case n == 1 || n == 2 || n == 22:
		return "bold"
	case n == 7 || n == 27:
		return "invert"
	case n == 3 || n == 23:
		return "italic"
	case n == 9 || n == 29:
		return "strike"
	case n == 4 || n == 24:
		return "underline"
	}
	return ""
}

// stripColors removes SGR sequences from data and returns them as inserts,
// in descending position order, marked as resets so highlights can replace
// them.
func stripColors(data string) (string, []insert) {
	var inserts []insert
	var b strings.Builder
	last, removed := 0, 0
	for _, loc := range sgrRE.FindAllStringIndex(data, -1) {
		b.WriteString(data[last:loc[0]])
		pos := loc[0] - removed
		for _, color := range decodeSGR(data[loc[0]:loc[1]], true) {
			color.pos = pos
			inserts = append([]insert{color}, inserts...)
		}
		removed += loc[1] - loc[0]
		last = loc[1]
	}
	if last == 0 {
		return data, inserts
	}
	b.WriteString(data[last:])
	return b.String(), inserts
}
