package chromaterm_test

import (
	"fmt"
	"strings"

	chromaterm "github.com/jurger/chromaterm-go"
)

func Example() {
	cfg := chromaterm.NewConfig()
	cfg.Load(chromaterm.DefaultConfig, true)
	out := cfg.Highlight("link to 10.0.0.1 is down")
	fmt.Println(strings.ReplaceAll(out, "\x1b", `\e`))
	// Output: link to \e[38;2;0;224;209m10.0.0.1\e[39m is \e[38;2;199;24;0mdown\e[39m
}

func ExampleConfig_Load() {
	cfg := chromaterm.NewConfig()
	errs := cfg.Load(`
palette:
  alert: "#ff0000"
rules:
- description: Failed logins
  regex: (?i)failed password for (\w+)
  color:
    0: f.alert
    1: bold
`, true)
	fmt.Println(len(cfg.Rules), errs)
	// Output: 1 []
}
