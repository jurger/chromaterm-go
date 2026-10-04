# chromaterm-go

A Go port of [ChromaTerm](https://github.com/hSaria/ChromaTerm) 0.10.7: colour terminal output by keyword (IP and MAC addresses, URLs, dates, times, sizes, numbers, and words like `error`, `failed`, `warning`, `ok`). No Python needed.

It keeps ChromaTerm's config format, default rules, colour handling and output buffering, so an existing `~/.chromaterm.yml` works as is. Its tests replay the original ChromaTerm's output for the default rules and the highlighting algorithm.

```sh
go get github.com/jurger/chromaterm-go
```

## Highlight text

```go
cfg := chromaterm.NewConfig()
cfg.Load(chromaterm.DefaultConfig, true) // true: 24-bit colour; false: xterm-256
fmt.Println(cfg.Highlight("link to 10.0.0.1 is down"))
```

`Highlight` keeps colours already in the text and nests its own inside them. A `Config` remembers the colours it emitted, so feed one stream through one `Config`.

## Highlight a program

`RunInPTY` runs a program in its own PTY: keystrokes pass through untouched, output is highlighted, the window size follows `SIGWINCH`, and the program's exit code is returned. This is how to colour `ssh` or a shell.

```go
code, err := chromaterm.RunInPTY([]string{"ssh", "host"}, cfg, os.Stdin, os.Stdout)
if err != nil {
	// No PTY (e.g. on Windows): run the program without highlighting.
}
os.Exit(code)
```

For a stream that is not a PTY, use `(&chromaterm.Stream{Config: cfg}).Run(in, out)`: it waits briefly for cut-off lines and leaves single typed characters alone, as ChromaTerm does.

Unlike ChromaTerm, output drawn on the alternate screen (vim, less, htop) is not highlighted.

## Config

```yaml
palette:
  alert: "#ff0000"

rules:
- description: Failed logins
  regex: (?i)failed password for (\w+)
  color:
    0: f.alert      # the whole match
    1: bold         # group 1
- description: Hosts
  regex: \bweb-\d+\b
  color: f#00aaff underline
  exclusive: true   # other rules can't colour these characters
```

- **Colours:** `f#rrggbb` / `b#rrggbb` (foreground / background), `f.name` / `b.name` from the palette, and the styles `blink`, `bold`, `invert`, `italic`, `strike` and `underline`, combined with spaces.
- **Regular expressions:** Python syntax, through [regexp2](https://github.com/dlclark/regexp2): lookaround, backreferences, `(?P<name>…)` / `(?P=name)` and inline flags such as `(?ix)`. Group numbers follow Python's order, named groups included.
- **Errors:** `Load` returns one error per rule it had to skip and keeps the rest.

`chromaterm.DefaultConfig` holds ChromaTerm's default palette and rules as YAML; write it to a file to give users something to edit.

## Platforms

Highlighting works everywhere. `RunInPTY` needs a Unix PTY (Linux, macOS, BSD); on Windows it returns an error (`PTYSupported` is false).

## License

MIT. ChromaTerm is © 2020 Saria H.; see [LICENSE](LICENSE).
