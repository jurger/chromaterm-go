// Package chromaterm colours terminal output by keyword. It is a Go port of
// ChromaTerm 0.10.7 (https://github.com/hSaria/ChromaTerm) that keeps its
// config format, default rules, colour handling and output buffering, so an
// existing ~/.chromaterm.yml works unchanged. No Python is involved.
//
// Highlight text with a Config:
//
//	cfg := chromaterm.NewConfig()
//	cfg.Load(chromaterm.DefaultConfig, true) // true for 24-bit colour
//	fmt.Println(cfg.Highlight("ping 10.0.0.1 failed"))
//
// Highlight a stream as it arrives, waiting briefly for cut-off lines the way
// ChromaTerm does, with (&Stream{Config: cfg}).Run(in, out). To colour an
// interactive program (ssh, a shell), run it with RunInPTY: the program gets
// its own PTY, keystrokes pass through, and its output is highlighted.
// Unlike ChromaTerm, output on the alternate screen (vim, less, htop) is left
// alone.
//
// Rules use ChromaTerm's (Python) regular expression syntax, including
// lookaround and backreferences, through github.com/dlclark/regexp2.
package chromaterm
